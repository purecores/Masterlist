package main

import "github.com/egoist/mygo/ui"

type statusTone struct {
	dot  ui.Color
	text ui.Color
	name string
}

type Palette struct {
	IsDark            bool
	AppBg             ui.Color
	SidebarBg         ui.Color
	MainBg            ui.Color
	StatusBarBg       ui.Color
	Surface           ui.Color
	SurfaceHover      ui.Color
	SurfaceActive     ui.Color
	RowEven           ui.Color
	RowOdd            ui.Color
	RowHover          ui.Color
	RowSelected       ui.Color
	Border            ui.Color
	BorderSubtle      ui.Color
	BorderStrong      ui.Color
	Divider           ui.Color
	Text              ui.Color
	TextMuted         ui.Color
	TextFaint         ui.Color
	Accent            ui.Color
	AccentHover       ui.Color
	AccentPressed     ui.Color
	AccentText        ui.Color
	AccentCyan        ui.Color
	AccentCyanBg      ui.Color
	Success           ui.Color
	SuccessText       ui.Color
	Warning           ui.Color
	WarningText       ui.Color
	Danger            ui.Color
	DangerText        ui.Color
}

var darkPalette = Palette{
	IsDark:            true,
	AppBg:             ui.Hex("#0B0F17"),
	SidebarBg:         ui.Hex("#0C101A"),
	MainBg:            ui.Hex("#0F172A"),
	StatusBarBg:       ui.Hex("#080C14"),
	Surface:           ui.Hex("#151F32"),
	SurfaceHover:      ui.Hex("#1E2D47"),
	SurfaceActive:     ui.Hex("#263959"),
	RowEven:           ui.Hex("#0F172A"),
	RowOdd:            ui.Hex("#111A2E"),
	RowHover:          ui.Hex("#1A2942"),
	RowSelected:       ui.Hex("#142E54"),
	Border:            ui.RGBA(255, 255, 255, 0.08),
	BorderSubtle:      ui.RGBA(255, 255, 255, 0.05),
	BorderStrong:      ui.RGBA(255, 255, 255, 0.16),
	Divider:           ui.RGBA(255, 255, 255, 0.07),
	Text:              ui.Hex("#F8FAFC"),
	TextMuted:         ui.Hex("#94A3B8"),
	TextFaint:         ui.Hex("#64748B"),
	Accent:            ui.Hex("#0284C7"),
	AccentHover:       ui.Hex("#0369A1"),
	AccentPressed:     ui.Hex("#075985"),
	AccentText:        ui.Hex("#FFFFFF"),
	AccentCyan:        ui.Hex("#38BDF8"),
	AccentCyanBg:      ui.RGBA(56, 189, 248, 0.12),
	Success:           ui.Hex("#10B981"),
	SuccessText:       ui.Hex("#34D399"),
	Warning:           ui.Hex("#F59E0B"),
	WarningText:       ui.Hex("#FBBF24"),
	Danger:            ui.Hex("#EF4444"),
	DangerText:        ui.Hex("#F87171"),
}

var lightPalette = Palette{
	IsDark:            false,
	AppBg:             ui.Hex("#E2E8F0"), // Slate-200 base for frame
	SidebarBg:         ui.Hex("#F8FAFC"), // Clean crisp off-white sidebar
	MainBg:            ui.Hex("#FFFFFF"), // Pure white engineering paper work area
	StatusBarBg:       ui.Hex("#F1F5F9"), // Dock bar
	Surface:           ui.Hex("#F1F5F9"), // Light surface
	SurfaceHover:      ui.Hex("#E2E8F0"), // Hover
	SurfaceActive:     ui.Hex("#CBD5E1"), // Active
	RowEven:           ui.Hex("#FFFFFF"), // Even row pure white
	RowOdd:            ui.Hex("#F8FAFC"), // Odd row slate-50
	RowHover:          ui.Hex("#F1F5F9"), // Row hover
	RowSelected:       ui.Hex("#E0F2FE"), // Sky-100 selected
	Border:            ui.RGBA(0, 0, 0, 0.08),
	BorderSubtle:      ui.RGBA(0, 0, 0, 0.04),
	BorderStrong:      ui.RGBA(0, 0, 0, 0.15),
	Divider:           ui.RGBA(0, 0, 0, 0.06),
	Text:              ui.Hex("#0F172A"), // High contrast deep slate
	TextMuted:         ui.Hex("#475569"), // Slate-600
	TextFaint:         ui.Hex("#64748B"), // Slate-500
	Accent:            ui.Hex("#0284C7"), // Sky-600
	AccentHover:       ui.Hex("#0369A1"), // Sky-700
	AccentPressed:     ui.Hex("#075985"), // Sky-800
	AccentText:        ui.Hex("#FFFFFF"),
	AccentCyan:        ui.Hex("#0284C7"), // High contrast technical sky blue for BHS in light mode
	AccentCyanBg:      ui.RGBA(2, 132, 199, 0.10),
	Success:           ui.Hex("#059669"), // Emerald-600
	SuccessText:       ui.Hex("#047857"),
	Warning:           ui.Hex("#D97706"), // Amber-600
	WarningText:       ui.Hex("#92400E"),
	Danger:            ui.Hex("#DC2626"), // Red-600
	DangerText:        ui.Hex("#B91C1C"),
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
		return statusTone{
			dot:  pal.TextFaint,
			text: pal.TextMuted,
			name: "停用",
		}
	case "obsolete":
		return statusTone{
			dot:  pal.Warning,
			text: pal.WarningText,
			name: "历史",
		}
	default:
		return statusTone{
			dot:  pal.Success,
			text: pal.SuccessText,
			name: "有效",
		}
	}
}

func setupTheme(base *ui.Theme, isLight bool) *ui.Theme {
	var t ui.Theme
	if isLight {
		t = *ui.LightTheme()
		pal := &lightPalette
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
		t.Scrollbar = ui.RGBA(0, 0, 0, 0.20)
	} else {
		t = *ui.DarkTheme()
		pal := &darkPalette
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
		t.Scrollbar = ui.RGBA(255, 255, 255, 0.14)
	}
	t.Radius = 6
	t.Spacing = 4
	t.FontSize = 13
	t.Font = "Segoe UI, Microsoft YaHei UI, PingFang SC, sans-serif"
	return &t
}
