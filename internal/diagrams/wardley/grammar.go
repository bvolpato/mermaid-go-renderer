// Ported from mermaid 12.2.0,
// packages/parser/src/language/wardley/wardley.langium and valueConverter.ts
// (MIT).

package wardley

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/langium"
)

// Wardley is the AST of the entry rule Wardley. A nil pointer is a property
// that the text does not set: undefined in the AST.
type Wardley struct {
	langium.Common
	Size           *Size
	Evolution      *Evolution
	Anchors        []Anchor
	Components     []Component
	Links          []Link
	Evolves        []Evolve
	Pipelines      []Pipeline
	Notes          []Note
	Annotations    []Annotations
	Annotation     []Annotation
	Accelerators   []Accelerator
	Deaccelerators []Deaccelerator
}

// Size is the AST of the rule Size.
type Size struct {
	Width  float64
	Height float64
}

// Evolution is the AST of the rule Evolution.
type Evolution struct {
	Stages []EvolutionStage
}

// EvolutionStage is the AST of the rule EvolutionStage.
type EvolutionStage struct {
	Name       string
	Boundary   *float64
	SecondName *string
}

// Anchor is the AST of the rule Anchor.
type Anchor struct {
	Name       string
	Visibility float64
	Evolution  float64
}

// Component is the AST of the rule Component.
type Component struct {
	Name       string
	Visibility float64
	Evolution  float64
	Label      *Label
	Decorator  *Decorator
	Inertia    bool
}

// Label is the AST of the rule Label.
type Label struct {
	NegX    bool
	OffsetX float64
	NegY    bool
	OffsetY float64
}

// Decorator is the AST of the rule Decorator.
type Decorator struct {
	Strategy string
}

// Link is the AST of the rule Link.
type Link struct {
	From      string
	FromPort  *string
	Arrow     *string
	To        string
	ToPort    *string
	LinkLabel *string
}

// Evolve is the AST of the rule Evolve.
type Evolve struct {
	Component string
	Target    float64
}

// Pipeline is the AST of the rule Pipeline.
type Pipeline struct {
	Parent     string
	Components []PipelineComponent
}

// PipelineComponent is the AST of the rule PipelineComponent.
type PipelineComponent struct {
	Name      string
	Evolution float64
	Label     *Label
}

// Note is the AST of the rule Note.
type Note struct {
	Text       string
	Visibility float64
	Evolution  float64
}

// Annotations is the AST of the rule Annotations.
type Annotations struct {
	X float64
	Y float64
}

// Annotation is the AST of the rule Annotation.
type Annotation struct {
	Number float64
	X      float64
	Y      float64
	Text   string
}

// Accelerator is the AST of the rules Accelerator and Deaccelerator.
type Accelerator struct {
	Name string
	X    float64
	Y    float64
}

// Deaccelerator is the AST of the rule Deaccelerator.
type Deaccelerator = Accelerator

var lexer = langium.MustLexer("wardley")

// parse is parse('wardley', text) of the parser package. The grammar is:
//
//	entry Wardley:
//	  NEWLINE*
//	  KW_WARDLEY
//	  (NEWLINE | TitleAndAccessibilities | Statement)*
//	;
//	fragment Statement:
//	  size=Size | evolution=Evolution | anchors+=Anchor | components+=Component | links+=Link
//	  | evolves+=Evolve | pipelines+=Pipeline | notes+=Note | annotations+=Annotations
//	  | annotation+=Annotation | accelerators+=Accelerator | deaccelerators+=Deaccelerator
//	;
//
// The first token decides the statement.
func parse(text string) (*Wardley, error) {
	p, err := lexer.Parse(text)
	if err != nil {
		return nil, err
	}
	ast := &Wardley{
		Anchors:        []Anchor{},
		Components:     []Component{},
		Links:          []Link{},
		Evolves:        []Evolve{},
		Pipelines:      []Pipeline{},
		Notes:          []Note{},
		Annotations:    []Annotations{},
		Annotation:     []Annotation{},
		Accelerators:   []Accelerator{},
		Deaccelerators: []Deaccelerator{},
	}
	p.Skip("NEWLINE")
	if _, err := p.Expect("KW_WARDLEY"); err != nil {
		return nil, err
	}
	for {
		var err error
		switch p.LA(1).Type.Name {
		case "NEWLINE":
			p.Next()
		case "ACC_DESCR", "ACC_TITLE", "TITLE":
			err = p.TitleAndAccessibilities(&ast.Common)
		case "KW_SIZE":
			var size Size
			size, err = sizeRule(p)
			ast.Size = &size
		case "KW_EVOLUTION":
			var evolution Evolution
			evolution, err = evolutionRule(p)
			ast.Evolution = &evolution
		case "KW_ANCHOR":
			var anchor Anchor
			anchor, err = anchorRule(p)
			ast.Anchors = append(ast.Anchors, anchor)
		case "KW_COMPONENT":
			var component Component
			component, err = componentRule(p)
			ast.Components = append(ast.Components, component)
		case "STRING", "ID", "NAME_WITH_SPACES":
			var link Link
			link, err = linkRule(p)
			ast.Links = append(ast.Links, link)
		case "KW_EVOLVE":
			var evolve Evolve
			evolve, err = evolveRule(p)
			ast.Evolves = append(ast.Evolves, evolve)
		case "KW_PIPELINE":
			var pipeline Pipeline
			pipeline, err = pipelineRule(p)
			ast.Pipelines = append(ast.Pipelines, pipeline)
		case "KW_NOTE":
			var note Note
			note, err = noteRule(p)
			ast.Notes = append(ast.Notes, note)
		case "KW_ANNOTATIONS":
			var annotations Annotations
			annotations, err = annotationsRule(p)
			ast.Annotations = append(ast.Annotations, annotations)
		case "KW_ANNOTATION":
			var annotation Annotation
			annotation, err = annotationRule(p)
			ast.Annotation = append(ast.Annotation, annotation)
		case "KW_ACCELERATOR":
			var accelerator Accelerator
			accelerator, err = acceleratorRule(p)
			ast.Accelerators = append(ast.Accelerators, accelerator)
		case "KW_DEACCELERATOR":
			var deaccelerator Deaccelerator
			deaccelerator, err = acceleratorRule(p)
			ast.Deaccelerators = append(ast.Deaccelerators, deaccelerator)
		default:
			if err := p.End(); err != nil {
				return nil, err
			}
			return ast, nil
		}
		if err != nil {
			return nil, err
		}
	}
}

