// Command mmdg renders Mermaid diagrams to SVG, PNG and PDF without a browser.
//
// It takes the options of mermaid-cli (mmdc) and writes files with the
// content of the files that mmdc writes, so that mmdg can replace mmdc in
// scripts:
//
//	mmdg -i diagram.mmd -o diagram.svg
//	mmdg -i diagram.mmd -o diagram.png -s 2
//	mmdg -i diagram.mmd -o diagram.pdf --pdf-paper-format A4
//	mmdg -i README.md -o out.svg        # one file per mermaid block: out-1.svg, out-2.svg
//	mmdg -i README.md -o README.out.md  # the markdown with images in place of the blocks
//	cat diagram.mmd | mmdg -i - -o - -e svg > diagram.svg
//
// By default mmdg reproduces the web page of mmdc, which has web fonts that
// a plain page has not, and puts the fonts that a diagram uses into the
// SVG, as mmdc does. --page plain renders as a page with only mermaid.
//
// See docs/CLI.md for every option and for the differences from mmdc.
//
// The handling of the options follows mermaid-cli 12.0.0, src/index.js
// (cli and run), checked against 12.2.0. The only change in index.js is
// a help description. The defaults and output options are unchanged.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"sync"
	"time"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// version is set by the release build.
var version = "dev"

// buildVersion returns the version of the command. A release build sets
// it. A build of "go install module@version" has the version of the module
// in its build information.
func buildVersion() string {
	if version != "dev" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return strings.TrimPrefix(v, "v")
		}
	}
	return version
}

func main() {
	if err := run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr); err != nil {
		fmt.Fprint(os.Stderr, errorText(err))
		os.Exit(1)
	}
}

// usageError is an error in the arguments. mmdc reports it through
// commander, as "error: message".
type usageError struct{ msg string }

func (e *usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return &usageError{msg: fmt.Sprintf(format, args...)}
}

// errorText is what mmdc prints to stderr for an error before it exits
// with code 1.
func errorText(err error) string {
	var usage *usageError
	if errors.As(err, &usage) {
		return "error: " + usage.msg + "\n"
	}
	return "\n" + err.Error() + "\n\n"
}

// stdoutPath is the output of mmdc for "-o -".
const stdoutPath = "/dev/stdout"

// config has the values of the options.
type config struct {
	// The options of mermaid-cli 12.
	theme, input, output, artefacts, outputFormat string
	hasInput                                      bool
	size, jobs                                    int
	backgroundColor, pdfPaperFormat               string
	configFile, cssFile, svgID                    string
	scale                                         float64
	quiet                                         bool
	iconPacks                                     []string
	// The options of mermaid-cli 11 that version 12 removed.
	width int
	// fontEmbed is false after --no-font-embed.
	fontEmbed bool
	// The options of mmdg.
	page       string
	fonts      string
	fontDirs   []string
	imageSizes string
	timeout    time.Duration
	now        string

	help, version bool
}

// option is one option of the command line.
type option struct {
	short, long string
	// arg is the name of the value. It is "" for an option without a value.
	arg string
	// variadic is true for an option that takes all the values that follow.
	variadic bool
	usage    string
	// set stores a value. It returns the reason when the value is not
	// valid.
	set func(c *config, value string) string
}

// flags is "-s, --long <arg>", as commander prints an option.
func (o *option) flags() string {
	var parts []string
	if o.short != "" {
		parts = append(parts, "-"+o.short)
	}
	if o.long != "" {
		parts = append(parts, "--"+o.long)
	}
	s := strings.Join(parts, ", ")
	if o.arg != "" {
		s += " <" + o.arg
		if o.variadic {
			s += "..."
		}
		s += ">"
	}
	return s
}

// themes are the choices of --theme in mermaid-cli 12.
var themes = []string{"base", "dark", "default", "forest", "neutral", "neo", "neo-dark", "redux", "redux-dark", "redux-color", "redux-dark-color", "null"}

var paperFormats = []string{"Letter", "Legal", "Tabloid", "Ledger", "A0", "A1", "A2", "A3", "A4", "A5", "A6"}

func choice(choices []string, store func(c *config, v string)) func(c *config, v string) string {
	return func(c *config, v string) string {
		for _, ch := range choices {
			if v == ch {
				store(c, v)
				return ""
			}
		}
		return "Allowed choices are " + strings.Join(choices, ", ") + "."
	}
}

