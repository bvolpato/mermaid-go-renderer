// Page script of gen-xml-serialize.mjs. It runs with the eval command of the
// browser oracle (oracle.mjs eval --bare).
//
// It repeats what mermaid-cli does with the string that mermaid.render()
// returns (renderMermaid in src/index.js of mermaid-cli): the string becomes
// the content of div#container, the page changes the svg element, and
// XMLSerializer writes it. The script does this for every case and every
// variant in args:
//
//   args.cases:    [{ name, svg, script? }]  script is JavaScript that runs
//                  with the svg element in scope, for DOM that the HTML
//                  parser cannot make
//   args.variants: [{ name, background?, size?, css? }]
//   args.trees:    true to also return the DOM tree of every case, so that a
//                  test can build the same tree without the HTML parser
//
// The result is [{ name, out: { variant: string | null }, tree? }].
const container = document.getElementById('container');

const dump = (node) => {
  if (node.nodeType === Node.TEXT_NODE) {
    return { t: node.data };
  }
  if (node.nodeType === Node.COMMENT_NODE) {
    return { c: node.data };
  }
  if (node.nodeType !== Node.ELEMENT_NODE) {
    return { other: node.nodeType };
  }
  const el = { ns: node.namespaceURI, tag: node.localName };
  if (node.prefix) {
    el.prefix = node.prefix;
  }
  el.attrs = Array.from(node.attributes, (a) => {
    const attr = { local: a.localName, value: a.value };
    if (a.namespaceURI) {
      attr.ns = a.namespaceURI;
    }
    if (a.prefix) {
      attr.prefix = a.prefix;
    }
    return attr;
  });
  el.kids = Array.from(node.childNodes, dump);
  return el;
};

const results = [];
for (const c of args.cases) {
  const result = { name: c.name, out: {} };
  for (const v of args.variants) {
    container.innerHTML = c.svg;
    const svg = container.getElementsByTagName('svg')[0];
    if (!svg) {
      result.out[v.name] = null;
      continue;
    }
    if (c.script) {
      new Function('svg', 'document', c.script)(svg, document);
    }
    // The steps of mermaid-cli, in its order.
    if (v.background !== undefined) {
      svg.style.backgroundColor = v.background;
    }
    if (v.size) {
      svg.style.maxWidth = `${v.size}px`;
      svg.style.maxHeight = `${v.size}px`;
      svg.removeAttribute('height');
    }
    if (v.css) {
      // mermaid-cli 11 adds the CSS file as a style element at the end.
      const style = document.createElementNS('http://www.w3.org/2000/svg', 'style');
      style.appendChild(document.createTextNode(v.css));
      svg.appendChild(style);
    }
    if (args.trees && result.tree === undefined) {
      // The tree before the style attribute changes is what a test starts
      // from. The first variant must have no step.
      result.tree = dump(svg);
    }
    result.out[v.name] = new XMLSerializer().serializeToString(svg);
  }
  results.push(result);
}
container.innerHTML = '';
return results;
