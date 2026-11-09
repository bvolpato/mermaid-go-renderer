// Oracle for the node spacing code of internal/elk/native (the packages
// nodespacing, adapters and nodesize): random ELK graphs that use the options
// of the node size calculation, laid out with the unminified elkjs 0.9.3.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-nodespacing.mjs          write the golden files
//   node gen-elk-nodespacing.mjs --stats  print what the cases cover
//   node gen-elk-nodespacing.mjs --overlaps  write only the golden file of
//                                         the overlap removal
//   node gen-elk-nodespacing.mjs --ports  write only the golden file of the
//                                         port comparator
//   node gen-elk-nodespacing.mjs --print I  print the graph of candidate I
//
// The golden file is internal/elk/native/nodesize/testdata/nodespacing.json.gz.
// A case has the ELK graph and, for each run of the layered algorithm, the
// steps of LabelAndNodeSizeProcessor and InnermostNodeMarginCalculator with
// the state of the LGraph before and after (the format of
// gen-elk-phases.mjs, which this script runs with --dump for each graph).
//
// A step has also the results of direct calls of the node spacing code, made
// on the graph right before the processor runs:
//
//   LabelAndNodeSizeProcessor      for each node of the adapted graph:
//       size       NodeLabelAndSizeCalculator.process(graph, node, false, true)
//       sizeAll    NodeLabelAndSizeCalculator.process(graph, node, false, false)
//       padH       computeInsideNodeLabelPadding(node, RIGHT)
//       padV       computeInsideNodeLabelPadding(node, DOWN)
//       padSingle  computeInsideNodeLabelPadding(adapt(node, false), RIGHT):
//                  the node adapter has no graph
//   InnermostNodeMarginCalculator  the margin of each node of the layers
//       margins    NodeMarginCalculator(adapt(graph, false)).process()
//       marginsNS  NodeMarginCalculator(adapt(graph, true)).process()
//       single     NodeMarginCalculator(adapt(graph, false))
//                      .processNode(adapt(node, false)) for each node
//     all three with the head and tail labels of the edges.
//
// The direct calls leave nothing that the processor does not write itself:
// the processor sets the same port ids and default property values, and it
// replaces each margin. The script checks that: the layout with the direct
// calls must be equal to the layout of gen-elk-phases.mjs, which has none.
//
// The script also writes two small golden files: the overlap removal for
// rectangles (see generateOverlaps) and the port comparator of
// LGraphAdapters (see generatePorts).
import { gzipSync } from 'node:zlib';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { execFile } from 'node:child_process';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk, env, stripHash } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const outFile = resolve(root, 'internal/elk/native/nodesize/testdata/nodespacing.json.gz');

// The number of random graphs. --n N sets another number, to try the script.
const CANDIDATES = process.argv.includes('--n') ? Number(process.argv[process.argv.indexOf('--n') + 1]) : 420;
const MAX_BYTES = 1_800_000; // of the gzip file
const PROCS = ['LabelAndNodeSizeProcessor', 'InnermostNodeMarginCalculator'];
// The number of gen-elk-phases.mjs processes at the same time.
const DUMPS_AT_ONCE = 4;

// ---------------------------------------------------------------------------
// Seeded PRNG (mulberry32).

function prng(seed) {
  let a = seed >>> 0;
  const next = () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  return {
    next,
    int: (lo, hi) => lo + Math.floor(next() * (hi - lo + 1)),
    pick: (arr) => arr[Math.floor(next() * arr.length)],
    chance: (p) => next() < p,
    // A size with a fraction, so that the last bits of sums matter.
    size: (lo, hi) => Math.round((lo + next() * (hi - lo)) * 64) / 64 + (next() < 0.3 ? next() / 7 : 0),
    subset: (arr, p) => arr.filter(() => next() < p),
  };
}

// ---------------------------------------------------------------------------
// Random graphs.

const SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];
const set = (items) => `[${items.join(', ')}]`;
const spacing = (R, lo, hi) => `[top=${R.int(lo, hi)},left=${R.int(lo, hi)},bottom=${R.int(lo, hi)},right=${R.int(lo, hi)}]`;

function nodeLabelPlacement(R) {
  if (R.chance(0.06)) return '[]';
  const items = [R.pick(['INSIDE', 'OUTSIDE']), R.pick(['H_LEFT', 'H_CENTER', 'H_RIGHT']), R.pick(['V_TOP', 'V_CENTER', 'V_BOTTOM'])];
  if (R.chance(0.35)) items.push('H_PRIORITY');
  // A combination that is no location, for example a set without a
  // vertical placement, leaves the label where it is.
  if (R.chance(0.05)) items.pop();
  return set(items);
}