// positiveInt is parseCommanderInt: parseInt(value, 10) of at least 1.
func positiveInt(store func(c *config, n int)) func(c *config, v string) string {
	return func(c *config, v string) string {
		n, ok := jsParseInt(v)
		if !ok || n < 1 {
			return "Not a positive integer."
		}
		store(c, n)
		return ""
	}
}

// jsParseInt is parseInt(s, 10): the digits at the start of the text, after
// white space and a sign. ok is false for NaN.
func jsParseInt(s string) (int, bool) {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	neg := false
	if s != "" && (s[0] == '+' || s[0] == '-') {
		neg = s[0] == '-'
		s = s[1:]
	}
	n, digits := 0, 0
	for digits < len(s) && s[digits] >= '0' && s[digits] <= '9' {
		if n < math.MaxInt32 {
			n = n*10 + int(s[digits]-'0')
		}
		digits++
	}
	if digits == 0 {
		return 0, false
	}
	if neg {
		n = -n
	}
	return n, true
}

// jsParseFloat is parseFloat(s): the longest prefix that is a decimal
// number. ok is false for NaN.
func jsParseFloat(s string) (float64, bool) {
	s = strings.TrimLeft(s, " \t\n\v\f\r")
	end := 0
	if end < len(s) && (s[end] == '+' || s[end] == '-') {
		end++
	}
	isDigit := func(i int) bool { return i < len(s) && s[i] >= '0' && s[i] <= '9' }
	start := end
	for isDigit(end) {
		end++
	}
	intDigits := end - start
	fracDigits := 0
	if end < len(s) && s[end] == '.' {
		p := end + 1
		for isDigit(p) {
			p++
			fracDigits++
		}
		if intDigits > 0 || fracDigits > 0 {
			end = p
		}
	}
	if intDigits == 0 && fracDigits == 0 {
		return 0, false
	}
	if end < len(s) && (s[end] == 'e' || s[end] == 'E') {
		p := end + 1
		if p < len(s) && (s[p] == '+' || s[p] == '-') {
			p++
		}
		if isDigit(p) {
			for isDigit(p) {
				p++
			}
			end = p
		}
	}
	var f float64
	if _, err := fmt.Sscanf(s[:end], "%g", &f); err != nil {
		return 0, false
	}
	return f, true
}

var outputSuffixes = []string{".svg", ".png", ".pdf", ".md", ".markdown"}

func hasSuffix(s string, suffixes ...string) bool {
	for _, suffix := range suffixes {
		if strings.HasSuffix(s, suffix) {
			return true
		}
	}
	return false
}

// isMarkdown is /\.(md|markdown)$/.test(name).
func isMarkdown(name string) bool { return hasSuffix(name, ".md", ".markdown") }

