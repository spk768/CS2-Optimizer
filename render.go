package main

// Renderizador por software: formas com anti-aliasing (SDF), gradientes
// e o efeito "liquid glass" (refracao na borda, brilho de contorno, sombra).
// Sem dependencias de Windows, para poder gerar previas fora dele.

import (
	"math"
	"math/rand"
)

type col struct{ r, g, b float64 }

func hex(v uint32) col {
	return col{float64(v>>16&0xFF) / 255, float64(v>>8&0xFF) / 255, float64(v&0xFF) / 255}
}

var white = col{1, 1, 1}
var black = col{0, 0, 0}

func mix(a, b col, t float64) col {
	return col{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// exp(-x) para x >= 0 por tabela (as sombras chamam isso milhoes de vezes)
var expTab = func() []float64 {
	t := make([]float64, 2049)
	for i := range t {
		t[i] = math.Exp(-float64(i) / 256)
	}
	return t
}()

func expNeg(x float64) float64 {
	if x <= 0 {
		return 1
	}
	if x >= 8 {
		return 0
	}
	f := x * 256
	i := int(f)
	return expTab[i] + (expTab[i+1]-expTab[i])*(f-float64(i))
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func tobyte(v float64) byte {
	v = v*255 + 0.5
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return byte(v)
}

// canvas em BGRA de cima para baixo (formato de DIB do Windows).
type canvas struct {
	w, h int
	pix  []byte
}

func newCanvas(w, h int) *canvas { return &canvas{w, h, make([]byte, w*h*4)} }

func (c *canvas) get(x, y int) col {
	i := (y*c.w + x) * 4
	return col{float64(c.pix[i+2]) / 255, float64(c.pix[i+1]) / 255, float64(c.pix[i]) / 255}
}

func (c *canvas) set(x, y int, cl col) {
	i := (y*c.w + x) * 4
	c.pix[i], c.pix[i+1], c.pix[i+2], c.pix[i+3] = tobyte(cl.b), tobyte(cl.g), tobyte(cl.r), 255
}

func (c *canvas) blend(x, y int, cl col, a float64) {
	if a <= 0.0005 {
		return
	}
	if a >= 1 {
		c.set(x, y, cl)
		return
	}
	i := (y*c.w + x) * 4
	ia := 1 - a
	p := c.pix[i : i+4 : i+4]
	p[0] = byte(float64(p[0])*ia + cl.b*a*255 + 0.5)
	p[1] = byte(float64(p[1])*ia + cl.g*a*255 + 0.5)
	p[2] = byte(float64(p[2])*ia + cl.r*a*255 + 0.5)
	p[3] = 255
}

// amostragem bilinear
func (c *canvas) sample(x, y float64) col {
	x -= 0.5
	y -= 0.5
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x > float64(c.w-1) {
		x = float64(c.w - 1)
	}
	if y > float64(c.h-1) {
		y = float64(c.h - 1)
	}
	x0, y0 := int(x), int(y)
	x1, y1 := x0+1, y0+1
	if x1 >= c.w {
		x1 = c.w - 1
	}
	if y1 >= c.h {
		y1 = c.h - 1
	}
	fx, fy := x-float64(x0), y-float64(y0)
	a := mix(c.get(x0, y0), c.get(x1, y0), fx)
	b := mix(c.get(x0, y1), c.get(x1, y1), fx)
	return mix(a, b, fy)
}

// retangulo arredondado em pixels do dispositivo
type rr struct{ x, y, w, h, r float64 }

func (s rr) sd(px, py float64) float64 {
	cx, cy := s.x+s.w/2, s.y+s.h/2
	qx := math.Abs(px-cx) - s.w/2 + s.r
	qy := math.Abs(py-cy) - s.h/2 + s.r
	ox, oy := math.Max(qx, 0), math.Max(qy, 0)
	return math.Sqrt(ox*ox+oy*oy) + math.Min(math.Max(qx, qy), 0) - s.r
}

// normal apontando para fora
func (s rr) normal(px, py float64) (float64, float64) {
	cx, cy := s.x+s.w/2, s.y+s.h/2
	dx, dy := px-cx, py-cy
	sx, sy := 1.0, 1.0
	if dx < 0 {
		sx = -1
	}
	if dy < 0 {
		sy = -1
	}
	qx := math.Abs(dx) - s.w/2 + s.r
	qy := math.Abs(dy) - s.h/2 + s.r
	if qx > 0 && qy > 0 {
		l := math.Hypot(qx, qy)
		return sx * qx / l, sy * qy / l
	}
	if qx > qy {
		return sx, 0
	}
	return 0, sy
}

func (s rr) offset(dx, dy float64) rr { return rr{s.x + dx, s.y + dy, s.w, s.h, s.r} }

func (c *canvas) each(s rr, pad float64, fn func(x, y int, px, py, d float64)) {
	x0 := int(math.Floor(s.x - pad))
	y0 := int(math.Floor(s.y - pad))
	x1 := int(math.Ceil(s.x + s.w + pad))
	y1 := int(math.Ceil(s.y + s.h + pad))
	if x0 < 0 {
		x0 = 0
	}
	if y0 < 0 {
		y0 = 0
	}
	if x1 > c.w {
		x1 = c.w
	}
	if y1 > c.h {
		y1 = c.h
	}
	for y := y0; y < y1; y++ {
		py := float64(y) + 0.5
		for x := x0; x < x1; x++ {
			px := float64(x) + 0.5
			fn(x, y, px, py, s.sd(px, py))
		}
	}
}

func (c *canvas) fill(s rr, cl col, a float64) {
	if a <= 0.002 {
		return
	}
	c.each(s, 1, func(x, y int, px, py, d float64) { c.blend(x, y, cl, a*clamp01(0.5-d)) })
}

func (c *canvas) fillV(s rr, top, bot col, aTop, aBot float64) {
	if aTop <= 0.002 && aBot <= 0.002 {
		return
	}
	c.each(s, 1, func(x, y int, px, py, d float64) {
		t := clamp01((py - s.y) / s.h)
		c.blend(x, y, mix(top, bot, t), (aTop+(aBot-aTop)*t)*clamp01(0.5-d))
	})
}

// sombra/brilho difuso fora da forma
func (c *canvas) glow(s rr, cl col, a, k float64) {
	if a <= 0.002 {
		return
	}
	c.each(s, k*4, func(x, y int, px, py, d float64) {
		if d > -1 {
			c.blend(x, y, cl, a*expNeg(d/k)*clamp01(d+1))
		}
	})
}

func (c *canvas) shadow(s rr, a, k float64) {
	if a <= 0.002 {
		return
	}
	c.each(s, k*4, func(x, y int, px, py, d float64) {
		c.blend(x, y, black, a*expNeg(d/k))
	})
}

// contorno com brilho: forte em cima (luz vindo de cima), fraco embaixo
func (c *canvas) rim(s rr, width, aTop, aBot float64) {
	c.each(s, 1, func(x, y int, px, py, d float64) {
		cov := clamp01(0.5-d) - clamp01(0.5-(d+width))
		if cov <= 0 {
			return
		}
		nx, ny := s.normal(px, py)
		light := clamp01(-0.35*nx - 0.94*ny)
		c.blend(x, y, white, cov*(aBot+(aTop-aBot)*light))
	})
}

// painel de vidro: refrata o fundo perto da borda, clareia e aplica brilho de contorno
func (c *canvas) glass(s rr, bg *canvas, k float64) {
	c.shadow(s.offset(0, 8*k), 0.30, 12*k)
	band := 16 * k
	c.each(s, 1, func(x, y int, px, py, d float64) {
		cov := clamp01(0.5 - d)
		if cov <= 0 {
			return
		}
		inner := math.Max(-d, 0)
		nx, ny := s.normal(px, py)
		sx, sy := px, py
		edge := 0.0
		if inner < band {
			f := 1 - inner/band
			edge = f * f
			off := edge * 18 * k
			sx += nx * off
			sy += ny * off
		}
		base := bg.sample(sx, sy)
		t := clamp01((py - s.y) / s.h)
		cl := mix(base, white, 0.075+0.05*(1-t))
		cl = mix(cl, white, 0.10*edge)
		rim := clamp01(1 - inner/(1.4*k))
		light := 0.18 + 0.82*clamp01(-0.35*nx-0.94*ny)
		cl = mix(cl, white, rim*light*0.70)
		c.blend(x, y, cl, cov)
	})
}

// fundo: gradiente escuro com manchas de cor difusas
func paintBackground(c *canvas) {
	type blob struct {
		x, y, s, a float64
		cl         col
	}
	blobs := []blob{
		{0.12, 0.02, 0.36, 0.16, hex(0xFFFFFF)},
		{0.96, 0.42, 0.30, 0.08, hex(0xFFFFFF)},
		{0.30, 1.00, 0.34, 0.07, hex(0xFFFFFF)},
	}
	top, bot := hex(0x0E0E0F), hex(0x070707)
	rng := rand.New(rand.NewSource(7))
	w, h := float64(c.w), float64(c.h)
	for y := 0; y < c.h; y++ {
		fy := float64(y) / h
		base := mix(top, bot, fy)
		for x := 0; x < c.w; x++ {
			fx := float64(x) / w
			cl := base
			for _, b := range blobs {
				dx := (fx - b.x) * w
				dy := (fy - b.y) * w * 1.0
				sg := b.s * w
				g := b.a * math.Exp(-(dx*dx+dy*dy)/(2*sg*sg))
				cl = col{cl.r + b.cl.r*g*0.55, cl.g + b.cl.g*g*0.55, cl.b + b.cl.b*g*0.55}
			}
			n := (rng.Float64() - 0.5) / 255 * 1.5
			cl = col{1 - math.Exp(-cl.r*1.3) + n, 1 - math.Exp(-cl.g*1.3) + n, 1 - math.Exp(-cl.b*1.3) + n}
			c.set(x, y, cl)
		}
	}
}

// ---------- Widgets ----------

var accentTop, accentBot = hex(0xFAFAFA), hex(0xC9C9C9)

// chave liga/desliga; t = 0..1 (animado), hov = 0..1, alpha global
func drawToggle(c *canvas, s rr, t, hov, alpha, k float64) {
	c.fill(s, white, (0.12+0.06*hov)*alpha)
	if t > 0 {
		c.glow(s, white, 0.10*t*alpha, 6*k)
		c.fillV(s, accentTop, accentBot, t*alpha, t*alpha)
	}
	c.rim(s, 1.1*k, 0.45*alpha, 0.10*alpha)
	pad := 3 * k
	dk := s.h - 2*pad
	stretch := 7 * k * math.Sin(math.Pi*t)
	travel := s.w - 2*pad - dk
	kx := s.x + pad + t*travel - stretch*t
	knob := rr{kx, s.y + pad, dk + stretch, dk, dk / 2}
	c.shadow(knob.offset(0, 1.5*k), 0.30*alpha, 2.5*k)
	c.fillV(knob, mix(white, hex(0x1C1C1C), t), mix(hex(0xE4E4E4), hex(0x0E0E0E), t), alpha, alpha)
	c.rim(knob, 1.2*k, 0.9*alpha, 0.2*alpha)
}

// botao de vidro comum
func drawGlassButton(c *canvas, s rr, hov, press, alpha, k float64) {
	c.shadow(s.offset(0, 3*k), 0.22*alpha, 6*k)
	c.fillV(s, white, white, (0.16+0.08*hov-0.06*press)*alpha, (0.08+0.06*hov-0.04*press)*alpha)
	c.rim(s, 1.2*k, (0.55+0.2*hov)*alpha, 0.10*alpha)
}

// botao principal (laranja)
func drawAccentButton(c *canvas, s rr, hov, press, alpha, k float64) {
	c.glow(s, white, (0.08+0.10*hov)*alpha, 9*k)
	top := mix(accentTop, white, 0.12*hov)
	bot := mix(accentBot, black, 0.15*press)
	c.fillV(s, top, bot, alpha, alpha)
	gloss := rr{s.x + 2*k, s.y + 2*k, s.w - 4*k, s.h/2 - 2*k, s.r - 2*k}
	c.fillV(gloss, white, white, 0.28*alpha, 0.02*alpha)
	c.rim(s, 1.2*k, 0.75*alpha, 0.15*alpha)
}

// seletor de 3 opcoes com "pilula" de vidro que desliza
func drawSegmented(c *canvas, s rr, pos, target float64, n int, k float64) {
	c.fill(s, black, 0.22)
	c.rim(s, 1.1*k, 0.10, 0.22)
	segW := s.w / float64(n)
	base := segW - 6*k
	stretch := math.Min(math.Abs(pos-target), 1) * 16 * k
	cx := s.x + segW*pos + segW/2
	pw := base + stretch
	pill := rr{cx - pw/2, s.y + 3*k, pw, s.h - 6*k, (s.h - 6*k) / 2}
	c.shadow(pill.offset(0, 2*k), 0.25, 4*k)
	c.fillV(pill, white, white, 0.26, 0.14)
	c.rim(pill, 1.2*k, 0.85, 0.18)
}

func drawDot(c *canvas, cx, cy, r float64, cl col, k float64) {
	d := rr{cx - r, cy - r, 2 * r, 2 * r, r}
	c.glow(d, cl, 0.6, 3*k)
	c.fill(d, cl, 1)
}

// linha com pontas arredondadas (para os icones de minimizar/fechar)
func (c *canvas) line(x1, y1, x2, y2, width float64, cl col, a float64) {
	minX, maxX := math.Min(x1, x2)-width, math.Max(x1, x2)+width
	minY, maxY := math.Min(y1, y2)-width, math.Max(y1, y2)+width
	dx, dy := x2-x1, y2-y1
	ll := dx*dx + dy*dy
	for y := int(math.Max(0, math.Floor(minY))); y < int(math.Min(float64(c.h), math.Ceil(maxY))); y++ {
		for x := int(math.Max(0, math.Floor(minX))); x < int(math.Min(float64(c.w), math.Ceil(maxX))); x++ {
			px, py := float64(x)+0.5, float64(y)+0.5
			t := 0.0
			if ll > 0 {
				t = clamp01(((px-x1)*dx + (py-y1)*dy) / ll)
			}
			qx, qy := x1+t*dx-px, y1+t*dy-py
			d := math.Sqrt(qx*qx+qy*qy) - width/2
			c.blend(x, y, cl, a*clamp01(0.5-d))
		}
	}
}

// ---------- Janela sem moldura: cantos arredondados + sombra ----------

// windowFrame guarda a mascara dos cantos e a sombra pre-calculadas.
// A janela final tem uma margem M em volta do conteudo para a sombra.
type windowFrame struct {
	w, h, W, H, M int
	mask          []byte // cobertura do retangulo arredondado (conteudo)
	shadow        []byte // BGRA pre-multiplicado da sombra (janela inteira)
}

func newWindowFrame(w, h int, k float64) *windowFrame {
	m := int(math.Round(18 * k))
	f := &windowFrame{w: w, h: h, W: w + 2*m, H: h + 2*m, M: m}
	r := 22 * k
	body := rr{0, 0, float64(w), float64(h), r}
	f.mask = make([]byte, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			f.mask[y*w+x] = tobyte(clamp01(0.5 - body.sd(float64(x)+0.5, float64(y)+0.5)))
		}
	}
	sh := rr{float64(m), float64(m) + 5*k, float64(w), float64(h), r}
	f.shadow = make([]byte, f.W*f.H*4)
	for y := 0; y < f.H; y++ {
		for x := 0; x < f.W; x++ {
			d := sh.sd(float64(x)+0.5, float64(y)+0.5)
			f.shadow[(y*f.W+x)*4+3] = tobyte(0.42 * expNeg(math.Max(d, 0)/(6*k)))
		}
	}
	return f
}

// compose junta conteudo (BGRA opaco) + sombra no destino (BGRA pre-multiplicado)
func (f *windowFrame) compose(src, dst []byte) {
	copy(dst, f.shadow)
	for y := 0; y < f.h; y++ {
		si := y * f.w
		di := ((y+f.M)*f.W + f.M) * 4
		for x := 0; x < f.w; x, si, di = x+1, si+1, di+4 {
			m := f.mask[si]
			s := src[si*4 : si*4+4 : si*4+4]
			d := dst[di : di+4 : di+4]
			switch m {
			case 0:
			case 255:
				d[0], d[1], d[2], d[3] = s[0], s[1], s[2], 255
			default:
				a := float64(m) / 255
				d[0] = byte(float64(s[0])*a + 0.5)
				d[1] = byte(float64(s[1])*a + 0.5)
				d[2] = byte(float64(s[2])*a + 0.5)
				d[3] = byte(255*a + float64(d[3])*(1-a) + 0.5)
			}
		}
	}
}
