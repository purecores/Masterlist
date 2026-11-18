package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

func TestPDFScannerRecursiveAndCache(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "manuals")
	sub := filepath.Join(root, "2024", "breakers")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, f := range []string{
		filepath.Join(root, "100200300_A.pdf"),
		filepath.Join(sub, "100200301_B.pdf"),
		filepath.Join(sub, "notes.txt"),
	} {
		if err := os.WriteFile(f, []byte("pdf"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	scanner := NewPDFScanner(tmp, []string{"manuals"})
	scanner.Scan()
	if scanner.Count() != 2 {
		t.Fatalf("expected 2 recursive PDFs, got %d", scanner.Count())
	}
	if !scanner.HasIndex() || scanner.FromCache() {
		t.Fatalf("expected a fresh persisted index, cache=%v index=%v", scanner.FromCache(), scanner.HasIndex())
	}
	for _, st := range scanner.Status() {
		if !st.Exists || st.Count != 2 {
			t.Errorf("status %+v expected to exist with 2 files", st)
		}
	}

	// A new scanner over the same roots reads the index instead of walking.
	again := NewPDFScanner(tmp, []string{"manuals"})
	if !again.HasIndex() || !again.FromCache() {
		t.Fatalf("expected the second scanner to load the cached index")
	}
	if again.Count() != 2 {
		t.Errorf("cached index has %d files, want 2", again.Count())
	}
	if got := again.FindForBHS("100200301"); len(got) != 1 {
		t.Errorf("cached index lost the nested file: %d matches", len(got))
	}
}

func TestPDFScannerOfflineRootKeepsCache(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "manuals")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "100300400_A.pdf"), []byte("pdf"), 0o644); err != nil {
		t.Fatal(err)
	}
	share := filepath.Join(tmp, "gone-share") // never created: an offline root
	scanner := NewPDFScanner(tmp, []string{"manuals", share})
	scanner.Scan()
	if scanner.Count() != 1 {
		t.Fatalf("expected 1 file, got %d", scanner.Count())
	}

	// Remove the local root: everything is unreachable now, and the scan
	// must keep the cached entries instead of emptying the library.
	if err := os.RemoveAll(root); err != nil {
		t.Fatal(err)
	}
	scanner.Scan()
	if scanner.Count() != 1 {
		t.Fatalf("a scan that reached no root dropped the cache: %d files left", scanner.Count())
	}
	statuses := scanner.Status()
	if len(statuses) != 2 || statuses[0].Exists {
		t.Errorf("unexpected statuses after offline scan: %+v", statuses)
	}
}

func TestPDFScannerSyncFileListMerge(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "manuals")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	first := filepath.Join(root, "100400500_A.pdf")
	if err := os.WriteFile(first, []byte("pdf-one"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanner := NewPDFScanner(tmp, []string{"manuals"})
	scanner.Scan()

	second := filepath.Join(root, "100400501_B.pdf")
	if err := os.WriteFile(second, []byte("pdf-two"), 0o644); err != nil {
		t.Fatal(err)
	}
	added, removed, kept, err := scanner.SyncFileList()
	if err != nil {
		t.Fatal(err)
	}
	if added != 1 || removed != 0 || kept != 0 {
		t.Errorf("sync added=%d removed=%d kept=%d, want 1/0/0", added, removed, kept)
	}
	if scanner.Count() != 2 {
		t.Errorf("sync left %d files, want 2", scanner.Count())
	}
	if got := scanner.FindForBHS("100400501"); len(got) != 1 || got[0].Name != "100400501_B.pdf" {
		t.Errorf("new file not matchable: %+v", got)
	}
}

func TestPDFScannerDedupesSameSize(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "manuals")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatal(err)
	}
	// Same number, same size: one document found twice.
	body := []byte("identical-bytes")
	for _, name := range []string{"100500600_EN.pdf", "100500600_CN.pdf"} {
		if err := os.WriteFile(filepath.Join(root, name), body, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(root, "100500600_full.pdf"), []byte("longer manual body"), 0o644); err != nil {
		t.Fatal(err)
	}
	scanner := NewPDFScanner(tmp, []string{"manuals"})
	scanner.Scan()
	matches := scanner.FindForBHS("100500600")
	if len(matches) != 2 {
		t.Fatalf("expected 2 distinct manuals after size dedupe, got %d (%+v)", len(matches), matches)
	}
	if hidden := scanner.DedupedGroupCount("100500600"); hidden != 1 {
		t.Errorf("expected 1 hidden duplicate, got %d", hidden)
	}
}

