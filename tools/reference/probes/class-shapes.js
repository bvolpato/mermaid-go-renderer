// Shape oracle for the classBox shape: calls the real insertNode of mermaid
// with the layout nodes that the real class db builds. Run with:
//
//   node oracle.mjs esm-eval probes/class-shapes.js --args cases.json --out out.json
//
// A case is { name, directive, text, pick, patch, node, dir }:
//   directive  a config object as in front matter
//   text       a class diagram. The probe parses it with the config of the
//              directive and takes a node of db.getData()
//   pick       the id of the node (default: the first node with the shape
//              classBox)
//   patch      properties to set on the node before it is drawn. A property
//              with the value null is deleted
//   node       a node to draw instead of a node of a diagram text
//
// A class node has ClassMember objects, whose methods the shape calls, so
// the node cannot come from JSON as in probes/shapes.js. The result has the
// node as JSON (the form in which the Go test reads it), and what
// probes/shapes.js returns: the effective config, the style sheet, the
// markup that the shape produced, the size that it stored in the node, and
// where a set of rays from outside hit the shape (node.intersect).
const { insertNode, setDiagramConfigScope, addDirective, getConfig, createUserStyles } = await internals(
  'insertNode',
  'setDiagramConfigScope',
  'addDirective',
  'getConfig',
  'createUserStyles'
);
const cfgMod = {
  reset: await internalsFrom('chunk-DLUCIBNR', 'reset'),
  setDiagramConfigScope,
  addDirective,
  getConfig,
};
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
mermaid.initialize({ startOnLoad: false });
// The class diagram registers its style sheet when it is first loaded.
await mermaid.mermaidAPI.getDiagramFromText('classDiagram\nclass A');
const container = document.getElementById('container');
const rays = [[-500, -300], [500, -300], [500, 300], [-500, 300], [0, -400], [0, 400], [-400, 0], [400, 0], [-37, 290], [123, -45], [1000, 10], [-3, -1000]];
const clean = (v) => JSON.parse(JSON.stringify(v));
const out = [];
for (const c of args.cases) {
  const res = { name: c.name, type: 'classDiagram', dir: c.dir };
  try {
    cfgMod.reset();
    cfgMod.setDiagramConfigScope('classDiagram');
    cfgMod.addDirective(c.directive || {});
    const config = cfgMod.getConfig();
    res.config = clean(config);
    let node;
    if (c.node) {
      node = { ...c.node };
    } else {
      const diag = await mermaid.mermaidAPI.getDiagramFromText(c.text);
      const nodes = diag.db.getData().nodes;
      const picked = c.pick ? nodes.find((n) => n.id === c.pick) : nodes.find((n) => n.shape === 'classBox');
      if (!picked) {
        throw new Error('no node to pick');
      }
      node = { ...picked };
      // render() of rendering-util/render.ts adds the id of the diagram.
      node.domId = `my-svg-${node.domId || node.id}`;
    }
    for (const [k, v] of Object.entries(c.patch || {})) {
      if (v === null) {
        delete node[k];
      } else {
        node[k] = v;
      }
    }
    res.node = clean(node);
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
    res.css = createUserStyles(config, 'classDiagram', new Map(), '#my-svg');
    style.innerHTML = res.css;
    svgEl.appendChild(style);
    const g = select(svgEl).append('g').attr('class', 'nodes');
    globalThis.__mmdgResetRandom();
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
