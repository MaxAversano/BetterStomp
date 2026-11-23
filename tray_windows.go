//go:build windows

package main

import (
	"os"
	"syscall"
	"time"
	"unsafe"
)

// ---------- Icona nell'area di notifica (vicino all'orologio) con menu Apri / Esci ----------

var (
	pShellNotifyIcon = syscall.NewLazyDLL("shell32.dll").NewProc("Shell_NotifyIconW")
	pLoadImage       = u32.NewProc("LoadImageW")
	pCreatePopupMenu = u32.NewProc("CreatePopupMenu")
	pAppendMenu      = u32.NewProc("AppendMenuW")
	pTrackPopupMenu  = u32.NewProc("TrackPopupMenu")
	pDestroyMenu     = u32.NewProc("DestroyMenu")
	pGetCursorPos    = u32.NewProc("GetCursorPos")
	pSetForeground   = u32.NewProc("SetForegroundWindow")
	pPostMessage     = u32.NewProc("PostMessageW")
)

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

const (
	wmTray      = 0x8000 + 1 // WM_APP + 1
	nimAdd      = 0
	nimDelete   = 2
	nifMessage  = 1
	nifIcon     = 2
	nifTip      = 4
	menuOpen    = 1
	menuOverlay = 2
	menuQuit    = 3
)

var (
	trayHwnd uintptr
	trayData notifyIconData
)

func trayQuit() {
	pShellNotifyIcon.Call(nimDelete, uintptr(unsafe.Pointer(&trayData)))
	done := make(chan struct{})
	go func() { engine.save(); close(done) }()
	select { // salva le statistiche, ma non resta mai appeso in chiusura
	case <-done:
	case <-time.After(3 * time.Second):
	}
	os.Exit(0)
}

func trayMenu(hwnd uintptr) {
	m, _, _ := pCreatePopupMenu.Call()
	add := func(id int, text string) {
		t, _ := syscall.UTF16PtrFromString(text)
		pAppendMenu.Call(m, 0, uintptr(id), uintptr(unsafe.Pointer(t)))
	}
	add(menuOpen, "Apri BetterStomp")
	if ovHidden {
		add(menuOverlay, "Mostra overlay")
	} else {
		add(menuOverlay, "Nascondi overlay")
	}
	pAppendMenu.Call(m, 0x800, 0, 0) // separatore
	add(menuQuit, "Esci da BetterStomp")
	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForeground.Call(hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(m, 0x100|0x80|0x2, uintptr(pt.x), uintptr(pt.y), 0, hwnd, 0)
	pPostMessage.Call(hwnd, 0, 0, 0)
	pDestroyMenu.Call(m)
	switch cmd {
	case menuOpen:
		go openAppWindow(appURL)
	case menuOverlay:
		ovHidden = !ovHidden
		overlayTick()
	case menuQuit:
		trayQuit()
	}
}

func trayProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if msg == wmTray {
		switch lparam & 0xFFFF {
		case 0x0203, 0x0202: // doppio clic / clic sinistro
			go openAppWindow(appURL)
		case 0x0205: // clic destro
			trayMenu(hwnd)
		}
		return 0
	}
	r, _, _ := pDefWindowProc.Call(hwnd, msg, wparam, lparam)
	return r
}

// va chiamata dal thread dell'overlay (stesso ciclo di messaggi)
func startTray(hinst uintptr) {
	cls, _ := syscall.UTF16PtrFromString("BetterStompTray")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(trayProc), hInstance: hinst, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))
	trayHwnd, _, _ = pCreateWindowEx.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, 0, 0, hinst, 0)
	if trayHwnd == 0 {
		return
	}
	cx, _, _ := pGetSystemMetrics.Call(49) // SM_CXSMICON
	icon, _, _ := pLoadImage.Call(hinst, 1, 1, cx, cx, 0)
	trayData = notifyIconData{hWnd: trayHwnd, uID: 1, uFlags: nifMessage | nifIcon | nifTip, uCallbackMessage: wmTray, hIcon: icon}
	trayData.cbSize = uint32(unsafe.Sizeof(trayData))
	tip, _ := syscall.UTF16FromString("BetterStomp · clic destro per uscire")
	copy(trayData.szTip[:], tip)
	pShellNotifyIcon.Call(nimAdd, uintptr(unsafe.Pointer(&trayData)))
}
