package main

import "testing"

func TestCrosshairCode(t *testing.T) {
	cases := map[string]string{
		"CSGO-3fN3P-tGJt5-DOMuB-Epnae-SXmkG":                          "CSGO-3fN3P-tGJt5-DOMuB-Epnae-SXmkG",
		"mira do s1mple: CSGO-3fN3P-tGJt5-DOMuB-Epnae-SXmkG obrigado": "CSGO-3fN3P-tGJt5-DOMuB-Epnae-SXmkG",
		"CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe":              "CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe",
		"CSGO-1234-abcde": "",
		"texto qualquer":  "",
	}
	for in, want := range cases {
		if got := extractCrosshairCode(in); got != want {
			t.Errorf("%q: got %q want %q", in, got, want)
		}
	}
}
