package display_test

// The payoff test: a client (the app side, linking client+protocol
// only) dials a RUNNING headless desktop over a real unix socket,
// builds UI from protocol text, exchanges events both ways, and
// disconnects - watching its windows disappear.

import (
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/phroun/tuitk/client"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/display"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/widgets"
)

// nullBackend: headless RenderBackend (display-test copy).
type nullBackend struct{ mu sync.Mutex }

func (n *nullBackend) Init() error { return nil }
func (n *nullBackend) Shutdown()   {}
func (n *nullBackend) Metrics() core.CellMetrics {
	return core.CellMetrics{CellWidth: 8, CellHeight: 16}
}
func (n *nullBackend) Size() core.UnitSize                                  { return core.UnitSize{Width: 8 * 80, Height: 16 * 24} }
func (n *nullBackend) BeginFrame()                                          {}
func (n *nullBackend) EndFrame()                                            {}
func (n *nullBackend) Clear(style.CellStyle)                                {}
func (n *nullBackend) SetClip(core.UnitRect)                                {}
func (n *nullBackend) DrawCell(core.Unit, core.Unit, rune, style.CellStyle) {}
func (n *nullBackend) DrawText(x, y core.Unit, t string, s style.CellStyle, f *core.Font) core.Unit {
	return 0
}
func (n *nullBackend) DrawTextAligned(core.UnitRect, string, core.Alignment, core.Alignment, style.CellStyle, *core.Font) {
}
func (n *nullBackend) FillRect(core.UnitRect, rune, style.CellStyle)                     {}
func (n *nullBackend) DrawRect(core.UnitRect, style.BorderStyle, style.CellStyle)        {}
func (n *nullBackend) DrawHLine(core.Unit, core.Unit, core.Unit, rune, style.CellStyle)  {}
func (n *nullBackend) DrawVLine(core.Unit, core.Unit, core.Unit, rune, style.CellStyle)  {}
func (n *nullBackend) DrawBox(core.UnitRect, style.BorderStyle, string, style.CellStyle) {}
func (n *nullBackend) PollEvent() core.Event                                             { return nil }
func (n *nullBackend) WaitEvent() core.Event                                             { return nil }
func (n *nullBackend) SetCursorVisible(bool)                                             {}
func (n *nullBackend) SetCursorPosition(core.Unit, core.Unit)                            {}
func (n *nullBackend) SupportsColor() bool                                               { return true }
func (n *nullBackend) SupportsMouse() bool                                               { return true }
func (n *nullBackend) SupportsUnicode() bool                                             { return true }
func (n *nullBackend) ColorDepth() int                                                   { return 256 }
func (n *nullBackend) GetClipboard() string                                              { return "" }
func (n *nullBackend) SetClipboard(string)                                               {}
func (n *nullBackend) Beep()                                                             {}

// onUI runs fn on the desktop's UI thread and waits.
func onUI(d *widgets.Desktop, fn func()) {
	done := make(chan struct{})
	d.Post(func() { fn(); close(done) })
	<-done
}

func TestRemoteAppOverUnixSocket(t *testing.T) {
	sock := filepath.Join(t.TempDir(), "display.sock")

	desktop := widgets.NewDesktop()
	desktop.SetBackend(&nullBackend{})

	ready := make(chan *display.Server, 1)
	desktop.SetOnStartup(func() {
		srv, err := display.Serve(desktop, sock)
		if err != nil {
			t.Errorf("serve: %v", err)
			desktop.Quit()
			return
		}
		ready <- srv
	})

	exited := make(chan int, 1)
	go func() { exited <- desktop.Run() }()
	var srv *display.Server
	select {
	case srv = <-ready:
	case <-time.After(5 * time.Second):
		t.Fatal("desktop did not start")
	}
	defer func() {
		srv.Close()
		desktop.Quit()
		select {
		case <-exited:
		case <-time.After(5 * time.Second):
			t.Error("desktop did not exit")
		}
	}()

	// THE moment: a separate "app" dials the running desktop.
	dispatched := make(chan string, 8)
	conn, err := client.Dial(sock, "Remote Test App", func(id string) { dispatched <- id })
	if err != nil {
		t.Fatalf("dial: %v", err)
	}

	ui, err := conn.Build(`
w=new window title="Remote Window" width=320 height=160 children={
	p=new panel layout=vbox children={
		cb=new checkbox caption="remote checkbox"
		inp=new textinput
		btn=new button caption="Go" action=remote.act
	}
}
wcb=w.p.cb
winp=w.p.inp
wbtn=w.p.btn
`)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	for _, k := range []string{"w", "wcb", "winp", "wbtn"} {
		if ui.ID(k) == 0 {
			t.Fatalf("missing surfaced id %q", k)
		}
	}

	// The connection appears as a full Application with the window.
	var appNames []string
	var winCount int
	var serverCb *widgets.Checkbox
	var serverInp *widgets.TextInput
	var serverBtn *widgets.Button
	onUI(desktop, func() {
		for _, a := range desktop.Applications() {
			appNames = append(appNames, a.Name())
			for _, w := range a.Windows() {
				winCount++
				if p, ok := w.Content().(*widgets.Panel); ok {
					kids := p.Children()
					if len(kids) == 3 {
						serverCb, _ = kids[0].(*widgets.Checkbox)
						serverInp, _ = kids[1].(*widgets.TextInput)
						serverBtn, _ = kids[2].(*widgets.Button)
					}
				}
			}
		}
	})
	if len(appNames) != 1 || appNames[0] != "Remote Test App" {
		t.Fatalf("applications = %v", appNames)
	}
	if winCount != 1 || serverCb == nil || serverInp == nil || serverBtn == nil {
		t.Fatalf("remote window content not found (windows=%d)", winCount)
	}

	// App -> display: write-through set lands in the real widget.
	inp := ui.TextInput("winp")
	if err := inp.SetText("over the wire"); err != nil {
		t.Fatalf("SetText: %v", err)
	}
	var got string
	onUI(desktop, func() { got = serverInp.Text() })
	if got != "over the wire" {
		t.Errorf("server text = %q", got)
	}

	// Display -> app: user toggles; the event crosses the socket into
	// the replica and the handler.
	cb := ui.Checkbox("wcb")
	toggled := make(chan bool, 1)
	cb.OnToggle(func(s protocol.FlagState) { toggled <- s == protocol.FlagTrue })
	onUI(desktop, func() { serverCb.Toggle() })
	select {
	case v := <-toggled:
		if !v {
			t.Error("toggle state = false, want true")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("toggle event never arrived")
	}
	if !cb.Checked() {
		t.Error("replica not updated")
	}

	// Command dispatch across the seam: click -> command event ->
	// app-side dispatch sink.
	onUI(desktop, func() { serverBtn.Click() })
	select {
	case id := <-dispatched:
		if id != "remote.act" {
			t.Errorf("dispatched %q", id)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("command never dispatched")
	}

	// Disconnect: the app and its windows leave the desktop.
	conn.Close()
	deadline := time.Now().Add(5 * time.Second)
	for {
		var apps int
		onUI(desktop, func() { apps = len(desktop.Applications()) })
		if apps == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("app still present after disconnect (%d)", apps)
		}
		time.Sleep(20 * time.Millisecond)
	}
}
