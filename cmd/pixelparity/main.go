// Command pixelparity measures how close the pure Go rasterizer is to
// Chromium. It renders each reference SVG with raster.Render, compares the
// result with the Chromium screenshot of the same SVG, and reports the
// difference per file and per diagram family.
//
// The screenshots come from the oracle:
//
//	node tools/reference/oracle.mjs shoot --out .parity/shots/s1 \
//	    --base .parity/ref/mermaid-e2e --list files.txt
//
// Usage:
//
//	pixelparity -ref .parity/ref/mermaid-e2e -shots .parity/shots/s1 \
//	    [-scale 1] [-filter flowchart/] [-diff DIR] [-json report.json] [-top 10]
//
// With -fonts embedded the renders use the embedded Liberation fonts and the
// rules of Chromium on Linux. The SVG files must then be the Linux reference
// (.parity/ref-linux/mermaid-e2e), and the screenshots must come from
// tools/reference/oracle-linux.mjs --fonts liberation shoot.
// tools/reference/gen-raster-linux-sample.mjs selects a sample of the corpus
// that has every diagram family and every text feature, and writes the
// lists for the oracle:
//
//	node tools/reference/gen-raster-linux-sample.mjs
//	node tools/reference/oracle-linux.mjs --fonts liberation shoot --quiet \
//	    --out .parity/shots/linux-ref-s1 --base .parity/ref-linux/mermaid-e2e \
//	    --list .parity/raster/linux/shoot-1.list
//	pixelparity -fonts embedded -ref .parity/ref-linux/mermaid-e2e \
//	    -shots .parity/shots/linux-ref-s1 -list .parity/raster/linux/sample.rel
//
// The results do not depend on -workers: a glyph mask is a function of its
// cache key.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/draw"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/bvolpato/mermaid-go-renderer/internal/font"
	"github.com/bvolpato/mermaid-go-renderer/internal/raster"
)

// fontLibrary is the font library of the renders. Nil is the library of
// the system.
var fontLibrary *font.Library

// threshold is the largest channel difference that counts as a match.
const threshold = 16

// FileResult is the comparison of one file.
type FileResult struct {
	File   string `json:"file"`
	Family string `json:"family"`
	Error  string `json:"error,omitempty"`
	// Width and Height are the size of the Go bitmap. RefWidth and
	// RefHeight are the size of the screenshot.
	Width     int `json:"width,omitempty"`
	Height    int `json:"height,omitempty"`
	RefWidth  int `json:"refWidth,omitempty"`
	RefHeight int `json:"refHeight,omitempty"`
	Pixels    int `json:"pixels"`
	// Mismatch is the number of pixels whose largest channel difference is
	// above the threshold.
	Mismatch int `json:"mismatch"`
	// MismatchTol is the number of those pixels that also differ from
	// every pixel within one pixel of them in the screenshot.
	MismatchTol int `json:"mismatchTol"`
	// MAD is the mean absolute channel difference, 0 to 255.
	MAD float64 `json:"mad"`
	// SSIM is the mean structural similarity of the luminance.
	SSIM float64 `json:"ssim"`
	// RenderMs is the wall time of raster.Render. RenderCPUMs is the CPU
	// time that the process used during the call: it is meaningful with one
	// worker, and it does not depend on the load of the machine.
	RenderMs    float64 `json:"renderMs"`
	RenderCPUMs float64 `json:"renderCpuMs"`
}

// MismatchPct is the mismatch as a percentage of the pixels.
func (r *FileResult) MismatchPct() float64 {
	if r.Pixels == 0 {
		return 0
	}
	return 100 * float64(r.Mismatch) / float64(r.Pixels)
}

// FamilyResult aggregates the files of one diagram family.
type FamilyResult struct {
	Family         string  `json:"family"`
	Files          int     `json:"files"`
	Errors         int     `json:"errors"`
	SizeMismatches int     `json:"sizeMismatches"`
	Pixels         int64   `json:"pixels"`
	MismatchPct    float64 `json:"mismatchPct"`
	MismatchTolPct float64 `json:"mismatchTolPct"`
	MAD            float64 `json:"mad"`
	SSIM           float64 `json:"ssim"`
	RenderMs       float64 `json:"renderMsMean"`
}

