// Swimlane layout capture for internal/ru/layout/swimlanes. It runs the
// real swimlane layout of mermaid in Chromium and records the input and the
// result of every phase, so that the Go port can be compared step by step.
//
// Run with:
//
//   node oracle.mjs esm-eval probes/swimlanes.js --args cases.json --out out.json
//
// args is { cases: [...] }. A case is one of
//
//   { name, text }            a diagram text: it is rendered with mermaid, so
//                             the layout data has the sizes that the browser
//                             measured
//   { name, data, sizes }     layout data as a diagram db returns it (nodes,
//                             edges, direction, config) and the sizes of the
//                             nodes by id ([width, height]): no DOM is used
//
// The functions of the layout are module bindings and cannot be replaced.
// So the capture runs the phases itself, in the order of
// runSwimlaneLayoutCore, sugiyamaLayout and postProcessSwimlaneLayout, with
// the real function for every phase, on a copy of the layout data. Then it
// runs the real runSwimlaneLayoutCore on the layout data itself and checks
// that both give the same result (the field `consistent`).
//
// A record has
//
//   raw       the layout data before the prepare phase
//   prepared  the layout data after it (lanes, label nodes, virtual edges)
//   input     the layout data when the layout core starts (with sizes)
//   sugiyama  the results of the Sugiyama phases
//   steps     the state of the layout after every later step, as a
//             difference to the step before: [name, changed nodes, changed
//             edges or the whole edge list]
//   issues    the result of validateSwimlanesLayout
//   direct    the results of direct calls of the layering functions with
//             options that the pipeline never passes (see directCalls)
//   error     the message of the error, when the layout throws
//
// A number that JSON cannot hold is { $num: "NaN" }, { $num: "-0" } and so
// on. An undefined value in a list is null.
const NAMES = [
  'toGraphView', 'writeBackToLayoutData', 'normalizeGraph', 'optimizeTopLaneOrder',
  'AUTOMATIC_LANE_ORDERING_RESTARTS', 'removeCycles_DFS', 'assignLayers_LaneAwareCompact',
  'assignLayers_Gravity', 'makeProperLayering', 'orderLayers', 'assignCoordinates', 'LAYERING',
  'sugiyamaLayout', 'routeEdgesOrthogonal', 'validateSwimlanesLayout', 'runSwimlaneLayoutCore',
  'prepareSwimlaneLayout', 'applySwimlaneLineJumps', 'applyLrDirectionTransform',
  'applyBtDirectionTransform', 'orthogonalizePolyline', 'simplifyPolyline', 'simplifyDetouredEdges',
  'straightenCollinearSiblingDetours', 'portSwapToLShape', 'anchorLabelsToPolyline',
  'clipEdgeEndpointsToNodeBoundaries', 'collapseShortTerminalStub', 'nudgeSharedInteriorSubpaths',
  'separateSharedRenderedTerminalLanes', 'collapseRedundantRectangularDoglegs',
  'liftObstacleHuggingSameSideRails', 'swapDestinationTerminalTailsToReduceCrossings',
  'resolveRenderedOrthogonalCrossings', 'reassignCrossingExternalRailChannels',
  'shortcutRedundantOrthogonalJogs', 'prepareEdgeEndpointsForRenderer',
  'liftTopLaneTitleBandsAboveRails', 'shiftLeftLaneTitleBandsLeftOfRails',
  'assignLayers_LongestPath', 'optimizeRanksByCrossings', 'adjustCrossLaneSources',
];
const S = {};
for (const name of NAMES) {
  S[name] = await internalsFrom('swimlanes-', name);
}
const { createCommonLayoutRenderer } = await internals('createCommonLayoutRenderer');

