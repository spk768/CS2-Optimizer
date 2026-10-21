package main

// Aba "Jogo": perfil de video do CS2 e abertura do jogo com opcoes de inicializacao.

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// opcoes passadas ao CS2 quando aberto pelo app (steam -applaunch 730 ...)
var gameOptions = []struct {
	label, desc string
	args        []string
	recommended bool
}{
	{"Remover o limite de FPS", "+fps_max 0  ·  recomendado com o NVIDIA Reflex ligado.", []string{"+fps_max", "0"}, true},
	{"Reduzir input lag", "+engine_low_latency_sleep_after_client_tick true  ·  o ganho varia de PC para PC.", []string{"+engine_low_latency_sleep_after_client_tick", "true"}, false},
}

func findVideoFiles() []string {
	var out []string
	seen := map[string]bool{}
	for _, s := range findSteamDirs() {
		matches, _ := filepath.Glob(filepath.Join(s, "userdata", "*", "730", "local", "cfg", "cs2_video.txt"))
		for _, m := range matches {
			key := strings.ToLower(m)
			if !seen[key] {
				seen[key] = true
				out = append(out, m)
			}
		}
	}
	return out
}

func cs2Running() bool { return len(findProcesses("cs2.exe")) > 0 }

func applyVideoProfile(idx int) error {
	if idx <= 0 || idx >= len(videoProfiles) {
		return nil
	}
	return applyVideoChanges(videoProfiles[idx].values, "Perfil de vídeo \""+videoProfiles[idx].name+"\"")
}

func applyResolution(idx int) error {
	if idx <= 0 || idx >= len(resolutions) {
		return nil
	}
	r := resolutions[idx]
	if err := applyVideoChanges(resolutionValues(r), "Resolução "+r.label); err != nil {
		return err
	}
	logf("Dica: se aparecerem barras pretas, ative o dimensionamento \"Tela cheia\" no painel da placa de vídeo.")
	return nil
}

// applyVideoChanges grava valores no cs2_video.txt de todas as contas, com copia de seguranca
func applyVideoChanges(values map[string]string, label string) error {
	if cs2Running() {
		return errors.New("feche o CS2 antes (ele regrava o arquivo de vídeo ao sair)")
	}
	files := findVideoFiles()
	if len(files) == 0 {
		return errors.New("cs2_video.txt não encontrado: abra o CS2 uma vez e feche, depois tente de novo")
	}
	if backup.Video == nil {
		backup.Video = map[string]string{}
	}
	dir := filepath.Join(appDir(), "video-backup")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	changed := 0
	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			logf("Não foi possível ler %s: %v", f, err)
			continue
		}
		if _, ok := backup.Video[f]; !ok {
			cp := filepath.Join(dir, fmt.Sprintf("cs2_video_%d.txt", len(backup.Video)+1))
			if err := os.WriteFile(cp, b, 0o644); err != nil {
				return fmt.Errorf("não foi possível salvar a cópia de segurança: %v", err)
			}
			backup.Video[f] = cp
			if err := saveBackup(); err != nil {
				return err
			}
		}
		out, ok := applyVideoValues(string(b), values)
		if !ok {
			logf("Formato inesperado, arquivo ignorado: %s", f)
			continue
		}
		if err := os.WriteFile(f, []byte(out), 0o644); err != nil {
			logf("Não foi possível gravar %s: %v", f, err)
			continue
		}
		changed++
	}
	if changed == 0 {
		return errors.New("nenhum arquivo de vídeo foi alterado")
	}
	logf("%s aplicado em %d conta(s) Steam.", label, changed)
	return nil
}

// videoStatus: texto do perfil atual e quantidade de contas encontradas
func videoStatus() (string, int) {
	files := findVideoFiles()
	if len(files) == 0 {
		return "arquivo de vídeo não encontrado", 0
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		return "não foi possível ler", len(files)
	}
	if p := matchProfile(string(b)); p > 0 {
		return videoProfiles[p].name, len(files)
	}
	return "personalizado", len(files)
}

func launchCS2(opts []bool, extra []string) error {
	var steamExe string
	for _, d := range findSteamDirs() {
		if p := filepath.Join(d, "steam.exe"); fileExists(p) {
			steamExe = p
			break
		}
	}
	if steamExe == "" {
		return errors.New("steam.exe não encontrado")
	}
	args := []string{"-applaunch", "730"}
	for i, on := range opts {
		if on && i < len(gameOptions) {
			args = append(args, gameOptions[i].args...)
		}
	}
	if settings.Crosshair != "" {
		args = append(args, "+apply_crosshair_code", settings.Crosshair)
	}
	args = append(args, extra...)
	cmd := exec.Command(steamExe, args...)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}

// ---------- preferencias da aba Jogo ----------

type appSettings struct {
	GameOptions  []bool
	ExtraOptions []bool
	Crosshair    string
}

func settingsPath() string { return filepath.Join(appDir(), "settings.json") }

func loadSettings() appSettings {
	var s appSettings
	if b, err := os.ReadFile(settingsPath()); err == nil {
		json.Unmarshal(b, &s)
	}
	if len(s.GameOptions) != len(gameOptions) {
		s.GameOptions = make([]bool, len(gameOptions))
		for i, o := range gameOptions {
			s.GameOptions[i] = o.recommended
		}
	}
	if len(s.ExtraOptions) != len(extraOptions) {
		s.ExtraOptions = make([]bool, len(extraOptions))
		for i, o := range extraOptions {
			s.ExtraOptions[i] = o.recommended
		}
	}
	return s
}

func saveSettings(s appSettings) {
	os.MkdirAll(appDir(), 0o755)
	if b, err := json.MarshalIndent(s, "", "  "); err == nil {
		os.WriteFile(settingsPath(), b, 0o644)
	}
}

func resolutionStatus() string {
	files := findVideoFiles()
	if len(files) == 0 {
		return "arquivo de vídeo não encontrado"
	}
	b, err := os.ReadFile(files[0])
	if err != nil {
		return "não foi possível ler"
	}
	w, ok1 := readVideoValue(string(b), "setting.defaultres")
	h, ok2 := readVideoValue(string(b), "setting.defaultresheight")
	if !ok1 || !ok2 {
		return "desconhecida"
	}
	return w + "×" + h
}
