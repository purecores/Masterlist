package main

import (
	"fmt"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) view(c *ui.Context) {
	pal := a.theme()
	c.SetTheme(setupTheme(c.Theme(), a.lightTheme))
	if a.loading {
		a.loadingView(c, pal)
		return
	}
	if a.loadErr != "" {
		a.errorView(c, pal)
		return
	}
	if c.Shortcut(ui.Cmd, ui.KeyF) {
		a.searchHandle.Focus(c)
	}
	ui.Row(c).Fill().Background(pal.AppBg).
		AlignItems(ui.Stretch).Children(func() {
		a.sidebar(c, pal)
		a.content(c, pal)
	})
	ui.Modal(c, &a.formOpen, func() { a.form(c, pal) })
	ui.Modal(c, &a.datasheetModalOpen, func() { a.settingsModal(c, pal) })
	ui.Modal(c, &a.syncModalOpen, func() { a.syncModal(c, pal) })
	ui.Modal(c, &a.fileListOpen, func() { a.fileListModal(c, pal) })
	if a.pendingToast != "" {
		msg := a.pendingToast
		a.pendingToast = ""
		c.Toast(msg)
	}
}

func (a *app) loadingView(c *ui.Context, pal *Palette) {
	ui.Row(c).Fill().Background(pal.AppBg).
		AlignItems(ui.Stretch).Children(func() {
		a.sidebarHeader(c, pal)
		ui.Column(c).Grow(1).Center().Gap(12).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在加载器件库").FontSize(18).Bold().TextColor(pal.Text)
			ui.Text(c, "读取原表结构、器件记录和维护状态").
				FontSize(12.5).TextColor(pal.TextMuted)
		})
	})
}

func (a *app) errorView(c *ui.Context, pal *Palette) {
	ui.Row(c).Fill().Background(pal.AppBg).
		AlignItems(ui.Stretch).Children(func() {
		a.sidebarHeader(c, pal)
		ui.Column(c).Grow(1).Center().Gap(10).Padding(40).Children(func() {
			if ic := icon("circle-alert"); ic != nil {
				ui.Icon(c, ic).FontSize(32).TextColor(pal.Danger)
			}
			ui.Text(c, "无法打开器件库").FontSize(20).Bold().TextColor(pal.Text)
			ui.Text(c, a.loadErr).FontSize(13).TextColor(pal.TextMuted).
				MaxWidth(680).Wrap()
			if a.dbPath != "" {
				ui.Textf(c, "数据库目标路径：%s", a.dbPath).
					FontSize(11).Font("monospace").TextColor(pal.TextFaint)
			}
			ui.Row(c).Margin(10, 0, 0, 0).Children(func() {
				if ui.PrimaryButton(c, "重试打开数据库").Height(32).Clicked() {
					a.loadErr = ""
					a.loading = true
					go a.loadData(a.win)
				}
			})
		})
	})
}

func (a *app) sidebar(c *ui.Context, pal *Palette) {
	bar := c.TitleBar()
	topPadding := max(bar.Height, 20) + 4
	ui.Column(c).Width(216).Shrink(0).FillHeight().
		Background(pal.SidebarBg).
		BorderWidth(0, 1, 0, 0).BorderColor(pal.Border).
		Padding(topPadding, 10, 10, 10).Gap(4).DragWindow().Children(func() {
		a.sidebarHeader(c, pal)
		a.allSourcesItem(c, pal)
		ui.Row(c).Padding(8, 8, 3, 8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "原表目录").FontSize(10.5).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.TextFaint)
			ui.Spacer(c)
			ui.Textf(c, "%d 份", len(a.books)).FontSize(10).Font("monospace").
				TextColor(pal.TextFaint)
		})
		ui.Scroll(c).Grow(1).Gap(2).Children(func() {
			for i := range a.books {
				a.renderSidebarItem(c, i, pal)
			}
		})
		ui.Divider(c).Background(pal.Border).Margin(2, 0)
		a.sidebarFooter(c, pal)
	})
}

func (a *app) sidebarHeader(c *ui.Context, pal *Palette) {
	ui.Row(c).Height(38).Padding(0, 6).Gap(9).AlignItems(ui.Center).Children(func() {
		if appIconSource != nil {
			ui.Image(c, appIconSource).Size(28, 28).Radius(6)
		} else {
			ui.Box(c).Size(28, 28).Center().Radius(6).Background(pal.Accent).Children(func() {
				ui.Text(c, "BHS").FontSize(9.5).Bold().LetterSpacing(0.5).TextColor(pal.AccentText)
			})
		}
		ui.Column(c).Gap(0).Children(func() {
			ui.Text(c, "电气柜元件库").FontSize(13.5).Bold().LetterSpacing(-0.2).TextColor(pal.Text)
			ui.Text(c, "MASTERLIST · 工业工控").FontSize(8.5).Bold().LetterSpacing(0.8).
				TextColor(pal.TextFaint)
		})
	})
}

func (a *app) allSourcesItem(c *ui.Context, pal *Palette) {
	selected := a.workbookSel == -1
	b := ui.ButtonBase(c.Key("nav:all")).Height(34).Padding(0, 8, 0, 4).Gap(8).
		Radius(6).AlignItems(ui.Center)
	if selected {
		b.Background(pal.AccentCyanBg)
	} else if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	if b.Clicked() {
		a.workbookSel = -1
		a.sheetSel = -1
		a.refreshResults(0)
	}
	iconColor := pal.TextFaint
	textColor := pal.TextMuted
	badgeColor := pal.TextFaint
	if selected {
		iconColor = pal.AccentCyan
		textColor = pal.Text
		badgeColor = pal.AccentCyan
	}
	b.Children(func() {
		if selected {
			ui.Box(c).Size(3, 14).Radius(1.5).Background(pal.AccentCyan)
		} else {
			ui.Box(c).Size(3, 14).Invisible()
		}
		if ic := icon("layout-dashboard"); ic != nil {
			ui.Icon(c, ic).FontSize(15).TextColor(iconColor)
		}
		ui.Text(c, "全部原表").FontSize(12.5).FontWeight(500).
			TextColor(textColor).Grow(1).SingleLine()
		ui.Textf(c, "%s", formatInt(a.summary.Total)).FontSize(10.5).
			Font("monospace").FontWeight(600).TextColor(badgeColor)
	})
}

func (a *app) renderSidebarItem(c *ui.Context, index int, pal *Palette) {
	selected := a.workbookSel == index
	book := &a.books[index]
	title := categoryName(book.Category)
	count := book.Count

	b := ui.ButtonBase(c.Key("source:"+book.SourceFile)).Height(32).
		Padding(0, 8, 0, 4).Gap(8).Radius(6).AlignItems(ui.Center)
	if selected {
		b.Background(pal.AccentCyanBg)
	} else if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	if b.Clicked() {
		a.workbookSel = index
		a.sheetSel = -1
		a.refreshResults(0)
	}
	iconColor := pal.TextFaint
	textColor := pal.TextMuted
	badgeColor := pal.TextFaint
	if selected {
		iconColor = pal.AccentCyan
		textColor = pal.Text
		badgeColor = pal.AccentCyan
	}
	b.Children(func() {
		if selected {
			ui.Box(c).Size(3, 14).Radius(1.5).Background(pal.AccentCyan)
		} else {
			ui.Box(c).Size(3, 14).Invisible()
		}
		if ic := icon("folder-open"); ic != nil {
			ui.Icon(c, ic).FontSize(14).TextColor(iconColor)
		}
		ui.Text(c, title).FontSize(12).FontWeight(500).
			TextColor(textColor).Grow(1).SingleLine()
		ui.Textf(c, "%s", formatInt(count)).FontSize(10.5).
			Font("monospace").TextColor(badgeColor).SingleLine()
	})
}

func (a *app) sidebarFooter(c *ui.Context, pal *Palette) {
	ui.Column(c).Padding(4, 6).Gap(4).Children(func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success)
			ui.Text(c, "数据已同步").FontSize(11).Bold().TextColor(pal.Text)
			ui.Spacer(c)
			syncBtn := iconButton(c, "rotate-cw", "检测/同步原表 Excel", pal)
			if syncBtn.Clicked() {
				a.triggerSync(c, false)
			}
		})
		ui.Textf(c, "%d 份原表 · %d 个工作表", a.summary.Files, a.summary.Sheets).
			FontSize(10.5).TextColor(pal.TextMuted)
		ui.Text(c, formatImportedAt(a.summary.ImportDate)).
			FontSize(10).Font("monospace").TextColor(pal.TextFaint)
	})
}

func (a *app) content(c *ui.Context, pal *Palette) {
	ui.Column(c).Grow(1).MinWidth(0).Background(pal.MainBg).Children(func() {
		a.pageHeader(c, pal)
		a.scopeStrip(c, pal)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			a.catalogBody(c, pal)
			if a.inspectorOpen && a.selectedPart() != nil {
				a.detailPanel(c, pal)
			}
		})
		a.statusBar(c, pal)
	})
}

