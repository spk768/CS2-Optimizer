package main

import (
	"regexp"
	"strings"
)

// Codigos de mira do CS2: formato antigo "CSGO-xxxxx-xxxxx-xxxxx-xxxxx-xxxxx"
// e o formato novo "CS" + 44 caracteres (update de 30/09/2026).
// O proprio jogo decodifica o codigo com o comando apply_crosshair_code.
const shareDict = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

var (
	legacyCrosshairRe = regexp.MustCompile(`^CSGO(-[` + shareDict + `]{5}){5}$`)
	cs2CrosshairRe    = regexp.MustCompile(`^CS[` + shareDict + `]{44}$`)
	findCrosshairRe   = regexp.MustCompile(`CSGO(-[` + shareDict + `]{5}){5}|CS[` + shareDict + `]{44}`)
)

// extractCrosshairCode acha um codigo de mira valido num texto colado (ex.: link ou frase)
func extractCrosshairCode(text string) string {
	for _, m := range findCrosshairRe.FindAllString(strings.TrimSpace(text), -1) {
		if legacyCrosshairRe.MatchString(m) || cs2CrosshairRe.MatchString(m) {
			return m
		}
	}
	return ""
}
