// This file ports no upstream file. It works around a defect of
// go-text/typesetting v0.3.5 in the faces of web fonts.

package font

import (
	"reflect"
	"unsafe"

	"github.com/go-text/typesetting/harfbuzz"
)

// HarfBuzz keeps a digest of the glyphs that a lookup covers: a filter that
// says that a glyph is not covered, so that the lookup is not tried. The
// digest never changes a result.
//
// The port of the digest in go-text v0.3.5 (harfbuzz/set_digest.go,
// setDigest.addRange) stops adding the ranges of a coverage table as soon
// as one of its three masks is full. HarfBuzz stops for that mask only. So
// for a coverage table with many ranges the digest says that glyphs with a
// high number are not covered when they are. The kern lookup of Recursive
// is such a table: "(a)" and ".B" get no kerning from go-text, and they get
// kerning from HarfBuzz and from Chromium.
//
// openDigests fills every digest of a HarfBuzz font of go-text, so that
// each lookup is tried for each glyph. The fields are not exported, so the
// function finds them by name. With another version of go-text that has
// other names the function changes nothing, and the tests that compare the
// widths of kerned text with Chromium then fail.
func openDigests(font *harfbuzz.Font) {
	v := reflect.ValueOf(font).Elem()
	for _, name := range [...]string{"gsubAccels", "gposAccels"} {
		accels := v.FieldByName(name)
		if !accels.IsValid() || accels.Kind() != reflect.Slice {
			continue
		}
		for i := 0; i < accels.Len(); i++ {
			accel := accels.Index(i)
			if accel.Kind() != reflect.Struct {
				continue
			}
			fillDigest(accel.FieldByName("digest"))
			subtables := accel.FieldByName("subtables")
			if !subtables.IsValid() || subtables.Kind() != reflect.Slice {
				continue
			}
			for j := 0; j < subtables.Len(); j++ {
				if sub := subtables.Index(j); sub.Kind() == reflect.Struct {
					fillDigest(sub.FieldByName("digest"))
				}
			}
		}
	}
}

// fillDigest sets the three masks of a digest to all ones.
func fillDigest(d reflect.Value) {
	if !d.IsValid() || !d.CanAddr() || d.Kind() != reflect.Array || d.Len() != 3 || d.Type().Elem().Kind() != reflect.Uint64 {
		return
	}
	masks := (*[3]uint64)(unsafe.Pointer(d.UnsafeAddr()))
	*masks = [3]uint64{^uint64(0), ^uint64(0), ^uint64(0)}
}
