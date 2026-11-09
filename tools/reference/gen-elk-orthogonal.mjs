// Oracle for internal/elk/native/p5edges/orthogonal: runs
// OrthogonalRoutingGenerator.routeEdges of the unminified elkjs 0.9.3 and
// records the input and the result of each call.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-orthogonal.mjs
//
// The golden is internal/elk/native/p5edges/orthogonal/testdata/routeedges.json.gz.
// It has two kinds of groups:
//
//   - "synthetic": the script builds LNode, LPort and LEdge objects in the
//     worker and calls routeEdges on them, for the three routing directions.
//     The coordinates come from a small lattice with jitter, so that the
//     calls meet conflicts, critical conflicts, critical cycles and the
//     hyper edge segment splitter. A part of the calls is constructed for
//     one branch of the code: exact coordinates on the limits of the
//     splitter, thresholds that are smaller than the rounding of the
//     coordinates, and coordinates that are NaN or an infinity. The build
//     functions say which branch each one is for.
//   - "layout": the script lays out random graphs with hierarchical ports
//     and records the calls that the orthogonal edge router and the
//     hierarchical port orthogonal edge router make.
//
// A group is the calls of one OrthogonalRoutingGenerator object, in order:
//
//   { kind, dir, spacing, calls: [call] }
//   call   { start, rnd [hi, lo] | null, src [node] | null, tgt [node] | null,
//            nodes [{ pos, ports [{ sd, pos, an, in [edge], out [edge] }] }],
//            edges [{ s [node, port], t [node, port], bp, jp | null }],
//            ret, rndAfter, after [{ bp, jp | null }] }
//          or, for a call that makes the worker throw, the input with
//          "throws": true and without ret, rndAfter and after
//
// dir is the ordinal of RoutingDirection. sd is the ordinal of PortSide. bp
// is the list of bend points and jp the value of the property junctionPoints
// (null without a value), before the call in edges and after it in after. A
// number is a JSON number, or {"$f": "-0" | "NaN" | "Infinity" | "-Infinity"}.
import { gzipSync } from 'node:zlib';
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk, env } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const outFile = resolve(root, 'internal/elk/native/p5edges/orthogonal/testdata/routeedges.json.gz');

const SYNTHETIC_GROUPS = 2400;
// The parts of the synthetic calls that buildBoundaryCall, buildRegionCall,
// buildCoverCall, buildUlpCall and buildSplitByCall make.
const BOUNDARY_SHARE = 0.13;
const REGION_SHARE = 0.08;
const COVER_SHARE = 0.08;
const ULP_SHARE = 0.1;
const SPLIT_BY_SHARE = 0.04;
// The part of the calls of buildCall that has coordinates that are NaN, an
// infinity or -0.
const SPECIAL_SHARE = 0.06;
const LAYOUT_GRAPHS = 160;
// The largest number of calls of the west to east direction that the layouts
// add. The phase goldens of OrthogonalEdgeRouter cover that direction.
const MAX_LAYOUT_WEST_EAST_CALLS = 80;

// ---------------------------------------------------------------------------
// A seeded generator for the script itself. The environment of the worker
// replaces Math.random.

function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rng = mulberry32(0x0e1c0d15);
const int = (n) => Math.floor(rng() * n);
const pick = (list) => list[int(list.length)];
const chance = (p) => rng() < p;

// ---------------------------------------------------------------------------
// The engine.

env.install();
const elk = loadElk();
const S = elk.hook.scope;

// One layout runs the class initializers of the layered algorithm.
env.beginLayout();
elk.layout({
  id: 'root',
  layoutOptions: { 'elk.algorithm': 'layered' },
  children: [
    { id: 'a', width: 10, height: 10 },
    { id: 'b', width: 10, height: 10 },
  ],
  edges: [{ id: 'e', sources: ['a'], targets: ['b'] }],
});

// W is the access to the worker scope.
const W = S(`({
  LGraph: LGraph, LNode: LNode, LPort: LPort, LEdge: LEdge, KVector_1: KVector_1, KVectorChain: KVectorChain,
  ArrayList: ArrayList, Random_0: Random_0, OrthogonalRoutingGenerator: OrthogonalRoutingGenerator,
  add: $add_3, addLast: $add_7, setProperty: $setProperty_0, get: $get_10, routeEdges: $routeEdges_0,
  RANDOM: RANDOM_0, JUNCTION_POINTS: JUNCTION_POINTS,
  sides: [UNDEFINED_5, NORTH_3, EAST_2, SOUTH_2, WEST_2],
})`);

// num keeps a double exact in JSON.
function num(x) {
  if (Number.isFinite(x) && !Object.is(x, -0)) return x;
  return { $f: Object.is(x, -0) ? '-0' : String(x) };
}
const vec = (v) => [num(v.x_0), num(v.y_0)];
const iterate = (coll) => {
  const out = [];
  for (const it = coll.iterator_0(); it.hasNext_0(); ) out.push(it.next_1());
  return out;
};
const junctionPoints = (edge) => {
  const v = edge.propertyMap ? W.get(edge.propertyMap, W.JUNCTION_POINTS) : null;
  return v === null || v === undefined ? null : iterate(v).map(vec);
};
const randomState = (graph) => {
  const r = graph.propertyMap ? W.get(graph.propertyMap, W.RANDOM) : null;
  return r === null || r === undefined ? null : [r.seedhi, r.seedlo];
};
const DIR_OF_SOURCE_SIDE = { EAST: 0, SOUTH: 1, NORTH: 2 };

