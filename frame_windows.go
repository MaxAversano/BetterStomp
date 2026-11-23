//go:build windows

package main

import (
	"strings"
	"sync/atomic"
	"syscall"
	"time"
	"unsafe"
)

// finestra dell'app senza la barra di Windows: la barra la disegna l'interfaccia
var (
	pEnumWindows        = u32.NewProc("EnumWindows")
	pGetWindowText      = u32.NewProc("GetWindowTextW")
	pGetClassName       = u32.NewProc("GetClassNameW")
	pIsWindowVisible    = u32.NewProc("IsWindowVisible")
	pIsZoomed           = u32.NewProc("IsZoomed")
	pDwmSetWindowAttrib = syscall.NewLazyDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute")
	appHwnd             atomic.Uintptr
	framelessOn         atomic.Bool
)

const (
	gwlStyle    = ^uintptr(15) // -16
	wsCaption   = 0x00C00000
	wsSysMenu   = 0x00080000
	swpFrameChg = 0x0020 | 0x0001 | 0x0002 | 0x0004 // FRAMECHANGED|NOSIZE|NOMOVE|NOZORDER
)

var enumCB = syscall.NewCallback(func(h, _ uintptr) uintptr {
	if v, _, _ := pIsWindowVisible.Call(h); v == 0 {
		return 1
	}
	cls := make([]uint16, 64)
	pGetClassName.Call(h, uintptr(unsafe.Pointer(&cls[0])), 64)
	if !strings.HasPrefix(syscall.UTF16ToString(cls), "Chrome_WidgetWin") {
		return 1
	}
	t := make([]uint16, 256)
	pGetWindowText.Call(h, uintptr(unsafe.Pointer(&t[0])), 256)
	if syscall.UTF16ToString(t) == "BetterStomp" {
		appHwnd.Store(h)
		return 0
	}
	return 1
})

func findAppWindow() uintptr {
	appHwnd.Store(0)
	pEnumWindows.Call(enumCB, 0)
	return appHwnd.Load()
}

var (
	pEnumChildWindows = u32.NewProc("EnumChildWindows")
	pGetWindowRect    = u32.NewProc("GetWindowRect")
	pSetWindowRgn     = u32.NewProc("SetWindowRgn")
	pGetDpiForWindow  = u32.NewProc("GetDpiForWindow")
	pCreateRoundRgn   = g32.NewProc("CreateRoundRectRgn")
	pGetWindowRgnBox  = u32.NewProc("GetWindowRgnBox")
	renderHwnd        atomic.Uintptr
	lastRgn           [5]int32
)

var childCB = syscall.NewCallback(func(h, _ uintptr) uintptr {
	cls := make([]uint16, 64)
	pGetClassName.Call(h, uintptr(unsafe.Pointer(&cls[0])), 64)
	if syscall.UTF16ToString(cls) == "Chrome_RenderWidgetHostHWND" {
		if v, _, _ := pIsWindowVisible.Call(h); v != 0 {
			renderHwnd.Store(h)
			return 0
		}
	}
	return 1
})

// ritaglia la finestra sulla sola pagina (senza la barra del browser) con angoli arrotondati
func applyFrameless(h uintptr) bool {
	renderHwnd.Store(0)
	pEnumChildWindows.Call(h, childCB, 0)
	ch := renderHwnd.Load()
	if ch == 0 {
		return false
	}
	var wr, cr rect
	pGetWindowRect.Call(h, uintptr(unsafe.Pointer(&wr)))
	pGetWindowRect.Call(ch, uintptr(unsafe.Pointer(&cr)))
	if cr.r-cr.l < 200 || cr.b-cr.t < 150 {
		return false
	}
	dpi, _, _ := pGetDpiForWindow.Call(h)
	if dpi == 0 {
		dpi = 96
	}
	rad := int32(16 * int(dpi) / 96)
	zoomed, _, _ := pIsZoomed.Call(h)
	if zoomed != 0 {
		rad = 0
	}
	cur := [5]int32{cr.l - wr.l, cr.t - wr.t, cr.r - wr.l, cr.b - wr.t, rad}
	if cur == lastRgn {
		var bx rect
		t, _, _ := pGetWindowRgnBox.Call(h, uintptr(unsafe.Pointer(&bx)))
		if t != 0 && bx.l == cur[0] && bx.t == cur[1] { // il ritaglio è ancora quello giusto
			return true
		}
	}
	rg, _, _ := pCreateRoundRgn.Call(uintptr(cur[0]), uintptr(cur[1]), uintptr(cur[2]+1), uintptr(cur[3]+1), uintptr(rad*2), uintptr(rad*2))
	if rg == 0 {
		return false
	}
	pSetWindowRgn.Call(h, rg, 1) // la regione passa al sistema
	lastRgn = cur
	return true
}

