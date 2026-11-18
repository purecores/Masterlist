package main

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// autoSyncState owns the background refresh that keeps the local workbook
// copies and the imported data current. It is cancelled and re-armed when the
// setting or the source folders change.
type autoSyncState struct {
	mu         sync.Mutex
	stop       chan struct{}
	jobRunning bool
	last       time.Time
	note       string
}

// startAutoSync runs the periodic sync when the interval is above zero.
func (a *app) startAutoSync() {
	a.autoSync.mu.Lock()
	defer a.autoSync.mu.Unlock()
	if a.autoSync.stop != nil {
		return
	}
	interval := a.autoSyncInterval()
	if interval <= 0 {
		return
	}
	stop := make(chan struct{})
	a.autoSync.stop = stop
	go a.autoSyncLoop(stop, interval)
}

// restartAutoSync cancels the running loop and arms it again with the
// interval and folders the settings now hold.
func (a *app) restartAutoSync() {
	a.stopAutoSync()
	a.startAutoSync()
}

func (a *app) stopAutoSync() {
	a.autoSync.mu.Lock()
	defer a.autoSync.mu.Unlock()
	if a.autoSync.stop != nil {
		close(a.autoSync.stop)
		a.autoSync.stop = nil
	}
}

func (a *app) autoSyncInterval() time.Duration {
	return time.Duration(a.cfg.AutoSyncMinutes) * time.Minute
}

func (a *app) autoSyncLoop(stop chan struct{}, interval time.Duration) {
	// A short first wait, so a launch after a long break catches up quickly,
	// then the configured interval.
	wait := min(interval, 5*time.Minute)
	timer := time.NewTimer(wait)
	defer timer.Stop()
	for {
		select {
		case <-stop:
			return
		case <-timer.C:
		}
		a.publish(func() { a.runAutoSync() })
		select {
		case <-stop:
			return
		case <-time.After(interval):
		}
	}
}

// runAutoSync copies the newest workbook versions and imports from them, on
// the app's own goroutine so the UI never blocks on a share.
func (a *app) runAutoSync() {
	store := a.store
	if store == nil || a.cfg.AutoSyncMinutes <= 0 {
		return
	}
	a.autoSync.mu.Lock()
	if a.autoSync.jobRunning {
		a.autoSync.mu.Unlock()
		return
	}
	a.autoSync.jobRunning = true
	a.autoSync.mu.Unlock()

	paths := append([]string(nil), a.cfg.MasterlistPaths...)
	exeDir := a.exeDir
	go func() {
		res, err := syncExcelMasterlist(context.Background(), store, exeDir, paths, false)
		a.publish(func() {
			a.autoSync.mu.Lock()
			a.autoSync.last = time.Now()
			a.autoSync.jobRunning = false
			if err != nil {
				a.autoSync.note = "自动同步失败：" + err.Error()
			} else {
				a.autoSync.note = fmt.Sprintf("自动同步完成：新增 %d，更新 %d",
					res.NewPartsAdded, res.UpdatedParts)
				if res.NewPartsAdded > 0 || res.UpdatedParts > 0 {
					_ = a.reload(0)
				}
			}
			a.autoSync.mu.Unlock()
			_ = res
		})
	}()
}