func (a *app) pageHeader(c *ui.Context, pal *Palette) {
	bar := c.TitleBar()
	topPadding := max(bar.Height-28, 10)
	// Reserve 148px on the right to cleanly avoid Windows window caption buttons
	rightPadding := max(bar.Right+16, 148)

	ui.Row(c).Height(54).Padding(topPadding, rightPadding, 0, 16).Gap(8).
		AlignItems(ui.Center).Background(pal.MainBg).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		DragWindow().Children(func() {
		ui.Row(c).Shrink(0).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, a.pageTitle()).FontSize(17).Bold().
				LetterSpacing(-0.2).TextColor(pal.Text).SingleLine().Shrink(0)
			ui.Row(c).Padding(2, 6).Radius(4).Background(pal.Surface).
				Border(1, pal.Border).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "%s 件", formatInt(len(a.results))).FontSize(11).
					Font("monospace").FontWeight(600).TextColor(pal.TextMuted).SingleLine()
			})
			if sheet := a.selectedSheet(); sheet != "" {
				ui.Text(c, "/").FontSize(12).TextColor(pal.TextFaint)
				ui.Text(c, sheet).FontSize(12.5).FontWeight(600).
					TextColor(pal.AccentCyan).SingleLine().Shrink(0)
			}
		})

		ui.Spacer(c)

		if ui.SearchField(c, &a.query).Label("搜索器件").
			Placeholder(a.searchPlaceholder()).Width(190).
			Bind(&a.searchHandle).
			Changed() {
			a.refreshResults(0)
			a.detailScroll.Y = 0
		}

		a.filterMenu(c, pal)
		a.sortMenu(c, pal)

		themeIcon := "sun"
		themeTip := "切换至浅色模式"
		if a.lightTheme {
			themeIcon = "moon"
			themeTip = "切换至深色模式"
		}
		themeBtn := iconButton(c, themeIcon, themeTip, pal)
		if themeBtn.Clicked() {
			a.toggleTheme(c)
		}

		infoBtn := iconButton(c, "columns-3", "详情栏开关", pal)
		if a.inspectorOpen && a.selectedPart() != nil {
			infoBtn.Background(pal.AccentCyanBg).TextColor(pal.AccentCyan)
		}
		if infoBtn.Clicked() {
			a.inspectorOpen = !a.inspectorOpen
		}

		dsBtn := ui.ButtonBase(c.Key("header-datasheet-btn")).Height(30).Padding(0, 9).Gap(6).
			Radius(6).Border(1, pal.Border).Background(pal.Surface).AlignItems(ui.Center)
		if dsBtn.Hovered() {
			dsBtn.Background(pal.SurfaceHover)
		}
		dsBtn.Children(func() {
			if ic := icon("file-down"); ic != nil {
				ui.Icon(c, ic).FontSize(13).TextColor(pal.AccentCyan)
			}
			ui.Textf(c, "手册库 (%s)", formatInt(a.pdfScanner.Count())).
				FontSize(11.5).FontWeight(500).TextColor(pal.Text)
		})
		if dsBtn.Clicked() {
			a.openFileList()
		}

		setBtn := iconButton(c, "settings", "目录与设置", pal)
		if setBtn.Clicked() {
			a.openDatasheetSettings()
		}

		syncTopBtn := ui.ButtonBase(c.Key("header-sync-btn")).Height(30).Padding(0, 9).Gap(5).
			Radius(6).Border(1, pal.Border).Background(pal.Surface).AlignItems(ui.Center)
		if syncTopBtn.Hovered() {
			syncTopBtn.Background(pal.SurfaceHover)
		}
		syncTopBtn.Children(func() {
			if ic := icon("rotate-cw"); ic != nil {
				ui.Icon(c, ic).FontSize(13).TextColor(pal.AccentCyan)
			}
			ui.Text(c, "同步原表").FontSize(11.5).FontWeight(500).TextColor(pal.Text)
		})
		if syncTopBtn.Clicked() {
			a.triggerSync(c, false)
		}

		add := ui.PrimaryButton(c, "+ 新增器件").Height(30)
		if add.Clicked() {
			a.openAdd()
		}
	})
}

func (a *app) pageTitle() string {
	if book := a.selectedBook(); book != nil {
		return categoryName(book.Category)
	}
	return "全部器件"
}

func (a *app) searchPlaceholder() string {
	if sheet := a.selectedSheet(); sheet != "" {
		return "在当前工作表中搜索"
	}
	if book := a.selectedBook(); book != nil {
		return "在 " + categoryName(book.Category) + " 中搜索"
	}
	return "搜索 BHS、型号、订货号..."
}

func (a *app) scopeStrip(c *ui.Context, pal *Palette) {
	book := a.selectedBook()
	if book == nil {
		ui.Row(c).Height(34).Padding(0, 16).AlignItems(ui.Center).Gap(10).
			BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
			Background(pal.AppBg).Children(func() {
			ui.Text(c, "提示：点击左侧任一原表分类，可快速切入对应的工作表浏览元器件。").
				FontSize(11.5).TextColor(pal.TextFaint).SingleLine()
			ui.Spacer(c)
			a.activeFilterChipsRow(c, pal)
		})
		return
	}
	// The sheet selector is a searchable list, not a sliding tab strip:
	// with many worksheets a strip cut names off and its overlay scrollbar
	// covered the tabs, which hid the sheet the user was looking at.
	a.ensureSheetChoice(book)
	ui.Row(c).Height(38).Padding(0, 16).AlignItems(ui.Center).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		Background(pal.AppBg).Children(func() {
		ui.Text(c, "工作表:").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Shrink(0)
		prev := ui.ButtonBase(c.Key("sheet-prev")).Size(22, 24).Radius(4).
			Border(1, pal.Border).Background(pal.Surface).Center().
			Label("上一个工作表").Tooltip("上一个工作表")
		if prev.Hovered() {
			prev.Background(pal.SurfaceHover).TextColor(pal.Text)
		}
		prev.Children(func() {
			if ic := icon("chevron-left"); ic != nil {
				ui.Icon(c, ic).FontSize(12).TextColor(pal.TextMuted)
			}
		})
		if prev.Clicked() {
			a.stepSheet(-1)
		}
		if ui.Combobox(c, &a.sheetChoice, a.sheetChoices).
			Label("工作表").Width(230).Changed() {
			a.chooseSheet(a.sheetChoice)
		}
		next := ui.ButtonBase(c.Key("sheet-next")).Size(22, 24).Radius(4).
			Border(1, pal.Border).Background(pal.Surface).Center().
			Label("下一个工作表").Tooltip("下一个工作表")
		if next.Hovered() {
			next.Background(pal.SurfaceHover).TextColor(pal.Text)
		}
		next.Children(func() {
			if ic := icon("chevron-right"); ic != nil {
				ui.Icon(c, ic).FontSize(12).TextColor(pal.TextMuted)
			}
		})
		if next.Clicked() {
			a.stepSheet(1)
		}
		ui.Text(c, a.sheetSummary(book)).FontSize(10.5).TextColor(pal.TextFaint).SingleLine()
		ui.Spacer(c)
		ui.Textf(c, "共 %d 个工作表", len(book.Sheets)).FontSize(10.5).
			TextColor(pal.TextFaint).SingleLine().Shrink(0)
		if a.filterActive() {
			ui.Box(c).Width(1).Height(20).Background(pal.Border).Shrink(0)
			a.activeFilterChipsRow(c, pal)
		}
	})
}

// sheetOptions builds the labels of the sheet selector: the whole workbook
// first, then every worksheet with its part count.
func (a *app) sheetOptions(book *WorkbookInfo) []string {
	out := make([]string, 0, len(book.Sheets)+1)
	out = append(out, fmt.Sprintf("全部工作表 (%s 件)", formatInt(book.Count)))
	for _, sheet := range book.Sheets {
		out = append(out, fmt.Sprintf("%s (%s 件)", sheet.Name, formatInt(sheet.Count)))
	}
	return out
}

// ensureSheetChoice keeps the selector showing the current sheet, also after
// the workbook was switched or the data reloaded.
func (a *app) ensureSheetChoice(book *WorkbookInfo) {
	if a.sheetChoiceFor == book.SourceFile && a.sheetChoices != nil &&
		slices.Index(a.sheetChoices, a.sheetChoice) == a.sheetSel+1 {
		return
	}
	a.sheetChoices = a.sheetOptions(book)
	a.sheetChoiceFor = book.SourceFile
	if a.sheetSel >= -1 && a.sheetSel < len(book.Sheets) {
		a.sheetChoice = a.sheetChoices[a.sheetSel+1]
	} else {
		a.sheetChoice = a.sheetChoices[0]
	}
}

func (a *app) chooseSheet(label string) {
	book := a.selectedBook()
	if book == nil {
		return
	}
	index := slices.Index(a.sheetChoices, label)
	if index < 0 {
		return
	}
	a.sheetSel = index - 1
	a.ensureSheetChoice(book)
	a.refreshResults(0)
	a.detailScroll.Y = 0
}

