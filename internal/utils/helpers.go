// Ported from mermaid 12.2.0, the small modules of mermaid/src/utils:
// base64.ts, diagramRoot.ts, imperativeState.ts and subGraphTitleMargins.ts
// (MIT). It also has ensureNodeFromSelector of tests/util.ts and a forward to
// getUserDefinedConfig of config.ts.

package utils

import (
	"encoding/base64"
	"errors"
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/dom"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

// ToBase64 is toBase64(str): the base64 text of the UTF-8 bytes of str. A
// lone surrogate becomes U+FFFD first, as TextEncoder does.
func ToBase64(str string) string {
	return base64.StdEncoding.EncodeToString([]byte(js.ToWellFormed(str)))
}

// GetDiagramRoot is getDiagramRoot(id, securityLevel).root: the selection of
// the body that diagram elements are queried from. The sandbox security
// level renders into an iframe, which this port does not have.
func GetDiagramRoot(ctx *render.Context, securityLevel string) (*dom.Node, error) {
	if securityLevel == "sandbox" {
		return nil, fmt.Errorf("Sandbox iframe #i%s is missing its content document", ctx.ID)
	}
	return ctx.Body, nil
}

// ImperativeState is resettable state storage (the class ImperativeState).
type ImperativeState[S any] struct {
	// Records is the current state.
	Records S
	init    func() S
}

// NewImperativeState is new ImperativeState(init). init creates the default
// state.
func NewImperativeState[S any](init func() S) *ImperativeState[S] {
	return &ImperativeState[S]{Records: init(), init: init}
}

// Reset sets the state to a new default state.
func (s *ImperativeState[S]) Reset() {
	s.Records = s.init()
}

// SubGraphTitleMargins is the result of getSubGraphTitleMargins.
type SubGraphTitleMargins struct {
	SubGraphTitleTopMargin    float64
	SubGraphTitleBottomMargin float64
	SubGraphTitleTotalMargin  float64
}

// GetSubGraphTitleMargins is getSubGraphTitleMargins({ flowchart }). config
// is an object with the property flowchart, for example the whole config.
// The margins are numbers in a valid config. A value of another kind is
// converted with Number(), where upstream keeps the value as it is.
func GetSubGraphTitleMargins(config *js.Obj[any]) SubGraphTitleMargins {
	margin := func(key string) float64 {
		// flowchart?.subGraphTitleMargin?.top ?? 0
		v := js.Path(config, "flowchart", "subGraphTitleMargin", key)
		if js.IsNullish(v) {
			return 0
		}
		return js.ToNumber(v)
	}
	subGraphTitleTopMargin := margin("top")
	subGraphTitleBottomMargin := margin("bottom")
	subGraphTitleTotalMargin := subGraphTitleTopMargin + subGraphTitleBottomMargin

	return SubGraphTitleMargins{
		SubGraphTitleTopMargin:    subGraphTitleTopMargin,
		SubGraphTitleBottomMargin: subGraphTitleBottomMargin,
		SubGraphTitleTotalMargin:  subGraphTitleTotalMargin,
	}
}

// EnsureNodeFromSelector retrieves the node from its parent with
// querySelector, and checks that it exists (tests/util.ts).
func EnsureNodeFromSelector(selector string, parent *dom.Node) (*dom.Node, error) {
	node := parent.Select(selector)
	if node == nil {
		return nil, errors.New("no node matches the selector " + selector)
	}
	return node, nil
}

// GetUserDefinedConfig is getUserDefinedConfig() of config.ts: the config of
// initialize merged with the directives, without the defaults. It forwards
// to the config state of the render.
func GetUserDefinedConfig(ctx *render.Context) *js.Obj[any] {
	return ctx.ConfigState.GetUserDefinedConfig()
}
