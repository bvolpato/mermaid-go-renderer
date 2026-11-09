// Ported from fontconfig 2.15.0, in part: this file follows src/fcxml.c,
// src/fccfg.c, src/fcmatch.c and src/fcdefault.c (HPND-sell-variant) for
// the parts that decide which font a request gives.
// License of fontconfig: LICENSE-fontconfig.txt.

package font

import (
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// Chromium on Linux asks fontconfig for fonts. fontconfig reads rules from
// its configuration files (/etc/fonts/fonts.conf and the files it
// includes). A rule edits the request: the default configuration replaces
// the name sans with sans-serif, adds the families that are
// metric-compatible with a family, and adds a list of preferred families
// for each generic family. Then fontconfig sorts all fonts by how well they
// match the edited request.
//
// This file has the rules that edit the family list and the language of a
// request, and the sort criteria that depend on them. A rule that tests or
// edits another property (hinting, antialiasing, the pixel size) has no
// effect on which font is chosen for the requests of Chromium.

// fcValue is a value of a property of a request, with its binding. A value
// with a weak binding counts less than the language of the font
// (FcValueBindingWeak).
type fcValue struct {
	s      string
	strong bool
}

// fcPattern is a request: the values of the properties family and lang.
type fcPattern struct {
	family []fcValue
	lang   []fcValue
}

func (p *fcPattern) values(name string) *[]fcValue {
	switch name {
	case "family":
		return &p.family
	case "lang":
		return &p.lang
	}
	return nil
}

// fcTest is a test element of a rule.
type fcTest struct {
	name    string
	all     bool   // qual="all"
	compare string // eq, not_eq, contains, not_contains
	values  []string
}

// fcEdit is an edit element of a rule.
type fcEdit struct {
	name    string
	mode    string // assign, assign_replace, prepend, prepend_first, append, append_last, delete, delete_all
	binding string // weak, strong, same
	values  []string
}

// fcRule is a match element for requests (target="pattern"), or an alias
// element, which fontconfig turns into such a rule.
type fcRule struct {
	tests []fcTest
	edits []fcEdit
}

// fcConfig is a fontconfig configuration.
type fcConfig struct {
	rules []fcRule
}

// fcSameFamily is FcStrCmpIgnoreBlanksAndCase for family names.
func fcSameFamily(a, b string) bool {
	return fcFamilyKey(a) == fcFamilyKey(b)
}

// fcFamilyKey is a family name in lower case without blanks.
func fcFamilyKey(s string) string {
	if !strings.ContainsAny(s, " ABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return s
	}
	var sb strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 'a' - 'A'
		}
		sb.WriteByte(c)
	}
	return sb.String()
}

// fcLangContains reports whether the language of a request satisfies a
// language of a test with compare="contains" or "eq": the same language,
// with or without a territory (FcLangCompare).
func fcLangContains(have, want string) bool {
	have, want = strings.ToLower(have), strings.ToLower(want)
	if have == want {
		return true
	}
	primary := func(s string) string {
		if i := strings.IndexAny(s, "-_"); i >= 0 {
			return s[:i]
		}
		return s
	}
	// "ja" is satisfied by "ja-jp", and "ja-jp" by "ja". A tag such as
	// und-zsye has no territory: it must be equal.
	if primary(have) == "und" || primary(want) == "und" {
		return false
	}
	return primary(have) == primary(want) && (have == primary(have) || want == primary(want))
}

func fcValueMatches(name, have, want string) bool {
	if name == "lang" {
		return fcLangContains(have, want)
	}
	return fcSameFamily(have, want)
}

// test evaluates a test on a request (FcConfigMatchValueList). pos is the
// index of the value that the edits of the rule are relative to, or -1: the
// value that satisfied a test with qual="any", and the first value for a
// test with qual="all".
func (t *fcTest) test(p *fcPattern) (ok bool, pos int) {
	list := p.values(t.name)
	var values []fcValue
	if list != nil {
		values = *list
	}
	negative := t.compare == "not_eq" || t.compare == "not_contains"
	match := func(v fcValue) bool {
		for _, want := range t.values {
			if fcValueMatches(t.name, v.s, want) != negative {
				return true
			}
		}
		return false
	}
	if len(values) == 0 {
		// A request without the property satisfies a test with
		// qual="all" only.
		return t.all, -1
	}
	if t.all {
		for _, v := range values {
			if !match(v) {
				return false, -1
			}
		}
		return true, 0
	}
	for i, v := range values {
		if match(v) {
			return true, i
		}
	}
	return false, -1
}

