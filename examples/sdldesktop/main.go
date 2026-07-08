//go:build sdl

// Command sdldesktop is the GRAPHICAL display service (D23): a blank
// tuitk desktop rendered as pixels in an SDL window, serving the
// protocol socket. Applications are separate processes that dial in:
//
//	terminal 1:  go run -tags sdl ./examples/sdldesktop
//	terminal 2:  go run ./examples/demoapp   (or ./examples/remoteapp)
package main

import (
	"fmt"
	"os"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/client"
	"github.com/phroun/tuitk/display"
	sdlplat "github.com/phroun/tuitk/sdl"
	"github.com/phroun/tuitk/widgets"
)

func main() {
	plat := sdlplat.New("tuitk", 1024, 768)
	plat.SetScale(2) // 2x font/cell size for now (per owner request)
	backend, err := plat.EnsureBackend()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	desktop := widgets.NewDesktop()
	desktop.SetBackend(backend) // seeds root metrics from the raster font

	// The desktop's own (windowless) application owns the base menu bar
	// until a client dials in.
	application := app.New(nil)
	application.SetName("SDL Desktop")
	desktop.AddApplication(application)

	// Start the display service: applications appear as they connect.
	desktop.SetOnStartup(func() {
		if _, err := display.Serve(desktop, client.DefaultSocketPath()); err != nil {
			if sb := desktop.StatusBar(); sb != nil {
				sb.SetText("display service unavailable: " + err.Error())
			}
		} else if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("display service on " + client.DefaultSocketPath() + " - run examples/demoapp to connect")
		}
	})

	os.Exit(desktop.RunOn(plat))
}
