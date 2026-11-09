package liberation

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
	"testing"
)

// sums are the SHA-256 digests of the font files of the Ubuntu package
// fonts-liberation 1:2.1.5-3.
var sums = map[string]string{
	"LiberationMono-Bold.ttf":        "626655e94dd82f3f42549daf995c921b0915fa8ab1f4b839559e8892ea41d240",
	"LiberationMono-BoldItalic.ttf":  "15eb161953e3ecc7fc05a3fec8a59e0f4e0a54a4e375736adf8e98d65981f813",
	"LiberationMono-Italic.ttf":      "a71b2c25c89da05cf0e7c4dbba8d473fdead0b181ae56165217747d2c1f39215",
	"LiberationMono-Regular.ttf":     "395fa5ab8d40c8eba390ced528744ea75a7f69aabf3e68b6f925ca0e39a27370",
	"LiberationSans-Bold.ttf":        "3973aa5054fb467dd5627245d3dc82e37bf16fe075756156a570455871351582",
	"LiberationSans-BoldItalic.ttf":  "c80fa7f2ffa0e01d4d8dcd6a6d1e43eda665222d1b4db597dde1174c456006cf",
	"LiberationSans-Italic.ttf":      "830c5fa600505fb4c1a271b4271c53c44bae43f492b2a240d0e98a3a7a380121",
	"LiberationSans-Regular.ttf":     "4659bc0c58c5028dd488ec928d41d9265db43d9b669fc14ca8b0832daca7b144",
	"LiberationSerif-Bold.ttf":       "9e66c25e20868756bf12eb24a520a4552727c9cd108f577f7ae33e3c0110e39c",
	"LiberationSerif-BoldItalic.ttf": "379010e87421a883f7bbfa7936d23dfe0257a54ce850819b74556e9f2c615a8a",
	"LiberationSerif-Italic.ttf":     "eaeaf7f2b12544ecde64290c1fa7ec403ff70fd58b313637b265400b20249242",
	"LiberationSerif-Regular.ttf":    "705903ae1382f0150f115d6f10a30f5b2f3d6ade649c474afe6366c39174fb65",
}

// TestFonts checks that every embedded file inflates to the file of the
// package.
func TestFonts(t *testing.T) {
	if len(Names()) != len(sums) {
		t.Fatalf("got %d names, want %d", len(Names()), len(sums))
	}
	for _, name := range Names() {
		data, err := Font(name)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(data)
		if got := hex.EncodeToString(sum[:]); got != sums[name] {
			t.Errorf("%s: sha256 %s, want %s", name, got, sums[name])
		}
	}
	if _, err := Font("missing.ttf"); err == nil {
		t.Error("Font(missing.ttf): no error")
	}
	if !strings.Contains(License, "SIL OPEN FONT LICENSE Version 1.1") {
		t.Error("License does not have the license text")
	}
}
