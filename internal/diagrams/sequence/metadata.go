// Ported from mermaid 12.2.0, diagrams/sequence/sequenceDb.ts (addActor): the
// participant metadata (MIT). The text between "@{" and "}" is YAML, and
// upstream reads it with yaml.load(text, { schema: yaml.JSON_SCHEMA }).

package sequence

import (
	"errors"
	"io"
	"math"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// Local copy of yaml.load with the JSON schema of js-yaml, reduced to what
// addActor reads; replace with the loader of internal/config (loadYAML) when
// that package exports it.
//
// loadParticipantMetaData returns the type and the alias of the metadata, as
// dynamic JavaScript values. The result is nil when the document is not a
// mapping: doc?.type and doc?.alias are then undefined.
func loadParticipantMetaData(text string) (*participantMetaData, error) {
	dec := yaml.NewDecoder(strings.NewReader(text))
	var doc yaml.Node
	if err := dec.Decode(&doc); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil
		}
		return nil, errors.New("YAMLException: " + err.Error())
	}
	var next yaml.Node
	if err := dec.Decode(&next); err == nil {
		return nil, errors.New("YAMLException: expected a single document in the stream, but found more")
	} else if !errors.Is(err, io.EOF) {
		return nil, errors.New("YAMLException: " + err.Error())
	}
	root := &doc
	if doc.Kind == yaml.DocumentNode {
		if len(doc.Content) == 0 {
			return nil, nil
		}
		root = doc.Content[0]
	}
	for root.Kind == yaml.AliasNode && root.Alias != nil {
		root = root.Alias
	}
	if root.Kind != yaml.MappingNode {
		return nil, nil
	}
	meta := &participantMetaData{}
	seen := map[string]bool{}
	for i := 0; i+1 < len(root.Content); i += 2 {
		key := js.ToString(yamlValue(root.Content[i]))
		if seen[key] {
			return nil, errors.New("YAMLException: duplicated mapping key")
		}
		seen[key] = true
		switch key {
		case "type":
			meta.typ = yamlValue(root.Content[i+1])
		case "alias":
			meta.alias = yamlValue(root.Content[i+1])
		}
	}
	return meta, nil
}

var (
	reYAMLInt   = regexp.MustCompile(`^[-+]?(?:0b[01_]+|0o[0-7_]+|0x[0-9a-fA-F_]+|[0-9][0-9_]*)$`)
	reYAMLFloat = regexp.MustCompile(`^(?:[-+]?(?:[0-9][0-9_]*)(?:\.[0-9_]*)?(?:[eE][-+]?[0-9]+)?|\.[0-9_]+(?:[eE][-+]?[0-9]+)?|[-+]?\.(?:inf|Inf|INF)|\.(?:nan|NaN|NAN))$`)
)

// yamlValue converts a node to a dynamic JavaScript value with the JSON
// schema of js-yaml. A sequence is an array and a mapping is an object.
func yamlValue(n *yaml.Node) any {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}
	switch n.Kind {
	case yaml.SequenceNode:
		arr := make([]any, 0, len(n.Content))
		for _, c := range n.Content {
			arr = append(arr, yamlValue(c))
		}
		return arr
	case yaml.MappingNode:
		obj := js.NewObj[any]()
		for i := 0; i+1 < len(n.Content); i += 2 {
			obj.Set(js.ToString(yamlValue(n.Content[i])), yamlValue(n.Content[i+1]))
		}
		return obj
	case yaml.ScalarNode:
		v := n.Value
		// Only a plain scalar is resolved. A quoted scalar and a block
		// scalar are strings.
		if n.Style != 0 {
			return v
		}
		switch v {
		case "", "~", "null", "Null", "NULL":
			return js.Null
		case "true", "True", "TRUE":
			return true
		case "false", "False", "FALSE":
			return false
		}
		if reYAMLInt.MatchString(v) && !strings.HasSuffix(v, "_") {
			return yamlInt(v)
		}
		if reYAMLFloat.MatchString(v) && !strings.HasSuffix(v, "_") {
			return yamlFloat(v)
		}
		return v
	}
	return nil
}

// yamlInt is constructYamlInteger of js-yaml.
func yamlInt(v string) any {
	value := strings.ReplaceAll(v, "_", "")
	sign := 1.0
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		if value[0] == '-' {
			sign = -1
		}
		value = value[1:]
	}
	if value == "0" {
		return 0.0
	}
	base := 10
	switch {
	case strings.HasPrefix(value, "0b"):
		base, value = 2, value[2:]
	case strings.HasPrefix(value, "0x"):
		base, value = 16, value[2:]
	case strings.HasPrefix(value, "0o"):
		base, value = 8, value[2:]
	}
	return float64(sign * js.ParseInt(value, base))
}

// yamlFloat is constructYamlFloat of js-yaml.
func yamlFloat(v string) any {
	value := strings.ToLower(strings.ReplaceAll(v, "_", ""))
	sign := 1.0
	if strings.HasPrefix(value, "-") {
		sign = -1
	}
	if strings.HasPrefix(value, "-") || strings.HasPrefix(value, "+") {
		value = value[1:]
	}
	switch value {
	case ".inf":
		return math.Inf(int(sign))
	case ".nan":
		return math.NaN()
	}
	f, err := strconv.ParseFloat(value, 64)
	if err != nil {
		return math.NaN()
	}
	return float64(sign * f)
}
