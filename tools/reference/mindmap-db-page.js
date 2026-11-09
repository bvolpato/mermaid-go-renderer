// Body of the page function of gen-mindmap-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. Every layout is
// replaced by a function that keeps the layout data and then stops the
// render. So the real parser, the real db and the real draw function of the
// mindmap renderer run up to the call of the layout, and the dump has the
// layout data as the layout receives it: the nodes and edges of getData with
// the changes of the renderer, the config, the node tree (rootNode, with the
// sections), the direction, the markers, the spacing, the shapes, the layout
// algorithm and the diagram id.
//
// A mindmap without a node draws nothing and calls no layout: the dump then
// has `empty`. A case that mermaid rejects has the error message.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page.
const mindmapChunk = internalsWhere('MindmapDB').find((f) => f.startsWith('mindmap-definition'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (mindmapChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const { diagram } = await import('/chunks/mermaid.esm/' + mindmapChunk);
  const SENTINEL = '__mindmap_db_dump__';
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

  const realDraw = diagram.renderer.draw;
  let captured;
  diagram.renderer = {
    draw: async (text, id, version, diag) => {
      layoutData = undefined;
      let sentinel = false;
      try {
        await realDraw(text, id, version, diag);
      } catch (e) {
        if (String((e && e.message) || e) !== SENTINEL || !layoutData) {
          throw e;
        }
        sentinel = true;
      }
      if (!sentinel) {
        // The draw function returned: the mindmap has no node.
        const data = diag.db.getData();
        captured = {
          empty: true,
          nodes: clean(data.nodes),
          edges: clean(data.edges),
          config: clean(data.config),
          dataKeys: Object.keys(data),
        };
        return;
      }
      const data = layoutData;
      captured = {
        nodes: clean(data.nodes),
        edges: clean(data.edges),
        config: clean(data.config),
        rootNode: clean(data.rootNode),
        direction: data.direction,
        dataType: data.type,
        layout: data.layoutAlgorithm,
        markers: clean(data.markers),
        nodeSpacing: clean(data.nodeSpacing),
        rankSpacing: clean(data.rankSpacing),
        shapes: clean(data.shapes),
        diagramId: data.diagramId,
        // The properties of the layout data, by name.
        dataKeys: Object.keys(data),
      };
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
        rec.type = 'mindmap';
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
const top = await (0, eval)(setupSrc)(mindmapChunk);

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(mindmapChunk, c.site);
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
