package config

import (
	"encoding/json"
	"fmt"
	"testing"
)

// TestYAML compares the front matter parser with js-yaml as mermaid calls it:
// yaml.load(text, { schema: yaml.JSON_SCHEMA }). The cases are the front
// matter of every corpus fixture and hand-written documents.
func TestYAML(t *testing.T) {
	var g struct {
		CorpusCount int
		Cases       []struct {
			Text  string
			Value json.RawMessage
			Error *goldenError
		}
	}
	readGolden(t, "yaml.json.gz", &g)
	failed, failedCorpus, skipped := 0, 0, 0
	for i, c := range g.Cases {
		if reason, ok := yamlKnownDivergences[c.Text]; ok {
			skipped++
			// A known divergence must still diverge. Otherwise the entry is stale.
			got, err := loadYAML(c.Text)
			same := (err != nil) == (c.Error != nil)
			if same && err == nil {
				same = diff("yaml", got, decode(t, c.Value)) == ""
			}
			if same {
				t.Errorf("case %d %q is listed as a known divergence (%s) but agrees with js-yaml", i, c.Text, reason)
			}
			continue
		}
		got, err := loadYAML(c.Text)
		var problem string
		switch {
		case c.Error != nil && err == nil:
			problem = fmt.Sprintf("got %s, want %s: %.80q", show(got), c.Error.Name, c.Error.Message)
		case c.Error == nil && err != nil:
			problem = fmt.Sprintf("unexpected error %v, want %.200s", err, c.Value)
		case c.Error == nil:
			problem = diff("yaml", got, decode(t, c.Value))
		}
		if problem != "" {
			failed++
			if i < g.CorpusCount {
				failedCorpus++
			}
			if failed <= 60 {
				t.Errorf("case %d %q:\n  %s", i, c.Text, problem)
			}
		}
	}
	t.Logf("%d golden cases (%d from the corpus), %d failed (%d from the corpus), %d known divergences", len(g.Cases), g.CorpusCount, failed, failedCorpus, skipped)
}

// yamlKnownDivergences lists the documents for which gopkg.in/yaml.v3 and
// js-yaml disagree, with the reason. None of them occurs in the corpus.
var yamlKnownDivergences = map[string]string{
	"a: ! 12":                               "yaml.v3 does not report the non-specific tag \"!\", which keeps a plain scalar a string in js-yaml",
	"%YAML 1.2\n---\na: 1":                  "yaml.v3 accepts only %YAML 1.1; a directive cannot precede a document inside front matter anyway",
	"%FOO bar\n---\na: 1":                   "yaml.v3 rejects an unknown directive; js-yaml warns and goes on",
	"- [x\n- x]":                            "js-yaml rejects a flow collection that continues on a line that starts like a block entry; yaml.v3 reads it",
	"- - \n- |+\n  x: y":                    "js-yaml rejects a block scalar that follows an empty entry of a nested sequence; yaml.v3 reads it",
	"- -  - # c\n  - #x\n    -  \n    -  x": "js-yaml flattens nested sequences whose empty entries carry comments",
}
