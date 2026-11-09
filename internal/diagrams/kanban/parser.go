// Ported from mermaid 12.2.0, the semantic actions of
// diagrams/kanban/parser/kanban.jison (MIT).

package kanban

import (
	grammar "github.com/bvolpato/mermaid-go-renderer/internal/grammar/kanban"
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
	case 15: // yy.addNode($1.length, $2.id, $2.descr, $2.type, $3);
		n := s.V(2).(parsedNode)
		shapeData := s.S(3)
		if err := db.AddNode(js.UTF16Len(s.S(1)), n.id, n.descr, n.typ, &shapeData); err != nil {
			return nil, err
		}
	case 16: // yy.addNode($1.length, $2.id, $2.descr, $2.type);
		n := s.V(2).(parsedNode)
		if err := db.AddNode(js.UTF16Len(s.S(1)), n.id, n.descr, n.typ, nil); err != nil {
			return nil, err
		}
	case 17: // yy.decorateNode({icon: $2});
		if err := db.DecorateNode(Decoration{Icon: s.S(2)}); err != nil {
			return nil, err
		}
	case 18: // yy.decorateNode({class: $2});
		if err := db.DecorateNode(Decoration{Class: s.S(2)}); err != nil {
			return nil, err
		}
	case 20: // yy.addNode(0, $1.id, $1.descr, $1.type, $2);
		n := s.V(1).(parsedNode)
		shapeData := s.S(2)
		if err := db.AddNode(0, n.id, n.descr, n.typ, &shapeData); err != nil {
			return nil, err
		}
	case 21: // yy.addNode(0, $1.id, $1.descr, $1.type);
		n := s.V(1).(parsedNode)
		if err := db.AddNode(0, n.id, n.descr, n.typ, nil); err != nil {
			return nil, err
		}
	case 22: // yy.decorateNode({icon: $1});
		if err := db.DecorateNode(Decoration{Icon: s.S(1)}); err != nil {
			return nil, err
		}
	case 23: // yy.decorateNode({class: $1});
		if err := db.DecorateNode(Decoration{Class: s.S(1)}); err != nil {
			return nil, err
		}
	case 27: // $$ = { id: $2, descr: $2, type: yy.getType($1, $3) };
		s.Result = parsedNode{id: s.S(2), descr: s.S(2), typ: db.GetType(s.S(1), s.S(3))}
	case 28: // $$ = { id: $1, descr: $1, type: 0 };
		s.Result = parsedNode{id: s.S(1), descr: s.S(1), typ: 0}
	case 29: // $$ = { id: $1, descr: $3, type: yy.getType($2, $4) };
		s.Result = parsedNode{id: s.S(1), descr: s.S(3), typ: db.GetType(s.S(2), s.S(4))}
	case 30: // $$ = $1 + $2;
		s.Result = js.Concat(s.S(1), s.S(2))
	case 31: // $$ = $1;
		s.Result = s.V(1)
	}
	return nil, nil
}
