// Package display is the display-service side of the D22 transport:
// a listener that accepts app connections speaking protocol text
// over a unix socket, giving each one a full Application in the
// desktop (menus, status bar, windows - peers of in-process apps).
//
// Threading (D21): socket readers parse batches off the wire and
// Post them onto the desktop's platform thread; everything that
// touches widgets runs there. Event emission enqueues onto a
// per-connection writer goroutine, so the UI thread never blocks on
// a slow client.
package display

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sync"
	"sync/atomic"

	"github.com/phroun/tuitk/app"
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/style"
	"github.com/phroun/tuitk/widgets"
	"github.com/phroun/tuitk/window"
)

// Server accepts display-protocol connections for one desktop.
type Server struct {
	desktop  *widgets.Desktop
	listener net.Listener
	sessions atomic.Uint64
	closed   atomic.Bool
}

// Serve listens on the unix socket at path (creating its directory,
// 0700) and serves connections until Close. Call from desktop wiring
// (e.g. SetOnStartup).
func Serve(desktop *widgets.Desktop, path string) (*Server, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, err
	}
	_ = os.Remove(path) // stale socket from a previous run
	ln, err := net.Listen("unix", path)
	if err != nil {
		return nil, err
	}
	s := &Server{desktop: desktop, listener: ln}
	go s.acceptLoop()
	return s, nil
}

// Addr returns the listener address (the socket path).
func (s *Server) Addr() string { return s.listener.Addr().String() }

// Close stops accepting and closes the listener. Existing
// connections end when their sockets close.
func (s *Server) Close() error {
	s.closed.Store(true)
	return s.listener.Close()
}

func (s *Server) acceptLoop() {
	for {
		nc, err := s.listener.Accept()
		if err != nil {
			return // listener closed
		}
		go s.serveConn(nc)
	}
}

// conn is one app connection's display-side state.
type conn struct {
	server *Server
	nc     net.Conn

	session *protocol.Session
	factory *hostFactory
	app     *app.Application

	// Accessibility routing state (the "Show/Speak Announcements"
	// toggles): the connection owns the intent, and installs a desktop
	// OnAnnounce handler reflecting it. Speech serializes through
	// speechMu so a new utterance cancels the previous one.
	announceVisual bool
	announceSpeak  bool
	speechMu       sync.Mutex
	speechCmd      *exec.Cmd

	// outbound statements; the writer goroutine owns the socket's
	// write side.
	out chan string
}

func (s *Server) serveConn(nc net.Conn) {
	defer nc.Close()
	scanner := protocol.NewScanner(nc)

	// Handshake: first batch must open with hello (D22
	// reattach-ready: we assign a session id and return it).
	first, err := readBatch(scanner)
	if err != nil || len(first) == 0 || first[0].Verb != "hello" {
		fmt.Fprintf(nc, "%s\n", protocol.EncodeError("handshake: expected hello"))
		return
	}
	appName := "Remote App"
	for _, a := range first[0].Args {
		if a.Name == "app" && a.Value != nil && a.Value.Kind == protocol.StringValue {
			appName = a.Value.Str
		}
	}
	sessionID := s.sessions.Add(1)

	c := &conn{
		server:  s,
		nc:      nc,
		session: protocol.NewSession(),
		out:     make(chan string, 1024),
	}

	// Per-connection BindContext: events encode onto the wire.
	// Dispatch stays nil - command events ARE the dispatch path
	// across the seam (the app-side registry lives in the app).
	ctx := &protocol.BindContext{
		Emit: func(ev *protocol.Event) { c.send(ev.Encode()) },
	}
	c.factory = &hostFactory{inner: protocol.NewRegistryFactory(ctx)}

	// The connection is a full Application (D22).
	application := app.New(nil)
	application.SetName(appName)
	c.app = application
	s.desktop.Post(func() { s.desktop.AddApplication(application) })
	defer s.desktop.Post(func() { c.teardown() })

	go c.writeLoop()
	defer close(c.out)

	c.send(fmt.Sprintf("welcome version=1 session=%d", sessionID))

	// Batch loop: read until end, execute on the UI thread, reply.
	for {
		batch, err := readBatch(scanner)
		if err != nil {
			return // disconnect -> deferred teardown
		}
		done := make(chan struct{})
		s.desktop.Post(func() {
			defer close(done)
			c.execute(batch)
		})
		<-done
	}
}

// readBatch collects statements until the D22 end terminator.
func readBatch(scanner *protocol.Scanner) ([]*protocol.Statement, error) {
	var batch []*protocol.Statement
	for {
		text, err := scanner.Next()
		if err != nil {
			return nil, err
		}
		script, err := protocol.Parse(text)
		if err != nil {
			// A malformed statement poisons the batch; report at
			// execution time by injecting a marker error.
			return nil, fmt.Errorf("parse: %w", err)
		}
		for _, stmt := range script.Statements {
			if stmt.Verb == "end" && stmt.Key == "" {
				return batch, nil
			}
			batch = append(batch, stmt)
		}
	}
}

