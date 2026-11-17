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

	summary Summary
	parts   []*Part
	books   []WorkbookInfo

	query         string
	workbookSel   int
	sheetSel      int
	statusSel     int
	sortLabel     string
	results       []*Part
	resultSel     int
	inspectorOpen bool

	workbookList ui.ListState
	resultList   ui.ListState
	searchHandle ui.Handle
	sheetScroll  ui.ScrollState
	detailScroll ui.ScrollState

	formOpen   bool
	editing    bool
	draft      Part
	formStatus string
	formError  string

	exeDir              string
	cfg                 Config
	pdfScanner          *PDFScanner
	datasheetModalOpen  bool
	datasheetPathsText  string
	masterlistPathsText string
	datasheetStatusMsg  string
	settingsTab         int // 0: PDF 手册, 1: Excel 原表

	lightTheme    bool
	syncModalOpen bool
	syncRunning   bool
	syncDryRun    bool
	syncResult    *SyncResult
	syncErr       string
}

var statusOptions = []string{"全部", "有效", "停用", "历史"}
var sortOptions = []string{"原表顺序", "BHS 编号", "最近修改"}

var statusLabels = map[string]string{
	"active":   "有效",
	"inactive": "停用",
	"obsolete": "历史",
}

var categoryNames = map[string]string{
	"Accessories":           "附件",
	"Air Conditioner":       "空调",
	"Aluminum Plate":        "铝板",
	"Battery":               "电池",
	"Bus bar":               "母线",
	"Cable & Wire":          "电缆与导线",
	"Circuit Breaker":       "断路器",
	"Controller":            "控制器",
	"Copacitor":             "电容",
	"DC Power Supply Unit":  "直流电源",
	"Diode":                 "二极管",
	"Display & Light":       "显示与照明",
	"Electronic":            "电子元件",
	"Emergency Stop Module": "急停模块",
	"Encoder":               "编码器",
	"Fan":                   "风扇",
	"Fuse System":           "熔断系统",
	"Heavy Duty Connectors": "重载连接器",
	"Horn":                  "蜂鸣器",
	"Inactive Parts":        "停用件",
	"Isolation":             "隔离器件",
	"Limit Switch":          "限位开关",
	"Moeller":               "穆勒",
	"Motor Protect Switch":  "电机保护开关",
	"Motors":                "电机",
	"Network":               "网络",
	"ODM":                   "ODM",
	"PLC":                   "PLC",
	"Push Button":           "按钮",
	"Relay & Contactor":     "继电器与接触器",
	"Schneider":             "施耐德",
	"Sensor":                "传感器",
	"Siemens":               "西门子",
	"Socket":                "插座",
	"Switch":                "开关",
	"System Components":     "系统组件",
	"Terminal Block":        "端子排",
	"Transformer":           "变压器",
	"UPS":                   "UPS",
	"Vacuum Generator":      "真空发生器",
	"Valve":                 "阀门",
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
		statusSel:           0,
		sortLabel:           sortOptions[0],
		query:               initialQuery,
		resultSel:           -1,
		inspectorOpen:       true,
		datasheetModalOpen:  openSettings,
		datasheetPathsText:  strings.Join(cfg.DatasheetPaths, "\n"),
		masterlistPathsText: strings.Join(cfg.MasterlistPaths, "\n"),
		lightTheme:          cfg.Theme == "light",
	}
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
	})

	// Scan PDF datasheets in background so window opens instantly without blocking startup
	go func() {
		a.pdfScanner.Scan()
		if a.win != nil {
			a.win.Update(func() {})
		}
	}()
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

	for _, p := range a.parts {
		if selectedCategory != "" && p.Category != selectedCategory && p.SourceFile != selectedFile {
			continue
		}
		if sheet != "" {
			if p.SourceSheet != sheet && p.Subcategory != sheet {
				continue
			}
		}
		if a.statusSel > 0 {
			switch a.statusSel {
			case 1:
				if p.Status != "active" {
					continue
				}
			case 2:
				if p.Status != "inactive" {
					continue
				}
			case 3:
				if p.Status != "obsolete" {
					continue
				}
			}
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
		filtered = append(filtered, p)
	}

	switch a.sortLabel {
	case "BHS 编号":
		slices.SortFunc(filtered, func(a, b *Part) int {
			return strings.Compare(strings.ToLower(a.BHSNo), strings.ToLower(b.BHSNo))
		})
	case "最近修改":
		slices.SortFunc(filtered, func(a, b *Part) int {
			return strings.Compare(b.UpdatedAt, a.UpdatedAt)
		})
	default:
		slices.SortFunc(filtered, func(a, b *Part) int {
			if a.SourceFile != b.SourceFile {
				return strings.Compare(a.SourceFile, b.SourceFile)
			}
			if a.SourceSheet != b.SourceSheet {
				return strings.Compare(a.SourceSheet, b.SourceSheet)
			}
			return int(a.SourceRow - b.SourceRow)
		})
	}

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

func expandQueryToken(token string) []string {
	variants := []string{token}
	synonyms := map[string][]string{
		"断路器": {"breaker", "空开", "circuit breaker"},
		"空开":   {"断路器", "breaker", "circuit breaker"},
		"breaker": {"断路器", "空开"},
		"端子":   {"terminal", "wago", "万可"},
		"terminal": {"端子", "wago"},
		"继电器": {"relay", "contactor", "接触器"},
		"relay": {"继电器", "接触器"},
		"接触器": {"contactor", "relay", "继电器"},
		"电源":   {"power supply", "dc power", "24v"},
		"变压器": {"transformer"},
		"电机":   {"motor", "马达"},
		"马达":   {"motor", "电机"},
		"开关":   {"switch"},
		"传感器": {"sensor"},
		"按钮":   {"push button", "button"},
		"铝板":   {"aluminum", "plate", "anodized"},
		"触摸屏": {"hmi", "touch panel", "display"},
		"万可":   {"wago"},
		"西门子": {"siemens"},
		"施耐德": {"schneider"},
		"魏德米勒": {"weidmuller"},
		"菲尼克斯": {"phoenix"},
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
		a.formError = "BHS 编号不能为空。"
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

func (a *app) toggleTheme(c *ui.Context) {
	a.lightTheme = !a.lightTheme
	if a.lightTheme {
		a.cfg.Theme = "light"
		c.Toast("已切换为白天浅色模式")
	} else {
		a.cfg.Theme = "dark"
		c.Toast("已切换为深色工业模式")
	}
	_ = saveConfig(a.exeDir, a.cfg)
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

	if err := saveConfig(a.exeDir, a.cfg); err != nil {
		a.datasheetStatusMsg = "保存配置文件失败: " + err.Error()
		return
	}
	a.pdfScanner.SetPaths(dsPaths)
	a.pdfScanner.Scan()
	count := a.pdfScanner.Count()
	c.Toast(fmt.Sprintf("路径配置已更新并保存，手册库检索到 %s 份 PDF", formatInt(count)))
	a.datasheetModalOpen = false
}

func (a *app) refreshDatasheetScan(c *ui.Context) {
	a.pdfScanner.Scan()
	count := a.pdfScanner.Count()
	c.Toast(fmt.Sprintf("扫描完成，检索到 %s 份 PDF", formatInt(count)))
}

func (a *app) openExcelFile(c *ui.Context, filename string) {
	resolved := resolveExcelPath(a.exeDir, a.cfg.MasterlistPaths, filename)
	if resolved == "" {
		c.Toast("未在原表目录中找到文件：" + filename + "，请先配置原表路径")
		a.openSettingsTab(1)
		return
	}
	if err := mygo.Shell.OpenPath(resolved); err != nil {
		c.Toast("打开 Excel 失败: " + err.Error())
	} else {
		c.Toast("正在打开原表: " + filename)
	}
}

func (a *app) locateExcelFile(c *ui.Context, filename string) {
	resolved := resolveExcelPath(a.exeDir, a.cfg.MasterlistPaths, filename)
	if resolved == "" {
		c.Toast("未在原表目录中找到文件：" + filename + "，请先配置原表路径")
		a.openSettingsTab(1)
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
		res, err := syncExcelMasterlist(context.Background(), a.store, a.exeDir, a.cfg.MasterlistPaths, dryRun)
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
