package main

import (
	"fmt"
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
		a.sheetScroll.X = 0
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

		a.statusMenu(c, pal)
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
		ui.Row(c).Height(32).Padding(0, 16).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
			Background(pal.AppBg).Children(func() {
			ui.Text(c, "提示：点击左侧任一原表分类，可快速切入对应的工作表浏览元器件。").
				FontSize(11.5).TextColor(pal.TextFaint)
		})
		return
	}
	ui.Row(c).Height(34).Padding(0, 16).AlignItems(ui.Center).Gap(8).
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		Background(pal.AppBg).Children(func() {
		ui.Text(c, "工作表:").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Shrink(0)
		ui.ScrollHorizontal(c).TrackScroll(&a.sheetScroll).Grow(1).Children(func() {
			ui.Row(c).Gap(4).AlignItems(ui.Center).Children(func() {
				a.sheetTab(c, -1, "全部工作表", book.Count, pal)
				for i, sheet := range book.Sheets {
					a.sheetTab(c, i, sheet.Name, sheet.Count, pal)
				}
			})
		})
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

func (a *app) statusMenu(c *ui.Context, pal *Palette) {
	label := statusOptions[a.statusSel]
	b := ui.ButtonBase(c.Key("status-menu")).Height(30).Padding(0, 9).Gap(5).
		Radius(6).Border(1, pal.Border).Background(pal.Surface).
		AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11.5).FontWeight(500).TextColor(pal.Text)
		if ic := icon("chevron-down"); ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(pal.TextMuted)
		}
	})
	b.Menu(func(m *ui.Menu) {
		for i, option := range statusOptions {
			if m.Item(option).Checked(a.statusSel == i).Chosen() {
				a.statusSel = i
				a.refreshResults(0)
			}
		}
	})
}

func (a *app) sortMenu(c *ui.Context, pal *Palette) {
	b := ui.ButtonBase(c.Key("sort-menu")).Height(30).Padding(0, 9).Gap(5).
		Radius(6).Border(1, pal.Border).Background(pal.Surface).
		AlignItems(ui.Center)
	if b.Hovered() {
		b.Background(pal.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, a.sortLabel).FontSize(11.5).FontWeight(500).TextColor(pal.Text)
		if ic := icon("chevron-down"); ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(pal.TextMuted)
		}
	})
	b.Menu(func(m *ui.Menu) {
		for _, option := range sortOptions {
			if m.Item(option).Checked(a.sortLabel == option).Chosen() {
				a.sortLabel = option
				a.refreshResults(0)
			}
		}
	})
}

func (a *app) tableHeader(c *ui.Context, pal *Palette) ui.Element {
	return ui.Row(c.Key("table-header")).Height(30).Padding(0, 12).Gap(8).AlignItems(ui.Center).Clip().
		BorderWidth(0, 0, 1, 0).BorderColor(pal.Border).
		Background(pal.AppBg).Children(func() {
		ui.Text(c, "BHS 编号").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(105).SingleLine()
		ui.Text(c, "元件名称").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(130).SingleLine()
		ui.Text(c, "型号 / 规格").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(145).SingleLine()
		ui.Text(c, "品牌/供应商").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(95).SingleLine()
		ui.Text(c, "原表 / 工作表").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(125).SingleLine()
		ui.Text(c, "状态").FontSize(11).FontWeight(600).
			TextColor(pal.TextFaint).Width(56).TextAlign(ui.Center).SingleLine()
	})
}

func (a *app) renderResultList(c *ui.Context, pal *Palette) ui.Element {
	a.resultList.Selected = &a.resultSel
	a.resultList.Key = func(i int) any { return a.results[i].ID }
	a.resultList.Label = func(i int) string {
		p := a.results[i]
		return p.BHSNo + " " + p.Name()
	}
	list := ui.List(c.Key("table-list"), &a.resultList, len(a.results)).Grow(1).RowWidth(720).
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
			TextColor(pal.AccentCyan).Width(105).SingleLine()

		ui.Text(c, p.Name()).FontSize(12.5).FontWeight(500).
			TextColor(textColor).Width(130).SingleLine()

		ui.Text(c, valueOr(p.Spec, "-")).FontSize(11.5).
			TextColor(pal.TextMuted).Width(145).SingleLine()

		ui.Text(c, valueOr(p.Manufacturer, "-")).FontSize(11.5).
			TextColor(pal.TextMuted).Width(95).SingleLine()

		ui.Text(c, p.SourceLabel()).FontSize(11).
			TextColor(pal.TextFaint).Width(125).SingleLine()

		ui.Row(c).Width(56).Justify(ui.Center).Children(func() {
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
				ui.Select(c, &a.formStatus, statusOptions[1:])
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

	ui.Column(c).Gap(6).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
			ui.Text(c, "数据手册 (DATASHEET)").FontSize(10).FontWeight(700).
				LetterSpacing(0.8).TextColor(pal.TextFaint)
			if len(matchedPDFs) > 0 {
				ui.Textf(c, "%d 份", len(matchedPDFs)).FontSize(9.5).Font("monospace").
					FontWeight(600).TextColor(pal.AccentCyan).
					Padding(1, 5).Radius(3).Background(pal.AccentCyanBg)
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
				ui.Row(c).Margin(2, 0, 0, 0).Children(func() {
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
				})
			})
			return
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
							ui.Textf(c, "大小: %s", formatFileSize(pdfFile.Size)).
								FontSize(10).Font("monospace").TextColor(pal.TextFaint)
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
							mygo.Shell.ShowItemInFolder(pdfFile.FullPath)
						}

						openBtn := ui.ButtonBase(c.Key(fmt.Sprintf("open:%s:%d", p.BHSNo, i))).
							Height(24).Padding(0, 8).Radius(4).
							Background(pal.Accent).TextColor(pal.AccentText).AlignItems(ui.Center)
						if openBtn.Hovered() {
							openBtn.Background(pal.AccentHover)
						}
						openBtn.Children(func() {
							ui.Text(c, "打开手册").FontSize(11).FontWeight(600).TextColor(pal.AccentText)
						})
						if openBtn.Clicked() {
							if err := mygo.Shell.OpenPath(pdfFile.FullPath); err != nil {
								c.Toast("打开 PDF 失败: " + err.Error())
							}
						}
					})
				})
			}
		})
	})
}