// snapshot returns the input of a call of routeEdges: the nodes of the two
// layers and each node that their edges reach, with the ports and the edges.
function snapshot(graph, src, tgt, startPos) {
  const nodes = [];
  const nodeIds = new Map();
  const edges = [];
  const edgeIds = new Map();
  const nodeId = (n) => {
    let id = nodeIds.get(n);
    if (id === undefined) {
      id = nodes.length;
      nodeIds.set(n, id);
      nodes.push(n);
    }
    return id;
  };
  const edgeId = (e) => {
    let id = edgeIds.get(e);
    if (id === undefined) {
      id = edges.length;
      edgeIds.set(e, id);
      edges.push(e);
    }
    return id;
  };
  const srcIds = src ? iterate(src).map(nodeId) : null;
  const tgtIds = tgt ? iterate(tgt).map(nodeId) : null;
  const outNodes = [];
  for (let i = 0; i < nodes.length; i++) {
    const n = nodes[i];
    const ports = n.ports.array.map((p) => {
      for (const e of p.incomingEdges.array) nodeId(e.source.owner);
      for (const e of p.outgoingEdges.array) nodeId(e.target.owner);
      return {
        sd: p.side.ordinal,
        pos: vec(p.pos),
        an: vec(p.anchor),
        in: p.incomingEdges.array.map(edgeId),
        out: p.outgoingEdges.array.map(edgeId),
      };
    });
    outNodes.push({ pos: vec(n.pos), ports });
  }
  const portRef = (p) => [nodeIds.get(p.owner), p.owner.ports.array.indexOf(p)];
  return {
    call: {
      start: num(startPos),
      rnd: randomState(graph),
      src: srcIds,
      tgt: tgtIds,
      nodes: outNodes,
      edges: edges.map((e) => ({ s: portRef(e.source), t: portRef(e.target), bp: iterate(e.bendPoints).map(vec), jp: junctionPoints(e) })),
    },
    edges,
  };
}

// finish adds the result of the call to its snapshot.
function finish(snap, graph, ret) {
  snap.call.ret = ret;
  snap.call.rndAfter = randomState(graph);
  snap.call.after = snap.edges.map((e) => ({ bp: iterate(e.bendPoints).map(vec), jp: junctionPoints(e) }));
  return snap.call;
}

// ---------------------------------------------------------------------------
// Synthetic calls.

const SIDE = { NORTH: 1, EAST: 2, SOUTH: 3, WEST: 4 };
const DIRS = [
  { src: SIDE.EAST, tgt: SIDE.WEST, vertical: true },
  { src: SIDE.SOUTH, tgt: SIDE.NORTH, vertical: false },
  { src: SIDE.NORTH, tgt: SIDE.SOUTH, vertical: false },
];