// execute runs one batch on the UI thread and replies.
func (c *conn) execute(batch []*protocol.Statement) {
	// Session-level app verbs the protocol session doesn't model are
	// handled here before the rest of the batch runs against the session.
	batch = c.handleAppVerbs(batch)

	script := &protocol.Script{Statements: batch}
	reply, err := c.session.Execute(script, c.factory)
	if err != nil {
		c.send(protocol.EncodeError(err.Error()))
		return
	}

	// Adopt what the batch created: windows join the connection's
	// application; a menubar/statusbar becomes the app's bar content.
	for _, target := range c.factory.take() {
		switch t := target.(type) {
		case *window.Window:
			// A window built as a child of an existing widget (e.g. an
			// MDI document appended into an mdipane) already has a home;
			// only genuinely top-level windows join the application.
			if t.Parent() == nil {
				c.app.AddWindow(t)
				// A window that asked to be the app's main window carries
				// the menu/status chrome when torn off.
				if t.MainRequested() {
					c.app.SetMainWindow(t)
				}
			}
		case *widgets.MessageBox:
			if t.Window.Parent() == nil {
				c.app.AddWindow(&t.Window)
			}
		case interface{ Menus() []*widgets.Menu }:
			c.app.SetMenuBarContent(t.Menus())
		case interface {
			Sections() []widgets.StatusSection
		}:
			c.app.SetStatusBarContent(t.Sections())
		}
	}
	c.server.desktop.RequestUpdate()

	c.send(protocol.EncodeReply(reply))
}

// handleAppVerbs consumes the session-level application verbs the display
// implements directly (the protocol session has a closed verb set), and
// returns the remaining statements for the session to run. These are the
// desktop-reaching actions a remote app can't perform through its own
// widget handles - the display does them on the app's behalf:
//
//	rawkey            - pass the next key straight to the focused widget
//	cut/copy/paste/   - the standard edit actions on the focused widget
//	  selectall
//	tile/cascade      - arrange the desktop's windows
//	theme             - toggle the dark/light terminal theme (+ retheme)
//	desktopfont NAME  - set the desktop font (tuesday | default)
//	announce_visual   - toggle showing announcements in the status bar
//	announce_speak    - toggle speaking announcements (macOS `say`)
func (c *conn) handleAppVerbs(batch []*protocol.Statement) []*protocol.Statement {
	d := c.server.desktop
	rest := batch[:0:0]
	for _, stmt := range batch {
		if stmt.Key != "" {
			rest = append(rest, stmt)
			continue
		}
		switch stmt.Verb {
		case "rawkey":
			d.ActivatePassNextKeyToWidget()
		case "status":
			if sb := d.StatusBar(); sb != nil {
				sb.SetText(argString(stmt, "text"))
			}
		case "cut", "copy", "paste", "selectall":
			editAction(d.FocusedWidget(), stmt.Verb)
		case "tile":
			if wm := d.WindowManager(); wm != nil {
				wm.TileWindows()
			}
		case "cascade":
			if wm := d.WindowManager(); wm != nil {
				wm.CascadeWindows()
			}
		case "theme":
			toggleTerminalTheme(d)
		case "desktopfont":
			d.SetFont(namedDesktopFont(firstWord(stmt)))
		case "announce_visual":
			c.announceVisual = !c.announceVisual
			c.updateAnnounce()
		case "announce_speak":
			c.announceSpeak = !c.announceSpeak
			c.updateAnnounce()
		default:
			rest = append(rest, stmt)
		}
	}
	return rest
}

// firstWord returns the first bare-word argument of a statement (the
// "tuesday" in `desktopfont tuesday`), or "" if there is none.
func firstWord(stmt *protocol.Statement) string {
	for _, a := range stmt.Args {
		if a.Flag == protocol.FlagTrue && a.Value == nil {
			return a.Name
		}
	}
	return ""
}

// argString returns the named string argument of a statement (the
// text= in `status text="..."`), or "" if absent.
func argString(stmt *protocol.Statement, name string) string {
	for _, a := range stmt.Args {
		if a.Name == name && a.Value != nil && a.Value.Kind == protocol.StringValue {
			return a.Value.Str
		}
	}
	return ""
}

// namedDesktopFont maps a desktopfont argument to a font (nil = the
// desktop default, Monday).
func namedDesktopFont(name string) *core.Font {
	if name == "tuesday" || name == "tuesday12" {
		return core.FontTuesday12
	}
	return nil
}

