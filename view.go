package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func (a *app) view(c *ui.Context) {
	uiFontScale = clampFontScale(a.cfg.FontScale)
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
	bar := c.TitleBar()
	ui.Column(c).Fill().Background(pal.AppBg).
		Padding(max(bar.Height-16, 16), 0, 0, 0).DragWindow().Children(func() {
		ui.Column(c).Grow(1).Center().Gap(10).Children(func() {
			ui.Spinner(c)
			ui.Text(c, "正在打开器件库").FontSize(sf(16)).FontWeight(600).TextColor(pal.Text)
			ui.Text(c, "读取原表结构与器件记录").
				FontSize(sf(11.5)).TextColor(pal.TextMuted)
		})
	})
}

func (a *app) errorView(c *ui.Context, pal *Palette) {
	bar := c.TitleBar()
	ui.Column(c).Fill().Background(pal.AppBg).
		Padding(max(bar.Height-16, 16), 0, 0, 0).DragWindow().Children(func() {
		ui.Column(c).Grow(1).Center().Gap(10).Padding(40).Children(func() {
			if ic := icon("circle-alert"); ic != nil {
				ui.Icon(c, ic).FontSize(sf(28)).TextColor(pal.Danger)
			}
			ui.Text(c, "无法打开器件库").FontSize(sf(17)).FontWeight(600).TextColor(pal.Text)
			ui.Text(c, a.loadErr).FontSize(sf(12)).TextColor(pal.TextMuted).
				MaxWidth(620).Wrap().TextAlign(ui.Center)
			if a.dbPath != "" {
				metaText(c, a.dbPath, pal)
			}
			if ui.PrimaryButton(c, "重试").Height(30).Margin(6, 0, 0, 0).Clicked() {
				a.loadErr = ""
				a.loading = true
				go a.loadData(a.win)
			}
		})
	})
}

func (a *app) sidebar(c *ui.Context, pal *Palette) {
	bar := c.TitleBar()
	topPadding := max(bar.Height, 20) + 2
	ui.Column(c).Width(228).Shrink(0).FillHeight().
		Background(pal.SidebarBg).
		BorderWidth(0, 1, 0, 0).BorderColor(pal.Border).
		Padding(topPadding, 12, 10, 12).Gap(2).DragWindow().Children(func() {
		a.sidebarHeader(c, pal)
		ui.Row(c).Height(8).Invisible()

		a.allSourcesItem(c, pal)

		ui.Row(c).Padding(2, 6, 6, 6).Margin(4, 0, 0, 0).AlignItems(ui.Center).Children(func() {
			sectionLabel(c, "原表目录", pal)
			ui.Spacer(c)
			ui.Textf(c, "%d 份", len(a.books)).FontSize(sf(10.5)).TextColor(pal.TextFaint).SingleLine()
		})

		ui.Scroll(c).Grow(1).Gap(1).Children(func() {
			for i := range a.books {
				a.renderSidebarItem(c, i, pal)
			}
		})

		ui.Divider(c).Background(pal.BorderSubtle).Margin(6, 4)
		a.sidebarFooter(c, pal)
	})
}

func (a *app) sidebarHeader(c *ui.Context, pal *Palette) {
	ui.Row(c).Height(40).Padding(0, 4).Gap(10).AlignItems(ui.Center).Children(func() {
		if appIconSource != nil {
			ui.Image(c, appIconSource).Size(30, 30).Radius(7)
		} else {
			ui.Box(c).Size(30, 30).Center().Radius(7).Background(pal.Accent).Children(func() {
				ui.Text(c, "ML").FontSize(sf(10)).Bold().TextColor(pal.AccentText)
			})
		}
		ui.Column(c).Gap(1).Children(func() {
			ui.Text(c, "电气柜元件库").FontSize(sf(14)).FontWeight(600).TextColor(pal.Text).SingleLine()
			ui.Text(c, "器件与手册").FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine()
		})
	})
}

// navItem is the single row shape of the sidebar: a marker rail on the left,
// an optional glyph, the label, and a count. Selected and hovered states only
// change the surface, never the layout.
func (a *app) navItem(c *ui.Context, pal *Palette, key string, selected bool, iconName, label string, count int) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(32).Padding(0, 8, 0, 10).Gap(9).
		Radius(7).AlignItems(ui.Center).Label(label)
	switch {
	case selected:
		b.Background(pal.SurfaceActive)
	case b.Hovered():
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		rail := pal.Accent
		if !selected {
			rail = pal.Border
		}
		ui.Box(c).Width(2).Height(16).Radius(1).Background(rail)
		glyphColor := pal.TextFaint
		textColor := pal.TextMuted
		countColor := pal.TextFaint
		if selected {
			glyphColor = pal.AccentCyan
			textColor = pal.Text
			countColor = pal.TextMuted
		}
		if iconName != "" {
			if ic := icon(iconName); ic != nil {
				ui.Icon(c, ic).FontSize(sf(14)).TextColor(glyphColor)
			}
		}
		ui.Text(c, label).FontSize(sf(12.5)).FontWeight(500).
			TextColor(textColor).Grow(1).SingleLine()
		ui.Textf(c, "%s", formatInt(count)).FontSize(sf(10.5)).
			TextColor(countColor).SingleLine()
	})
	return b
}

func (a *app) allSourcesItem(c *ui.Context, pal *Palette) {
	b := a.navItem(c, pal, "nav:all", a.workbookSel == -1, "layout-dashboard",
		"全部器件", a.summary.Total)
	if b.Clicked() {
		a.workbookSel = -1
		a.sheetSel = -1
		a.refreshResults(0)
	}
}

func (a *app) renderSidebarItem(c *ui.Context, index int, pal *Palette) {
	book := &a.books[index]
	b := a.navItem(c, pal, "source:"+book.SourceFile, a.workbookSel == index,
		"", originalTableName(book.SourceFile), book.Count)
	if b.Clicked() {
		a.workbookSel = index
		a.sheetSel = -1
		a.refreshResults(0)
	}
}

func (a *app) sidebarFooter(c *ui.Context, pal *Palette) {
	ui.Column(c).Padding(0, 2).Gap(7).Children(func() {
		ui.Row(c).Gap(2).Children(func() {
			if syncBtn := quietButton(c, "footer-sync", "同步原表", "rotate-cw", pal); syncBtn.Clicked() {
				a.triggerSync(c, false)
			}
			if libBtn := quietButton(c, "footer-library", "手册 "+formatInt(a.pdfScanner.Count()), "file-down", pal); libBtn.Clicked() {
				a.openFileList()
			}
		})
		ui.Column(c).Padding(8, 8, 2, 8).Gap(2).Children(func() {
			ui.Textf(c, "%d 份原表 · %d 个工作表", a.summary.Files, a.summary.Sheets).
				FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine()
			ui.Text(c, "上次同步 "+valueOr(formatImportedAt(a.summary.ImportDate), "—")).
				FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine()
		})
	})
}

func (a *app) content(c *ui.Context, pal *Palette) {
	ui.Column(c).Grow(1).MinWidth(0).Background(pal.MainBg).Children(func() {
		a.pageHeader(c, pal)
		a.scopeStrip(c, pal)
		ui.Row(c).Grow(1).MinHeight(0).AlignItems(ui.Stretch).Children(func() {
			a.catalogBody(c, pal)
			// The detail pane is permanent by default: it keeps its place
			// with a placeholder while nothing is selected, instead of the
			// table jumping wider when a row is chosen.
			if a.detailVisible() {
				a.detailPanel(c, pal)
			}
		})
		a.statusBar(c, pal)
	})
}

