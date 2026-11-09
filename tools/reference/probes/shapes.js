// Shape oracle: calls the real insertNode of mermaid for each case and
// returns what it built. Run with:
//
//   node oracle.mjs esm-eval probes/shapes.js --args cases.json --out out.json
//
// A case is { name, type, directive, node, dir }: `type` is the diagram type
// whose config scope applies (for example "flowchart-v2"), `directive` is a
// config object as in front matter, and `node` is a layout node (the
// properties of rendering-util/types.ts). The result has the effective
// config, the style sheet, the markup that the shape produced, the size
// that it stored in the node, and where a set of rays from outside hit the
// shape (node.intersect).
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
// A diagram type registers its style sheet when it is first loaded. Load
// each type of the cases once, so that createUserStyles has its styles.
const samples = {
  'flowchart-v2': 'flowchart TD\nA',
  classDiagram: 'classDiagram\nclass A',
  stateDiagram: 'stateDiagram-v2\n[*] --> A',
  er: 'erDiagram\nA',
  requirement: 'requirementDiagram\nrequirement r {\nid: 1\n}',
  mindmap: 'mindmap\nroot',
  kanban: 'kanban\ntodo\n  a',
  block: 'block-beta\na',
};
for (const type of new Set(args.cases.map((c) => c.type || 'flowchart-v2'))) {
  const text = (args.samples && args.samples[type]) || samples[type];
  if (text) {
    await mermaid.mermaidAPI.getDiagramFromText(text);
  }
}
const container = document.getElementById('container');
const rays = [[-500, -300], [500, -300], [500, 300], [-500, 300], [0, -400], [0, 400], [-400, 0], [400, 0], [-37, 290], [123, -45], [1000, 10], [-3, -1000]];
const out = [];
for (const c of args.cases) {
  const res = { name: c.name };
  try {
    cfgMod.reset();
    cfgMod.setDiagramConfigScope(c.type || 'flowchart-v2');
    cfgMod.addDirective(c.directive || {});
    const config = cfgMod.getConfig();
    res.config = JSON.parse(JSON.stringify(config));
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
    res.css = createUserStyles(config, c.type || 'flowchart-v2', new Map(), '#my-svg');
    style.innerHTML = res.css;
    svgEl.appendChild(style);
    const g = select(svgEl).append('g').attr('class', 'nodes');
    globalThis.__mmdgResetRandom();
    const node = { ...c.node };
    const el = await insertNode(g, node, { config, dir: c.dir });
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
