package widgets

import (
	"reflect"
	"testing"

	"github.com/phroun/tuitk/core"
)

func TestWrapTextMondayWordBoundaries(t *testing.T) {
	// Monday: every character is 8 units. 80 units = 10 characters.
	got := wrapText("hello world again", 80, core.FontMonday12)
	want := []string{"hello", "world", "again"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapTextMondayFitsTwoWords(t *testing.T) {
	// "to be" = 5 chars = 40 units, fits in 48.
	got := wrapText("to be or", 48, core.FontMonday12)
	want := []string{"to be", "or"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapTextTuesdayMeasuresDoubleWidth(t *testing.T) {
	// Tuesday: letters are 16 units, space is 8. "hello world" needs
	// 80 + 8 + 80 = 168 units; at 160 it must break between the words.
	got := wrapText("hello world", 160, core.FontTuesday12)
	want := []string{"hello", "world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}

	// The same call in Monday (88 units total) fits on one line at 160.
	got = wrapText("hello world", 160, core.FontMonday12)
	want = []string{"hello world"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("Monday control: got %q, want %q", got, want)
	}
}

func TestWrapTextBreaksOverlongWordByCharacters(t *testing.T) {
	// Monday, 40 units = 5 characters per line.
	got := wrapText("abcdefghijkl", 40, core.FontMonday12)
	want := []string{"abcde", "fghij", "kl"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapTextPreservesExplicitNewlines(t *testing.T) {
	got := wrapText("alpha\n\nbeta", 800, core.FontMonday12)
	want := []string{"alpha", "", "beta"}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestWrapTextZeroWidth(t *testing.T) {
	if got := wrapText("anything", 0, core.FontMonday12); got != nil {
		t.Errorf("got %q, want nil", got)
	}
}
