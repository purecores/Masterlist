package main

import (
	_ "embed"
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed data/masterlist.db
var seedDatabase []byte

//go:embed assets/mygo-sqlite3.dll
var embeddedSQLiteDLL []byte

func main() {
	dbFlag := flag.String("db", "", "existing SQLite database to open")
	queryFlag := flag.String("query", "", "initial search query")
	settingsFlag := flag.Bool("settings", false, "open settings modal on launch")
	addFlag := flag.Bool("add", false, "open add part modal on launch")
	catFlag := flag.Int("cat", -1, "initial workbook index to select")
	syncFlag := flag.Bool("sync", false, "open sync modal on launch")
	flag.Parse()

	exe, err := os.Executable()
	var exeDir string
	if err == nil {
		exeDir = filepath.Dir(exe)
	} else {
		exeDir = "."
	}
	if abs, err := filepath.Abs(exeDir); err == nil {
		exeDir = abs
	}

	// Strictly confine all MyGo user data to Portable directory
	// Ensures zero registry writing, zero %APPDATA% pollution, standard User permission compatible.
	mygo.App.SetPath(mygo.PathUserData, exeDir)

	// Ensure mygo-sqlite3.dll exists locally so SQLite never tries to download it from GitHub.
	// This completely eliminates the 1-minute timeout and database connection failures on work computers.
	dllPath := filepath.Join(exeDir, "mygo-sqlite3.dll")
	if fi, err := os.Stat(dllPath); errors.Is(err, os.ErrNotExist) || (err == nil && fi.Size() == 0) {
		if len(embeddedSQLiteDLL) > 0 {
			_ = os.WriteFile(dllPath, embeddedSQLiteDLL, 0o666)
		}
	}
	if fi, err := os.Stat(dllPath); err == nil && fi.Size() > 0 {
		_ = os.Setenv("MYGO_SQLITE_LIBRARY", dllPath)
	}

	a := newApp(*dbFlag, *queryFlag, *settingsFlag)
	if *addFlag {
		a.openAdd()
	}
	if *catFlag >= 0 {
		a.workbookSel = *catFlag
	}
	if *syncFlag {
		a.syncModalOpen = true
		a.syncDryRun = true
	}
	mygo.App.WhenReady(func() {
		dbPath, err := resolveDatabase(*dbFlag, exeDir)
		if err != nil {
			a.loading = false
			a.loadErr = err.Error()
		}
		a.dbPath = dbPath
		win := mygo.NewWindow(mygo.WindowOptions{
			Title:           "电气柜器件库",
			Width:           1360,
			Height:          860,
			MinWidth:        1060,
			MinHeight:       640,
			StateKey:        "masterlist-native-window-v4",
			TitleBarStyle:   mygo.TitleBarHidden,
			TitleBarHeight:  42,
			AutoHideMenuBar: true,
			Content:         ui.View(a.view),
		})
		a.win = win
		a.appReady = true
		// Follow the desktop's light/dark appearance, and react when it
		// changes while the app is open.
		mygo.Theme.SetSource(mygo.ThemeSystem)
		a.applyTheme()
		mygo.Theme.OnUpdated(func() {
			a.applyTheme()
			win.Invalidate()
		})
		if len(appIconBytes) > 0 {
			_ = win.SetIcon(appIconBytes)
		}
		win.OnClose(a.handleClose)
		if runtime.GOOS == "darwin" {
			mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
				{Role: mygo.RoleAppMenu},
				{Role: mygo.RoleEditMenu},
				{Role: mygo.RoleWindowMenu},
			}))
		}
		// The tray icon lives as long as the app: closing the window hides
		// it there when the setting asks for that.
		a.setupTray()
		a.startAutoSync()
		if a.loadErr == "" {
			go a.loadData(win)
		}
		mygo.App.OnQuit(func() {
			a.quitting = true
			a.stopAutoSync()
			a.destroyTray()
			if a.store != nil {
				_ = a.store.Close()
			}
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}

func resolveDatabase(explicit, exeDir string) (string, error) {
	if explicit != "" {
		path, err := filepath.Abs(explicit)
		if err != nil {
			return "", err
		}
		if _, err := os.Stat(path); err != nil {
			return "", fmt.Errorf("指定的数据库文件不存在: %s", path)
		}
		return path, nil
	}

	// 1. Check <exeDir>/data/masterlist.db
	target1 := filepath.Join(exeDir, "data", "masterlist.db")
	if fi, err := os.Stat(target1); err == nil && !fi.IsDir() {
		return target1, nil
	}

	// 2. Check ./data/masterlist.db (working directory)
	target2 := filepath.Join("data", "masterlist.db")
	if fi, err := os.Stat(target2); err == nil && !fi.IsDir() {
		abs, err := filepath.Abs(target2)
		if err == nil {
			return abs, nil
		}
		return target2, nil
	}

	// 3. Check <exeDir>/masterlist.db
	target3 := filepath.Join(exeDir, "masterlist.db")
	if fi, err := os.Stat(target3); err == nil && !fi.IsDir() {
		return target3, nil
	}

	// 4. Default to <exeDir>/data/masterlist.db, create data dir and extract embedded seed
	dataDir := filepath.Join(exeDir, "data")
	if err := os.MkdirAll(dataDir, 0o755); err != nil {
		return "", fmt.Errorf("创建数据目录失败 (%s): %w", dataDir, err)
	}
	target := filepath.Join(dataDir, "masterlist.db")
	if fi, err := os.Stat(target); errors.Is(err, os.ErrNotExist) || (err == nil && fi.Size() == 0) {
		if len(seedDatabase) > 0 {
			if err := os.WriteFile(target, seedDatabase, 0o666); err != nil {
				return "", fmt.Errorf("释放内置数据库失败 (%s): %w", target, err)
			}
		}
	} else if err != nil {
		return "", fmt.Errorf("检查数据库文件失败 (%s): %w", target, err)
	}
	return target, nil
}