// options are the options of mmdg, in the order of the help text.
var options = []option{
	{short: "V", long: "version", usage: "Print the version.",
		set: func(c *config, _ string) string { c.version = true; return "" }},
	{short: "t", long: "theme", arg: "theme", usage: "Theme of the diagram: " + strings.Join(themes, ", ") + ". The default is the theme that mermaid selects for the diagram type.",
		set: choice(themes, func(c *config, v string) { c.theme = v })},
	{long: "size", arg: "size", usage: "Largest width and height of the diagram in px. A PNG fills this size. An SVG gets it as max-width and max-height.",
		set: positiveInt(func(c *config, n int) { c.size = n })},
	{short: "i", long: "input", arg: "input", usage: "Input file. A file that ends in .md or .markdown is Markdown: every mermaid code block is a diagram. Use - for stdin.",
		set: func(c *config, v string) string { c.input, c.hasInput = v, true; return "" }},
	{short: "o", long: "output", arg: "output", usage: "Output file. It ends in .svg, .png, .pdf, .md or .markdown. Use - for stdout. The default is the input name with .svg.",
		set: func(c *config, v string) string {
			if v == "-" {
				c.output = stdoutPath
				return ""
			}
			if !hasSuffix(v, outputSuffixes...) {
				return "Output file must be either md, svg, png, or pdf."
			}
			c.output = v
			return ""
		}},
	{short: "a", long: "artefacts", arg: "artefacts", usage: "Directory for the images of a Markdown input. The default is the directory of the output file.",
		set: func(c *config, v string) string { c.artefacts = v; return "" }},
	{short: "j", long: "jobs", arg: "jobs", usage: "Number of diagrams of a Markdown input to render at the same time. The default is half of the CPUs.",
		set: positiveInt(func(c *config, n int) { c.jobs = n })},
	{short: "e", long: "outputFormat", arg: "format", usage: "Output format: svg, png or pdf. The default comes from the name of the output file.",
		set: choice([]string{"svg", "png", "pdf"}, func(c *config, v string) { c.outputFormat = v })},
	{short: "b", long: "backgroundColor", arg: "backgroundColor", usage: "Background color of the diagram, as a CSS color: transparent, red, '#F0F0F0'. The default is white.",
		set: func(c *config, v string) string { c.backgroundColor = v; return "" }},
	{short: "c", long: "configFile", arg: "configFile", usage: "JSON file with the mermaid config: the argument of mermaid.initialize.",
		set: func(c *config, v string) string { c.configFile = v; return "" }},
	{short: "C", long: "cssFile", arg: "cssFile", usage: "CSS file for the diagram. It becomes the themeCSS of the mermaid config.",
		set: func(c *config, v string) string { c.cssFile = v; return "" }},
	{short: "I", long: "svgId", arg: "svgId", usage: "The id attribute of the svg element. The default is my-svg.",
		set: func(c *config, v string) string { c.svgID = v; return "" }},
	{short: "s", long: "scale", arg: "scale", usage: "Number of PNG pixels for each px of the diagram. The default is 1.",
		set: func(c *config, v string) string {
			f, ok := jsParseFloat(v)
			if !ok || f <= 0 {
				return "Not a positive number."
			}
			c.scale = f
			return ""
		}},
	{short: "q", long: "quiet", usage: "Do not print messages about the progress.",
		set: func(c *config, _ string) string { c.quiet = true; return "" }},
	{short: "w", long: "width", arg: "width", usage: "Width of the page in px, as in mermaid-cli 11. The default is 800.",
		set: positiveInt(func(c *config, n int) { c.width = n })},
	{short: "H", long: "height", arg: "height", usage: "Height of the page in px, as in mermaid-cli 11. It is accepted and has no effect.",
		set: positiveInt(func(*config, int) {})},
	{short: "f", long: "pdfFit", usage: "Accepted for mermaid-cli 11. The page fits the diagram unless --pdf-paper-format is set.",
		set: func(*config, string) string { return "" }},
	{long: "pdf-paper-format", arg: "format", usage: "Paper size for PDF output: Letter, Legal, Tabloid, Ledger, or A0 through A6. By default the page fits the diagram.",
		set: choice(paperFormats, func(c *config, v string) { c.pdfPaperFormat = v })},
	{long: "no-font-embed", usage: "Do not embed the fonts of the page into an SVG. The file is smaller, and it shows other fonts on a machine that has not the fonts of the page.",
		set: func(c *config, _ string) string { c.fontEmbed = false; return "" }},
	{short: "p", long: "puppeteerConfigFile", arg: "puppeteerConfigFile", usage: "Accepted for mermaid-cli and not read. mmdg uses no browser.",
		set: func(*config, string) string { return "" }},
	{long: "iconPacks", arg: "icon-packages", variadic: true, usage: "Icon packs of mermaid-cli. They are not supported.",
		set: func(c *config, v string) string { c.iconPacks = append(c.iconPacks, v); return "" }},
	{long: "iconPacksNamesAndUrls", arg: "prefix#iconsurl", variadic: true, usage: "Icon packs of mermaid-cli. They are not supported.",
		set: func(c *config, v string) string { c.iconPacks = append(c.iconPacks, v); return "" }},
	{long: "page", arg: "page", usage: "Web page that the render reproduces: mermaid-cli (the page of mmdc, which has the web fonts Recursive Variable, Open Sans and Font Awesome) or plain (a page with only mermaid). The default is mermaid-cli.",
		set: choice([]string{"mermaid-cli", "plain"}, func(c *config, v string) { c.page = v })},
	{long: "fonts", arg: "fonts", usage: "Fonts that measure the text: embedded (the fonts in mmdg, the same result on every machine) or system (the fonts of this machine, as a browser here). The default is embedded.",
		set: choice([]string{"embedded", "system"}, func(c *config, v string) { c.fonts = v })},
	{long: "fontDir", arg: "dir", usage: "Directory with more font files for --fonts embedded. Repeat the option for more directories.",
		set: func(c *config, v string) string { c.fontDirs = append(c.fontDirs, v); return "" }},
	{long: "imageSizes", arg: "file", usage: "JSON file with the size of the images that diagrams name by URL: {\"url\": [width, height]}. mmdg does not read from the network.",
		set: func(c *config, v string) string { c.imageSizes = v; return "" }},
	{long: "timeout", arg: "duration", usage: "Longest time for one diagram, for example 30s. The default is no limit.",
		set: func(c *config, v string) string {
			d, err := time.ParseDuration(v)
			if err != nil || d <= 0 {
				return "Not a positive duration, such as 30s."
			}
			c.timeout = d
			return ""
		}},
	{long: "now", arg: "time", usage: "The time that diagrams see as now, for example 2026-01-15T12:00:00Z. The default is the clock and the time zone of this machine.",
		set: func(c *config, v string) string {
			if _, err := time.Parse(time.RFC3339, v); err != nil {
				return "Not a time in the form 2026-01-15T12:00:00Z."
			}
			c.now = v
			return ""
		}},
	{short: "h", long: "help", usage: "Print this help.",
		set: func(c *config, _ string) string { c.help = true; return "" }},
}

