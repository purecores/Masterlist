package main

import (
	"fmt"
	"strconv"
	"strings"

	xls "github.com/shakinm/xlsReader/xls"
	"github.com/xuri/excelize/v2"
)

// workbook is the common view the sync uses of an Excel file, whatever its
// format: sheet name and its rows as text.
type workbookReader struct {
	excel     *excelize.File
	xls       *xls.Workbook
	sheets    []worksheetData
	err       string
	redStyles map[int]bool
}

type worksheetData struct {
	name string
	rows [][]string
}

// openWorkbook reads a workbook of either format. It never fails outright:
// a book that opened with a warning still yields the sheets it could read,
// and the warning rides along so the sync reports it.
func openWorkbook(path string) (*workbookReader, error) {
	if strings.EqualFold(filepathExt(path), ".xls") {
		return openXLSWorkbook(path)
	}
	return openXLSXWorkbook(path)
}

func filepathExt(path string) string {
	if i := strings.LastIndexByte(path, '.'); i >= 0 {
		return path[i:]
	}
	return ""
}

func openXLSXWorkbook(path string) (*workbookReader, error) {
	f, err := excelize.OpenFile(path)
	if err != nil {
		return nil, err
	}
	wb := &workbookReader{excel: f, redStyles: map[int]bool{}}
	for _, name := range f.GetSheetList() {
		rows, err := f.GetRows(name)
		if err != nil {
			if wb.err == "" {
				wb.err = fmt.Sprintf("工作表 %s 读取失败: %v", name, err)
			}
			continue
		}
		wb.sheets = append(wb.sheets, worksheetData{name: name, rows: rows})
	}
	return wb, nil
}

// openXLSWorkbook reads a BIFF8 .xls with the pure-Go reader, so the
// library's older sheets import without Excel or a converter installed.
func openXLSWorkbook(path string) (*workbookReader, error) {
	book, err := xls.OpenFile(path)
	if err != nil {
		return nil, err
	}
	wb := &workbookReader{xls: &book}
	count := book.GetNumberSheets()
	for i := 0; i < count; i++ {
		sheet, err := book.GetSheet(i)
		if err != nil {
			if wb.err == "" {
				wb.err = fmt.Sprintf("第 %d 个工作表读取失败: %v", i+1, err)
			}
			continue
		}
		rows := make([][]string, 0, sheet.GetNumberRows())
		for _, row := range sheet.GetRows() {
			cols := row.GetCols()
			cells := make([]string, len(cols))
			for j, cell := range cols {
				cells[j] = cell.GetString()
			}
			rows = append(rows, cells)
		}
		wb.sheets = append(wb.sheets, worksheetData{name: sheet.GetName(), rows: rows})
	}
	return wb, nil
}

// cellRed reports whether a cell's text or fill is red, the production
// sheets' mark for a discontinued part. Styles are read once per workbook,
// not once per row.
func (w *workbookReader) cellRed(sheet string, row, col int) bool {
	if w == nil || w.excel == nil || row < 1 || col < 1 {
		return false
	}
	cell, err := excelize.CoordinatesToCellName(col, row)
	if err != nil {
		return false
	}
	styleID, err := w.excel.GetCellStyle(sheet, cell)
	if err != nil {
		return false
	}
	if red, ok := w.redStyles[styleID]; ok {
		return red
	}
	style, err := w.excel.GetStyle(styleID)
	if err != nil {
		return false
	}
	red := false
	if style.Font != nil {
		red = isRedColor(style.Font.Color) || style.Font.ColorIndexed == 2
		// newer files store colours in the theme instead of a plain ARGB,
		// so a red theme colour with a plain index also counts
		if !red && style.Font.ColorTheme != nil {
			red = false
		}
	}
	if !red {
		for _, color := range style.Fill.Color {
			if isRedColor(color) {
				red = true
				break
			}
		}
	}
	w.redStyles[styleID] = red
	return red
}

// isRedColor recognises explicit reds, including ARGB values such as
// FFFF0000 and the "#RRGGBB" form of newer files.
func isRedColor(value string) bool {
	value = strings.TrimPrefix(strings.TrimSpace(value), "#")
	if len(value) == 8 {
		// AARRGGBB: drop the alpha byte.
		value = value[2:]
	}
	if len(value) != 6 {
		return false
	}
	r, err1 := strconv.ParseInt(value[0:2], 16, 64)
	g, err2 := strconv.ParseInt(value[2:4], 16, 64)
	b, err3 := strconv.ParseInt(value[4:6], 16, 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return false
	}
	return r >= 0xB0 && g <= 0x80 && b <= 0x80
}

func (w *workbookReader) Close() {
	if w == nil {
		return
	}
	if w.excel != nil {
		_ = w.excel.Close()
		w.excel = nil
	}
	w.xls = nil
}
