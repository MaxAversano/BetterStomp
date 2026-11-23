//go:build windows

package main

import (
	"math"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"unsafe"
)

// ---------- Overlay in-game nativo (finestra trasparente, sempre in primo piano, click-through) ----------
// Funziona con League in modalità "Senza bordi" (borderless). In schermo intero esclusivo
// Windows non permette a nessuna finestra di stare sopra il gioco.

var (
	u32 = syscall.NewLazyDLL("user32.dll")
	g32 = syscall.NewLazyDLL("gdi32.dll")
	k32 = syscall.NewLazyDLL("kernel32.dll")

	pRegisterClassEx  = u32.NewProc("RegisterClassExW")
	pCreateWindowEx   = u32.NewProc("CreateWindowExW")
	pDefWindowProc    = u32.NewProc("DefWindowProcW")
	pGetMessage       = u32.NewProc("GetMessageW")
	pTranslateMessage = u32.NewProc("TranslateMessage")
	pDispatchMessage  = u32.NewProc("DispatchMessageW")
	pSetLayeredAttr   = u32.NewProc("SetLayeredWindowAttributes")
	pSetWindowPos     = u32.NewProc("SetWindowPos")
	pShowWindow       = u32.NewProc("ShowWindow")
	pInvalidateRect   = u32.NewProc("InvalidateRect")
	pBeginPaint       = u32.NewProc("BeginPaint")
	pEndPaint         = u32.NewProc("EndPaint")
	pSetTimer         = u32.NewProc("SetTimer")
	pRegisterHotKey   = u32.NewProc("RegisterHotKey")
	pUnregisterHotKey = u32.NewProc("UnregisterHotKey")
	pGetAsyncKeyState = u32.NewProc("GetAsyncKeyState")
	pGetSystemMetrics = u32.NewProc("GetSystemMetrics")
	pFillRect         = u32.NewProc("FillRect")
	pDrawText         = u32.NewProc("DrawTextW")
	pGetModuleHandle  = k32.NewProc("GetModuleHandleW")

	pCreateCompatibleDC    = g32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBmp   = g32.NewProc("CreateCompatibleBitmap")
	pSelectObject          = g32.NewProc("SelectObject")
	pDeleteObject          = g32.NewProc("DeleteObject")
	pDeleteDC              = g32.NewProc("DeleteDC")
	pBitBlt                = g32.NewProc("BitBlt")
	pCreateSolidBrush      = g32.NewProc("CreateSolidBrush")
	pCreatePen             = g32.NewProc("CreatePen")
	pRoundRect             = g32.NewProc("RoundRect")
	pCreateFont            = g32.NewProc("CreateFontW")
	pSetTextColor          = g32.NewProc("SetTextColor")
	pSetBkMode             = g32.NewProc("SetBkMode")
	pGetStockObject        = g32.NewProc("GetStockObject")
	pSetTextCharacterExtra = g32.NewProc("SetTextCharacterExtra")
	pRectangle             = g32.NewProc("Rectangle")
	pSetCapture            = u32.NewProc("SetCapture")
	pReleaseCapture        = u32.NewProc("ReleaseCapture")
	pGetWindowLongPtr      = u32.NewProc("GetWindowLongPtrW")
	pSetWindowLongPtr      = u32.NewProc("SetWindowLongPtrW")
	pLoadCursor            = u32.NewProc("LoadCursorW")
	pSetCursor             = u32.NewProc("SetCursor")
)

const (
	wsExLayered     = 0x00080000
	wsExTransparent = 0x00000020
	wsExTopmost     = 0x00000008
	wsExToolWindow  = 0x00000080
	wsExNoActivate  = 0x08000000
	wsPopup         = 0x80000000
	lwaColorKey     = 0x1
	wmPaint         = 0x000F
	wmTimer         = 0x0113
	wmHotkey        = 0x0312
	wmEraseBkgnd    = 0x0014
	swpNoMove       = 0x0002
	swpNoSize       = 0x0001
	swpNoActivate   = 0x0010
	swShowNA        = 8
	swHide          = 0
	modAlt          = 0x1
	modControl      = 0x2
	modShift        = 0x4
	modNoRepeat     = 0x4000
	dtLeft          = 0x0
	dtCenter        = 0x1
	dtRight         = 0x2
	dtVCenter       = 0x4
	dtSingleLine    = 0x20
	dtNoPrefix      = 0x800
	dtEndEllipsis   = 0x8000
	srcCopy         = 0x00CC0020
	keyColor        = 0x00FF00FF // magenta = trasparente
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }
type msgT struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	priv    uint32
}
type rect struct{ l, t, r, b int32 }
type paintStruct struct {
	hdc      uintptr
	fErase   int32
	rcPaint  rect
	fRestore int32
	fIncUp   int32
	reserved [32]byte
}

