// Ported from mermaid 12.2.0, diagrams/requirement/types.ts (MIT).

package requirement

// The values of RequirementDB.RequirementType.
const (
	TypeRequirement            = "Requirement"
	TypeFunctionalRequirement  = "Functional Requirement"
	TypeInterfaceRequirement   = "Interface Requirement"
	TypePerformanceRequirement = "Performance Requirement"
	TypePhysicalRequirement    = "Physical Requirement"
	TypeDesignConstraint       = "Design Constraint"
)

// The values of RequirementDB.RiskLevel.
const (
	RiskLow    = "Low"
	RiskMedium = "Medium"
	RiskHigh   = "High"
)

// The values of RequirementDB.VerifyType.
const (
	VerifyAnalysis      = "Analysis"
	VerifyDemonstration = "Demonstration"
	VerifyInspection    = "Inspection"
	VerifyTest          = "Test"
)

// The values of RequirementDB.Relationships.
const (
	RelContains  = "contains"
	RelCopies    = "copies"
	RelDerives   = "derives"
	RelSatisfies = "satisfies"
	RelVerifies  = "verifies"
	RelRefines   = "refines"
	RelTraces    = "traces"
)

// Requirement is the Requirement interface.
type Requirement struct {
	Name          string
	Type          string
	RequirementID string
	Text          string
	Risk          string
	VerifyMethod  string
	CSSStyles     []string
	Classes       []string
}

// Relation is the Relation interface.
type Relation struct {
	Type string
	Src  string
	Dst  string
}

// Element is the Element interface.
type Element struct {
	Name      string
	Type      string
	DocRef    string
	CSSStyles []string
	Classes   []string
}

// RequirementClass is the RequirementClass interface: a classDef.
type RequirementClass struct {
	ID         string
	Styles     []string
	TextStyles []string
}