func findOption(short, long string) *option {
	for i := range options {
		o := &options[i]
		if (short != "" && o.short == short) || (long != "" && o.long == long) {
			return o
		}
	}
	return nil
}

// parseArgs reads the arguments as commander does: "--name value",
// "--name=value", "-n value", "-nvalue", and "-ab" for two options without
// a value.
func parseArgs(args []string) (*config, error) {
	c := &config{backgroundColor: "white", scale: 1, jobs: max(runtime.NumCPU()/2, 1), fontEmbed: true, page: "mermaid-cli"}
	store := func(o *option, value string) error {
		if reason := o.set(c, value); reason != "" {
			return usagef("option '%s' argument '%s' is invalid. %s", o.flags(), value, reason)
		}
		return nil
	}
	// value takes the value of an option from the next argument.
	value := func(o *option, i *int) error {
		if *i+1 >= len(args) {
			return usagef("option '%s' argument missing", o.flags())
		}
		*i++
		if err := store(o, args[*i]); err != nil {
			return err
		}
		for o.variadic && *i+1 < len(args) && !strings.HasPrefix(args[*i+1], "-") {
			*i++
			if err := store(o, args[*i]); err != nil {
				return err
			}
		}
		return nil
	}
	var operands []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--":
			operands = append(operands, args[i+1:]...)
			i = len(args)
		case strings.HasPrefix(arg, "--"):
			name, inline, hasInline := strings.Cut(arg[2:], "=")
			o := findOption("", name)
			if o == nil {
				return nil, usagef("unknown option '%s'", arg)
			}
			switch {
			case o.arg == "" && hasInline:
				return nil, usagef("unknown option '%s'", arg)
			case o.arg == "":
				o.set(c, "")
			case hasInline:
				if err := store(o, inline); err != nil {
					return nil, err
				}
			default:
				if err := value(o, &i); err != nil {
					return nil, err
				}
			}
		case len(arg) > 1 && arg[0] == '-':
			// One or more short options. The first one that takes a value
			// takes the rest of the argument, or the next argument.
			for rest := arg[1:]; rest != ""; {
				o := findOption(rest[:1], "")
				if o == nil {
					return nil, usagef("unknown option '-%s'", rest[:1])
				}
				rest = rest[1:]
				if o.arg == "" {
					o.set(c, "")
					continue
				}
				if rest != "" {
					if err := store(o, rest); err != nil {
						return nil, err
					}
				} else if err := value(o, &i); err != nil {
					return nil, err
				}
				break
			}
		default:
			operands = append(operands, arg)
		}
	}
	if c.help || c.version {
		return c, nil
	}
	if len(operands) > 0 {
		return nil, usagef("too many arguments. Expected 0 arguments but got %d: %s.", len(operands), strings.Join(operands, ", "))
	}
	return c, nil
}

func printHelp(w io.Writer) {
	fmt.Fprintln(w, "Usage: mmdg [options]")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Render Mermaid diagrams to SVG, PNG or PDF without a browser. The options are those of mermaid-cli (mmdc).")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "Options:")
	width := 0
	for i := range options {
		width = max(width, len(options[i].flags()))
	}
	for i := range options {
		o := &options[i]
		fmt.Fprintf(w, "  %-*s  %s\n", width, o.flags(), o.usage)
	}
}

