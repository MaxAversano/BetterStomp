//go:build windows

package main

// Disegno dei riquadri "ricchi" dell'overlay: scheda con ritratto del campione
// (KDA, CS, timer obiettivi) e schermata delle squadre.

import (
	"bytes"
	"image"
	"image/color"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unsafe"
)

var (
	pStretchDIBits = g32.NewProc("StretchDIBits")

	portraitMu   sync.Mutex
	portraitSrc  = map[string]image.Image{} // id campione → immagine originale
	portraitPix  = map[string][]byte{}      // "id|lato|sfondo" → pixel BGRA già ritagliati a cerchio
	portraitWant = map[string]bool{}
)

// ritratto del campione (scaricato una volta da Data Dragon e tenuto in cartella)
func portraitImage(id string) image.Image {
	if id == "" {
		return nil
	}
	v := static.version()
	return remoteImage(id, "https://ddragon.leagueoflegends.com/cdn/"+v+"/img/champion/"+id+".png", filepath.Join(dataDir, "ddragon", v, "img", "champion", id+".png"))
}

// immagine scaricata una volta e tenuta in cartella; nil finché non è pronta
func remoteImage(id, u, p string) image.Image {
	portraitMu.Lock()
	img, ok := portraitSrc[id]
	if ok || portraitWant[id] {
		portraitMu.Unlock()
		return img
	}
	portraitWant[id] = true
	portraitMu.Unlock()
	go func() {
		b, err := os.ReadFile(p)
		if err != nil {
			c := &http.Client{Timeout: 10 * time.Second}
			resp, err := c.Get(u)
			if err != nil {
				portraitMu.Lock()
				delete(portraitWant, id) // riprova più avanti
				portraitMu.Unlock()
				return
			}
			b, _ = io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode == 200 {
				os.MkdirAll(filepath.Dir(p), 0o755)
				os.WriteFile(p, b, 0o644)
			}
		}
		im, _, err := image.Decode(bytes.NewReader(b))
		if err != nil {
			return
		}
		portraitMu.Lock()
		portraitSrc[id] = im
		portraitMu.Unlock()
	}()
	return nil
}

// pixel del ritratto ridimensionato e ritagliato a cerchio sopra il colore di sfondo
func portraitBits(id string, size int, bg uint32) []byte {
	key := id + "|" + itoa(size) + "|" + itoa(int(bg))
	portraitMu.Lock()
	if px, ok := portraitPix[key]; ok {
		portraitMu.Unlock()
		return px
	}
	portraitMu.Unlock()
	src := portraitImage(id)
	if src == nil {
		return nil
	}
	b := src.Bounds()
	// i ritratti di Data Dragon hanno un bordino: lo taglio un po'
	crop := float64(b.Dx()) * 0.07
	sw := float64(b.Dx()) - 2*crop
	px := make([]byte, size*size*4)
	bgR, bgG, bgB := float64(bg&0xFF), float64(bg>>8&0xFF), float64(bg>>16&0xFF)
	c := float64(size-1) / 2
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			// media 2x2 per un ridimensionamento morbido
			var r, g, bl float64
			for sy := 0; sy < 2; sy++ {
				for sx := 0; sx < 2; sx++ {
					fx := crop + (float64(x)+0.25+0.5*float64(sx))*sw/float64(size)
					fy := crop + (float64(y)+0.25+0.5*float64(sy))*sw/float64(size)
					cr, cg, cb, _ := color.NRGBAModel.Convert(src.At(b.Min.X+int(fx), b.Min.Y+int(fy))).RGBA()
					r += float64(cr >> 8)
					g += float64(cg >> 8)
					bl += float64(cb >> 8)
				}
			}
			r, g, bl = r/4, g/4, bl/4
			// bordo del cerchio sfumato
			d := math.Hypot(float64(x)-c, float64(y)-c)
			a := math.Max(0, math.Min(1, c-d+0.5))
			i := (y*size + x) * 4
			px[i] = byte(bl*a + bgB*(1-a))
			px[i+1] = byte(g*a + bgG*(1-a))
			px[i+2] = byte(r*a + bgR*(1-a))
		}
	}
	portraitMu.Lock()
	portraitPix[key] = px
	portraitMu.Unlock()
	return px
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}

func blitBGRA(hdc uintptr, x, y int32, size int, px []byte) {
	var bmi struct {
		size                 uint32
		w, h                 int32
		planes, bits         uint16
		compression, imgSize uint32
		xppm, yppm           int32
		clrUsed, clrImp      uint32
	}
	bmi.size = uint32(unsafe.Sizeof(bmi))
	bmi.w, bmi.h = int32(size), -int32(size) // dall'alto in basso
	bmi.planes, bmi.bits = 1, 32
	pStretchDIBits.Call(hdc, uintptr(x), uintptr(y), uintptr(size), uintptr(size), 0, 0, uintptr(size), uintptr(size),
		uintptr(unsafe.Pointer(&px[0])), uintptr(unsafe.Pointer(&bmi)), 0, srcCopy)
}

