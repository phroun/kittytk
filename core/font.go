// Package core provides fundamental types for the TUI toolkit.
package core

import "github.com/phroun/tuitk/style"

// FontStyle represents text styling attributes that can be combined.
type FontStyle uint16

const (
	// FontStyleNormal is the default style with no modifications.
	FontStyleNormal FontStyle = 0

	// FontStyleDim reduces the intensity of the text.
	FontStyleDim FontStyle = 1 << iota

	// FontStyleBright increases the intensity of the text.
	FontStyleBright

	// FontStyleBold makes the text bold/heavy.
	FontStyleBold

	// FontStyleItalic makes the text italic/oblique.
	FontStyleItalic

	// FontStyleUnderline adds an underline to the text.
	FontStyleUnderline

	// FontStyleStrikeThru adds a line through the text.
	FontStyleStrikeThru
)

// FontColor represents a color that can be default (inherit from scheme) or explicit.
type FontColor struct {
	IsDefault bool
	Color     style.Color
}

// DefaultFontColor returns a FontColor that uses the default/inherited color.
func DefaultFontColor() FontColor {
	return FontColor{IsDefault: true}
}

// ExplicitFontColor returns a FontColor with an explicit color value.
func ExplicitFontColor(c style.Color) FontColor {
	return FontColor{IsDefault: false, Color: c}
}

// Font represents a typeface with styling information.
// Fonts provide metrics for text measurement in units and control text attributes.
// Use MeasureText or MeasureRunes to determine the width of text in units.
type Font struct {
	// Name identifies the font family.
	// Built-in fonts: "Monday" (standard width), "Tuesday" (double width)
	Name string

	// Style contains styling flags (bold, italic, underline, etc.)
	Style FontStyle

	// Size is the point size (e.g., 12 for 12pt)
	Size int

	// Foreground is the text color (default = inherit from color scheme)
	Foreground FontColor

	// Background is the background color (default = inherit from color scheme)
	Background FontColor
}

// Predefined fonts
var (
	// FontMonday12 is the standard font (8 units per character).
	FontMonday12 = &Font{Name: "Monday", Size: 12}

	// FontTuesday12 is the wide font (16 units per character).
	FontTuesday12 = &Font{Name: "Tuesday", Size: 12}
)

// DefaultFont returns the default font (Monday 12pt).
func DefaultFont() *Font {
	return FontMonday12
}

// LineHeight returns the height of a line of text in units.
func (f *Font) LineHeight() Unit {
	// All current fonts are 16 units tall
	return 16
}

// baseCharWidth returns the internal unit width per character for this font.
// This is private to prevent leaking implementation details.
func (f *Font) baseCharWidth() Unit {
	if f == nil || f.Name == "" || f.Name == "Monday" {
		return 8
	}
	if f.Name == "Tuesday" {
		return 16
	}
	// Default to Monday width for unknown fonts
	return 8
}

// MeasureText returns the width in units needed to display the given text.
// This accounts for the font's metrics and handles special characters like CJK.
func (f *Font) MeasureText(text string) Unit {
	if f == nil {
		f = DefaultFont()
	}

	charWidth := f.baseCharWidth()
	total := Unit(0)

	for _, ch := range text {
		// Add width for each rune
		total += charWidth

		// Wide characters (CJK, etc.) need additional width
		if isWideChar(ch) {
			total += 8
		}
	}

	return total
}

// MeasureRunes returns the width in units for a given number of runes.
// This is useful when you already know the rune count and characters are standard width.
func (f *Font) MeasureRunes(runeCount int) Unit {
	if f == nil {
		f = DefaultFont()
	}
	return Unit(runeCount) * f.baseCharWidth()
}

// HasStyle returns true if the font has the given style flag set.
func (f *Font) HasStyle(s FontStyle) bool {
	if f == nil {
		return false
	}
	return f.Style&s != 0
}

// WithStyle returns a copy of the font with additional style flags.
func (f *Font) WithStyle(s FontStyle) *Font {
	if f == nil {
		return &Font{Name: "Monday", Size: 12, Style: s}
	}
	copy := *f
	copy.Style |= s
	return &copy
}

// WithForeground returns a copy of the font with the specified foreground color.
func (f *Font) WithForeground(c FontColor) *Font {
	if f == nil {
		return &Font{Name: "Monday", Size: 12, Foreground: c}
	}
	copy := *f
	copy.Foreground = c
	return &copy
}

// WithBackground returns a copy of the font with the specified background color.
func (f *Font) WithBackground(c FontColor) *Font {
	if f == nil {
		return &Font{Name: "Monday", Size: 12, Background: c}
	}
	copy := *f
	copy.Background = c
	return &copy
}

// isWideChar returns true if the character is a wide character (CJK, emoji, etc.)
func isWideChar(ch rune) bool {
	// CJK Unified Ideographs
	if ch >= 0x4E00 && ch <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if ch >= 0x3400 && ch <= 0x4DBF {
		return true
	}
	// CJK Unified Ideographs Extension B
	if ch >= 0x20000 && ch <= 0x2A6DF {
		return true
	}
	// CJK Compatibility Ideographs
	if ch >= 0xF900 && ch <= 0xFAFF {
		return true
	}
	// Hangul Syllables
	if ch >= 0xAC00 && ch <= 0xD7AF {
		return true
	}
	// Hiragana
	if ch >= 0x3040 && ch <= 0x309F {
		return true
	}
	// Katakana
	if ch >= 0x30A0 && ch <= 0x30FF {
		return true
	}
	// Fullwidth Forms
	if ch >= 0xFF00 && ch <= 0xFFEF {
		return true
	}
	return false
}

// FontProvider is implemented by widgets that can provide a font.
type FontProvider interface {
	// Font returns the font set on this provider, or nil if not set.
	Font() *Font
}

// FindEffectiveFont walks up the widget tree to find the effective font.
// It checks the widget, then its parent window, then the desktop/MDI pane.
// Returns DefaultFont() if no font is set anywhere in the chain.
func FindEffectiveFont(w Widget) *Font {
	if w == nil {
		return DefaultFont()
	}

	// Check if the widget itself has a font
	if fp, ok := w.(FontProvider); ok {
		if f := fp.Font(); f != nil {
			return f
		}
	}

	// Walk up the parent chain
	current := w.Parent()
	for current != nil {
		if fp, ok := current.(FontProvider); ok {
			if f := fp.Font(); f != nil {
				return f
			}
		}
		if widget, ok := current.(Widget); ok {
			current = widget.Parent()
		} else {
			break
		}
	}

	return DefaultFont()
}
