//go:build windows

package main

import (
	"io"
	"os"
	"syscall"
	"unsafe"
)

const enableVirtualTerminalProcessing = 0x0004

// habilitarColorTerminal activa las secuencias ANSI en la consola de Windows (cmd.exe y PowerShell antiguos no las
// interpretan hasta que se pide ENABLE_VIRTUAL_TERMINAL_PROCESSING). Devuelve false si w no es una consola.
func habilitarColorTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	if !ok {
		return false
	}
	k32 := syscall.NewLazyDLL("kernel32.dll")
	getMode, setMode := k32.NewProc("GetConsoleMode"), k32.NewProc("SetConsoleMode")
	var mode uint32
	if r, _, _ := getMode.Call(f.Fd(), uintptr(unsafe.Pointer(&mode))); r == 0 {
		return false
	}
	if mode&enableVirtualTerminalProcessing != 0 {
		return true
	}
	r, _, _ := setMode.Call(f.Fd(), uintptr(mode|enableVirtualTerminalProcessing))
	return r != 0
}
