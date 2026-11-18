package main

import (
	"log"
	"strings"

	"github.com/egoist/mygo"
)

// setupTray creates the notification-area icon when the setting is on. The
// icon's menu is rebuilt each time it opens, so it always shows the current
// part count and library size.
func (a *app) setupTray() {
	if !a.cfg.TrayEnabled {
		return
	}
	icon := trayIconBytes
	if len(icon) == 0 {
		icon = appIconBytes
	}
	if len(icon) == 0 {
		return
	}
	tray, err := mygo.NewTray(mygo.TrayOptions{
		ID:      "bhs-masterlist",
		Icon:    icon,
		ToolTip: a.trayToolTip(),
		Menu:    a.buildTrayMenu(),
	})
	if err != nil {
		a.trayError = err.Error()
		log.Printf("tray: %v", err)
		return
	}
	a.tray = tray
	a.trayReady = true
	// A left click shows the window; the right click opens the menu, as
	// Windows' own notification-area icons mostly do.
	tray.OnClick(func() {
		a.showFromTray()
	})
	tray.OnRightClick(func() {
		tray.SetMenu(a.buildTrayMenu())
		mygo.RunOnMain(func() { tray.PopUpMenu() })
	})
}

func (a *app) trayToolTip() string {
	base := "电气柜器件库"
	return base + " · " + formatInt(a.summary.Total) + " 件"
}

// refreshTray updates the icon's tooltip after the data or the library
// changed. Tray calls are safe from any goroutine.
func (a *app) refreshTray() {
	if a.tray == nil || !a.trayReady {
		return
	}
	a.tray.SetToolTip(a.trayToolTip())
}

func (a *app) destroyTray() {
	if a.tray != nil {
		a.tray.Destroy()
		a.tray = nil
		a.trayReady = false
	}
}

// applyTraySetting turns the icon on or off to match the setting.
func (a *app) applyTraySetting() {
	if a.cfg.TrayEnabled {
		if !a.trayReady {
			a.setupTray()
		}
	} else {
		a.destroyTray()
		if a.cfg.CloseToTray {
			// Without an icon, closing to "tray" would lose the window.
			a.cfg.CloseToTray = false
			_ = saveConfig(a.exeDir, a.cfg)
		}
	}
}

func (a *app) buildTrayMenu() *mygo.Menu {
	return mygo.NewMenu([]*mygo.MenuItem{
		{
			ID:    "show",
			Label: "打开器件库",
			Click: func(*mygo.MenuItem, *mygo.Window) { a.showFromTray() },
		},
		{
			ID:    "library",
			Label: "手册库文件列表...",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.showFromTray()
				a.openFileList()
			},
		},
		{
			ID:    "scan",
			Label: "重新扫描数据手册",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.scanDatasheetsAsync(false)
			},
		},
		mygo.Separator(),
		{
			ID:      "tray",
			Label:   "关闭窗口时最小化到状态栏",
			Type:    mygo.MenuItemCheckbox,
			Checked: a.cfg.CloseToTray,
			Click: func(item *mygo.MenuItem, _ *mygo.Window) {
				a.cfg.CloseToTray = item.IsChecked()
				_ = saveConfig(a.exeDir, a.cfg)
				a.winInvalidate()
			},
		},
		mygo.Separator(),
		{
			ID:    "quit",
			Label: "退出",
			Click: func(*mygo.MenuItem, *mygo.Window) {
				a.quitting = true
				if a.store != nil {
					_ = a.store.Close()
					a.store = nil
				}
				a.destroyTray()
				mygo.App.Quit()
			},
		},
	})
}

// showFromTray restores the main window and brings it to the front.
func (a *app) showFromTray() {
	if a.win == nil {
		return
	}
	win := a.win
	mygo.RunOnMain(func() {
		if win.IsDestroyed() {
			return
		}
		if !win.IsVisible() {
			win.Show()
		}
		if win.IsMinimized() {
			win.Restore()
		}
		win.Focus()
		win.SetSkipTaskbar(false)
	})
}

// handleClose decides what closing the window does when the setting hides it
// to the tray instead.
func (a *app) handleClose(e *mygo.CloseEvent) {
	if a.quitting || !a.cfg.CloseToTray || !a.cfg.TrayEnabled || !a.trayReady {
		return
	}
	e.PreventDefault()
	a.win.Hide()
	a.win.SetSkipTaskbar(true)
}

func (a *app) winInvalidate() {
	if a.win != nil {
		a.win.Invalidate()
	}
}

// trayStatusLine is the short state the settings tab shows about the tray.
func (a *app) trayStatusLine() string {
	if !a.cfg.TrayEnabled {
		return "状态栏图标已关闭"
	}
	if a.trayError != "" {
		return "状态栏图标创建失败：" + a.trayError
	}
	if !a.trayReady {
		return "状态栏图标未创建（窗口尚未就绪）"
	}
	line := "状态栏图标已启用"
	if a.cfg.CloseToTray {
		line += "，关闭窗口时隐藏到状态栏"
	}
	if strings.HasSuffix(line, "。") {
		line = strings.TrimSuffix(line, "。")
	}
	return line
}