func run(args []string, stdin io.Reader, stdout, stderr io.Writer) error {
	c, err := parseArgs(args)
	if err != nil {
		return err
	}
	if c.help {
		printHelp(stdout)
		return nil
	}
	if c.version {
		fmt.Fprintf(stdout, "mmdg %s (mermaid %s)\n", buildVersion(), mermaid.MermaidVersion)
		return nil
	}
	warn := func(message string) { fmt.Fprintf(stderr, "\n%s\n\n", message) }

	// check input file
	input := c.input
	switch {
	case !c.hasInput || input == "":
		warn("No input file specified, reading from stdin. " +
			"If you want to specify an input file, please use `-i <input>.` " +
			"You can use `-i -` to read from stdin and to suppress this warning.")
		input = ""
	case input == "-":
		// `--input -` means read from stdin, but suppress the above warning
		input = ""
	default:
		if _, err := os.Stat(input); err != nil {
			return fmt.Errorf(`Input file "%s" doesn't exist`, input)
		}
	}

	// check output file
	output, outputFormat, quiet := c.output, c.outputFormat, c.quiet
	if output == "" {
		// The input file gives the name. For stdin the name is out.svg.
		format := outputFormat
		if format == "" {
			format = "svg"
		}
		if input != "" {
			output = input + "." + format
		} else {
			output = "out." + format
		}
	} else if output == stdoutPath {
		quiet = true
		if outputFormat == "" {
			outputFormat = "svg"
			warn("No output format specified, using svg. " +
				"If you want to specify an output format and suppress this warning, " +
				"please use `-e <format>.` ")
		}
	}

	if c.artefacts != "" {
		if input == "" || !isMarkdown(input) {
			return errors.New("Artefacts [-a|--artefacts] path can only be used with Markdown input file")
		}
		if err := os.MkdirAll(c.artefacts, 0o755); err != nil {
			return err
		}
	}

	if outputDir := filepath.Dir(output); output != stdoutPath {
		if _, err := os.Stat(outputDir); err != nil {
			return fmt.Errorf(`Output directory "%s/" doesn't exist`, outputDir)
		}
	}

	// check config files
	var opts []mermaid.Option
	if c.theme != "" {
		opts = append(opts, mermaid.WithTheme(c.theme))
	}
	hasThemeCSS := false
	if c.configFile != "" {
		data, err := os.ReadFile(c.configFile)
		if err != nil {
			return fmt.Errorf(`Configuration file "%s" doesn't exist`, c.configFile)
		}
		// The config file comes after the theme option, as in mmdc: a
		// theme in the file replaces the option.
		opts = append(opts, mermaid.WithConfigJSON(data))
		var keys map[string]json.RawMessage
		if json.Unmarshal(data, &keys) == nil {
			_, hasThemeCSS = keys["themeCSS"]
		}
	}
	if c.cssFile != "" {
		css, err := os.ReadFile(c.cssFile)
		if err != nil {
			return fmt.Errorf(`CSS file "%s" doesn't exist`, c.cssFile)
		}
		if hasThemeCSS {
			warn(fmt.Sprintf(`Overriding existing themeCSS in mermaidConfig with CSS from "%s"`, c.cssFile))
		}
		opts = append(opts, mermaid.WithConfig(map[string]any{"themeCSS": string(css)}))
	}
	if len(c.iconPacks) > 0 {
		return errors.New("Icon packs are not supported: --iconPacks and --iconPacksNamesAndUrls have no effect in mmdg yet")
	}
	if c.svgID != "" {
		opts = append(opts, mermaid.WithID(c.svgID))
	}

	// The window of mmdc. --size makes it a square of the size plus the
	// margins of the body.
	viewportWidth := float64(mmdc.DefaultViewportWidth)
	if c.width > 0 {
		viewportWidth = float64(c.width)
	}
	if c.size > 0 {
		viewportWidth = float64(c.size + 16)
	}
	opts = append(opts, mermaid.WithViewportWidth(viewportWidth))

	// The fonts. The options go to the renderer and to the rasterizer, and
	// the library goes to the layout of the screenshot.
	var fontOpts []mermaid.Option
	var fonts *font.Library
	switch {
	case c.fonts == "system":
		fontOpts, fonts = []mermaid.Option{mermaid.WithSystemFonts()}, font.System()
	case len(c.fontDirs) > 0:
		fontOpts, fonts = []mermaid.Option{mermaid.WithFontDirs(c.fontDirs...)}, font.EmbeddedWith(c.fontDirs...)
	default:
		fonts = font.Embedded()
	}
	// The page. mmdc renders in a page that has web fonts and style
	// sheets. They come before the fonts of the machine.
	var pageSheets []string
	if c.page == "mermaid-cli" {
		fontOpts = append(fontOpts, mermaid.WithPage(mermaid.PageMermaidCLI))
		fonts = fonts.WithWebFonts()
		pageSheets = render.PageSheets(render.PageMermaidCLI)
	}
	opts = append(opts, fontOpts...)

	if c.imageSizes != "" {
		sizes, err := readImageSizes(c.imageSizes)
		if err != nil {
			return err
		}
		opts = append(opts, mermaid.WithImageSize(func(src string) (float64, float64, bool) {
			size, ok := sizes[src]
			return size[0], size[1], ok
		}))
	}

	// A browser shows the time of the machine. --now fixes it.
	now, loc := time.Now(), time.Local
	if c.now != "" {
		now, _ = time.Parse(time.RFC3339, c.now)
		loc = now.Location()
	}
	opts = append(opts, mermaid.WithTime(now, loc))

	r := &renderer{
		opts:           opts,
		fontOpts:       fontOpts,
		page:           mmdc.Options{Background: c.backgroundColor, SetBackground: true, Size: c.size, Fonts: fonts, PageSheets: pageSheets},
		fontEmbed:      c.fontEmbed,
		viewport:       viewportWidth,
		pdfPaperFormat: c.pdfPaperFormat,
		scale:          c.scale,
		timeout:        c.timeout,
	}
	info := func(message string) {
		if !quiet {
			fmt.Fprintln(stdout, message)
		}
	}

	if outputFormat == "" {
		outputFormat = strings.TrimPrefix(extname(output), ".")
		if outputFormat == "md" || outputFormat == "markdown" {
			// fallback to svg in case no outputFormat is given and output file is MD
			outputFormat = "svg"
		}
	}
	if !hasSuffix(outputFormat, "svg", "png", "pdf") {
		return errors.New(`Error: Output format must be one of "svg", "png" or "pdf"`)
	}
	r.format = outputFormat

	definition, err := readInput(input, stdin)
	if err != nil {
		return err
	}

	if input != "" && isMarkdown(input) {
		if output == stdoutPath {
			return errors.New("Error: Cannot use `stdout` with markdown input")
		}
		return runMarkdown(r, definition, output, c.artefacts, c.jobs, info)
	}

	info("Generating single mermaid chart")
	image, err := r.render(definition)
	if err != nil {
		return err
	}
	if output == stdoutPath {
		_, err = stdout.Write(image.data)
		return err
	}
	return os.WriteFile(output, image.data, 0o644)
}

