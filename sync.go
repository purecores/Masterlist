package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"
	"unicode"
)

type SyncResult struct {
	FilesScanned      int      `json:"files_scanned"`
	LocalCopies       int      `json:"local_copies"`
	LocalCopiesKept   int      `json:"local_copies_kept"`
	LocalCopiesFailed int      `json:"local_copies_failed"`
	FilesSkipped      int      `json:"files_skipped"`
	SheetsScanned     int      `json:"sheets_scanned"`
	NewPartsAdded     int      `json:"new_parts_added"`
	UpdatedParts      int      `json:"updated_parts"`
	UnchangedParts    int      `json:"unchanged_parts"`
	ManualPreserved   int      `json:"manual_preserved"`
	Errors            []string `json:"errors"`
	SummaryMessage    string   `json:"summary_message"`
	DryRun            bool     `json:"dry_run"`
}

var headerCleanRe = regexp.MustCompile(`[\s_\-]+`)
var multiSpaceRe = regexp.MustCompile(`\s+`)
var scientificRe = regexp.MustCompile(`^\d+(\.\d+)?[eE]+[+-]?\d+$`)

// Old versions of a sheet are marked with the word "Old", but not by a
// substring: a Fuse Holder sheet is not historical.
var oldWordRe = regexp.MustCompile(`(?i)\bold\b`)

// cleanCellText applies the import specification's whitespace rules: line
// breaks and zero-width characters go, and runs of spaces become one space.
func cleanCellText(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\u200b", "")
	// Full-width digits, letters and punctuation become their ASCII forms,
	// as the import specification requires: the production library stores
	// them this way, so old and new values compare equal.
	s = strings.Map(widthFold, s)
	return strings.TrimSpace(multiSpaceRe.ReplaceAllString(s, " "))
}

func widthFold(r rune) rune {
	switch {
	case r >= 0xFF01 && r <= 0xFF5E:
		return r - 0xFEE0
	case r == 0x3000 || r == 0x00A0:
		return ' '
	default:
		return r
	}
}

func normalizeHeaderName(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	return headerCleanRe.ReplaceAllString(s, "")
}

// canonicalCategory keeps the names the files use, normalising the
// ampersands to the spaced form the library already stores ("Cable&Wire"
// becomes "Cable & Wire").
func canonicalCategory(raw string) string {
	raw = cleanCellText(raw)
	if strings.Contains(raw, "&") && !strings.Contains(raw, " & ") {
		fields := strings.Split(raw, "&")
		for i := range fields {
			fields[i] = strings.TrimSpace(fields[i])
		}
		raw = strings.Join(fields, " & ")
	}
	return cleanCellText(raw)
}

func categoryFromFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	const prefix = "master e-list_"
	if len(base) >= len(prefix) && strings.EqualFold(base[:len(prefix)], prefix) {
		base = base[len(prefix):]
	}
	return canonicalCategory(base)
}

type colMapping struct {
	bhsNo       int
	descCN      int
	descEN      int
	descCombo   int
	spec        int
	mfr         int
	orderNo     int
	unit        int
	dispo       int
	cert        int
	machine     int
	remark      int
	importFlag  int
	price       int
	spareLevel  int
	activeFlag  int
	alternateNo int
	relatedPart int
	// alt maps a second occurrence of a column: the relay kit list and the
	// inactive-parts cross-reference carry two number/name pairs in one
	// row, and both are real parts.
	alt *colMapping
}

func emptyMapping() colMapping {
	return colMapping{
		bhsNo: -1, descCN: -1, descEN: -1, descCombo: -1,
		spec: -1, mfr: -1, orderNo: -1, unit: -1,
		dispo: -1, cert: -1, machine: -1, remark: -1,
		importFlag: -1, price: -1, spareLevel: -1, activeFlag: -1,
		alternateNo: -1, relatedPart: -1,
	}
}

func mappingField(m *colMapping, field string) *int {
	switch field {
	case "bhs":
		return &m.bhsNo
	case "descCN":
		return &m.descCN
	case "descEN":
		return &m.descEN
	case "descCombo":
		return &m.descCombo
	case "spec":
		return &m.spec
	case "mfr":
		return &m.mfr
	case "orderNo":
		return &m.orderNo
	case "unit":
		return &m.unit
	case "dispo":
		return &m.dispo
	case "cert":
		return &m.cert
	case "machine":
		return &m.machine
	case "remark":
		return &m.remark
	case "importFlag":
		return &m.importFlag
	case "price":
		return &m.price
	case "spareLevel":
		return &m.spareLevel
	case "activeFlag":
		return &m.activeFlag
	case "alternateNo":
		return &m.alternateNo
	case "relatedPart":
		return &m.relatedPart
	}
	return nil
}

