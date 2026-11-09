// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/mindmap/parser/mindmap.jison (MIT).

package mindmap

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/mindmap"
	"github.com/bvolpato/mermaid-go-renderer/internal/jison"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// parseText runs the generated parser on text with db as parser.yy.
func parseText(db *DB, text string) error {
	_, err := grammar.Parse(text, db, performAction)
	return err
}

// parsedNode is the value of the productions node, nodeWithId and
// nodeWithoutId: { id, descr, type }.
type parsedNode struct {
	id    string
	descr string
	typ   int
}

// performAction is parser.performAction of the generated parser.
func performAction(yy any, s *jison.ParserState, production int) (any, error) {
	db := yy.(*DB)
	switch production {
	case 6, 7: // return yy;
		return db, nil
	case 15: // yy.addNode($1.length, $2.id, $2.descr, $2.type);
		n := s.V(2).(parsedNode)
		if err := db.AddNode(js.UTF16Len(s.S(1)), n.id, n.descr, n.typ); err != nil {
			return nil, err
		}
	case 16: // yy.decorateNode({icon: $2});
		if err := db.DecorateNode(Decoration{Icon: s.S(2)}); err != nil {
			return nil, err
		}
	case 17: // yy.decorateNode({class: $2});
		if err := db.DecorateNode(Decoration{Class: s.S(2)}); err != nil {
			return nil, err
		}
	case 19: // yy.addNode(0, $1.id, $1.descr, $1.type);
		n := s.V(1).(parsedNode)
		if err := db.AddNode(0, n.id, n.descr, n.typ); err != nil {
			return nil, err
		}
	case 20: // yy.decorateNode({icon: $1});
		if err := db.DecorateNode(Decoration{Icon: s.S(1)}); err != nil {
			return nil, err
		}
	case 21: // yy.decorateNode({class: $1});
		if err := db.DecorateNode(Decoration{Class: s.S(1)}); err != nil {
			return nil, err
		}
	case 25: // $$ = { id: $2, descr: $2, type: yy.getType($1, $3) };
		s.Result = parsedNode{id: s.S(2), descr: s.S(2), typ: db.GetType(s.S(1), s.S(3))}
	case 26: // $$ = { id: $1, descr: $1, type: yy.nodeType.DEFAULT };
		s.Result = parsedNode{id: s.S(1), descr: s.S(1), typ: nodeTypeDefault}
	case 27: // $$ = { id: $1, descr: $3, type: yy.getType($2, $4) };
		s.Result = parsedNode{id: s.S(1), descr: s.S(3), typ: db.GetType(s.S(2), s.S(4))}
	}
	return nil, nil
}
