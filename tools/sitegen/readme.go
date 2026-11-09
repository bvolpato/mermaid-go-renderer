// This file ports no upstream file.

package main

import (
	"fmt"
	"path/filepath"
	"strings"

	mermaid "github.com/bvolpato/mermaid-go-renderer"
	"github.com/bvolpato/mermaid-go-renderer/internal/perfdata"
)

// readmeGallery lists the diagrams of the gallery that the README shows as
// PNG files.
var readmeGallery = []string{"flowchart", "sequence", "class", "state", "mindmap", "gantt", "gitgraph"}

// chartTheme has the colors of an image of the README for one color scheme
// of GitHub. The images have no background, so that they sit on the page.
// The three colors of the marks are apart for readers with a color vision
// deficiency, on the two surfaces of GitHub (#ffffff and #0d1117).
type chartTheme struct {
	name      string
	primary   string // text
	secondary string
	muted     string
	accent    string // the mark of mmdg, and "same bytes"
	blue      string // "same error"
	orange    string // "not supported"
	gray      string // the mark of the thing that mmdg is compared with
	hairline  string
}

var chartThemes = []chartTheme{
	{"light", "#0b1f2a", "#4b5f6b", "#6b7c87", "#0d9488", "#2a78d6", "#eb6834", "#8796a1", "#d7dee3"},
	{"dark", "#e6edf3", "#9fb0bb", "#8394a0", "#12a594", "#3987e5", "#d95926", "#5d7a89", "#2a3743"},
}

const chartFont = `-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif`

func writeReadmeImages(dir string, gallery []galleryItem, perf *perfdata.Data, par *parityData) error {
	for _, th := range chartThemes {
		if err := writeFile(filepath.Join(dir, "speed-"+th.name+".svg"), []byte(speedChart(th, perf))); err != nil {
			return err
		}
		if err := writeFile(filepath.Join(dir, "parity-"+th.name+".svg"), []byte(parityChart(th, par))); err != nil {
			return err
		}
	}
	bySource := map[string]string{}
	for _, item := range gallery {
		bySource[item.File] = item.Source
	}
	for _, name := range readmeGallery {
		png, err := mermaid.RenderPNG(bySource[name], mermaid.WithPage(mermaid.PageMermaidCLI), mermaid.WithScale(2))
		if err != nil {
			return fmt.Errorf("%s: %w", name, err)
		}
		if err := writeFile(filepath.Join(dir, "gallery-"+name+".png"), png); err != nil {
			return err
		}
	}
	return nil
}

// bar is a horizontal bar that grows from x: square at the baseline and
// round at the data end.
func bar(x, y, w, h float64, fill string) string {
	if w < 3 {
		w = 3
	}
	r := 4.0
	if w < r {
		r = w
	}
	return fmt.Sprintf(`<path d="M%.1f %.1fh%.1fa%.1f %.1f 0 0 1 %.1f %.1fv%.1fa%.1f %.1f 0 0 1 -%.1f %.1fh-%.1fz" fill="%s"/>`,
		x, y, w-r, r, r, r, r, h-2*r, r, r, r, r, w-r, fill)
}

func msText(v float64) string {
	switch {
	case v >= 100:
		return fmt.Sprintf("%.0f ms", v)
	case v >= 10:
		return fmt.Sprintf("%.1f ms", v)
	}
	return fmt.Sprintf("%.2f ms", v)
}

