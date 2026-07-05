package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/layout"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/style"
)

// Wire registration for Panel (see docs/property-vocabulary.md).
// Order matters where properties interact: set layout before spacing.
func init() {
	regWidget("panel",
		func() core.Widget { return NewPanel() },
		map[string]protocol.PropertyApplier{
			"border": boolProp("border", (*Panel).SetBorder),
			"border_style": wprop("border_style", func(_ *protocol.BindContext, p *Panel, v *protocol.Value, f protocol.FlagState) error {
				w, err := protocol.AsWord("border_style", v, f)
				if err != nil {
					return err
				}
				styles := map[string]style.BorderStyle{
					"single":  style.BorderSingle,
					"double":  style.BorderDouble,
					"rounded": style.BorderRounded,
					"heavy":   style.BorderHeavy,
					"ascii":   style.BorderASCII,
				}
				bs, ok := styles[w]
				if !ok {
					return fmt.Errorf("border_style: unknown value %q", w)
				}
				p.SetBorderStyle(bs)
				return nil
			}),
			"layout": wprop("layout", func(_ *protocol.BindContext, p *Panel, v *protocol.Value, f protocol.FlagState) error {
				w, err := protocol.AsWord("layout", v, f)
				if err != nil {
					return err
				}
				switch w {
				case "vbox":
					p.SetLayoutManager(layout.NewBoxLayout(core.Vertical))
				case "hbox":
					p.SetLayoutManager(layout.NewBoxLayout(core.Horizontal))
				case "none":
					// no layout manager
				default:
					return fmt.Errorf("layout: unknown value %q (grid arrives later)", w)
				}
				return nil
			}),
			"spacing": wprop("spacing", func(_ *protocol.BindContext, p *Panel, v *protocol.Value, f protocol.FlagState) error {
				n, err := protocol.AsInt("spacing", v, f)
				if err != nil {
					return err
				}
				lm, ok := p.LayoutManager().(interface{ SetSpacing(core.Unit) })
				if !ok {
					return fmt.Errorf("spacing: set layout before spacing")
				}
				lm.SetSpacing(core.Unit(n))
				return nil
			}),
		},
		func(parent, child core.Widget) error {
			parent.(*Panel).AddChild(child)
			return nil
		},
		nil,
	)
}
