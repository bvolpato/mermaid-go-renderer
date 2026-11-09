// Body of the page function of gen-flowchart-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }], links: [[str, startStr]] }
//
// For each case the script calls the real mermaid.render. Every layout
// algorithm is replaced by one that dumps the layout data that the real
// draw function of the flowchart renderer built (the nodes and edges of
// getData, the layout properties), the db (the vertices, the edges, the
// subgraphs, the classes, the tooltips, the direction, the title, the
// accessibility values, the click callbacks), the config and a checksum of
// the text of the style element, and then stops the render. So the dump is
// what a layout algorithm of upstream receives.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page.
// In the shared page the DOMPurify hooks of mermaid are installed before the
// first case, so that the state is the same for every case.
const flowChunk = internalsWhere('createFlowDiagram').find((f) => f.startsWith('chunk-'));
const layoutChunk = internalsWhere('getRegisteredLayoutAlgorithm').find((f) => f.startsWith('chunk-'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (flowChunk, layoutChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const { diagram } = await import('/chunks/mermaid.esm/' + flowChunk);
  const { getRegisteredLayoutAlgorithm, registerLayoutLoaders } = await import('/chunks/mermaid.esm/' + layoutChunk);
  const SENTINEL = '__flowchart_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );
  // textHash is a checksum of a text: its length and two 32-bit hashes of
  // its UTF-16 code units. The Go test computes the same.
  const textHash = (s) => {
    let h1 = 0x811c9dc5;
    let h2 = 0x9e3779b9;
    for (let i = 0; i < s.length; i++) {
      const u = s.charCodeAt(i);
      h1 = Math.imul(h1 ^ u, 16777619);
      h2 = Math.imul(h2 + u, 0x85ebca6b) ^ (h2 >>> 13);
    }
    return `${s.length}:${(h1 >>> 0).toString(16)}:${(h2 >>> 0).toString(16)}`;
  };
  let captured;
  // The real draw function runs. This wrapper only remembers the diagram.
  let current;
  const realDraw = diagram.renderer.draw;
  diagram.renderer.draw = (text, id, version, diag) => {
    current = { id, diag };
    return realDraw(text, id, version, diag);
  };
  const dump = (data) => {
    const { id, diag } = current;
    const db = diag.db;
    // The style element of the diagram is in the svg before the draw function runs.
    const style = document.querySelector(`[id="${id}"] style`);
    const edges = db.getEdges();
    const subGraphs = db.getSubGraphs();
    const rec = {
      type: diag.type,
      nodes: clean(data.nodes),
      edges: clean(data.edges),
      other: clean(data.other),
      config: clean(data.config),
      // The properties that the draw function adds to the layout data.
      layoutData: clean({
        type: data.type,
        layoutAlgorithm: data.layoutAlgorithm,
        direction: data.direction,
        nodeSpacing: data.nodeSpacing,
        rankSpacing: data.rankSpacing,
        markers: data.markers,
        diagramId: data.diagramId,
      }),
      classes: clean([...db.getClasses().entries()]),
      subGraphs: clean(subGraphs),
      vertices: clean([...db.getVertices().entries()]),
      rawEdges: clean([...edges]),
      tooltips: clean([...db.tooltips.entries()]),
      direction: db.getDirection(),
      title: db.getDiagramTitle(),
      accTitle: db.getAccTitle(),
      accDescription: db.getAccDescription(),
      domIds: [...db.getVertices().keys()].map((k) => db.lookUpDomId(k)),
      styleHash: style ? textHash(style.textContent) : null,
    };
    if (edges.defaultStyle !== undefined) {
      rec.defaultStyle = clean(edges.defaultStyle);
    }
    if (edges.defaultInterpolate !== undefined) {
      rec.defaultInterpolate = clean(edges.defaultInterpolate);
    }
    rec.getTooltip = clean(data.nodes.map((n) => db.getTooltip(n.id) ?? null));
    // The click callbacks are closures of the db. To see them, bind them to
    // elements with the DOM ids of the nodes, click each element, and record
    // the calls of the functions that the cases name.
    const withCallback = [...db.getVertices().values()].filter((v) => v.haveCallback);
    if (withCallback.length > 0) {
      const calls = [];
      for (const fn of ['callback', 'noargs', 'two', 'cb', 'other']) {
        window[fn] = (...a) => calls.push({ fn, args: a });
      }
      window.a = { b: { c: (...a) => calls.push({ fn: 'a.b.c', args: a }) } };
      const holder = document.createElement('div');
      for (const v of withCallback) {
        const el = document.createElement('div');
        el.id = db.lookUpDomId(v.id);
        el.dataset.vertex = v.id;
        holder.appendChild(el);
      }
      document.body.appendChild(holder);
      db.bindFunctions(holder);
      rec.callbacks = [];
      for (const el of holder.children) {
        calls.length = 0;
        el.click();
        rec.callbacks.push({ id: el.dataset.vertex, calls: clean(calls) });
      }
      holder.remove();
      document.querySelectorAll('.mermaidTooltip').forEach((e) => e.remove());
    }
    db.indexNodes();
    rec.depthFirstPos = clean(subGraphs.map((_, i) => db.getDepthFirstPos(i) ?? null));
    return rec;
  };
  // Every layout of the full build of mermaid becomes the layout that dumps.
  const layoutNames = ['dagre', 'swimlane', 'cose-bilkent', 'elk', 'elk.stress', 'elk.force', 'elk.mrtree', 'elk.sporeOverlap', 'elk.box', 'elk.rectpacking'];
  for (const name of layoutNames) {
    // Fails when the build does not have the layout.
    if (getRegisteredLayoutAlgorithm(name) !== name) {
      throw new Error(`the layout ${name} is not registered`);
    }
  }
  registerLayoutLoaders(
    layoutNames.map((name) => ({
      name,
      loader: async () => ({
        render: async (data4Layout) => {
          captured = dump(data4Layout);
          throw new Error(SENTINEL);
        },
      }),
    }))
  );
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    try {
      await mermaid.render('my-svg', 'flowchart TD\n  warm[up]', document.getElementById('container'));
    } catch {
      // The layout that dumps stops the render.
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
  // destructLink is a pure function of the db.
  const links = (list) => {
    const db = diagram.db;
    return list.map(([str, startStr]) => {
      try {
        return clean(db.destructLink(str, startStr));
      } catch (e) {
        return { error: String((e && e.message) || e) };
      }
    });
  };
  return { run, warmUp, links, defaultStyle: diagram.db.defaultStyle() };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(flowChunk, layoutChunk);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(flowChunk, layoutChunk, c.site);
    return await page.run(c);
  } finally {
    frame.remove();
  }
};

const out = [];
for (const c of args.cases) {
  out.push(c.fresh ? await runFresh(c) : await top.run(c));
}
return {
  cases: out,
  links: top.links(args.links || []),
  defaultStyle: top.defaultStyle,
};
