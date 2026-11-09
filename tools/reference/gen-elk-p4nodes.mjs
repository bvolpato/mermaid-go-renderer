// Extra phase goldens for the node placers of internal/elk/native/p4nodes:
// BKNodePlacer, SimpleNodePlacer, LinearSegmentsNodePlacer and
// NetworkSimplexPlacer.
//
// The phase goldens of gen-elk-phases.mjs come from the graphs that mermaid
// produces and from generic graphs. They do not reach each branch of the
// node placers: the compaction without edge straightening, each fixed
// alignment, the split of a linear segment, the flexible nodes of the network
// simplex placer. This script makes random graphs with a seeded generator
// and with the options that select these branches. For each graph it runs
//
//   node gen-elk-phases.mjs --dump FILE
//
// in a child process, so the states come from the same dump code as the
// phase goldens, and it keeps the steps of the node placers: the state of the
// LGraph before and after the placer, as elkjs 0.9.3 had it. A step where the
// placer of elkjs throws has no state after it and is marked "throws".
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-p4nodes.mjs               write the goldens
//   node gen-elk-p4nodes.mjs --search N    look in N more graphs of each
//                                          family for the rare branches
//
// The argument --only PROC limits both to the family of one placer.
// --only round writes only the golden of the helper function.
//
// Output: internal/elk/native/p4nodes/bk/testdata/bknodeplacer.jsonl.gz and
// internal/elk/native/p4nodes/testdata/{simplenodeplacer,
// linearsegmentsnodeplacer,networksimplexplacer}.jsonl.gz, and the golden of
// one helper function, testdata/roundtodouble.json. Each line is one
// case: {"name", "in", "steps": [{"kind", "proc", "slot", "graph", "before",
// "after", "throws"}], "states": [...]}. The state format is documented in
// internal/elk/native/nativetest/state.go.
import { execFile } from 'node:child_process';
import { gzipSync } from 'node:zlib';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');

