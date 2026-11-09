// Body of the page function of gen-agentflow-db.mjs. The oracle runs it with
// the esm-eval command and `args` in scope:
//
//   {
//     cases: [{ name, text, fresh, site }],
//     api: [{ name, ops: [[method, ...args]] }],
//     links: [[str, startStr]],
//     commas: [body],
//     proto: [jsonText],
//     shapes: [{ shape, isGroup }],
//   }
//
// For each case the script calls the real mermaid.render. Every layout
// algorithm is replaced by one that dumps the layout data that the real
// draw function of the agentflow renderer built (the nodes and edges of
// getData, the connectors, the layout properties), the db (the vertices,
// the edges, the containers, the classes, the tooltips, the direction, the
// title, the accessibility values, the click callbacks, the diagnostics,
// the element mappings, the semantic model), the config and a checksum of
// the text of the style element, and then stops the render. So the dump is
// what a layout algorithm of upstream receives.
//
// A case with `fresh` runs in a new iframe: a page that has made no other
// render, as in the reference renders. Its `site` is the site config, the
// argument of mermaid.initialize. The other cases share the top page. In
// the shared page the DOMPurify hooks of mermaid are installed before the
// first case, so that the state is the same for every case.
//
// An api case calls the methods of a new AgentFlowDB directly, for the
// branches that the grammar does not reach. The other lists are arguments
// for functions of the agentflow chunk that take plain values.
const afChunk = internalsWhere('AgentFlowDB')[0];
const layoutChunk = internalsWhere('getRegisteredLayoutAlgorithm').find((f) => f.startsWith('chunk-'));