function portLabelPlacement(R) {
  const items = [];
  const where = R.pick(['INSIDE', 'OUTSIDE', 'OUTSIDE', 'INSIDE', undefined]);
  if (where) items.push(where);
  if (R.chance(0.4)) items.push('NEXT_TO_PORT_IF_POSSIBLE');
  if (R.chance(0.35)) items.push(R.pick(['ALWAYS_SAME_SIDE', 'ALWAYS_OTHER_SAME_SIDE', 'SPACE_EFFICIENT']));
  return set(items);
}

// The size options, without SPACE_EFFICIENT_PORT_LABELS. The node size
// calculation does not read that option. DeprecatedLayoutOptionReplacer
// turns it into the port label placement SPACE_EFFICIENT, and for a node
// without a port label placement elkjs adds that constant to the default
// set of the option, which all later nodes and layouts of the engine share
// (the EnumSet of GWT is not cloned). A graph with that option would make
// the result depend on the layouts before it.
function sizeOptions(R) {
  const items = R.subset(
    ['DEFAULT_MINIMUM_SIZE', 'MINIMUM_SIZE_ACCOUNTS_FOR_PADDING', 'COMPUTE_PADDING', 'OUTSIDE_NODE_LABELS_OVERHANG', 'PORTS_OVERHANG', 'UNIFORM_PORT_SPACING', 'FORCE_TABULAR_NODE_LABELS', 'ASYMMETRICAL'],
    0.3,
  );
  return set(items);
}

function label(R, text) {
  return { text, width: R.size(6, 70), height: R.pick([10, 12, 14.5, 21]) };
}

// decorate gives a node the options, the labels and the ports that the node
// size calculation reads.
function decorate(R, node, o) {
  const lo = node.layoutOptions ?? {};
  if (R.chance(0.75)) lo['elk.nodeSize.constraints'] = set(R.subset(['PORTS', 'PORT_LABELS', 'NODE_LABELS', 'MINIMUM_SIZE'], 0.5));
  if (R.chance(0.45)) lo['elk.nodeSize.options'] = sizeOptions(R);
  if (R.chance(0.4)) lo['elk.nodeSize.minimum'] = `(${R.pick([0, R.int(10, 160)])}, ${R.pick([0, R.int(10, 110)])})`;
  if (R.chance(0.6)) lo['elk.nodeLabels.placement'] = nodeLabelPlacement(R);
  if (R.chance(0.12)) lo['elk.nodeLabels.padding'] = spacing(R, 0, 9);
  if (R.chance(0.04) && !o.compound) lo['elk.topdownLayout'] = true;
  const labels = R.pick([0, 0, 1, 1, 1, 2, 3]);
  if (labels > 0) {
    node.labels = [];
    for (let i = 0; i < labels; i++) {
      const l = label(R, `${node.id}.l${i}`);
      if (R.chance(0.35)) l.layoutOptions = { 'elk.nodeLabels.placement': nodeLabelPlacement(R) };
      node.labels.push(l);
    }
  }
  const ports = R.pick([0, 0, 1, 2, 3, 4, 5, 7]);
  if (ports > 0) {
    const constraint = R.pick(['FREE', 'FIXED_SIDE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_ORDER', 'FIXED_POS', 'FIXED_RATIO', undefined]);
    if (constraint) lo['elk.portConstraints'] = constraint;
    // Many ports on one side reach the placement with overlap removal.
    const mainSide = R.chance(0.5) ? R.pick(SIDES) : undefined;
    node.ports = [];
    for (let p = 0; p < ports; p++) {
      const port = { id: `${node.id}.p${p}`, width: R.pick([0, 4, 6.5, 10]), height: R.pick([0, 4, 6.5, 10]) };
      const plo = {};
      const side = mainSide && R.chance(0.75) ? mainSide : R.pick(SIDES);
      if (constraint && constraint !== 'FREE') plo['elk.port.side'] = side;
      if (constraint === 'FIXED_ORDER' && R.chance(0.6)) plo['elk.port.index'] = R.int(0, ports);
      if (constraint === 'FIXED_POS' || constraint === 'FIXED_RATIO') {
        const w = node.width ?? 60;
        const h = node.height ?? 40;
        port.x = side === 'WEST' ? -port.width : side === 'EAST' ? w : R.size(0, Math.max(1, w - port.width));
        port.y = side === 'NORTH' ? -port.height : side === 'SOUTH' ? h : R.size(0, Math.max(1, h - port.height));
      }
      if (R.chance(0.3)) plo['elk.port.borderOffset'] = R.pick([-6, -2.5, 0, 3, 8]);
      const plabels = R.pick([0, 0, 1, 1, 2]);
      if (plabels > 0) {
        port.labels = [];
        for (let i = 0; i < plabels; i++) {
          const l = label(R, `${port.id}.l${i}`);
          // A fixed port label keeps its position.
          if (R.chance(0.5)) {
            l.x = R.size(-25, 25);
            l.y = R.size(-18, 18);
          }
          port.labels.push(l);
        }
      }
      if (Object.keys(plo).length > 0) port.layoutOptions = plo;
      node.ports.push(port);
    }
    if (R.chance(0.7)) lo['elk.portLabels.placement'] = portLabelPlacement(R);
    if (R.chance(0.3)) lo['elk.portLabels.treatAsGroup'] = R.chance(0.5);
    if (R.chance(0.6)) lo['elk.portAlignment.default'] = R.pick(['CENTER', 'BEGIN', 'END', 'DISTRIBUTED', 'JUSTIFIED']);
    for (const s of ['north', 'south', 'east', 'west']) {
      if (R.chance(0.12)) lo[`elk.portAlignment.${s}`] = R.pick(['CENTER', 'BEGIN', 'END', 'DISTRIBUTED', 'JUSTIFIED']);
    }
    if (R.chance(0.25)) lo['elk.spacing.portsSurrounding'] = spacing(R, 0, 14);
  }
  if (Object.keys(lo).length > 0) node.layoutOptions = lo;
}

