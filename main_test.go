package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/xuri/excelize/v2"
)

func TestView(t *testing.T) {
	a := newApp("data/masterlist.db", "", false)
	a.loading = false
	a.summary = Summary{Total: 10, Files: 2, Sheets: 4}
	a.parts = []*Part{
		{ID: 1, BHSNo: "123456", DescriptionCN: "测试端子", Spec: "2.5mm", Manufacturer: "WAGO", Status: "active"},
		{ID: 2, BHSNo: "234567", DescriptionCN: "测试微断", Spec: "16A", Manufacturer: "Siemens", Status: "active"},
	}
	a.books = []WorkbookInfo{
		{
			Category:   "Accessories",
			SourceFile: "Master E-list_Accessories.xlsx",
			Count:      5,
			Sheets:     []SheetInfo{{Name: "Terminal", Count: 3}, {Name: "Rail", Count: 2}},
		},
	}
	a.refreshResults(0)

	tt := ui.NewTester(a.view, 1280, 760)
	if !tt.HasText("电气柜元件库") {
		t.Errorf("texts %q missing header", tt.Texts())
	}
	if !tt.HasText("全部原表") {
		t.Errorf("texts %q missing all sources", tt.Texts())
	}
	if err := tt.Click("+ 新增器件"); err != nil {
		t.Fatal(err)
	}
	if !a.formOpen {
		t.Errorf("expected formOpen to be true after clicking add button")
	}
	a.formOpen = false

	// Test Datasheet button click
	a.openDatasheetSettings()
	if !a.datasheetModalOpen {
		t.Errorf("expected datasheetModalOpen to be true after openDatasheetSettings")
	}
}

func TestPDFScanner(t *testing.T) {
	tmp := t.TempDir()
	pdfDir := filepath.Join(tmp, "pdfs")
	if err := os.MkdirAll(pdfDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create dummy test PDFs
	files := []string{
		"100917800_VNA_EN.pdf",
		"100917800_V1_CN.pdf",
		"OTHER_100917800_DOC.pdf",
		"200000000_VNA_DE.pdf",
		"not_a_pdf.txt",
	}
	for _, f := range files {
		_ = os.WriteFile(filepath.Join(pdfDir, f), []byte("test content"), 0o644)
	}

	scanner := NewPDFScanner(tmp, []string{"pdfs"})
	scanner.Scan()

	if scanner.Count() != 4 {
		t.Errorf("expected 4 PDFs scanned, got %d", scanner.Count())
	}

	matches := scanner.FindForBHS("100917800")
	// The three files of this BHS number have identical byte sizes, so the
	// size-based duplicate check shows only the first of them.
	if len(matches) != 1 {
		t.Fatalf("expected 1 deduplicated match for 100917800, got %d", len(matches))
	}
	if matches[0].Name != "100917800_V1_CN.pdf" && matches[0].Name != "100917800_VNA_EN.pdf" {
		t.Errorf("expected prefix match first, got %s", matches[0].Name)
	}
	if hidden := scanner.DedupedGroupCount("100917800"); hidden != 2 {
		t.Errorf("expected 2 hidden duplicates, got %d", hidden)
	}

	// Case-insensitive test
	matchesLower := scanner.FindForBHS("other_100917800")
	if len(matchesLower) != 1 {
		t.Errorf("expected 1 match for other_100917800, got %d", len(matchesLower))
	}

	// Short BHS should return nil
	if len(scanner.FindForBHS("12")) != 0 {
		t.Errorf("expected 0 matches for short BHS")
	}
}

func TestResolveDatabase(t *testing.T) {
	tmp := t.TempDir()
	dataDir := filepath.Join(tmp, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		t.Fatal(err)
	}
	dbFile := filepath.Join(dataDir, "masterlist.db")
	if err := os.WriteFile(dbFile, []byte("sqlite-placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved, err := resolveDatabase("", tmp)
	if err != nil {
		t.Fatalf("resolveDatabase failed: %v", err)
	}
	if resolved != dbFile {
		t.Errorf("expected %s, got %s", dbFile, resolved)
	}
}

func TestThemeSwitching(t *testing.T) {
	a := newApp("data/masterlist.db", "", false)
	a.lightTheme = false
	darkPal := a.theme()
	if !darkPal.IsDark {
		t.Errorf("expected dark theme when lightTheme is false")
	}

	a.lightTheme = true
	lightPal := a.theme()
	if lightPal.IsDark {
		t.Errorf("expected light theme when lightTheme is true")
	}
}

func TestResolveExcel(t *testing.T) {
	tmp := t.TempDir()
	mlDir := filepath.Join(tmp, "Masterlist")
	if err := os.MkdirAll(mlDir, 0o755); err != nil {
		t.Fatal(err)
	}
	testFile := filepath.Join(mlDir, "Master E-list_Test.xlsx")
	if err := os.WriteFile(testFile, []byte("dummy-content"), 0o644); err != nil {
		t.Fatal(err)
	}

	resolved := resolveExcelPath(tmp, []string{"Masterlist", "other"}, "Master E-list_Test.xlsx")
	if resolved != testFile {
		t.Errorf("expected %s, got %s", testFile, resolved)
	}

	// Non-existent file
	if found := resolveExcelPath(tmp, []string{"Masterlist"}, "non_existent.xlsx"); found != "" {
		t.Errorf("expected empty string for non-existent file, got %s", found)
	}
}

func TestSyncMasterlist(t *testing.T) {
	ctx := context.Background()
	store, err := OpenStore(ctx, "data/masterlist.example.db")
	if err != nil {
		t.Skip("skipping test if db not present:", err)
		return
	}
	defer store.Close()

	tmp := t.TempDir()
	excelPath := filepath.Join(tmp, "Master E-list_SyncTest.xlsx")
	f := excelize.NewFile()
	sheet := "Breaker"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		t.Fatal(err)
	}
	rows := [][]any{
		{"BHS Part No.", "Description CN", "Description EN", "Specification", "Manufacture", "ID No.", "Unit"},
		{"SYNC000001", "同步测试器件", "Sync Test Part", "Test / 1A", "Test Vendor", "TEST-001", "PC"},
	}
	for rowIndex, row := range rows {
		cell, _ := excelize.CoordinatesToCellName(1, rowIndex+1)
		if err := f.SetSheetRow(sheet, cell, &row); err != nil {
			t.Fatal(err)
		}
	}
	if err := f.SaveAs(excelPath); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}

	res, err := syncExcelMasterlist(ctx, store, tmp, []string{"."}, true)
	if err != nil {
		t.Fatal(err)
	}
	if res.FilesScanned != 1 || res.SheetsScanned != 1 || res.NewPartsAdded != 1 {
		t.Errorf("unexpected sync result: %+v", res)
	}
}
