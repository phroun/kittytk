// Package widgets: display-protocol registration infrastructure.
//
// Per the project architecture, each widget's own codebase registers
// its wire type and property mappings in a sibling *_protocol.go file
// (see button_protocol.go etc.); this file provides the shared helpers
// and registers the COMMON properties every widget supports.
package widgets

import (
	"fmt"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
)

// wprop adapts a widget-typed applier to protocol.PropertyApplier.
func wprop[T any](name string, fn func(ctx *protocol.BindContext, w T, v *protocol.Value, f protocol.FlagState) error) protocol.PropertyApplier {
	return func(ctx *protocol.BindContext, target any, v *protocol.Value, f protocol.FlagState) error {
		w, ok := target.(T)
		if !ok {
			return fmt.Errorf("%s: wrong target type %T", name, target)
		}
		return fn(ctx, w, v, f)
	}
}

// stringProp is the common shape: a quoted string into a setter.
func stringProp[T any](name string, set func(w T, s string)) protocol.PropertyApplier {
	return wprop(name, func(_ *protocol.BindContext, w T, v *protocol.Value, f protocol.FlagState) error {
		s, err := protocol.AsString(name, v, f)
		if err != nil {
			return err
		}
		set(w, s)
		return nil
	})
}

// boolProp is the common shape: a flag into a setter.
func boolProp[T any](name string, set func(w T, b bool)) protocol.PropertyApplier {
	return wprop(name, func(_ *protocol.BindContext, w T, v *protocol.Value, f protocol.FlagState) error {
		b, err := protocol.AsBool(name, v, f)
		if err != nil {
			return err
		}
		set(w, b)
		return nil
	})
}

// intProp is the common shape: an integer into a setter.
func intProp[T any](name string, set func(w T, n int)) protocol.PropertyApplier {
	return wprop(name, func(_ *protocol.BindContext, w T, v *protocol.Value, f protocol.FlagState) error {
		n, err := protocol.AsInt(name, v, f)
		if err != nil {
			return err
		}
		set(w, n)
		return nil
	})
}

// actionProp records the command bound to a control (action=). The
// control's activation wiring - set once in its Bind function -
// consults BindContext.Action/FireAction, so assigning or replacing
// the action never re-wires callbacks.
func actionProp(name string) protocol.PropertyApplier {
	return wprop(name, func(ctx *protocol.BindContext, w core.Widget, v *protocol.Value, f protocol.FlagState) error {
		id, err := protocol.AsWord(name, v, f)
		if err != nil {
			return err
		}
		if ctx.Dispatch == nil && ctx.Emit == nil {
			return fmt.Errorf("%s: no command dispatcher on this connection", name)
		}
		ctx.SetAction(widgetID(w), id)
		return nil
	})
}

// widgetID returns a widget's stable object identity as a wire ID.
func widgetID(w core.Widget) uint64 {
	if iw, ok := w.(interface{ ObjectID() core.ObjectID }); ok {
		return uint64(iw.ObjectID())
	}
	return 0
}

// regWidget registers a widget type whose targets are core.Widgets.
// bind, when non-nil, wires the widget's event emission into the
// connection (called once at construction).
func regWidget(name string, construct func() core.Widget, props map[string]protocol.PropertyApplier, appendFn func(parent, child core.Widget) error, bind func(ctx *protocol.BindContext, w core.Widget)) {
	spec := &protocol.TypeSpec{
		New:   func() any { return construct() },
		Props: props,
		ID: func(t any) uint64 {
			if w, ok := t.(interface{ ObjectID() core.ObjectID }); ok {
				return uint64(w.ObjectID())
			}
			return 0
		},
	}
	if appendFn != nil {
		spec.Append = func(p, c any) error {
			pw, ok1 := p.(core.Widget)
			cw, ok2 := c.(core.Widget)
			if !ok1 || !ok2 {
				return fmt.Errorf("%s: children must be widgets", name)
			}
			return appendFn(pw, cw)
		}
	}
	if bind != nil {
		spec.Bind = func(ctx *protocol.BindContext, t any) {
			if w, ok := t.(core.Widget); ok {
				bind(ctx, w)
			}
		}
	}
	protocol.RegisterType(name, spec)
}

