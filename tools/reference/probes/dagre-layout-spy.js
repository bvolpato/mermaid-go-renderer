// Dagre layout spy: renders diagrams with the real mermaid and records what
// the dagre layout (rendering-util/layout-algorithms/dagre/index.js and
// mermaid-graphlib.js) receives, builds and returns. Run with:
//
//   node oracle.mjs esm-eval probes/dagre-layout-spy.js --args fixtures.json --out out.json
//
// args is { fixtures: [{ name, text, init }] }: `init` is an optional config
// for mermaid.initialize. gen-dagre-layout.mjs builds the fixture list and
// stores the result for the Go test of internal/ru/layout/dagre.
//
// The functions of the layout are module bindings and cannot be replaced.
// So the spy loads a second copy of the dagre chunk whose imports are
// wrapped, and makes the layout loader of mermaid return that copy. The
// copy imports the same module instances as mermaid does, so the module
// state (clusterDb, the node elements, the uniqueId counter) is shared.
// The wrappers return the promise of the wrapped function itself, so the
// order of the asynchronous work is the order of an unpatched mermaid.
//
// For each fixture the result has:
//
//   input         data4Layout as prepareLayoutForDagre receives it
//   adjustBefore  the graph before adjustClustersAndEdges
//   adjustAfter   the graph after it, with the nested cluster graphs
//   clusterDb0, descendants, parents   the module state after it
//   graphs        per graph ('' is the top graph, else the cluster id):
//     nodes       the insertMeasuredNode calls with the measured sizes
//     edgeLabels  the insertEdgeLabel calls with the label texts and the measured sizes
//     idCounter   the uniqueId counter of lodash before the layout
//     before      the graph handed to the layout of dagre
//     after       the positions that dagre wrote
//     position    the positionNode calls (nested graphs)
//     edges       the insertEdge and positionEdgeLabel calls (nested graphs)
//   bounds        the sizes that updateNodeBounds read, by cluster id
//   clusters      the insertCluster calls: the node before and after
//   core          data4Layout after runDagreLayoutCore
//   paintNodes    the nodes that the common paint gets, with skipNode and isCluster
//   edgeNodes     the nodes that the common paint gets for the edge ends
//   clusterDbEnd  clusterDb after the render
const CH = '/chunks/mermaid.esm/';
const dagreFile = internalsWhere('prepareLayoutForDagre').find((f) => f.startsWith('dagre-'));
const graphlibFile = internalsWhere('extractDescendants')[0];
const lodashFile = internalsWhere('uniqueId_default').find((f) => internalsWhere('idCounter').includes(f));
const clusterFile = internalsWhere('insertCluster')[0];
const clusterDb = await internalsFrom(graphlibFile, 'clusterDb');
const descendants = await internalsFrom(graphlibFile, 'descendants');
const parents = await internalsFrom(graphlibFile, 'parents');
const clusterShapes = await internalsFrom(clusterFile, 'shapes');
const layoutAlgorithms = await internalsFrom(internalsWhere('layoutAlgorithms')[0], 'layoutAlgorithms');
const lodash = await import(CH + lodashFile);
const idCounter = () => lodash.__x_idCounter;

// ---- serialization ----
// A number that JSON cannot hold is { $n: text }. An undefined value, null
// and an empty string are left out: the Go model does not tell them apart.
const num = (v) => (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0)) ? { $n: Object.is(v, -0) ? '-0' : String(v) } : v);
const put = (o, k, v) => {
  if (v !== undefined && v !== null && v !== '') {
    o[k] = num(v);
  }
};
// dyn keeps a config value as it is: undefined is { $u: 1 }.
const dyn = (v) => (v === undefined ? { $u: 1 } : num(v));
const pos = (n) => [n.id ?? null, dyn(n.x), dyn(n.y), dyn(n.width), dyn(n.height), dyn(n.diff), n.clusterNode === true];
const geom = (n) => (n ? [dyn(n.x), dyn(n.y), dyn(n.width), dyn(n.height)] : null);

