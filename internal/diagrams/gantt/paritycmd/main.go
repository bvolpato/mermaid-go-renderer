// Command paritycmd is the parity command with only the gantt diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/gantt/paritycmd -filter gantt/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/gantt"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
