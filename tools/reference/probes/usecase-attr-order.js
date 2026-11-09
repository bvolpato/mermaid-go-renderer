// Probe: the order of the class and the style attribute of the svg element
// of a usecase diagram. The plain page of oracle.mjs render has style first.
// The page of mermaid-cli has class first. Run with oracle.mjs esm-eval
// (the ESM build of mermaid, which mermaid-cli loads) or with eval (the
// IIFE build). args: { text }.
const mermaid = globalThis.mermaid;
mermaid.initialize({ startOnLoad: false });
const container = document.getElementById('container');
const { svg } = await mermaid.render('my-svg', args.text, container);
return { head: svg.slice(0, svg.indexOf('>')).replace(/style="[^"]*"/, 'style="..."') };
