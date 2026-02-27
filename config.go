package mermaid

type Theme struct {
	Background               string
	PrimaryColor             string
	PrimaryBorderColor       string
	PrimaryTextColor         string
	LineColor                string
	SecondaryColor           string
	TertiaryColor            string
	EdgeLabelBackground      string
	ClusterBorder            string
	TextColor                string
	FontFamily               string
	FontSize                 float64
	GitColors                []string
	GitInvColors             []string
	GitBranchLabelColors     []string
	GitCommitLabelColor      string
	GitCommitLabelBackground string
	GitTagLabelColor         string
	GitTagLabelBackground    string
	GitTagLabelBorder        string
	PieColors                []string
	PieTitleTextSize         float64
	PieTitleTextColor        string
	PieSectionTextSize       float64
	PieSectionTextColor      string
	PieLegendTextSize        float64
	PieLegendTextColor       string
	PieStrokeColor           string
	PieStrokeWidth           float64
	PieOuterStrokeWidth      float64
	PieOuterStrokeColor      string
	PieOpacity               float64
}

func ModernTheme() Theme {
	primaryColor := "#F8FAFC"
	secondaryColor := "#E2E8F0"
	tertiaryColor := "#FFFFFF"
	pieColors := defaultPieColors(primaryColor, secondaryColor, tertiaryColor)
	return Theme{
		Background:               "#ffffff",
		PrimaryColor:             primaryColor,
		PrimaryBorderColor:       "#94A3B8",
		PrimaryTextColor:         "#0F172A",
		LineColor:                "#64748B",
		SecondaryColor:           secondaryColor,
		TertiaryColor:            tertiaryColor,
		EdgeLabelBackground:      "#FFFFFF",
		ClusterBorder:            "#CBD5E1",
		TextColor:                "#0F172A",
		FontFamily:               "Inter, ui-sans-serif, system-ui, -apple-system, \"Segoe UI\", sans-serif",
		FontSize:                 14,
		GitColors:                append([]string(nil), mermaidGitColors...),
		GitInvColors:             append([]string(nil), mermaidGitInvColors...),
		GitBranchLabelColors:     append([]string(nil), mermaidGitBranchLabelColors...),
		GitCommitLabelColor:      mermaidGitCommitLabelColor,
		GitCommitLabelBackground: mermaidGitCommitLabelBackground,
		GitTagLabelColor:         mermaidGitTagLabelColor,
		GitTagLabelBackground:    mermaidGitTagLabelBackground,
		GitTagLabelBorder:        mermaidGitTagLabelBorder,
		PieColors:                pieColors,
		PieTitleTextSize:         25.0,
		PieTitleTextColor:        "#0F172A",
		PieSectionTextSize:       17.0,
		PieSectionTextColor:      "#0F172A",
		PieLegendTextSize:        17.0,
		PieLegendTextColor:       "#0F172A",
		PieStrokeColor:           "#334155",
		PieStrokeWidth:           1.6,
		PieOuterStrokeWidth:      1.6,
		PieOuterStrokeColor:      "#CBD5E1",
		PieOpacity:               0.85,
	}
}

