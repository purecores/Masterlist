package main

import (
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// openFileList shows the manual file-list window: every indexed PDF, its
// folder, size and modification time, with a search box and a sync button.
func (a *app) openFileList() {
	a.fileListOpen = true
	a.fileListQuery = ""
	a.fileListSel = -1
	a.fileListOnlyThisPart = false
	a.fileListDisplay = a.pdfScanner.SearchPDFs("")
	a.fileListScroll.Y = 0
	a.fileListList.ScrollTo(0, ui.Start)
}

func (a *app) refreshFileListDisplay() {
	query := a.fileListQuery
	var files []PDFFile
	if a.fileListOnlyThisPart {
		if p := a.selectedPart(); p != nil {
			files = a.pdfScanner.FindForBHS(p.BHSNo)
		}
	} else {
		files = a.pdfScanner.SearchPDFs(query)
	}
	a.fileListDisplay = files
	if a.fileListSel >= len(files) {
		a.fileListSel = -1
	}
}

// scanDatasheetsAsync rescans the datasheet roots off the UI thread. manual
// reports the result as a toast and keeps the file list window in sync.
func (a *app) scanDatasheetsAsync(manual bool) {
	if a.pdfScanner.Scanning() {
		a.toastAuto("正在扫描数据手册，请稍候")
		return
	}
	a.scanDoneAt = ""
	go func() {
		start := time.Now()
		a.pdfScanner.Scan()
		count := a.pdfScanner.Count()
		elapsed := time.Since(start)
		a.publish(func() {
			a.scanDoneAt = time.Now().Format("15:04:05")
			if a.fileListOpen {
				a.refreshFileListDisplay()
			}
			a.refreshTray()
			if manual {
				a.toastAuto(fmt.Sprintf("数据手册扫描完成：%s 份 PDF，用时 %.1f 秒",
					formatInt(count), elapsed.Seconds()))
			}
		})
	}()
}

// syncFileListAsync runs the manual file-list sync, which merges instead of
// replacing so a temporarily missing network share keeps its entries.
func (a *app) syncFileListAsync() {
	if a.fileListSyncing || a.pdfScanner.Scanning() {
		return
	}
	a.fileListSyncing = true
	a.fileListStatus = "正在同步文件列表..."
	go func() {
		start := time.Now()
		added, removed, kept, err := a.pdfScanner.SyncFileList()
		elapsed := time.Since(start)
		a.publish(func() {
			a.fileListSyncing = false
			a.refreshFileListDisplay()
			a.refreshTray()
			if err != nil {
				a.fileListStatus = "同步失败：" + err.Error()
				return
			}
			a.fileListStatus = fmt.Sprintf(
				"同步完成：新增 %d 份，移除 %d 份，沿用缓存 %d 份，共 %s 份，用时 %.1f 秒",
				added, removed, kept, formatInt(a.pdfScanner.Count()), elapsed.Seconds())
		})
	}()
}

// publish runs fn on the UI thread through the window's update queue, so a
// worker never touches the view's state directly.
func (a *app) publish(fn func()) {
	if a.win == nil {
		fn()
		return
	}
	a.win.Update(fn)
}

// toastAuto queues a message for the next frame, where the view shows it as a
// toast. Toasts need the view's Context, which only frames have.
func (a *app) toastAuto(msg string) {
	a.pendingToast = msg
	if a.win != nil {
		a.win.Invalidate()
	}
}

// fileLibraryMeta is the status line above the file list.
func (a *app) fileLibraryMeta() string {
	count := a.pdfScanner.Count()
	scanned := a.pdfScanner.Scanning()
	since := ""
	if at := a.pdfScanner.ScannedAt(); !at.IsZero() {
		age := time.Since(at)
		switch {
		case a.pdfScanner.FromCache():
			since = fmt.Sprintf("索引来自缓存（%s 前）", humanAge(age))
		case scanned:
			since = "正在扫描..."
		default:
			since = fmt.Sprintf("索引更新于 %s", at.Format("01.02 15:04:05"))
		}
	}
	if since == "" {
		if scanned {
			since = "正在扫描..."
		} else {
			since = "尚未建立索引"
		}
	}
	roots := len(a.pdfScanner.Paths())
	return fmt.Sprintf("%s 份 PDF · %d 个根目录 · %s", formatInt(count), roots, since)
}

func humanAge(d time.Duration) string {
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%d 秒", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天", int(d.Hours()/24))
	}
}

// fileListGroupRows builds the rows of the file list grouped by folder, with
// a header row per folder carrying its file count.
func (a *app) fileListGroupRows(files []PDFFile) (rows []fileRow) {
	grouped := map[string][]PDFFile{}
	order := []string{}
	for _, f := range files {
		dir := f.Dir
		if _, ok := grouped[dir]; !ok {
			order = append(order, dir)
		}
		grouped[dir] = append(grouped[dir], f)
	}
	slices.SortFunc(order, func(x, y string) int { return strings.Compare(strings.ToLower(x), strings.ToLower(y)) })
	for _, dir := range order {
		rows = append(rows, fileRow{dir: dir, file: PDFFile{}, header: true, count: len(grouped[dir])})
		for _, f := range grouped[dir] {
			rows = append(rows, fileRow{dir: dir, file: f})
		}
	}
	return rows
}

type fileRow struct {
	dir    string
	file   PDFFile
	header bool
	count  int
}

// openPDFInDefault opens a datasheet; the setting decides whether the file
// itself or a read-only duplicate is opened.
func (a *app) openPDFInDefault(c *ui.Context, path string) {
	if err := a.OpenDatasheetPDF(path); err != nil {
		c.Toast("打开 PDF 失败: " + err.Error())
		return
	}
	if a.cfg.ReadOnlyPDF {
		c.Toast("已以只读方式打开手册")
	}
}

// revealPDF selects a file in Explorer.
func revealPDF(path string) {
	mygo.Shell.ShowItemInFolder(path)
}
