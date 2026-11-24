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

const (
	hookAfterSanitizeAttributes = iota
	hookAfterSanitizeElements
	hookAfterSanitizeShadowDOM
	hookBeforeSanitizeAttributes
	hookBeforeSanitizeElements
	hookBeforeSanitizeShadowDOM
	hookUponSanitizeAttribute
	hookUponSanitizeElement
	hookUponSanitizeShadowNode
	numHookPoints
)

func hookIndex(point HookPoint) int {
	switch point {
	case AfterSanitizeAttributes:
		return hookAfterSanitizeAttributes
	case AfterSanitizeElements:
		return hookAfterSanitizeElements
	case AfterSanitizeShadowDOM:
		return hookAfterSanitizeShadowDOM
	case BeforeSanitizeAttributes:
		return hookBeforeSanitizeAttributes
	case BeforeSanitizeElements:
		return hookBeforeSanitizeElements
	case BeforeSanitizeShadowDOM:
		return hookBeforeSanitizeShadowDOM
	case UponSanitizeAttribute:
		return hookUponSanitizeAttribute
	case UponSanitizeElement:
		return hookUponSanitizeElement
	case UponSanitizeShadowNode:
		return hookUponSanitizeShadowNode
	}
	return -1
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
	idx := hookIndex(entryPoint)
	if hookFunction == nil || idx < 0 {
		return 0
	}
	p.nextID++
	p.hooks[idx] = append(p.hooks[idx], hookEntry{p.nextID, hookFunction})
	p.hookMask |= 1 << idx
	return p.nextID
}

// RemoveHook is DOMPurify.removeHook. With an id it removes the last hook of
// the entry point that has the id. With no id it removes the hook that was
// added last. It returns the removed hook, or nil.
func (p *Purifier) RemoveHook(entryPoint HookPoint, id ...HookID) Hook {
	idx := hookIndex(entryPoint)
	if idx < 0 {
		return nil
	}
	list := p.hooks[idx]
	if len(id) > 0 {
		for i := len(list) - 1; i >= 0; i-- {
			if list[i].id == id[0] {
				fn := list[i].fn
				list = append(list[:i:i], list[i+1:]...)
				p.hooks[idx] = list
				if len(list) == 0 {
					p.hookMask &^= 1 << idx
				}
				return fn
			}
		}
		return nil
	}
	if len(list) == 0 {
		return nil
	}
	fn := list[len(list)-1].fn
	list = list[: len(list)-1 : len(list)-1]
	p.hooks[idx] = list
	if len(list) == 0 {
		p.hookMask &^= 1 << idx
	}
	return fn
}

// RemoveHooks is DOMPurify.removeHooks.
func (p *Purifier) RemoveHooks(entryPoint HookPoint) {
	if idx := hookIndex(entryPoint); idx >= 0 {
		p.hooks[idx] = nil
		p.hookMask &^= 1 << idx
	}
}

// RemoveAllHooks is DOMPurify.removeAllHooks.
func (p *Purifier) RemoveAllHooks() {
	p.hooks = [numHookPoints][]hookEntry{}
	p.hookMask = 0
}

// executeHooksIdx is _executeHooks for a known hook index. Array.prototype.forEach
// reads the length once and then reads each index from the live array.
func (p *Purifier) executeHooksIdx(idx int, currentNode *dom.Node, data *HookEvent) {
	if p.hookMask&(1<<idx) == 0 {
		return
	}
	n := len(p.hooks[idx])
	for i := 0; i < n; i++ {
		list := p.hooks[idx]
		if i >= len(list) {
			return
		}
		list[i].fn(currentNode, data, p.config)
	}
}
