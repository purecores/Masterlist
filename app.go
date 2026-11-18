package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type app struct {
	win     *mygo.Window
	dbPath  string
	store   *Store
	loading bool
	loadErr string

	tray      *mygo.Tray
	trayReady bool
	quitting  bool
	trayError string

	summary Summary
	parts   []*Part
	books   []WorkbookInfo

	query          string
	workbookSel    int
	sheetSel       int
	sheetChoice    string
	sheetChoiceFor string
	sheetChoices   []string
	sortLabel      string
	sortDesc       bool
	results        []*Part
	resultSel      int
	inspectorOpen  bool

	// Filtering inside the part list beyond the sidebar's workbook and sheet.
	filterOpen     bool
	filterStatus   int    // index into filterStatusOptions, 0 = keep a.StatusSel
	filterCategory string // category filter, "" = all
	filterMfr      string // manufacturer contains
	filterTypes    int    // 0 all, 1 has datasheet, 2 no datasheet
	filterStats    filterStats

	workbookList ui.ListState
	resultList   ui.ListState
	searchHandle ui.Handle
	detailScroll ui.ScrollState

	formOpen   bool
	editing    bool
	draft      Part
	formStatus string
	formError  string

	exeDir              string
	cfg                 Config
	pdfScanner          *PDFScanner
	pdfScanStart        time.Time
	datasheetModalOpen  bool
	datasheetPathsText  string
	masterlistPathsText string
	excelExcludeText    string
	datasheetStatusMsg  string
	settingsTab         int // 0: PDF 手册, 1: Excel 原表, 2: 常规选项

	fileListOpen         bool
	fileListQuery        string
	fileListScroll       ui.ScrollState
	fileListList         ui.ListState
	fileListSel          int
	fileListDisplay      []PDFFile
	fileListStatus       string // last manual sync result
	fileListOnlyThisPart bool
	fileListSyncing      bool

	lightTheme    bool
	systemDark    bool
	appReady      bool
	syncModalOpen bool
	syncRunning   bool
	syncDryRun    bool
	syncResult    *SyncResult
	syncErr       string
	scanDoneAt    string
	pendingToast  string
	autoSync      autoSyncState
	// settleScanOnLaunch remembers the last index sync, so the first launch
	// of a day refreshes it and later launches open from the cache.
	settleScanOnLaunch bool
}

// filterStats reports how many parts the active filters kept and why a
// result set may be empty.
type filterStats struct {
	Total            int
	Kept             int
	WithDatasheet    int
	WithoutDatasheet int
}

var filterStatusOptions = []string{"全部状态", "有效", "停用", "历史"}
var filterTypeOptions = []string{"全部器件", "有数据手册", "无数据手册"}

var statusLabels = map[string]string{
	"active":   "有效",
	"inactive": "停用",
	"obsolete": "历史",
}

var categoryNames = map[string]string{
	"Accessories":             "附件",
	"Air Conditioner":         "空调",
	"Aluminum Plate":          "铝板",
	"Battery":                 "电池",
	"Bus bar":                 "母线",
	"Cable & Wire":            "电缆与导线",
	"Circuit Breaker":         "断路器",
	"Controller":              "控制器",
	"Copacitor":               "电容",
	"DC Power Supply Unit":    "直流电源",
	"Diode":                   "二极管",
	"Display & Light":         "显示与照明",
	"Electronic":              "电子元件",
	"Emergency Stop Module":   "急停模块",
	"Encoder":                 "编码器",
	"Fan":                     "风扇",
	"Fuse System":             "熔断系统",
	"Heavy Duty Connectors":   "重载连接器",
	"Horn":                    "蜂鸣器",
	"Inactive Parts":          "停用件",
	"Isolation":               "隔离器件",
	"Limit Switch":            "限位开关",
	"Moeller":                 "穆勒",
	"Motor Protect Switch":    "电机保护开关",
	"Motors":                  "电机",
	"Network":                 "网络",
	"ODM":                     "ODM",
	"PLC":                     "PLC",
	"Push Button":             "按钮",
	"Relay & Contactor":       "继电器与接触器",
	"Schneider":               "施耐德",
	"Sensor":                  "传感器",
	"Siemens":                 "西门子",
	"Socket":                  "插座",
	"Softwore":                "软件",
	"Switch":                  "开关",
	"Switch Cabinet":          "开关柜",
	"System Components":       "系统组件",
	"Terminal Block":          "端子排",
	"Throttle & Resistor":     "电抗与电阻",
	"Touch Panel & PC":        "触摸屏与工控机",
	"Transformer":             "变压器",
	"UPS":                     "UPS",
	"Vacuum":                  "真空发生器",
	"Vacuum Generator":        "真空发生器",
	"Valve":                   "阀门",
	"Wiring Book":             "接线手册",
	"Wire Channel & DIN Rail": "线槽与导轨",
}