// pageHeader holds identity on the left and one row of actions on the right.
// The search field sits first on the right, because it is what people reach
// for most; the primary action closes the row.
func (a *app) pageHeader(c *ui.Context, pal *Palette) {
	bar := c.TitleBar()
	topPadding := max(bar.Height-30, 10)
	// Keep clear of the Windows window controls at the top right corner.
	rightPadding := max(bar.Right+16, 148)

	ui.Row(c).Height(58).Padding(topPadding, rightPadding, 0, 20).Gap(10).
		AlignItems(ui.Center).Background(pal.MainBg).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		DragWindow().Children(func() {
		ui.Column(c).Shrink(0).Gap(1).Justify(ui.Center).Children(func() {
			title := a.scopeTitle()
			ui.Text(c, title).FontSize(sf(17)).FontWeight(600).
				TextColor(pal.Text).SingleLine().Shrink(0).Selectable()
			ui.Text(c, a.scopeBreadcrumb()).FontSize(sf(10.5)).
				TextColor(pal.TextFaint).SingleLine().Selectable()
		})

		ui.Spacer(c)

		if ui.SearchField(c, &a.query).Label("搜索器件").
			Placeholder(a.searchPlaceholder()).Width(210).
			Bind(&a.searchHandle).
			Changed() {
			a.refreshResults(0)
			a.detailScroll.Y = 0
		}

		a.filterMenu(c, pal)
		a.sortMenu(c, pal)

		ui.Box(c).Width(1).Height(20).Background(pal.BorderSubtle).Shrink(0)

		themeIcon, themeTip := a.themeButton()
		if themeBtn := iconButton(c, themeIcon, themeTip, pal); themeBtn.Clicked() {
			a.toggleTheme(c)
		}

		infoBtn := iconButton(c, "columns-3", "详情栏", pal)
		if a.detailVisible() {
			infoBtn.Background(pal.AccentCyanBg)
		}
		if infoBtn.Clicked() {
			a.inspectorOpen = !a.inspectorOpen
			a.cfg.DetailPinned = a.inspectorOpen
			_ = saveConfig(a.exeDir, a.cfg)
		}

		if setBtn := iconButton(c, "settings", "目录与设置", pal); setBtn.Clicked() {
			a.openDatasheetSettings()
		}

		if add := ui.PrimaryButton(c, "+ 新增器件").Height(30); add.Clicked() {
			a.openAdd()
		}
	})
}

// originalTableName is the label the sidebar uses for a source workbook: the
// original file's own name, without the shared prefix and extension, so
// "Master E-list_Cable&Wire.xlsx" reads "Cable&Wire".
func originalTableName(file string) string {
	base := filepath.Base(file)
	stem := strings.TrimSuffix(base, filepath.Ext(base))
	const prefix = "master e-list_"
	if len(stem) >= len(prefix) && strings.EqualFold(stem[:len(prefix)], prefix) {
		stem = stem[len(prefix):]
	}
	return stem
}

// scopeTitle names the current view in one line: the sheet when one is
// selected, otherwise the workbook, otherwise the whole library.
func (a *app) scopeTitle() string {
	if sheet := a.selectedSheet(); sheet != "" {
		return sheet
	}
	if book := a.selectedBook(); book != nil {
		return originalTableName(book.SourceFile)
	}
	return "全部器件"
}

// scopeBreadcrumb tells where the view sits and how much it holds:
// "附件 · 1,225 件" or "附件 / cable duct · 32 件".
func (a *app) scopeBreadcrumb() string {
	count := formatInt(len(a.results)) + " 件"
	if book := a.selectedBook(); book != nil {
		trail := originalTableName(book.SourceFile)
		if sheet := a.selectedSheet(); sheet != "" {
			trail += " / " + sheet
		}
		return trail + " · " + count
	}
	return "共 " + count + "，来自 " + formatInt(len(a.books)) + " 份原表"
}

func (a *app) searchPlaceholder() string {
	if sheet := a.selectedSheet(); sheet != "" {
		return "在当前工作表中搜索"
	}
	if book := a.selectedBook(); book != nil {
		return "在 " + originalTableName(book.SourceFile) + " 中搜索"
	}
	return "搜索零件号、型号、订货号..."
}

// scopeStrip answers one question: what am I looking at? A sheet selector
// when a workbook is open, and the active filters when any are on. It never
// draws a scroll bar over its own content.
func (a *app) scopeStrip(c *ui.Context, pal *Palette) {
	book := a.selectedBook()
	ui.Row(c).Height(42).Padding(0, 20).AlignItems(ui.Center).Gap(10).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.BorderSubtle).
		Background(pal.AppBg).Children(func() {
		if book != nil {
			a.ensureSheetChoice(book)
			ui.Text(c, "工作表").FontSize(sf(11)).FontWeight(500).
				TextColor(pal.TextFaint).Shrink(0)
			prev := iconButton(c, "chevron-left", "上一个工作表", pal)
			if prev.Clicked() {
				a.stepSheet(-1)
			}
			if ui.Combobox(c, &a.sheetChoice, a.sheetChoices).
				Label("工作表").Width(260).Changed() {
				a.chooseSheet(a.sheetChoice)
			}
			next := iconButton(c, "chevron-right", "下一个工作表", pal)
			if next.Clicked() {
				a.stepSheet(1)
			}
			ui.Text(c, a.sheetSummary(book)).FontSize(sf(11)).TextColor(pal.TextFaint).SingleLine()
		}
		ui.Spacer(c)
		a.activeFilterChipsRow(c, pal)
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

// toolMenuButton is the shared shape of the toolbar menus: an optional glyph,
// the current value, and a chevron. It looks like a control, not a card.
func (a *app) toolMenuButton(c *ui.Context, key, iconName, label string, active bool, pal *Palette) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(30).Padding(0, 9).Gap(6).
		Radius(6).AlignItems(ui.Center).Label(label).Tooltip(label)
	b.Border(1, pal.Border).Background(pal.Surface)
	if active {
		b.Border(1, pal.Accent).Background(pal.AccentCyanBg)
	}
	if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		color := pal.TextMuted
		textColor := pal.Text
		if active {
			color = pal.Accent
			textColor = pal.Accent
		}
		if ic := icon(iconName); ic != nil {
			ui.Icon(c, ic).FontSize(sf(12)).TextColor(color)
		}
		ui.Text(c, label).FontSize(sf(11.5)).FontWeight(500).TextColor(textColor).SingleLine()
		if ic := icon("chevron-down"); ic != nil {
			ui.Icon(c, ic).FontSize(sf(10)).TextColor(pal.TextFaint)
		}
	})
	return b
}

func (a *app) sortMenu(c *ui.Context, pal *Palette) {
	label := a.sortLabel
	if a.sortDesc {
		label += " ↓"
	} else {
		label += " ↑"
	}
	b := a.toolMenuButton(c, "sort-menu", "arrow-up-down", label,
		sortModeFromLabel(a.sortLabel) != SortSourceOrder, pal)
	b.Menu(func(m *ui.Menu) {
		for _, option := range sortModeOptions() {
			if m.Item(option).Checked(a.sortLabel == option).Chosen() {
				a.setSort(option)
			}
		}
		m.Separator()
		if m.Item("升序").Checked(!a.sortDesc).Chosen() {
			a.setSortDesc(false)
		}
		if m.Item("降序").Checked(a.sortDesc).Chosen() {
			a.setSortDesc(true)
		}
	})
}

