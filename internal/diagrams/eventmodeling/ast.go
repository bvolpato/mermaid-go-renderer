// Ported from mermaid 12.2.0, the grammar
// packages/parser/src/language/eventmodeling/event-modeling.langium and the
// generated AST types of the parser package (MIT).

package eventmodeling

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// EventModel is the AST of the entry rule EventModel. A nil pointer is a
// property that the text does not set: undefined in the AST.
type EventModel struct {
	AccDescr *string
	AccTitle *string
	Title    *string

	ModelEntities []*EmModelEntity
	Frames        []*EmFrame
	DataEntities  []*EmDataEntity
	NoteEntities  []*EmNoteEntity
	GwtEntities   []*EmGwt
}

// The values of EmFrame.Type: the $type of the two frame rules.
const (
	typeTimeFrame  = "EmTimeFrame"
	typeResetFrame = "EmResetFrame"
)

// EmFrame is the AST of the rules EmTimeFrame and EmResetFrame. A cross
// reference is the text of its token ($refText): the db reads nothing else
// of a reference.
type EmFrame struct {
	// Type is the $type: EmTimeFrame or EmResetFrame.
	Type             string
	Name             string
	ModelEntityType  string
	EntityIdentifier string
	SourceFrames     []string
	DataReference    *string
	DataType         *string
	DataInlineValue  *string
	// Line is $cstNode.range.start.line: the line of the first token of the
	// frame, counted from 0.
	Line int
}

// EmModelEntity is the AST of the rule EmModelEntity.
type EmModelEntity struct {
	Name string
}

// EmDataEntity is the AST of the rule EmDataEntity.
type EmDataEntity struct {
	Name           string
	DataType       *string
	DataBlockValue string
}

// EmNoteEntity is the AST of the rule EmNoteEntity.
type EmNoteEntity struct {
	SourceFrame    string
	DataType       *string
	DataBlockValue string
}

// EmGwt is the AST of the rule EmGwt.
type EmGwt struct {
	SourceFrame     string
	GivenStatements []*EmGwtStatement
	WhenStatements  []*EmGwtStatement
	ThenStatements  []*EmGwtStatement
}

// EmGwtStatement is the AST of the rule EmGwtStatement. The grammar does
// not assign the entity type of the statement to a property.
type EmGwtStatement struct {
	EntityIdentifier string
}

var lexer = langium.MustLexer("eventmodeling")

// The keywords of the data type rules EmModelEntityType and EmDataType.
var (
	modelEntityTypes = []string{"rmo", "readmodel", "ui", "cmd", "command", "evt", "event", "pcr", "processor"}
	dataTypes        = []string{"json", "jsobj", "figma", "salt", "uri", "md", "html", "text"}
)

// parse is parse('eventmodeling', text) of the parser package. The grammar
// is:
//
//	entry EventModel:
//	    'eventmodeling'
//	    (accDescr=EM_ACC_DESCR | accTitle=EM_ACC_TITLE | title=EM_TITLE
//	    | modelEntities+=EmModelEntity | frames+=EmFrame
//	    | dataEntities+=EmDataEntity | noteEntities+=EmNoteEntity
//	    | gwtEntities+=EmGwt)*;
//
// The terminals EM_ACC_DESCR, EM_ACC_TITLE and EM_TITLE have no value
// converter, so the property is the text of the token.
func parse(text string) (*EventModel, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &EventModel{
		ModelEntities: []*EmModelEntity{},
		Frames:        []*EmFrame{},
		DataEntities:  []*EmDataEntity{},
		NoteEntities:  []*EmNoteEntity{},
		GwtEntities:   []*EmGwt{},
	}
	if _, err := p.Expect("eventmodeling"); err != nil {
		return nil, err
	}
	for {
		switch {
		case p.At("EM_ACC_DESCR"):
			image := p.Next().Image
			ast.AccDescr = &image
		case p.At("EM_ACC_TITLE"):
			image := p.Next().Image
			ast.AccTitle = &image
		case p.At("EM_TITLE"):
			image := p.Next().Image
			ast.Title = &image
		case p.At("entity"):
			entity, err := emModelEntity(p)
			if err != nil {
				return nil, err
			}
			ast.ModelEntities = append(ast.ModelEntities, entity)
		case p.At("tf", "timeframe", "rf", "resetframe"):
			frame, err := emFrame(p)
			if err != nil {
				return nil, err
			}
			ast.Frames = append(ast.Frames, frame)
		case p.At("data"):
			entity, err := emDataEntity(p)
			if err != nil {
				return nil, err
			}
			ast.DataEntities = append(ast.DataEntities, entity)
		case p.At("note"):
			note, err := emNoteEntity(p)
			if err != nil {
				return nil, err
			}
			ast.NoteEntities = append(ast.NoteEntities, note)
		case p.At("gwt"):
			gwt, err := emGwt(p)
			if err != nil {
				return nil, err
			}
			ast.GwtEntities = append(ast.GwtEntities, gwt)
		default:
			return ast, p.End()
		}
	}
}

