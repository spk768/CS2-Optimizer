package main

// Ajustes de sistema extras: energia de USB/PCIe, mouse, cache de shaders,
// timer de 0,5 ms, limpeza da memoria em espera e vibrancia digital.

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	ntdllDLL               = syscall.NewLazyDLL("ntdll.dll")
	pNtSetTimerResolution  = ntdllDLL.NewProc("NtSetTimerResolution")
	pNtQuerySystemInfo     = ntdllDLL.NewProc("NtQuerySystemInformation")
	pNtSetSystemInfo       = ntdllDLL.NewProc("NtSetSystemInformation")
	pOpenProcessToken      = advapi32.NewProc("OpenProcessToken")
	pLookupPrivilegeValueW = advapi32.NewProc("LookupPrivilegeValueW")
	pAdjustTokenPrivileges = advapi32.NewProc("AdjustTokenPrivileges")
	pGetCurrentProcess     = kernel32.NewProc("GetCurrentProcess")
	pGetForegroundWindow   = user32.NewProc("GetForegroundWindow")
	pGetWindowThreadProcId = user32.NewProc("GetWindowThreadProcessId")
	pSystemParametersInfoW = user32.NewProc("SystemParametersInfoW")
	pSendMessageTimeoutW   = user32.NewProc("SendMessageTimeoutW")
)

// ---------- configuracoes do plano de energia (com copia dos valores anteriores) ----------

const (
	subUSB       = "2a737441-1930-4402-8d77-b2bebba308a3"
	setUSBSusp   = "48e6b7a6-50f5-4782-a5d4-53bb8f07e226" // suspensao seletiva de USB
	subPCIe      = "501a4d13-42af-4429-9fd1-a8218c268e20"
	setPCIeASPM  = "ee12f906-d277-404b-b6da-e5fa1a576df5" // gerenciamento de energia do link PCIe
	powerHexExpr = `0x[0-9a-fA-F]{8}`
)

type powerSettingBackup struct {
	Scheme, Sub, Setting string
	AC, DC               uint32
}

var powerHexRe = regexp.MustCompile(powerHexExpr)

func activeScheme() (string, error) {
	out, err := powercfg("/getactivescheme")
	g := guidRe.FindString(out)
	if err != nil || g == "" {
		return "", errors.New("não foi possível ler o plano de energia ativo")
	}
	return g, nil
}

// setPowerSetting grava o valor (tomada e bateria) no plano ativo, guardando o valor anterior
func setPowerSetting(sub, setting string, value uint32) error {
	scheme, err := activeScheme()
	if err != nil {
		return err
	}
	known := false
	for _, b := range backup.PowerSet {
		if strings.EqualFold(b.Scheme, scheme) && b.Setting == setting {
			known = true
		}
	}
	if !known {
		out, err := powercfg("/query", scheme, sub, setting)
		vals := powerHexRe.FindAllString(out, -1)
		if err != nil || len(vals) < 2 {
			return errors.New("configuração de energia não disponível neste PC")
		}
		ac, _ := strconv.ParseUint(vals[len(vals)-2][2:], 16, 32)
		dc, _ := strconv.ParseUint(vals[len(vals)-1][2:], 16, 32)
		backup.PowerSet = append(backup.PowerSet, powerSettingBackup{scheme, sub, setting, uint32(ac), uint32(dc)})
		if err := saveBackup(); err != nil {
			return err
		}
	}
	v := strconv.Itoa(int(value))
	if out, err := powercfg("/setacvalueindex", scheme, sub, setting, v); err != nil {
		return fmt.Errorf("powercfg: %s", strings.TrimSpace(out))
	}
	powercfg("/setdcvalueindex", scheme, sub, setting, v)
	powercfg("/setactive", scheme)
	return nil
}

func restorePowerSettings() []string {
	var errs []string
	for _, b := range backup.PowerSet {
		if out, err := powercfg("/setacvalueindex", b.Scheme, b.Sub, b.Setting, strconv.Itoa(int(b.AC))); err != nil {
			errs = append(errs, "energia: "+strings.TrimSpace(out))
		}
		powercfg("/setdcvalueindex", b.Scheme, b.Sub, b.Setting, strconv.Itoa(int(b.DC)))
	}
	if len(backup.PowerSet) > 0 {
		powercfg("/setactive", "SCHEME_CURRENT")
	}
	return errs
}

