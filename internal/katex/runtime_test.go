// These tests exercise the Go runtime for KaTeX 0.18.11 dist/katex.mjs (MIT).
package katex

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/dop251/goja"
)

func TestRenderMacroIsolation(t *testing.T) {
	cases := loadOracle(t)
	rt, err := newRuntime()
	if err != nil {
		t.Fatal(err)
	}
	// Use one runtime so pool scheduling cannot hide a leaked definition.
	for _, output := range []Output{OutputMathML, OutputHTMLAndMathML} {
		for _, name := range []string{
			"macro-global", "macro-global-absent", "macro-local", "macro-local-absent",
			"macro-global-before-error", "macro-failed-absent", "fraction",
		} {
			t.Run(string(output)+"/"+name, func(t *testing.T) {
				c := findOracle(t, cases, name, output)
				got, reuse, err := rt.run(context.Background(), c.Tex.value(), c.Output)
				if err := compareOracle(c, got, err); err != nil {
					t.Fatal(err)
				}
				if !reuse {
					t.Fatal("ordinary render or parse error discarded the runtime")
				}
			})
		}
	}
}

func TestRenderConcurrent(t *testing.T) {
	cases := loadOracle(t)
	var work []oracleCase
	for _, output := range []Output{OutputMathML, OutputHTMLAndMathML} {
		for _, name := range []string{"fraction", "cases", "macro-global", "macro-global-absent", "error-macro-loop", "unicode-text"} {
			work = append(work, findOracle(t, cases, name, output))
		}
	}
	const workers = 8
	start := make(chan struct{})
	failures := make(chan error, workers)
	var wg sync.WaitGroup
	for worker := 0; worker < workers; worker++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for _, c := range work {
				got, err := RenderToString(c.Tex.value(), c.Output)
				if err := compareOracle(c, got, err); err != nil {
					failures <- fmt.Errorf("%s/%s: %w", c.Output, c.Name, err)
					return
				}
			}
		}()
	}
	close(start)
	wg.Wait()
	close(failures)
	for err := range failures {
		t.Error(err)
	}
}

func TestRenderUnknownOutput(t *testing.T) {
	for _, output := range []Output{"", "html", "MathML"} {
		t.Run(string(output), func(t *testing.T) {
			got, err := RenderToString("x", output)
			want := fmt.Sprintf("katex: unknown output %q", output)
			if got != "" || err == nil || err.Error() != want {
				t.Fatalf("RenderToString = %q, %v; want empty output and %q", got, err, want)
			}
		})
	}
}

func TestRenderCancelledContext(t *testing.T) {
	for _, output := range []Output{OutputMathML, OutputHTMLAndMathML} {
		t.Run(string(output), func(t *testing.T) {
			cancelled, cancel := context.WithCancel(context.Background())
			cancel()
			expired, stop := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
			defer stop()
			for _, ctx := range []context.Context{cancelled, expired} {
				got, err := RenderToStringContext(ctx, "x", output)
				if got != "" || !errors.Is(err, ErrTimeout) || !errors.Is(err, ctx.Err()) {
					t.Fatalf("RenderToStringContext = %q, %v; want ErrTimeout and %v", got, err, ctx.Err())
				}
			}
		})
	}
}

func TestRenderCancellationDuringJavaScript(t *testing.T) {
	cases := loadOracle(t)
	for _, output := range []Output{OutputMathML, OutputHTMLAndMathML} {
		t.Run(string(output), func(t *testing.T) {
			rt, err := newRuntime()
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			console := rt.vm.Get("console").ToObject(rt.vm)
			called := false
			// Cancel only after KaTeX reaches its real console macro.
			if err := console.Set("log", func(goja.FunctionCall) goja.Value {
				called = true
				cancel()
				return goja.Undefined()
			}); err != nil {
				t.Fatal(err)
			}
			got, reuse, err := rt.run(ctx, `\message{cancel}`+strings.Repeat("x+", 4096)+"1", output)
			if !called {
				t.Fatal("KaTeX did not reach the cancellation point")
			}
			if got != "" || reuse || !errors.Is(err, ErrTimeout) || !errors.Is(err, context.Canceled) {
				t.Fatalf("run = %q, reuse %v, %v; want empty output, no reuse, ErrTimeout and context.Canceled", got, reuse, err)
			}
			c := findOracle(t, cases, "fraction", output)
			got, err = RenderToStringContext(context.Background(), c.Tex.value(), output)
			if err := compareOracle(c, got, err); err != nil {
				t.Fatalf("render after cancellation: %v", err)
			}
		})
	}
}
