package main

import (
	"bytes"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunRendersSVGFile(t *testing.T) {
	tmp := t.TempDir()
	inputPath := filepath.Join(tmp, "diagram.mmd")
	outputPath := filepath.Join(tmp, "diagram.svg")
	input := "flowchart LR\nA --> B --> C\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mmdg", "-i", inputPath, "-o", outputPath, "-e", "svg"}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	out, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if !strings.Contains(string(out), "<svg") {
		t.Fatalf("expected SVG output")
	}
}

func TestRunRendersPNGFile(t *testing.T) {
	tmp := t.TempDir()
	inputPath := filepath.Join(tmp, "diagram.mmd")
	outputPath := filepath.Join(tmp, "diagram.png")
	input := "flowchart LR\nA --> B --> C\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write input: %v", err)
	}

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mmdg", "-i", inputPath, "-o", outputPath, "-e", "png"}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	out, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected PNG bytes")
	}
	if _, err := png.Decode(bytes.NewReader(out)); err != nil {
		t.Fatalf("invalid PNG output: %v", err)
	}
}

func TestRunMarkdownMultiOutput(t *testing.T) {
	tmp := t.TempDir()
	inputPath := filepath.Join(tmp, "docs.md")
	outputDir := filepath.Join(tmp, "out")
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	input := "text\n``` mermaid\nflowchart LR\nA-->B\n```\n" +
		"~~~ mermaid\nsequenceDiagram\nA->>B: hi\n~~~\n"
	if err := os.WriteFile(inputPath, []byte(input), 0o644); err != nil {
		t.Fatalf("write markdown: %v", err)
	}

	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mmdg", "-i", inputPath, "-o", outputDir, "-e", "svg"}

	if err := run(); err != nil {
		t.Fatalf("run() error = %v", err)
	}

	for _, name := range []string{"diagram-1.svg", "diagram-2.svg"} {
		path := filepath.Join(outputDir, name)
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("missing %s: %v", name, err)
		}
		if !strings.Contains(string(content), "<svg") {
			t.Fatalf("invalid SVG in %s", name)
		}
	}
}

func TestParseAspectRatioValue(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"16:9", 16.0 / 9.0},
		{"4/3", 4.0 / 3.0},
		{"1.5", 1.5},
	}
	for _, tc := range cases {
		got, err := parseAspectRatioValue(tc.input)
		if err != nil {
			t.Fatalf("parseAspectRatioValue(%q) error = %v", tc.input, err)
		}
		if got < tc.want-0.0001 || got > tc.want+0.0001 {
			t.Fatalf("ratio mismatch for %q: got %f want %f", tc.input, got, tc.want)
		}
	}
}

func TestRunHelp(t *testing.T) {
	oldArgs := os.Args
	defer func() { os.Args = oldArgs }()
	os.Args = []string{"mmdg", "--help"}

	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe() error = %v", err)
	}
	os.Stdout = w
	defer func() { os.Stdout = oldStdout }()

	runErr := run()
	_ = w.Close()
	out, _ := io.ReadAll(r)
	_ = r.Close()

	if runErr != nil {
		t.Fatalf("run() with --help error = %v", runErr)
	}
	output := string(out)
	if !strings.Contains(output, "Usage: mmdg [flags]") {
		t.Fatalf("expected usage header in help output, got: %q", output)
	}
	if !strings.Contains(output, "Render Mermaid diagrams to SVG or PNG without browser/chromium.") {
		t.Fatalf("expected help description, got: %q", output)
	}
}