// ---------- mouse e efeitos visuais ----------

// applyMouseFromRegistry aplica na hora os valores do registro (aceleracao do mouse)
func applyMouseFromRegistry() {
	get := func(name string) int32 {
		n, _ := strconv.Atoi(strings.TrimSpace(regString(regRead(HKCU, `Control Panel\Mouse`, name))))
		return int32(n)
	}
	vals := [3]int32{get("MouseThreshold1"), get("MouseThreshold2"), get("MouseSpeed")}
	pSystemParametersInfoW.Call(0x0004, 0, uintptr(unsafe.Pointer(&vals[0])), 2) // SPI_SETMOUSE, SPIF_SENDCHANGE
}

// broadcastSettingChange avisa o Explorer para reler as configuracoes de tema (transparencia)
func broadcastSettingChange() {
	var res uintptr
	pSendMessageTimeoutW.Call(0xFFFF, 0x001A, 0, uintptr(unsafe.Pointer(u16("ImmersiveColorSet"))), 2, 1000, uintptr(unsafe.Pointer(&res)))
}

// ---------- cache de shaders ----------

func clearShaderCaches() (int64, int) {
	local := os.Getenv("LOCALAPPDATA")
	if local == "" {
		return 0, 0
	}
	dirs := []string{
		filepath.Join(local, "NVIDIA", "DXCache"),
		filepath.Join(local, "NVIDIA", "GLCache"),
		filepath.Join(local, "D3DSCache"),
		filepath.Join(local, "AMD", "DxCache"),
		filepath.Join(local, "AMD", "DxcCache"),
		filepath.Join(local, "AMD", "VkCache"),
		filepath.Join(local, "Intel", "ShaderCache"),
	}
	var freed int64
	var n int
	for _, d := range dirs {
		filepath.WalkDir(d, func(p string, de fs.DirEntry, err error) error {
			if err != nil || !de.Type().IsRegular() {
				return nil
			}
			if info, err := de.Info(); err == nil && os.Remove(p) == nil {
				freed += info.Size()
				n++
			}
			return nil
		})
	}
	return freed, n
}

// ---------- timer de 0,5 ms ----------

var timerRequested bool

func requestTimer(on bool) {
	if on == timerRequested {
		return
	}
	var actual uint32
	set := uintptr(0)
	if on {
		set = 1
	}
	pNtSetTimerResolution.Call(5000, set, uintptr(unsafe.Pointer(&actual))) // 5000 x 100 ns = 0,5 ms
	timerRequested = on
	if on {
		logf("Timer do Windows em %.2f ms enquanto o app estiver aberto.", float64(actual)/10000)
	}
}

// ---------- limpeza da memoria em espera (como o ISLC) ----------

var (
	standbyPrivOK   bool
	standbyLastLog  time.Time
	standbyPrivDone bool
)

func enablePrivilege(name string) bool {
	var token uintptr
	proc, _, _ := pGetCurrentProcess.Call()
	if r, _, _ := pOpenProcessToken.Call(proc, 0x0020|0x0008, uintptr(unsafe.Pointer(&token))); r == 0 {
		return false
	}
	defer pCloseHandle.Call(token)
	var tp struct {
		Count      uint32
		LowPart    uint32
		HighPart   int32
		Attributes uint32
	}
	if r, _, _ := pLookupPrivilegeValueW.Call(0, uintptr(unsafe.Pointer(u16(name))), uintptr(unsafe.Pointer(&tp.LowPart))); r == 0 {
		return false
	}
	tp.Count, tp.Attributes = 1, 2 // SE_PRIVILEGE_ENABLED
	r, _, err := pAdjustTokenPrivileges.Call(token, 0, uintptr(unsafe.Pointer(&tp)), 0, 0, 0)
	errno, _ := err.(syscall.Errno)
	return r != 0 && errno == 0 // ERROR_NOT_ALL_ASSIGNED (1300) = privilegio negado
}

