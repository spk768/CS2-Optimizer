package main

import (
	"strings"
	"testing"
)

const sampleVideo = "\"video.cfg\"\r\n{\r\n\t\"setting.defaultres\"\t\t\"1280\"\r\n\t\"setting.mat_vsync\"\t\t\"1\"\r\n\t\"Version\"\t\t\"16\"\r\n\t\"setting.videocfg_shadow_quality\"\t\t\"2\"\r\n\t\"setting.videocfg_texture_detail\"\t\t\"0\"\r\n\t\"setting.videocfg_ao_detail\"\t\t\"2\"\r\n\t\"setting.videocfg_hdr_detail\"\t\t\"-1\"\r\n\t\"setting.r_texturefilteringquality\"\t\t\"5\"\r\n\t\"setting.r_low_latency\"\t\t\"1\"\r\n\t\"setting.r_csgo_cmaa_enable\"\t\t\"0\"\r\n\t\"setting.shaderquality\"\t\t\"0\"\r\n\t\"setting.msaa_samples\"\t\t\"0\"\r\n\t\"setting.videocfg_dynamic_shadows\"\t\t\"1\"\r\n}\r\n"

func TestApplyVideo(t *testing.T) {
	if matchProfile(sampleVideo) != 0 {
		t.Fatal("amostra nao deveria bater com perfil")
	}
	out, ok := applyVideoValues(sampleVideo, videoProfiles[2].values)
	if !ok {
		t.Fatal("falhou")
	}
	if matchProfile(out) != 2 {
		t.Fatalf("esperava Competitivo:\n%s", out)
	}
	// chaves nao conhecidas ficam intactas
	for _, keep := range []string{`"setting.videocfg_hdr_detail"		"-1"`, `"setting.videocfg_dynamic_shadows"		"1"`, `"setting.defaultres"		"1280"`, `"Version"		"16"`} {
		if !strings.Contains(out, keep) {
			t.Fatalf("perdeu %s", keep)
		}
	}
	if strings.Count(out, "\"setting.mat_vsync\"") != 1 {
		t.Fatal("duplicou chave")
	}
	if !strings.HasSuffix(out, "}\r\n") || strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Fatal("quebrou finais de linha")
	}
	// aplicar de novo nao muda nada
	out2, _ := applyVideoValues(out, videoProfiles[2].values)
	if out2 != out {
		t.Fatal("nao idempotente")
	}
	out3, _ := applyVideoValues(out, videoProfiles[1].values)
	if matchProfile(out3) != 1 {
		t.Fatal("esperava Desempenho")
	}
	t.Log("\n" + out)
}

func TestResolution(t *testing.T) {
	out, _ := applyVideoValues(sampleVideo, resolutionValues(resolutions[2]))
	if resolutionIndex(out) != 2 {
		t.Fatal("esperava 1440x1080")
	}
	if v, _ := readVideoValue(out, "setting.aspectratiomode"); v != "0" {
		t.Fatal("aspecto")
	}
	if matchProfile(out) != 0 || !strings.Contains(out, `"setting.mat_vsync"		"1"`) {
		t.Fatal("mexeu em outras chaves")
	}
}