// What dagre reads from a label: an absent property takes the default, and
// a property that exists is converted with Number().
const dagreNum = (o, k, def) => num(k in o ? Number(o[k]) : def);

const NODE_STR = ['id', 'parentId', 'dir', 'domId', 'shape', 'label', 'labelStyle', 'style'];
const NODE_NUM = ['width', 'height', 'x', 'y', 'diff', 'padding'];
const projectNode = (n, deep) => {
  const o = {};
  for (const k of NODE_STR) {
    put(o, k, n[k]);
  }
  for (const k of NODE_NUM) {
    put(o, k, n[k]);
  }
  if (n.isGroup) {
    o.isGroup = true;
  }
  if (n.clusterNode) {
    o.clusterNode = true;
  }
  if (n.clusterData) {
    o.clusterData = n.clusterData.id ?? null;
  }
  o.d = [dagreNum(n, 'width', 0), dagreNum(n, 'height', 0)];
  if (deep && n.graph) {
    o.graph = projectGraph(n.graph, deep);
  }
  return o;
};

const EDGE_STR = ['id', 'start', 'end', 'label', 'labelpos', 'fromCluster', 'toCluster', 'arrowTypeStart', 'arrowTypeEnd', 'startLabelLeft', 'startLabelRight', 'endLabelLeft', 'endLabelRight'];
const EDGE_NUM = ['minlen', 'width', 'height', 'x', 'y'];
const projectEdge = (e) => {
  const o = {};
  for (const k of EDGE_STR) {
    put(o, k, e[k]);
  }
  for (const k of EDGE_NUM) {
    put(o, k, e[k]);
  }
  if (e.points) {
    o.points = e.points.map((p) => [num(p.x), num(p.y)]);
  }
  if (e.selfLoop) {
    o.selfLoop = [e.selfLoop.id ?? null, e.selfLoop.order];
  }
  if (e.originalEdge) {
    o.originalEdge = e.originalEdge.id ?? null;
  }
  o.d = [dagreNum(e, 'minlen', 1), dagreNum(e, 'weight', 1), dagreNum(e, 'width', 0), dagreNum(e, 'height', 0), dagreNum(e, 'labeloffset', 10), e.labelpos === undefined ? 'r' : e.labelpos];
  return o;
};

const projectGraphLabel = (l) => {
  if (!l) {
    return null;
  }
  const o = {};
  put(o, 'rankdir', l.rankdir);
  o.nodesep = dyn(l.nodesep);
  o.ranksep = dyn(l.ranksep);
  put(o, 'marginx', l.marginx);
  put(o, 'marginy', l.marginy);
  put(o, 'width', l.width);
  put(o, 'height', l.height);
  return o;
};

const projectGraph = (g, deep) => ({
  value: projectGraphLabel(g.graph()),
  nodes: g.nodes().map((v) => {
    const o = { v };
    const parent = g.parent(v);
    if (parent !== undefined) {
      o.parent = parent;
    }
    const value = g.node(v);
    if (value !== undefined) {
      o.value = projectNode(value, deep);
    }
    return o;
  }),
  edges: g.edges().map((e) => {
    const o = { v: e.v, w: e.w };
    if (e.name !== undefined) {
      o.name = e.name;
    }
    const value = g.edge(e);
    if (value !== undefined) {
      o.value = projectEdge(value);
    }
    return o;
  }),
});

// The result of a layout: only what dagre writes.
const projectLayoutResult = (g) => ({
  graph: [dyn(g.graph().width), dyn(g.graph().height)],
  nodes: g.nodes().map((v) => {
    const n = g.node(v);
    return n ? [dyn(n.x), dyn(n.y), dyn(n.width), dyn(n.height)] : null;
  }),
  edges: g.edges().map((e) => {
    const l = g.edge(e);
    return [(l.points || []).map((p) => [num(p.x), num(p.y)]), dyn(l.x), dyn(l.y)];
  }),
});