// extname is path.extname of Node.js: the extension of the last element of
// the path. A name that starts with its only dot has no extension.
func extname(path string) string {
	base := filepath.Base(path)
	i := strings.LastIndexByte(base, '.')
	if i <= 0 {
		return ""
	}
	return base[i:]
}

func readInput(input string, stdin io.Reader) (string, error) {
	if input == "" {
		if stdin == nil {
			return "", nil
		}
		data, err := io.ReadAll(stdin)
		return string(data), err
	}
	data, err := os.ReadFile(input)
	return string(data), err
}

func readImageSizes(path string) (map[string][2]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf(`Image sizes file "%s" doesn't exist`, path)
	}
	var sizes map[string][2]float64
	if err := json.Unmarshal(data, &sizes); err != nil {
		return nil, fmt.Errorf(`Image sizes file "%s" must be a JSON object from a URL to [width, height]: %v`, path, err)
	}
	return sizes, nil
}

// runMarkdown renders every mermaid block of a Markdown text to a numbered
// file. When the output is a Markdown file, it also writes the text with an
// image in place of every block.
func runMarkdown(r *renderer, definition, output, artefacts string, jobs int, info func(string)) error {
	blocks := findMermaidBlocks(definition)
	outputAbs, err := filepath.Abs(output)
	if err != nil {
		return err
	}
	type result struct {
		url   string
		image *rendered
		err   error
	}
	results := make([]result, len(blocks))
	if len(blocks) > 0 {
		info(fmt.Sprintf("Found %d mermaid charts in Markdown input", len(blocks)))
	} else {
		info("No mermaid charts found in Markdown input")
	}
	var wg sync.WaitGroup
	var infoMu sync.Mutex
	sem := make(chan struct{}, max(jobs, 1))
	for i, block := range blocks {
		// The output is the template of the numbered files: out.png gives
		// out-1.png. A Markdown output gives images of the output format:
		// out.md gives out-1.svg.
		outputFile := output
		for _, suffix := range []string{".markdown", ".md", ".png", ".svg", ".pdf"} {
			if strings.HasSuffix(output, suffix) {
				outputFile = fmt.Sprintf("%s-%d%s", strings.TrimSuffix(output, suffix), i+1, suffix)
				break
			}
		}
		for _, suffix := range []string{".markdown", ".md"} {
			if strings.HasSuffix(outputFile, suffix) {
				outputFile = strings.TrimSuffix(outputFile, suffix) + "." + r.format
				break
			}
		}
		if artefacts != "" {
			outputFile = filepath.Join(artefacts, filepath.Base(outputFile))
		}
		outputFileAbs, err := filepath.Abs(outputFile)
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(filepath.Dir(outputAbs), outputFileAbs)
		if err != nil {
			return err
		}
		results[i].url = "./" + filepath.ToSlash(rel)

		wg.Add(1)
		sem <- struct{}{}
		go func() {
			defer wg.Done()
			defer func() { <-sem }()
			res := &results[i]
			res.image, res.err = r.render(block.definition)
			if res.err != nil {
				return
			}
			if res.err = os.WriteFile(outputFile, res.image.data, 0o644); res.err != nil {
				return
			}
			infoMu.Lock()
			info(" ✅ " + res.url)
			infoMu.Unlock()
		}()
	}
	wg.Wait()
	for i := range results {
		if results[i].err != nil {
			return results[i].err
		}
	}

	if isMarkdown(output) {
		var out strings.Builder
		last := 0
		for i, block := range blocks {
			out.WriteString(definition[last:block.start])
			alt := results[i].image.desc
			if alt == "" {
				alt = "diagram"
			}
			out.WriteString(markdownImage(results[i].url, results[i].image.title, alt))
			last = block.end
		}
		out.WriteString(definition[last:])
		if err := os.WriteFile(output, []byte(out.String()), 0o644); err != nil {
			return err
		}
		info(" ✅ " + output)
	}
	return nil
}

