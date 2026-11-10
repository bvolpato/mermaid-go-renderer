// These tests compare KaTeX 0.18.11 dist/katex.mjs (MIT) with its Node oracle.
package katex

import (
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"testing"
	"unicode/utf16"
	"unicode/utf8"
)

type oracleText struct {
	Text  string   `json:"text"`
	Units []uint16 `json:"units"`
}

func (s oracleText) value() string {
	if s.Units == nil {
		return s.Text
	}
	var out []byte
	for i := 0; i < len(s.Units); i++ {
		u := s.Units[i]
		if u >= 0xd800 && u <= 0xdbff && i+1 < len(s.Units) {
			v := s.Units[i+1]
			if v >= 0xdc00 && v <= 0xdfff {
				out = utf8.AppendRune(out, utf16.DecodeRune(rune(u), rune(v)))
				i++
				continue
			}
		}
		if u >= 0xd800 && u <= 0xdfff {
			out = append(out, byte(0xe0|u>>12), byte(0x80|(u>>6)&0x3f), byte(0x80|u&0x3f))
		} else {
			out = utf8.AppendRune(out, rune(u))
		}
	}
	return string(out)
}

type oracleError struct {
	Parse       bool       `json:"parse"`
	Name        string     `json:"name"`
	Message     oracleText `json:"message"`
	RawMessage  oracleText `json:"rawMessage"`
	HasPosition bool       `json:"hasPosition"`
	Position    int        `json:"position"`
	Length      int        `json:"length"`
}

type oracleCase struct {
	Name   string       `json:"name"`
	Tex    oracleText   `json:"tex"`
	Output Output       `json:"output"`
	Want   *oracleText  `json:"want"`
	Error  *oracleError `json:"error"`
}

func loadOracle(t *testing.T) []oracleCase {
	t.Helper()
	f, err := os.Open("testdata/render.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	defer r.Close()
	var golden struct {
		Version      string `json:"version"`
		SourceSHA256 string `json:"sourceSHA256"`
		Options      struct {
			ThrowOnError bool `json:"throwOnError"`
			DisplayMode  bool `json:"displayMode"`
		} `json:"options"`
		Cases []oracleCase `json:"cases"`
	}
	if err := json.NewDecoder(r).Decode(&golden); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256([]byte(katexSource))
	if golden.Version != Version || golden.SourceSHA256 != hex.EncodeToString(sum[:]) {
		t.Fatal("Node oracle and embedded KaTeX have different versions or source bytes")
	}
	if !golden.Options.ThrowOnError || !golden.Options.DisplayMode || len(golden.Cases) == 0 {
		t.Fatal("Node oracle has no cases or uses different rendering options")
	}
	return golden.Cases
}

func findOracle(t *testing.T, cases []oracleCase, name string, output Output) oracleCase {
	t.Helper()
	for _, c := range cases {
		if c.Name == name && c.Output == output {
			return c
		}
	}
	t.Fatalf("missing Node oracle case %s/%s", name, output)
	return oracleCase{}
}

func compareOracle(c oracleCase, got string, err error) error {
	if c.Error == nil {
		if err != nil {
			return fmt.Errorf("unexpected %T: %w", err, err)
		}
		if c.Want == nil {
			return errors.New("oracle has neither output nor error")
		}
		want := c.Want.value()
		if got != want {
			i := 0
			for i < len(got) && i < len(want) && got[i] == want[i] {
				i++
			}
			return fmt.Errorf("output differs at byte %d (got %d bytes, want %d):\ngot  %q\nwant %q", i, len(got), len(want), excerpt(got, i), excerpt(want, i))
		}
		return nil
	}
	if got != "" {
		return fmt.Errorf("error returned nonempty output %q", excerpt(got, 0))
	}
	want := c.Error
	if err == nil || err.Error() != want.Message.value() {
		return fmt.Errorf("error = %v, want %q", err, want.Message.value())
	}
	if want.Parse {
		var parsed *ParseError
		if !errors.As(err, &parsed) {
			return fmt.Errorf("error type = %T, want *ParseError", err)
		}
		if parsed.RawMessage != want.RawMessage.value() || parsed.HasPosition != want.HasPosition || parsed.Position != want.Position || parsed.Length != want.Length {
			return fmt.Errorf("parse metadata = %#v, want rawMessage %q, hasPosition %v, position %d, length %d", parsed, want.RawMessage.value(), want.HasPosition, want.Position, want.Length)
		}
	} else {
		var other *Error
		if !errors.As(err, &other) || other.Name != want.Name {
			return fmt.Errorf("error = %#v, want *Error with name %q", err, want.Name)
		}
	}
	return nil
}

func excerpt(s string, at int) string {
	return s[max(0, at-50):min(len(s), at+100)]
}

func TestRenderToStringNodeOracle(t *testing.T) {
	for _, c := range loadOracle(t) {
		t.Run(string(c.Output)+"/"+c.Name, func(t *testing.T) {
			got, err := RenderToString(c.Tex.value(), c.Output)
			if err := compareOracle(c, got, err); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestRenderInvalidUTF8(t *testing.T) {
	cases := loadOracle(t)
	for _, output := range []Output{OutputMathML, OutputHTMLAndMathML} {
		for _, invalid := range []string{"\xff", "\x80", "\xc0", "\xe2\x82"} {
			t.Run(fmt.Sprintf("%s/%x", output, invalid), func(t *testing.T) {
				// Each invalid byte becomes a replacement character.
				name := "unicode-replacement"
				if len(invalid) > 1 {
					name = "unicode-replacement-double"
				}
				c := findOracle(t, cases, name, output)
				got, err := RenderToString(invalid, output)
				if err := compareOracle(c, got, err); err != nil {
					t.Fatal(err)
				}
			})
		}
	}
}
