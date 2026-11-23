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
		pPostMessage.Call(h, 0x0112, 0xF012, 0) // WM_SYSCOMMAND, SC_MOVE|HTCAPTION
	case "min":
		pShowWindow.Call(h, 6)
	case "max":
		if z, _, _ := pIsZoomed.Call(h); z != 0 {
			pShowWindow.Call(h, 9)
		} else {
			pShowWindow.Call(h, 3)
		}
	case "size1", "size2", "size3", "size4", "size5", "size6", "size7", "size8":
		pPostMessage.Call(h, 0x0112, uintptr(0xF000+int(a[4]-'0')), 0) // SC_SIZE + bordo
	case "close":
		pPostMessage.Call(h, 0x0010, 0, 0) // WM_CLOSE
	}
}

func isFrameless() bool { return framelessOn.Load() }
