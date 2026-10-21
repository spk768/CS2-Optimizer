package main

// Layout e desenho da tela (independente de plataforma).
// Coordenadas logicas a 96 DPI; k = escala do monitor.

import (
	"fmt"
	"math"
)

const (
	uiW = 660
	uiH = 650
)

// papeis de fonte (criadas pelo Windows em main_windows.go)
const (
	fTitle = iota
	fSub
	fRowTitle
	fRowDesc
	fLabel
	fBtn
	fLog
	fChip
	fSeg
	fontCount
)

const (
	alLeft   = 0
	alCenter = 1
	alRight  = 2
)

// cores de texto 0xRRGGBB
const (
	cWhite    = 0xFFFFFF
	cDim      = 0xBDBDBD
	cDim2     = 0x8A8A8A
	cDisabled = 0x5A5A5A
	cDark     = 0x111111
)

// ids dos elementos clicaveis
const (
	idRow0     = 1
	idSeg0     = 40
	idBtnRec   = 50
	idBtnClear = 51
	idBtnRest  = 52
	idBtnApply = 53
	idLogArea  = 54
	idTab0     = 60
	idVid0     = 70
	idGame0    = 80
	idPlay     = 90
	idMin      = 91
	idClose    = 92
	idNv0      = 100
	idEx0      = 110
	idRes0     = 130
	idBench    = 140
	idPaste    = 141
	idClearX   = 142
	idStart0   = 200 // ate 299
)

// listas com rolagem
const (
	listWin   = 1
	listEx    = 2
	listStart = 3

	winVisible   = 7
	exVisible    = 3
	startVisible = 4
)

type rowInfo struct {
	title, desc string
	recommended bool
}

type textItem struct {
	s          string
	x, y, w, h float64
	font       int
	color      uint32
	align      int
}

type uiState struct {
	tab, hagsSel, vidSel   int
	tabPos, segPos, vidPos float64
	rows                   []rowInfo
	selected, enabled      []bool
	togT                   []float64
	gameRows               []rowInfo
	gameSel                []bool
	gameT                  []float64
	hov                    map[int]float64
	hoverID, pressed       int
	busy, cs2Found         bool
	hagsState, videoState  string
	videoAccounts          int
	logLines               []string
	logScroll              int

	// aba Extras
	tab3Rows                []rowInfo // NVIDIA primeiro, depois "ao jogar"
	nvSel, exSel, exEnabled []bool
	nvT, exT                []float64
	nvAvailable             bool
	nvState, resState       string
	resSel                  int
	resPos                  float64

	// aba Mais
	startRows    []rowInfo
	startOn      []bool
	startT       []float64
	benchRunning bool
	benchLines   []string
	crosshair    string

	// rolagem
	winScroll, exScroll, startScroll int
}

func newState(rows, gameRows []rowInfo) *uiState {
	return newStateFull(rows, gameRows, make([]rowInfo, 3), make([]rowInfo, 7))
}

func newStateFull(rows, gameRows, nvRows, exRows []rowInfo) *uiState {
	st := &uiState{
		rows: rows, selected: make([]bool, len(rows)), enabled: make([]bool, len(rows)), togT: make([]float64, len(rows)),
		gameRows: gameRows, gameSel: make([]bool, len(gameRows)), gameT: make([]float64, len(gameRows)),
		hov:      map[int]float64{},
		tab3Rows: append(append([]rowInfo{}, nvRows...), exRows...),
		nvSel:    make([]bool, len(nvRows)), nvT: make([]float64, len(nvRows)),
		exSel: make([]bool, len(exRows)), exT: make([]float64, len(exRows)), exEnabled: make([]bool, len(exRows)),
	}
	for i := range st.exEnabled {
		st.exEnabled[i] = true
	}
	for i := range st.enabled {
		st.enabled[i] = true
	}
	return st
}

func (st *uiState) setStartup(rows []rowInfo, on []bool) {
	st.startRows = rows
	st.startOn = on
	st.startT = make([]float64, len(rows))
	for i, v := range on {
		if v {
			st.startT[i] = 1
		}
	}
	if st.startScroll > maxScroll(len(rows), startVisible) {
		st.startScroll = maxScroll(len(rows), startVisible)
	}
}

