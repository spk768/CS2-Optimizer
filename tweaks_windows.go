package main

// Ajustes, backup e restauracao.

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

const (
	hagsKey   = `SYSTEM\CurrentControlSet\Control\GraphicsDrivers`
	layersKey = `Software\Microsoft\Windows NT\CurrentVersion\AppCompatFlags\Layers`
	gpuKey    = `Software\Microsoft\DirectX\UserGpuPreferences`
	fsoFlag   = "DISABLEDXMAXIMIZEDWINDOWEDMODE"
	hpPlan    = "8c5e7fda-e8bf-4a96-9a85-a6e23a8c635c"
)

// ---------- Backup ----------

type regBackup struct {
	Root    string
	Path    string
	Name    string
	Existed bool
	Type    uint32
	Data    []byte
}

type backupData struct {
	Reg         []regBackup
	Power       string            // plano de energia que estava ativo antes
	CreatedPlan string            // copia do plano Alto desempenho criada por este app
	Video       map[string]string // cs2_video.txt original -> copia de seguranca
	Nvidia      []nvBackup        // configuracoes do perfil NVIDIA antes da mudanca
	PowerSet    []powerSettingBackup
}

var backup backupData

func appDir() string { return filepath.Join(os.Getenv("APPDATA"), "CS2Optimizer") }

func backupPath() string { return filepath.Join(appDir(), "restore-data.json") }

func loadBackup() {
	b, err := os.ReadFile(backupPath())
	if err != nil {
		return
	}
	var d backupData
	if json.Unmarshal(b, &d) == nil {
		backup = d
	}
}

