// Edge oracle: calls the real edge functions of mermaid (insertMarkers,
// insertEdgeLabel, insertEdge, positionEdgeLabel, the line jump functions
// and the pure helpers of edges.js) for each case and returns what they
// built. Run with:
//
//   node oracle.mjs esm-eval probes/edges.js --args cases.json --out out.json
//
// args is { cases: [...] }. A case is one of:
//
//   { name, pure: { fn, args } }
//     Calls one pure function: computeLabelTransform, getLabelStyles,
//     hasEdgeLabel, intersection, generateRoundedPath,
//     applyMarkerOffsetsToPoints, resolveEdgeLabelPosition,
//     findEdgeIntersections, processEdgesWithJumps, isStraightPath,
//     curveSupportsLineHops. The result is { result } or { error }.
//
//   { name, type, directive, id, markers, edges, ops }
//     A scenario in a page like the one of mermaid.render: an svg with the
//     style sheet of the diagram type and a root <g> with the groups
//     g.edgePaths and g.edgeLabels. `markers` are inserted first with
//     insertMarkers. `edges` are edge objects (rendering-util/types.ts).
//     `ops` run in order on them:
//       { op: 'label', edge: i }            insertEdgeLabel(edgeLabels, edges[i])
//       { op: 'labels', edges: [i, ...] }   Promise.all of insertEdgeLabel calls
//       { op: 'edge', edge: i, startNode, endNode, clusterDb, skipIntersect,
//         layoutAlgorithm, diagramType, noDiagramId, randomAt }
//                                           insertEdge(edgePaths, edges[i], ...);
//                                           randomAt is the number of
//                                           Math.random draws before the call
//       { op: 'position', edge: i, paths }  positionEdgeLabel(edges[i], paths);
//                                           paths is { fromOp: n } (the result
//                                           of an earlier edge op), an object
//                                           { updatedPath, originalPath } with
//                                           points [x, y], or null
//       { op: 'curve', edge: i }            resolveEdgeCurveType(edges[i].curve)
//       { op: 'clear' }                     clear()
//       { op: 'lineJumps', edges: [geom...], config }
//                                           applyLineJumpsToSvg(edgePaths, ...)
//     A node is null (the empty object), { fixed: [x, y] } (intersect
//     returns that point) or { rect: { x, y, width, height } } (intersect
//     is intersect.rect). clusterDb is { id: { x, y, width, height } }.
//     The result has the effective config, the style sheet, the markup of
//     the root group, the result of each op (`results`) and the edges
//     afterwards.
//
// In the cases, the strings "#NaN", "#Infinity", "#-Infinity" and
// "#undefined" stand for those values. The results use the same strings for
// numbers that JSON cannot hold.
const E = (name) => internalsFrom('chunk-RLXJHMIZ', name);
const J = (name) => internalsFrom('chunk-UETRLHRR', name);
const fns = {
  insertEdge: await E('insertEdge'),
  insertEdgeLabel: await E('insertEdgeLabel'),
  positionEdgeLabel: await E('positionEdgeLabel'),
  insertMarkers: await E('markers_default'),
  clear: await E('clear'),
  hasEdgeLabel: await E('hasEdgeLabel'),
  resolveEdgeLabelPosition: await E('resolveEdgeLabelPosition'),
  computeLabelTransform: await E('computeLabelTransform'),
  getLabelStyles: await E('getLabelStyles'),
  resolveEdgeCurveType: await E('resolveEdgeCurveType'),
  intersection: await E('intersection'),
  generateRoundedPath: await E('generateRoundedPath'),
  applyMarkerOffsetsToPoints: await E('applyMarkerOffsetsToPoints'),
  findEdgeIntersections: await J('findEdgeIntersections'),
  rewriteEdgePath: await J('rewriteEdgePath'),
  pointToString: await J('pointToString'),
  isStraightPath: await J('isStraightPath'),
  curveSupportsLineHops: await J('curveSupportsLineHops'),
  applyLineJumpsToSvg: await J('applyLineJumpsToSvg'),
};
// The bundle of mermaid has no processEdgesWithJumps and no plainPath: only
// the tests of upstream call them, so the bundler dropped them. These two
// are their source text from lineJump.ts, on top of the real
// findEdgeIntersections, rewriteEdgePath and pointToString of the bundle.
const plainPath = (points) => {
  if (points.length === 0) {
    return '';
  }
  const parts = [`M${fns.pointToString(points[0])}`];
  for (let i = 1; i < points.length; i++) {
    parts.push(`L${fns.pointToString(points[i])}`);
  }
  return parts.join(' ');
};
fns.processEdgesWithJumps = (edges, config) => {
  const result = new Map();
  if (!config.enabled) {
    for (const edge of edges) {
      result.set(edge.id, plainPath(edge.points));
    }
    return result;
  }
  const crossings = fns.findEdgeIntersections(edges);
  const jumpsByEdge = new Map();
  for (const c of crossings) {
    const list = jumpsByEdge.get(c.jumpEdgeId) ?? [];
    list.push(c);
    jumpsByEdge.set(c.jumpEdgeId, list);
  }
  for (const edge of edges) {
    const jumps = jumpsByEdge.get(edge.id);
    if (!jumps || jumps.length === 0) {
      result.set(edge.id, plainPath(edge.points));
    } else {
      result.set(edge.id, fns.rewriteEdgePath(edge, jumps, config));
    }
  }
  return result;
};
const edgeLabels = await E('edgeLabels');
const terminalLabels = await E('terminalLabels');
const cfgMod = {
  reset: await internalsFrom('chunk-DLUCIBNR', 'reset'),
  setDiagramConfigScope: await internalsFrom('chunk-DLUCIBNR', 'setDiagramConfigScope'),
  addDirective: await internalsFrom('chunk-DLUCIBNR', 'addDirective'),
  getConfig: await internalsFrom('chunk-DLUCIBNR', 'getConfig'),
};
const createUserStyles = await internalsFrom('mermaid.esm.mjs', 'createUserStyles');
const select = await internalsFrom('chunk-OHNBJXQ2', 'select_default');
const intersectRect = await internalsFrom('chunk-VFTINRM2', 'intersect_rect_default');
mermaid.initialize({ startOnLoad: false });

