package tui

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
	"time"
)

// With read-back enabled, GetClipboard emits the OSC 52 query and returns the
// terminal's reply (delivered on osc52Resp as the keyboard handler would).
func TestTUIClipboardReadBackSuccess(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Clipboard = true
	opts.OSC52Paste = true
	b := NewTUIBackend(opts)
	b.SetClipboard("internal-old")

	// Deliver the terminal's reply shortly after the query goes out.
	go func() {
		time.Sleep(10 * time.Millisecond)
		b.osc52Resp <- "from-terminal"
	}()

	if got := b.GetClipboard(); got != "from-terminal" {
		t.Errorf("read-back = %q, want from-terminal", got)
	}
	if !strings.Contains(out.String(), "\033]52;c;?\a") {
		t.Errorf("OSC 52 query not emitted; output = %q", out.String())
	}
}

// When the terminal stays silent, GetClipboard falls back to the internal
// clipboard after the timeout.
func TestTUIClipboardReadBackFallback(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Paste = true
	b := NewTUIBackend(opts)
	b.SetClipboard("internal-fallback")

	if got := b.GetClipboard(); got != "internal-fallback" {
		t.Errorf("silent-terminal fallback = %q, want internal-fallback", got)
	}
}

// Without read-back, GetClipboard never queries the terminal - it just returns
// the internal clipboard.
func TestTUIClipboardNoReadBackNoQuery(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Paste = false
	b := NewTUIBackend(opts)
	b.SetClipboard("kept")
	out.Reset() // ignore the SetClipboard OSC 52 write

	if got := b.GetClipboard(); got != "kept" {
		t.Errorf("GetClipboard = %q, want kept", got)
	}
	if out.Len() != 0 {
		t.Errorf("read-back off should emit no query, got %q", out.String())
	}
}

// With OSC 52 enabled, SetClipboard stores the text internally AND emits the
// OSC 52 set sequence (ESC ] 52 ; c ; <base64> BEL) so the terminal's clipboard
// receives the copy. GetClipboard returns the internal copy (Paste source).
func TestTUIClipboardOSC52(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Clipboard = true
	b := NewTUIBackend(opts)

	b.SetClipboard("hi there")

	if got := b.GetClipboard(); got != "hi there" {
		t.Errorf("GetClipboard = %q, want %q", got, "hi there")
	}

	want := "\033]52;c;" + base64.StdEncoding.EncodeToString([]byte("hi there")) + "\a"
	if got := out.String(); got != want {
		t.Errorf("OSC 52 output = %q, want %q", got, want)
	}
}

// With OSC 52 disabled ([tui] clipboard=internal), SetClipboard writes nothing
// to the terminal and keeps an internal-only clipboard.
func TestTUIClipboardInternalOnly(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Clipboard = false
	b := NewTUIBackend(opts)

	b.SetClipboard("secret")

	if out.Len() != 0 {
		t.Errorf("internal-only mode wrote %q to the terminal, want nothing", out.String())
	}
	if got := b.GetClipboard(); got != "secret" {
		t.Errorf("GetClipboard = %q, want secret", got)
	}
}

// The emitted sequence must be a well-formed OSC 52 set: no stray ESC/BEL
// inside, correct target selection ("c").
func TestTUIClipboardOSC52Framing(t *testing.T) {
	var out bytes.Buffer
	opts := DefaultTUIOptions()
	opts.Output = &out
	opts.OSC52Clipboard = true
	b := NewTUIBackend(opts)

	b.SetClipboard("x")
	s := out.String()
	if !strings.HasPrefix(s, "\033]52;c;") {
		t.Errorf("missing OSC 52 header in %q", s)
	}
	if !strings.HasSuffix(s, "\a") {
		t.Errorf("OSC 52 not BEL-terminated in %q", s)
	}
}
