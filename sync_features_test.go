package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	xls "github.com/shakinm/xlsReader/xls"
	"github.com/xuri/excelize/v2"
)

// writeXLSX builds a small workbook at the given path, one sheet per entry.
func writeXLSX(t *testing.T, path string, sheets map[string][][]any) {
	t.Helper()
	f := excelize.NewFile()
	first := true
	for name, rows := range sheets {
		if first {
			if err := f.SetSheetName("Sheet1", name); err != nil {
				t.Fatal(err)
			}
			first = false
		} else if _, err := f.NewSheet(name); err != nil {
			t.Fatal(err)
		}
		for i, row := range rows {
			cell, _ := excelize.CoordinatesToCellName(1, i+1)
			if err := f.SetSheetRow(name, cell, &row); err != nil {
				t.Fatal(err)
			}
		}
	}
	if err := f.SaveAs(path); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
}

// openTestStore copies the seed database into a temporary folder, so tests
// start from the shipped schema without touching the repository's file.
func openTestStore(t *testing.T) *Store {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("data", "masterlist.example.db"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "test.db")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := OpenStore(context.Background(), path)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	for _, table := range []string{"parts", "source_sheets", "activity", "import_batches", "meta"} {
		if _, err := store.db.Execute(ctx, "DELETE FROM "+table); err != nil {
			t.Fatal(err)
		}
	}
	t.Cleanup(func() { _ = store.Close() })
	return store
}

// A workbook with a combined Description column keeps the Chinese and
// English names apart, whichever column label the sheet uses.
func TestSyncSplitsCombinedDescription(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Master E-list_Accessories.xlsx")
	writeXLSX(t, path, map[string][][]any{
		"Cable terminal": {
			{"BHS零件号", "零件名称-英文", "零件名称-中文", "型号", "品牌"},
			{"106581800", "End sleeve", "管状预绝缘头", "ET 0.3-6", "ZheTian"},
			{"106582000", "End sleeve", "管状预绝缘头", "E7508", "ZheTian"},
		},
	})
	res, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewPartsAdded != 2 {
		t.Fatalf("expected 2 parts, got %+v", res)
	}
	parts, _, _, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range parts {
		if p.DescriptionCN != "管状预绝缘头" || p.DescriptionEN != "End sleeve" {
			t.Errorf("part %s split wrong: CN=%q EN=%q", p.BHSNo, p.DescriptionCN, p.DescriptionEN)
		}
	}
}

// A sheet that labels the columns the other way round still gets the
// Chinese name in the Chinese field.
func TestSyncSwappedDescriptionColumns(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Master E-list_Switch Cabinet.xlsx")
	writeXLSX(t, path, map[string][][]any{
		"infomation": {
			{"BHS零件号", "零件名称-英文", "零件名称-中文", "型号"},
			{"143655390", "主电箱框架", "Switch Cabinet", "B1000/H2000/T500"},
		},
	})
	if _, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, false); err != nil {
		t.Fatal(err)
	}
	parts, _, _, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part, got %d", len(parts))
	}
	if parts[0].DescriptionCN != "主电箱框架" || parts[0].DescriptionEN != "Switch Cabinet" {
		t.Errorf("swapped columns not repaired: CN=%q EN=%q", parts[0].DescriptionCN, parts[0].DescriptionEN)
	}
}

// A row with two number/name pairs (a relay kit and its components) imports
// both parts, sharing the row's spec and brand.
func TestSyncTwoPartsPerRow(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	path := filepath.Join(dir, "Master E-list_Relay&Contactor.xlsx")
	writeXLSX(t, path, map[string][][]any{
		"relay": {
			{"BHS零件号(S)", "零件名称", "BHS零件号(K)", "零件名称", "系统型号", "品牌"},
			{"111537716", "1CO继电器套装", "111537821", "继电器座", "95.05", "Finder"},
		},
	})
	if _, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, false); err != nil {
		t.Fatal(err)
	}
	parts, _, _, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 2 {
		t.Fatalf("expected the kit and its component, got %d parts: %+v", len(parts), parts)
	}
	found := map[string]bool{}
	for _, p := range parts {
		found[p.BHSNo] = true
		if p.Spec != "95.05" || p.Manufacturer != "Finder" {
			t.Errorf("part %s lost the shared spec/brand: %q %q", p.BHSNo, p.Spec, p.Manufacturer)
		}
	}
	if !found["111537716"] || !found["111537821"] {
		t.Errorf("missing one of the pair: %v", found)
	}
}

// Words such as "Old" mark a historical sheet, but "Fuse Holder" must not
// become historical just because it contains "old".
func TestSyncStatusClassification(t *testing.T) {
	cases := []struct {
		sheet, category, active string
		want                    string
	}{
		{"Old incl. Outscourcing", "Switch Cabinet", "", "obsolete"},
		{"Fuse Holder", "Fuse System", "", "active"},
		{"Inative part number", "Switch Cabinet", "", "inactive"},
		{"Terminal", "Accessories", "No", "inactive"},
		{"Terminal", "Accessories", "1", "active"},
	}
	for _, c := range cases {
		if got := rowStatus(c.sheet, c.category, c.active); got != c.want {
			t.Errorf("rowStatus(%q,%q,%q) = %q, want %q", c.sheet, c.category, c.active, got, c.want)
		}
	}
}