func maxScroll(n, visible int) int {
	if n <= visible {
		return 0
	}
	return n - visible
}

// ---------- Layout (logico) ----------

type lr struct{ x, y, w, h, r float64 }

func (l lr) dev(k float64) rr { return rr{l.x * k, l.y * k, l.w * k, l.h * k, l.r * k} }
func (l lr) has(px, py float64) bool {
	return px >= l.x && px < l.x+l.w && py >= l.y && py < l.y+l.h
}

var tabNames = []string{"Windows", "Jogo", "Extras", "Mais"}

var (
	lTabs    = lr{240, 28, 316, 36, 18}
	lMin     = lr{572, 31, 30, 30, 15}
	lClose   = lr{610, 31, 30, 30, 15}
	lLog     = lr{20, 574, 620, 54, 16}
	lBtnRec  = lr{20, 520, 156, 40, 20}
	lBtnClr  = lr{184, 520, 118, 40, 20}
	lBtnRest = lr{338, 520, 136, 40, 20}
	lBtnAppl = lr{482, 520, 158, 40, 20}

	// aba Windows
	lCard1 = lr{20, 84, 620, 318, 22}
	lCard2 = lr{20, 414, 620, 92, 22}
	lSeg   = lr{36, 452, 360, 36, 18}

	// aba Jogo
	lCardA  = lr{20, 84, 620, 196, 22}
	lCardB  = lr{20, 292, 620, 110, 22}
	lCardC  = lr{20, 414, 620, 92, 22}
	lVidSeg = lr{36, 120, 420, 36, 18}
	lChip   = lr{468, 120, 156, 36, 18}
	lPlay   = lr{36, 448, 230, 44, 22}

	// aba Extras
	lCardN  = lr{20, 84, 620, 162, 22}
	lCardE  = lr{20, 258, 620, 162, 22}
	lCardR  = lr{20, 432, 620, 74, 22}
	lResSeg = lr{36, 462, 420, 34, 17}

	// aba Mais
	lCardS  = lr{20, 84, 620, 202, 22}
	lCardBm = lr{20, 298, 620, 102, 22}
	lCardX  = lr{20, 412, 620, 94, 22}
	lBench  = lr{36, 334, 200, 42, 21}
	lPaste  = lr{36, 448, 200, 40, 20}
	lClearX = lr{244, 448, 100, 40, 20}
)

// linhas de uma lista: slot = posicao visivel (0..), y0 = topo da primeira linha
func lListRow(y0 float64, slot int) lr { return lr{28, y0 + 40*float64(slot), 604, 38, 12} }
func lListToggle(y0 float64, slot int) lr {
	return lr{574, y0 + 40*float64(slot) + 6, 46, 26, 13}
}

const (
	yWinRows   = 118
	yNvRows    = 118
	yExRows    = 292
	yGameRows  = 322
	yStartRows = 118
)

func segItem(s lr, n, j int) lr {
	w := s.w / float64(n)
	return lr{s.x + w*float64(j), s.y, w, s.h, 0}
}

// ---------- Camadas fixas: fundo + paineis de vidro de cada aba ----------

func buildBases(w, h int, k float64, nRows int) [4]*canvas {
	bg := newCanvas(w, h)
	paintBackground(bg)
	mk := func(cards ...lr) *canvas {
		b := newCanvas(w, h)
		copy(b.pix, bg.pix)
		for _, c := range cards {
			b.glass(c.dev(k), bg, k)
		}
		return b
	}
	sep := func(b *canvas, ys ...float64) {
		for _, y := range ys {
			b.fill(rr{40 * k, (y - 1) * k, 520 * k, math.Max(1, k*0.8), 0}, white, 0.07)
		}
	}
	// contorno brilhante da janela
	edge := rr{0, 0, float64(w), float64(h), 22 * k}
	bg.rim(edge, 1.3*k, 0.38, 0.10)
	win := mk(lCard1, lCard2, lLog)
	for i := 1; i < winVisible; i++ {
		sep(win, yWinRows+40*float64(i))
	}
	game := mk(lCardA, lCardB, lCardC, lLog)
	sep(game, 362)
	extra := mk(lCardN, lCardE, lCardR, lLog)
	sep(extra, 158, 198, 332, 372)
	more := mk(lCardS, lCardBm, lCardX, lLog)
	sep(more, 158, 198, 238)
	return [4]*canvas{win, game, extra, more}
}

