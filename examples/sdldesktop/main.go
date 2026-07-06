//go:build sdl

// Command sdldesktop is the GRAPHICAL display service (D23): the
// same tuitk desktop, rendered as pixels in an SDL window, serving
// the same protocol socket. Apps cannot tell the difference:
//
//	terminal 1:  go run -tags sdl ./examples/sdldesktop
//	terminal 2:  go run ./examples/remoteapp
package main

import (
	"fmt"
	"os"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/client"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/display"
	"github.com/phroun/tuitk/protocol"
	sdlplat "github.com/phroun/tuitk/sdl"
	"github.com/phroun/tuitk/widgets"
	"github.com/phroun/tuitk/window"
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

	application := app.New(nil)
	application.SetName("SDL Desktop")
	desktop.AddApplication(application)

	desktop.SetOnStartup(func() {
		if w := welcomeWindow(application); w != nil {
			application.AddWindow(w)
		}
		if _, err := display.Serve(desktop, client.DefaultSocketPath()); err != nil {
			if sb := desktop.StatusBar(); sb != nil {
				sb.SetText("display service unavailable: " + err.Error())
			}
		} else if sb := desktop.StatusBar(); sb != nil {
			sb.SetText("display service on " + client.DefaultSocketPath() + " - run examples/remoteapp to connect")
		}
	})

	os.Exit(desktop.RunOn(plat))
}

// welcomeWindow is protocol-built, of course.
func welcomeWindow(application *app.Application) *window.Window {
	conn := client.NewInProcess(func(id string) { application.Commands().Dispatch(id) })
	ui, err := conn.Build(`
w=new window title="Graphical tuitk" x=24 y=40 width=460 height=280 children={
	p=new panel layout=vbox spacing=0 children={
		new label caption="This desktop is rendered as PIXELS - same widgets," wrap
		new label caption="same protocol, same everything. Only the display" wrap
		new label caption="service binary changed."
		new separator caption="Try it"
		new label caption="Run examples/remoteapp in another terminal: its"
		new label caption="window appears here, identical to the TUI desktop."
		status=new label caption="Interact below; events appear here."
		cb=new checkbox caption="A checkbox, now in pixels" tristate
		inp=new textinput placeholder="Type here..."
	}
}
watch=w.p.status
wcb=w.p.cb
winp=w.p.inp
`)
	if err != nil {
		return nil
	}
	status := ui.Label("watch")
	ui.Checkbox("wcb").OnToggle(func(s protocol.FlagState) {
		state := map[protocol.FlagState]string{
			protocol.FlagTrue: "on", protocol.FlagFalse: "off", protocol.FlagIndeterminate: "mixed",
		}[s]
		status.SetCaption("toggle -> " + state)
	})
	ui.TextInput("winp").OnChange(func(s string) {
		status.SetCaption(fmt.Sprintf("change -> %q", s))
	})

	w, _ := ui.Object("w").Target().(*window.Window)
	_ = core.Unit(0) // keep core imported for future geometry tweaks
	return w
}
