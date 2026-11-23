//go:build !windows

package main

import "fmt"

func openAppWindow(url string)              { fmt.Println("BetterStomp:", url) }
func processCommandLine(name string) string { return "" }
func setDPIAware()                          {}
func startOverlay()                         {}

func startOCR()          {}
func ocrCS() (int, bool) { return 0, false }

func framelessLoop()          {}
func windowAction(a string)   {}
func isFrameless() bool       { return false }
func setInner(h, dpr float64) {}