// ---------- Desenho completo ----------

func renderScene(c *canvas, bases [4]*canvas, st *uiState, k float64) []textItem {
	copy(c.pix, bases[st.tab].pix)
	var tx []textItem
	add := func(s string, l lr, font int, color uint32, align int) {
		tx = append(tx, textItem{s, l.x * k, l.y * k, l.w * k, l.h * k, font, color, align})
	}
	press := func(id int) float64 {
		if st.pressed == id && st.hoverID == id {
			return 1
		}
		return 0
	}
	segment := func(s lr, labels []string, sel int, pos float64, idBase int) {
		n := len(labels)
		for j := 0; j < n; j++ {
			if j != sel && !st.busy {
				it := segItem(s, n, j)
				c.fill(rr{(it.x + 4) * k, (it.y + 4) * k, (it.w - 8) * k, (it.h - 8) * k, (it.h - 8) / 2 * k}, white, 0.06*st.hov[idBase+j])
			}
		}
		drawSegmented(c, s.dev(k), pos, float64(sel), n, k)
		for j, l := range labels {
			color := uint32(cDim)
			if j == sel {
				color = cWhite
			}
			add(l, segItem(s, n, j), fSeg, color, alCenter)
		}
	}
	toggleRow := func(row rowInfo, rowL, togL lr, id int, t float64, enabled bool) {
		if enabled && !st.busy {
			c.fill(rowL.dev(k), white, 0.07*st.hov[id])
		}
		alpha := 1.0
		if !enabled {
			alpha = 0.35
		} else if st.busy {
			alpha = 0.6
		}
		drawToggle(c, togL.dev(k), t, st.hov[id], alpha, k)
		tc, dc := uint32(cWhite), uint32(cDim)
		if !enabled {
			tc, dc = cDisabled, cDisabled
		}
		add(row.title, lr{40, rowL.y + 2, 520, 19, 0}, fRowTitle, tc, alLeft)
		add(row.desc, lr{40, rowL.y + 20, 520, 16, 0}, fRowDesc, dc, alLeft)
	}
	// barra de rolagem fina na borda direita do cartao
	scrollbar := func(card lr, y0 float64, visible, total, scroll int) {
		if total <= visible {
			return
		}
		trackH := 40*float64(visible) - 8
		thumbH := math.Max(24, trackH*float64(visible)/float64(total))
		pos := float64(scroll) / float64(total-visible)
		x := card.x + card.w - 9
		c.fill(lr{x, y0 + 4, 3, trackH, 1.5}.dev(k), white, 0.06)
		c.fill(lr{x, y0 + 4 + (trackH-thumbH)*pos, 3, thumbH, 1.5}.dev(k), white, 0.35)
	}
	list := func(card lr, y0 float64, rows []rowInfo, ts []float64, enabled func(int) bool, idBase, visible, scroll int) {
		for slot := 0; slot < visible; slot++ {
			i := scroll + slot
			if i >= len(rows) {
				break
			}
			toggleRow(rows[i], lListRow(y0, slot), lListToggle(y0, slot), idBase+i, ts[i], enabled(i))
		}
		scrollbar(card, y0, visible, len(rows), scroll)
	}

	// cabecalho + abas
	add("CS2 Optimizer", lr{28, 16, 210, 36, 0}, fTitle, cWhite, alLeft)
	add("Otimização segura e reversível", lr{28, 52, 210, 20, 0}, fSub, cDim, alLeft)
	segment(lTabs, tabNames, st.tab, st.tabPos, idTab0)
	drawWindowButton(c, lMin.dev(k), false, st.hov[idMin], k)
	drawWindowButton(c, lClose.dev(k), true, st.hov[idClose], k)

	switch st.tab {
	case 0:
		nSel := 0
		for _, s := range st.selected {
			if s {
				nSel++
			}
		}
		add("AJUSTES DO WINDOWS", lr{40, 90, 260, 24, 0}, fLabel, cDim2, alLeft)
		add(fmt.Sprintf("%d de %d selecionados  ·  role para ver mais", nSel, len(st.rows)), lr{280, 90, 340, 24, 0}, fLabel, cDim2, alRight)
		list(lCard1, yWinRows, st.rows, st.togT, func(i int) bool { return st.enabled[i] }, idRow0, winVisible, st.winScroll)
		add("HAGS  ·  AGENDAMENTO DE GPU POR HARDWARE", lr{40, 420, 380, 24, 0}, fLabel, cDim2, alLeft)
		add("Atual: "+st.hagsState, lr{400, 420, 224, 24, 0}, fLabel, cDim, alRight)
		segment(lSeg, []string{"Não alterar", "Ativar", "Desativar"}, st.hagsSel, st.segPos, idSeg0)
		add("Exige reiniciar o PC.", lr{414, 450, 212, 18, 0}, fRowDesc, cDim2, alLeft)
		add("Ideal para GPUs recentes.", lr{414, 469, 212, 18, 0}, fRowDesc, cDim2, alLeft)

	case 1:
		add("PERFIL DE VÍDEO DO CS2", lr{40, 90, 260, 24, 0}, fLabel, cDim2, alLeft)
		add("Atual: "+st.videoState, lr{300, 90, 320, 24, 0}, fLabel, cDim, alRight)
		segment(lVidSeg, []string{"Não alterar", "Desempenho", "Competitivo"}, st.vidSel, st.vidPos, idVid0)
		chip := lChip.dev(k)
		c.fill(chip, white, 0.10)
		c.rim(chip, 1.1*k, 0.45, 0.08)
		dotCol, chipText := hex(0xF2F2F2), "CS2 encontrado"
		if !st.cs2Found {
			dotCol, chipText = hex(0x6E6E6E), "CS2 não encontrado"
		}
		drawDot(c, (lChip.x+18)*k, (lChip.y+lChip.h/2)*k, 4*k, dotCol, k)
		add(chipText, lr{lChip.x + 30, lChip.y, lChip.w - 36, lChip.h, 0}, fChip, cWhite, alLeft)
		for i, line := range videoDescs[st.vidSel] {
			add(line, lr{40, 166 + 19*float64(i), 580, 18, 0}, fRowDesc, cDim, alLeft)
		}
		note := "Feche o CS2 antes de aplicar. O \"Restaurar tudo\" volta o arquivo original."
		if st.videoAccounts > 1 {
			note = fmt.Sprintf("%d contas Steam encontradas. ", st.videoAccounts) + note
		}
		add(note, lr{40, 244, 580, 18, 0}, fRowDesc, cDim2, alLeft)
		add("AO ABRIR O JOGO PELO APP", lr{40, 296, 300, 24, 0}, fLabel, cDim2, alLeft)
		for i, row := range st.gameRows {
			toggleRow(row, lListRow(yGameRows, i), lListToggle(yGameRows, i), idGame0+i, st.gameT[i], true)
		}
		add("JOGAR", lr{40, 420, 300, 24, 0}, fLabel, cDim2, alLeft)
		pa := 1.0
		if st.busy || !st.cs2Found || st.benchRunning {
			pa = 0.5
		}
		drawAccentButton(c, lPlay.dev(k), st.hov[idPlay], press(idPlay), pa, k)
		add("Jogar CS2 otimizado", lPlay, fBtn, cDark, alCenter)
		add("Abre o CS2 pela Steam com as opções acima", lr{284, 449, 340, 20, 0}, fRowDesc, cDim, alLeft)
		add("e as da aba Extras, em prioridade Alta.", lr{284, 469, 340, 20, 0}, fRowDesc, cDim, alLeft)

	case 2:
		add("PLACA DE VÍDEO NVIDIA", lr{40, 90, 260, 24, 0}, fLabel, cDim2, alLeft)
		add(st.nvState, lr{300, 90, 320, 24, 0}, fLabel, cDim, alRight)
		for i := range st.nvSel {
			toggleRow(st.tab3Rows[i], lListRow(yNvRows, i), lListToggle(yNvRows, i), idNv0+i, st.nvT[i], st.nvAvailable)
		}
		add("AO JOGAR PELO APP", lr{40, 264, 300, 24, 0}, fLabel, cDim2, alLeft)
		if len(st.exSel) > exVisible {
			add("role para ver mais", lr{300, 264, 320, 24, 0}, fLabel, cDim2, alRight)
		}
		nv := len(st.nvSel)
		list(lCardE, yExRows, st.tab3Rows[nv:], st.exT, func(i int) bool { return st.exEnabled[i] }, idEx0, exVisible, st.exScroll)
		add("RESOLUÇÃO DO CS2 (4:3 ESTICADA)", lr{40, 436, 300, 24, 0}, fLabel, cDim2, alLeft)
		add("Atual: "+st.resState, lr{300, 436, 320, 24, 0}, fLabel, cDim, alRight)
		segment(lResSeg, []string{"Não alterar", "1280×960", "1440×1080"}, st.resSel, st.resPos, idRes0)
		add("Feche o CS2 antes", lr{470, 462, 160, 17, 0}, fRowDesc, cDim2, alLeft)
		add("de aplicar.", lr{470, 479, 160, 17, 0}, fRowDesc, cDim2, alLeft)

	case 3:
		nOn := 0
		for _, v := range st.startOn {
			if v {
				nOn++
			}
		}
		add("PROGRAMAS NA INICIALIZAÇÃO DO WINDOWS", lr{40, 90, 330, 24, 0}, fLabel, cDim2, alLeft)
		add(fmt.Sprintf("%d de %d ativos  ·  muda na hora", nOn, len(st.startRows)), lr{360, 90, 260, 24, 0}, fLabel, cDim2, alRight)
		if len(st.startRows) == 0 {
			add("Nenhum programa configurado para abrir com o Windows.", lr{40, 130, 580, 20, 0}, fRowDesc, cDim, alLeft)
		}
		list(lCardS, yStartRows, st.startRows, st.startT, func(int) bool { return true }, idStart0, startVisible, st.startScroll)

		add("BENCHMARK DE FPS", lr{40, 304, 300, 24, 0}, fLabel, cDim2, alLeft)
		ba := 1.0
		if st.busy || st.benchRunning || !st.cs2Found {
			ba = 0.5
		}
		drawAccentButton(c, lBench.dev(k), st.hov[idBench], press(idBench), ba, k)
		bt := "Rodar benchmark"
		if st.benchRunning {
			bt = "Medindo..."
		}
		add(bt, lBench, fBtn, cDark, alCenter)
		for i, line := range st.benchLines {
			if i >= 3 {
				break
			}
			color := uint32(cDim)
			if i == 0 {
				color = cWhite
			}
			add(line, lr{252, 330 + 19*float64(i), 372, 18, 0}, fRowDesc, color, alLeft)
		}

		add("MIRA DE PRO (CÓDIGO DE COMPARTILHAMENTO)", lr{40, 418, 360, 24, 0}, fLabel, cDim2, alLeft)
		drawGlassButton(c, lPaste.dev(k), st.hov[idPaste], press(idPaste), 1, k)
		add("Colar código de mira", lPaste, fBtn, cWhite, alCenter)
		ca := 1.0
		if st.crosshair == "" {
			ca = 0.4
		}
		drawGlassButton(c, lClearX.dev(k), st.hov[idClearX], press(idClearX), ca, k)
		add("Remover", lClearX, fBtn, cWhite, alCenter)
		if st.crosshair == "" {
			add("Copie o código da mira de um pro (ex.: no site da", lr{356, 448, 270, 19, 0}, fRowDesc, cDim, alLeft)
			add("HLTV ou ProSettings) e clique em \"Colar\".", lr{356, 467, 270, 19, 0}, fRowDesc, cDim, alLeft)
		} else {
			add("Mira salva: "+st.crosshair, lr{356, 448, 270, 19, 0}, fRowDesc, cWhite, alLeft)
			add("Aplicada ao abrir o CS2 pelo app.", lr{356, 467, 270, 19, 0}, fRowDesc, cDim, alLeft)
		}
	}

	// botoes
	ba := 1.0
	if st.busy {
		ba = 0.5
	}
	for _, b := range []struct {
		id    int
		l     lr
		label string
	}{{idBtnRec, lBtnRec, "Recomendados"}, {idBtnClear, lBtnClr, "Limpar"}, {idBtnRest, lBtnRest, "Restaurar tudo"}} {
		drawGlassButton(c, b.l.dev(k), st.hov[b.id], press(b.id), ba, k)
		add(b.label, b.l, fBtn, cWhite, alCenter)
	}
	drawAccentButton(c, lBtnAppl.dev(k), st.hov[idBtnApply], press(idBtnApply), ba, k)
	applyText := "Aplicar selecionados"
	if st.busy {
		applyText = "Aplicando..."
	}
	add(applyText, lBtnAppl, fBtn, cDark, alCenter)

	// log (ultimas 3 linhas; role com a roda do mouse)
	end := len(st.logLines) - st.logScroll
	start := end - 3
	if start < 0 {
		start = 0
	}
	for i, line := range st.logLines[start:end] {
		add(line, lr{34, lLog.y + 6 + 14*float64(i), 592, 14, 0}, fLog, cDim, alLeft)
	}
	return tx
}

