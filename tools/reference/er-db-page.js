// Body of the page function of gen-er-db.mjs. The oracle runs it with the
// esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. Every layout is
// replaced by a function that keeps the layout data and then stops the
// render. So the real draw function of the ER renderer runs up to the call
// of the layout: the dump has the layout data as the layout receives it (the
// nodes and edges of getData, the config with the spacing of the renderer,
// the direction, the markers, the layout algorithm), and the content of the
// db (the entities, the relationships, the classes, the subgraphs, the
// direction, the title and the accessibility values).
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page. In the
// shared page the DOMPurify hooks of mermaid are installed before the first
// case, so that the state is the same for every case.
const erChunk = internalsWhere('ErDB').find((f) => f.startsWith('erDiagram'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (erChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const { diagram } = await import('/chunks/mermaid.esm/' + erChunk);
  const SENTINEL = '__er_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );

  // Every layout keeps the layout data and stops the render.
  let layoutData;
  const capture = async () => ({
    render: async (data4Layout) => {
      layoutData = data4Layout;
      throw new Error(SENTINEL);
    },
  });
  const elk = ['elk.stress', 'elk.force', 'elk.mrtree', 'elk.sporeOverlap', 'elk.box', 'elk.rectpacking'];
  mermaid.registerLayoutLoaders([
    { name: 'dagre', loader: capture },
    { name: 'swimlane', loader: capture },
    { name: 'cose-bilkent', loader: capture },
    { name: 'elk', loader: capture, algorithm: 'elk.layered' },
    ...elk.map((name) => ({ name, loader: capture, algorithm: name })),
  ]);

  // The draw function of the renderer runs between two dumps of the db:
  // getData changes the entities.
  const realDraw = diagram.renderer.draw;
  let captured;
  diagram.renderer = {
    draw: async (text, id, version, diag) => {
      const db = diag.db;
      const rec = {
        type: diag.type,
        entitiesBefore: clean([...db.getEntities().entries()]),
        subgraphDepth: db.subgraphDepth,
      };
      layoutData = undefined;
      try {
        await realDraw(text, id, version, diag);
      } catch (e) {
        if (String((e && e.message) || e) !== SENTINEL || !layoutData) {
          throw e;
        }
      }
      if (!layoutData) {
        throw new Error('the draw function did not call a layout');
      }
      const data = layoutData;
      Object.assign(rec, {
        nodes: clean(data.nodes),
        edges: clean(data.edges),
        other: clean(data.other),
        config: clean(data.config),
        dataDirection: data.direction,
        dataType: data.type,
        layout: data.layoutAlgorithm,
        markers: clean(data.markers),
        diagramId: data.diagramId,
        // The other properties of the layout data, by name.
        dataKeys: Object.keys(data),
        entities: clean([...db.getEntities().entries()]),
        relationships: clean(db.getRelationships()),
        classes: clean([...db.getClasses().entries()]),
        subGraphs: clean(db.getSubGraphs()),
        direction: db.getDirection(),
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
        erConfig: clean(db.getConfig()),
      });
      captured = rec;
      throw new Error(SENTINEL);
    },
  };
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    try {
      await mermaid.render('my-svg', 'erDiagram\n  accTitle: warm\n  WARM', document.getElementById('container'));
    } catch {
      // The replaced draw function stops the render.
    }
  };
  const run = async (c) => {
    const rec = { name: c.name };
    captured = undefined;
    try {
      const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
      rec.type = res.diagramType;
    } catch (e) {
      const message = String((e && e.message) || e);
      if (message !== SENTINEL || !captured) {
        rec.error = message;
        if (e && e.name) {
          rec.errorName = e.name;
        }
      }
    }
    if (captured && !rec.error) {
      Object.assign(rec, captured);
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return rec;
  };
  return { run, warmUp };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(erChunk);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(erChunk, c.site);
    return await page.run(c);
  } finally {
    frame.remove();
  }
};

const out = [];
for (const c of args.cases) {
  out.push(c.fresh ? await runFresh(c) : await top.run(c));
}
return { cases: out };