// assign puts a column into its first free slot of the group, or the
// alternate group when the primary one already has it.
func (m *colMapping) assign(field string, c int) {
	first := mappingField(m, field)
	if first == nil {
		return
	}
	if *first == -1 {
		*first = c
		return
	}
	if m.alt == nil {
		alt := emptyMapping()
		m.alt = &alt
	}
	second := mappingField(m.alt, field)
	if *second == -1 {
		*second = c
	}
}

func detectHeaders(rows [][]string) (int, colMapping) {
	mapping := emptyMapping()

	// Header rows often sit below a title banner or a summary line, so the
	// first ten rows are inspected, as the import specification requires.
	maxScan := min(10, len(rows))
	for r := 0; r < maxScan; r++ {
		hasBHS := false
		cur := mapping
		for c, cell := range rows[r] {
			h := normalizeHeaderName(cell)
			if h == "" {
				continue
			}
			field := ""
			switch {
			case (strings.Contains(h, "bhs") && (strings.Contains(h, "part") || strings.Contains(h, "零件"))) ||
				h == "bhsno" || h == "materialnumber" || h == "bhs零件号" || h == "零件号" || h == "bhs编号":
				field = "bhs"
				hasBHS = true
			case strings.Contains(h, "descriptioncn") || strings.Contains(h, "中文") || h == "零件名称中文":
				field = "descCN"
			case strings.Contains(h, "descriptionen") || strings.Contains(h, "英文") || h == "零件名称英文":
				field = "descEN"
			case strings.Contains(h, "description") || strings.Contains(h, "零件名称") || h == "名称" || h == "品名":
				field = "descCombo"
			case strings.Contains(h, "specification") || strings.Contains(h, "型号") || strings.Contains(h, "规格") || h == "type":
				field = "spec"
			case strings.Contains(h, "manufacture") || strings.Contains(h, "品牌") || strings.Contains(h, "供应商") ||
				strings.Contains(h, "厂商") || strings.Contains(h, "制作商") || strings.Contains(h, "制造商") || strings.Contains(h, "生产商"):
				field = "mfr"
			case strings.Contains(h, "idno") || strings.Contains(h, "订货号") || strings.Contains(h, "orderno"):
				field = "orderNo"
			case strings.Contains(h, "unit") || strings.Contains(h, "单位"):
				field = "unit"
			case strings.Contains(h, "dispo") || strings.Contains(h, "属性"):
				field = "dispo"
			case strings.Contains(h, "cert") || strings.Contains(h, "认证"):
				field = "cert"
			case strings.Contains(h, "machine") || strings.Contains(h, "机型") || strings.Contains(h, "功能"):
				field = "machine"
			case strings.Contains(h, "remark") || strings.Contains(h, "备注"):
				field = "remark"
			case strings.Contains(h, "import") || strings.Contains(h, "whr") || strings.Contains(h, "进口"):
				field = "importFlag"
			case strings.Contains(h, "price") || strings.Contains(h, "价格") || strings.Contains(h, "单价"):
				field = "price"
			case strings.Contains(h, "spare") || strings.Contains(h, "备件"):
				field = "spareLevel"
			case strings.Contains(h, "激活") || strings.Contains(h, "i/a") ||
				strings.Contains(h, "active") || strings.Contains(h, "有效"):
				field = "activeFlag"
			case strings.Contains(h, "二手零件") || strings.Contains(h, "二手件") ||
				strings.Contains(h, "alternate") || strings.Contains(h, "旧零件") || strings.Contains(h, "gno"):
				field = "alternateNo"
			case strings.Contains(h, "关联零件") || strings.Contains(h, "相关零件") ||
				strings.Contains(h, "relatedpart") || strings.Contains(h, "组装件"):
				field = "relatedPart"
			}
			if field != "" {
				cur.assign(field, c)
			}
		}
		if hasBHS {
			return r, cur
		}
	}
	return -1, mapping
}

func getCell(row []string, col int) string {
	if col >= 0 && col < len(row) {
		return cleanCellText(row[col])
	}
	return ""
}

// getRawCell keeps the line breaks a combined Description column uses to
// separate the English and Chinese names.
func getRawCell(row []string, col int) string {
	if col >= 0 && col < len(row) {
		return row[col]
	}
	return ""
}

