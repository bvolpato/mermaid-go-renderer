package mermaid

type Theme struct {
	Background          string
	PrimaryColor        string
	PrimaryBorderColor  string
	PrimaryTextColor    string
	LineColor           string
	SecondaryColor      string
	TertiaryColor       string
	EdgeLabelBackground string
	FontFamily          string
	FontSize            float64
}

func ModernTheme() Theme {
	return Theme{
		Background:          "#ffffff",
		PrimaryColor:        "#f8f9fa",
		PrimaryBorderColor:  "#3a6ea5",
		PrimaryTextColor:    "#1b263b",
		LineColor:           "#3d5a80",
		SecondaryColor:      "#e3f2fd",
		TertiaryColor:       "#edf7ed",
		EdgeLabelBackground: "#ffffff",
		FontFamily:          "Inter, -apple-system, BlinkMacSystemFont, Segoe UI, sans-serif",
		FontSize:            13,
	}
}

func MermaidDefaultTheme() Theme {
	return Theme{
		Background:          "#ffffff",
		PrimaryColor:        "#ECECFF",
		PrimaryBorderColor:  "#9370DB",
		PrimaryTextColor:    "#333333",
		LineColor:           "#333333",
		SecondaryColor:      "#ffffde",
		TertiaryColor:       "#ffe4cc",
		EdgeLabelBackground: "#e8e8e8",
		FontFamily:          "Arial, Helvetica, sans-serif",
		FontSize:            16,
	}
}

type LayoutConfig struct {
	NodeSpacing          float64
	RankSpacing          float64
	PreferredAspectRatio *float64
	FastTextMetrics      bool
}

func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		NodeSpacing: 60,
		RankSpacing: 80,
	}
}

type RenderConfig struct {
	Width      float64
	Height     float64
	Background string
}

func DefaultRenderConfig() RenderConfig {
	return RenderConfig{
		Width:      1200,
		Height:     800,
		Background: "#ffffff",
	}
}

type Config struct {
	Theme  Theme
	Layout LayoutConfig
	Render RenderConfig
}

func DefaultConfig() Config {
	theme := ModernTheme()
	render := DefaultRenderConfig()
	render.Background = theme.Background
	return Config{
		Theme:  theme,
		Layout: DefaultLayoutConfig(),
		Render: render,
	}
}

type RenderOptions struct {
	Theme  Theme
	Layout LayoutConfig
}

func DefaultRenderOptions() RenderOptions {
	return RenderOptions{
		Theme:  ModernTheme(),
		Layout: DefaultLayoutConfig(),
	}
}

func ModernOptions() RenderOptions {
	return DefaultRenderOptions()
}

func MermaidDefaultOptions() RenderOptions {
	opts := DefaultRenderOptions()
	opts.Theme = MermaidDefaultTheme()
	return opts
}

func (o RenderOptions) WithNodeSpacing(spacing float64) RenderOptions {
	if spacing > 0 {
		o.Layout.NodeSpacing = spacing
	}
	return o
}

func (o RenderOptions) WithRankSpacing(spacing float64) RenderOptions {
	if spacing > 0 {
		o.Layout.RankSpacing = spacing
	}
	return o
}

func (o RenderOptions) WithPreferredAspectRatio(ratio float64) RenderOptions {
	if ratio > 0 {
		o.Layout.PreferredAspectRatio = &ratio
	}
	return o
}

func (o RenderOptions) WithPreferredAspectRatioParts(width, height float64) RenderOptions {
	if width > 0 && height > 0 {
		r := width / height
		o.Layout.PreferredAspectRatio = &r
	}
	return o
}
