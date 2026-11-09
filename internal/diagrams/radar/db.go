// Ported from mermaid 12.2.0, diagrams/radar/db.ts and types.ts (MIT).

package radar

import (
	"errors"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// RadarAxis is one axis of the chart.
type RadarAxis struct {
	Name  string
	Label string
}

// RadarCurve is one curve of the chart, with one entry for each axis.
type RadarCurve struct {
	Name    string
	Entries []float64
	Label   string
}

// RadarOptions holds the options of the chart. A nil Max is null: the
// renderer then takes the largest entry.
type RadarOptions struct {
	ShowLegend bool
	Ticks      float64
	Max        *float64
	Min        float64
	Graticule  string
}

func defaultOptions() RadarOptions {
	return RadarOptions{
		ShowLegend: true,
		Ticks:      5,
		Max:        nil,
		Min:        0,
		Graticule:  "circle",
	}
}

// Can't see people using more than this, since the gradient makes the diagram unreadable.
const maxTicks = 32

// DB is the db of the radar chart.
type DB struct {
	render.CommonDB
	ctx     *render.Context
	axes    []RadarAxis
	curves  []RadarCurve
	options RadarOptions
}

func newDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, axes: []RadarAxis{}, curves: []RadarCurve{}, options: defaultOptions()}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetConfig is getConfig(): the default config of the radar chart with the
// config of the render on top.
func (db *DB) GetConfig() *js.Obj[any] {
	spread := js.NewObj[any]()
	js.AsObj(config.DefaultSection("radar")).Each(func(key string, v any) { spread.Set(key, v) })
	if user := js.AsObj(db.ctx.Config.Get("radar")); user != nil {
		user.Each(func(key string, v any) { spread.Set(key, v) })
	}
	return utils.CleanAndMerge(spread, nil)
}

// GetAxes is getAxes().
func (db *DB) GetAxes() []RadarAxis { return db.axes }

// GetCurves is getCurves().
func (db *DB) GetCurves() []RadarCurve { return db.curves }

// GetOptions is getOptions().
func (db *DB) GetOptions() RadarOptions { return db.options }

// SetAxes is setAxes(axes).
func (db *DB) SetAxes(axes []Axis) {
	db.axes = make([]RadarAxis, len(axes))
	for i, axis := range axes {
		label := axis.Name
		if axis.Label != nil {
			label = *axis.Label
		}
		db.axes[i] = RadarAxis{Name: axis.Name, Label: label}
	}
}

// SetCurves is setCurves(curves).
func (db *DB) SetCurves(curves []Curve) error {
	list := make([]RadarCurve, len(curves))
	for i, curve := range curves {
		label := curve.Name
		if curve.Label != nil {
			label = *curve.Label
		}
		entries, err := db.computeCurveEntries(curve.Entries)
		if err != nil {
			return err
		}
		list[i] = RadarCurve{Name: curve.Name, Label: label, Entries: entries}
	}
	db.curves = list
	return nil
}

// computeCurveEntries is computeCurveEntries(entries).
func (db *DB) computeCurveEntries(entries []Entry) ([]float64, error) {
	if len(entries) == 0 {
		// entries[0] is undefined: upstream throws a TypeError.
		return nil, errors.New("Cannot read properties of undefined (reading 'axis')")
	}
	// If entries have axis reference, we must order them according to the axes
	if entries[0].Axis == nil {
		values := make([]float64, len(entries))
		for i, entry := range entries {
			values[i] = entry.Value
		}
		return values, nil
	}
	axes := db.GetAxes()
	if len(axes) == 0 {
		return nil, errors.New("Axes must be populated before curves for reference entries")
	}
	values := make([]float64, len(axes))
	for i, axis := range axes {
		found := false
		for _, entry := range entries {
			if entry.Axis != nil && *entry.Axis == axis.Name {
				values[i] = entry.Value
				found = true
				break
			}
		}
		if !found {
			return nil, errors.New("Missing entry for axis " + axis.Label)
		}
	}
	return values, nil
}

// SetOptions is setOptions(options).
func (db *DB) SetOptions(options []Option) {
	// Create a map from option names to option objects for quick lookup
	optionMap := map[string]Option{}
	for _, option := range options {
		optionMap[option.Name] = option
	}

	db.options = defaultOptions()
	if o, ok := optionMap["showLegend"]; ok {
		db.options.ShowLegend = o.Value.(bool)
	}
	if o, ok := optionMap["ticks"]; ok {
		db.options.Ticks = o.Value.(float64)
	}
	if o, ok := optionMap["max"]; ok {
		v := o.Value.(float64)
		db.options.Max = &v
	}
	if o, ok := optionMap["min"]; ok {
		db.options.Min = o.Value.(float64)
	}
	if o, ok := optionMap["graticule"]; ok {
		db.options.Graticule = o.Value.(string)
	}

	if db.options.Ticks > maxTicks {
		db.options.Ticks = maxTicks
	}
}
