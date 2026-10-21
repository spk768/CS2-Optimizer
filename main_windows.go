package main

// CS2 Optimizer - janela com interface "liquid glass" desenhada pelo proprio app.

import (
	"fmt"
	"math"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

const appTitle = "CS2 Optimizer"

const (
	wmLog     = WM_APP + 1
	wmDone    = WM_APP + 2
	wmPresent = WM_APP + 3
	wmBench   = WM_APP + 4
	timerFast = 3
	timerMon  = 1
	timerAnim = 2
)

var (
	hInst, hwndMain uintptr
	dpi             = 96
	kScale          = 1.0
	memDC, layDC    uintptr
	layPix          []byte
	frame           *windowFrame
	presentPending  bool
	cv              *canvas
	bases           [4]*canvas
	startList       []startupEntry
	settings        appSettings
	st              *uiState
	fonts           [fontCount]uintptr
	animating       bool
	tracking        bool
	cs2Path         string
	monitorOn, busy atomic.Bool
	logMu           sync.Mutex
	logQueue        []string
	cursorArrow     uintptr
	cursorHand      uintptr
)

func init() { runtime.LockOSThread() }

func makeFont(pt float64, weight int, face string) uintptr {
	h := -int(math.Round(pt * float64(dpi) / 72))
	f, _, _ := pCreateFontW.Call(uintptr(h), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(u16(face))))
	return f
}

func colorref(v uint32) uintptr {
	return uintptr((v>>16)&0xFF | v&0xFF00 | (v&0xFF)<<16)
}

// ---------- Log ----------

// logf pode ser chamado de qualquer thread.
func logf(format string, a ...any) {
	line := fmt.Sprintf("[%s] %s", time.Now().Format("15:04:05"), fmt.Sprintf(format, a...))
	logMu.Lock()
	logQueue = append(logQueue, line)
	logMu.Unlock()
	pPostMessageW.Call(hwndMain, wmLog, 0, 0)
}

func drainLog() {
	logMu.Lock()
	lines := logQueue
	logQueue = nil
	logMu.Unlock()
	if len(lines) > 0 {
		st.logLines = append(st.logLines, lines...)
		st.logScroll = 0
		invalidate()
	}
}

// ---------- Estado ----------

func refreshRows() {
	for i, t := range tweaks {
		desc := t.desc
		if i == fsoIndex && cs2Path != "" {
			if fsoDisabled() {
				desc += "  ·  Atual: desativadas"
			} else {
				desc += "  ·  Atual: ativadas"
			}
		}
		st.rows[i] = rowInfo{t.label, desc, t.recommended}
	}
	st.hagsState = hagsState()
	st.videoState, st.videoAccounts = videoStatus()
	st.resState = resolutionStatus()
}

func refreshNvidia() {
	if st.busy {
		return // o "Aplicar" esta usando o driver agora
	}
	st.nvAvailable, st.nvState = nvidiaStatus()
}

// invalidate agenda um redesenho (varias chamadas seguidas viram um so)
func invalidate() {
	if !presentPending {
		presentPending = true
		pPostMessageW.Call(hwndMain, wmPresent, 0, 0)
	}
}

func kick() {
	if !animating {
		animating = true
		pSetTimer.Call(hwndMain, timerAnim, 15, 0)
	}
	invalidate()
}

func setBusy(b bool) {
	busy.Store(b)
	st.busy = b
	kick()
}

// ---------- Desenho ----------

