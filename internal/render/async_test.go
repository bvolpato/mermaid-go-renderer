package render

import (
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

type asyncOp struct {
	T     string      `json:"t"`
	V     string      `json:"v"`
	Ops   []asyncOp   `json:"ops"`
	Tasks [][]asyncOp `json:"tasks"`
}

// asyncProgram runs a program of async operations the way a port of the
// same JavaScript would: with the calls and the convention of async.go.
type asyncProgram struct {
	ctx *Context
	log []string
}

// body runs the body of an async function without the await of its caller.
func (p *asyncProgram) body(ops []asyncOp, name string) {
	for _, op := range ops {
		switch op.T {
		case "log":
			p.log = append(p.log, name+":"+op.V)
		case "await":
			p.ctx.Await()
		case "measure":
			p.ctx.AwaitMeasure()
		case "call":
			// An async function: its last step is the await of the caller.
			p.body(op.Ops, name)
			p.ctx.Await()
		case "sync":
			// A function that is not async: the caller awaits its value.
			for _, inner := range op.Ops {
				p.log = append(p.log, name+":"+inner.V)
			}
			p.ctx.Await()
		case "all":
			_ = p.ctx.All(len(op.Tasks), func(i int) error {
				p.body(op.Tasks[i], fmt.Sprintf("%s.%d", name, i))
				return nil
			})
		case "timeout":
			p.ctx.AwaitTimeout()
		}
	}
}

// TestAsyncOrderAgainstJavaScript replays programs that ran in node with
// real promises (tools/reference/gen-async-order.mjs) and compares the
// order of the log entries.
func TestAsyncOrderAgainstJavaScript(t *testing.T) {
	f, err := os.Open("testdata/async_order.json.gz")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	zr, err := gzip.NewReader(f)
	if err != nil {
		t.Fatal(err)
	}
	var cases []struct {
		Name string    `json:"name"`
		Ops  []asyncOp `json:"ops"`
		Log  []string  `json:"log"`
	}
	if err := json.NewDecoder(zr).Decode(&cases); err != nil {
		t.Fatal(err)
	}
	failed := 0
	for _, c := range cases {
		p := &asyncProgram{ctx: &Context{}}
		// The program itself is one async function that the test awaits.
		_ = p.ctx.All(1, func(int) error {
			p.body(c.Ops, "r")
			return nil
		})
		got, want := strings.Join(p.log, " "), strings.Join(c.Log, " ")
		if got != want {
			failed++
			if failed <= 5 {
				ops, _ := json.Marshal(c.Ops)
				t.Errorf("%s\n ops: %s\n got: %s\nwant: %s", c.Name, ops, got, want)
			}
		}
	}
	if failed > 0 {
		t.Errorf("%d of %d cases differ", failed, len(cases))
	}
}

// TestAllPropagatesErrorsAndPanics checks the two ways a task can fail.
func TestAllPropagatesErrorsAndPanics(t *testing.T) {
	ctx := &Context{}
	want := fmt.Errorf("boom")
	err := ctx.All(3, func(i int) error {
		ctx.Await()
		if i == 1 {
			return want
		}
		return nil
	})
	if err != want {
		t.Fatalf("All returned %v", err)
	}
	defer func() {
		if p := recover(); p != "bang" {
			t.Fatalf("recovered %v", p)
		}
	}()
	_ = ctx.All(2, func(i int) error {
		ctx.AwaitMeasure()
		if i == 0 {
			panic("bang")
		}
		return nil
	})
}

// TestCancelEndsEveryTask cancels a render while tasks of All wait. The
// cancel error must come back, and no task goroutine may stay behind.
func TestCancelEndsEveryTask(t *testing.T) {
	before := runtime.NumGoroutine()
	for round := 0; round < 20; round++ {
		std, cancel := context.WithCancel(context.Background())
		ctx := &Context{Cancel: std}
		var got any
		func() {
			defer func() { got = recover() }()
			_ = ctx.All(4, func(i int) error {
				ctx.Await()
				_ = ctx.All(3, func(j int) error {
					ctx.AwaitMeasure()
					if i == 2 && j == 1 {
						cancel()
					}
					ctx.Await()
					ctx.AwaitTimeout()
					return nil
				})
				ctx.AwaitMeasure()
				return nil
			})
		}()
		c, ok := got.(canceled)
		if !ok || c.err != context.Canceled {
			t.Fatalf("round %d: recovered %v", round, got)
		}
		cancel()
	}
	// The task goroutines end right after they signal the scheduler.
	deadline := time.Now().Add(2 * time.Second)
	for runtime.NumGoroutine() > before && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if after := runtime.NumGoroutine(); after > before {
		t.Fatalf("%d goroutines before, %d after", before, after)
	}
}