const projectClusterDb = () =>
  [...clusterDb.entries()].map(([key, entry]) => {
    const o = { key };
    put(o, 'id', entry.id);
    if (entry.externalConnections) {
      o.externalConnections = true;
    }
    if (entry.clusterData) {
      o.clusterData = entry.clusterData.id ?? null;
    }
    if (entry.node) {
      o.node = geom(entry.node);
    }
    return o;
  });

// The properties whose value is undefined although the property exists.
// dagre and the object spreads of the layout treat these differently from
// absent properties, and the Go model cannot tell them apart.
const explicitUndefined = (o, keys) => keys.filter((k) => k in o && o[k] === undefined);

const projectInput = (data, siteConfig) => ({
  type: data.type,
  diagramId: data.diagramId,
  layoutAlgorithm: data.layoutAlgorithm,
  direction: dyn(data.direction),
  nodeSpacing: dyn(data.nodeSpacing),
  rankSpacing: dyn(data.rankSpacing),
  dataConfig: {
    nodeSpacing: dyn(data.config?.nodeSpacing),
    rankSpacing: dyn(data.config?.rankSpacing),
    flowchartNodeSpacing: dyn(data.config?.flowchart?.nodeSpacing),
    flowchartRankSpacing: dyn(data.config?.flowchart?.rankSpacing),
  },
  subGraphTitleMargin: siteConfig.flowchart?.subGraphTitleMargin ?? null,
  nodes: data.nodes.map((n) => {
    const o = projectNode(n, false);
    delete o.d;
    if (n.label !== undefined) {
      o.hasLabel = true;
    }
    const undef = explicitUndefined(n, ['width', 'height']);
    if (undef.length > 0) {
      o.undef = undef;
    }
    return o;
  }),
  edges: data.edges.map((e) => {
    const o = projectEdge(e);
    delete o.d;
    if (e.label !== undefined) {
      o.hasLabel = true;
    }
    for (const k of ['weight', 'labeloffset']) {
      if (k in e) {
        o[k] = dyn(e[k]);
      }
    }
    const undef = explicitUndefined(e, ['minlen', 'weight', 'width', 'height', 'labeloffset', 'labelpos', 'arrowTypeStart', 'arrowTypeEnd']);
    if (undef.length > 0) {
      o.undef = undef;
    }
    return o;
  }),
});

// ---- the spy ----
let rec = null;
let cur = null; // the graph of the measureDagreGraph call that just started
let curPaint = null; // the graph of the paintDagreLayoutCore call that just started
let lastEdge = null;
const graphKey = new WeakMap(); // graph -> key
const elKey = new WeakMap(); // group element -> key
const graphRec = (key) => (rec.graphs[key] ||= { nodes: [], edgeLabels: [], position: [], edges: [] });