// labeledPorts replaces the ports of a node with ports that have labels, on
// fixed sides most of the time, and sets the options of the port labels.
function labeledPorts(R, node) {
  const lo = (node.layoutOptions ??= {});
  const constraint = R.pick(['FIXED_SIDE', 'FIXED_SIDE', 'FIXED_ORDER', 'FREE']);
  lo['elk.portConstraints'] = constraint;
  node.ports = [];
  const ports = R.int(2, 6);
  for (let p = 0; p < ports; p++) {
    const port = { id: `${node.id}.p${p}`, width: R.pick([0, 4, 6.5, 10]), height: R.pick([0, 4, 6.5, 10]) };
    if (constraint !== 'FREE') port.layoutOptions = { 'elk.port.side': R.pick(SIDES) };
    if (R.chance(0.8)) {
      port.labels = [];
      const k = R.int(1, 2);
      for (let i = 0; i < k; i++) port.labels.push(label(R, `${port.id}.l${i}`));
    }
    node.ports.push(port);
  }
  const placement = [R.pick(['INSIDE', 'OUTSIDE'])];
  if (R.chance(0.5)) placement.push('NEXT_TO_PORT_IF_POSSIBLE');
  lo['elk.portLabels.placement'] = set(placement);
  if (R.chance(0.6)) lo['elk.portLabels.treatAsGroup'] = false;
  else delete lo['elk.portLabels.treatAsGroup'];
  lo['elk.nodeSize.constraints'] = set(R.chance(0.7) ? ['PORTS', 'PORT_LABELS', ...R.subset(['NODE_LABELS', 'MINIMUM_SIZE'], 0.4)] : R.subset(['PORTS', 'PORT_LABELS', 'NODE_LABELS', 'MINIMUM_SIZE'], 0.5));
}

// graphOptions are the options that the node size calculation reads from the
// graph of a node.
function graphOptions(R, lo) {
  if (R.chance(0.8)) lo['elk.direction'] = R.pick(['RIGHT', 'RIGHT', 'DOWN', 'DOWN', 'LEFT', 'UP']);
  if (R.chance(0.3)) lo['elk.spacing.portPort'] = R.pick([0, 4, 12.5, 20]);
  if (R.chance(0.3)) lo['elk.spacing.labelLabel'] = R.pick([0, 2, 3.5, 8]);
  if (R.chance(0.3)) lo['elk.spacing.labelNode'] = R.pick([0, 2, 7.25, 12]);
  if (R.chance(0.3)) lo['elk.spacing.labelPortHorizontal'] = R.pick([-3, 0, 2, 5.5]);
  if (R.chance(0.3)) lo['elk.spacing.labelPortVertical'] = R.pick([-3, 0, 2, 5.5]);
  if (R.chance(0.25)) lo['elk.spacing.portsSurrounding'] = spacing(R, 0, 14);
  if (R.chance(0.15)) lo['elk.nodeLabels.padding'] = spacing(R, 0, 9);
  if (R.chance(0.15)) lo['elk.nodeLabels.placement'] = nodeLabelPlacement(R);
  if (R.chance(0.2)) lo['elk.portLabels.placement'] = portLabelPlacement(R);
  if (R.chance(0.08)) lo['elk.nodeSize.fixedGraphSize'] = true;
  return lo;
}