// buildCall makes the objects of one call in the worker. The coordinate of a
// port on the axis of the hyper edge segments is a lattice value with
// jitter.
function buildCall(dir, spacing) {
  const d = DIRS[dir];
  const step = pick([10, 10, 5, 2.5, 7, 20, 1, spacing / 2, spacing / 4, spacing]);
  const span = 2 + int(pick([3, 5, 8, 14]));
  const jitterMode = pick(['none', 'none', 'none', 'tiny', 'ulp', 'tolerance', 'threshold', 'fifth', 'fifth', 'free', 'mixed']);
  // A few calls have coordinates that are not finite numbers, or -0.
  const special = chance(SPECIAL_SHARE);
  const jitter = () => {
    const mode =
      jitterMode === 'mixed'
        ? pick(['none', 'none', 'tiny', 'tolerance', 'threshold', 'fifth', 'free'])
        : jitterMode === 'fifth' && chance(0.7)
          ? 'none'
          : jitterMode;
    switch (mode) {
      case 'fifth':
        // The critical conflict threshold is a fifth of the smallest distance
        // between two connections of one side, and a free area starts and
        // ends at that distance from a connection. An offset of a few fifths
        // of the lattice step puts connections exactly on those limits.
        return pick([0.2, 0.4, 0.4, 0.6, 0.8, -0.2, -0.4, -0.4, 1.2, 1.4]) * step;
      case 'tiny':
        return pick([0, 0, 1e-12, -1e-12, 4e-16]);
      case 'tolerance':
        return pick([0, 0, 0.0005, -0.0005, 0.001, -0.001, 0.0011, 0.002, -0.002]);
      case 'threshold':
        return pick([0, 0, 0.1, -0.1, 0.2, -0.2, 0.5, 1]) * step * pick([1, 1, 0.5, 0.25]);
      case 'free':
        return rng() * step;
      default:
        return 0;
    }
  };
  // In the mode ulp a coordinate is a lattice value or one of the doubles
  // next to it. The smallest distance of a side is then a few units in the
  // last place, and the thresholds are smaller than the rounding of a sum.
  const coord = () => {
    if (special && chance(0.3)) return pick([NaN, NaN, Infinity, Infinity, -Infinity, -0]);
    const c = int(span) * step + jitter();
    return jitterMode === 'ulp' ? c * (1 + pick([0, 0, 0, 1, -1, 2]) * Number.EPSILON) : c;
  };

  const graph = new W.LGraph();
  let seeded = true;
  if (chance(0.97)) {
    const random = new W.Random_0(1);
    random.seedhi = int(1 << 24);
    random.seedlo = int(1 << 24);
    W.setProperty(graph, W.RANDOM, random);
  } else {
    seeded = false;
  }

  // A node, with a base position that the ports are relative to. A base of 0
  // and an anchor of 0 give the lattice value without rounding.
  const makeNode = () => {
    const node = new W.LNode(graph);
    const base = chance(0.5) ? 0 : pick([10, 30.5, 77, rng() * 200, -15]);
    const other = pick([0, 100, 37.25, rng() * 300]);
    if (d.vertical) {
      node.pos.x_0 = other;
      node.pos.y_0 = base;
    } else {
      node.pos.x_0 = base;
      node.pos.y_0 = other;
    }
    return { node, base, ports: { src: [], tgt: [], odd: [] } };
  };
  const makePort = (n, side) => {
    const port = new W.LPort();
    port.side = W.sides[side];
    const anchor = chance(0.6) ? 0 : pick([2.5, 1, 4, rng() * 5]);
    const c = coord();
    const along = c - n.base - anchor;
    const across = pick([0, 0, 5, rng() * 30]);
    if (d.vertical) {
      port.pos.x_0 = across;
      port.pos.y_0 = along;
      port.anchor.x_0 = pick([0, 2.5]);
      port.anchor.y_0 = anchor;
    } else {
      port.pos.x_0 = along;
      port.pos.y_0 = across;
      port.anchor.x_0 = anchor;
      port.anchor.y_0 = pick([0, 2.5]);
    }
    port.owner = n.node;
    W.add(n.node.ports, port);
    const kind = side === d.src ? 'src' : side === d.tgt ? 'tgt' : 'odd';
    n.ports[kind].push(port);
    return port;
  };

  const layer = (count, side) => {
    const out = [];
    for (let i = 0; i < count; i++) {
      const n = makeNode();
      const ports = 1 + int(pick([1, 2, 3, 4, 6]));
      for (let k = 0; k < ports; k++) {
        // Most ports are on the side that faces the other layer.
        const s = chance(0.9) ? side : pick([SIDE.NORTH, SIDE.EAST, SIDE.SOUTH, SIDE.WEST]);
        makePort(n, s);
      }
      out.push(n);
    }
    return out;
  };
  const srcNodes = layer(pick([0, 1, 1, 2, 2, 3, 4, 6]), d.src);
  const tgtNodes = layer(pick([0, 1, 1, 2, 2, 3, 4, 6]), d.tgt);
  // Nodes that are in no layer, as the nodes that the edges of hierarchical
  // ports reach.
  const otherNodes = chance(0.15) ? layer(1 + int(2), pick([d.src, d.tgt])) : [];

  const portsOf = (nodes, kinds) => nodes.flatMap((n) => kinds.flatMap((k) => n.ports[k]));
  const srcPorts = portsOf(srcNodes, ['src']);
  const tgtPorts = portsOf(tgtNodes, ['tgt']);
  const allPorts = portsOf([...srcNodes, ...tgtNodes, ...otherNodes], ['src', 'tgt', 'odd']);

  const connect = (source, target) => {
    if (!source || !target || source === target) return;
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
    // An edge can have bend points and junction points before the call.
    if (chance(0.03)) W.addLast(edge.bendPoints, new W.KVector_1(rng() * 50, rng() * 50));
    if (chance(0.02)) {
      const chain = new W.KVectorChain();
      if (chance(0.5)) W.addLast(chain, new W.KVector_1(int(5) * step, int(5) * step));
      W.setProperty(edge, W.JUNCTION_POINTS, chain);
    }
  };

  const shuffle = (list) => {
    const a = list.slice();
    for (let i = a.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [a[i], a[j]] = [a[j], a[i]];
    }
    return a;
  };

  const mode = pick(['matching', 'matching', 'matching', 'random', 'random', 'hyper', 'mixed']);
  if (srcPorts.length > 0 && tgtPorts.length > 0) {
    if (mode === 'matching' || mode === 'mixed') {
      // Each port has one edge at most: crossings between plain edges, which
      // give the critical cycles when the two sides share coordinates.
      const a = shuffle(srcPorts);
      const b = shuffle(tgtPorts);
      const count = Math.min(a.length, b.length) - (chance(0.3) ? int(2) : 0);
      for (let i = 0; i < count; i++) {
        if (chance(0.12)) connect(b[i], a[i]);
        else connect(a[i], b[i]);
      }
    }
    if (mode === 'random' || mode === 'mixed') {
      const count = int(srcPorts.length + tgtPorts.length + 1);
      for (let i = 0; i < count; i++) {
        if (chance(0.15)) connect(pick(tgtPorts), pick(srcPorts));
        else connect(pick(srcPorts), pick(tgtPorts));
      }
    }
    if (mode === 'hyper') {
      // A few ports with many edges: hyper edges.
      const hubs = 1 + int(3);
      for (let h = 0; h < hubs; h++) {
        const hub = pick(chance(0.7) ? srcPorts : tgtPorts);
        const fan = 2 + int(4);
        for (let i = 0; i < fan; i++) {
          const other = pick(chance(0.8) ? (srcPorts.includes(hub) ? tgtPorts : srcPorts) : allPorts);
          if (chance(0.25)) connect(other, hub);
          else connect(hub, other);
        }
      }
      const count = int(4);
      for (let i = 0; i < count; i++) connect(pick(srcPorts), pick(tgtPorts));
    }
  }
  // Edges of other kinds: inside a layer, self loops, ports of other sides.
  if (allPorts.length > 1) {
    const extra = chance(0.35) ? 1 + int(3) : 0;
    for (let i = 0; i < extra; i++) connect(pick(allPorts), pick(allPorts));
  }
  // Edges between two new ports of the target side. Such a segment has no
  // connection on the source side, and the topological numbering moves it to
  // the last routing slot when no segment must be after it.
  if (tgtNodes.length > 0 && chance(0.3)) {
    const inLayer = 1 + int(2);
    for (let i = 0; i < inLayer; i++) connect(makePort(pick(tgtNodes), d.tgt), makePort(pick(tgtNodes), d.tgt));
  }

  const list = (nodes) => {
    const l = new W.ArrayList();
    for (const n of nodes) W.add(l, n.node);
    return l;
  };
  // A layer can be null, as the layer before the first one.
  const src = srcNodes.length === 0 && chance(0.7) ? null : list(srcNodes);
  const tgt = tgtNodes.length === 0 && chance(0.7) ? null : list(tgtNodes);
  const startPos = pick([0, 10, 42.5, -20, rng() * 500]);
  return { graph, src, tgt, startPos, seeded };
}