const enc = (v) => {
  if (v === undefined) {
    return null;
  }
  if (typeof v !== 'number') {
    return v;
  }
  if (!Number.isFinite(v)) {
    return { $num: String(v) };
  }
  return Object.is(v, -0) ? { $num: '-0' } : v;
};
const encRecord = (o) => {
  const out = {};
  for (const k of Object.keys(o)) {
    out[k] = enc(o[k]);
  }
  return out;
};
const flat = (points) => {
  const out = [];
  for (const p of points) {
    out.push(enc(p.x), enc(p.y));
  }
  return out;
};

// The properties of a node and an edge that the layout reads or writes.
const snapNode = (n) => ({
  id: n.id,
  parentId: n.parentId,
  isGroup: n.isGroup,
  shape: n.shape,
  label: n.label,
  labelStyle: n.labelStyle,
  look: n.look,
  colorIndex: enc(n.colorIndex),
  padding: enc(n.padding),
  dir: n.dir,
  direction: n.direction,
  isEdgeLabel: n.isEdgeLabel,
  isDummy: n.isDummy,
  edgeStart: n.edgeStart,
  edgeEnd: n.edgeEnd,
  width: enc(n.width),
  height: enc(n.height),
  x: enc(n.x),
  y: enc(n.y),
});
const snapEdge = (e) => ({
  id: e.id,
  start: e.start,
  end: e.end,
  label: e.label,
  text: e.text,
  type: e.type,
  labelStyle: e.labelStyle,
  labelNodeId: e.labelNodeId,
  isLayoutOnly: e.isLayoutOnly,
  curve: e.curve,
  points: e.points ? flat(e.points) : undefined,
});
const snapData = (d) => ({
  direction: d.direction,
  config: {
    look: d.config?.look,
    flowchart: { nodeSpacing: enc(d.config?.flowchart?.nodeSpacing), rankSpacing: enc(d.config?.flowchart?.rankSpacing) },
    swimlane: {
      ignoreCrossLaneEdges: d.config?.swimlane?.ignoreCrossLaneEdges,
      optimizeRanksByCrossings: d.config?.swimlane?.optimizeRanksByCrossings,
      automaticLaneOrdering: d.config?.swimlane?.automaticLaneOrdering,
    },
  },
  nodes: d.nodes.map(snapNode),
  edges: d.edges.map(snapEdge),
});

// The state of the layout: what the steps change.
const nodeState = (n) => [
  enc(n.x),
  enc(n.y),
  enc(n.width),
  enc(n.height),
  n.parentId ?? null,
  enc(n.layer),
  enc(n.order),
  enc(n.swimlaneContentTop),
  n.groupTitleRect
    ? [enc(n.groupTitleRect.left), enc(n.groupTitleRect.right), enc(n.groupTitleRect.top), enc(n.groupTitleRect.bottom)]
    : null,
];
const edgeState = (e) => [e.id, e.curve ?? null, e.points ? flat(e.points) : null];
const stateOf = (d) => ({
  nodes: d.nodes.map((n) => JSON.stringify(nodeState(n))),
  edges: d.edges.map((e) => JSON.stringify(edgeState(e))),
  edgeIds: d.edges.map((e) => e.id),
});
// A step is [name, [[node index, state]...], edges]. edges is the whole
// list when the edges of the layout changed, and [[edge index, state]...]
// in an object { d: [...] } otherwise.
const diffStep = (name, prev, next) => {
  const nodes = [];
  for (let i = 0; i < next.nodes.length; i++) {
    if (!prev || prev.nodes[i] !== next.nodes[i]) {
      nodes.push([i, JSON.parse(next.nodes[i])]);
    }
  }
  let edges;
  if (!prev || JSON.stringify(prev.edgeIds) !== JSON.stringify(next.edgeIds)) {
    edges = next.edges.map((e) => JSON.parse(e));
  } else {
    const d = [];
    for (let i = 0; i < next.edges.length; i++) {
      if (prev.edges[i] !== next.edges[i]) {
        d.push([i, JSON.parse(next.edges[i])]);
      }
    }
    edges = { d };
  }
  return [name, nodes, edges];
};

