// This file ports no upstream file.

package render

// The rasterizer has the port of the stroker of Skia. Its package registers
// the stroker with the font package (font.RegisterGlyphStroker), which
// needs it for the bounds of a glyph with a synthetic bold: a text in a web
// font that has no bold face, in the page of mermaid-cli. The import makes
// every render have the stroker, also in a program that writes no PNG.
import _ "github.com/bvolpato/mermaid-go-renderer/internal/raster"