func MermaidDefaultTheme() Theme {
	primaryColor := "#ECECFF"
	secondaryColor := "#FFFFDE"
	tertiaryColor := "#ECECFF"
	pieColors := defaultPieColors(primaryColor, secondaryColor, tertiaryColor)
	return Theme{
		Background:               "#FFFFFF",
		PrimaryColor:             primaryColor,
		PrimaryBorderColor:       "#7B88A8",
		PrimaryTextColor:         "#333333",
		LineColor:                "#2F3B4D",
		SecondaryColor:           secondaryColor,
		TertiaryColor:            tertiaryColor,
		EdgeLabelBackground:      "rgba(248,250,252, 0.92)",
		ClusterBorder:            "#AAAA33",
		TextColor:                "#333333",
		FontFamily:               "'trebuchet ms', verdana, arial, sans-serif",
		FontSize:                 16,
		GitColors:                append([]string(nil), mermaidGitColors...),
		GitInvColors:             append([]string(nil), mermaidGitInvColors...),
		GitBranchLabelColors:     append([]string(nil), mermaidGitBranchLabelColors...),
		GitCommitLabelColor:      mermaidGitCommitLabelColor,
		GitCommitLabelBackground: mermaidGitCommitLabelBackground,
		GitTagLabelColor:         mermaidGitTagLabelColor,
		GitTagLabelBackground:    mermaidGitTagLabelBackground,
		GitTagLabelBorder:        mermaidGitTagLabelBorder,
		PieColors:                pieColors,
		PieTitleTextSize:         25.0,
		PieTitleTextColor:        "#333333",
		PieSectionTextSize:       17.0,
		PieSectionTextColor:      "#333333",
		PieLegendTextSize:        17.0,
		PieLegendTextColor:       "#333333",
		PieStrokeColor:           "#000000",
		PieStrokeWidth:           2.0,
		PieOuterStrokeWidth:      2.0,
		PieOuterStrokeColor:      "#000000",
		PieOpacity:               0.7,
	}
}

var mermaidGitColors = []string{
	"hsl(240, 100%, 46.2745098039%)",
	"hsl(60, 100%, 43.5294117647%)",
	"hsl(80, 100%, 46.2745098039%)",
	"hsl(210, 100%, 46.2745098039%)",
	"hsl(180, 100%, 46.2745098039%)",
	"hsl(150, 100%, 46.2745098039%)",
	"hsl(300, 100%, 46.2745098039%)",
	"hsl(0, 100%, 46.2745098039%)",
}

var mermaidGitInvColors = []string{
	"hsl(60, 100%, 3.7254901961%)",
	"rgb(0, 0, 160.5)",
	"rgb(48.8333333334, 0, 146.5000000001)",
	"rgb(146.5000000001, 73.2500000001, 0)",
	"rgb(146.5000000001, 0, 0)",
	"rgb(146.5000000001, 0, 73.2500000001)",
	"rgb(0, 146.5000000001, 0)",
	"rgb(0, 146.5000000001, 146.5000000001)",
}

var mermaidGitBranchLabelColors = []string{
	"#ffffff", "black", "black", "#ffffff", "black", "black", "black", "black",
}

const (
	mermaidGitCommitLabelColor      = "#000021"
	mermaidGitCommitLabelBackground = "#ffffde"
	mermaidGitTagLabelColor         = "#131300"
	mermaidGitTagLabelBackground    = "#ECECFF"
	mermaidGitTagLabelBorder        = "hsl(240, 60%, 86.2745098039%)"
)

func defaultPieColors(primary, secondary, tertiary string) []string {
	return []string{
		primary,
		secondary,
		tertiary,
		adjustColor(primary, 0.0, 0.0, -10.0),
		adjustColor(secondary, 0.0, 0.0, -10.0),
		adjustColor(tertiary, 0.0, 0.0, -10.0),
		adjustColor(primary, 60.0, 0.0, -10.0),
		adjustColor(primary, -60.0, 0.0, -10.0),
		adjustColor(primary, 120.0, 0.0, 0.0),
		adjustColor(primary, 60.0, 0.0, -20.0),
		adjustColor(primary, -60.0, 0.0, -20.0),
		adjustColor(primary, 120.0, 0.0, -10.0),
	}
}

type PieConfig struct {
	TextPosition               float64
	Height                     float64
	Margin                     float64
	LegendRectSize             float64
	LegendSpacing              float64
	LegendHorizontalMultiplier float64
	MinPercent                 float64
	UseMaxWidth                bool
}

func DefaultPieConfig() PieConfig {
	return PieConfig{
		TextPosition:               0.75,
		Height:                     360.0,
		Margin:                     28.0,
		LegendRectSize:             14.0,
		LegendSpacing:              3.0,
		LegendHorizontalMultiplier: 10.0,
		MinPercent:                 1.0,
		UseMaxWidth:                true,
	}
}