const graphSnap = (g) => ({ nodes: [...g.nodes], edges: g.edges.map((e) => [e.id, e.src, e.dst]) });
const layersSnap = (layers) => layers.map((l) => [...l]);

// The pipeline calls the layering functions with a few fixed options, so
// parts of them never run in a layout. directCalls runs them on the acyclic
// graph of the case with other options:
//
//   longestPath  assignLayers_LongestPath for each entry of LP_OPTIONS
//   gravity      assignLayers_Gravity with ignoreCrossLaneEdges
//   compact      assignLayers_LaneAwareCompact without ignoreCrossLaneEdges
//   optimize     optimizeRanksByCrossings for ranks with slack (loose)
//   adjust       adjustCrossLaneSources for the same ranks
//
// A call that throws is { error }.
const LP_OPTIONS = [
  null,
  { ignoreCrossLaneEdges: true, compactSingleInput: true, optimizeRanksByCrossings: true },
  { ignoreCrossLaneEdges: true, compactSingleInput: false, optimizeRanksByCrossings: false },
  { ignoreCrossLaneEdges: false, compactSingleInput: true, optimizeRanksByCrossings: false },
];
const directCalls = (gAcyclic, direction) => {
  const tryCall = (fn) => {
    try {
      return fn();
    } catch (e) {
      return { error: String((e && e.message) || e) };
    }
  };
  const layeringSnap = (l) => ({ layers: layersSnap(l.layers), rankOf: { ...l.rankOf } });
  const direct = {};
  direct.longestPath = LP_OPTIONS.map((o) =>
    tryCall(() => layeringSnap(S.assignLayers_LongestPath(gAcyclic, o ?? undefined)))
  );
  direct.gravity = tryCall(() =>
    layeringSnap(
      S.assignLayers_Gravity(gAcyclic, {
        ignoreCrossLaneEdges: true,
        compactSingleInput: true,
        optimizeRanksByCrossings: true,
      })
    )
  );
  direct.compact = tryCall(() =>
    layeringSnap(S.assignLayers_LaneAwareCompact(gAcyclic, { ignoreCrossLaneEdges: false, direction }))
  );
  // Ranks with slack: twice the longest-path rank plus 0, 1 or 2.
  const base = direct.longestPath[0];
  if (!base.error) {
    const gn = S.normalizeGraph(gAcyclic);
    const loose = {};
    let k = 0;
    for (const v of gn.nodes) {
      loose[v] = (base.rankOf[v] ?? 0) * 2 + (k++ % 3);
    }
    direct.loose = { ...loose };
    direct.optimize = tryCall(() => ({ ...S.optimizeRanksByCrossings(gn, loose) }));
    direct.adjust = tryCall(() => {
      const ranks = { ...loose };
      S.adjustCrossLaneSources(gn, ranks);
      return ranks;
    });
  }
  return direct;
};

const cloneData = (d) => ({
  ...d,
  nodes: d.nodes.map((n) => ({ ...n, ...(n.groupTitleRect ? { groupTitleRect: { ...n.groupTitleRect } } : {}) })),
  edges: d.edges.map((e) => ({ ...e, ...(e.points ? { points: e.points.map((p) => ({ ...p })) } : {}) })),
});

