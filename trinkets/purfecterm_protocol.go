package trinkets

import (
	"fmt"

	"github.com/phroun/kittytk/core"
	"github.com/phroun/kittytk/protocol"
)

// Wire registration for the PurfecTerm terminal surface.
//
// feed= is the streaming pseudo-property: every application writes
// APPENDS bytes to the terminal - it is a channel, not state, and is
// never read back:
//
//	term=new terminal
//	set term feed="\e[1mhello\e[0m\r\n"
//
// Arbitrary bytes travel via the \xNN string escape (and \e for ESC);
// the O6 bulk frame arrives with the transport phase as a more
// efficient encoding of the same statement.
//
// shell is an in-process convenience: it starts the trinket's own
// local shell (what the demo uses). Under the display-protocol split
// the PTY belongs to the APP, which pumps bytes through feed= - the
// flag exists because in-process apps may still want the shortcut.
func init() {
	regTrinket("terminal",
		func() core.Trinket { return NewPurfecTerm() },
		map[string]protocol.Property{
			"feed": protocol.NewProperty("stream", wprop("feed", func(_ *protocol.BindContext, t *PurfecTerm, v *protocol.Value, f protocol.FlagState) error {
				s, err := protocol.AsString("feed", v, f)
				if err != nil {
					return err
				}
				// Display direction: parsed into the screen buffer
				// like program output. (Terminal.Write would be
				// keyboard input to the child process - and silently
				// dropped with no PTY running.)
				t.Feed([]byte(s))
				return nil
			})).Tip("Append bytes to the terminal display"),
			"shell": protocol.NewProperty("flag", wprop("shell", func(_ *protocol.BindContext, t *PurfecTerm, v *protocol.Value, f protocol.FlagState) error {
				b, err := protocol.AsBool("shell", v, f)
				if err != nil {
					return err
				}
				if b {
					if err := t.Start(); err != nil {
						return fmt.Errorf("shell: %w", err)
					}
				}
				return nil
			})).Tip("Start the built-in local shell").Def("false"),
			// font / font-size pick the monospace face and point size the
			// terminal's cell grid derives from on graphical targets. Text
			// mode ignores them (cells are cells).
			"font": protocol.NewProperty("string", wprop("font", func(_ *protocol.BindContext, t *PurfecTerm, v *protocol.Value, f protocol.FlagState) error {
				s, err := protocol.AsString("font", v, f)
				if err != nil {
					return err
				}
				t.SetTerminalFontFamily(s)
				return nil
			})).Tip("Monospace font family for the grid").Def("Monday"),
			"font_size": protocol.NewProperty("int", wprop("font_size", func(_ *protocol.BindContext, t *PurfecTerm, v *protocol.Value, f protocol.FlagState) error {
				pt, err := protocol.AsInt("font_size", v, f)
				if err != nil {
					return err
				}
				t.SetTerminalFontSize(pt)
				return nil
			})).Tip("Font point size for the grid").Def("12"),
		},
		nil,
		nil,
	)
}