func init() {
	protocol.RegisterCommonProperty("enabled", boolProp("enabled", core.Widget.SetEnabled))
	protocol.RegisterCommonProperty("visible", boolProp("visible", core.Widget.SetVisible))
	protocol.RegisterCommonProperty("name", stringProp("name", core.Widget.SetName))

	protocol.RegisterCommonProperty("min_width", sizeProp("min_width", true, true))
	protocol.RegisterCommonProperty("min_height", sizeProp("min_height", true, false))
	protocol.RegisterCommonProperty("max_width", sizeProp("max_width", false, true))
	protocol.RegisterCommonProperty("max_height", sizeProp("max_height", false, false))

	protocol.RegisterCommonProperty("column_units", unitsProp("column_units", true))
	protocol.RegisterCommonProperty("row_units", unitsProp("row_units", false))

	protocol.RegisterCommonProperty("font", wprop("font", func(_ *protocol.BindContext, w core.Widget, v *protocol.Value, f protocol.FlagState) error {
		s, err := protocol.AsString("font", v, f)
		if err != nil {
			return err
		}
		fnt, ok := map[string]*core.Font{
			"Monday":  core.FontMonday12,
			"Tuesday": core.FontTuesday12,
		}[s]
		if !ok {
			return fmt.Errorf("font: unknown family %q", s)
		}
		fw, ok := w.(interface{ SetFont(*core.Font) })
		if !ok {
			return fmt.Errorf("font: not supported by this type")
		}
		fw.SetFont(fnt)
		return nil
	}))

	protocol.RegisterCommonProperty("acc_name", wprop("acc_name", func(_ *protocol.BindContext, w core.Widget, v *protocol.Value, f protocol.FlagState) error {
		s, err := protocol.AsString("acc_name", v, f)
		if err != nil {
			return err
		}
		if aw, ok := w.(interface{ SetAccessibleName(string) }); ok {
			aw.SetAccessibleName(s)
			return nil
		}
		return fmt.Errorf("acc_name: not supported by this type")
	}))
}

func sizeProp(name string, min, isWidth bool) protocol.PropertyApplier {
	return wprop(name, func(_ *protocol.BindContext, w core.Widget, v *protocol.Value, f protocol.FlagState) error {
		n, err := protocol.AsInt(name, v, f)
		if err != nil {
			return err
		}
		if min {
			s := w.MinimumSize()
			if isWidth {
				s.Width = core.Unit(n)
			} else {
				s.Height = core.Unit(n)
			}
			w.SetMinimumSize(s)
		} else {
			s := w.MaximumSize()
			if isWidth {
				s.Width = core.Unit(n)
			} else {
				s.Height = core.Unit(n)
			}
			w.SetMaximumSize(s)
		}
		return nil
	})
}

func unitsProp(name string, isColumn bool) protocol.PropertyApplier {
	return wprop(name, func(_ *protocol.BindContext, w core.Widget, v *protocol.Value, f protocol.FlagState) error {
		n, err := protocol.AsInt(name, v, f)
		if err != nil {
			return err
		}
		type metriced interface {
			CellMetricsOverride() *core.CellMetrics
			EffectiveCellMetrics() core.CellMetrics
			SetCellMetrics(*core.CellMetrics)
		}
		mw, ok := w.(metriced)
		if !ok {
			return fmt.Errorf("%s: not supported by this type", name)
		}
		m := mw.EffectiveCellMetrics()
		if ov := mw.CellMetricsOverride(); ov != nil {
			m = *ov
		}
		if isColumn {
			m.CellWidth = core.Unit(n)
		} else {
			m.CellHeight = core.Unit(n)
		}
		mw.SetCellMetrics(&m)
		return nil
	})
}