// cerchio con il ritratto, o con l'iniziale finché l'immagine non è pronta
func drawPortrait(hdc uintptr, id, name string, x, y int32, size int32, ring uintptr, k float64) {
	bg := rgb("#1B1D24")
	roundRect(hdc, rect{x - 2, y - 2, x + size + 2, y + size + 2}, size+4, ring, ring)
	if px := portraitBits(id, int(size), uint32(bg)); px != nil {
		blitBGRA(hdc, x, y, int(size), px)
		return
	}
	roundRect(hdc, rect{x, y, x + size, y + size}, size, rgb("#2C2F3A"), rgb("#2C2F3A"))
	if name != "" {
		drawText(hdc, strings.ToUpper(name[:1]), rect{x, y, x + size, y + size}, rgb("#E6E8EE"), font(int(float64(size)*0.45), true), dtCenter)
	}
}

// icona di un obiettivo: la sagoma ufficiale (da CommunityDragon) colorata, in un cerchio
func objIcon(name string) (string, string) {
	n := strings.ToLower(name)
	switch {
	case strings.Contains(n, "antico"):
		return "elder", "#B8F0FF"
	case strings.Contains(n, "drag"):
		return "dragon", "#FF8A3D"
	case strings.Contains(n, "baron") || strings.Contains(n, "barone"):
		return "baron", "#B48CFF"
	case strings.Contains(n, "araldo"):
		return "herald", "#C79BFF"
	case strings.Contains(n, "larve"):
		return "grub", "#A98BFF"
	case strings.Contains(n, "atakhan"):
		return "atakhan", "#FF5C5C"
	}
	return "", "#38D6FF"
}

// icone a colori della minimappa di gioco (CommunityDragon)
func objIconURL(kind string) string {
	file := map[string]string{"dragon": "dragon", "elder": "dragon_elder", "baron": "baron", "herald": "riftherald", "grub": "grub", "atakhan": "atakhan_v"}[kind]
	return "https://raw.communitydragon.org/latest/game/assets/ux/minimap/icons/" + file + ".png"
}

// icona a colori ridimensionata, fusa sopra lo sfondo del cerchio
func iconBits(kind string, size int, bg uint32) []byte {
	key := "obj2:" + kind + "|" + itoa(size)
	portraitMu.Lock()
	if px, ok := portraitPix[key]; ok {
		portraitMu.Unlock()
		return px
	}
	portraitMu.Unlock()
	src := remoteImage("obj2:"+kind, objIconURL(kind), filepath.Join(dataDir, "icone", "mm_"+kind+".png"))
	if src == nil {
		return nil
	}
	b := src.Bounds()
	px := make([]byte, size*size*4)
	br, bgg, bb := float64(bg&0xFF), float64(bg>>8&0xFF), float64(bg>>16&0xFF)
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, bl, a float64
			for sy := 0; sy < 3; sy++ {
				for sx := 0; sx < 3; sx++ {
					fx := (float64(x) + (float64(sx)+0.5)/3) * float64(b.Dx()) / float64(size)
					fy := (float64(y) + (float64(sy)+0.5)/3) * float64(b.Dy()) / float64(size)
					c := color.NRGBAModel.Convert(src.At(b.Min.X+int(fx), b.Min.Y+int(fy))).(color.NRGBA)
					al := float64(c.A) / 255
					r += float64(c.R) * al
					g += float64(c.G) * al
					bl += float64(c.B) * al
					a += al
				}
			}
			r, g, bl, a = r/9, g/9, bl/9, a/9
			i := (y*size + x) * 4
			px[i] = byte(math.Min(255, bl+bb*(1-a)))
			px[i+1] = byte(math.Min(255, g+bgg*(1-a)))
			px[i+2] = byte(math.Min(255, r+br*(1-a)))
		}
	}
	portraitMu.Lock()
	portraitPix[key] = px
	portraitMu.Unlock()
	return px
}