// setup runs in the window of a page. It returns the functions that run the cases.
const setup = async function (afChunk, layoutChunk, site) {
  if (!globalThis.mermaid) {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
  }
  const mermaid = globalThis.mermaid;
  mermaid.initialize({ startOnLoad: false, ...(site || {}) });
  const mod = await import('/chunks/mermaid.esm/' + afChunk);
  const diagram = mod.diagram;
  const { getRegisteredLayoutAlgorithm, registerLayoutLoaders } = await import('/chunks/mermaid.esm/' + layoutChunk);
  const SENTINEL = '__agentflow_db_dump__';
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
  const errorRecord = (e) => {
    const rec = { error: String((e && e.message) || e) };
    if (e && e.name) {
      rec.errorName = e.name;
    }
    if (e && e.hash !== undefined) {
      rec.hash = clean(e.hash);
    }
    if (e && e.mark && typeof e.mark === 'object') {
      rec.mark = clean({ line: e.mark.line, column: e.mark.column });
    }
    return rec;
  };
  const tag = (m) => (m ? `${m.id}|${m.type}` : null);
  // dbState is the content of a db, without the layout data.
  const dbState = (db) => {
    const edges = db.getEdges();
    const subGraphs = db.getSubGraphs();
    const rec = {
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
      connectors: clean(db.getConnectors()),
      globalNodes: [...db.globalNodes],
      tools: db.getTools().map((v) => v.id),
      diagnostics: clean(db.getDiagnostics()),
      mappings: clean(db.getElementMappings()),
      mappingStats: clean(db.getMappingStats()),
      semanticModel: clean(db.getSemanticModel()),
    };
    if (edges.defaultStyle !== undefined) {
      rec.defaultStyle = clean(edges.defaultStyle);
    }
    if (edges.defaultInterpolate !== undefined) {
      rec.defaultInterpolate = clean(edges.defaultInterpolate);
    }
    // The queries of the element mappings, for the positions of the mappings.
    const mappings = db.getElementMappings();
    const lines = new Set();
    for (const m of mappings) {
      lines.add(m.position.startLine);
      lines.add(m.position.endLine);
      lines.add(m.position.endLine + 1);
    }
    rec.onLine = [...lines]
      .sort((a, b) => a - b)
      .slice(0, 16)
      .map((l) => [l, db.getElementsOnLine(l).map(tag)]);
    rec.atPosition = mappings
      .slice(0, 24)
      .flatMap((m) => [
        [m.position.startLine, m.position.startColumn],
        [m.position.endLine, m.position.endColumn],
        [m.position.startLine, m.position.endColumn],
        [m.position.endLine, m.position.startColumn + 1],
      ])
      .map(([l, c]) => [l, c, tag(db.getElementAtPosition(l, c))]);
    rec.byId = [...new Set(mappings.map((m) => m.id))].slice(0, 24).map((id) => clean(db.getElementById(id) ?? null));
    db.indexNodes();
    rec.depthFirstPos = clean(subGraphs.map((_, i) => db.getDepthFirstPos(i) ?? null));
    return rec;
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
    if (!current) {
      // Another diagram type drew: the text is not an agentflow diagram.
      return { otherDiagram: true };
    }
    const { id, diag } = current;
    const db = diag.db;
    // The style element of the diagram is in the svg before the draw function runs.
    const style = document.querySelector(`[id="${id}"] style`);
    const rec = {
      type: diag.type,
      // The text that Diagram.fromText gave to the parser.
      textHash: textHash(diag.text),
      nodes: clean(data.nodes),
      edges: clean(data.edges),
      other: clean(data.other),
      dataConnectors: clean(data.connectors),
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
      styleHash: style ? textHash(style.textContent) : null,
      ...dbState(db),
    };
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
      await mermaid.render('my-svg', 'agentflow-beta TB\n  warm["up"]', document.getElementById('container'));
    } catch {
      // The layout that dumps stops the render.
    }
  };
  const run = async (c) => {
    let rec = { name: c.name };
    captured = undefined;
    current = undefined;
    try {
      const res = await mermaid.render('my-svg', c.text, document.getElementById('container'));
      rec.type = res.diagramType;
    } catch (e) {
      const message = String((e && e.message) || e);
      if (message !== SENTINEL || !captured) {
        rec = { ...rec, ...errorRecord(e) };
      }
    }
    if (captured && !rec.error) {
      Object.assign(rec, captured);
    }
    document.querySelectorAll('#container > *, body > :not(#container):not(iframe)').forEach((el) => el.remove());
    return rec;
  };

  // ---- api cases: the methods of the db, called directly ----
  const und = (v) => (v !== null && typeof v === 'object' && v.$u === 1 ? undefined : v);
  const runApi = (c) => {
    const rec = { name: c.name, returns: [] };
    const db = new mod.__x_AgentFlowDB();
    try {
      for (const [method, ...rawArgs] of c.ops) {
        const a = rawArgs.map(und);
        let result;
        switch (method) {
          case 'addSubGraphSame': {
            // The id and the title are the same object.
            const obj = a[0];
            result = db.addSubGraph(obj, a[1], obj, a[2]);
            break;
          }
          case 'getElementAtPosition':
            result = tag(db.getElementAtPosition(a[0], a[1]));
            break;
          case 'getElementById':
            result = db.getElementById(a[0]) ?? null;
            break;
          case 'getElementsOnLine':
            result = db.getElementsOnLine(a[0]).map(tag);
            break;
          case 'isToolDefinition':
            result = db.isToolDefinition(db.getVertices().get(a[0]));
            break;
          case 'getData': {
            const data = db.getData();
            result = { nodes: data.nodes, edges: data.edges, other: data.other, connectors: data.connectors };
            break;
          }
          case 'assignColorSlots': {
            // The nodes, the kind of each node id, and the order of the containers.
            const nodes = a[0];
            mod.__x_assignColorSlots(nodes, (id) => a[1][id], new Map(a[2]));
            result = nodes;
            break;
          }
          case 'transformData': {
            const data = { nodes: a[0], edges: [] };
            mod.__x_normaliseNodeShapes(data.nodes, a[1] ? db : undefined);
            result = data.nodes;
            break;
          }
          default:
            result = db[method](...a);
        }
        rec.returns.push(clean(result === undefined ? { $u: 1 } : result));
      }
    } catch (e) {
      Object.assign(rec, errorRecord(e));
    }
    rec.config = clean(mermaid.mermaidAPI.getConfig());
    rec.state = dbState(db);
    return rec;
  };

  // ---- functions of the chunk that take plain values ----
  const links = (list) => {
    const db = new mod.__x_AgentFlowDB();
    return list.map(([str, startStr]) => {
      try {
        return clean(db.destructLink(str, startStr ?? undefined));
      } catch (e) {
        return { error: String((e && e.message) || e) };
      }
    });
  };
  const commas = (list) => list.map((body) => mod.__x_stripLineTrailingCommas(body));
  const proto = (list) => list.map((text) => clean(mod.__x_stripPrototypeKeys(JSON.parse(text))));
  const shapes = (list) =>
    list.map((n) => {
      const diags = [];
      const node = { id: 'n' };
      if (n.shape !== null && n.shape !== undefined) {
        node.shape = n.shape;
      }
      if (n.isGroup) {
        node.isGroup = true;
      }
      mod.__x_normaliseNodeShapes([node], {
        emitError: (id, message, ctx) => diags.push({ kind: 'error', id, message, ctx }),
        emitWarning: (id, message, ctx) => diags.push({ kind: 'warning', id, message, ctx }),
      });
      return clean({ shape: node.shape ?? null, alias: mod.__x_resolveShapeAlias(n.shape ?? undefined) ?? null, diags });
    });
  const slots = () => {
    const out = [];
    for (let paletteLength = 0; paletteLength <= 16; paletteLength++) {
      const row = [mod.__x_containerSlotCount(paletteLength)];
      for (let n = 0; n <= 20; n++) {
        row.push(mod.__x_containerSlot(n, paletteLength));
      }
      out.push(row);
    }
    return out;
  };
  const constants = () => ({
    defaultStyle: new mod.__x_AgentFlowDB().defaultStyle(),
    kinds: mod.__x_KINDS,
    kindCount: mod.__x_KIND_COUNT,
    kindClass: mod.__x_KINDS.map((k) => mod.__x_kindClass(k)),
    warnings: Object.values(mod.__x_AgentflowWarning),
    emitted: [...mod.__x_PARSER_EMITTED_DIAGNOSTICS],
    reserved: [...mod.__x_RESERVED_DIAGNOSTICS],
    aliases: [...mod.__x_SHAPE_ALIASES.entries()],
    removed: [...mod.__x_REMOVED_SHAPES],
    allowed: [...mod.__x_ALLOWED_SHAPES],
    defaultShape: mod.__x_DEFAULT_SHAPE,
    skipKeys: [...mod.__x_SEMANTIC_METADATA_SKIP_KEYS],
    subroutineAliases: [...mod.__x_SUBROUTINE_ALIASES],
    domIdPrefix: mod.__x_MERMAID_DOM_ID_PREFIX,
  });
  return { run, runApi, warmUp, links, commas, proto, shapes, slots, constants };
};

