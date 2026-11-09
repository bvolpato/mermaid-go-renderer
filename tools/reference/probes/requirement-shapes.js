// Shape oracle for the requirementBox shape: calls the real insertNode of
// mermaid with the layout nodes that the real requirement db builds. Run
// with:
//
//   node oracle.mjs esm-eval probes/requirement-shapes.js --args cases.json --out out.json
//
// A case is { name, directive, text, source, pick, patch, node, dir }:
//   directive  a config object as in front matter
//   text       a requirement diagram. The probe parses it with the config of
//              the directive and takes a node of db.getData()
//   source     a diagram text with front matter and directives, as a corpus
//              fixture is. It replaces directive and text: the probe takes
//              them from the source as mermaid.render does
//   pick       the id of the node (default: the first node)
//   patch      properties to set on the node before it is drawn. A property
//              with the value null is deleted
//   node       a node to draw instead of a node of a diagram text
//   all        draw every node of the diagram text, together, as a layout
//              does: await Promise.all(nodes.map(async (node) => ...)).
//              The nodes then draw in the order of the microtask queue,
//              which decides the random numbers that each node takes
//
// The result has the node as JSON (the form in which the Go test reads it),
// and what probes/shapes.js returns: the effective config, the style sheet,
// the markup that the shape produced, the size that it stored in the node,
// the number of random draws, and where a set of rays from outside hit the
// shape (node.intersect). A case with `all` has the list of the nodes and
// the list of their sizes.
const { insertNode, setDiagramConfigScope, addDirective, getConfig, createUserStyles, preprocessDiagram } = await internals(
  'insertNode',
  'setDiagramConfigScope',
  'addDirective',
  'getConfig',
  'createUserStyles',
  'preprocessDiagram'
);
const cfgMod = {
  reset: await internalsFrom('chunk-DLUCIBNR', 'reset'),
  setDiagramConfigScope,
  addDirective,
  getConfig,
};
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
mermaid.initialize({ startOnLoad: false });
// The requirement diagram registers its style sheet when it is first loaded.
await mermaid.mermaidAPI.getDiagramFromText('requirementDiagram\nrequirement r {\nid: 1\n}');
const container = document.getElementById('container');
const rays = [[-500, -300], [500, -300], [500, 300], [-500, 300], [0, -400], [0, 400], [-400, 0], [400, 0], [-37, 290], [123, -45], [1000, 10], [-3, -1000]];
const clean = (v) => JSON.parse(JSON.stringify(v));
const out = [];
for (const c of args.cases) {
  const res = { name: c.name, type: 'requirement', dir: c.dir };
  try {
    let directive = c.directive || {};
    let text = c.text;
    if (c.source) {
      const processed = preprocessDiagram(c.source);
      directive = processed.config ?? {};
      text = processed.code.cleaned;
    }
    cfgMod.reset();
    cfgMod.setDiagramConfigScope('requirement');
    cfgMod.addDirective(directive);
    const config = cfgMod.getConfig();
    res.config = clean(config);
    let node;
    let all;
    if (c.node) {
      node = { ...c.node };
    } else if (c.all) {
      const diag = await mermaid.mermaidAPI.getDiagramFromText(text);
      // render() of rendering-util/render.ts adds the id of the diagram.
      all = diag.db.getData().nodes.map((n) => ({ ...n, domId: `my-svg-${n.domId || n.id}` }));
      res.nodes = clean(all);
    } else {
      const diag = await mermaid.mermaidAPI.getDiagramFromText(text);
      const nodes = diag.db.getData().nodes;
      const picked = c.pick ? nodes.find((n) => n.id === c.pick) : nodes[0];
      if (!picked) {
        throw new Error('no node to pick');
      }
      node = { ...picked };
      // render() of rendering-util/render.ts adds the id of the diagram.
      node.domId = `my-svg-${node.domId || node.id}`;
    }
    if (node) {
      for (const [k, v] of Object.entries(c.patch || {})) {
        if (v === null) {
          delete node[k];
        } else {
          node[k] = v;
        }
      }
      res.node = clean(node);
    }
    container.innerHTML = '';
    const div = document.createElement('div');
    div.id = 'dmy-svg';
    div.setAttribute('style', `font-family: ${config.fontFamily}`);
    container.appendChild(div);
    const svgEl = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svgEl.setAttribute('id', 'my-svg');
    svgEl.setAttribute('width', '100%');
    svgEl.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
    div.appendChild(svgEl);
    const style = document.createElement('style');
    res.css = createUserStyles(config, 'requirement', new Map(), '#my-svg');
    style.innerHTML = res.css;
    svgEl.appendChild(style);
    const g = select(svgEl).append('g').attr('class', 'nodes');
    globalThis.__mmdgResetRandom();
    if (all) {
      await Promise.all(
        all.map(async (n) => {
          await insertNode(g, n, { config, dir: c.dir });
        })
      );
      res.html = g.node().innerHTML;
      res.sizes = all.map((n) => [n.width, n.height]);
      res.randomCalls = globalThis.__mmdgRandomCalls;
      out.push(res);
      continue;
    }
    await insertNode(g, node, { config, dir: c.dir });
    res.html = g.node().innerHTML;
    res.width = node.width;
    res.height = node.height;
    res.randomCalls = globalThis.__mmdgRandomCalls;
    res.labelStyle = node.labelStyle;
    res.shape = node.shape;
    node.x = 0;
    node.y = 0;
    res.intersects = rays.map(([x, y]) => {
      try {
        const p = node.intersect({ x, y });
        return [p.x, p.y];
      } catch (e) {
        return 'ERR ' + e.message;
      }
    });
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  out.push(res);
}
container.innerHTML = '';
return out;
