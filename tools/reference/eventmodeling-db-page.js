// Body of the page function of gen-eventmodeling-db.mjs. The oracle runs it
// with the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. The draw function
// of the eventmodeling renderer is replaced by a function that keeps what
// the db holds and then stops the render. So the real parser and the real
// db run inside a real render, with the config of the front matter and of
// the directives. The dump has the result of getDiagramProps() and of
// getState(): the swimlanes, the boxes and the relations, in their order,
// and the title and accessibility texts. An object that another object
// refers to is given by a number: a swimlane by its index, a box by its
// position in the boxes, a frame by its position in the frames.
//
// A case that mermaid rejects has the error message and the error name.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page.
const chunk = internalsWhere('evolveFramePositioned')[0];

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (chunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const mod = await import('/chunks/mermaid.esm/' + chunk);
  const diagram = mod.diagram;
  const SENTINEL = '__eventmodeling_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );

  let captured;
  diagram.renderer = {
    draw: (text, id, version, diag) => {
      const db = diag.db;
      const state = db.getState();
      const frames = mod.__x_store.ast.frames;
      const swimlane = (s) => ({ index: s.index, label: s.label, namespace: s.namespace, r: s.r, y: s.y, height: s.height, maxHeight: s.maxHeight });
      captured = clean({
        props: db.getDiagramProps(),
        stateKeys: Object.keys(state),
        maxR: state.maxR,
        previousSwimlaneNumber: state.previousSwimlaneNumber,
        previousFrame: state.previousFrame === undefined ? undefined : frames.indexOf(state.previousFrame),
        swimlaneKeys: Object.keys(state.swimlanes),
        swimlanes: Object.values(state.swimlanes).map(swimlane),
        sortedSwimlanes: state.sortedSwimlanesArray.map((s) => s.index),
        boxes: state.boxes.map((b) => ({
          r: b.r,
          x: b.x,
          y: b.y,
          dimension: b.dimension,
          leftSibling: b.leftSibling,
          swimlane: b.swimlane.index,
          visual: b.visual,
          text: b.text,
          frame: frames.indexOf(b.frame),
          index: b.index,
        })),
        relations: state.relations.map((r) => ({
          visual: r.visual,
          source: r.source,
          target: r.target,
          sourceBox: state.boxes.indexOf(r.sourceBox),
          targetBox: state.boxes.indexOf(r.targetBox),
        })),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
        diagramTitle: db.getDiagramTitle(),
      });
      throw new Error(SENTINEL);
    },
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
      } else {
        rec.type = 'eventmodeling';
      }
    }
    if (captured && !rec.error) {
      Object.assign(rec, captured);
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return rec;
  };
  return { run };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(chunk);

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(chunk, c.site);
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