// buildBoundaryCall makes a call whose coordinates are exact: the nodes are at
// 0 and the anchors are 0, so the coordinate of a port is its lattice value.
// The call has pairs of edges that cross between the same two lattice values
// (a cycle of critical dependencies, which the splitter breaks). One pair is
// between two neighbor values, so the smallest distance of each side is the
// lattice step, and the critical conflict threshold is a fifth of the step.
// Marker edges have one end at a whole number of thresholds from a
// connection of the other side, and no connection of their own side is
// nearer than the step. Such an end is exactly on a limit of the splitter:
// two thresholds give a free area of size 0, and four thresholds give a free
// area that is exactly as large as two thresholds.
function buildBoundaryCall(dir) {
  const d = DIRS[dir];
  const step = pick([10, 10, 5, 20, 2.5]);
  const threshold = 0.2 * step;
  const graph = new W.LGraph();
  const random = new W.Random_0(1);
  random.seedhi = int(1 << 24);
  random.seedlo = int(1 << 24);
  W.setProperty(graph, W.RANDOM, random);

  const nodes = { src: [], tgt: [] };
  const side = { src: d.src, tgt: d.tgt };
  // The lattice indices that have a port of a side, and the indices where a
  // port of the side would be nearer to a marker than the step.
  const used = { src: new Set(), tgt: new Set() };
  const blocked = { src: new Set(), tgt: new Set() };
  const free = (which, k) => k >= 0 && !used[which].has(k) && !blocked[which].has(k);
  const makePort = (which, c) => {
    const node = new W.LNode(graph);
    const port = new W.LPort();
    port.side = W.sides[side[which]];
    if (d.vertical) port.pos.y_0 = c;
    else port.pos.x_0 = c;
    port.owner = node;
    W.add(node.ports, port);
    nodes[which].push(node);
    return port;
  };
  const latticePort = (which, k) => {
    used[which].add(k);
    return makePort(which, k * step);
  };
  const connect = (source, target) => {
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
  };
  const crossingPair = (k1, k2) => {
    if (k1 === k2 || !free('src', k1) || !free('src', k2) || !free('tgt', k1) || !free('tgt', k2)) return;
    const a1 = latticePort('src', k1);
    const a2 = latticePort('src', k2);
    const b1 = latticePort('tgt', k1);
    const b2 = latticePort('tgt', k2);
    connect(a1, b2);
    connect(a2, b1);
  };
  // crossingCycle makes a cycle of critical dependencies between three or
  // more segments: the edge of each lattice value goes to the next value,
  // and the last one to the first. The segments have different extents, so
  // the segment that causes a split covers a part of the free areas of the
  // segment that is split.
  const crossingCycle = (ks) => {
    if (new Set(ks).size !== ks.length || ks.some((k) => !free('src', k) || !free('tgt', k))) return;
    const a = ks.map((k) => latticePort('src', k));
    const b = ks.map((k) => latticePort('tgt', k));
    for (let i = 0; i < ks.length; i++) connect(a[i], b[(i + 1) % ks.length]);
  };

  const size = 8 + int(10);
  const first = int(size - 1);
  crossingPair(first, first + 1);

  // The markers come before the other pairs, so that they find free indices.
  const markers = 1 + int(4);
  for (let i = 0; i < markers; i++) {
    const which = chance(0.5) ? 'src' : 'tgt';
    const other = which === 'src' ? 'tgt' : 'src';
    const j = int(size);
    const factor = pick([2, 2, 2, 4, 4, 4, 1, 3, -2, -2, -4, -4, -1]);
    // The marker is between the indices j and j + 1, or j - 1 and j.
    const neighbor = factor > 0 ? j + 1 : j - 1;
    if (neighbor < 0 || !free(which, j) || !free(which, neighbor)) continue;
    // The other end of the marker edge: a free index of the other side.
    const far = int(size);
    if (far === j || !free(other, far)) continue;
    // The connection of the other side that the marker is measured from.
    if (!used[other].has(j)) {
      const partner = int(size);
      if (!free(other, j) || partner === j || partner === neighbor || !free(which, partner)) continue;
      const a = latticePort(other, j);
      const b = latticePort(which, partner);
      if (other === 'src') connect(a, b);
      else connect(b, a);
    }
    blocked[which].add(j);
    blocked[which].add(neighbor);
    const marker = makePort(which, j * step + factor * threshold);
    const end = latticePort(other, far);
    if (which === 'tgt') connect(end, marker);
    else connect(marker, end);
  }

  const pairs = 1 + int(4);
  for (let i = 0; i < pairs; i++) {
    if (chance(0.35)) crossingCycle(chance(0.7) ? [int(size), int(size), int(size)] : [int(size), int(size), int(size), int(size)]);
    else crossingPair(int(size), int(size));
  }

  // Plain edges between lattice values.
  const plain = int(3);
  for (let i = 0; i < plain; i++) {
    const j = int(size);
    const k = int(size);
    if (!free('src', j) || !free('tgt', k)) continue;
    connect(latticePort('src', j), latticePort('tgt', k));
  }

  const list = (layer) => {
    const l = new W.ArrayList();
    // The order of the nodes is the order of the hyper edge segments.
    for (let i = layer.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [layer[i], layer[j]] = [layer[j], layer[i]];
    }
    for (const n of layer) W.add(l, n);
    return l;
  };
  return { graph, src: list(nodes.src), tgt: list(nodes.tgt), startPos: pick([0, 10, 42.5, -20]), seeded: true };
}

