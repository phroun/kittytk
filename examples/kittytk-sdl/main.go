//go:build sdl

// Command kittytk-sdl is the GRAPHICAL display service (D23): a blank
// KittyTK desktop rendered as pixels in an SDL window, serving the
// protocol socket. It is one of two interchangeable hosts - its terminal
// twin is kittytk-tui. Applications dial the socket and attach to
// whichever host is running, without knowing (or caring) which renderer
// is on the other end:
//
//	terminal 1:  go run -tags sdl ./examples/kittytk-sdl   (graphical)
//	         or:  go run ./examples/kittytk-tui             (terminal)
//	terminal 2:  go run ./examples/demoapp   (or ./examples/remoteapp)
package main

import (
	"fmt"
	"os"

	"github.com/phroun/kittytk/app"
	"github.com/phroun/kittytk/client"
	"github.com/phroun/kittytk/display"
	sdlplat "github.com/phroun/kittytk/sdl"
	"github.com/phroun/kittytk/trinkets"
)

func main() {
	plat := sdlplat.New("KittyTK", 1024, 768)
	plat.SetScale(2) // 2x font/cell size for now (per owner request)
	backend, err := plat.EnsureBackend()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	desktop := trinkets.NewDesktop()
	desktop.SetBackend(backend) // seeds root metrics from the raster font

	// The desktop's own (windowless) application owns the base menu bar
	// until a client dials in.
	application := app.New(nil)
	application.SetName("KittyTK (SDL)")
	desktop.AddApplication(application)

	// Start the display service: applications appear as they connect.
	desktop.SetOnStartup(func() {
		endpoint := client.DefaultEndpoint()
		srv, err := display.ServeConfig(desktop, display.DefaultConfig(desktop, endpoint))
		if sb := desktop.StatusBar(); sb != nil {
			switch {
			case err != nil:
				sb.SetText("display service unavailable: " + err.Error())
			case srv.TLSFingerprint != "":
				sb.SetText("display service on " + endpoint + " (" + srv.TLSFingerprint + ")")
			default:
				sb.SetText("display service on " + endpoint + " - run examples/demoapp to connect")
			}
		}
	})

	os.Exit(desktop.RunOn(plat))
}
