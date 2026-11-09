package config

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// The rules of styles.ts that every diagram gets. Each one prints theme
// variables into the CSS of the rendered SVG, after stylis has minified it.
var referenceRules = []struct {
	re   *regexp.Regexp
	vars []string
}{
	{regexp.MustCompile(`^#my-svg\{font-family:([^;]*);font-size:([^;]*);fill:([^;]*);\}`), []string{"fontFamily", "fontSize", "textColor"}},
	{regexp.MustCompile(`#my-svg \.error-icon\{fill:([^;]*);\}`), []string{"errorBkgColor"}},
	{regexp.MustCompile(`#my-svg \.error-text\{fill:([^;]*);stroke:([^;]*);\}`), []string{"errorTextColor", "errorTextColor"}},
	{regexp.MustCompile(`#my-svg \.marker\{fill:([^;]*);stroke:([^;]*);\}`), []string{"lineColor", "lineColor"}},
	{regexp.MustCompile(`#my-svg \.marker\.cross\{stroke:([^;]*);\}`), []string{"lineColor"}},
	{regexp.MustCompile(`#my-svg svg\{font-family:([^;]*);font-size:([^;]*);\}`), []string{"fontFamily", "fontSize"}},
	{regexp.MustCompile(`#my-svg \.node \.neo-node\{stroke:([^;]*);\}`), []string{"nodeBorder"}},
}

var styleRe = regexp.MustCompile(`(?s)<style>(.*?)</style>`)

// squeeze drops white space, which stylis removes from values.
func squeeze(s string) string {
	return strings.Join(strings.Fields(s), "")
}

// TestReferenceSVG is a cross-check with the output of the real mermaid in
// Chromium (.parity/ref). The effective config of Resolve is the config that
// mermaid.render builds the stylesheet from, so the theme variables must be
// in the <style> of the reference SVG, and the diagram type must be the one
// that render returned. The test is skipped when the reference directory is
// absent: it is generated, and not part of the repository.
func TestReferenceSVG(t *testing.T) {
	refRoot := filepath.Join("..", "..", ".parity", "ref", "mermaid-e2e")
	corpusRoot := filepath.Join("..", "..", "testdata", "corpus", "mermaid-e2e")
	if _, err := os.Stat(refRoot); err != nil {
		t.Skip("no reference output in .parity/ref")
	}
	site := js.NewObj[any]()
	site.Set("startOnLoad", false)

	var paths []string
	err := filepath.WalkDir(refRoot, func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			paths = append(paths, path)
		}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	type result struct {
		svg, noStyle, typeCompared, typeMismatch bool
		values, valueMismatch                    int
		problems                                 []string
	}
	results := make([]result, len(paths))
	forEachCase(len(paths), func(i int) string {
		r := &results[i]
		path := paths[i]
		rel, _ := filepath.Rel(refRoot, strings.TrimSuffix(path, ".json"))
		text, err := os.ReadFile(filepath.Join(corpusRoot, rel+".mmd"))
		if err != nil {
			return "" // not a corpus fixture
		}
		var meta struct {
			DiagramType string
			Error       string
		}
		data, err := os.ReadFile(path)
		if err != nil || json.Unmarshal(data, &meta) != nil {
			return "" // still being written
		}
		got, err := Resolve(string(text), site)
		if err != nil {
			r.problems = append(r.problems, fmt.Sprintf("%s: Resolve: %v", rel, err))
			return ""
		}
		if meta.Error == "" && meta.DiagramType != "" {
			r.typeCompared = true
			if got.DiagramType != meta.DiagramType {
				r.typeMismatch = true
				r.problems = append(r.problems, fmt.Sprintf("%s: diagram type %q, the reference has %q", rel, got.DiagramType, meta.DiagramType))
			}
		}
		svg, err := os.ReadFile(filepath.Join(refRoot, rel+".svg"))
		if err != nil {
			return "" // the reference render failed, or is not there yet
		}
		m := styleRe.FindSubmatch(svg)
		if m == nil {
			// The wardley renderer replaces the content of the SVG element,
			// and the stylesheet with it.
			r.noStyle = true
			return ""
		}
		r.svg = true
		css := strings.NewReplacer("&lt;", "<", "&gt;", ">", "&amp;", "&").Replace(string(m[1]))
		themeVariables, _ := got.Config.Get("themeVariables").(*js.Obj[any])
		for _, rule := range referenceRules {
			found := rule.re.FindStringSubmatch(css)
			if found == nil {
				r.problems = append(r.problems, fmt.Sprintf("%s: rule %s is not in the reference CSS", rel, rule.re))
				r.valueMismatch++
				continue
			}
			for i, name := range rule.vars {
				r.values++
				want := squeeze(js.ToString(themeVariables.Get(name)))
				if squeeze(found[i+1]) != want {
					r.valueMismatch++
					r.problems = append(r.problems, fmt.Sprintf("%s: %s is %q, the reference CSS has %q", rel, name, want, found[i+1]))
				}
			}
		}
		return ""
	})

	var svgs, noStyle, types, typeMismatch, valueMismatch, values int
	var problems []string
	for _, r := range results {
		if r.svg {
			svgs++
		}
		if r.noStyle {
			noStyle++
		}
		if r.typeCompared {
			types++
		}
		if r.typeMismatch {
			typeMismatch++
		}
		values += r.values
		valueMismatch += r.valueMismatch
		problems = append(problems, r.problems...)
	}
	sort.Strings(problems)
	for i, p := range problems {
		if i == 25 {
			t.Errorf("... and %d more", len(problems)-i)
			break
		}
		t.Error(p)
	}
	t.Logf("%d reference SVGs with a stylesheet (%d without): %d theme values compared, %d differ; %d diagram types compared, %d differ",
		svgs, noStyle, values, valueMismatch, types, typeMismatch)
}
