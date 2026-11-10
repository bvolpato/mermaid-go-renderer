package mmdc

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"io"
	"io/fs"
	"os"
	"reflect"
	"regexp"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/webfonts"
)

// TestTokenizeCSS compares the port of tokenizeCSS.js with what the
// JavaScript returns for many texts (testdata/tokenizecss.json, written by
// tools/reference/gen-mmdc-tokenize.mjs).
func TestTokenizeCSS(t *testing.T) {
	data, err := os.ReadFile("testdata/tokenizecss.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Text          string              `json:"text"`
		Families      []string            `json:"families"`
		FamiliesError *string             `json:"familiesError"`
		Sources       []map[string]string `json:"sources"`
		SourcesError  *string             `json:"sourcesError"`
		Src           string              `json:"src"`
	}
	if err := json.Unmarshal(data, &cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) < 2000 {
		t.Fatalf("only %d cases", len(cases))
	}
	failed := 0
	fail := func(format string, args ...any) {
		failed++
		if failed <= 20 {
			t.Errorf(format, args...)
		}
	}
	for _, c := range cases {
		families, err := parseFontFamily(c.Text)
		switch {
		case c.FamiliesError != nil:
			if err == nil || err.Error() != *c.FamiliesError {
				fail("parseFontFamily(%q): error %v, want %q", c.Text, err, *c.FamiliesError)
			}
		case err != nil:
			fail("parseFontFamily(%q): error %v, want %q", c.Text, err, c.Families)
		case len(families) != len(c.Families) || (len(families) > 0 && !reflect.DeepEqual(families, c.Families)):
			fail("parseFontFamily(%q) = %q, want %q", c.Text, families, c.Families)
		}

		sources, err := tokenizeCSSSrc(c.Text)
		switch {
		case c.SourcesError != nil:
			if err == nil || err.Error() != *c.SourcesError {
				fail("tokenizeCSSSrc(%q): error %v, want %q", c.Text, err, *c.SourcesError)
			}
			continue
		case err != nil:
			fail("tokenizeCSSSrc(%q): error %v, want %v", c.Text, err, c.Sources)
			continue
		}
		got := make([]map[string]string, 0, len(sources))
		for _, s := range sources {
			m := map[string]string{}
			if s.hasURL {
				m["url"] = s.url
			}
			if s.hasFormat {
				m["format"] = s.format
			}
			if s.hasTech {
				m["tech"] = s.tech
			}
			if s.hasLocal {
				m["local"] = s.local
			}
			got = append(got, m)
		}
		if len(got) != len(c.Sources) || (len(got) > 0 && !reflect.DeepEqual(got, c.Sources)) {
			fail("tokenizeCSSSrc(%q) = %v, want %v", c.Text, got, c.Sources)
			continue
		}
		if src := stringifyCSSSrc(sources); src != c.Src {
			fail("stringifyCSSSrc of %q = %q, want %q", c.Text, src, c.Src)
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d texts differ", failed, len(cases))
	}
}

// mermaidCLISheets is the text of the style sheets of the page of
// mermaid-cli, as render.PageSheets returns it.
func mermaidCLISheets(t *testing.T) []string {
	t.Helper()
	var sheets []string
	for _, sheet := range webfonts.Sheets {
		data, err := fs.ReadFile(webfonts.FS, sheet.Path)
		if err != nil {
			t.Fatal(err)
		}
		sheets = append(sheets, string(data))
	}
	return sheets
}

// TestPageSheetsMatchAssets checks that the CSSOM record uses the current
// style sheets in the order that mermaid-cli loads them.
func TestPageSheetsMatchAssets(t *testing.T) {
	sheets, err := pageSheets()
	if err != nil {
		t.Fatal(err)
	}
	if len(sheets) != len(webfonts.Sheets) {
		t.Fatalf("%d recorded style sheets, want %d", len(sheets), len(webfonts.Sheets))
	}
	for i, asset := range webfonts.Sheets {
		sheet := sheets[i]
		if "files/"+sheet.Path != asset.Path {
			t.Errorf("style sheet %d: %s, want %s", i, sheet.Path, asset.Path)
		}
		source, err := webfonts.FS.ReadFile(asset.Path)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(source)
		if hex.EncodeToString(sum[:]) != sheet.SHA256 {
			t.Errorf("%s: CSSOM record is stale. Run tools/reference/gen-mmdc-sheets.mjs", asset.Path)
		}
	}
}

var fontPayload = regexp.MustCompile(`(data:[^;,"]*;base64,)([A-Za-z0-9+/=]+)`)

// hashFonts replaces the base64 text of every font with "sha256:" and the
// hash of the bytes of the font, as the golden file has it.
func hashFonts(text string) string {
	return fontPayload.ReplaceAllStringFunc(text, func(m string) string {
		parts := fontPayload.FindStringSubmatch(m)
		data, err := base64.StdEncoding.DecodeString(parts[2])
		if err != nil {
			return m
		}
		sum := sha256.Sum256(data)
		return parts[1] + "sha256:" + hex.EncodeToString(sum[:])
	})
}

// embedPending lists the cases of TestEmbedFonts that do not pass yet, with
// the reason. The test tells when one of them passes.
var embedPending = map[string]string{}

// TestEmbedFonts compares the port of the font embedder with what real
// mermaid-cli added to the SVG of small diagrams (testdata/fontembed.json.gz,
// written by tools/reference/gen-mmdc-embed.mjs). The input of a case is
// the .svg file of mermaid-cli without fonts, so the test needs no
// renderer.
func TestEmbedFonts(t *testing.T) {
	f, err := os.Open("testdata/fontembed.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	data, err := io.ReadAll(zr)
	if err != nil {
		t.Fatal(err)
	}
	var golden struct {
		Styles map[string]string `json:"styles"`
		Cases  []struct {
			Name        string   `json:"name"`
			SVG         string   `json:"svg"`
			SHA256      string   `json:"sha256"`
			EmbedSHA256 string   `json:"embedSha256"`
			EmbedStyles []string `json:"embedStyles"`
		} `json:"cases"`
	}
	if err := json.Unmarshal(data, &golden); err != nil {
		t.Fatal(err)
	}
	if len(golden.Cases) < 15 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	sheets := mermaidCLISheets(t)
	fonts := font.Embedded().WithWebFonts()
	sha := func(s string) string {
		sum := sha256.Sum256([]byte(s))
		return hex.EncodeToString(sum[:])
	}
	for _, c := range golden.Cases {
		t.Run(c.Name, func(t *testing.T) {
			page, err := Load(c.SVG, Options{Fonts: fonts, PageSheets: sheets})
			if err != nil {
				t.Fatal(err)
			}
			// The file without fonts goes through the parser and the
			// serializer without a change.
			if got := sha(page.XML()); got != c.SHA256 {
				t.Fatalf("the file without fonts changed: %s, want %s", got, c.SHA256)
			}
			if err := page.EmbedFonts(); err != nil {
				t.Fatal(err)
			}
			// The style elements that follow the style element of mermaid.
			var added []string
			for s := page.SVG.Select("style").Next; s != nil && s.Type == dom.ElementNode && s.Tag == "style"; s = s.Next {
				added = append(added, hashFonts(s.TextContent()))
			}
			ok := len(added) == len(c.EmbedStyles) && sha(page.XML()) == c.EmbedSHA256
			for i := 0; ok && i < len(added); i++ {
				ok = added[i] == golden.Styles[c.EmbedStyles[i]]
			}
			reason, pending := embedPending[c.Name]
			switch {
			case ok && pending:
				t.Logf("the case passes now: remove it from embedPending (%s)", reason)
			case ok || pending:
			default:
				var want []int
				for _, id := range c.EmbedStyles {
					want = append(want, len(golden.Styles[id]))
				}
				var got []int
				for _, s := range added {
					got = append(got, len(s))
				}
				t.Errorf("style elements of %v characters, mmdc has %v; file %s, mmdc has %s", got, want, sha(page.XML()), c.EmbedSHA256)
				for i := 0; i < len(added) && i < len(c.EmbedStyles); i++ {
					if want := golden.Styles[c.EmbedStyles[i]]; added[i] != want {
						d := 0
						for d < len(added[i]) && d < len(want) && added[i][d] == want[d] {
							d++
						}
						t.Errorf("style %d differs at %d:\n got: %.160q\nwant: %.160q", i, d, added[i][max(0, d-40):], want[max(0, d-40):])
						break
					}
				}
			}
		})
	}
}