const hooks = {
  adjustClustersAndEdges(fn, [graph, depth]) {
    if (!rec) {
      return fn(graph, depth);
    }
    rec.adjustBefore = projectGraph(graph, false);
    fn(graph, depth);
    rec.adjustAfter = projectGraph(graph, true);
    rec.clusterDb0 = projectClusterDb();
    rec.descendants = [...descendants.entries()];
    rec.parents = [...parents.entries()];
  },
  createCommonLayoutRenderer(fn, [def]) {
    const { getConfig } = def.__spyHelpers;
    const options = def.paintOptions;
    return fn({
      ...def,
      prepareLayout: (data, context) => {
        if (rec) {
          rec.input = projectInput(data, getConfig());
        }
        return def.prepareLayout(data, context);
      },
      runLayoutCore: (data, context) => {
        const result = def.runLayoutCore(data, context);
        if (rec) {
          rec.core = { nodes: data.nodes.map(pos), edges: data.edges.map(projectEdge) };
        }
        return result;
      },
      paintOptions: {
        ...options,
        getNodes: (data, context) => {
          const nodes = options.getNodes(data, context);
          if (rec) {
            rec.paintNodes = nodes.map((n) => [...pos(n), n.isGroup === true, options.skipNode(n, context), options.isCluster(n, context)]);
          }
          return nodes;
        },
        getEdgeNode: (id, edge, context) => {
          const node = options.getEdgeNode(id, edge, context);
          if (rec) {
            rec.edgeNodes.push([edge.id ?? null, id ?? null, node ? pos(node) : null]);
          }
          return node;
        },
      },
    });
  },
  createLayoutElementGroups(fn, args) {
    const groups = fn(...args);
    if (rec && cur) {
      for (const k of ['clusters', 'edgePaths', 'edgeLabels', 'nodes', 'rootGroups']) {
        elKey.set(groups[k].node(), cur.key);
      }
    }
    return groups;
  },
  insertMeasuredNode(fn, [nodesGroup, node, options]) {
    const promise = fn(nodesGroup, node, options);
    if (rec) {
      const entry = { id: node.id ?? null, dir: dyn(options.dir) };
      graphRec(elKey.get(nodesGroup.node())).nodes.push(entry);
      promise.then(
        () => {
          entry.size = [dyn(node.width), dyn(node.height)];
        },
        () => {}
      );
    }
    return promise;
  },
  insertEdgeLabel(fn, [elem, edge]) {
    const promise = fn(elem, edge);
    if (rec) {
      // The texts of the label and of the four terminal labels.
      const entry = { id: edge.id ?? null, texts: [edge.label, edge.startLabelLeft, edge.startLabelRight, edge.endLabelLeft, edge.endLabelRight].map((t) => t || null) };
      graphRec(elKey.get(elem.node())).edgeLabels.push(entry);
      promise.then(
        () => {
          entry.size = [dyn(edge.width), dyn(edge.height)];
        },
        () => {}
      );
    }
    return promise;
  },
  updateNodeBounds(fn, [node, element]) {
    fn(node, element);
    if (rec) {
      rec.bounds[node.id] = [dyn(node.width), dyn(node.height)];
    }
  },
  positionNode(fn, [node]) {
    if (rec && curPaint) {
      graphRec(curPaint.key).position.push(pos(node));
    }
    return fn(node);
  },
  insertEdge(fn, args) {
    const [elem, edge, , , startNode, endNode] = args;
    if (rec) {
      lastEdge = {
        edge: projectEdge(edge),
        startNode: startNode ? (startNode.id ?? null) : null,
        endNode: endNode ? (endNode.id ?? null) : null,
      };
      delete lastEdge.edge.d;
      if (edge.toCluster) {
        lastEdge.toCluster = geom(clusterDb.get(edge.toCluster)?.node);
      }
      if (edge.fromCluster) {
        lastEdge.fromCluster = geom(clusterDb.get(edge.fromCluster)?.node);
      }
      graphRec(elKey.get(elem.node())).edges.push(lastEdge);
    }
    return fn(...args);
  },
  positionEdgeLabel(fn, [edge, paths]) {
    if (rec && lastEdge) {
      lastEdge.label = [dyn(edge.x), dyn(edge.y)];
    }
    return fn(edge, paths);
  },
  dagreLayout(fn, [graph]) {
    if (!rec) {
      return fn(graph);
    }
    const g = graphRec(graphKey.get(graph));
    g.idCounter = idCounter();
    g.before = projectGraph(graph, false);
    rec.layoutOrder.push(graphKey.get(graph));
    fn(graph);
    g.after = projectLayoutResult(graph);
  },
};

globalThis.__dagreSpy = {
  call: (name, fn, args) => (hooks[name] ? hooks[name](fn, args) : fn(...args)),
  enterMeasure: (graph, parentCluster) => {
    const key = parentCluster ? String(parentCluster.id) : '';
    graphKey.set(graph, key);
    cur = { key };
    if (rec) {
      graphRec(key);
      rec.measureOrder.push(key);
    }
  },
  enterPaint: (graph) => {
    curPaint = { key: graphKey.get(graph) };
  },
};