func categoryName(cat string) string {
	if name, ok := categoryNames[cat]; ok {
		return name
	}
	return cat
}

func newApp(dbPath, initialQuery string, openSettings bool) *app {
	exe, err := os.Executable()
	var exeDir string
	if err == nil {
		exeDir = filepath.Dir(exe)
	} else {
		exeDir = "."
	}
	if abs, err := filepath.Abs(exeDir); err == nil {
		exeDir = abs
	}
	cfg := loadConfig(exeDir)
	scanner := NewPDFScanner(exeDir, cfg.DatasheetPaths)

	return &app{
		exeDir:              exeDir,
		cfg:                 cfg,
		pdfScanner:          scanner,
		dbPath:              dbPath,
		loading:             true,
		workbookSel:         -1,
		sheetSel:            -1,
		sortLabel:           sortModeLabel(cfg.SortMode),
		sortDesc:            cfg.SortDesc,
		query:               initialQuery,
		resultSel:           -1,
		inspectorOpen:       cfg.DetailPinned,
		datasheetModalOpen:  openSettings,
		datasheetPathsText:  strings.Join(cfg.DatasheetPaths, "\n"),
		masterlistPathsText: strings.Join(cfg.MasterlistPaths, "\n"),
		lightTheme:          false,
		filterCategory:      cfg.CategoryFilter,
		filterMfr:           cfg.ManufacturerFilter,
		filterTypes:         boolToIndex(cfg.DatasheetOnly),
	}
}

func boolToIndex(on bool) int {
	if on {
		return 1
	}
	return 0
}

func (a *app) loadData(win *mygo.Window) {
	ctx := context.Background()
	store, err := OpenStore(ctx, a.dbPath)
	if err != nil {
		win.Update(func() {
			a.loading = false
			a.loadErr = "打开数据库失败：" + err.Error()
		})
		return
	}
	parts, books, summary, err := store.LoadAll(ctx)
	if err != nil {
		_ = store.Close()
		win.Update(func() {
			a.loading = false
			a.loadErr = "读取器件记录失败：" + err.Error()
		})
		return
	}
	win.Update(func() {
		a.store = store
		a.parts = parts
		a.books = books
		a.summary = summary
		a.loading = false
		if a.resultSel >= 0 && a.resultSel < len(a.parts) {
			a.refreshResults(a.parts[a.resultSel].ID)
		} else {
			a.refreshResults(0)
		}
		a.refreshTray()
		// First launch, or a folder of copies that was cleared: fetch the
		// original tables once, so the library has a local set to work from.
		if a.cfg.AutoSyncMinutes > 0 && (len(parts) == 0 || !hasLocalWorkbooks(a.exeDir)) {
			a.runAutoSync()
		}
	})

	// A cached datasheet index opens instantly. It is refreshed on launch
	// only when it is older than the expiry setting, which keeps a network
	// share from being walked at every start.
	if !a.pdfScanner.HasIndex() || a.pdfIndexExpired() {
		a.scanDatasheetsAsync(false)
	}
	if a.fileListOpen {
		a.refreshFileListDisplay()
	}
}

