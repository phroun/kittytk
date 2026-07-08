// Command kittytk-tui is the TERMINAL display service: a blank KittyTK
// desktop rendered as text in your terminal, serving the protocol socket.
// It is the terminal twin of kittytk-sdl - the two hosts are
// interchangeable. Applications dial the socket and attach to whichever
// host is running, without knowing (or caring) which renderer is on the
// other end; the display protocol is identical either way:
//
//	terminal 1:  go run ./examples/kittytk-tui             (terminal)
//	         or:  go run -tags sdl ./examples/kittytk-sdl   (graphical)
//	terminal 2:  go run ./examples/demoapp   (or ./examples/remoteapp)
package main

import (
	"os"

	"github.com/phroun/kittytk/app"
	"github.com/phroun/kittytk/backend"
	"github.com/phroun/kittytk/client"
	"github.com/phroun/kittytk/display"
	"github.com/phroun/kittytk/trinkets"
)

func main() {
	tuiBackend := backend.NewTUIBackend(backend.DefaultTUIOptions())

	desktop := trinkets.NewDesktop()
	desktop.SetBackend(tuiBackend) // seeds root metrics from the cell grid

	// The desktop's own (windowless) application owns the base menu bar
	// until a client dials in.
	application := app.New(nil)
	application.SetName("KittyTK (TUI)")
	desktop.AddApplication(application)

	// Start the display service: applications appear as they connect. The
	// service only touches the desktop via Post, so it is agnostic to the
	// backend - the very same Serve call powers kittytk-sdl.
	desktop.SetOnStartup(func() {
		if _, err := display.Serve(desktop, client.DefaultSocketPath()); err != nil {
			if sb := desktop.StatusBar(); sb != nil {
				sb.SetText("display service unavailable: " + err.Error())
			}
		} else if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("display service on " + client.DefaultSocketPath() + " - run examples/demoapp to connect")
		}
	})

	os.Exit(desktop.Run())
}
