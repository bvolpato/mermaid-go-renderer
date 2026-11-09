// This file ports no upstream file.

// Command mmdcdiff shows what mermaid-cli does to a diagram besides the
// fonts and the style sheets of its page.
//
// It reads two references of the same corpus:
//
//   - the output of real mermaid-cli (tools/reference/render-mmdc-ref.mjs):
//     the .svg file of mmdc --no-font-embed
//   - the output of mermaid in a plain page that has the web fonts and the
//     style sheets of the page of mermaid-cli
//     (tools/reference/render-webfonts-ref.mjs): the string that
//     mermaid.render() returns
//
// It runs the steps of mermaid-cli that this module ports on the second one
// (the background and the XML serialization, internal/mmdc) and compares
// the result with the first one. A fixture that differs shows something
// that the page of mermaid-cli does and a plain page with the same fonts
// does not.
//
// Usage:
//
//	go run ./tools/mmdcdiff -mmdc .parity/ref-mmdc/linux/mermaid-e2e -webfonts .parity/ref-webfonts/linux/mermaid-e2e
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/mmdc"
	"github.com/bvolpato/mermaid-go-renderer/internal/parity"
)

type meta struct {
	DiagramType string `json:"diagramType"`
	Error       string `json:"error"`
	RandomTotal *int   `json:"randomTotal"`
	// The fields of oracle.mjs render.
	RandomBefore *int `json:"randomBefore"`
	RandomCalls  *int `json:"randomCalls"`
}

func readMeta(path string) (*meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	m := &meta{}
	return m, json.Unmarshal(data, m)
}

func main() {
	mmdcDir := flag.String("mmdc", ".parity/ref-mmdc/linux/mermaid-e2e", "reference of real mermaid-cli")
	webfontsDir := flag.String("webfonts", ".parity/ref-webfonts/linux/mermaid-e2e", "reference of mermaid in a plain page with the web fonts")
	show := flag.Int("show", 40, "number of differences to print")
	flag.Parse()

	var names []string
	_ = filepath.WalkDir(*mmdcDir, func(path string, d os.DirEntry, err error) error {
		if err == nil && !d.IsDir() && strings.HasSuffix(path, ".json") {
			rel, _ := filepath.Rel(*mmdcDir, path)
			names = append(names, strings.TrimSuffix(rel, ".json"))
		}
		return nil
	})
	sort.Strings(names)

	tally := map[string]int{}
	byFamily := map[string]map[string]int{}
	var lines []string
	note := func(status, name, detail string) {
		tally[status]++
		family := strings.Split(filepath.ToSlash(name), "/")[0]
		if byFamily[family] == nil {
			byFamily[family] = map[string]int{}
		}
		byFamily[family][status]++
		if status != "same" && status != "same-error" {
			lines = append(lines, fmt.Sprintf("[%s] %s\n    %s", status, name, detail))
		}
	}
	for _, name := range names {
		a, err := readMeta(filepath.Join(*mmdcDir, name+".json"))
		if err != nil {
			note("no-mmdc", name, err.Error())
			continue
		}
		b, err := readMeta(filepath.Join(*webfontsDir, name+".json"))
		if err != nil {
			note("no-webfonts", name, "the plain page has no result")
			continue
		}
		switch {
		case a.Error != "" && b.Error != "":
			note("same-error", name, "")
			continue
		case a.Error != "":
			note("mmdc-error", name, "mmdc fails where the plain page renders: "+a.Error)
			continue
		case b.Error != "":
			note("plain-error", name, "mmdc renders ("+a.DiagramType+") where the plain page fails: "+b.Error)
			continue
		}
		want, err := os.ReadFile(filepath.Join(*mmdcDir, name+".svg"))
		if err != nil {
			note("no-mmdc", name, err.Error())
			continue
		}
		plain, err := os.ReadFile(filepath.Join(*webfontsDir, name+".svg"))
		if err != nil {
			note("no-webfonts", name, err.Error())
			continue
		}
		page, err := mmdc.Load(string(plain), mmdc.Options{Background: "white", SetBackground: true})
		if err != nil {
			note("error", name, err.Error())
			continue
		}
		got := page.XML()
		if got == string(want) {
			note("same", name, "")
			continue
		}
		detail := ""
		if _, diff := parity.CompareSVG(got, string(want), 0); diff != nil {
			detail = diff.String()
		}
		// The draws of Math.random tell a diagram whose result depends on
		// the order of the draws.
		if a.RandomTotal != nil && b.RandomBefore != nil && b.RandomCalls != nil {
			detail += fmt.Sprintf("\n    Math.random: mmdc %d draws, plain page %d before the render and %d in it", *a.RandomTotal, *b.RandomBefore, *b.RandomCalls)
		}
		note("different", name+" ("+a.DiagramType+")", detail)
	}

	families := make([]string, 0, len(byFamily))
	for f := range byFamily {
		families = append(families, f)
	}
	sort.Strings(families)
	statuses := []string{"same", "same-error", "different", "mmdc-error", "plain-error", "no-webfonts", "no-mmdc", "error"}
	fmt.Printf("%-22s", "family")
	for _, s := range statuses {
		fmt.Printf(" %11s", s)
	}
	fmt.Println()
	for _, f := range families {
		fmt.Printf("%-22s", f)
		for _, s := range statuses {
			fmt.Printf(" %11d", byFamily[f][s])
		}
		fmt.Println()
	}
	fmt.Printf("%-22s", "TOTAL")
	for _, s := range statuses {
		fmt.Printf(" %11d", tally[s])
	}
	fmt.Println()
	for i, line := range lines {
		if i >= *show {
			fmt.Printf("\nand %d more\n", len(lines)-i)
			break
		}
		fmt.Println()
		fmt.Println(line)
	}
}
