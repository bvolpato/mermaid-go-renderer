// Body of the page function of gen-class-db.mjs. The oracle runs it with the
// esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. The draw function
// of the class renderer is replaced by a function that dumps the db
// (getData, the classes, the relations, the notes, the namespaces, the
// interfaces, the style classes, the direction, the title and the
// accessibility values) and the config, and then stops the render. So the db
// and the config are in the state that the real draw function sees.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page. In the
// shared page the DOMPurify hooks of mermaid are installed before the first
// case, so that the state is the same for every case. The counter of the
// class DOM ids is a variable of the module, so it grows from case to case
// in the shared page: each record has its value before the parse.
const classChunk = internalsWhere('ClassDB').find((f) => f.startsWith('classDiagram'));
const layoutChunk = internalsWhere('getRegisteredLayoutAlgorithm').find((f) => f.startsWith('chunk-'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (classChunk, layoutChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const mod = await import('/chunks/mermaid.esm/' + classChunk);
  const diagram = mod.diagram;
  const { getRegisteredLayoutAlgorithm } = await import('/chunks/mermaid.esm/' + layoutChunk);
  const SENTINEL = '__class_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );
  let captured;
  diagram.renderer.draw = async (text, id, version, diag) => {
    const db = diag.db;
    db.setDiagramId(id);
    const data = db.getData();
    const classes = [...db.getClasses().entries()];
    const rec = {
      type: diag.type,
      nodes: clean(data.nodes),
      edges: clean(data.edges),
      other: clean(data.other),
      config: clean(data.config),
      dataDirection: data.direction,
      layout: getRegisteredLayoutAlgorithm(data.config.layout),
      classes: clean(classes),
      relations: clean(db.getRelations()),
      notes: clean([...db.getNotes().entries()]),
      namespaces: [...db.getNamespaces().entries()].map(([key, ns]) => [
        key,
        clean({
          id: ns.id,
          label: ns.label,
          domId: ns.domId,
          parent: ns.parent,
          explicit: ns.explicit,
          classes: [...ns.classes.keys()],
          notes: [...ns.notes.keys()],
          children: [...ns.children.keys()],
        }),
      ]),
      interfaces: clean(db.interfaces),
      styleClasses: clean([...db.styleClasses.entries()]),
      direction: db.getDirection(),
      title: db.getDiagramTitle(),
      accTitle: db.getAccTitle(),
      accDescription: db.getAccDescription(),
      domIds: classes.map(([key]) => db.lookUpDomId(key)),
      tooltips: clean(classes.map(([key]) => db.getTooltip(key) ?? null)),
      // The functions that bindFunctions runs: the tooltip setup and one for
      // each click callback.
      functions: db.functions.length,
      displayDetails: clean(
        classes.map(([, c]) => [...c.members, ...c.methods].map((m) => ({ ...m.getDisplayDetails(), classifierStyle: m.parseClassifier() })))
      ),
      classConfig: clean(db.getConfig()),
    };
    captured = rec;
    throw new Error(SENTINEL);
  };
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    try {
      await mermaid.render('my-svg', 'classDiagram\n  class Warm', document.getElementById('container'));
    } catch {
      // The replaced draw function stops the render.
    }
  };
  const run = async (c) => {
    const rec = { name: c.name, classCounter: mod.__x_classCounter };
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
const top = await (0, eval)(setupSrc)(classChunk, layoutChunk);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(classChunk, layoutChunk, c.site);
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
