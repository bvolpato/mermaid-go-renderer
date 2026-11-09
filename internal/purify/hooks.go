// Ported from dompurify 3.4.12, src/purify.ts: the hooks API (addHook,
// removeHook, removeHooks, removeAllHooks, _executeHooks) (MPL-2.0 OR
// Apache-2.0).

package purify

import "github.com/bvolpato/mermaid-go-renderer/internal/dom"

// HookPoint is the name of an entry point for hooks.
type HookPoint string

// The entry points of DOMPurify.
const (
	AfterSanitizeAttributes  HookPoint = "afterSanitizeAttributes"
	AfterSanitizeElements    HookPoint = "afterSanitizeElements"
	AfterSanitizeShadowDOM   HookPoint = "afterSanitizeShadowDOM"
	BeforeSanitizeAttributes HookPoint = "beforeSanitizeAttributes"
	BeforeSanitizeElements   HookPoint = "beforeSanitizeElements"
	BeforeSanitizeShadowDOM  HookPoint = "beforeSanitizeShadowDOM"
	UponSanitizeAttribute    HookPoint = "uponSanitizeAttribute"
	UponSanitizeElement      HookPoint = "uponSanitizeElement"
	UponSanitizeShadowNode   HookPoint = "uponSanitizeShadowNode"
)

func validHookPoint(point HookPoint) bool {
	switch point {
	case AfterSanitizeAttributes, AfterSanitizeElements, AfterSanitizeShadowDOM,
		BeforeSanitizeAttributes, BeforeSanitizeElements, BeforeSanitizeShadowDOM,
		UponSanitizeAttribute, UponSanitizeElement, UponSanitizeShadowNode:
		return true
	}
	return false
}

// HookEvent is the data argument of a hook. It is nil for every entry point
// except uponSanitizeElement and uponSanitizeAttribute.
type HookEvent struct {
	// TagName and AllowedTags are set for uponSanitizeElement. A hook can
	// change AllowedTags for the current call of Sanitize.
	TagName     string
	AllowedTags map[string]bool

	// The fields below are set for uponSanitizeAttribute. A hook can change
	// AttrValue, KeepAttr, ForceKeepAttr and AllowedAttributes.
	AttrName          string
	AttrValue         string
	KeepAttr          bool
	AllowedAttributes map[string]bool
	ForceKeepAttr     bool
}

// Hook is a hook function. node is the current node. For the shadow DOM
// entry points node is the template element whose contents are sanitized.
// cfg is the configuration of the current call. A hook that removes a node
// must use Purifier.RemoveNode.
type Hook func(node *dom.Node, event *HookEvent, cfg *Config)

// HookID identifies a hook that AddHook added. JavaScript compares function
// objects for removeHook. Go functions cannot be compared.
type HookID int

type hookEntry struct {
	id HookID
	fn Hook
}

// AddHook is DOMPurify.addHook. It returns an id for RemoveHook. The id is 0
// when the entry point is unknown or the function is nil.
func (p *Purifier) AddHook(entryPoint HookPoint, hookFunction Hook) HookID {
	if hookFunction == nil || !validHookPoint(entryPoint) {
		return 0
	}
	p.nextID++
	p.hooks[entryPoint] = append(p.hooks[entryPoint], hookEntry{p.nextID, hookFunction})
	return p.nextID
}

// RemoveHook is DOMPurify.removeHook. With an id it removes the last hook of
// the entry point that has the id. With no id it removes the hook that was
// added last. It returns the removed hook, or nil.
func (p *Purifier) RemoveHook(entryPoint HookPoint, id ...HookID) Hook {
	list := p.hooks[entryPoint]
	if len(id) > 0 {
		for i := len(list) - 1; i >= 0; i-- {
			if list[i].id == id[0] {
				fn := list[i].fn
				p.hooks[entryPoint] = append(list[:i:i], list[i+1:]...)
				return fn
			}
		}
		return nil
	}
	if len(list) == 0 {
		return nil
	}
	fn := list[len(list)-1].fn
	p.hooks[entryPoint] = list[: len(list)-1 : len(list)-1]
	return fn
}

// RemoveHooks is DOMPurify.removeHooks.
func (p *Purifier) RemoveHooks(entryPoint HookPoint) {
	if validHookPoint(entryPoint) {
		p.hooks[entryPoint] = nil
	}
}

// RemoveAllHooks is DOMPurify.removeAllHooks.
func (p *Purifier) RemoveAllHooks() {
	p.hooks = map[HookPoint][]hookEntry{}
}

// executeHooks is _executeHooks. Array.prototype.forEach reads the length
// once and then reads each index from the live array.
func (p *Purifier) executeHooks(entryPoint HookPoint, currentNode *dom.Node, data *HookEvent) {
	n := len(p.hooks[entryPoint])
	for i := 0; i < n; i++ {
		list := p.hooks[entryPoint]
		if i >= len(list) {
			return
		}
		list[i].fn(currentNode, data, p.config)
	}
}