// buildRegionCall makes a call with exact coordinates, as buildBoundaryCall,
// where most free areas of the splitter are on a limit. Several pairs of
// crossing edges span a region. In the region, one side has a connection at
// each lattice value, and the other side has a connection at the same number
// of thresholds after each of them. The splitter must split one segment for
// each pair, so the later splits use the areas that the earlier ones left.
function buildRegionCall(dir) {
  const d = DIRS[dir];
  const step = pick([10, 10, 5, 20, 2.5]);
  const threshold = 0.2 * step;
  const graph = new W.LGraph();
  const random = new W.Random_0(1);
  random.seedhi = int(1 << 24);
  random.seedlo = int(1 << 24);
  W.setProperty(graph, W.RANDOM, random);

  const nodes = { src: [], tgt: [] };
  const side = { src: d.src, tgt: d.tgt };
  const makePort = (which, c) => {
    const node = new W.LNode(graph);
    const port = new W.LPort();
    port.side = W.sides[side[which]];
    if (d.vertical) port.pos.y_0 = c;
    else port.pos.x_0 = c;
    port.owner = node;
    W.add(node.ports, port);
    nodes[which].push(node);
    return port;
  };
  const connect = (source, target) => {
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
  };

  const pairs = 2 + int(5);
  const region = 1 + int(4);
  const factor = pick([4, 4, 4, 2, 2, 1, 3]);
  const which = chance(0.5) ? 'src' : 'tgt';
  const other = which === 'src' ? 'tgt' : 'src';
  // The lattice indices: the lower ends of the pairs, the region, one index
  // without a connection, and the upper ends of the pairs.
  const lower = [];
  const upper = [];
  for (let i = 0; i < pairs; i++) {
    lower.push(i);
    upper.push(pairs + region + 1 + i);
  }
  // The pairs are nested, or matched at random.
  if (chance(0.5)) upper.reverse();
  else {
    for (let i = upper.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [upper[i], upper[j]] = [upper[j], upper[i]];
    }
  }
  for (let i = 0; i < pairs; i++) {
    const a1 = makePort('src', lower[i] * step);
    const a2 = makePort('src', upper[i] * step);
    const b1 = makePort('tgt', lower[i] * step);
    const b2 = makePort('tgt', upper[i] * step);
    connect(a1, b2);
    connect(a2, b1);
  }
  for (let r = pairs; r < pairs + region; r++) {
    const lattice = makePort(which, r * step);
    const shifted = makePort(other, r * step + factor * threshold);
    const forward = which === 'src' ? [lattice, shifted] : [shifted, lattice];
    if (chance(0.85)) connect(forward[0], forward[1]);
    else connect(forward[1], forward[0]);
  }

  const list = (layer) => {
    const l = new W.ArrayList();
    for (let i = layer.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [layer[i], layer[j]] = [layer[j], layer[i]];
    }
    for (const n of layer) W.add(l, n);
    return l;
  };
  return { graph, src: list(nodes.src), tgt: list(nodes.tgt), startPos: pick([0, 10, 42.5, -20]), seeded: true };
}

// buildCoverCall makes a call with exact coordinates where the splitter must
// use free areas that are on a limit. Each free area next to an end of a
// crossing pair is larger than two thresholds, and the splitter takes the
// largest area when the crossings are equal. So two plain edges cover those
// areas: an area inside the extent of such an edge gets one crossing more.
// The areas between the two covers are on a limit: with the offset t they
// are exactly as large as two thresholds, and with the offset 3 t they have
// the size 0. With more pairs than such areas, a later split uses a part of
// size 0 that an earlier split left.
//
// With the lattice step D, the threshold t = D / 5 and the offset o, for m
// pairs and r short edges, on the axis of the hyper edge segments:
//
//   low cover    one side at 0, the other side at (m + 1) D
//   pairs        both sides at D ... m D, crossing to the upper ends
//   short edges  one side at k D - o, the other side at k D
//   high cover   one side at (m + r + 2) D - o, the other side above
//   upper ends   both sides at (m + r + 3) D ...
function buildCoverCall(dir) {
  const d = DIRS[dir];
  const step = pick([10, 10, 5, 20, 2.5]);
  const threshold = 0.2 * step;
  const graph = new W.LGraph();
  const random = new W.Random_0(1);
  random.seedhi = int(1 << 24);
  random.seedlo = int(1 << 24);
  W.setProperty(graph, W.RANDOM, random);

  const pairs = 2 + int(5);
  const shorts = pick([0, 0, 0, 1, 1, 2]);
  const offset = pick([1, 1, 3]) * threshold;
  const top = (2 * pairs + shorts + 3) * step;
  // The call can be mirrored on the axis, and the two sides can change
  // places.
  const mirror = chance(0.5);
  const at = (c) => (mirror ? top - c : c);
  const swap = chance(0.5);
  const sideOf = (which) => ((which === 'src') !== swap ? 'src' : 'tgt');
  const side = { src: d.src, tgt: d.tgt };
  const nodes = { src: [], tgt: [] };
  const makePort = (which, c) => {
    const real = sideOf(which);
    const node = new W.LNode(graph);
    const port = new W.LPort();
    port.side = W.sides[side[real]];
    if (d.vertical) port.pos.y_0 = at(c);
    else port.pos.x_0 = at(c);
    port.owner = node;
    W.add(node.ports, port);
    nodes[real].push(node);
    return { port, real };
  };
  // connect makes an edge from the port of the source layer to the port of
  // the target layer, or the other way with a small probability.
  const connect = (a, b) => {
    let [source, target] = a.real === 'src' ? [a.port, b.port] : [b.port, a.port];
    if (chance(0.1)) [source, target] = [target, source];
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
  };

  connect(makePort('src', 0), makePort('tgt', (pairs + 1) * step));
  const upper = [];
  for (let i = 0; i < pairs; i++) upper.push(pairs + shorts + 3 + i);
  if (chance(0.5)) upper.reverse();
  else {
    for (let i = upper.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [upper[i], upper[j]] = [upper[j], upper[i]];
    }
  }
  for (let i = 0; i < pairs; i++) {
    const a1 = makePort('src', (i + 1) * step);
    const a2 = makePort('src', upper[i] * step);
    const b1 = makePort('tgt', (i + 1) * step);
    const b2 = makePort('tgt', upper[i] * step);
    connect(a1, b2);
    connect(a2, b1);
  }
  for (let k = pairs + 2; k < pairs + 2 + shorts; k++) {
    connect(makePort('src', k * step - offset), makePort('tgt', k * step));
  }
  connect(makePort('src', (pairs + shorts + 2) * step - offset), makePort('tgt', (2 * pairs + shorts + 3) * step));

  const list = (layer) => {
    const l = new W.ArrayList();
    for (let i = layer.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [layer[i], layer[j]] = [layer[j], layer[i]];
    }
    for (const n of layer) W.add(l, n);
    return l;
  };
  return { graph, src: list(nodes.src), tgt: list(nodes.tgt), startPos: pick([0, 10, 42.5, -20]), seeded: true };
}