// substitute is FcConfigSubstitute for a request: it applies every rule in
// the order of the configuration.
func (c *fcConfig) substitute(p *fcPattern) {
	for ri := range c.rules {
		r := &c.rules[ri]
		// The position of the tested value, for each property.
		pos := map[string]int{}
		ok := true
		for ti := range r.tests {
			t := &r.tests[ti]
			pass, at := t.test(p)
			if !pass {
				ok = false
				break
			}
			// The first test of a property gives the position.
			if _, seen := pos[t.name]; !seen {
				pos[t.name] = at
			}
		}
		if !ok {
			continue
		}
		for ei := range r.edits {
			e := &r.edits[ei]
			list := p.values(e.name)
			if list == nil {
				continue
			}
			at, tested := pos[e.name]
			if !tested {
				at = -1
			}
			// binding="same" takes the binding of the tested value, and
			// is weak without one (FcConfigAdd).
			strong := e.binding == "strong"
			if e.binding == "same" && at >= 0 && at < len(*list) {
				strong = (*list)[at].strong
			}
			add := make([]fcValue, len(e.values))
			for i, v := range e.values {
				add[i] = fcValue{v, strong}
			}
			insert := func(i int) {
				out := make([]fcValue, 0, len(*list)+len(add))
				out = append(out, (*list)[:i]...)
				out = append(out, add...)
				out = append(out, (*list)[i:]...)
				*list = out
			}
			switch e.mode {
			case "assign":
				if at >= 0 && at < len(*list) {
					out := make([]fcValue, 0, len(*list)+len(add))
					out = append(out, (*list)[:at]...)
					out = append(out, add...)
					out = append(out, (*list)[at+1:]...)
					*list = out
					// The values that follow keep their place: the tested
					// value is now the first new value.
				} else {
					*list = add
				}
			case "assign_replace":
				*list = add
			case "prepend":
				if at >= 0 && at <= len(*list) {
					insert(at)
					pos[e.name] = at + len(add)
				} else {
					insert(0)
				}
			case "prepend_first":
				insert(0)
				if at >= 0 {
					pos[e.name] = at + len(add)
				}
			case "append":
				if at >= 0 && at < len(*list) {
					insert(at + 1)
				} else {
					insert(len(*list))
				}
			case "append_last":
				insert(len(*list))
			case "delete":
				if at >= 0 && at < len(*list) {
					*list = append((*list)[:at:at], (*list)[at+1:]...)
					pos[e.name] = -1
				}
			case "delete_all":
				*list = nil
				pos[e.name] = -1
			}
		}
	}
}

// ---- configuration files ----

// fcNode is an element of a configuration file.
type fcNode struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Children []fcNode   `xml:",any"`
	Text     string     `xml:",chardata"`
}

