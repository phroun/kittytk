package trinkets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for Splitter (see docs/property-vocabulary.md).
// The first child becomes the first pane, the second the second pane;
// a third is an error.
func init() {
	regTrinket("splitter",
		func() core.Trinket { return NewVSplitter() },
		map[string]protocol.PropertyApplier{
			"orientation": wprop("orientation", func(_ *protocol.BindContext, s *Splitter, v *protocol.Value, f protocol.FlagState) error {
				w, err := protocol.AsWord("orientation", v, f)
				if err != nil {
					return err
				}
				switch w {
				case "horizontal":
					s.SetOrientation(core.Horizontal)
				case "vertical":
					s.SetOrientation(core.Vertical)
				default:
					return fmt.Errorf("orientation: unknown value %q", w)
				}
				return nil
			}),
			"position": wprop("position", func(_ *protocol.BindContext, s *Splitter, v *protocol.Value, f protocol.FlagState) error {
				pos, err := protocol.AsFloat("position", v, f)
				if err != nil {
					return err
				}
				s.SetPosition(pos)
				return nil
			}),
			"caption": stringProp("caption", (*Splitter).SetTitle),
		},
		func(parent, child core.Trinket) error {
			s := parent.(*Splitter)
			switch {
			case s.First() == nil:
				s.SetFirst(child)
			case s.Second() == nil:
				s.SetSecond(child)
			default:
				return fmt.Errorf("splitter: already has two panes")
			}
			return nil
		},
		nil,
	)
}