// le scorciatoie sono attive solo durante la partita (così Ctrl+T ecc. restano liberi fuori dal gioco)
var hotkeysOn bool

func setHotkeys(on bool) {
	if on == hotkeysOn {
		return
	}
	hotkeysOn = on
	ids := []int{20, 21, 22, 23}
	for i := 1; i <= 6; i++ {
		ids = append(ids, i, 10+i)
	}
	if !on {
		for _, id := range ids {
			pUnregisterHotKey.Call(ovHwnd, uintptr(id))
		}
		return
	}
	for i := 1; i <= 6; i++ {
		pRegisterHotKey.Call(ovHwnd, uintptr(i), modControl|modShift|modNoRepeat, uintptr('0'+i))
		pRegisterHotKey.Call(ovHwnd, uintptr(10+i), modControl|modAlt|modNoRepeat, uintptr('0'+i))
	}
	pRegisterHotKey.Call(ovHwnd, 20, modControl|modNoRepeat, 'T')
	pRegisterHotKey.Call(ovHwnd, 21, modControl|modShift|modNoRepeat, 'O')
	pRegisterHotKey.Call(ovHwnd, 22, modControl|modShift|modNoRepeat, 'C')
	pRegisterHotKey.Call(ovHwnd, 23, modControl|modShift|modNoRepeat, 'E')
}

var (
	ovHwnd    uintptr
	ovW, ovH  int32
	ovModel   OverlayModel
	ovShown   bool
	ovHidden  bool // nascosto con Ctrl+Shift+O
	ovFonts   = map[int]uintptr{}
	ovGameKey string
)

func rgb(hex string) uintptr {
	h := strings.TrimPrefix(hex, "#")
	if len(h) != 6 {
		return 0x00FFFFFF
	}
	v, _ := strconv.ParseUint(h, 16, 32)
	r, g, b := (v>>16)&0xFF, (v>>8)&0xFF, v&0xFF
	if r == 255 && g == 0 && b == 255 {
		g = 1
	}
	return uintptr(r | g<<8 | b<<16)
}

func font(size int, bold bool) uintptr {
	key := size * 2
	if bold {
		key++
	}
	if f, ok := ovFonts[key]; ok {
		return f
	}
	w := 500
	if bold {
		w = 700
	}
	face, _ := syscall.UTF16PtrFromString("Segoe UI")
	neg := -size
	f, _, _ := pCreateFont.Call(uintptr(neg), 0, 0, 0, uintptr(w), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	ovFonts[key] = f
	return f
}

func drawText(hdc uintptr, s string, r rect, color uintptr, f uintptr, flags uintptr) {
	p, err := syscall.UTF16FromString(s)
	if err != nil {
		return
	}
	pSelectObject.Call(hdc, f)
	pSetTextColor.Call(hdc, color)
	pDrawText.Call(hdc, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), uintptr(unsafe.Pointer(&r)), flags|dtSingleLine|dtVCenter|dtNoPrefix)
}

func roundRect(hdc uintptr, r rect, rad int32, fill, border uintptr) {
	br, _, _ := pCreateSolidBrush.Call(fill)
	pen, _, _ := pCreatePen.Call(0, 1, border)
	ob, _, _ := pSelectObject.Call(hdc, br)
	op, _, _ := pSelectObject.Call(hdc, pen)
	pRoundRect.Call(hdc, uintptr(r.l), uintptr(r.t), uintptr(r.r), uintptr(r.b), uintptr(rad), uintptr(rad))
	pSelectObject.Call(hdc, ob)
	pSelectObject.Call(hdc, op)
	pDeleteObject.Call(br)
	pDeleteObject.Call(pen)
}

func fillRect(hdc uintptr, r rect, color uintptr) {
	br, _, _ := pCreateSolidBrush.Call(color)
	pFillRect.Call(hdc, uintptr(unsafe.Pointer(&r)), br)
	pDeleteObject.Call(br)
}

type hitRect struct {
	key    string
	r      rect
	center bool
}

var (
	ovHits   []hitRect
	dragKey  string
	dragOffX int32
	dragOffY int32
	dragW    int32
	dragC    bool
)

