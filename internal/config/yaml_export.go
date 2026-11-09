// This file ports no upstream file. It exports the YAML loader of yaml.go
// for the diagram dbs.

package config

// LoadYAML is yaml.load(text, { schema: yaml.JSON_SCHEMA }) of js-yaml. The
// diagram dbs read the metadata of a node or a participant with it (for
// example flowDb.ts addVertex). The result is a dynamic JavaScript value:
// nil (undefined) for a text with no document. The error is an *Error with
// the name YAMLException.
func LoadYAML(text string) (any, error) {
	return loadYAML(text)
}
