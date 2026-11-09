// The khroma functions that the style templates call.

package styles

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/khroma"
)

// These wrappers turn an error of khroma into a throw, because the templates
// call khroma in the middle of a template literal. A colour is a dynamic
// value: a theme variable can be missing or have any type.

// channel is khroma.channel(color, channel).
func channel(color any, name byte) float64 {
	v, err := khroma.Channel(color, name)
	if err != nil {
		throw(err)
	}
	return v
}

// lighten is khroma.lighten(color, amount).
func lighten(color any, amount float64) string {
	out, err := khroma.Lighten(color, amount)
	if err != nil {
		throw(err)
	}
	return out
}

// darken is khroma.darken(color, amount).
func darken(color any, amount float64) string {
	out, err := khroma.Darken(color, amount)
	if err != nil {
		throw(err)
	}
	return out
}

// isDark is khroma.isDark(color).
func isDark(color any) bool {
	dark, err := khroma.IsDark(color)
	if err != nil {
		throw(err)
	}
	return dark
}

// fade is the helper of the same name that the flowchart, block, agentflow
// and er style files each define: the colour with a new opacity.
func fade(color any, opacity float64) string {
	r := channel(color, 'r')
	g := channel(color, 'g')
	b := channel(color, 'b')

	return khroma.RGBA(r, g, b, opacity)
}
