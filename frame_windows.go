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

func applyFrameless(h uintptr) {
	st, _, _ := pGetWindowLongPtr.Call(h, gwlStyle)
	if st&wsCaption == 0 {
		return
	}
	pSetWindowLongPtr.Call(h, gwlStyle, st&^wsCaption)
	pSetWindowPos.Call(h, 0, 0, 0, 0, 0, swpFrameChg)
	pref := uint32(2) // DWMWCP_ROUND: angoli arrotondati (Windows 11)
	pDwmSetWindowAttrib.Call(h, 33, uintptr(unsafe.Pointer(&pref)), 4)
	dark := uint32(1)
	pDwmSetWindowAttrib.Call(h, 20, uintptr(unsafe.Pointer(&dark)), 4)
}

// controlla di continuo: se la finestra viene riaperta o il browser rimette la barra, la toglie di nuovo
func framelessLoop() {
	for {
		if getSettings().NativeFrame {
			framelessOn.Store(false)
		} else if h := findAppWindow(); h != 0 {
			applyFrameless(h)
			framelessOn.Store(true)
		}
		time.Sleep(700 * time.Millisecond)
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
	case "close":
		pPostMessage.Call(h, 0x0010, 0, 0) // WM_CLOSE
	}
}

func isFrameless() bool { return framelessOn.Load() }
