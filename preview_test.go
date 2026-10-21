package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"
)

// Gera preview.png (sem texto) + preview.json (textos) para conferir o visual fora do Windows.
func TestPreview(t *testing.T) {
	k := 1.5
	w, h := int(uiW*k), int(uiH*k)
	rows := []rowInfo{
		{"Plano de energia: Alto desempenho", "Evita que a CPU reduza o clock durante o jogo. Em notebook, gasta mais bateria.", true},
		{"Ativar o Modo de Jogo do Windows", "Prioriza o jogo e evita atualizações e reinícios do Windows durante a partida.", true},
		{"Desativar a gravação em segundo plano (Game DVR)", "Remove a captura automática, que custa FPS. A Game Bar continua funcionando.", true},
		{"Usar a placa de vídeo dedicada no CS2", "Para PCs com vídeo integrado + placa dedicada (comum em notebooks).", true},
		{"Desativar as otimizações de tela cheia no cs2.exe", "Pode reduzir o input lag em alguns PCs.  ·  Atual: ativadas", false},
		{"Colocar o CS2 em prioridade Alta", "Vale enquanto este app estiver aberto (pode minimizar). Nunca usa Tempo real.", true},
		{"Limpar arquivos temporários antigos", "Apaga só temporários com mais de 2 dias. Isso não é desfeito pelo Restaurar.", false},
		{"Desligar a suspensão seletiva de USB", "Evita que o Windows desligue portas USB para economizar energia (microtravadas no mouse).", true},
		{"Desligar a economia de energia do PCIe", "A placa de vídeo e o SSD não entram em modo econômico durante o jogo.", false},
	}
	gameRows := []rowInfo{
		{"Remover o limite de FPS", "+fps_max 0  ·  recomendado com o NVIDIA Reflex ligado.", true},
		{"Reduzir input lag", "+engine_low_latency_sleep_after_client_tick true  ·  o ganho varia de PC para PC.", false},
	}
	nvRows := []rowInfo{
		{"Modo de baixa latência: Ativado", "Perfil do driver para o cs2.exe, como no Painel de Controle NVIDIA.", true},
		{"Gerenciamento de energia: Desempenho máximo", "A placa não reduz o clock durante a partida. Gasta um pouco mais de energia.", true},
		{"Cache de shaders: ilimitado", "Configuração global do driver. Evita refazer shaders e engasgos depois de atualizações.", true},
	}
	exRows := []rowInfo{
		{"Fechar outros launchers de jogos", "Epic, Battle.net, EA app, Ubisoft Connect e GOG Galaxy, ao clicar em \"Jogar\".", true},
		{"Fechar navegadores", "Chrome, Edge, Firefox, Opera e Brave. As abas voltam quando você reabrir.", false},
		{"Prender o CS2 nos núcleos mais rápidos", "Intel híbrido detectado (8 núcleos P + 16 E): o CS2 roda só nos núcleos P.", true},
		{"Mover apps de fundo para os núcleos E", "Discord, Spotify, Steam, navegadores, OneDrive e Teams. Voltam ao normal ao fechar o app.", false},
		{"Vibrância digital 80% durante o jogo (NVIDIA)", "Cores mais vivas só com o CS2 em primeiro plano. Ao sair do jogo, volta ao normal.", false},
		{"Limpar a memória em espera durante o jogo", "Como o ISLC: libera a RAM em cache quando a livre cai abaixo de 1 GB.", false},
		{"Timer de 0,5 ms durante o jogo", "Pode deixar os frames mais regulares.", false},
	}
	st := newStateFull(rows, gameRows, nvRows, exRows)
	st.nvAvailable, st.nvState = true, "NVIDIA detectada"
	st.nvSel[0], st.nvT[0], st.nvSel[1], st.nvT[1], st.nvSel[2], st.nvT[2] = true, 1, true, 1, true, 1
	st.exSel[0], st.exT[0] = true, 1
	st.exSel[2], st.exT[2] = true, 1
	st.resSel, st.resPos, st.resState = 1, 1, "1920×1080"
	st.gameSel[0], st.gameT[0] = true, 1
	st.vidSel, st.vidPos = 2, 2
	st.videoState, st.videoAccounts = "personalizado", 1
	st.cs2Found = true
	st.hagsState = "ativado"
	for i, on := range []bool{true, true, true, false, false, true, false, true, false} {
		st.selected[i] = on
		if on {
			st.togT[i] = 1
		}
	}
	st.togT[3] = 0.5 // meio da animacao, para ver o efeito "liquido"
	st.selected[3] = true
	st.hov[idRow0+3] = 1
	st.hoverID = idBtnApply
	st.hov[idBtnApply] = 1
	st.hagsSel, st.segPos = 1, 1
	st.logLines = []string{
		"[13:10:02] CS2: C:\\Program Files (x86)\\Steam\\steamapps\\common\\Counter-Strike Global Offensive\\game\\bin\\win64\\cs2.exe",
		"[13:10:02] Perfil de vídeo e opções de inicialização ficam na aba Jogo.",
		"[13:10:02] Pronto. Escolha os ajustes e clique em \"Aplicar selecionados\".",
	}

	if os.Getenv("TAB") == "1" {
		st.tab, st.tabPos = 1, 1
		st.hoverID = idPlay
		st.hov[idPlay] = 1
	}
	if os.Getenv("TAB") == "3" {
		st.tab, st.tabPos = 3, 3
		st.setStartup([]rowInfo{
			{"Discord", "C:\\Users\\voce\\AppData\\Local\\Discord\\Update.exe", false},
			{"Microsoft OneDrive", "C:\\Program Files\\Microsoft OneDrive\\OneDrive.exe", false},
			{"Steam", "C:\\Program Files (x86)\\Steam\\steam.exe", false},
			{"Spotify", "C:\\Users\\voce\\AppData\\Roaming\\Spotify\\Spotify.exe", false},
			{"NVIDIA Broadcast", "C:\\Program Files\\NVIDIA Corporation\\NVIDIA Broadcast\\NVIDIA Broadcast UI.exe", false},
		}, []bool{true, false, true, false, true})
		st.benchLines = []string{"Último: 342 FPS médio  ·  1% low 211  (21/10 14:20)", "Anterior: 318 FPS  ·  1% low 187  (21/10 14:05)", "Diferença: +8% no médio  ·  +13% no 1% low"}
		st.crosshair = "CSGO-3fN3P-tGJt5-DOMuB-Epnae-SXmkG"
		st.hoverID = idBench
		st.hov[idBench] = 1
	}
	if os.Getenv("TAB") == "2" {
		st.tab, st.tabPos = 2, 2
		st.hoverID = idEx0 + 1
		st.hov[idEx0+1] = 1
	}
	bases := buildBases(w, h, k, len(rows))
	c := newCanvas(w, h)
	items := renderScene(c, bases, st, k)

	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = c.pix[i*4+2], c.pix[i*4+1], c.pix[i*4], 255
	}
	f, _ := os.Create("preview.png")
	png.Encode(f, img)
	f.Close()

	type jt struct {
		S          string
		X, Y, W, H float64
		Font       int
		Color      uint32
		Align      int
	}
	var out []jt
	for _, it := range items {
		out = append(out, jt{it.s, it.x, it.y, it.w, it.h, it.font, it.color, it.align})
	}
	b, _ := json.Marshal(out)
	os.WriteFile("preview.json", b, 0o644)

	// mascara dos cantos e sombra (mesmos dados usados pela janela real)
	fr := newWindowFrame(w, h, k)
	mask := image.NewGray(image.Rect(0, 0, w, h))
	copy(mask.Pix, fr.mask)
	mf, _ := os.Create("preview-mask.png")
	png.Encode(mf, mask)
	mf.Close()
	sh := image.NewGray(image.Rect(0, 0, fr.W, fr.H))
	for i := range sh.Pix {
		sh.Pix[i] = fr.shadow[i*4+3]
	}
	sf, _ := os.Create("preview-shadow.png")
	png.Encode(sf, sh)
	sf.Close()
	os.WriteFile("preview-margin.txt", []byte(fmt.Sprint(fr.M)), 0o644)
}

func BenchmarkFrame(b *testing.B) {
	k := 1.5
	w, h := int(uiW*k), int(uiH*k)
	rows := make([]rowInfo, 7)
	st := newState(rows, make([]rowInfo, 2))
	bases := buildBases(w, h, k, 7)
	c := newCanvas(w, h)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		renderScene(c, bases, st, k)
	}
}
