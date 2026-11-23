//go:build windows

package main

import (
	"fmt"
	"image"
	"image/png"
	"log"
	"math"
	"os"
	"path/filepath"
	"sync"
	"time"
	"unsafe"
)

// ---------- CS in tempo reale letti dalla barra in alto a destra del gioco ----------
// L'API di Riot aggiorna i CS solo a scatti di 10: BetterStomp legge il numero giallo
// dei CS direttamente dallo schermo (2 volte al secondo). Le cifre vengono imparate
// dall'orologio di gioco, di cui l'API fornisce il valore esatto.

var (
	pGetDC     = u32.NewProc("GetDC")
	pReleaseDC = u32.NewProc("ReleaseDC")
	pGetDIBits = g32.NewProc("GetDIBits")
)

type bmiHeader struct {
	Size          uint32
	Width         int32
	Height        int32
	Planes        uint16
	BitCount      uint16
	Compression   uint32
	SizeImage     uint32
	XPelsPerMeter int32
	YPelsPerMeter int32
	ClrUsed       uint32
	ClrImportant  uint32
}

const gW, gH = 8, 12 // griglia normalizzata di una cifra

type digitTpl struct {
	Count [10]int          `json:"count"`
	Sum   [10][gW * gH]int `json:"sum"`
}

var (
	ocrMu    sync.Mutex
	ocrTpl   digitTpl
	ocrRes   int32 // altezza schermo dei modelli
	ocrValue = -1
	ocrAt    time.Time
	ocrDirty bool
)

func ocrPath() string { return filepath.Join(dataDir, "ocr_cifre.json") }

