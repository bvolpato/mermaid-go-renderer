// Command paritycmd is the parity command with only the pie chart linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/pie/paritycmd -filter pie/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/pie"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
