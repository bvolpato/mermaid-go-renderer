// ELK layout capture for internal/ru/layout/elk. It renders diagrams with the
// real mermaid and records, for every render that goes through an ELK layout:
//
//   entry     the edges of data4Layout when the layout render starts
//   prepared  the edges after prepareLayoutForElk (buildEdgeData)
//   data      data4Layout when elk.layout is called: the measured nodes and
//             edges, and the config values that the layout reads
//   in        JSON.stringify of the graph that elk.layout receives
//   out       JSON.stringify of the graph that elk.layout resolves with
//   intersect every call of node.intersect during the post-processing, in
//             call order: [index of the node in the ELK graph (depth first),
//             point x, point y, result x, result y]. The result is null,
//             null when the function returns no point.
//
// A number that is not finite is recorded as { $num: "NaN" } and so on.
//   result    the nodes and edges of data4Layout when runElkLayoutCore ends:
//             what positionNode and insertEdge read
//   random    the number of Math.random calls during elk.layout
//
// Run with:
//
//   node oracle.mjs esm-eval probes/ru-elk.js --args cases.json --out out.json
//
// args is { cases: [{ name, text }] }. See gen-ru-elk.mjs.
//
// The functions of the layout are module bindings and cannot be replaced.
// The capture wraps the layout loaders (to see data4Layout), the layout
// method of the ELK class, and two array methods of data4Layout.nodes:
// createGraphWithElements calls nodes.map first, after the prepare phase,
// and orderNodesForElkPaint calls nodes.sort last, after the layout.
const layoutAlgorithms = await internalsFrom('chunk-YVPGI2KZ', 'layoutAlgorithms');
const elkBundled = await internalsFrom('elk-', 'import_elk_bundled');
const ELK = elkBundled.default;

// A number that is not finite is kept as { $num: "NaN" }. JSON drops
// functions and undefined.
const enc = (v) =>
  JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x));
const clean = (v) => {
  const s = enc(v);
  return s === undefined ? undefined : JSON.parse(s);
};

let current = null; // the record of the fixture that renders now
let active = null; // { r, data4Layout, helpers } of the layout render that runs now

const snapData = (d, helpers) => {
  const cfg = helpers.getConfig();
  return {
    type: d.type,
    layoutAlgorithm: d.layoutAlgorithm,
    direction: d.direction,
    diagramId: d.diagramId,
    nodes: clean(Array.prototype.slice.call(d.nodes)),
    edges: clean(Array.prototype.slice.call(d.edges)),
    // 1 for a node that has an intersect function.
    intersect: Array.prototype.map.call(d.nodes, (n) => (typeof n.intersect === 'function' ? 1 : 0)),
    curveTypes: Array.prototype.map.call(d.edges, (e) => typeof e.curve),
    edgesDefaultInterpolate: clean(d.edges.defaultInterpolate),
    edgesDefaultStyle: clean(d.edges.defaultStyle),
    config: { elk: clean(d.config && d.config.elk) },
    getConfig: { curve: clean(cfg.curve), flowchart: { wrappingWidth: clean(cfg.flowchart && cfg.flowchart.wrappingWidth) } },
  };
};

const point = (p) => (p ? { x: p.x, y: p.y } : null);
const snapResult = (d) => ({
  nodes: Array.prototype.map.call(d.nodes, (n) =>
    clean({ id: n.id, x: n.x, y: n.y, width: n.width, height: n.height, labelData: n.labelData, labels: n.labels })
  ),
  edges: Array.prototype.map.call(d.edges, (e) =>
    clean({
      id: e.id,
      points: Array.isArray(e.points) ? e.points.map(point) : e.points,
      x: e.x,
      y: e.y,
      curve: typeof e.curve === 'string' ? e.curve : { $type: typeof e.curve },
      terminalLabelCenters: e.terminalLabelCenters,
    })
  ),
});

