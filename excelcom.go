package main

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// shellOpenPath is the plain shell open, kept behind a variable so the COM
// path and the fallback read the same.
var shellOpenPath = func(path string) error { return mygo.Shell.OpenPath(path) }

// openInExcelWithPassword opens a workbook with the installed Excel through
// its automation object, supplying the edit password so a protected sheet
// opens without a prompt, and opens it read-only so the local copy is never
// written back.
func openInExcelWithPassword(path, password string) error {
	// COM objects live in the apartment of the thread that made them, and
	// Go moves goroutines between threads; the call stays on one thread.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()

	unknown, err := oleutil.CreateObject("Excel.Application")
	if err != nil {
		return fmt.Errorf("Excel 不可用: %w", err)
	}
	defer unknown.Release()

	excel, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		return err
	}
	defer excel.Release()

	if _, err := oleutil.PutProperty(excel, "Visible", false); err != nil {
		return err
	}
	if _, err := oleutil.PutProperty(excel, "DisplayAlerts", false); err != nil {
		return err
	}

	workbooks, err := oleutil.GetProperty(excel, "Workbooks")
	if err != nil {
		return fmt.Errorf("无法取得 Excel 工作簿集合: %w", err)
	}
	books := workbooks.ToIDispatch()
	if books == nil {
		return fmt.Errorf("Excel 未返回工作簿集合")
	}
	defer books.Release()

	// Workbooks.Open takes fifteen position-based optional arguments, and a
	// COM call cannot skip one, so every position is supplied: nil leaves a
	// position at its default, and the rest carry what the app wants.
	var opened *ole.VARIANT
	if password != "" {
		// WriteResPassword opens a protected workbook; Password itself belongs
		// to older formats and is left out, so the call passes the three
		// leading positions and the write-reservation one.
		opened, err = oleutil.CallMethod(books, "Open", path,
			nil,      // UpdateLinks
			true,     // ReadOnly: the local copy is never written back
			nil,      // Format
			password, // WriteResPassword: the workbook's reserved password
		)
	} else {
		opened, err = oleutil.CallMethod(books, "Open", path,
			nil, true, nil, nil, nil,
			true, nil, nil, true, false, nil,
			false, false, nil)
	}
	if err != nil {
		return err
	}
	opened.Clear()

	// The workbook is open in Excel's own window; the automation object
	// this call created goes away, and Excel stays in the user's hands.
	_ = oleutil.MustPutProperty(excel, "DisplayAlerts", true)
	if _, err := oleutil.PutProperty(excel, "Visible", true); err != nil {
		return err
	}
	return nil
}