func setupSurface() {
	w, h := int(math.Round(uiW*kScale)), int(math.Round(uiH*kScale))
	bmi := bitmapInfo{Width: int32(w), Height: -int32(h), Planes: 1, BitCount: 32}
	bmi.Size = 40
	screen, _, _ := pGetDC.Call(0)
	memDC, _, _ = pCreateCompatibleDC.Call(screen)
	var bits unsafe.Pointer
	dib, _, _ := pCreateDIBSection.Call(screen, uintptr(unsafe.Pointer(&bmi)), 0, uintptr(unsafe.Pointer(&bits)), 0, 0)
	pReleaseDC.Call(0, screen)
	if dib == 0 || bits == nil {
		panic("não foi possível criar a superfície de desenho")
	}
	pSelectObject.Call(memDC, dib)
	cv = &canvas{w: w, h: h, pix: unsafe.Slice((*byte)(bits), w*h*4)}
	bases = buildBases(w, h, kScale, len(tweaks))

	// superficie da janela inteira (com margem para a sombra), com transparencia por pixel
	frame = newWindowFrame(w, h, kScale)
	lbmi := bitmapInfo{Width: int32(frame.W), Height: -int32(frame.H), Planes: 1, BitCount: 32}
	lbmi.Size = 40
	screen, _, _ = pGetDC.Call(0)
	layDC, _, _ = pCreateCompatibleDC.Call(screen)
	var lbits unsafe.Pointer
	ldib, _, _ := pCreateDIBSection.Call(screen, uintptr(unsafe.Pointer(&lbmi)), 0, uintptr(unsafe.Pointer(&lbits)), 0, 0)
	pReleaseDC.Call(0, screen)
	if ldib == 0 || lbits == nil {
		panic("não foi possível criar a superfície da janela")
	}
	pSelectObject.Call(layDC, ldib)
	layPix = unsafe.Slice((*byte)(lbits), frame.W*frame.H*4)
}