function randomGraph(R) {
  const g = { id: 'root', layoutOptions: graphOptions(R, { 'elk.algorithm': 'layered' }), children: [], edges: [] };
  if (R.chance(0.35)) g.layoutOptions['elk.hierarchyHandling'] = R.pick(['INCLUDE_CHILDREN', 'SEPARATE_CHILDREN']);
  const leaves = [];
  const containerOf = new Map();
  const n = R.int(1, 5);
  for (let i = 0; i < n; i++) {
    const node = { id: `n${i}`, width: R.size(20, 150), height: R.size(20, 100) };
    decorate(R, node, {});
    g.children.push(node);
    containerOf.set(node.id, g);
    leaves.push(node);
  }
  // A compound node with children: its ports can have connections to the
  // inside, and its graph has external port dummies. A part of the graphs
  // has a compound node with labeled ports on fixed sides, for the port
  // labels of compound nodes and of external port dummies.
  const focus = R.chance(0.3);
  if (focus || R.chance(0.4)) {
    const c = { id: 'c', children: [], edges: [], layoutOptions: graphOptions(R, {}) };
    if (R.chance(0.5)) c.layoutOptions['elk.padding'] = spacing(R, 4, 30);
    if (R.chance(0.2)) c.layoutOptions['elk.insideSelfLoops.activate'] = true;
    decorate(R, c, { compound: true });
    if (focus) labeledPorts(R, c);
    const m = R.int(1, 3);
    for (let i = 0; i < m; i++) {
      const node = { id: `c.n${i}`, width: R.size(20, 110), height: R.size(20, 80) };
      decorate(R, node, {});
      c.children.push(node);
      containerOf.set(node.id, c);
      leaves.push(node);
    }
    g.children.push(c);
    containerOf.set(c.id, g);
    leaves.push(c);
  }
  // A comment box that the layered algorithm attaches to its node.
  if (R.chance(0.2)) {
    const target = R.pick(g.children);
    const comment = { id: 'comment', width: R.size(20, 60), height: R.size(12, 30), layoutOptions: { 'elk.commentBox': true } };
    if (R.chance(0.5)) comment.labels = [label(R, 'comment.l')];
    g.children.push(comment);
    g.edges.push({ id: 'ec', sources: [comment.id], targets: [target.id] });
  }
  // A node without children that is treated as a compound node, with
  // labeled ports: the inside port labels that are not next to their port.
  if (focus && R.chance(0.5)) {
    const node = R.pick(g.children.filter((child) => !child.children));
    labeledPorts(R, node);
    node.layoutOptions['elk.insideSelfLoops.activate'] = true;
  }
  // Edges between the ports of the compound node and its children.
  if (focus) {
    const c = g.children.find((node) => node.id === 'c');
    for (const port of c.ports) {
      if (!R.chance(0.6)) continue;
      const child = R.pick(c.children);
      const inside = child.ports && R.chance(0.5) ? R.pick(child.ports).id : child.id;
      c.edges.push(R.chance(0.5) ? { id: `i.${port.id}`, sources: [port.id], targets: [inside] } : { id: `i.${port.id}`, sources: [inside], targets: [port.id] });
    }
  }
  const endpoint = (node) => (node.ports && R.chance(0.75) ? R.pick(node.ports).id : node.id);
  const edges = R.int(0, 2 * leaves.length);
  for (let e = 0; e < edges; e++) {
    const a = R.pick(leaves);
    // Some self loops.
    const b = R.chance(0.12) ? a : R.pick(leaves);
    const pa = containerOf.get(a.id);
    const pb = containerOf.get(b.id);
    // The edge is in the lowest common ancestor. A child of the compound
    // node and the compound node itself meet in the compound node.
    let owner = pa === pb ? pa : g;
    if (a.id === 'c' && pb.id === 'c') owner = b === a ? g : pb;
    if (b.id === 'c' && pa.id === 'c') owner = pa;
    const edge = { id: `e${e}`, sources: [endpoint(a)], targets: [endpoint(b)] };
    if (R.chance(0.35)) {
      edge.labels = [];
      const k = R.int(1, 2);
      for (let i = 0; i < k; i++) {
        const l = label(R, `e${e}.l${i}`);
        l.layoutOptions = { 'elk.edgeLabels.placement': R.pick(['HEAD', 'TAIL', 'CENTER']) };
        edge.labels.push(l);
      }
    }
    owner.edges.push(edge);
  }
  return g;
}

