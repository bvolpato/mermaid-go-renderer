package mermaid

import (
	"errors"
	"os"
)

func WriteOutputSVG(svg string, outputPath string) error {
	if outputPath == "" {
		_, err := os.Stdout.WriteString(svg)
		return err
	}
	return os.WriteFile(outputPath, []byte(svg), 0o644)
}

func WriteOutputPNG(_ string, _ string) error {
	return errors.New("PNG output is not implemented yet; use SVG output")
}