// buildUlpCall makes a call where the critical conflict threshold is smaller
// than the rounding of most coordinates. Two connections of each side are at
// 0 and at 2^-50, so the smallest distance of a side is 2^-50. The two edges
// between them cross, which gives a cycle of critical dependencies. The
// other connections are at lattice values, where a sum with the threshold
// rounds to the lattice value: a free area starts and ends exactly at a
// connection, and two connections at one lattice value are not a critical
// conflict. Edges from the connections near 0 to lattice values make the
// segments of the cycle hyper edges that reach the lattice.
function buildUlpCall(dir) {
  const d = DIRS[dir];
  const unit = 2 ** -50;
  // A small step puts lattice values nearer than the conflict threshold.
  const step = pick([10, 5, 5, 2.5, 2.5, 2.5, 1]);
  const graph = new W.LGraph();
  const random = new W.Random_0(1);
  random.seedhi = int(1 << 24);
  random.seedlo = int(1 << 24);
  W.setProperty(graph, W.RANDOM, random);

  const nodes = { src: [], tgt: [] };
  const side = { src: d.src, tgt: d.tgt };
  const makePort = (which, c) => {
    const node = new W.LNode(graph);
    const port = new W.LPort();
    port.side = W.sides[side[which]];
    if (d.vertical) port.pos.y_0 = c;
    else port.pos.x_0 = c;
    port.owner = node;
    W.add(node.ports, port);
    nodes[which].push(node);
    return port;
  };
  const connect = (source, target) => {
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
  };

  const nearSrc = [makePort('src', 0), makePort('src', unit)];
  const nearTgt = [makePort('tgt', 0), makePort('tgt', unit)];
  connect(nearSrc[0], nearTgt[1]);
  connect(nearSrc[1], nearTgt[0]);

  const size = 3 + int(6);
  const farSrc = [];
  const farTgt = [];
  for (let k = 1; k <= size; k++) {
    if (chance(0.7)) farSrc.push(makePort('src', k * step));
    if (chance(0.7)) farTgt.push(makePort('tgt', k * step));
  }
  // Two segments that have one lattice value on the two sides, which is not
  // a critical conflict here. One of them has a second connection near that
  // value, so the count of the conflicts between the two segments depends on
  // which list of connections the count moves forward in.
  if (chance(0.6)) {
    const c = (1 + int(size)) * step;
    const near = c + pick([1, 2.5, 0.5]);
    const x1 = (1 + int(size + 2)) * step;
    const x2 = (1 + int(size + 2)) * step;
    if (chance(0.5)) {
      const source = makePort('src', x1);
      connect(source, makePort('tgt', c));
      connect(source, makePort('tgt', near));
      connect(makePort('src', c), makePort('tgt', x2));
    } else {
      const target = makePort('tgt', x2);
      connect(makePort('src', c), target);
      connect(makePort('src', near), target);
      connect(makePort('src', x1), makePort('tgt', c));
    }
  }
  if (farSrc.length > 0 && farTgt.length > 0) {
    // The segments of the cycle reach the lattice.
    const reach = 1 + int(4);
    for (let i = 0; i < reach; i++) {
      if (chance(0.5)) connect(pick(nearSrc), pick(farTgt));
      else connect(pick(farSrc), pick(nearTgt));
    }
    // Edges between lattice values. A port can have more than one edge.
    const plain = int(2 * (farSrc.length + farTgt.length) + 1);
    for (let i = 0; i < plain; i++) {
      if (chance(0.15)) connect(pick(farTgt), pick(farSrc));
      else connect(pick(farSrc), pick(farTgt));
    }
  }

  const list = (layer) => {
    const l = new W.ArrayList();
    for (let i = layer.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [layer[i], layer[j]] = [layer[j], layer[i]];
    }
    for (const n of layer) W.add(l, n);
    return l;
  };
  return { graph, src: list(nodes.src), tgt: list(nodes.tgt), startPos: pick([0, 10, 42.5, -20]), seeded: true };
}

