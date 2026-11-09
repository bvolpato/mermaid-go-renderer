// Package grammar is the parent of the generated grammar packages. Each
// subdirectory holds the tables of one jison grammar of mermaid 12.2.0, which
// tools/reference/gen-jison.mjs writes. The driver is internal/jison.
//
// The tests of this package compare the Go driver with the JavaScript parser
// that jison generates. tools/reference/gen-jison-golden.mjs writes the
// golden files in testdata.
package grammar
