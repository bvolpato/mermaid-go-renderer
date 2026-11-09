// Edge spy: renders diagrams with the real mermaid and records the inputs
// that insertEdge received for every edge, so that edge cases can use true
// edge objects and point lists. Run with:
//
//   node oracle.mjs esm-eval probes/edges-spy.js --args fixtures.json --out out.json
//
// args is { fixtures: [{ name, text }], layout }. `layout` (optional) is a
// site-level layout, for example "dagre". For each rendered edge path the
// result has the edge object as the layout left it, the diagram type, the
// markers of the diagram, the directive of the text, the points that
// insertEdge wrote to data-points, the cluster nodes that the edge names,
// the markup of the path, and the number of Math.random draws before the
// call (randomAt). The result also has the order of the edges of the layout
// data and the elk.lineHops setting, for replaying the line hops of a whole
// diagram.
//
// insertEdge is a module binding and cannot be replaced, so the spy wraps
// the layout loaders (to see data4Layout) and reads the rest from the
// rendered svg.
const layoutAlgorithms = await internalsFrom('chunk-YVPGI2KZ', 'layoutAlgorithms');
const clusterDb = await internalsFrom('chunk-WA6RL3DZ', 'clusterDb');
const preprocessDiagram = await internalsFrom('mermaid.esm.mjs', 'preprocessDiagram');
let current = null;
// insertEdge calls btoa once, with the points that it writes to
// data-points. The number of Math.random draws at that moment is the place
// of the edge in the random stream, which a hand-drawn edge depends on.
const drawsByPoints = new Map();
const origBtoa = globalThis.btoa.bind(globalThis);
globalThis.btoa = (text) => {
  const result = origBtoa(text);
  const queue = drawsByPoints.get(result) || [];
  queue.push(globalThis.__mmdgRandomCalls);
  drawsByPoints.set(result, queue);
  return result;
};
for (const name of Object.keys(layoutAlgorithms)) {
  const def = layoutAlgorithms[name];
  const origLoader = def.loader;
  def.loader = async () => {
    const mod = await origLoader();
    return {
      ...mod,
      render: async (data, svg, helpers, options) => {
        current = { data, name };
        return mod.render(data, svg, helpers, options);
      },
    };
  };
}
const clean = (v) =>
  JSON.parse(
    JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? `#num:${x}` : typeof x === 'function' ? undefined : x))
  );
const container = document.getElementById('container');
const out = [];
for (const { name, text } of args.fixtures) {
  const res = { name, edges: [] };
  out.push(res);
  try {
    mermaid.initialize({ startOnLoad: false, ...(args.layout ? { layout: args.layout } : {}) });
    current = null;
    drawsByPoints.clear();
    container.innerHTML = '';
    globalThis.__mmdgResetRandom();
    const { svg } = await mermaid.render('my-svg', text, container);
    if (!current) {
      res.skipped = 'no unified layout';
      continue;
    }
    const data = current.data;
    res.type = data.type;
    res.layout = current.name;
    res.elkLineHops = data.config?.elk?.lineHops;
    // The ids of the edges in the order of data4Layout.edges, which is the
    // order in which the common layout paints them.
    res.edgeOrder = data.edges.map((e) => ({ id: e.id, isLayoutOnly: e.isLayoutOnly === true, points: Array.isArray(e.points) ? e.points.length : -1 }));
    res.layoutAlgorithm = data.layoutAlgorithm;
    res.markers = data.markers;
    res.diagramId = data.diagramId;
    let directive = {};
    try {
      directive = preprocessDiagram(text).config || {};
    } catch {
      directive = {};
    }
    res.directive = clean(directive);
    const cfg = mermaid.mermaidAPI.getConfig();
    res.effective = {
      look: cfg.look,
      theme: cfg.theme,
      layout: cfg.layout,
      handDrawnSeed: cfg.handDrawnSeed,
      htmlLabels: cfg.htmlLabels,
      flowchartCurve: cfg.flowchart?.curve,
      flowchartHtmlLabels: cfg.flowchart?.htmlLabels,
    };
    const doc = new DOMParser().parseFromString(svg, 'image/svg+xml');
    const byId = new Map();
    for (const e of data.edges) {
      if (!byId.has(e.id)) {
        byId.set(e.id, e);
      }
    }
    for (const p of doc.querySelectorAll('path[data-edge="true"]')) {
      const id = p.getAttribute('data-id');
      const edge = byId.get(id);
      const rec = { id, html: p.outerHTML, parentClass: p.parentNode.getAttribute('class') };
      const draws = drawsByPoints.get(p.getAttribute('data-points'));
      if (draws && draws.length > 0) {
        rec.randomAt = draws.shift();
      }
      try {
        rec.dataPoints = JSON.parse(atob(p.getAttribute('data-points')));
      } catch (e) {
        rec.dataPointsError = String(e);
      }
      if (edge) {
        rec.edge = clean(edge);
        rec.pointKeys = (edge.points || []).map((pt) => Object.keys(pt).join(','));
        const clusters = {};
        for (const key of [edge.toCluster, edge.fromCluster]) {
          if (key && clusterDb.get(key)?.node) {
            const n = clusterDb.get(key).node;
            clusters[key] = { x: n.x, y: n.y, width: n.width, height: n.height };
          }
        }
        rec.clusterDb = clusters;
      }
      res.edges.push(rec);
    }
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
}
container.innerHTML = '';
return out;