func capture(x, y, w, h int32) []byte {
	sdc, _, _ := pGetDC.Call(0)
	if sdc == 0 {
		return nil
	}
	defer pReleaseDC.Call(0, sdc)
	mem, _, _ := pCreateCompatibleDC.Call(sdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(sdc, uintptr(w), uintptr(h))
	old, _, _ := pSelectObject.Call(mem, bmp)
	pBitBlt.Call(mem, 0, 0, uintptr(w), uintptr(h), sdc, uintptr(x), uintptr(y), srcCopy)
	pSelectObject.Call(mem, old)
	bi := bmiHeader{Size: 40, Width: w, Height: -h, Planes: 1, BitCount: 32}
	buf := make([]byte, int(w)*int(h)*4)
	r, _, _ := pGetDIBits.Call(mem, bmp, 0, uintptr(h), uintptr(unsafe.Pointer(&buf[0])), uintptr(unsafe.Pointer(&bi)), 0)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
	if r == 0 {
		return nil
	}
	return buf
}

type glyph struct{ x0, x1, y0, y1 int }

// colonne contigue di pixel "accesi" → glifi
func glyphs(mask []bool, w, h int, minH int) []glyph {
	var out []glyph
	start := -1
	colOn := func(x int) bool {
		for y := 0; y < h; y++ {
			if mask[y*w+x] {
				return true
			}
		}
		return false
	}
	for x := 0; x <= w; x++ {
		on := x < w && colOn(x)
		if on && start < 0 {
			start = x
		}
		if !on && start >= 0 {
			g := glyph{start, x - 1, h, -1}
			for yy := 0; yy < h; yy++ {
				for xx := g.x0; xx <= g.x1; xx++ {
					if mask[yy*w+xx] {
						if yy < g.y0 {
							g.y0 = yy
						}
						if yy > g.y1 {
							g.y1 = yy
						}
					}
				}
			}
			if g.y1-g.y0+1 >= minH {
				out = append(out, g)
			}
			start = -1
		}
	}
	return out
}

func grid(mask []bool, w int, g glyph) [gW * gH]bool {
	var out [gW * gH]bool
	gw, gh := g.x1-g.x0+1, g.y1-g.y0+1
	for cy := 0; cy < gH; cy++ {
		for cx := 0; cx < gW; cx++ {
			x0 := g.x0 + cx*gw/gW
			x1 := g.x0 + (cx+1)*gw/gW
			y0 := g.y0 + cy*gh/gH
			y1 := g.y0 + (cy+1)*gh/gH
			if x1 <= x0 {
				x1 = x0 + 1
			}
			if y1 <= y0 {
				y1 = y0 + 1
			}
			on, tot := 0, 0
			for y := y0; y < y1; y++ {
				for x := x0; x < x1; x++ {
					tot++
					if mask[y*w+x] {
						on++
					}
				}
			}
			out[cy*gW+cx] = on*3 >= tot
		}
	}
	return out
}

func recognize(g [gW * gH]bool, narrow bool) (int, int) {
	best, bestD := -1, 1<<30
	for d := 0; d < 10; d++ {
		n := ocrTpl.Count[d]
		if n < 2 {
			continue
		}
		dist := 0
		for i := range g {
			on := ocrTpl.Sum[d][i]*2 >= n
			if on != g[i] {
				dist++
			}
		}
		if d == 1 && narrow {
			dist -= 10 // l'"1" è molto più stretto delle altre cifre
		}
		if dist < bestD {
			best, bestD = d, dist
		}
	}
	return best, bestD
}

func ocrStep() {
	d, gt := live.snapshot()
	if d == nil {
		return
	}
	W, H := screenSize()
	s := float64(H) / 1080
	rw, rh := int32(420*s), int32(34*s)
	x0 := W - rw
	buf := capture(x0, 0, rw, rh)
	if buf == nil {
		return
	}
	w, h := int(rw), int(rh)
	yellow := make([]bool, w*h)
	white := make([]bool, w*h)
	for i := 0; i < w*h; i++ {
		b, g, r := int(buf[i*4]), int(buf[i*4+1]), int(buf[i*4+2])
		// cifre dei CS: giallo chiaro; orologio: bianco/grigio chiaro
		yellow[i] = r > 200 && g > 195 && b > 60 && b < 185 && abs(r-g) < 22
		mx, mn := max3(r, g, b), min3(r, g, b)
		white[i] = r > 150 && g > 150 && b > 150 && mx-mn < 35
	}
	minH := int(7 * s)
	ys := glyphs(yellow, w, h, minH)
	ws := glyphs(white, w, h, int(2*s))
	csRight := -1
	if len(ys) > 0 {
		csRight = ys[len(ys)-1].x1
	}
	// orologio: glifi bianchi a destra dei CS
	var clock []glyph
	for _, g := range ws {
		if g.x0 > csRight {
			clock = append(clock, g)
		}
	}
	ocrMu.Lock()
	defer ocrMu.Unlock()
	// diagnostica: ogni 20 s salva la striscia letta e cosa ha capito (per sistemare la lettura)
	if time.Since(ocrDbgAt) > 20*time.Second {
		ocrDbgAt = time.Now()
		saveStrip(buf, w, h)
		api := -1
		for i := range d.AllPlayers {
			if samePlayer(d.AllPlayers[i], d.ActivePlayer.RiotID, d.ActivePlayer.RiotName) || samePlayer(d.AllPlayers[i], d.ActivePlayer.SummonerName, "") {
				api = d.AllPlayers[i].Scores.CreepScore
			}
		}
		log.Printf("OCR: schermo %dx%d, glifi gialli %d, glifi orologio %d, cifre imparate %v, CS dal gioco %d, ultima lettura %d", W, H, len(ys), len(clock), ocrTpl.Count, api, ocrValue)
	}
	if ocrRes != H {
		ocrTpl, ocrRes = digitTpl{}, H
		var saved struct {
			H   int32    `json:"h"`
			Tpl digitTpl `json:"tpl"`
		}
		if readJSON(ocrPath(), &saved) == nil && saved.H == H {
			ocrTpl = saved.Tpl
		}
	}
	// impara le cifre dall'orologio (mm:ss), lontano dal cambio di secondo
	frac := gt - math.Floor(gt)
	if len(clock) == 5 && frac > 0.2 && frac < 0.8 {
		sec := int(gt)
		txt := fmt.Sprintf("%02d:%02d", sec/60, sec%60)
		colonW := clock[2].x1 - clock[2].x0 + 1
		if colonW <= int(4*s)+1 && clock[2].y1-clock[2].y0 < clock[0].y1-clock[0].y0 {
			for i, g := range clock {
				if i == 2 {
					continue
				}
				dg := int(txt[i] - '0')
				gr := grid(white, w, g)
				ocrTpl.Count[dg]++
				for k, on := range gr {
					if on {
						ocrTpl.Sum[dg][k]++
					}
				}
				if ocrTpl.Count[dg] > 400 { // dimezzo per restare reattivo
					ocrTpl.Count[dg] /= 2
					for k := range ocrTpl.Sum[dg] {
						ocrTpl.Sum[dg][k] /= 2
					}
				}
			}
			ocrDirty = true
		}
	}
	// legge i CS
	if len(ys) == 0 || len(ys) > 4 {
		return
	}
	val := 0
	for _, g := range ys {
		gw, gh := g.x1-g.x0+1, g.y1-g.y0+1
		dg, dist := recognize(grid(yellow, w, g), float64(gw)/float64(gh) < 0.42)
		if dg < 0 || dist > 26 {
			return
		}
		val = val*10 + dg
	}
	ocrValue, ocrAt = val, time.Now()
}

var ocrDbgAt time.Time

func saveStrip(buf []byte, w, h int) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for i := 0; i < w*h; i++ {
		img.Pix[i*4], img.Pix[i*4+1], img.Pix[i*4+2], img.Pix[i*4+3] = buf[i*4+2], buf[i*4+1], buf[i*4], 255
	}
	if f, err := os.Create(filepath.Join(dataDir, "ocr_striscia.png")); err == nil {
		png.Encode(f, img)
		f.Close()
	}
}

func abs(a int) int {
	if a < 0 {
		return -a
	}
	return a
}
func max3(a, b, c int) int { return max(a, max(b, c)) }
func min3(a, b, c int) int { return min(a, min(b, c)) }

func startOCR() {
	lastSave := time.Now()
	for {
		time.Sleep(400 * time.Millisecond)
		if !getSettings().Overlay.OCR {
			continue
		}
		func() {
			defer func() { recover() }()
			ocrStep()
		}()
		ocrMu.Lock()
		if ocrDirty && time.Since(lastSave) > 20*time.Second {
			writeJSON(ocrPath(), struct {
				H   int32    `json:"h"`
				Tpl digitTpl `json:"tpl"`
			}{ocrRes, ocrTpl})
			ocrDirty = false
			lastSave = time.Now()
		}
		ocrMu.Unlock()
	}
}

// CS letti dallo schermo, se recenti
func ocrCS() (int, bool) {
	ocrMu.Lock()
	defer ocrMu.Unlock()
	if ocrValue < 0 || time.Since(ocrAt) > 2*time.Second {
		return 0, false
	}
	return ocrValue, true
}