func (a *app) stepSheet(delta int) {
	book := a.selectedBook()
	if book == nil || len(book.Sheets) == 0 {
		return
	}
	next := a.sheetSel + delta
	if next < -1 {
		next = len(book.Sheets) - 1
	}
	if next >= len(book.Sheets) {
		next = -1
	}
	a.sheetSel = next
	a.ensureSheetChoice(book)
	a.refreshResults(0)
	a.detailScroll.Y = 0
}

func (a *app) sheetSummary(book *WorkbookInfo) string {
	sheet := a.selectedSheet()
	if sheet == "" {
		return fmt.Sprintf("显示全部 %s 件器件", formatInt(book.Count))
	}
	count := 0
	for _, s := range book.Sheets {
		if s.Name == sheet {
			count = s.Count
			break
		}
	}
	return fmt.Sprintf("当前工作表 %s · %s 件", sheet, formatInt(count))
}

// activeFilterChipsRow draws the chips of the filters that are on and the
// button that clears them all.
func (a *app) activeFilterChipsRow(c *ui.Context, pal *Palette) {
	if !a.filterActive() {
		return
	}
	ui.Row(c).Shrink(0).Gap(4).AlignItems(ui.Center).Children(func() {
		a.activeFilterChips(c, pal)
		clear := ui.ButtonBase(c.Key("chip:clear-all")).Height(20).Padding(0, 7).
			Radius(10).Border(1, pal.Border).AlignItems(ui.Center).Label("清除筛选")
		clear.Children(func() {
			ui.Text(c, "清除筛选").FontSize(10.5).TextColor(pal.TextMuted).SingleLine()
		})
		if clear.Clicked() {
			a.clearFilters()
		}
	})
}

func (a *app) sheetTab(c *ui.Context, index int, label string, count int, pal *Palette) {
	selected := a.sheetSel == index
	tab := ui.ButtonBase(c.Key(fmt.Sprintf("sheet:%d", index))).Label(label).
		Height(24).Padding(0, 8).Gap(5).Radius(4).AlignItems(ui.Center)
	if selected {
		tab.Background(pal.Accent)
	} else {
		tab.Background(pal.Surface).Border(1, pal.Border)
		if tab.Hovered() {
			tab.Background(pal.SurfaceHover)
		}
	}
	textColor := pal.TextMuted
	countColor := pal.TextFaint
	if selected {
		textColor = pal.AccentText
		countColor = pal.AccentText.Alpha(0.8)
	}
	tab.Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(500).
			TextColor(textColor).SingleLine()
		ui.Textf(c, "%s", formatInt(count)).FontSize(10).Font("monospace").
			TextColor(countColor)
	})
	if tab.Clicked() {
		a.sheetSel = index
		a.refreshResults(0)
		a.detailScroll.Y = 0
	}
}

func (a *app) catalogBody(c *ui.Context, pal *Palette) {
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		if len(a.results) == 0 {
			a.emptyResults(c, pal)
			return
		}
		ui.Column(c).Grow(1).MinWidth(0).Children(func() {
			head := a.tableHeader(c, pal)
			list := a.renderResultList(c, pal)
			head.ScrollWith(list)
		})
	})
}

// setSort switches the ordering, keeping the list scrolled at its start.
// The app remembers it, so the next launch starts the same way.
func (a *app) setSort(label string) {
	a.sortLabel = label
	a.cfg.SortMode = sortModeFromLabel(label)
	a.cfg.SortDesc = a.sortDesc
	_ = saveConfig(a.exeDir, a.cfg)
	a.refreshResults(0)
}

func (a *app) setSortDesc(desc bool) {
	a.sortDesc = desc
	a.cfg.SortDesc = desc
	_ = saveConfig(a.exeDir, a.cfg)
	a.refreshResults(0)
}

func (a *app) sortMenu(c *ui.Context, pal *Palette) {
	dirIcon := "arrow-up"
	if a.sortDesc {
		dirIcon = "arrow-down"
	}
	b := ui.ButtonBase(c.Key("sort-menu")).Height(30).Padding(0, 9).Gap(5).
		Radius(6).Border(1, pal.Border).Background(pal.Surface).
		AlignItems(ui.Center).Label("排序: " + a.sortLabel + descLabel(a.sortDesc)).Tooltip("排序方式")
	if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		if ic := icon(dirIcon); ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(pal.TextMuted)
		}
		ui.Text(c, a.sortLabel).FontSize(11.5).FontWeight(500).TextColor(pal.Text)
		if ic := icon("chevron-down"); ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(pal.TextMuted)
		}
	})
	b.Menu(func(m *ui.Menu) {
		for _, option := range sortModeOptions() {
			if m.Item(option).Checked(a.sortLabel == option).Chosen() {
				a.setSort(option)
			}
		}
		m.Separator()
		if m.Item("升序 (A → Z)").Checked(!a.sortDesc).Chosen() {
			a.setSortDesc(false)
		}
		if m.Item("降序 (Z → A)").Checked(a.sortDesc).Chosen() {
			a.setSortDesc(true)
		}
	})
}

func descLabel(desc bool) string {
	if desc {
		return " ↓"
	}
	return " ↑"
}

// clearFilters turns every extra list filter off, keeping the sidebar scope.
func (a *app) clearFilters() {
	a.filterStatus = 0
	a.filterCategory = ""
	a.filterMfr = ""
	a.filterTypes = 0
	a.cfg.CategoryFilter = ""
	a.cfg.ManufacturerFilter = ""
	a.cfg.DatasheetOnly = false
	_ = saveConfig(a.exeDir, a.cfg)
	a.refreshResults(0)
}

// filterMenu is a single menu holding every list filter: status, category,
// manufacturer and datasheet presence. Its label shows how many are on.
func (a *app) filterMenu(c *ui.Context, pal *Palette) {
	active := 0
	if a.filterStatus > 0 {
		active++
	}
	if a.filterCategory != "" {
		active++
	}
	if a.filterMfr != "" {
		active++
	}
	if a.filterTypes > 0 {
		active++
	}
	b := ui.ButtonBase(c.Key("filter-menu")).Height(30).Padding(0, 9).Gap(5).
		Radius(6).Border(1, pal.Border).Background(pal.Surface).
		AlignItems(ui.Center).Label("筛选器件").Tooltip("筛选器件")
	if active > 0 {
		b.Border(1, pal.AccentCyan).Background(pal.AccentCyanBg)
	}
	if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		if ic := icon("sliders-horizontal"); ic != nil {
			color := pal.TextMuted
			if active > 0 {
				color = pal.AccentCyan
			}
			ui.Icon(c, ic).FontSize(12).TextColor(color)
		}
		label := "筛选"
		if active > 0 {
			label = fmt.Sprintf("筛选 %d", active)
		}
		textColor := pal.Text
		if active > 0 {
			textColor = pal.AccentCyan
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(500).TextColor(textColor)
		if ic := icon("chevron-down"); ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(pal.TextMuted)
		}
	})
	b.Menu(func(m *ui.Menu) {
		m.Submenu("状态", func(sm *ui.Menu) {
			for i, option := range filterStatusOptions {
				if sm.Item(option).Checked(a.filterStatus == i).Chosen() {
					a.filterStatus = i
					_ = saveConfig(a.exeDir, a.cfg)
					a.refreshResults(0)
				}
			}
		})
		m.Submenu("类别", func(sm *ui.Menu) {
			if sm.Item("全部类别").Checked(a.filterCategory == "").Chosen() {
				a.filterCategory = ""
				a.cfg.CategoryFilter = ""
				_ = saveConfig(a.exeDir, a.cfg)
				a.refreshResults(0)
			}
			for _, cat := range a.categoryList() {
				if sm.Item(categoryName(cat)).Checked(a.filterCategory == cat).Chosen() {
					a.filterCategory = cat
					a.cfg.CategoryFilter = cat
					_ = saveConfig(a.exeDir, a.cfg)
					a.refreshResults(0)
				}
			}
		})
		m.Submenu("数据手册", func(sm *ui.Menu) {
			for i, option := range filterTypeOptions {
				if sm.Item(option).Checked(a.filterTypes == i).Chosen() {
					a.filterTypes = i
					a.cfg.DatasheetOnly = i == 1
					_ = saveConfig(a.exeDir, a.cfg)
					a.refreshResults(0)
				}
			}
		})
		m.Submenu("品牌 / 供应商", func(sm *ui.Menu) {
			if sm.Item("全部品牌").Checked(a.filterMfr == "").Chosen() {
				a.filterMfr = ""
				a.cfg.ManufacturerFilter = ""
				_ = saveConfig(a.exeDir, a.cfg)
				a.refreshResults(0)
			}
			for _, mfr := range a.manufacturerList() {
				if sm.Item(mfr).Checked(strings.EqualFold(a.filterMfr, mfr)).Chosen() {
					a.filterMfr = mfr
					a.cfg.ManufacturerFilter = mfr
					_ = saveConfig(a.exeDir, a.cfg)
					a.refreshResults(0)
				}
			}
		})
		m.Separator()
		if m.Item("清除全部筛选").Disabled(!a.filterActive() && a.filterMfr == "" && a.filterCategory == "").Chosen() {
			a.clearFilters()
		}
	})
}

