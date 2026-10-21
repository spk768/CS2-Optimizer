package main

// API de desenho/janela usada pela interface personalizada.

import "syscall"

var (
	dwmapi = syscall.NewLazyDLL("dwmapi.dll")

	pBeginPaint         = user32.NewProc("BeginPaint")
	pEndPaint           = user32.NewProc("EndPaint")
	pInvalidateRect     = user32.NewProc("InvalidateRect")
	pDrawTextW          = user32.NewProc("DrawTextW")
	pSetCapture         = user32.NewProc("SetCapture")
	pReleaseCapture     = user32.NewProc("ReleaseCapture")
	pTrackMouseEvent    = user32.NewProc("TrackMouseEvent")
	pSetCursor          = user32.NewProc("SetCursor")
	pKillTimer          = user32.NewProc("KillTimer")
	pScreenToClient     = user32.NewProc("ScreenToClient")
	pSystemParamsInfo   = user32.NewProc("SystemParametersInfoW")
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pBitBlt             = gdi32.NewProc("BitBlt")
	pGdiFlush           = gdi32.NewProc("GdiFlush")
	pDwmSetWindowAttr   = dwmapi.NewProc("DwmSetWindowAttribute")
	pUpdateLayeredWin   = user32.NewProc("UpdateLayeredWindow")
	pValidateRect       = user32.NewProc("ValidateRect")
)

const (
	WM_PAINT       = 0x000F
	WM_NCHITTEST   = 0x0084
	WS_POPUP       = 0x80000000
	WS_EX_LAYERED  = 0x00080000
	WS_EX_APPWIN   = 0x00040000
	WM_ERASEBKGND  = 0x0014
	WM_SETCURSOR   = 0x0020
	WM_MOUSEMOVE   = 0x0200
	WM_LBUTTONDOWN = 0x0201
	WM_LBUTTONUP   = 0x0202
	WM_MOUSEWHEEL  = 0x020A
	WM_MOUSELEAVE  = 0x02A3

	DT_CENTER       = 0x1
	DT_RIGHT        = 0x2
	DT_VCENTER      = 0x4
	DT_SINGLELINE   = 0x20
	DT_NOPREFIX     = 0x800
	DT_END_ELLIPSIS = 0x8000
	SRCCOPY         = 0x00CC0020
)

type paintStruct struct {
	Hdc       uintptr
	Erase     int32
	RcPaint   rect
	Restore   int32
	IncUpdate int32
	Reserved  [32]byte
}

type bitmapInfo struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
	Colors        uint32
}

type trackMouseEvent struct {
	Size      uint32
	Flags     uint32
	HwndTrack uintptr
	HoverTime uint32
}
