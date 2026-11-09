package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"image/png"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

// The golden files of these tests come from real mermaid-cli (mmdc) runs.
// tools/reference/gen-mmdc-golden.mjs writes them.

var updateMmdc = flag.Bool("update-mmdc", false, "rewrite testdata/mmdc/corpus.differs.txt")

func loadGolden(t *testing.T, name string, v any) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", "mmdc", name))
	if err != nil {
		t.Skipf("no golden file: run tools/reference/gen-mmdc-golden.mjs (%v)", err)
	}
	if err := json.Unmarshal(data, v); err != nil {
		t.Fatal(err)
	}
}

// referenceFonts reports whether this machine has the fonts of the machine
// that ran mmdc for the golden files. mmdc measures text with the fonts of
// its machine.
func referenceFonts(t *testing.T) bool {
	t.Helper()
	m, err := parity.LoadManifest(filepath.Join("..", "..", "testdata", "reference", "macos.json"))
	if err != nil {
		t.Fatal(err)
	}
	return parity.SystemFontFingerprint() == m.FontFingerprint
}

func hash16(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])[:16]
}

func sortedLines(s string) []string {
	lines := strings.Split(s, "\n")
	sort.Strings(lines)
	return lines
}

// TestCLIAgainstMmdc runs mmdg with the arguments of recorded mmdc runs and
// compares the exit code, the messages, the names of the output files and
// the text of the Markdown output.
func TestCLIAgainstMmdc(t *testing.T) {
	var golden struct {
		Small    string `json:"small"`
		Markdown string `json:"markdown"`
		Cases    []struct {
			Args       []string          `json:"args"`
			Stdin      *string           `json:"stdin"`
			Code       int               `json:"code"`
			Stdout     *string           `json:"stdout"`
			StdoutHash string            `json:"stdoutHash"`
			Stderr     string            `json:"stderr"`
			Files      map[string]string `json:"files"`
			Exists     map[string]bool   `json:"exists"`
		} `json:"cases"`
	}
	loadGolden(t, "cli.json", &golden)
	if len(golden.Cases) < 20 {
		t.Fatalf("only %d cases", len(golden.Cases))
	}
	sameFonts := referenceFonts(t)
	for _, c := range golden.Cases {
		t.Run(strings.Join(c.Args, " "), func(t *testing.T) {
			dir := t.TempDir()
			for name, text := range map[string]string{
				"small.mmd":   golden.Small,
				"doc.md":      golden.Markdown,
				"none.md":     "# no charts\n\ntext\n",
				"config.json": `{"theme":"forest"}`,
				"style.css":   ".node rect { fill: #fdd !important; }\n",
			} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte(text), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			t.Chdir(dir)
			args := append([]string{}, c.Args...)
			if sameFonts {
				args = append(args, "--fonts", "system")
			}
			stdin := ""
			if c.Stdin != nil {
				stdin = *c.Stdin
			}
			var stdout, stderr bytes.Buffer
			err := run(args, strings.NewReader(stdin), &stdout, &stderr)
			code := 0
			if err != nil {
				code = 1
				stderr.WriteString(errorText(err))
			}
			if code != c.Code {
				t.Errorf("exit code %d, mmdc has %d (%v)", code, c.Code, err)
			}
			if stderr.String() != c.Stderr {
				t.Errorf("stderr\n got: %q\nmmdc: %q", stderr.String(), c.Stderr)
			}
			// mmdc prints the names of the images of a Markdown file in the
			// order in which the renders end.
			if c.Stdout != nil && fmt.Sprint(sortedLines(stdout.String())) != fmt.Sprint(sortedLines(*c.Stdout)) {
				t.Errorf("stdout\n got: %q\nmmdc: %q", stdout.String(), *c.Stdout)
			}
			if c.StdoutHash != "" && sameFonts && hash16(stdout.Bytes()) != c.StdoutHash {
				t.Errorf("the SVG on stdout differs from the SVG of mmdc: %.200s", stdout.String())
			}
			for name, want := range c.Files {
				got, err := os.ReadFile(name)
				if err != nil {
					t.Errorf("%s: %v", name, err)
				} else if string(got) != want {
					t.Errorf("%s\n got: %q\nmmdc: %q", name, got, want)
				}
			}
			for name := range c.Exists {
				if _, err := os.Stat(name); err != nil {
					t.Errorf("mmdc wrote %s: %v", name, err)
				}
			}
		})
	}
}

// TestPNGSizeAgainstMmdc compares the size of PNG output with the size of
// the PNG that mmdc wrote, for several diagrams, scales, --size and
// backgrounds.
func TestPNGSizeAgainstMmdc(t *testing.T) {
	var golden struct {
		Diagrams map[string]string `json:"diagrams"`
		Cases    []struct {
			Diagram   string   `json:"diagram"`
			Args      []string `json:"args"`
			Width     int      `json:"width"`
			Height    int      `json:"height"`
			ColorType int      `json:"colorType"`
		} `json:"cases"`
	}
	loadGolden(t, "png.json", &golden)
	if !referenceFonts(t) {
		t.Skip("the sizes of mmdc depend on the fonts of the machine that ran it")
	}
	dir := t.TempDir()
	for _, c := range golden.Cases {
		t.Run(c.Diagram+" "+strings.Join(c.Args, " "), func(t *testing.T) {
			input := filepath.Join(dir, c.Diagram+".mmd")
			if err := os.WriteFile(input, []byte(golden.Diagrams[c.Diagram]), 0o644); err != nil {
				t.Fatal(err)
			}
			output := filepath.Join(dir, "out.png")
			args := append([]string{"-i", input, "-o", output, "-q", "--fonts", "system"}, c.Args...)
			var stdout, stderr bytes.Buffer
			if err := run(args, nil, &stdout, &stderr); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(output)
			if err != nil {
				t.Fatal(err)
			}
			cfg, err := png.DecodeConfig(bytes.NewReader(data))
			if err != nil {
				t.Fatal(err)
			}
			if cfg.Width != c.Width || cfg.Height != c.Height {
				t.Errorf("size %dx%d, mmdc has %dx%d", cfg.Width, cfg.Height, c.Width, c.Height)
			}
			// Color type 6 is RGB with alpha: mmdc writes it for a
			// transparent background only.
			if alpha := data[25] == 6; alpha != (c.ColorType == 6) {
				t.Errorf("color type %d, mmdc has %d", data[25], c.ColorType)
			}
		})
	}
}