// markdownImage is markdownImage of mmdc: the Markdown of an image,
// ![alt](url "title").
func markdownImage(url, title, alt string) string {
	altEscaped := strings.NewReplacer("[", `\[`, "]", `\]`, `\`, `\\`).Replace(alt)
	if title != "" {
		titleEscaped := strings.NewReplacer(`"`, `\"`, `\`, `\\`).Replace(title)
		return fmt.Sprintf(`![%s](%s "%s")`, altEscaped, url, titleEscaped)
	}
	return fmt.Sprintf("![%s](%s)", altEscaped, url)
}

// mermaidBlock is one mermaid code block of a Markdown text.
type mermaidBlock struct {
	// start and end are the byte range of the block in the text.
	start, end int
	// definition is the text between the fences.
	definition string
}

// jsSpace reports whether r is white space for \s in a regular expression
// of JavaScript.
func jsSpace(r rune) bool {
	switch r {
	case '\t', '\n', '\v', '\f', '\r', ' ', 0xa0, 0x1680, 0x2028, 0x2029, 0x202f, 0x205f, 0x3000, 0xfeff:
		return true
	}
	return r >= 0x2000 && r <= 0x200a
}

// lineTerminator reports whether ^ and $ of a multiline regular expression
// of JavaScript match next to r.
func lineTerminator(r rune) bool {
	return r == '\n' || r == '\r' || r == 0x2028 || r == 0x2029
}