// Report is the JSON report.
type Report struct {
	Scale     float64        `json:"scale"`
	Threshold int            `json:"threshold"`
	Total     FamilyResult   `json:"total"`
	Families  []FamilyResult `json:"families"`
	Files     []FileResult   `json:"files"`
}

func loadPNG(path string) (*image.NRGBA, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, err := png.Decode(f)
	if err != nil {
		return nil, err
	}
	b := img.Bounds()
	out := image.NewNRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
	return out, nil
}

func absDiff(a, b uint8) int {
	if a > b {
		return int(a - b)
	}
	return int(b - a)
}

// compare computes the metrics of a Go bitmap against a screenshot. Both
// are opaque. diff receives a visualization when it is not nil.
func compare(got *image.RGBA, ref *image.NRGBA, res *FileResult, diff *image.NRGBA) {
	w := max(got.Rect.Dx(), ref.Rect.Dx())
	h := max(got.Rect.Dy(), ref.Rect.Dy())
	res.Width, res.Height = got.Rect.Dx(), got.Rect.Dy()
	res.RefWidth, res.RefHeight = ref.Rect.Dx(), ref.Rect.Dy()
	res.Pixels = w * h
	gw, gh := got.Rect.Dx(), got.Rect.Dy()
	rw, rh := ref.Rect.Dx(), ref.Rect.Dy()
	at := func(pix []uint8, stride, iw, ih, x, y int) (r, g, b uint8, ok bool) {
		if x < 0 || y < 0 || x >= iw || y >= ih {
			return 0, 0, 0, false
		}
		o := y*stride + x*4
		return pix[o], pix[o+1], pix[o+2], true
	}
	var sum int64
	lumG := make([]float64, w*h)
	lumR := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r0, g0, b0, ok0 := at(got.Pix, got.Stride, gw, gh, x, y)
			r1, g1, b1, ok1 := at(ref.Pix, ref.Stride, rw, rh, x, y)
			if !ok0 {
				r0, g0, b0 = 255, 0, 255
			}
			if !ok1 {
				r1, g1, b1 = 0, 255, 0
			}
			dr, dg, db := absDiff(r0, r1), absDiff(g0, g1), absDiff(b0, b1)
			sum += int64(dr + dg + db)
			lumG[y*w+x] = float64(0.299*float64(r0)) + float64(0.587*float64(g0)) + float64(0.114*float64(b0))
			lumR[y*w+x] = float64(0.299*float64(r1)) + float64(0.587*float64(g1)) + float64(0.114*float64(b1))
			d := max(dr, dg, db)
			if diff != nil {
				// Gray copy of the screenshot, mismatches in red.
				l := uint8(lumR[y*w+x])
				l = 128 + l/2
				o := y*diff.Stride + x*4
				diff.Pix[o], diff.Pix[o+1], diff.Pix[o+2], diff.Pix[o+3] = l, l, l, 255
			}
			if d <= threshold {
				continue
			}
			res.Mismatch++
			// One pixel of tolerance: any neighbor of the screenshot
			// that matches this pixel.
			pass := false
			if ok0 {
				for dy := -1; dy <= 1 && !pass; dy++ {
					for dx := -1; dx <= 1; dx++ {
						rn, gn, bn, ok := at(ref.Pix, ref.Stride, rw, rh, x+dx, y+dy)
						if ok && max(absDiff(r0, rn), absDiff(g0, gn), absDiff(b0, bn)) <= threshold {
							pass = true
							break
						}
					}
				}
			}
			if !pass {
				res.MismatchTol++
			}
			if diff != nil {
				o := y*diff.Stride + x*4
				if pass {
					diff.Pix[o], diff.Pix[o+1], diff.Pix[o+2] = 255, 160, 0
				} else {
					diff.Pix[o], diff.Pix[o+1], diff.Pix[o+2] = 255, 0, 0
				}
			}
		}
	}
	res.MAD = float64(sum) / float64(3*w*h)
	res.SSIM = ssim(lumG, lumR, w, h)
}

