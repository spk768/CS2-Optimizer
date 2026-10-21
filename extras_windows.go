package main

// Aba "Extras": fechar programas ao jogar e afinidade de CPU para Ryzen X3D.

import (
	"fmt"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
)

var extraOptions = []struct {
	label, desc string
	recommended bool
}{
	{"Fechar outros launchers de jogos", "Epic, Battle.net, EA app, Ubisoft Connect e GOG Galaxy, ao clicar em \"Jogar\".", true},
	{"Fechar navegadores", "Chrome, Edge, Firefox, Opera e Brave. As abas voltam quando você reabrir.", false},
	{"Prender o CS2 nos núcleos mais rápidos", "Ryzen X3D com 2 CCDs: núcleos com 3D V-Cache. Intel 12ª–14ª ger.: só núcleos P.", true},
	{"Mover apps de fundo para os núcleos E", "Discord, Spotify, Steam, navegadores, OneDrive e Teams. Voltam ao normal ao fechar o app.", false},
	{"Vibrância digital 80% durante o jogo (NVIDIA)", "Cores mais vivas só com o CS2 em primeiro plano. Ao sair do jogo, volta ao normal.", false},
	{"Limpar a memória em espera durante o jogo", "Como o ISLC: libera a RAM em cache quando a livre cai abaixo de 1 GB. Útil com 16 GB ou menos.", false},
	{"Timer de 0,5 ms durante o jogo", "Pode deixar os frames mais regulares. Precisa também da opção \"Permitir timer\" da aba Windows.", false},
}

const (
	extraLaunchers = 0
	extraBrowsers  = 1
	extraX3D       = 2
	extraECores    = 3
	extraVibrance  = 4
	extraStandby   = 5
	extraTimer     = 6
)

var (
	launcherProcs = []string{"EpicGamesLauncher.exe", "Battle.net.exe", "EADesktop.exe", "upc.exe", "UbisoftConnect.exe", "GalaxyClient.exe"}
	browserProcs  = []string{"chrome.exe", "msedge.exe", "firefox.exe", "opera.exe", "brave.exe"}

	x3dMask        atomic.Uint64 // mascara pendente para o cs2.exe (0 = nao aplicar)
	x3dAppliedPIDs = map[uint32]bool{}
)

func cpuName() string {
	return strings.TrimSpace(regString(regRead(HKLM, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "ProcessorNameString")))
}

// x3dDualCCD: Ryzen X3D com dois CCDs (7900X3D, 7950X3D, 9900X3D, 9950X3D...).
// Nesses modelos o CCD0 (primeira metade dos processadores logicos) tem o 3D V-Cache.
func x3dDualCCD() (bool, uint64) {
	name := strings.ToUpper(cpuName())
	n := runtime.NumCPU()
	if !strings.Contains(name, "X3D") || n < 24 || n > 64 {
		return false, 0
	}
	half := n / 2
	return true, (uint64(1) << uint(half)) - 1
}

// closePrograms fecha os programas escolhidos; navegadores recebem fechamento normal
func closePrograms(sel []bool) {
	taskkill := filepath.Join(system32(), "taskkill.exe")
	closed := 0
	run := func(names []string, force bool) {
		for _, n := range names {
			if len(findProcesses(n)) == 0 {
				continue
			}
			args := []string{"/IM", n, "/T"}
			if force {
				args = append([]string{"/F"}, args...)
			}
			if _, err := runHidden(taskkill, args...); err == nil {
				closed++
			}
		}
	}
	if sel[extraLaunchers] {
		run(launcherProcs, true)
	}
	if sel[extraBrowsers] {
		run(browserProcs, false)
	}
	if closed > 0 {
		logf("%d programa(s) fechado(s) antes do jogo.", closed)
	}
}

// applyX3DAffinity e chamado pelo timer: prende cada cs2.exe novo no CCD com V-Cache
func applyX3DAffinity() {
	mask := x3dMask.Load()
	if mask == 0 {
		return
	}
	for _, pid := range findProcesses("cs2.exe") {
		if x3dAppliedPIDs[pid] {
			continue
		}
		h, _, _ := pOpenProcess.Call(0x0200|0x1000, 0, uintptr(pid))
		if h == 0 {
			continue
		}
		ok, _, _ := pSetProcessAffinityMask.Call(h, uintptr(mask))
		pCloseHandle.Call(h)
		x3dAppliedPIDs[pid] = true
		if ok != 0 {
			logf("CS2 preso nos núcleos mais rápidos.")
		}
	}
}

var pSetProcessAffinityMask = kernel32.NewProc("SetProcessAffinityMask")

// fastCores descreve onde o CS2 deve rodar neste PC (X3D com 2 CCDs ou Intel hibrido)
func fastCores() (desc string, mask uint64, hybrid cpuTopology) {
	if ok, m := x3dDualCCD(); ok {
		return "Ryzen X3D detectado: o CS2 roda só no CCD com 3D V-Cache.", m, cpuTopology{}
	}
	t := readTopology()
	if t.hybrid {
		return fmt.Sprintf("Intel híbrido detectado (%d núcleos P + %d E): o CS2 roda só nos núcleos P.", t.pCores, t.eCores), t.pMask, t
	}
	return "", 0, t
}

// windows11: build 22000 ou maior
func windows11() bool {
	b := regString(regRead(HKLM, `SOFTWARE\Microsoft\Windows NT\CurrentVersion`, "CurrentBuildNumber"))
	n := 0
	for _, c := range b {
		if c < '0' || c > '9' {
			break
		}
		n = n*10 + int(c-'0')
	}
	return n >= 22000
}