func TestSortAndFilter(t *testing.T) {
	a := newApp("data/masterlist.db", "", false)
	a.loading = false
	a.summary = Summary{Total: 3}
	a.parts = []*Part{
		{ID: 1, BHSNo: "B002", DescriptionCN: "端子", Manufacturer: "WAGO", Category: "Accessories", Spec: "2.5mm", Status: "active", SourceFile: "A.xlsx", SourceSheet: "S1", SourceRow: 1},
		{ID: 2, BHSNo: "A001", DescriptionCN: "电源", Manufacturer: "Siemens", Category: "DC Power Supply Unit", Spec: "24V", Status: "inactive", SourceFile: "A.xlsx", SourceSheet: "S1", SourceRow: 2},
		{ID: 3, BHSNo: "C003", DescriptionCN: "风扇", Manufacturer: "WAGO", Category: "Fan", Spec: "12V", Status: "active", SourceFile: "B.xlsx", SourceSheet: "S2", SourceRow: 3},
	}
	a.refreshResults(0)
	if len(a.results) != 3 {
		t.Fatalf("want 3 results, got %d", len(a.results))
	}

	// Sorting by BHS number, both directions.
	a.setSort("零件号")
	if a.results[0].BHSNo != "A001" {
		t.Errorf("ascending BHS sort starts with %s", a.results[0].BHSNo)
	}
	a.setSortDesc(true)
	if a.results[0].BHSNo != "C003" {
		t.Errorf("descending BHS sort starts with %s", a.results[0].BHSNo)
	}

	// Filtering by manufacturer keeps the WAGO parts.
	a.setSortDesc(false)
	a.filterMfr = "WAGO"
	a.refreshResults(0)
	if len(a.results) != 2 {
		t.Fatalf("manufacturer filter gave %d results, want 2", len(a.results))
	}
	a.filterMfr = ""

	// Filtering by category uses the stored category name.
	a.filterCategory = "Fan"
	a.refreshResults(0)
	if len(a.results) != 1 || a.results[0].ID != 3 {
		t.Fatalf("category filter gave %+v", a.results)
	}
	a.filterCategory = ""

	// Status filter: inactive only ("停用" is index 2 of filterStatusOptions).
	a.filterStatus = 2
	a.refreshResults(0)
	if len(a.results) != 1 || a.results[0].Status != "inactive" {
		t.Fatalf("status filter gave %+v", a.results)
	}
	a.clearFilters()
	if len(a.results) != 3 || a.filterActive() {
		t.Fatalf("clearFilters left %d results, active=%v", len(a.results), a.filterActive())
	}
}

func TestSearchTokenSynonyms(t *testing.T) {
	tokens := expandQueryToken("断路器")
	if len(tokens) < 2 {
		t.Fatalf("expected synonyms for 断路器, got %v", tokens)
	}
	if slices.Contains(expandQueryToken("100917800"), "breaker") {
		t.Errorf("a plain part number must not gain synonyms")
	}
}