// buildSplitByCall makes a call with exact coordinates where the crossings
// with the segment that causes a split decide which free area the splitter
// takes. Three edges make a cycle of critical dependencies between the
// lattice values c1 < c2 < c3: one segment spans the three values, and the
// two others span one part each. When the splitter splits the long segment
// because of the segment of one part, the free areas of that part get two
// crossings more. One or two plain edges from a value m between c1 and c2 to
// a value n after c3 cover the other part, so that the areas of the two
// parts are one crossing apart.
function buildSplitByCall(dir) {
  const d = DIRS[dir];
  const step = pick([10, 5, 20]);
  const graph = new W.LGraph();
  const random = new W.Random_0(1);
  random.seedhi = int(1 << 24);
  random.seedlo = int(1 << 24);
  W.setProperty(graph, W.RANDOM, random);

  const c1 = int(3);
  const m = c1 + 1 + int(2);
  const c2 = m + 1 + int(2);
  const c3 = c2 + 1 + int(3);
  const n = c3 + 1 + int(2);
  // The call can be mirrored on the axis.
  const mirror = chance(0.5);
  const at = (k) => (mirror ? n + 1 - k : k) * step;
  const nodes = { src: [], tgt: [] };
  const side = { src: d.src, tgt: d.tgt };
  const makePort = (which, k) => {
    const node = new W.LNode(graph);
    const port = new W.LPort();
    port.side = W.sides[side[which]];
    if (d.vertical) port.pos.y_0 = at(k);
    else port.pos.x_0 = at(k);
    port.owner = node;
    W.add(node.ports, port);
    nodes[which].push(node);
    return port;
  };
  const connect = (source, target) => {
    const edge = new W.LEdge();
    edge.source = source;
    W.add(source.outgoingEdges, edge);
    edge.target = target;
    W.add(target.incomingEdges, edge);
  };

  const a = [c1, c2, c3].map((k) => makePort('src', k));
  const b = [c1, c2, c3].map((k) => makePort('tgt', k));
  if (chance(0.7)) {
    connect(a[0], b[2]);
    connect(a[2], b[1]);
    connect(a[1], b[0]);
  } else {
    connect(a[0], b[1]);
    connect(a[1], b[2]);
    connect(a[2], b[0]);
  }
  const first = chance(0.5);
  if (first || chance(0.5)) connect(makePort('src', m), makePort('tgt', n));
  if (!first || chance(0.5)) connect(makePort('src', n), makePort('tgt', m));

  const list = (layer) => {
    const l = new W.ArrayList();
    for (let i = layer.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [layer[i], layer[j]] = [layer[j], layer[i]];
    }
    for (const node of layer) W.add(l, node);
    return l;
  };
  return { graph, src: list(nodes.src), tgt: list(nodes.tgt), startPos: pick([0, 10, 42.5, -20]), seeded: true };
}

// synthetic returns the synthetic groups. A graph without a random number
// generator makes the worker throw when routeEdges needs a random number
// (Java fails with a NullPointerException). Such a call is recorded with
// "throws": true and without a result, and it ends its group, because the
// generator object is in an unknown state after it.
function synthetic() {
  const groups = [];
  let throwing = 0;
  while (groups.length < SYNTHETIC_GROUPS) {
    const dir = int(3);
    const spacing = pick([10, 10, 10, 8, 12.5, 3, 20, 5, 0.10000000149011612]);
    const generator = new W.OrthogonalRoutingGenerator(dir, spacing);
    const calls = [];
    const count = pick([1, 1, 1, 2, 3]);
    for (let i = 0; i < count; i++) {
      const family = rng();
      const c =
        family < COVER_SHARE
          ? buildCoverCall(dir)
          : family < COVER_SHARE + REGION_SHARE
            ? buildRegionCall(dir)
            : family < COVER_SHARE + REGION_SHARE + BOUNDARY_SHARE
              ? buildBoundaryCall(dir)
              : family < COVER_SHARE + REGION_SHARE + BOUNDARY_SHARE + ULP_SHARE
                ? buildUlpCall(dir)
                : family < COVER_SHARE + REGION_SHARE + BOUNDARY_SHARE + ULP_SHARE + SPLIT_BY_SHARE
                  ? buildSplitByCall(dir)
                  : buildCall(dir, spacing);
      const snap = snapshot(c.graph, c.src, c.tgt, c.startPos);
      let ret;
      try {
        ret = W.routeEdges(generator, c.graph, c.src, c.tgt, c.startPos);
      } catch (e) {
        if (c.seeded) throw e;
        snap.call.throws = true;
        calls.push(snap.call);
        throwing++;
        break;
      }
      calls.push(finish(snap, c.graph, ret));
    }
    groups.push({ kind: 'synthetic', dir, spacing: num(spacing), calls });
  }
  return { groups, throwing };
}

// ---------------------------------------------------------------------------
// Calls of real layouts: random graphs with a compound node that has ports
// on each side.

