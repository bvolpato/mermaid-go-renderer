// Command paritycmd is the parity command with only the quadrant chart linked
// in. Run it from the repository root:
//
//	go run ./internal/diagrams/quadrant/paritycmd -filter quadrant-chart/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/quadrant"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
