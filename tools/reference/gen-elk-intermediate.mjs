// Extra phase goldens for internal/elk/native/intermediate: ELK graphs that
// reach the branches of the processors of that package which the shared
// phase goldens do not reach (layout directions with both direction
// congruencies, layer constraints, hierarchical ports, northern and southern
// ports, fixed port orders, port indices, alignments, node label placements,
// spline routing, merged hyperedges).
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-intermediate.mjs
//
// The script runs `node gen-elk-phases.mjs --dump` for each graph, so the
// states come from the same instrumented elkjs 0.9.3 as the shared goldens,
// in the same format (internal/elk/native/nativetest/state.go). It keeps the
// steps of the processors of the package and the states that they name, and
// writes internal/elk/native/intermediate/testdata/phases.json.gz. A step
// where the processor throws an exception has the member "throws" with the
// message, and no state after it.
//
// The self loop processors of ELK hide each self loop before the processors
// for northern and southern ports run, so the code of those two processors
// for self loops is dead in elkjs. To have states for that code too, the
// graphs of the kind "self-loops-off" run in a worker where
// SelfLoopHolder.needsSelfLoopProcessing returns false: a copy of
// elk-phases-lib.mjs with one more text patch, in a temporary directory
// below this one. The processors that write the states are the unchanged
// ones of elkjs.
import { execFile } from 'node:child_process';
import { mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const outFile = resolve(here, '../../internal/elk/native/intermediate/testdata/phases.json.gz');

const PROCESSORS = new Set([
  'GraphTransformer:TO_INPUT_DIRECTION',
  'GraphTransformer:TO_INTERNAL_LTR',
  'EdgeAndLayerConstraintEdgeReverser',
  'LayerConstraintPreprocessor',
  'LayerConstraintPostprocessor',
  'LongEdgeSplitter',
  'LongEdgeJoiner',
  'ReversedEdgeRestorer',
  'PortSideProcessor',
  'PortListSorter',
  'InLayerConstraintProcessor',
  'LayerSizeAndGraphHeightCalculator',
  'NorthSouthPortPreprocessor',
  'NorthSouthPortPostprocessor',
  'HyperedgeDummyMerger',
]);

// A state larger than this is too large for a golden.
const MAX_STATE_BYTES = 40000;

// mulberry32: a small seeded generator.
function rng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

const DIRECTIONS = ['RIGHT', 'LEFT', 'DOWN', 'UP'];
const SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];
const ALIGNMENTS = ['AUTOMATIC', 'LEFT', 'RIGHT', 'TOP', 'BOTTOM', 'CENTER'];
const PLACEMENTS = [
  '[H_LEFT, V_TOP, INSIDE]',
  '[H_RIGHT, V_BOTTOM, OUTSIDE]',
  '[H_CENTER, V_CENTER, INSIDE]',
  '[H_LEFT, V_CENTER, OUTSIDE, H_PRIORITY]',
  '[H_RIGHT, V_TOP, INSIDE, H_PRIORITY]',
  '[H_CENTER, V_BOTTOM, OUTSIDE]',
  '[]',
];
const SIDE_SELECTIONS = ['ALWAYS_UP', 'ALWAYS_DOWN', 'DIRECTION_UP', 'DIRECTION_DOWN', 'SMART_UP', 'SMART_DOWN'];
const LAYER_CONSTRAINTS = ['FIRST', 'LAST', 'FIRST_SEPARATE', 'LAST_SEPARATE'];
const PORT_CONSTRAINTS = ['FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS', 'FIXED_RATIO'];