// keyword is a data type rule that is a choice of keywords:
// EmModelEntityType and EmDataType. The value is the text of the keyword.
func keyword(p *langium.Parser, names []string) (string, error) {
	if !p.At(names...) {
		return "", p.Unexpected(names...)
	}
	return p.Next().Image, nil
}

// qualifiedName is the data type rule
//
//	QualifiedName returns string: EM_EID ('.' EM_EID)*;
//
// The value is the text of its tokens, without the hidden tokens between
// them.
func qualifiedName(p *langium.Parser) (string, error) {
	t, err := p.Expect("EM_ID")
	if err != nil {
		return "", err
	}
	name := t.Image
	for p.At(".") {
		name += p.Next().Image
		t, err := p.Expect("EM_ID")
		if err != nil {
			return "", err
		}
		name += t.Image
	}
	return name, nil
}

// emDataType is the start of the fragments EmDataInline and EmDataBlock:
//
//	('`' dataType=EmDataType '`')?
func emDataType(p *langium.Parser) (*string, error) {
	if p.Accept("`") == nil {
		return nil, nil
	}
	dataType, err := keyword(p, dataTypes)
	if err != nil {
		return nil, err
	}
	if _, err := p.Expect("`"); err != nil {
		return nil, err
	}
	return &dataType, nil
}

// emFrame is the rule EmFrame: EmTimeFrame | EmResetFrame. Both are
//
//	('tf' | 'timeframe') name=EM_FI modelEntityType=EmModelEntityType
//	entityIdentifier=QualifiedName ('->>' sourceFrames+=[EmFrame:EM_FI])*
//	('[[' dataReference=[EmDataEntity:EM_EID] ']]')? EmDataInline?;
//
// with the keywords 'rf' and 'resetframe' for a reset frame.
func emFrame(p *langium.Parser) (*EmFrame, error) {
	start := p.Next()
	frame := &EmFrame{Type: typeTimeFrame, SourceFrames: []string{}, Line: start.Line - 1}
	if start.Type.Name == "rf" || start.Type.Name == "resetframe" {
		frame.Type = typeResetFrame
	}
	name, err := p.Expect("EM_FID")
	if err != nil {
		return nil, err
	}
	frame.Name = name.Image
	if frame.ModelEntityType, err = keyword(p, modelEntityTypes); err != nil {
		return nil, err
	}
	if frame.EntityIdentifier, err = qualifiedName(p); err != nil {
		return nil, err
	}
	for p.Accept("->>") != nil {
		source, err := p.Expect("EM_FID")
		if err != nil {
			return nil, err
		}
		frame.SourceFrames = append(frame.SourceFrames, source.Image)
	}
	if p.Accept("[[") != nil {
		reference, err := p.Expect("EM_ID")
		if err != nil {
			return nil, err
		}
		frame.DataReference = &reference.Image
		if _, err := p.Expect("]]"); err != nil {
			return nil, err
		}
	}
	// fragment EmDataInline: ('`' dataType=EmDataType '`')? dataInlineValue=EM_DATA_INLINE;
	if p.At("`", "EM_DATA_INLINE") {
		if frame.DataType, err = emDataType(p); err != nil {
			return nil, err
		}
		value, err := p.Expect("EM_DATA_INLINE")
		if err != nil {
			return nil, err
		}
		frame.DataInlineValue = &value.Image
	}
	return frame, nil
}