// Seeded PRNG (mulberry32).
function prng(seed) {
  let a = seed >>> 0;
  const next = () => {
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  return {
    next,
    int: (lo, hi) => lo + Math.floor(next() * (hi - lo + 1)),
    chance: (p) => next() < p,
    pick: (arr) => arr[Math.floor(next() * arr.length)],
  };
}

const SIDES = ['WEST', 'EAST', 'NORTH', 'SOUTH'];

// size returns a node or label size. Some sizes are not integers, as the
// text sizes of mermaid.
function size(R, lo, hi) {
  const s = R.int(lo, hi);
  return R.chance(0.3) ? s + R.pick([0.5, 0.25, 0.015625, 0.3]) : s;
}

// genGraph makes one ELK graph. o selects the features:
//   minN, maxN        the number of nodes
//   ports             the chance that a node has ports
//   sides             the port sides to pick from
//   portConstraints   the port constraints to pick from
//   edgeLabels        the chance that an edge has a label
//   nodeLabels        the chance that a node has a label
//   selfLoops         the chance of a self loop
//   backEdges         the chance that an edge points backward
//   priorities        the chance that an edge has a straightness priority
//   hierarchy         the chance of a compound node with children
//   flexibility       the chance that a node has the option nodeFlexibility
//   wildPorts         the chance that a port with a fixed position is far
//                     outside the border of its node
function genGraph(R, o, rootOptions) {
  const n = R.int(o.minN ?? 3, o.maxN ?? 10);
  const nodes = [];
  for (let i = 0; i < n; i++) {
    const node = { id: `n${i}`, width: size(R, 20, 110), height: size(R, 20, 70) };
    const lo = {};
    if (R.chance(o.ports ?? 0)) {
      const constraint = R.pick(o.portConstraints ?? ['FIXED_SIDE']);
      lo['elk.portConstraints'] = constraint;
      node.ports = [];
      const k = R.int(1, 4);
      for (let p = 0; p < k; p++) {
        const side = R.pick(o.sides ?? SIDES);
        const pw = R.pick([0, 4, 5, 8]);
        const ph = R.pick([0, 4, 5, 8]);
        const port = { id: `n${i}p${p}`, width: pw, height: ph };
        if (constraint !== 'FREE' && constraint !== 'UNDEFINED') port.layoutOptions = { 'elk.port.side': side };
        if (constraint === 'FIXED_POS' || constraint === 'FIXED_RATIO') {
          // A wild port is far outside the border of its node. A family
          // without wild ports draws no number for the choice.
          const along = o.wildPorts && R.chance(o.wildPorts) ? R.int(-30, 40) / 10 : R.next();
          if (side === 'WEST') Object.assign(port, { x: -pw, y: along * (node.height - ph) });
          if (side === 'EAST') Object.assign(port, { x: node.width, y: along * (node.height - ph) });
          if (side === 'NORTH') Object.assign(port, { x: along * (node.width - pw), y: -ph });
          if (side === 'SOUTH') Object.assign(port, { x: along * (node.width - pw), y: node.height });
        }
        if (R.chance(0.15)) port.labels = [{ text: `p${p}`, width: size(R, 8, 20), height: 12 }];
        node.ports.push(port);
      }
      if (R.chance(0.2)) lo['elk.spacing.portPort'] = R.pick([2, 5, 12.5]);
      if (R.chance(0.2)) lo['elk.spacing.portsSurrounding'] = R.pick(['[top=4,left=0,bottom=6,right=0]', '[top=10,left=2,bottom=1.5,right=2]']);
    }
    if (R.chance(o.nodeLabels ?? 0)) {
      node.labels = [{ text: `node ${i}`, width: size(R, 20, 60), height: size(R, 10, 20) }];
      lo['elk.nodeLabels.placement'] = R.pick(['[H_CENTER, V_CENTER, INSIDE]', '[H_CENTER, V_BOTTOM, INSIDE]', '[H_LEFT, V_TOP, INSIDE]', '[H_CENTER, V_TOP, OUTSIDE]', '[H_RIGHT, V_BOTTOM, OUTSIDE]']);
      if (R.chance(0.3)) lo['elk.nodeSize.constraints'] = R.pick(['[NODE_LABELS]', '[NODE_LABELS, PORTS]', '[MINIMUM_SIZE]']);
    }
    if (R.chance(o.flexibility ?? 0)) {
      lo['elk.layered.nodePlacement.networkSimplex.nodeFlexibility'] = R.pick(['NONE', 'PORT_POSITION', 'NODE_SIZE_WHERE_SPACE_PERMITS', 'NODE_SIZE']);
    }
    if (Object.keys(lo).length) node.layoutOptions = lo;
    nodes.push(node);
  }

  const end = (i) => {
    const node = nodes[i];
    if (node.ports && R.chance(0.75)) return R.pick(node.ports).id;
    return node.id;
  };
  const edges = [];
  const m = n - 1 + R.int(0, Math.max(1, Math.floor(n * (o.density ?? 0.6))));
  for (let e = 0; e < m; e++) {
    let a = R.int(0, n - 1);
    let b = R.int(0, n - 1);
    if (e < n - 1 && R.chance(0.8)) {
      // Most graphs are connected: the node e + 1 gets an edge from a node
      // before it.
      b = e + 1;
      a = R.int(Math.max(0, b - (o.span ?? 4)), b - 1);
    }
    if (a === b) {
      if (!R.chance(o.selfLoops ?? 0)) continue;
    } else if (a > b !== R.chance(o.backEdges ?? 0.1)) {
      [a, b] = [b, a];
    }
    const edge = { id: `e${e}`, sources: [end(a)], targets: [end(b)] };
    if (R.chance(o.edgeLabels ?? 0)) edge.labels = [{ text: `l${e}`, width: size(R, 10, 50), height: size(R, 10, 18) }];
    if (R.chance(o.priorities ?? 0)) edge.layoutOptions = { 'elk.layered.priority.straightness': R.pick([1, 2, 5, 10]) };
    edges.push(edge);
  }

  const graph = { id: 'root', layoutOptions: { 'elk.algorithm': 'layered', ...rootOptions }, children: nodes, edges };
  if (R.chance(o.hierarchy ?? 0) && n >= 4) {
    // A compound node takes a run of nodes as its children.
    const from = R.int(0, n - 3);
    const count = R.int(2, Math.min(4, n - from - 1));
    const inner = nodes.slice(from, from + count);
    const compound = {
      id: 'c0',
      layoutOptions: { 'elk.padding': R.pick(['[top=20,left=12,bottom=12,right=12]', '[top=8,left=8,bottom=8,right=8]']) },
      children: inner,
      edges: [],
    };
    if (R.chance(0.4)) compound.labels = [{ text: 'group', width: 40, height: 14 }];
    if (R.chance(0.4)) compound.layoutOptions['elk.nodeLabels.placement'] = '[H_CENTER, V_TOP, INSIDE]';
    graph.children = [...nodes.slice(0, from), compound, ...nodes.slice(from + count)];
    graph.layoutOptions['elk.hierarchyHandling'] = R.pick(['INCLUDE_CHILDREN', 'INCLUDE_CHILDREN', 'SEPARATE_CHILDREN']);
    // The nested graph reads its options from the compound node.
    for (const [k, v] of Object.entries(rootOptions)) {
      if (k.includes('nodePlacement') || k.includes('thoroughness')) compound.layoutOptions[k] = v;
    }
    if (graph.layoutOptions['elk.hierarchyHandling'] === 'SEPARATE_CHILDREN') {
      // Without the hierarchy an edge must stay inside one graph.
      const isInner = (id) => inner.some((c) => c.id === id || (c.ports ?? []).some((p) => p.id === id));
      graph.edges = [];
      for (const e of edges) {
        const s = isInner(e.sources[0]);
        const t = isInner(e.targets[0]);
        if (s && t) compound.edges.push(e);
        else if (!s && !t) graph.edges.push(e);
        else if (s) graph.edges.push({ ...e, sources: ['c0'] });
        else graph.edges.push({ ...e, targets: ['c0'] });
      }
      compound.layoutOptions['elk.algorithm'] = 'layered';
      for (const [k, v] of Object.entries(rootOptions)) compound.layoutOptions[k] = v;
    }
  }
  return graph;
}

// commonOptions returns root options that all families vary.
function commonOptions(R) {
  const lo = { 'elk.randomSeed': 1 };
  lo['elk.direction'] = R.pick(['RIGHT', 'RIGHT', 'DOWN', 'DOWN', 'LEFT', 'UP']);
  if (R.chance(0.4)) lo['elk.spacing.nodeNode'] = R.pick([10, 20, 35.5, 50]);
  if (R.chance(0.3)) lo['elk.layered.spacing.nodeNodeBetweenLayers'] = R.pick([15, 40, 62.5]);
  if (R.chance(0.3)) lo['elk.spacing.edgeNode'] = R.pick([5, 12, 25]);
  if (R.chance(0.3)) lo['elk.spacing.edgeEdge'] = R.pick([3, 10, 17.5]);
  if (R.chance(0.2)) lo['elk.layered.spacing.baseValue'] = R.pick([20, 40]);
  if (R.chance(0.3)) lo['elk.layered.cycleBreaking.strategy'] = R.pick(['GREEDY', 'DEPTH_FIRST', 'MODEL_ORDER', 'GREEDY_MODEL_ORDER']);
  if (R.chance(0.2)) lo['elk.layered.layering.strategy'] = R.pick(['NETWORK_SIMPLEX', 'LONGEST_PATH', 'LONGEST_PATH_SOURCE']);
  if (R.chance(0.2)) lo['elk.layered.considerModelOrder.strategy'] = R.pick(['NODES_AND_EDGES', 'PREFER_EDGES', 'PREFER_NODES']);
  if (R.chance(0.15)) lo['elk.layered.feedbackEdges'] = true;
  if (R.chance(0.15)) lo['elk.layered.mergeEdges'] = true;
  if (R.chance(0.15)) lo['elk.layered.unnecessaryBendpoints'] = true;
  if (R.chance(0.15)) lo['elk.edgeRouting'] = R.pick(['POLYLINE', 'SPLINES']);
  return lo;
}

const ALIGNMENTS = ['NONE', 'BALANCED', 'LEFTUP', 'RIGHTUP', 'LEFTDOWN', 'RIGHTDOWN'];
const STRAIGHTENING = ['NONE', 'IMPROVE_STRAIGHTNESS'];
const FAVOR = [undefined, true, false];

// The families: for each placer the number of graphs, and the options and
// features of the graph with the index i.
const FAMILIES = [
  {
    proc: 'BKNodePlacer',
    file: 'internal/elk/native/p4nodes/bk/testdata/bknodeplacer.jsonl.gz',
    count: 432,
    seed: 0x62b00000,
    make(R, i) {
      const lo = commonOptions(R);
      // Each combination of alignment, straightening and favorStraightEdges
      // comes the same number of times.
      const alignment = ALIGNMENTS[i % 6];
      const straightening = STRAIGHTENING[Math.floor(i / 6) % 2];
      const favor = FAVOR[Math.floor(i / 12) % 3];
      if (alignment !== 'NONE' || R.chance(0.5)) lo['elk.layered.nodePlacement.bk.fixedAlignment'] = alignment;
      if (straightening !== 'IMPROVE_STRAIGHTNESS' || R.chance(0.5)) lo['elk.layered.nodePlacement.bk.edgeStraightening'] = straightening;
      if (favor !== undefined) lo['elk.layered.nodePlacement.favorStraightEdges'] = favor;
      if (R.chance(0.3)) lo['elk.layered.nodePlacement.strategy'] = 'BRANDES_KOEPF';
      const kind = Math.floor(i / 36) % 6;
      if (kind === 5) {
        // Without a spacing two nodes of a layer touch, and the test of the
        // node order (checkOrderConstraint) fails for each layout. A spacing
        // below zero makes the nodes overlap.
        const spacing = R.pick([0, 0, 0, -4]);
        lo['elk.spacing.nodeNode'] = spacing;
        lo['elk.spacing.edgeNode'] = spacing;
        lo['elk.spacing.edgeEdge'] = spacing;
      }
      const features = [
        // plain graphs with long edges
        { minN: 4, maxN: 11, density: 0.9, span: 5, backEdges: 0.1, priorities: 0.15 },
        // ports on each side, self loops and edges inside a layer
        { minN: 3, maxN: 9, ports: 0.6, portConstraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS', 'FREE'], selfLoops: 0.5, backEdges: 0.25, priorities: 0.1 },
        // labels
        { minN: 3, maxN: 9, edgeLabels: 0.5, nodeLabels: 0.4, ports: 0.15, backEdges: 0.15 },
        // hierarchy
        { minN: 4, maxN: 8, hierarchy: 0.8, edgeLabels: 0.2, ports: 0.2, sides: ['WEST', 'EAST'], backEdges: 0.1 },
        // ports with a fixed position far outside the node: the blocks get
        // large inner shifts
        { minN: 4, maxN: 10, ports: 0.8, sides: ['WEST', 'EAST'], portConstraints: ['FIXED_POS'], wildPorts: 0.7, density: 1.2, span: 3, backEdges: 0.15 },
        // no spacing between the nodes of a layer
        { minN: 4, maxN: 10, density: 1, span: 4, backEdges: 0.1, ports: 0.2, edgeLabels: 0.15 },
      ][kind];
      return genGraph(R, features, lo);
    },
  },
  {
    proc: 'SimpleNodePlacer',
    file: 'internal/elk/native/p4nodes/testdata/simplenodeplacer.jsonl.gz',
    count: 24,
    seed: 0x51300000,
    make(R, i) {
      const lo = commonOptions(R);
      lo['elk.layered.nodePlacement.strategy'] = 'SIMPLE';
      const features = [
        { minN: 3, maxN: 10, density: 0.8 },
        { minN: 3, maxN: 8, ports: 0.5, portConstraints: ['FIXED_SIDE', 'FIXED_POS'], selfLoops: 0.4, edgeLabels: 0.3 },
        { minN: 4, maxN: 8, hierarchy: 0.8, nodeLabels: 0.3 },
      ][i % 3];
      return genGraph(R, features, lo);
    },
  },
  {
    proc: 'LinearSegmentsNodePlacer',
    file: 'internal/elk/native/p4nodes/testdata/linearsegmentsnodeplacer.jsonl.gz',
    count: 96,
    seed: 0x11500000,
    // Graphs of --search in which a linear segment is split.
    extra: [181, 293, 377, 457, 613, 645, 661, 937],
    make(R, i) {
      const lo = commonOptions(R);
      lo['elk.layered.nodePlacement.strategy'] = 'LINEAR_SEGMENTS';
      if (R.chance(0.5)) lo['elk.layered.thoroughness'] = R.pick([1, 2, 7, 20, 50]);
      if (R.chance(0.4)) lo['elk.layered.nodePlacement.linearSegments.deflectionDampening'] = R.pick([0.1, 0.3, 0.75, 1]);
      const features = [
        { minN: 4, maxN: 11, density: 0.9, span: 5, priorities: 0.2 },
        // north and south ports make dummy nodes that change the order of
        // the segments between two layers
        { minN: 3, maxN: 9, ports: 0.8, sides: ['NORTH', 'SOUTH', 'NORTH', 'SOUTH', 'WEST', 'EAST'], portConstraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'], selfLoops: 0.3, backEdges: 0.3, density: 1.2 },
        { minN: 3, maxN: 9, edgeLabels: 0.5, nodeLabels: 0.3, ports: 0.3, backEdges: 0.2, priorities: 0.2 },
        { minN: 4, maxN: 8, hierarchy: 0.8, ports: 0.3, edgeLabels: 0.2 },
      ][i % 4];
      return genGraph(R, features, lo);
    },
  },
  {
    proc: 'NetworkSimplexPlacer',
    file: 'internal/elk/native/p4nodes/testdata/networksimplexplacer.jsonl.gz',
    count: 144,
    seed: 0x25000000,
    // Graphs of --search in which the placer of elkjs throws.
    extra: [277, 281, 389, 449, 457, 713, 869, 1089, 1109, 1301, 1305, 1461],
    make(R, i) {
      const lo = commonOptions(R);
      lo['elk.layered.nodePlacement.strategy'] = 'NETWORK_SIMPLEX';
      const favor = FAVOR[i % 3];
      if (favor !== undefined) lo['elk.layered.nodePlacement.favorStraightEdges'] = favor;
      const flexDefault = [undefined, 'NONE', 'PORT_POSITION', 'NODE_SIZE_WHERE_SPACE_PERMITS', 'NODE_SIZE'][Math.floor(i / 3) % 5];
      if (flexDefault !== undefined) lo['elk.layered.nodePlacement.networkSimplex.nodeFlexibility.default'] = flexDefault;
      if (R.chance(0.3)) lo['elk.layered.thoroughness'] = R.pick([1, 3, 7, 30]);
      if (R.chance(0.2)) lo['elk.spacing.portPort'] = R.pick([4, 10, 15.5]);
      const features = [
        { minN: 4, maxN: 10, density: 0.8, span: 5, priorities: 0.2, flexibility: 0.4 },
        // ports whose position is free, so that a node can be flexible
        { minN: 3, maxN: 9, ports: 0.7, sides: ['WEST', 'EAST', 'WEST', 'EAST', 'NORTH', 'SOUTH'], portConstraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FREE', 'FIXED_POS'], flexibility: 0.6, nodeLabels: 0.4, selfLoops: 0.2, backEdges: 0.2 },
        { minN: 3, maxN: 9, ports: 0.5, portConstraints: ['FIXED_SIDE', 'FIXED_ORDER'], flexibility: 0.5, edgeLabels: 0.4, nodeLabels: 0.5, backEdges: 0.3, priorities: 0.2 },
        { minN: 4, maxN: 8, hierarchy: 0.8, ports: 0.4, flexibility: 0.4, nodeLabels: 0.3 },
      ][i % 4];
      return genGraph(R, features, lo);
    },
  },
];

// families returns the families to work on: each one, or the one that the
// argument --only names.
function families() {
  const at = process.argv.indexOf('--only');
  if (at < 0) return FAMILIES;
  return FAMILIES.filter((f) => f.proc === process.argv[at + 1]);
}

// dump runs gen-elk-phases.mjs --dump for one graph file.
function dump(file) {
  return new Promise((done, fail) => {
    // elkjs does not end for some graphs. Such a graph is left out.
    execFile(process.execPath, [join(here, 'gen-elk-phases.mjs'), '--dump', file], { cwd: here, maxBuffer: 1 << 30, timeout: 600000 }, (err, stdout) => {
      if (err && err.killed) done(null);
      else if (err) fail(err);
      else done(JSON.parse(stdout));
    });
  });
}

// extract keeps the steps of one node placer with their states.
function extract(proc, name, input, r) {
  if (r === null) {
    console.log(`${name}: elkjs did not end, the graph is left out`);
    return null;
  }
  const steps = [];
  const states = [];
  for (const run of r.runs) {
    const index = new Map();
    const state = (i) => {
      if (i === undefined || i === null) return undefined;
      if (!index.has(i)) {
        index.set(i, states.length);
        states.push(run.states[i]);
      }
      return index.get(i);
    };
    for (const s of run.steps) {
      if (s.proc !== proc || s.before === undefined) continue;
      const step = { kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, before: state(s.before) };
      if (s.after === undefined) step.throws = true;
      else step.after = state(s.after);
      steps.push(step);
    }
  }
  if (steps.length === 0) return null;
  return { name, in: input, steps, states };
}

async function pool(items, width, fn) {
  const out = new Array(items.length);
  let next = 0;
  await Promise.all(
    Array.from({ length: width }, async () => {
      while (next < items.length) {
        const i = next++;
        out[i] = await fn(items[i], i);
      }
    }),
  );
  return out;
}

// ---------------------------------------------------------------------------
// --search: find graphs that reach the rare branches of the node placers.
//
// The search runs many graphs in one engine, without a state dump, and counts
// what happens inside the node placer: a layout of Brandes and Koepf that
// breaks the node order (checkOrderConstraint returns false), a linear
// segment that is split, a placer that throws. It prints the index of each
// graph that reaches such a branch. The indices that the search found are in
// the member extra of the families above: the generator adds these graphs to
// the goldens.

async function search(limit) {
  const { loadElk, env } = await import('./elk-phases-lib.mjs');
  env.install();
  const elk = loadElk();
  const S = elk.hook.scope;
  const getName = S('$getName');
  const getClass = S('getClass__Ljava_lang_Class___devirtual$');
  const seen = { infeasible: 0, split: 0 };
  elk.hook.count = (what) => {
    seen[what]++;
  };
  // Wrap functions of the worker. The eval of the hook runs in the scope of
  // the worker, so it can replace a function there.
  S(`(function () {
    var check = $checkOrderConstraint;
    $checkOrderConstraint = function (g, bal, m) { var r = check(g, bal, m); if (!r) $$hook.count('infeasible'); return r; };
    var split = $split_3;
    $split_3 = function (a, b, c) { $$hook.count('split'); return split(a, b, c); };
  })()`);
  let current = null;
  elk.hook.before = (kind, p) => {
    current = getName(getClass(p)).replace(/^.*\./, '');
  };
  elk.hook.after = () => {
    current = null;
  };
  for (const family of families()) {
    const found = { infeasible: [], split: [], throws: [] };
    for (let i = family.count; i < family.count + limit; i++) {
      const graph = family.make(prng(family.seed + i), i);
      seen.infeasible = 0;
      seen.split = 0;
      current = null;
      env.beginLayout();
      let threw = false;
      try {
        elk.layout(JSON.parse(JSON.stringify(graph)));
      } catch (e) {
        threw = current === family.proc;
      }
      if (seen.infeasible > 0) found.infeasible.push(i);
      if (seen.split > 0) found.split.push(i);
      if (threw) found.throws.push(i);
    }
    for (const [what, list] of Object.entries(found)) {
      console.log(`${family.proc}: ${what}: ${list.length} graphs: ${list.slice(0, 40).join(', ')}`);
    }
  }
}

// ---------------------------------------------------------------------------
// The goldens.

async function generate() {
  const tmp = mkdtempSync(join(tmpdir(), 'elk-p4nodes-'));
  try {
    for (const family of families()) {
      const inputs = [];
      const indices = [...Array(family.count).keys(), ...(family.extra ?? [])];
      for (const i of indices) {
        const R = prng(family.seed + i);
        inputs.push({ name: `${family.proc}-${i}`, graph: family.make(R, i) });
      }
      const cases = await pool(inputs, 8, async (c, k) => {
        const file = join(tmp, `${family.proc}-${k}.json`);
        writeFileSync(file, JSON.stringify(c.graph));
        return extract(family.proc, c.name, c.graph, await dump(file));
      });
      const kept = cases.filter(Boolean);
      const stepCount = kept.reduce((a, c) => a + c.steps.length, 0);
      const throwCount = kept.reduce((a, c) => a + c.steps.filter((s) => s.throws).length, 0);
      const body = kept.map((c) => JSON.stringify(c)).join('\n') + '\n';
      const gz = gzipSync(Buffer.from(body), { level: 9 });
      const out = resolve(root, family.file);
      mkdirSync(dirname(out), { recursive: true });
      writeFileSync(out, gz);
      console.log(`${family.proc}: ${kept.length} cases of ${inputs.length} graphs, ${stepCount} steps, ${throwCount} steps throw, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
    }
  } finally {
    rmSync(tmp, { recursive: true, force: true });
  }
}

// roundGolden writes the golden of roundToDouble in networksimplexplacer.go:
// the Java expression (double) Math.round(y), computed with the functions of
// the worker that GWT compiled it to. A value is the hexadecimal text of the
// bits of the double.
async function roundGolden() {
  const { loadElk, env } = await import('./elk-phases-lib.mjs');
  env.install();
  const S = loadElk().hook.scope;
  const toDouble = S('toDouble_0');
  const fromDouble = S('fromDouble_0');
  const bits = (x) => {
    const b = Buffer.alloc(8);
    b.writeDoubleBE(x);
    return b.toString('hex');
  };
  const values = [
    0, -0, 0.5, -0.5, 1.5, -1.5, 2.5, -2.5, 0.49999999999999994, -0.49999999999999994, 0.2, -0.2, 74.015625, -74.015625,
    4503599627370495.5, 4503599627370497, 9007199254740991, -9007199254740991,
    17592186044415, 17592186044415.5, 17592186044415.998, 17592186044416, 17592186044416.5, 17592186044417,
    -17592186044415, -17592186044415.5, -17592186044416, -17592186044416.5, -17592186044417,
    1e15, -1e15, 1e16, -1e16, 1e18, -1e18, 9223372036854774784, 9223372036854775807, 9223372036854777856, 1e19,
    -9223372036854774784, -9223372036854775808, -9223372036854777856, -1e19, 1e300, -1e300, Number.MAX_VALUE, -Number.MAX_VALUE,
    Number.MIN_VALUE, -Number.MIN_VALUE, NaN, Infinity, -Infinity,
  ];
  const R = prng(0x7e57);
  for (let i = 0; i < 200; i++) {
    const scale = 10 ** R.int(-3, 20);
    values.push((R.next() - 0.5) * scale);
    values.push(Math.round((R.next() - 0.5) * 2000) / 2);
  }
  const cases = values.map((y) => [bits(y), bits(toDouble(fromDouble(Math.round(y))))]);
  const out = resolve(root, 'internal/elk/native/p4nodes/testdata/roundtodouble.json');
  mkdirSync(dirname(out), { recursive: true });
  writeFileSync(out, JSON.stringify(cases) + '\n');
  console.log(`roundToDouble: ${cases.length} values`);
}

if (process.argv.includes('--search')) {
  await search(Number(process.argv[process.argv.indexOf('--search') + 1]) || 2000);
} else {
  if (!process.argv.includes('--only') || process.argv.includes('round')) await roundGolden();
  await generate();
}
