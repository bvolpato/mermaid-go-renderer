// Cluster oracle: calls the real insertCluster of mermaid for each case and
// returns what it built. Run with:
//
//   node oracle.mjs esm-eval probes/clusters.js --args cases.json --out out.json
//
// The args are { cases, all, samples, geometry, paintsTitle }.
//
// A case is { name, type, directive, node }: `type` is the diagram type
// whose config scope and style sheet apply (for example "flowchart-v2"),
// `directive` is a config object as in front matter, and `node` is a
// laid-out cluster node (the properties of rendering-util/types.ts, with x,
// y, width and height). The result has the effective config, the style
// sheet, the markup that the cluster shape produced, the returned labelBBox,
// the fields that the shape wrote into the node (`after`: width, height,
// diff, offsetX, offsetY, labelBBox), the number of random draws and where
// a set of rays hit the cluster (node.intersect).
//
// An entry of `all` is { name, type, directive, nodes }: the clusters of the
// nodes are inserted together, as the dagre layout does:
//
//   await Promise.all(nodes.map(async (node) => { await insertCluster(g, node); ... }))
//
// The result has the markup, the order in which the calls finished and the
// number of random draws. The order of the work shows in the markup: with
// the handDrawn look the shapes draw from Math.random.
//
// `samples` maps a diagram type to a diagram text that loads the type, so
// that its style sheet is registered. `geometry` is a list of
// { node, labelBBox } for getUsecaseSystemBoundaryGeometry, and
// `paintsTitle` is a list of shape names for clusterPaintsTitle (null
// stands for undefined).
const { setDiagramConfigScope, addDirective, getConfig, createUserStyles } = await internals(
  'setDiagramConfigScope',
  'addDirective',
  'getConfig',
  'createUserStyles'
);
const clusterChunk = internalsWhere('insertCluster')[0];
const insertCluster = await internalsFrom(clusterChunk, 'insertCluster');
const clusterPaintsTitle = await internalsFrom(clusterChunk, 'clusterPaintsTitle');
const getUsecaseSystemBoundaryGeometry = await internalsFrom(clusterChunk, 'getUsecaseSystemBoundaryGeometry');
const reset = await internalsFrom(internalsWhere('setDiagramConfigScope')[0], 'reset');
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
mermaid.initialize({ startOnLoad: false });

// A diagram type registers its style sheet when it is first loaded. Load
// each type of the cases once, so that createUserStyles has its styles.
const samples = {
  'flowchart-v2': 'flowchart TD\nA',
  classDiagram: 'classDiagram\nclass A',
  stateDiagram: 'stateDiagram-v2\n[*] --> A',
  kanban: 'kanban\ntodo\n  a',
  block: 'block-beta\na',
  ...(args.samples || {}),
};
const loadErrors = {};
for (const type of new Set([...(args.cases || []), ...(args.all || [])].map((c) => c.type || 'flowchart-v2'))) {
  if (samples[type]) {
    try {
      await mermaid.mermaidAPI.getDiagramFromText(samples[type]);
    } catch (e) {
      loadErrors[type] = String((e && e.message) || e);
    }
  }
}

// JSON has no NaN and no undefined: both are written as strings.
const num = (v) => (v === undefined ? 'undefined' : typeof v === 'number' && !Number.isFinite(v) ? String(v) : v);
const box = (b) => (b === undefined || b === null ? null : { x: num(b.x), y: num(b.y), width: num(b.width), height: num(b.height) });

const container = document.getElementById('container');
const rays = [[-500, -300], [500, -300], [500, 300], [-500, 300], [0, -400], [0, 400], [-400, 0], [400, 0], [-37, 290], [123, -45], [1000, 10], [-3, -1000]];

// setup applies the config of a case and builds its page: the svg with the
// style sheet and the group for the clusters. It records the config and
// the style sheet in res and resets the seeded Math.random.
const setup = (c, res) => {
  reset();
  setDiagramConfigScope(c.type || 'flowchart-v2');
  addDirective(c.directive || {});
  const config = getConfig();
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
  const g = select(svgEl).append('g').attr('class', 'clusters');
  globalThis.__mmdgResetRandom();
  return g;
};

const out = [];
for (const c of args.cases || []) {
  const res = { name: c.name };
  try {
    const g = setup(c, res);
    const node = JSON.parse(JSON.stringify(c.node));
    const result = await insertCluster(g, node);
    res.html = g.node().innerHTML;
    res.randomCalls = globalThis.__mmdgRandomCalls;
    res.clusterIsLastChild = result.cluster.node() === g.node().lastElementChild;
    res.labelBBox = box(result.labelBBox);
    res.after = {
      width: num(node.width),
      height: num(node.height),
      diff: num(node.diff),
      offsetX: num(node.offsetX),
      offsetY: num(node.offsetY),
      labelBBox: box(node.labelBBox),
    };
    res.intersects = rays.map(([x, y]) => {
      try {
        const p = node.intersect({ x, y });
        return [num(p.x), num(p.y)];
      } catch (e) {
        return 'ERR ' + e.message;
      }
    });
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  out.push(res);
}

const all = [];
for (const c of args.all || []) {
  const res = { name: c.name };
  try {
    const g = setup(c, res);
    const nodes = JSON.parse(JSON.stringify(c.nodes));
    const order = [];
    await Promise.all(
      nodes.map(async (node, i) => {
        await insertCluster(g, node);
        order.push(i);
      })
    );
    res.html = g.node().innerHTML;
    res.randomCalls = globalThis.__mmdgRandomCalls;
    res.order = order;
    res.after = nodes.map((node) => ({
      width: num(node.width),
      height: num(node.height),
      diff: num(node.diff),
      offsetX: num(node.offsetX),
      offsetY: num(node.offsetY),
      labelBBox: box(node.labelBBox),
    }));
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  all.push(res);
}
container.innerHTML = '';

const geometry = (args.geometry || []).map((g) => {
  try {
    const r = getUsecaseSystemBoundaryGeometry(g.node, g.labelBBox);
    return Object.fromEntries(Object.entries(r).map(([k, v]) => [k, num(v)]));
  } catch (e) {
    return { error: String((e && e.message) || e) };
  }
});
const paintsTitle = (args.paintsTitle || []).map((shape) => clusterPaintsTitle(shape === null ? undefined : shape));

return { cases: out, all, geometry, paintsTitle, loadErrors };