type GitGraphConfig struct {
	DiagramPadding                         float64
	TitleTopMargin                         float64
	UseMaxWidth                            bool
	MainBranchName                         string
	MainBranchOrder                        float64
	ShowCommitLabel                        bool
	ShowBranches                           bool
	RotateCommitLabel                      bool
	ParallelCommits                        bool
	CommitStep                             float64
	LayoutOffset                           float64
	DefaultPos                             float64
	BranchSpacing                          float64
	BranchSpacingRotateExtra               float64
	BranchLabelRotateExtra                 float64
	BranchLabelTranslateX                  float64
	BranchLabelBGOffsetX                   float64
	BranchLabelBGOffsetY                   float64
	BranchLabelBGPadX                      float64
	BranchLabelBGPadY                      float64
	BranchLabelTextOffsetX                 float64
	BranchLabelTextOffsetY                 float64
	BranchLabelTBBGOffsetX                 float64
	BranchLabelTBTextOffsetX               float64
	BranchLabelTBOffsetY                   float64
	BranchLabelBTOffsetY                   float64
	BranchLabelCornerRadius                float64
	BranchLabelFontSize                    float64
	BranchLabelLineHeight                  float64
	TextWidthScale                         float64
	CommitLabelFontSize                    float64
	CommitLabelLineHeight                  float64
	CommitLabelOffsetY                     float64
	CommitLabelBGOffsetY                   float64
	CommitLabelPadding                     float64
	CommitLabelBGOpacity                   float64
	CommitLabelRotateAngle                 float64
	CommitLabelRotateTranslateXBase        float64
	CommitLabelRotateTranslateXScale       float64
	CommitLabelRotateTranslateXWidthOffset float64
	CommitLabelRotateTranslateYBase        float64
	CommitLabelRotateTranslateYScale       float64
	CommitLabelTBTextExtra                 float64
	CommitLabelTBBGExtra                   float64
	CommitLabelTBTextOffsetY               float64
	CommitLabelTBBGOffsetY                 float64
	TagLabelFontSize                       float64
	TagLabelLineHeight                     float64
	TagTextOffsetY                         float64
	TagPolygonOffsetY                      float64
	TagSpacingY                            float64
	TagPaddingX                            float64
	TagPaddingY                            float64
	TagHoleRadius                          float64
	TagRotateTranslate                     float64
	TagTextRotateTranslate                 float64
	TagRotateAngle                         float64
	TagTextOffsetXTB                       float64
	TagTextOffsetYTB                       float64
	ArrowRerouteRadius                     float64
	ArrowRadius                            float64
	LaneSpacing                            float64
	LaneMaxDepth                           int
	CommitRadius                           float64
	MergeRadiusOuter                       float64
	MergeRadiusInner                       float64
	HighlightOuterSize                     float64
	HighlightInnerSize                     float64
	ReverseCrossSize                       float64
	ReverseStrokeWidth                     float64
	CherryPickDotRadius                    float64
	CherryPickDotOffsetX                   float64
	CherryPickDotOffsetY                   float64
	CherryPickStemStartOffsetY             float64
	CherryPickStemEndOffsetY               float64
	CherryPickStemStrokeWidth              float64
	CherryPickAccentColor                  string
	ArrowStrokeWidth                       float64
	BranchStrokeWidth                      float64
	BranchDasharray                        string
}

