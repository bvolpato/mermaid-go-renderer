// Body of the page function of gen-usecase-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. Every layout is
// replaced by a function that keeps the layout data and then stops the
// render. So the real draw function of the usecase renderer runs up to the
// call of the layout: the dump has the layout data as the layout receives it
// (the nodes and edges of getData after prepareUsecaseLayoutData and the
// DOM id prefix of render, the config, the direction, the spacing, the
// fonts, the markers, the layout algorithm), and the content of the db (the
// actors, the use cases, the system boundaries, the relationships, the
// notes, the JSON nodes, the class definitions, the direction, the title and
// the accessibility values).
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page. In the
// shared page the DOMPurify hooks of mermaid are installed before the first
// case, so that the state is the same for every case.
const usecaseChunk = internalsWhere('prepareUsecaseLayoutData').find((f) => f.startsWith('usecaseDiagram'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (usecaseChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const { diagram } = await import('/chunks/mermaid.esm/' + usecaseChunk);
  const SENTINEL = '__usecase_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );
  // ordered keeps the property order of an object: { $o: [[key, value], ...] }.
  const ordered = (v) => {
    if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
      return { $n: Object.is(v, -0) ? '-0' : String(v) };
    }
    if (v === null || typeof v !== 'object') {
      return v;
    }
    if (Array.isArray(v)) {
      return v.map(ordered);
    }
    return { $o: Object.keys(v).map((k) => [k, ordered(v[k])]) };
  };
  // The model of the db, in the form of the function dumpModel of
  // gen-usecase-parser.mjs.
  const dumpModel = (db) => ({
    actors: [...db.getActors()].map(([key, a]) => [
      key,
      a.id,
      a.label,
      a.labelType,
      a.type,
      a.icon ?? null,
      a.business,
      a.stereotype ?? null,
      a.parentId ?? null,
      a.classes,
      a.styles,
    ]),
    useCases: [...db.getUseCases()].map(([key, u]) => [
      key,
      u.id,
      u.label,
      u.labelType,
      u.shape,
      u.business,
      u.stereotype ?? null,
      u.parentId ?? null,
      u.classes,
      u.styles,
    ]),
    systemBoundaries: [...db.getSystemBoundaries()].map(([key, b]) => [key, b.id, b.label, b.labelType, b.type, b.members, b.classes, b.styles]),
    relationships: db.getRelationships().map((r) => [
      r.id,
      r.explicitId,
      r.source,
      r.target,
      r.type,
      r.arrowType,
      r.label ?? null,
      r.labelType ?? null,
      r.minlen,
      r.classes,
      r.styles,
      r.animate,
      r.animation ?? null,
    ]),
    notes: [...db.getNotes()].map(([key, n]) => [key, n.id, n.target, n.label, n.labelType]),
    jsonNodes: [...db.getJsonNodes()].map(([key, j]) => [key, j.id, ordered(j.value), ordered(j.propertyOrder), j.classes, j.styles]),
    classDefs: [...db.getClassDefs()].map(([key, c]) => [key, c.id, c.styles]),
    direction: db.getDirection(),
  });

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

  const realDraw = diagram.renderer.draw;
  let captured;
  diagram.renderer = {
    draw: async (text, id, version, diag) => {
      const db = diag.db;
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
      // The properties of the layout data other than the nodes, the edges
      // and the config.
      const other = {};
      for (const key of Object.keys(data)) {
        if (key !== 'nodes' && key !== 'edges' && key !== 'config') {
          other[key] = data[key];
        }
      }
      captured = {
        type: diag.type,
        nodes: clean(data.nodes),
        edges: clean(data.edges),
        config: clean(data.config),
        data: clean(other),
        // The names of the properties of the layout data, also of those
        // that are undefined.
        dataKeys: Object.keys(data),
        model: dumpModel(db),
        title: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
        usecaseConfig: clean(db.getConfig()),
      };
      throw new Error(SENTINEL);
    },
  };
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    try {
      await mermaid.render('my-svg', 'usecase-beta\n  accTitle: warm\n  actor Warm\n', document.getElementById('container'));
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
const top = await (0, eval)(setupSrc)(usecaseChunk);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(usecaseChunk, c.site);
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
