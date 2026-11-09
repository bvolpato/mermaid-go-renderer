// This file ports no upstream file.

// Package perfdata has the result of the performance measurement:
// docs/performance.json. tools/perfdoc writes the file, and the website and
// the README take their numbers from it (tools/sitegen).
package perfdata

import (
	"encoding/json"
	"os"
)

// Pair is one measurement on both sides.
type Pair struct {
	// Mermaid is the value of mermaid in Chromium, or of mmdc.
	Mermaid float64 `json:"mermaid"`
	// Mmdg is the value of this module.
	Mmdg float64 `json:"mmdg"`
}

// Ratio says how many times mmdg is faster.
func (p Pair) Ratio() float64 {
	if p.Mmdg == 0 {
		return 0
	}
	return p.Mermaid / p.Mmdg
}

// Type has the medians of the diagrams of one diagram type, in
// milliseconds.
type Type struct {
	Type     string `json:"type"`
	Diagrams int    `json:"diagrams"`
	// Mermaid and Mmdg are the medians of a warm render. Mmdg90 is the
	// 90th percentile of mmdg.
	Mermaid float64 `json:"mermaid"`
	Mmdg    float64 `json:"mmdg"`
	Mmdg90  float64 `json:"mmdg90"`
	// MermaidFirst and MmdgFirst are the medians of a first render. They
	// are zero when the measurement has none.
	MermaidFirst float64 `json:"mermaidFirst"`
	MmdgFirst    float64 `json:"mmdgFirst"`
}

// CLIRow is one run of the two command-line tools, in milliseconds.
type CLIRow struct {
	Input  string  `json:"input"`
	Output string  `json:"output"`
	Mmdc   float64 `json:"mmdc"`
	Mmdg   float64 `json:"mmdg"`
}

// CLI is the comparison of the command-line tools
// (tools/reference/bench-cli.mjs).
type CLI struct {
	// Mmdc is the version of mermaid-cli.
	Mmdc   string   `json:"mmdc"`
	Runs   int      `json:"runs"`
	Warmup int      `json:"warmup"`
	Rows   []CLIRow `json:"rows"`
}

// Data is the content of docs/performance.json. Times are in milliseconds,
// except WarmTotal, which is in seconds.
type Data struct {
	// Measured is the day of the measurement, as 2006-01-02.
	Measured string `json:"measured"`
	Mermaid  string `json:"mermaid"`
	Chromium string `json:"chromium"`
	// Platform is the machine of the measurement, for example darwin/arm64.
	Platform string `json:"platform"`
	CPUs     int    `json:"cpus"`
	Corpus   string `json:"corpus"`
	// Diagrams is the number of diagrams that both sides render.
	Diagrams int `json:"diagrams"`
	// RepeatMermaid and RepeatMmdg are the numbers of renders of each
	// diagram. The fastest one counts.
	RepeatMermaid int `json:"repeatMermaid"`
	RepeatMmdg    int `json:"repeatMmdg"`

	WarmMedian  Pair `json:"warmMedian"`
	WarmP90     Pair `json:"warmP90"`
	WarmTotal   Pair `json:"warmTotalSeconds"`
	FirstMedian Pair `json:"firstMedian"`
	FirstP90    Pair `json:"firstP90"`

	Types []Type `json:"types"`
	CLI   *CLI   `json:"cli,omitempty"`
}

// Load reads a file that Save wrote.
func Load(path string) (*Data, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	d := &Data{}
	if err := json.Unmarshal(raw, d); err != nil {
		return nil, err
	}
	return d, nil
}

// Save writes the data as JSON.
func (d *Data) Save(path string) error {
	raw, err := json.MarshalIndent(d, "", " ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0o644)
}
