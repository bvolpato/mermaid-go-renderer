// The exceptions that the ported code throws. UnknownDiagramError is ported
// from packages/mermaid/src/errors.ts (mermaid 12.2.0).

package config

import "fmt"

// Error is an exception that the upstream JavaScript throws for an input.
// Name is the JavaScript error class.
type Error struct {
	// Name is "TypeError", "UnknownDiagramError", "YAMLException" or "Error".
	Name    string
	Message string
	// Err is the underlying Go error, when there is one.
	Err error
}

func (e *Error) Error() string { return e.Name + ": " + e.Message }
func (e *Error) Unwrap() error { return e.Err }

func typeError(format string, args ...any) *Error {
	return &Error{Name: "TypeError", Message: fmt.Sprintf(format, args...)}
}

// unknownDiagramError is errors.ts UnknownDiagramError.
func unknownDiagramError(message string) *Error {
	return &Error{Name: "UnknownDiagramError", Message: message}
}

// IsUnknownDiagram reports whether err is an UnknownDiagramError: no diagram
// detector matches the text.
func IsUnknownDiagram(err error) bool {
	e, ok := err.(*Error)
	return ok && e.Name == "UnknownDiagramError"
}