func hasCJK(s string) bool {
	for _, r := range s {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}

// splitDescription splits one Description column into the Chinese and English
// names that most Master E-lists keep on separate lines
// ("Copacitor\n电容").
func splitDescription(raw string) (cn, en string) {
	raw = strings.ReplaceAll(raw, "\r", "")
	var cnParts, enParts []string
	for _, line := range strings.Split(raw, "\n") {
		line = cleanCellText(line)
		if line == "" {
			continue
		}
		if hasCJK(line) {
			cnParts = append(cnParts, line)
		} else {
			enParts = append(enParts, line)
		}
	}
	return strings.Join(cnParts, " "), strings.Join(enParts, " ")
}

// cleanBHS removes spacing, repairs numbers Excel stored in scientific
// notation, and treats a cell with no letter or digit as empty.
func cleanBHS(val string) string {
	val = cleanCellText(val)
	val = strings.ReplaceAll(val, " ", "")
	if scientificRe.MatchString(val) {
		var f float64
		if _, err := fmt.Sscanf(val, "%g", &f); err == nil && f < 1e15 {
			val = fmt.Sprintf("%.0f", f)
		}
	}
	for _, r := range val {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return val
		}
	}
	return ""
}

func looksLikeSectionRow(values ...string) bool {
	joined := strings.TrimSpace(strings.Join(values, " "))
	if joined == "" {
		return false
	}
	for _, prefix := range []string{"===", "---", "***", "###"} {
		if strings.HasPrefix(joined, prefix) {
			return true
		}
	}
	return false
}

// rowStatus maps the sheet, category and activation column onto the three
// statuses the database allows.
func rowStatus(sheet, category, activeFlag string) string {
	lsheet := strings.ToLower(sheet)
	lcat := strings.ToLower(category)
	if strings.Contains(lsheet, "obsolete") || strings.Contains(lcat, "obsolete") ||
		oldWordRe.MatchString(sheet) || oldWordRe.MatchString(category) ||
		strings.Contains(sheet, "历史") || strings.Contains(category, "历史") ||
		strings.Contains(sheet, "作废") || strings.Contains(category, "作废") {
		return "obsolete"
	}
	// "Inative" is a typo in one of the production workbooks.
	if strings.Contains(lsheet, "inactive") || strings.Contains(lcat, "inactive") ||
		strings.Contains(lsheet, "inative") || strings.Contains(lcat, "inative") ||
		strings.Contains(sheet, "停用") || strings.Contains(category, "停用") {
		return "inactive"
	}
	switch strings.ToLower(strings.TrimSpace(activeFlag)) {
	case "no", "0", "false", "n", "停用", "否", "无效":
		return "inactive"
	case "obsolete", "历史", "作废":
		return "obsolete"
	}
	return "active"
}

// normalizeSourceStem identifies a workbook independently of whether the
// folder holds it as .xlsx or .xls.
func normalizeSourceStem(filename string) string {
	base := filepath.Base(filename)
	return strings.ToLower(strings.TrimSuffix(base, filepath.Ext(base)))
}

// normSourceSQL matches source_file values written with either extension.
const normSourceSQL = `lower(replace(replace(replace(source_file, '.xlsx', ''), '.xlsm', ''), '.xls', ''))`

// migrateSourceFile moves rows of an older .xlsx/.xls copy of the same
// workbook to the file the folder has now, so a format change updates parts
// instead of adding a second copy of every row.
func migrateSourceFile(ctx context.Context, store *Store, fileName string) {
	stem := normalizeSourceStem(fileName)
	_, _ = store.db.Execute(ctx, `
		DELETE FROM source_sheets
		WHERE `+normSourceSQL+` = ? AND lower(source_file) <> lower(?)
	`, stem, fileName)
	_, _ = store.db.Execute(ctx, `
		UPDATE parts SET source_file = ?
		WHERE `+normSourceSQL+` = ? AND lower(source_file) <> lower(?)
	`, fileName, stem, fileName)
}

type workbookCandidate struct {
	rank int
	path string
}

func extensionRank(ext string) int {
	switch ext {
	case ".xlsx":
		return 3
	case ".xlsm":
		return 2
	case ".xls":
		return 1
	}
	return 0
}

