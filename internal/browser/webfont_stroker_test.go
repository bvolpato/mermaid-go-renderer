package browser_test

// The tests of the web fonts (webfont_probe_test.go) measure text with a
// synthetic bold: Open Sans has no bold face. Its bounds need the stroker
// of the rasterizer, which registers itself with the font package. The
// package browser cannot import the rasterizer, and this external test
// package can: the import puts the stroker into the test program.
import _ "github.com/bvolpato/mermaid-go-renderer/internal/raster"