// The cluster shapes are called from the dagre chunk (nested graphs) and
// from the common paint (the top graph). The map of the shapes is an
// object, so its functions can be wrapped in place.
for (const name of Object.keys(clusterShapes)) {
  const original = clusterShapes[name];
  clusterShapes[name] = (parent, node) => {
    if (!rec) {
      return original(parent, node);
    }
    const entry = { id: node.id ?? null, shape: name, pre: geom(node) };
    rec.clusters.push(entry);
    const result = original(parent, node);
    Promise.resolve(result).then(
      () => {
        entry.post = [dyn(node.width), dyn(node.height), dyn(node.diff), dyn(node.offsetX), dyn(node.offsetY)];
      },
      () => {}
    );
    return result;
  };
}

// ---- the patched copy of the dagre chunk ----
const WRAP = [
  'adjustClustersAndEdges',
  'createCommonLayoutRenderer',
  'createLayoutElementGroups',
  'insertMeasuredNode',
  'insertEdge',
  'insertEdgeLabel',
  'positionEdgeLabel',
  'positionNode',
  'updateNodeBounds',
];
let src = await (await fetch(CH + dagreFile)).text();
const abs = location.origin + CH;
src = src.replace(/from "\.\//g, `from "${abs}`).replace(/^import "\.\//gm, `import "${abs}`);
const headEnd = src.indexOf('\n// ../../node_modules/');
let head = src.slice(0, headEnd);
let body = src.slice(headEnd);
for (const name of WRAP) {
  const re = new RegExp(`(\\n\\s+)${name}(,?\\n)`);
  if (!re.test(head)) {
    throw new Error(`dagre-layout-spy: the dagre chunk does not import ${name}`);
  }
  head = head.replace(re, `$1${name} as __o_${name}$2`);
}
head += '\n' + WRAP.map((name) => `const ${name} = (...a) => globalThis.__dagreSpy.call(${JSON.stringify(name)}, __o_${name}, a);`).join('\n') + '\n';
const patch = (from, to) => {
  if (body.split(from).length !== 2) {
    throw new Error(`dagre-layout-spy: expected one occurrence of ${JSON.stringify(from)}`);
  }
  body = body.replace(from, to);
};
patch('\n  layout(graph);\n', '\n  globalThis.__dagreSpy.call("dagreLayout", layout, [graph]);\n');
patch('\n  const dir = graph.graph().rankdir;\n', '\n  const dir = graph.graph().rankdir;\n  globalThis.__dagreSpy.enterMeasure(graph, parentCluster);\n');
patch('\n  let diff = 0;\n', '\n  let diff = 0;\n  globalThis.__dagreSpy.enterPaint(graph);\n');
patch('var render = createCommonLayoutRenderer({\n', 'var render = createCommonLayoutRenderer({\n  __spyHelpers: { getConfig },\n');
const patched = await import(URL.createObjectURL(new Blob([head + body], { type: 'text/javascript' })));
layoutAlgorithms.dagre.loader = async () => patched;

// ---- the renders ----
const container = document.getElementById('container');
const out = [];
for (const fixture of args.fixtures) {
  const res = { name: fixture.name };
  out.push(res);
  rec = { graphs: {}, measureOrder: [], layoutOrder: [], bounds: {}, clusters: [], edgeNodes: [] };
  cur = null;
  curPaint = null;
  lastEdge = null;
  try {
    mermaid.initialize({ startOnLoad: false, ...(fixture.init || {}) });
    container.innerHTML = '';
    globalThis.__mmdgResetRandom();
    const { diagramType } = await mermaid.render('my-svg', fixture.text, container);
    res.diagramType = diagramType;
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
  if (rec.input) {
    rec.clusterDbEnd = projectClusterDb();
    Object.assign(res, rec);
  } else if (!res.error) {
    res.skipped = 'the dagre layout did not run';
  }
  rec = null;
}
container.innerHTML = '';
return out;
