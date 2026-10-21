package main

// Perfil do driver NVIDIA para o cs2.exe (mesmo efeito do Painel de Controle NVIDIA),
// via NVAPI DRS. IDs e estruturas conferidos com github.com/NVIDIA/nvapi.

import (
	"errors"
	"fmt"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

const (
	nvInitialize        = 0x0150e828
	nvCreateSession     = 0x0694d52e
	nvDestroySession    = 0xdad9cff8
	nvLoadSettings      = 0x375dbd6b
	nvSaveSettings      = 0xfcbc7e14
	nvCreateProfile     = 0xcc176068
	nvCreateApplication = 0x4347a9de
	nvFindAppByName     = 0xeee566b2
	nvFindProfileByName = 0x7e4a9a0b
	nvSetSetting        = 0x577dd202
	nvGetSetting        = 0x73bf8338
	nvDeleteSetting     = 0xe4a26362

	nvSettingNotFound     = -160
	nvProfileNotFound     = -163
	nvExeNotFound         = -166
	nvIncompatibleVersion = -9

	settingPrerenderLimit = 0x007BA09E // "Modo de baixa latência" (1 = Ativado)
	settingPreferredState = 0x1057EB71 // "Gerenciamento de energia" (1 = Desempenho máximo)

	nvOwnProfile = "CS2 Optimizer"
)

// NVDRS_SETTING_V1 (pack 4) = 12320 bytes
type nvSetting struct {
	Version             uint32
	SettingName         [2048]uint16
	SettingID           uint32
	SettingType         uint32
	SettingLocation     uint32
	IsCurrentPredefined uint32
	IsPredefinedValid   uint32
	Predefined          [4100]byte
	Current             [4100]byte
}

// NVDRS_APPLICATION_V1 = 12296 bytes
type nvApplication struct {
	Version          uint32
	IsPredefined     uint32
	AppName          [2048]uint16
	UserFriendlyName [2048]uint16
	Launcher         [2048]uint16
}

// NVDRS_PROFILE_V1 = 4116 bytes
type nvProfile struct {
	Version       uint32
	ProfileName   [2048]uint16
	GpuSupport    uint32
	IsPredefined  uint32
	NumOfApps     uint32
	NumOfSettings uint32
}

type nvBackup struct {
	SettingID uint32
	Existed   bool
	Value     uint32
	Global    bool // perfil global (ex.: tamanho do cache de shaders)
}

const (
	settingShaderCacheSize = 0x00AC8497 // "Tamanho do cache de shaders" (0xFFFFFFFF = ilimitado)
	nvGetGlobalProfile     = 0x617bff9f
)

var nvSettingsList = []struct {
	id     uint32
	value  uint32
	global bool
}{
	{settingPrerenderLimit, 1, false},
	{settingPreferredState, 1, false},
	{settingShaderCacheSize, 0xFFFFFFFF, true},
}

func nvGlobal(session uintptr) (uintptr, error) {
	var p uintptr
	if r := nvCall(nvGetGlobalProfile, session, uintptr(unsafe.Pointer(&p))); r != 0 {
		return 0, fmt.Errorf("NVAPI GetCurrentGlobalProfile falhou (%d)", r)
	}
	return p, nil
}

var nvQuery *syscall.LazyProc

func nvFunc(id uint32) (uintptr, error) {
	if nvQuery == nil {
		dll := syscall.NewLazyDLL("nvapi64.dll")
		if err := dll.Load(); err != nil {
			return 0, errors.New("driver NVIDIA não encontrado")
		}
		nvQuery = dll.NewProc("nvapi_QueryInterface")
	}
	fn, _, _ := nvQuery.Call(uintptr(id))
	if fn == 0 {
		return 0, fmt.Errorf("função NVAPI 0x%08x indisponível", id)
	}
	return fn, nil
}

func nvCall(id uint32, args ...uintptr) int32 {
	fn, err := nvFunc(id)
	if err != nil {
		return -1
	}
	r, _, _ := syscall.SyscallN(fn, args...)
	return int32(r)
}

func nvString(s string) *[2048]uint16 {
	var buf [2048]uint16
	copy(buf[:2047], utf16.Encode([]rune(s)))
	return &buf
}

func nvidiaAvailable() bool {
	if _, err := nvFunc(nvInitialize); err != nil {
		return false
	}
	return nvCall(nvInitialize) == 0
}

// nvSession abre a base de perfis e acha (ou cria) o perfil que contem o cs2.exe
func nvSession(create bool) (session, profile uintptr, err error) {
	if !nvidiaAvailable() {
		return 0, 0, errors.New("placa NVIDIA não encontrada")
	}
	if r := nvCall(nvCreateSession, uintptr(unsafe.Pointer(&session))); r != 0 {
		return 0, 0, fmt.Errorf("NVAPI CreateSession falhou (%d)", r)
	}
	fail := func(e error) (uintptr, uintptr, error) {
		nvCall(nvDestroySession, session)
		return 0, 0, e
	}
	if r := nvCall(nvLoadSettings, session); r != 0 {
		return fail(fmt.Errorf("NVAPI LoadSettings falhou (%d)", r))
	}
	// tenta as versoes da estrutura NVDRS_APPLICATION (V1..V4), conforme o driver aceitar
	var appBuf [20492]byte
	var r int32
	for _, v := range []struct{ size, ver uint32 }{{12296, 1}, {16392, 2}, {16396, 3}, {20492, 4}} {
		appBuf = [20492]byte{}
		ver := v.size | v.ver<<16
		appBuf[0], appBuf[1], appBuf[2], appBuf[3] = byte(ver), byte(ver>>8), byte(ver>>16), byte(ver>>24)
		r = nvCall(nvFindAppByName, session, uintptr(unsafe.Pointer(nvString("cs2.exe"))), uintptr(unsafe.Pointer(&profile)), uintptr(unsafe.Pointer(&appBuf[0])))
		if r != nvIncompatibleVersion {
			break
		}
	}
	if r == 0 {
		return session, profile, nil
	}
	if r != nvExeNotFound || !create {
		return fail(fmt.Errorf("perfil do cs2.exe não encontrado no driver (%d)", r))
	}
	// o driver nao tem perfil do CS2: cria um perfil proprio com o cs2.exe
	if r := nvCall(nvFindProfileByName, session, uintptr(unsafe.Pointer(nvString(nvOwnProfile))), uintptr(unsafe.Pointer(&profile))); r != 0 {
		var p nvProfile
		p.Version = uint32(unsafe.Sizeof(p)) | 1<<16
		copy(p.ProfileName[:2047], utf16.Encode([]rune(nvOwnProfile)))
		if r := nvCall(nvCreateProfile, session, uintptr(unsafe.Pointer(&p)), uintptr(unsafe.Pointer(&profile))); r != 0 {
			return fail(fmt.Errorf("NVAPI CreateProfile falhou (%d)", r))
		}
		var a nvApplication
		a.Version = uint32(unsafe.Sizeof(a)) | 1<<16
		copy(a.AppName[:2047], utf16.Encode([]rune("cs2.exe")))
		if r := nvCall(nvCreateApplication, session, profile, uintptr(unsafe.Pointer(&a))); r != 0 {
			return fail(fmt.Errorf("NVAPI CreateApplication falhou (%d)", r))
		}
	}
	return session, profile, nil
}

// nvGet: inProfile = valor definido pelo usuario neste perfil (nao herdado e nao predefinido pela NVIDIA)
func nvGet(session, profile uintptr, id uint32) (value uint32, inProfile bool, ok bool) {
	var s nvSetting
	s.Version = uint32(unsafe.Sizeof(s)) | 1<<16
	r := nvCall(nvGetSetting, session, profile, uintptr(id), uintptr(unsafe.Pointer(&s)))
	if r != 0 {
		return 0, false, r == nvSettingNotFound
	}
	v := uint32(s.Current[0]) | uint32(s.Current[1])<<8 | uint32(s.Current[2])<<16 | uint32(s.Current[3])<<24
	return v, s.SettingLocation == 0 && s.IsCurrentPredefined == 0, true
}

func nvSet(session, profile uintptr, id, value uint32) error {
	var s nvSetting
	s.Version = uint32(unsafe.Sizeof(s)) | 1<<16
	s.SettingID = id
	s.SettingType = 0 // DWORD
	s.Current[0], s.Current[1], s.Current[2], s.Current[3] = byte(value), byte(value>>8), byte(value>>16), byte(value>>24)
	if r := nvCall(nvSetSetting, session, profile, uintptr(unsafe.Pointer(&s))); r != 0 {
		return fmt.Errorf("NVAPI SetSetting falhou (%d)", r)
	}
	return nil
}

// applyNvidia aplica as configuracoes escolhidas (indices em nvSettingsList), guardando o estado anterior
func applyNvidia(which []int) error {
	if len(which) == 0 {
		return nil
	}
	session, profile, err := nvSession(true)
	if err != nil {
		return err
	}
	defer nvCall(nvDestroySession, session)
	for _, i := range which {
		item := nvSettingsList[i]
		prof := profile
		if item.global {
			g, err := nvGlobal(session)
			if err != nil {
				return err
			}
			prof = g
		}
		known := false
		for _, b := range backup.Nvidia {
			if b.SettingID == item.id && b.Global == item.global {
				known = true
			}
		}
		if !known {
			v, inProfile, ok := nvGet(session, prof, item.id)
			if !ok {
				return fmt.Errorf("não foi possível ler a configuração atual do driver (0x%08X)", item.id)
			}
			backup.Nvidia = append(backup.Nvidia, nvBackup{item.id, inProfile, v, item.global})
			if err := saveBackup(); err != nil {
				return err
			}
		}
		if err := nvSet(session, prof, item.id, item.value); err != nil {
			return err
		}
	}
	if r := nvCall(nvSaveSettings, session); r != 0 {
		return fmt.Errorf("NVAPI SaveSettings falhou (%d)", r)
	}
	return nil
}

func restoreNvidia() error {
	if len(backup.Nvidia) == 0 {
		return nil
	}
	session, profile, err := nvSession(false)
	if err != nil {
		return err
	}
	defer nvCall(nvDestroySession, session)
	var firstErr error
	for _, b := range backup.Nvidia {
		prof := profile
		if b.Global {
			g, err := nvGlobal(session)
			if err != nil {
				firstErr = err
				continue
			}
			prof = g
		}
		if b.Existed {
			if err := nvSet(session, prof, b.SettingID, b.Value); err != nil && firstErr == nil {
				firstErr = err
			}
		} else if r := nvCall(nvDeleteSetting, session, prof, uintptr(b.SettingID)); r != 0 && r != nvSettingNotFound && firstErr == nil {
			firstErr = fmt.Errorf("NVAPI DeleteProfileSetting falhou (%d)", r)
		}
	}
	if r := nvCall(nvSaveSettings, session); r != 0 && firstErr == nil {
		firstErr = fmt.Errorf("NVAPI SaveSettings falhou (%d)", r)
	}
	return firstErr
}

// nvidiaStatus resume o estado atual para a interface
func nvidiaStatus() (bool, string) {
	session, profile, err := nvSession(false)
	if err != nil {
		if nvidiaAvailable() {
			return true, "NVIDIA detectada"
		}
		return false, "sem placa NVIDIA"
	}
	defer nvCall(nvDestroySession, session)
	ll, _, _ := nvGet(session, profile, settingPrerenderLimit)
	ps, _, _ := nvGet(session, profile, settingPreferredState)
	parts := "NVIDIA detectada"
	if ll == 1 && ps == 1 {
		parts += "  ·  já otimizada"
	}
	return true, parts
}