// name is the alternatives (STRING | ID | NAME_WITH_SPACES) of a name
// property, with the value converter of each terminal.
func name(p *langium.Parser) (string, error) {
	switch {
	case p.At("STRING"):
		return langium.ConvertString(p.Next().Image), nil
	case p.At("ID"):
		return langium.ConvertID(p.Next().Image), nil
	case p.At("NAME_WITH_SPACES"):
		return p.Next().Image, nil
	}
	return "", p.Unexpected("STRING", "ID", "NAME_WITH_SPACES")
}

// wardleyNumber is a property with the terminal WARDLEY_NUMBER.
func wardleyNumber(p *langium.Parser) (float64, error) {
	t, err := p.Expect("WARDLEY_NUMBER")
	if err != nil {
		return 0, err
	}
	return langium.ConvertNumber(t.Image), nil
}

// integer is a property with the terminal INT.
func integer(p *langium.Parser) (float64, error) {
	t, err := p.Expect("INT")
	if err != nil {
		return 0, err
	}
	return langium.ConvertInt(t.Image), nil
}

// coordinateValue is the rule CoordinateValue: WARDLEY_NUMBER | INT. It
// returns number, so the value is Number(text).
func coordinateValue(p *langium.Parser) (float64, error) {
	if !p.At("WARDLEY_NUMBER", "INT") {
		return 0, p.Unexpected("WARDLEY_NUMBER", "INT")
	}
	return langium.ConvertNumber(p.Next().Image), nil
}

// expectAll consumes the tokens of the types in order.
func expectAll(p *langium.Parser, names ...string) error {
	for _, n := range names {
		if _, err := p.Expect(n); err != nil {
			return err
		}
	}
	return nil
}

// pair is '[' first ',' second ']' for two properties of the same kind.
func pair(p *langium.Parser, value func(*langium.Parser) (float64, error)) (first, second float64, err error) {
	if _, err = p.Expect("["); err != nil {
		return
	}
	if first, err = value(p); err != nil {
		return
	}
	if _, err = p.Expect(","); err != nil {
		return
	}
	if second, err = value(p); err != nil {
		return
	}
	_, err = p.Expect("]")
	return
}

// sizeRule is the rule
//
//	Size: KW_SIZE '[' width=INT ',' height=INT ']' EOL;
func sizeRule(p *langium.Parser) (size Size, err error) {
	p.Next()
	if size.Width, size.Height, err = pair(p, integer); err != nil {
		return
	}
	return size, p.EOL()
}

// evolutionRule is the rule
//
//	Evolution: KW_EVOLUTION stages+=EvolutionStage (ARROW stages+=EvolutionStage)+ EOL;
func evolutionRule(p *langium.Parser) (Evolution, error) {
	p.Next()
	evolution := Evolution{}
	stage, err := evolutionStage(p)
	if err != nil {
		return evolution, err
	}
	evolution.Stages = append(evolution.Stages, stage)
	if !p.At("ARROW") {
		return evolution, p.Unexpected("ARROW")
	}
	for p.Accept("ARROW") != nil {
		stage, err := evolutionStage(p)
		if err != nil {
			return evolution, err
		}
		evolution.Stages = append(evolution.Stages, stage)
	}
	return evolution, p.EOL()
}

