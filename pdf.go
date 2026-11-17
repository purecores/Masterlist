package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"sync"
	"time"
)

// pdfIndexVersion changes when the on-disk index layout changes; an index of
// another version is ignored and rebuilt.
const pdfIndexVersion = 2

// PDFFile is one datasheet PDF found by the scanner.
type PDFFile struct {
	Name     string `json:"name"`
	FullPath string `json:"full_path"`
	Size     int64  `json:"size"`
	// ModTime is the file's modification time in Unix nanoseconds, 0 when
	// the entry's metadata could not be read.
	ModTime int64  `json:"mod_time"`
	Dir     string `json:"dir"`
	// Root is the configured directory (resolved) that the file was found
	// under, so a failed network root can keep its cached entries.
	Root string `json:"root"`
}

func (f PDFFile) ModTimeValue() time.Time {
	if f.ModTime <= 0 {
		return time.Time{}
	}
	return time.Unix(0, f.ModTime)
}

type DirScanResult struct {
	ConfiguredPath string `json:"configured_path"`
	ResolvedPath   string `json:"resolved_path"`
	Exists         bool   `json:"exists"`
	Count          int    `json:"count"`
	// Error describes why a directory could not be scanned, when it exists
	// but portions of it were unreadable (offline network share, permissions).
	Error string `json:"error"`
}

type pdfIndexFile struct {
	Version   int             `json:"version"`
	Paths     []string        `json:"paths"`
	ScannedAt string          `json:"scanned_at"`
	Files     []PDFFile       `json:"files"`
	Status    []DirScanResult `json:"status"`
}

// skippedWalkDirs are directories a datasheet scan does not descend into:
// system or development caches that cannot hold the user's datasheets.
var skippedWalkDirs = map[string]bool{
	"$recycle.bin":              true,
	"system volume information": true,
	"node_modules":              true,
	"__pycache__":               true,
	".git":                      true,
	".svn":                      true,
	".venv":                     true,
	"venv":                      true,
}

const maxPDFFiles = 200000

// PDFScanner indexes PDF datasheets under configured directories, recursively.
// It works with local folders, UNC shares and mapped Windows drives, tolerates
// offline network roots by keeping their last known entries, and persists its
// index so a launch reads local JSON instead of walking a slow network again.
type PDFScanner struct {
	scanMu sync.Mutex // one scan at a time

	mu        sync.RWMutex
	exeDir    string
	indexPath string
	paths     []string

	files      []PDFFile
	nameLower  []string
	status     []DirScanResult
	scannedAt  time.Time
	fromCache  bool
	haveIndex  bool
	scanning   bool
	grams      map[string][]int32
	gen        uint64
	matchCache map[string][]PDFFile
}

func NewPDFScanner(exeDir string, paths []string) *PDFScanner {
	s := &PDFScanner{
		exeDir:     exeDir,
		indexPath:  filepath.Join(exeDir, "data", "pdf-index.json"),
		paths:      slices.Clone(paths),
		grams:      nil,
		matchCache: map[string][]PDFFile{},
	}
	s.loadCache()
	return s
}

// SetPaths replaces the configured roots, loading the cached index if it was
// made for the same roots.
func (s *PDFScanner) SetPaths(paths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	newPaths := slices.Clone(paths)
	if slices.EqualFunc(s.paths, newPaths, strings.EqualFold) {
		return
	}
	s.paths = newPaths
	s.gen++
	s.files = nil
	s.nameLower = nil
	s.status = nil
	s.grams = nil
	s.matchCache = map[string][]PDFFile{}
	s.haveIndex = false
	s.fromCache = false
	go s.loadCache()
}

func (s *PDFScanner) Paths() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.paths)
}

func (s *PDFScanner) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.files)
}

func (s *PDFScanner) Status() []DirScanResult {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.status)
}

func (s *PDFScanner) ScannedAt() time.Time {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scannedAt
}

func (s *PDFScanner) FromCache() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.fromCache
}

// HasIndex reports whether a persisted index was loaded, including an empty
// one: an app with cached roots should not walk a network share on every start.
func (s *PDFScanner) HasIndex() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.haveIndex
}

func (s *PDFScanner) Scanning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.scanning
}

func (s *PDFScanner) AllFiles() []PDFFile {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return slices.Clone(s.files)
}