// collectWorkbooks finds Master E-list_ workbooks in the configured folders.
// Helper spreadsheets in the same folder (replacement lists, icon catalogs,
// price summaries) are not part of the library, so they are skipped and
// counted. When one folder holds the same table as .xlsx and .xls, the .xlsx
// is read.
func collectWorkbooks(exeDir string, paths []string) (map[string]string, int, []string) {
	candidates := map[string]workbookCandidate{}
	skipped := 0
	var notes []string
	for _, p := range paths {
		dir := resolveConfiguredDir(exeDir, p)
		if dir == "" {
			continue
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			notes = append(notes, fmt.Sprintf("%s: %v", dir, err))
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() || strings.HasPrefix(entry.Name(), "~$") {
				continue
			}
			name := entry.Name()
			ext := strings.ToLower(filepath.Ext(name))
			rank := extensionRank(ext)
			if rank == 0 {
				continue
			}
			if !strings.HasPrefix(strings.ToLower(name), "master e-list_") {
				skipped++
				continue
			}
			stem := normalizeSourceStem(name)
			if cur, ok := candidates[stem]; !ok || rank > cur.rank {
				candidates[stem] = workbookCandidate{rank: rank, path: filepath.Join(dir, name)}
			}
		}
	}
	files := make(map[string]string, len(candidates))
	for _, cand := range candidates {
		files[filepath.Base(cand.path)] = cand.path
	}
	return files, skipped, notes
}

type existingImport struct {
	id     int64
	origin string
	p      Part
}

func syncExcelMasterlist(ctx context.Context, store *Store, exeDir string, paths []string, dryRun bool) (*SyncResult, error) {
	res := &SyncResult{DryRun: dryRun}
	now := time.Now().Format(time.RFC3339)

	files, skipped, notes := collectWorkbooks(exeDir, paths)
	res.FilesSkipped = skipped
	res.Errors = append(res.Errors, notes...)
	if len(files) == 0 {
		res.SummaryMessage = "未检索到 Master E-list_*.xlsx / *.xls 原表，请检查原表路径设置。"
		return res, nil
	}
	res.FilesScanned = len(files)

	// Import from the app's own copy of each workbook. The source folders may
	// be network shares: reading the local copies keeps the import fast and
	// reliable, and leaves the originals untouched.
	if !dryRun {
		copied, unchanged, missing, copyNotes := syncLocalWorkbooks(exeDir, files)
		res.LocalCopies = copied
		res.LocalCopiesKept = unchanged
		res.LocalCopiesFailed = missing
		res.Errors = append(res.Errors, copyNotes...)
		for name := range files {
			if local := localWorkbookFor(exeDir, name); local != "" {
				files[name] = local
			}
		}
	}

	names := make([]string, 0, len(files))
	for name := range files {
		names = append(names, name)
	}
	slices.Sort(names)

	for _, fileName := range names {
		workbook, err := openWorkbook(files[fileName])
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: 打开失败: %v", fileName, err))
			continue
		}
		if workbook.err != "" {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: %s", fileName, workbook.err))
		}
		category := categoryFromFilename(fileName)
		if !dryRun {
			migrateSourceFile(ctx, store, fileName)
		}

		for _, sheet := range workbook.sheets {
			if len(sheet.rows) == 0 {
				continue
			}
			res.SheetsScanned++
			headerIdx, m := detectHeaders(sheet.rows)
			if headerIdx < 0 || m.bhsNo < 0 {
				continue
			}
			if !dryRun {
				_, _ = store.db.Execute(ctx, `
					INSERT OR IGNORE INTO source_sheets (source_file, source_sheet, category)
					VALUES (?, ?, ?)
				`, fileName, sheet.name, category)
			}

			for r := headerIdx + 1; r < len(sheet.rows); r++ {
				red := workbook.cellRed(sheet.name, r+1, m.bhsNo+1)
				for _, p := range partsFromRow(sheet.rows[r], m, fileName, sheet.name, category, int64(r+1), red) {
					if red {
						// The production sheets mark a disused part by painting
						// its number red; it stays listed as historical.
						p.Status = "obsolete"
					}
					p.Origin = "import"
					p.SearchText = partSearchText(p)

					existing, err := findExistingImport(ctx, store, fileName, sheet.name, p.SourceRow, p)
					if err != nil {
						res.Errors = append(res.Errors, fmt.Sprintf("%s / %s 第 %d 行: %v", fileName, sheet.name, p.SourceRow, err))
						continue
					}
					if existing == nil {
						res.NewPartsAdded++
						if !dryRun {
							if err := insertImportedPart(ctx, store, p, now); err != nil {
								res.Errors = append(res.Errors, fmt.Sprintf("%s / %s 第 %d 行: %v", fileName, sheet.name, p.SourceRow, err))
							}
						}
						continue
					}
					if existing.origin == "manual" {
						res.ManualPreserved++
						continue
					}
					if importedPartEqual(&existing.p, p) {
						res.UnchangedParts++
						continue
					}
					res.UpdatedParts++
					if os.Getenv("MASTERLIST_DEBUG_SYNC") != "" && res.UpdatedParts <= 30 {
						fmt.Fprintf(os.Stderr, "UPDATE id=%d oldRow=%d newRow=%d :: %s\n",
							existing.id, existing.p.SourceRow, p.SourceRow, partDiff(&existing.p, p))
					}
					if !dryRun {
						if err := updateImportedPart(ctx, store, existing.id, p, now); err != nil {
							res.Errors = append(res.Errors, fmt.Sprintf("%s / %s 第 %d 行: %v", fileName, sheet.name, p.SourceRow, err))
						}
					}
				}
			}
		}
		workbook.Close()
	}

	if !dryRun {
		_, _ = store.db.Execute(ctx, `
			INSERT OR REPLACE INTO meta (key, value) VALUES ('imported_at', ?)
		`, time.Now().Format("2006.01.02 / 15:04"))
		_, _ = store.db.Execute(ctx, `
			INSERT INTO activity (action, title, detail, operator, created_at)
			VALUES ('excel_sync', '从 Excel 原表同步', ?, '设计同步', ?)
		`, fmt.Sprintf("新增 %d 件，更新 %d 件，保护手工维护 %d 件", res.NewPartsAdded, res.UpdatedParts, res.ManualPreserved), now)
	}

	skipNote := ""
	if res.FilesSkipped > 0 {
		skipNote = fmt.Sprintf("，跳过 %d 份非 Master E-list 表格", res.FilesSkipped)
	}
	if dryRun {
		res.SummaryMessage = fmt.Sprintf("检测完成：已扫描 %d 份 Master E-list 表格（%d 个工作表）%s，检测到新增 %d 件器件，更新 %d 件，无变更 %d 件，已保护手工维护 %d 件。",
			res.FilesScanned, res.SheetsScanned, skipNote, res.NewPartsAdded, res.UpdatedParts, res.UnchangedParts, res.ManualPreserved)
	} else {
		res.SummaryMessage = fmt.Sprintf("同步成功：已同步 %d 份 Master E-list 表格（%d 个工作表）%s，成功入库新增 %d 件器件，更新 %d 件，已保护手工维护 %d 件。",
			res.FilesScanned, res.SheetsScanned, skipNote, res.NewPartsAdded, res.UpdatedParts, res.ManualPreserved)
	}
	return res, nil
}