// editAction invokes one of the standard edit operations on a widget
// that supports them (text inputs, edit boxes); a nil or non-editing
// widget is a no-op.
func editAction(w core.Widget, verb string) {
	ea, ok := w.(interface {
		Cut()
		Copy()
		Paste()
		SelectAll()
	})
	if !ok {
		return
	}
	switch verb {
	case "cut":
		ea.Cut()
	case "copy":
		ea.Copy()
	case "paste":
		ea.Paste()
	case "selectall":
		ea.SelectAll()
	}
}

// toggleTerminalTheme flips the active dark/light terminal theme and
// repaints; embedded terminals follow via their own palette.
func toggleTerminalTheme(d *widgets.Desktop) {
	dark := style.ActiveTermTheme() == style.TermThemeLight
	if dark {
		style.SetActiveTermTheme(style.TermThemeDark)
	} else {
		style.SetActiveTermTheme(style.TermThemeLight)
	}
	if wm := d.WindowManager(); wm != nil {
		for _, w := range wm.Windows() {
			applyTerminalTheme(w, dark)
		}
	}
	d.RequestUpdate()
}

// applyTerminalTheme walks a widget subtree and puts every PurfecTerm
// into the given dark/light mode.
func applyTerminalTheme(w core.Widget, dark bool) {
	if w == nil {
		return
	}
	if term, ok := w.(*widgets.PurfecTerm); ok {
		term.SetDarkTheme(dark)
	}
	if cont, ok := w.(core.Container); ok {
		for _, child := range cont.Children() {
			applyTerminalTheme(child, dark)
		}
	}
}

// updateAnnounce installs or clears the desktop's OnAnnounce handler to
// reflect this connection's visual/speech toggles.
func (c *conn) updateAnnounce() {
	am := c.server.desktop.AccessibilityManager()
	if am == nil {
		return
	}
	if !c.announceVisual && !c.announceSpeak {
		am.OnAnnounce = nil
		return
	}
	am.OnAnnounce = func(a core.AccessibilityAnnouncement) {
		if c.announceVisual {
			if sb := c.server.desktop.StatusBar(); sb != nil {
				prefix := "\U0001F4E2"
				if a.Priority == "assertive" {
					prefix = "⚠️"
				}
				sb.SetText(fmt.Sprintf("%s [%s] %s", prefix, a.Priority, a.Message))
			}
		}
		if c.announceSpeak && a.Vocal && runtime.GOOS == "darwin" {
			c.speak(a.Message)
		}
	}
	// Announce the toggle itself so the change is perceptible.
	am.AnnouncePolite("Announcements updated")
}

// speak voices a message via macOS `say`, cancelling any in-flight
// utterance first (navigation throttling already thinned the stream).
func (c *conn) speak(msg string) {
	go func() {
		c.speechMu.Lock()
		if c.speechCmd != nil && c.speechCmd.Process != nil {
			_ = c.speechCmd.Process.Kill()
			_ = c.speechCmd.Wait()
		}
		c.speechCmd = exec.Command("say", "-r", "250", msg)
		c.speechMu.Unlock()
		_ = c.speechCmd.Run()
		c.speechMu.Lock()
		c.speechCmd = nil
		c.speechMu.Unlock()
	}()
}

// teardown runs on the UI thread at disconnect: the app and its
// windows leave the desktop (D22 v1; reattach arrives with D4).
func (c *conn) teardown() {
	for _, w := range c.app.Windows() {
		w.Close()
	}
	c.server.desktop.RemoveApplication(c.app)
	c.server.desktop.RequestUpdate()
}

// send enqueues one outbound statement line (non-blocking for the UI
// thread; a slow client eventually loses its connection rather than
// stalling the display).
func (c *conn) send(statement string) {
	select {
	case c.out <- statement:
	default:
		// Queue full: drop the connection's socket; the reader will
		// notice and tear down.
		c.nc.Close()
	}
}

func (c *conn) writeLoop() {
	for line := range c.out {
		if _, err := c.nc.Write([]byte(line + "\n")); err != nil {
			return
		}
	}
}

// hostFactory records batch-created top-level targets for adoption
// and forwards EventControl.
type hostFactory struct {
	inner   protocol.Factory
	created []any
}

func (f *hostFactory) New(typeName string) (protocol.Object, error) {
	o, err := f.inner.New(typeName)
	if err != nil {
		return nil, err
	}
	if tg, ok := o.(interface{ Target() any }); ok {
		f.created = append(f.created, tg.Target())
	}
	return o, nil
}

// take returns and clears the created list (called per batch, on the
// UI thread).
func (f *hostFactory) take() []any {
	out := f.created
	f.created = nil
	return out
}

func (f *hostFactory) Subscribe(id uint64, typ string) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Subscribe(id, typ)
	}
}
func (f *hostFactory) Unsubscribe(id uint64, typ string) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Unsubscribe(id, typ)
	}
}
func (f *hostFactory) Suppressed(fn func()) {
	if ec, ok := f.inner.(protocol.EventControl); ok {
		ec.Suppressed(fn)
		return
	}
	fn()
}