// pdfIndexExpired reports whether the cached index is older than the expiry
// setting; 0 minutes restores the index on every start.
func (a *app) pdfIndexExpired() bool {
	at := a.pdfScanner.ScannedAt()
	if at.IsZero() {
		return true
	}
	minutes := a.cfg.PDFIndexMaxAgeMinutes
	if minutes <= 0 {
		return true
	}
	return time.Since(at) > time.Duration(minutes)*time.Minute
}

func (a *app) selectedBook() *WorkbookInfo {
	if a.workbookSel >= 0 && a.workbookSel < len(a.books) {
		return &a.books[a.workbookSel]
	}
	return nil
}

func (a *app) selectedSheet() string {
	book := a.selectedBook()
	if book == nil {
		return ""
	}
	if a.sheetSel >= 0 && a.sheetSel < len(book.Sheets) {
		return book.Sheets[a.sheetSel].Name
	}
	return ""
}

func (a *app) selectedPart() *Part {
	if a.resultSel >= 0 && a.resultSel < len(a.results) {
		return a.results[a.resultSel]
	}
	return nil
}

// effectiveStatusFilter is the status the list filters on: index 0 of
// filterStatusOptions keeps every status.
func (a *app) effectiveStatusFilter() int {
	return a.filterStatus
}

func matchesStatus(status string, sel int) bool {
	switch sel {
	case 1:
		return status == "active"
	case 2:
		return status == "inactive"
	case 3:
		return status == "obsolete"
	default:
		return true
	}
}

// filterActive reports whether any list filter beyond the sidebar is on.
func (a *app) filterActive() bool {
	return a.filterStatus > 0 || a.filterCategory != "" || a.filterMfr != "" || a.filterTypes > 0
}

// jumpToPart selects the part a number names, if it is in the library.
// It keeps the current folder, filters by the number, picks the first
// match, and scrolls it into view.
func (a *app) jumpToPart(number string) bool {
	number = strings.TrimSpace(number)
	if number == "" {
		return false
	}
	for _, sel := range []struct {
		match func(*Part) bool
	}{{func(p *Part) bool { return strings.EqualFold(p.BHSNo, number) }},
		{func(p *Part) bool { return strings.EqualFold(p.RelatedPart, number) }},
		{func(p *Part) bool { return strings.EqualFold(p.AlternateNo, number) }}} {
		for i, p := range a.results {
			if sel.match(p) {
				a.resultSel = i
				a.inspectorOpen = true
				a.detailScroll.Y = 0
				a.resultList.ScrollIntoView(i)
				return true
			}
		}
	}
	// Not in this view: search for the number so the part shows up even
	// under another workbook.
	for _, p := range a.parts {
		if strings.EqualFold(p.BHSNo, number) {
			a.query = number
			a.workbookSel = -1
			a.sheetSel = -1
			a.refreshResults(p.ID)
			a.inspectorOpen = true
			a.detailScroll.Y = 0
			return true
		}
	}
	return false
}

