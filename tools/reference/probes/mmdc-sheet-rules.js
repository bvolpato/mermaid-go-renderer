// Probe: the rules of the style sheets of the page of mermaid-cli, as the
// font embedder of mermaid-cli reads them (src/browser/fontEmbedder.js):
// the cssText of every rule, the family and the src of a @font-face rule,
// and the selector of a style rule. Run with oracle.mjs eval --bare
// --webfonts. gen-mmdc-sheets.mjs writes the result for internal/mmdc.
const sheets = [];
for (const link of document.querySelectorAll('link')) {
  if (!link.classList.contains('mermaid-cli-css') || !link.sheet) {
    continue;
  }
  const rules = [];
  for (const rule of link.sheet.cssRules) {
    if (rule instanceof CSSFontFaceRule) {
      rules.push({ kind: 'font-face', family: rule.style.fontFamily, src: rule.style.src, text: rule.cssText });
    } else if (rule instanceof CSSStyleRule) {
      rules.push({ kind: 'style', selector: rule.selectorText, text: rule.cssText });
    } else {
      rules.push({ kind: 'other', text: rule.cssText });
    }
  }
  sheets.push({ href: link.href, rules });
}
return { origin: location.origin, sheets };