// SearchPDFs filters the index by a case-insensitive substring of the file
// name or full path.
func (s *PDFScanner) SearchPDFs(query string) []PDFFile {
	s.mu.RLock()
	files := s.files
	names := s.nameLower
	s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return slices.Clone(files)
	}
	matches := make([]PDFFile, 0, 64)
	for i, f := range files {
		if strings.Contains(names[i], q) || strings.Contains(strings.ToLower(f.FullPath), q) {
			matches = append(matches, f)
		}
	}
	return matches
}

// resolveScanRoot resolves a configured path. Relative paths are relative to
// the executable, variables such as %USERPROFILE% are expanded, and UNC paths
// and mapped drives are accepted as they are.
func expandWindowsEnv(p string) string {
	for {
		start := strings.IndexByte(p, '%')
		if start < 0 {
			break
		}
		end := strings.IndexByte(p[start+1:], '%')
		if end < 0 {
			break
		}
		end += start + 1
		name := p[start+1 : end]
		value, ok := os.LookupEnv(name)
		if !ok {
			// An undefined name: keep it literal and scan past it.
			p = p[:start] + "%\x00%" + p[end+1:]
			continue
		}
		p = p[:start] + value + p[end+1:]
	}
	return strings.ReplaceAll(p, "%\x00%", "%")
}

func (s *PDFScanner) resolveScanRoot(p string) string {
	p = strings.TrimSpace(p)
	p = expandWindowsEnv(p)
	p = os.ExpandEnv(p)
	p = strings.Trim(p, `"`)
	if p == "" {
		return ""
	}
	if filepath.IsAbs(p) || strings.HasPrefix(p, `\\`) {
		return filepath.Clean(p)
	}
	return filepath.Clean(filepath.Join(s.exeDir, p))
}

// Scan walks every configured root recursively and replaces the index. Roots
// that are unreachable keep their cached entries, so a flaky network share
// does not empty the datasheet list.
func (s *PDFScanner) Scan() {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return
	}
	s.scanning = true
	paths := slices.Clone(s.paths)
	oldFiles := slices.Clone(s.files)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	var newFiles []PDFFile
	statuses := make([]DirScanResult, 0, len(paths))
	seenPaths := map[string]bool{}
	succeededRoots := map[string]bool{}
	rootOfFailed := map[string]bool{}

	for _, p := range paths {
		resolved := s.resolveScanRoot(p)
		if resolved == "" {
			continue
		}
		st := DirScanResult{ConfiguredPath: p, ResolvedPath: resolved}
		rootFiles, exists, walkErr := s.walkRoot(resolved, seenPaths)
		st.Exists = exists
		st.Count = len(rootFiles)
		if walkErr != "" {
			st.Error = walkErr
		}
		statuses = append(statuses, st)
		if !exists {
			rootOfFailed[resolved] = true
			continue
		}
		succeededRoots[resolved] = true
		newFiles = append(newFiles, rootFiles...)
	}

	// Carry over cached entries of roots that were unreachable this time.
	files := newFiles
	if len(rootOfFailed) > 0 && len(oldFiles) > 0 {
		kept := 0
		for _, f := range oldFiles {
			if rootOfFailed[f.Root] {
				key := strings.ToLower(f.FullPath)
				if seenPaths[key] {
					continue
				}
				seenPaths[key] = true
				files = append(files, f)
				kept++
			}
		}
		if kept > 0 {
			for i := range statuses {
				if rootOfFailed[statuses[i].ResolvedPath] {
					statuses[i].Error = joinErr(statuses[i].Error,
						"网络或磁盘不可用，保留上次索引中的条目")
					// The cached entries belonging to this root.
					for _, f := range oldFiles {
						if f.Root == statuses[i].ResolvedPath {
							statuses[i].Count++
						}
					}
				}
			}
		}
	}

	// A scan that reached nothing while a cache exists is not allowed to
	// wipe the datasheet library.
	if len(succeededRoots) == 0 && len(oldFiles) > 0 {
		s.mu.Lock()
		s.status = statuses
		s.mu.Unlock()
		return
	}

	files = dedupePDFIndex(files)
	sortPDFFiles(files)
	lower := lowerNames(files)
	grams := buildTrigramIndex(lower)

	now := time.Now()
	s.mu.Lock()
	s.files = files
	s.nameLower = lower
	s.status = statuses
	s.scannedAt = now
	s.fromCache = false
	s.haveIndex = true
	s.grams = grams
	s.gen++
	s.matchCache = map[string][]PDFFile{}
	s.mu.Unlock()

	s.saveCache(paths, now, files, statuses)
}

func joinErr(a, b string) string {
	if a == "" {
		return b
	}
	if b == "" {
		return a
	}
	return a + "；" + b
}

