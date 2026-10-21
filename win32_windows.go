package main

// Chamadas diretas da API do Windows (sem dependencias externas).

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	gdi32    = syscall.NewLazyDLL("gdi32.dll")
	comctl32 = syscall.NewLazyDLL("comctl32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassExW  = user32.NewProc("RegisterClassExW")
	pCreateWindowExW   = user32.NewProc("CreateWindowExW")
	pDefWindowProcW    = user32.NewProc("DefWindowProcW")
	pGetMessageW       = user32.NewProc("GetMessageW")
	pTranslateMessage  = user32.NewProc("TranslateMessage")
	pDispatchMessageW  = user32.NewProc("DispatchMessageW")
	pIsDialogMessageW  = user32.NewProc("IsDialogMessageW")
	pPostQuitMessage   = user32.NewProc("PostQuitMessage")
	pSendMessageW      = user32.NewProc("SendMessageW")
	pPostMessageW      = user32.NewProc("PostMessageW")
	pMessageBoxW       = user32.NewProc("MessageBoxW")
	pSetWindowTextW    = user32.NewProc("SetWindowTextW")
	pEnableWindow      = user32.NewProc("EnableWindow")
	pSetTimer          = user32.NewProc("SetTimer")
	pLoadIconW         = user32.NewProc("LoadIconW")
	pLoadImageW        = user32.NewProc("LoadImageW")
	pLoadCursorW       = user32.NewProc("LoadCursorW")
	pGetSystemMetrics  = user32.NewProc("GetSystemMetrics")
	pAdjustWindowRect  = user32.NewProc("AdjustWindowRect")
	pShowWindow        = user32.NewProc("ShowWindow")
	pUpdateWindow      = user32.NewProc("UpdateWindow")
	pDestroyWindow     = user32.NewProc("DestroyWindow")
	pGetDC             = user32.NewProc("GetDC")
	pReleaseDC         = user32.NewProc("ReleaseDC")
	pGetSysColorBrush  = user32.NewProc("GetSysColorBrush")
	pCreateFontW       = gdi32.NewProc("CreateFontW")
	pGetDeviceCaps     = gdi32.NewProc("GetDeviceCaps")
	pSetTextColor      = gdi32.NewProc("SetTextColor")
	pSetBkMode         = gdi32.NewProc("SetBkMode")
	pGetModuleHandleW  = kernel32.NewProc("GetModuleHandleW")
	pCreateToolhelp32  = kernel32.NewProc("CreateToolhelp32Snapshot")
	pProcess32FirstW   = kernel32.NewProc("Process32FirstW")
	pProcess32NextW    = kernel32.NewProc("Process32NextW")
	pOpenProcess       = kernel32.NewProc("OpenProcess")
	pCloseHandle       = kernel32.NewProc("CloseHandle")
	pGetPriorityClass  = kernel32.NewProc("GetPriorityClass")
	pSetPriorityClass  = kernel32.NewProc("SetPriorityClass")
	pInitCommonCtrlsEx = comctl32.NewProc("InitCommonControlsEx")
	pRegOpenKeyExW     = advapi32.NewProc("RegOpenKeyExW")
	pRegCreateKeyExW   = advapi32.NewProc("RegCreateKeyExW")
	pRegQueryValueExW  = advapi32.NewProc("RegQueryValueExW")
	pRegSetValueExW    = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW   = advapi32.NewProc("RegDeleteValueW")
	pRegCloseKey       = advapi32.NewProc("RegCloseKey")
)

const (
	WS_CHILD         = 0x40000000
	WS_VISIBLE       = 0x10000000
	WS_TABSTOP       = 0x00010000
	WS_VSCROLL       = 0x00200000
	WS_CAPTION       = 0x00C00000
	WS_SYSMENU       = 0x00080000
	WS_MINIMIZEBOX   = 0x00020000
	WS_EX_CLIENTEDGE = 0x00000200

	BS_PUSHBUTTON    = 0x0
	BS_AUTOCHECKBOX  = 0x3
	BS_GROUPBOX      = 0x7
	BS_MULTILINE     = 0x2000
	ES_MULTILINE     = 0x4
	ES_AUTOVSCROLL   = 0x40
	ES_READONLY      = 0x800
	CBS_DROPDOWNLIST = 0x3

	WM_DESTROY        = 0x0002
	WM_CLOSE          = 0x0010
	WM_SETFONT        = 0x0030
	WM_GETTEXTLENGTH  = 0x000E
	WM_COMMAND        = 0x0111
	WM_TIMER          = 0x0113
	WM_CTLCOLORSTATIC = 0x0138
	WM_APP            = 0x8000
	BM_GETCHECK       = 0x00F0
	BM_SETCHECK       = 0x00F1
	CB_ADDSTRING      = 0x0143
	CB_GETCURSEL      = 0x0147
	CB_SETCURSEL      = 0x014E
	EM_SETSEL         = 0x00B1
	EM_REPLACESEL     = 0x00C2
	EM_SETLIMITTEXT   = 0x00C5
	BN_CLICKED        = 0

	MB_OK              = 0x0
	MB_YESNO           = 0x4
	MB_ICONERROR       = 0x10
	MB_ICONQUESTION    = 0x20
	MB_ICONWARNING     = 0x30
	MB_ICONINFORMATION = 0x40
	IDYES              = 6

	COLOR_BTNFACE = 15
)

type wndClassEx struct {
	CbSize     uint32
	Style      uint32
	WndProc    uintptr
	ClsExtra   int32
	WndExtra   int32
	Instance   uintptr
	Icon       uintptr
	Cursor     uintptr
	Background uintptr
	MenuName   *uint16
	ClassName  *uint16
	IconSm     uintptr
}

type point struct{ X, Y int32 }

type msgT struct {
	Hwnd     uintptr
	Message  uint32
	WParam   uintptr
	LParam   uintptr
	Time     uint32
	Pt       point
	LPrivate uint32
}

type rect struct{ Left, Top, Right, Bottom int32 }

type processEntry32 struct {
	Size            uint32
	Usage           uint32
	ProcessID       uint32
	DefaultHeapID   uintptr
	ModuleID        uint32
	Threads         uint32
	ParentProcessID uint32
	PriClassBase    int32
	Flags           uint32
	ExeFile         [260]uint16
}

func u16(s string) *uint16 {
	p, err := syscall.UTF16PtrFromString(s)
	if err != nil {
		p, _ = syscall.UTF16PtrFromString("")
	}
	return p
}

func sendMessage(h, msg, w, l uintptr) uintptr {
	r, _, _ := pSendMessageW.Call(h, msg, w, l)
	return r
}

func setText(h uintptr, s string) {
	pSetWindowTextW.Call(h, uintptr(unsafe.Pointer(u16(s))))
}

func enable(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	pEnableWindow.Call(h, v)
}

func isChecked(h uintptr) bool { return sendMessage(h, BM_GETCHECK, 0, 0) == 1 }

func setChecked(h uintptr, on bool) {
	v := uintptr(0)
	if on {
		v = 1
	}
	sendMessage(h, BM_SETCHECK, v, 0)
}

func msgBox(owner uintptr, text, title string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(owner, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags)
	return int(r)
}