// filterMenu is one menu for every list filter: status, category,
// manufacturer and datasheet presence.
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
	label := "筛选"
	if active > 0 {
		label = fmt.Sprintf("筛选 · %d", active)
	}
	b := a.toolMenuButton(c, "filter-menu", "sliders-horizontal", label, active > 0, pal)
	b.Menu(func(m *ui.Menu) {
		m.Submenu("状态", func(sm *ui.Menu) {
			for i, option := range filterStatusOptions {
				if sm.Item(option).Checked(a.filterStatus == i).Chosen() {
					a.filterStatus = i
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
		m.Submenu("品牌", func(sm *ui.Menu) {
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
		if a.filterActive() {
			m.Separator()
			if m.Item("清除全部筛选").Chosen() {
				a.clearFilters()
			}
		}
	})
}

// activeFilterChipsRow shows what is filtered, and only exists while
// something is: no empty placeholder row.
// clearFilters turns every list filter off, keeping the sidebar scope.
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

func (a *app) activeFilterChipsRow(c *ui.Context, pal *Palette) {
	if !a.filterActive() {
		return
	}
	a.activeFilterChips(c, pal)
	if clear := accentLink(c, "chip:clear-all", "清除筛选", pal); clear.Clicked() {
		a.clearFilters()
	}
}

// activeFilterChips lists the filters that are on, each removable on its own.
func (a *app) activeFilterChips(c *ui.Context, pal *Palette) {
	chip := func(key, label string, clear func()) {
		b := ui.ButtonBase(c.Key("chip:"+key)).Height(22).Padding(0, 8).Gap(5).
			Radius(11).Background(pal.AccentCyanBg).AlignItems(ui.Center).Label(label)
		b.Children(func() {
			ui.Text(c, label).FontSize(sf(10.5)).FontWeight(500).TextColor(pal.Accent).SingleLine()
			if ic := icon("x"); ic != nil {
				ui.Icon(c, ic).FontSize(sf(9)).TextColor(pal.Accent)
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
// mode it sets, and the width the rows reserve for it. A zero width means the
// column grows to fill what the others leave.
type tableColumn struct {
	label string
	mode  SortMode
	width float32
}

var resultColumns = []tableColumn{
	{"零件号", SortBHS, 98},
	{"元件名称", SortName, 0},
	{"型号 / 规格", SortSpec, 122},
	{"品牌 / 供应商", SortManufacturer, 82},
	{"工作表", SortSourceOrder, 92},
	{"手册", "", 48},
	{"状态", "", 56},
}

// columnSpacing is the gap between table columns, shared by the header and
// body so the two stay aligned.
const columnSpacing float32 = 8

func (a *app) tableHeader(c *ui.Context, pal *Palette) ui.Element {
	return ui.Row(c.Key("table-header")).Height(34).Padding(0, 20).
		Gap(columnSpacing).AlignItems(ui.Center).Clip().Background(pal.MainBg).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).Children(func() {
		for _, col := range resultColumns {
			if col.mode != "" {
				a.sortColumnHeader(c, col, pal)
				continue
			}
			t := ui.Text(c, col.label).FontSize(sf(10.5)).FontWeight(500).
				TextColor(pal.TextFaint).SingleLine()
			if col.width == 0 {
				t.Grow(1)
			} else {
				t.Width(col.width)
			}
			if col.label == "手册" || col.label == "状态" {
				t.TextAlign(ui.Center)
			}
		}
	})
}

// sortColumnHeader draws one title; clicking sorts by it and a second click
// reverses the order. Only the active column shows an arrow, which keeps the
// header from reading as seven controls.
func (a *app) sortColumnHeader(c *ui.Context, col tableColumn, pal *Palette) {
	active := sortModeFromLabel(a.sortLabel) == col.mode
	// The box is sized like the row cells so header and body line up; the
	// active state changes only the colours.
	box := ui.Row(c).Height(26).AlignItems(ui.Center).Shrink(0)
	if col.width == 0 {
		box.Grow(1).MinWidth(130)
	} else {
		box.Width(col.width)
	}
	box.Children(func() {
		h := ui.ButtonBase(c.Key("col:"+col.label)).Height(24).Padding(0, 4).
			Radius(5).AlignItems(ui.Center).Justify(ui.Start).
			Label("按 " + col.label + " 排序")
		if active {
			// Only the colour and the arrow mark the active sort; a filled
			// header would compete with the selected row.
			h.Background(ui.Color{})
		} else if h.Hovered() {
			h.Background(pal.SurfaceHover)
		}
		textColor := pal.TextFaint
		if active {
			textColor = pal.Accent
		}
		h.Children(func() {
			ui.Text(c, col.label).FontSize(sf(10.5)).FontWeight(500).
				TextColor(textColor).SingleLine()
			if active {
				iconName := "arrow-up"
				if a.sortDesc {
					iconName = "arrow-down"
				}
				if ic := icon(iconName); ic != nil {
					ui.Icon(c, ic).FontSize(sf(9)).TextColor(textColor).Margin(3, 0, 0, 0)
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
	})
}

func (a *app) renderResultList(c *ui.Context, pal *Palette) ui.Element {
	a.resultList.Selected = &a.resultSel
	a.resultList.Key = func(i int) any { return a.results[i].ID }
	a.resultList.Label = func(i int) string {
		p := a.results[i]
		return p.BHSNo + " " + p.Name()
	}
	list := ui.List(c.Key("table-list"), &a.resultList, len(a.results)).Grow(1).
		RowWidth(706).Rows(func(row ui.ListRow) { a.resultRow(row, pal) }).Label("器件列表")
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
	if row.Index%2 == 1 {
		background = pal.RowOdd
	}
	if selected {
		background = pal.RowSelected
	}
	r := ui.Row(c.Key(fmt.Sprintf("result:%d", p.ID))).Height(42).
		Padding(0, 20).Gap(columnSpacing).AlignItems(ui.Center).
		Background(background)
	if selected {
		// A left rail marks the chosen row without shifting its cells: it is
		// painted over the row's edge, not laid out inside it.
		left := pal.Accent
		r.DrawOver(func(p *ui.Painter, rect ui.Rect) {
			p.Fill(ui.Rect{X: rect.X, Y: rect.Y, W: 2, H: rect.H}, left, 0)
		})
	}
	if !selected && r.Hovered() {
		r.Background(pal.RowHover)
	}
	r.Children(func() {
		bhs := p.BHSNo
		if bhs == "" {
			bhs = "未编号"
		}
		ui.Text(c, bhs).Font("monospace").FontSize(sf(11.5)).FontWeight(600).
			TextColor(pal.AccentCyan).Width(98).SingleLine()

		nameColor := pal.Text
		if selected {
			nameColor = pal.Text
		}
		ui.Row(c).Grow(1).MinWidth(120).Gap(6).AlignItems(ui.Center).Clip().Children(func() {
			ui.Text(c, p.Name()).FontSize(sf(12.5)).FontWeight(500).
				TextColor(nameColor).Shrink(1).SingleLine()
			if p.Discontinued {
				ui.Text(c, "弃用").FontSize(sf(9.5)).FontWeight(600).
					TextColor(pal.WarningText).Shrink(0).SingleLine()
			}
		})

		ui.Text(c, valueOr(p.Spec, "—")).FontSize(sf(11.5)).
			TextColor(pal.TextMuted).Width(122).SingleLine()

		ui.Text(c, valueOr(p.Manufacturer, "—")).FontSize(sf(11.5)).
			TextColor(pal.TextMuted).Width(82).SingleLine()

		ui.Text(c, p.SourceLabel()).FontSize(sf(10.5)).
			TextColor(pal.TextFaint).Width(92).SingleLine()

		ui.Row(c).Width(48).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			pdfs := a.pdfScanner.FindForBHS(p.BHSNo)
			if len(pdfs) == 0 {
				ui.Text(c, "—").FontSize(sf(10.5)).TextColor(pal.TextFaint)
				return
			}
			b := ui.ButtonBase(c.Key(fmt.Sprintf("pdf-count:%d", p.ID))).Height(22).
				Padding(0, 6).Radius(5).AlignItems(ui.Center).Label("查看数据手册")
			b.Children(func() {
				if ic := icon("file-down"); ic != nil {
					ui.Icon(c, ic).FontSize(sf(11)).TextColor(pal.Accent)
				}
				ui.Textf(c, "%d", len(pdfs)).FontSize(sf(10.5)).FontWeight(600).
					TextColor(pal.Accent)
			})
			if b.Hovered() {
				b.Background(pal.AccentCyanBg)
			}
			if b.Clicked() {
				a.resultSel = row.Index
				a.inspectorOpen = true
				a.detailScroll.Y = 0
				if len(pdfs) == 1 {
					if err := a.OpenDatasheetPDF(pdfs[0].FullPath); err != nil {
						c.Toast("打开 PDF 失败: " + err.Error())
					}
				} else {
					c.Toast(fmt.Sprintf("%s 有 %d 份数据手册，见右侧详情", p.BHSNo, len(pdfs)))
				}
			}
		})

		ui.Row(c).Width(56).Justify(ui.Center).Children(func() {
			statusIndicator(c, p.Status, a.lightTheme)
		})
	})
}

func (a *app) emptyResults(c *ui.Context, pal *Palette) {
	emptyStateView(c, "search", "没有找到匹配的器件",
		"换个关键词试试，或从左侧选择其他原表分类。", pal)
}

// detailVisible reports whether the detail pane takes room: the pinned
// setting keeps it there, otherwise it appears with a selection.
func (a *app) detailVisible() bool {
	if a.cfg.DetailPinned {
		return true
	}
	return a.inspectorOpen && a.selectedPart() != nil
}

func (a *app) detailPanel(c *ui.Context, pal *Palette) {
	p := a.selectedPart()
	if p == nil {
		a.detailPlaceholder(c, pal)
		return
	}
	ui.Column(c).Width(336).Shrink(0).FillHeight().
		Background(pal.SidebarBg).
		BorderWidth(0, 0, 0, 1).BorderColor(pal.Border).
		Children(func() {
			a.detailHeader(c, p, pal)
			ui.Divider(c).Background(pal.BorderSubtle)
			ui.Scroll(c).TrackScroll(&a.detailScroll).Grow(1).Selectable().
				Padding(14, 18, 24, 18).Gap(14).Children(func() {
				a.datasheetCard(c, p, pal)
				a.sourceCard(c, p, pal)
				a.detailSection(c, "规格", [][2]string{
					{"型号", p.Spec},
					{"品牌", p.Manufacturer},
					{"订货号", p.OrderNo},
					{"单位", p.Unit},
					{"属性", p.Dispo},
				}, pal)
				a.detailSection(c, "分类与机型", [][2]string{
					{"类别", categoryName(p.Category)},
					{"子类别", p.Subcategory},
					{"机型 / 功能", p.Machine},
					{"认证", p.Cert},
					{"WHR 进口", p.ImportFlag},
				}, pal)
				a.detailSection(c, "替代与关联", [][2]string{
					{"CN 零件号", p.CNPartNo},
					{"DongAn 外包号", p.DonganPartNo},
					{"S 零件号", p.SPartNo},
				}, pal)
				a.alternateLinks(c, p, pal)
				a.detailSection(c, "库存与供货", [][2]string{
					{"价格", p.Price},
					{"备件级别", p.SpareLevel},
					{"当前库存", p.Stock},
					{"安全库存", p.SafetyStock},
					{"协议库存", p.ProtocolStock},
					{"起订量 MOQ", p.MOQ},
					{"货期", p.Delivery},
					{"SLOB", p.SLOB},
				}, pal)
				if p.Remark != "" {
					a.detailSection(c, "备注", [][2]string{{"说明", p.Remark}}, pal)
				}
				ui.Column(c).Gap(3).Padding(0, 0, 8, 0).Children(func() {
					ui.Textf(c, "记录 #%d · 更新于 %s", p.ID, valueOr(formatImportedAt(p.UpdatedAt), "—")).
						FontSize(sf(10)).TextColor(pal.TextFaint).Wrap()
				})
			})
		})
}

// copyOnDouble copies a value when its selectable element is double
// clicked. The element keeps its own styling; only the gesture is added.
func copyOnDouble(c *ui.Context, value string, el ui.Element, toast func(string)) {
	if el.DoubleClicked() {
		c.Services().WriteClipboard(value)
		toast("已复制：" + value)
	}
}

// copyableText shows a value the user can drag to select, and copies the
// whole value on a double click: selecting a part number or a specification
// and pasting it is the common move in a parts library, so it takes one
// gesture instead of two.
func (a *app) copyableText(c *ui.Context, value string, size float32, color ui.Color, toast func(string)) {
	t := ui.Text(c, value).FontSize(sf(size)).TextColor(color).Grow(1).Wrap().Selectable()
	if t.DoubleClicked() {
		c.Services().WriteClipboard(value)
		toast("已复制：" + value)
	}
}

// alternateLinks shows the replacement part numbers as links: in the
// production sheets the 二手零件号 / 关联零件号 column names the current part
// that replaces this row, so a click jumps to that part in the same table.
func (a *app) alternateLinks(c *ui.Context, p *Part, pal *Palette) {
	links := []struct {
		label string
		value string
	}{
		{"旧零件号 / G No.", p.AlternateNo},
		{"二手 / 关联零件号", p.RelatedPart},
	}
	hasAny := false
	for _, l := range links {
		if strings.TrimSpace(l.value) != "" {
			hasAny = true
			break
		}
	}
	if !hasAny {
		return
	}
	ui.Column(c).Gap(6).Children(func() {
		sectionLabel(c, "替代零件", pal)
		ui.Column(c).Children(func() {
			for _, l := range links {
				value := strings.TrimSpace(l.value)
				if value == "" {
					continue
				}
				link := l
				ui.Row(c).AlignItems(ui.Start).Gap(10).Padding(4, 0).Children(func() {
					ui.Text(c, link.label).FontSize(sf(10.5)).TextColor(pal.TextFaint).
						Width(96).Shrink(0)
					// Split on separators so several numbers each act as a
					// link, and any text stays readable.
					ui.Row(c).Grow(1).Gap(8).Children(func() {
						for _, token := range splitPartNumbers(value) {
							a.partLink(c, token, pal)
						}
					})
				})
			}
		})
	})
}

// partLink jumps to the part a number names, or copies the number when the
// library has no such part (a loss-machine number, say).
func (a *app) partLink(c *ui.Context, number string, pal *Palette) {
	number = strings.TrimSpace(number)
	if number == "" {
		return
	}
	if strings.TrimSpace(number) == "" || !looksLikePartNumber(number) {
		ui.Text(c, number).FontSize(sf(11.5)).TextColor(pal.Text).Wrap().Selectable()
		return
	}
	key := "part-link:" + number
	if b := accentLink(c, key, number, pal); b.Clicked() {
		if a.jumpToPart(number) {
			return
		}
		c.Services().WriteClipboard(number)
		c.Toast("库中没有 " + number + "，已复制该零件号")
	}
}

// detailPlaceholder fills the detail pane while nothing is chosen, so the
// pane keeps its width and the table does not jump on every selection.
func (a *app) detailPlaceholder(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(336).Shrink(0).FillHeight().
		Background(pal.SidebarBg).
		BorderWidth(0, 0, 0, 1).BorderColor(pal.Border).
		Children(func() {
			ui.Row(c).Padding(14, 18).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "器件详情").FontSize(sf(12)).FontWeight(600).TextColor(pal.TextMuted)
				ui.Spacer(c)
				if closeBtn := iconButton(c, "x", "隐藏详情栏", pal); closeBtn.Clicked() {
					a.inspectorOpen = false
					a.cfg.DetailPinned = false
					_ = saveConfig(a.exeDir, a.cfg)
				}
			})
			ui.Column(c).Grow(1).Center().Gap(8).Padding(24).Children(func() {
				if ic := icon("info"); ic != nil {
					ui.Icon(c, ic).FontSize(sf(22)).TextColor(pal.TextFaint)
				}
				ui.Text(c, "选择一个器件").FontSize(sf(12.5)).FontWeight(500).TextColor(pal.TextMuted)
				ui.Text(c, "点击列表中的任意一行，规格、来源与手册会显示在这里。").
					FontSize(sf(11)).TextColor(pal.TextFaint).TextAlign(ui.Center).Wrap()
			})
		})
}

func (a *app) detailHeader(c *ui.Context, p *Part, pal *Palette) {
	ui.Column(c).Gap(8).Padding(14, 18, 14, 18).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			statusIndicator(c, p.Status, a.lightTheme)
			ui.Spacer(c)
			if copyBtn := iconButton(c, "copy", "复制零件号", pal); copyBtn.Clicked() && p.BHSNo != "" {
				c.Services().WriteClipboard(p.BHSNo)
				c.Toast("已复制 " + p.BHSNo)
			}
			if closeBtn := iconButton(c, "x", "收起详情", pal); closeBtn.Clicked() {
				a.inspectorOpen = false
			}
		})
		ui.Column(c).Gap(2).Selectable().Children(func() {
			bhsText := ui.Text(c, valueOr(p.BHSNo, "未编号")).FontSize(sf(19)).FontWeight(600).
				Font("monospace").LetterSpacing(-0.3).TextColor(pal.Text).SingleLine().Selectable()
			copyOnDouble(c, p.BHSNo, bhsText, c.Toast)
			nameText := ui.Text(c, p.Name()).FontSize(sf(13)).FontWeight(500).TextColor(pal.Text).Wrap().Selectable()
			copyOnDouble(c, p.Name(), nameText, c.Toast)
			if p.SecondaryName() != "" {
				ui.Text(c, p.SecondaryName()).FontSize(sf(11)).TextColor(pal.TextFaint).Wrap()
			}
			line := p.Spec
			if p.Manufacturer != "" {
				if line != "" {
					line += " · "
				}
				line += p.Manufacturer
			}
			if line != "" {
				ui.Text(c, line).FontSize(sf(11)).TextColor(pal.TextMuted).SingleLine()
			}
		})
		ui.Row(c).Gap(8).Margin(2, 0, 0, 0).Children(func() {
			edit := ui.ButtonBase(c.Key("btn:edit-part")).Height(28).Padding(0, 12).
				Radius(6).Background(pal.Accent).AlignItems(ui.Center)
			if edit.Hovered() {
				edit.Background(pal.AccentHover)
			}
			edit.Children(func() {
				ui.Text(c, "修改器件").FontSize(sf(11.5)).FontWeight(600).TextColor(pal.AccentText)
			})
			if edit.Clicked() {
				a.openEdit()
			}
			if p.SourceFile != "" {
				if loc := quietButton(c, "btn:locate-source", "在分类中定位", "folder-open", pal); loc.Clicked() {
					a.showPartSource(p)
				}
			}
		})
	})
}

// sourceCard is the workbook origin: where this row came from and how to
// reach it again.
func (a *app) sourceCard(c *ui.Context, p *Part, pal *Palette) {
	if p.SourceFile == "" {
		ui.Column(c).Gap(4).Children(func() {
			sectionLabel(c, "来源", pal)
			ui.Text(c, "手动维护记录（未关联原表）").FontSize(sf(11)).TextColor(pal.TextMuted).Wrap()
		})
		return
	}
	book := a.bookByFile(p.SourceFile)
	ui.Column(c).Gap(6).Children(func() {
		sectionLabel(c, "来源", pal)
		ui.Column(c).Gap(3).Selectable().Children(func() {
			fileText := ui.Text(c, p.SourceFile).FontSize(sf(11.5)).FontWeight(500).TextColor(pal.Text).Wrap().Selectable()
			copyOnDouble(c, p.SourceFile, fileText, c.Toast)
			sheetLabel := p.SourceSheet
			if sheetLabel == "" {
				sheetLabel = p.Subcategory
			}
			if p.SourceRow > 0 {
				sheetLabel = fmt.Sprintf("%s · 第 %d 行", sheetLabel, p.SourceRow)
			}
			metaText(c, sheetLabel, pal)
		})
		ui.Row(c).Gap(10).Margin(2, 0, 0, 0).Children(func() {
			if openExcel := accentLink(c, "btn:open-excel-part", "打开本地副本", pal); openExcel.Clicked() {
				a.openExcelFile(c, p.SourceFile)
			}
			if locate := accentLink(c, "btn:locate-excel-file", "定位副本", pal); locate.Clicked() {
				a.locateExcelFile(c, p.SourceFile)
			}
			if book != nil && len(book.Sheets) > 1 {
				if cbtn := accentLink(c, "btn:switch-sheet", "下一个工作表", pal); cbtn.Clicked() {
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

// detailSection renders a group of label/value pairs only when it holds a
// value, so an empty field never leaves a blank row behind.
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
	ui.Column(c).Gap(6).Children(func() {
		sectionLabel(c, title, pal)
		ui.Column(c).Children(func() {
			for _, field := range fields {
				if strings.TrimSpace(field[1]) == "" {
					continue
				}
				ui.Row(c).AlignItems(ui.Start).Gap(10).Padding(4, 0).Children(func() {
					ui.Text(c, field[0]).FontSize(sf(10.5)).TextColor(pal.TextFaint).
						Width(96).Shrink(0)
					a.copyableText(c, field[1], 11.5, pal.Text, c.Toast)
				})
			}
		})
	})
}

func (a *app) statusBar(c *ui.Context, pal *Palette) {
	ui.Row(c).Height(28).Padding(0, 20).Gap(10).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(pal.BorderSubtle).
		Background(pal.StatusBarBg).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success)
		ui.Text(c, "已连接数据库").FontSize(sf(10.5)).TextColor(pal.TextMuted).SingleLine()
		ui.Text(c, "共 "+formatInt(a.summary.Total)+" 件器件").
			FontSize(sf(10.5)).TextColor(pal.TextFaint).SingleLine()
		if n := a.pdfScanner.Count(); n > 0 {
			ui.Textf(c, "手册库 %s 份", formatInt(n)).
				FontSize(sf(10.5)).TextColor(pal.TextFaint).SingleLine()
		}
		if a.filterActive() {
			ui.Textf(c, "当前筛选显示 %s 件", formatInt(len(a.results))).
				FontSize(sf(10.5)).TextColor(pal.Accent).SingleLine()
		}
		ui.Spacer(c)
		ui.Text(c, "Ctrl+F 搜索 · 双击行修改").
			FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine().Selectable()
	})
}

func (a *app) form(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(660).Padding(20, 24).Gap(14).Radius(10).Selectable().
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				title := "新增器件"
				if a.editing {
					title = "修改器件"
				}
				ui.Text(c, title).FontSize(sf(17)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "填写参数并保存到本地器件库。").FontSize(sf(11.5)).TextColor(pal.TextMuted).Selectable()
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.formOpen = false
				a.formError = ""
			}
		})
		ui.Divider(c).Background(pal.BorderSubtle)
		ui.Grid(c).Columns(2).GapX(14).GapY(7).Children(func() {
			a.formField(c, "零件号 *", func() {
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
			ui.Textf(c, "另有 %d 条相同零件号的记录；保存后它们都会保留。", dupes).
				FontSize(sf(10.5)).TextColor(pal.WarningText)
		}
		if a.formError != "" {
			ui.Text(c, a.formError).FontSize(sf(11)).TextColor(pal.DangerText).
				Padding(5, 8).Radius(4).Background(pal.Danger.Alpha(0.12))
		}
		ui.Row(c).Justify(ui.End).Gap(8).Margin(8, 0, 0, 0).Children(func() {
			if ui.Button(c, "取消").Height(30).Clicked() {
				a.formOpen = false
				a.formError = ""
			}
			if ui.PrimaryButton(c, "保存").Height(30).Clicked() {
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
			sectionLabel(c, "数据手册", pal)
			if len(matchedPDFs) > 0 {
				ui.Textf(c, "%d 份", len(matchedPDFs)).FontSize(sf(10)).
					FontWeight(600).TextColor(pal.Accent)
			}
			ui.Spacer(c)
			if a.pdfScanner.Scanning() {
				ui.Text(c, "同步中…").FontSize(sf(10)).TextColor(pal.TextFaint)
			}
			if listBtn := accentLink(c, "btn:pdf-filelist", "文件列表", pal); listBtn.Clicked() {
				a.openFileList()
			}
		})

		if len(matchedPDFs) == 0 {
			ui.Column(c).Gap(6).Padding(10, 12).Radius(7).Background(pal.Surface).
				Border(1, pal.BorderSubtle).Children(func() {
				ui.Text(c, "没有匹配到 PDF 手册").FontSize(sf(11.5)).TextColor(pal.TextMuted)
				metaText(c, "匹配文件名中的零件号：'"+valueOr(p.BHSNo, "-")+"'", pal)
				ui.Row(c).Gap(10).Margin(2, 0, 0, 0).Children(func() {
					if cfg := accentLink(c, "btn:cfg-pdf-inline", "配置手册目录", pal); cfg.Clicked() {
						a.openDatasheetSettings()
					}
					if sync := accentLink(c, "btn:sync-pdf-inline", "同步手册索引", pal); sync.Clicked() {
						a.scanDatasheetsAsync(true)
					}
				})
			})
			return
		}

		ui.Column(c).Children(func() {
			for i, pdf := range matchedPDFs {
				a.datasheetRow(c, p, i, pdf, pal)
			}
		})
		if hiddenDuplicates > 0 {
			ui.Textf(c, "另有 %d 份同名同大小的文件按重复处理，未显示。", hiddenDuplicates).
				FontSize(sf(10)).TextColor(pal.TextFaint).Wrap()
		}
	})
}

// datasheetRow is one datasheet: name, size and date, and the open/locate
// actions. A rule between rows keeps the list readable without a card per
// file.
func (a *app) datasheetRow(c *ui.Context, p *Part, i int, pdf PDFFile, pal *Palette) {
	ui.Column(c).Key(fmt.Sprintf("pdf:%s:%d", p.BHSNo, i)).Children(func() {
		if i > 0 {
			ui.Divider(c).Background(pal.BorderSubtle)
		}
		ui.Column(c).Padding(7, 0).Gap(4).Selectable().Children(func() {
			nameText := ui.Text(c, pdf.Name).FontSize(sf(11)).FontWeight(500).
				TextColor(pal.Text).Wrap().Selectable()
			copyOnDouble(c, pdf.Name, nameText, c.Toast)
			ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
				metaText(c, formatFileSize(pdf.Size)+" · "+formatModTime(pdf.ModTime), pal)
				ui.Spacer(c)
				if open := accentLink(c, fmt.Sprintf("open:%s:%d", p.BHSNo, i), "打开", pal); open.Clicked() {
					a.openPDFInDefault(c, pdf.FullPath)
				}
				if loc := accentLink(c, fmt.Sprintf("loc:%s:%d", p.BHSNo, i), "定位", pal); loc.Clicked() {
					revealPDF(pdf.FullPath)
				}
			})
		})
	})
}

func (a *app) settingsModal(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(700).HeightPercent(86).Padding(20, 24).Gap(14).Radius(10).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "设置").FontSize(sf(17)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "手册与原始表格的存放位置、状态栏和 PDF 打开方式。").
					FontSize(sf(11.5)).TextColor(pal.TextMuted).Selectable()
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.datasheetModalOpen = false
			}
		})
		ui.Divider(c).Background(pal.BorderSubtle)

		ui.Tabs(c, &a.settingsTab, "数据手册目录", "原表目录", "常规")

		// The tab body scrolls: the general tab holds several groups and
		// would otherwise run past the window.
		ui.Scroll(c).Grow(1).Selectable().
			Padding(0, 2, 2, 2).Children(func() {
			ui.Column(c).Gap(12).Children(func() {
				switch a.settingsTab {
				case 0:
					a.settingsDatasheetTab(c, pal)
				case 1:
					a.settingsExcelTab(c, pal)
				default:
					a.settingsGeneralTab(c, pal)
				}
			})
		})

		if a.datasheetStatusMsg != "" {
			ui.Text(c, a.datasheetStatusMsg).FontSize(sf(11)).TextColor(pal.DangerText).
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
	ui.Field(c, "手册根目录（每行一条，含子文件夹）", func() {
		ui.TextArea(c, &a.datasheetPathsText).Lines(4, 5)
	})
	ui.Text(c, `可填写本地文件夹、映射网络盘（Z:\手册）或 UNC 路径（\\server\share\pdf）。网络盘不可达时沿用上次索引。`).
		FontSize(sf(10.5)).TextColor(pal.TextFaint).Wrap()

	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			sectionLabel(c, "扫描状态", pal)
			ui.Spacer(c)
			if a.pdfScanner.Scanning() {
				ui.Text(c, "正在扫描…").FontSize(sf(10)).TextColor(pal.Accent)
			}
		})
		ui.Column(c).Children(func() {
			statuses := a.pdfScanner.Status()
			if len(statuses) == 0 {
				ui.Row(c).Padding(8, 10).Children(func() {
					ui.Text(c, "暂无已配置路径").FontSize(sf(11)).TextColor(pal.TextMuted)
				})
			}
			for _, st := range statuses {
				stCopy := st
				ui.Row(c).AlignItems(ui.Start).Gap(9).Padding(7, 2).Children(func() {
					if stCopy.Exists {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Success).Margin(2, 0, 0, 0)
					} else {
						ui.Box(c).Size(6, 6).Radius(3).Background(pal.Danger).Margin(2, 0, 0, 0)
					}
					ui.Column(c).Grow(1).Gap(2).Children(func() {
						ui.Text(c, stCopy.ConfiguredPath).FontSize(sf(11.5)).FontWeight(500).
							TextColor(pal.Text).SingleLine()
						statusDesc := fmt.Sprintf("%s · 含子文件夹 %s 份 PDF", stCopy.ResolvedPath, formatInt(stCopy.Count))
						if !stCopy.Exists {
							statusDesc = fmt.Sprintf("%s · 目录或网络盘不可用", stCopy.ResolvedPath)
						}
						metaText(c, statusDesc, pal)
						if stCopy.Error != "" {
							ui.Text(c, stCopy.Error).FontSize(sf(10)).
								TextColor(pal.WarningText).Wrap()
						}
					})
				})
			}
		})
	})

	ui.Row(c).AlignItems(ui.Center).Gap(8).Children(func() {
		ui.Textf(c, "共索引 %s 份 PDF", formatInt(a.pdfScanner.Count())).
			FontSize(sf(11.5)).FontWeight(500).TextColor(pal.Text).SingleLine()
		ui.Spacer(c)
		if a.pdfScanner.FromCache() {
			ui.Text(c, "来自本地缓存").FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine()
		}
		if at := a.pdfScanner.ScannedAt(); !at.IsZero() {
			metaText(c, at.Format("2006.01.02 15:04"), pal)
		}
	})
}