// partsFromRow reads one data row into one part, or two when the workbook
// carries a second number/name pair (a relay kit with its component, or an
// inactive part beside its replacement). Empty and decorative rows produce
// none.
func partsFromRow(row []string, m colMapping, fileName, sheetName, category string, sourceRow int64, red bool) []*Part {
	primary := partFromRowWith(row, &m, fileName, sheetName, category, sourceRow, nil, red)
	var parts []*Part
	if primary != nil {
		parts = append(parts, primary)
	}
	if m.alt == nil || m.alt.bhsNo < 0 {
		return parts
	}
	second := partFromRowWith(row, m.alt, fileName, sheetName, category, sourceRow, &m, false)
	if second == nil {
		return parts
	}
	if primary == nil || second.BHSNo != primary.BHSNo ||
		second.Spec != primary.Spec || second.DescriptionCN != primary.DescriptionCN {
		parts = append(parts, second)
	}
	return parts
}

func partFromRowWith(row []string, m *colMapping, fileName, sheetName, category string, sourceRow int64, fallback *colMapping, red bool) *Part {
	// shared reads a field from this group, then from the primary group
	// when this group lacks it: a relay's components keep the system spec
	// and brand in one pair of columns shared with the kit.
	shared := func(field string) string {
		if col := mappingField(m, field); col != nil && *col >= 0 {
			if v := getCell(row, *col); v != "" {
				return v
			}
		}
		if fallback != nil {
			if col := mappingField(fallback, field); col != nil && *col >= 0 {
				return getCell(row, *col)
			}
		}
		return ""
	}

	bhsNo := cleanBHS(getCell(row, m.bhsNo))

	descCN := getCell(row, m.descCN)
	descEN := getCell(row, m.descEN)
	if m.descCombo >= 0 && (descCN == "" || descEN == "") {
		cn, en := splitDescription(getRawCell(row, m.descCombo))
		if descCN == "" {
			descCN = cn
		}
		if descEN == "" {
			descEN = en
		}
	}
	if descCN != "" || descEN != "" {
		cnHasCJK, enHasCJK := hasCJK(descCN), hasCJK(descEN)
		if !cnHasCJK && enHasCJK {
			descCN, descEN = descEN, descCN
		}
	}
	spec := shared("spec")
	// A number or a name identifies a part; a spec shared across a whole
	// row is not a part of its own.
	if bhsNo == "" && descCN == "" && descEN == "" {
		return nil
	}
	if bhsNo != "" && len([]rune(bhsNo)) < 6 && descCN == "" && descEN == "" && spec == "" &&
		shared("mfr") == "" && shared("orderNo") == "" {
		return nil
	}
	if looksLikeSectionRow(bhsNo, descCN, descEN, spec) {
		return nil
	}

	unit := shared("unit")
	if unit == "" {
		unit = "个"
	}
	activeFlag := shared("activeFlag")
	return &Part{
		BHSNo:         bhsNo,
		DescriptionCN: descCN,
		DescriptionEN: descEN,
		Spec:          spec,
		Manufacturer:  shared("mfr"),
		OrderNo:       shared("orderNo"),
		Category:      category,
		Subcategory:   sheetName,
		Unit:          unit,
		Dispo:         shared("dispo"),
		Cert:          shared("cert"),
		Machine:       shared("machine"),
		ImportFlag:    shared("importFlag"),
		Remark:        shared("remark"),
		Price:         shared("price"),
		SpareLevel:    shared("spareLevel"),
		ActiveFlag:    activeFlag,
		AlternateNo:   shared("alternateNo"),
		RelatedPart:   shared("relatedPart"),
		Status:        rowStatus(sheetName, category, activeFlag),
		Discontinued:  red,
		SourceFile:    fileName,
		SourceSheet:   sheetName,
		SourceRow:     sourceRow,
	}
}

