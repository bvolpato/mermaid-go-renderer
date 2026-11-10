package main

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"
)

// runIn runs mmdg in a new directory that has the given files.
func runIn(t *testing.T, files map[string]string, stdin string, args ...string) (stdout, stderr string, err error) {
	t.Helper()
	dir := t.TempDir()
	for name, text := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	t.Chdir(dir)
	var out, errOut bytes.Buffer
	err = run(args, strings.NewReader(stdin), &out, &errOut)
	return out.String(), errOut.String(), err
}

func TestStdinToStdout(t *testing.T) {
	out, errOut, err := runIn(t, nil, "info\n", "-i", "-", "-o", "-", "-e", "svg")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(out, `<svg id="my-svg"`) || !strings.Contains(out, "background-color: white;") {
		t.Fatalf("unexpected output: %.200s", out)
	}
	if errOut != "" {
		t.Fatalf("unexpected message: %q", errOut)
	}
}

func TestMarkdownInput(t *testing.T) {
	text := "# Title\n\n```mermaid\ninfo\n```\n\ntext\n\n```mermaid\ninfo showInfo\n```\n"
	if _, _, err := runIn(t, map[string]string{"doc.md": text}, "", "-i", "doc.md", "-o", "out.svg", "-q", "--svgId", "x"); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"out-1.svg", "out-2.svg"} {
		data, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(data), `<svg id="x"`) {
			t.Fatalf("%s: unexpected output: %.120s", name, data)
		}
	}
}

