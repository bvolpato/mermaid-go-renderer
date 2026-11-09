// Layout scaffolding oracle: calls the real functions of
// rendering-util/createGraph.ts and layout-algorithms/common/index.ts of
// mermaid for each case and returns what they built. Run with:
//
//   node oracle.mjs esm-eval probes/graph.js --args cases.json --out out.json
//
// The args are { cases, samples }. `samples` maps a diagram type to a
// diagram text that loads the type, so that its style sheet is registered.
// A case is { kind, name, type, directive, ... }: `type` is the diagram
// type whose config scope and style sheet apply and `directive` is a config
// object as in front matter. The kinds:
//
//   groups       { options }: createLayoutElementGroups(element, options).
//                `options` null stands for no argument.
//   measureGroup { node, measureWidth }: measureGroupLabel. `measureWidth`
//                is a number, "Infinity", or absent for undefined.
//   measuredNode { node, dir }: insertMeasuredNode.
//   createGraph  { data, options, detached, statsOnly }:
//                createGraphWithElements on the layout data { nodes, edges,
//                direction }. `detached` passes an empty selection. The
//                result also says whether handling the nodes one after
//                another, where upstream uses Promise.all, gives the same
//                markup (`sequential`). With `statsOnly` the result has
//                only that, and not the markup.
//   terminal     { edge, key, path }: terminalLabelTranslate. `path` null
//                stands for undefined.
//   margins      { marginConfig }: getSubGraphTitleMargins.
//   render       { data, def }: a layout made with
//                createCommonLayoutRenderer and run on the layout data. Its
//                core is the toy layout below, which the Go test has too.
//                `def` selects the phases and the paint options, see
//                makeDefinition.
const { setDiagramConfigScope, addDirective, getConfig, createUserStyles, insertEdgeLabel, hasEdgeLabel } = await internals(
  'setDiagramConfigScope',
  'addDirective',
  'getConfig',
  'createUserStyles',
  'insertEdgeLabel',
  'hasEdgeLabel'
);
const graphChunk = internalsWhere('createGraphWithElements')[0];
const fromGraph = (name) => internalsFrom(graphChunk, name);
const createLayoutElementGroups = await fromGraph('createLayoutElementGroups');
const measureGroupLabel = await fromGraph('measureGroupLabel');
const insertMeasuredNode = await fromGraph('insertMeasuredNode');
const createGraphWithElements = await fromGraph('createGraphWithElements');
const resolveNodeDir = await fromGraph('resolveNodeDir');
const createCommonLayoutRenderer = await fromGraph('createCommonLayoutRenderer');
const defaultMeasureLayout = await fromGraph('defaultMeasureLayout');
const paintLayoutData = await fromGraph('paintLayoutData');
const terminalLabelTranslate = await fromGraph('terminalLabelTranslate');
const getSubGraphTitleMargins = await internalsFrom(internalsWhere('getSubGraphTitleMargins')[0], 'getSubGraphTitleMargins');
const reset = await internalsFrom(internalsWhere('setDiagramConfigScope')[0], 'reset');
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
mermaid.initialize({ startOnLoad: false });

const samples = {
  'flowchart-v2': 'flowchart TD\nA',
  classDiagram: 'classDiagram\nclass A',
  stateDiagram: 'stateDiagram-v2\n[*] --> A',
  er: 'erDiagram\nA',
  ...(args.samples || {}),
};
const loadErrors = {};
for (const type of new Set((args.cases || []).map((c) => c.type || 'flowchart-v2'))) {
  if (samples[type]) {
    try {
      await mermaid.mermaidAPI.getDiagramFromText(samples[type]);
    } catch (e) {
      loadErrors[type] = String((e && e.message) || e);
    }
  }
}

// JSON has no NaN and no undefined: both are written as strings.
const num = (v) => (v === undefined ? 'undefined' : typeof v === 'number' && !Number.isFinite(v) ? String(v) : v);
const box = (b) => (b === undefined || b === null ? null : { x: num(b.x), y: num(b.y), width: num(b.width), height: num(b.height) });
const clone = (v) => JSON.parse(JSON.stringify(v));
const rays = [[-500, -300], [500, -300], [500, 300], [-500, 300], [0, -400], [0, 400], [-400, 0], [400, 0], [-37, 290], [123, -45], [1000, 10], [-3, -1000]];

const container = document.getElementById('container');

