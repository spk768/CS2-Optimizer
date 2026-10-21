package main

// Benchmark de FPS: abre o CS2 no mapa de benchmark da Oficina ("CS2 FPS Benchmark", ID 3240880604),
// com -condebug, e le o resultado que o proprio jogo escreve no console.log:
//   [VProf] FPS: Avg=312.4, P1=198.7

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"
)

const benchWorkshopID = "3240880604"

var vprofRe = regexp.MustCompile(`\[VProf\] FPS: Avg=([0-9.]+), P1=([0-9.]+)`)

type benchResult struct {
	Time time.Time
	Avg  float64
	P1   float64
}

func benchHistoryPath() string { return filepath.Join(appDir(), "benchmarks.json") }

func loadBenchHistory() []benchResult {
	var h []benchResult
	if b, err := os.ReadFile(benchHistoryPath()); err == nil {
		json.Unmarshal(b, &h)
	}
	return h
}

func saveBenchResult(r benchResult) {
	h := append(loadBenchHistory(), r)
	if len(h) > 20 {
		h = h[len(h)-20:]
	}
	os.MkdirAll(appDir(), 0o755)
	if b, err := json.MarshalIndent(h, "", "  "); err == nil {
		os.WriteFile(benchHistoryPath(), b, 0o644)
	}
}

// benchSummary monta as linhas da interface: ultimo resultado, comparacao e instrucao
func benchSummary() []string {
	h := loadBenchHistory()
	if len(h) == 0 {
		return []string{"Mede FPS médio e 1% low num mapa de teste (~2 min).",
			"Rode uma vez antes e outra depois dos ajustes.",
			"Precisa do mapa \"CS2 FPS Benchmark\" da Oficina."}
	}
	last := h[len(h)-1]
	lines := []string{fmt.Sprintf("Último: %.0f FPS médio  ·  1%% low %.0f  (%s)", last.Avg, last.P1, last.Time.Format("02/01 15:04"))}
	if len(h) >= 2 {
		prev := h[len(h)-2]
		lines = append(lines, fmt.Sprintf("Anterior: %.0f FPS  ·  1%% low %.0f  (%s)", prev.Avg, prev.P1, prev.Time.Format("02/01 15:04")))
		if prev.Avg > 0 && prev.P1 > 0 {
			lines = append(lines, fmt.Sprintf("Diferença: %+.0f%% no médio  ·  %+.0f%% no 1%% low", (last.Avg/prev.Avg-1)*100, (last.P1/prev.P1-1)*100))
		}
	} else {
		lines = append(lines, "Rode de novo depois dos ajustes para comparar.")
	}
	return lines
}

func benchMapInstalled() bool {
	for _, d := range findSteamDirs() {
		if st, err := os.Stat(filepath.Join(d, "steamapps", "workshop", "content", "730", benchWorkshopID)); err == nil && st.IsDir() {
			return true
		}
	}
	return false
}

func openBenchMapPage() {
	runHidden(filepath.Join(os.Getenv("SystemRoot"), "explorer.exe"), "steam://url/CommunityFilePage/"+benchWorkshopID)
}

func consoleLogPath() string {
	if cs2Path == "" {
		return ""
	}
	// ...\game\bin\win64\cs2.exe -> ...\game\csgo\console.log
	game := filepath.Dir(filepath.Dir(filepath.Dir(cs2Path)))
	return filepath.Join(game, "csgo", "console.log")
}

// prepareBenchmark deixa o console.log limpo e devolve os argumentos extras do jogo
func prepareBenchmark() ([]string, error) {
	logPath := consoleLogPath()
	if logPath == "" {
		return nil, errors.New("CS2 não encontrado")
	}
	if _, err := os.Stat(logPath); err == nil {
		os.Remove(logPath + ".old")
		if err := os.Rename(logPath, logPath+".old"); err != nil {
			return nil, fmt.Errorf("não foi possível preparar o console.log: %v", err)
		}
	}
	return []string{"-condebug", "+map_workshop", benchWorkshopID, "de_dust2"}, nil
}

// waitBenchmark acompanha o console.log ate o resultado aparecer (roda em goroutine)
func waitBenchmark(done func(benchResult, error)) {
	logPath := consoleLogPath()
	deadline := time.Now().Add(9 * time.Minute)
	sawGame := false
	for time.Now().Before(deadline) {
		time.Sleep(2 * time.Second)
		running := cs2Running()
		if running {
			sawGame = true
		}
		if b, err := os.ReadFile(logPath); err == nil {
			if m := vprofRe.FindAllSubmatch(b, -1); len(m) > 0 {
				last := m[len(m)-1]
				avg, _ := strconv.ParseFloat(string(last[1]), 64)
				p1, _ := strconv.ParseFloat(string(last[2]), 64)
				r := benchResult{time.Now(), avg, p1}
				saveBenchResult(r)
				time.Sleep(2 * time.Second)
				runHidden(filepath.Join(system32(), "taskkill.exe"), "/IM", "cs2.exe", "/F")
				done(r, nil)
				return
			}
		}
		if sawGame && !running {
			done(benchResult{}, errors.New("o CS2 foi fechado antes de terminar a medição"))
			return
		}
	}
	done(benchResult{}, errors.New("tempo esgotado esperando o resultado"))
}