func drawObjIcon(hdc uintptr, name string, r rect) {
	kind, col := objIcon(name)
	bg := rgb("#24202A")
	roundRect(hdc, r, r.r-r.l, bg, rgb(col))
	in := int((r.r - r.l) * 80 / 100)
	if kind != "" {
		if px := iconBits(kind, in, uint32(bg)); px != nil {
			off := (r.r - r.l - int32(in)) / 2
			blitBGRA(hdc, r.l+off, r.t+off, in, px)
			return
		}
	}
	d := (r.r - r.l) / 3 // in attesa dell'icona: un pallino colorato
	roundRect(hdc, rect{r.l + d, r.t + d, r.r - d, r.b - d}, d, rgb(col), rgb(col))
}

// scheda in stile "card": ritratto, KDA, CS e timer degli obiettivi
func drawCard(hdc uintptr, w OWidget, x, y int32, k float64, interactive bool, s OverlaySettings) (rect, []iconHit) {
	sc := func(v float64) int32 { return int32(v*k + 0.5) }
	c := w.Card
	width := sc(200)
	pad := sc(14)
	topH := sc(50)
	csH := sc(46)
	rowH := sc(20)
	objH := sc(44)
	h := pad + topH + csH + rowH*int32(len(c.Rows)) + pad
	if len(c.Objs) > 0 {
		h += sc(12) + objH*int32(len(c.Objs))
	}
	if w.Anchor == "center" {
		x -= width / 2
	}
	x, y = clampXY(x, y, width, h)
	r := rect{x, y, x + width, y + h}
	roundRect(hdc, r, sc(14), rgb("#1B1D24"), rgb("#2C2F3A"))
	label := rgb("#E6E8EE")
	muted := rgb("#8C93A3")
	left := x + pad
	right := x + width - pad
	cy := y + pad
	tx := left + sc(14)
	drawText(hdc, "KDA", rect{tx, cy, right, cy + sc(18)}, label, font(int(13*k+0.5), true), dtLeft)
	drawText(hdc, c.KDA, rect{tx, cy + sc(18), right, cy + sc(46)}, label, font(int(18*k+0.5), true), dtLeft)
	var icons []iconHit
	if w.Icons && interactive { // lucchetto / impostazioni / chiudi solo quando l'overlay è cliccabile
		is := sc(15)
		ix := right - is
		sym := symFont(int(10*k + 0.5))
		lockTxt, lockCol := "🔓", muted
		if w.Locked {
			lockTxt, lockCol = "🔒", rgb("#38D6FF")
		}
		for _, ic := range []struct {
			a, t string
			col  uintptr
		}{{"close", "✕", muted}, {"gear", "⚙", muted}, {"lock", lockTxt, lockCol}} {
			ir := rect{ix, y + sc(6), ix + is, y + sc(6) + is}
			drawText(hdc, ic.t, ir, ic.col, sym, dtCenter)
			icons = append(icons, iconHit{w.Key, ic.a, rect{ir.l - 2, ir.t - 2, ir.r + 2, ir.b + 2}})
			ix -= is + sc(4)
		}
	}
	cy += topH
	drawText(hdc, "CS", rect{left + sc(14), cy, right, cy + sc(18)}, label, font(int(13*k+0.5), true), dtLeft)
	drawText(hdc, c.CS, rect{left + sc(14), cy + sc(18), left + sc(80), cy + csH}, rgb(c.CSColor), font(int(18*k+0.5), true), dtLeft)
	drawText(hdc, c.CSSub, rect{left + sc(70), cy + sc(18), right, cy + csH}, muted, font(int(10.5*k+0.5), false), dtRight)
	cy += csH
	for _, l := range c.Rows {
		ef := font(int(11.5*k+0.5), false)
		ex := ""
		if l.Extra != "" {
			ex = " / " + l.Extra
		}
		ew := textWidth(hdc, ex, ef)
		drawText(hdc, ex, rect{left, cy, right, cy + rowH}, muted, ef, dtRight)
		drawText(hdc, l.Right, rect{left, cy, right - ew, cy + rowH}, rgb(l.Color), font(int(11.5*k+0.5), true), dtRight)
		drawText(hdc, l.Text, rect{left + sc(14), cy, right, cy + rowH}, muted, font(int(11.5*k+0.5), true), dtLeft)
		cy += rowH
	}
	if len(c.Objs) > 0 {
		cy += sc(6)
		fillRect(hdc, rect{x + 1, cy, x + width - 1, cy + 1}, rgb("#2C2F3A"))
		cy += sc(6)
		for _, o := range c.Objs {
			is := sc(34)
			ir := rect{left + sc(2), cy + (objH-is)/2, left + sc(2) + is, cy + (objH-is)/2 + is}
			drawObjIcon(hdc, o.Text, ir)
			nx := ir.r + sc(12)
			drawText(hdc, o.Text, rect{nx, cy + sc(4), right, cy + sc(22)}, label, font(int(12*k+0.5), true), dtLeft|dtEndEllipsis)
			drawText(hdc, o.Right, rect{nx, cy + sc(21), right, cy + objH - sc(2)}, rgb(o.Color), font(int(15*k+0.5), true), dtLeft)
			cy += objH
		}
	}
	if interactive && w.Key != "" {
		if s.EditMode {
			dashRect(hdc, r, rgb("#38D6FF"))
		} else if !w.Locked {
			dashRect(hdc, r, rgb("#5C6378"))
		}
	}
	return r, icons
}