const markersOfNumber = { '#NaN': NaN, '#Infinity': Infinity, '#-Infinity': -Infinity };
// dec replaces the marker strings of a case by their values.
const dec = (v) => {
  if (typeof v === 'string') {
    return v in markersOfNumber ? markersOfNumber[v] : v === '#undefined' ? undefined : v;
  }
  if (Array.isArray(v)) {
    return v.map(dec);
  }
  if (v && typeof v === 'object') {
    const o = {};
    for (const [k, x] of Object.entries(v)) {
      o[k] = dec(x);
    }
    return o;
  }
  return v;
};
// enc makes a value JSON-safe: non-finite numbers become marker strings,
// a Map becomes its entries, functions and DOM nodes are dropped.
const enc = (v) => {
  if (typeof v === 'number') {
    return Number.isFinite(v) ? v : `#${v}`;
  }
  if (v === undefined) {
    return '#undefined';
  }
  if (v === null || typeof v === 'string' || typeof v === 'boolean') {
    return v;
  }
  if (typeof v === 'function') {
    return undefined;
  }
  if (v instanceof Map) {
    return [...v.entries()].map(([k, x]) => [k, enc(x)]);
  }
  if (Array.isArray(v)) {
    return v.map(enc);
  }
  if (v instanceof Node || typeof v.node === 'function') {
    return undefined;
  }
  const o = {};
  for (const [k, x] of Object.entries(v)) {
    if (x !== undefined && typeof x !== 'function') {
      const e = enc(x);
      if (e !== undefined) {
        o[k] = e;
      }
    }
  }
  return o;
};
const toPoints = (pts) => (pts == null ? pts : pts.map(([x, y]) => ({ x, y })));
const encPoints = (pts) => (pts === undefined ? '#undefined' : pts.map((p) => [enc(p.x), enc(p.y)]));

