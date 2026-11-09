// Body of the page function of gen-railroad-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site }] }
//
// For each case the script calls the real mermaid.render, so the config of
// the front matter and of the directives applies, and then reads the db of
// the railroad diagrams (the module state of railroadDb.ts, which the four
// diagram types share): the title, the accessibility texts, the rules in
// their order, and the rule map as [name, index of the rule in the rules].
// The renderer does not change the db, so this is what the db holds after
// the parse. A case that mermaid rejects has the error message.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page.
const railroadChunk = internalsWhere('RailroadRenderer')[0];

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (railroadChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const mod = await import('/chunks/mermaid.esm/' + railroadChunk);
  const db = mod.db;
  const ruleMap = mod.__x_ruleMap;

  // encode keeps what JSON.stringify loses: undefined is {"$u":1}, and NaN,
  // the infinities and -0 are {"$n":"<text>"}.
  const encode = (v) => {
    if (v === undefined) return { $u: 1 };
    if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
      return { $n: Object.is(v, -0) ? '-0' : String(v) };
    }
    if (v === null || typeof v !== 'object') return v;
    if (Array.isArray(v)) return Array.from(v, encode);
    const o = {};
    for (const k of Object.keys(v)) {
      if (typeof v[k] !== 'function') o[k] = encode(v[k]);
    }
    return o;
  };

  const run = async (c) => {
    const rec = { name: c.name };
    try {
      const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
      rec.type = res.diagramType;
    } catch (e) {
      rec.error = String((e && e.message) || e);
    }
    if (rec.error === undefined && /^railroad/.test(rec.type)) {
      const rules = db.getRules();
      rec.db = encode({
        title: db.getTitle(),
        diagramTitle: db.getDiagramTitle(),
        accTitle: db.getAccTitle(),
        accDescription: db.getAccDescription(),
        rules,
        ruleMap: [...ruleMap.keys()].map((name) => [name, rules.indexOf(db.getRule(name))]),
      });
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return rec;
  };
  return { run };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(railroadChunk);

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(railroadChunk, c.site);
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