// ---------------------------------------------------------------------------
// The direct calls.

// num keeps a double exact in JSON.
const num = (x) => (Number.isFinite(x) && !Object.is(x, -0) ? x : Object.is(x, -0) ? '-0' : String(x));

function makeDirect() {
  env.install();
  const elk = loadElk();
  const S = elk.hook.scope;
  const getName = S('$getName');
  const getClass = S('getClass__Ljava_lang_Class___devirtual$');
  const procName = (p) => (typeof p === 'string' ? p : getName(getClass(p)).replace(/^.*\./, ''));
  S('$clinit_LGraphAdapters')();
  S('$clinit_Direction_0')();
  const LGraphAdapter = S('LGraphAdapters$LGraphAdapter');
  const LNodeAdapter = S('LGraphAdapters$LNodeAdapter');
  const NormalFilter = S('LabelAndNodeSizeProcessor$lambda$0$Type');
  const AllFilter = S('LGraphAdapters$lambda$0$Type');
  const NodeMarginCalculator = S('NodeMarginCalculator');
  const process = S('process');
  const computeInsideNodeLabelPadding = S('computeInsideNodeLabelPadding');
  const marginProcess = S('$process');
  const marginProcessNode = S('$processNode');
  const RIGHT = S('RIGHT_6');
  const DOWN = S('DOWN_1');
  const vec = (v) => [num(v.x_0), num(v.y_0)];
  const sp = (s) => [num(s.top_0), num(s.right), num(s.bottom), num(s.left)];
  const layerNodes = (graph) => {
    const out = [];
    for (const layer of graph.layers.array) for (const node of layer.nodes.array) out.push(node);
    return out;
  };

  let runs = null;
  let run = null;
  elk.hook.runStart = () => {
    run = [];
    runs.push(run);
  };
  elk.hook.before = (kind, p, graph) => {
    if (!runs) return;
    const name = procName(p);
    if (name === 'LabelAndNodeSizeProcessor') {
      const ga = new LGraphAdapter(graph, true, true, new NormalFilter());
      const nodes = [];
      for (const na of ga.getNodes().array) {
        nodes.push({
          size: vec(process(ga, na, false, true)),
          sizeAll: vec(process(ga, na, false, false)),
          padH: sp(computeInsideNodeLabelPadding(na, RIGHT)),
          padV: sp(computeInsideNodeLabelPadding(na, DOWN)),
          padSingle: sp(computeInsideNodeLabelPadding(new LNodeAdapter(null, na.element, false), RIGHT)),
        });
      }
      run.push({ proc: name, nodes });
    } else if (name === 'InnermostNodeMarginCalculator') {
      const all = layerNodes(graph);
      marginProcess(new NodeMarginCalculator(new LGraphAdapter(graph, false, false, new AllFilter())));
      const margins = all.map((node) => sp(node.margin));
      marginProcess(new NodeMarginCalculator(new LGraphAdapter(graph, true, false, new AllFilter())));
      const marginsNS = all.map((node) => sp(node.margin));
      const calc = new NodeMarginCalculator(new LGraphAdapter(graph, false, false, new AllFilter()));
      const single = all.map((node) => {
        marginProcessNode(calc, new LNodeAdapter(null, node, false));
        return sp(node.margin);
      });
      run.push({ proc: name, margins, marginsNS, single });
    }
  };

  return (input) => {
    runs = [];
    env.beginLayout();
    try {
      const out = JSON.stringify(elk.layout(JSON.parse(input)));
      return { runs, out };
    } catch (e) {
      return { err: String(e && e.message !== undefined ? e.message : e) };
    } finally {
      runs = null;
    }
  };
}

// ---------------------------------------------------------------------------
// The states, from gen-elk-phases.mjs --dump.

function dumpStates(dir, name, input) {
  const file = join(dir, `${name}.json`);
  writeFileSync(file, input);
  return new Promise((done, fail) => {
    execFile(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, maxBuffer: 1 << 30, encoding: 'utf8' }, (err, stdout, stderr) => {
      if (err) fail(new Error(`gen-elk-phases.mjs --dump failed: ${stderr || err}`));
      else done(JSON.parse(stdout));
    });
  });
}