func (a *app) refreshResults(keepID int64) {
	filtered := make([]*Part, 0, len(a.parts))
	q := strings.ToLower(strings.TrimSpace(a.query))
	tokens := strings.Fields(q)
	terms := make([][]string, len(tokens))
	for i, t := range tokens {
		terms[i] = expandQueryToken(t)
	}

	selectedCategory := ""
	selectedFile := ""
	if book := a.selectedBook(); book != nil {
		selectedCategory = book.Category
		selectedFile = book.SourceFile
	}
	sheet := a.selectedSheet()
	statusFilter := a.effectiveStatusFilter()
	catFilter := strings.ToLower(a.filterCategory)
	mfrFilter := strings.ToLower(strings.TrimSpace(a.filterMfr))

	stats := filterStats{Total: len(a.parts)}
	for _, p := range a.parts {
		if selectedCategory != "" && p.Category != selectedCategory && p.SourceFile != selectedFile {
			continue
		}
		if sheet != "" {
			if p.SourceSheet != sheet && p.Subcategory != sheet {
				continue
			}
		}
		if !matchesStatus(p.Status, statusFilter) {
			continue
		}
		if catFilter != "" && strings.ToLower(p.Category) != catFilter && strings.ToLower(categoryName(p.Category)) != catFilter {
			continue
		}
		if mfrFilter != "" && !strings.Contains(strings.ToLower(p.Manufacturer), mfrFilter) {
			continue
		}
		if len(terms) > 0 {
			blob := p.SearchBlob()
			match := true
			for _, termSet := range terms {
				termMatch := false
				for _, term := range termSet {
					if strings.Contains(blob, term) {
						termMatch = true
						break
					}
				}
				if !termMatch {
					match = false
					break
				}
			}
			if !match {
				continue
			}
		}
		if a.filterTypes > 0 {
			hasPDF := a.pdfScanner.HasMatch(p.BHSNo)
			if hasPDF {
				stats.WithDatasheet++
			} else {
				stats.WithoutDatasheet++
			}
			if a.filterTypes == 1 && !hasPDF {
				continue
			}
			if a.filterTypes == 2 && hasPDF {
				continue
			}
		}
		filtered = append(filtered, p)
	}
	stats.Kept = len(filtered)
	a.filterStats = stats

	sortMode := sortModeFromLabel(a.sortLabel)
	slices.SortFunc(filtered, func(x, y *Part) int {
		c := compareParts(sortMode, x, y)
		if a.sortDesc {
			return -c
		}
		return c
	})

	a.results = filtered

	a.results = filtered

	newSel := -1
	if keepID > 0 {
		for i, p := range a.results {
			if p.ID == keepID {
				newSel = i
				break
			}
		}
	}
	if newSel == -1 && len(a.results) > 0 {
		newSel = 0
	}
	a.resultSel = newSel
}

// compareParts orders two parts by a sort mode, always ascending; the caller
// reverses it for a descending list.
// splitPartNumbers breaks a replacement cell into numbers. The sheets use
// spaces, commas, slashes and line breaks between several replacements.
func splitPartNumbers(value string) []string {
	fields := strings.FieldsFunc(value, func(r rune) bool {
		switch r {
		case ' ', ',', ';', '/', '\n', '\r', '\t', '|':
			return true
		case 0x3001, 0xFF1B, 0xFF0C: // 、 ； ，
			return true
		}
		return false
	})
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		f = strings.Trim(f, "()[]{}.")
		if f != "" {
			out = append(out, f)
		}
	}
	return out
}

// looksLikePartNumber reports whether a token names a part rather than a
// note: a number, letter or dash group of at least six characters.
func looksLikePartNumber(token string) bool {
	digits := 0
	for _, r := range token {
		if r >= '0' && r <= '9' {
			digits++
		}
	}
	word := false
	for _, r := range token {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r == '-', r == '.':
			word = true
		default:
			word = false
		}
		if !word {
			break
		}
	}
	return len(token) >= 6 && digits >= 4
}

func compareParts(mode SortMode, x, y *Part) int {
	switch mode {
	case SortBHS:
		if c := strings.Compare(strings.ToLower(x.BHSNo), strings.ToLower(y.BHSNo)); c != 0 {
			return c
		}
	case SortName:
		if c := strings.Compare(strings.ToLower(x.Name()), strings.ToLower(y.Name())); c != 0 {
			return c
		}
	case SortManufacturer:
		if c := strings.Compare(strings.ToLower(x.Manufacturer), strings.ToLower(y.Manufacturer)); c != 0 {
			return c
		}
	case SortSpec:
		if c := strings.Compare(strings.ToLower(x.Spec), strings.ToLower(y.Spec)); c != 0 {
			return c
		}
	case SortUpdated:
		if c := strings.Compare(y.UpdatedAt, x.UpdatedAt); c != 0 {
			return c
		}
	}
	// A tie-break that keeps the list stable and familiar: source order.
	if x.SourceFile != y.SourceFile {
		return strings.Compare(x.SourceFile, y.SourceFile)
	}
	if x.SourceSheet != y.SourceSheet {
		return strings.Compare(x.SourceSheet, y.SourceSheet)
	}
	if x.SourceRow != y.SourceRow {
		return int(x.SourceRow - y.SourceRow)
	}
	return strings.Compare(x.BHSNo, y.BHSNo)
}