// One random graph. The kind chooses the features that the graph has for
// sure, so that each branch gets several graphs.
function randomGraph(seed, kind) {
  const r = rng(seed);
  const int = (n) => Math.floor(r() * n);
  const pick = (list) => list[int(list.length)];
  const chance = (p) => r() < p;
  const size = () => 10 + int(60) + (chance(0.3) ? int(100) / 7 : 0);

  const rootOptions = { 'elk.algorithm': 'layered' };
  rootOptions['elk.direction'] = kind.direction ?? pick(DIRECTIONS);
  if (kind.rotation ?? chance(0.5)) rootOptions['elk.layered.directionCongruency'] = 'ROTATION';
  if (kind.routing) rootOptions['elk.edgeRouting'] = kind.routing;
  else if (chance(0.15)) rootOptions['elk.edgeRouting'] = pick(['POLYLINE', 'SPLINES']);
  if (chance(0.3)) rootOptions['elk.layered.unnecessaryBendpoints'] = true;
  if (kind.mergeEdges ?? chance(0.2)) rootOptions['elk.layered.mergeEdges'] = true;
  if (chance(0.4)) rootOptions['elk.layered.edgeLabels.sideSelection'] = pick(SIDE_SELECTIONS);
  if (chance(0.3)) rootOptions['elk.nodeLabels.padding'] = `[top=${int(9)},left=${int(9)},bottom=${int(9)},right=${int(9)}]`;
  if (kind.portDegree ?? chance(0.25)) rootOptions['elk.layered.portSortingStrategy'] = 'PORT_DEGREE';
  if (chance(0.3)) rootOptions['elk.spacing.portsSurrounding'] = `[top=${int(15)},left=${int(15)},bottom=${int(15)},right=${int(15)}]`;
  if (chance(0.3)) rootOptions['elk.padding'] = `[top=${int(30)},left=${int(30)},bottom=${int(30)},right=${int(30)}]`;
  if (kind.includeChildren) rootOptions['elk.hierarchyHandling'] = 'INCLUDE_CHILDREN';
  if (chance(0.2)) rootOptions['elk.layered.feedbackEdges'] = true;
  // The greedy switch does not know the dummy nodes of self loops.
  if (kind.patched) rootOptions['elk.layered.crossingMinimization.greedySwitch.type'] = 'OFF';

  let nextEdge = 0;
  const ports = (prefix, count, options) => {
    const out = [];
    for (let i = 0; i < count; i++) {
      const p = { id: `${prefix}.p${i}`, width: chance(0.7) ? 4 + int(6) : 0, height: chance(0.7) ? 4 + int(6) : 0 };
      const lo = {};
      if (options.sides) lo['elk.port.side'] = pick(options.sides);
      if (options.index && chance(0.8)) lo['elk.port.index'] = int(2 * count) - (chance(0.3) ? count : 0);
      if (options.positions) {
        p.x = int(50);
        p.y = int(50);
      }
      if (chance(0.25)) lo['elk.layered.allowNonFlowPortsToSwitchSides'] = true;
      if (chance(0.2)) lo['elk.port.borderOffset'] = int(12) - 6;
      if (Object.keys(lo).length) p.layoutOptions = lo;
      if (chance(0.3)) p.labels = [{ text: `P${i}`, width: 6 + int(10), height: 5 + int(6) }];
      out.push(p);
    }
    return out;
  };
  const makeNode = (id, withPorts) => {
    const n = { id, width: size(), height: size() };
    // A port of a node without a size gets no side from its position.
    if (chance(kind.zeroSize ?? 0.02)) n.width = n.height = 0;
    const lo = {};
    if (chance(kind.alignment ?? 0.3)) lo['elk.alignment'] = pick(ALIGNMENTS);
    if (chance(kind.position ?? 0.15)) lo['elk.position'] = `(${int(200)},${int(200)})`;
    if (chance(0.3)) lo['elk.nodeLabels.placement'] = pick(PLACEMENTS);
    if (chance(0.2)) lo['elk.nodeSize.minimum'] = `(${int(80)},${int(80)})`;
    if (chance(kind.layerConstraint ?? 0.1)) lo['elk.layered.layering.layerConstraint'] = pick(kind.layerConstraints ?? LAYER_CONSTRAINTS);
    if (chance(0.3)) {
      n.labels = [{ text: id.toUpperCase(), width: 8 + int(30), height: 6 + int(10) }];
      if (chance(0.5)) n.labels[0].layoutOptions = { 'elk.nodeLabels.placement': pick(PLACEMENTS) };
      if (chance(0.2)) n.labels.push({ text: 'second', width: 20, height: 9 });
    }
    if (withPorts) {
      const pc = kind.portConstraints ? pick(kind.portConstraints) : pick(PORT_CONSTRAINTS);
      const count = 1 + int(kind.maxPorts ?? 5);
      const fixedSide = pc !== 'FREE';
      lo['elk.portConstraints'] = pc;
      n.ports = ports(id, count, {
        sides: !kind.noSides && (fixedSide || chance(0.3)) ? kind.sides ?? SIDES : undefined,
        index: pc === 'FIXED_ORDER' || chance(0.2),
        positions: !kind.noSides && (pc === 'FIXED_POS' || pc === 'FIXED_RATIO' || chance(0.3)),
      });
    }
    if (Object.keys(lo).length) n.layoutOptions = lo;
    return n;
  };
  const endpoints = (nodes) => {
    const out = [];
    for (const n of nodes) {
      if (n.ports?.length) {
        for (const p of n.ports) out.push(p.id);
        if (chance(0.3)) out.push(n.id);
      } else {
        out.push(n.id);
      }
    }
    return out;
  };
  const makeEdges = (nodes, count, prefix, extra = []) => {
    const eps = [...endpoints(nodes), ...extra];
    const owner = (ep) => ep.split('.')[0];
    const out = [];
    for (let i = 0; i < count; i++) {
      const s = pick(eps);
      let t = pick(eps);
      // Self loops only for the kind that asks for them: the self loop
      // processors hide them from the processors of this package.
      if (!(kind.selfLoops && chance(0.4))) {
        for (let k = 0; k < 8 && owner(t) === owner(s); k++) t = pick(eps);
        if (owner(t) === owner(s)) continue;
      }
      const e = { id: `${prefix}e${nextEdge++}`, sources: [s], targets: [t] };
      if (chance(0.06)) e.layoutOptions = { 'elk.edge.thickness': chance(0.5) ? -2 : 3 };
      if (chance(kind.edgeLabels ?? 0.3)) {
        e.labels = [{ text: 'l', width: 6 + int(30), height: 5 + int(12) }];
        if (chance(0.5)) e.labels[0].layoutOptions = { 'elk.edgeLabels.placement': pick(['CENTER', 'HEAD', 'TAIL']) };
        if (chance(0.2)) e.labels.push({ text: 'm', width: 12, height: 8, layoutOptions: { 'elk.edgeLabels.placement': pick(['HEAD', 'TAIL']) } });
      }
      out.push(e);
    }
    return out;
  };

  const nodeCount = (kind.minNodes ?? 3) + int(kind.extraNodes ?? 5);
  const nodes = [];
  for (let i = 0; i < nodeCount; i++) nodes.push(makeNode(`n${i}`, chance(kind.ports ?? 0.4)));
  const graph = { id: 'root', layoutOptions: rootOptions, children: nodes };

  // A node with children and ports: the ports become external port dummies
  // in the graph of the children.
  if (kind.compound) {
    const c = makeNode('c', false);
    const inner = [];
    const innerCount = 2 + int(4);
    for (let i = 0; i < innerCount; i++) inner.push(makeNode(`c${i}`, chance(kind.ports ?? 0.3)));
    c.children = inner;
    const lo = (c.layoutOptions ??= {});
    delete lo['elk.layered.layering.layerConstraint'];
    lo['elk.algorithm'] = 'layered';
    lo['elk.direction'] = kind.innerDirection ?? pick(DIRECTIONS);
    if (chance(0.5)) lo['elk.layered.directionCongruency'] = 'ROTATION';
    lo['elk.portConstraints'] = pick(kind.compoundPortConstraints ?? ['FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS']);
    if (chance(0.4)) lo['elk.padding'] = `[top=${int(30)},left=${int(30)},bottom=${int(30)},right=${int(30)}]`;
    // The LNode of c and the LGraph of its children get the same objects for
    // these options, and the graph transformer changes them in place.
    if (chance(0.5)) lo['elk.position'] = `(${int(200)},${int(200)})`;
    if (chance(0.5)) lo['elk.nodeLabels.placement'] = pick(PLACEMENTS);
    if (chance(0.5)) lo['elk.nodeLabels.padding'] = `[top=${int(9)},left=${int(9)},bottom=${int(9)},right=${int(9)}]`;
    if (chance(0.4)) lo['elk.spacing.portsSurrounding'] = `[top=${int(15)},left=${int(15)},bottom=${int(15)},right=${int(15)}]`;
    if (rootOptions['elk.edgeRouting'] && chance(0.5)) lo['elk.edgeRouting'] = rootOptions['elk.edgeRouting'];
    const portCount = 2 + int(5);
    c.ports = ports('c', portCount, {
      sides: lo['elk.portConstraints'] === 'FREE' ? undefined : SIDES,
      index: lo['elk.portConstraints'] === 'FIXED_ORDER',
      positions: true,
    });
    // Edges inside the node, to and from its ports.
    c.edges = makeEdges(inner, 1 + int(4), 'i');
    for (const p of c.ports) {
      const target = pick(endpoints(inner));
      if (chance(0.85)) c.edges.push(chance(0.5) ? { id: `ie${nextEdge++}`, sources: [p.id], targets: [target] } : { id: `ie${nextEdge++}`, sources: [target], targets: [p.id] });
      if (chance(0.25)) c.edges.push({ id: `ie${nextEdge++}`, sources: [p.id], targets: [pick(endpoints(inner))] });
    }
    // Edges between two ports of the node: edges between two external port
    // dummies.
    for (let k = int(3); k > 0 && c.ports.length > 1; k--) {
      const a = pick(c.ports).id;
      const b = pick(c.ports).id;
      if (a !== b) c.edges.push({ id: `ie${nextEdge++}`, sources: [a], targets: [b] });
    }
    nodes.push(c);
    graph.edges = makeEdges(
      nodes.filter((n) => n !== c),
      1 + int(nodeCount + 2),
      '',
      c.ports.map((p) => p.id),
    );
  } else {
    graph.edges = makeEdges(nodes, (kind.minEdges ?? 2) + int(nodeCount + 3), '');
  }
  return graph;
}

