package main

import (
	"github.com/egoist/mygo/ui"
)

// iconButton is a quiet square icon control that only shows its surface on
// hover, so a toolbar stays calm until the pointer arrives.
func iconButton(c *ui.Context, name, label string, pal *Palette) ui.Element {
	b := ui.ButtonBase(c.Key("icon-btn:"+name+":"+label)).
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

// quietButton is a text-and-icon action without a permanent border: the
// secondary weight most toolbar and footer actions should have.
func quietButton(c *ui.Context, key, label, iconName string, pal *Palette) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(30).Padding(0, 8).Gap(6).
		Radius(6).AlignItems(ui.Center).Label(label).Tooltip(label).
		TextColor(pal.TextMuted)
	if b.Hovered() {
		b.Background(pal.SurfaceHover).TextColor(pal.Text)
	}
	b.Children(func() {
		if ic := icon(iconName); ic != nil {
			ui.Icon(c, ic).FontSize(13).TextColor(pal.TextMuted)
		}
		ui.Text(c, label).FontSize(11.5).FontWeight(500).TextColor(pal.TextMuted).SingleLine()
	})
	return b
}

// accentLink is a compact text action that reads as a link rather than a
// button, for "open", "locate" and similar row-level commands.
func accentLink(c *ui.Context, key, label string, pal *Palette) ui.Element {
	b := ui.ButtonBase(c.Key(key)).Height(22).Padding(0, 3).Radius(4).
		AlignItems(ui.Center).Label(label).Tooltip(label)
	color := pal.Accent
	if b.Hovered() {
		color = pal.AccentHover
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(color).SingleLine()
	})
	return b
}

func statusIndicator(c *ui.Context, status string, isLight bool) ui.Element {
	info := statusInfo(status, isLight)
	return ui.Row(c).Height(20).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Box(c).Size(6, 6).Radius(3).Background(info.dot)
		ui.Text(c, info.name).FontSize(10.5).FontWeight(500).TextColor(info.text).SingleLine()
	})
}

// sectionLabel is the quiet caption that introduces a group: sentence case,
// normal tracking, just weight and a faint tone.
func sectionLabel(c *ui.Context, text string, pal *Palette) {
	ui.Text(c, text).FontSize(11.5).FontWeight(600).TextColor(pal.TextMuted).SingleLine()
}

// metaText is the small technical line under a name: size, date, folder.
func metaText(c *ui.Context, text string, pal *Palette) {
	ui.Text(c, text).FontSize(10).TextColor(pal.TextFaint).SingleLine()
}

func emptyStateView(c *ui.Context, iconName, title, detail string, pal *Palette) {
	ui.Column(c).Grow(1).Center().Gap(8).Padding(40).Children(func() {
		if iconName != "" {
			if ic := icon(iconName); ic != nil {
				ui.Icon(c, ic).FontSize(26).TextColor(pal.TextFaint).Margin(0, 0, 2, 0)
			}
		}
		ui.Text(c, title).FontSize(14.5).FontWeight(600).TextColor(pal.Text).TextAlign(ui.Center)
		ui.Text(c, detail).FontSize(11.5).TextColor(pal.TextMuted).
			TextAlign(ui.Center).MaxWidth(420).LineHeight(1.5)
	})
}
