package tui

import (
	"bytes"
	"encoding/base64"
	"strings"
	"testing"
)

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