const samples = {
  'flowchart-v2': 'flowchart TD\nA',
  classDiagram: 'classDiagram\nclass A',
  stateDiagram: 'stateDiagram-v2\n[*] --> A',
  er: 'erDiagram\nA',
  requirement: 'requirementDiagram\nrequirement r {\nid: 1\n}',
  mindmap: 'mindmap\nroot',
  kanban: 'kanban\ntodo\n  a',
  block: 'block-beta\na',
};
// A diagram type registers its style sheet when it is first loaded.
for (const type of new Set(args.cases.filter((c) => !c.pure).map((c) => c.type || 'flowchart-v2'))) {
  if (samples[type]) {
    await mermaid.mermaidAPI.getDiagramFromText(samples[type]);
  }
}

const runPure = (c) => {
  const a = dec(c.pure.args);
  switch (c.pure.fn) {
    case 'computeLabelTransform':
      return fns.computeLabelTransform(a[0], a[1]);
    case 'getLabelStyles':
      return fns.getLabelStyles(a[0]);
    case 'hasEdgeLabel':
      return fns.hasEdgeLabel(a[0]);
    case 'intersection': {
      const p = fns.intersection(a[0], a[1], a[2]);
      return [p.x, p.y];
    }
    case 'generateRoundedPath':
      return fns.generateRoundedPath(toPoints(a[0]), a[1]);
    case 'applyMarkerOffsetsToPoints':
      return fns.applyMarkerOffsetsToPoints(toPoints(a[0]), a[1]).map((p) => [p.x, p.y]);
    case 'resolveEdgeLabelPosition': {
      const paths = a[1] == null ? a[1] : {};
      if (paths && a[1].updatedPath) {
        paths.updatedPath = toPoints(a[1].updatedPath);
      }
      if (paths && a[1].originalPath) {
        paths.originalPath = toPoints(a[1].originalPath);
      }
      const p = fns.resolveEdgeLabelPosition(a[0], paths);
      return [p.x, p.y];
    }
    case 'findEdgeIntersections':
      return fns.findEdgeIntersections(a[0].map((e) => ({ ...e, points: toPoints(e.points) }))).map((x) => ({
        jumpEdgeId: x.jumpEdgeId,
        otherEdgeId: x.otherEdgeId,
        segIndex: x.segIndex,
        t: x.t,
        point: [x.point.x, x.point.y],
      }));
    case 'processEdgesWithJumps':
      return fns.processEdgesWithJumps(
        a[0].map((e) => ({ ...e, points: toPoints(e.points) })),
        a[1]
      );
    case 'isStraightPath':
      return fns.isStraightPath(a[0]);
    case 'curveSupportsLineHops':
      return fns.curveSupportsLineHops(a[0]);
  }
  throw new Error('unknown pure function ' + c.pure.fn);
};

const makeNode = (spec) => {
  if (!spec) {
    return {};
  }
  if (spec.fixed) {
    const [x, y] = dec(spec.fixed);
    return { intersect: () => ({ x, y }) };
  }
  const node = { ...dec(spec.rect) };
  node.intersect = (point) => intersectRect(node, point);
  return node;
};