// ssim is the mean structural similarity over 8 by 8 windows with a step
// of 4 pixels, with the standard constants for 8-bit data.
func ssim(a, b []float64, w, h int) float64 {
	const win = 8
	const step = 4
	const c1 = (0.01 * 255) * (0.01 * 255)
	const c2 = (0.03 * 255) * (0.03 * 255)
	if w < win || h < win {
		// A tiny image: one window over everything.
		return ssimWindow(a, b, w, 0, 0, w, h, c1, c2)
	}
	var total float64
	n := 0
	for y := 0; y+win <= h; y += step {
		for x := 0; x+win <= w; x += step {
			total += ssimWindow(a, b, w, x, y, win, win, c1, c2)
			n++
		}
	}
	return total / float64(n)
}

func ssimWindow(a, b []float64, stride, x0, y0, ww, wh int, c1, c2 float64) float64 {
	var sa, sb, saa, sbb, sab float64
	for y := y0; y < y0+wh; y++ {
		for x := x0; x < x0+ww; x++ {
			va, vb := a[y*stride+x], b[y*stride+x]
			sa += va
			sb += vb
			saa += float64(va * va)
			sbb += float64(vb * vb)
			sab += float64(va * vb)
		}
	}
	n := float64(ww * wh)
	ma, mb := sa/n, sb/n
	va := saa/n - float64(ma*ma)
	vb := sbb/n - float64(mb*mb)
	cov := sab/n - float64(ma*mb)
	return ((float64(2*ma*mb) + c1) * (float64(2*cov) + c2)) / ((float64(ma*ma) + float64(mb*mb) + c1) * (va + vb + c2))
}

func familyOf(rel string) string {
	if i := strings.IndexByte(rel, '/'); i >= 0 {
		return rel[:i]
	}
	return "."
}

func main() {
	refDir := flag.String("ref", ".parity/ref/mermaid-e2e", "directory of reference SVG files")
	shotDir := flag.String("shots", ".parity/shots/s1", "directory of Chromium screenshots, with the layout of -ref")
	scale := flag.Float64("scale", 1, "device scale factor of the screenshots")
	filter := flag.String("filter", "", "only files whose relative path contains this text")
	listFile := flag.String("list", "", "file with relative or absolute SVG paths, one per line")
	diffDir := flag.String("diff", "", "write diff images and the Go bitmaps of mismatching files here")
	diffMin := flag.Float64("diffmin", 0, "write a diff image only above this mismatch percentage")
	jsonOut := flag.String("json", "", "write the JSON report here")
	top := flag.Int("top", 10, "print the worst files")
	workers := flag.Int("workers", 1, "number of files to render at the same time")
	fonts := flag.String("fonts", "system", "font library: system, or embedded for the Liberation fonts with the rules of Chromium on Linux")
	every := flag.Int("every", 1, "take every n-th file of each family")
	quiet := flag.Bool("quiet", false, "print only the tables")
	flag.Parse()
	switch *fonts {
	case "system":
	case "embedded":
		fontLibrary = font.Embedded()
	default:
		fmt.Fprintf(os.Stderr, "pixelparity: unknown -fonts %q\n", *fonts)
		os.Exit(2)
	}

	var files []string
	if *listFile != "" {
		data, err := os.ReadFile(*listFile)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		absRef, _ := filepath.Abs(*refDir)
		for _, line := range strings.Split(string(data), "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			if filepath.IsAbs(line) {
				if rel, err := filepath.Rel(absRef, line); err == nil {
					line = rel
				}
			}
			files = append(files, line)
		}
	} else {
		filepath.WalkDir(*refDir, func(p string, d os.DirEntry, err error) error {
			if err == nil && !d.IsDir() && strings.HasSuffix(p, ".svg") {
				rel, _ := filepath.Rel(*refDir, p)
				files = append(files, rel)
			}
			return nil
		})
	}
	sort.Strings(files)
	if *filter != "" || *every > 1 {
		kept := files[:0]
		count := map[string]int{}
		for _, f := range files {
			if *filter != "" && !strings.Contains(f, *filter) {
				continue
			}
			fam := familyOf(f)
			count[fam]++
			if (count[fam]-1)%*every != 0 {
				continue
			}
			kept = append(kept, f)
		}
		files = kept
	}
	if len(files) == 0 {
		fmt.Fprintln(os.Stderr, "pixelparity: no files")
		os.Exit(1)
	}

	results := make([]FileResult, len(files))
	var wg sync.WaitGroup
	jobs := make(chan int)
	n := *workers
	if n <= 0 {
		n = runtime.NumCPU()
	}
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for idx := range jobs {
				results[idx] = runOne(files[idx], *refDir, *shotDir, *scale, *diffDir, *diffMin)
			}
		}()
	}
	for i := range files {
		jobs <- i
	}
	close(jobs)
	wg.Wait()

	report := summarize(results, *scale)
	printTables(report, *top, *quiet)
	if *jsonOut != "" {
		data, _ := json.MarshalIndent(report, "", " ")
		if err := os.WriteFile(*jsonOut, append(data, '\n'), 0o644); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
	}
}

