package katex

import (
	"fmt"
	"testing"

	"github.com/dlclark/regexp2/v2"
)

func TestScratch2(t *testing.T) {
	for _, c := range []struct {
		pat string
		in  []rune
	}{
		{`[\ud800-\udbff][\udc00-\udfff]`, []rune{'a', 0xd835, 0xdc00, 'b'}},
	} {
		re, err := regexp2.Compile(c.pat, regexp2.ECMAScript, regexp2.OptionDebug())
		if err != nil {
			fmt.Println(c.pat, err)
			continue
		}
		m, err := re.FindRunesMatch(c.in)
		if m != nil {
			fmt.Printf("%q on %U => %d+%d %v\n", c.pat, c.in, m.RuneIndex, m.RuneLength, err)
		} else {
			fmt.Printf("%q on %U => nil %v\n", c.pat, c.in, err)
		}
	}
}
