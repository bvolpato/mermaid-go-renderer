// Command paritycmd is the parity command with only the xychart diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/xychart/paritycmd -filter xychart/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/xychart"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