func runOne(rel, refDir, shotDir string, scale float64, diffDir string, diffMin float64) (res FileResult) {
	res.File = rel
	res.Family = familyOf(rel)
	defer func() {
		if r := recover(); r != nil {
			res.Error = fmt.Sprint("panic: ", r)
		}
	}()
	svg, err := os.ReadFile(filepath.Join(refDir, rel))
	if err != nil {
		res.Error = err.Error()
		return res
	}
	shot := filepath.Join(shotDir, strings.TrimSuffix(rel, ".svg")+".png")
	ref, err := loadPNG(shot)
	if err != nil {
		res.Error = err.Error()
		return res
	}
	start, cpuStart := time.Now(), cpuTime()
	got, err := raster.Render(string(svg), raster.Options{Scale: scale, Fonts: fontLibrary})
	res.RenderMs = float64(time.Since(start).Microseconds()) / 1000
	res.RenderCPUMs = float64((cpuTime() - cpuStart).Microseconds()) / 1000
	if err != nil {
		res.Error = err.Error()
		res.Pixels = ref.Rect.Dx() * ref.Rect.Dy()
		res.Mismatch, res.MismatchTol = res.Pixels, res.Pixels
		res.MAD = 255
		return res
	}
	var diff *image.NRGBA
	if diffDir != "" {
		diff = image.NewNRGBA(image.Rect(0, 0, max(got.Rect.Dx(), ref.Rect.Dx()), max(got.Rect.Dy(), ref.Rect.Dy())))
	}
	compare(got, ref, &res, diff)
	if diff != nil && res.MismatchPct() >= diffMin && (res.Mismatch > 0 || diffMin == 0) {
		base := filepath.Join(diffDir, strings.TrimSuffix(rel, ".svg"))
		os.MkdirAll(filepath.Dir(base), 0o755)
		writePNG(base+".diff.png", diff)
		writePNG(base+".go.png", got)
	}
	return res
}

func writePNG(path string, img image.Image) {
	f, err := os.Create(path)
	if err != nil {
		return
	}
	defer f.Close()
	raster.EncodePNG(f, img)
}

