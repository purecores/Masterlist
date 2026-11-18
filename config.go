package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// SortMode is how the part list is ordered.
type SortMode string

const (
	SortSourceOrder  SortMode = "source"
	SortBHS          SortMode = "bhs"
	SortName         SortMode = "name"
	SortManufacturer SortMode = "manufacturer"
	SortSpec         SortMode = "spec"
	SortUpdated      SortMode = "updated"
)

type Config struct {
	DatasheetPaths        []string `json:"datasheet_paths"`
	MasterlistPaths       []string `json:"masterlist_paths"`
	Theme                 string   `json:"theme"` // "auto", "dark" or "light"
	TrayEnabled           bool     `json:"tray_enabled"`
	CloseToTray           bool     `json:"close_to_tray"`
	ReadOnlyPDF           bool     `json:"read_only_pdf"`
	PDFReaderPath         string   `json:"pdf_reader_path"`
	SortMode              SortMode `json:"sort_mode"`
	SortDesc              bool     `json:"sort_desc"`
	CategoryFilter        string   `json:"category_filter"`
	ManufacturerFilter    string   `json:"manufacturer_filter"`
	DatasheetOnly         bool     `json:"datasheet_only"`
	PDFIndexMaxAgeMinutes int      `json:"pdf_index_max_age_minutes"`
	// DetailPinned keeps the detail pane visible even without a selection.
	DetailPinned bool `json:"detail_pinned"`
	// OpenExcelPassword asks Excel for the original workbook's edit password
	// when it opens the local copy.
	OpenExcelPassword bool   `json:"open_excel_password"`
	ExcelPassword     string `json:"excel_password"`
	// AutoSyncMinutes refreshes the local copy of the original workbooks and
	// re-imports changes on this interval; 0 turns it off.
	AutoSyncMinutes int `json:"auto_sync_minutes"`
	// FontScale multiplies the UI text size: 1 is the default size.
	FontScale float64 `json:"font_scale"`
	// ExcludedWorkbooks lists workbook files, or folders, the sync leaves
	// out. Each line is a file name or a full path; a folder line excludes
	// every workbook inside it.
	ExcludedWorkbooks []string `json:"excluded_workbooks"`
}

// DefaultPaths are the folders the app ships with: the production master list
// and datasheet libraries, used until the settings name others.
const (
	defaultMasterlistPath = `W:\TD\1_部门资料 Section\TE\2.1_E Parts Database\2.1.2_Master list-总表`
	defaultDatasheetA     = `D:\Knowledge\MM01\Datasheet`
	defaultDatasheetB     = `W:\TD\1_部门资料 Section\TE\2.1_E Parts Database\2.1.1_Data sheet`
)

func defaultConfig() Config {
	return Config{
		DatasheetPaths: []string{
			defaultDatasheetA,
			defaultDatasheetB,
		},
		MasterlistPaths: []string{
			defaultMasterlistPath,
		},
		Theme:                 "auto",
		TrayEnabled:           true,
		CloseToTray:           true,
		ReadOnlyPDF:           false,
		PDFReaderPath:         "",
		SortMode:              SortSourceOrder,
		SortDesc:              false,
		CategoryFilter:        "",
		ManufacturerFilter:    "",
		DatasheetOnly:         false,
		PDFIndexMaxAgeMinutes: 0,
		DetailPinned:          true,
		OpenExcelPassword:     false,
		ExcelPassword:         "",
		AutoSyncMinutes:       60,
		FontScale:             1.0,
		ExcludedWorkbooks:     nil,
	}
}

func configPath(exeDir string) string {
	return filepath.Join(exeDir, "config.json")
}