func (a *app) settingsExcelTab(c *ui.Context, pal *Palette) {
	ui.Field(c, "原表目录（每行一条）", func() {
		ui.TextArea(c, &a.masterlistPathsText).Lines(4, 5)
	})

	ui.Field(c, "排除的原表（每行一个文件名或完整路径，也可填目录排除其下全部原表）", func() {
		ui.TextArea(c, &a.excelExcludeText).Lines(3, 4)
	})
	ui.Text(c, "排除项不参与导入，也不会下载到本地副本。").
		FontSize(sf(10.5)).TextColor(pal.TextFaint).Wrap()

	ui.Column(c).Gap(6).Children(func() {
		ui.Text(c, "当前原表路径检测与 Excel 文件数:").FontSize(sf(10.5)).FontWeight(700).
			LetterSpacing(0.8).TextColor(pal.TextFaint)
		ui.Column(c).Border(1, pal.Border).Radius(6).Background(pal.Surface).
			Dividers(1, pal.BorderSubtle).Children(func() {
			lines := strings.Split(a.masterlistPathsText, "\n")
			statuses := checkExcelDirStatuses(a.exeDir, lines)
			if len(statuses) == 0 {
				ui.Row(c).Padding(8, 10).Children(func() {
					ui.Text(c, "暂无已配置路径").FontSize(sf(11)).TextColor(pal.TextMuted)
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
						ui.Text(c, stCopy.ConfiguredPath).FontSize(sf(11.5)).FontWeight(600).
							TextColor(pal.Text).SingleLine()
						statusDesc := fmt.Sprintf("-> %s (已检索到 %d 份 Excel 表格)", stCopy.ResolvedPath, stCopy.FileCount)
						if !stCopy.Exists {
							statusDesc = fmt.Sprintf("-> %s (目录不存在)", stCopy.ResolvedPath)
						}
						ui.Text(c, statusDesc).FontSize(sf(10.5)).TextColor(pal.TextMuted).SingleLine()
					})
				})
			}
		})
	})
}