// schermata delle squadre: due colonne con ritratto, campione, giocatore, rango e win rate
func drawTeams(hdc uintptr, w OWidget, x, y int32, k float64, interactive bool, s OverlaySettings) (rect, []iconHit) {
	sc := func(v float64) int32 { return int32(v*k + 0.5) }
	t := w.Teams
	n := len(t.Ally)
	if len(t.Enemy) > n {
		n = len(t.Enemy)
	}
	width := sc(780)
	pad := sc(16)
	headH := sc(40)
	rowH := sc(52)
	h := headH + rowH*int32(n) + pad
	x -= width / 2
	x, y = clampXY(x, y, width, h)
	r := rect{x, y, x + width, y + h}
	roundRect(hdc, r, sc(14), rgb("#1B1D24"), rgb("#2C2F3A"))
	muted := rgb("#8C93A3")
	colW := (width - pad*3) / 2
	var icons []iconHit
	for side, rows := range [][]OTeamRow{t.Ally, t.Enemy} {
		cx := x + pad + int32(side)*(colW+pad)
		title, col := "LA TUA SQUADRA", rgb("#38D6FF")
		if side == 1 {
			title, col = "SQUADRA NEMICA", rgb("#FF5C7A")
		}
		fillRect(hdc, rect{cx, y + sc(14), cx + sc(3), y + sc(28)}, col)
		pSetTextCharacterExtra.Call(hdc, uintptr(sc(1.6)))
		drawText(hdc, title, rect{cx + sc(10), y + sc(8), cx + colW, y + sc(34)}, col, font(int(11*k+0.5), true), dtLeft)
		pSetTextCharacterExtra.Call(hdc, 0)
		cy := y + headH
		for _, p := range rows {
			if p.Me {
				roundRect(hdc, rect{cx - sc(6), cy + sc(2), cx + colW + sc(6), cy + rowH - sc(2)}, sc(10), rgb("#232733"), rgb("#2E3444"))
			}
			ps := sc(38)
			drawPortrait(hdc, p.Champ, p.ChampNm, cx+2, cy+(rowH-ps)/2, ps, col, k)
			nx := cx + ps + sc(12)
			rw := sc(120)
			drawText(hdc, p.ChampNm, rect{nx, cy + sc(7), cx + colW - rw, cy + sc(26)}, rgb("#E6E8EE"), font(int(13*k+0.5), true), dtLeft|dtEndEllipsis)
			drawText(hdc, p.Name, rect{nx, cy + sc(26), cx + colW - rw, cy + sc(44)}, muted, font(int(11*k+0.5), false), dtLeft|dtEndEllipsis)
			drawText(hdc, p.Rank, rect{cx + colW - rw, cy + sc(7), cx + colW, cy + sc(26)}, rgb(p.RankCol), font(int(12*k+0.5), true), dtRight)
			sub := p.WR
			if p.Games != "" {
				sub += "  ·  " + p.Games
			}
			drawText(hdc, sub, rect{cx + colW - rw - sc(40), cy + sc(26), cx + colW, cy + sc(44)}, rgb(p.WRCol), font(int(11*k+0.5), true), dtRight)
			cy += rowH
		}
	}
	if w.Icons && interactive {
		is := sc(15)
		ix := r.r - pad - is
		sym := symFont(int(10*k + 0.5))
		lockTxt, lockCol := "🔓", muted
		if w.Locked {
			lockTxt, lockCol = "🔒", rgb("#38D6FF")
		}
		for _, ic := range []struct {
			a, t string
			col  uintptr
		}{{"close", "✕", muted}, {"lock", lockTxt, lockCol}} {
			ir := rect{ix, y + sc(12), ix + is, y + sc(12) + is}
			drawText(hdc, ic.t, ir, ic.col, sym, dtCenter)
			icons = append(icons, iconHit{w.Key, ic.a, rect{ir.l - 2, ir.t - 2, ir.r + 2, ir.b + 2}})
			ix -= is + sc(4)
		}
		if s.EditMode {
			dashRect(hdc, r, rgb("#38D6FF"))
		} else if !w.Locked {
			dashRect(hdc, r, rgb("#5C6378"))
		}
	}
	return r, icons
}
