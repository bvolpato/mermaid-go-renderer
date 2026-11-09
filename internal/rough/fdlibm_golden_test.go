package rough

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"strconv"
	"testing"
)

// The golden file testdata/fdlibm.json comes from
// tools/reference/gen-rough-fdlibm.mjs. It has the results of the fdlibm
// functions in JavaScript (tools/reference/rough-fdlibm.mjs) for 730000
// random arguments, as one hash for each block of 500 arguments, and the
// results for a list of special arguments. The Go versions in fdlibm_test.go
// must give the same results, so that TestGolden and the generator of its
// golden file use the same transcendental functions.

type fdlibmGolden struct {
	Meta struct {
		Node  string `json:"node"`
		V8    string `json:"v8"`
		Block int    `json:"block"`
		Seed  uint32 `json:"seed"`
	} `json:"meta"`
	Random []struct {
		Fn     string   `json:"fn"`
		Expo   int      `json:"expo"`
		Hashes []string `json:"hashes"`
	} `json:"random"`
	Fixed   []string    `json:"fixed"`
	Special [][3]string `json:"special"`
}

var fdlibmFuncs = map[string]func(float64) float64{
	"sin":  fdlibmSin,
	"cos":  fdlibmCos,
	"tan":  fdlibmTan,
	"atan": fdlibmAtan,
	"asin": fdlibmAsin,
}

// prng is the PRNG of the generator script (mulberry32).
type fdlibmPRNG struct{ state uint32 }

func (p *fdlibmPRNG) u32() uint32 {
	p.state += 0x6d2b79f5
	t := p.state
	t = (t ^ (t >> 15)) * (t | 1)
	t ^= t + (t^(t>>7))*(t|61)
	return t ^ (t >> 14)
}

// input returns a number in [-2^expo, 2^expo) with 53 random bits. Every
// operation is exact, so the fusing of operations does not change the result.
func (p *fdlibmPRNG) input(expo int) float64 {
	hi := p.u32() >> 11
	lo := p.u32()
	m := float64(float64(hi)*4294967296) + float64(lo)
	return float64((m - 4503599627370496) * math.Ldexp(1, expo-52))
}

func fdlibmBitsHex(v float64) string {
	if v != v {
		return "7ff8000000000000"
	}
	return fmt.Sprintf("%016x", math.Float64bits(v))
}

func loadFdlibmGolden(t *testing.T) *fdlibmGolden {
	t.Helper()
	data, err := os.ReadFile("testdata/fdlibm.json")
	if err != nil {
		t.Fatal(err)
	}
	var g fdlibmGolden
	if err := json.Unmarshal(data, &g); err != nil {
		t.Fatal(err)
	}
	return &g
}

func TestFdlibmRandom(t *testing.T) {
	g := loadFdlibmGolden(t)
	p := &fdlibmPRNG{state: g.Meta.Seed}
	total, badBlocks := 0, 0
	for _, r := range g.Random {
		fn := fdlibmFuncs[r.Fn]
		for b, want := range r.Hashes {
			h := sha256.New()
			var first float64
			for i := 0; i < g.Meta.Block; i++ {
				x := p.input(r.Expo)
				if i == 0 {
					first = x
				}
				h.Write([]byte(fdlibmBitsHex(fn(x))))
				total++
			}
			if got := hex.EncodeToString(h.Sum(nil))[:16]; got != want {
				badBlocks++
				if badBlocks <= 10 {
					t.Errorf("%s, arguments below 2^%d, block %d (first argument %s): hash %s, want %s", r.Fn, r.Expo, b, fdlibmBitsHex(first), got, want)
				}
			}
		}
	}
	// asin of k / 1e9.
	for b, want := range g.Fixed {
		h := sha256.New()
		for i := 0; i < g.Meta.Block; i++ {
			k := float64(p.u32()%2000000001) - 1000000000
			h.Write([]byte(fdlibmBitsHex(fdlibmAsin(k / 1000000000))))
			total++
		}
		if got := hex.EncodeToString(h.Sum(nil))[:16]; got != want {
			badBlocks++
			t.Errorf("asin(k/1e9), block %d: hash %s, want %s", b, got, want)
		}
	}
	t.Logf("%d arguments, %d blocks differ from the JavaScript functions", total, badBlocks)
}

func TestFdlibmSpecial(t *testing.T) {
	g := loadFdlibmGolden(t)
	for _, s := range g.Special {
		u, err := strconv.ParseUint(s[1], 16, 64)
		if err != nil {
			t.Fatal(err)
		}
		x := math.Float64frombits(u)
		if got := fdlibmBitsHex(fdlibmFuncs[s[0]](x)); got != s[2] {
			t.Errorf("%s(%v) = %s, want %s", s[0], x, got, s[2])
		}
	}
}
