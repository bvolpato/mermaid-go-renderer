package set3

import (
	"compress/gzip"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net/url"
	"os"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// TestImageSizeAgainstChromium replays testdata/images/sizes.json.gz, which
// tools/reference/gen-image-size.mjs records: for each image source, the
// natural size that Chromium gives after img.decode(), or its error.
func TestImageSizeAgainstChromium(t *testing.T) {
	f, err := os.Open("testdata/images/sizes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name   string   `json:"name"`
		Src    string   `json:"src"`
		Width  *float64 `json:"width"`
		Height *float64 `json:"height"`
		Error  string   `json:"error"`
	}
	if err := json.NewDecoder(gz).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatal("no cases")
	}
	ctx := render.NewContext(render.ContextOptions{Config: js.NewObj[any]()})
	failed := 0
	for _, c := range cases {
		got, err := naturalImageSize(ctx, c.Src)
		switch {
		case c.Error != "":
			if err == nil {
				failed++
				t.Errorf("%s: got %v x %v, want the error %q", c.Name, got.Width, got.Height, c.Error)
			} else if err.Error() != c.Error {
				failed++
				t.Errorf("%s: got the error %q, want %q", c.Name, err, c.Error)
			}
		case err != nil:
			failed++
			t.Errorf("%s: got the error %v, want %v x %v", c.Name, err, *c.Width, *c.Height)
		case got.Width != *c.Width || got.Height != *c.Height:
			failed++
			t.Errorf("%s: got %v x %v, want %v x %v", c.Name, got.Width, got.Height, *c.Width, *c.Height)
		}
	}
	t.Logf("%d images, %d different", len(cases), failed)
}

// TestImageResolver checks that a source that is not a data URL goes to
// the resolver of the render, and fails without one.
func TestImageResolver(t *testing.T) {
	ctx := render.NewContext(render.ContextOptions{Config: js.NewObj[any]()})
	if _, err := naturalImageSize(ctx, "https://example.com/a.png"); !errors.Is(err, ErrImageDecode) {
		t.Errorf("no resolver: got %v, want ErrImageDecode", err)
	}
	SetImageResolver(ctx, func(src string) (ImageSize, error) {
		if src != "https://example.com/a.png" {
			t.Errorf("resolver got %q", src)
		}
		return ImageSize{Width: 120, Height: 80}, nil
	})
	got, err := naturalImageSize(ctx, " https://example.com/a.png\n")
	if err != nil || got != (ImageSize{Width: 120, Height: 80}) {
		t.Errorf("resolver: got %v, %v", got, err)
	}
	if _, err := naturalImageSize(ctx, ""); !errors.Is(err, ErrImageDecode) {
		t.Errorf("empty source: got %v, want ErrImageDecode", err)
	}
}

// TestImageSizeMalformed feeds changed image sources to the decoder: every
// source of the golden file with bytes cut off, bytes changed and bytes
// added. The decoder must not panic, and it must give a size or an error.
func TestImageSizeMalformed(t *testing.T) {
	f, err := os.Open("testdata/images/sizes.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Src string `json:"src"`
	}
	if err := json.NewDecoder(gz).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	ctx := render.NewContext(render.ContextOptions{Config: js.NewObj[any]()})
	// A small generator with a fixed seed, so that the test is the same on
	// every run.
	seed := uint32(0x5e73)
	next := func(n int) int {
		seed = seed*1664525 + 1013904223
		return int(seed>>8) % n
	}
	tried := 0
	check := func(src string) {
		tried++
		size, err := naturalImageSize(ctx, src)
		if err == nil && (size.Width != size.Width || size.Height != size.Height || size.Width < 0 || size.Height < 0) {
			t.Errorf("bad size %v for %q", size, src)
		}
	}
	for _, c := range cases {
		comma := strings.IndexByte(c.Src, ',')
		if comma < 0 || !strings.HasPrefix(c.Src, "data:") {
			check(c.Src)
			continue
		}
		head, body := c.Src[:comma+1], c.Src[comma+1:]
		var data []byte
		isBase64 := strings.Contains(head, "base64")
		if isBase64 {
			decoded, ok := forgivingBase64Decode([]byte(body))
			if !ok {
				check(c.Src)
				continue
			}
			data = decoded
		} else {
			data = percentDecode(body)
		}
		encode := func(b []byte) string {
			if isBase64 {
				return head + base64.StdEncoding.EncodeToString(b)
			}
			return head + url.PathEscape(string(b))
		}
		// Every prefix of a short file, and some prefixes of a long one.
		step := max(len(data)/64, 1)
		for n := 0; n <= len(data); n += step {
			check(encode(data[:n]))
		}
		for i := 0; i < 40 && len(data) > 0; i++ {
			changed := append([]byte(nil), data...)
			for k := 0; k <= next(4); k++ {
				changed[next(len(changed))] = byte(next(256))
			}
			check(encode(changed))
			at := next(len(data) + 1)
			grown := append(append(append([]byte(nil), data[:at]...), byte(next(256)), byte(next(256))), data[at:]...)
			check(encode(grown))
		}
	}
	t.Logf("%d sources", tried)
}
