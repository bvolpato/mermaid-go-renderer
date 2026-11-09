// Layout data capture: builds the layout data of diagram texts with
// upstream mermaid, as the diagram renderers hand it to render(). Run with:
//
//   node oracle.mjs esm-eval probes/graph-capture.js --args fixtures.json --out out.json
//
// The args are { fixtures: [{ name, text }] }. The result is a list of
// { fixture, type, directive, data }: `data` is db.getData() with the
// properties that the renderer adds (type, direction, diagramId, markers)
// and with the DOM ids of the nodes prefixed as render() does, without the
// config. `directive` is the config of the front matter and the init
// directives of the diagram text. gen-graph.mjs feeds this data to
// probes/graph.js as realistic cases.
const { processAndSetConfigs } = await internals('processAndSetConfigs');

// JSON has no NaN and no undefined: properties with these values, and
// functions, are left out.
const plain = (value) =>
  JSON.parse(
    JSON.stringify(value, (key, v) => {
      if (typeof v === 'function' || (typeof v === 'number' && !Number.isFinite(v))) {
        return undefined;
      }
      return v;
    })
  );

const out = [];
for (const fixture of args.fixtures) {
  try {
    mermaid.initialize({ startOnLoad: false });
    // As mermaid.render does: the config of the text applies to getData.
    let directive = {};
    try {
      directive = plain(processAndSetConfigs(fixture.text).config || {});
    } catch (e) {
      directive = {};
    }
    const diag = await mermaid.mermaidAPI.getDiagramFromText(fixture.text);
    if (typeof diag.db.getData !== 'function') {
      out.push({ fixture: fixture.name, error: 'the db has no getData' });
      continue;
    }
    if (typeof diag.db.setDiagramId === 'function') {
      diag.db.setDiagramId('my-svg');
    }
    const data = diag.db.getData();
    const result = {
      nodes: data.nodes,
      edges: data.edges,
      type: diag.type,
      direction: typeof diag.db.getDirection === 'function' ? diag.db.getDirection() : undefined,
      diagramId: 'my-svg',
      markers: ['point', 'circle', 'cross'],
    };
    // render() of rendering-util/render.ts.
    for (const node of result.nodes) {
      const originalDomId = node.domId || node.id;
      node.domId = `my-svg-${originalDomId}`;
    }
    out.push({ fixture: fixture.name, type: diag.type, directive, data: plain(result) });
  } catch (e) {
    out.push({ fixture: fixture.name, error: String((e && e.message) || e) });
  }
}
return out;