func present() {
	presentPending = false
	pGdiFlush.Call()
	items := renderScene(cv, bases, st, kScale)
	pSetBkMode.Call(memDC, 1)
	for _, it := range items {
		pSelectObject.Call(memDC, fonts[it.font])
		pSetTextColor.Call(memDC, colorref(it.color))
		r := rect{int32(it.x), int32(it.y), int32(it.x + it.w), int32(it.y + it.h)}
		flags := uintptr(DT_SINGLELINE | DT_VCENTER | DT_NOPREFIX | DT_END_ELLIPSIS)
		switch it.align {
		case alCenter:
			flags |= DT_CENTER
		case alRight:
			flags |= DT_RIGHT
		}
		pDrawTextW.Call(memDC, uintptr(unsafe.Pointer(u16(it.s))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags)
	}
	pGdiFlush.Call()
	frame.compose(cv.pix, layPix)
	size := [2]int32{int32(frame.W), int32(frame.H)}
	src := point{0, 0}
	blend := uint32(255<<16 | 1<<24) // AC_SRC_OVER, alpha constante 255, AC_SRC_ALPHA
	screen, _, _ := pGetDC.Call(0)
	pUpdateLayeredWin.Call(hwndMain, screen, 0, uintptr(unsafe.Pointer(&size)), layDC, uintptr(unsafe.Pointer(&src)), 0, uintptr(unsafe.Pointer(&blend)), 2) // ULW_ALPHA
	pReleaseDC.Call(0, screen)
}

// ---------- Acoes ----------

func onApply() {
	var selected []int
	for i, on := range st.selected {
		if on && st.enabled[i] {
			selected = append(selected, i)
		}
	}
	hags := st.hagsSel
	vid := st.vidSel
	res := st.resSel
	var nvWhich []int
	for i, on := range st.nvSel {
		if on && st.nvAvailable {
			nvWhich = append(nvWhich, i)
		}
	}
	if len(selected) == 0 && hags == 0 && vid == 0 && res == 0 && len(nvWhich) == 0 {
		msgBox(hwndMain, "Nenhuma opção selecionada.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	setBusy(true)
	go func() {
		reboot := uintptr(0)
		logf("Criando ponto de restauração do Windows (pode levar alguns segundos)...")
		if err := createRestorePoint(); err != nil {
			logf("Ponto de restauração não criado (proteção do sistema desligada?). Seguindo; o Restaurar tudo continua valendo.")
		} else {
			logf("Ponto de restauração pronto.")
		}
		for _, i := range selected {
			t := tweaks[i]
			if err := t.apply(); err != nil {
				logf("FALHOU: %s - %v", t.label, err)
			} else {
				logf("OK: %s", t.label)
			}
		}
		if vid > 0 {
			if err := applyVideoProfile(vid); err != nil {
				logf("FALHOU: perfil de vídeo - %v", err)
			}
		}
		if res > 0 {
			if err := applyResolution(res); err != nil {
				logf("FALHOU: resolução - %v", err)
			}
		}
		if len(nvWhich) > 0 {
			if err := applyNvidia(nvWhich); err != nil {
				logf("FALHOU: perfil NVIDIA - %v", err)
			} else {
				logf("OK: perfil NVIDIA do cs2.exe atualizado.")
			}
		}
		if hags == 1 || hags == 2 {
			v, name := uint32(2), "ativado"
			if hags == 2 {
				v, name = 1, "desativado"
			}
			if err := setReg("HKLM", hagsKey, "HwSchMode", REG_DWORD, dwordData(v)); err != nil {
				logf("FALHOU: HAGS - %v", err)
			} else {
				logf("OK: HAGS %s (vale após reiniciar).", name)
				reboot = 1
			}
		}
		if err := saveBackup(); err != nil {
			logf("Aviso: não foi possível salvar o backup: %v", err)
		}
		logf("Concluído.")
		pPostMessageW.Call(hwndMain, wmDone, reboot, 0)
	}()
}

func onRestore() {
	if !hasBackup() {
		msgBox(hwndMain, "Não há alterações deste app para desfazer.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	if len(backup.Video) > 0 && cs2Running() {
		msgBox(hwndMain, "Feche o CS2 antes de restaurar (ele regrava o arquivo de vídeo ao sair).", appTitle, MB_OK|MB_ICONWARNING)
		return
	}
	if msgBox(hwndMain, "Desfazer todas as alterações feitas por este app?", appTitle, MB_YESNO|MB_ICONQUESTION) != IDYES {
		return
	}
	hagsChanged := false
	for _, e := range backup.Reg {
		if e.Name == "HwSchMode" {
			hagsChanged = true
		}
	}
	monitorOn.Store(false)
	for _, e := range restoreAll() {
		logf("Falha ao restaurar: %s", e)
	}
	refreshRows()
	refreshNvidia()
	x3dMask.Store(0)
	restoreBackgroundAffinity()
	requestTimer(false)
	logf("Configurações originais restauradas.")
	invalidate()
	if hagsChanged {
		msgBox(hwndMain, "O HAGS volta ao estado original depois de reiniciar o PC.", appTitle, MB_OK|MB_ICONINFORMATION)
	}
}

func onPlay() {
	if launchGame(nil) {
		logf("Abrindo o CS2 pela Steam... Deixe este app aberto (pode minimizar) para manter as otimizações.")
	}
}

func onBenchmark() {
	if st.benchRunning || st.busy {
		return
	}
	if !st.cs2Found {
		msgBox(hwndMain, "O CS2 não foi encontrado na Steam deste PC.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	if !benchMapInstalled() {
		openBenchMapPage()
		msgBox(hwndMain, "Falta o mapa \"CS2 FPS Benchmark\" da Oficina.\n\nA página dele vai abrir na Steam: clique em \"Inscrever-se\", espere baixar e clique em \"Rodar benchmark\" de novo.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	if cs2Running() {
		msgBox(hwndMain, "Feche o CS2 antes de rodar o benchmark.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	extra, err := prepareBenchmark()
	if err != nil {
		logf("Benchmark: %v", err)
		return
	}
	if !launchGame(extra) {
		return
	}
	st.benchRunning = true
	st.benchLines = []string{"Medindo... não mexa no PC até o CS2 fechar sozinho.", "Leva uns 2 minutos depois que o mapa carregar."}
	logf("Benchmark iniciado: o CS2 vai abrir no mapa de teste e fechar sozinho no fim.")
	go waitBenchmark(func(r benchResult, err error) {
		if err != nil {
			logf("Benchmark: %v", err)
		} else {
			logf("Benchmark: %.0f FPS médio, 1%% low %.0f.", r.Avg, r.P1)
		}
		pPostMessageW.Call(hwndMain, wmBench, 0, 0)
	})
	kick()
}

func onPasteCrosshair() {
	text := clipboardText()
	code := extractCrosshairCode(text)
	if code == "" {
		msgBox(hwndMain, "Nenhum código de mira na área de transferência.\n\nCopie um código como CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx (ou o formato novo que começa com CS) e clique de novo.", appTitle, MB_OK|MB_ICONINFORMATION)
		return
	}
	settings.Crosshair = code
	saveSettings(settings)
	st.crosshair = code
	logf("Mira salva (%s). Ela é aplicada quando você abrir o CS2 pelo app.", code)
}

// launchGame abre o CS2 com as opcoes das abas Jogo e Extras (e argumentos extras)
func launchGame(extra []string) bool {
	if !st.cs2Found {
		msgBox(hwndMain, "O CS2 não foi encontrado na Steam deste PC.", appTitle, MB_OK|MB_ICONINFORMATION)
		return false
	}
	if cs2Running() {
		msgBox(hwndMain, "O CS2 já está aberto. Feche e clique de novo para usar as opções.", appTitle, MB_OK|MB_ICONINFORMATION)
		return false
	}
	closePrograms(st.exSel)
	if err := launchCS2(st.gameSel, extra); err != nil {
		logf("Não foi possível abrir o CS2: %v", err)
		return false
	}
	requestTimer(st.exSel[extraTimer])
	monitorOn.Store(true)
	if _, mask, _ := fastCores(); mask != 0 && st.exSel[extraX3D] {
		x3dMask.Store(mask)
	} else {
		x3dMask.Store(0)
	}
	if t := readTopology(); t.hybrid && st.exSel[extraECores] {
		eCoreMask = t.eMask
		moveBackgroundToECores()
	} else {
		restoreBackgroundAffinity()
	}
	return true
}

func onClick(id int) {
	switch {
	case id >= idRow0 && id < idRow0+len(st.rows):
		i := id - idRow0
		if !st.busy && st.enabled[i] {
			st.selected[i] = !st.selected[i]
		}
	case id >= idSeg0 && id < idSeg0+3:
		if !st.busy {
			st.hagsSel = id - idSeg0
		}
	case id == idTab0+3:
		st.tab = 3
		refreshStartup()
		if !st.benchRunning {
			st.benchLines = benchSummary()
		}
	case id >= idStart0 && id < idStart0+len(st.startRows):
		i := id - idStart0
		if !st.busy && i < len(startList) {
			on := !st.startOn[i]
			if err := setStartupEnabled(startList[i], on); err != nil {
				logf("Não foi possível mudar %s: %v", startList[i].name, err)
			} else {
				st.startOn[i] = on
				state := "desativado"
				if on {
					state = "ativado"
				}
				logf("%s %s na inicialização.", startList[i].name, state)
			}
		}
	case id == idBench:
		onBenchmark()
	case id == idPaste:
		onPasteCrosshair()
	case id == idClearX:
		settings.Crosshair = ""
		saveSettings(settings)
		st.crosshair = ""
		logf("Mira removida das opções do app (a mira atual do jogo continua até você mudar).")
	case id >= idTab0 && id < idTab0+2:
		st.tab = id - idTab0
		if st.tab == 1 {
			st.videoState, st.videoAccounts = videoStatus()
		}
	case id >= idVid0 && id < idVid0+3:
		if !st.busy {
			st.vidSel = id - idVid0
		}
	case id >= idGame0 && id < idGame0+len(st.gameRows):
		if !st.busy {
			i := id - idGame0
			st.gameSel[i] = !st.gameSel[i]
			settings.GameOptions = append([]bool{}, st.gameSel...)
			saveSettings(settings)
		}
	case id >= idTab0 && id < idTab0+3 && id == idTab0+2:
		st.tab = 2
		refreshNvidia()
		st.resState = resolutionStatus()
	case id >= idNv0 && id < idNv0+len(st.nvSel):
		if !st.busy && st.nvAvailable {
			st.nvSel[id-idNv0] = !st.nvSel[id-idNv0]
		}
	case id >= idEx0 && id < idEx0+len(st.exSel):
		if !st.busy && st.exEnabled[id-idEx0] {
			st.exSel[id-idEx0] = !st.exSel[id-idEx0]
			settings.ExtraOptions = append([]bool{}, st.exSel...)
			saveSettings(settings)
		}
	case id >= idRes0 && id < idRes0+3:
		if !st.busy {
			st.resSel = id - idRes0
		}
	case id == idPlay:
		if !st.busy {
			onPlay()
		}
	case id == idBtnRec:
		if !st.busy {
			for i := range st.selected {
				st.selected[i] = st.enabled[i] && st.rows[i].recommended
			}
			if st.cs2Found {
				st.vidSel = 2
			}
			for i := range st.gameSel {
				st.gameSel[i] = gameOptions[i].recommended
			}
			for i := range st.nvSel {
				st.nvSel[i] = st.nvAvailable
			}
			for i := range st.exSel {
				st.exSel[i] = st.exEnabled[i] && extraOptions[i].recommended
			}
			settings.GameOptions = append([]bool{}, st.gameSel...)
			settings.ExtraOptions = append([]bool{}, st.exSel...)
			saveSettings(settings)
		}
	case id == idBtnClear:
		if !st.busy {
			for i := range st.selected {
				st.selected[i] = false
			}
			st.hagsSel = 0
			st.vidSel = 0
			st.resSel = 0
			for i := range st.nvSel {
				st.nvSel[i] = false
			}
		}
	case id == idBtnRest:
		if !st.busy {
			onRestore()
		}
	case id == idBtnApply:
		if !st.busy {
			onApply()
		}
	case id == idMin:
		st.hoverID = 0
		pShowWindow.Call(hwndMain, 6) // SW_MINIMIZE
	case id == idClose:
		pPostMessageW.Call(hwndMain, WM_CLOSE, 0, 0)
	}
	kick()
}

// ---------- Janela ----------

func mousePos(l uintptr) (float64, float64) {
	x := float64(int16(l&0xFFFF)) - float64(frame.M)
	y := float64(int16((l>>16)&0xFFFF)) - float64(frame.M)
	return x / kScale, y / kScale
}

func wndProc(h, msg, w, l uintptr) uintptr {
	switch msg {
	case WM_PAINT:
		pValidateRect.Call(h, 0)
		present()
		return 0
	case wmPresent:
		present()
		return 0
	case WM_NCHITTEST:
		pt := point{int32(int16(l & 0xFFFF)), int32(int16((l >> 16) & 0xFFFF))}
		pScreenToClient.Call(h, uintptr(unsafe.Pointer(&pt)))
		if inDragArea(float64(int(pt.X)-frame.M)/kScale, float64(int(pt.Y)-frame.M)/kScale) {
			return 2 // HTCAPTION: arrasta a janela
		}
		return 1 // HTCLIENT
	case WM_ERASEBKGND:
		return 1
	case WM_MOUSEMOVE:
		if !tracking {
			tme := trackMouseEvent{Flags: 2, HwndTrack: h} // TME_LEAVE
			tme.Size = uint32(unsafe.Sizeof(tme))
			pTrackMouseEvent.Call(uintptr(unsafe.Pointer(&tme)))
			tracking = true
		}
		x, y := mousePos(l)
		if id := hitTest(x, y, st); id != st.hoverID {
			st.hoverID = id
			kick()
		}
		return 0
	case WM_MOUSELEAVE:
		tracking = false
		if st.hoverID != 0 {
			st.hoverID = 0
			kick()
		}
		return 0
	case WM_LBUTTONDOWN:
		x, y := mousePos(l)
		st.pressed = hitTest(x, y, st)
		pSetCapture.Call(h)
		kick()
		return 0
	case WM_LBUTTONUP:
		pReleaseCapture.Call()
		x, y := mousePos(l)
		id := hitTest(x, y, st)
		pressed := st.pressed
		st.pressed = 0
		if id == pressed && clickable(id) {
			onClick(id)
		}
		kick()
		return 0
	case WM_MOUSEWHEEL:
		pt := point{int32(int16(l & 0xFFFF)), int32(int16((l >> 16) & 0xFFFF))}
		pScreenToClient.Call(h, uintptr(unsafe.Pointer(&pt)))
		px, py := float64(int(pt.X)-frame.M)/kScale, float64(int(pt.Y)-frame.M)/kScale
		if d := int16((w >> 16) & 0xFFFF); d != 0 {
			step := 1
			if d < 0 {
				step = -1
			}
			if scrollList(px, py, step, st) {
				st.hoverID = hitTest(px, py, st)
				kick()
				return 0
			}
		}
		if hitTest(px, py, st) == idLogArea {
			delta := int16((w >> 16) & 0xFFFF)
			maxScroll := len(st.logLines) - 3
			if delta > 0 && st.logScroll < maxScroll {
				st.logScroll++
			} else if delta < 0 && st.logScroll > 0 {
				st.logScroll--
			}
			invalidate()
		}
		return 0
	case WM_SETCURSOR:
		if l&0xFFFF == 1 { // HTCLIENT
			if clickable(st.hoverID) {
				pSetCursor.Call(cursorHand)
			} else {
				pSetCursor.Call(cursorArrow)
			}
			return 1
		}
	case WM_TIMER:
		switch w {
		case timerAnim:
			if !stepAnim(st) {
				pKillTimer.Call(h, timerAnim)
				animating = false
			}
			invalidate()
		case timerMon:
			if monitorOn.Load() && boostCS2() > 0 {
				logf("cs2.exe colocado em prioridade Alta.")
			}
			applyX3DAffinity()
			moveBackgroundToECores()
			if monitorOn.Load() && st.exSel[extraStandby] && cs2Running() {
				cleanStandbyIfNeeded()
			}
		case timerFast:
			updateVibrance(monitorOn.Load() && st.exSel[extraVibrance])
		}
		return 0
	case wmLog:
		drainLog()
		return 0
	case wmBench:
		drainLog()
		st.benchRunning = false
		st.benchLines = benchSummary()
		kick()
		return 0
	case wmDone:
		drainLog()
		setBusy(false)
		refreshRows()
		refreshNvidia()
		if monitorOn.Load() {
			if boostCS2() > 0 {
				logf("cs2.exe colocado em prioridade Alta.")
			} else {
				logf("Monitor de prioridade ativo: quando o CS2 abrir, ele vai para prioridade Alta.")
			}
		}
		if w == 1 {
			msgBox(hwndMain, "A mudança no HAGS só vale depois de reiniciar o PC.", appTitle, MB_OK|MB_ICONINFORMATION)
		}
		return 0
	case WM_CLOSE:
		if busy.Load() {
			msgBox(hwndMain, "Aguarde terminar de aplicar os ajustes.", appTitle, MB_OK|MB_ICONWARNING)
			return 0
		}
		pDestroyWindow.Call(h)
		return 0
	case WM_DESTROY:
		updateVibrance(false)
		requestTimer(false)
		restoreBackgroundAffinity()
		pPostQuitMessage.Call(0)
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(h, msg, w, l)
	return r
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			msgBox(0, fmt.Sprintf("Erro inesperado: %v", r), appTitle, MB_OK|MB_ICONERROR)
		}
	}()

	hInst, _, _ = pGetModuleHandleW.Call(0)
	if dc, _, _ := pGetDC.Call(0); dc != 0 {
		if v, _, _ := pGetDeviceCaps.Call(dc, 90); v > 0 { // LOGPIXELSY
			dpi = int(v)
		}
		pReleaseDC.Call(0, dc)
	}
	kScale = float64(dpi) / 96

	// se a janela nao couber na tela (escala alta em tela pequena), reduz a escala
	var work rect
	pSystemParamsInfo.Call(0x30, 0, uintptr(unsafe.Pointer(&work)), 0) // SPI_GETWORKAREA
	if avail := float64(work.Bottom-work.Top) - 16*kScale; avail > 0 && (uiH+36)*kScale > avail {
		kScale = avail / (uiH + 36)
		dpi = int(kScale * 96)
	}

	fonts[fTitle] = makeFont(20, 700, "Segoe UI")
	fonts[fSub] = makeFont(9.5, 400, "Segoe UI")
	fonts[fRowTitle] = makeFont(9.5, 600, "Segoe UI")
	fonts[fRowDesc] = makeFont(8, 400, "Segoe UI")
	fonts[fLabel] = makeFont(7.5, 700, "Segoe UI")
	fonts[fBtn] = makeFont(9.5, 600, "Segoe UI")
	fonts[fLog] = makeFont(7.5, 400, "Consolas")
	fonts[fChip] = makeFont(9, 600, "Segoe UI")
	fonts[fSeg] = makeFont(9.5, 600, "Segoe UI")

	cursorArrow, _, _ = pLoadCursorW.Call(0, 32512)
	cursorHand, _, _ = pLoadCursorW.Call(0, 32649)
	icon, _, _ := pLoadIconW.Call(hInst, 1)
	smX, _, _ := pGetSystemMetrics.Call(49)
	smY, _, _ := pGetSystemMetrics.Call(50)
	iconSm, _, _ := pLoadImageW.Call(hInst, 1, 1, smX, smY, 0)

	className := u16("CS2OptimizerGlass")
	wc := wndClassEx{
		WndProc:   syscall.NewCallback(wndProc),
		Instance:  hInst,
		Icon:      icon,
		Cursor:    cursorArrow,
		ClassName: className,
		IconSm:    iconSm,
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	if r, _, _ := pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		msgBox(0, "Não foi possível iniciar a janela.", appTitle, MB_OK|MB_ICONERROR)
		return
	}

	// estado inicial
	settings = loadSettings()
	gameRows := make([]rowInfo, len(gameOptions))
	for i, o := range gameOptions {
		gameRows[i] = rowInfo{o.label, o.desc, o.recommended}
	}
	nvRows := []rowInfo{
		{"Modo de baixa latência: Ativado", "Perfil do driver para o cs2.exe, como no Painel de Controle NVIDIA.", true},
		{"Gerenciamento de energia: Desempenho máximo", "A placa não reduz o clock durante a partida. Gasta um pouco mais de energia.", true},
		{"Cache de shaders: ilimitado", "Configuração global do driver. Evita refazer shaders e engasgos depois de atualizações.", true},
	}
	exRows := make([]rowInfo, len(extraOptions))
	for i, o := range extraOptions {
		exRows[i] = rowInfo{o.label, o.desc, o.recommended}
	}
	st = newStateFull(make([]rowInfo, len(tweaks)), gameRows, nvRows, exRows)
	copy(st.exSel, settings.ExtraOptions)
	st.crosshair = settings.Crosshair
	st.benchLines = benchSummary()
	if !windows11() {
		st.exEnabled[extraTimer] = false
		st.exSel[extraTimer] = false
		st.tab3Rows[len(nvRows)+extraTimer].desc = "Indisponível: no Windows 10 o timer de outro programa não afeta o CS2."
		for i, t := range tweaks {
			if strings.HasPrefix(t.label, "Permitir timer") {
				st.enabled[i] = false
			}
		}
	}
	if !nvidiaAvailable() {
		st.exEnabled[extraVibrance] = false
		st.exSel[extraVibrance] = false
		st.tab3Rows[len(nvRows)+extraVibrance].desc = "Indisponível: precisa de placa NVIDIA."
	}
	if desc, mask, _ := fastCores(); mask == 0 {
		st.exEnabled[extraX3D] = false
		st.exSel[extraX3D] = false
		st.tab3Rows[len(nvRows)+extraX3D].desc = "Indisponível: só para Ryzen X3D com 2 CCDs ou Intel 12ª, 13ª e 14ª geração."
	} else {
		st.tab3Rows[len(nvRows)+extraX3D].desc = desc
	}
	if t := readTopology(); !t.hybrid {
		st.exEnabled[extraECores] = false
		st.exSel[extraECores] = false
		st.tab3Rows[len(nvRows)+extraECores].desc = "Indisponível: só para processadores com núcleos P e E (Intel 12ª, 13ª e 14ª geração)."
	}
	for i, on := range st.exSel {
		if on {
			st.exT[i] = 1
		}
	}
	copy(st.gameSel, settings.GameOptions)
	for i, on := range st.gameSel {
		if on {
			st.gameT[i] = 1
		}
	}
	loadBackup()
	cs2Path = findCS2()
	st.cs2Found = cs2Path != ""
	for i, t := range tweaks {
		st.enabled[i] = !(t.needsCS2 && cs2Path == "")
	}
	refreshRows()
	refreshNvidia()

	setupSurface()

	style := uintptr(WS_POPUP | WS_SYSMENU | WS_MINIMIZEBOX)
	ww, wh := frame.W, frame.H
	x := int(work.Left) + (int(work.Right-work.Left)-ww)/2
	y := int(work.Top) + (int(work.Bottom-work.Top)-wh)/2
	if y < int(work.Top) {
		y = int(work.Top)
	}

	hwndMain, _, _ = pCreateWindowExW.Call(WS_EX_LAYERED|WS_EX_APPWIN, uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(u16(appTitle))), style,
		uintptr(x), uintptr(y), uintptr(ww), uintptr(wh), 0, 0, hInst, 0)
	if hwndMain == 0 {
		msgBox(0, "Não foi possível criar a janela.", appTitle, MB_OK|MB_ICONERROR)
		return
	}

	if cs2Path != "" {
		logf("CS2: %s", cs2Path)
	} else {
		logf("cs2.exe não encontrado na Steam. Os ajustes específicos do jogo ficam desativados.")
	}
	if w := microcodeWarning(); w != "" {
		logf("%s", w)
	}
	if hasBackup() {
		logf("Há alterações já aplicadas por este app. \"Restaurar tudo\" desfaz todas elas.")
	} else {
		logf("Pronto. Escolha os ajustes e clique em \"Aplicar selecionados\".")
	}

	pSetTimer.Call(hwndMain, timerMon, 5000, 0)
	pSetTimer.Call(hwndMain, timerFast, 1000, 0)
	drainLog()
	present()
	pShowWindow.Call(hwndMain, 1)

	var m msgT
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

func refreshStartup() {
	startList = listStartup()
	rows := make([]rowInfo, len(startList))
	on := make([]bool, len(startList))
	for i, e := range startList {
		rows[i] = rowInfo{e.name, startupDesc(e.command), false}
		on[i] = e.enabled
	}
	st.setStartup(rows, on)
}

var (
	pOpenClipboard    = user32.NewProc("OpenClipboard")
	pCloseClipboard   = user32.NewProc("CloseClipboard")
	pGetClipboardData = user32.NewProc("GetClipboardData")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pLstrlenW         = kernel32.NewProc("lstrlenW")
	pRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")
)

func clipboardText() string {
	if r, _, _ := pOpenClipboard.Call(hwndMain); r == 0 {
		return ""
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(13) // CF_UNICODETEXT
	if h == 0 {
		return ""
	}
	p, _, _ := pGlobalLock.Call(h)
	if p == 0 {
		return ""
	}
	defer pGlobalUnlock.Call(h)
	n, _, _ := pLstrlenW.Call(p)
	if n == 0 || n > 8192 {
		return ""
	}
	buf := make([]uint16, n)
	pRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), p, n*2)
	return syscall.UTF16ToString(buf)
}
