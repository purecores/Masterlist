package main

import (
	"fmt"
	"strings"

	"github.com/egoist/mygo/ui"
)

// fileListModal is the datasheet library's file list: every indexed PDF by
// folder, searchable, with the manual sync that merges instead of replacing.
func (a *app) fileListModal(c *ui.Context, pal *Palette) {
	// Rows follow the live index: a background scan or a manual sync shows
	// up without the user closing and reopening the window.
	a.refreshFileListDisplay()
	files := a.fileListDisplay
	rows := a.fileListGroupRows(files)

	ui.Column(c).Width(880).MaxHeightPercent(88).Padding(16, 20).Gap(10).Radius(8).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "数据手册文件列表").FontSize(16).Bold().TextColor(pal.Text)
				ui.Text(c, a.fileLibraryMeta()).FontSize(11.5).TextColor(pal.TextMuted).SingleLine()
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.fileListOpen = false
			}
		})
		ui.Divider(c).Background(pal.Border)

		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			if ui.SearchField(c, &a.fileListQuery).Label("搜索手册文件").
				Placeholder("按文件名、BHS 编号或目录搜索").Grow(1).
				Changed() {
				a.refreshFileListDisplay()
			}
			only := a.fileListOnlyThisPart
			onlyBox := ui.Checkbox(c, &only, "仅当前器件").Label("仅当前器件")
			if onlyBox.Changed() {
				a.fileListOnlyThisPart = only
				a.refreshFileListDisplay()
			}
			_ = onlyBox
			syncBtn := ui.PrimaryButton(c, "同步文件列表").Height(30)
			if a.fileListSyncing || a.pdfScanner.Scanning() {
				syncBtn.Disabled(true)
			}
			if syncBtn.Clicked() {
				a.syncFileListAsync()
			}
			rescan := ui.Button(c, "重建索引").Height(30)
			if a.pdfScanner.Scanning() {
				rescan.Disabled(true)
			}
			if rescan.Clicked() {
				a.scanDatasheetsAsync(true)
			}
		})

		if a.fileListStatus != "" {
			msgColor := pal.TextFaint
			bg := pal.Surface
			if strings.Contains(a.fileListStatus, "失败") {
				msgColor = pal.DangerText
				bg = pal.Danger.Alpha(0.12)
			}
			ui.Text(c, a.fileListStatus).FontSize(10.5).TextColor(msgColor).
				Padding(5, 9).Radius(4).Background(bg).Wrap()
		}

		ui.Row(c).Height(28).Padding(0, 12).Gap(8).AlignItems(ui.Center).
			Background(pal.AppBg).BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
			Children(func() {
				ui.Text(c, "文件名 / 目录").FontSize(11).FontWeight(600).TextColor(pal.TextFaint).Grow(1)
				ui.Text(c, "大小").FontSize(11).FontWeight(600).TextColor(pal.TextFaint).Width(74)
				ui.Text(c, "修改时间").FontSize(11).FontWeight(600).TextColor(pal.TextFaint).Width(126)
				ui.Text(c, "操作").FontSize(11).FontWeight(600).TextColor(pal.TextFaint).Width(120)
			})

		if len(rows) == 0 {
			emptyStateView(c, "search", "没有匹配的手册文件",
				"检查手册根目录设置，或点击“重建索引”重新扫描本地与网络共享目录。", pal)
			return
		}

		a.fileListList.Selected = &a.fileListSel
		a.fileListList.Key = func(i int) any {
			if rows[i].header {
				return "h:" + rows[i].dir
			}
			return "f:" + rows[i].file.FullPath
		}
		a.fileListList.Label = func(i int) string {
			if rows[i].header {
				return rows[i].dir
			}
			return rows[i].file.Name
		}
		a.fileListList.Header = func(i int) bool { return rows[i].header }
		list := ui.List(c.Key("pdf-file-list"), &a.fileListList, len(rows)).Grow(1).
			MinHeight(320).RowWidth(812).Label("数据手册文件列表").
			Rows(func(row ui.ListRow) { a.fileListRow(row, rows, pal) })
		if list.Submitted() {
			if i := a.fileListSel; i >= 0 && i < len(rows) && !rows[i].header {
				a.openPDFInDefault(c, rows[i].file.FullPath)
			}
		}

		ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
			ui.Textf(c, "共 %s 份文件 · %d 个文件夹", formatInt(len(files)), a.fileListDirCount(rows)).
				FontSize(10.5).TextColor(pal.TextFaint)
			ui.Spacer(c)
			ui.Text(c, "按文件大小去重的相同手册仅显示一份；此列表包含全部物理文件。").
				FontSize(10).TextColor(pal.TextFaint).SingleLine()
		})
	})
}