func (a *app) settingsModal(c *ui.Context, pal *Palette) {
	ui.Column(c).Width(600).Padding(16, 20).Gap(12).Radius(8).
		Background(pal.MainBg).Border(1, pal.BorderStrong).
		Shadow(0, 16, 48, 0, ui.RGBA(0, 0, 0, 0.4)).Children(func() {
		ui.Row(c).AlignItems(ui.Center).Children(func() {
			ui.Column(c).Gap(2).Grow(1).Children(func() {
				ui.Text(c, "目录路径与关联设置").FontSize(16).Bold().TextColor(pal.Text)
				ui.Text(c, "自定义 PDF 数据手册与 Excel 原始表格的存放目录（支持相对/绝对路径）。").
					FontSize(11.5).TextColor(pal.TextMuted)
			})
			closeBtn := iconButton(c, "x", "关闭", pal)
			if closeBtn.Clicked() {
				a.datasheetModalOpen = false
			}
		})
		ui.Divider(c).Background(pal.Border)

		// Tabs for PDF vs Excel Masterlist
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			tab0 := ui.ButtonBase(c.Key("tab:pdf")).Height(28).Padding(0, 12).Radius(5).AlignItems(ui.Center)
			if a.settingsTab == 0 {
				tab0.Background(pal.Accent).TextColor(pal.AccentText)
			} else {
				tab0.Background(pal.Surface).Border(1, pal.Border).TextColor(pal.TextMuted)
			}
			tab0.Children(func() {
				tc := pal.TextMuted
				if a.settingsTab == 0 {
					tc = pal.AccentText
				}
				ui.Text(c, "数据手册 PDF 目录").FontSize(11.5).FontWeight(600).TextColor(tc)
			})
			if tab0.Clicked() {
				a.settingsTab = 0
			}

			tab1 := ui.ButtonBase(c.Key("tab:masterlist")).Height(28).Padding(0, 12).Radius(5).AlignItems(ui.Center)
			if a.settingsTab == 1 {
				tab1.Background(pal.Accent).TextColor(pal.AccentText)
			} else {
				tab1.Background(pal.Surface).Border(1, pal.Border).TextColor(pal.TextMuted)
			}
			tab1.Children(func() {
				tc := pal.TextMuted
				if a.settingsTab == 1 {
					tc = pal.AccentText
				}
				ui.Text(c, "Excel 原表 Masterlist 目录").FontSize(11.5).FontWeight(600).TextColor(tc)
			})
			if tab1.Clicked() {
				a.settingsTab = 1
			}
		})

		if a.settingsTab == 0 {
			ui.Field(c, "PDF 手册目录（每行一条，优先匹配上层）", func() {
				ui.TextArea(c, &a.datasheetPathsText).Lines(4, 5)
			})

			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "当前扫描状态与路径解析:").FontSize(10.5).FontWeight(700).
					LetterSpacing(0.8).TextColor(pal.TextFaint)
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
									TextColor(pal.Text)
								statusDesc := fmt.Sprintf("-> %s (已检索到 %d 份 PDF)", stCopy.ResolvedPath, stCopy.Count)
								if !stCopy.Exists {
									statusDesc = fmt.Sprintf("-> %s (目录不存在)", stCopy.ResolvedPath)
								}
								ui.Text(c, statusDesc).FontSize(10.5).TextColor(pal.TextMuted).SingleLine()
							})
						})
					}
				})
			})

			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Textf(c, "总计已检索并索引 %s 份 PDF 数据手册", formatInt(a.pdfScanner.Count())).
					FontSize(11.5).Bold().TextColor(pal.AccentCyan)
			})
		} else {
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
									TextColor(pal.Text)
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

		if a.datasheetStatusMsg != "" {
			ui.Text(c, a.datasheetStatusMsg).FontSize(11).TextColor(pal.DangerText).
				Padding(5, 8).Radius(4).Background(pal.Danger.Alpha(0.12))
		}

		ui.Row(c).Justify(ui.End).Gap(8).Margin(6, 0, 0, 0).Children(func() {
			if a.settingsTab == 0 {
				if ui.Button(c, "重新扫描").Height(30).Clicked() {
					a.refreshDatasheetScan(c)
				}
			}
			if ui.Button(c, "取消").Height(30).Clicked() {
				a.datasheetModalOpen = false
			}
			if ui.PrimaryButton(c, "保存并应用").Height(30).Clicked() {
				a.saveDatasheetSettings(c)
			}
		})
	})
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