// evolutionStage is the rule
//
//	EvolutionStage:
//	  name=(ID | STRING | NAME_WITH_SPACES) ('@' boundary=WARDLEY_NUMBER)?
//	  ('/' secondName=(ID | STRING | NAME_WITH_SPACES))?
//	;
func evolutionStage(p *langium.Parser) (EvolutionStage, error) {
	stage := EvolutionStage{}
	var err error
	if stage.Name, err = name(p); err != nil {
		return stage, err
	}
	if p.Accept("@") != nil {
		boundary, err := wardleyNumber(p)
		if err != nil {
			return stage, err
		}
		stage.Boundary = &boundary
	}
	if p.Accept("/") != nil {
		secondName, err := name(p)
		if err != nil {
			return stage, err
		}
		stage.SecondName = &secondName
	}
	return stage, nil
}

// anchorRule is the rule
//
//	Anchor:
//	  KW_ANCHOR name=(STRING | ID | NAME_WITH_SPACES)
//	  '[' visibility=WARDLEY_NUMBER ',' evolution=WARDLEY_NUMBER ']' EOL
//	;
func anchorRule(p *langium.Parser) (anchor Anchor, err error) {
	p.Next()
	if anchor.Name, err = name(p); err != nil {
		return
	}
	if anchor.Visibility, anchor.Evolution, err = pair(p, wardleyNumber); err != nil {
		return
	}
	return anchor, p.EOL()
}

// componentRule is the rule
//
//	Component:
//	  KW_COMPONENT name=(STRING | ID | NAME_WITH_SPACES)
//	  '[' visibility=WARDLEY_NUMBER ',' evolution=WARDLEY_NUMBER ']'
//	  label=Label? decorator=Decorator?
//	  (inertia?=KW_INERTIA | '(' inertia?=KW_INERTIA ')')?
//	  EOL
//	;
//	Decorator: '(' strategy=STRATEGY ')';
func componentRule(p *langium.Parser) (component Component, err error) {
	p.Next()
	if component.Name, err = name(p); err != nil {
		return
	}
	if component.Visibility, component.Evolution, err = pair(p, wardleyNumber); err != nil {
		return
	}
	if p.At("KW_LABEL") {
		var label Label
		if label, err = labelRule(p); err != nil {
			return
		}
		component.Label = &label
	}
	// A "(" starts a decorator or the inertia in parentheses. The token
	// after it decides.
	if p.At("(") && p.LA(2).Type.Name == "STRATEGY" {
		p.Next()
		component.Decorator = &Decorator{Strategy: p.Next().Image}
		if _, err = p.Expect(")"); err != nil {
			return
		}
	}
	switch {
	case p.At("KW_INERTIA"):
		p.Next()
		component.Inertia = true
	case p.At("("):
		p.Next()
		if err = expectAll(p, "KW_INERTIA", ")"); err != nil {
			return
		}
		component.Inertia = true
	}
	return component, p.EOL()
}

// labelRule is the rule
//
//	Label: KW_LABEL '[' negX?='-'? offsetX=INT ',' negY?='-'? offsetY=INT ']';
func labelRule(p *langium.Parser) (label Label, err error) {
	p.Next()
	if _, err = p.Expect("["); err != nil {
		return
	}
	label.NegX = p.Accept("-") != nil
	if label.OffsetX, err = integer(p); err != nil {
		return
	}
	if _, err = p.Expect(","); err != nil {
		return
	}
	label.NegY = p.Accept("-") != nil
	if label.OffsetY, err = integer(p); err != nil {
		return
	}
	_, err = p.Expect("]")
	return
}

// linkRule is the rule
//
//	Link:
//	  from=(STRING | ID | NAME_WITH_SPACES) fromPort=LINK_PORT?
//	  arrow=(LINK_ARROW | ARROW)? to=(STRING | ID | NAME_WITH_SPACES) toPort=LINK_PORT?
//	  linkLabel=LINK_LABEL? EOL
//	;
func linkRule(p *langium.Parser) (link Link, err error) {
	if link.From, err = name(p); err != nil {
		return
	}
	if t := p.Accept("LINK_PORT"); t != nil {
		link.FromPort = &t.Image
	}
	if p.At("LINK_ARROW", "ARROW") {
		arrow := p.Next().Image
		link.Arrow = &arrow
	}
	if link.To, err = name(p); err != nil {
		return
	}
	if t := p.Accept("LINK_PORT"); t != nil {
		link.ToPort = &t.Image
	}
	if t := p.Accept("LINK_LABEL"); t != nil {
		// Strip the leading ';' and trim whitespace
		label := js.TrimSpace(t.Image[1:])
		link.LinkLabel = &label
	}
	return link, p.EOL()
}