// The kinds of graphs, and the number of graphs of each kind.
const KINDS = [
  { name: 'plain', n: 10 },
  { name: 'left', n: 6, direction: 'LEFT', alignment: 0.8, position: 0.5 },
  { name: 'up-rotation', n: 6, direction: 'UP', rotation: true, alignment: 0.8, position: 0.5 },
  { name: 'down-rotation', n: 6, direction: 'DOWN', rotation: true, alignment: 0.8, position: 0.5 },
  { name: 'up-reading', n: 4, direction: 'UP', rotation: false, alignment: 0.8, ports: 0.7 },
  { name: 'layer-constraints', n: 10, layerConstraint: 0.45, edgeLabels: 0.6, ports: 0.2 },
  { name: 'constraints-dense', n: 24, layerConstraint: 0.75, edgeLabels: 0.5, ports: 0, minNodes: 4, extraNodes: 3, minEdges: 4 },
  { name: 'first-last', n: 8, layerConstraint: 0.5, layerConstraints: ['FIRST', 'LAST'], edgeLabels: 0.8, ports: 0 },
  { name: 'fixed-side', n: 10, ports: 0.9, portConstraints: ['FIXED_SIDE'], minEdges: 5 },
  { name: 'undefined-sides', n: 8, ports: 0.9, portConstraints: ['FIXED_SIDE', 'FIXED_ORDER'], noSides: true, zeroSize: 0.6, minEdges: 5 },
  { name: 'east-west', n: 10, ports: 0.9, portConstraints: ['FIXED_SIDE'], sides: ['EAST', 'WEST'], minEdges: 6, maxPorts: 3, layerConstraint: 0.15 },
  { name: 'north-south', n: 12, ports: 0.9, portConstraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'], sides: ['NORTH', 'SOUTH'], minEdges: 6 },
  { name: 'north-south-splines', n: 4, ports: 0.9, portConstraints: ['FIXED_SIDE'], sides: ['NORTH', 'SOUTH', 'EAST'], minEdges: 6, routing: 'SPLINES' },
  { name: 'self-loops', n: 10, ports: 1, portConstraints: ['FIXED_SIDE', 'FIXED_ORDER'], sides: ['NORTH', 'SOUTH', 'NORTH', 'SOUTH', 'EAST', 'WEST'], selfLoops: true, minEdges: 8, minNodes: 2, extraNodes: 3 },
  { name: 'self-loops-off', n: 16, patched: true, ports: 1, portConstraints: ['FIXED_SIDE', 'FIXED_SIDE', 'FIXED_ORDER'], sides: ['NORTH', 'SOUTH', 'NORTH', 'SOUTH', 'EAST'], selfLoops: true, minEdges: 8, minNodes: 2, extraNodes: 3, layerConstraint: 0 },
  { name: 'self-loops-off-splines', n: 3, patched: true, routing: 'SPLINES', ports: 1, portConstraints: ['FIXED_SIDE'], sides: ['NORTH', 'SOUTH'], selfLoops: true, minEdges: 8, minNodes: 2, extraNodes: 3, layerConstraint: 0 },
  { name: 'fixed-order', n: 10, ports: 0.9, portConstraints: ['FIXED_ORDER', 'FIXED_POS', 'FIXED_RATIO'], minEdges: 5 },
  { name: 'port-degree', n: 8, ports: 0.9, portConstraints: ['FIXED_SIDE'], sides: ['EAST', 'WEST', 'WEST', 'SOUTH'], portDegree: true, minEdges: 8, maxPorts: 6 },
  { name: 'merge-edges', n: 8, mergeEdges: true, minEdges: 8, extraNodes: 7, edgeLabels: 0.4 },
  { name: 'compound', n: 24, compound: true },
  { name: 'compound-fixed', n: 12, compound: true, compoundPortConstraints: ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'], ports: 0.5 },
  { name: 'compound-include', n: 10, compound: true, includeChildren: true },
];

// patchedDir makes the directory with the worker loader that has self loop
// processing off, and returns its path.
function patchedDir() {
  const dir = mkdtempSync(join(here, '.tmp-elk-intermediate-'));
  const lib = readFileSync(join(here, 'elk-phases-lib.mjs'), 'utf8');
  const anchor = 'const PATCHES = [\n';
  if (lib.split(anchor).length !== 2) throw new Error('elk-phases-lib.mjs: the list of patches is not where it was');
  const patch = `${anchor}  {
    name: 'SelfLoopHolder.needsSelfLoopProcessing (off)',
    old: \`function needsSelfLoopProcessing(lNode){\`,
    new: \`function needsSelfLoopProcessing(lNode){
  return false;\`,
  },
`;
  writeFileSync(join(dir, 'elk-phases-lib.mjs'), lib.replace(anchor, () => patch));
  writeFileSync(join(dir, 'gen-elk-phases.mjs'), readFileSync(join(here, 'gen-elk-phases.mjs'), 'utf8'));
  return dir;
}

function dump(scriptDir, file) {
  return new Promise((done) => {
    execFile(process.execPath, [join(scriptDir, 'gen-elk-phases.mjs'), '--dump', file], { cwd: here, maxBuffer: 1 << 30 }, (err, stdout) => {
      if (err) return done({ err: String(err.message).split('\n')[0] });
      try {
        done(JSON.parse(stdout));
      } catch (e) {
        done({ err: `the output is not JSON: ${e.message}` });
      }
    });
  });
}

async function main() {
  const tmp = mkdtempSync(join(tmpdir(), 'elk-intermediate-'));
  const jobs = [];
  let seed = 20261109;
  for (const kind of KINDS) {
    for (let i = 0; i < kind.n; i++) jobs.push({ name: `${kind.name}-${i}`, graph: randomGraph(seed++, kind), patched: !!kind.patched });
  }
  const results = new Array(jobs.length);
  const patched = patchedDir();
  try {
    let next = 0;
    const worker = async () => {
      while (next < jobs.length) {
        const i = next++;
        const file = join(tmp, `${jobs[i].name}.json`);
        writeFileSync(file, JSON.stringify(jobs[i].graph));
        results[i] = await dump(jobs[i].patched ? patched : here, file);
      }
    };
    await Promise.all(Array.from({ length: 8 }, worker));
  } finally {
    rmSync(patched, { recursive: true, force: true });
    rmSync(tmp, { recursive: true, force: true });
  }

  const cases = [];
  const steps = new Map();
  const throwing = new Map();
  let failed = 0;
  let tooLarge = 0;
  for (const [i, r] of results.entries()) {
    const job = jobs[i];
    // A graph whose layout fails still gives the steps before the failure.
    if (r.err !== undefined && !r.runs) {
      failed++;
      continue;
    }
    if (r.err !== undefined) failed++;
    const runs = [];
    for (const run of r.runs ?? []) {
      const kept = [];
      const states = [];
      const indexOf = new Map();
      const state = (k) => {
        if (!indexOf.has(k)) {
          indexOf.set(k, states.length);
          states.push(run.states[k]);
        }
        return indexOf.get(k);
      };
      let large = false;
      for (const [si, s] of run.steps.entries()) {
        if (!PROCESSORS.has(s.proc) || s.before == null) continue;
        if (s.after == null) {
          // The processor of the last step of a layout that failed threw the
          // exception.
          if (r.err === undefined || run !== r.runs[r.runs.length - 1] || si !== run.steps.length - 1) continue;
          if (JSON.stringify(run.states[s.before]).length > MAX_STATE_BYTES) continue;
          kept.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, before: state(s.before), throws: r.err });
          throwing.set(s.proc, (throwing.get(s.proc) ?? 0) + 1);
          continue;
        }
        if (JSON.stringify(run.states[s.before]).length > MAX_STATE_BYTES || JSON.stringify(run.states[s.after]).length > MAX_STATE_BYTES) {
          large = true;
          continue;
        }
        kept.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, before: state(s.before), after: state(s.after) });
        const c = steps.get(s.proc) ?? { steps: 0, changed: 0 };
        c.steps++;
        if (s.before !== s.after) c.changed++;
        steps.set(s.proc, c);
      }
      if (large) tooLarge++;
      if (kept.length) runs.push({ elk: run.elk, steps: kept, states });
    }
    if (runs.length) cases.push({ name: job.name, source: job.patched ? 'intermediate, self loop processing off' : 'intermediate', in: job.graph, runs });
  }
  mkdirSync(dirname(outFile), { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify({ cases })), { level: 9 });
  writeFileSync(outFile, gz);
  console.log(`${cases.length} cases of ${jobs.length} graphs (${failed} layouts failed, ${tooLarge} runs have a state that is too large), ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  console.log(' steps change throws');
  for (const [proc, c] of [...steps].sort((a, b) => (a[0] < b[0] ? -1 : 1))) console.log(String(c.steps).padStart(6), String(c.changed).padStart(6), String(throwing.get(proc) ?? 0).padStart(6), proc);
}

await main();
