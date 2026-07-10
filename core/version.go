package core

// Version is the KittyTK toolkit version and the single source of truth for
// it: bump it here (and the README) to cut a release. Anything that needs to
// report the version - About dialogs, banners, handshake strings - reads it
// from here rather than hard-coding a literal.
const Version = "0.1.0"

// Build is the per-commit build counter. Unlike Version (a hand-set release
// number), it is bumped automatically by `make increment`, whose awk script
// rewrites the single `const Build = N` line below - so keep it on its own
// line in exactly that form.
const Build = 1

// Name is the stylized product wordmark; Tagline is the recursive-acronym
// expansion of it (see the README). About dialogs and banners read the
// product identity from here so it lives in one place.
const (
	Name    = "KittyTK"
	Tagline = "image/tty Trinket Kit"
)
