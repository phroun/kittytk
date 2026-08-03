//go:build sdl

package sdlcompat

import (
	"testing"

	sdl3 "github.com/Zyko0/go-sdl3/sdl"
)

// Every translated event must report the ADAPTER's type constants, not
// SDL3's raw event numbers: the platform layer switches on the former,
// so passing the latter through silently drops the event. That is
// exactly how mouse clicks went missing — presses arrived as
// SDL3's EVENT_MOUSE_BUTTON_DOWN (1025) while the platform compared
// against MOUSEBUTTONDOWN (3), so no comparison ever matched and every
// press was treated as a release.
func TestTranslateReportsAdapterEventTypes(t *testing.T) {
	cases := []struct {
		name string
		in   sdl3.EventType
		want interface{}
		typ  uint32 // expected translated Type, where the value carries one
	}{
		{"button down", sdl3.EVENT_MOUSE_BUTTON_DOWN, &MouseButtonEvent{}, MOUSEBUTTONDOWN},
		{"button up", sdl3.EVENT_MOUSE_BUTTON_UP, &MouseButtonEvent{}, MOUSEBUTTONUP},
		{"key down", sdl3.EVENT_KEY_DOWN, &KeyboardEvent{}, KEYDOWN},
		{"key up", sdl3.EVENT_KEY_UP, &KeyboardEvent{}, KEYUP},
	}
	for _, c := range cases {
		got := translate(&sdl3.Event{Type: c.in})
		if got == nil {
			t.Errorf("%s: not translated", c.name)
			continue
		}
		switch v := got.(type) {
		case *MouseButtonEvent:
			if v.Type != c.typ {
				t.Errorf("%s: Type = %d, want %d", c.name, v.Type, c.typ)
			}
			// A press must also report the pressed state.
			if c.typ == MOUSEBUTTONDOWN && v.State != 1 {
				t.Errorf("%s: State = %d, want 1", c.name, v.State)
			}
		case *KeyboardEvent:
			if v.Type != c.typ {
				t.Errorf("%s: Type = %d, want %d", c.name, v.Type, c.typ)
			}
		default:
			t.Errorf("%s: translated to %T, want %T", c.name, got, c.want)
		}
	}
}

// SDL3's distinct window event types collapse back onto SDL2's
// WINDOWEVENT subtypes, which is what the platform switches on.
func TestTranslateWindowSubtypes(t *testing.T) {
	cases := []struct {
		in   sdl3.EventType
		want uint8
	}{
		{sdl3.EVENT_WINDOW_RESIZED, WINDOWEVENT_SIZE_CHANGED},
		{sdl3.EVENT_WINDOW_PIXEL_SIZE_CHANGED, WINDOWEVENT_SIZE_CHANGED},
		{sdl3.EVENT_WINDOW_FOCUS_GAINED, WINDOWEVENT_FOCUS_GAINED},
		{sdl3.EVENT_WINDOW_FOCUS_LOST, WINDOWEVENT_FOCUS_LOST},
		{sdl3.EVENT_WINDOW_MOUSE_LEAVE, WINDOWEVENT_LEAVE},
	}
	for _, c := range cases {
		we, ok := translate(&sdl3.Event{Type: c.in}).(*WindowEvent)
		if !ok {
			t.Errorf("SDL3 event %d did not translate to a WindowEvent", c.in)
			continue
		}
		if we.Event != c.want {
			t.Errorf("SDL3 event %d: subtype = %d, want %d", c.in, we.Event, c.want)
		}
	}
}

// A quit translates; an event the host does not handle returns nil
// rather than a zero-valued struct that would be misread downstream.
func TestTranslateQuitAndUnhandled(t *testing.T) {
	if _, ok := translate(&sdl3.Event{Type: sdl3.EVENT_QUIT}).(*QuitEvent); !ok {
		t.Error("quit did not translate")
	}
	if got := translate(&sdl3.Event{Type: sdl3.EVENT_JOYSTICK_ADDED}); got != nil {
		t.Errorf("unhandled event translated to %T, want nil", got)
	}
}