// traceCore is runSwimlaneLayoutCore with a record of every step.
const traceCore = (data) => {
  const rec = { steps: [] };
  let prev = null;
  const push = (name) => {
    const next = stateOf(data);
    rec.steps.push(diffStep(name, prev, next));
    prev = next;
  };
  // The first step has the whole state.
  push('input');

  const g = S.toGraphView(data);
  const nodeGap = data.config.flowchart?.nodeSpacing ?? 40;
  const layerGap = data.config.flowchart?.rankSpacing ?? 100;
  const ignoreCrossLaneEdges = data.config.swimlane?.ignoreCrossLaneEdges ?? true;
  const optimizeRanksByCrossings = data.config.swimlane?.optimizeRanksByCrossings ?? true;
  const automaticLaneOrdering = data.config.swimlane?.automaticLaneOrdering ?? false;
  const direction = data.direction ?? 'TB';
  const opts = { nodeGap, layerGap, ignoreCrossLaneEdges, optimizeRanksByCrossings, automaticLaneOrdering, direction };

  // sugiyamaLayout
  const sug = {};
  sug.view = graphSnap(g);
  const g0 = S.normalizeGraph(g);
  sug.normalized = graphSnap(g0);
  const laneOrder = opts.automaticLaneOrdering
    ? S.optimizeTopLaneOrder(g0, { restarts: S.AUTOMATIC_LANE_ORDERING_RESTARTS })
    : undefined;
  sug.laneOrder = laneOrder ? [...laneOrder] : null;
  const cycleRes = S.removeCycles_DFS(g0);
  const gAcyclic = cycleRes.acyclic;
  sug.acyclic = gAcyclic.edges.map((e) => [e.id, e.src, e.dst]);
  sug.reversed = cycleRes.reversed.map((e) => [e.id, e.src, e.dst]);
  rec.direct = directCalls(gAcyclic, opts.direction);
  const layering = (opts.ignoreCrossLaneEdges ?? true)
    ? S.assignLayers_LaneAwareCompact(gAcyclic, {
        compactSingleInput: opts.compactSingleInput ?? S.LAYERING.DEFAULT_COMPACT_SINGLE_INPUT,
        ignoreCrossLaneEdges: true,
        direction: opts.direction,
      })
    : S.assignLayers_Gravity(gAcyclic, {
        compactSingleInput: opts.compactSingleInput ?? S.LAYERING.DEFAULT_COMPACT_SINGLE_INPUT,
        ignoreCrossLaneEdges: false,
        optimizeRanksByCrossings: opts.optimizeRanksByCrossings ?? true,
      });
  sug.initial = { layers: layersSnap(layering.layers), rankOf: { ...layering.rankOf } };
  const { layering: properLayering, graphWithDummies } = S.makeProperLayering(layering, gAcyclic);
  sug.proper = {
    layers: layersSnap(properLayering.layers),
    rankOf: { ...properLayering.rankOf },
    dummy: [...properLayering.dummy],
  };
  sug.withDummies = graphSnap(graphWithDummies);
  const ordered = S.orderLayers(properLayering, graphWithDummies, { laneOrder });
  sug.ordered = layersSnap(ordered.layers);
  const coordinates = S.assignCoordinates(ordered, graphWithDummies, {
    layerGap: opts.layerGap,
    nodeGap: opts.nodeGap,
    direction: opts.direction,
    laneOrder,
  });
  sug.coords = { x: encRecord(coordinates.x), y: encRecord(coordinates.y) };
  rec.sugiyama = sug;

  S.writeBackToLayoutData(g, ordered, coordinates, { nodeGap, layerGap });
  push('writeBack');

  for (const edge of data.edges ?? []) {
    delete edge.points;
  }
  S.routeEdgesOrthogonal(data, direction);
  push('route');

  for (const edge of data.edges ?? []) {
    if (!edge.curve || edge.curve === 'basis') {
      edge.curve = 'rounded';
    }
  }

  // postProcessSwimlaneLayout
  const post = () => {
    const layout = data;
    const nodes = layout.nodes ?? [];
    const edges = layout.edges ?? [];
    const contentNodes = nodes.filter((n) => !n.isGroup);
    if (
      (direction === 'LR' || direction === 'RL') &&
      contentNodes.length > 0 &&
      !S.applyLrDirectionTransform(layout, direction)
    ) {
      return;
    }
    if (direction === 'BT' && contentNodes.length > 0 && !S.applyBtDirectionTransform(layout)) {
      return;
    }
    push('direction');
    for (const edge of edges) {
      if (edge.isLayoutOnly) {
        continue;
      }
      const pts = edge.points;
      if (!pts || pts.length < 2) {
        continue;
      }
      edge.points = S.simplifyPolyline(S.orthogonalizePolyline(pts));
    }
    push('orthogonalize');
    S.simplifyDetouredEdges(edges, nodes);
    push('simplifyDetouredEdges');
    S.straightenCollinearSiblingDetours(edges, nodes);
    push('straightenCollinearSiblingDetours');
    S.portSwapToLShape(edges, nodes);
    push('portSwapToLShape');
    const nodeByIdMap = new Map();
    for (const n of nodes) {
      nodeByIdMap.set(String(n.id), n);
    }
    S.anchorLabelsToPolyline(edges, nodeByIdMap);
    push('anchorLabelsToPolyline');
    S.clipEdgeEndpointsToNodeBoundaries(edges, nodeByIdMap);
    push('clipEdgeEndpointsToNodeBoundaries');
    S.collapseShortTerminalStub(edges, nodeByIdMap);
    push('collapseShortTerminalStub');
    S.nudgeSharedInteriorSubpaths(edges, nodeByIdMap);
    push('nudgeSharedInteriorSubpaths');
    S.separateSharedRenderedTerminalLanes(edges, nodeByIdMap);
    push('separateSharedRenderedTerminalLanes');
    S.collapseRedundantRectangularDoglegs(edges, nodeByIdMap);
    push('collapseRedundantRectangularDoglegs');
    S.liftObstacleHuggingSameSideRails(edges, nodeByIdMap);
    push('liftObstacleHuggingSameSideRails');
    S.swapDestinationTerminalTailsToReduceCrossings(edges, nodeByIdMap);
    push('swapDestinationTerminalTailsToReduceCrossings');
    const finalizeRenderedEdges = (round) => {
      S.resolveRenderedOrthogonalCrossings(edges, nodeByIdMap);
      push(round + '.resolveRenderedOrthogonalCrossings');
      S.separateSharedRenderedTerminalLanes(edges, nodeByIdMap);
      push(round + '.separateSharedRenderedTerminalLanes');
      S.reassignCrossingExternalRailChannels(edges, nodeByIdMap);
      push(round + '.reassignCrossingExternalRailChannels');
      S.shortcutRedundantOrthogonalJogs(edges, nodeByIdMap);
      push(round + '.shortcutRedundantOrthogonalJogs');
      S.anchorLabelsToPolyline(edges, nodeByIdMap);
      push(round + '.anchorLabelsToPolyline1');
      S.prepareEdgeEndpointsForRenderer(edges, nodeByIdMap);
      push(round + '.prepareEdgeEndpointsForRenderer1');
      S.liftObstacleHuggingSameSideRails(edges, nodeByIdMap);
      push(round + '.liftObstacleHuggingSameSideRails');
      S.anchorLabelsToPolyline(edges, nodeByIdMap);
      push(round + '.anchorLabelsToPolyline2');
      S.prepareEdgeEndpointsForRenderer(edges, nodeByIdMap);
      push(round + '.prepareEdgeEndpointsForRenderer2');
    };
    finalizeRenderedEdges('finalize1');
    S.nudgeSharedInteriorSubpaths(edges, nodeByIdMap);
    push('nudgeSharedInteriorSubpaths2');
    finalizeRenderedEdges('finalize2');
    S.liftTopLaneTitleBandsAboveRails(edges, nodeByIdMap, direction);
    push('liftTopLaneTitleBandsAboveRails1');
    S.shiftLeftLaneTitleBandsLeftOfRails(edges, nodeByIdMap, direction);
    push('shiftLeftLaneTitleBandsLeftOfRails1');
    S.liftTopLaneTitleBandsAboveRails(edges, nodeByIdMap, direction);
    push('liftTopLaneTitleBandsAboveRails2');
    S.shiftLeftLaneTitleBandsLeftOfRails(edges, nodeByIdMap, direction);
    push('shiftLeftLaneTitleBandsLeftOfRails2');
  };
  post();

  rec.issues = S.validateSwimlanesLayout(data).map((i) => [i.type, i.edgeId, i.targetId, i.detail]);
  push('end');
  rec.finalState = prev;
  return rec;
};

