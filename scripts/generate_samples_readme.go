package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	// Build mmdg first
	fmt.Println("Building mmdg...")
	cmd := exec.Command("go", "build", "-o", "bin/mmdg", "./cmd/mmdg")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		fmt.Printf("Failed to build mmdg: %v\n", err)
		os.Exit(1)
	}

	samplesDir := "samples"
	entries, err := os.ReadDir(samplesDir)
	if err != nil {
		fmt.Printf("Failed to read samples dir: %v\n", err)
		os.Exit(1)
	}

	var mmdFiles []string
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".mmd") {
			mmdFiles = append(mmdFiles, entry.Name())
		}
	}
	sort.Strings(mmdFiles)

	var readme bytes.Buffer
	readme.WriteString("# mmdg Samples\n\n")
	readme.WriteString("This directory contains a suite of samples. The following table compares GitHub's native Mermaid rendering (left) with `mmdg` generated PNGs (right).\n\n")
	readme.WriteString("*(Note: To regenerate these, run `go run scripts/generate_samples_readme.go` from the root)*\n\n")

	for _, mmdFile := range mmdFiles {
		base := strings.TrimSuffix(mmdFile, ".mmd")
		inPath := filepath.Join(samplesDir, mmdFile)
		outPng := filepath.Join(samplesDir, base+".png")

		fmt.Printf("Generating %s...\n", outPng)
		cmd := exec.Command("bin/mmdg", "-i", inPath, "-o", outPng, "-e", "png", "--allowApproximate")
		if out, err := cmd.CombinedOutput(); err != nil {
			fmt.Printf("Failed to generate %s: %v\n%s\n", outPng, err, out)
		}

		content, err := os.ReadFile(inPath)
		if err != nil {
			fmt.Printf("Failed to read %s: %v\n", inPath, err)
			continue
		}

		// Use GitHub HTML table + empty lines for markdown parsing
		readme.WriteString(fmt.Sprintf("## %s\n\n", mmdFile))
		readme.WriteString("<table>\n<tr>\n<th width=\"50%%\">GitHub Native (Mermaid JS)</th>\n<th width=\"50%%\"><code>mmdg</code> PNG</th>\n</tr>\n<tr>\n<td>\n\n```mermaid\n")
		readme.WriteString(strings.TrimSpace(string(content)))
		readme.WriteString("\n```\n\n</td>\n<td>\n\n")
		readme.WriteString(fmt.Sprintf("<img src=\"%s.png\" style=\"max-width:100%%; background:white;\" />\n\n", base))
		readme.WriteString("</td>\n</tr>\n</table>\n\n")
	}

	readmePath := filepath.Join(samplesDir, "README.md")
	if err := os.WriteFile(readmePath, readme.Bytes(), 0644); err != nil {
		fmt.Printf("Failed to write %s: %v\n", readmePath, err)
		os.Exit(1)
	}
	fmt.Printf("Successfully updated %s\n", readmePath)
}