// findExistingImport finds the row an import updates: the same sheet and
// row first, then the same sheet with the same number and specification (a
// row that moved), then anywhere in the workbook with the same number and
// specification (a renamed sheet).
func findExistingImport(ctx context.Context, store *Store, fileName, sheet string, sourceRow int64, fresh *Part) (*existingImport, error) {
	stem := normalizeSourceStem(fileName)
	base := `
		SELECT id, origin, bhs_no, description_cn, description_en, spec,
		       manufacturer, order_no, category, subcategory, unit, dispo,
		       cert, machine, import_flag, remark, price, spare_level,
		       active_flag, alternate_no, related_part, status, extra_json,
		       source_file, source_sheet, source_row
		FROM parts
		WHERE ` + normSourceSQL + ` = ?`

	// One source row can name several parts, and a workbook repeats the
	// same number in several rows with different machine ranges. A fallback
	// therefore only accepts a candidate whose imported fields all match;
	// otherwise the row is a second, distinct part and is inserted rather
	// than overwriting an older row.
	direct := base
	var args []any
	if fresh.BHSNo != "" {
		direct += ` AND bhs_no = ? AND source_sheet = ? AND source_row = ? LIMIT 1`
		args = []any{stem, fresh.BHSNo, sheet, sourceRow}
	} else {
		direct += ` AND description_cn = ? AND source_sheet = ? AND source_row = ? LIMIT 1`
		args = []any{stem, fresh.DescriptionCN, sheet, sourceRow}
	}
	if hit, err := queryOneImport(ctx, store, direct, args); err != nil {
		return nil, err
	} else if hit != nil {
		return hit, nil
	}

	// A row that moved: look for an exact field match in the same sheet.
	var candidates []string
	var candidateArgs [][]any
	if fresh.BHSNo != "" {
		candidates = append(candidates, base+` AND bhs_no = ? AND source_sheet = ? LIMIT 20`)
		candidateArgs = append(candidateArgs, []any{stem, fresh.BHSNo, sheet})
	} else {
		candidates = append(candidates, base+` AND description_cn = ? AND source_sheet = ? LIMIT 20`)
		candidateArgs = append(candidateArgs, []any{stem, fresh.DescriptionCN, sheet})
	}
	// A renamed sheet: the same number and spec anywhere in the workbook.
	if fresh.BHSNo != "" {
		candidates = append(candidates, base+` AND bhs_no = ? AND spec = ? LIMIT 20`)
		candidateArgs = append(candidateArgs, []any{stem, fresh.BHSNo, fresh.Spec})
	}
	for i, query := range candidates {
		hits, err := queryImportCandidates(ctx, store, query, candidateArgs[i])
		if err != nil {
			return nil, err
		}
		for _, hit := range hits {
			if sameSheet := i == 0 || i == 1; sameSheet {
				if importedPartEqual(&hit.p, fresh) {
					return hit, nil
				}
				continue
			}
			// The sheet may have been renamed: every other imported field
			// must still match, and there must be exactly one such row.
			if len(hits) == 1 && importedPartEqualExceptSheet(&hit.p, fresh) {
				return hit, nil
			}
		}
	}
	return nil, nil
}