func textWidth(hdc uintptr, s string, f uintptr) int32 {
	p, err := syscall.UTF16FromString(s)
	if err != nil || len(p) <= 1 {
		return 0
	}
	pSelectObject.Call(hdc, f)
	r := rect{0, 0, 2000, 100}
	pDrawText.Call(hdc, uintptr(unsafe.Pointer(&p[0])), uintptr(len(p)-1), uintptr(unsafe.Pointer(&r)), 0x400|dtSingleLine|dtNoPrefix) // DT_CALCRECT
	return r.r - r.l
}

func dashRect(hdc uintptr, r rect, color uintptr) {
	pen, _, _ := pCreatePen.Call(1, 1, color) // PS_DASH
	nb, _, _ := pGetStockObject.Call(5)       // NULL_BRUSH
	op, _, _ := pSelectObject.Call(hdc, pen)
	ob, _, _ := pSelectObject.Call(hdc, nb)
	pRectangle.Call(hdc, uintptr(r.l-3), uintptr(r.t-3), uintptr(r.r+3), uintptr(r.b+3))
	pSelectObject.Call(hdc, op)
	pSelectObject.Call(hdc, ob)
	pDeleteObject.Call(pen)
}

// font con nome (per i simboli: lucchetto, ingranaggio, chiudi)
var symFonts = map[int]uintptr{}

func symFont(size int) uintptr {
	if f, ok := symFonts[size]; ok {
		return f
	}
	face, _ := syscall.UTF16PtrFromString("Segoe UI Symbol")
	neg := -size
	f, _, _ := pCreateFont.Call(uintptr(neg), 0, 0, 0, 400, 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(face)))
	symFonts[size] = f
	return f
}

// testo con contorno scuro (timer sulla minimappa, come Blitz)
func outlinedText(hdc uintptr, s string, r rect, color uintptr, f uintptr) {
	for _, d := range [][2]int32{{-1, 0}, {1, 0}, {0, -1}, {0, 1}, {-1, -1}, {1, 1}, {-1, 1}, {1, -1}} {
		drawText(hdc, s, rect{r.l + d[0], r.t + d[1], r.r + d[0], r.b + d[1]}, 0x00080808, f, dtCenter)
	}
	drawText(hdc, s, r, color, f, dtCenter)
}

type iconHit struct {
	key    string
	action string // lock | gear | close
	r      rect
}

var ovIcons []iconHit

