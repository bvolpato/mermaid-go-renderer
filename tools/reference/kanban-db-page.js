// Body of the page function of gen-kanban-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render. The draw function
// of the kanban renderer is replaced by a function that keeps the state of
// the db and then stops the render. So the real parser and the real db run
// inside a real render, with the config of the front matter and of the
// directives. The dump has:
//
//   nodes     the module-level array `nodes` of kanbanDb.ts,
//   sections  the index in `nodes` of each element of getSections(),
//   cnt       the module-level counter,
//   data      the result of getData(): what the renderer receives,
//   config    the config values that the db and the renderer read.
//
// A case that mermaid rejects has the error message and the error name.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page.
const kanbanChunk = internalsWhere('kanbanDb_default').find((f) => f.startsWith('kanban-definition'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (kanbanChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const mod = await import('/chunks/mermaid.esm/' + kanbanChunk);
  const diagram = mod.diagram;
  const SENTINEL = '__kanban_db_dump__';
  // JSON drops undefined and functions. A number that is not finite is kept
  // as { $num: "NaN" }.
  const clean = (v) =>
    JSON.parse(
      JSON.stringify(v, (k, x) => (typeof x === 'number' && !Number.isFinite(x) ? { $num: String(x) } : x)) ?? 'null'
    );

  let captured;
  diagram.renderer = {
    draw: async (text, id, version, diag) => {
      const nodes = mod.__x_nodes;
      captured = {
        nodes: clean(nodes),
        sections: diag.db.getSections().map((s) => nodes.indexOf(s)),
        cnt: mod.__x_cnt,
      };
      const data = diag.db.getData();
      captured.data = {
        nodes: clean(data.nodes),
        edges: clean(data.edges),
        other: clean(data.other),
        keys: Object.keys(data),
      };
      const config = data.config;
      captured.config = clean({
        look: config.look,
        theme: config.theme,
        htmlLabels: config.htmlLabels,
        securityLevel: config.securityLevel,
        kanban: config.kanban,
        mindmap: config.mindmap,
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
        rec.type = 'kanban';
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
const top = await (0, eval)(setupSrc)(kanbanChunk);

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(kanbanChunk, c.site);
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
