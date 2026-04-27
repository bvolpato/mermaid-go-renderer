package mermaid

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"
)

type samplesConformanceBaseline struct {
	Width             int                              `json:"width"`
	Height            int                              `json:"height"`
	GeneratedAt       string                           `json:"generated_at"`
	ReferenceRenderer string                           `json:"reference_renderer"`
	ReferenceVersion  string                           `json:"reference_version,omitempty"`
	NodeVersion       string                           `json:"node_version,omitempty"`
	FixtureCount      int                              `json:"fixture_count"`
	Entries           []samplesConformanceBaselineEntry `json:"entries"`
}

type samplesConformanceBaselineEntry struct {
	Sample   string  `json:"sample"`
	Mismatch float64 `json:"mismatch"`
}

func TestSamplesConformanceAgainstMMDC(t *testing.T) {
	if os.Getenv("MMDG_SAMPLES_CONFORMANCE") != "1" {
		t.Skip("set MMDG_SAMPLES_CONFORMANCE=1 to run samples conformance checks")
	}

	samples := discoverSampleFixtures(t)
	if len(samples) == 0 {
		t.Fatal("no .mmd files found in samples/")
	}

	const width = 1600
	const height = 1200
	updateBaseline := os.Getenv("MMDG_SAMPLES_UPDATE_BASELINE") == "1"
	allowedRegression := 0.01

	baselinePath := filepath.Join("testdata", "conformance", "samples_mismatch_baseline.json")
	baselineMap := map[string]float64{}
	if !updateBaseline {
		data, err := os.ReadFile(baselinePath)
		if err != nil {
			t.Fatalf("load samples baseline %s: %v (run with MMDG_SAMPLES_UPDATE_BASELINE=1 to generate)", baselinePath, err)
		}
		var baseline samplesConformanceBaseline
		if err := json.Unmarshal(data, &baseline); err != nil {
			t.Fatalf("parse samples baseline: %v", err)
		}
		for _, entry := range baseline.Entries {
			baselineMap[entry.Sample] = entry.Mismatch
		}
	}

	computed := map[string]float64{}
	for _, sample := range samples {
		sample := sample
		t.Run(sample.name, func(t *testing.T) {
			refImg, refPNG := renderWithMMDCPNG(t, sample.diagram, width, height)
			gotImg, gotPNG, gotSVG := renderWithMMDGPNG(t, sample.diagram, width, height)

			mismatch, meanDelta, considered := compareDrawnPixels(gotImg, refImg)
			computed[sample.name] = mismatch
			t.Logf(
				"sample=%s mismatch=%.4f mean_delta=%.4f considered_pixels=%d",
				sample.name, mismatch, meanDelta, considered,
			)

			if updateBaseline {
				return
			}

			baselineMismatch, ok := baselineMap[sample.name]
			if !ok {
				saveConformanceArtifacts(t, "samples-"+sample.name, gotSVG, refPNG, gotImg, refImg)
				savePNGConformanceOutputs(t, "samples-"+sample.name, gotPNG, refPNG)
				t.Fatalf("missing samples baseline entry for %s (rerun with MMDG_SAMPLES_UPDATE_BASELINE=1)", sample.name)
			}
			if mismatch > baselineMismatch+allowedRegression {
				saveConformanceArtifacts(t, "samples-"+sample.name, gotSVG, refPNG, gotImg, refImg)
				savePNGConformanceOutputs(t, "samples-"+sample.name, gotPNG, refPNG)
				t.Fatalf(
					"samples mismatch regression %.4f > baseline %.4f + allowed %.4f for %s",
					mismatch, baselineMismatch, allowedRegression, sample.name,
				)
			}

			if mismatch > 0.10 {
				t.Logf("⚠ sample %s mismatch %.4f still exceeds 10%% target", sample.name, mismatch)
			}
		})
	}

	if updateBaseline {
		baseline := samplesConformanceBaseline{
			Width:             width,
			Height:            height,
			GeneratedAt:       time.Now().UTC().Format(time.RFC3339),
			ReferenceRenderer: "mmdc",
			FixtureCount:      len(computed),
			Entries:           make([]samplesConformanceBaselineEntry, 0, len(computed)),
		}
		baseline.ReferenceVersion, baseline.NodeVersion = detectReferenceVersions()

		names := make([]string, 0, len(computed))
		for name := range computed {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			baseline.Entries = append(baseline.Entries, samplesConformanceBaselineEntry{
				Sample:   name,
				Mismatch: computed[name],
			})
		}

		data, err := json.MarshalIndent(baseline, "", "  ")
		if err != nil {
			t.Fatalf("marshal samples baseline: %v", err)
		}
		if err := os.MkdirAll(filepath.Dir(baselinePath), 0o755); err != nil {
			t.Fatalf("create testdata dir: %v", err)
		}
		if err := os.WriteFile(baselinePath, append(data, '\n'), 0o644); err != nil {
			t.Fatalf("write samples baseline: %v", err)
		}
		t.Logf("updated samples baseline at %s with %d entries", baselinePath, len(baseline.Entries))

		// Print summary
		t.Logf("\n%-35s %10s", "SAMPLE", "MISMATCH")
		t.Logf("%s", strings.Repeat("-", 50))
		totalMismatch := 0.0
		for _, name := range names {
			m := computed[name]
			totalMismatch += m
			flag := "✓"
			if m > 0.10 {
				flag = "✗"
			}
			t.Logf("%-35s %9.4f%% %s", name, m*100, flag)
		}
		avg := totalMismatch / float64(len(names))
		t.Logf("%s", strings.Repeat("-", 50))
		t.Logf("%-35s %9.4f%%", "AVERAGE", avg*100)
	}
}