// TestCorpusAgainstMmdc renders a sample of the corpus for several sets of
// options, and compares every .svg file with the file that mmdc wrote for
// the same diagram, by its hash. mmdc rendered the sample from one Markdown
// file. mmdg renders each diagram alone, so that a diagram that it cannot
// render does not stop the others.
//
// The files that differ are listed, with the reason, in
// testdata/mmdc/corpus.differs.txt. The test fails when another file
// differs. Run it with -update-mmdc to write the list again.
func TestCorpusAgainstMmdc(t *testing.T) {
	var golden struct {
		CSSFile  string `json:"cssFile"`
		Fixtures []struct {
			Name      string `json:"name"`
			PageFonts bool   `json:"pageFonts"`
		} `json:"fixtures"`
		Sets map[string]struct {
			Args  []string `json:"args"`
			Files map[string]struct {
				Hash      string `json:"hash"`
				FontEmbed bool   `json:"fontEmbed"`
			} `json:"files"`
		} `json:"sets"`
	}
	loadGolden(t, "corpus.json", &golden)
	if !referenceFonts(t) {
		t.Skip("the files of mmdc depend on the fonts of the machine that ran it")
	}
	if testing.Short() {
		t.Skip("renders the sample for every set of options")
	}
	differsPath := filepath.Join("testdata", "mmdc", "corpus.differs.txt")
	expected := map[string]bool{}
	if data, err := os.ReadFile(differsPath); err == nil {
		for _, line := range strings.Split(strings.TrimSpace(string(data)), "\n") {
			if fields := strings.Split(line, "\t"); len(fields) >= 2 {
				expected[fields[0]+"\t"+fields[1]] = true
			}
		}
	}
	differsAbs, err := filepath.Abs(differsPath)
	if err != nil {
		t.Fatal(err)
	}

	corpus, err := filepath.Abs(filepath.Join("..", "..", "testdata", "corpus", "mermaid-e2e"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	// The text of each diagram is the text of its code block in the
	// Markdown file that mmdc read.
	for i, f := range golden.Fixtures {
		src, err := os.ReadFile(filepath.Join(corpus, filepath.FromSlash(f.Name)+".mmd"))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(string(src), "\n") {
			src = append(src, '\n')
		}
		if err := os.WriteFile(filepath.Join(dir, fmt.Sprintf("d%d.mmd", i)), src, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dir, "style.css"), []byte(golden.CSSFile), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	names := make([]string, 0, len(golden.Sets))
	for name := range golden.Sets {
		names = append(names, name)
	}
	sort.Strings(names)
	var list strings.Builder
	var worse, better []string
	for _, name := range names {
		set := golden.Sets[name]
		same, embedded, pageFonts, failed, other := 0, 0, 0, 0, 0
		for i, f := range golden.Fixtures {
			args := append([]string{"-i", fmt.Sprintf("d%d.mmd", i), "-o", "out.svg", "-q", "--fonts", "system"}, set.Args...)
			var stdout, stderr bytes.Buffer
			runErr := run(args, nil, &stdout, &stderr)
			var data []byte
			if runErr == nil {
				if data, err = os.ReadFile("out.svg"); err != nil {
					t.Fatal(err)
				}
			}
			want := set.Files[f.Name]
			key := name + "\t" + f.Name
			if runErr == nil && hash16(data) == want.Hash {
				same++
				if expected[key] {
					better = append(better, key)
				}
				continue
			}
			// The reason for a file that differs.
			reason := "other"
			switch {
			case runErr != nil:
				reason = "mmdg: " + strings.ReplaceAll(runErr.Error(), "\n", " ")
				failed++
			case want.FontEmbed:
				reason = "mmdc embedded the fonts of its page into the SVG"
				embedded++
			case f.PageFonts:
				reason = "the diagram uses a font or a style sheet of the page of mmdc"
				pageFonts++
			default:
				other++
			}
			list.WriteString(key + "\t" + reason + "\n")
			if !expected[key] {
				worse = append(worse, key+" ("+reason+")")
			}
		}
		t.Logf("%-14s %3d of %d files are the files of mmdc; %d differ because mmdc embedded fonts, %d because of the fonts of its page, %d for another reason, %d are an error in mmdg",
			name, same, len(golden.Fixtures), embedded, pageFonts, other, failed)
	}
	if *updateMmdc {
		if err := os.WriteFile(differsAbs, []byte(list.String()), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	if len(better) > 0 {
		t.Logf("%d files are now the files of mmdc: run go test -run TestCorpusAgainstMmdc -update-mmdc ./cmd/mmdg", len(better))
	}
	if len(worse) > 0 {
		t.Errorf("files that differ from mmdc and are not in corpus.differs.txt:\n  %s", strings.Join(worse, "\n  "))
	}
}
