// Ported from mermaid 12.2.0, diagrams/cynefin/cynefinDb.ts, cynefinParser.ts
// and types.ts (MIT).

package cynefin

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// Item is CynefinItem.
type Item struct {
	Label string
}

// Domain is CynefinDomain.
type Domain struct {
	Name  string
	Items []Item
}

// TransitionData is CynefinTransition. Label is "" when the transition has
// no label: upstream stores undefined for an empty label too.
type TransitionData struct {
	From  string
	To    string
	Label string
}

// DB is the db of the cynefin diagram.
type DB struct {
	render.CommonDB
	ctx         *render.Context
	domains     *js.Map[string, *Domain]
	transitions []TransitionData
}

func newDB(ctx *render.Context) *DB {
	db := &DB{ctx: ctx, domains: js.NewMap[string, *Domain](), transitions: []TransitionData{}}
	db.Sanitize = ctx.SanitizeText
	return db
}

// GetDomains is getDomains().
func (db *DB) GetDomains() *js.Map[string, *Domain] { return db.domains }

// GetTransitions is getTransitions().
func (db *DB) GetTransitions() []TransitionData { return db.transitions }

// SetDomains is setDomains(blocks).
func (db *DB) SetDomains(blocks []DomainBlock) {
	for _, block := range blocks {
		domainName := block.Domain
		items := make([]Item, len(block.Items))
		for i, item := range block.Items {
			items[i] = Item{Label: item.Label}
		}
		db.domains.Set(domainName, &Domain{Name: domainName, Items: items})
	}
}

// SetTransitions is setTransitions(transitions). A transition from a domain
// to itself is skipped.
func (db *DB) SetTransitions(transitions []Transition) {
	db.transitions = []TransitionData{}
	for _, t := range transitions {
		if t.From == t.To {
			continue
		}
		label := ""
		if t.Label != nil {
			label = *t.Label
		}
		db.transitions = append(db.transitions, TransitionData{From: t.From, To: t.To, Label: label})
	}
}

// spread is { ...target, ...value }: it copies the own enumerable
// properties of value into target. A string has its code units as
// properties, and an array its elements.
func spread(target *js.Obj[any], value any) {
	switch v := value.(type) {
	case *js.Obj[any]:
		v.Each(func(key string, e any) { target.Set(key, e) })
	case []any:
		for i, e := range v {
			target.Set(js.Str(float64(i)), e)
		}
	case string:
		for i, u := range js.UTF16Units(v) {
			target.Set(js.Str(float64(i)), js.FromUTF16Units([]rune{u}))
		}
	}
}

// GetConfig is getConfig(): cleanAndMerge({ ...DEFAULT_CONFIG.cynefin,
// ...getConfig().cynefin }).
func (db *DB) GetConfig() *js.Obj[any] {
	merged := js.NewObj[any]()
	spread(merged, config.DefaultSection("cynefin"))
	spread(merged, db.ctx.Config.Get("cynefin"))
	return utils.CleanAndMerge(merged, nil)
}

// parseDiagram is parser.parse of cynefinParser.ts.
func parseDiagram(_ *render.Context, rdb render.DB, text string) error {
	ast, err := parse(text)
	if err != nil {
		return err
	}
	populate(ast, rdb.(*DB))
	return nil
}

// populate is populate(ast) of cynefinParser.ts.
func populate(ast *Cynefin, db *DB) {
	ast.Populate(db)
	db.SetDomains(ast.Domains)
	db.SetTransitions(ast.Transitions)
}
