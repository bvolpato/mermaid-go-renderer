// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/c4/parser/c4Diagram.jison (MIT). The actions are listed in
// internal/grammar/c4diagram/actions.js.txt.

package c4

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/grammar/c4diagram"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// parse is parser.parse(text) with parser.yy set to the db.
func parse(_ *render.Context, d render.DB, text string) error {
	_, err := c4diagram.Parse(text, d.(*db), performAction)
	return err
}

// attributes is $2 of a statement: the array of the attributes.
func attributes(v any) []any {
	a, _ := v.([]any)
	return a
}

// arg is one argument of `fn(...attrs)`: undefined after the last attribute.
func arg(attrs []any, i int) any {
	if i < len(attrs) {
		return attrs[i]
	}
	return nil
}

// splice2 is `attrs.splice(2, 0, value)`: it inserts the value at index 2,
// or at the end of a shorter array.
func splice2(attrs []any, value any) []any {
	at := min(2, len(attrs))
	out := make([]any, 0, len(attrs)+1)
	out = append(out, attrs[:at]...)
	out = append(out, value)
	return append(out, attrs[at:]...)
}

// personOrSystemTypes, containerTypes and componentTypes give the first
// argument of the add function for each production.
var personOrSystemTypes = map[int]string{
	39: "person",
	40: "external_person",
	41: "system",
	42: "system_db",
	43: "system_queue",
	44: "external_system",
	45: "external_system_db",
	46: "external_system_queue",
}

var containerTypes = map[int]string{
	47: "container",
	48: "container_db",
	49: "container_queue",
	50: "external_container",
	51: "external_container_db",
	52: "external_container_queue",
}

var componentTypes = map[int]string{
	53: "component",
	54: "component_db",
	55: "component_queue",
	56: "external_component",
	57: "external_component_db",
	58: "external_component_queue",
}

var relTypes = map[int]string{
	60: "rel",
	61: "birel",
	62: "rel_u",
	63: "rel_d",
	64: "rel_l",
	65: "rel_r",
	66: "rel_b",
}

var nodeTypes = map[int]string{
	32: "node",
	33: "nodeL",
	34: "nodeR",
}

// performAction is the generated parser.performAction.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	d := yy.(*db)
	switch production {
	case 3, 4, 5, 6:
		// yy.setDirection('TB'): the db of the C4 diagram has no setDirection.
		return nil, &jison.Error{Name: "TypeError", Message: "yy.setDirection is not a function"}
	case 8, 9, 10, 11, 12:
		d.setC4Type(s.S(1))
	case 19:
		// yy.setTitle($1.substring(6));$$=$1.substring(6);
		title := js.StringSliceFrom(s.S(1), 6)
		d.setTitle(title)
		s.Result = title
	case 20:
		// yy.setAccDescription($1.substring(15));$$=$1.substring(15);
		descr := js.StringSliceFrom(s.S(1), 15)
		d.SetAccDescription(descr)
		s.Result = descr
	case 21:
		// $$=$2.trim();yy.setTitle($$);
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.setTitle(value)
	case 22:
		// $$=$2.trim();yy.setAccDescription($$);
		value := js.TrimSpace(s.S(2))
		s.Result = value
		d.SetAccDescription(value)
	case 23:
		// $$=$1.trim();yy.setAccDescription($$);
		value := js.TrimSpace(s.S(1))
		s.Result = value
		d.SetAccDescription(value)
	case 28, 29:
		// $2.splice(2, 0, 'ENTERPRISE'); yy.addPersonOrSystemBoundary(...$2); $$=$2;
		kind := "ENTERPRISE"
		if production == 29 {
			kind = "SYSTEM"
		}
		a := splice2(attributes(s.V(2)), kind)
		if err := d.addPersonOrSystemBoundary(arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4)); err != nil {
			return nil, err
		}
		s.Result = a
	case 30:
		// yy.addPersonOrSystemBoundary(...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addPersonOrSystemBoundary(arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4)); err != nil {
			return nil, err
		}
		s.Result = a
	case 31:
		// $2.splice(2, 0, 'CONTAINER'); yy.addContainerBoundary(...$2); $$=$2;
		a := splice2(attributes(s.V(2)), "CONTAINER")
		if err := d.addContainerBoundary(arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4)); err != nil {
			return nil, err
		}
		s.Result = a
	case 32, 33, 34:
		// yy.addDeploymentNode('node', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addDeploymentNode(nodeTypes[production], arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6)); err != nil {
			return nil, err
		}
		s.Result = a
	case 35:
		d.popBoundaryParseStack()
	case 39, 40, 41, 42, 43, 44, 45, 46:
		// yy.addPersonOrSystem('person', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addPersonOrSystem(personOrSystemTypes[production], arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5)); err != nil {
			return nil, err
		}
		s.Result = a
	case 47, 48, 49, 50, 51, 52:
		// yy.addContainer('container', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addContainer(containerTypes[production], arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6)); err != nil {
			return nil, err
		}
		s.Result = a
	case 53, 54, 55, 56, 57, 58:
		// yy.addComponent('component', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addComponent(componentTypes[production], arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6)); err != nil {
			return nil, err
		}
		s.Result = a
	case 60, 61, 62, 63, 64, 65, 66:
		// yy.addRel('rel', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.addRel(relTypes[production], arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6), arg(a, 7)); err != nil {
			return nil, err
		}
		s.Result = a
	case 67:
		// $2.splice(0, 1); yy.addRel('rel', ...$2); $$=$2;
		a := attributes(s.V(2))
		if len(a) > 0 {
			a = a[1:]
		}
		if err := d.addRel("rel", arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6), arg(a, 7)); err != nil {
			return nil, err
		}
		s.Result = a
	case 68:
		// yy.updateElStyle('update_el_style', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.updateElStyle("update_el_style", arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5), arg(a, 6), arg(a, 7), arg(a, 8), arg(a, 9)); err != nil {
			return nil, err
		}
		s.Result = a
	case 69:
		// yy.updateRelStyle('update_rel_style', ...$2); $$=$2;
		a := attributes(s.V(2))
		if err := d.updateRelStyle("update_rel_style", arg(a, 0), arg(a, 1), arg(a, 2), arg(a, 3), arg(a, 4), arg(a, 5)); err != nil {
			return nil, err
		}
		s.Result = a
	case 70:
		// yy.updateLayoutConfig('update_layout_config', ...$2); $$=$2;
		a := attributes(s.V(2))
		d.updateLayoutConfig("update_layout_config", arg(a, 0), arg(a, 1))
		s.Result = a
	case 71:
		// $$ = [$1];
		s.Result = []any{s.V(1)}
	case 72:
		// $2.unshift($1); $$=$2;
		rest := attributes(s.V(2))
		a := make([]any, 0, len(rest)+1)
		a = append(a, s.V(1))
		s.Result = append(a, rest...)
	case 73, 75:
		// $$ = $1.trim();
		s.Result = js.TrimSpace(s.S(1))
	case 74:
		// let kv={}; kv[$1.trim()]=$2.trim(); $$=kv;
		kv := js.NewObj[any]()
		setKey(kv, js.TrimSpace(s.S(1)), js.TrimSpace(s.S(2)))
		s.Result = kv
	case 76:
		// $$ = "";
		s.Result = ""
	}
	return nil, nil
}