func expandQueryToken(token string) []string {
	variants := []string{token}
	synonyms := map[string][]string{
		"断路器":      {"breaker", "空开", "circuit breaker"},
		"空开":       {"断路器", "breaker", "circuit breaker"},
		"breaker":  {"断路器", "空开"},
		"端子":       {"terminal", "wago", "万可"},
		"terminal": {"端子", "wago"},
		"继电器":      {"relay", "contactor", "接触器"},
		"relay":    {"继电器", "接触器"},
		"接触器":      {"contactor", "relay", "继电器"},
		"电源":       {"power supply", "dc power", "24v"},
		"变压器":      {"transformer"},
		"电机":       {"motor", "马达"},
		"马达":       {"motor", "电机"},
		"开关":       {"switch"},
		"传感器":      {"sensor"},
		"按钮":       {"push button", "button"},
		"铝板":       {"aluminum", "plate", "anodized"},
		"触摸屏":      {"hmi", "touch panel", "display"},
		"万可":       {"wago"},
		"西门子":      {"siemens"},
		"施耐德":      {"schneider"},
		"魏德米勒":     {"weidmuller"},
		"菲尼克斯":     {"phoenix"},
	}
	for key, syns := range synonyms {
		if strings.Contains(token, key) {
			variants = append(variants, syns...)
		}
	}
	return variants
}

func (a *app) showPartSource(p *Part) {
	if p.SourceFile == "" {
		return
	}
	for i := range a.books {
		if a.books[i].SourceFile == p.SourceFile {
			a.workbookSel = i
			a.sheetSel = -1
			sheetName := p.SourceSheet
			if sheetName == "" {
				sheetName = p.Subcategory
			}
			for j, sheet := range a.books[i].Sheets {
				if sheet.Name == sheetName {
					a.sheetSel = j
					break
				}
			}
			a.query = ""
			a.refreshResults(p.ID)
			return
		}
	}
}

func (a *app) openAdd() {
	a.editing = false
	a.formError = ""
	a.draft = Part{
		Status: "active",
		Unit:   "个",
		Origin: "manual",
	}
	a.formStatus = "有效"
	if book := a.selectedBook(); book != nil {
		a.draft.Category = book.Category
		if sheet := a.selectedSheet(); sheet != "" {
			a.draft.SourceFile = book.SourceFile
			a.draft.SourceSheet = sheet
			a.draft.Subcategory = sheet
		}
	}
	a.formOpen = true
}

func (a *app) openEdit() {
	p := a.selectedPart()
	if p == nil {
		return
	}
	a.editing = true
	a.formError = ""
	a.draft = *p
	a.formStatus = statusLabel(p.Status)
	a.formOpen = true
}

func (a *app) duplicateDraftCount() int {
	bhs := strings.TrimSpace(a.draft.BHSNo)
	if bhs == "" {
		return 0
	}
	count := 0
	for _, p := range a.parts {
		if p.BHSNo == bhs && p.ID != a.draft.ID {
			count++
		}
	}
	return count
}