// page builds the page of a case: the svg with the style sheet of the
// config and a root group. It also resets the seeded Math.random.
const page = (config, type) => {
  container.innerHTML = '';
  const div = document.createElement('div');
  div.id = 'dmy-svg';
  div.setAttribute('style', `font-family: ${config.fontFamily}`);
  container.appendChild(div);
  const svgEl = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
  svgEl.setAttribute('id', 'my-svg');
  svgEl.setAttribute('width', '100%');
  svgEl.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
  div.appendChild(svgEl);
  const style = document.createElement('style');
  const css = createUserStyles(config, type, new Map(), '#my-svg');
  style.innerHTML = css;
  svgEl.appendChild(style);
  const root = select(svgEl).append('g');
  globalThis.__mmdgResetRandom();
  return { svgEl, root, css };
};

const nodeState = (n) => ({
  id: n.id,
  x: num(n.x),
  y: num(n.y),
  width: num(n.width),
  height: num(n.height),
  diff: num(n.diff),
  offsetX: num(n.offsetX),
  offsetY: num(n.offsetY),
  labelBBox: box(n.labelBBox),
  hasIntersect: typeof n.intersect === 'function',
});
const edgeState = (e) => ({ id: e.id, width: num(e.width), height: num(e.height), x: num(e.x), y: num(e.y) });

// sequentialGraph is createGraphWithElements with the nodes handled one
// after another. Upstream starts them together and awaits them with
// Promise.all.
const sequentialGraph = async (element, data, options) => {
  const config = getConfig();
  const groups = createLayoutElementGroups(element);
  const nodeById = new Map(data.nodes.map((node) => [node.id, node]));
  for (const node of data.nodes) {
    if (node.isGroup) {
      const unwrap = options.unwrapGroupLabels && node.labelType !== 'markdown';
      await measureGroupLabel(groups.nodes, node, unwrap ? Number.POSITIVE_INFINITY : undefined);
    } else {
      await insertMeasuredNode(groups.nodes, node, { config, dir: resolveNodeDir(node, nodeById, data.direction) });
    }
  }
  for (const edge of [...data.edges]) {
    if (hasEdgeLabel(edge)) {
      await insertEdgeLabel(groups.edgeLabels, edge);
    }
  }
};

// toyCore is the layout core of the render cases: the nodes in a row with
// three heights, each group around its descendants, and each edge through
// the centers of its nodes and a point below the middle. The Go test has
// the same function with the same floating-point expressions.
const toyCore = (data) => {
  const byId = new Map(data.nodes.map((n) => [n.id, n]));
  let x = 0;
  let i = 0;
  for (const node of data.nodes) {
    if (node.isGroup) {
      continue;
    }
    node.x = x + node.width / 2;
    node.y = 40 + (i % 3) * 70 + node.height / 2;
    x = x + node.width + 30;
    i++;
  }
  const inGroup = (node, group) => {
    let p = node.parentId;
    const seen = new Set();
    while (p && !seen.has(p)) {
      if (p === group.id) {
        return true;
      }
      seen.add(p);
      p = byId.get(p)?.parentId;
    }
    return false;
  };
  for (const group of data.nodes) {
    if (!group.isGroup) {
      continue;
    }
    let minX = Infinity;
    let minY = Infinity;
    let maxX = -Infinity;
    let maxY = -Infinity;
    for (const node of data.nodes) {
      if (node.isGroup || !inGroup(node, group)) {
        continue;
      }
      minX = Math.min(minX, node.x - node.width / 2);
      minY = Math.min(minY, node.y - node.height / 2);
      maxX = Math.max(maxX, node.x + node.width / 2);
      maxY = Math.max(maxY, node.y + node.height / 2);
    }
    if (minX === Infinity) {
      minX = x;
      minY = 40;
      maxX = x + 100;
      maxY = 100;
      x = x + 130;
    }
    const top = group.labelBBox?.height ?? 0;
    group.width = maxX - minX + 40;
    group.height = maxY - minY + 40 + top;
    group.x = minX - 20 + group.width / 2;
    group.y = minY - 20 - top + group.height / 2;
  }
  for (const edge of data.edges) {
    const s = byId.get(edge.start);
    const t = byId.get(edge.end);
    if (!s || !t) {
      continue;
    }
    const mx = (s.x + t.x) / 2;
    const my = (s.y + t.y) / 2 + 10;
    edge.points = [
      { x: s.x, y: s.y },
      { x: mx, y: my },
      { x: t.x, y: t.y },
    ];
    edge.x = mx;
    edge.y = my;
  }
  return { nodes: data.nodes.length };
};