func TestSettingsPersistToggles(t *testing.T) {
	tmp := t.TempDir()
	cfg := defaultConfig()
	cfg.TrayEnabled = true
	cfg.CloseToTray = true
	cfg.ReadOnlyPDF = true
	cfg.SortMode = SortName
	cfg.SortDesc = true
	cfg.ManufacturerFilter = "WAGO"
	if err := saveConfig(tmp, cfg); err != nil {
		t.Fatal(err)
	}
	loaded := loadConfig(tmp)
	if !loaded.TrayEnabled || !loaded.CloseToTray || !loaded.ReadOnlyPDF {
		t.Errorf("booleans lost on reload: %+v", loaded)
	}
	if loaded.SortMode != SortName || !loaded.SortDesc || loaded.ManufacturerFilter != "WAGO" {
		t.Errorf("list options lost on reload: %+v", loaded)
	}

	// A config written before these keys existed keeps the tray on.
	legacy := []byte(`{"theme":"light","datasheet_paths":["pdfs"]}`)
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), legacy, 0o644); err != nil {
		t.Fatal(err)
	}
	legacyCfg := loadConfig(tmp)
	if !legacyCfg.TrayEnabled || !legacyCfg.CloseToTray {
		t.Errorf("legacy config turned the tray off: %+v", legacyCfg)
	}
	if legacyCfg.Theme != "light" || len(legacyCfg.DatasheetPaths) != 1 {
		t.Errorf("legacy config not read: %+v", legacyCfg)
	}

	// An explicit false in the file is respected.
	explicit := []byte(`{"tray_enabled":false,"close_to_tray":false}`)
	if err := os.WriteFile(filepath.Join(tmp, "config.json"), explicit, 0o644); err != nil {
		t.Fatal(err)
	}
	offCfg := loadConfig(tmp)
	if offCfg.TrayEnabled || offCfg.CloseToTray {
		t.Errorf("explicit false ignored: %+v", offCfg)
	}
}

func TestReadOnlyCopy(t *testing.T) {
	tmp := t.TempDir()
	src := filepath.Join(tmp, "100600700_manual.pdf")
	body := []byte("%PDF-1.4 test body")
	if err := os.WriteFile(src, body, 0o644); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(tmp, "prog")
	copyPath, err := copyReadOnlyPDF(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	if copyPath == src {
		t.Fatalf("read-only mode must open a copy, not the library file")
	}
	copied, err := os.ReadFile(copyPath)
	if err != nil {
		t.Fatal(err)
	}
	if string(copied) != string(body) {
		t.Errorf("the copy differs from the source")
	}
	if !strings.Contains(filepath.Base(copyPath), "只读") {
		t.Errorf("copy name %q should say it is read-only", filepath.Base(copyPath))
	}

	// Opening again reuses the copy instead of copying once more.
	again, err := copyReadOnlyPDF(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	if again != copyPath {
		t.Errorf("second copy path %s differs from %s", again, copyPath)
	}

	// A changed source produces a fresh copy.
	if err := os.WriteFile(src, append(body, []byte(" v2")...), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(src, time.Now().Add(time.Minute), time.Now().Add(time.Minute)); err != nil {
		t.Fatal(err)
	}
	updated, err := copyReadOnlyPDF(dir, src)
	if err != nil {
		t.Fatal(err)
	}
	if updated == copyPath {
		t.Errorf("a changed source reused the stale copy")
	}
}

func TestResolveScanRootNetworkAndVars(t *testing.T) {
	tmp := t.TempDir()
	s := NewPDFScanner(tmp, nil)
	if got := s.resolveScanRoot(`\\server\share\pdf`); got != `\\server\share\pdf` {
		t.Errorf("UNC path resolved to %q", got)
	}
	if got := s.resolveScanRoot(`Z:\manuals`); got != `Z:\manuals` {
		t.Errorf("mapped drive resolved to %q", got)
	}
	t.Setenv("BHS_PDF_ROOT", filepath.Join(tmp, "root"))
	if got := s.resolveScanRoot(`%BHS_PDF_ROOT%\sub`); got != filepath.Join(tmp, "root", "sub") {
		t.Errorf("env var path resolved to %q", got)
	}
}
