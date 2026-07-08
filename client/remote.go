package client

import (
	"fmt"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/phroun/tuitk/protocol"
)

// DisplayEnv is the environment variable naming the display endpoint
// (a unix socket path). DefaultSocketPath is used when unset.
const DisplayEnv = "TUITK_DISPLAY"

// DefaultSocketPath returns the conventional endpoint:
// $TUITK_DISPLAY, else $XDG_RUNTIME_DIR/tuitk/display-0.sock.
func DefaultSocketPath() string {
	if p := os.Getenv(DisplayEnv); p != "" {
		return p
	}
	runtimeDir := os.Getenv("XDG_RUNTIME_DIR")
	if runtimeDir == "" {
		runtimeDir = os.TempDir()
	}
	return filepath.Join(runtimeDir, "tuitk", "display-0.sock")
}

// Dial connects to a display service (D22 transport: protocol text
// over a unix socket). appName identifies the application in the
// handshake; dispatch receives action= command IDs (may be nil).
//
// Remote-connection caveats: event handlers run on the connection's
// reader goroutine, and Handle.Target() is always nil (the trinkets
// live in the display service's process).
func Dial(path, appName string, dispatch func(commandID string)) (*Conn, error) {
	return dial(path, appName, dispatch, false)
}

// DialSolo is Dial for an app that wants to be the whole display: its
// `main` window replaces the desktop entirely (no system menu, dock or
// wallpaper), rendered like a torn-off window filling the surface. The
// host quits when the last window closes (see docs/solo-app-plan.md).
func DialSolo(path, appName string, dispatch func(commandID string)) (*Conn, error) {
	return dial(path, appName, dispatch, true)
}

func dial(path, appName string, dispatch func(commandID string), solo bool) (*Conn, error) {
	nc, err := net.Dial("unix", path)
	if err != nil {
		return nil, err
	}

	c := newConn(dispatch)
	rt := &remoteTransport{
		conn:    c,
		nc:      nc,
		scanner: protocol.NewScanner(nc),
		replies: make(chan replyOrError, 1),
		events:  make(chan *protocol.Event, 256),
	}
	c.transport = rt

	// Handshake: hello out, welcome back (reattach-ready: the reply
	// carries the server-assigned session id). The optional `solo` flag
	// asks the display to run this app as the whole surface.
	hello := fmt.Sprintf("hello version=1 app=%s", protocol.Quote(appName))
	if solo {
		hello += " solo"
	}
	if _, err := nc.Write([]byte(hello + "\nend\n")); err != nil {
		nc.Close()
		return nil, err
	}
	welcome, err := rt.scanner.Next()
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("handshake: %w", err)
	}
	script, err := protocol.Parse(welcome)
	if err != nil || len(script.Statements) == 0 || script.Statements[0].Verb != "welcome" {
		nc.Close()
		return nil, fmt.Errorf("handshake: unexpected response %q", welcome)
	}

	go rt.readLoop()
	go rt.eventLoop()
	return c, nil
}

type replyOrError struct {
	reply *protocol.Reply
	err   error
}

// remoteTransport speaks D22 over a socket: batches out (terminated
// by end), reply/error/event statements in.
type remoteTransport struct {
	conn    *Conn
	nc      net.Conn
	scanner *protocol.Scanner

	writeMu sync.Mutex
	replies chan replyOrError

	// events are delivered on their own goroutine so a handler that
	// executes statements (SetCaption inside OnToggle) cannot
	// deadlock the reader that must route the reply.
	events chan *protocol.Event

	closeOnce sync.Once
}

func (t *remoteTransport) exec(src string) (*protocol.Reply, error) {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()

	if _, err := t.nc.Write([]byte(src + "\nend\n")); err != nil {
		return nil, err
	}
	r, ok := <-t.replies
	if !ok {
		return nil, fmt.Errorf("connection closed")
	}
	return r.reply, r.err
}

func (t *remoteTransport) close() error {
	var err error
	t.closeOnce.Do(func() { err = t.nc.Close() })
	return err
}

// readLoop routes inbound statements: replies/errors complete a
// pending exec; events queue for the event goroutine.
func (t *remoteTransport) readLoop() {
	defer func() {
		t.close()
		close(t.replies)
		close(t.events)
		t.conn.markClosed()
	}()
	for {
		text, err := t.scanner.Next()
		if err != nil {
			return
		}
		script, err := protocol.Parse(text)
		if err != nil {
			continue // malformed inbound statement; skip
		}
		for _, stmt := range script.Statements {
			switch stmt.Verb {
			case "reply":
				r, err := protocol.DecodeReply(stmt)
				t.replies <- replyOrError{reply: r, err: err}
			case "error":
				msg := "display error"
				for _, a := range stmt.Args {
					if a.Name == "text" && a.Value != nil && a.Value.Kind == protocol.StringValue {
						msg = a.Value.Str
					}
				}
				t.replies <- replyOrError{err: fmt.Errorf("%s", msg)}
			case "event":
				if ev, err := protocol.ParseEvent(text); err == nil {
					t.events <- ev
				}
			}
		}
	}
}

// eventLoop delivers events in order on a dedicated goroutine (the
// remote handler-thread: replica folding then app handlers).
func (t *remoteTransport) eventLoop() {
	for ev := range t.events {
		t.conn.deliver(ev)
	}
}
