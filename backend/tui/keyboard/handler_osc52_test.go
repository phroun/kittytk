package keyboard

import (
	"encoding/base64"
	"io"
	"testing"
	"time"
)

// feed starts a Handler over a pipe, writes raw, and collects the OnClipboard
// response, OnPaste content, and any keys seen (to prove OSC responses don't
// leak as keystrokes). It waits briefly for parsing to settle.
func feed(t *testing.T, raw string) (clip string, sel byte, gotClip bool, paste string, keys []string) {
	t.Helper()
	pr, pw := io.Pipe()
	h := New(Options{InputReader: pr})

	clipCh := make(chan struct{}, 1)
	h.OnClipboard = func(s byte, data []byte) {
		sel = s
		clip = string(data)
		gotClip = true
		select {
		case clipCh <- struct{}{}:
		default:
		}
	}
	h.OnPaste = func(data []byte) { paste = string(data) }
	h.OnKey = func(k string) { keys = append(keys, k) }

	if err := h.Start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	defer h.Stop()

	if _, err := pw.Write([]byte(raw)); err != nil {
		t.Fatalf("write: %v", err)
	}
	// Give the reader goroutine time to parse.
	select {
	case <-clipCh:
	case <-time.After(200 * time.Millisecond):
	}
	time.Sleep(20 * time.Millisecond)
	return
}

func osc52(sel, payload string) string {
	return "\x1b]52;" + sel + ";" + base64.StdEncoding.EncodeToString([]byte(payload)) + "\a"
}

func TestHandlerOSC52BEL(t *testing.T) {
	clip, sel, got, _, keys := feed(t, osc52("c", "hello clipboard"))
	if !got {
		t.Fatal("OnClipboard never fired")
	}
	if clip != "hello clipboard" {
		t.Errorf("clipboard = %q, want %q", clip, "hello clipboard")
	}
	if sel != 'c' {
		t.Errorf("selection = %q, want 'c'", sel)
	}
	if len(keys) != 0 {
		t.Errorf("OSC 52 response leaked as keys: %v", keys)
	}
}

func TestHandlerOSC52ST(t *testing.T) {
	// ST-terminated (ESC \) rather than BEL.
	raw := "\x1b]52;c;" + base64.StdEncoding.EncodeToString([]byte("via ST")) + "\x1b\\"
	clip, _, got, _, keys := feed(t, raw)
	if !got || clip != "via ST" {
		t.Errorf("ST clipboard = %q got=%v, want 'via ST'", clip, got)
	}
	if len(keys) != 0 {
		t.Errorf("ST OSC 52 leaked as keys: %v", keys)
	}
}

func TestHandlerOSC52Interleaved(t *testing.T) {
	// Real keystrokes around the clipboard response: keys pass through, the
	// response is captured and does not appear among the keys.
	clip, _, got, _, keys := feed(t, "a"+osc52("c", "MID")+"b")
	if !got || clip != "MID" {
		t.Errorf("clipboard = %q got=%v, want MID", clip, got)
	}
	joined := ""
	for _, k := range keys {
		joined += k
	}
	if joined != "ab" {
		t.Errorf("keys = %v, want just a,b", keys)
	}
}

// Bracketed paste is untouched by the OSC 52 addition.
func TestHandlerBracketedPasteStillWorks(t *testing.T) {
	_, _, _, paste, _ := feed(t, "\x1b[200~pasted text\x1b[201~")
	if paste != "pasted text" {
		t.Errorf("bracketed paste = %q, want 'pasted text'", paste)
	}
}

// A malformed base64 body is dropped without firing OnClipboard or leaking.
func TestHandlerOSC52Malformed(t *testing.T) {
	_, _, got, _, keys := feed(t, "\x1b]52;c;!!notbase64!!\a")
	if got {
		t.Error("malformed OSC 52 should not fire OnClipboard")
	}
	if len(keys) != 0 {
		t.Errorf("malformed OSC 52 leaked as keys: %v", keys)
	}
}