func removeFrameless(h uintptr) {
	if lastRgn != ([5]int32{}) {
		pSetWindowRgn.Call(h, 0, 1)
		lastRgn = [5]int32{}
	}
}

// controlla di continuo: finestra riaperta, ridimensionata o ingrandita → aggiorna il ritaglio
func framelessLoop() {
	var last uintptr
	for {
		h := findAppWindow()
		if h != last {
			lastRgn = [5]int32{}
			last = h
		}
		if h != 0 {
			if getSettings().NativeFrame {
				removeFrameless(h)
				framelessOn.Store(false)
			} else {
				framelessOn.Store(applyFrameless(h))
			}
		} else {
			framelessOn.Store(false)
		}
		time.Sleep(150 * time.Millisecond)
	}
}

func windowAction(a string) {
	h := appHwnd.Load()
	if h == 0 {
		if h = findAppWindow(); h == 0 {
			return
		}
	}
	switch a {
	case "drag":
		go trackMouse(h, 0)
	case "min":
		pShowWindow.Call(h, 6)
	case "max":
		if z, _, _ := pIsZoomed.Call(h); z != 0 {
			pShowWindow.Call(h, 9)
		} else {
			pShowWindow.Call(h, 3)
		}
	case "size1", "size2", "size3", "size4", "size5", "size6", "size7", "size8":
		go trackMouse(h, int(a[4]-'0'))
	case "close":
		pPostMessage.Call(h, 0x0010, 0, 0) // WM_CLOSE
	}
}

func isFrameless() bool { return framelessOn.Load() }

var (
	pGetCursorPosF = u32.NewProc("GetCursorPos")
	pGetKeyStateF  = u32.NewProc("GetAsyncKeyState")
	dragging       atomic.Bool
)

type pt struct{ x, y int32 }

// sposta (edge=0) o ridimensiona la finestra seguendo il mouse finché il tasto sinistro è premuto
// (edge: 1 sx, 2 dx, 3 su, 4 su-sx, 5 su-dx, 6 giù, 7 giù-sx, 8 giù-dx)
func trackMouse(h uintptr, edge int) {
	if !dragging.CompareAndSwap(false, true) {
		return
	}
	defer dragging.Store(false)
	var c0 pt
	var r0 rect
	pGetCursorPosF.Call(uintptr(unsafe.Pointer(&c0)))
	if z, _, _ := pIsZoomed.Call(h); z != 0 {
		if edge != 0 {
			return
		}
		// finestra ingrandita: torna normale e resta sotto il mouse
		pShowWindow.Call(h, 9)
		time.Sleep(60 * time.Millisecond)
		pGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r0)))
		w := r0.r - r0.l
		nx := c0.x - w/2
		pSetWindowPos.Call(h, 0, uintptr(nx), uintptr(c0.y-20), 0, 0, 0x0001|0x0004|0x0010)
	}
	pGetWindowRect.Call(h, uintptr(unsafe.Pointer(&r0)))
	const minW, minH = 900, 560
	for {
		if s, _, _ := pGetKeyStateF.Call(0x01); s&0x8000 == 0 {
			return
		}
		var c pt
		pGetCursorPosF.Call(uintptr(unsafe.Pointer(&c)))
		dx, dy := c.x-c0.x, c.y-c0.y
		r := r0
		switch edge {
		case 0:
			r.l += dx
			r.r += dx
			r.t += dy
			r.b += dy
		default:
			if edge == 1 || edge == 4 || edge == 7 {
				r.l = min(r0.l+dx, r0.r-minW)
			}
			if edge == 2 || edge == 5 || edge == 8 {
				r.r = max(r0.r+dx, r0.l+minW)
			}
			if edge == 3 || edge == 4 || edge == 5 {
				r.t = min(r0.t+dy, r0.b-minH)
			}
			if edge == 6 || edge == 7 || edge == 8 {
				r.b = max(r0.b+dy, r0.t+minH)
			}
		}
		pSetWindowPos.Call(h, 0, uintptr(r.l), uintptr(r.t), uintptr(r.r-r.l), uintptr(r.b-r.t), 0x0004|0x0010) // NOZORDER|NOACTIVATE
		time.Sleep(8 * time.Millisecond)
	}
}