func (a *app) fileListDirCount(rows []fileRow) int {
	n := 0
	for _, r := range rows {
		if r.header {
			n++
		}
	}
	return n
}

func (a *app) fileListRow(row ui.ListRow, rows []fileRow, pal *Palette) {
	c := row.Context
	r := rows[row.Index]
	if r.header {
		ui.Row(c.Key("dir:"+r.dir)).Height(26).Padding(0, 12).Gap(6).AlignItems(ui.Center).
			Background(pal.Surface).BorderWidth(1, 0, 1, 0).BorderColor(pal.BorderSubtle).
			Children(func() {
				if ic := icon("folder-open"); ic != nil {
					ui.Icon(c, ic).FontSize(12).TextColor(pal.TextFaint)
				}
				ui.Text(c, r.dir).FontSize(10.5).FontWeight(600).
					TextColor(pal.TextMuted).Grow(1).SingleLine()
				ui.Textf(c, "%d 份", r.count).FontSize(10).Font("monospace").
					TextColor(pal.TextFaint).SingleLine()
			})
		return
	}

	f := r.file
	selected := row.Selected()
	background := pal.RowEven
	if selected {
		background = pal.RowSelected
	} else if row.Index%2 == 0 {
		background = pal.RowOdd
	}
	ur := ui.Row(c.Key("file:"+f.FullPath)).Height(32).Padding(0, 12).Gap(8).
		AlignItems(ui.Center).Background(background)
	if !selected && ur.Hovered() {
		ur.Background(pal.RowHover)
	}
	ur.Children(func() {
		ui.Column(c).Grow(1).MinWidth(0).Gap(0).Children(func() {
			ui.Text(c, f.Name).FontSize(11.5).FontWeight(500).TextColor(pal.Text).SingleLine()
			ui.Text(c, f.Dir).FontSize(9.5).TextColor(pal.TextFaint).SingleLine()
		})
		ui.Text(c, formatFileSize(f.Size)).FontSize(10.5).Font("monospace").
			TextColor(pal.TextMuted).Width(74).SingleLine()
		ui.Text(c, formatModTime(f.ModTime)).FontSize(10.5).Font("monospace").
			TextColor(pal.TextMuted).Width(126).SingleLine()
		ui.Row(c).Width(120).Gap(4).AlignItems(ui.Center).Children(func() {
			open := ui.ButtonBase(c.Key(fmt.Sprintf("fl-open:%d", row.Index))).
				Height(22).Padding(0, 7).Radius(4).Background(pal.Accent).
				TextColor(pal.AccentText).AlignItems(ui.Center)
			if open.Hovered() {
				open.Background(pal.AccentHover)
			}
			open.Children(func() {
				ui.Text(c, "打开").FontSize(10.5).FontWeight(600).TextColor(pal.AccentText)
			})
			if open.Clicked() {
				a.onPDFRowClick(row.Index, rows, nil)
			}
			loc := ui.ButtonBase(c.Key(fmt.Sprintf("fl-loc:%d", row.Index))).
				Height(22).Padding(0, 7).Radius(4).Background(pal.SurfaceHover).
				Border(1, pal.Border).AlignItems(ui.Center)
			loc.Children(func() {
				ui.Text(c, "定位").FontSize(10.5).TextColor(pal.TextMuted)
			})
			if loc.Clicked() {
				revealPDF(f.FullPath)
			}
		})
	})
}

// onPDFRowClick opens a file-list row; context is nil when a row button asked.
func (a *app) onPDFRowClick(index int, rows []fileRow, c *ui.Context) {
	if index < 0 || index >= len(rows) || rows[index].header {
		return
	}
	if c != nil {
		a.openPDFInDefault(c, rows[index].file.FullPath)
		return
	}
	if err := a.OpenDatasheetPDF(rows[index].file.FullPath); err != nil {
		a.toastAuto("打开 PDF 失败: " + err.Error())
	}
}
