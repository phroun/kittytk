package protocol

import (
	"fmt"
)

// Object is a UI object under construction, provided by a Factory.
// The widget binder implements this against real widgets; tests use
// mocks. Set receives either a value (flag == FlagNone) or a flag
// assertion (value == nil); typed conversion and property validation
// are the binder's job.
type Object interface {
	Set(name string, value *Value, flag FlagState) error
	Append(child Object) error
	ID() uint64
}

// Factory creates objects for builtin (lowercase) type names.
type Factory interface {
	New(typeName string) (Object, error)
}

// Reply reports server-assigned IDs for a request: top-level
// correlation keys plus explicitly surfaced names (D11/D15).
type Reply struct {
	IDs map[string]uint64
}

// Session holds connection-scoped interpretation state: alias and
// template dictionaries (D10/D14). Correlation keys are request-scoped
// and live only for the duration of one Execute call (D11).
type Session struct {
	aliases   map[string]string
	templates map[string]*templateDef
}

type templateDef struct {
	base string // builtin or another template name
	args []*Arg
}

// NewSession creates an empty session.
func NewSession() *Session {
	return &Session{
		aliases:   make(map[string]string),
		templates: make(map[string]*templateDef),
	}
}

func isUpperInitial(name string) bool {
	return name != "" && name[0] >= 'A' && name[0] <= 'Z'
}

func isLowerInitial(name string) bool {
	return name != "" && (name[0] >= 'a' && name[0] <= 'z' || name[0] == '_')
}

// execState is the per-request state: the key table (hierarchical
// paths -> ids) and the reply under construction.
type execState struct {
	keys  map[string]uint64
	reply *Reply
}

// Execute runs a parsed script against the factory, applying and
// updating session state (aliases, templates) and returning the
// request's reply (top-level keys + surfaced names).
func (s *Session) Execute(script *Script, f Factory) (*Reply, error) {
	st := &execState{
		keys:  make(map[string]uint64),
		reply: &Reply{IDs: make(map[string]uint64)},
	}

	for _, stmt := range script.Statements {
		if err := s.executeTopLevel(stmt, f, st); err != nil {
			return nil, err
		}
	}
	return st.reply, nil
}

func (s *Session) executeTopLevel(stmt *Statement, f Factory, st *execState) error {
	// Surfacing reference: key=path (D15)
	if stmt.Verb == "" {
		id, ok := st.keys[stmt.Ref]
		if !ok {
			return fmt.Errorf("surfacing %s=%s: unknown key path %q", stmt.Key, stmt.Ref, stmt.Ref)
		}
		st.reply.IDs[stmt.Key] = id
		return nil
	}

	switch stmt.Verb {
	case "alias":
		if stmt.Key != "" {
			return fmt.Errorf("alias: correlation keys do not apply")
		}
		return s.declareAliases(stmt.Args)
	case "template":
		if stmt.Key != "" {
			return fmt.Errorf("template: correlation keys do not apply")
		}
		return s.declareTemplate(stmt.Args)
	case "new":
		obj, err := s.instantiate(stmt.Args, f, st, stmt.Key)
		if err != nil {
			return err
		}
		if stmt.Key != "" {
			st.keys[stmt.Key] = obj.ID()
			st.reply.IDs[stmt.Key] = obj.ID()
		}
		return nil
	default:
		return fmt.Errorf("unknown verb %q", stmt.Verb)
	}
}

// declareAliases handles `alias C="caption" ...`: targets are strings
// (lexical macros - D17 addendum), names must begin uppercase (D18).
func (s *Session) declareAliases(args []*Arg) error {
	if len(args) == 0 {
		return fmt.Errorf("alias: nothing to declare")
	}
	for _, a := range args {
		if a.Value == nil {
			return fmt.Errorf("alias %s: expected %s=\"target\"", a.Name, a.Name)
		}
		if a.Value.Kind != StringValue {
			return fmt.Errorf("alias %s: target must be a string (aliases are lexical macros)", a.Name)
		}
		if !isUpperInitial(a.Name) {
			return fmt.Errorf("alias %s: user-defined aliases must begin with an uppercase letter (D18)", a.Name)
		}
		s.aliases[a.Name] = a.Value.Str
	}
	return nil
}

// declareTemplate handles `template Name=base props...` (D14/D18).
func (s *Session) declareTemplate(args []*Arg) error {
	if len(args) == 0 || args[0].Value == nil || args[0].Value.Kind != WordValue {
		return fmt.Errorf("template: expected Name=type")
	}
	name := args[0].Name
	base := args[0].Value.Word
	if !isUpperInitial(name) {
		return fmt.Errorf("template %s: user-defined templates must begin with an uppercase letter (D18)", name)
	}
	if isUpperInitial(base) {
		if _, ok := s.templates[base]; !ok {
			return fmt.Errorf("template %s: unknown base template %q", name, base)
		}
	} else if !isLowerInitial(base) {
		return fmt.Errorf("template %s: invalid base type %q", name, base)
	}
	s.templates[name] = &templateDef{base: base, args: args[1:]}
	return nil
}

