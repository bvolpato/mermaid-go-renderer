// Body of the page function of gen-state-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site, api }], seed }
//
// For each case the script calls the real mermaid.render. Two functions of
// the state renderer are replaced:
//   - getClasses, which mermaid calls between the parse and the draw, dumps
//     the db as the parser left it and then runs the real function,
//   - draw runs the first steps of the real draw function (extract, getData,
//     the layout settings), dumps the db and the layout data, and then stops
//     the render.
// So the db and the config are in the states that the real render sees.
//
// A dump is a JSON text, so that the order of the properties is kept.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page, where
// the DOMPurify hooks of mermaid are installed before the first case.
//
// A case with `api` tests the functions of the db that the grammar does not
// reach. `api` is { version, ops }: the draw function creates a db with
// new StateDB(version) and calls a function for each op [name, ...args],
// where a null argument stands for undefined. The case returns the result
// or the error of each call, each with the state of the db after the call.
//
// Math.random is the seeded generator of the oracle. Before each render it
// is reset and makes one draw, as the bundle of the reference page does
// before a render. The counter of generateId is read before each case: the
// case returns it as idCounter.
const stateChunk = internalsWhere('StateDB').find((f) => f.startsWith('stateDiagram-v2'));
const layoutChunk = internalsWhere('getRegisteredLayoutAlgorithm').find((f) => f.startsWith('chunk-'));
const utilsChunk = internalsWhere('generateId').find((f) => f.startsWith('chunk-'));

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (stateChunk, layoutChunk, utilsChunk, seed, site) {
  if (!globalThis.__mmdgResetRandom) {
    let a = seed >>> 0;
    globalThis.__mmdgResetRandom = () => {
      a = seed >>> 0;
    };
    Math.random = function () {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  }
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const stateModule = await import('/chunks/mermaid.esm/' + stateChunk);
  const { diagram } = stateModule;
  const StateDB = stateModule.StateDB ?? stateModule.__x_StateDB;
  const stateDomId = stateModule.stateDomId ?? stateModule.__x_stateDomId;
  const layoutModule = await import('/chunks/mermaid.esm/' + layoutChunk);
  const getRegisteredLayoutAlgorithm = layoutModule.getRegisteredLayoutAlgorithm ?? layoutModule.__x_getRegisteredLayoutAlgorithm;
  const utilsModule = await import('/chunks/mermaid.esm/' + utilsChunk);
  const generateId = utilsModule.generateId ?? utilsModule.__x_generateId;
  const getConfig = () => mermaid.mermaidAPI.getConfig();
  const SENTINEL = '__state_db_dump__';

  // snapshot is the state of the db as a JSON text.
  const snapshot = (db) =>
    JSON.stringify({
      rootDoc: db.rootDoc,
      states: [...db.getStates().entries()],
      relations: db.getRelations(),
      classes: [...db.getClasses().entries()],
      links: [...db.getLinks().entries()],
      direction: db.getDirection(),
      title: db.getDiagramTitle(),
      accTitle: db.getAccTitle(),
      accDescription: db.getAccDescription(),
      dividerCnt: db.dividerCnt,
      startEndCount: db.startEndCount,
    });

  // runApi runs the calls of an api case on a new db.
  const runApi = (api) => {
    const db = new StateDB(api.version);
    const results = [];
    for (const [op, ...opArgs] of api.ops) {
      let rec;
      try {
        const a = opArgs.map((x) => (x === null ? undefined : x));
        let r = op === 'stateDomId' ? stateDomId(...a) : db[op](...a);
        if (op === 'getData') {
          r = { nodes: r.nodes, edges: r.edges, direction: r.direction };
        } else if (r instanceof Map) {
          r = [...r.entries()];
        }
        rec = { value: r === undefined ? null : JSON.parse(JSON.stringify(r)) };
      } catch (e) {
        rec = { error: String((e && e.message) || e) };
      }
      rec.snapshot = JSON.parse(snapshot(db));
      results.push(rec);
    }
    return results;
  };

  let parsed;
  let captured;
  let current;
  const realGetClasses = diagram.renderer.getClasses;
  diagram.renderer.getClasses = (text, diag) => {
    parsed = snapshot(diag.db);
    return realGetClasses(text, diag);
  };
  diagram.renderer.draw = async (text, id, version, diag) => {
    if (current.api) {
      captured = JSON.stringify({ type: diag.type, apiResults: runApi(current.api) });
      throw new Error(SENTINEL);
    }
    const db = diag.db;
    const { state: conf, layout } = getConfig();
    db.extract(db.getRootDocV2());
    const data = db.getData();
    captured = JSON.stringify({
      type: diag.type,
      nodes: data.nodes,
      edges: data.edges,
      other: data.other,
      config: data.config,
      dataDirection: data.direction,
      layout: getRegisteredLayoutAlgorithm(layout),
      nodeSpacing: conf?.nodeSpacing || 50,
      rankSpacing: conf?.rankSpacing || 50,
      markers: getConfig().look === 'neo' ? ['barbNeo'] : ['barb'],
      stateConfig: db.getConfig() ?? null,
      drawn: JSON.parse(snapshot(db)),
    });
    throw new Error(SENTINEL);
  };
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    try {
      await mermaid.render('my-svg', 'stateDiagram-v2\n  warm --> up: label', document.getElementById('container'));
    } catch {
      // The replaced draw function stops the render.
    }
  };
  const run = async (c) => {
    const rec = { name: c.name };
    captured = undefined;
    parsed = undefined;
    current = c;
    const probe = generateId();
    rec.idCounter = Number(probe.slice(probe.lastIndexOf('-') + 1));
    globalThis.__mmdgResetRandom();
    Math.random();
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
    if (parsed !== undefined) {
      rec.parsed = JSON.parse(parsed);
    }
    if (captured !== undefined && !rec.error) {
      Object.assign(rec, JSON.parse(captured));
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return JSON.stringify(rec);
  };
  return { run, warmUp };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(stateChunk, layoutChunk, utilsChunk, args.seed);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(stateChunk, layoutChunk, utilsChunk, args.seed, c.site);
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
