package main

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
)

type PDFFile struct {
	Name     string
	FullPath string
	Size     int64
	Dir      string
}

type DirScanResult struct {
	ConfiguredPath string
	ResolvedPath   string
	Exists         bool
	Count          int
}

type PDFScanner struct {
	mu     sync.RWMutex
	exeDir string
	paths  []string
	files  []PDFFile
	status []DirScanResult
}

func NewPDFScanner(exeDir string, paths []string) *PDFScanner {
	return &PDFScanner{
		exeDir: exeDir,
		paths:  slices.Clone(paths),
	}
}

func (s *PDFScanner) SetPaths(paths []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.paths = slices.Clone(paths)
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

func (s *PDFScanner) Scan() {
	s.mu.Lock()
	defer s.mu.Unlock()

	var allFiles []PDFFile
	var statuses []DirScanResult
	seenPaths := make(map[string]bool)

	for _, p := range s.paths {
		trimmed := strings.TrimSpace(p)
		if trimmed == "" {
			continue
		}
		var resolved string
		if filepath.IsAbs(trimmed) {
			resolved = filepath.Clean(trimmed)
		} else {
			resolved = filepath.Clean(filepath.Join(s.exeDir, trimmed))
		}

		info, err := os.Stat(resolved)
		if err != nil || !info.IsDir() {
			statuses = append(statuses, DirScanResult{
				ConfiguredPath: trimmed,
				ResolvedPath:   resolved,
				Exists:         false,
				Count:          0,
			})
			continue
		}

		entries, err := os.ReadDir(resolved)
		if err != nil {
			statuses = append(statuses, DirScanResult{
				ConfiguredPath: trimmed,
				ResolvedPath:   resolved,
				Exists:         true,
				Count:          0,
			})
			continue
		}

		count := 0
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			name := e.Name()
			lower := strings.ToLower(name)
			if strings.HasSuffix(lower, ".pdf") {
				full := filepath.Join(resolved, name)
				if seenPaths[full] {
					continue
				}
				seenPaths[full] = true
				var size int64
				if fi, err := e.Info(); err == nil {
					size = fi.Size()
				}
				allFiles = append(allFiles, PDFFile{
					Name:     name,
					FullPath: full,
					Size:     size,
					Dir:      resolved,
				})
				count++
			}
		}

		statuses = append(statuses, DirScanResult{
			ConfiguredPath: trimmed,
			ResolvedPath:   resolved,
			Exists:         true,
			Count:          count,
		})
	}

	slices.SortFunc(allFiles, func(a, b PDFFile) int {
		return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
	})

	s.files = allFiles
	s.status = statuses
}

func (s *PDFScanner) FindForBHS(bhs string) []PDFFile {
	bhs = strings.TrimSpace(bhs)
	if len(bhs) < 3 {
		return nil
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	bhsLower := strings.ToLower(bhs)
	var matches []PDFFile
	for _, f := range s.files {
		if strings.Contains(strings.ToLower(f.Name), bhsLower) {
			matches = append(matches, f)
		}
	}

	prefix1 := bhsLower + "_"
	prefix2 := bhsLower + "."
	prefix3 := bhsLower + "-"
	slices.SortStableFunc(matches, func(a, b PDFFile) int {
		aLower := strings.ToLower(a.Name)
		bLower := strings.ToLower(b.Name)
		aPrefix := strings.HasPrefix(aLower, prefix1) || strings.HasPrefix(aLower, prefix2) || strings.HasPrefix(aLower, prefix3)
		bPrefix := strings.HasPrefix(bLower, prefix1) || strings.HasPrefix(bLower, prefix2) || strings.HasPrefix(bLower, prefix3)
		if aPrefix && !bPrefix {
			return -1
		}
		if !aPrefix && bPrefix {
			return 1
		}
		return strings.Compare(aLower, bLower)
	})

	return matches
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