func (a *app) settingsGeneralTab(c *ui.Context, pal *Palette) {
	sectionLabel(c, "外观与界面", pal)
	ui.Column(c).Background(pal.Surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "外观").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "跟随系统时随 Windows 的浅色/深色设置变化，也可以固定为浅色或深色。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			appearance := 0
			switch a.cfg.Theme {
			case "light":
				appearance = 1
			case "dark":
				appearance = 2
			}
			if seg := ui.Segmented(c, &appearance, "跟随系统", "浅色", "深色").
				Label("外观"); seg.Changed() {
				switch appearance {
				case 1:
					a.cfg.Theme = "light"
				case 2:
					a.cfg.Theme = "dark"
				default:
					a.cfg.Theme = "auto"
				}
				a.applyTheme()
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "文字大小").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Textf(c, "当前 %d%%；拖动滑块或点 − / + 调整，新的字号立即生效。",
					int(clampFontScale(a.cfg.FontScale)*100)).
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			scale := float64(clampFontScale(a.cfg.FontScale))
			ui.Row(c).Width(200).Shrink(0).Gap(6).AlignItems(ui.Center).Children(func() {
				if ui.Button(c, "−").Height(26).Width(28).Clicked() {
					a.nudgeFontScale(-0.05)
				}
				if slider := ui.Slider(c, &scale, 0.8, 1.5).Label("文字大小").Grow(1); slider.Changed() {
					a.setFontScale(scale)
				}
				if ui.Button(c, "+").Height(26).Width(28).Clicked() {
					a.nudgeFontScale(0.05)
				}
			})
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "详情栏常驻").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "没有选中器件时也保留右侧详情栏，列表宽度保持不变。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.DetailPinned
			if ui.Switch(c, &on).Label("详情栏常驻").Changed() {
				a.cfg.DetailPinned = on
				a.inspectorOpen = on
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
	})

	ui.Row(c).Margin(8, 0, 0, 0).Children(func() { sectionLabel(c, "状态栏", pal) })
	ui.Column(c).Background(pal.Surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "常驻状态栏").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "在 Windows 通知区域显示程序图标，并可从那里唤起窗口。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
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
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "关闭窗口时隐藏到状态栏").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "关闭窗口不退出程序，改从状态栏图标恢复；退出请用图标菜单的“退出”。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
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
		ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(4, 2, 9, 2).Children(func() {
			ui.Text(c, a.trayStatusLine()).FontSize(sf(10.5)).TextColor(pal.TextFaint).SingleLine()
			ui.Spacer(c)
			if a.trayReady {
				ui.Text(c, "图标已创建").FontSize(sf(10)).TextColor(pal.SuccessText)
			}
		})
	})

	ui.Row(c).Margin(8, 0, 0, 0).Children(func() { sectionLabel(c, "原始表格", pal) })
	ui.Column(c).Background(pal.Surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "使用密码打开原表").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "原表设置了编辑密码时，用已保存的密码通过 Excel 打开本地副本，不再弹密码框。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.OpenExcelPassword
			if ui.Switch(c, &on).Label("使用密码打开原表").Changed() {
				a.cfg.OpenExcelPassword = on
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).Gap(8).Padding(0, 2, 9, 2).Children(func() {
			ui.TextInput(c, &a.cfg.ExcelPassword).Password().
				Label("原表编辑密码").Grow(1).Placeholder("原表的编辑密码")
			if ui.Button(c, "保存").Height(28).Clicked() {
				_ = saveConfig(a.exeDir, a.cfg)
				c.Toast("已保存原表密码")
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "自动同步原表").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Textf(c, "每 %d 分钟把原表的最新版本下载到本地副本并导入；0 表示只在手动同步时进行。原表本身不会被修改。",
					a.cfg.AutoSyncMinutes).
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			minutes := float64(a.cfg.AutoSyncMinutes)
			ui.Row(c).Width(140).Shrink(0).Children(func() {
				in := ui.NumberInput(c, &minutes, 0, 1440, 15).
					Label("自动同步间隔（分钟）").Grow(1)
				if in.Changed() {
					a.cfg.AutoSyncMinutes = int(minutes)
					_ = saveConfig(a.exeDir, a.cfg)
					a.restartAutoSync()
				}
			})
		})
		ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(4, 2, 9, 2).Children(func() {
			ui.Textf(c, "本地副本目录：%s", localWorkbookDir(a.exeDir)).
				FontSize(sf(10)).TextColor(pal.TextFaint).SingleLine().Selectable()
			ui.Spacer(c)
			if ui.Button(c, "打开副本目录").Height(26).Clicked() {
				_ = shellOpenPath(localWorkbookDir(a.exeDir))
			}
		})
	})

	ui.Row(c).Margin(8, 0, 0, 0).Children(func() { sectionLabel(c, "数据手册", pal) })
	ui.Column(c).Background(pal.Surface).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "以只读方式打开 PDF").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "打开数据手册时使用只读副本，避免在手册上误编辑或保存，也不会占用网络盘上的原文件。").
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			on := a.cfg.ReadOnlyPDF
			if ui.Switch(c, &on).Label("以只读方式打开 PDF").Changed() {
				a.cfg.ReadOnlyPDF = on
				_ = saveConfig(a.exeDir, a.cfg)
			}
		})
		ui.Row(c).Gap(8).Padding(0, 2, 9, 2).Children(func() {
			ui.TextInput(c, &a.cfg.PDFReaderPath).Label("PDF 阅读器路径").Grow(1).
				Placeholder(`C:\Program Files\SumatraPDF\SumatraPDF.exe`)
			if ui.Button(c, "浏览...").Height(28).Clicked() {
				a.browseReader()
			}
		})
		ui.Row(c).AlignItems(ui.Center).Gap(10).Padding(9, 2).Children(func() {
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, "索引过期时间").FontSize(sf(12)).FontWeight(600).TextColor(pal.Text)
				ui.Textf(c, "当前 %d 分钟；索引保存在 data\\pdf-index.json，下次启动秒开。",
					a.cfg.PDFIndexMaxAgeMinutes).
					FontSize(sf(10.5)).TextColor(pal.TextMuted).Wrap()
			})
			minutes := float64(a.cfg.PDFIndexMaxAgeMinutes)
			ui.Row(c).Width(140).Shrink(0).Children(func() {
				in := ui.NumberInput(c, &minutes, 0, 1440, 5).
					Label("索引过期时间（分钟）").Grow(1)
				if in.Changed() {
					a.cfg.PDFIndexMaxAgeMinutes = int(minutes)
					_ = saveConfig(a.exeDir, a.cfg)
				}
			})
		})
		ui.Row(c).AlignItems(ui.Center).Gap(8).Padding(4, 2, 9, 2).Children(func() {
			ui.Text(c, a.fileLibraryMeta()).FontSize(sf(10.5)).TextColor(pal.TextMuted).SingleLine()
			ui.Spacer(c)
			if ui.Button(c, "重建索引").Height(26).Clicked() {
				a.scanDatasheetsAsync(true)
			}
			if ui.Button(c, "手册文件列表").Height(26).Clicked() {
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
	ui.Column(c).Width(580).Padding(20, 24).Gap(14).Radius(10).Selectable().
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 24, 64, 0, ui.RGBA(0, 0, 0, 0.35)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				title := "同步原表"
				if a.syncDryRun {
					title = "预览同步结果"
				}
				ui.Text(c, title).FontSize(sf(17)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "只读取原表，增量更新器件库；手工维护的记录不会被覆盖。").
					FontSize(sf(11.5)).TextColor(pal.TextMuted).Selectable()
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
				ui.Text(c, "正在读取原表并比对器件库").FontSize(sf(13.5)).FontWeight(600).TextColor(pal.Text)
				ui.Text(c, "识别每个工作表的表头与零件号").FontSize(sf(11)).TextColor(pal.TextMuted)
			})
		} else if a.syncErr != "" {
			ui.Column(c).Padding(12).Gap(6).Radius(6).Background(pal.Danger.Alpha(0.10)).
				Border(1, pal.Danger.Alpha(0.3)).Children(func() {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					if ic := icon("circle-alert"); ic != nil {
						ui.Icon(c, ic).FontSize(sf(16)).TextColor(pal.Danger)
					}
					ui.Text(c, "同步过程中出现错误").FontSize(sf(13)).Bold().TextColor(pal.DangerText)
				})
				ui.Text(c, a.syncErr).FontSize(sf(11.5)).TextColor(pal.Text).Wrap()
			})
		} else if a.syncResult != nil {
			res := a.syncResult
			ui.Column(c).Gap(10).Children(func() {
				ui.Column(c).Padding(10, 14).Gap(4).Radius(7).Background(pal.Surface).
					Border(1, pal.BorderSubtle).Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						if ic := icon("circle-check"); ic != nil {
							ui.Icon(c, ic).FontSize(sf(16)).TextColor(pal.Success)
						}
						ui.Text(c, res.SummaryMessage).FontSize(sf(12.5)).FontWeight(600).
							TextColor(pal.Text).Wrap()
					})
				})

				ui.Grid(c).Columns(3).GapX(10).GapY(8).Children(func() {
					a.syncStatBox(c, "扫描表格", fmt.Sprintf("%d 份", res.FilesScanned), pal)
					a.syncStatBox(c, "扫描工作表", fmt.Sprintf("%d 个", res.SheetsScanned), pal)
					a.syncStatBox(c, "新增器件", fmt.Sprintf("+%d", res.NewPartsAdded), pal)
					a.syncStatBox(c, "更新器件", fmt.Sprintf("%d", res.UpdatedParts), pal)
					a.syncStatBox(c, "无变化", fmt.Sprintf("%d", res.UnchangedParts), pal)
					a.syncStatBox(c, "手工保护", fmt.Sprintf("%d", res.ManualPreserved), pal)
				})

				if len(res.Errors) > 0 {
					ui.Column(c).Padding(8).Gap(3).Radius(6).Background(pal.Warning.Alpha(0.10)).Children(func() {
						ui.Text(c, "部分文件读取提示:").FontSize(sf(11)).Bold().TextColor(pal.WarningText)
						for _, e := range res.Errors {
							ui.Text(c, "· "+e).FontSize(sf(10.5)).TextColor(pal.TextMuted)
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
	ui.Column(c).Padding(9, 12).Gap(3).Radius(7).Background(pal.Surface).Children(func() {
		ui.Text(c, label).FontSize(sf(10.5)).TextColor(pal.TextFaint).SingleLine()
		ui.Text(c, value).FontSize(sf(14)).FontWeight(600).TextColor(pal.Text).SingleLine()
	})
}
