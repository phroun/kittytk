package main

import (
	"testing"

	"github.com/phroun/tuitk/protocol"
)

// The demoapp's build scripts must be well-formed protocol so the client
// can send them over the socket.
func TestScriptsParse(t *testing.T) {
	if _, err := protocol.Parse(mainScript); err != nil {
		t.Errorf("mainScript: %v", err)
	}
	if _, err := protocol.Parse(secondaryScript(1)); err != nil {
		t.Errorf("secondaryScript: %v", err)
	}
}