// Full-width text becomes its ASCII form, and the BHS cleaning leaves no
// spacing or scientific notation behind.
func TestSyncCellCleaning(t *testing.T) {
	if got := cleanCellText("ＡＢＣ　１２３"); got != "ABC 123" {
		t.Errorf("cleanCellText = %q", got)
	}
	if got := cleanBHS("1.00918E+08"); got != "100918000" {
		t.Errorf("cleanBHS scientific = %q", got)
	}
	if got := cleanBHS("100 917 800"); got != "100917800" {
		t.Errorf("cleanBHS spaces = %q", got)
	}
	if got := cleanBHS("?"); got != "" {
		t.Errorf("cleanBHS placeholder = %q", got)
	}
}

// Only Master E-list_ workbooks import; helper spreadsheets do not become
// categories, and the run reports how many it skipped.
func TestSyncSkipsHelperWorkbooks(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	writeXLSX(t, filepath.Join(dir, "Master E-list_Accessories.xlsx"), map[string][][]any{
		"Terminal": {
			{"BHS零件号", "零件名称", "型号"},
			{"166325830", "端子", "2002-6301"},
		},
	})
	writeXLSX(t, filepath.Join(dir, "IE3 to IE4 transform list.xlsx"), map[string][][]any{
		"Sheet1": {
			{"BHS零件号", "型号"},
			{"100000001", "X"},
		},
	})
	res, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesScanned != 1 || res.FilesSkipped != 1 {
		t.Fatalf("scan/skip = %d/%d, want 1/1", res.FilesScanned, res.FilesSkipped)
	}
	parts, books, _, err := store.LoadAll(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 || len(books) != 1 {
		t.Fatalf("helper workbook leaked into the library: %d parts, %d books", len(parts), len(books))
	}
	if books[0].Category != "Accessories" {
		t.Errorf("category = %q", books[0].Category)
	}
}

// A second sync of the same files changes nothing, so the counter of
// updates tells the truth.
func TestSyncIsIdempotent(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	writeXLSX(t, filepath.Join(dir, "Master E-list_Accessories.xlsx"), map[string][][]any{
		"Terminal": {
			{"BHS零件号", "零件名称", "型号", "品牌"},
			{"166325830", "端子", "2002-6301", "WAGO"},
			{"166325810", "盖板", "2002-6391", "WAGO"},
		},
	})
	ctx := context.Background()
	if _, err := syncExcelMasterlist(ctx, store, dir, []string{"."}, false); err != nil {
		t.Fatal(err)
	}
	res, err := syncExcelMasterlist(ctx, store, dir, []string{"."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewPartsAdded != 0 || res.UpdatedParts != 0 || res.UnchangedParts != 2 {
		t.Fatalf("second sync new=%d updated=%d unchanged=%d, want 0/0/2",
			res.NewPartsAdded, res.UpdatedParts, res.UnchangedParts)
	}
}

// A spec edited in place updates the row instead of adding a second copy of
// the same number.
func TestSyncUpdatesSpecInPlace(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "Master E-list_Accessories.xlsx")
	writeXLSX(t, file, map[string][][]any{
		"Cable terminal": {
			{"BHS零件号", "零件名称", "型号", "品牌"},
			{"106581800", "管状预绝缘头", "ET 0.3-6 0.25mm²", "ZheTian"},
		},
	})
	ctx := context.Background()
	if _, err := syncExcelMasterlist(ctx, store, dir, []string{"."}, false); err != nil {
		t.Fatal(err)
	}
	// The same row, with the spec normalized in the file ("²" → "2").
	writeXLSX(t, file, map[string][][]any{
		"Cable terminal": {
			{"BHS零件号", "零件名称", "型号", "品牌"},
			{"106581800", "管状预绝缘头", "ET 0.3-6 0.25mm2", "ZheTian"},
		},
	})
	res, err := syncExcelMasterlist(ctx, store, dir, []string{"."}, false)
	if err != nil {
		t.Fatal(err)
	}
	if res.NewPartsAdded != 0 || res.UpdatedParts != 1 {
		t.Fatalf("spec change: new=%d updated=%d, want 0/1", res.NewPartsAdded, res.UpdatedParts)
	}
	parts, _, _, err := store.LoadAll(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(parts) != 1 {
		t.Fatalf("expected 1 part after the spec change, got %d", len(parts))
	}
	if parts[0].Spec != "ET 0.3-6 0.25mm2" {
		t.Errorf("spec = %q", parts[0].Spec)
	}
}

// An old-format .xls workbook imports through the same pipeline as .xlsx.
func TestSyncReadsXLS(t *testing.T) {
	store := openTestStore(t)
	dir := t.TempDir()
	// Build the .xls through the fixture shipped with the xls reader.
	src := filepath.Join(moduleDir(t, "github.com/shakinm/xlsReader"), "testfie", "table.xls")
	if _, err := os.Stat(src); err != nil {
		t.Skip("fixture not available:", err)
	}
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "Master E-list_Moeller.xls")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	book, err := xls.OpenFile(path)
	if err != nil {
		t.Fatal(err)
	}
	_ = book
	if _, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, true); err != nil {
		t.Fatal(err)
	}
	// The fixture has no BHS columns, so only opening it is asserted here.
	if res, err := syncExcelMasterlist(context.Background(), store, dir, []string{"."}, false); err != nil {
		t.Fatal(err)
	} else if res.FilesScanned != 1 {
		t.Fatalf("the .xls was not scanned: %+v", res)
	}
}

func moduleDir(t *testing.T, module string) string {
	t.Helper()
	out, err := exec.Command("go", "list", "-m", "-f", "{{.Dir}}", module).Output()
	if err != nil {
		t.Skip("module not in the build list:", err)
	}
	return filepath.Clean(strings.TrimSpace(string(out)))
}