// ---------- Interacao ----------

type hitBox struct {
	id int
	l  lr
}

func hitList(px, py, y0 float64, n, visible, scroll, idBase int, enabled func(int) bool) (int, bool) {
	for slot := 0; slot < visible; slot++ {
		i := scroll + slot
		if i >= n {
			break
		}
		if lListRow(y0, slot).has(px, py) {
			if enabled(i) {
				return idBase + i, true
			}
			return 0, true
		}
	}
	return 0, false
}

func hitTest(px, py float64, st *uiState) int {
	boxes := []hitBox{{idMin, lMin}, {idClose, lClose}}
	for j := range tabNames {
		boxes = append(boxes, hitBox{idTab0 + j, segItem(lTabs, len(tabNames), j)})
	}
	switch st.tab {
	case 0:
		if id, ok := hitList(px, py, yWinRows, len(st.rows), winVisible, st.winScroll, idRow0, func(i int) bool { return st.enabled[i] }); ok {
			return id
		}
		for j := 0; j < 3; j++ {
			boxes = append(boxes, hitBox{idSeg0 + j, segItem(lSeg, 3, j)})
		}
	case 1:
		for i := range st.gameRows {
			boxes = append(boxes, hitBox{idGame0 + i, lListRow(yGameRows, i)})
		}
		for j := 0; j < 3; j++ {
			boxes = append(boxes, hitBox{idVid0 + j, segItem(lVidSeg, 3, j)})
		}
		boxes = append(boxes, hitBox{idPlay, lPlay})
	case 2:
		if id, ok := hitList(px, py, yNvRows, len(st.nvSel), len(st.nvSel), 0, idNv0, func(int) bool { return st.nvAvailable }); ok {
			return id
		}
		if id, ok := hitList(px, py, yExRows, len(st.exSel), exVisible, st.exScroll, idEx0, func(i int) bool { return st.exEnabled[i] }); ok {
			return id
		}
		for j := 0; j < 3; j++ {
			boxes = append(boxes, hitBox{idRes0 + j, segItem(lResSeg, 3, j)})
		}
	case 3:
		if id, ok := hitList(px, py, yStartRows, len(st.startRows), startVisible, st.startScroll, idStart0, func(int) bool { return true }); ok {
			return id
		}
		boxes = append(boxes, hitBox{idBench, lBench}, hitBox{idPaste, lPaste})
		if st.crosshair != "" {
			boxes = append(boxes, hitBox{idClearX, lClearX})
		}
	}
	boxes = append(boxes, hitBox{idBtnRec, lBtnRec}, hitBox{idBtnClear, lBtnClr}, hitBox{idBtnRest, lBtnRest}, hitBox{idBtnApply, lBtnAppl}, hitBox{idLogArea, lLog})
	for _, b := range boxes {
		if b.l.has(px, py) {
			return b.id
		}
	}
	return 0
}