func (a *app) saveDraft(c *ui.Context) {
	a.draft.BHSNo = cleanField(a.draft.BHSNo)
	a.draft.Category = cleanField(a.draft.Category)
	a.draft.DescriptionCN = cleanField(a.draft.DescriptionCN)
	a.draft.DescriptionEN = cleanField(a.draft.DescriptionEN)
	a.draft.Spec = cleanField(a.draft.Spec)
	a.draft.Manufacturer = cleanField(a.draft.Manufacturer)
	a.draft.OrderNo = cleanField(a.draft.OrderNo)
	a.draft.Unit = cleanField(a.draft.Unit)
	a.draft.Dispo = cleanField(a.draft.Dispo)
	a.draft.Cert = cleanField(a.draft.Cert)
	a.draft.Machine = cleanField(a.draft.Machine)
	a.draft.Remark = strings.TrimSpace(a.draft.Remark)

	if a.draft.BHSNo == "" {
		a.formError = "零件号不能为空。"
		return
	}
	if a.draft.Category == "" {
		a.formError = "类别不能为空。"
		return
	}
	a.draft.Status = statusValue(a.formStatus)
	if a.draft.SourceFile != "" && a.draft.SourceSheet == "" {
		a.draft.SourceFile = ""
	}
	if a.draft.SourceFile == "" {
		a.draft.SourceSheet = ""
	}
	if a.draft.Origin == "" {
		a.draft.Origin = "manual"
	}
	keepID := a.draft.ID
	if _, err := a.store.SavePart(context.Background(), &a.draft, !a.editing); err != nil {
		a.formError = "保存失败：" + err.Error()
		return
	}
	if keepID == 0 {
		keepID = a.draft.ID
	}
	a.formOpen = false
	a.formError = ""
	if err := a.reload(keepID); err != nil {
		a.formError = "保存成功，但重新读取失败：" + err.Error()
		c.Toast(a.formError)
		return
	}
	c.Toast(fmt.Sprintf("已保存 %s", a.draft.BHSNo))
}

func statusValue(label string) string {
	switch label {
	case "停用":
		return "inactive"
	case "历史":
		return "obsolete"
	default:
		return "active"
	}
}

func cleanField(s string) string {
	return strings.Join(strings.Fields(strings.TrimSpace(s)), " ")
}

func (a *app) reload(keepID int64) error {
	selectedFile := ""
	selectedSheet := a.selectedSheet()
	if book := a.selectedBook(); book != nil {
		selectedFile = book.SourceFile
	}
	parts, books, summary, err := a.store.LoadAll(context.Background())
	if err != nil {
		return err
	}
	a.parts = parts
	a.books = books
	a.summary = summary
	a.sheetChoices = nil
	a.sheetChoiceFor = ""
	a.workbookSel = -1
	if selectedFile != "" {
		for i := range a.books {
			if a.books[i].SourceFile == selectedFile {
				a.workbookSel = i
				a.sheetSel = -1
				for j, sheet := range a.books[i].Sheets {
					if sheet.Name == selectedSheet {
						a.sheetSel = j
						break
					}
				}
				break
			}
		}
	}
	a.refreshResults(keepID)
	a.refreshTray()
	return nil
}

func formatImportedAt(value string) string {
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return value
	}
	return t.Format("2006.01.02 / 15:04")
}

func logif(format string, args ...any) {
	log.Printf(format, args...)
}

// desktopIsDark reads the desktop's appearance. It is a variable so tests
// can present a light or a dark desktop without one.
var desktopIsDark = func() bool { return mygo.Theme.IsDark() }

// setFontScale saves a new text size; the next frame builds with it.
func (a *app) setFontScale(scale float64) {
	scale = float64(clampFontScale(scale))
	if scale == a.cfg.FontScale {
		return
	}
	a.cfg.FontScale = scale
	_ = saveConfig(a.exeDir, a.cfg)
	a.winInvalidate()
}

func (a *app) nudgeFontScale(delta float64) {
	a.setFontScale(a.cfg.FontScale + delta)
}

