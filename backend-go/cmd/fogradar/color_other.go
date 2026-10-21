//go:build !windows

package main

import "io"

// habilitarColorTerminal: en Linux y macOS las terminales ya interpretan ANSI.
func habilitarColorTerminal(io.Writer) bool { return true }