func clickable(id int) bool { return id != 0 && id != idLogArea }

// scrollList rola a lista sob o mouse; devolve true se algo mudou
func scrollList(px, py float64, delta int, st *uiState) bool {
	type target struct {
		card       lr
		ptr        *int
		n, visible int
	}
	var t *target
	switch {
	case st.tab == 0 && lCard1.has(px, py):
		t = &target{lCard1, &st.winScroll, len(st.rows), winVisible}
	case st.tab == 2 && lCardE.has(px, py):
		t = &target{lCardE, &st.exScroll, len(st.exSel), exVisible}
	case st.tab == 3 && lCardS.has(px, py):
		t = &target{lCardS, &st.startScroll, len(st.startRows), startVisible}
	}
	if t == nil {
		return false
	}
	v := *t.ptr - delta
	if v < 0 {
		v = 0
	}
	if m := maxScroll(t.n, t.visible); v > m {
		v = m
	}
	if v == *t.ptr {
		return false
	}
	*t.ptr = v
	return true
}

// avanca as animacoes; retorna true enquanto algo ainda se move
func stepAnim(st *uiState) bool {
	moving := false
	step := func(v, target, speed float64) float64 {
		if math.Abs(target-v) < 0.004 {
			return target
		}
		moving = true
		return v + (target-v)*speed
	}
	b2f := func(b bool) float64 {
		if b {
			return 1
		}
		return 0
	}
	for i := range st.togT {
		st.togT[i] = step(st.togT[i], b2f(st.selected[i]), 0.22)
	}
	for i := range st.gameT {
		st.gameT[i] = step(st.gameT[i], b2f(st.gameSel[i]), 0.22)
	}
	for i := range st.nvT {
		st.nvT[i] = step(st.nvT[i], b2f(st.nvSel[i]), 0.22)
	}
	for i := range st.exT {
		st.exT[i] = step(st.exT[i], b2f(st.exSel[i]), 0.22)
	}
	for i := range st.startT {
		st.startT[i] = step(st.startT[i], b2f(st.startOn[i]), 0.22)
	}
	ids := []int{idBtnRec, idBtnClear, idBtnRest, idBtnApply, idPlay, idMin, idClose, idBench, idPaste, idClearX}
	for j := range tabNames {
		ids = append(ids, idTab0+j)
	}
	for i := range st.rows {
		ids = append(ids, idRow0+i)
	}
	for i := range st.gameRows {
		ids = append(ids, idGame0+i)
	}
	for j := 0; j < 3; j++ {
		ids = append(ids, idSeg0+j, idVid0+j, idRes0+j)
	}
	for i := range st.nvSel {
		ids = append(ids, idNv0+i)
	}
	for i := range st.exSel {
		ids = append(ids, idEx0+i)
	}
	for i := range st.startRows {
		ids = append(ids, idStart0+i)
	}
	for _, id := range ids {
		st.hov[id] = step(st.hov[id], b2f(st.hoverID == id), 0.25)
	}
	st.segPos = step(st.segPos, float64(st.hagsSel), 0.2)
	st.vidPos = step(st.vidPos, float64(st.vidSel), 0.2)
	st.tabPos = step(st.tabPos, float64(st.tab), 0.2)
	st.resPos = step(st.resPos, float64(st.resSel), 0.2)
	return moving
}