// toggleTheme walks the appearance through auto → light → dark → auto, so
// the switch never lands in a state the setting cannot name.
func (a *app) toggleTheme(c *ui.Context) {
	// The toolbar button flips the appearance the user sees; following the
	// system again is a choice in the settings.
	switch a.cfg.Theme {
	case "auto":
		if a.lightTheme {
			a.cfg.Theme = "dark"
			c.Toast("已切换为深色外观")
		} else {
			a.cfg.Theme = "light"
			c.Toast("已切换为浅色外观")
		}
	case "light":
		a.cfg.Theme = "dark"
		c.Toast("已切换为深色外观")
	default:
		a.cfg.Theme = "light"
		c.Toast("已切换为浅色外观")
	}
	a.applyTheme()
	_ = saveConfig(a.exeDir, a.cfg)
}

// applyTheme resolves the configured appearance. "auto" follows the desktop,
// and a change of the desktop appearance reaches the window through
// Theme.OnUpdated registered in main.
func (a *app) applyTheme() {
	switch a.cfg.Theme {
	case "light":
		a.lightTheme = true
	case "dark":
		a.lightTheme = false
	default:
		if a.appReady || mygo.App.IsReady() {
			// Theme.IsDark reports the desktop's appearance, while the
			// palette switch is "use the light palette", so it is inverted.
			a.systemDark = desktopIsDark()
			a.lightTheme = !a.systemDark
		} else {
			// Before the app starts, its desktop is not available; the
			// first ready frame resolves the real appearance.
			a.lightTheme = false
		}
	}
}

// themeButton describes the switch for the toolbar: its icon shows what the
// next click gives, and its label names the current source.
func (a *app) themeButton() (iconName, tip string) {
	switch a.cfg.Theme {
	case "auto":
		if a.systemDark {
			return "sun", "外观跟随系统（深色），点击切换为浅色"
		}
		return "moon", "外观跟随系统（浅色），点击切换为深色"
	case "light":
		return "moon", "浅色外观，点击切换为深色"
	default:
		return "sun", "深色外观，点击切换为浅色"
	}
}

func (a *app) openSettingsTab(tab int) {
	a.settingsTab = tab
	a.datasheetPathsText = strings.Join(a.cfg.DatasheetPaths, "\n")
	a.masterlistPathsText = strings.Join(a.cfg.MasterlistPaths, "\n")
	a.datasheetStatusMsg = ""
	a.datasheetModalOpen = true
}

func (a *app) openDatasheetSettings() {
	a.openSettingsTab(0)
}

func (a *app) openExcelSettings() {
	a.openSettingsTab(1)
}

func (a *app) saveDatasheetSettings(c *ui.Context) {
	// Parse datasheet paths
	lines := strings.Split(a.datasheetPathsText, "\n")
	var dsPaths []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			dsPaths = append(dsPaths, trimmed)
		}
	}
	if len(dsPaths) == 0 {
		dsPaths = []string{`..\PartPDF_merged`, `PartPDF_merged`}
	}
	pathsChanged := !slices.EqualFunc(a.cfg.DatasheetPaths, dsPaths, strings.EqualFold)
	a.cfg.DatasheetPaths = dsPaths

	// Parse masterlist paths
	linesML := strings.Split(a.masterlistPathsText, "\n")
	var mlPaths []string
	for _, line := range linesML {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			mlPaths = append(mlPaths, trimmed)
		}
	}
	if len(mlPaths) == 0 {
		mlPaths = []string{`..\Masterlist`, `Masterlist`}
	}
	a.cfg.MasterlistPaths = mlPaths

	// Parse excluded workbooks or folders.
	var excluded []string
	for _, line := range strings.Split(a.excelExcludeText, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" {
			excluded = append(excluded, trimmed)
		}
	}
	a.cfg.ExcludedWorkbooks = excluded

	// A configured reader must be an existing executable; a relative path is
	// resolved against the program directory.
	reader := strings.TrimSpace(a.cfg.PDFReaderPath)
	if reader != "" {
		if !filepath.IsAbs(reader) {
			reader = filepath.Join(a.exeDir, reader)
		}
		if abs, err := filepath.Abs(reader); err == nil {
			reader = abs
		}
		if fi, err := os.Stat(reader); err != nil || fi.IsDir() {
			a.datasheetStatusMsg = "只读阅读器路径不存在：" + reader
			return
		}
		a.cfg.PDFReaderPath = reader
	}

	if err := saveConfig(a.exeDir, a.cfg); err != nil {
		a.datasheetStatusMsg = "保存配置文件失败: " + err.Error()
		return
	}
	a.applyTraySetting()
	if pathsChanged {
		a.pdfScanner.SetPaths(dsPaths)
		a.scanDatasheetsAsync(true)
	} else {
		c.Toast("路径配置已更新并保存")
	}
	// New folders or a new interval re-arm the background refresh.
	a.restartAutoSync()
	a.datasheetModalOpen = false
}