// makeDefinition builds the layout definition of a render case from its
// flags:
//
//   prepare        add a prepareLayout that returns { tag: 'prepared' }
//   measure        'unwrap': the default measure with unwrapGroupLabels
//                  'custom': a measure that inserts the nodes itself, also
//                  the groups of paintOptions.notCluster, and returns only
//                  the groups
//   paint          'custom': a paintLayout that calls paintLayoutData with
//                  every edge skipped
//   afterPaint     add an afterPaint that appends g.after-paint to the root
//   paintOptions   getNodes: 'reverse', skipNode: [ids], notCluster: [ids],
//                  skipEdge: [ids], skipIntersect: true or [ids],
//                  getEdgeNode: 'none', clusterDb: [ids]
//
// Each phase adds a line to the log of the case.
const makeDefinition = (c, data, log) => {
  const flags = c.def || {};
  const po = flags.paintOptions;
  const byId = new Map(data.nodes.map((n) => [n.id, n]));
  const def = {};
  if (flags.prepare) {
    def.prepareLayout = (d, ctx) => {
      log.push('prepare');
      return { tag: 'prepared' };
    };
  }
  if (flags.measure === 'unwrap') {
    def.measureLayout = (d, ctx) => {
      log.push('measure:' + ctx.preparedLayout?.tag);
      return defaultMeasureLayout(d, ctx, { unwrapGroupLabels: true });
    };
  } else if (flags.measure === 'custom') {
    def.measureLayout = async (d, ctx) => {
      log.push('measure:' + ctx.preparedLayout?.tag);
      const groups = createLayoutElementGroups(ctx.element, { edgePathsClass: 'edgePaths' });
      for (const node of d.nodes) {
        if (!node.isGroup || po?.notCluster?.includes(node.id)) {
          await insertMeasuredNode(groups.nodes, node, { config: getConfig(), dir: d.direction });
        } else {
          await measureGroupLabel(groups.nodes, node);
        }
      }
      return { groups };
    };
  }
  def.runLayoutCore = (d, ctx) => {
    log.push('core:' + ctx.preparedLayout?.tag + ':' + ctx.options?.algorithm);
    return toyCore(d);
  };
  if (flags.paint === 'custom') {
    def.paintLayout = async (d, ctx, core) => {
      log.push('paint:' + core.nodes);
      await paintLayoutData(d, ctx, { skipEdge: () => true });
    };
  }
  if (flags.afterPaint) {
    def.afterPaint = (d, ctx, core) => {
      log.push('afterPaint:' + core.nodes);
      ctx.measure.groups.rootGroups.append('g').attr('class', 'after-paint');
    };
  }
  if (po) {
    const options = {};
    if (po.getNodes === 'reverse') {
      options.getNodes = (d) => [...d.nodes].reverse();
    }
    if (po.skipNode) {
      options.skipNode = (node) => po.skipNode.includes(node.id);
    }
    if (po.notCluster) {
      options.isCluster = (node) => !po.notCluster.includes(node.id);
    }
    if (po.skipEdge) {
      options.skipEdge = (edge) => po.skipEdge.includes(edge.id);
    }
    if (po.skipIntersect === true) {
      options.skipIntersect = true;
    } else if (Array.isArray(po.skipIntersect)) {
      options.skipIntersect = (edge) => po.skipIntersect.includes(edge.id);
    }
    if (po.getEdgeNode === 'none') {
      options.getEdgeNode = () => undefined;
    }
    if (po.clusterDb) {
      options.clusterDb = new Map(po.clusterDb.map((id) => [id, { node: byId.get(id) }]));
    }
    def.paintOptions = options;
  }
  return def;
};