// runBoth records the trace on a copy and runs the real layout core on the
// data itself.
const runBoth = (data, record) => {
  record.input = snapData(data);
  const copy = cloneData(data);
  let traceError;
  try {
    const rec = traceCore(copy);
    record.sugiyama = rec.sugiyama;
    record.direct = rec.direct;
    record.steps = rec.steps;
    record.issues = rec.issues;
    record.traceFinal = rec.finalState;
  } catch (e) {
    traceError = String((e && e.message) || e);
  }
  let coreError;
  let result;
  try {
    result = S.runSwimlaneLayoutCore(data);
  } catch (e) {
    coreError = String((e && e.message) || e);
  }
  if (coreError !== undefined) {
    record.error = coreError;
    record.consistent = traceError === coreError;
    delete record.traceFinal;
    if (traceError !== coreError) {
      record.traceError = traceError ?? null;
    }
    throw Object.assign(new Error(coreError), { layoutError: true });
  }
  if (traceError !== undefined) {
    record.traceError = traceError;
    record.consistent = false;
    return result;
  }
  const real = stateOf(data);
  record.consistent = JSON.stringify(real) === JSON.stringify(record.traceFinal);
  delete record.traceFinal;
  return result;
};

const records = [];
let current = null;

// The layout that mermaid runs for a diagram text: the real phases, with
// the capture around them.
const hookedRender = createCommonLayoutRenderer({
  prepareLayout: (data4Layout) => {
    current.raw = snapData(data4Layout);
    S.prepareSwimlaneLayout(data4Layout);
    current.prepared = snapData(data4Layout);
  },
  runLayoutCore: (data4Layout) => runBoth(data4Layout, current),
  afterPaint: S.applySwimlaneLineJumps,
});
mermaid.registerLayoutLoaders([{ name: 'swimlane', loader: async () => ({ render: hookedRender }) }]);

