//go:build !sdl

// Command kittytk-sdl requires the sdl build tag (and libsdl2-dev). For a
// terminal desktop with no such requirement, use kittytk-tui instead:
//
//	go run -tags sdl ./examples/kittytk-sdl   (graphical)
//	go run ./examples/kittytk-tui             (terminal)
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "kittytk-sdl requires the sdl build tag: go run -tags sdl ./examples/kittytk-sdl")
	fmt.Fprintln(os.Stderr, "or run the terminal desktop instead: go run ./examples/kittytk-tui")
	os.Exit(1)
}