const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  const res = { name: c.name };
  out.push(res);
  if (c.pure) {
    try {
      res.result = enc(runPure(c));
    } catch (e) {
      res.error = String((e && e.message) || e);
    }
    continue;
  }
  try {
    const type = c.type || 'flowchart-v2';
    const id = c.id || 'my-svg';
    cfgMod.reset();
    cfgMod.setDiagramConfigScope(type);
    cfgMod.addDirective(c.directive || {});
    const config = cfgMod.getConfig();
    res.config = JSON.parse(JSON.stringify(config));
    res.location = {
      protocol: window.location.protocol,
      host: window.location.host,
      pathname: window.location.pathname,
      search: window.location.search,
    };
    container.innerHTML = '';
    const div = document.createElement('div');
    div.id = 'd' + id;
    div.setAttribute('style', `font-family: ${config.fontFamily}`);
    container.appendChild(div);
    const svgEl = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svgEl.setAttribute('id', id);
    svgEl.setAttribute('width', '100%');
    svgEl.setAttribute('xmlns', 'http://www.w3.org/2000/svg');
    div.appendChild(svgEl);
    const style = document.createElement('style');
    res.css = createUserStyles(config, type, new Map(), '#' + id);
    style.innerHTML = res.css;
    svgEl.appendChild(style);
    const root = select(svgEl).append('g');
    fns.clear();
    globalThis.__mmdgResetRandom();
    if (c.markers) {
      fns.insertMarkers(root, c.markers, type, id);
    }
    const edgePaths = root.insert('g').attr('class', 'edgePaths');
    const edgeLabelsGroup = root.insert('g').attr('class', 'edgeLabels');
    const edges = (c.edges || []).map(dec);
    res.results = [];
    const pathsOf = [];
    let randomPlaced = false;
    for (const [i, op] of (c.ops || []).entries()) {
      const r = {};
      res.results.push(r);
      try {
        switch (op.op) {
          case 'label': {
            const el = await fns.insertEdgeLabel(edgeLabelsGroup, edges[op.edge]);
            r.tag = el.tagName;
            break;
          }
          case 'labels':
            await Promise.all(op.edges.map((k) => fns.insertEdgeLabel(edgeLabelsGroup, edges[k])));
            break;
          case 'edge': {
            if (op.randomAt !== undefined) {
              // Put the random stream where it was in the real render. The
              // replay in Go skips the draws once, before the first edge,
              // so a later edge must follow without a gap.
              if (randomPlaced && globalThis.__mmdgRandomCalls !== op.randomAt) {
                res.randomGap = true;
              }
              if (!randomPlaced || res.randomGap) {
                globalThis.__mmdgResetRandom();
                for (let k = 0; k < op.randomAt; k++) {
                  Math.random();
                }
              }
              randomPlaced = true;
            }
            const clusterDb = new Map(Object.entries(dec(op.clusterDb || {})).map(([k, v]) => [k, { node: v }]));
            const paths = fns.insertEdge(
              edgePaths,
              edges[op.edge],
              clusterDb,
              op.diagramType || type,
              makeNode(op.startNode),
              makeNode(op.endNode),
              op.noDiagramId ? undefined : id,
              op.skipIntersect || false,
              op.layoutAlgorithm
            );
            pathsOf[i] = paths;
            r.paths = {};
            if ('updatedPath' in paths) {
              r.paths.updatedPath = encPoints(paths.updatedPath);
            }
            if ('originalPath' in paths) {
              r.paths.originalPath = encPoints(paths.originalPath);
            }
            break;
          }
          case 'position': {
            let paths = op.paths;
            if (paths && 'fromOp' in paths) {
              paths = pathsOf[paths.fromOp];
            } else if (paths) {
              paths = {};
              if (op.paths.updatedPath) {
                paths.updatedPath = toPoints(dec(op.paths.updatedPath));
              }
              if (op.paths.originalPath) {
                paths.originalPath = toPoints(dec(op.paths.originalPath));
              }
            } else if (paths === null) {
              paths = undefined;
            }
            fns.positionEdgeLabel(edges[op.edge], paths);
            break;
          }
          case 'curve':
            r.curve = enc(fns.resolveEdgeCurveType(edges[op.edge].curve));
            break;
          case 'clear':
            fns.clear();
            break;
          case 'lineJumps':
            fns.applyLineJumpsToSvg(
              edgePaths,
              dec(op.edges).map((e) => ({ ...e, points: toPoints(e.points) })),
              op.config
            );
            break;
          default:
            throw new Error('unknown op ' + op.op);
        }
      } catch (e) {
        r.error = String((e && e.message) || e);
      }
    }
    res.html = root.node().innerHTML;
    res.randomCalls = globalThis.__mmdgRandomCalls;
    res.edgeLabelIds = [...edgeLabels.keys()];
    res.terminalLabelIds = [...terminalLabels.entries()].map(([k, v]) => [k, Object.keys(v)]);
    res.edgesAfter = edges.map((e) => ({
      width: enc(e.width),
      height: enc(e.height),
      labelStyle: enc(e.labelStyle),
      terminalLabelSizes: e.terminalLabelSizes === undefined ? '#undefined' : enc(e.terminalLabelSizes),
      points: e.points === undefined ? '#undefined' : encPoints(e.points),
    }));
  } catch (e) {
    res.error = String((e && e.message) || e);
  }
}
container.innerHTML = '';
return out;