func (s *PDFScanner) walkRoot(root string, seenPaths map[string]bool) ([]PDFFile, bool, string) {
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return nil, false, ""
	}

	var files []PDFFile
	errCount := 0
	var firstErr string
	walkErr := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			errCount++
			if firstErr == "" {
				firstErr = err.Error()
			}
			return nil
		}
		if d.IsDir() {
			name := strings.ToLower(d.Name())
			if skippedWalkDirs[name] {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".pdf") {
			return nil
		}
		key := strings.ToLower(path)
		if seenPaths[key] {
			return nil
		}
		seenPaths[key] = true
		var size, modTime int64
		if fi, err := d.Info(); err == nil && !fi.IsDir() {
			size = fi.Size()
			modTime = fi.ModTime().UnixNano()
		}
		files = append(files, PDFFile{
			Name:     d.Name(),
			FullPath: path,
			Size:     size,
			ModTime:  modTime,
			Dir:      filepath.Dir(path),
			Root:     root,
		})
		if len(files) >= maxPDFFiles {
			return errors.New("手册文件数量超过索引上限")
		}
		return nil
	})
	if walkErr != nil && firstErr == "" {
		firstErr = walkErr.Error()
	}
	var note string
	if errCount > 0 || (walkErr != nil && len(firstErr) > 0) {
		note = fmt.Sprintf("%d 个子目录无法访问（网络波动或权限限制）", max(errCount, 1))
		if firstErr != "" {
			note += "：" + firstErr
		}
	}
	return files, true, note
}

// dedupePDFIndex drops the same file found under several configured roots:
// same name (case-insensitive) and same size is treated as one file.
func dedupePDFIndex(files []PDFFile) []PDFFile {
	seen := make(map[string]bool, len(files))
	out := files[:0]
	for _, f := range files {
		key := strings.ToLower(f.Name) + "\x00" + strconv.FormatInt(f.Size, 10)
		if f.Size > 0 {
			if seen[key] {
				continue
			}
			seen[key] = true
		}
		out = append(out, f)
	}
	return out
}

func sortPDFFiles(files []PDFFile) {
	slices.SortFunc(files, func(a, b PDFFile) int {
		if c := strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name)); c != 0 {
			return c
		}
		return strings.Compare(a.FullPath, b.FullPath)
	})
}

func lowerNames(files []PDFFile) []string {
	out := make([]string, len(files))
	for i, f := range files {
		out[i] = strings.ToLower(f.Name)
	}
	return out
}

// trigrams are the keys of the filename index: a query's first three
// characters narrow the candidate files before the full substring check.
func firstTrigram(s string) (string, bool) {
	r := []rune(s)
	if len(r) < 3 {
		return "", false
	}
	return string(r[:3]), true
}

func buildTrigramIndex(names []string) map[string][]int32 {
	grams := make(map[string][]int32, len(names)*4)
	for i, name := range names {
		if len(name) > 96 {
			name = name[:96]
		}
		r := []rune(name)
		if len(r) < 3 {
			continue
		}
		seen := map[string]bool{}
		for j := 0; j+3 <= len(r); j++ {
			g := string(r[j : j+3])
			if seen[g] {
				continue
			}
			seen[g] = true
			grams[g] = append(grams[g], int32(i))
		}
	}
	return grams
}

