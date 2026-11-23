//go:build !windows

package main

import "fmt"

func openAppWindow(url string)                { fmt.Println("BetterStomp:", url) }
func processCommandLine(name string) string { return "" }
func setDPIAware()                          {}
func startOverlay()                         {}