// features names what the nodes of a graph have, to choose graphs that
// differ.
function features(graph) {
  const out = new Set();
  const walk = (parent) => {
    const gp = parent.layoutOptions ?? {};
    out.add(`dir=${gp['elk.direction'] ?? '-'}`);
    for (const key of ['elk.spacing.portPort', 'elk.spacing.labelLabel', 'elk.spacing.labelNode', 'elk.spacing.labelPortHorizontal', 'elk.spacing.labelPortVertical', 'elk.nodeSize.fixedGraphSize']) {
      if (gp[key] !== undefined) out.add(`graph ${key}=${gp[key]}`);
    }
    for (const node of parent.children ?? []) {
      const lo = node.layoutOptions ?? {};
      const kind = node.children ? 'compound' : 'node';
      const list = (id) => (lo[id] ?? gp[id] ?? '-').replace(/[\[\] ]/g, '').split(',');
      const pc = lo['elk.portConstraints'] ?? '-';
      const plp = list('elk.portLabels.placement').join('+');
      const sides = new Map();
      let portLabels = 0;
      let offsets = false;
      for (const port of node.ports ?? []) {
        const side = port.layoutOptions?.['elk.port.side'] ?? '?';
        sides.set(side, (sides.get(side) ?? 0) + 1);
        portLabels = Math.max(portLabels, (port.labels ?? []).length);
        offsets ||= port.layoutOptions?.['elk.port.borderOffset'] !== undefined;
      }
      const many = [...sides].filter(([, c]) => c > 2).map(([side]) => side[0]).sort().join('');
      const sc = list('elk.nodeSize.constraints');
      out.add(`${kind} pc=${pc} plp=${plp} labels=${portLabels} many=${many}`);
      if (kind === 'compound' && portLabels > 0) {
        const group = lo['elk.portLabels.treatAsGroup'] ?? '-';
        out.add(`compound ports plp=${plp} group=${group} sides=${[...sides.keys()].map((side) => side[0]).sort().join('')} pl=${sc.includes('PORT_LABELS')} dir=${lo['elk.direction'] ?? gp['elk.direction'] ?? '-'}`);
      }
      out.add(`${kind} pc=${pc} offsets=${offsets} labels=${portLabels > 0}`);
      out.add(`${kind} sc=${sc.join('+')} pc=${pc}`);
      for (const o of list('elk.nodeSize.options')) out.add(`${kind} so=${o} sc=${sc.join('+')}`);
      const labels = node.labels ?? [];
      out.add(`${kind} nlp=${list('elk.nodeLabels.placement').join('+')} n=${labels.length}`);
      for (const l of labels) {
        if (l.layoutOptions) out.add(`${kind} label nlp=${l.layoutOptions['elk.nodeLabels.placement']}`);
      }
      for (const key of ['default', 'north', 'south', 'east', 'west']) {
        const v = lo[`elk.portAlignment.${key}`];
        if (v !== undefined) out.add(`${kind} align.${key}=${v} pc=${key === 'default' ? pc : '*'} ports=${Math.min(2, (node.ports ?? []).length)}`);
      }
      if (lo['elk.portLabels.treatAsGroup'] !== undefined) out.add(`${kind} group=${lo['elk.portLabels.treatAsGroup']} plp=${plp} labels=${portLabels}`);
      if (lo['elk.topdownLayout']) out.add(`${kind} topdown sc=${sc.join('+')}`);
      if (lo['elk.commentBox']) out.add(`comment labels=${labels.length}`);
      if (lo['elk.insideSelfLoops.activate']) out.add(`insideSelfLoops plp=${plp}`);
      if (node.children) walk(node);
    }
    for (const edge of parent.edges ?? []) {
      if (edge.sources[0] === edge.targets[0] || edge.sources[0].split('.p')[0] === edge.targets[0].split('.p')[0]) out.add('selfloop');
      for (const l of edge.labels ?? []) out.add(`edge label ${l.layoutOptions['elk.edgeLabels.placement']}`);
    }
  };
  walk(graph);
  return out;
}

// pool runs the tasks with a few of them at the same time.
async function pool(tasks, width) {
  const results = new Array(tasks.length);
  let next = 0;
  const worker = async () => {
    while (next < tasks.length) {
      const i = next++;
      results[i] = await tasks[i]();
    }
  };
  await Promise.all(Array.from({ length: width }, worker));
  return results;
}

