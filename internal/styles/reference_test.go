package styles

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"
)

// refDir holds what real mermaid gives in Chromium for every fixture of the
// corpus: the SVG, and a JSON file with the diagram type or the error. The
// directory is generated and is not part of the repository, so the test skips
// when it is missing.
const refDir = "../../.parity/ref/mermaid-e2e"

var styleElement = regexp.MustCompile(`(?s)<style>(.*?)</style>`)

// referenceStyle reads the text of the <style> element from a reference SVG.
func referenceStyle(svg string) (string, bool) {
	// With securityLevel sandbox the SVG is inside the data URL of an iframe.
	if strings.HasPrefix(svg, "<iframe") {
		const marker = "base64,"
		i := strings.Index(svg, marker)
		if i < 0 {
			return "", false
		}
		rest := svg[i+len(marker):]
		j := strings.Index(rest, `"`)
		if j < 0 {
			return "", false
		}
		decoded, err := base64.StdEncoding.DecodeString(rest[:j])
		if err != nil {
			return "", false
		}
		svg = string(decoded)
	}
	m := styleElement.FindStringSubmatch(svg)
	if m == nil {
		return "", false
	}
	return m[1], true
}

// unescapeHTML undoes the escaping of the HTML serializer. Mermaid passes the
// SVG text through DOMPurify, which parses it and serializes it again, and
// the serializer escapes the text of the style element.
func unescapeHTML(s string) string {
	return strings.NewReplacer("&lt;", "<", "&gt;", ">", "&nbsp;", " ", "&amp;", "&").Replace(s)
}

// cssTextOfCompiled turns compiled rules such as
// "#id .a>*{fill:red!important;}" back into the cssText that Chromium gives
// for them: ".a > * { fill: red !important; }", one rule on each line. It
// handles the simple rules that mermaid makes for classDefs.
func cssTextOfCompiled(compiled, svgID string) (string, bool) {
	var rules []string
	for compiled != "" {
		open := strings.Index(compiled, "{")
		end := strings.Index(compiled, "}")
		if open < 0 || end < open {
			return "", false
		}
		selector, ok := strings.CutPrefix(compiled[:open], svgID+" ")
		if !ok {
			return "", false
		}
		selector = strings.ReplaceAll(selector, ">", " > ")
		var declarations []string
		for _, declaration := range strings.Split(strings.TrimSuffix(compiled[open+1:end], ";"), ";") {
			name, value, ok := strings.Cut(declaration, ":")
			if !ok {
				return "", false
			}
			value = strings.ReplaceAll(value, "!important", " !important")
			declarations = append(declarations, name+": "+value+";")
		}
		rules = append(rules, selector+" { "+strings.Join(declarations, " ")+" }")
		compiled = compiled[end+1:]
	}
	return strings.Join(rules, "\n"), true
}

// themeCSSText is what cssStyleSheetToString gives in Chromium for the
// themeCSS of the one fixture of the corpus that has it. The source is
// ".loopText { fill: #ffffff !important; } .labelBox { fill: red !important; }".
const themeCSSFixture = "sequence/nightly-check/should-render-alt-else-section-titles-with-label-box-backgrounds"
const themeCSSText = ".loopText { fill: rgb(255, 255, 255) !important; }\n.labelBox { fill: red !important; }"

// Causes of a difference between Build(type, "", config, id) and the style
// text of the reference SVG.
const (
	refExact    = "exact"
	refClassDef = "classDef rules"
	refThemeCSS = "themeCSS"
	refNoStyle  = "no style element"
	refOther    = "not explained"
)

