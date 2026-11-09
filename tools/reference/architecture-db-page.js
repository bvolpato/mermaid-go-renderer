// Body of the page function of gen-architecture-db.mjs. The oracle runs it
// with the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, site, full }] }
//
// For each case the script calls the real mermaid.render. The draw function
// of the architecture renderer is wrapped: the wrapper copies what the db
// holds when the draw starts (the real parser and the real db filled it),
// and then
//   - for a case with `full`, it runs the real draw function and copies the
//     size that drawServices writes to each service, or the error of the
//     draw;
//   - for the other cases, it stops the render.
//
// The dump of the db has
//   title, accTitle, accDescr   the values of the common db
//   nodes       getNodes(): each node with the indexes of its edges in
//               getEdges() (a node holds the same edge objects)
//   services    the ids of getServices(), junctions the ids of getJunctions()
//   groups      getGroups()
//   edges       getEdges()
//   layoutHints getLayoutHints()
//   registeredIds  the entries of the map
//   dataStructures getDataStructures(): each Map as the list of its entries
//   config      getConfig()
// A property that is undefined is absent. A case that mermaid rejects has
// the error message.
//
// `site` is the site config, the argument of mermaid.initialize.
const chunk = internalsWhere('ArchitectureDB').find((f) => f.startsWith('architectureDiagram'));
const { diagram } = await import('/chunks/mermaid.esm/' + chunk);
const SENTINEL = '__architecture_db_dump__';

const entries = (map) => [...map.entries()];
const dumpDb = (db) => {
  const edges = db.getEdges();
  const out = {
    title: db.getDiagramTitle(),
    accTitle: db.getAccTitle(),
    accDescr: db.getAccDescription(),
    nodes: db.getNodes().map((n) => ({ ...n, edges: n.edges.map((e) => edges.indexOf(e)) })),
    services: db.getServices().map((n) => n.id),
    junctions: db.getJunctions().map((n) => n.id),
    groups: db.getGroups(),
    edges,
    layoutHints: db.getLayoutHints(),
    registeredIds: entries(db.registeredIds),
    config: db.getConfig(),
  };
  try {
    const ds = db.getDataStructures();
    out.dataStructures = {
      adjList: entries(ds.adjList).map(([id, m]) => [id, entries(m)]),
      spatialMaps: ds.spatialMaps.map(entries),
      groupAlignments: entries(ds.groupAlignments),
    };
  } catch (e) {
    out.dataStructuresError = String((e && e.message) || e);
  }
  // A copy: the real draw changes the services.
  return JSON.parse(JSON.stringify(out));
};

const realDraw = diagram.renderer.draw;
let current;
diagram.renderer = {
  draw: async (text, id, version, diag) => {
    const db = diag.db;
    current.db = dumpDb(db);
    if (!current.full) {
      throw new Error(SENTINEL);
    }
    try {
      await realDraw(text, id, version, diag);
    } finally {
      current.sizes = db.getServices().map((s) => [s.id, s.width ?? null, s.height ?? null]);
    }
  },
};

const out = [];
for (const c of args.cases) {
  const rec = { name: c.name };
  current = { full: !!c.full };
  mermaid.initialize({ startOnLoad: false, ...(c.site || {}) });
  globalThis.__mmdgResetRandom();
  Math.random();
  try {
    const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
    rec.type = res.diagramType;
  } catch (e) {
    const message = String((e && e.message) || e);
    if (message === SENTINEL && current.db) {
      rec.type = 'architecture';
    } else if (current.db) {
      // The draw failed.
      rec.type = 'architecture';
      rec.drawError = message;
      if (e && e.name) {
        rec.drawErrorName = e.name;
      }
    } else {
      rec.error = message;
      if (e && e.name) {
        rec.errorName = e.name;
      }
    }
  }
  if (current.db) {
    rec.db = current.db;
  }
  if (current.sizes) {
    rec.sizes = current.sizes;
  }
  document.querySelectorAll('#container > *, body > :not(#container)').forEach((el) => el.remove());
  out.push(rec);
}
return { cases: out };