// activeFilterChips lists the filters that are on, for the strip under the
// toolbar where they can be removed one by one.
func (a *app) activeFilterChips(c *ui.Context, pal *Palette) {
	chip := func(key, label string, clear func()) {
		b := ui.ButtonBase(c.Key("chip:"+key)).Height(20).Padding(0, 7).Gap(4).
			Radius(10).Background(pal.AccentCyanBg).AlignItems(ui.Center)
		b.Children(func() {
			ui.Text(c, label).FontSize(10.5).FontWeight(500).TextColor(pal.AccentCyan).SingleLine()
			if ic := icon("x"); ic != nil {
				ui.Icon(c, ic).FontSize(10).TextColor(pal.AccentCyan)
			}
		})
		if b.Clicked() {
			clear()
			_ = saveConfig(a.exeDir, a.cfg)
			a.refreshResults(0)
		}
	}

	if a.filterStatus > 0 {
		chip("status", "状态: "+filterStatusOptions[a.filterStatus], func() { a.filterStatus = 0 })
	}
	if a.filterCategory != "" {
		chip("cat", "类别: "+categoryName(a.filterCategory), func() {
			a.filterCategory = ""
			a.cfg.CategoryFilter = ""
		})
	}
	if a.filterMfr != "" {
		chip("mfr", "品牌: "+a.filterMfr, func() {
			a.filterMfr = ""
			a.cfg.ManufacturerFilter = ""
		})
	}
	if a.filterTypes > 0 {
		chip("pdf", filterTypeOptions[a.filterTypes], func() {
			a.filterTypes = 0
			a.cfg.DatasheetOnly = false
		})
	}
}

// categoryList returns the distinct categories of the loaded parts, by name.
func (a *app) categoryList() []string {
	seen := map[string]bool{}
	var out []string
	for _, p := range a.parts {
		if p.Category == "" || seen[p.Category] {
			continue
		}
		seen[p.Category] = true
		out = append(out, p.Category)
	}
	slices.SortFunc(out, func(x, y string) int {
		return strings.Compare(strings.ToLower(categoryName(x)), strings.ToLower(categoryName(y)))
	})
	return out
}

// manufacturerList returns the distinct manufacturers, most common first.
func (a *app) manufacturerList() []string {
	counts := map[string]int{}
	for _, p := range a.parts {
		if p.Manufacturer != "" {
			counts[p.Manufacturer]++
		}
	}
	out := make([]string, 0, len(counts))
	for m := range counts {
		out = append(out, m)
	}
	slices.SortFunc(out, func(x, y string) int {
		if counts[x] != counts[y] {
			return counts[y] - counts[x]
		}
		return strings.Compare(strings.ToLower(x), strings.ToLower(y))
	})
	return out
}

// tableColumn is a sortable column of the result table: its header, the sort
// mode it sets, and the width the rows reserve for it.
type tableColumn struct {
	label string
	mode  SortMode
	width float32
}

var resultColumns = []tableColumn{
	{"BHS 编号", SortBHS, 100},
	{"元件名称", SortName, 120},
	{"型号 / 规格", SortSpec, 130},
	{"品牌/供应商", SortManufacturer, 90},
	{"原表 / 工作表", SortSourceOrder, 112},
	{"数据手册", "", 60},
	{"状态", "", 50},
}

// sortColumnHeader draws one sortable column title; a click sorts by that
// column, and a second click reverses the order.
func (a *app) sortColumnHeader(c *ui.Context, col tableColumn, pal *Palette) {
	active := col.mode != "" && sortModeFromLabel(a.sortLabel) == col.mode
	h := ui.ButtonBase(c.Key("col:"+col.label)).Width(col.width).Height(26).
		Padding(0, 2).AlignItems(ui.Center).Radius(4).Label("按 " + col.label + " 排序")
	h.Background(pal.AppBg)
	if active {
		h.Background(pal.AccentCyanBg)
	} else if h.Hovered() {
		h.Background(pal.SurfaceHover)
	}
	textColor := pal.TextFaint
	if active {
		textColor = pal.AccentCyan
	}
	h.Children(func() {
		ui.Text(c, col.label).FontSize(11).FontWeight(600).
			TextColor(textColor).SingleLine()
		if active {
			iconName := "arrow-up"
			if a.sortDesc {
				iconName = "arrow-down"
			}
			if ic := icon(iconName); ic != nil {
				ui.Icon(c, ic).FontSize(9).TextColor(textColor).Margin(2, 0, 0, 0)
			}
		}
	})
	if h.Clicked() {
		if a.sortLabel == col.label {
			a.setSortDesc(!a.sortDesc)
		} else {
			a.setSort(col.label)
		}
	}
}

func (a *app) tableHeader(c *ui.Context, pal *Palette) ui.Element {
	return ui.Row(c.Key("table-header")).Height(30).Padding(0, 12).Gap(8).AlignItems(ui.Center).Clip().
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		Background(pal.AppBg).Children(func() {
		for _, col := range resultColumns {
			if col.mode != "" {
				a.sortColumnHeader(c, col, pal)
				continue
			}
			ui.Text(c, col.label).FontSize(11).FontWeight(600).
				TextColor(pal.TextFaint).Width(col.width).TextAlign(ui.Center).SingleLine()
		}
	})
}

func (a *app) renderResultList(c *ui.Context, pal *Palette) ui.Element {
	a.resultList.Selected = &a.resultSel
	a.resultList.Key = func(i int) any { return a.results[i].ID }
	a.resultList.Label = func(i int) string {
		p := a.results[i]
		return p.BHSNo + " " + p.Name()
	}
	list := ui.List(c.Key("table-list"), &a.resultList, len(a.results)).Grow(1).RowWidth(700).
		Rows(func(row ui.ListRow) { a.resultRow(row, pal) }).Label("器件列表")
	if list.Changed() {
		a.inspectorOpen = true
		a.detailScroll.Y = 0
	}
	if list.Submitted() {
		a.openEdit()
	}
	return list
}

func (a *app) resultRow(row ui.ListRow, pal *Palette) {
	c := row.Context
	p := a.results[row.Index]
	selected := row.Selected()
	background := pal.RowEven
	textColor := pal.Text
	if selected {
		background = pal.RowSelected
	} else if row.Index%2 == 1 {
		background = pal.RowOdd
	}
	r := ui.Row(c.Key(fmt.Sprintf("result:%d", p.ID))).Height(34).
		Padding(0, 12).Gap(8).AlignItems(ui.Center).Background(background).
		TextColor(textColor)
	if !selected && r.Hovered() {
		r.Background(pal.RowHover)
	}
	r.Children(func() {
		ui.Text(c, p.BHSNo).Font("monospace").FontSize(11.5).FontWeight(600).
			TextColor(pal.AccentCyan).Width(100).SingleLine()

		ui.Text(c, p.Name()).FontSize(12.5).FontWeight(500).
			TextColor(textColor).Width(120).SingleLine()

		ui.Text(c, valueOr(p.Spec, "-")).FontSize(11.5).
			TextColor(pal.TextMuted).Width(130).SingleLine()

		ui.Text(c, valueOr(p.Manufacturer, "-")).FontSize(11.5).
			TextColor(pal.TextMuted).Width(90).SingleLine()

		ui.Text(c, p.SourceLabel()).FontSize(11).
			TextColor(pal.TextFaint).Width(112).SingleLine()

		// Datasheet count: a click opens the part's datasheets right away.
		ui.Row(c).Width(60).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			pdfs := a.pdfScanner.FindForBHS(p.BHSNo)
			if len(pdfs) == 0 {
				ui.Text(c, "-").FontSize(10.5).Font("monospace").TextColor(pal.TextFaint)
				return
			}
			b := ui.ButtonBase(c.Key(fmt.Sprintf("pdf-count:%d", p.ID))).Height(20).
				Padding(0, 6).Radius(3).Background(pal.AccentCyanBg).AlignItems(ui.Center).
				Label(fmt.Sprintf("%s 有 %d 份数据手册", p.BHSNo, len(pdfs)))
			b.Children(func() {
				if ic := icon("file-down"); ic != nil {
					ui.Icon(c, ic).FontSize(10).TextColor(pal.AccentCyan)
				}
				ui.Textf(c, "%d", len(pdfs)).FontSize(10.5).Font("monospace").
					FontWeight(600).TextColor(pal.AccentCyan)
			})
			if b.Clicked() {
				a.resultSel = row.Index
				a.inspectorOpen = true
				a.detailScroll.Y = 0
				if len(pdfs) == 1 {
					if err := a.OpenDatasheetPDF(pdfs[0].FullPath); err != nil {
						c.Toast("打开 PDF 失败: " + err.Error())
					}
				} else {
					c.Toast(fmt.Sprintf("%s 有 %d 份数据手册，见右侧详情栏", p.BHSNo, len(pdfs)))
				}
			}
		})

		ui.Row(c).Width(50).Justify(ui.Center).Children(func() {
			statusIndicator(c, p.Status, a.lightTheme)
		})
	})
}

