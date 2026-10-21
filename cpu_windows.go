package main

// Processadores hibridos (Intel 12a, 13a e 14a geracao: nucleos P + nucleos E).
// A classificacao vem do proprio Windows (EfficiencyClass de cada nucleo), sem tabela de modelos.

import (
	"encoding/binary"
	"fmt"
	"strings"
	"unsafe"
)

var pGetLogicalProcInfoEx = kernel32.NewProc("GetLogicalProcessorInformationEx")
var pGetProcessAffinityMask = kernel32.NewProc("GetProcessAffinityMask")

// readTopology le os nucleos fisicos (RelationProcessorCore) e separa por classe de eficiencia.
func readTopology() cpuTopology {
	var size uint32
	pGetLogicalProcInfoEx.Call(0, 0, uintptr(unsafe.Pointer(&size)))
	if size == 0 || size > 1<<20 {
		return cpuTopology{}
	}
	buf := make([]byte, size)
	if r, _, _ := pGetLogicalProcInfoEx.Call(0, uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&size))); r == 0 {
		return cpuTopology{}
	}
	return parseTopology(buf[:size])
}

// ---------- apps de fundo nos nucleos E ----------

var backgroundProcs = []string{
	"Discord.exe", "Spotify.exe", "steamwebhelper.exe", "OneDrive.exe", "ms-teams.exe",
	"chrome.exe", "msedge.exe", "firefox.exe", "opera.exe", "brave.exe",
}

var (
	eCoreMask    uint64                // 0 = desligado
	eCoreMovedTo = map[uint32]uint64{} // pid -> mascara original
)

// moveBackgroundToECores e chamado pelo timer enquanto a opcao estiver ativa
func moveBackgroundToECores() {
	if eCoreMask == 0 {
		return
	}
	moved := 0
	for _, name := range backgroundProcs {
		for _, pid := range findProcesses(name) {
			if _, done := eCoreMovedTo[pid]; done {
				continue
			}
			h, _, _ := pOpenProcess.Call(0x0200|0x0400, 0, uintptr(pid)) // SET_INFORMATION | QUERY_INFORMATION
			if h == 0 {
				eCoreMovedTo[pid] = 0
				continue
			}
			var procMask, sysMask uintptr
			okMask, _, _ := pGetProcessAffinityMask.Call(h, uintptr(unsafe.Pointer(&procMask)), uintptr(unsafe.Pointer(&sysMask)))
			orig := uint64(procMask)
			// processos filhos herdam a mascara do pai ja movido: nesse caso o "original" e o sistema todo
			if okMask == 0 || orig == 0 || orig&^eCoreMask == 0 {
				orig = uint64(sysMask)
			}
			if ok, _, _ := pSetProcessAffinityMask.Call(h, uintptr(eCoreMask)); ok != 0 && orig != 0 {
				eCoreMovedTo[pid] = orig
				moved++
			} else {
				eCoreMovedTo[pid] = 0
			}
			pCloseHandle.Call(h)
		}
	}
	if moved > 0 {
		logf("%d app(s) de fundo movido(s) para os núcleos E.", moved)
	}
}

// restoreBackgroundAffinity devolve os apps aos nucleos originais (ao restaurar ou fechar o app)
func restoreBackgroundAffinity() {
	for pid, orig := range eCoreMovedTo {
		if orig == 0 {
			continue
		}
		if h, _, _ := pOpenProcess.Call(0x0200|0x0400, 0, uintptr(pid)); h != 0 {
			pSetProcessAffinityMask.Call(h, uintptr(orig))
			pCloseHandle.Call(h)
		}
	}
	eCoreMovedTo = map[uint32]uint64{}
	eCoreMask = 0
}

// ---------- microcodigo dos 13a/14a geracao ----------

const intelFixedMicrocode = 0x12F // correcao mais recente da Intel para a instabilidade "Vmin Shift" (maio/2025)

// microcodeWarning devolve um aviso se for um Intel 13a/14a geracao de desktop com microcodigo antigo
func microcodeWarning() string {
	name := cpuName()
	if !raptorDesktopRe.MatchString(name) {
		return ""
	}
	v := regRead(HKLM, `HARDWARE\DESCRIPTION\System\CentralProcessor\0`, "Update Revision")
	if !v.Exists || len(v.Data) < 8 {
		return ""
	}
	rev := binary.LittleEndian.Uint32(v.Data[4:8])
	if rev == 0 || rev >= intelFixedMicrocode {
		return ""
	}
	short := strings.TrimSpace(name[strings.Index(strings.ToLower(name), "gen intel")+3:])
	return fmt.Sprintf("Atenção: %s com microcódigo 0x%X. Atualize a BIOS da placa-mãe para uma versão com o microcódigo 0x12F da Intel (correção da instabilidade dos 13ª/14ª geração).", short, rev)
}