// speedChart draws two panels. Each one has a ratio as its headline and two
// bars on one axis: the thing that mmdg is compared with, and mmdg.
func speedChart(th chartTheme, perf *perfdata.Data) string {
	warm := perf.WarmMedian
	cli := perf.CLI.Rows[0]
	type panel struct {
		eyebrow, other string
		a, b           float64
	}
	panels := []panel{
		{"ONE RENDER IN A WARM PROCESS", "mermaid in Chromium", warm.Mermaid, warm.Mmdg},
		{"COMMAND-LINE RUN, FLOWCHART TO SVG", "mmdc", cli.Mmdc, cli.Mmdg},
	}
	const width, height, panelW = 880.0, 196.0, 420.0
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.0f %.0f" role="img" font-family="%s">`, width, height, width, height, chartFont)
	fmt.Fprintf(&b, `<title>mmdg is %.0f times faster than mermaid in a warm Chromium page, and %.0f times faster than mmdc as a command</title>`, warm.Mermaid/warm.Mmdg, cli.Mmdc/cli.Mmdg)
	for i, p := range panels {
		x := float64(i) * (panelW + 40)
		const labelW, barMax = 150.0, 190.0
		fmt.Fprintf(&b, `<text x="%.0f" y="18" font-size="11" letter-spacing="0.8" font-weight="600" fill="%s">%s</text>`, x, th.muted, p.eyebrow)
		fmt.Fprintf(&b, `<text x="%.0f" y="74" font-size="44" font-weight="700" fill="%s">%.0f×<tspan font-size="20" font-weight="500" fill="%s" dx="8">faster</tspan></text>`, x, th.primary, p.a/p.b, th.secondary)
		rows := []struct {
			label string
			v     float64
			fill  string
			bold  string
		}{{p.other, p.a, th.gray, "400"}, {"mmdg", p.b, th.accent, "700"}}
		for j, r := range rows {
			y := 112.0 + float64(j)*36
			w := barMax * r.v / p.a
			fmt.Fprintf(&b, `<text x="%.0f" y="%.0f" font-size="13" font-weight="%s" fill="%s">%s</text>`, x, y+11, r.bold, th.secondary, r.label)
			b.WriteString(bar(x+labelW, y, w, 14, r.fill))
			fmt.Fprintf(&b, `<text x="%.1f" y="%.0f" font-size="13" font-weight="600" fill="%s">%s</text>`, x+labelW+max(w, 3)+8, y+11, th.primary, msText(r.v))
		}
		// The baseline of the two bars.
		fmt.Fprintf(&b, `<path d="M%.1f 104v52" stroke="%s" stroke-width="1"/>`, x+labelW-0.5, th.hairline)
	}
	b.WriteString(`</svg>`)
	return b.String()
}

// parityChart draws one square for each diagram of the end-to-end corpus.
func parityChart(th chartTheme, par *parityData) string {
	res := par.Sets[0].Corpora[0]
	const cols, cell, gap = 75, 9.5, 2.0
	pitch := cell + gap
	rows := (res.Total + cols - 1) / cols
	const top = 86.0
	width := cols*pitch - gap
	height := top + float64(rows)*pitch + 4
	paths := map[byte]*strings.Builder{statusExact: {}, statusSameError: {}, statusOther: {}}
	for i := 0; i < res.Total; i++ {
		x := float64(i%cols) * pitch
		y := top + float64(i/cols)*pitch
		fmt.Fprintf(paths[res.Status[i]], "M%.1f %.1fh%.1fv%.1fh-%.1fz", x, y, cell, cell, cell)
	}
	var b strings.Builder
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" width="%.0f" height="%.0f" viewBox="0 0 %.1f %.1f" role="img" font-family="%s">`, width, height, width, height, chartFont)
	fmt.Fprintf(&b, `<title>%d of the %d diagrams of the end-to-end tests of mermaid pass: %d have the same bytes as mermaid, and %d fail with the error of mermaid</title>`, res.Pass(), res.Total, res.Exact, res.SameError)
	fmt.Fprintf(&b, `<text x="0" y="18" font-size="11" letter-spacing="0.8" font-weight="600" fill="%s">EVERY SQUARE IS ONE DIAGRAM OF THE TEST SUITE OF MERMAID</text>`, th.muted)
	fmt.Fprintf(&b, `<text x="0" y="50" font-size="26" font-weight="700" fill="%s">%d of %d<tspan font-size="15" font-weight="500" fill="%s" dx="8">give the output of mermaid in Chromium, byte for byte</tspan></text>`, th.primary, res.Pass(), res.Total, th.secondary)
	legend := []struct {
		fill, label string
		n           int
	}{
		{th.accent, "Same bytes", res.Exact},
		{th.blue, "Same error as mermaid", res.SameError},
		{th.orange, "Not supported (KaTeX math)", res.Other},
	}
	x := 0.0
	for _, l := range legend {
		fmt.Fprintf(&b, `<rect x="%.0f" y="62" width="10" height="10" fill="%s"/>`, x, l.fill)
		text := fmt.Sprintf("%s", l.label)
		fmt.Fprintf(&b, `<text x="%.0f" y="71" font-size="12" fill="%s">%s <tspan font-weight="700" fill="%s">%d</tspan></text>`, x+16, th.secondary, text, th.primary, l.n)
		x += 16 + float64(len(text))*6.1 + float64(len(fmt.Sprint(l.n)))*8 + 30
	}
	for _, c := range []struct {
		status byte
		fill   string
	}{{statusExact, th.accent}, {statusSameError, th.blue}, {statusOther, th.orange}} {
		if paths[c.status].Len() > 0 {
			fmt.Fprintf(&b, `<path d="%s" fill="%s"/>`, paths[c.status].String(), c.fill)
		}
	}
	b.WriteString(`</svg>`)
	return b.String()
}
