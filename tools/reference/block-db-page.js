// Body of the page function of gen-block-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   { cases: [{ name, text, fresh, site, svg }], seed }
//
// For each case the script calls the real mermaid.render. Three functions
// are wrapped, and each wrapper runs the real function:
//   - renderer.getClasses, which mermaid calls between the parse and the
//     draw, dumps the db as the parser left it, with the config,
//   - db.getBlock dumps the block sizes at its first call for the id "root"
//     in a draw. That call is the first statement of layout(), so the sizes
//     are the ones that calculateBlockSizes measured,
//   - renderer.draw dumps the block sizes and the bounds after the draw.
//     insertBlocks and insertEdges do not change them, so they are the
//     result of layout().
//
// A dump is a JSON text, so that the order of the properties is kept. A
// number that JSON cannot hold (NaN, an infinity, -0) is {"$n": "<text>"}.
// The key of a block whose id is undefined is {"$undefined": true}.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page, where
// the DOMPurify hooks of mermaid are installed before the first case.
//
// Math.random is the seeded generator of the oracle. Before each render it
// is reset and makes one draw, as the bundle of the reference page does
// before a render. The counter of generateId of blockDB.ts is read before
// each case: the case returns it as idCounter.
const blockChunk = internalsWhere('populateBlockDatabase')[0];

// setup runs in the window of a page. It returns the function that runs one case.
const setup = async function (blockChunk, seed, site) {
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
  const mod = await import('/chunks/mermaid.esm/' + blockChunk);
  const diagram = mod.diagram;
  const db = diagram.db;
  const getConfig = () => mermaid.mermaidAPI.getConfig();

  const replacer = function (key, value) {
    if (typeof value === 'number') {
      if (!Number.isFinite(value)) {
        return { $n: String(value) };
      }
      if (Object.is(value, -0)) {
        return { $n: '-0' };
      }
      return value;
    }
    // size.node is the d3 selection of the measured element.
    if (key === 'node' && value && typeof value === 'object' && '_groups' in value) {
      return undefined;
    }
    return value;
  };
  const keyOf = (k) => (k === undefined ? { $undefined: true } : k);
  // The block database is a module-level Map that clear() replaces: read
  // the live binding each time.
  const database = () => mod.__x_blockDatabase;

  // treeRefs lists, for the blocks of the hierarchy in pre-order, the index
  // of the same object in the flat list, or -1.
  const treeRefs = (blocks, flat, out) => {
    for (const b of blocks) {
      out.push(flat.indexOf(b));
      if (b && Array.isArray(b.children)) {
        treeRefs(b.children, flat, out);
      }
    }
    return out;
  };

  // snapshot is the state of the db as a JSON text.
  const snapshot = () => {
    const entries = [...database().entries()];
    const flat = entries.map(([, v]) => v);
    return JSON.stringify(
      {
        blocks: db.getBlocks(),
        flat: entries.map(([k, v]) => [keyOf(k), v]),
        treeRefs: treeRefs(db.getBlocks(), flat, []),
        columns: entries.map(([k]) => db.getColumns(k)),
        edges: db.getEdges(),
        classes: [...db.getClasses().entries()],
        // The block db has no title and no accessibility functions.
        dbFunctions: ['getAccTitle', 'getAccDescription', 'getDiagramTitle', 'setDiagramTitle', 'setAccTitle', 'setAccDescription'].filter(
          (f) => typeof db[f] === 'function'
        ),
        blockConfig: db.getConfig() ?? null,
        config: getConfig(),
      },
      replacer
    );
  };

  // sizes is the size of every block of the flat list, as a JSON text.
  const sizes = () =>
    JSON.stringify(
      [...database().values()].map((b) => ({
        size: b.size ? { width: b.size.width, height: b.size.height, x: b.size.x, y: b.size.y } : null,
        intersect: typeof b.intersect === 'function',
      })),
      replacer
    );

  let parsed;
  let measured;
  let drawn;
  let inDraw = false;
  const realGetClasses = diagram.renderer.getClasses;
  diagram.renderer.getClasses = (text, diag) => {
    parsed = snapshot();
    return realGetClasses(text, diag);
  };
  const realGetBlock = db.getBlock;
  db.getBlock = (id) => {
    if (inDraw && measured === undefined && id === 'root') {
      measured = sizes();
    }
    return realGetBlock(id);
  };
  const realDraw = diagram.renderer.draw;
  diagram.renderer.draw = async (text, id, version, diag) => {
    inDraw = true;
    try {
      await realDraw(text, id, version, diag);
    } finally {
      inDraw = false;
      const root = realGetBlock('root');
      let bounds = null;
      if (root && measured !== undefined) {
        try {
          const { minX, minY, maxX, maxY } = mod.__x_findBounds(root);
          bounds = { x: minX, y: minY, width: maxX - minX, height: maxY - minY };
        } catch {
          bounds = null;
        }
      }
      drawn = JSON.stringify({ sizes: JSON.parse(sizes()), bounds }, replacer);
    }
  };
  // Install the DOMPurify hooks of mermaid.
  const warmUp = async () => {
    await mermaid.render('my-svg', 'block\n  warm["<b>up</b>"]', document.getElementById('container'));
  };
  const run = async (c) => {
    const rec = { name: c.name };
    parsed = undefined;
    measured = undefined;
    drawn = undefined;
    rec.idCounter = mod.__x_cnt;
    globalThis.__mmdgResetRandom();
    Math.random();
    try {
      const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
      rec.type = res.diagramType;
      if (c.svg) {
        rec.svg = res.svg;
      }
    } catch (e) {
      rec.error = String((e && e.message) || e);
      if (e && e.name) {
        rec.errorName = e.name;
      }
    }
    if (parsed !== undefined) {
      rec.parsed = JSON.parse(parsed);
    }
    if (measured !== undefined) {
      rec.measured = JSON.parse(measured);
    }
    if (drawn !== undefined && measured !== undefined) {
      rec.drawn = JSON.parse(drawn);
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return JSON.stringify(rec);
  };
  return { run, warmUp };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(blockChunk, args.seed);
await top.warmUp();

const runFresh = async (c) => {
  const frame = document.createElement('iframe');
  // The viewport of the reference page.
  frame.style.cssText = 'width: 800px; height: 600px; border: 0;';
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(blockChunk, args.seed, c.site);
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