// The layout method is on the prototype of the class ELK of elk-api.js.
let proto = ELK.prototype;
while (proto && !Object.prototype.hasOwnProperty.call(proto, 'layout')) {
  proto = Object.getPrototypeOf(proto);
}
const origLayout = proto.layout;
proto.layout = function (graph, ...rest) {
  if (current) {
    current.elkCalls++;
  }
  if (!active) {
    return origLayout.call(this, graph, ...rest);
  }
  const call = { intersect: [] };
  active.r.elk.push(call);
  try {
    call.data = snapData(active.data4Layout, active.helpers);
    call.in = JSON.stringify(graph);
  } catch (e) {
    call.captureError = String((e && e.message) || e);
  }
  // The nodes of the graph are the objects of nodeDb. The post-processing
  // calls their intersect functions.
  let index = 0;
  const walk = (n) => {
    const i = index++;
    if (n && typeof n.intersect === 'function') {
      const orig = n.intersect;
      n.intersect = function (p) {
        const res = orig.call(this, p);
        call.intersect.push(clean([i, p.x, p.y, res ? res.x : null, res ? res.y : null]));
        return res;
      };
    }
    for (const c of (n && n.children) || []) {
      walk(c);
    }
  };
  for (const c of graph.children || []) {
    walk(c);
  }
  const before = globalThis.__mmdgRandomCalls;
  return origLayout.call(this, graph, ...rest).then(
    (result) => {
      call.random = globalThis.__mmdgRandomCalls - before;
      call.same = result === graph;
      try {
        call.out = JSON.stringify(result);
      } catch (e) {
        call.captureError = String((e && e.message) || e);
      }
      return result;
    },
    (err) => {
      call.error = String((err && err.message) || err);
      throw err;
    }
  );
};

const hide = (obj, name, value) => Object.defineProperty(obj, name, { value, enumerable: false, configurable: true, writable: true });

for (const name of Object.keys(layoutAlgorithms)) {
  if (name !== 'elk' && !name.startsWith('elk.')) {
    continue;
  }
  const def = layoutAlgorithms[name];
  const origLoader = def.loader;
  def.loader = async () => {
    const mod = await origLoader();
    return {
      ...mod,
      render: async (data4Layout, svg, helpers, options) => {
        const r = { layout: name, algorithm: options && options.algorithm, elk: [], results: [] };
        if (current) {
          current.renders.push(r);
        }
        const cfgBefore = enc(helpers.getConfig());
        r.entry = clean(Array.prototype.slice.call(data4Layout.edges));
        const nodes = data4Layout.nodes;
        let first = true;
        hide(nodes, 'map', function (...a) {
          if (first) {
            first = false;
            r.prepared = clean(Array.prototype.slice.call(data4Layout.edges));
            r.preparedCurveTypes = Array.prototype.map.call(data4Layout.edges, (e) => typeof e.curve);
            // Whether setConfig(getConfig()) of syncHostConfig changed the config.
            r.configSame = enc(helpers.getConfig()) === cfgBefore;
          }
          return Array.prototype.map.apply(this, a);
        });
        hide(nodes, 'sort', function (...a) {
          const out = Array.prototype.sort.apply(this, a);
          r.results.push(snapResult(data4Layout));
          return out;
        });
        const prev = active;
        active = { r, data4Layout, helpers };
        try {
          return await mod.render(data4Layout, svg, helpers, options);
        } finally {
          active = prev;
          r.sameNodesArray = data4Layout.nodes === nodes;
          delete nodes.map;
          delete nodes.sort;
        }
      },
    };
  };
}

const container = document.getElementById('container');
mermaid.initialize({ startOnLoad: false });
const out = [];
for (const c of args.cases) {
  const rec = { name: c.name, elkCalls: 0, renders: [] };
  current = rec;
  container.innerHTML = '';
  // A reference render starts with one Math.random call made: mermaid draws
  // one number when it loads (randomBefore in the reference metadata).
  globalThis.__mmdgResetRandom();
  Math.random();
  try {
    const { diagramType } = await mermaid.render('my-svg', c.text, container);
    rec.type = diagramType;
  } catch (e) {
    rec.error = String((e && e.message) || e);
  }
  current = null;
  document.querySelectorAll('body > :not(#container)').forEach((el) => el.remove());
  // A fixture that does not use ELK is reported by name only.
  out.push(rec.elkCalls > 0 || rec.renders.length > 0 ? rec : { name: c.name, type: rec.type, error: rec.error, elkCalls: 0 });
}
return out;