// area do cabecalho que arrasta a janela (fora dos controles)
func inDragArea(px, py float64) bool {
	if py < 0 || py >= 80 || px < 0 || px >= uiW {
		return false
	}
	for _, l := range []lr{lTabs, lMin, lClose} {
		if l.has(px, py) {
			return false
		}
	}
	return true
}

// botoes redondos de vidro da barra de titulo
func drawWindowButton(c *canvas, s rr, isClose bool, hov, k float64) {
	if isClose && hov > 0 {
		c.glow(s, white, 0.12*hov, 6*k)
	}
	c.fillV(s, white, white, 0.16+0.08*hov, 0.08+0.06*hov)
	if isClose {
		c.fillV(s, hex(0x5A5A5A), hex(0x3A3A3A), hov, hov)
	}
	c.rim(s, 1.1*k, 0.55+0.2*hov, 0.10)
	cx, cy := s.x+s.w/2, s.y+s.h/2
	g := 4.5 * k
	if isClose {
		c.line(cx-g, cy-g, cx+g, cy+g, 1.6*k, white, 0.95)
		c.line(cx-g, cy+g, cx+g, cy-g, 1.6*k, white, 0.95)
	} else {
		c.line(cx-g, cy, cx+g, cy, 1.6*k, white, 0.95)
	}
}