// TestOptions checks the handling of the options: the forms of an option,
// the checks, and what each option does to the output.
func TestOptions(t *testing.T) {
	files := map[string]string{
		"d.mmd":       "flowchart LR\n  A --> B\n",
		"config.json": `{"theme":"forest","themeCSS":".x{fill:red}"}`,
		"style.css":   ".node rect { fill: #abcdef; }\n",
		"sizes.json":  `{"https://example.com/a.png": [40, 30]}`,
		"bad.json":    `[1, 2]`,
	}
	for _, c := range []struct {
		name string
		args []string
		// wantErr is a part of the error text. "" means success.
		wantErr string
		// file is the output file to read. contains lists texts that it
		// must have, and lacks lists texts that it must not have.
		file     string
		contains []string
		lacks    []string
		stderr   string
	}{
		{name: "long options", args: []string{"--input", "d.mmd", "--output", "o.svg", "--quiet"}, file: "o.svg", contains: []string{`<svg id="my-svg"`}},
		{name: "long options with =", args: []string{"--input=d.mmd", "--output=o.svg", "--svgId=k"}, file: "o.svg", contains: []string{`<svg id="k"`}},
		{name: "short options with the value attached", args: []string{"-id.mmd", "-oo.svg", "-Ik"}, file: "o.svg", contains: []string{`<svg id="k"`}},
		{name: "short options together", args: []string{"-qi", "d.mmd", "-o", "o.svg"}, file: "o.svg"},
		{name: "default output name", args: []string{"-i", "d.mmd"}, file: "d.mmd.svg"},
		{name: "default output name with a format", args: []string{"-i", "d.mmd", "-e", "png"}, file: "d.mmd.png", contains: []string{"\x89PNG"}},
		{name: "the format option wins over the file name", args: []string{"-i", "d.mmd", "-o", "o.svg", "-e", "png"}, file: "o.svg", contains: []string{"\x89PNG"}},
		{name: "background", args: []string{"-i", "d.mmd", "-o", "o.svg", "-b", "#F0F0F0"}, file: "o.svg", contains: []string{"background-color: rgb(240, 240, 240);"}},
		{name: "background transparent", args: []string{"-i", "d.mmd", "-o", "o.svg", "-b", "transparent"}, file: "o.svg", contains: []string{"background-color: transparent;"}},
		{name: "theme", args: []string{"-i", "d.mmd", "-o", "o.svg", "-t", "dark"}, file: "o.svg", contains: []string{"fill:#ccc;"}},
		{name: "config file replaces the theme option", args: []string{"-i", "d.mmd", "-o", "o.svg", "-t", "dark", "-c", "config.json"}, file: "o.svg", contains: []string{"fill:#000000;", "#my-svg .x{fill:red;}"}},
		{name: "css file", args: []string{"-i", "d.mmd", "-o", "o.svg", "-C", "style.css"}, file: "o.svg", contains: []string{"#my-svg .node rect{fill:rgb(171, 205, 239);}"}},
		{name: "css file replaces themeCSS", args: []string{"-i", "d.mmd", "-o", "o.svg", "-C", "style.css", "-c", "config.json"}, file: "o.svg", contains: []string{"#my-svg .node rect{fill:rgb(171, 205, 239);}"},
			stderr: "\nOverriding existing themeCSS in mermaidConfig with CSS from \"style.css\"\n\n"},
		{name: "size", args: []string{"-i", "d.mmd", "-o", "o.svg", "--size", "300"}, file: "o.svg", contains: []string{"max-width: 300px;", "max-height: 300px;"}},
		{name: "width and height of mermaid-cli 11", args: []string{"-i", "d.mmd", "-o", "o.svg", "-w", "1024", "-H", "768", "-f"}, file: "o.svg"},
		{name: "options without effect", args: []string{"-i", "d.mmd", "-o", "o.svg", "-p", "missing.json", "--pdf-paper-format", "A4", "-j", "2"}, file: "o.svg"},
		{name: "fonts", args: []string{"-i", "d.mmd", "-o", "o.svg", "--fonts", "embedded", "--fontDir", ".", "--fontDir", "."}, file: "o.svg"},
		{name: "image sizes", args: []string{"-i", "d.mmd", "-o", "o.svg", "--imageSizes", "sizes.json"}, file: "o.svg"},
		{name: "timeout", args: []string{"-i", "d.mmd", "-o", "o.svg", "--timeout", "1m"}, file: "o.svg"},
		{name: "scale", args: []string{"-i", "d.mmd", "-o", "o.png", "-s", "2"}, file: "o.png", contains: []string{"\x89PNG"}},
		// The default theme of a flowchart names Recursive Variable, a font
		// of the page of mermaid-cli: the SVG gets the font.
		{name: "fonts of the page are embedded", args: []string{"-i", "d.mmd", "-o", "o.svg"}, file: "o.svg",
			contains: []string{"</style><style>/* Copyright 2019 The Recursive Project Authors", `src: url("data:font/woff2;base64,`}},
		{name: "no font embed", args: []string{"-i", "d.mmd", "-o", "o.svg", "--no-font-embed"}, file: "o.svg", lacks: []string{"</style><style>", "base64"}},
		{name: "page mermaid-cli", args: []string{"-i", "d.mmd", "-o", "o.svg", "--page", "mermaid-cli", "-t", "default"}, file: "o.svg", lacks: []string{"</style><style>"}},
		{name: "page plain", args: []string{"-i", "d.mmd", "-o", "o.svg", "--page", "plain"}, file: "o.svg", lacks: []string{"</style><style>", "base64"}},
		{name: "page plain as PNG", args: []string{"-i", "d.mmd", "-o", "o.png", "--page=plain"}, file: "o.png", contains: []string{"\x89PNG"}},

		{name: "pdf", args: []string{"-i", "d.mmd", "-o", "o.pdf"}, file: "o.pdf", contains: []string{"%PDF-"}},
		{name: "pdf by the format option", args: []string{"-i", "d.mmd", "-e", "pdf", "--pdf-paper-format", "Letter"}, file: "d.mmd.pdf", contains: []string{"%PDF-"}},
		{name: "icon packs", args: []string{"-i", "d.mmd", "--iconPacks", "@iconify-json/logos", "@iconify-json/mdi", "-q"}, wantErr: "Icon packs are not supported"},
		{name: "icon packs by url", args: []string{"-i", "d.mmd", "--iconPacksNamesAndUrls", "azure#file:///tmp/icons.json"}, wantErr: "Icon packs are not supported"},
		{name: "missing value", args: []string{"-i", "d.mmd", "-o"}, wantErr: "option '-o, --output <output>' argument missing"},
		{name: "value for an option without a value", args: []string{"-i", "d.mmd", "--quiet=yes"}, wantErr: "unknown option '--quiet=yes'"},
		{name: "fonts choice", args: []string{"-i", "d.mmd", "--fonts", "web"}, wantErr: "Allowed choices are embedded, system."},
		{name: "page choice", args: []string{"-i", "d.mmd", "--page", "mmdc"}, wantErr: "Allowed choices are mermaid-cli, plain."},
		{name: "paper format choice", args: []string{"-i", "d.mmd", "--pdf-paper-format", "B5"}, wantErr: "Allowed choices are Letter, Legal"},
		{name: "timeout value", args: []string{"-i", "d.mmd", "--timeout", "soon"}, wantErr: "Not a positive duration"},
		{name: "timeout reached", args: []string{"-i", "d.mmd", "--timeout", "1ns"}, wantErr: "did not finish in 1ns"},
		{name: "now value", args: []string{"-i", "d.mmd", "--now", "today"}, wantErr: "Not a time in the form"},
		{name: "image sizes missing", args: []string{"-i", "d.mmd", "--imageSizes", "none.json"}, wantErr: `Image sizes file "none.json" doesn't exist`},
		{name: "image sizes not an object", args: []string{"-i", "d.mmd", "--imageSizes", "bad.json"}, wantErr: "must be a JSON object"},
		{name: "config file not an object", args: []string{"-i", "d.mmd", "-c", "bad.json"}, wantErr: "the config must be a JSON object"},
		{name: "diagram with an error", args: []string{"-i", "config.json", "-o", "o.svg"}, wantErr: "No diagram type detected"},
	} {
		t.Run(c.name, func(t *testing.T) {
			_, stderr, err := runIn(t, files, "", c.args...)
			if c.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), c.wantErr) {
					t.Fatalf("error %v, want one with %q", err, c.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if stderr != c.stderr {
				t.Errorf("stderr %q, want %q", stderr, c.stderr)
			}
			data, err := os.ReadFile(c.file)
			if err != nil {
				t.Fatal(err)
			}
			for _, text := range c.contains {
				if !strings.Contains(string(data), text) {
					t.Errorf("%s has no %q: %.300s", c.file, text, data)
				}
			}
			for _, text := range c.lacks {
				if strings.Contains(string(data), text) {
					t.Errorf("%s has %q", c.file, text)
				}
			}
		})
	}
}