func (a *app) emptyResults(c *ui.Context, pal *Palette) {
	emptyStateView(c, "search", "未找到匹配的器件",
		"可尝试更换搜索关键词，或在左侧切换到其他原表分类。", pal)
}

func (a *app) detailPanel(c *ui.Context, pal *Palette) {
	p := a.selectedPart()
	if p == nil {
		return
	}
	ui.Column(c).Width(310).Shrink(0).FillHeight().
		Background(pal.SidebarBg).
		BorderWidth(0, 0, 0, 1).BorderColor(pal.Border).
		Children(func() {
			a.detailHeader(c, p, pal)
			ui.Divider(c).Background(pal.Border)
			ui.Scroll(c).TrackScroll(&a.detailScroll).Grow(1).
				Padding(12, 14, 20, 14).Gap(10).Children(func() {
				a.sourceCard(c, p, pal)
				a.datasheetCard(c, p, pal)
				a.detailSection(c, "基本规格 (SPECIFICATIONS)", [][2]string{
					{"型号 / 规格", p.Spec},
					{"品牌/供应商", p.Manufacturer},
					{"订货号", p.OrderNo},
					{"单位", p.Unit},
					{"属性 / Dispo", p.Dispo},
				}, pal)
				a.detailSection(c, "分类与工程 (ENGINEERING)", [][2]string{
					{"类别", categoryName(p.Category)},
					{"子类别", p.Subcategory},
					{"机型 / 功能", p.Machine},
					{"认证", p.Cert},
					{"WHR 进口", p.ImportFlag},
					{"旧零件号 / G No.", p.AlternateNo},
					{"CN 零件号", p.CNPartNo},
					{"DongAn 外包号", p.DonganPartNo},
					{"S 零件号", p.SPartNo},
				}, pal)
				a.detailSection(c, "库存与供货 (LOGISTICS)", [][2]string{
					{"价格", p.Price},
					{"备件级别", p.SpareLevel},
					{"当前库存", p.Stock},
					{"安全库存", p.SafetyStock},
					{"协议库存", p.ProtocolStock},
					{"起订量", p.MOQ},
					{"货期", p.Delivery},
					{"死库存", p.SLOB},
				}, pal)
				if p.Remark != "" {
					a.detailSection(c, "备注 (REMARKS)", [][2]string{{"备注", p.Remark}}, pal)
				}
			})
		})
}

func (a *app) detailHeader(c *ui.Context, p *Part, pal *Palette) {
	ui.Column(c).Gap(6).Padding(12, 16, 12, 16).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			statusIndicator(c, p.Status, a.lightTheme)
			ui.Spacer(c)
			closeBtn := iconButton(c, "x", "收起详情", pal)
			if closeBtn.Clicked() {
				a.inspectorOpen = false
			}
		})
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, valueOr(p.BHSNo, "未编号")).FontSize(18).Bold().
				Font("monospace").LetterSpacing(-0.3).TextColor(pal.Text).SingleLine()
			copyBtn := iconButton(c, "copy", "复制 BHS 编号", pal)
			if copyBtn.Clicked() && p.BHSNo != "" {
				mygo.Clipboard.WriteText(p.BHSNo)
				c.Toast("已复制编号：" + p.BHSNo)
			}
		})
		ui.Text(c, p.Name()).FontSize(13.5).FontWeight(600).TextColor(pal.Text).Wrap()
		if p.SecondaryName() != "" {
			ui.Text(c, p.SecondaryName()).FontSize(11).
				TextColor(pal.TextMuted).SingleLine()
		}
		ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
			edit := ui.ButtonBase(c.Key("btn:edit-part")).Height(28).Padding(0, 12).
				Radius(4).Background(pal.Accent).TextColor(pal.AccentText).AlignItems(ui.Center)
			if edit.Hovered() {
				edit.Background(pal.AccentHover)
			}
			edit.Children(func() {
				ui.Text(c, "修改器件").FontSize(11.5).FontWeight(600).TextColor(pal.AccentText)
			})
			if edit.Clicked() {
				a.openEdit()
			}

			if p.SourceFile != "" {
				loc := ui.ButtonBase(c.Key("btn:locate-source")).Height(28).Padding(0, 10).
					Radius(4).Background(pal.Surface).Border(1, pal.Border).
					TextColor(pal.TextMuted).AlignItems(ui.Center)
				if loc.Hovered() {
					loc.Background(pal.SurfaceHover).TextColor(pal.Text)
				}
				loc.Children(func() {
					ui.Text(c, "定位分类").FontSize(11.5).FontWeight(500)
				})
				if loc.Clicked() {
					a.showPartSource(p)
				}
			}
		})
	})
}

func (a *app) sourceCard(c *ui.Context, p *Part, pal *Palette) {
	if p.SourceFile == "" {
		ui.Column(c).Padding(8, 10).Gap(3).Radius(6).Background(pal.Surface).
			Border(1, pal.Border).Children(func() {
			ui.Text(c, "手动维护记录").FontSize(10).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.TextFaint)
			ui.Text(c, "独立录入，未关联原表").FontSize(11).TextColor(pal.TextMuted)
		})
		return
	}
	book := a.bookByFile(p.SourceFile)
	ui.Column(c).Padding(9, 11).Gap(5).Radius(6).Background(pal.Surface).
		Border(1, pal.Border).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			if ic := icon("folder-open"); ic != nil {
				ui.Icon(c, ic).FontSize(13).TextColor(pal.AccentCyan)
			}
			ui.Text(c, "原表归属 (WORKBOOK ORIGIN)").FontSize(10).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.AccentCyan)
		})
		ui.Text(c, p.SourceFile).FontSize(11.5).FontWeight(600).
			TextColor(pal.Text).Wrap()
		sheetLabel := p.SourceSheet
		if p.SourceRow > 0 {
			sheetLabel = fmt.Sprintf("%s · 第 %d 行", sheetLabel, p.SourceRow)
		}
		ui.Text(c, sheetLabel).FontSize(11).TextColor(pal.TextMuted).SingleLine()

		// Action buttons row for Excel opening & sheet cycling
		ui.Row(c).Gap(6).Margin(2, 0, 0, 0).Children(func() {
			openExcelBtn := ui.ButtonBase(c.Key("btn:open-excel-part")).Height(24).Padding(0, 8).
				Radius(4).Background(pal.Accent).TextColor(pal.AccentText).AlignItems(ui.Center)
			if openExcelBtn.Hovered() {
				openExcelBtn.Background(pal.AccentHover)
			}
			openExcelBtn.Children(func() {
				ui.Text(c, "打开 Excel 原表").FontSize(10.5).FontWeight(600).TextColor(pal.AccentText)
			})
			if openExcelBtn.Clicked() {
				a.openExcelFile(c, p.SourceFile)
			}

			locExcelBtn := ui.ButtonBase(c.Key("btn:locate-excel-file")).Height(24).Padding(0, 7).
				Radius(4).Background(pal.SurfaceHover).Border(1, pal.Border).
				TextColor(pal.TextMuted).AlignItems(ui.Center)
			if locExcelBtn.Hovered() {
				locExcelBtn.TextColor(pal.Text)
			}
			locExcelBtn.Children(func() {
				ui.Text(c, "定位文件").FontSize(10.5)
			})
			if locExcelBtn.Clicked() {
				a.locateExcelFile(c, p.SourceFile)
			}

			if book != nil && len(book.Sheets) > 1 {
				btn := ui.ButtonBase(c.Key("btn:switch-sheet")).Height(24).Padding(0, 7).
					Radius(4).Background(pal.SurfaceHover).Border(1, pal.Border).
					TextColor(pal.TextMuted).AlignItems(ui.Center)
				if btn.Hovered() {
					btn.TextColor(pal.Text)
				}
				btn.Children(func() {
					ui.Text(c, "切换工作表").FontSize(10.5)
				})
				if btn.Clicked() {
					a.cycleSheet(book)
				}
			}
		})
	})
}

func (a *app) cycleSheet(book *WorkbookInfo) {
	for i, sheet := range book.Sheets {
		if sheet.Name == a.selectedSheet() {
			a.sheetSel = (i + 1) % len(book.Sheets)
			a.refreshResults(0)
			return
		}
	}
	if len(book.Sheets) > 0 {
		a.sheetSel = 0
		a.refreshResults(0)
	}
}