// TestReferenceSVG is comparison (b): the style text of the SVG that real
// mermaid makes in Chromium, against Build for the same fixture.
//
// Build makes everything but the parts of createCssStyles that need the CSSOM
// of the browser, so a fixture with classDefs or themeCSS cannot match with
// userStyles "". For those, the test checks that Build gives the exact text
// when it gets the rules that the browser made.
func TestReferenceSVG(t *testing.T) {
	if _, err := os.Stat(refDir); err != nil {
		t.Skipf("no reference output in %s", refDir)
	}
	g := loadGolden(t)
	const svgID = "#my-svg"

	type row struct {
		svgs   int
		causes map[string]int
	}
	rows := map[string]*row{}
	totals := map[string]int{}
	svgs, noSVG, typeDiffers := 0, 0, 0
	for _, fx := range g.file.Fixtures {
		svgBytes, err := os.ReadFile(filepath.Join(refDir, fx.Path+".svg"))
		if err != nil {
			// Mermaid threw for this fixture, so there is no SVG.
			noSVG++
			continue
		}
		svgs++
		metaBytes, err := os.ReadFile(filepath.Join(refDir, fx.Path+".json"))
		if err != nil {
			t.Fatal(err)
		}
		var meta struct {
			DiagramType string `json:"diagramType"`
		}
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			t.Fatal(err)
		}
		if fx.Output == nil {
			t.Errorf("%s: the reference has an SVG, but node found no diagram type", fx.Path)
			continue
		}
		out := g.file.Outputs[*fx.Output]
		if meta.DiagramType != out.Type {
			typeDiffers++
			t.Errorf("%s: type %q in the reference, %q from detectType in node", fx.Path, meta.DiagramType, out.Type)
		}
		r := rows[meta.DiagramType]
		if r == nil {
			r = &row{causes: map[string]int{}}
			rows[meta.DiagramType] = r
		}
		r.svgs++

		cfg := g.config(t, out.Config)
		got, err := Build(meta.DiagramType, "", cfg, svgID)
		if err != nil {
			t.Errorf("%s: Build: %v", fx.Path, err)
			continue
		}

		cause := refOther
		escaped, ok := referenceStyle(string(svgBytes))
		want := unescapeHTML(escaped)
		switch {
		case !ok:
			// The wardley renderer removes every child of the SVG before it
			// draws, and the style element is one of them.
			if meta.DiagramType == "wardley" {
				cause = refNoStyle
			}
		case got == escaped || got == want:
			cause = refExact
		case strings.HasPrefix(want, got):
			// The rest is the classDef rules. Build gives the same text when
			// it gets these rules as userStyles, in the cssText form that the
			// browser gives to mermaid.
			rules, ok := cssTextOfCompiled(want[len(got):], svgID)
			if !ok {
				break
			}
			with, err := Build(meta.DiagramType, rules, cfg, svgID)
			if err == nil && with == want && cfg.Get("themeCSS") == nil {
				cause = refClassDef
			}
		case fx.Path == themeCSSFixture:
			// The browser prints "#ffffff" as "rgb(255, 255, 255)".
			text := themeCSSText
			with, err := CreateUserStyles(cfg, meta.DiagramType, CreateCssStyles(cfg, &text, nil), svgID)
			if err == nil && with == want {
				cause = refThemeCSS
			}
		}
		if cause == refOther {
			t.Errorf("%s (%s): the style text differs and no known cause applies: %s", fx.Path, meta.DiagramType, firstDiff(got, want))
		}
		r.causes[cause]++
		totals[cause]++
	}

	names := make([]string, 0, len(rows))
	for name := range rows {
		names = append(names, name)
	}
	sort.Strings(names)
	causes := []string{refExact, refClassDef, refThemeCSS, refNoStyle, refOther}
	var sb strings.Builder
	fmt.Fprintf(&sb, "  %-16s %5s", "type", "svgs")
	for _, c := range causes {
		fmt.Fprintf(&sb, " %16s", c)
	}
	sb.WriteByte('\n')
	for _, name := range names {
		r := rows[name]
		fmt.Fprintf(&sb, "  %-16s %5d", name, r.svgs)
		for _, c := range causes {
			fmt.Fprintf(&sb, " %16d", r.causes[c])
		}
		sb.WriteByte('\n')
	}
	fmt.Fprintf(&sb, "  %-16s %5d", "total", svgs)
	for _, c := range causes {
		fmt.Fprintf(&sb, " %16d", totals[c])
	}
	t.Logf("reference SVGs against Build(type, \"\", config, %q): %d fixtures, %d with no SVG because mermaid threw, %d with another type than node\n%s",
		svgID, len(g.file.Fixtures), noSVG, typeDiffers, sb.String())
}