func paintOverlay(hdc uintptr) {
	mem, _, _ := pCreateCompatibleDC.Call(hdc)
	bmp, _, _ := pCreateCompatibleBmp.Call(hdc, uintptr(ovW), uintptr(ovH))
	old, _, _ := pSelectObject.Call(mem, bmp)
	fillRect(mem, rect{0, 0, ovW, ovH}, keyColor)
	pSetBkMode.Call(mem, 1)

	s := getSettings().Overlay
	k := float64(ovH) / 1080 * s.Scale
	if k <= 0 {
		k = 1
	}
	sc := func(v float64) int32 { return int32(v*k + 0.5) }
	fs := func(v float64, bold bool) uintptr { return font(int(v*k+0.5), bold) }
	// palette in stile Blitz
	panel := rgb("#1B1D24")
	head := rgb("#22252E")
	border := rgb("#2C2F3A")
	label := rgb("#E6E8EE")
	muted := rgb("#8C93A3")
	interactive := editStyle
	var hits []hitRect
	var icons []iconHit

	for _, w := range ovModel.Widgets {
		if w.Pill {
			ps := s.PillScale
			if ps <= 0 {
				ps = 0.8
			}
			u := float64(ovH) / 1080
			x := ovW - int32(w.X*u)
			y := ovH - int32(w.Y*u)
			pw, ph := sc(40*ps), sc(16*ps)
			r := rect{x - pw/2, y - ph/2, x + pw/2, y + ph/2}
			if len(w.Lines) > 0 {
				if w.Text {
					outlinedText(mem, w.Lines[0].Text, r, rgb(w.Lines[0].Color), fs(12*ps, true))
				} else {
					roundRect(mem, r, ph, panel, rgb(w.Accent))
					drawText(mem, w.Lines[0].Text, r, rgb(w.Lines[0].Color), fs(10*ps, true), dtCenter)
				}
			}
			continue
		}
		x := int32(w.X * float64(ovW))
		y := int32(w.Y * float64(ovH))
		pad := sc(8)

		// versione riassunta: una riga sola
		if len(w.Segs) > 0 {
			gap := sc(5)
			total := pad * 2
			fonts := make([]uintptr, len(w.Segs))
			widths := make([]int32, len(w.Segs))
			for i, sg := range w.Segs {
				fonts[i] = fs(11.5, sg.Bold)
				widths[i] = textWidth(mem, sg.Text, fonts[i])
				total += widths[i] + gap
			}
			h := sc(24)
			if w.Anchor == "center" {
				x -= total / 2
			}
			x, y = clampXY(x, y, total, h)
			roundRect(mem, rect{x, y, x + total, y + h}, sc(6), panel, border)
			cx := x + pad
			for i, sg := range w.Segs {
				drawText(mem, sg.Text, rect{cx, y, cx + widths[i] + 2, y + h}, rgb(sg.Color), fonts[i], dtLeft)
				cx += widths[i] + gap
			}
			hits = append(hits, hitRect{w.Key, rect{x, y, x + total, y + h}, w.Anchor == "center"})
			continue
		}

		width := sc(float64(w.Width))
		headH := int32(0)
		if w.Title != "" {
			headH = sc(26)
		}
		rowH := sc(18)
		h := headH + pad*2
		for _, l := range w.Lines {
			if l.Size >= 16 {
				h += sc(26)
			} else {
				h += rowH
			}
		}
		if w.Anchor == "center" {
			x -= width / 2
		}
		x, y = clampXY(x, y, width, h)
		roundRect(mem, rect{x, y, x + width, y + h}, sc(8), panel, border)
		left := x + sc(10)
		right := x + width - sc(10)
		cy := y
		if w.Title != "" {
			roundRect(mem, rect{x + 1, y + 1, x + width - 1, y + headH}, sc(8), head, head)
			fillRect(mem, rect{x + 1, y + headH - sc(6), x + width - 1, y + headH}, head)
			pSetTextCharacterExtra.Call(mem, uintptr(sc(1.6)))
			drawText(mem, w.Title, rect{left, y, right, y + headH}, label, fs(10.5, true), dtLeft)
			pSetTextCharacterExtra.Call(mem, 0)
			if w.Icons && w.Key != "" {
				is := sc(16)
				ix := right - is
				sym := symFont(int(11*k + 0.5))
				iy := y + (headH-is)/2
				lockTxt, lockCol := "🔓", muted
				if w.Locked {
					lockTxt, lockCol = "🔒", rgb("#38D6FF")
				}
				for _, c := range []struct {
					a, t string
					col  uintptr
				}{{"close", "✕", muted}, {"gear", "⚙", muted}, {"lock", lockTxt, lockCol}} {
					r := rect{ix, iy, ix + is, iy + is}
					drawText(mem, c.t, r, c.col, sym, dtCenter)
					icons = append(icons, iconHit{w.Key, c.a, rect{r.l - 2, r.t - 2, r.r + 2, r.b + 2}})
					ix -= is + sc(4)
				}
			}
			cy += headH
		}
		cy += pad
		for _, l := range w.Lines {
			lh := rowH
			size := 11.5
			if l.Size >= 16 {
				lh, size = sc(26), float64(l.Size)
			}
			r := rect{left, cy, right, cy + lh}
			switch {
			case l.Extra != "":
				ef := fs(size, false)
				ex := " / " + l.Extra
				ew := textWidth(mem, ex, ef)
				drawText(mem, ex, r, muted, ef, dtRight)
				drawText(mem, l.Right, rect{left, cy, right - ew, cy + lh}, rgb(l.Color), fs(size, true), dtRight)
				drawText(mem, l.Text, rect{left, cy, right - sc(60), cy + lh}, label, fs(size, true), dtLeft|dtEndEllipsis)
			case l.Right != "":
				drawText(mem, l.Right, r, rgb(l.Color), fs(size, true), dtRight)
				drawText(mem, l.Text, rect{left, cy, right - sc(56), cy + lh}, label, fs(size, true), dtLeft|dtEndEllipsis)
			default:
				drawText(mem, l.Text, r, rgb(l.Color), fs(size, l.Bold), dtLeft|dtEndEllipsis)
			}
			cy += lh
		}
		hits = append(hits, hitRect{w.Key, rect{x, y, x + width, y + h}, w.Anchor == "center"})
		if interactive && w.Key != "" {
			if s.EditMode {
				dashRect(mem, rect{x, y, x + width, y + h}, rgb("#38D6FF"))
			} else if !w.Locked {
				dashRect(mem, rect{x, y, x + width, y + h}, rgb("#5C6378"))
			}
		}
	}

	// riquadro della minimappa: trascinalo e ridimensionalo dall'angolo in alto a sinistra
	if interactive && s.Minimap {
		u := float64(ovH) / 1080
		size := int32(float64(s.MinimapSize) * u)
		r := rect{ovW - int32(float64(s.MinimapRight)*u) - size, ovH - int32(float64(s.MinimapBottom)*u) - size, 0, 0}
		r.r, r.b = r.l+size, r.t+size
		dashRect(mem, r, rgb("#F5B947"))
		hd := sc(12)
		fillRect(mem, rect{r.l - hd/2, r.t - hd/2, r.l + hd/2, r.t + hd/2}, rgb("#F5B947"))
		drawText(mem, "MINIMAPPA · trascina / ridimensiona dall'angolo", rect{r.l, r.t - sc(22), r.r, r.t - sc(4)}, rgb("#F5B947"), fs(10, true), dtLeft)
		hits = append(hits, hitRect{"minimap", r, false})
		icons = append(icons, iconHit{"minimap", "resize", rect{r.l - hd, r.t - hd, r.l + hd, r.t + hd}})
	}

	if interactive && ovPanel != "" {
		for _, hr := range hits {
			if hr.key == ovPanel {
				icons = append(icons, drawPanel(mem, hr.r, s, k)...)
				break
			}
		}
	} else if !interactive {
		ovPanel = ""
	}

	if s.EditMode {
		msg := "MODALITÀ MODIFICA  ·  trascina gli elementi  ·  in partita basta tenere premuto TAB"
		f := fs(14, true)
		tw := textWidth(mem, msg, f) + sc(40)
		r := rect{ovW/2 - tw/2, sc(14), ovW/2 + tw/2, sc(14) + sc(36)}
		roundRect(mem, r, sc(10), rgb("#22252E"), rgb("#38D6FF"))
		drawText(mem, msg, r, rgb("#FFFFFF"), f, dtCenter)
	}
	ovHits, ovIcons = hits, icons
	pBitBlt.Call(hdc, 0, 0, uintptr(ovW), uintptr(ovH), mem, 0, 0, srcCopy)
	pSelectObject.Call(mem, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
}

func clampXY(x, y, w, h int32) (int32, int32) {
	if x+w > ovW {
		x = ovW - w - 4
	}
	if y+h > ovH {
		y = ovH - h - 4
	}
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	return x, y
}

// sposta un elemento: aggiorna la posizione nelle impostazioni (frazione dello schermo)
func setWidgetPos(key string, fx, fy float64) {
	clamp := func(v float64) float64 {
		if v < 0 {
			return 0
		}
		if v > 1 {
			return 1
		}
		return v
	}
	fx, fy = clamp(fx), clamp(fy)
	settingsMu.Lock()
	o := &settings.Overlay
	var w *Widget
	switch key {
	case "cs":
		w = &o.CS
	case "objectives":
		w = &o.Objectives
	case "camps":
		w = &o.Camps
	case "buffs":
		w = &o.Buffs
	case "deaths":
		w = &o.Deaths
	case "itemDiff":
		w = &o.ItemDiff
	case "loading":
		w = &o.Loading
	case "notify":
		w = &o.Notify
	}
	if w != nil {
		w.X, w.Y = fx, fy
	}
	settingsMu.Unlock()
}

func widgetPtr(o *OverlaySettings, key string) *Widget {
	switch key {
	case "cs":
		return &o.CS
	case "objectives":
		return &o.Objectives
	case "camps":
		return &o.Camps
	case "buffs":
		return &o.Buffs
	case "deaths":
		return &o.Deaths
	case "itemDiff":
		return &o.ItemDiff
	case "loading":
		return &o.Loading
	case "notify":
		return &o.Notify
	}
	return nil
}

func widgetLocked(key string) bool {
	s := getSettings()
	if w := widgetPtr(&s.Overlay, key); w != nil {
		return w.Locked
	}
	return false
}

// pulsanti nell'intestazione: lucchetto, impostazioni (apre BetterStomp), chiudi
func widgetAction(key, action string) {
	if action == "gear" { // impostazioni direttamente in partita
		if ovPanel == key {
			ovPanel = ""
		} else {
			ovPanel = key
		}
		return
	}
	if action == "app" {
		ovPanel = ""
		go openAppWindow(appURL + "#/overlay")
		return
	}
	if strings.HasPrefix(action, "opt:") {
		panelOption(strings.TrimPrefix(action, "opt:"))
		return
	}
	s := getSettings()
	w := widgetPtr(&s.Overlay, key)
	if w == nil {
		return
	}
	switch action {
	case "lock":
		w.Locked = !w.Locked
	case "close":
		w.On = false
	}
	saveSettings(s)
}

func lparamXY(l uintptr) (int32, int32) {
	return int32(int16(l & 0xFFFF)), int32(int16((l >> 16) & 0xFFFF))
}

func screenSize() (int32, int32) {
	w, _, _ := pGetSystemMetrics.Call(0)
	h, _, _ := pGetSystemMetrics.Call(1)
	return int32(w), int32(h)
}

func overlayTick() {
	down, _, _ := pGetAsyncKeyState.Call(0x09) // TAB
	live.mu.Lock()
	live.tabHeld = down&0x8000 != 0
	key := live.gameKey
	live.mu.Unlock()
	if key != ovGameKey {
		// nuova partita: l'overlay torna sempre visibile
		ovGameKey = key
		ovHidden = false
	}
	if w, h := screenSize(); w != ovW || h != ovH {
		ovW, ovH = w, h
		pSetWindowPos.Call(ovHwnd, ^uintptr(0), 0, 0, uintptr(w), uintptr(h), swpNoActivate)
	}
	s := getSettings().Overlay
	d, _ := live.snapshot()
	setHotkeys(s.Enabled && (d != nil || getClientState().Phase == "InProgress"))
	live.mu.RLock()
	tab := live.tabHeld
	live.mu.RUnlock()
	// tenendo premuto TAB in partita l'overlay diventa cliccabile: trascini, blocchi, chiudi
	setEditStyle(s.EditMode || (tab && d != nil) || dragKey != "")
	ovModel = buildOverlayModel(s.EditMode)
	show := (s.Enabled && !ovHidden && ovModel.Active && len(ovModel.Widgets) > 0) || s.EditMode
	if show {
		if !ovShown {
			pShowWindow.Call(ovHwnd, swShowNA)
			ovShown = true
		}
		// resta sopra il gioco anche dopo alt-tab
		pSetWindowPos.Call(ovHwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate)
		pInvalidateRect.Call(ovHwnd, 0, 0)
	} else if ovShown {
		pShowWindow.Call(ovHwnd, swHide)
		ovShown = false
	}
}

var editStyle bool

// in modalità modifica la finestra smette di essere "trasparente ai clic"
func setEditStyle(on bool) {
	if on == editStyle {
		return
	}
	editStyle = on
	gwlExStyle := ^uintptr(19) // -20
	st, _, _ := pGetWindowLongPtr.Call(ovHwnd, gwlExStyle)
	if on {
		st &^= wsExTransparent
	} else {
		st |= wsExTransparent
		dragKey = ""
	}
	pSetWindowLongPtr.Call(ovHwnd, gwlExStyle, st)
	pSetWindowPos.Call(ovHwnd, ^uintptr(0), 0, 0, 0, 0, swpNoMove|swpNoSize|swpNoActivate|0x0020) // SWP_FRAMECHANGED
}

func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
	case 0x0020: // WM_SETCURSOR
		if editStyle {
			c, _, _ := pLoadCursor.Call(0, 32646) // IDC_SIZEALL
			pSetCursor.Call(c)
			return 1
		}
	case 0x0201: // WM_LBUTTONDOWN
		if editStyle {
			mx, my := lparamXY(lparam)
			for _, ic := range ovIcons {
				if mx >= ic.r.l && mx <= ic.r.r && my >= ic.r.t && my <= ic.r.b {
					if ic.action == "resize" {
						dragKey = "minimap-resize"
						pSetCapture.Call(hwnd)
						return 0
					}
					widgetAction(ic.key, ic.action)
					overlayTick()
					return 0
				}
			}
			for i := len(ovHits) - 1; i >= 0; i-- {
				h := ovHits[i]
				if h.key != "" && !widgetLocked(h.key) && mx >= h.r.l && mx <= h.r.r && my >= h.r.t && my <= h.r.b {
					dragKey, dragOffX, dragOffY, dragW, dragC = h.key, mx-h.r.l, my-h.r.t, h.r.r-h.r.l, h.center
					pSetCapture.Call(hwnd)
					break
				}
			}
		}
		return 0
	case 0x0200: // WM_MOUSEMOVE
		if dragKey == "minimap" || dragKey == "minimap-resize" {
			mx, my := lparamXY(lparam)
			u := float64(ovH) / 1080
			settingsMu.Lock()
			o := &settings.Overlay
			size := float64(o.MinimapSize) * u
			if dragKey == "minimap" {
				l, t := float64(mx-dragOffX), float64(my-dragOffY)
				o.MinimapRight = int(math.Max(0, (float64(ovW)-l-size)/u))
				o.MinimapBottom = int(math.Max(0, (float64(ovH)-t-size)/u))
			} else {
				right := float64(ovW) - float64(o.MinimapRight)*u
				bottom := float64(ovH) - float64(o.MinimapBottom)*u
				ns := math.Max(right-float64(mx), bottom-float64(my))
				o.MinimapSize = int(math.Min(700, math.Max(120, ns/u)))
			}
			settingsMu.Unlock()
			ovModel = buildOverlayModel(true)
			pInvalidateRect.Call(hwnd, 0, 0)
			return 0
		}
		if dragKey != "" {
			mx, my := lparamXY(lparam)
			l, t := mx-dragOffX, my-dragOffY
			fx := float64(l) / float64(ovW)
			if dragC {
				fx = float64(l+dragW/2) / float64(ovW)
			}
			setWidgetPos(dragKey, fx, float64(t)/float64(ovH))
			ovModel = buildOverlayModel(true)
			pInvalidateRect.Call(hwnd, 0, 0)
		}
		return 0
	case 0x0202: // WM_LBUTTONUP
		if dragKey != "" {
			dragKey = ""
			pReleaseCapture.Call()
			saveSettings(getSettings())
		}
		return 0
	case wmEraseBkgnd:
		return 1
	case wmPaint:
		var ps paintStruct
		hdc, _, _ := pBeginPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		paintOverlay(hdc)
		pEndPaint.Call(hwnd, uintptr(unsafe.Pointer(&ps)))
		return 0
	case wmTimer:
		overlayTick()
		return 0
	case wmHotkey:
		id := int(wparam)
		switch {
		case id >= 1 && id <= 6:
			live.markCamp("ally", id)
		case id >= 11 && id <= 16:
			live.markCamp("enemy", id-10)
		case id == 20:
			live.mu.Lock()
			live.showLoad = !live.showLoad
			live.mu.Unlock()
		case id == 21:
			ovHidden = !ovHidden
		case id == 23:
			st := getSettings()
			st.Overlay.EditMode = !st.Overlay.EditMode
			saveSettings(st)
		case id == 22:
			st := getSettings()
			st.Overlay.Compact = !st.Overlay.Compact
			saveSettings(st)
		}
		overlayTick()
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

func startOverlay() {
	runtime.LockOSThread()
	hinst, _, _ := pGetModuleHandle.Call(0)
	startTray(hinst)
	cls, _ := syscall.UTF16PtrFromString("BetterStompOverlay")
	title, _ := syscall.UTF16PtrFromString("BetterStomp Overlay")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: hinst, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	ovW, ovH = screenSize()
	ovHwnd, _, _ = pCreateWindowEx.Call(
		wsExLayered|wsExTransparent|wsExTopmost|wsExToolWindow|wsExNoActivate,
		uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(title)), wsPopup,
		0, 0, uintptr(ovW), uintptr(ovH), 0, 0, hinst, 0)
	if ovHwnd == 0 {
		// senza overlay resta comunque attivo il ciclo per l'icona nell'area di notifica
		var m msgT
		for {
			r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
			if int32(r) <= 0 {
				return
			}
			pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
			pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
		}
	}
	pSetLayeredAttr.Call(ovHwnd, keyColor, 0, lwaColorKey)

	pSetTimer.Call(ovHwnd, 1, 250, 0)
	var m msgT
	for {
		r, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// ---------- impostazioni dentro l'overlay (rotella ⚙) ----------

var ovPanel string

func panelOption(opt string) {
	s := getSettings()
	o := &s.Overlay
	tierIdx := 0
	for i, t := range Tiers {
		if t == o.CSGoalTier {
			tierIdx = i
		}
	}
	switch opt {
	case "elo":
		if o.CSGoal == "off" {
			o.CSGoal = "elo"
		} else {
			o.CSGoal = "off"
		}
	case "tier-":
		if tierIdx > 0 {
			o.CSGoalTier = Tiers[tierIdx-1]
		}
	case "tier+":
		if tierIdx < len(Tiers)-1 {
			o.CSGoalTier = Tiers[tierIdx+1]
		}
	case "goal-":
		o.CSGoalValue = math.Max(0, o.CSGoalValue-0.5)
	case "goal+":
		if o.CSGoalValue == 0 {
			o.CSGoalValue = 6
		} else {
			o.CSGoalValue = math.Min(12, o.CSGoalValue+0.5)
		}
	case "compact":
		o.Compact = !o.Compact
	case "kp":
		o.ShowKP = !o.ShowKP
	case "hide":
		if w := widgetPtr(o, ovPanel); w != nil {
			w.On = false
		}
		ovPanel = ""
	}
	saveSettings(s)
}

func drawPanel(hdc uintptr, wr rect, o OverlaySettings, k float64) []iconHit {
	sc := func(v float64) int32 { return int32(v*k + 0.5) }
	f := font(int(11.5*k+0.5), true)
	fn := font(int(11.5*k+0.5), false)
	type row struct {
		label string
		btns  [][2]string // testo, azione
		value string
	}
	onOff := func(b bool) string {
		if b {
			return "ON"
		}
		return "OFF"
	}
	var rows []row
	if ovPanel == "cs" {
		goal := "off"
		if o.CSGoalValue > 0 {
			goal = strconv.FormatFloat(o.CSGoalValue, 'f', 1, 64) + "/min"
		}
		rows = []row{
			{"Confronto con elo", [][2]string{{onOff(o.CSGoal != "off"), "opt:elo"}}, ""},
			{"Elo di riferimento", [][2]string{{"◀", "opt:tier-"}, {"▶", "opt:tier+"}}, tierIT[o.CSGoalTier]},
			{"Obiettivo CS fisso", [][2]string{{"−", "opt:goal-"}, {"+", "opt:goal+"}}, goal},
			{"Versione riassunta", [][2]string{{onOff(o.Compact), "opt:compact"}}, ""},
			{"Partecipazione uccisioni", [][2]string{{onOff(o.ShowKP), "opt:kp"}}, ""},
		}
	}
	rows = append(rows, row{"Nascondi questo riquadro", [][2]string{{"✕", "opt:hide"}}, ""}, row{"Tutte le impostazioni", [][2]string{{"Apri app", "app"}}, ""})
	w := sc(290)
	rowH := sc(30)
	h := sc(36) + rowH*int32(len(rows)) + sc(8)
	x := wr.r + sc(8)
	if x+w > ovW {
		x = wr.l - sc(8) - w
	}
	y := wr.t
	if y+h > ovH {
		y = ovH - h
	}
	roundRect(hdc, rect{x, y, x + w, y + h}, sc(8), rgb("#1B1D24"), rgb("#38D6FF"))
	pSetTextCharacterExtra.Call(hdc, uintptr(sc(1.4)))
	drawText(hdc, "IMPOSTAZIONI", rect{x + sc(12), y, x + w, y + sc(34)}, rgb("#E6E8EE"), font(int(10.5*k+0.5), true), dtLeft)
	pSetTextCharacterExtra.Call(hdc, 0)
	var out []iconHit
	cy := y + sc(34)
	for _, r := range rows {
		drawText(hdc, r.label, rect{x + sc(12), cy, x + w - sc(110), cy + rowH}, rgb("#8C93A3"), fn, dtLeft|dtEndEllipsis)
		bx := x + w - sc(12)
		for i := len(r.btns) - 1; i >= 0; i-- {
			b := r.btns[i]
			bw := textWidth(hdc, b[0], f) + sc(16)
			if bw < sc(26) {
				bw = sc(26)
			}
			br := rect{bx - bw, cy + sc(4), bx, cy + rowH - sc(4)}
			col := rgb("#2C2F3A")
			if b[0] == "ON" {
				col = rgb("#1F6F5A")
			}
			roundRect(hdc, br, sc(6), col, rgb("#3A3E4C"))
			drawText(hdc, b[0], br, rgb("#FFFFFF"), f, dtCenter)
			out = append(out, iconHit{ovPanel, b[1], br})
			bx -= bw + sc(6)
			if i == 1 && r.value != "" { // valore tra i due pulsanti
				vw := textWidth(hdc, r.value, f) + sc(12)
				drawText(hdc, r.value, rect{bx - vw, cy, bx, cy + rowH}, rgb("#38D6FF"), f, dtCenter)
				bx -= vw + sc(4)
			}
		}
		cy += rowH
	}
	return out
}