func (a *app) detailSection(c *ui.Context, title string, fields [][2]string, pal *Palette) {
	hasValue := false
	for _, field := range fields {
		if strings.TrimSpace(field[1]) != "" {
			hasValue = true
			break
		}
	}
	if !hasValue {
		return
	}
	ui.Column(c).Gap(5).Children(func() {
		ui.Text(c, title).FontSize(10).FontWeight(700).
			LetterSpacing(0.8).TextColor(pal.TextFaint)
		ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).
			Dividers(1, pal.BorderSubtle).Children(func() {
			for _, field := range fields {
				if strings.TrimSpace(field[1]) == "" {
					continue
				}
				ui.Row(c).AlignItems(ui.Start).Gap(8).Padding(5, 9).Children(func() {
					ui.Text(c, field[0]).FontSize(10.5).TextColor(pal.TextFaint).Width(86)
					ui.Text(c, field[1]).FontSize(11.5).TextColor(pal.Text).
						Grow(1).Wrap()
				})
			}
		})
	})
}

func (a *app) statusBar(c *ui.Context, pal *Palette) {
	ui.Row(c).Height(26).Padding(0, 16).Gap(8).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(pal.Border).
		Background(pal.StatusBarBg).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success)
		ui.Text(c, "SQLite 正常就绪").FontSize(10.5).FontWeight(500).TextColor(pal.SuccessText)
		ui.Text(c, "·").FontSize(10).TextColor(pal.TextFaint)
		ui.Textf(c, "共 %s 件器件", formatInt(a.summary.Total)).
			FontSize(10.5).Font("monospace").TextColor(pal.TextMuted)
		ui.Spacer(c)
		ui.Text(c, "快捷键: Ctrl+F 搜索 · 双击行编辑").
			FontSize(10).TextColor(pal.TextFaint)
	})
}

func (a *app) form(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(640).Padding(16, 20).Gap(10).Radius(8).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(1).Grow(1).Children(func() {
				title := "新增单元器件"
				if a.editing {
					title = "修改单元器件"
				}
				ui.Text(c, title).FontSize(16).Bold().TextColor(pal.Text)
				ui.Text(c, "输入器件参数并保存至数据库。").
					FontSize(11.5).TextColor(pal.TextMuted)
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.formOpen = false
				a.formError = ""
			}
		})
		ui.Divider(c).Background(pal.Border)
		ui.Grid(c).Columns(2).GapX(14).GapY(7).Children(func() {
			a.formField(c, "BHS 编号 *", func() {
				ui.TextInput(c, &a.draft.BHSNo).AutoFocus()
			})
			a.formField(c, "类别 *", func() {
				ui.TextInput(c, &a.draft.Category)
			})
			a.formField(c, "中文名称", func() {
				ui.TextInput(c, &a.draft.DescriptionCN)
			})
			a.formField(c, "英文名称", func() {
				ui.TextInput(c, &a.draft.DescriptionEN)
			})
			a.formField(c, "型号 / 规格", func() {
				ui.TextInput(c, &a.draft.Spec)
			})
			a.formField(c, "品牌 / 供应商", func() {
				ui.TextInput(c, &a.draft.Manufacturer)
			})
			a.formField(c, "订货号", func() {
				ui.TextInput(c, &a.draft.OrderNo)
			})
			a.formField(c, "单位", func() {
				ui.TextInput(c, &a.draft.Unit)
			})
			a.formField(c, "属性 / Dispo", func() {
				ui.TextInput(c, &a.draft.Dispo)
			})
			a.formField(c, "认证", func() {
				ui.TextInput(c, &a.draft.Cert)
			})
			a.formField(c, "机型 / 功能", func() {
				ui.TextInput(c, &a.draft.Machine)
			})
			a.formField(c, "状态", func() {
				ui.Select(c, &a.formStatus, []string{"有效", "停用", "历史"})
			})
			ui.Field(c, "备注", func() {
				ui.TextArea(c, &a.draft.Remark).Lines(2, 2)
			}).ColumnSpan(2)
		})
		if dupes := a.duplicateDraftCount(); dupes > 0 {
			ui.Textf(c, "注意：已存在 %d 条相同 BHS 编号的记录，保存将保留此多版本。", dupes).
				FontSize(11).TextColor(pal.WarningText).
				Padding(5, 8).Radius(4).Background(pal.Warning.Alpha(0.12))
		}
		if a.formError != "" {
			ui.Text(c, a.formError).FontSize(11).TextColor(pal.DangerText).
				Padding(5, 8).Radius(4).Background(pal.Danger.Alpha(0.12))
		}
		ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0, 0).Children(func() {
			if ui.Button(c, "取消").Height(30).Clicked() {
				a.formOpen = false
				a.formError = ""
			}
			if ui.PrimaryButton(c, "保存记录").Height(30).Clicked() {
				a.saveDraft(c)
			}
		})
	})
}

func (a *app) formField(c *ui.Context, label string, control func()) {
	ui.Field(c, label, control).Grow(1)
}

func (a *app) bookByFile(file string) *WorkbookInfo {
	for i := range a.books {
		if a.books[i].SourceFile == file {
			return &a.books[i]
		}
	}
	return nil
}

func statusLabel(status string) string {
	if label := statusLabels[status]; label != "" {
		return label
	}
	return "有效"
}

func sourceOrigin(p *Part) string {
	if p.SourceFile == "" {
		return "手动维护"
	}
	if p.Origin == "manual" {
		return "手动维护 (归档于原表)"
	}
	return "Excel 导入"
}

func shortFilename(file string) string {
	value := strings.TrimSuffix(file, ".xlsx")
	value = strings.TrimSuffix(value, ".xls")
	value = strings.TrimPrefix(value, "Master E-list_")
	return value
}

func valueOr(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	return value
}

func formatInt(n int) string {
	text := fmt.Sprintf("%d", n)
	for i := len(text) - 3; i > 0; i -= 3 {
		text = text[:i] + "," + text[i:]
	}
	return text
}

func (a *app) datasheetCard(c *ui.Context, p *Part, pal *Palette) {
	matchedPDFs := a.pdfScanner.FindForBHS(p.BHSNo)
	hiddenDuplicates := a.pdfScanner.DedupedGroupCount(p.BHSNo)

	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "数据手册 (DATASHEET)").FontSize(10).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.TextFaint)
			if len(matchedPDFs) > 0 {
				ui.Textf(c, "%d 份", len(matchedPDFs)).FontSize(9.5).Font("monospace").
					FontWeight(600).TextColor(pal.AccentCyan).
					Padding(1, 5).Radius(3).Background(pal.AccentCyanBg)
			}
			ui.Spacer(c)
			if a.pdfScanner.Scanning() {
				ui.Text(c, "索引同步中...").FontSize(9.5).TextColor(pal.TextFaint)
			}
			listBtn := ui.ButtonBase(c.Key("btn:pdf-filelist")).Height(20).Padding(0, 6).
				Radius(4).Background(pal.SurfaceHover).Border(1, pal.Border).
				AlignItems(ui.Center).Label("打开手册库文件列表")
			listBtn.Children(func() {
				ui.Text(c, "文件列表").FontSize(10).TextColor(pal.TextMuted)
			})
			if listBtn.Clicked() {
				a.openFileList()
			}
		})

		if len(matchedPDFs) == 0 {
			ui.Column(c).Padding(9, 11).Gap(5).Radius(6).Background(pal.Surface).
				Border(1, pal.Border).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					if ic := icon("info"); ic != nil {
						ui.Icon(c, ic).FontSize(13).TextColor(pal.TextFaint)
					}
					ui.Text(c, "未检索到匹配的 PDF 手册").FontSize(11).TextColor(pal.TextMuted)
				})
				ui.Textf(c, "匹配规则：文件名包含 BHS 零件号 '%s'", valueOr(p.BHSNo, "-")).
					FontSize(10).TextColor(pal.TextFaint)
				ui.Row(c).Gap(6).Margin(2, 0, 0, 0).Children(func() {
					btn := ui.ButtonBase(c.Key("btn:cfg-pdf-inline")).Height(24).Padding(0, 8).
						Radius(4).Background(pal.SurfaceHover).Border(1, pal.Border).
						TextColor(pal.TextMuted).AlignItems(ui.Center)
					if btn.Hovered() {
						btn.TextColor(pal.Text)
					}
					btn.Children(func() {
						ui.Text(c, "配置手册路径").FontSize(11)
					})
					if btn.Clicked() {
						a.openDatasheetSettings()
					}
					sync := ui.ButtonBase(c.Key("btn:sync-pdf-inline")).Height(24).Padding(0, 8).
						Radius(4).Background(pal.AccentCyanBg).Border(1, pal.Border).
						AlignItems(ui.Center).Label("同步手册索引")
					sync.Children(func() {
						ui.Text(c, "同步索引").FontSize(11).TextColor(pal.AccentCyan)
					})
					if sync.Clicked() {
						a.scanDatasheetsAsync(true)
					}
				})
				ui.Textf(c, "索引：%s 份 PDF", formatInt(a.pdfScanner.Count())).
					FontSize(9.5).TextColor(pal.TextFaint).SingleLine()
			})
			return
		}

		if hiddenDuplicates > 0 {
			ui.Textf(c, "已按文件大小去重：另有 %d 份疑似相同文件未显示。", hiddenDuplicates).
				FontSize(9.5).TextColor(pal.TextFaint)
		}

		ui.Column(c).Gap(5).Children(func() {
			for i, pdf := range matchedPDFs {
				pdfFile := pdf
				ui.Column(c).Key(fmt.Sprintf("pdf:%s:%d", p.BHSNo, i)).
					Padding(8, 10).Gap(6).Radius(6).Background(pal.Surface).
					Border(1, pal.Border).Children(func() {
					ui.Row(c).AlignItems(ui.Start).Gap(7).Children(func() {
						if ic := icon("file-down"); ic != nil {
							ui.Icon(c, ic).FontSize(15).TextColor(pal.AccentCyan)
						}
						ui.Column(c).Grow(1).Gap(1).Children(func() {
							ui.Text(c, pdfFile.Name).FontSize(11.5).FontWeight(600).
								TextColor(pal.Text).Wrap()
							ui.Textf(c, "%s · %s · %s", formatFileSize(pdfFile.Size),
								formatModTime(pdfFile.ModTime), pdfFile.Dir).
								FontSize(9.5).Font("monospace").TextColor(pal.TextFaint).SingleLine()
						})
					})
					ui.Row(c).Justify(ui.End).Gap(6).Children(func() {
						locBtn := ui.ButtonBase(c.Key(fmt.Sprintf("loc:%s:%d", p.BHSNo, i))).
							Height(24).Padding(0, 8).Radius(4).
							Background(pal.SurfaceHover).Border(1, pal.Border).
							TextColor(pal.TextMuted).AlignItems(ui.Center)
						if locBtn.Hovered() {
							locBtn.TextColor(pal.Text)
						}
						locBtn.Children(func() {
							ui.Text(c, "定位文件").FontSize(11)
						})
						if locBtn.Clicked() {
							revealPDF(pdfFile.FullPath)
						}

						openBtn := ui.ButtonBase(c.Key(fmt.Sprintf("open:%s:%d", p.BHSNo, i))).
							Height(24).Padding(0, 8).Radius(4).
							Background(pal.Accent).TextColor(pal.AccentText).AlignItems(ui.Center)
						if openBtn.Hovered() {
							openBtn.Background(pal.AccentHover)
						}
						openBtn.Children(func() {
							label := "打开手册"
							if a.cfg.ReadOnlyPDF {
								label = "只读打开"
							}
							ui.Text(c, label).FontSize(11).FontWeight(600).TextColor(pal.AccentText)
						})
						if openBtn.Clicked() {
							a.openPDFInDefault(c, pdfFile.FullPath)
						}
					})
				})
			}
		})
	})
}