// findMermaidBlocks finds what the regular expression of mmdc finds, with
// the flags g and m:
//
//	/^[^\S\n]*[`:]{3}(?:mermaid)([^\S\n]*\r?\n([\s\S]*?))[`:]{3}[^\S\n]*$/
//
// The regular expressions of Go have other rules for \s, ^ and $, so the
// search is written out.
func findMermaidBlocks(markdown string) []mermaidBlock {
	rs := []rune(markdown)
	// offset has the byte offset of every rune, and the length of the text.
	offset := make([]int, len(rs)+1)
	n := 0
	for i, r := range rs {
		offset[i] = n
		n += len(string(r))
	}
	offset[len(rs)] = n

	// blank skips [^\S\n]*.
	blank := func(i int) int {
		for i < len(rs) && rs[i] != '\n' && jsSpace(rs[i]) {
			i++
		}
		return i
	}
	// fence matches [`:]{3}.
	fence := func(i int) bool {
		if i+3 > len(rs) {
			return false
		}
		for _, r := range rs[i : i+3] {
			if r != '`' && r != ':' {
				return false
			}
		}
		return true
	}
	const word = "mermaid"

	var blocks []mermaidBlock
	for pos := 0; pos < len(rs); pos++ {
		// ^
		if pos > 0 && !lineTerminator(rs[pos-1]) {
			continue
		}
		i := blank(pos)
		if !fence(i) || i+3+len(word) > len(rs) || string(rs[i+3:i+3+len(word)]) != word {
			continue
		}
		// [^\S\n]*\r?\n: the class has \r, so the line feed must follow.
		i = blank(i + 3 + len(word))
		if i >= len(rs) || rs[i] != '\n' {
			continue
		}
		body := i + 1
		// [\s\S]*? takes as little as it can: the first fence that the end
		// of a line follows closes the block.
		for q := body; q+3 <= len(rs); q++ {
			if !fence(q) {
				continue
			}
			// [^\S\n]*$ takes as much as it can, and gives back until $
			// matches: at the end of the text or before a line terminator.
			end := -1
			for k := blank(q + 3); k >= q+3; k-- {
				if k == len(rs) || lineTerminator(rs[k]) {
					end = k
					break
				}
			}
			if end < 0 {
				continue
			}
			blocks = append(blocks, mermaidBlock{start: offset[pos], end: offset[end], definition: string(rs[body:q])})
			// The next search starts where this match ends.
			pos = end - 1
			break
		}
	}
	return blocks
}

// renderer renders one diagram to the bytes of an output file.
type renderer struct {
	// opts are the options of the render. fontOpts are those of them that
	// the rasterizer also needs: the fonts and the page.
	opts, fontOpts []mermaid.Option
	page           mmdc.Options
	// fontEmbed is true to embed the fonts of the page into an SVG.
	fontEmbed              bool
	viewport               float64
	scale                  float64
	format, pdfPaperFormat string
	timeout                time.Duration
}

// rendered is an output file and the accessible text of its diagram.
type rendered struct {
	data        []byte
	title, desc string
}

func (r *renderer) render(definition string) (*rendered, error) {
	ctx := context.Background()
	if r.timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, r.timeout)
		defer cancel()
	}
	res, err := mermaid.RenderContext(ctx, definition, r.opts...)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, fmt.Errorf("The diagram did not finish in %s (--timeout)", r.timeout)
		}
		return nil, err
	}
	page, err := mmdc.Load(res.SVG, r.page)
	if err != nil {
		return nil, err
	}
	out := &rendered{title: page.Title(), desc: page.Desc()}
	switch r.format {
	case "pdf":
		out.data, err = page.PDF(r.viewport, r.pdfPaperFormat)
		return out, err
	case "png":
		out.data, err = r.png(page)
		return out, err
	}
	if r.fontEmbed {
		// For SVGs, mmdc embeds the fonts of its page into the SVG.
		if err := page.EmbedFonts(); err != nil {
			return nil, errors.New("Error: " + err.Error())
		}
	}
	out.data = []byte(page.XML())
	return out, nil
}

// png is the screenshot of mmdc: an image of the size of the svg element in
// the page, with the diagram at the size that it has when mmdc takes the
// screenshot.
func (r *renderer) png(page *mmdc.Page) ([]byte, error) {
	shot, err := page.Screenshot(r.viewport)
	if err != nil {
		return nil, err
	}
	var background color.Color = shot.Diagram
	if shot.Transparent {
		background = color.Transparent
	}
	opts := append([]mermaid.Option{mermaid.WithScale(r.scale), mermaid.WithBackground(background)}, r.fontOpts...)
	data, err := mermaid.SVGToPNG(shot.SVG, opts...)
	if err != nil {
		return nil, err
	}
	diagram, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	width := int(math.Round(float64(shot.Width) * r.scale))
	height := int(math.Round(float64(shot.Height) * r.scale))
	x := int(math.Round(shot.Box.X * r.scale))
	y := int(math.Round(shot.Box.Y * r.scale))
	if x == 0 && y == 0 && diagram.Bounds().Dx() == width && diagram.Bounds().Dy() == height {
		return data, nil
	}
	// The page shows around the diagram.
	canvas := image.NewNRGBA(image.Rect(0, 0, width, height))
	if !shot.Transparent {
		draw.Draw(canvas, canvas.Bounds(), image.NewUniform(shot.Page), image.Point{}, draw.Src)
	}
	draw.Draw(canvas, diagram.Bounds().Sub(diagram.Bounds().Min).Add(image.Pt(x, y)), diagram, diagram.Bounds().Min, draw.Src)
	var buf bytes.Buffer
	if err := png.Encode(&buf, canvas); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