func TestHelpAndVersion(t *testing.T) {
	out, _, err := runIn(t, nil, "", "--help")
	if err != nil {
		t.Fatal(err)
	}
	// Every option has one line in the help.
	for i := range options {
		o := &options[i]
		found := 0
		for _, line := range strings.Split(out, "\n") {
			if strings.HasPrefix(line, "  "+o.flags()+" ") {
				found++
			}
		}
		if found != 1 || o.usage == "" {
			t.Errorf("the help has %d lines for %s", found, o.flags())
		}
	}
	out, _, err = runIn(t, nil, "", "-V", "-i", "missing.mmd")
	if err != nil || !strings.HasPrefix(out, "mmdg ") || !strings.Contains(out, "mermaid 12.2.0") {
		t.Errorf("version: %q, %v", out, err)
	}
}

// TestNow checks that --now fixes the time that a diagram shows, and that
// the default is the clock of the machine.
func TestNow(t *testing.T) {
	files := map[string]string{"g.mmd": "gantt\n  dateFormat YYYY-MM-DD\n  section S\n  A task :a1, 2026-01-01, 30d\n"}
	render := func(args ...string) string {
		out, _, err := runIn(t, files, "", append([]string{"-i", "g.mmd", "-o", "-", "-e", "svg"}, args...)...)
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	a := render("--now", "2026-01-15T12:00:00Z")
	b := render("--now", "2026-01-15T12:00:00Z")
	c := render("--now", "2026-01-20T12:00:00Z")
	if a != b {
		t.Error("two renders with the same --now differ")
	}
	if a == c {
		t.Error("the today marker does not follow --now")
	}
	if time.Now().Year() != 2026 || time.Now().Month() != time.January {
		if render() == a {
			t.Error("without --now the diagram does not show the time of the machine")
		}
	}
}

func TestFindMermaidBlocks(t *testing.T) {
	for _, c := range []struct {
		name     string
		markdown string
		want     []string
		// rest is the text with "<>" in place of every block.
		rest string
	}{
		{"backticks", "a\n```mermaid\ninfo\n```\nb\n", []string{"info\n"}, "a\n<>\nb\n"},
		{"colons", ":::mermaid\ninfo\n:::\n", []string{"info\n"}, "<>\n"},
		{"mixed fence characters", "`:`mermaid\ninfo\n:`:\n", []string{"info\n"}, "<>\n"},
		{"indent and spaces after the fences", "  \t```mermaid \t\ninfo\n```  \nb", []string{"info\n"}, "<>\nb"},
		{"carriage returns", "a\r\n```mermaid\r\ninfo\r\n```\r\nb", []string{"info\r\n"}, "a\r\n<>\nb"},
		{"end of the text", "```mermaid\ninfo\n```", []string{"info\n"}, "<>"},
		{"two blocks", "```mermaid\na\n```\n\n```mermaid\nb\n```\n", []string{"a\n", "b\n"}, "<>\n\n<>\n"},
		{"empty block", "```mermaid\n```\n", []string{""}, "<>\n"},
		{"the fence can end a line of the diagram", "```mermaid\nA --> B```\nx\n```\n", []string{"A --> B"}, "<>\nx\n```\n"},
		{"a fence inside a line does not close", "```mermaid\nA:::cls --> B\n```\n", []string{"A:::cls --> B\n"}, "<>\n"},
		{"no line start", "x ```mermaid\ninfo\n```\n", nil, "x ```mermaid\ninfo\n```\n"},
		{"other language", "```js\ninfo\n```\n", nil, "```js\ninfo\n```\n"},
		{"text after the word", "```mermaid x\ninfo\n```\n", nil, "```mermaid x\ninfo\n```\n"},
		{"not closed", "```mermaid\ninfo\n", nil, "```mermaid\ninfo\n"},
		{"byte order mark and no-break space", "\ufeff\u00a0```mermaid\u00a0\ninfo\n```\u2003\n", []string{"info\n"}, "<>\n"},
		{"line separator ends a line", "a\u2028```mermaid\ninfo\n```\u2028b", []string{"info\n"}, "a\u2028<>\u2028b"},
		{"text of several bytes before a block", "é😀\n```mermaid\ninfo\n```\n", []string{"info\n"}, "é😀\n<>\n"},
	} {
		t.Run(c.name, func(t *testing.T) {
			blocks := findMermaidBlocks(c.markdown)
			var got []string
			var rest strings.Builder
			last := 0
			for _, b := range blocks {
				got = append(got, b.definition)
				rest.WriteString(c.markdown[last:b.start] + "<>")
				last = b.end
			}
			rest.WriteString(c.markdown[last:])
			if !reflect.DeepEqual(got, c.want) {
				t.Errorf("blocks %q, want %q", got, c.want)
			}
			if rest.String() != c.rest {
				t.Errorf("rest %q, want %q", rest.String(), c.rest)
			}
		})
	}
}

func TestJSNumbers(t *testing.T) {
	for _, c := range []struct {
		in   string
		want int
		ok   bool
	}{{"12", 12, true}, {" 7px", 7, true}, {"1.9", 1, true}, {"-3", -3, true}, {"+4", 4, true}, {"x", 0, false}, {"", 0, false}, {"1e3", 1, true}} {
		if got, ok := jsParseInt(c.in); got != c.want || ok != c.ok {
			t.Errorf("parseInt(%q) = %d, %v, want %d, %v", c.in, got, ok, c.want, c.ok)
		}
	}
	for _, c := range []struct {
		in   string
		want float64
		ok   bool
	}{{"2", 2, true}, {"1.5x", 1.5, true}, {".5", 0.5, true}, {"1e2", 100, true}, {"1e", 1, true}, {" 3.", 3, true}, {"-2", -2, true}, {"abc", 0, false}, {".", 0, false}, {"", 0, false}} {
		if got, ok := jsParseFloat(c.in); got != c.want || ok != c.ok {
			t.Errorf("parseFloat(%q) = %v, %v, want %v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestMarkdownImage(t *testing.T) {
	if got, want := markdownImage("./a-1.svg", `T "q" \x`, `d [x] \y`), `![d \[x\] \\y](./a-1.svg "T \"q\" \\x")`; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
	if got, want := markdownImage("./a-1.svg", "", "diagram"), "![diagram](./a-1.svg)"; got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}
