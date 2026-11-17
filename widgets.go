package main

import (
	"github.com/egoist/mygo/ui"
)

func iconButton(c *ui.Context, name, label string, pal *Palette) ui.Element {
	b := ui.ButtonBase(c.Key("icon-btn:" + name + ":" + label)).
		Size(28, 28).Radius(6).Center().Label(label).Tooltip(label).
		TextColor(pal.TextMuted)
	if b.Hovered() {
		b.Background(pal.SurfaceHover).TextColor(pal.Text)
	}
	if ic := icon(name); ic != nil {
		b.Children(func() { ui.Icon(c, ic).FontSize(14) })
	}
	return b
}

func statusIndicator(c *ui.Context, status string, isLight bool) ui.Element {
	info := statusInfo(status, isLight)
	return ui.Row(c).Height(20).Gap(5).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(info.dot)
		ui.Text(c, info.name).FontSize(11).FontWeight(500).TextColor(info.text).SingleLine()
	})
}

func emptyStateView(c *ui.Context, iconName, title, detail string, pal *Palette) {
	ui.Column(c).Grow(1).Center().Gap(8).Padding(40).Children(func() {
		if iconName != "" {
			if ic := icon(iconName); ic != nil {
				ui.Icon(c, ic).FontSize(32).TextColor(pal.TextFaint).Margin(0, 0, 4, 0)
			}
		}
		ui.Text(c, title).FontSize(16).FontWeight(600).TextColor(pal.Text).TextAlign(ui.Center)
		ui.Text(c, detail).FontSize(12.5).TextColor(pal.TextMuted).
			TextAlign(ui.Center).MaxWidth(420).LineHeight(1.4)
	})
}