async function generate() {
  const direct = makeDirect();
  const dir = mkdtempSync(join(tmpdir(), 'elk-nodespacing-'));
  const seen = new Set();
  const chosen = [];
  let failed = 0;
  let skipped = 0;
  // The direct calls, with one engine for all graphs. Keep a graph that has
  // something that no graph before it has.
  for (let i = 0; i < CANDIDATES; i++) {
    const graph = randomGraph(prng(77000 + i));
    const input = JSON.stringify(graph);
    const d = direct(input);
    if (d.err !== undefined) {
      failed++;
      continue;
    }
    if (!d.runs.some((run) => run.length > 0)) continue;
    let fresh = false;
    for (const f of features(graph)) {
      if (!seen.has(f)) {
        seen.add(f);
        fresh = true;
      }
    }
    if (!fresh) {
      skipped++;
      continue;
    }
    chosen.push({ i, graph, input, d });
  }
  // The states, with a new engine for each graph. The outputs must be equal:
  // a graph whose result depends on the layouts before it is an error here.
  let cases;
  try {
    cases = await pool(
      chosen.map((c) => async () => {
        const dump = await dumpStates(dir, `g${c.i}`, c.input);
        if (dump.err !== undefined || stripHash(dump.out) !== stripHash(c.d.out)) throw new Error(`graph ${c.i}: the two runs of elkjs differ`);
        const runs = [];
        dump.runs.forEach((run, ri) => {
          const states = [];
          const index = new Map();
          const keep = (state) => {
            if (!index.has(state)) {
              index.set(state, states.length);
              states.push(run.states[state]);
            }
            return index.get(state);
          };
          const steps = [];
          let k = 0;
          for (const step of run.steps) {
            if (!PROCS.includes(step.proc)) continue;
            const dd = c.d.runs[ri][k++];
            if (!dd || dd.proc !== step.proc) throw new Error(`graph ${c.i}: the direct calls and the steps do not match`);
            steps.push({ ...dd, kind: step.kind, proc: step.proc, slot: step.slot, graph: step.graph, before: keep(step.before), after: keep(step.after) });
          }
          if (k !== c.d.runs[ri].length) throw new Error(`graph ${c.i}: the direct calls and the steps do not match`);
          if (steps.length > 0) runs.push({ elk: run.elk, steps, states });
        });
        return { name: `nodespacing-${c.i}`, in: c.graph, runs };
      }),
      DUMPS_AT_ONCE,
    );
  } finally {
    rmSync(dir, { recursive: true, force: true });
  }
  let gz;
  // Drop cases from the end until the file is small enough.
  for (;;) {
    gz = gzipSync(Buffer.from(JSON.stringify({ cases })), { level: 9 });
    if (gz.length <= MAX_BYTES) break;
    cases.length = Math.floor(cases.length * 0.9);
  }
  mkdirSync(dirname(outFile), { recursive: true });
  writeFileSync(outFile, gz);
  const steps = cases.reduce((a, c) => a + c.runs.reduce((b, r) => b + r.steps.length, 0), 0);
  console.log(`${cases.length} cases, ${steps} steps, ${seen.size} features, ${(gz.length / 1e6).toFixed(2)} MB gzip (${failed} graphs failed in elkjs, ${skipped} had nothing new)`);
  if (process.argv.includes('--stats')) for (const f of [...seen].sort()) console.log(f);
}

// ---------------------------------------------------------------------------
// The overlap removal for rectangles along a strip
// (org.eclipse.elk.alg.common.overlaps), which the port label placement
// uses: random rectangles, each of the four directions.
//
// The golden file is
// internal/elk/native/nodespacing/overlaps/testdata/overlaps.json.gz. A case
// has the direction, the vertical gap, the start coordinate and the
// rectangles [x, y, width, height]. out has the rectangles after
// removeOverlaps, in the order of the input, and strip has its result.

const overlapsFile = resolve(root, 'internal/elk/native/nodespacing/overlaps/testdata/overlaps.json.gz');