func queryOneImport(ctx context.Context, store *Store, query string, args []any) (*existingImport, error) {
	hits, err := queryImportCandidates(ctx, store, query, args)
	if err != nil || len(hits) == 0 {
		return nil, err
	}
	return hits[0], nil
}

func queryImportCandidates(ctx context.Context, store *Store, query string, args []any) ([]*existingImport, error) {
	rows, err := store.db.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	idx := columnIndex(rows.Columns)
	hits := make([]*existingImport, 0, len(rows.Values))
	for _, v := range rows.Values {
		part := Part{
			BHSNo:         stringValue(v, idx, "bhs_no"),
			DescriptionCN: stringValue(v, idx, "description_cn"),
			DescriptionEN: stringValue(v, idx, "description_en"),
			Spec:          stringValue(v, idx, "spec"),
			Manufacturer:  stringValue(v, idx, "manufacturer"),
			OrderNo:       stringValue(v, idx, "order_no"),
			Category:      stringValue(v, idx, "category"),
			Subcategory:   stringValue(v, idx, "subcategory"),
			Unit:          stringValue(v, idx, "unit"),
			Dispo:         stringValue(v, idx, "dispo"),
			Cert:          stringValue(v, idx, "cert"),
			Machine:       stringValue(v, idx, "machine"),
			ImportFlag:    stringValue(v, idx, "import_flag"),
			Remark:        stringValue(v, idx, "remark"),
			Price:         stringValue(v, idx, "price"),
			SpareLevel:    stringValue(v, idx, "spare_level"),
			ActiveFlag:    stringValue(v, idx, "active_flag"),
			AlternateNo:   stringValue(v, idx, "alternate_no"),
			RelatedPart:   stringValue(v, idx, "related_part"),
			Status:        stringValue(v, idx, "status"),
			ExtraJSON:     stringValue(v, idx, "extra_json"),
			SourceFile:    stringValue(v, idx, "source_file"),
			SourceSheet:   stringValue(v, idx, "source_sheet"),
			SourceRow:     intValue(v, idx, "source_row"),
		}
		part.Discontinued = strings.Contains(part.ExtraJSON, `"discontinued":true`)
		hits = append(hits, &existingImport{
			id:     intValue(v, idx, "id"),
			origin: stringValue(v, idx, "origin"),
			p:      part,
		})
	}
	return hits, nil
}

// importedPartEqual compares every column the sync writes, so a change to
// just a certification, machine range or note is not overlooked.
func importedPartEqual(old, fresh *Part) bool {
	return normalizeSourceStem(old.SourceFile) == normalizeSourceStem(fresh.SourceFile) &&
		old.BHSNo == fresh.BHSNo &&
		old.DescriptionCN == fresh.DescriptionCN &&
		old.DescriptionEN == fresh.DescriptionEN &&
		old.Spec == fresh.Spec &&
		old.Manufacturer == fresh.Manufacturer &&
		old.OrderNo == fresh.OrderNo &&
		old.Category == fresh.Category &&
		old.Subcategory == fresh.Subcategory &&
		old.Unit == fresh.Unit &&
		old.Dispo == fresh.Dispo &&
		old.Cert == fresh.Cert &&
		old.Machine == fresh.Machine &&
		old.ImportFlag == fresh.ImportFlag &&
		old.Remark == fresh.Remark &&
		old.Price == fresh.Price &&
		old.SpareLevel == fresh.SpareLevel &&
		old.ActiveFlag == fresh.ActiveFlag &&
		old.AlternateNo == fresh.AlternateNo &&
		old.RelatedPart == fresh.RelatedPart &&
		old.Status == fresh.Status &&
		old.Discontinued == fresh.Discontinued &&
		old.SourceSheet == fresh.SourceSheet
}

func partDiff(old, fresh *Part) string {
	var b strings.Builder
	add := func(name, a, c string) {
		if a != c {
			fmt.Fprintf(&b, "%s[%q->%q] ", name, a, c)
		}
	}
	add("file", old.SourceFile, fresh.SourceFile)
	add("bhs", old.BHSNo, fresh.BHSNo)
	add("cn", old.DescriptionCN, fresh.DescriptionCN)
	add("en", old.DescriptionEN, fresh.DescriptionEN)
	add("spec", old.Spec, fresh.Spec)
	add("mfr", old.Manufacturer, fresh.Manufacturer)
	add("order", old.OrderNo, fresh.OrderNo)
	add("cat", old.Category, fresh.Category)
	add("sub", old.Subcategory, fresh.Subcategory)
	add("unit", old.Unit, fresh.Unit)
	add("dispo", old.Dispo, fresh.Dispo)
	add("cert", old.Cert, fresh.Cert)
	add("machine", old.Machine, fresh.Machine)
	add("import", old.ImportFlag, fresh.ImportFlag)
	add("remark", old.Remark, fresh.Remark)
	add("price", old.Price, fresh.Price)
	add("spare", old.SpareLevel, fresh.SpareLevel)
	add("active", old.ActiveFlag, fresh.ActiveFlag)
	add("alt", old.AlternateNo, fresh.AlternateNo)
	add("rel", old.RelatedPart, fresh.RelatedPart)
	add("status", old.Status, fresh.Status)
	if old.Discontinued != fresh.Discontinued {
		fmt.Fprintf(&b, "disc[%v->%v] ", old.Discontinued, fresh.Discontinued)
	}
	return b.String()
}

