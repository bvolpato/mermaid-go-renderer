package crmath

import (
	"math"
	"os"
	"testing"
)

// platformArgs returns arguments of pow and of tanh from the generator of
// the golden files.
func platformArgs() (pow, tanh []float64) {
	return mathInputs("pow", 20000), mathInputs("tanh", 20000)
}

func platformSame(a, b float64) bool {
	return math.Float64bits(a) == math.Float64bits(b) || (math.IsNaN(a) && math.IsNaN(b))
}

// TestPlatformEnv checks the platform of the process: the value of the
// environment variable MMDG_MATH, or the default. The other tests restore
// the platform that they change.
func TestPlatformEnv(t *testing.T) {
	want := DefaultPlatform
	if p, ok := ParsePlatform(os.Getenv(PlatformEnv)); ok {
		want = p
	}
	if got := CurrentPlatform(); got != want {
		t.Errorf("the current platform is %v, %s=%q selects %v", got, PlatformEnv, os.Getenv(PlatformEnv), want)
	}
	if DefaultPlatform != PlatformLinux {
		t.Errorf("the default platform is %v", DefaultPlatform)
	}
}

func TestParsePlatform(t *testing.T) {
	for _, c := range []struct {
		name string
		want Platform
		ok   bool
	}{
		{"linux", PlatformLinux, true},
		{"darwin", PlatformDarwin, true},
		{" Linux\n", PlatformLinux, true},
		{"DARWIN", PlatformDarwin, true},
		{"", DefaultPlatform, false},
		{"macos", DefaultPlatform, false},
		{"windows", DefaultPlatform, false},
	} {
		got, ok := ParsePlatform(c.name)
		if got != c.want || ok != c.ok {
			t.Errorf("ParsePlatform(%q) = %v, %v, want %v, %v", c.name, got, ok, c.want, c.ok)
		}
	}
	for _, p := range []Platform{PlatformLinux, PlatformDarwin} {
		if got, ok := ParsePlatform(p.String()); !ok || got != p {
			t.Errorf("ParsePlatform(%q) = %v, %v", p.String(), got, ok)
		}
	}
}

// TestPlatformSwitch checks that Pow and Tanh are the functions of the
// current platform, and that the two platforms give different results.
func TestPlatformSwitch(t *testing.T) {
	defer SetPlatform(CurrentPlatform())
	powArgs, tanhArgs := platformArgs()
	for _, c := range []struct {
		platform Platform
		pow      func(x, y float64) float64
		tanh     func(x float64) float64
	}{
		{PlatformLinux, PowLinux, TanhLinux},
		{PlatformDarwin, PowDarwin, TanhDarwin},
	} {
		SetPlatform(c.platform)
		if got := CurrentPlatform(); got != c.platform {
			t.Fatalf("the current platform is %v after SetPlatform(%v)", got, c.platform)
		}
		for i := 0; i+1 < len(powArgs); i += 2 {
			x, y := powArgs[i], powArgs[i+1]
			if got, want := Pow(x, y), c.pow(x, y); !platformSame(got, want) {
				t.Fatalf("%v: Pow(%v, %v) = %v, the function of the platform gives %v", c.platform, x, y, got, want)
			}
			if got, want := powLibm(x, y), c.pow(x, y); !platformSame(got, want) {
				if _, special := powSpecial(x, y); !special {
					t.Fatalf("%v: powLibm(%v, %v) = %v, the function of the platform gives %v", c.platform, x, y, got, want)
				}
			}
		}
		for _, x := range tanhArgs {
			if got, want := Tanh(x), c.tanh(x); !platformSame(got, want) {
				t.Fatalf("%v: Tanh(%v) = %v, the function of the platform gives %v", c.platform, x, got, want)
			}
		}
	}
	if !HasDarwinLibm() {
		t.Log("the package does not have the functions of macOS: both platforms have the functions of Linux")
		return
	}
	powDiffer, tanhDiffer := 0, 0
	for i := 0; i+1 < len(powArgs); i += 2 {
		if !platformSame(PowLinux(powArgs[i], powArgs[i+1]), PowDarwin(powArgs[i], powArgs[i+1])) {
			powDiffer++
		}
	}
	for _, x := range tanhArgs {
		if !platformSame(TanhLinux(x), TanhDarwin(x)) {
			tanhDiffer++
		}
	}
	if powDiffer == 0 || tanhDiffer == 0 {
		t.Errorf("Linux and macOS differ for %d results of pow and %d results of tanh: the test has no argument that tells them apart", powDiffer, tanhDiffer)
	}
}

// TestPlatformDarwinFallback states what the package does without a port of
// the pow and the tanh of the C library of Apple. The module has no such
// port. No file sets darwinLibmPow and darwinLibmTanh, so the two variables
// are nil, and PlatformDarwin has the functions of Linux.
func TestPlatformDarwinFallback(t *testing.T) {
	if got, want := HasDarwinLibm(), darwinLibmPow != nil && darwinLibmTanh != nil; got != want {
		t.Errorf("HasDarwinLibm() = %v, the function variables say %v", got, want)
	}
	if (darwinLibmPow == nil) != (darwinLibmTanh == nil) {
		t.Errorf("the package has one function of macOS without the other: pow %v, tanh %v", darwinLibmPow != nil, darwinLibmTanh != nil)
	}
	t.Logf("the package has the functions of macOS: %v", HasDarwinLibm())

	// The state of the module: no function of macOS.
	pow, tanh := darwinLibmPow, darwinLibmTanh
	defer func() { darwinLibmPow, darwinLibmTanh = pow, tanh }()
	defer SetPlatform(CurrentPlatform())
	darwinLibmPow, darwinLibmTanh = nil, nil
	if HasDarwinLibm() {
		t.Error("HasDarwinLibm() = true without the function variables")
	}
	SetPlatform(PlatformDarwin)
	powArgs, tanhArgs := platformArgs()
	for i := 0; i+1 < len(powArgs); i += 2 {
		x, y := powArgs[i], powArgs[i+1]
		want := PowLinux(x, y)
		if got := PowDarwin(x, y); !platformSame(got, want) {
			t.Fatalf("PowDarwin(%v, %v) = %v without the pow of macOS, PowLinux gives %v", x, y, got, want)
		}
		if got := Pow(x, y); !platformSame(got, want) {
			t.Fatalf("Pow(%v, %v) = %v without the pow of macOS, PowLinux gives %v", x, y, got, want)
		}
	}
	for _, x := range tanhArgs {
		want := TanhLinux(x)
		if got := TanhDarwin(x); !platformSame(got, want) {
			t.Fatalf("TanhDarwin(%v) = %v without the tanh of macOS, TanhLinux gives %v", x, got, want)
		}
		if got := Tanh(x); !platformSame(got, want) {
			t.Fatalf("Tanh(%v) = %v without the tanh of macOS, TanhLinux gives %v", x, got, want)
		}
	}
}
