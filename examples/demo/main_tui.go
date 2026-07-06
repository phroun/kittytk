//go:build !sdl

package main

import (
	"github.com/phroun/tuitk/backend"
	"github.com/phroun/tuitk/widgets"
)

// The text-mode demo: the classic TUI desktop in your terminal.
//
//	go run ./examples/demo
func main() {
	opts := backend.DefaultTUIOptions()
	tuiBackend := backend.NewTUIBackend(opts)

	desktop := widgets.NewDesktop()
	desktop.SetBackend(tuiBackend)

	buildDemo(desktop)
	desktop.Run()
}
