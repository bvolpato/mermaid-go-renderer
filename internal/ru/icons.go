// Ported from mermaid 12.2.0, rendering-util/icons.ts (MIT).
//
// Upstream keeps the registered icon packs in module variables. Here they
// belong to one render: a diagram, or the program that starts the render,
// registers its packs in the context. The reference renderer registers no
// packs, so there every icon is the unknown icon.

package ru

import (
	"errors"
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// IconLoader is IconLoader: an icon pack to register. Set Icons for a pack
// that is at hand (SyncIconLoader), or Loader for a pack that is loaded on
// first use (AsyncIconLoader). An icon set is IconifyJSON as a dynamic
// value: parse it with js.ParseJSON.
type IconLoader struct {
	Name   string
	Icons  *js.Obj[any]
	Loader func() (*js.Obj[any], error)
}

// unknownIcon is unknownIcon: the icon for a name that is not registered.
func unknownIcon() *js.Obj[any] {
	o := js.NewObj[any]()
	o.Set("body", `<g><rect width="80" height="80" style="fill: #087ebf; stroke-width: 0px;"/><text transform="translate(21.16 64.67)" style="fill: #fff; font-family: ArialMT, Arial; font-size: 67.75px;"><tspan x="0" y="0">?</tspan></text></g>`)
	o.Set("height", 80.0)
	o.Set("width", 80.0)
	return o
}

// iconStores are iconsStore and loaderStore of icons.ts.
type iconStores struct {
	iconsStore  *js.Map[string, *js.Obj[any]]
	loaderStore *js.Map[string, func() (*js.Obj[any], error)]
}

func iconState(ctx *render.Context) *iconStores {
	return ctx.Value("ru.icons", func() any {
		return &iconStores{
			iconsStore:  js.NewMap[string, *js.Obj[any]](),
			loaderStore: js.NewMap[string, func() (*js.Obj[any], error)](),
		}
	}).(*iconStores)
}

// RegisterIconPacks is registerIconPacks(iconLoaders). The packs are
// registered for the render of ctx only.
func RegisterIconPacks(ctx *render.Context, iconLoaders ...IconLoader) error {
	state := iconState(ctx)
	for _, iconLoader := range iconLoaders {
		if iconLoader.Name == "" {
			return errors.New(`Invalid icon loader. Must have a "name" property with non-empty string value.`)
		}
		if iconLoader.Loader != nil {
			state.loaderStore.Set(iconLoader.Name, iconLoader.Loader)
		} else if iconLoader.Icons != nil {
			state.iconsStore.Set(iconLoader.Name, iconLoader.Icons)
		} else {
			return errors.New(`Invalid icon loader. Must have either "icons" or "loader" property.`)
		}
	}
	return nil
}

// getRegisteredIconData is getRegisteredIconData(iconName, fallbackPrefix).
// hasFallbackPrefix is false when the fallback prefix is undefined.
func getRegisteredIconData(ctx *render.Context, iconName string, fallbackPrefix string, hasFallbackPrefix bool) (*js.Obj[any], error) {
	data, ok := stringToIcon(iconName, true, hasFallbackPrefix)
	if !ok {
		return nil, fmt.Errorf("Invalid icon name: %s", iconName)
	}
	prefix := data.prefix
	if prefix == "" {
		prefix = fallbackPrefix
	}
	if prefix == "" {
		return nil, fmt.Errorf("Icon name must contain a prefix: %s", iconName)
	}
	state := iconState(ctx)
	icons := state.iconsStore.Get(prefix)
	if icons == nil {
		loader := state.loaderStore.Get(prefix)
		if loader == nil {
			return nil, fmt.Errorf("Icon set not found: %s", data.prefix)
		}
		loaded, err := loader()
		if err != nil || loaded == nil {
			return nil, fmt.Errorf("Failed to load icon set: %s", data.prefix)
		}
		// icons = { ...loaded, prefix }
		icons = js.NewObj[any]()
		loaded.Each(func(k string, v any) { icons.Set(k, v) })
		icons.Set("prefix", prefix)
		state.iconsStore.Set(prefix, icons)
	}
	iconData, err := getIconData(icons, data.name)
	if err != nil {
		return nil, err
	}
	if iconData == nil {
		return nil, fmt.Errorf("Icon not found: %s", iconName)
	}
	return iconData, nil
}

// IsIconAvailable is isIconAvailable(iconName).
func IsIconAvailable(ctx *render.Context, iconName string) bool {
	_, err := getRegisteredIconData(ctx, iconName, "", false)
	return err == nil
}

// IconAttrs builds the extra attributes of GetIconSVG from pairs of a name
// and a value.
func IconAttrs(pairs ...string) *js.Obj[string] {
	o := js.NewObj[string]()
	for i := 0; i+1 < len(pairs); i += 2 {
		o.Set(pairs[i], pairs[i+1])
	}
	return o
}

// GetIconSVG is getIconSVG(iconName, customisations, extraAttributes): the
// svg element of an icon, as markup. An icon that is not registered gives
// the unknown icon.
//
// customisations is IconifyIconCustomisations with the optional
// fallbackPrefix: the keys are width, height, rotate, hFlip, vFlip and
// fallbackPrefix, and the values are dynamic values (float64, string, bool
// or js.Null). A key that is absent is undefined.
//
// The error is the TypeError of upstream for icon data that it cannot
// render, for example a registered icon that has no body.
func GetIconSVG(ctx *render.Context, iconName string, customisations map[string]any, extraAttributes *js.Obj[string]) (string, error) {
	fallbackPrefix := ""
	fallback, hasFallbackPrefix := customisations["fallbackPrefix"]
	if hasFallbackPrefix && fallback == nil {
		hasFallbackPrefix = false
	}
	if hasFallbackPrefix {
		// A fallback prefix is a string. Any other value that is truthy
		// names no icon set.
		fallbackPrefix, _ = fallback.(string)
	}
	iconData, err := getRegisteredIconData(ctx, iconName, fallbackPrefix, hasFallbackPrefix)
	if err != nil {
		iconData = unknownIcon()
	}
	var custom *js.Obj[any]
	if customisations != nil {
		custom = js.NewObj[any]()
		for k, v := range customisations {
			custom.Set(k, v)
		}
	}
	renderData, err := iconToSVG(iconData, custom)
	if err != nil {
		return "", err
	}
	// { ...renderData.attributes, ...extraAttributes }
	attributes := js.NewObj[string]()
	renderData.attributes.Each(func(k, v string) { attributes.Set(k, v) })
	if extraAttributes != nil {
		extraAttributes.Each(func(k, v string) { attributes.Set(k, v) })
	}
	// replaceIDs passes on a body that is not a string, and iconToHTML then
	// throws a TypeError, because it calls body.indexOf.
	body, ok := renderData.body.(string)
	if !ok {
		return "", errIconType
	}
	svg := iconToHTML(replaceIDs(ctx, body), attributes)
	return ctx.SanitizeText(svg), nil
}
