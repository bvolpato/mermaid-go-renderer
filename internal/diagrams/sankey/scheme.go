// Ported from d3-scale-chromatic 3.1.0, d3-scale-chromatic/src/colors.js and
// d3-scale-chromatic/src/categorical/Tableau10.js (ISC).

package sankey

// colors is colors(specifier): it splits a string of hexadecimal colors of
// six digits into a list of colors.
func colors(specifier string) []string {
	n := len(specifier) / 6
	colors := make([]string, n)
	for i := 0; i < n; i++ {
		colors[i] = "#" + specifier[i*6:(i+1)*6]
	}
	return colors
}

// schemeTableau10 is d3.schemeTableau10.
var schemeTableau10 = colors("4e79a7f28e2ce1575976b7b259a14fedc949af7aa1ff9da79c755fbab0ab")