func loadConfig(exeDir string) Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath(exeDir))
	if err != nil {
		return cfg
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err == nil {
		if len(loaded.DatasheetPaths) > 0 {
			cfg.DatasheetPaths = cleanPathList(loaded.DatasheetPaths)
		}
		if len(loaded.MasterlistPaths) > 0 {
			cfg.MasterlistPaths = cleanPathList(loaded.MasterlistPaths)
		}
		if loaded.Theme == "light" || loaded.Theme == "dark" || loaded.Theme == "auto" {
			cfg.Theme = loaded.Theme
		}
		if validSortMode(loaded.SortMode) {
			cfg.SortMode = loaded.SortMode
		}
		cfg.SortDesc = loaded.SortDesc
		cfg.CategoryFilter = strings.TrimSpace(loaded.CategoryFilter)
		cfg.ManufacturerFilter = strings.TrimSpace(loaded.ManufacturerFilter)
		cfg.DatasheetOnly = loaded.DatasheetOnly
		// Bools default to true when absent from an older config: JSON
		// cannot tell "false" from "not written", so only a file that
		// mentions the key turns them off.
		var raw map[string]json.RawMessage
		if err := json.Unmarshal(data, &raw); err == nil {
			if v, ok := raw["tray_enabled"]; ok {
				_ = json.Unmarshal(v, &cfg.TrayEnabled)
			}
			if v, ok := raw["close_to_tray"]; ok {
				_ = json.Unmarshal(v, &cfg.CloseToTray)
			}
			if v, ok := raw["read_only_pdf"]; ok {
				_ = json.Unmarshal(v, &cfg.ReadOnlyPDF)
			}
			if v, ok := raw["datasheet_only"]; ok {
				_ = json.Unmarshal(v, &cfg.DatasheetOnly)
			}
			if v, ok := raw["detail_pinned"]; ok {
				_ = json.Unmarshal(v, &cfg.DetailPinned)
			}
			if v, ok := raw["open_excel_password"]; ok {
				_ = json.Unmarshal(v, &cfg.OpenExcelPassword)
			}
		}
		cfg.PDFReaderPath = strings.TrimSpace(loaded.PDFReaderPath)
		cfg.ExcelPassword = strings.TrimSpace(loaded.ExcelPassword)
		if loaded.AutoSyncMinutes >= 0 {
			cfg.AutoSyncMinutes = loaded.AutoSyncMinutes
		}
		if loaded.FontScale >= 0.8 && loaded.FontScale <= 1.5 {
			cfg.FontScale = loaded.FontScale
		}
		if len(loaded.ExcludedWorkbooks) > 0 {
			cfg.ExcludedWorkbooks = cleanPathList(loaded.ExcludedWorkbooks)
		}
		if cfg.AutoSyncMinutes < 0 {
			cfg.AutoSyncMinutes = 0
		}
		if cfg.PDFIndexMaxAgeMinutes < 0 {
			cfg.PDFIndexMaxAgeMinutes = 0
		}
		// Keys another version wrote are not lost when this one saves.
	}
	return cfg
}

func cleanPathList(list []string) []string {
	var cleaned []string
	for _, p := range list {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return cleaned
}

func validSortMode(m SortMode) bool {
	switch m {
	case SortSourceOrder, SortBHS, SortName, SortManufacturer, SortSpec, SortUpdated:
		return true
	}
	return false
}

func saveConfig(exeDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(exeDir), data, 0o666)
}

// resolveConfiguredDir resolves a configured directory: whitespace and quotes
// are trimmed, %VAR% and $VAR are expanded, a relative path is relative to the
// program directory, and UNC paths and mapped drives are kept as they are.
func resolveConfiguredDir(exeDir, configured string) string {
	p := strings.TrimSpace(configured)
	p = expandWindowsEnv(p)
	p = os.ExpandEnv(p)
	p = strings.Trim(p, `"`)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, `\\`) || strings.HasPrefix(p, `//`) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(exeDir, p))
}

