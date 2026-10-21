package main

// Perfis de video do CS2 (arquivo cs2_video.txt em Steam\userdata\<conta>\730\local\cfg).
// So mexe em chaves cujo significado e conhecido; todo o resto do arquivo fica intacto.

import (
	"regexp"
	"strings"
)

type videoProfile struct {
	name   string
	values map[string]string
}

var videoProfiles = []videoProfile{
	{}, // 0 = nao alterar
	{"Desempenho", map[string]string{
		"setting.mat_vsync":                 "0", // V-Sync desligado
		"setting.msaa_samples":              "0", // sem MSAA
		"setting.r_csgo_cmaa_enable":        "0", // sem CMAA2
		"setting.videocfg_shadow_quality":   "0", // sombras baixas
		"setting.videocfg_texture_detail":   "0", // texturas baixas
		"setting.r_texturefilteringquality": "0", // bilinear
		"setting.shaderquality":             "0", // shaders baixos
		"setting.videocfg_particle_detail":  "0", // particulas baixas
		"setting.videocfg_ao_detail":        "0", // oclusao de ambiente desligada
		"setting.videocfg_fsr_detail":       "0", // FSR desligado
		"setting.r_low_latency":             "1", // NVIDIA Reflex ligado
	}},
	{"Competitivo", map[string]string{
		"setting.mat_vsync":                 "0",
		"setting.msaa_samples":              "0",
		"setting.r_csgo_cmaa_enable":        "1", // CMAA2
		"setting.videocfg_shadow_quality":   "1", // sombras medias (mostram sombras de inimigos)
		"setting.videocfg_texture_detail":   "0",
		"setting.r_texturefilteringquality": "0",
		"setting.shaderquality":             "0",
		"setting.videocfg_particle_detail":  "0",
		"setting.videocfg_ao_detail":        "0",
		"setting.videocfg_fsr_detail":       "0",
		"setting.r_low_latency":             "2", // NVIDIA Reflex ligado + Boost
	}},
}

var videoDescs = [][]string{
	{"Mantém as suas configurações de vídeo atuais do CS2."},
	{"Máximo de FPS, para PCs mais fracos: tudo no mínimo, sem anti-aliasing,",
		"V-Sync desligado e NVIDIA Reflex ligado.",
		"Sombras baixas podem esconder sombras de inimigos."},
	{"Equilíbrio usado em guias competitivos: sombras médias, CMAA2,",
		"texturas, shaders e partículas no baixo, oclusão desligada,",
		"V-Sync desligado e NVIDIA Reflex + Boost."},
}

func videoKeyRe(key string) *regexp.Regexp {
	return regexp.MustCompile(`(?m)^(\s*"` + regexp.QuoteMeta(key) + `"\s+)"[^"\r\n]*"`)
}

// applyVideoValues troca os valores das chaves; chaves ausentes sao inseridas antes do "}" final.
func applyVideoValues(content string, values map[string]string) (string, bool) {
	end := strings.LastIndex(content, "}")
	if end < 0 || !strings.Contains(content, "{") {
		return content, false
	}
	nl := "\n"
	if strings.Contains(content, "\r\n") {
		nl = "\r\n"
	}
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sortStrings(keys)
	var missing []string
	for _, k := range keys {
		re := videoKeyRe(k)
		if re.MatchString(content) {
			v := values[k]
			content = re.ReplaceAllStringFunc(content, func(m string) string {
				sub := re.FindStringSubmatch(m)
				return sub[1] + `"` + v + `"`
			})
		} else {
			missing = append(missing, "\t\""+k+"\"\t\t\""+values[k]+"\""+nl)
		}
	}
	if len(missing) > 0 {
		end = strings.LastIndex(content, "}")
		content = content[:end] + strings.Join(missing, "") + content[end:]
	}
	return content, true
}

func readVideoValue(content, key string) (string, bool) {
	m := regexp.MustCompile(`(?m)^\s*"` + regexp.QuoteMeta(key) + `"\s+"([^"\r\n]*)"`).FindStringSubmatch(content)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// matchProfile diz qual perfil o arquivo segue (0 = personalizado)
func matchProfile(content string) int {
	for i := 1; i < len(videoProfiles); i++ {
		ok := true
		for k, v := range videoProfiles[i].values {
			if cur, found := readVideoValue(content, k); !found || cur != v {
				ok = false
				break
			}
		}
		if ok {
			return i
		}
	}
	return 0
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// resolucoes 4:3 esticadas (como muitos jogadores profissionais usam)
type resolution struct {
	label string
	w, h  int
}

var resolutions = []resolution{{}, {"1280×960", 1280, 960}, {"1440×1080", 1440, 1080}}

func resolutionValues(r resolution) map[string]string {
	return map[string]string{
		"setting.defaultres":       itoa(r.w),
		"setting.defaultresheight": itoa(r.h),
		"setting.aspectratiomode":  "0", // 4:3
		"setting.fullscreen":       "1", // tela cheia (necessario para esticar)
	}
}

// resolutionIndex diz qual resolucao da lista o arquivo usa (0 = outra)
func resolutionIndex(content string) int {
	w, _ := readVideoValue(content, "setting.defaultres")
	h, _ := readVideoValue(content, "setting.defaultresheight")
	for i := 1; i < len(resolutions); i++ {
		if w == itoa(resolutions[i].w) && h == itoa(resolutions[i].h) {
			return i
		}
	}
	return 0
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
