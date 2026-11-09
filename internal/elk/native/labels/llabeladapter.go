// Ported from the classes AbstractLShapeAdapter and LLabelAdapter of
// plugins/org.eclipse.elk.alg.layered/src/org/eclipse/elk/alg/layered/graph/LGraphAdapters.java
// of Eclipse ELK 0.9.1 (EPL-2.0).

package labels

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/adapters"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/core"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/emath"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/lgraph"
	"github.com/bvolpato/mermaid-go-renderer/internal/elk/native/prop"
)

// labelSide is LabelSide.LABEL_SIDE, the property that
// LLabelAdapter.getSide() reads. It is not InternalProperties.LABEL_SIDE.
var labelSide = prop.NewDefault("org.eclipse.elk.labelSide", core.LabelSideUnknown)

// LLabelAdapter is LGraphAdapters.LLabelAdapter: the adapter of a label of
// the LGraph. The label cells of the end labels hold such adapters.
//
// TODO(group E): the adapters of the LGraph belong to the port of
// LGraphAdapters. Use that port when it has the label adapter.
type LLabelAdapter struct {
	element *lgraph.LLabel
}

var _ adapters.LabelAdapter = (*LLabelAdapter)(nil)

// AdaptLabel is LGraphAdapters.adapt(LLabel).
func AdaptLabel(label *lgraph.LLabel) *LLabelAdapter { return &LLabelAdapter{element: label} }

// Element returns the adapted label.
func (a *LLabelAdapter) Element() *lgraph.LLabel { return a.element }

// Size is getSize().
func (a *LLabelAdapter) Size() *emath.KVector { return &a.element.Size }

// SetSize is setSize(size).
func (a *LLabelAdapter) SetSize(size *emath.KVector) {
	a.element.Size.X = size.X
	a.element.Size.Y = size.Y
}

// Position is getPosition().
func (a *LLabelAdapter) Position() *emath.KVector { return &a.element.Pos }

// SetPosition is setPosition(pos).
func (a *LLabelAdapter) SetPosition(pos *emath.KVector) {
	a.element.Pos.X = pos.X
	a.element.Pos.Y = pos.Y
}

// Holder returns the label: getProperty and hasProperty read its properties.
func (a *LLabelAdapter) Holder() prop.Holder { return a.element }

// VolatileID is getVolatileId().
func (a *LLabelAdapter) VolatileID() int { return a.element.ID }

// SetVolatileID is setVolatileId(volatileId).
func (a *LLabelAdapter) SetVolatileID(volatileID int) { a.element.ID = volatileID }

// Side is getSide().
func (a *LLabelAdapter) Side() core.LabelSide { return prop.Get(a.element, labelSide) }

// Text is getText().
func (a *LLabelAdapter) Text() (text string, ok bool) { return a.element.Text, a.element.HasText }
