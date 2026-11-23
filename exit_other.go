//go:build !windows

package main

import "os"

func appExit() { engine.save(); os.Exit(0) }

func relaunchLater(exe string) {}

func notify(title, msg string) {}
func setAutostart(on bool)     {}