func (a *app) settingsModal(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(680).Padding(16, 20).Gap(12).Radius(8).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "目录路径与关联设置").FontSize(16).Bold().TextColor(pal.Text)
				ui.Text(c, "自定义 PDF 数据手册与 Excel 原始表格的存放目录（支持相对/绝对路径与网络盘）。").
					FontSize(11.5).TextColor(pal.TextMuted)
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.datasheetModalOpen = false
			}
		})
		ui.Divider(c).Background(pal.Border)

		ui.Tabs(c, &a.settingsTab, "数据手册 PDF 目录", "Excel 原表目录", "常规选项")

		switch a.settingsTab {
		case 0:
			a.settingsDatasheetTab(c, pal)
		case 1:
			a.settingsExcelTab(c, pal)
		default:
			a.settingsGeneralTab(c, pal)
		}

		if a.datasheetStatusMsg != "" {
			ui.Text(c, a.datasheetStatusMsg).FontSize(11).TextColor(pal.DangerText).
				Padding(5, 8).Radius(4).Background(pal.Danger.Alpha(0.12))
		}

		ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0, 0).Children(func() {
			if a.settingsTab == 0 {
				if ui.Button(c, "同步手册文件列表").Height(30).Clicked() {
					a.syncFileListAsync()
				}
			}
			if a.settingsTab != 2 {
				if ui.PrimaryButton(c, "保存路径设置").Height(30).Clicked() {
					a.saveDatasheetSettings(c)
				}
			}
			if ui.Button(c, "关闭").Height(30).Clicked() {
				a.datasheetModalOpen = false
			}
		})
	})
}

func (a *app) settingsDatasheetTab(c *ui.Context, pal *Palette) {
	ui.Field(c, "PDF 手册根目录（每行一条，递归检索子文件夹）", func() {
		ui.TextArea(c, &a.datasheetPathsText).Lines(4, 5)
	})
	ui.Text(c, `可填写本地文件夹、映射网络盘（如 Z:\手册）或 UNC 路径（如 \\server\share\pdf）。网络盘不可达时自动沿用上次索引。`).
		FontSize(10.5).TextColor(pal.TextMuted).Wrap()

	ui.Column(c).Gap(4).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "当前扫描状态与路径解析:").FontSize(10.5).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.TextFaint)
			ui.Spacer(c)
			if a.pdfScanner.Scanning() {
				ui.Text(c, "正在扫描...").FontSize(10).TextColor(pal.AccentCyan)
			}
		})
		ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).
			Dividers(1, pal.BorderSubtle).Children(func() {
			statuses := a.pdfScanner.Status()
			if len(statuses) == 0 {
				ui.Row(c).Padding(8, 10).Children(func() {
					ui.Text(c, "暂无已配置路径").FontSize(11).TextColor(pal.TextMuted)
				})
			}
			for _, st := range statuses {
				stCopy := st
				ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(6, 10).Children(func() {
					if stCopy.Exists {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success)
					} else {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Danger)
					}
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, stCopy.ConfiguredPath).FontSize(11.5).FontWeight(600).
							TextColor(pal.Text).SingleLine()
						statusDesc := fmt.Sprintf("-> %s（含子文件夹共 %s 份 PDF）", stCopy.ResolvedPath, formatInt(stCopy.Count))
						if !stCopy.Exists {
							statusDesc = fmt.Sprintf("-> %s（目录或网络盘不可用）", stCopy.ResolvedPath)
						}
						ui.Text(c, statusDesc).FontSize(10.5).TextColor(pal.TextMuted).SingleLine()
						if stCopy.Error != "" {
							ui.Text(c, "提示: "+stCopy.Error).FontSize(10).
								TextColor(pal.WarningText).Wrap()
						}
					})
				})
			}
		})
	})

	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Textf(c, "总计索引 %s 份 PDF 数据手册", formatInt(a.pdfScanner.Count())).
			FontSize(11.5).Bold().TextColor(pal.AccentCyan)
		ui.Spacer(c)
		if a.pdfScanner.FromCache() {
			ui.Text(c, "（来自本地索引缓存）").FontSize(10).TextColor(pal.TextFaint)
		}
		if at := a.pdfScanner.ScannedAt(); !at.IsZero() {
			ui.Text(c, at.Format("2006.01.02 15:04:05")).FontSize(10).
				Font("monospace").TextColor(pal.TextFaint)
		}
	})
}

func (a *app) settingsExcelTab(c *ui.Context, pal *Palette) {
	ui.Field(c, "Excel 原表目录（存放 Master E-list_*.xlsx 的目录，每行一条）", func() {
		ui.TextArea(c, &a.masterlistPathsText).Lines(4, 5)
	})

	ui.Column(c).Gap(4).Children(func() {
		ui.Text(c, "当前原表路径检测与 Excel 文件数:").FontSize(10.5).FontWeight(700).
			LetterSpacing(0.8).TextColor(pal.TextFaint)
		ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).
			Dividers(1, pal.BorderSubtle).Children(func() {
			lines := strings.Split(a.masterlistPathsText, "\n")
			statuses := checkExcelDirStatuses(a.exeDir, lines)
			if len(statuses) == 0 {
				ui.Row(c).Padding(8, 10).Children(func() {
					ui.Text(c, "暂无已配置路径").FontSize(11).TextColor(pal.TextMuted)
				})
			}
			for _, st := range statuses {
				stCopy := st
				ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(6, 10).Children(func() {
					if stCopy.Exists {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success)
					} else {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Danger)
					}
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Text(c, stCopy.ConfiguredPath).FontSize(11.5).FontWeight(600).
							TextColor(pal.Text).SingleLine()
						statusDesc := fmt.Sprintf("-> %s (已检索到 %d 份 Excel 表格)", stCopy.ResolvedPath, stCopy.FileCount)
						if !stCopy.Exists {
							statusDesc = fmt.Sprintf("-> %s (目录不存在)", stCopy.ResolvedPath)
						}
						ui.Text(c, statusDesc).FontSize(10.5).TextColor(pal.TextMuted).SingleLine()
					})
				})
			}
		})
	})
}