// resolveType expands a type name through the template chain (D14:
// expansion at instantiation; transitive with cycle guard), returning
// the final builtin type and the accumulated template args, base-most
// first.
func (s *Session) resolveType(typeName string) (string, []*Arg, error) {
	var chain []*templateDef
	visited := map[string]bool{}

	name := typeName
	for isUpperInitial(name) {
		if visited[name] {
			return "", nil, fmt.Errorf("template %s: cyclic template chain", typeName)
		}
		visited[name] = true
		def, ok := s.templates[name]
		if !ok {
			return "", nil, fmt.Errorf("unknown template %q", name)
		}
		chain = append(chain, def)
		name = def.base
	}
	if !isLowerInitial(name) {
		return "", nil, fmt.Errorf("invalid type %q", name)
	}

	// Accumulate args base-most first so later (more specific,
	// then instance) properties override earlier ones.
	var merged []*Arg
	for i := len(chain) - 1; i >= 0; i-- {
		merged = append(merged, chain[i].args...)
	}
	return name, merged, nil
}

// instantiate executes a `new` statement's args: the leading bare word
// is the type (builtin or template); remaining args are properties,
// flags, and children blocks. keyPath is the hierarchical key prefix
// for registering nested keys ("" when the statement is unkeyed).
func (s *Session) instantiate(args []*Arg, f Factory, st *execState, keyPath string) (Object, error) {
	if len(args) == 0 || args[0].Value != nil || args[0].Flag != FlagTrue {
		return nil, fmt.Errorf("new: expected a type name")
	}
	typeName := args[0].Name

	builtin, templateArgs, err := s.resolveType(typeName)
	if err != nil {
		return nil, err
	}

	obj, err := f.New(builtin)
	if err != nil {
		return nil, err
	}

	// Template properties first, instance properties after: later Set
	// calls override earlier ones (scalars), and children concatenate
	// in application order (template children, then instance children).
	if err := s.applyArgs(obj, templateArgs, f, st, keyPath); err != nil {
		return nil, err
	}
	if err := s.applyArgs(obj, args[1:], f, st, keyPath); err != nil {
		return nil, err
	}
	return obj, nil
}

// applyArgs applies properties, flags, and children blocks to obj.
func (s *Session) applyArgs(obj Object, args []*Arg, f Factory, st *execState, keyPath string) error {
	for _, a := range args {
		name := a.Name
		// Alias substitution (lexical, property-name position, D10/D18):
		// uppercase-initial names must be declared aliases.
		if isUpperInitial(name) {
			target, ok := s.aliases[name]
			if !ok {
				return fmt.Errorf("unknown alias %q (property names are lowercase; aliases must be declared)", name)
			}
			name = target
		}

		if name == "children" {
			if a.Value == nil || a.Value.Kind != BlockValue {
				return fmt.Errorf("children: expected a {} block")
			}
			if err := s.buildChildren(obj, a.Value.Block, f, st, keyPath); err != nil {
				return err
			}
			continue
		}

		if a.Value == nil {
			if err := obj.Set(name, nil, a.Flag); err != nil {
				return err
			}
			continue
		}
		if err := obj.Set(name, a.Value, FlagNone); err != nil {
			return err
		}
	}
	return nil
}

// buildChildren executes a children block (D13): each statement must
// be a (possibly keyed) `new`. Keys register hierarchically under the
// enclosing key path (D15); with no enclosing key they remain
// internal-only.
func (s *Session) buildChildren(parent Object, block *Script, f Factory, st *execState, keyPath string) error {
	for _, stmt := range block.Statements {
		if stmt.Verb != "new" {
			if stmt.Verb == "" {
				return fmt.Errorf("children: surfacing references are top-level statements")
			}
			return fmt.Errorf("children: only new statements allowed, found %q", stmt.Verb)
		}

		childPath := ""
		if stmt.Key != "" && keyPath != "" {
			childPath = keyPath + "." + stmt.Key
		}

		child, err := s.instantiate(stmt.Args, f, st, childPath)
		if err != nil {
			return err
		}
		if childPath != "" {
			st.keys[childPath] = child.ID()
		}
		if err := parent.Append(child); err != nil {
			return err
		}
	}
	return nil
}

// Aliases returns a copy of the session's alias table (for debugging
// and tests).
func (s *Session) Aliases() map[string]string {
	out := make(map[string]string, len(s.aliases))
	for k, v := range s.aliases {
		out[k] = v
	}
	return out
}

// HasTemplate reports whether a template is declared.
func (s *Session) HasTemplate(name string) bool {
	_, ok := s.templates[name]
	return ok
}

