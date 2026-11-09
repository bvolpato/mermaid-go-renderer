// Command paritycmd is the parity command with only the gitGraph diagram
// linked in. Run it from the repository root:
//
//	go run ./internal/diagrams/git/paritycmd -filter git/
package main

import (
	_ "github.com/bvolpato/mermaid-go-renderer/internal/diagrams/git"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity/cli"
)

func main() { cli.Main() }