func resolveExcelPath(exeDir string, paths []string, filename string) string {
	if filename == "" {
		return ""
	}
	for _, p := range paths {
		targetDir := p
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(exeDir, targetDir)
		}
		candidate := filepath.Join(targetDir, filename)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

type DirCountStatus struct {
	ConfiguredPath string
	ResolvedPath   string
	Exists         bool
	FileCount      int
}

func checkExcelDirStatuses(baseDir string, paths []string) []DirCountStatus {
	var list []DirCountStatus
	for _, p := range paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		resolved := trimmed
		if !filepath.IsAbs(resolved) && !strings.HasPrefix(resolved, `\\`) {
			resolved = filepath.Join(baseDir, resolved)
		}
		abs, err := filepath.Abs(resolved)
		if err == nil {
			resolved = abs
		}
		st := DirCountStatus{
			ConfiguredPath: trimmed,
			ResolvedPath:   resolved,
		}
		fi, err := os.Stat(resolved)
		if err == nil && fi.IsDir() {
			st.Exists = true
			entries, _ := os.ReadDir(resolved)
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
					st.FileCount++
				}
			}
		}
		list = append(list, st)
	}
	return list
}

// readerCandidates lists viewers a read-only open can use, in order: the
// configured one, then the ones that reliably open a PDF without modifying it.
func readerCandidates(configured string) []string {
	var out []string
	if strings.TrimSpace(configured) != "" {
		out = append(out, strings.TrimSpace(configured))
	}
	programFiles := os.Getenv("ProgramFiles")
	programFilesX86 := os.Getenv("ProgramFiles(x86)")

	// SumatraPDF: opens fast, remembers its state, never locks the file for
	// writing beyond what a viewer needs.
	for _, dir := range []string{programFiles, programFilesX86, os.Getenv("LOCALAPPDATA")} {
		if dir == "" {
			continue
		}
		out = append(out, filepath.Join(dir, "SumatraPDF", "SumatraPDF.exe"))
		out = append(out, filepath.Join(dir, "Programs", "SumatraPDF", "SumatraPDF.exe"))
	}
	// Adobe Acrobat and Reader, which open a PDF read-only with a /A
	// parameter; the app only passes it the file to open.
	commonAdobedDirs := []string{
		"Adobe\\Acrobat DC\\Acrobat\\Acrobat.exe",
		"Adobe\\Acrobat Reader DC\\Reader\\AcroRd32.exe",
		"Adobe\\Acrobat Reader DC\\Reader\\Acrobat.exe",
		"Adobe\\Acrobat Reader\\Reader\\AcroRd32.exe",
		"Adobe\\Acrobat 2017\\Acrobat\\Acrobat.exe",
		"Adobe\\Acrobat 2017\\Acrobat\\AcroRd32.exe",
	}
	for _, dir := range []string{programFiles, programFilesX86} {
		if dir == "" {
			continue
		}
		for _, rel := range commonAdobedDirs {
			out = append(out, filepath.Join(dir, rel))
		}
	}
	// Microsoft Edge opens PDFs in a browser tab and can be told the file
	// is read-only by a copy in a temp folder.
	for _, dir := range []string{programFiles, programFilesX86, os.Getenv("LOCALAPPDATA")} {
		if dir == "" {
			continue
		}
		out = append(out, filepath.Join(dir, "Microsoft", "Edge", "Application", "msedge.exe"))
	}
	return out
}

// cleanPDFReaderArgument reports whether the configured reader path is a
// usable executable.
func cleanPDFReaderArgument(p string) string {
	p = strings.Trim(strings.TrimSpace(os.ExpandEnv(p)), `"`)
	if p == "" {
		return ""
	}
	if fi, err := os.Stat(p); err != nil || fi.IsDir() {
		return ""
	}
	return p
}

func sortModeOptions() []string {
	return []string{"原表顺序", "零件号", "元件名称", "品牌/供应商", "型号 / 规格", "最近修改"}
}

func sortModeLabel(m SortMode) string {
	switch m {
	case SortBHS:
		return "零件号"
	case SortName:
		return "元件名称"
	case SortManufacturer:
		return "品牌/供应商"
	case SortSpec:
		return "型号 / 规格"
	case SortUpdated:
		return "最近修改"
	default:
		return "原表顺序"
	}
}

func sortModeFromLabel(label string) SortMode {
	switch label {
	case "零件号":
		return SortBHS
	case "元件名称":
		return SortName
	case "品牌/供应商":
		return SortManufacturer
	case "型号 / 规格":
		return SortSpec
	case "最近修改":
		return SortUpdated
	default:
		return SortSourceOrder
	}
}
