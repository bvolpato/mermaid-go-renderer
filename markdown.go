package mermaid

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func ExtractMermaidBlocks(input string) []string {
	blocks := make([]string, 0, 4)
	var current []string
	inBlock := false
	fence := ""

	for _, rawLine := range strings.Split(input, "\n") {
		line := strings.TrimSpace(rawLine)
		if !inBlock {
			if startFence, ok := detectMermaidFence(line); ok {
				inBlock = true
				fence = startFence
				current = current[:0]
				continue
			}
		} else {
			if isFenceEnd(line, fence) {
				inBlock = false
				blocks = append(blocks, strings.Join(current, "\n"))
				continue
			}
			current = append(current, rawLine)
		}
	}
	return blocks
}

func detectMermaidFence(line string) (string, bool) {
	switch {
	case strings.HasPrefix(line, "```"):
		rest := strings.TrimSpace(strings.TrimLeft(line, "`"))
		return "```", strings.HasPrefix(lower(rest), "mermaid")
	case strings.HasPrefix(line, "~~~"):
		rest := strings.TrimSpace(strings.TrimLeft(line, "~"))
		return "~~~", strings.HasPrefix(lower(rest), "mermaid")
	case strings.HasPrefix(line, ":::"):
		rest := strings.TrimSpace(strings.TrimLeft(line, ":"))
		return ":::", strings.HasPrefix(lower(rest), "mermaid")
	default:
		return "", false
	}
}

func isFenceEnd(line, fence string) bool {
	if !strings.HasPrefix(line, fence) {
		return false
	}
	return strings.TrimSpace(line[len(fence):]) == ""
}

func ResolveMultiOutputs(basePath string, ext string, count int) ([]string, error) {
	if basePath == "" {
		return nil, fmt.Errorf("output path required for markdown input")
	}
	if count <= 0 {
		return nil, fmt.Errorf("invalid diagram count")
	}

	info, err := os.Stat(basePath)
	if err == nil && info.IsDir() {
		outputs := make([]string, 0, count)
		for i := 0; i < count; i++ {
			outputs = append(outputs, filepath.Join(basePath, fmt.Sprintf("diagram-%d.%s", i+1, ext)))
		}
		return outputs, nil
	}

	parent := filepath.Dir(basePath)
	name := filepath.Base(basePath)
	stem := strings.TrimSuffix(name, filepath.Ext(name))
	if stem == "" || stem == "." {
		stem = "diagram"
	}
	outputs := make([]string, 0, count)
	for i := 0; i < count; i++ {
		outputs = append(outputs, filepath.Join(parent, fmt.Sprintf("%s-%d.%s", stem, i+1, ext)))
	}
	return outputs, nil
}
