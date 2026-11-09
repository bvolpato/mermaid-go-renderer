// Ported from mermaid 12.2.0, diagrams/treeView/icons.ts (MIT).

package treeview

import (
	"errors"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
)

// treeViewIconsPrefix is treeViewIcons.prefix.
const treeViewIconsPrefix = "mermaid-treeview"

// treeViewIcons is treeViewIcons: the built-in icon pack for treeView
// nodes.
//
// It contains only the two default icons (file and folder). Any other icon
// must come from a registered iconify pack and is referenced from the
// diagram text as `icon(pack:name)`, or as `icon(name)` together with the
// `defaultIconPack` config option.
//
// Icons use `currentColor` so they can be themed via CSS `color`.
func treeViewIcons() *js.Obj[any] {
	icon := func(body string) *js.Obj[any] {
		o := js.NewObj[any]()
		o.Set("body", body)
		return o
	}
	icons := js.NewObj[any]()
	icons.Set("folder", icon(`<path fill="currentColor" d="M10.59 4.59A2 2 0 0 0 9.17 4H4a2 2 0 0 0-2 2v12a2 2 0 0 0 2 2h16a2 2 0 0 0 2-2V8a2 2 0 0 0-2-2h-7.17z"/>`))
	icons.Set("file", icon(`<path fill="currentColor" fill-rule="evenodd" d="M6 2a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8.83a2 2 0 0 0-.59-1.42l-4.82-4.82A2 2 0 0 0 13.17 2H6Zm7.5 1.9l4.6 4.6h-3.6a1 1 0 0 1-1-1V3.9Z" clip-rule="evenodd"/>`))
	pack := js.NewObj[any]()
	pack.Set("prefix", treeViewIconsPrefix)
	pack.Set("height", 24.0)
	pack.Set("width", 24.0)
	pack.Set("icons", icons)
	return pack
}

// errIconNotString is the TypeError of upstream for an icon reference that
// has no method includes: a number, a boolean, an object or a function.
var errIconNotString = errors.New("icon.includes is not a function")

// detectIcon is detectIcon(name, config): it detects a file-type icon name
// from the configured maps. There is no built-in mapping: without
// `filenameIcons`/`extensionIcons` config, files get the built-in `file`
// icon. Filename matches win over extension matches. Extensions match
// case-insensitively. The result is nil when nothing matches.
//
// Upstream reads the maps with map[name], so a name such as "constructor"
// finds a property of the prototype of the map. The result is then a
// js.Inherited.
func detectIcon(name string, config *js.Obj[any]) any {
	filenameIcon := js.GetMember(config.Get("filenameIcons"), name)
	if js.Truthy(filenameIcon) {
		return filenameIcon
	}
	dotIdx := strings.LastIndex(name, ".")
	if dotIdx > 0 {
		ext := js.ToLowerCase(name[dotIdx:])
		extensionIcons := config.Get("extensionIcons")
		if v := js.GetMember(extensionIcons, ext); !js.IsNullish(v) {
			return v
		}
		return js.GetMember(extensionIcons, ext[1:])
	}
	return nil
}

// qualifyIcon is qualifyIcon(icon, defaultIconPack): it qualifies an
// unprefixed icon reference. Built-ins win, then the defaultIconPack.
//
// icon is a string in a valid config. An array also has the method
// includes, so upstream accepts it: the result is then the array itself or
// a text.
func qualifyIcon(icon any, defaultIconPack any) (any, error) {
	switch x := icon.(type) {
	case string:
		if strings.Contains(x, ":") {
			return x, nil
		}
	case []any:
		for _, e := range x {
			if e == ":" {
				return x, nil
			}
		}
	case js.Inherited:
		// The prototype of a string is a String object with the value ""
		// and the prototype of an array is an empty array. Both have the
		// method includes and both print as "".
		if x.Name != "__proto__" || (x.Kind != "string" && x.Kind != "array") {
			return nil, errIconNotString
		}
		icon = ""
	default:
		return nil, errIconNotString
	}
	name := js.ToString(icon)
	// icon in treeViewIcons.icons
	if js.GetMember(treeViewIcons().Get("icons"), name) != nil || !js.Truthy(defaultIconPack) {
		return treeViewIconsPrefix + ":" + name, nil
	}
	return js.ToString(defaultIconPack) + ":" + name, nil
}

// getNodeIcon is getNodeIcon(node, config): it resolves the
// (fully-qualified) iconify reference to render for a node.
//
// An explicit `icon()` annotation always wins (with `none` hiding the
// icon). Otherwise, when `showIcons` is enabled, files are matched against
// the configured `filenameIcons`/`extensionIcons` maps, falling back to the
// built-in file/folder icon. The result is nil when no icon should be
// rendered. It is a string, or an array for a config that maps a file to
// an array.
func getNodeIcon(node *Node, config *js.Obj[any]) (any, error) {
	if node.Icon == "none" {
		return nil, nil
	}
	if node.Icon != "" {
		return qualifyIcon(node.Icon, config.Get("defaultIconPack"))
	}
	if !js.Truthy(config.Get("showIcons")) {
		return nil, nil
	}
	if node.NodeType == fileNode {
		detected := detectIcon(node.Name, config)
		if detected == "none" {
			return nil, nil
		}
		if js.Truthy(detected) {
			return qualifyIcon(detected, config.Get("defaultIconPack"))
		}
	}
	if node.NodeType == directoryNode {
		return treeViewIconsPrefix + ":folder", nil
	}
	return treeViewIconsPrefix + ":file", nil
}