// FindForBHS returns the datasheets whose filename contains the BHS number.
// Prefix matches sort first. Files of the same byte size are treated as the
// same document and shown once, as the simple duplicate check the library
// needs; the file list shows every physical file.
func (s *PDFScanner) FindForBHS(bhs string) []PDFFile {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(bhs), " ", ""))
	r := []rune(key)
	if len(r) < 3 {
		return nil
	}

	s.mu.RLock()
	if cached, ok := s.matchCache[key]; ok {
		s.mu.RUnlock()
		return cached
	}
	files := s.files
	names := s.nameLower
	grams := s.grams
	gen := s.gen
	s.mu.RUnlock()

	candidates := make([]int32, 0, len(files))
	if grams != nil {
		if g, ok := firstTrigram(key); ok {
			if ids, ok := grams[g]; ok {
				candidates = append(candidates, ids...)
			}
		} else {
			for i := range files {
				candidates = append(candidates, int32(i))
			}
		}
	} else {
		for i := range files {
			candidates = append(candidates, int32(i))
		}
	}

	var matches []PDFFile
	for _, id := range candidates {
		i := int(id)
		if i < 0 || i >= len(files) {
			continue
		}
		if !strings.Contains(names[i], key) {
			continue
		}
		matches = append(matches, files[i])
	}

	// Sort first, so the size-based duplicate check keeps the file whose
	// name starts with the number the user asked for.
	prefix1 := key + "_"
	prefix2 := key + "."
	prefix3 := key + "-"
	slices.SortStableFunc(matches, func(a, b PDFFile) int {
		aLower := strings.ToLower(a.Name)
		bLower := strings.ToLower(b.Name)
		aPrefix := strings.HasPrefix(aLower, prefix1) || strings.HasPrefix(aLower, prefix2) || strings.HasPrefix(aLower, prefix3)
		bPrefix := strings.HasPrefix(bLower, prefix1) || strings.HasPrefix(bLower, prefix2) || strings.HasPrefix(bLower, prefix3)
		if aPrefix != bPrefix {
			if aPrefix {
				return -1
			}
			return 1
		}
		return strings.Compare(aLower, bLower)
	})

	// The simple duplicate rule the library relies on: identical byte size
	// means the same document copied under another name or folder.
	seenSizes := make(map[int64]bool, len(matches))
	out := matches[:0]
	for _, f := range matches {
		if f.Size > 0 {
			if seenSizes[f.Size] {
				continue
			}
			seenSizes[f.Size] = true
		}
		out = append(out, f)
	}
	matches = out

	s.mu.Lock()
	if s.gen == gen {
		if len(s.matchCache) > 4096 {
			s.matchCache = map[string][]PDFFile{}
		}
		s.matchCache[key] = matches
	}
	s.mu.Unlock()
	return matches
}

// HasMatch reports whether the part has at least one datasheet.
func (s *PDFScanner) HasMatch(bhs string) bool {
	return len(s.FindForBHS(bhs)) > 0
}

// DedupedGroupCount reports how many of the matches would be hidden by the
// size-based duplicate check.
func (s *PDFScanner) DedupedGroupCount(bhs string) int {
	key := strings.ToLower(strings.ReplaceAll(strings.TrimSpace(bhs), " ", ""))
	r := []rune(key)
	if len(r) < 3 {
		return 0
	}
	s.mu.RLock()
	names := s.nameLower
	files := s.files
	s.mu.RUnlock()
	all := 0
	sizes := map[int64]bool{}
	for i := range files {
		if !strings.Contains(names[i], key) {
			continue
		}
		all++
		if files[i].Size > 0 {
			sizes[files[i].Size] = true
		}
	}
	shown := len(s.FindForBHS(bhs))
	if all <= shown {
		return 0
	}
	return all - shown
}

func (s *PDFScanner) loadCache() {
	s.mu.RLock()
	indexPath := s.indexPath
	paths := slices.Clone(s.paths)
	s.mu.RUnlock()
	if len(paths) == 0 {
		return
	}
	data, err := os.ReadFile(indexPath)
	if err != nil {
		return
	}
	var idx pdfIndexFile
	if err := json.Unmarshal(data, &idx); err != nil {
		return
	}
	if idx.Version != pdfIndexVersion {
		return
	}
	if len(idx.Paths) != len(paths) || !slices.EqualFunc(idx.Paths, paths, strings.EqualFold) {
		return
	}
	for i := range idx.Files {
		if idx.Files[i].FullPath == "" {
			return
		}
	}
	lower := lowerNames(idx.Files)
	grams := buildTrigramIndex(lower)
	ts, _ := time.Parse(time.RFC3339, idx.ScannedAt)

	s.mu.Lock()
	defer s.mu.Unlock()
	// Another goroutine may have scanned meanwhile.
	if len(s.files) > 0 || s.scanning {
		return
	}
	s.files = idx.Files
	s.nameLower = lower
	s.gen++
	s.status = idx.Status
	if len(s.status) == 0 {
		s.status = statusFromCache(idx.Files, paths)
	}
	s.scannedAt = ts
	s.fromCache = true
	s.haveIndex = true
	s.grams = grams
	s.matchCache = map[string][]PDFFile{}
}

func statusFromCache(files []PDFFile, paths []string) []DirScanResult {
	out := make([]DirScanResult, 0, len(paths))
	for _, p := range paths {
		out = append(out, DirScanResult{ConfiguredPath: p, ResolvedPath: p, Exists: true, Count: len(files)})
	}
	return out
}

