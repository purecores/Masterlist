package main

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/go-ole/go-ole"
	"github.com/go-ole/go-ole/oleutil"
)

// Excel's automation object opens a write-protected workbook when it gets
// the reservation password, which is what the app passes for the original
// tables. The test builds such a workbook with Excel itself and opens it
// through the same function the app uses.
func TestOpenProtectedWorkbookWithPassword(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Excel automation is Windows-only")
	}
	path := filepath.Join(t.TempDir(), "protected.xlsx")

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	ole.CoInitializeEx(0, ole.COINIT_APARTMENTTHREADED)
	defer ole.CoUninitialize()
	unknown, err := oleutil.CreateObject("Excel.Application")
	if err != nil {
		t.Skip("Excel is not installed:", err)
	}
	defer unknown.Release()
	excel, err := unknown.QueryInterface(ole.IID_IDispatch)
	if err != nil {
		t.Fatal(err)
	}
	defer excel.Release()
	_, _ = oleutil.PutProperty(excel, "Visible", false)
	_, _ = oleutil.PutProperty(excel, "DisplayAlerts", false)
	booksVar, err := oleutil.GetProperty(excel, "Workbooks")
	if err != nil {
		t.Fatal(err)
	}
	books := booksVar.ToIDispatch()
	bookVar, err := oleutil.CallMethod(books, "Add")
	if err != nil {
		t.Fatal(err)
	}
	book := bookVar.ToIDispatch()
	// SaveAs(FileName, FileFormat, Password, WriteResPassword,
	// ReadOnlyRecommended, CreateBackup)
	if _, err := oleutil.CallMethod(book, "SaveAs", path, nil, nil, "pw", nil, nil); err != nil {
		t.Fatal(err)
	}
	_, _ = oleutil.CallMethod(book, "Close", false)
	books.Release()
	book.Release()
	ole.CoUninitialize()
	runtime.UnlockOSThread()

	if _, err := os.Stat(path); err != nil {
		t.Skip("Excel did not create the workbook:", err)
	}
	if err := openInExcelWithPassword(path, "pw"); err != nil {
		t.Fatalf("opening a protected workbook failed: %v", err)
	}
}