const out = [];
for (const c of args.cases || []) {
  const res = { name: c.name };
  try {
    if (c.kind === 'terminal') {
      const p = terminalLabelTranslate(c.edge, c.key, c.path === null ? undefined : c.path);
      res.point = [num(p.x), num(p.y)];
      out.push(res);
      continue;
    }
    if (c.kind === 'margins') {
      const m = getSubGraphTitleMargins(c.marginConfig);
      res.margins = [num(m.subGraphTitleTopMargin), num(m.subGraphTitleBottomMargin), num(m.subGraphTitleTotalMargin)];
      out.push(res);
      continue;
    }
    const type = c.type || 'flowchart-v2';
    reset();
    setDiagramConfigScope(type);
    addDirective(c.directive || {});
    const config = getConfig();
    res.config = clone(config);
    const { svgEl, root, css } = page(config, type);
    res.css = css;

    if (c.kind === 'groups') {
      const groups = c.options === null ? createLayoutElementGroups(root) : createLayoutElementGroups(root, c.options);
      res.html = root.node().innerHTML;
      res.order = ['clusters', 'edgePaths', 'edgeLabels', 'nodes'].map((k) => [...groups.rootGroups.node().children].indexOf(groups[k].node()));
    } else if (c.kind === 'measureGroup') {
      const nodes = root.append('g').attr('class', 'nodes');
      const node = clone(c.node);
      const width = c.measureWidth === 'Infinity' ? Number.POSITIVE_INFINITY : c.measureWidth;
      await measureGroupLabel(nodes, node, width);
      res.html = nodes.node().innerHTML;
      res.randomCalls = globalThis.__mmdgRandomCalls;
      res.node = nodeState(node);
    } else if (c.kind === 'measuredNode') {
      const nodes = root.append('g').attr('class', 'nodes');
      const node = clone(c.node);
      const el = await insertMeasuredNode(nodes, node, { config, dir: c.dir });
      res.html = nodes.node().innerHTML;
      res.randomCalls = globalThis.__mmdgRandomCalls;
      res.node = nodeState(node);
      res.elementIsLastChild = el.node() === nodes.node().lastElementChild;
      node.x = 0;
      node.y = 0;
      res.intersects = rays.map(([x, y]) => {
        try {
          const p = node.intersect({ x, y });
          return [num(p.x), num(p.y)];
        } catch (e) {
          return 'ERR ' + e.message;
        }
      });
    } else if (c.kind === 'createGraph') {
      const data = clone(c.data);
      const edgeCount = data.edges.length;
      const result = await createGraphWithElements(c.detached ? select(null) : root, data, c.options || {});
      res.html = root.node().innerHTML;
      res.randomCalls = globalThis.__mmdgRandomCalls;
      res.nodes = data.nodes.map(nodeState);
      res.edges = data.edges.map(edgeState);
      res.edgesAdded = data.edges.length - edgeCount;
      const graph = result.graph;
      res.graph = {
        directed: graph.isDirected(),
        multigraph: graph.isMultigraph(),
        compound: graph.isCompound(),
        nodes: graph.nodes(),
        labels: graph.nodes().map((v) => {
          const label = graph.node(v);
          return { id: label.id, width: num(label.width), height: num(label.height), labelBBox: box(label.labelBBox), isCopy: label !== data.nodes.find((n) => n.id === v) };
        }),
        edges: graph.edges().map((e) => [e.v, e.w, e.name === undefined ? null : e.name]),
        edgeLabels: graph.edges().map((e) => ({ id: graph.edge(e).id, width: num(graph.edge(e).width), height: num(graph.edge(e).height) })),
      };
      res.nodeElements = [...result.nodeElements.keys()];
      res.groupClasses = ['clusters', 'edgePaths', 'edgeLabels', 'nodes', 'rootGroups'].map((k) => (result.groups[k].node() ? result.groups[k].attr('class') : null));
      if (!c.detached) {
        // The same work with the nodes one after another.
        const second = page(config, type);
        const data2 = clone(c.data);
        try {
          await sequentialGraph(second.root, data2, c.options || {});
          res.sequential = {
            sameHtml: second.root.node().innerHTML === res.html,
            sameRandomCalls: globalThis.__mmdgRandomCalls === res.randomCalls,
            sameSizes: JSON.stringify(data2.nodes.map(nodeState)) === JSON.stringify(res.nodes),
          };
        } catch (e) {
          res.sequential = { error: String((e && e.message) || e) };
        }
      }
    } else if (c.kind === 'render') {
      const data = clone(c.data);
      data.config = config;
      const log = [];
      const render = createCommonLayoutRenderer(makeDefinition(c, data, log));
      await render(data, select(svgEl), undefined, { algorithm: (c.def || {}).algorithm });
      res.html = root.node().innerHTML;
      res.randomCalls = globalThis.__mmdgRandomCalls;
      res.log = log;
      res.nodes = data.nodes.map(nodeState);
      res.edges = data.edges.map(edgeState);
    } else {
      throw new Error('unknown case kind ' + c.kind);
    }
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  if (c.statsOnly) {
    out.push({ name: res.name, error: res.error, sequential: res.sequential, randomCalls: res.randomCalls });
    continue;
  }
  out.push(res);
}
container.innerHTML = '';
return { cases: out, loadErrors };