func summarize(results []FileResult, scale float64) *Report {
	rep := &Report{Scale: scale, Threshold: threshold, Files: results}
	type acc struct {
		fr                    FamilyResult
		mismatch, mismatchTol int64
		mad, ssim, ms         float64
		ok                    int
	}
	byFam := map[string]*acc{}
	total := &acc{fr: FamilyResult{Family: "TOTAL"}}
	add := func(a *acc, r *FileResult) {
		a.fr.Files++
		if r.Error != "" {
			a.fr.Errors++
		}
		if r.Width != r.RefWidth || r.Height != r.RefHeight {
			a.fr.SizeMismatches++
		}
		a.fr.Pixels += int64(r.Pixels)
		a.mismatch += int64(r.Mismatch)
		a.mismatchTol += int64(r.MismatchTol)
		a.mad += float64(r.MAD * float64(r.Pixels))
		a.ssim += r.SSIM
		a.ms += r.RenderCPUMs
		a.ok++
	}
	for i := range results {
		r := &results[i]
		a := byFam[r.Family]
		if a == nil {
			a = &acc{fr: FamilyResult{Family: r.Family}}
			byFam[r.Family] = a
		}
		add(a, r)
		add(total, r)
	}
	finish := func(a *acc) FamilyResult {
		if a.fr.Pixels > 0 {
			a.fr.MismatchPct = 100 * float64(a.mismatch) / float64(a.fr.Pixels)
			a.fr.MismatchTolPct = 100 * float64(a.mismatchTol) / float64(a.fr.Pixels)
			a.fr.MAD = a.mad / float64(a.fr.Pixels)
		}
		if a.ok > 0 {
			a.fr.SSIM = a.ssim / float64(a.ok)
			a.fr.RenderMs = a.ms / float64(a.ok)
		}
		return a.fr
	}
	for _, a := range byFam {
		rep.Families = append(rep.Families, finish(a))
	}
	sort.Slice(rep.Families, func(i, j int) bool { return rep.Families[i].Family < rep.Families[j].Family })
	rep.Total = finish(total)
	return rep
}

func printTables(rep *Report, top int, quiet bool) {
	fmt.Printf("scale %g, threshold %d/255\n", rep.Scale, rep.Threshold)
	fmt.Printf("%-22s %5s %4s %4s %9s %9s %7s %7s %8s\n", "family", "files", "err", "size", "mismatch%", "tol1px%", "MAD", "SSIM", "cpu ms")
	row := func(f FamilyResult) {
		fmt.Printf("%-22s %5d %4d %4d %9.4f %9.4f %7.3f %7.4f %8.2f\n", f.Family, f.Files, f.Errors, f.SizeMismatches, f.MismatchPct, f.MismatchTolPct, f.MAD, f.SSIM, f.RenderMs)
	}
	for _, f := range rep.Families {
		row(f)
	}
	row(rep.Total)
	if top <= 0 {
		return
	}
	sorted := append([]FileResult(nil), rep.Files...)
	sort.Slice(sorted, func(i, j int) bool {
		a, b := sorted[i].MismatchPct(), sorted[j].MismatchPct()
		if a != b {
			return a > b
		}
		return sorted[i].File < sorted[j].File
	})
	fmt.Printf("\nworst %d files by mismatch:\n", min(top, len(sorted)))
	for i := 0; i < top && i < len(sorted); i++ {
		r := &sorted[i]
		tol := 0.0
		if r.Pixels > 0 {
			tol = 100 * float64(r.MismatchTol) / float64(r.Pixels)
		}
		note := ""
		if r.Error != "" {
			note = " ERROR " + r.Error
		} else if r.Width != r.RefWidth || r.Height != r.RefHeight {
			note = fmt.Sprintf(" SIZE %dx%d vs %dx%d", r.Width, r.Height, r.RefWidth, r.RefHeight)
		}
		fmt.Printf("%8.4f%% tol %8.4f%% mad %7.3f ssim %.4f  %s%s\n", r.MismatchPct(), tol, r.MAD, r.SSIM, r.File, note)
	}
	if !quiet {
		summary := func(name string, value func(*FileResult) float64) {
			var ms []float64
			for i := range rep.Files {
				ms = append(ms, value(&rep.Files[i]))
			}
			sort.Float64s(ms)
			if len(ms) > 0 {
				fmt.Printf("%s: median %.2f ms, p90 %.2f ms, max %.2f ms\n", name, ms[len(ms)/2], ms[int(math.Min(float64(len(ms)-1), float64(len(ms))*0.9))], ms[len(ms)-1])
			}
		}
		fmt.Println()
		summary("render wall time", func(r *FileResult) float64 { return r.RenderMs })
		summary("render CPU time (one worker)", func(r *FileResult) float64 { return r.RenderCPUMs })
	}
}

// cpuTime returns the CPU time that the process has used.
func cpuTime() time.Duration {
	var ru syscall.Rusage
	if err := syscall.Getrusage(syscall.RUSAGE_SELF, &ru); err != nil {
		return 0
	}
	return time.Duration(ru.Utime.Nano() + ru.Stime.Nano())
}
