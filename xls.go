package main

import (
	"fmt"
	"strings"

	xls "github.com/shakinm/xlsReader/xls"
	"github.com/xuri/excelize/v2"
)

// workbook is the common view the sync uses of an Excel file, whatever its
// format: sheet name and its rows as text.
type workbookReader struct {
	excel  *excelize.File
	xls    *xls.Workbook
	sheets []worksheetData
	err    string
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
	wb := &workbookReader{excel: f}
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
