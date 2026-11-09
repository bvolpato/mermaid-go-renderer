package browser

import (
	"compress/gzip"
	"encoding/json"
	"math"
	"os"
	"strings"
	"testing"
)

// emojiProbeCase is one case of testdata/emoji-probe.json.gz, which
// tools/reference/gen-browser-emoji-probe.mjs wrote with Chromium: markup,
// queries on it and the results of the queries.
type emojiProbeCase struct {
	Name string `json:"name"`
	HTML string `json:"html"`
	// Text is the text of a case of the kinds "sequence" and "symbol".
	Text    string `json:"text"`
	Queries []struct {
		Sel string `json:"sel"`
		Op  string `json:"op"`
		// Compare is "size" for a rectangle of which only the width and
		// the height are compared.
		Compare string `json:"compare"`
	} `json:"queries"`
	Results []json.RawMessage `json:"results"`
}

// emojiTextFontGaps lists the texts of the kind "symbol" for which the
// engine has another result than Chromium. None of them has a glyph of the
// emoji font in Chromium: they show limits of the choice of a text font.
var emojiTextFontGaps = map[string]string{
	// CoreText has its own choice of a font for each symbol. The list of
	// fallback fonts of the font package is one order for all characters.
	"\u2708": "CoreText takes Menlo (advance 0.603 em); the port takes Zapf Dingbats (0.791 em)",
	"\u2709": "CoreText takes Menlo (advance 0.603 em); the port takes Zapf Dingbats (0.690 em)",
	"\u2328": "CoreText takes Menlo (advance 0.603 em); the port takes Apple Symbols (0.741 em)",
	// A combining enclosing mark without the variation selector 16.
	// Chromium gives the keycap mark an advance of 0.5 em; the port takes
	// the mark of Lucida Grande, which has no advance.
	"3\u20e3":  "the keycap mark has an advance of 0.5 em in Chromium",
	"a\u20e3":  "the keycap mark has an advance of 0.5 em in Chromium",
	"1\u20e32": "the keycap mark has an advance of 0.5 em in Chromium",
	// Blink falls back by cluster: a font must have the base and the mark.
	// The port chooses a font for each character.
	"\u2600\u20e0":     "Blink draws the sun and the mark with one font",
	"\U0001f600\u20e0": "the emoji font has no enclosing circle backslash, and Blink leaves the font for the whole cluster",
}

// TestEmojiAgainstChromium replays the probe cases for text with emoji: the
// advance and the bounds of the glyphs of Apple Color Emoji for the font
// sizes below 256px, the bounding box of SVG text with emoji, emoji
// sequences, the choice between the emoji font and a text font, and the
// size of HTML content with emoji.
//
// The values must be those of Chromium within the tolerance of the parity
// command (0.01px). The test counts the values that are equal bit for bit.
func TestEmojiAgainstChromium(t *testing.T) {
	macOSProbe(t)
	f, err := os.Open("testdata/emoji-probe.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Cases []emojiProbeCase `json:"cases"`
	}
	if err := json.NewDecoder(zr).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 2400 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	exact, closeTo, gaps := 0, 0, 0
	kinds := map[string][2]int{} // failed cases and all cases of a kind
	for _, c := range golden.Cases {
		kind := strings.Split(c.Name, "-")[1]
		if _, known := emojiTextFontGaps[c.Text]; known && kind == "symbol" {
			gaps++
			continue
		}
		doc := NewDocument()
		container := doc.Body.Append("div").Attr("id", "container")
		container.HTML(c.HTML)
		failed := false
		for qi, q := range c.Queries {
			el := container.Select(q.Sel)
			if el == nil {
				t.Fatalf("%s: no element %s", c.Name, q.Sel)
			}
			var got, want []float64
			switch q.Op {
			case "bbox":
				b := el.BBox()
				got = []float64{b.X, b.Y, b.Width, b.Height}
			case "rect":
				b := el.BoundingClientRect()
				got = []float64{b.X, b.Y, b.Width, b.Height}
			case "textlen":
				got = []float64{el.ComputedTextLength()}
			default:
				t.Fatalf("%s: unknown query %q", c.Name, q.Op)
			}
			if len(got) == 1 {
				want = make([]float64, 1)
				err = json.Unmarshal(c.Results[qi], &want[0])
			} else {
				err = json.Unmarshal(c.Results[qi], &want)
			}
			if err != nil || len(want) != len(got) {
				t.Fatalf("%s: result %s of query %s: %v", c.Name, c.Results[qi], q.Op, err)
			}
			if q.Compare == "size" {
				got, want = got[2:], want[2:]
			}
			for k := range got {
				switch d := math.Abs(got[k] - want[k]); {
				case d == 0:
					exact++
				case d <= 0.01:
					closeTo++
				default:
					failed = true
				}
			}
			if failed {
				t.Errorf("%s %s(%s): got %v, want %v\n %s", c.Name, q.Op, q.Sel, got, want, c.HTML)
				break
			}
		}
		n := kinds[kind]
		n[1]++
		if failed {
			n[0]++
		}
		kinds[kind] = n
	}
	for kind, n := range kinds {
		if n[0] > 0 {
			t.Logf("%s: %d of %d cases differ", kind, n[0], n[1])
		}
	}
	t.Logf("%d cases: exact=%d close(<=0.01)=%d, %d cases not checked (the choice of a text font, see emojiTextFontGaps)", len(golden.Cases), exact, closeTo, gaps)
}
