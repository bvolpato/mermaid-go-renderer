package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	var (
		inputPath            string
		outputPath           string
		outputFormat         string
		width                float64
		height               float64
		preferredAspectRatio string
		nodeSpacing          float64
		rankSpacing          float64
		timing               bool
		fastText             bool
		allowApproximate     bool
	)

	fs := flag.NewFlagSet("mmdg", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	fs.StringVar(&inputPath, "i", "", "input file (.mmd/.md) or '-' for stdin")
	fs.StringVar(&outputPath, "o", "", "output file path")
	fs.StringVar(&outputFormat, "e", "svg", "output format: svg|png")
	fs.Float64Var(&width, "w", 1200, "render width (reserved)")
	fs.Float64Var(&height, "H", 800, "render height (reserved)")
	fs.StringVar(&preferredAspectRatio, "preferredAspectRatio", "", "preferred ratio: 16:9, 4/3, or decimal")
	fs.Float64Var(&nodeSpacing, "nodeSpacing", 0, "node spacing")
	fs.Float64Var(&rankSpacing, "rankSpacing", 0, "rank spacing")
	fs.BoolVar(&timing, "timing", false, "print timing as JSON to stderr")
	fs.BoolVar(&fastText, "fastText", false, "use fast text width approximation")
	fs.BoolVar(&allowApproximate, "allowApproximate", false, "allow rendering for experimental low-fidelity diagram families")
	if err := fs.Parse(os.Args[1:]); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(fs)
			return nil
		}
		return err
	}

	_ = width
	_ = height

	input, markdown, err := readInput(inputPath)
	if err != nil {
		return err
	}

	options := mermaid.DefaultRenderOptions()
	if nodeSpacing > 0 {
		options = options.WithNodeSpacing(nodeSpacing)
	}
	if rankSpacing > 0 {
		options = options.WithRankSpacing(rankSpacing)
	}
	if preferredAspectRatio != "" {
		ratio, parseErr := parseAspectRatioValue(preferredAspectRatio)
		if parseErr != nil {
			return parseErr
		}
		options = options.WithPreferredAspectRatio(ratio)
	}
	options.Layout.FastTextMetrics = fastText
	options.Layout.AllowApproximate = allowApproximate

	switch lower(outputFormat) {
	case "svg", "png":
	default:
		return fmt.Errorf("unsupported output format %q", outputFormat)
	}

	diagrams := []string{input}
	if markdown {
		diagrams = mermaid.ExtractMermaidBlocks(input)
		if len(diagrams) == 0 {
			return errors.New("no Mermaid blocks found in markdown input")
		}
	}

	if len(diagrams) == 1 {
		return renderOne(diagrams[0], outputPath, outputFormat, options, timing)
	}

	if outputPath == "" {
		return errors.New("output path is required when rendering multiple diagrams")
	}
	ext := strings.ToLower(outputFormat)
	outputs, err := mermaid.ResolveMultiOutputs(outputPath, ext, len(diagrams))
	if err != nil {
		return err
	}
	for i, diagram := range diagrams {
		if err := renderOne(diagram, outputs[i], outputFormat, options, false); err != nil {
			return fmt.Errorf("diagram %d: %w", i+1, err)
		}
	}
	return nil
}

func printUsage(fs *flag.FlagSet) {
	fmt.Fprintln(os.Stdout, "Usage: mmdg [flags]")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Render Mermaid diagrams to SVG without browser/chromium.")
	fmt.Fprintln(os.Stdout)
	fmt.Fprintln(os.Stdout, "Flags:")
	fs.SetOutput(os.Stdout)
	fs.PrintDefaults()
	fs.SetOutput(io.Discard)
}

func renderOne(diagram, outputPath, outputFormat string, options mermaid.RenderOptions, timing bool) error {
	if timing {
		result, err := mermaid.RenderWithTiming(diagram, options)
		if err != nil {
			return err
		}
		if err := writeOutput(result.SVG, outputPath, outputFormat); err != nil {
			return err
		}
		payload, _ := json.Marshal(map[string]any{
			"parse_us":  result.ParseUS,
			"layout_us": result.LayoutUS,
			"render_us": result.RenderUS,
			"total_us":  result.TotalUS(),
		})
		fmt.Fprintln(os.Stderr, string(payload))
		return nil
	}

	svg, err := mermaid.RenderWithOptions(diagram, options)
	if err != nil {
		return err
	}
	return writeOutput(svg, outputPath, outputFormat)
}

func writeOutput(svg, outputPath, outputFormat string) error {
	switch lower(outputFormat) {
	case "svg":
		return mermaid.WriteOutputSVG(svg, outputPath)
	case "png":
		return mermaid.WriteOutputPNG(svg, outputPath)
	default:
		return fmt.Errorf("unsupported output format %q", outputFormat)
	}
}

func readInput(path string) (content string, markdown bool, err error) {
	if path == "" || path == "-" {
		data, readErr := io.ReadAll(os.Stdin)
		if readErr != nil {
			return "", false, readErr
		}
		return string(data), false, nil
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		return "", false, readErr
	}
	ext := lower(filepath.Ext(path))
	return string(data), ext == ".md" || ext == ".markdown", nil
}

func parseAspectRatioValue(raw string) (float64, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return 0, errors.New("aspect ratio cannot be empty")
	}
	parsePair := func(left, right string) (float64, error) {
		w, ok := parseFloat(left)
		if !ok || w <= 0 {
			return 0, errors.New("invalid aspect ratio width")
		}
		h, ok := parseFloat(right)
		if !ok || h <= 0 {
			return 0, errors.New("invalid aspect ratio height")
		}
		return w / h, nil
	}
	if parts := strings.SplitN(value, ":", 2); len(parts) == 2 {
		return parsePair(parts[0], parts[1])
	}
	if parts := strings.SplitN(value, "/", 2); len(parts) == 2 {
		return parsePair(parts[0], parts[1])
	}
	ratio, ok := parseFloat(value)
	if !ok || ratio <= 0 {
		return 0, errors.New("invalid aspect ratio")
	}
	return ratio, nil
}

func lower(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func parseFloat(raw string) (float64, bool) {
	value, err := strconv.ParseFloat(strings.TrimSpace(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return 0, false
	}
	return value, true
}