// refreshDatasheetScan starts a manual, asynchronous index sync.
func (a *app) refreshDatasheetScan(c *ui.Context) {
	a.scanDatasheetsAsync(true)
}

// openExcelFile opens the workbook's local copy, never the source: the
// configured folder may be a network share, and the app promises not to lock
// or edit it. Excel gets the edit password when the setting is on.
func (a *app) openExcelFile(c *ui.Context, filename string) {
	if !fileExists(localWorkbookPath(a.exeDir, filename)) {
		syncErr := a.ensureLocalWorkbook(filename)
		if syncErr != nil {
			c.Toast(syncErr.Error())
			return
		}
	}
	res := openWorkbookInExcel(a.exeDir, filename, a.cfg.ExcelPassword, a.cfg.OpenExcelPassword)
	if res.Err != nil {
		c.Toast("打开 Excel 失败: " + res.Err.Error())
		return
	}
	if res.Private {
		c.Toast("已打开本地副本: " + filename)
	} else {
		c.Toast("已打开原表: " + filename)
	}
}

// ensureLocalWorkbook fetches one workbook when its copy is missing, so the
// first open of a file works even before a full sync.
func (a *app) ensureLocalWorkbook(filename string) error {
	for _, source := range resolveAllExcelPaths(a.exeDir, filename, a.cfg.MasterlistPaths) {
		if err := copyFileAtomic(source, localWorkbookPath(a.exeDir, filename)); err != nil {
			return fmt.Errorf("复制原表副本失败: %w", err)
		}
		return nil
	}
	return fmt.Errorf("未在原表目录中找到文件：%s，请先配置原表路径", filename)
}

// locateExcelFile shows the local copy in Explorer. Revealing the source
// would mean browsing the share, which the setting keeps the app out of.
func (a *app) locateExcelFile(c *ui.Context, filename string) {
	resolved := localWorkbookFor(a.exeDir, filename)
	if resolved == "" {
		if err := a.ensureLocalWorkbook(filename); err != nil {
			c.Toast(err.Error())
			return
		}
		resolved = localWorkbookFor(a.exeDir, filename)
	}
	if resolved == "" {
		c.Toast("没有这份原表的本地副本")
		return
	}
	mygo.Shell.ShowItemInFolder(resolved)
}

func (a *app) triggerSync(c *ui.Context, dryRun bool) {
	a.syncModalOpen = true
	a.syncRunning = true
	a.syncDryRun = dryRun
	a.syncResult = nil
	a.syncErr = ""

	go func() {
		res, err := syncExcelMasterlist(context.Background(), a.store, a.exeDir, a.cfg.MasterlistPaths, a.cfg.ExcludedWorkbooks, dryRun)
		if a.win != nil {
			a.win.Update(func() {
				a.syncRunning = false
				if err != nil {
					a.syncErr = err.Error()
				} else {
					a.syncResult = res
					if !dryRun && (res.NewPartsAdded > 0 || res.UpdatedParts > 0) {
						_ = a.reload(0)
					}
				}
			})
		}
	}()
}
