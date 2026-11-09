// Ported from mermaid 12.2.0, diagrams/usecase/usecaseTypes.ts (MIT).
//
// An optional string property of the JavaScript objects is "" here where
// upstream sets the property only for a non-empty value.

package usecase

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

// Actor is Actor.
type Actor struct {
	ID        string
	Label     string
	LabelType string
	// Type is the actor type: normal, hollow, awesome or icon.
	Type       string
	Icon       string
	Business   bool
	Stereotype string
	ParentID   string
	Classes    []string
	Styles     []string
}

// UseCase is UseCase.
type UseCase struct {
	ID        string
	Label     string
	LabelType string
	// Shape is ellipse or rect.
	Shape      string
	Business   bool
	Stereotype string
	ParentID   string
	Classes    []string
	Styles     []string
}

// SystemBoundary is SystemBoundary.
type SystemBoundary struct {
	ID        string
	Label     string
	LabelType string
	// Type is rect or package.
	Type    string
	Members []string
	Classes []string
	Styles  []string
}

// ARROW_TYPE of usecaseTypes.ts.
const (
	arrowSolidArrow          = 0
	arrowBackArrow           = 1
	arrowLineSolid           = 2
	arrowCircleArrow         = 3
	arrowCrossArrow          = 4
	arrowCircleArrowReversed = 5
	arrowCrossArrowReversed  = 6
)

// Relationship is Relationship.
type Relationship struct {
	ID         string
	ExplicitID bool
	Source     string
	Target     string
	// Type is association, include, extend or generalization.
	Type      string
	ArrowType int
	// Label and LabelType are set together. HasLabel is false when the
	// relationship has no label property.
	Label     string
	LabelType string
	HasLabel  bool
	Minlen    float64
	Classes   []string
	Styles    []string
	Animate   bool
	// Animation is fast or slow, or "".
	Animation string
}

// UsecaseNote is UsecaseNote.
type UsecaseNote struct {
	ID        string
	Target    string
	Label     string
	LabelType string
}

// UsecaseJsonNode is UsecaseJsonNode.
type UsecaseJsonNode struct {
	ID string
	// Value is the parsed JSON object.
	Value *js.Obj[any]
	// PropertyOrder has the property names of each object of the value in
	// source order, by JSON pointer.
	PropertyOrder *js.Obj[[]string]
	Classes       []string
	Styles        []string
}

// defaultDirection is DEFAULT_DIRECTION.
const defaultDirection = "LR"

// ClassDef is ClassDef.
type ClassDef struct {
	ID     string
	Styles []string
}

// UsecaseJsonRow is UsecaseJsonRow.
type UsecaseJsonRow struct {
	// Key is the visual key cell. Scalar-array rows after the first use an empty string.
	Key string
	// AccessibleKey is the key repeated for assistive technology even when the visual key is blank.
	AccessibleKey string
	Value         string
}

// fields is UsecaseFields: the model that the parser commits to the db. The
// property ast of upstream is not ported: see visitor.go.
type fields struct {
	actors              *js.Map[string, *Actor]
	useCases            *js.Map[string, *UseCase]
	systemBoundaries    *js.Map[string, *SystemBoundary]
	relationships       []*Relationship
	notes               *js.Map[string, *UsecaseNote]
	jsonNodes           *js.Map[string, *UsecaseJsonNode]
	classDefs           *js.Map[string, *ClassDef]
	symbols             *js.Map[string, string]
	direction           string
	relationshipCounter int
	noteCounter         int
	accTitle            string
	accDescription      string
	// config is structuredClone(DEFAULT_USECASE_CONFIG).
	config *js.Obj[any]
}
