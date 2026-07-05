package protocol

import (
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
)

// This file is the type/property registry behind the wire vocabulary.
// Per the owner's architecture: each widget's own codebase registers
// its type and property mappings (widgets import protocol; protocol
// imports nothing of tuitk). The registry is write-at-init,
// read-at-execute.

// BindContext carries per-connection services into property appliers.
// It is instance-scoped, never global (multi-display guardrail): one
// app may hold several connections, each with its own context.
type BindContext struct {
	// Dispatch delivers an activated command ID (action= wiring).
	// Nil when the connection has no command sink; appliers that
	// need it must error clearly.
	Dispatch func(commandID string)
}

// PropertyApplier applies one wire property to a target object,
// performing D17-typed conversion (see AsString and friends).
type PropertyApplier func(ctx *BindContext, target any, v *Value, flag FlagState) error

// TypeSpec describes a registered builtin type.
type TypeSpec struct {
	// New constructs the target (a widget, or a virtual item's record).
	New func() any

	// Props maps property names to appliers. Type-specific properties
	// take precedence over common ones.
	Props map[string]PropertyApplier

	// Append attaches a constructed child target to a parent target
	// (children blocks, D13). Nil means the type takes no children.
	Append func(parent, child any) error

	// ID returns the target's stable object identity. Nil for Virtual
	// types, which get factory-assigned virtual IDs.
	ID func(target any) uint64

	// Virtual marks pseudo-object types (e.g. combobox items): they
	// skip common properties and widget identity.
	Virtual bool
}

var (
	regMu     sync.RWMutex
	regTypes  = map[string]*TypeSpec{}
	regCommon = map[string]PropertyApplier{}
)

// RegisterType registers a builtin type. Builtin names begin lowercase
// (D18). Panics on programmer error (duplicate, bad spec) - callers
// are init functions.
func RegisterType(name string, spec *TypeSpec) {
	if !isLowerInitial(name) {
		panic(fmt.Sprintf("protocol: builtin type %q must begin lowercase (D18)", name))
	}
	if spec == nil || spec.New == nil {
		panic(fmt.Sprintf("protocol: type %q: spec.New is required", name))
	}
	if !spec.Virtual && spec.ID == nil {
		panic(fmt.Sprintf("protocol: type %q: spec.ID is required for non-virtual types", name))
	}
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := regTypes[name]; dup {
		panic(fmt.Sprintf("protocol: type %q registered twice", name))
	}
	regTypes[name] = spec
}

// RegisterCommonProperty registers a property available on every
// non-virtual type (enabled, visible, font, ...).
func RegisterCommonProperty(name string, apply PropertyApplier) {
	regMu.Lock()
	defer regMu.Unlock()
	if _, dup := regCommon[name]; dup {
		panic(fmt.Sprintf("protocol: common property %q registered twice", name))
	}
	regCommon[name] = apply
}

// RegisteredTypes returns the sorted names of registered types.
func RegisteredTypes() []string {
	regMu.RLock()
	defer regMu.RUnlock()
	names := make([]string, 0, len(regTypes))
	for n := range regTypes {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

var virtualIDCounter atomic.Uint64

// RegistryFactory implements Factory over the registered types, bound
// to one connection's BindContext.
type RegistryFactory struct {
	ctx *BindContext
}

// NewRegistryFactory creates a factory for one connection.
func NewRegistryFactory(ctx *BindContext) *RegistryFactory {
	if ctx == nil {
		ctx = &BindContext{}
	}
	return &RegistryFactory{ctx: ctx}
}

// New implements Factory.
func (f *RegistryFactory) New(typeName string) (Object, error) {
	regMu.RLock()
	spec := regTypes[typeName]
	regMu.RUnlock()
	if spec == nil {
		return nil, fmt.Errorf("unknown widget type %q", typeName)
	}
	o := &registryObject{ctx: f.ctx, spec: spec, target: spec.New()}
	if spec.Virtual {
		o.virtualID = virtualIDCounter.Add(1)
	}
	return o, nil
}

type registryObject struct {
	ctx       *BindContext
	spec      *TypeSpec
	target    any
	virtualID uint64
}

// Target exposes the constructed object (the widget) so the embedding
// application can, e.g., set a built tree as window content.
func (o *registryObject) Target() any { return o.target }

// Set implements Object.
func (o *registryObject) Set(name string, v *Value, flag FlagState) error {
	if apply, ok := o.spec.Props[name]; ok {
		return apply(o.ctx, o.target, v, flag)
	}
	if !o.spec.Virtual {
		regMu.RLock()
		apply, ok := regCommon[name]
		regMu.RUnlock()
		if ok {
			return apply(o.ctx, o.target, v, flag)
		}
	}
	return fmt.Errorf("property %q is not supported by this type", name)
}

// Append implements Object.
func (o *registryObject) Append(child Object) error {
	if o.spec.Append == nil {
		return fmt.Errorf("this type does not accept children")
	}
	c, ok := child.(*registryObject)
	if !ok {
		return fmt.Errorf("cannot append foreign object")
	}
	return o.spec.Append(o.target, c.target)
}

// ID implements Object.
func (o *registryObject) ID() uint64 {
	if o.spec.Virtual || o.spec.ID == nil {
		return o.virtualID
	}
	return o.spec.ID(o.target)
}

// --- D17 typed-conversion helpers for property appliers ---

// AsString requires a quoted string value.
func AsString(name string, v *Value, flag FlagState) (string, error) {
	if flag != FlagNone || v == nil || v.Kind != StringValue {
		return "", fmt.Errorf("%s: expected a quoted string", name)
	}
	return v.Str, nil
}

// AsWord requires a bare word (enum or identifier).
func AsWord(name string, v *Value, flag FlagState) (string, error) {
	if flag != FlagNone || v == nil || v.Kind != WordValue {
		return "", fmt.Errorf("%s: expected a bare word", name)
	}
	return v.Word, nil
}

// AsInt requires an integer-valued numeric.
func AsInt(name string, v *Value, flag FlagState) (int, error) {
	if flag != FlagNone || v == nil || v.Kind != NumberValue || !v.IsInt {
		return 0, fmt.Errorf("%s: expected an integer", name)
	}
	return int(v.Number), nil
}

// AsFloat requires a numeric (int or float).
func AsFloat(name string, v *Value, flag FlagState) (float64, error) {
	if flag != FlagNone || v == nil || v.Kind != NumberValue {
		return 0, fmt.Errorf("%s: expected a number", name)
	}
	return v.Number, nil
}

// AsBool accepts flag form (canonical) or the true/false long form
// (D12); asserted-indeterminate is rejected - properties for which
// indeterminate is meaningful read the FlagState directly (D16).
func AsBool(name string, v *Value, flag FlagState) (bool, error) {
	switch flag {
	case FlagTrue:
		return true, nil
	case FlagFalse:
		return false, nil
	case FlagIndeterminate:
		return false, fmt.Errorf("%s: indeterminate is not meaningful for this property", name)
	}
	if v != nil && v.Kind == WordValue {
		switch v.Word {
		case "true":
			return true, nil
		case "false":
			return false, nil
		}
	}
	return false, fmt.Errorf("%s: expected a flag", name)
}
