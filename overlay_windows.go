//go:build windows

package main

import (
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

	pCreateCompatibleDC  = g32.NewProc("CreateCompatibleDC")
	pCreateCompatibleBmp = g32.NewProc("CreateCompatibleBitmap")
	pSelectObject        = g32.NewProc("SelectObject")
	pDeleteObject        = g32.NewProc("DeleteObject")
	pDeleteDC            = g32.NewProc("DeleteDC")
	pBitBlt              = g32.NewProc("BitBlt")
	pCreateSolidBrush    = g32.NewProc("CreateSolidBrush")
	pCreatePen           = g32.NewProc("CreatePen")
	pRoundRect           = g32.NewProc("RoundRect")
	pCreateFont          = g32.NewProc("CreateFontW")
	pSetTextColor        = g32.NewProc("SetTextColor")
	pSetBkMode           = g32.NewProc("SetBkMode")
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
	hdc       uintptr
	fErase    int32
	rcPaint   rect
	fRestore  int32
	fIncUp    int32
	reserved  [32]byte
}

// le scorciatoie sono attive solo durante la partita (così Ctrl+T ecc. restano liberi fuori dal gioco)
var hotkeysOn bool

func setHotkeys(on bool) {
	if on == hotkeysOn {
		return
	}
	hotkeysOn = on
	ids := []int{20, 21}
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
	sc := func(v float64) int32 { return int32(v * k) }
	panel := rgb("#11132A")
	border := rgb("#2B3060")

	for _, w := range ovModel.Widgets {
		if w.Pill {
			x := ovW - int32(w.X)
			y := ovH - int32(w.Y)
			pw, ph := sc(40), sc(17)
			r := rect{x - pw/2, y - ph/2, x + pw/2, y + ph/2}
			roundRect(mem, r, sc(10), panel, rgb(w.Accent))
			if len(w.Lines) > 0 {
				drawText(mem, w.Lines[0].Text, r, rgb(w.Lines[0].Color), font(int(10*k)+1, true), dtCenter)
			}
			continue
		}
		width := sc(float64(w.Width))
		pad := sc(10)
		titleH := int32(0)
		if w.Title != "" {
			titleH = sc(18)
		}
		h := pad*2 + titleH
		for _, l := range w.Lines {
			if l.Size >= 16 {
				h += sc(28)
			} else {
				h += sc(21)
			}
		}
		x := int32(w.X * float64(ovW))
		y := int32(w.Y * float64(ovH))
		if w.Anchor == "center" {
			x -= width / 2
		}
		if x+width > ovW {
			x = ovW - width - 4
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
		roundRect(mem, rect{x, y, x + width, y + h}, sc(14), panel, border)
		fillRect(mem, rect{x + sc(4), y + sc(10), x + sc(7), y + h - sc(10)}, rgb(w.Accent))
		cy := y + pad
		left := x + sc(16)
		right := x + width - sc(12)
		if w.Title != "" {
			drawText(mem, w.Title, rect{left, cy, right, cy + titleH}, rgb(w.Accent), font(int(11*k)+1, true), dtLeft)
			cy += titleH
		}
		for _, l := range w.Lines {
			lh := sc(21)
			size := 13
			if l.Size >= 16 {
				lh, size = sc(28), l.Size
			}
			f := font(int(float64(size)*k)+1, l.Bold)
			col := rgb(l.Color)
			if l.Right != "" {
				drawText(mem, l.Right, rect{left, cy, right, cy + lh}, col, f, dtRight)
				drawText(mem, l.Text, rect{left, cy, right - sc(70), cy + lh}, col, font(int(float64(size)*k)+1, false), dtLeft|dtEndEllipsis)
			} else {
				drawText(mem, l.Text, rect{left, cy, right, cy + lh}, col, f, dtLeft|dtEndEllipsis)
			}
			cy += lh
		}
	}
	pBitBlt.Call(hdc, 0, 0, uintptr(ovW), uintptr(ovH), mem, 0, 0, srcCopy)
	pSelectObject.Call(mem, old)
	pDeleteObject.Call(bmp)
	pDeleteDC.Call(mem)
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
	ovModel = buildOverlayModel(false)
	show := s.Enabled && !ovHidden && ovModel.Active && len(ovModel.Widgets) > 0
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

func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	switch msg {
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
		return
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
