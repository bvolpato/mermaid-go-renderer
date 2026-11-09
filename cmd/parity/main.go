// Command parity compares this renderer with the reference output of
// upstream mermaid for every corpus fixture. See internal/parity/cli.
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
