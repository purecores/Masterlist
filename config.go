package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	DatasheetPaths  []string `json:"datasheet_paths"`
	MasterlistPaths []string `json:"masterlist_paths"`
	Theme           string   `json:"theme"` // "dark" or "light"
}

func defaultConfig() Config {
	return Config{
		DatasheetPaths: []string{
			`..\PartPDF_merged`,
			`PartPDF_merged`,
		},
		MasterlistPaths: []string{
			`..\Masterlist`,
			`Masterlist`,
		},
		Theme: "dark",
	}
}

func configPath(exeDir string) string {
	return filepath.Join(exeDir, "config.json")
}

func loadConfig(exeDir string) Config {
	cfg := defaultConfig()
	data, err := os.ReadFile(configPath(exeDir))
	if err != nil {
		return cfg
	}
	var loaded Config
	if err := json.Unmarshal(data, &loaded); err == nil {
		if len(loaded.DatasheetPaths) > 0 {
			var cleaned []string
			for _, p := range loaded.DatasheetPaths {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					cleaned = append(cleaned, trimmed)
				}
			}
			if len(cleaned) > 0 {
				cfg.DatasheetPaths = cleaned
			}
		}
		if len(loaded.MasterlistPaths) > 0 {
			var cleaned []string
			for _, p := range loaded.MasterlistPaths {
				trimmed := strings.TrimSpace(p)
				if trimmed != "" {
					cleaned = append(cleaned, trimmed)
				}
			}
			if len(cleaned) > 0 {
				cfg.MasterlistPaths = cleaned
			}
		}
		if loaded.Theme == "light" || loaded.Theme == "dark" {
			cfg.Theme = loaded.Theme
		}
	}
	return cfg
}

func saveConfig(exeDir string, cfg Config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(exeDir), data, 0o666)
}

func resolveExcelPath(exeDir string, paths []string, filename string) string {
	if filename == "" {
		return ""
	}
	for _, p := range paths {
		targetDir := p
		if !filepath.IsAbs(targetDir) {
			targetDir = filepath.Join(exeDir, targetDir)
		}
		candidate := filepath.Join(targetDir, filename)
		if fi, err := os.Stat(candidate); err == nil && !fi.IsDir() {
			abs, err := filepath.Abs(candidate)
			if err == nil {
				return abs
			}
			return candidate
		}
	}
	return ""
}

type DirCountStatus struct {
	ConfiguredPath string
	ResolvedPath   string
	Exists         bool
	FileCount      int
}

func checkExcelDirStatuses(baseDir string, paths []string) []DirCountStatus {
	var list []DirCountStatus
	for _, p := range paths {
		resolved := p
		if !filepath.IsAbs(resolved) {
			resolved = filepath.Join(baseDir, resolved)
		}
		abs, err := filepath.Abs(resolved)
		if err == nil {
			resolved = abs
		}
		st := DirCountStatus{
			ConfiguredPath: p,
			ResolvedPath:   resolved,
		}
		fi, err := os.Stat(resolved)
		if err == nil && fi.IsDir() {
			st.Exists = true
			entries, _ := os.ReadDir(resolved)
			for _, entry := range entries {
				if !entry.IsDir() && strings.HasSuffix(strings.ToLower(entry.Name()), ".xlsx") {
					st.FileCount++
				}
			}
		}
		list = append(list, st)
	}
	return list
}
