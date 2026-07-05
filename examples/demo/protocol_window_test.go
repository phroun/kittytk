package main

import (
	"testing"

	"github.com/phroun/tuitk/core"
	"github.com/phroun/tuitk/protocol"
	"github.com/phroun/tuitk/widgets"
)

// The Protocol Demo window's script must execute cleanly and surface
// every key createProtocolWindow depends on. This guards the demo
// against silently dropping the window (it returns nil on error).
func TestProtocolWindowScriptBuilds(t *testing.T) {
	// action= requires a command dispatcher on the connection, just
	// like the real demo wiring provides.
	ctx := &protocol.BindContext{Dispatch: func(string) {}}
	factory := &idCaptureFactory{
		inner: protocol.NewRegistryFactory(ctx),
		byID:  make(map[uint64]any),
	}

	script, err := protocol.Parse(protocolWindowScript)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	reply, err := protocol.NewSession().Execute(script, factory)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}

	for _, key := range []string{"root", "watch", "wcb", "winp", "wcombo"} {
		if _, ok := reply.IDs[key]; !ok {
			t.Errorf("reply missing surfaced key %q", key)
		}
	}

	if _, ok := factory.byID[reply.IDs["root"]].(core.Widget); !ok {
		t.Errorf("root is %T, want core.Widget", factory.byID[reply.IDs["root"]])
	}
	if _, ok := factory.byID[reply.IDs["watch"]].(*widgets.Label); !ok {
		t.Errorf("watch is %T, want *widgets.Label", factory.byID[reply.IDs["watch"]])
	}
	if _, ok := factory.byID[reply.IDs["wcb"]].(*widgets.Checkbox); !ok {
		t.Errorf("wcb is %T, want *widgets.Checkbox", factory.byID[reply.IDs["wcb"]])
	}
	if _, ok := factory.byID[reply.IDs["winp"]].(*widgets.TextInput); !ok {
		t.Errorf("winp is %T, want *widgets.TextInput", factory.byID[reply.IDs["winp"]])
	}
	if _, ok := factory.byID[reply.IDs["wcombo"]].(*widgets.ComboBox); !ok {
		t.Errorf("wcombo is %T, want *widgets.ComboBox", factory.byID[reply.IDs["wcombo"]])
	}
}
