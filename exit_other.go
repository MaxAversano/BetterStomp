//go:build !windows

package main

import "os"

func appExit() { engine.save(); os.Exit(0) }

func relaunchLater(exe string) {}