func (a *app) settingsGeneralTab(c *ui.Context, pal *Palette) {
	ui.Text(c, "状态栏与只读选项").FontSize(10.5).FontWeight(700).
		LetterSpacing(0.8).TextColor(pal.TextFaint)

	ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).
		Dividers(1, pal.BorderSubtle).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "常驻状态栏").FontSize(12).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "在 Windows 通知区域显示程序图标，并可从那里唤起窗口。").
					FontSize(10.5).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.TrayEnabled
			if ui.Switch(c, &on).Label("常驻状态栏").Changed() {
				a.cfg.TrayEnabled = on
				if !on {
					a.cfg.CloseToTray = false
					if a.win != nil {
						a.win.SetSkipTaskbar(false)
					}
				}
				_ = saveConfig(a.exeDir, a.cfg)
				a.applyTraySetting()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "关闭窗口时隐藏到状态栏").FontSize(12).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "关闭窗口不退出程序，改从状态栏图标恢复；退出请用图标菜单的“退出”。").
					FontSize(10.5).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.CloseToTray
			sw := ui.Switch(c, &on).Label("关闭窗口时隐藏到状态栏")
			if !a.cfg.TrayEnabled {
				sw.Disabled(true)
			}
			if sw.Changed() && a.cfg.TrayEnabled {
				a.cfg.CloseToTray = on
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "以只读方式打开 PDF").FontSize(12).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "打开数据手册时使用只读副本，避免在手册上误编辑或保存，也不会占用网络盘上的原文件。").
					FontSize(10.5).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.ReadOnlyPDF
			if ui.Switch(c, &on).Label("以只读方式打开 PDF").Changed() {
				a.cfg.ReadOnlyPDF = on
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(4).Padding(9, 12, 4, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "只读副本使用的 PDF 阅读器（可选）").FontSize(12).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "留空时自动查找 SumatraPDF，再退回系统关联程序；也可指定其他阅读器的完整路径。").
					FontSize(10.5).TextColor(pal.TextMuted).Wrap()
			})
		})
		ui.Row(c).Gap(8).Padding(0, 12, 9, 12).Children(func() {
			ui.TextInput(c, &a.cfg.PDFReaderPath).Label("PDF 阅读器路径").Grow(1).
				Placeholder(`C:\Program Files\SumatraPDF\SumatraPDF.exe`)
			if ui.Button(c, "浏览...").Height(28).Clicked() {
				a.browseReader()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "索引在多少分钟后过期").FontSize(12).FontWeight(600).TextColor(pal.Text)
				ui.Textf(c, "当前 %d 分钟；索引保存在 data\\pdf-index.json，下次启动秒开。",
					a.cfg.PDFIndexMaxAgeMinutes).
					FontSize(10.5).TextColor(pal.TextMuted).Wrap()
			})
			minutes := float64(a.cfg.PDFIndexMaxAgeMinutes)
			in := ui.NumberInput(c, &minutes, 0, 1440, 5).
				Label("索引过期时间（分钟）").Width(90)
			if in.Changed() {
				a.cfg.PDFIndexMaxAgeMinutes = int(minutes)
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(6, 12).Children(func() {
			ui.Text(c, a.trayStatusLine()).FontSize(10.5).TextColor(pal.TextFaint).SingleLine()
			ui.Spacer(c)
			if a.trayReady {
				ui.Text(c, "图标已创建").FontSize(10).TextColor(pal.SuccessText)
			}
		})
	})

	ui.Text(c, "数据手册索引").FontSize(10.5).FontWeight(700).
		LetterSpacing(0.8).TextColor(pal.TextFaint).Margin(6, 0, 0, 0)
	ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 12).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, a.fileLibraryMeta()).FontSize(11).TextColor(pal.TextMuted).Wrap()
			})
			if ui.Button(c, "重建索引").Height(28).Clicked() {
				a.scanDatasheetsAsync(true)
			}
			if ui.Button(c, "手册库文件列表").Height(28).Clicked() {
				a.openFileList()
			}
		})
	})
}

// browseReader asks Windows for an executable and stores its path as the
// read-only viewer.
func (a *app) browseReader() {
	if a.win == nil {
		return
	}
	opts := mygo.OpenDialogOptions{
		Parent:  a.win,
		Title:   "选择 PDF 阅读器",
		Filters: []mygo.FileFilter{{Name: "程序 (*.exe)", Extensions: []string{"exe"}}},
	}
	paths, err := mygo.Dialog.Open(opts)
	if err != nil || len(paths) == 0 {
		return
	}
	a.cfg.PDFReaderPath = paths[0]
	_ = saveConfig(a.exeDir, a.cfg)
	a.win.Invalidate()
}

func (a *app) syncModal(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(560).Padding(16, 20).Gap(12).Radius(8).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				title := "Excel 原表同步至 SQL 库"
				if a.syncDryRun {
					title = "检测 Excel 原表变更 (预览)"
				}
				ui.Text(c, title).FontSize(16).Bold().TextColor(pal.Text)
				ui.Text(c, "扫描 Masterlist 目录中的 Excel 原表，增量追加新器件，保护手工维护记录。").
					FontSize(11.5).TextColor(pal.TextMuted)
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.syncModalOpen = false
			}
		})
		ui.Divider(c).Background(pal.Border)

		if a.syncRunning {
			ui.Column(c).Center().Gap(10).Padding(30).Children(func() {
				ui.Spinner(c)
				ui.Text(c, "正在扫描 Excel 原表并比对数据库...").FontSize(14).Bold().TextColor(pal.Text)
				ui.Text(c, "解析各工作表字段结构，匹配 BHS 零件号与规格").FontSize(11.5).TextColor(pal.TextMuted)
			})
		} else if a.syncErr != "" {
			ui.Column(c).Padding(12).Gap(6).Radius(6).Background(pal.Danger.Alpha(0.10)).
				Border(1, pal.Danger.Alpha(0.3)).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if ic := icon("circle-alert"); ic != nil {
						ui.Icon(c, ic).FontSize(16).TextColor(pal.Danger)
					}
					ui.Text(c, "同步过程中出现错误").FontSize(13).Bold().TextColor(pal.DangerText)
				})
				ui.Text(c, a.syncErr).FontSize(11.5).TextColor(pal.Text).Wrap()
			})
		} else if a.syncResult != nil {
			res := a.syncResult
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Padding(10, 14).Gap(4).Radius(6).Background(pal.Surface).
					Border(1, pal.Border).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if ic := icon("circle-check"); ic != nil {
							ui.Icon(c, ic).FontSize(16).TextColor(pal.Success)
						}
						ui.Text(c, res.SummaryMessage).FontSize(12.5).FontWeight(600).
							TextColor(pal.Text).Wrap()
					})
				})

				ui.Grid(c).Columns(3).GapX(10).GapY(8).Children(func() {
					a.syncStatBox(c, "扫描表格", fmt.Sprintf("%d 份", res.FilesScanned), pal)
					a.syncStatBox(c, "扫描工作表", fmt.Sprintf("%d 个", res.SheetsScanned), pal)
					a.syncStatBox(c, "新增入库", fmt.Sprintf("+%d 件", res.NewPartsAdded), pal)
					a.syncStatBox(c, "更新器件", fmt.Sprintf("%d 件", res.UpdatedParts), pal)
					a.syncStatBox(c, "保持一致", fmt.Sprintf("%d 件", res.UnchangedParts), pal)
					a.syncStatBox(c, "保护手工项", fmt.Sprintf("%d 件", res.ManualPreserved), pal)
				})

				if len(res.Errors) > 0 {
					ui.Column(c).Padding(8).Gap(3).Radius(6).Background(pal.Warning.Alpha(0.10)).Children(func() {
						ui.Text(c, "部分文件读取提示:").FontSize(11).Bold().TextColor(pal.WarningText)
						for _, e := range res.Errors {
							ui.Text(c, "· "+e).FontSize(10.5).TextColor(pal.TextMuted)
						}
					})
				}
			})
		}

		ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0, 0).Children(func() {
			if !a.syncRunning {
				dryBtn := ui.Button(c, "检测同步 (预览)").Height(30)
				if dryBtn.Clicked() {
					a.triggerSync(c, true)
				}

				syncBtn := ui.PrimaryButton(c, "立即执行同步").Height(30)
				if syncBtn.Clicked() {
					a.triggerSync(c, false)
				}
			}
			closeBtn := ui.Button(c, "关闭").Height(30)
			if closeBtn.Clicked() {
				a.syncModalOpen = false
			}
		})
	})
}

func (a *app) syncStatBox(c *ui.Context, label, value string, pal *Palette) {
	ui.Column(c).Padding(8, 10).Gap(2).Radius(6).Background(pal.Surface).
		Border(1, pal.Border).Children(func() {
		ui.Text(c, label).FontSize(10.5).TextColor(pal.TextFaint)
		ui.Text(c, value).FontSize(13).Font("monospace").FontWeight(600).TextColor(pal.AccentCyan)
	})
}