func DefaultGitGraphConfig() GitGraphConfig {
	return GitGraphConfig{
		DiagramPadding:                         6.0,
		TitleTopMargin:                         22.0,
		UseMaxWidth:                            true,
		MainBranchName:                         "main",
		MainBranchOrder:                        0.0,
		ShowCommitLabel:                        true,
		ShowBranches:                           true,
		RotateCommitLabel:                      true,
		ParallelCommits:                        false,
		CommitStep:                             34.0,
		LayoutOffset:                           8.0,
		DefaultPos:                             24.0,
		BranchSpacing:                          40.0,
		BranchSpacingRotateExtra:               32.0,
		BranchLabelRotateExtra:                 24.0,
		BranchLabelTranslateX:                  -16.0,
		BranchLabelBGOffsetX:                   3.0,
		BranchLabelBGOffsetY:                   6.0,
		BranchLabelBGPadX:                      14.0,
		BranchLabelBGPadY:                      3.0,
		BranchLabelTextOffsetX:                 10.0,
		BranchLabelTextOffsetY:                 -1.0,
		BranchLabelTBBGOffsetX:                 8.0,
		BranchLabelTBTextOffsetX:               4.0,
		BranchLabelTBOffsetY:                   0.0,
		BranchLabelBTOffsetY:                   0.0,
		BranchLabelCornerRadius:                4.0,
		BranchLabelFontSize:                    0.0,
		BranchLabelLineHeight:                  1.54,
		TextWidthScale:                         1.0,
		CommitLabelFontSize:                    10.0,
		CommitLabelLineHeight:                  1.2,
		CommitLabelOffsetY:                     20.0,
		CommitLabelBGOffsetY:                   10.5,
		CommitLabelPadding:                     1.5,
		CommitLabelBGOpacity:                   0.5,
		CommitLabelRotateAngle:                 -45.0,
		CommitLabelRotateTranslateXBase:        -6.0,
		CommitLabelRotateTranslateXScale:       8.0 / 25.0,
		CommitLabelRotateTranslateXWidthOffset: 8.0,
		CommitLabelRotateTranslateYBase:        8.0,
		CommitLabelRotateTranslateYScale:       7.5 / 25.0,
		CommitLabelTBTextExtra:                 12.0,
		CommitLabelTBBGExtra:                   16.0,
		CommitLabelTBTextOffsetY:               -10.0,
		CommitLabelTBBGOffsetY:                 -10.0,
		TagLabelFontSize:                       10.0,
		TagLabelLineHeight:                     1.2,
		TagTextOffsetY:                         13.0,
		TagPolygonOffsetY:                      16.0,
		TagSpacingY:                            16.0,
		TagPaddingX:                            3.0,
		TagPaddingY:                            1.5,
		TagHoleRadius:                          1.3,
		TagRotateTranslate:                     10.0,
		TagTextRotateTranslate:                 12.0,
		TagRotateAngle:                         45.0,
		TagTextOffsetXTB:                       4.0,
		TagTextOffsetYTB:                       2.0,
		ArrowRerouteRadius:                     8.0,
		ArrowRadius:                            16.0,
		LaneSpacing:                            8.0,
		LaneMaxDepth:                           5,
		CommitRadius:                           8.0,
		MergeRadiusOuter:                       7.5,
		MergeRadiusInner:                       5.0,
		HighlightOuterSize:                     16.0,
		HighlightInnerSize:                     10.0,
		ReverseCrossSize:                       4.0,
		ReverseStrokeWidth:                     2.5,
		CherryPickDotRadius:                    2.2,
		CherryPickDotOffsetX:                   2.5,
		CherryPickDotOffsetY:                   1.6,
		CherryPickStemStartOffsetY:             0.8,
		CherryPickStemEndOffsetY:               -4.0,
		CherryPickStemStrokeWidth:              0.8,
		CherryPickAccentColor:                  "#fff",
		ArrowStrokeWidth:                       6.0,
		BranchStrokeWidth:                      0.8,
		BranchDasharray:                        "2",
	}
}

type LayoutConfig struct {
	NodeSpacing          float64
	RankSpacing          float64
	LabelLineHeight      float64
	PreferredAspectRatio *float64
	FastTextMetrics      bool
	AllowApproximate     bool
	Pie                  PieConfig
	GitGraph             GitGraphConfig
}

func DefaultLayoutConfig() LayoutConfig {
	return LayoutConfig{
		NodeSpacing:     50,
		RankSpacing:     50,
		LabelLineHeight: 1.5,
		Pie:             DefaultPieConfig(),
		GitGraph:        DefaultGitGraphConfig(),
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
	theme := MermaidDefaultTheme()
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
		Theme:  MermaidDefaultTheme(),
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

func (o RenderOptions) WithAllowApproximate(allow bool) RenderOptions {
	o.Layout.AllowApproximate = allow
	return o
}