function randomHierarchicalGraph(index) {
  const sides = ['NORTH', 'SOUTH', 'NORTH', 'SOUTH', 'EAST', 'WEST'];
  const direction = pick(['RIGHT', 'RIGHT', 'DOWN', 'LEFT', 'UP']);
  const outer = [];
  const outerCount = 1 + int(4);
  for (let i = 0; i < outerCount; i++) outer.push({ id: `o${i}`, width: 20 + int(40), height: 20 + int(30) });
  const rootEdges = [];
  const compounds = [];
  const compoundCount = 1 + (chance(0.25) ? 1 : 0);
  let edgeCount = 0;
  for (let c = 0; c < compoundCount; c++) {
    const id = `c${c}`;
    const portCount = 2 + int(7);
    const ports = [];
    for (let p = 0; p < portCount; p++) {
      ports.push({ id: `${id}.p${p}`, width: pick([0, 5, 8]), height: pick([0, 5, 8]), layoutOptions: { 'elk.port.side': pick(sides) } });
    }
    const children = [];
    const childCount = 1 + int(5);
    for (let k = 0; k < childCount; k++) children.push({ id: `${id}.n${k}`, width: 20 + int(40), height: 20 + int(30) });
    const edges = [];
    for (const port of ports) {
      // The edges of the port inside the node and outside it.
      const inside = 1 + (chance(0.3) ? int(3) : 0);
      const inward = chance(0.5);
      for (let k = 0; k < inside; k++) {
        const child = pick(children).id;
        edges.push(inward ? { id: `e${edgeCount++}`, sources: [port.id], targets: [child] } : { id: `e${edgeCount++}`, sources: [child], targets: [port.id] });
      }
      if (chance(0.85)) {
        const o = pick(outer).id;
        rootEdges.push(inward ? { id: `e${edgeCount++}`, sources: [o], targets: [port.id] } : { id: `e${edgeCount++}`, sources: [port.id], targets: [o] });
      }
    }
    const inner = int(childCount + 1);
    for (let k = 0; k < inner; k++) {
      const a = pick(children).id;
      const b = pick(children).id;
      if (a !== b) edges.push({ id: `e${edgeCount++}`, sources: [a], targets: [b] });
    }
    compounds.push({
      id,
      layoutOptions: { 'elk.portConstraints': pick(['FIXED_SIDE', 'FIXED_SIDE', 'FIXED_ORDER', 'FREE']), 'elk.direction': direction },
      ports,
      children,
      edges,
    });
  }
  const between = int(outerCount);
  for (let k = 0; k < between; k++) {
    const a = pick(outer).id;
    const b = pick(outer).id;
    if (a !== b) rootEdges.push({ id: `e${edgeCount++}`, sources: [a], targets: [b] });
  }
  const options = { 'elk.algorithm': 'layered', 'elk.direction': direction, 'elk.hierarchyHandling': 'INCLUDE_CHILDREN' };
  if (chance(0.3)) options['elk.spacing.edgeEdge'] = String(pick([5, 8, 15]));
  if (chance(0.3)) options['elk.spacing.nodeNode'] = String(pick([10, 30, 50]));
  if (chance(0.2)) options['elk.randomSeed'] = String(1 + int(1000));
  return { id: `root${index}`, layoutOptions: options, children: [...outer, ...compounds], edges: rootEdges };
}

function layouts() {
  const groups = [];
  const byGenerator = new Map();
  let open = null;
  let westEast = 0;
  let errors = 0;
  elk.hook.routeBefore = (g, graph, src, tgt, startPos) => {
    open = null;
    const dir = DIR_OF_SOURCE_SIDE[g.routingStrategy.getSourcePortSide().name_0];
    if (dir === 0 && westEast >= MAX_LAYOUT_WEST_EAST_CALLS) return;
    open = { g, dir, snap: snapshot(graph, src, tgt, startPos) };
  };
  elk.hook.routeAfter = (g, graph, ret) => {
    if (!open || open.g !== g) return;
    const call = finish(open.snap, graph, ret);
    const dir = open.dir;
    open = null;
    // Keep each call of the two directions of the hierarchical ports. Of the
    // west to east direction, keep the calls that route an edge: a group of
    // that direction does not have each call of its generator. The generator
    // keeps no state from one call to the next, so the group is still a
    // valid sequence.
    if (dir === 0) {
      if (ret === 0) return;
      westEast++;
    }
    let group = byGenerator.get(g);
    if (!group) {
      group = { kind: 'layout', dir, spacing: num(g.edgeSpacing), calls: [] };
      byGenerator.set(g, group);
      groups.push(group);
    }
    group.calls.push(call);
  };
  S(`$routeEdges_0 = (function (orig) {
    return function (g, graph, src, tgt, startPos) {
      $$hook.routeBefore(g, graph, src, tgt, startPos);
      var r = orig(g, graph, src, tgt, startPos);
      $$hook.routeAfter(g, graph, r);
      return r;
    };
  })($routeEdges_0)`);
  for (let i = 0; i < LAYOUT_GRAPHS; i++) {
    const graph = randomHierarchicalGraph(i);
    env.beginLayout();
    byGenerator.clear();
    try {
      elk.layout(graph);
    } catch (e) {
      errors++;
    }
  }
  return { groups, errors };
}

// ---------------------------------------------------------------------------

const syn = synthetic();
const lay = layouts();
const groups = [...syn.groups, ...lay.groups];
const stats = { groups: groups.length, calls: 0, byDir: [0, 0, 0], routed: 0, withJunctions: 0, drewRandom: 0, noRandom: 0 };
for (const g of groups) {
  for (const c of g.calls) {
    stats.calls++;
    stats.byDir[g.dir]++;
    if (c.throws) continue;
    if (c.ret > 0) stats.routed++;
    if (c.after.some((e, i) => e.jp !== null && JSON.stringify(e.jp) !== JSON.stringify(c.edges[i].jp))) stats.withJunctions++;
    if (c.rnd === null) stats.noRandom++;
    else if (c.rnd[0] !== c.rndAfter[0] || c.rnd[1] !== c.rndAfter[1]) stats.drewRandom++;
  }
}
mkdirSync(dirname(outFile), { recursive: true });
const gz = gzipSync(Buffer.from(JSON.stringify({ groups })), { level: 9 });
writeFileSync(outFile, gz);
console.log(JSON.stringify({ ...stats, syntheticThrowing: syn.throwing, layoutGroups: lay.groups.length, layoutErrors: lay.errors, bytes: gz.length }));
