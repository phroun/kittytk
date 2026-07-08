package widgets

import (
	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// Wire registration for the spacer widget (see docs/property-vocabulary.md).
func init() {
	regWidget("spacer",
		func() core.Widget { return NewSpacer() },
		map[string]protocol.PropertyApplier{
			"width": wprop("width", func(_ *protocol.BindContext, s *Spacer, v *protocol.Value, f protocol.FlagState) error {
				n, err := protocol.AsInt("width", v, f)
				if err != nil {
					return err
				}
				size := s.Size()
				size.Width = core.Unit(n)
				s.SetSize(size)
				return nil
			}),
			"height": wprop("height", func(_ *protocol.BindContext, s *Spacer, v *protocol.Value, f protocol.FlagState) error {
				n, err := protocol.AsInt("height", v, f)
				if err != nil {
					return err
				}
				size := s.Size()
				size.Height = core.Unit(n)
				s.SetSize(size)
				return nil
			}),
		},
		nil,
		nil,
	)
}
