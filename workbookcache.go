package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// localWorkbookDir is where the app keeps its own copy of the original
// workbooks. Nothing the app does writes to the configured source folders:
// imports read the local copy, and "open original" opens that copy too, so a
// network share is never locked or edited by Excel.
func localWorkbookDir(exeDir string) string {
	return filepath.Join(exeDir, "data", "workbooks")
}

// localWorkbookPath returns the copy of a source workbook, keyed by its file
// name so a copy from another folder replaces it rather than accumulating.
func localWorkbookPath(exeDir, fileName string) string {
	stem := strings.TrimSuffix(fileName, filepath.Ext(fileName))
	sum := sha256.Sum256([]byte(strings.ToLower(stem)))
	key := hex.EncodeToString(sum[:6])
	// Square brackets are special in Excel's workbook names, so the cache
	// key uses a plain suffix.
	return filepath.Join(localWorkbookDir(exeDir), sanitizeFileName(stem)+" _"+key+filepath.Ext(fileName))
}

// syncLocalWorkbooks copies every Master E-list_ workbook from the source
// folders into the program's own folder. It reports how many were copied or
// refreshed, and never touches the source files. A copy is kept when the
// share is unreachable, so the library still opens and imports.
func syncLocalWorkbooks(exeDir string, files map[string]string) (copied, unchanged, missing int, notes []string) {
	dir := localWorkbookDir(exeDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		notes = append(notes, err.Error())
		return 0, 0, 0, notes
	}

	for _, sourcePath := range files {
		fileName := filepath.Base(sourcePath)
		dst := localWorkbookPath(exeDir, fileName)
		srcInfo, err := os.Stat(sourcePath)
		if err != nil {
			continue
		}
		if dstInfo, err := os.Stat(dst); err == nil &&
			dstInfo.Size() == srcInfo.Size() && !dstInfo.ModTime().Before(srcInfo.ModTime()) {
			unchanged++
			continue
		}
		if err := copyFileAtomic(sourcePath, dst); err != nil {
			missing++
			notes = append(notes, fmt.Sprintf("%s: %v", fileName, err))
			continue
		}
		copied++
	}
	return copied, unchanged, missing, notes
}

func copyFileAtomic(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return copyErr
		}
		return closeErr
	}
	if fi, err := os.Stat(src); err == nil {
		_ = os.Chtimes(tmp, time.Now(), fi.ModTime())
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// localWorkbookFor resolves the local copy of a workbook, falling back to the
// source file when the copy is missing (for example a manual workbook).
func localWorkbookFor(exeDir, fileName string) string {
	if dst := localWorkbookPath(exeDir, fileName); fileExists(dst) {
		return dst
	}
	return ""
}

// hasLocalWorkbooks reports whether the app already holds copies of the
// source tables, so a launch can skip the first import when there are none.
func hasLocalWorkbooks(exeDir string) bool {
	entries, err := os.ReadDir(localWorkbookDir(exeDir))
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasPrefix(strings.ToLower(e.Name()), "master e-list_") {
			return true
		}
	}
	return false
}

func fileExists(path string) bool {
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir()
}

// openWorkbookInExcel opens the local copy with Excel. With the password
// option on, Excel is told the workbook's edit password so a protected sheet
// opens without a prompt. Excel is reached over COM; when no copy exists the
// source file is opened as a last resort, with a note.
type excelOpenResult struct {
	Path    string
	Private bool
	Err     error
}

func openWorkbookInExcel(exeDir, fileName, password string, usePassword bool) excelOpenResult {
	path := localWorkbookFor(exeDir, fileName)
	private := path != ""
	if path == "" {
		// Never fall back to the source: it is the folder the app promises
		// not to touch, and it may be a network share.
		return excelOpenResult{Err: fmt.Errorf("本地还没有这份原表的副本，请先同步原表")}
	}
	if usePassword && password != "" {
		if err := openInExcelWithPassword(path, password); err == nil {
			return excelOpenResult{Path: path, Private: private}
		}
		// COM failed (no Excel, or the object model refused the password);
		// fall back to the shell so the workbook still opens.
	}
	return excelOpenResult{Path: path, Private: private, Err: openPathShell(path)}
}

func openPathShell(path string) error {
	return shellOpenPath(path)
}

// resolveAllExcelPaths lists every configured folder that holds the workbook.
func resolveAllExcelPaths(exeDir, fileName string, paths []string) []string {
	var out []string
	for _, p := range paths {
		dir := resolveConfiguredDir(exeDir, p)
		if dir == "" {
			continue
		}
		candidate := filepath.Join(dir, fileName)
		if fileExists(candidate) {
			out = append(out, candidate)
		}
	}
	return out
}