// memoryLists devolve (memoria livre, memoria em espera) em bytes
func memoryLists() (uint64, uint64, bool) {
	var info [22]uint64 // SYSTEM_MEMORY_LIST_INFORMATION (x64) = 176 bytes
	var ret uint32
	if r, _, _ := pNtQuerySystemInfo.Call(80, uintptr(unsafe.Pointer(&info[0])), unsafe.Sizeof(info), uintptr(unsafe.Pointer(&ret))); r != 0 {
		return 0, 0, false
	}
	free := info[0] + info[1] // zeradas + livres
	var standby uint64
	for i := 5; i < 13; i++ { // PageCountByPriority[8]
		standby += info[i]
	}
	return free * 4096, standby * 4096, true
}

// cleanStandbyIfNeeded: libera a memoria em espera se a livre ficou abaixo de 1 GB e a em espera passou de 1 GB
func cleanStandbyIfNeeded() {
	if !standbyPrivDone {
		standbyPrivOK = enablePrivilege("SeProfileSingleProcessPrivilege")
		standbyPrivDone = true
	}
	if !standbyPrivOK {
		return
	}
	free, standby, ok := memoryLists()
	if !ok || free >= 1<<30 || standby < 1<<30 {
		return
	}
	cmd := uint32(4) // MemoryPurgeStandbyList
	if r, _, _ := pNtSetSystemInfo.Call(80, uintptr(unsafe.Pointer(&cmd)), 4); r == 0 && time.Since(standbyLastLog) > time.Minute {
		standbyLastLog = time.Now()
		logf("Memória em espera liberada (%d MB).", standby>>20)
	}
}

// ---------- janela em primeiro plano ----------

func foregroundPID() uint32 {
	h, _, _ := pGetForegroundWindow.Call()
	if h == 0 {
		return 0
	}
	var pid uint32
	pGetWindowThreadProcId.Call(h, uintptr(unsafe.Pointer(&pid)))
	return pid
}

func cs2InForeground() bool {
	fg := foregroundPID()
	if fg == 0 {
		return false
	}
	for _, p := range findProcesses("cs2.exe") {
		if p == fg {
			return true
		}
	}
	return false
}

// ---------- vibrancia digital (NVIDIA) ----------
// IDs nao documentados publicamente; conferidos em github.com/arcnmx/nvapi-rs (nvid.rs).

const (
	nvEnumDisplayHandle = 0x9abdd40d
	nvGetDVCInfo        = 0x4085de45
	nvSetDVCLevel       = 0x172409b4
)

type dvcInfo struct {
	Version, Current, Min, Max int32
}

type vibranceDisplay struct {
	handle           uintptr
	original, target int32
}

var (
	vibDisplays []vibranceDisplay
	vibActive   bool
	vibProbed   bool
)

func vibranceProbe() bool {
	if vibProbed {
		return len(vibDisplays) > 0
	}
	vibProbed = true
	if !nvidiaAvailable() {
		return false
	}
	for i := uintptr(0); i < 8; i++ {
		var h uintptr
		if nvCall(nvEnumDisplayHandle, i, uintptr(unsafe.Pointer(&h))) != 0 {
			break
		}
		info := dvcInfo{Version: int32(unsafe.Sizeof(dvcInfo{})) | 1<<16}
		if nvCall(nvGetDVCInfo, h, 0, uintptr(unsafe.Pointer(&info))) != 0 || info.Max <= info.Min {
			continue
		}
		// 80% no Painel NVIDIA: a escala vai de 50% (minimo) a 100% (maximo)
		target := info.Min + int32(float64(info.Max-info.Min)*0.6+0.5)
		vibDisplays = append(vibDisplays, vibranceDisplay{h, info.Current, target})
	}
	return len(vibDisplays) > 0
}

// updateVibrance liga a vibrancia com o CS2 em primeiro plano e volta ao normal fora dele
func updateVibrance(wanted bool) {
	if wanted && !vibranceProbe() {
		return
	}
	on := wanted && cs2InForeground()
	if on == vibActive {
		return
	}
	for _, d := range vibDisplays {
		level := d.original
		if on {
			level = d.target
		}
		nvCall(nvSetDVCLevel, d.handle, 0, uintptr(level))
	}
	vibActive = on
}