type sampleFixture struct {
	name    string
	diagram string
}

func discoverSampleFixtures(t *testing.T) []sampleFixture {
	t.Helper()
	entries, err := os.ReadDir("samples")
	if err != nil {
		t.Fatalf("read samples dir: %v", err)
	}
	var fixtures []sampleFixture
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".mmd") {
			continue
		}
		name := strings.TrimSuffix(e.Name(), ".mmd")
		content, err := os.ReadFile(filepath.Join("samples", e.Name()))
		if err != nil {
			t.Fatalf("read %s: %v", e.Name(), err)
		}
		fixtures = append(fixtures, sampleFixture{
			name:    name,
			diagram: strings.TrimSpace(string(content)),
		})
	}
	sort.Slice(fixtures, func(i, j int) bool {
		return fixtures[i].name < fixtures[j].name
	})
	return fixtures
}

func TestSamplesConformanceBaselineCompleteness(t *testing.T) {
	baselinePath := filepath.Join("testdata", "conformance", "samples_mismatch_baseline.json")
	data, err := os.ReadFile(baselinePath)
	if err != nil {
		t.Skip("samples baseline not yet generated")
	}
	var baseline samplesConformanceBaseline
	if err := json.Unmarshal(data, &baseline); err != nil {
		t.Fatalf("parse baseline: %v", err)
	}
	samples := discoverSampleFixtures(t)

	baselineNames := map[string]bool{}
	for _, entry := range baseline.Entries {
		baselineNames[entry.Sample] = true
	}

	var missing []string
	for _, s := range samples {
		if !baselineNames[s.name] {
			missing = append(missing, s.name)
		}
	}
	if len(missing) > 0 {
		t.Fatalf("samples baseline missing entries for: %s (rerun with MMDG_SAMPLES_UPDATE_BASELINE=1)", strings.Join(missing, ", "))
	}

	if baseline.FixtureCount != len(baseline.Entries) {
		t.Fatalf("fixture_count mismatch: metadata=%d entries=%d", baseline.FixtureCount, len(baseline.Entries))
	}

	// Summary
	totalMismatch := 0.0
	passing := 0
	for _, entry := range baseline.Entries {
		totalMismatch += entry.Mismatch
		if entry.Mismatch <= 0.10 {
			passing++
		}
	}
	avg := totalMismatch / float64(len(baseline.Entries))
	t.Logf("samples conformance: %d/%d passing (<=10%%), average mismatch: %.4f%%", passing, len(baseline.Entries), avg*100)

	// Show worst offenders
	sorted := make([]samplesConformanceBaselineEntry, len(baseline.Entries))
	copy(sorted, baseline.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Mismatch > sorted[j].Mismatch
	})
	t.Logf("\nTop 10 worst mismatches:")
	for i, entry := range sorted {
		if i >= 10 {
			break
		}
		t.Logf("  %-35s %9.4f%%", entry.Sample, entry.Mismatch*100)
	}
	_ = fmt.Sprintf("") // suppress unused import
}
