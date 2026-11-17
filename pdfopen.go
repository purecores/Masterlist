package main

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/egoist/mygo"
)

// OpenDatasheetPDF opens a datasheet PDF. Normal mode hands the file to the
// associated viewer. Read-only mode never lets a viewer touch the library
// copy: it opens a private read-only duplicate instead, which also survives
// a network share going away after the viewer started.
func (a *app) OpenDatasheetPDF(path string) error {
	if !a.cfg.ReadOnlyPDF {
		return mygo.Shell.OpenPath(path)
	}
	return a.openReadOnly(path)
}

func (a *app) openReadOnly(path string) error {
	if fi, err := os.Stat(path); err != nil || fi.IsDir() {
		if err != nil {
			return err
		}
		return errors.New("PDF 路径无效")
	}

	copied, err := copyReadOnlyPDF(a.exeDir, path)
	if err != nil {
		return fmt.Errorf("创建只读副本失败: %w", err)
	}
	go cleanupReadOnlyCopies(filepath.Dir(copied), 14*24*time.Hour)

	if reader := cleanPDFReaderArgument(a.cfg.PDFReaderPath); reader != "" {
		if err := startDetached(reader, readerArgs(reader, copied)); err != nil {
			return fmt.Errorf("用配置的只读阅读器打开失败: %w", err)
		}
		return nil
	}
	if sumatra := findSumatraPDF(); sumatra != "" {
		// -restrict -ro keeps the duplicate itself from being saved over.
		if err := startDetached(sumatra, []string{"-restrict", "-ro", copied}); err == nil {
			return nil
		}
	}
	return mygo.Shell.OpenPath(copied)
}

// readerArgs gives the configured viewer a read-only hint when it understands
// one; other viewers simply open the duplicate, which is itself read-only.
func readerArgs(reader, path string) []string {
	switch strings.ToLower(filepath.Base(reader)) {
	case "sumatrapdf.exe":
		return []string{"-restrict", "-ro", path}
	default:
		return []string{path}
	}
}

// startDetached starts a program without a console window and without waiting
// for it to exit, so the UI never blocks on a viewer.
func startDetached(exe string, args []string) error {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	go func() {
		_ = cmd.Wait()
	}()
	return nil
}

func findSumatraPDF() string {
	for _, p := range readerCandidates("") {
		if strings.EqualFold(filepath.Base(p), "SumatraPDF.exe") {
			if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
				return p
			}
		}
	}
	return ""
}

// copyReadOnlyPDF copies the document into <program>/cache/readonly, keyed by
// its path, size and modification time, so reopening it does not copy again.
// The copy is marked read-only and 0444.
func copyReadOnlyPDF(exeDir, src string) (string, error) {
	fi, err := os.Stat(src)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s|%d|%d", src, fi.Size(), fi.ModTime().UnixNano())))
	key := hex.EncodeToString(sum[:8])
	base := strings.TrimSuffix(filepath.Base(src), filepath.Ext(src))
	if len(base) > 80 {
		base = base[:80]
	}
	dir := filepath.Join(exeDir, "cache", "readonly")
	dst := filepath.Join(dir, sanitizeFileName(base+" (只读 "+key+").pdf"))
	if out, err := os.Stat(dst); err == nil && out.Size() == fi.Size() {
		return dst, nil
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	in, err := os.Open(src)
	if err != nil {
		return "", err
	}
	defer in.Close()
	tmp := dst + ".tmp"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o666)
	if err != nil {
		return "", err
	}
	_, copyErr := io.Copy(out, in)
	closeErr := out.Close()
	if copyErr != nil || closeErr != nil {
		_ = os.Remove(tmp)
		if copyErr != nil {
			return "", copyErr
		}
		return "", closeErr
	}
	_ = os.Remove(dst)
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return "", err
	}
	_ = setReadOnlyAttribute(dst, true)
	return dst, nil
}

// sanitizeFileName replaces characters Windows file names cannot hold.
func sanitizeFileName(name string) string {
	replacer := strings.NewReplacer(`\`, `_`, `"`, `_`, "*", "_", "?", "_", "<", "_", ">", "_",
		":", "_", "|", "_", "/", "_")
	return strings.TrimSpace(replacer.Replace(name))
}

// setReadOnlyAttribute marks a file read-only (or clears the mark). On
// Windows, Go's Chmod maps the write bit to FILE_ATTRIBUTE_READONLY.
func setReadOnlyAttribute(path string, readOnly bool) error {
	if readOnly {
		return os.Chmod(path, 0o444)
	}
	return os.Chmod(path, 0o666)
}

// cleanupReadOnlyCopies removes duplicates older than age, so the folder does
// not grow without bound.
func cleanupReadOnlyCopies(dir string, age time.Duration) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	cutoff := time.Now().Add(-age)
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(strings.ToLower(e.Name()), ".pdf") {
			continue
		}
		if info, err := e.Info(); err == nil && info.ModTime().Before(cutoff) {
			_ = setReadOnlyAttribute(filepath.Join(dir, e.Name()), false)
			_ = os.Remove(filepath.Join(dir, e.Name()))
		}
	}
}
