// Ported from mermaid 12.2.0, diagrams/sequence/sequenceDb.ts (addActor): the
// participant metadata (MIT). The text between "@{" and "}" is YAML, and
// upstream reads it with yaml.load(text, { schema: yaml.JSON_SCHEMA }).

package sequence

import (
	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// loadParticipantMetaData returns the type and the alias of the metadata, as
// dynamic JavaScript values. The result is nil when the document is not a
// mapping: doc?.type and doc?.alias are then undefined.
func loadParticipantMetaData(text string) (*participantMetaData, error) {
	v, err := config.LoadYAML(text)
	if err != nil {
		return nil, err
	}
	obj, ok := v.(*js.Obj[any])
	if !ok || obj == nil {
		return nil, nil
	}
	meta := &participantMetaData{}
	if typ, exists := obj.Lookup("type"); exists {
		meta.typ = typ
	}
	if alias, exists := obj.Lookup("alias"); exists {
		meta.alias = alias
	}
	return meta, nil
}