// evolveRule is the rule
//
//	Evolve: KW_EVOLVE component=(STRING | ID | NAME_WITH_SPACES) target=WARDLEY_NUMBER EOL;
func evolveRule(p *langium.Parser) (evolve Evolve, err error) {
	p.Next()
	if evolve.Component, err = name(p); err != nil {
		return
	}
	if evolve.Target, err = wardleyNumber(p); err != nil {
		return
	}
	return evolve, p.EOL()
}

// pipelineRule is the rule
//
//	Pipeline:
//	  KW_PIPELINE parent=(STRING | ID | NAME_WITH_SPACES) '{' NEWLINE+
//	  components+=PipelineComponent+
//	  '}' EOL
//	;
//	PipelineComponent:
//	  KW_COMPONENT name=(STRING | ID | NAME_WITH_SPACES) '[' evolution=WARDLEY_NUMBER ']'
//	  label=Label? EOL
//	;
func pipelineRule(p *langium.Parser) (pipeline Pipeline, err error) {
	p.Next()
	if pipeline.Parent, err = name(p); err != nil {
		return
	}
	if _, err = p.Expect("{"); err != nil {
		return
	}
	if p.Skip("NEWLINE") == 0 {
		return pipeline, p.Unexpected("NEWLINE")
	}
	if !p.At("KW_COMPONENT") {
		return pipeline, p.Unexpected("KW_COMPONENT")
	}
	for p.Accept("KW_COMPONENT") != nil {
		component := PipelineComponent{}
		if component.Name, err = name(p); err != nil {
			return
		}
		if _, err = p.Expect("["); err != nil {
			return
		}
		if component.Evolution, err = wardleyNumber(p); err != nil {
			return
		}
		if _, err = p.Expect("]"); err != nil {
			return
		}
		if p.At("KW_LABEL") {
			var label Label
			if label, err = labelRule(p); err != nil {
				return
			}
			component.Label = &label
		}
		if err = p.EOL(); err != nil {
			return
		}
		pipeline.Components = append(pipeline.Components, component)
	}
	if _, err = p.Expect("}"); err != nil {
		return
	}
	return pipeline, p.EOL()
}

// noteRule is the rule
//
//	Note: KW_NOTE text=STRING '[' visibility=WARDLEY_NUMBER ',' evolution=WARDLEY_NUMBER ']' EOL;
func noteRule(p *langium.Parser) (note Note, err error) {
	p.Next()
	text, err := p.Expect("STRING")
	if err != nil {
		return note, err
	}
	note.Text = langium.ConvertString(text.Image)
	if note.Visibility, note.Evolution, err = pair(p, wardleyNumber); err != nil {
		return
	}
	return note, p.EOL()
}

// annotationsRule is the rule
//
//	Annotations: KW_ANNOTATIONS '[' x=CoordinateValue ',' y=CoordinateValue ']' EOL;
func annotationsRule(p *langium.Parser) (annotations Annotations, err error) {
	p.Next()
	if annotations.X, annotations.Y, err = pair(p, coordinateValue); err != nil {
		return
	}
	return annotations, p.EOL()
}

// annotationRule is the rule
//
//	Annotation:
//	  KW_ANNOTATION number=INT ',' '[' x=CoordinateValue ',' y=CoordinateValue ']' text=STRING EOL
//	;
func annotationRule(p *langium.Parser) (annotation Annotation, err error) {
	p.Next()
	if annotation.Number, err = integer(p); err != nil {
		return
	}
	if _, err = p.Expect(","); err != nil {
		return
	}
	if annotation.X, annotation.Y, err = pair(p, coordinateValue); err != nil {
		return
	}
	text, err := p.Expect("STRING")
	if err != nil {
		return annotation, err
	}
	annotation.Text = langium.ConvertString(text.Image)
	return annotation, p.EOL()
}

// acceleratorRule is the rules
//
//	Accelerator:
//	  KW_ACCELERATOR name=(STRING | ID | NAME_WITH_SPACES) '[' x=WARDLEY_NUMBER ',' y=WARDLEY_NUMBER ']' EOL
//	;
//	Deaccelerator:
//	  KW_DEACCELERATOR name=(STRING | ID | NAME_WITH_SPACES) '[' x=WARDLEY_NUMBER ',' y=WARDLEY_NUMBER ']' EOL
//	;
func acceleratorRule(p *langium.Parser) (accelerator Accelerator, err error) {
	p.Next()
	if accelerator.Name, err = name(p); err != nil {
		return
	}
	if accelerator.X, accelerator.Y, err = pair(p, wardleyNumber); err != nil {
		return
	}
	return accelerator, p.EOL()
}
