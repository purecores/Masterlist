package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/xuri/excelize/v2"
)

type SyncResult struct {
	FilesScanned    int      `json:"files_scanned"`
	SheetsScanned   int      `json:"sheets_scanned"`
	NewPartsAdded   int      `json:"new_parts_added"`
	UpdatedParts    int      `json:"updated_parts"`
	UnchangedParts  int      `json:"unchanged_parts"`
	ManualPreserved int      `json:"manual_preserved"`
	Errors          []string `json:"errors"`
	SummaryMessage  string   `json:"summary_message"`
	DryRun          bool     `json:"dry_run"`
}

var headerCleanRe = regexp.MustCompile(`[\s_\-]+`)
var multiSpaceRe = regexp.MustCompile(`\s+`)

func cleanCellText(val string) string {
	return strings.TrimSpace(multiSpaceRe.ReplaceAllString(val, " "))
}

func normalizeHeaderName(val string) string {
	val = strings.ToLower(strings.TrimSpace(val))
	return headerCleanRe.ReplaceAllString(val, "")
}

func categoryFromFilename(filename string) string {
	base := filepath.Base(filename)
	base = strings.TrimSuffix(base, filepath.Ext(base))
	base = strings.TrimPrefix(base, "Master E-list_")
	return base
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
}

func detectHeaders(rows [][]string) (int, colMapping) {
	mapping := colMapping{
		bhsNo: -1, descCN: -1, descEN: -1, descCombo: -1,
		spec: -1, mfr: -1, orderNo: -1, unit: -1,
		dispo: -1, cert: -1, machine: -1, remark: -1,
		importFlag: -1, price: -1, spareLevel: -1, activeFlag: -1,
		alternateNo: -1,
	}

	maxScan := min(5, len(rows))
	for r := 0; r < maxScan; r++ {
		row := rows[r]
		hasBHS := false
		cur := mapping

		for c, cell := range row {
			h := normalizeHeaderName(cell)
			if h == "" {
				continue
			}
			if (strings.Contains(h, "bhs") && (strings.Contains(h, "part") || strings.Contains(h, "零件"))) ||
				h == "bhsno" || h == "materialnumber" || h == "bhs零件号" || h == "零件号" {
				cur.bhsNo = c
				hasBHS = true
			} else if strings.Contains(h, "descriptioncn") || strings.Contains(h, "中文") || h == "零件名称中文" {
				cur.descCN = c
			} else if strings.Contains(h, "descriptionen") || strings.Contains(h, "英文") || h == "零件名称英文" {
				cur.descEN = c
			} else if strings.Contains(h, "description") || strings.Contains(h, "零件名称") || h == "名称" {
				cur.descCombo = c
			} else if strings.Contains(h, "specification") || strings.Contains(h, "型号") || strings.Contains(h, "规格") || h == "type" {
				cur.spec = c
			} else if strings.Contains(h, "manufacture") || strings.Contains(h, "品牌") || strings.Contains(h, "供应商") || strings.Contains(h, "厂商") {
				cur.mfr = c
			} else if strings.Contains(h, "idno") || strings.Contains(h, "订货号") || strings.Contains(h, "orderno") {
				cur.orderNo = c
			} else if strings.Contains(h, "unit") || strings.Contains(h, "单位") {
				cur.unit = c
			} else if strings.Contains(h, "dispo") || strings.Contains(h, "属性") {
				cur.dispo = c
			} else if strings.Contains(h, "cert") || strings.Contains(h, "认证") {
				cur.cert = c
			} else if strings.Contains(h, "machine") || strings.Contains(h, "机型") || strings.Contains(h, "功能") {
				cur.machine = c
			} else if strings.Contains(h, "remark") || strings.Contains(h, "备注") {
				cur.remark = c
			} else if strings.Contains(h, "import") || strings.Contains(h, "whr") || strings.Contains(h, "进口") {
				cur.importFlag = c
			} else if strings.Contains(h, "price") || strings.Contains(h, "价格") || strings.Contains(h, "单价") {
				cur.price = c
			} else if strings.Contains(h, "spare") || strings.Contains(h, "备件") {
				cur.spareLevel = c
			} else if strings.Contains(h, "active") || strings.Contains(h, "有效") {
				cur.activeFlag = c
			} else if strings.Contains(h, "alternate") || strings.Contains(h, "旧零件") || strings.Contains(h, "gno") {
				cur.alternateNo = c
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

func syncExcelMasterlist(ctx context.Context, store *Store, exeDir string, paths []string, dryRun bool) (*SyncResult, error) {
	res := &SyncResult{DryRun: dryRun}
	now := time.Now().Format(time.RFC3339)

	// Collect unique files
	scannedFiles := map[string]string{}
	for _, p := range paths {
		targetDir := p
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(exeDir, targetDir)
		}
		fi, err := os.Stat(targetDir)
		if err != nil || !fi.IsDir() {
			continue
		}
		entries, err := os.ReadDir(targetDir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			name := entry.Name()
			if strings.HasPrefix(name, "~$") || !strings.HasSuffix(strings.ToLower(name), ".xlsx") {
				continue
			}
			if _, seen := scannedFiles[name]; !seen {
				scannedFiles[name] = filepath.Join(targetDir, name)
			}
		}
	}

	if len(scannedFiles) == 0 {
		res.SummaryMessage = "未检索到 Excel 原表文件，请检查原表路径设置。"
		return res, nil
	}

	res.FilesScanned = len(scannedFiles)

	for fileName, fullPath := range scannedFiles {
		category := categoryFromFilename(fileName)
		f, err := excelize.OpenFile(fullPath)
		if err != nil {
			res.Errors = append(res.Errors, fmt.Sprintf("%s: 打开失败: %v", fileName, err))
			continue
		}

		sheets := f.GetSheetList()
		for _, sheet := range sheets {
			rows, err := f.GetRows(sheet)
			if err != nil || len(rows) == 0 {
				continue
			}
			res.SheetsScanned++

			headerIdx, m := detectHeaders(rows)
			if headerIdx < 0 || m.bhsNo < 0 {
				continue
			}

			// Ensure source_sheet is recorded
			if !dryRun {
				_, _ = store.db.Execute(ctx, `
					INSERT OR IGNORE INTO source_sheets (source_file, source_sheet, category)
					VALUES (?, ?, ?)
				`, fileName, sheet, category)
			}

			for r := headerIdx + 1; r < len(rows); r++ {
				row := rows[r]
				bhsNo := getCell(row, m.bhsNo)
				bhsNo = strings.ReplaceAll(bhsNo, " ", "")
				if bhsNo == "" {
					continue
				}

				descCN := getCell(row, m.descCN)
				descEN := getCell(row, m.descEN)
				if descCN == "" && descEN == "" && m.descCombo >= 0 {
					combo := getCell(row, m.descCombo)
					descCN = combo
				}

				spec := getCell(row, m.spec)
				mfr := getCell(row, m.mfr)
				orderNo := getCell(row, m.orderNo)
				unit := getCell(row, m.unit)
				if unit == "" {
					unit = "个"
				}
				dispo := getCell(row, m.dispo)
				cert := getCell(row, m.cert)
				machine := getCell(row, m.machine)
				remark := getCell(row, m.remark)
				importFlag := getCell(row, m.importFlag)
				price := getCell(row, m.price)
				spareLevel := getCell(row, m.spareLevel)
				activeFlag := getCell(row, m.activeFlag)
				alternateNo := getCell(row, m.alternateNo)
				sourceRow := int64(r + 1)

				status := "active"
				if strings.Contains(strings.ToLower(sheet), "inactive") ||
					strings.Contains(strings.ToLower(category), "inactive") ||
					strings.EqualFold(activeFlag, "no") || strings.EqualFold(activeFlag, "0") {
					status = "inactive"
				}

				// Check existing in DB
				qRows, err := store.db.Query(ctx, `
					SELECT id, origin, bhs_no, spec, description_cn, manufacturer, order_no
					FROM parts
					WHERE source_file = ? AND source_sheet = ? AND source_row = ?
					LIMIT 1
				`, fileName, sheet, sourceRow)

				var existingID int64 = 0
				var existingOrigin string
				var oldBHS, oldSpec, oldDescCN, oldMfr, oldOrder string

				if err == nil && len(qRows.Values) > 0 {
					idx := columnIndex(qRows.Columns)
					vals := qRows.Values[0]
					existingID = intValue(vals, idx, "id")
					existingOrigin = stringValue(vals, idx, "origin")
					oldBHS = stringValue(vals, idx, "bhs_no")
					oldSpec = stringValue(vals, idx, "spec")
					oldDescCN = stringValue(vals, idx, "description_cn")
					oldMfr = stringValue(vals, idx, "manufacturer")
					oldOrder = stringValue(vals, idx, "order_no")
				} else {
					// Fallback query by bhs_no + spec + source_file
					qRows2, err2 := store.db.Query(ctx, `
						SELECT id, origin, bhs_no, spec, description_cn, manufacturer, order_no
						FROM parts
						WHERE bhs_no = ? AND spec = ? AND source_file = ?
						LIMIT 1
					`, bhsNo, spec, fileName)
					if err2 == nil && len(qRows2.Values) > 0 {
						idx := columnIndex(qRows2.Columns)
						vals := qRows2.Values[0]
						existingID = intValue(vals, idx, "id")
						existingOrigin = stringValue(vals, idx, "origin")
						oldBHS = stringValue(vals, idx, "bhs_no")
						oldSpec = stringValue(vals, idx, "spec")
						oldDescCN = stringValue(vals, idx, "description_cn")
						oldMfr = stringValue(vals, idx, "manufacturer")
						oldOrder = stringValue(vals, idx, "order_no")
					}
				}

				if existingID > 0 {
					if existingOrigin == "manual" {
						res.ManualPreserved++
						continue
					}
					// Check changed
					if oldBHS != bhsNo || oldSpec != spec || oldDescCN != descCN || oldMfr != mfr || oldOrder != orderNo {
						res.UpdatedParts++
						if !dryRun {
							p := &Part{
								ID:            existingID,
								BHSNo:         bhsNo,
								DescriptionCN: descCN,
								DescriptionEN: descEN,
								Spec:          spec,
								Manufacturer:  mfr,
								OrderNo:       orderNo,
								Category:      category,
								Subcategory:   sheet,
								Unit:          unit,
								Dispo:         dispo,
								Cert:          cert,
								Machine:       machine,
								ImportFlag:    importFlag,
								Remark:        remark,
								Price:         price,
								SpareLevel:    spareLevel,
								ActiveFlag:    activeFlag,
								AlternateNo:   alternateNo,
								Status:        status,
								Origin:        "import",
								SourceFile:    fileName,
								SourceSheet:   sheet,
								SourceRow:     sourceRow,
							}
							p.SearchText = partSearchText(p)
							_, _ = store.db.Execute(ctx, `
								UPDATE parts SET
									bhs_no = ?, description_cn = ?, description_en = ?, spec = ?,
									manufacturer = ?, order_no = ?, category = ?, subcategory = ?,
									unit = ?, dispo = ?, cert = ?, machine = ?, import_flag = ?,
									remark = ?, price = ?, spare_level = ?, active_flag = ?,
									alternate_no = ?, status = ?, search_text = ?, source_row = ?,
									updated_at = ?
								WHERE id = ?
							`,
								bhsNo, descCN, descEN, spec,
								mfr, orderNo, category, sheet,
								unit, dispo, cert, machine, importFlag,
								remark, price, spareLevel, activeFlag,
								alternateNo, status, p.SearchText, sourceRow,
								now, existingID,
							)
						}
					} else {
						res.UnchangedParts++
					}
				} else {
					// Insert new part
					res.NewPartsAdded++
					if !dryRun {
						p := &Part{
							BHSNo:         bhsNo,
							DescriptionCN: descCN,
							DescriptionEN: descEN,
							Spec:          spec,
							Manufacturer:  mfr,
							OrderNo:       orderNo,
							Category:      category,
							Subcategory:   sheet,
							Unit:          unit,
							Dispo:         dispo,
							Cert:          cert,
							Machine:       machine,
							ImportFlag:    importFlag,
							Remark:        remark,
							Price:         price,
							SpareLevel:    spareLevel,
							ActiveFlag:    activeFlag,
							AlternateNo:   alternateNo,
							Status:        status,
							Origin:        "import",
							SourceFile:    fileName,
							SourceSheet:   sheet,
							SourceRow:     sourceRow,
							CreatedAt:     now,
							UpdatedAt:     now,
						}
						p.SearchText = partSearchText(p)
						_, _ = store.db.Execute(ctx, `
							INSERT INTO parts (
								bhs_no, description_cn, description_en, spec, manufacturer,
								order_no, category, subcategory, unit, dispo, cert, machine,
								import_flag, remark, price, spare_level, active_flag,
								alternate_no, status, origin, search_text, source_file,
								source_sheet, source_row, created_at, updated_at
							) VALUES (
								?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?
							)
						`,
							bhsNo, descCN, descEN, spec, mfr,
							orderNo, category, sheet, unit, dispo, cert, machine,
							importFlag, remark, price, spareLevel, activeFlag,
							alternateNo, status, "import", p.SearchText, fileName,
							sheet, sourceRow, now, now,
						)
					}
				}
			}
		}
		_ = f.Close()
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

	if dryRun {
		res.SummaryMessage = fmt.Sprintf("检测完成：已扫描 %d 份 Excel 表格（%d 个工作表），检测到新增 %d 件器件，更新 %d 件，无变更 %d 件，已保护手工维护 %d 件。",
			res.FilesScanned, res.SheetsScanned, res.NewPartsAdded, res.UpdatedParts, res.UnchangedParts, res.ManualPreserved)
	} else {
		res.SummaryMessage = fmt.Sprintf("同步成功：已同步 %d 份 Excel 表格（%d 个工作表），成功入库新增 %d 件器件，更新 %d 件，已保护手工维护 %d 件。",
			res.FilesScanned, res.SheetsScanned, res.NewPartsAdded, res.UpdatedParts, res.ManualPreserved)
	}

	return res, nil
}
