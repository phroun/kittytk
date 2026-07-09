package hostcfg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/phroun/kittytk/client"
)

// apply parses recognized keys onto a Config, tolerates section headers
// and comments, and leaves defaults for absent/typo'd keys.
func TestApplyParsesKnownKeys(t *testing.T) {
	cfg := Defaults()
	apply([]byte(`
# a comment
; another
[window]
title = My Desk
width = 800
height = 600
scale = 1

[service]
endpoint = tcp://0.0.0.0:9797
token = s3cret
bogus = ignored
scale = notanumber
`), &cfg)

	if cfg.Title != "My Desk" || cfg.Width != 800 || cfg.Height != 600 || cfg.Scale != 1 {
		t.Errorf("window: %+v", cfg)
	}
	if cfg.Endpoint != "tcp://0.0.0.0:9797" || cfg.Token != "s3cret" {
		t.Errorf("service: endpoint=%q token=%q", cfg.Endpoint, cfg.Token)
	}
}

// A malformed number leaves the default rather than zeroing the field.
func TestApplyKeepsDefaultOnBadNumber(t *testing.T) {
	cfg := Defaults()
	apply([]byte("scale = oops\nwidth = -5\n"), &cfg)
	if cfg.Scale != Defaults().Scale || cfg.Width != Defaults().Width {
		t.Errorf("bad numbers should keep defaults: scale=%d width=%d", cfg.Scale, cfg.Width)
	}
}

// Section headers are optional: keys are matched by name.
func TestApplyIgnoresSections(t *testing.T) {
	cfg := Defaults()
	apply([]byte("title = No Sections Here\nscale = 3\n"), &cfg)
	if cfg.Title != "No Sections Here" || cfg.Scale != 3 {
		t.Errorf("sectionless keys should apply: %+v", cfg)
	}
}

// Load uses the first kittytk.ini found; the current directory is searched
// before the exe dir and the user config dir.
func TestLoadFirstFoundWinsFromCWD(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, IniName), []byte("title = FromCWD\nwidth = 640\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg := Load()
	if cfg.Title != "FromCWD" || cfg.Width != 640 {
		t.Errorf("expected CWD ini to win: %+v", cfg)
	}
	if cfg.Source != filepath.Join(dir, IniName) {
		t.Errorf("Source = %q, want the CWD ini", cfg.Source)
	}
}

// With no ini anywhere, Load returns the built-in defaults.
func TestLoadDefaultsWhenNoIni(t *testing.T) {
	empty := t.TempDir()
	t.Chdir(empty)
	// Point the user config dir at another empty dir so no stray ini is found.
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("APPDATA", t.TempDir())

	cfg := Load()
	if cfg != Defaults() {
		t.Errorf("no ini should yield defaults, got %+v", cfg)
	}
}

// Environment variables win over the ini for endpoint and token.
func TestResolveEnvOverrides(t *testing.T) {
	cfg := Config{Endpoint: "tcp://ini:1", Token: "initoken"}

	t.Setenv(client.DisplayEnv, "tcp://env:2")
	if got := cfg.ResolveEndpoint(); got != "tcp://env:2" {
		t.Errorf("endpoint env should win: %q", got)
	}
	t.Setenv(client.TokenEnv, "envtoken")
	if got := cfg.ResolveToken(); got != "envtoken" {
		t.Errorf("token env should win: %q", got)
	}
}

// Without the env vars, the ini's values are used (and blank endpoint
// falls back to the conventional default).
func TestResolveFallsBackToIniAndDefault(t *testing.T) {
	t.Setenv(client.DisplayEnv, "")
	t.Setenv(client.TokenEnv, "")

	ini := Config{Endpoint: "tcp://ini:1", Token: "initoken"}
	if got := ini.ResolveEndpoint(); got != "tcp://ini:1" {
		t.Errorf("ini endpoint should be used: %q", got)
	}
	if got := ini.ResolveToken(); got != "initoken" {
		t.Errorf("ini token should be used: %q", got)
	}

	blank := Config{}
	if got := blank.ResolveEndpoint(); got != client.DefaultEndpoint() {
		t.Errorf("blank endpoint should fall back to default: %q", got)
	}
}
