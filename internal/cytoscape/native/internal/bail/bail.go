// Package bail is how a ported layout says that it does not cover an input.
//
// A port keeps the control flow of the JavaScript, so the place that finds an
// option, an element kind or a branch that is not ported is deep in the code.
// It calls Out there. The entry point of the port recovers with Recover and
// reports the reason, and its caller runs the interpreter.
package bail

import "fmt"

// NotCovered is the panic value of Out.
type NotCovered struct {
	Reason string
}

func (n NotCovered) Error() string { return n.Reason }

// Out stops the layout: the port does not cover the input.
func Out(format string, args ...any) {
	panic(NotCovered{Reason: fmt.Sprintf(format, args...)})
}

// Recover is deferred by an entry point. It turns each panic into a reason:
// the reason of Out, or the text of a runtime panic. A runtime panic stands
// for an input on which the JavaScript throws or on which the port is wrong.
// In both cases the interpreter decides the result.
func Recover(reason *string) {
	r := recover()
	if r == nil {
		return
	}
	if n, ok := r.(NotCovered); ok {
		*reason = n.Reason
		return
	}
	*reason = fmt.Sprintf("panic: %v", r)
}
