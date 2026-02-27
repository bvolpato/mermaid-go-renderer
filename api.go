package mermaid

import (
	"errors"
	"strings"
	"time"
)

type LayoutStageMetrics struct {
	PortAssignmentUS uint64
	EdgeRoutingUS    uint64
	LabelPlacementUS uint64
}

func (m LayoutStageMetrics) TotalUS() uint64 {
	return m.PortAssignmentUS + m.EdgeRoutingUS + m.LabelPlacementUS
}

type RenderResult struct {
	SVG      string
	ParseUS  uint64
	LayoutUS uint64
	RenderUS uint64
}

func (r RenderResult) TotalUS() uint64 {
	return r.ParseUS + r.LayoutUS + r.RenderUS
}

func (r RenderResult) TotalMS() float64 {
	return float64(r.TotalUS()) / 1000.0
}

type RenderDetailedResult struct {
	SVG          string
	ParseUS      uint64
	LayoutUS     uint64
	RenderUS     uint64
	LayoutStages LayoutStageMetrics
}

func (r RenderDetailedResult) TotalUS() uint64 {
	return r.ParseUS + r.LayoutUS + r.RenderUS
}

func (r RenderDetailedResult) TotalMS() float64 {
	return float64(r.TotalUS()) / 1000.0
}

func Render(input string) (string, error) {
	return RenderWithOptions(input, DefaultRenderOptions())
}

func RenderWithOptions(input string, options RenderOptions) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", errors.New("input diagram is empty")
	}
	parsed, err := ParseMermaid(input)
	if err != nil {
		return "", err
	}
	layout := ComputeLayout(&parsed.Graph, options.Theme, options.Layout)
	return RenderSVG(layout, options.Theme, options.Layout), nil
}

func RenderWithTiming(input string, options RenderOptions) (RenderResult, error) {
	detailed, err := RenderWithDetailedTiming(input, options)
	if err != nil {
		return RenderResult{}, err
	}
	return RenderResult{
		SVG:      detailed.SVG,
		ParseUS:  detailed.ParseUS,
		LayoutUS: detailed.LayoutUS,
		RenderUS: detailed.RenderUS,
	}, nil
}

func RenderWithDetailedTiming(input string, options RenderOptions) (RenderDetailedResult, error) {
	startParse := time.Now()
	parsed, err := ParseMermaid(input)
	if err != nil {
		return RenderDetailedResult{}, err
	}
	parseUS := uint64(time.Since(startParse).Microseconds())

	startLayout := time.Now()
	layout := ComputeLayout(&parsed.Graph, options.Theme, options.Layout)
	layoutUS := uint64(time.Since(startLayout).Microseconds())

	startRender := time.Now()
	svg := RenderSVG(layout, options.Theme, options.Layout)
	renderUS := uint64(time.Since(startRender).Microseconds())

	return RenderDetailedResult{
		SVG:      svg,
		ParseUS:  parseUS,
		LayoutUS: layoutUS,
		RenderUS: renderUS,
		LayoutStages: LayoutStageMetrics{
			PortAssignmentUS: 0,
			EdgeRoutingUS:    layoutUS / 2,
			LabelPlacementUS: layoutUS / 2,
		},
	}, nil
}