const setupSrc = '(' + String(setup) + ')';
const top = await (0, eval)(setupSrc)(afChunk, layoutChunk);
await top.warmUp();

const inFrame = async (site, fn) => {
  const frame = document.createElement('iframe');
  frame.src = '/index.html';
  const loaded = new Promise((resolve) => frame.addEventListener('load', resolve, { once: true }));
  document.body.appendChild(frame);
  await loaded;
  try {
    const page = await frame.contentWindow.eval(setupSrc)(afChunk, layoutChunk, site);
    return await fn(page);
  } finally {
    frame.remove();
  }
};

const out = [];
for (const c of args.cases) {
  out.push(c.fresh ? await inFrame(c.site, (page) => page.run(c)) : await top.run(c));
}
const api = [];
for (const c of args.api || []) {
  // Each api case runs in a new page, so that the DOMPurify hooks are not
  // installed and the config is the site config of the case.
  api.push(await inFrame(c.site, (page) => page.runApi(c)));
}
// The result is a JSON text: the transport of the oracle drops a property
// named __proto__ of an object.
return JSON.stringify({
  cases: out,
  api,
  links: top.links(args.links || []),
  commas: top.commas(args.commas || []),
  proto: top.proto(args.proto || []),
  shapes: top.shapes(args.shapes || []),
  slots: top.slots(),
  constants: top.constants(),
});
