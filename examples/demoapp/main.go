// Command demoapp is the TUI Toolkit demo as a display-protocol
// APPLICATION: it links only client + protocol (no rendering backend),
// dials a running tuitk display service, and drives its whole UI - main
// window, menu bar, status bar - over the socket.
//
//	terminal 1:  go run -tags sdl ./examples/sdldesktop
//	terminal 2:  go run ./examples/demoapp
//
// "File > New Window" opens a second, independent application by dialing
// another connection (one Application per connection, D22), so each has
// its own menu bar - the same shape the in-process demo produced.
package main

import (
	"fmt"
	"os"
	"sync"

	"github.com/phroun/tuitk/client"
	"github.com/phroun/tuitk/protocol"
)

func main() {
	path := client.DefaultSocketPath()

	app, err := newApp(path, "TUI Toolkit Demo", mainScript, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cannot reach display service at %s: %v\n", path, err)
		fmt.Fprintln(os.Stderr, "start the desktop first: go run -tags sdl ./examples/sdldesktop")
		os.Exit(1)
	}
	app.run() // blocks until the primary app quits
}

// secondaryCount names the windows opened via File > New Window.
var (
	secondaryMu    sync.Mutex
	secondaryCount int
	openApps       sync.WaitGroup
)

// app is one connection = one Application on the desktop.
type app struct {
	path     string
	conn     *client.Conn
	ui       *client.UI
	commands chan string
	primary  bool
}

// newApp dials a fresh connection, builds its UI over the socket, and
// wires its widget events. primary marks the app whose quit ends the
// process.
func newApp(path, name, script string, primary bool) (*app, error) {
	a := &app{path: path, commands: make(chan string, 16), primary: primary}
	conn, err := client.Dial(path, name, func(id string) { a.commands <- id })
	if err != nil {
		return nil, err
	}
	a.conn = conn

	ui, err := conn.Build(script)
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("build: %w", err)
	}
	a.ui = ui
	a.wire()
	return a, nil
}

// wire subscribes to the window's interactive widgets.
func (a *app) wire() {
	status := a.ui.Label("status")

	a.ui.Button("ok").OnClick(func() { status.SetCaption("OK clicked (round-tripped the socket)") })
	a.ui.Button("cancel").OnClick(func() { status.SetCaption("Cancel clicked") })

	// The checkbox drives the window's font over the wire: a window
	// property the app owns, not a hard-coded desktop menu.
	win := a.ui.Object("w")
	a.ui.Checkbox("fontcb").OnToggle(func(s protocol.FlagState) {
		if s == protocol.FlagTrue {
			_ = win.Set("font=tuesday12")
			status.SetCaption("Window font: proportional")
		} else {
			_ = win.Set("font=default")
			status.SetCaption("Window font: default")
		}
	})

	a.ui.TextInput("inp").OnChange(func(text string) {
		status.SetCaption(fmt.Sprintf("Text: %q", text))
	})
}

// run processes menu commands until this app quits, then (for the primary
// app) waits for any secondary apps to close too.
func (a *app) run() {
	openApps.Add(1)
	for id := range a.commands {
		switch id {
		case "demo.quit":
			a.conn.Close()
			openApps.Done()
			if a.primary {
				openApps.Wait()
				return
			}
			return
		case "demo.newwindow":
			openSecondary(a.path)
		case "demo.rawkey":
			// The "raw key input" action is a session verb: the display
			// passes the next key straight to the focused widget.
			_, _ = a.conn.Exec("rawkey")
		}
	}
}

// openSecondary dials a new connection - a new Application with its own
// menu bar - the client-side meaning of "New Window".
func openSecondary(path string) {
	secondaryMu.Lock()
	secondaryCount++
	n := secondaryCount
	secondaryMu.Unlock()

	name := fmt.Sprintf("App %d", n)
	sec, err := newApp(path, name, secondaryScript(n), false)
	if err != nil {
		return
	}
	go sec.run()
}

const mainScript = `
w=new window title="TUI Toolkit Demo" x=48 y=48 width=520 height=300 children={
	p=new panel layout=vbox spacing=8 children={
		new label caption="This application runs in its OWN process and drives" wrap
		new label caption="its entire UI - window, menu bar, status bar - over" wrap
		new label caption="the display protocol socket."
		new separator caption="Interact"
		row=new panel layout=hbox spacing=8 children={
			ok=new button caption="OK"
			cancel=new button caption="Cancel"
		}
		fontcb=new checkbox caption="Proportional window font (a window property)"
		inp=new textinput placeholder="Type here; changes narrate below..."
		status=new label caption="Ready. File > New Window opens another app."
	}
}
ok=w.p.row.ok
cancel=w.p.row.cancel
fontcb=w.p.fontcb
inp=w.p.inp
status=w.p.status
mb=new menubar children={
	new menu caption="&File" children={
		new menuitem caption="&New Window" shortcut="^N" action=demo.newwindow
		new menuitem separator
		new menuitem caption="&Quit" shortcut="^Q" action=demo.quit
	}
	new menu caption="&Edit" children={
		new menuitem caption="&Raw Key Input" shortcut="^\\" action=demo.rawkey
	}
}
sb=new statusbar children={new section children={new span text="TUI Toolkit Demo - connected over the socket"}}
`

// secondaryScript is a second application's window + menu bar, opened via
// File > New Window on its own connection.
func secondaryScript(n int) string {
	return fmt.Sprintf(`
w=new window title="App %d Window" x=%d y=%d width=420 height=200 children={
	p=new panel layout=vbox spacing=8 children={
		new label caption="This window belongs to Application #%d," wrap
		new label caption="a separate connection with its own menu bar."
		inp=new textinput placeholder="Its events cross the socket too..."
	}
}
inp=w.p.inp
mb=new menubar children={
	new menu caption="&App %d" children={
		new menuitem caption="&Close Window" shortcut="^Q" action=demo.quit
	}
}
sb=new statusbar children={new section children={new span text="Secondary Application #%d"}}
`, n, 80+n*24, 80+n*24, n, n, n)
}
