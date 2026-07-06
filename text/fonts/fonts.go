// Package fonts embeds the toolkit's default graphical typefaces:
// Noto Sans (UI text) and Noto Sans Mono (monospace), chosen for
// their broad Unicode coverage. Both are licensed under the SIL Open
// Font License 1.1 (see OFL.txt).
package fonts

import _ "embed"

//go:embed NotoSans-Regular.ttf
var SansRegular []byte

//go:embed NotoSans-Bold.ttf
var SansBold []byte

//go:embed NotoSans-Italic.ttf
var SansItalic []byte

//go:embed NotoSans-BoldItalic.ttf
var SansBoldItalic []byte

//go:embed NotoSansMono-Regular.ttf
var MonoRegular []byte

//go:embed NotoSansMono-Bold.ttf
var MonoBold []byte