func saveBackup() error {
	if err := os.MkdirAll(appDir(), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(backup, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(backupPath(), b, 0o644)
}

func hasBackup() bool {
	return len(backup.Reg) > 0 || backup.Power != "" || len(backup.Video) > 0 || len(backup.Nvidia) > 0 || len(backup.PowerSet) > 0
}

// setReg grava um valor guardando antes o valor ORIGINAL (so na primeira vez).
func setReg(root, path, name string, typ uint32, data []byte) error {
	known := false
	for _, e := range backup.Reg {
		if e.Root == root && strings.EqualFold(e.Path, path) && strings.EqualFold(e.Name, name) {
			known = true
			break
		}
	}
	if !known {
		old := regRead(rootOf(root), path, name)
		backup.Reg = append(backup.Reg, regBackup{root, path, name, old.Exists, old.Type, old.Data})
		if err := saveBackup(); err != nil {
			return fmt.Errorf("não foi possível salvar o backup: %v", err)
		}
	}
	return regWrite(rootOf(root), path, name, typ, data)
}

func restoreAll() []string {
	var errs []string
	for i := len(backup.Reg) - 1; i >= 0; i-- {
		e := backup.Reg[i]
		var err error
		if e.Existed {
			err = regWrite(rootOf(e.Root), e.Path, e.Name, e.Type, e.Data)
		} else {
			err = regDelete(rootOf(e.Root), e.Path, e.Name)
		}
		if err != nil {
			errs = append(errs, err.Error())
		}
	}
	errs = append(errs, restorePowerSettings()...) // antes de apagar a copia do plano
	if backup.Power != "" {
		if out, err := powercfg("/setactive", backup.Power); err != nil {
			errs = append(errs, "plano de energia: "+strings.TrimSpace(out))
		}
	}
	if backup.CreatedPlan != "" && !strings.EqualFold(backup.CreatedPlan, backup.Power) {
		powercfg("/delete", backup.CreatedPlan)
	}
	for orig, copyPath := range backup.Video {
		if b, err := os.ReadFile(copyPath); err == nil {
			if err := os.WriteFile(orig, b, 0o644); err != nil {
				errs = append(errs, "vídeo do CS2: "+err.Error())
				continue
			}
			os.Remove(copyPath)
		} else {
			errs = append(errs, "cópia do vídeo do CS2 não encontrada: "+copyPath)
		}
	}
	if err := restoreNvidia(); err != nil {
		errs = append(errs, "perfil NVIDIA: "+err.Error())
	}
	applyMouseFromRegistry()
	broadcastSettingChange()
	backup = backupData{}
	os.Remove(backupPath())
	return errs
}

// ---------- Utilidades ----------

func system32() string {
	root := os.Getenv("SystemRoot")
	if root == "" {
		root = `C:\Windows`
	}
	return filepath.Join(root, "System32")
}

func runHidden(exe string, args ...string) (string, error) {
	cmd := exec.Command(exe, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func powercfg(args ...string) (string, error) {
	return runHidden(filepath.Join(system32(), "powercfg.exe"), args...)
}

var guidRe = regexp.MustCompile(`[0-9a-fA-F]{8}-(?:[0-9a-fA-F]{4}-){3}[0-9a-fA-F]{12}`)
var vdfRe = regexp.MustCompile(`"path"\s+"([^"]+)"`)

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// ---------- Localizar o CS2 ----------

func findSteamDirs() []string {
	var steamDirs []string
	if v := regString(regRead(HKCU, `Software\Valve\Steam`, "SteamPath")); v != "" {
		steamDirs = append(steamDirs, strings.ReplaceAll(v, "/", `\`))
	}
	if v := regString(regRead(HKLM, `SOFTWARE\WOW6432Node\Valve\Steam`, "InstallPath")); v != "" {
		steamDirs = append(steamDirs, v)
	}
	return steamDirs
}

func findCS2() string {
	steamDirs := findSteamDirs()
	roots := append([]string{}, steamDirs...)
	for _, s := range steamDirs {
		b, err := os.ReadFile(filepath.Join(s, "steamapps", "libraryfolders.vdf"))
		if err != nil {
			continue
		}
		for _, m := range vdfRe.FindAllStringSubmatch(string(b), -1) {
			roots = append(roots, strings.ReplaceAll(m[1], `\\`, `\`))
		}
	}
	for _, r := range roots {
		p := filepath.Join(r, "steamapps", "common", "Counter-Strike Global Offensive", "game", "bin", "win64", "cs2.exe")
		if fileExists(p) {
			return p
		}
	}
	return ""
}

// ---------- Estado atual ----------

func hagsState() string {
	v, ok := regDword(regRead(HKLM, hagsKey, "HwSchMode"))
	switch {
	case ok && v == 2:
		return "ativado"
	case ok && v == 1:
		return "desativado"
	}
	return "padrão do Windows/driver"
}

func fsoDisabled() bool {
	if cs2Path == "" {
		return false
	}
	return strings.Contains(strings.ToUpper(regString(regRead(HKCU, layersKey, cs2Path))), fsoFlag)
}

// ---------- Ajustes ----------

func setHighPerformance() error {
	out, err := powercfg("/getactivescheme")
	if err != nil {
		return fmt.Errorf("powercfg falhou: %s", strings.TrimSpace(out))
	}
	cur := guidRe.FindString(out)
	if backup.Power == "" && cur != "" {
		backup.Power = cur
		saveBackup()
	}
	list, _ := powercfg("/list")
	lower := strings.ToLower(list)
	target := ""
	switch {
	case strings.Contains(lower, hpPlan):
		target = hpPlan
	case backup.CreatedPlan != "" && strings.Contains(lower, strings.ToLower(backup.CreatedPlan)):
		target = backup.CreatedPlan
	default:
		dup, err := powercfg("/duplicatescheme", hpPlan)
		g := guidRe.FindString(dup)
		if err != nil || g == "" {
			return errors.New("o plano Alto desempenho não existe neste PC (comum em notebooks com Modern Standby)")
		}
		target = g
		backup.CreatedPlan = g
		saveBackup()
	}
	if out, err := powercfg("/setactive", target); err != nil {
		return fmt.Errorf("powercfg falhou: %s", strings.TrimSpace(out))
	}
	return nil
}

func disableFSO() error {
	if cs2Path == "" {
		return errors.New("cs2.exe não encontrado")
	}
	cur := strings.TrimSpace(regString(regRead(HKCU, layersKey, cs2Path)))
	if strings.Contains(strings.ToUpper(cur), fsoFlag) {
		return nil
	}
	val := "~ " + fsoFlag
	if cur != "" {
		if !strings.HasPrefix(cur, "~") {
			cur = "~ " + cur
		}
		val = cur + " " + fsoFlag
	}
	return setReg("HKCU", layersKey, cs2Path, REG_SZ, strData(val))
}

func clearTemp() (int64, int) {
	limit := time.Now().Add(-48 * time.Hour)
	var freed int64
	var count int
	dirs := []string{os.TempDir()}
	if r := os.Getenv("SystemRoot"); r != "" {
		dirs = append(dirs, filepath.Join(r, "Temp"))
	}
	for _, d := range dirs {
		filepath.WalkDir(d, func(p string, de fs.DirEntry, err error) error {
			if err != nil || !de.Type().IsRegular() {
				return nil
			}
			info, err := de.Info()
			if err != nil || !info.ModTime().Before(limit) {
				return nil
			}
			if os.Remove(p) == nil {
				freed += info.Size()
				count++
			}
			return nil
		})
	}
	return freed, count
}

// ---------- Prioridade do CS2 ----------

func findProcesses(exe string) []uint32 {
	snap, _, _ := pCreateToolhelp32.Call(2, 0) // TH32CS_SNAPPROCESS
	if snap == ^uintptr(0) {
		return nil
	}
	defer pCloseHandle.Call(snap)
	var e processEntry32
	e.Size = uint32(unsafe.Sizeof(e))
	var pids []uint32
	r, _, _ := pProcess32FirstW.Call(snap, uintptr(unsafe.Pointer(&e)))
	for r != 0 {
		if strings.EqualFold(syscall.UTF16ToString(e.ExeFile[:]), exe) {
			pids = append(pids, e.ProcessID)
		}
		r, _, _ = pProcess32NextW.Call(snap, uintptr(unsafe.Pointer(&e)))
	}
	return pids
}

// boostCS2 coloca o cs2.exe em prioridade Alta (nunca Tempo real). Retorna quantos mudou.
func boostCS2() int {
	const high = 0x80
	n := 0
	for _, pid := range findProcesses("cs2.exe") {
		h, _, _ := pOpenProcess.Call(0x0200|0x1000, 0, uintptr(pid)) // SET_INFORMATION | QUERY_LIMITED_INFORMATION
		if h == 0 {
			continue
		}
		if pc, _, _ := pGetPriorityClass.Call(h); pc != high {
			if ok, _, _ := pSetPriorityClass.Call(h, high); ok != 0 {
				n++
			}
		}
		pCloseHandle.Call(h)
	}
	return n
}

// ---------- Lista de ajustes ----------

type tweak struct {
	label       string
	desc        string
	recommended bool
	needsCS2    bool
	apply       func() error
}

var tweaks = []tweak{
	{
		label:       "Plano de energia: Alto desempenho",
		desc:        "Evita que a CPU reduza o clock durante o jogo. Em notebook, gasta mais bateria.",
		recommended: true,
		apply:       setHighPerformance,
	},
	{
		label:       "Ativar o Modo de Jogo do Windows",
		desc:        "Prioriza o jogo e evita atualizações e reinícios do Windows durante a partida.",
		recommended: true,
		apply: func() error {
			if err := setReg("HKCU", `Software\Microsoft\GameBar`, "AutoGameModeEnabled", REG_DWORD, dwordData(1)); err != nil {
				return err
			}
			return setReg("HKCU", `Software\Microsoft\GameBar`, "AllowAutoGameMode", REG_DWORD, dwordData(1))
		},
	},
	{
		label:       "Desativar a gravação em segundo plano (Game DVR)",
		desc:        "Remove a captura automática, que custa FPS. A Game Bar continua funcionando.",
		recommended: true,
		apply: func() error {
			if err := setReg("HKCU", `System\GameConfigStore`, "GameDVR_Enabled", REG_DWORD, dwordData(0)); err != nil {
				return err
			}
			return setReg("HKCU", `Software\Microsoft\Windows\CurrentVersion\GameDVR`, "AppCaptureEnabled", REG_DWORD, dwordData(0))
		},
	},
	{
		label:       "Usar a placa de vídeo dedicada no CS2",
		desc:        "Para PCs com vídeo integrado + placa dedicada (comum em notebooks).",
		recommended: true,
		needsCS2:    true,
		apply: func() error {
			if cs2Path == "" {
				return errors.New("cs2.exe não encontrado")
			}
			return setReg("HKCU", gpuKey, cs2Path, REG_SZ, strData("GpuPreference=2;"))
		},
	},
	{
		label:    "Desativar as otimizações de tela cheia no cs2.exe",
		desc:     "Pode reduzir o input lag em alguns PCs.",
		needsCS2: true,
		apply:    disableFSO,
	},
	{
		label:       "Colocar o CS2 em prioridade Alta",
		desc:        "Vale enquanto este app estiver aberto (pode minimizar). Nunca usa Tempo real.",
		recommended: true,
		apply: func() error {
			monitorOn.Store(true)
			return nil
		},
	},
	{
		label: "Limpar arquivos temporários antigos",
		desc:  "Apaga só temporários com mais de 2 dias. Isso não é desfeito pelo Restaurar.",
		apply: func() error {
			freed, n := clearTemp()
			logf("%d arquivos temporários apagados (%.0f MB liberados).", n, float64(freed)/1048576)
			return nil
		},
	},
	{
		label:       "Desligar a suspensão seletiva de USB",
		desc:        "Evita que o Windows desligue portas USB para economizar energia (microtravadas no mouse).",
		recommended: true,
		apply:       func() error { return setPowerSetting(subUSB, setUSBSusp, 0) },
	},
	{
		label: "Desligar a economia de energia do PCIe",
		desc:  "A placa de vídeo e o SSD não entram em modo econômico durante o jogo. Em notebook, gasta mais bateria.",
		apply: func() error { return setPowerSetting(subPCIe, setPCIeASPM, 0) },
	},
	{
		label: "Prioridade de jogos no agendador multimídia",
		desc:  "Ajusta o MMCSS (SystemResponsiveness e tarefa \"Games\"). Ganho pequeno; não atrapalha nada.",
		apply: func() error {
			base := `SOFTWARE\Microsoft\Windows NT\CurrentVersion\Multimedia\SystemProfile`
			games := base + `\Tasks\Games`
			for _, op := range []struct {
				path, name string
				typ        uint32
				data       []byte
			}{
				{base, "SystemResponsiveness", REG_DWORD, dwordData(10)},
				{base, "NetworkThrottlingIndex", REG_DWORD, dwordData(0xFFFFFFFF)},
				{games, "GPU Priority", REG_DWORD, dwordData(8)},
				{games, "Priority", REG_DWORD, dwordData(6)},
				{games, "Scheduling Category", REG_SZ, strData("High")},
				{games, "SFIO Priority", REG_SZ, strData("High")},
			} {
				if err := setReg("HKLM", op.path, op.name, op.typ, op.data); err != nil {
					return err
				}
			}
			return nil
		},
	},
	{
		label: "Permitir timer de 0,5 ms para o CS2 (Windows 11)",
		desc:  "Necessário para o \"Timer de 0,5 ms\" da aba Extras valer para o jogo no Windows 11. Exige reiniciar.",
		apply: func() error {
			return setReg("HKLM", `SYSTEM\CurrentControlSet\Control\Session Manager\kernel`, "GlobalTimerResolutionRequests", REG_DWORD, dwordData(1))
		},
	},
	{
		label: "Desligar os efeitos de transparência",
		desc:  "Barra de tarefas e menus ficam opacos. Alivia um pouco a placa de vídeo em PCs mais fracos.",
		apply: func() error {
			if err := setReg("HKCU", `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, "EnableTransparency", REG_DWORD, dwordData(0)); err != nil {
				return err
			}
			broadcastSettingChange()
			return nil
		},
	},
	{
		label: "Desligar a aceleração do mouse no Windows",
		desc:  "\"Aumentar precisão do ponteiro\" desligado. O CS2 já usa entrada direta; muda só fora do jogo.",
		apply: func() error {
			for _, n := range []string{"MouseSpeed", "MouseThreshold1", "MouseThreshold2"} {
				if err := setReg("HKCU", `Control Panel\Mouse`, n, REG_SZ, strData("0")); err != nil {
					return err
				}
			}
			applyMouseFromRegistry()
			return nil
		},
	},
	{
		label: "Limpar o cache de shaders da placa de vídeo",
		desc:  "Use se o jogo engasgar depois de atualizar o driver. A 1ª partida refaz o cache. Não é desfeito.",
		apply: func() error {
			freed, n := clearShaderCaches()
			logf("Cache de shaders limpo: %d arquivos (%.0f MB).", n, float64(freed)/1048576)
			return nil
		},
	},
}

const fsoIndex = 4
