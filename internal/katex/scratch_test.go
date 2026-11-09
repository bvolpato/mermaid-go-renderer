package katex

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestScratch(t *testing.T) {
	t0 := time.Now()
	_, _, err := programs()
	fmt.Println("compile", time.Since(t0), err)
	t0 = time.Now()
	rt, err := newRuntime()
	fmt.Println("start", time.Since(t0), err)
	_ = rt
	for _, tex := range []string{`x^2`, `\frac{a}{b}`, `\gdef\foo{x}\foo`, `\foo`, `\frac{`, "\U0001D400", "a\xed\xa0\x80b", `\char"D800`, `\message{hi}x`, `% c`, "é", "x́"} {
		t0 = time.Now()
		s, err := RenderToString(tex, OutputMathML)
		fmt.Printf("%q -> %q err=%v (%T) %v\n", tex, s, err, err, time.Since(t0))
	}
	for _, n := range []int{100, 500, 1000, 2000, 3000, 5000, 10000, 20000} {
		tex := strings.Repeat("{", n) + "x" + strings.Repeat("}", n)
		t0 = time.Now()
		s, err := RenderToString(tex, OutputMathML)
		fmt.Printf("depth %d -> len %d err=%v %v\n", n, len(s), err, time.Since(t0))
	}
}
