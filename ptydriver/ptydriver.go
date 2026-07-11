// Package ptydriver runs a child process on the CLIENT side of the
// display protocol and bridges it to a terminal surface.
//
// Under KittyTK's network-rendering model the render server draws the
// terminal but never spawns the child: the process belongs to the
// application (the client). A Driver owns a real PTY, spawns the shell (or
// any command) into it, and pumps the child's output to a caller-supplied
// sink - typically the terminal's feed= property over the wire, or its
// Feed method in-process. The reverse direction (the user's keystrokes,
// mouse reports, and paste, plus grid-size changes) is delivered back to
// the Driver via Input and Resize, which it writes to the PTY.
package ptydriver

import (
	"os"
	"os/exec"

	"github.com/phroun/purfecterm"
)

// Driver bridges a client-side PTY to a terminal surface. Create one with
// Start; feed user input to Input/Resize; close it with Close.
type Driver struct {
	pty  purfecterm.PTY
	cmd  *exec.Cmd
	done chan struct{}
}

// defaultShell picks the user's shell, falling back to a POSIX sh.
func defaultShell() string {
	if s := os.Getenv("SHELL"); s != "" {
		return s
	}
	return "/bin/sh"
}

// Start spawns command (empty name = the user's login shell) in a fresh
// PTY and begins pumping its output to feed. feed is called from a reader
// goroutine with each chunk the child writes; it must be safe to call
// concurrently with the caller's other work (the wire client and the
// in-process Feed both are). The child inherits the environment with
// TERM/COLORTERM advertising a modern terminal.
func Start(command string, feed func([]byte), args ...string) (*Driver, error) {
	pty, err := purfecterm.NewPTY()
	if err != nil {
		return nil, err
	}
	if command == "" {
		command = defaultShell()
	}
	cmd := exec.Command(command, args...)
	cmd.Env = append(os.Environ(),
		"TERM=xterm-256color",
		"COLORTERM=truecolor",
	)
	if err := pty.Start(cmd); err != nil {
		pty.Close()
		return nil, err
	}
	d := &Driver{pty: pty, cmd: cmd, done: make(chan struct{})}
	go d.readLoop(feed)
	return d, nil
}

// readBufSize is the pty read buffer. Each read becomes one wire feed batch,
// and each batch is a synchronous round-trip that stalls the reader until the
// host applies it and replies - so bulk output (a fast-scrolling program, a
// full-screen animation) is drained one buffer per round-trip. A large buffer
// lets a single read grab the whole pty backlog that accumulated during the
// previous round-trip, collapsing many batches into one and multiplying
// throughput; interactive output is small and unaffected.
const readBufSize = 128 * 1024

// readLoop forwards child output to the feed sink until the PTY closes.
func (d *Driver) readLoop(feed func([]byte)) {
	defer close(d.done)
	buf := make([]byte, readBufSize)
	for {
		n, err := d.pty.Read(buf)
		if n > 0 && feed != nil {
			// Copy: buf is reused on the next read, and feed may hand
			// the slice to another goroutine (the wire encoder).
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			feed(chunk)
		}
		if err != nil {
			return
		}
	}
}

// Input writes bytes the user produced (keystrokes, mouse reports, paste)
// to the child process.
func (d *Driver) Input(b []byte) {
	if len(b) > 0 {
		_, _ = d.pty.Write(b)
	}
}

// Resize sets the child's PTY winsize to cols x rows.
func (d *Driver) Resize(cols, rows int) {
	if cols > 0 && rows > 0 {
		_ = d.pty.Resize(cols, rows)
	}
}

// Done returns a channel closed when the child's output stream ends (the
// process exited or the PTY was closed).
func (d *Driver) Done() <-chan struct{} { return d.done }

// Close terminates the child and releases the PTY.
func (d *Driver) Close() {
	if d.cmd != nil && d.cmd.Process != nil {
		_ = d.cmd.Process.Kill()
	}
	if d.pty != nil {
		_ = d.pty.Close()
	}
}