function generateOverlaps() {
  env.install();
  const elk = loadElk();
  const S = elk.hook.scope;
  S('$clinit_RectangleStripOverlapRemover$OverlapRemovalDirection')();
  const directions = { UP: S('UP'), DOWN: S('DOWN'), LEFT: S('LEFT_0'), RIGHT: S('RIGHT_0') };
  const createForDirection = S('createForDirection');
  const withGap = S('$withGap');
  const add = S('$add_3');
  const RectangleNode = S('RectangleStripOverlapRemover$RectangleNode');
  const importRectangle = S('$importRectangle');
  const removeOverlaps = S('$removeOverlaps_0');
  const ElkRectangle = S('ElkRectangle_0');
  const R = prng(424242);
  const cases = [];
  for (let i = 0; i < 600; i++) {
    const direction = R.pick(['UP', 'DOWN', 'LEFT', 'RIGHT']);
    const gap = R.pick([0, 1, 2.5, 5, -1]);
    const start = R.pick([0, -7.25, 13, R.size(-50, 50)]);
    // Coordinates on a grid give rectangles with equal borders, which the
    // sorted set of the scan line treats as one.
    const grid = R.chance(0.6);
    const coord = (lo, hi) => (grid ? 5 * R.int(Math.ceil(lo / 5), Math.floor(hi / 5)) : R.size(lo, hi));
    const rects = [];
    const n = R.int(0, 9);
    for (let k = 0; k < n; k++) rects.push([coord(-40, 80), coord(-30, 30), coord(0, 45), coord(0, 30)]);
    const remover = withGap(createForDirection(directions[direction]), gap);
    const objects = rects.map((r) => new ElkRectangle(r[0], r[1], r[2], r[3]));
    for (const o of objects) add(remover.rectangleNodes, new RectangleNode(o, importRectangle(remover, o)));
    remover.startCoordinate = start;
    const strip = removeOverlaps(remover);
    cases.push({ direction, gap, start: num(start), rects: rects.map((r) => r.map(num)), out: objects.map((o) => [num(o.x_0), num(o.y_0), num(o.width_0), num(o.height)]), strip: num(strip) });
  }
  const gz = gzipSync(Buffer.from(JSON.stringify({ cases })), { level: 9 });
  mkdirSync(dirname(overlapsFile), { recursive: true });
  writeFileSync(overlapsFile, gz);
  console.log(`${cases.length} overlap removal cases, ${(gz.length / 1e3).toFixed(0)} kB gzip`);
}

// ---------------------------------------------------------------------------
// The port comparator of LGraphAdapters (DEFAULT_PORTLIST_SORTER): random
// pairs of ports.
//
// The golden file is internal/elk/native/adapters/testdata/portcomparator.json.gz.
// A case has two ports (side, x, y and the port index or null) and cmp, the
// result of the comparison, or throws for a port without a side.

const portsFile = resolve(root, 'internal/elk/native/adapters/testdata/portcomparator.json.gz');

function generatePorts() {
  env.install();
  const elk = loadElk();
  const S = elk.hook.scope;
  S('$clinit_LayeredOptions')();
  S('$clinit_PortSide')();
  const LPort = S('LPort');
  const sides = { UNDEFINED: S('UNDEFINED_5'), NORTH: S('NORTH_3'), EAST: S('EAST_2'), SOUTH: S('SOUTH_2'), WEST: S('WEST_2') };
  const setProperty = S('$setProperty_0');
  const PORT_INDEX = S('PORT_INDEX');
  const boxInt = S('valueOf_3');
  const compare = S('$compare_10');
  if (!String(compare).includes('Port side is undefined')) throw new Error('$compare_10 is not the port comparator of LGraphAdapters');
  const R = prng(31337);
  const coords = [0, -0, 1, 2.5, -3, 10, NaN, Infinity, -Infinity];
  const make = (d) => {
    const port = new LPort();
    port.side = sides[d.side];
    port.pos.x_0 = d.x;
    port.pos.y_0 = d.y;
    if (d.index !== null) setProperty(port, PORT_INDEX, boxInt(d.index));
    return port;
  };
  const cases = [];
  for (let i = 0; i < 1500; i++) {
    const side = R.chance(0.04) ? 'UNDEFINED' : R.pick(SIDES);
    const describe = (s) => ({ side: s, x: R.pick(coords), y: R.pick(coords), index: R.chance(0.5) ? R.pick([0, 1, 2, 7, -4]) : null });
    const a = describe(side);
    const b = describe(R.chance(0.75) ? side : R.pick(SIDES));
    const c = { a: { ...a, x: num(a.x), y: num(a.y) }, b: { ...b, x: num(b.x), y: num(b.y) } };
    try {
      c.cmp = compare(make(a), make(b));
    } catch (e) {
      c.throws = true;
    }
    cases.push(c);
  }
  const gz = gzipSync(Buffer.from(JSON.stringify({ cases })), { level: 9 });
  mkdirSync(dirname(portsFile), { recursive: true });
  writeFileSync(portsFile, gz);
  console.log(`${cases.length} port comparator cases (${cases.filter((c) => c.throws).length} throw), ${(gz.length / 1e3).toFixed(0)} kB gzip`);
}

if (process.argv.includes('--print')) {
  // The graph of one candidate, to look at it with gen-elk-phases.mjs --dump.
  const i = Number(process.argv[process.argv.indexOf('--print') + 1]);
  console.log(JSON.stringify(randomGraph(prng(77000 + i))));
} else if (process.argv.includes('--overlaps')) {
  generateOverlaps();
} else if (process.argv.includes('--ports')) {
  generatePorts();
} else {
  generateOverlaps();
  generatePorts();
  await generate();
}
