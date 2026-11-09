// Cluster capture: renders diagram texts with upstream mermaid and records
// the nodes that the layouts hand to insertCluster. Run with:
//
//   node oracle.mjs esm-eval probes/clusters-capture.js --args fixtures.json --out out.json
//
// The args are { fixtures: [{ name, text, init }] }: `init` is an optional
// config for mermaid.initialize, for example { layout: 'dagre' }. The
// result is a list of { fixture, type, directive, shape, layout, node }:
// `node` is the cluster node right before the cluster shape function runs,
// without its functions, `directive` is the config of the front matter and
// the init directives of the diagram text, `layout` is the layout of the
// effective config and `type` is the diagram type. gen-clusters.mjs feeds
// these nodes to probes/clusters.js as realistic cases.
const clusterChunk = internalsWhere('insertCluster')[0];
const shapes = await internalsFrom(clusterChunk, 'shapes');
const { processAndSetConfigs, getConfig } = await internals('processAndSetConfigs', 'getConfig');

// JSON has no NaN and no undefined: properties with these values, and
// functions and nested graphs, are left out.
const plain = (value) =>
  JSON.parse(
    JSON.stringify(value, (key, v) => {
      if (typeof v === 'function' || key === 'graph' || key === 'domNode') {
        return undefined;
      }
      if (typeof v === 'number' && !Number.isFinite(v)) {
        return undefined;
      }
      return v;
    })
  );

const captured = [];
let current = null;
const original = {};
for (const name of Object.keys(shapes)) {
  original[name] = shapes[name];
  shapes[name] = (parent, node) => {
    if (current) {
      try {
        captured.push({ fixture: current.name, shape: node.shape || 'rect', layout: getConfig().layout, node: plain(node) });
      } catch (e) {
        // A node that cannot be serialized is not a useful case.
      }
    }
    return original[name](parent, node);
  };
}

const container = document.getElementById('container');
const out = [];
for (const fixture of args.fixtures) {
  current = fixture;
  const before = captured.length;
  try {
    mermaid.initialize({ startOnLoad: false, ...(fixture.init || {}) });
    container.innerHTML = '';
    const { diagramType } = await mermaid.render('my-svg', fixture.text, container);
    let directive = {};
    try {
      directive = plain(processAndSetConfigs(fixture.text).config || {});
    } catch (e) {
      directive = {};
    }
    for (const c of captured.slice(before)) {
      out.push({ ...c, type: diagramType, directive });
    }
  } catch (e) {
    out.push({ fixture: fixture.name, error: String((e && e.message) || e) });
  }
}
current = null;
for (const name of Object.keys(original)) {
  shapes[name] = original[name];
}
container.innerHTML = '';
return out;