// emModelEntity is the rule
//
//	EmModelEntity: 'entity' name=QualifiedName;
func emModelEntity(p *langium.Parser) (*EmModelEntity, error) {
	p.Next()
	name, err := qualifiedName(p)
	if err != nil {
		return nil, err
	}
	return &EmModelEntity{Name: name}, nil
}

// emDataBlock is the fragment
//
//	EmDataBlock: ('`' dataType=EmDataType '`')? dataBlockValue=EM_DATA_BLOCK;
func emDataBlock(p *langium.Parser) (dataType *string, value string, err error) {
	if dataType, err = emDataType(p); err != nil {
		return nil, "", err
	}
	block, err := p.Expect("EM_DATA_BLOCK")
	if err != nil {
		return nil, "", err
	}
	return dataType, block.Image, nil
}

// emDataEntity is the rule
//
//	EmDataEntity: 'data' name=EM_EID EmDataBlock;
func emDataEntity(p *langium.Parser) (*EmDataEntity, error) {
	p.Next()
	name, err := p.Expect("EM_ID")
	if err != nil {
		return nil, err
	}
	entity := &EmDataEntity{Name: name.Image}
	if entity.DataType, entity.DataBlockValue, err = emDataBlock(p); err != nil {
		return nil, err
	}
	return entity, nil
}

// emNoteEntity is the rule
//
//	EmNoteEntity: 'note' sourceFrame=[EmFrame:EM_FI] EmDataBlock;
func emNoteEntity(p *langium.Parser) (*EmNoteEntity, error) {
	p.Next()
	source, err := p.Expect("EM_FID")
	if err != nil {
		return nil, err
	}
	note := &EmNoteEntity{SourceFrame: source.Image}
	if note.DataType, note.DataBlockValue, err = emDataBlock(p); err != nil {
		return nil, err
	}
	return note, nil
}

// emGwt is the rule
//
//	EmGwt: 'gwt' sourceFrame=[EmFrame:EM_FI]
//	    'given' givenStatements+=EmGwtStatement+
//	    ('when' whenStatements+=EmGwtStatement+)?
//	    'then' thenStatements+=EmGwtStatement+;
func emGwt(p *langium.Parser) (*EmGwt, error) {
	p.Next()
	source, err := p.Expect("EM_FID")
	if err != nil {
		return nil, err
	}
	gwt := &EmGwt{
		SourceFrame:     source.Image,
		GivenStatements: []*EmGwtStatement{},
		WhenStatements:  []*EmGwtStatement{},
		ThenStatements:  []*EmGwtStatement{},
	}
	if _, err := p.Expect("given"); err != nil {
		return nil, err
	}
	if gwt.GivenStatements, err = emGwtStatements(p); err != nil {
		return nil, err
	}
	if p.Accept("when") != nil {
		if gwt.WhenStatements, err = emGwtStatements(p); err != nil {
			return nil, err
		}
	}
	if _, err := p.Expect("then"); err != nil {
		return nil, err
	}
	if gwt.ThenStatements, err = emGwtStatements(p); err != nil {
		return nil, err
	}
	return gwt, nil
}

// emGwtStatements is EmGwtStatement+, with the rule
//
//	EmGwtStatement: EmModelEntityType entityIdentifier=[EmModelEntity:EM_EID];
func emGwtStatements(p *langium.Parser) ([]*EmGwtStatement, error) {
	statements := []*EmGwtStatement{}
	for {
		if _, err := keyword(p, modelEntityTypes); err != nil {
			return nil, err
		}
		identifier, err := p.Expect("EM_ID")
		if err != nil {
			return nil, err
		}
		statements = append(statements, &EmGwtStatement{EntityIdentifier: identifier.Image})
		if !p.At(modelEntityTypes...) {
			return statements, nil
		}
	}
}