func (n *fcNode) attr(name, fallback string) string {
	for _, a := range n.Attrs {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return fallback
}

func (n *fcNode) childStrings(tag string) []string {
	var out []string
	for i := range n.Children {
		if c := &n.Children[i]; c.XMLName.Local == tag {
			out = append(out, strings.TrimSpace(c.Text))
		}
	}
	return out
}

// fcOpen opens a file of a configuration. It lets a configuration come from
// the file system or from embedded data.
type fcOpen func(name string) (io.ReadCloser, error)

// fcList lists the configuration files of a directory, sorted by name.
type fcList func(dir string) []string

// loadFontconfig reads a configuration file and the files that it
// includes.
func loadFontconfig(file string, open fcOpen, list fcList) *fcConfig {
	cfg := &fcConfig{}
	cfg.load(file, open, list, 0)
	return cfg
}

func (c *fcConfig) load(file string, open fcOpen, list fcList, depth int) {
	if depth > 8 {
		return
	}
	rc, err := open(file)
	if err != nil {
		return
	}
	data, err := io.ReadAll(rc)
	rc.Close()
	if err != nil {
		return
	}
	var root fcNode
	dec := xml.NewDecoder(strings.NewReader(string(data)))
	dec.Strict = false
	if err := dec.Decode(&root); err != nil {
		return
	}
	for i := range root.Children {
		n := &root.Children[i]
		switch n.XMLName.Local {
		case "include":
			// An include of the user's own directory (prefix="xdg") is not
			// read: the reference browser has none.
			if n.attr("prefix", "default") == "xdg" {
				continue
			}
			target := strings.TrimSpace(n.Text)
			if !filepath.IsAbs(target) {
				target = filepath.Join(filepath.Dir(file), target)
			}
			if files := list(target); files != nil {
				for _, f := range files {
					c.load(f, open, list, depth+1)
				}
			} else {
				c.load(target, open, list, depth+1)
			}
		case "alias":
			c.addAlias(n)
		case "match":
			c.addMatch(n)
		}
	}
}

// addAlias turns an alias element into rules, as FcParseAlias does: the
// preferred families go before the family, the accepted families after it,
// and the default families at the end of the list.
func (c *fcConfig) addAlias(n *fcNode) {
	families := n.childStrings("family")
	if len(families) == 0 {
		return
	}
	binding := n.attr("binding", "weak")
	var edits []fcEdit
	for i := range n.Children {
		child := &n.Children[i]
		mode := ""
		switch child.XMLName.Local {
		case "prefer":
			mode = "prepend"
		case "accept":
			mode = "append"
		case "default":
			mode = "append_last"
		default:
			continue
		}
		if values := child.childStrings("family"); len(values) > 0 {
			edits = append(edits, fcEdit{name: "family", mode: mode, binding: binding, values: values})
		}
	}
	if len(edits) == 0 {
		return
	}
	for _, family := range families {
		c.rules = append(c.rules, fcRule{
			tests: []fcTest{{name: "family", compare: "eq", values: []string{family}}},
			edits: edits,
		})
	}
}

// addMatch reads a match element for requests. A rule with a test or an
// edit that this file does not model is dropped when the property is not
// family or lang: such a test cannot pass for the requests of Chromium, or
// such an edit does not change the choice.
func (c *fcConfig) addMatch(n *fcNode) {
	if n.attr("target", "pattern") != "pattern" {
		return
	}
	var r fcRule
	for i := range n.Children {
		child := &n.Children[i]
		switch child.XMLName.Local {
		case "test":
			if child.attr("target", "pattern") != "pattern" && child.attr("target", "pattern") != "default" {
				return
			}
			t := fcTest{
				name:    child.attr("name", ""),
				all:     child.attr("qual", "any") == "all",
				compare: child.attr("compare", "eq"),
			}
			for j := range child.Children {
				t.values = append(t.values, strings.TrimSpace(child.Children[j].Text))
			}
			if t.name != "family" && t.name != "lang" {
				// The requests of Chromium have no other property that a
				// rule tests, apart from weight, slant and scalable, which
				// the default rules do not combine with family edits.
				// A test for a missing property passes only with
				// qual="all".
				if !t.all {
					return
				}
				continue
			}
			switch t.compare {
			case "eq", "not_eq", "contains", "not_contains":
			default:
				return
			}
			r.tests = append(r.tests, t)
		case "edit":
			e := fcEdit{
				name:    child.attr("name", ""),
				mode:    child.attr("mode", "assign"),
				binding: child.attr("binding", "weak"),
			}
			if e.name != "family" && e.name != "lang" {
				continue
			}
			for j := range child.Children {
				e.values = append(e.values, strings.TrimSpace(child.Children[j].Text))
			}
			r.edits = append(r.edits, e)
		}
	}
	if len(r.edits) > 0 {
		c.rules = append(c.rules, r)
	}
}

// fcFileSystem reads a configuration from the file system.
func fcFileSystem() (fcOpen, fcList) {
	open := func(name string) (io.ReadCloser, error) { return os.Open(name) }
	list := func(dir string) []string {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			return nil
		}
		entries, err := os.ReadDir(dir)
		if err != nil {
			return []string{}
		}
		// fontconfig reads the files whose names start with a digit and
		// end in .conf, in the order of their names.
		files := []string{}
		for _, e := range entries {
			name := e.Name()
			if !e.IsDir() && len(name) > 5 && name[0] >= '0' && name[0] <= '9' && strings.HasSuffix(name, ".conf") {
				files = append(files, filepath.Join(dir, name))
			}
		}
		sort.Strings(files)
		return files
	}
	return open, list
}
