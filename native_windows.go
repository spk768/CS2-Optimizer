package main

// Chamadas nativas do Windows no lugar de programas externos (PowerShell, taskkill, explorer).
// Alem de mais rapido, evita padroes que antivirus costumam achar suspeitos.

import (
	"errors"
	"syscall"
	"unsafe"
)

var (
	srclient            = syscall.NewLazyDLL("srclient.dll")
	shell32             = syscall.NewLazyDLL("shell32.dll")
	pSRSetRestorePointW = srclient.NewProc("SRSetRestorePointW")
	pShellExecuteW      = shell32.NewProc("ShellExecuteW")
	pTerminateProcess   = kernel32.NewProc("TerminateProcess")
	pEnumWindows        = user32.NewProc("EnumWindows")
	pIsWindowVisible    = user32.NewProc("IsWindowVisible")
)

// RESTOREPOINTINFOW (pack 1): tipo do evento, tipo do ponto, sequencia, descricao[256]
type restorePointInfo struct {
	EventType   uint32
	PointType   uint32
	Sequence    int64
	Description [256]uint16
}

// STATEMGRSTATUS (pack 1): status + sequencia = 12 bytes
type stateMgrStatus [12]byte

// createRestorePoint cria um ponto de restauracao do Windows pela API do proprio sistema
func createRestorePoint() error {
	if err := srclient.Load(); err != nil {
		return errors.New("Restauração do Sistema indisponível")
	}
	info := restorePointInfo{EventType: 100, PointType: 12} // BEGIN_SYSTEM_CHANGE, MODIFY_SETTINGS
	copy(info.Description[:255], syscall.StringToUTF16("CS2 Optimizer"))
	var st stateMgrStatus
	r, _, _ := pSRSetRestorePointW.Call(uintptr(unsafe.Pointer(&info)), uintptr(unsafe.Pointer(&st)))
	if r == 0 {
		return errors.New("a Restauração do Sistema está desligada ou recusou o pedido")
	}
	seq := int64(0)
	for i := 0; i < 8; i++ {
		seq |= int64(st[4+i]) << (8 * i)
	}
	end := restorePointInfo{EventType: 101, PointType: 12, Sequence: seq} // END_SYSTEM_CHANGE
	pSRSetRestorePointW.Call(uintptr(unsafe.Pointer(&end)), uintptr(unsafe.Pointer(&st)))
	return nil
}

// terminateByName encerra todos os processos com esse nome (como taskkill /F)
func terminateByName(name string) int {
	n := 0
	for _, pid := range findProcesses(name) {
		h, _, _ := pOpenProcess.Call(0x0001, 0, uintptr(pid)) // PROCESS_TERMINATE
		if h == 0 {
			continue
		}
		if r, _, _ := pTerminateProcess.Call(h, 0); r != 0 {
			n++
		}
		pCloseHandle.Call(h)
	}
	return n
}

var (
	closeTargets map[uint32]bool
	closeCount   int
	closeEnumCB  = syscall.NewCallback(func(hwnd, _ uintptr) uintptr {
		var pid uint32
		pGetWindowThreadProcId.Call(hwnd, uintptr(unsafe.Pointer(&pid)))
		if closeTargets[pid] {
			if v, _, _ := pIsWindowVisible.Call(hwnd); v != 0 {
				pPostMessageW.Call(hwnd, WM_CLOSE, 0, 0)
				closeCount++
			}
		}
		return 1
	})
)

// closeGracefully pede para as janelas do programa fecharem (como clicar no X)
func closeGracefully(name string) int {
	pids := findProcesses(name)
	if len(pids) == 0 {
		return 0
	}
	closeTargets = map[uint32]bool{}
	for _, p := range pids {
		closeTargets[p] = true
	}
	closeCount = 0
	pEnumWindows.Call(closeEnumCB, 0)
	closeTargets = nil
	return closeCount
}

// openURL abre um link (ex.: steam://) pelo programa associado
func openURL(url string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(url))), 0, 0, 1)
}
