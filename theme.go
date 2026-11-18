package main

import "github.com/egoist/mygo/ui"

type statusTone struct {
	dot  ui.Color
	text ui.Color
	name string
}

type Palette struct {
	IsDark        bool
	AppBg         ui.Color
	SidebarBg     ui.Color
	MainBg        ui.Color
	StatusBarBg   ui.Color
	Surface       ui.Color
	SurfaceHover  ui.Color
	SurfaceActive ui.Color
	RowEven       ui.Color
	RowOdd        ui.Color
	RowHover      ui.Color
	RowSelected   ui.Color
	Border        ui.Color
	BorderSubtle  ui.Color
	BorderStrong  ui.Color
	Divider       ui.Color
	Text          ui.Color
	TextMuted     ui.Color
	TextFaint     ui.Color
	Accent        ui.Color
	AccentHover   ui.Color
	AccentPressed ui.Color
	AccentText    ui.Color
	AccentCyan    ui.Color
	AccentCyanBg  ui.Color
	Success       ui.Color
	SuccessText   ui.Color
	Warning       ui.Color
	WarningText   ui.Color
	Danger        ui.Color
	DangerText    ui.Color
}

// The light theme is a cool drawing-paper surface around a crisp white
// instrument panel; the dark one is the same instrument after hours, with
// only the identifiers and the active rail carrying color.
var darkPalette = Palette{
	IsDark:        true,
	AppBg:         ui.Hex("#0C1216"),
	SidebarBg:     ui.Hex("#111920"),
	MainBg:        ui.Hex("#0F171D"),
	StatusBarBg:   ui.Hex("#0B1115"),
	Surface:       ui.Hex("#162028"),
	SurfaceHover:  ui.Hex("#1C2831"),
	SurfaceActive: ui.Hex("#24333E"),
	RowEven:       ui.Hex("#111A21"),
	RowOdd:        ui.Hex("#121D24"),
	RowHover:      ui.Hex("#18242D"),
	RowSelected:   ui.Hex("#16324A"),
	Border:        ui.RGBA(255, 255, 255, 0.08),
	BorderSubtle:  ui.RGBA(255, 255, 255, 0.05),
	BorderStrong:  ui.RGBA(255, 255, 255, 0.16),
	Divider:       ui.RGBA(255, 255, 255, 0.07),
	Text:          ui.Hex("#E9EFF4"),
	TextMuted:     ui.Hex("#93A2AE"),
	TextFaint:     ui.Hex("#6E7E8B"),
	Accent:        ui.Hex("#4B9EF6"),
	AccentHover:   ui.Hex("#6BB2FF"),
	AccentPressed: ui.Hex("#2F82D8"),
	AccentText:    ui.Hex("#FFFFFF"),
	AccentCyan:    ui.Hex("#7CC1FF"),
	AccentCyanBg:  ui.RGBA(91, 167, 255, 0.12),
	Success:       ui.Hex("#34D399"),
	SuccessText:   ui.Hex("#6EE7B7"),
	Warning:       ui.Hex("#FBBF24"),
	WarningText:   ui.Hex("#FCD34D"),
	Danger:        ui.Hex("#F87171"),
	DangerText:    ui.Hex("#FCA5A5"),
}

var lightPalette = Palette{
	IsDark:        false,
	AppBg:         ui.Hex("#EEF2F5"),
	SidebarBg:     ui.Hex("#FFFFFF"),
	MainBg:        ui.Hex("#F6F8FA"),
	StatusBarBg:   ui.Hex("#EEF2F5"),
	Surface:       ui.Hex("#FFFFFF"),
	SurfaceHover:  ui.Hex("#F1F5F9"),
	SurfaceActive: ui.Hex("#E4EBF1"),
	RowEven:       ui.Hex("#FFFFFF"),
	RowOdd:        ui.Hex("#FAFBFC"),
	RowHover:      ui.Hex("#F0F5FA"),
	RowSelected:   ui.Hex("#E7F1FE"),
	Border:        ui.Hex("#DCE3EA"),
	BorderSubtle:  ui.Hex("#E9EEF3"),
	BorderStrong:  ui.Hex("#C6D0DA"),
	Divider:       ui.Hex("#E5EBF0"),
	Text:          ui.Hex("#0E1720"),
	TextMuted:     ui.Hex("#5A6874"),
	TextFaint:     ui.Hex("#8694A1"),
	Accent:        ui.Hex("#1467D2"),
	AccentHover:   ui.Hex("#0F56B3"),
	AccentPressed: ui.Hex("#0B448F"),
	AccentText:    ui.Hex("#FFFFFF"),
	AccentCyan:    ui.Hex("#0B63B5"),
	AccentCyanBg:  ui.Hex("#E8F2FF"),
	Success:       ui.Hex("#0E8A5F"),
	SuccessText:   ui.Hex("#0B7350"),
	Warning:       ui.Hex("#B26B00"),
	WarningText:   ui.Hex("#8A5200"),
	Danger:        ui.Hex("#B42318"),
	DangerText:    ui.Hex("#912018"),
}

// uiFontScale is the app-wide text scale, set at the start of each view
// build from the saved setting; sf applies it to the explicit sizes in the
// layouts, while setupTheme applies it to the widget theme.
var uiFontScale float32 = 1

func sf(v float32) float32 { return v * uiFontScale }

func clampFontScale(v float64) float32 {
	if v < 0.8 {
		return 0.8
	}
	if v > 1.5 {
		return 1.5
	}
	if v == 0 {
		return 1
	}
	return float32(v)
}

func (a *app) theme() *Palette {
	if a.lightTheme {
		return &lightPalette
	}
	return &darkPalette
}

func statusInfo(status string, isLight bool) statusTone {
	pal := &darkPalette
	if isLight {
		pal = &lightPalette
	}
	switch status {
	case "inactive":
		return statusTone{dot: pal.TextFaint, text: pal.TextMuted, name: "停用"}
	case "obsolete":
		return statusTone{dot: pal.Warning, text: pal.WarningText, name: "历史"}
	default:
		return statusTone{dot: pal.Success, text: pal.SuccessText, name: "有效"}
	}
}

func setupTheme(base *ui.Theme, isLight bool) *ui.Theme {
	var t ui.Theme
	pal := &darkPalette
	if isLight {
		pal = &lightPalette
		t = *ui.LightTheme()
		t.Scrollbar = ui.RGBA(15, 23, 32, 0.18)
	} else {
		t = *ui.DarkTheme()
		t.Scrollbar = ui.RGBA(255, 255, 255, 0.14)
	}
	t.Background = pal.MainBg
	t.Surface = pal.Surface
	t.SurfaceHover = pal.SurfaceHover
	t.SurfacePressed = pal.SurfaceActive
	t.Border = pal.Border
	t.Text = pal.Text
	t.TextMuted = pal.TextMuted
	t.Accent = pal.Accent
	t.AccentHover = pal.AccentHover
	t.AccentPressed = pal.AccentPressed
	t.AccentText = pal.AccentText
	t.Selection = pal.RowSelected
	t.Radius = 7
	t.Spacing = 4
	t.FontSize = sf(13)
	t.Font = "Segoe UI, Microsoft YaHei UI, PingFang SC, sans-serif"
	return &t
}
