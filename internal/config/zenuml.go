// Ported from @mermaid-js/mermaid-zenuml 1.0.1, src/detector.ts, and
// mermaid 12.2.0, mermaidAPI.ts (processAndSetConfigs) (MIT).

package config

import "github.com/bvolpato/mermaid-go-renderer/internal/js"

var detectsZenUML = re(`^\s*zenuml`)

// DetectTypeMermaidCLI includes the extra detector that mermaid-cli loads.
// The plain-page detector list stays unchanged.
func DetectTypeMermaidCLI(text string, cfg *js.Obj[any]) (string, error) {
	return detect(text, cfg, true)
}

// ResolveMermaidCLI resolves config with the plugins of the mermaid-cli page.
func ResolveMermaidCLI(text string, site *js.Obj[any]) (*Resolved, error) {
	return resolve(text, site, true)
}
