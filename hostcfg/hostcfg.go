// Package hostcfg loads launch configuration for the KittyTK display
// hosts (kittytk-sdl, kittytk-tui) from a plain kittytk.ini, so a
// non-technical user can configure the app by editing a text file
// instead of passing command-line arguments.
//
// The first kittytk.ini found in this order wins (whole file; later
// locations are a fallback, not merged):
//
//  1. the current working directory
//  2. the directory holding the executable
//  3. the user config dir (%APPDATA%\kittytk on Windows, else
//     $XDG_CONFIG_HOME/kittytk or ~/.config/kittytk)
//
// The file is section-tolerant but keys are matched by name, so section
// headers are cosmetic and a user who omits them still gets a working
// config:
//
//	[window]
//	title  = KittyTK
//	width  = 1024
//	height = 768
//	scale  = 2
//
//	[service]
//	endpoint =            ; blank = default; tcp://host:port, tls://…, or a socket path
//	token    =            ; optional shared secret
//
// Environment variables still take precedence over the file: KITTYTK_DISPLAY
// for the endpoint and KITTYTK_TOKEN for the token.
package hostcfg

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/phroun/kittytk/client"
)

// IniName is the configuration file basename both hosts look for.
const IniName = "kittytk.ini"

// Config is the resolved launch configuration. Window fields apply only
// to graphical hosts (kittytk-sdl); the terminal host ignores them.
type Config struct {
	Title  string // window title bar text
	Width  int    // window width in pixels
	Height int    // window height in pixels
	Scale  int    // pixels per abstract unit (1 = small, 2 = crisp/large)

	Endpoint string // service endpoint ("" = the conventional default)
	Token    string // optional shared secret

	// Source is the path of the ini that was loaded, or "" if none was
	// found (defaults were used).
	Source string
}

// Defaults returns the built-in configuration used when no ini is found
// (and as the base every ini is applied onto).
func Defaults() Config {
	return Config{Title: "KittyTK", Width: 1024, Height: 768, Scale: 2}
}

// SearchPaths returns the ordered candidate ini paths (see the package
// doc). Unreadable directories are simply skipped.
func SearchPaths() []string {
	var ps []string
	if wd, err := os.Getwd(); err == nil {
		ps = append(ps, filepath.Join(wd, IniName))
	}
	if exe, err := os.Executable(); err == nil {
		ps = append(ps, filepath.Join(filepath.Dir(exe), IniName))
	}
	ps = append(ps, filepath.Join(client.ConfigDir(), IniName))
	return ps
}

// Load returns the configuration from the first readable kittytk.ini in
// SearchPaths (whole file wins), or Defaults() if none is found.
func Load() Config {
	cfg := Defaults()
	for _, p := range SearchPaths() {
		data, err := os.ReadFile(p)
		if err != nil {
			continue
		}
		apply(data, &cfg)
		cfg.Source = p
		break // first found wins
	}
	return cfg
}

// apply parses ini text and sets the recognized keys on cfg. Section
// headers are tolerated but ignored; keys are matched by name (case-
// insensitive). Unknown keys and malformed numbers are skipped so a
// stray typo never prevents the host from starting.
func apply(data []byte, cfg *Config) {
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(strings.TrimRight(raw, "\r"))
		if line == "" || line[0] == ';' || line[0] == '#' || line[0] == '[' {
			continue
		}
		eq := strings.IndexByte(line, '=')
		if eq < 0 {
			continue
		}
		key := strings.ToLower(strings.TrimSpace(line[:eq]))
		val := strings.TrimSpace(stripInlineComment(line[eq+1:]))
		switch key {
		case "title":
			cfg.Title = val
		case "width":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Width = n
			}
		case "height":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Height = n
			}
		case "scale":
			if n, err := strconv.Atoi(val); err == nil && n > 0 {
				cfg.Scale = n
			}
		case "endpoint":
			cfg.Endpoint = val
		case "token":
			cfg.Token = val
		}
	}
}

// stripInlineComment removes a trailing `;`/`#` comment from a value. A
// comment starts only where the marker begins the value or follows
// whitespace, so a value that itself contains ';' or '#' with no leading
// space (a token like "a;b", a "#rrggbb" is not a hostcfg value) is kept.
func stripInlineComment(v string) string {
	for i := 0; i < len(v); i++ {
		if c := v[i]; c == ';' || c == '#' {
			if i == 0 || v[i-1] == ' ' || v[i-1] == '\t' {
				return v[:i]
			}
		}
	}
	return v
}

// ResolveEndpoint returns the endpoint to serve on: $KITTYTK_DISPLAY if
// set (env wins), else the ini's endpoint, else the conventional default.
func (c Config) ResolveEndpoint() string {
	if os.Getenv(client.DisplayEnv) != "" {
		return client.DefaultEndpoint() // honors the env var itself
	}
	if c.Endpoint != "" {
		return c.Endpoint
	}
	return client.DefaultEndpoint()
}

// ResolveToken returns the shared secret: $KITTYTK_TOKEN if set (env
// wins), else the ini's token.
func (c Config) ResolveToken() string {
	if t := os.Getenv(client.TokenEnv); t != "" {
		return t
	}
	return c.Token
}