for (const c of args.cases) {
  const record = { name: c.name, kind: c.text !== undefined ? 'render' : 'data' };
  current = record;
  records.push(record);
  if (c.text !== undefined) {
    // One container per diagram. The config is reset, so that the
    // frontmatter of one diagram does not reach the next.
    const container = document.getElementById('container');
    container.innerHTML = '';
    try {
      mermaid.mermaidAPI.reset();
      mermaid.initialize({ startOnLoad: false });
      // The page of the reference renders has drawn one random number when
      // the render starts.
      globalThis.__mmdgResetRandom();
      Math.random();
      const { svg, diagramType } = await mermaid.render('my-svg', c.text, container);
      if (c.keepSvg) {
        record.svg = svg;
        record.diagramType = diagramType;
      }
      if (record.input === undefined) {
        record.skipped = 'the diagram did not run the swimlane layout';
      }
    } catch (e) {
      if (!(e && e.layoutError)) {
        record.renderError = String((e && e.message) || e);
      }
      if (c.keepSvg) {
        // The error that mermaid.render rejects with.
        record.outcomeError = String((e && e.message) || e);
      }
    }
    document.querySelectorAll('#dmy-svg, #my-svg, [id^="d"][id$="my-svg"]').forEach((el) => el.remove());
    continue;
  }
  const data = {
    ...c.data,
    type: 'swimlane',
    layoutAlgorithm: 'swimlane',
    nodes: c.data.nodes.map((n) => ({ ...n })),
    edges: c.data.edges.map((e) => ({ ...e })),
  };
  try {
    record.raw = snapData(data);
    S.prepareSwimlaneLayout(data);
    for (const n of data.nodes) {
      const size = c.sizes[n.id];
      if (size) {
        n.width = size[0];
        n.height = size[1];
      }
    }
    record.prepared = snapData(data);
    runBoth(data, record);
  } catch (e) {
    if (!(e && e.layoutError)) {
      record.error = String((e && e.message) || e);
    }
  }
}
return { records };
