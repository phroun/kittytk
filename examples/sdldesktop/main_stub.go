//go:build !sdl

// Command sdldesktop requires the sdl build tag (and libsdl2-dev):
//
//	go run -tags sdl ./examples/sdldesktop
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Fprintln(os.Stderr, "sdldesktop requires the sdl build tag: go run -tags sdl ./examples/sdldesktop")
	os.Exit(1)
}