func (s *PDFScanner) saveCache(paths []string, at time.Time, files []PDFFile, status []DirScanResult) {
	idx := pdfIndexFile{
		Version:   pdfIndexVersion,
		Paths:     paths,
		ScannedAt: at.Format(time.RFC3339),
		Files:     files,
		Status:    status,
	}
	data, err := json.Marshal(idx)
	if err != nil {
		log.Printf("save pdf index: %v", err)
		return
	}
	if err := os.MkdirAll(filepath.Dir(s.indexPath), 0o755); err != nil {
		log.Printf("save pdf index: %v", err)
		return
	}
	tmp := s.indexPath + ".tmp"
	if err := os.WriteFile(tmp, data, 0o666); err != nil {
		log.Printf("save pdf index: %v", err)
		return
	}
	if err := os.Rename(tmp, s.indexPath); err != nil {
		_ = os.Remove(s.indexPath)
		_ = os.Rename(tmp, s.indexPath)
	}
}

func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatModTime(mod int64) string {
	if mod <= 0 {
		return "-"
	}
	return time.Unix(0, mod).Format("2006.01.02 15:04")
}

// SyncFileList is the manual "sync the file list" action: it rescans only
// when the user asks, merging with the index it holds. A root that fails
// keeps every cached entry, one that succeeds replaces its own entries, and
// the result reports what changed so the user can decide to trust it. This
// is what keeps a flaky network share from emptying the library at launch.
func (s *PDFScanner) SyncFileList() (added, removed, kept int, err error) {
	s.scanMu.Lock()
	defer s.scanMu.Unlock()

	s.mu.Lock()
	if s.scanning {
		s.mu.Unlock()
		return 0, 0, 0, nil
	}
	s.scanning = true
	paths := slices.Clone(s.paths)
	oldFiles := slices.Clone(s.files)
	oldStatus := slices.Clone(s.status)
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.scanning = false
		s.mu.Unlock()
	}()

	oldByRoot := map[string][]PDFFile{}
	for _, f := range oldFiles {
		oldByRoot[f.Root] = append(oldByRoot[f.Root], f)
	}

	statuses := make([]DirScanResult, 0, len(paths))
	var files []PDFFile
	seenPaths := map[string]bool{}
	failures := 0
	var firstFailure string

	for _, p := range paths {
		resolved := s.resolveScanRoot(p)
		if resolved == "" {
			continue
		}
		st := DirScanResult{ConfiguredPath: p, ResolvedPath: resolved}
		rootFiles, exists, walkErr := s.walkRoot(resolved, seenPaths)
		st.Exists = exists
		st.Count = len(rootFiles)
		if walkErr != "" {
			st.Error = walkErr
		}
		if !exists {
			failures++
			if firstFailure == "" {
				firstFailure = resolved
			}
			cached := oldByRoot[resolved]
			st.Error = strings.TrimSpace(joinErr(st.Error, "不可访问，已沿用上次索引"))
			st.Exists = len(cached) > 0
			for _, f := range cached {
				key := strings.ToLower(f.FullPath)
				if seenPaths[key] {
					continue
				}
				seenPaths[key] = true
				files = append(files, f)
				kept++
			}
			st.Count = len(cached)
			statuses = append(statuses, st)
			continue
		}
		files = append(files, rootFiles...)
		statuses = append(statuses, st)
	}

	files = dedupePDFIndex(files)
	sortPDFFiles(files)
	lower := lowerNames(files)
	grams := buildTrigramIndex(lower)

	added, removed = diffPDFFiles(oldFiles, files)
	if failures == len(paths) && len(oldFiles) > 0 {
		err = fmt.Errorf("所有数据手册根目录都不可访问（%s），已保留全部缓存条目", firstFailure)
		return 0, 0, len(oldFiles), err
	}

	now := time.Now()
	s.mu.Lock()
	s.files = files
	s.nameLower = lower
	s.status = statuses
	s.scannedAt = now
	s.fromCache = false
	s.haveIndex = true
	s.grams = grams
	s.gen++
	s.matchCache = map[string][]PDFFile{}
	s.mu.Unlock()
	s.saveCache(paths, now, files, statuses)
	_ = oldStatus
	return added, removed, kept, nil
}

// diffPDFFiles reports how many entries a sync added and removed.
func diffPDFFiles(before, after []PDFFile) (added, removed int) {
	seen := make(map[string]bool, len(after))
	for _, f := range after {
		seen[strings.ToLower(f.FullPath)] = true
	}
	for _, f := range before {
		if !seen[strings.ToLower(f.FullPath)] {
			removed++
		}
	}
	seenBefore := make(map[string]bool, len(before))
	for _, f := range before {
		seenBefore[strings.ToLower(f.FullPath)] = true
	}
	for _, f := range after {
		if !seenBefore[strings.ToLower(f.FullPath)] {
			added++
		}
	}
	return added, removed
}