// discontinuedExtra keeps the extra_json column's other keys while recording
// whether this row is marked as disused in the original table.
func discontinuedExtra(discontinued bool) string {
	if discontinued {
		return `{"discontinued":true}`
	}
	return "{}"
}

func importedPartEqualExceptSheet(old, fresh *Part) bool {
	return normalizeSourceStem(old.SourceFile) == normalizeSourceStem(fresh.SourceFile) &&
		old.BHSNo == fresh.BHSNo &&
		old.DescriptionCN == fresh.DescriptionCN &&
		old.DescriptionEN == fresh.DescriptionEN &&
		old.Spec == fresh.Spec &&
		old.Manufacturer == fresh.Manufacturer &&
		old.OrderNo == fresh.OrderNo &&
		old.Category == fresh.Category &&
		old.Subcategory == fresh.Subcategory &&
		old.Unit == fresh.Unit &&
		old.Dispo == fresh.Dispo &&
		old.Cert == fresh.Cert &&
		old.Machine == fresh.Machine &&
		old.ImportFlag == fresh.ImportFlag &&
		old.Remark == fresh.Remark &&
		old.Price == fresh.Price &&
		old.SpareLevel == fresh.SpareLevel &&
		old.ActiveFlag == fresh.ActiveFlag &&
		old.AlternateNo == fresh.AlternateNo &&
		old.RelatedPart == fresh.RelatedPart &&
		old.Status == fresh.Status &&
		old.Discontinued == fresh.Discontinued
}

func updateImportedPart(ctx context.Context, store *Store, id int64, p *Part, now string) error {
	extra := discontinuedExtra(p.Discontinued)
	_, err := store.db.Execute(ctx, `
		UPDATE parts SET
			bhs_no = ?, description_cn = ?, description_en = ?, spec = ?,
			manufacturer = ?, order_no = ?, category = ?, subcategory = ?,
			unit = ?, dispo = ?, cert = ?, machine = ?, import_flag = ?,
			remark = ?, price = ?, spare_level = ?, active_flag = ?,
			alternate_no = ?, related_part = ?, status = ?, search_text = ?, extra_json = ?,
			source_file = ?, source_sheet = ?, source_row = ?, updated_at = ?
		WHERE id = ?
	`,
		p.BHSNo, p.DescriptionCN, p.DescriptionEN, p.Spec,
		p.Manufacturer, p.OrderNo, p.Category, p.Subcategory,
		p.Unit, p.Dispo, p.Cert, p.Machine, p.ImportFlag,
		p.Remark, p.Price, p.SpareLevel, p.ActiveFlag,
		p.AlternateNo, p.RelatedPart, p.Status, p.SearchText, extra,
		p.SourceFile, p.SourceSheet, p.SourceRow, now, id,
	)
	return err
}

func insertImportedPart(ctx context.Context, store *Store, p *Part, now string) error {
	extra := discontinuedExtra(p.Discontinued)
	_, err := store.db.Execute(ctx, `
		INSERT INTO parts (
			bhs_no, description_cn, description_en, spec, manufacturer,
			order_no, category, subcategory, unit, dispo, cert, machine,
			import_flag, remark, price, spare_level, active_flag,
			alternate_no, related_part, status, origin, search_text, extra_json, source_file,
			source_sheet, source_row, created_at, updated_at
		) VALUES (
			?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
		)
	`,
		p.BHSNo, p.DescriptionCN, p.DescriptionEN, p.Spec, p.Manufacturer,
		p.OrderNo, p.Category, p.Subcategory, p.Unit, p.Dispo, p.Cert, p.Machine,
		p.ImportFlag, p.Remark, p.Price, p.SpareLevel, p.ActiveFlag,
		p.AlternateNo, p.RelatedPart, p.Status, "import", p.SearchText, extra, p.SourceFile,
		p.SourceSheet, p.SourceRow, now, now,
	)
	return err
}
