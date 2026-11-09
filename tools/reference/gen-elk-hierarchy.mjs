// Extra phase goldens for internal/elk/native/compound and
// internal/elk/native/hports: the states of the LGraph before and after the
// compound graph processors, the hierarchical node resizing, the inverted
// port processor and the four hierarchical port processors of ELK layered.
//
// The shared phase goldens come from mermaid graphs, which have no ports and
// no port constraints. So there the hierarchical port processors do nothing
// but set the x coordinate of dummy nodes. The graphs of this script have
// what mermaid does not use: ports on compound nodes, on each side, with
// each port constraint, port labels, edges that cross one or two hierarchy
// borders, edges to the parent node, merged hierarchy edges, edge labels on
// cross-hierarchy edges, hyperedges (junction points), minimum sizes,
// content alignment, child nodes that are laid out separately, inverted
// ports and inside self loops. The root node has no ports: elkjs fails for
// a port of the root node that has an edge to the inside.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-hierarchy.mjs
//
// The script makes seeded random ELK graphs and runs
// `node gen-elk-phases.mjs --dump` for each of them, so the states come from
// the same instrumented elkjs 0.9.3 as the shared goldens. It keeps the
// steps of the processors of each package and the states that they name, in
// the format of internal/elk/native/nativetest (one Case for each graph,
// with the input and the layout result of elkjs), and writes the two files
// below. It also keeps the layouts that fail in elkjs in one of these
// processors, with the state before the failure.
//
//   internal/elk/native/compound/testdata/compound-steps.json.gz
//   internal/elk/native/hports/testdata/hports-steps.json.gz
import { execFile } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');

const PACKAGES = [
  {
    file: resolve(root, 'internal/elk/native/compound/testdata/compound-steps.json.gz'),
    source: 'compound',
    processors: new Set(['CompoundGraphPreprocessor', 'CompoundGraphPostprocessor']),
    // The largest state and the largest case that a golden keeps of the
    // random graphs, and the number of random cases.
    maxStateBytes: 60000,
    maxCaseBytes: 400000,
    maxCases: 125,
  },
  {
    file: resolve(root, 'internal/elk/native/hports/testdata/hports-steps.json.gz'),
    source: 'hports',
    processors: new Set([
      'HierarchicalNodeResizingProcessor',
      'InvertedPortProcessor',
      'HierarchicalPortConstraintProcessor',
      'HierarchicalPortDummySizeProcessor',
      'HierarchicalPortPositionProcessor',
      'HierarchicalPortOrthogonalEdgeRouter',
    ]),
    maxStateBytes: 40000,
    maxCaseBytes: 330000,
    maxCases: 110,
    // The graphs with two hierarchy levels have many large states, and the
    // random graphs have such hierarchies too.
    skipSpecials: /^special-deep/,
  },
];

const DIRECTIONS = ['RIGHT', 'DOWN', 'LEFT', 'UP'];
const PORT_SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];
const PORT_CONSTRAINTS = ['FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_RATIO', 'FIXED_POS'];

// The number of graphs.
const GRAPHS = Number(process.env.ELK_HIERARCHY_GRAPHS ?? 240);

// mulberry32 is a small seeded generator.
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

// graph makes the ELK graph with the given index.
//
// ELK lays out a node with the option SEPARATE_CHILDREN in a run of its own,
// and an edge cannot cross the border of such a node. So each end of an edge
// has the regions in which it can connect: the region of the graph that has
// its node, and for a port of a node with children also the region inside
// that node.
function graph(index) {
  const rnd = mulberry32(0x41e2a2c + index * 7919);
  const int = (n) => Math.floor(rnd() * n);
  const pick = (list) => list[int(list.length)];
  const chance = (p) => rnd() < p;
  // Sizes with fractions, as text measurement gives them.
  const size = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;

  // The kind of the graph decides what it is about.
  //   0  compound layout without ports: edges that cross hierarchy borders
  //   1  compound layout with ports on the compound nodes
  //   2  each node with children is laid out separately, and has ports
  //   3  compound layout in which some nodes are laid out separately
  //   4  flat graph with inverted ports
  //   5  compound layout with everything
  const kind = index % 6;
  const rootInclude = kind !== 2 && kind !== 4;

  const ends = [];
  let nodeCounter = 0;
  let portCounter = 0;

  const portLabel = (id) => ({ text: id, width: size(8, 40), height: size(8, 18), x: size(-30, 10), y: size(-20, 10) });

  // ports makes the ports of a node. sides limits the sides.
  const ports = (node, count, withLabels, sides) => {
    const list = [];
    for (let p = 0; p < count; p++) {
      const side = pick(sides);
      const pw = pick([0, 6, 8, 10.5]);
      const ph = pick([0, 6, 8, 10.5]);
      const port = { id: `p${portCounter++}`, width: pw, height: ph };
      const w = node.width ?? 60;
      const h = node.height ?? 40;
      if (side === 'WEST') {
        port.x = -pw;
        port.y = size(0, h);
      } else if (side === 'EAST') {
        port.x = w;
        port.y = size(0, h);
      } else if (side === 'NORTH') {
        port.x = size(0, w);
        port.y = -ph;
      } else {
        port.x = size(0, w);
        port.y = h;
      }
      const lo = {};
      if (chance(0.9)) lo['elk.port.side'] = side;
      if (chance(0.3)) lo['elk.port.index'] = int(6);
      if (chance(0.25)) lo['elk.port.borderOffset'] = pick([-12, -4.5, 0, 3, 7.25]);
      if (chance(0.12)) lo['elk.port.anchor'] = `(${size(0, 8)}, ${size(0, 8)})`;
      if (Object.keys(lo).length > 0) port.layoutOptions = lo;
      if (withLabels && chance(0.6)) {
        port.labels = [portLabel(`${port.id}.a`)];
        if (chance(0.3)) port.labels.push(portLabel(`${port.id}.b`));
      }
      list.push(port);
    }
    return list;
  };

  const leaf = (region) => {
    const node = { id: `n${nodeCounter++}`, width: size(20, 110), height: size(20, 80) };
    if ((kind === 4 || kind === 5) && chance(kind === 4 ? 0.8 : 0.25)) {
      node.ports = ports(node, 1 + int(4), false, PORT_SIDES);
      node.layoutOptions = { 'elk.portConstraints': pick(['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS']) };
      for (const p of node.ports) ends.push({ id: p.id, node: node.id, regions: [region], port: true });
    }
    if (chance(0.15)) node.labels = [{ text: node.id, width: size(10, 50), height: size(10, 20) }];
    ends.push({ id: node.id, node: node.id, regions: [region] });
    return node;
  };

  // containerOptions sets the options of a node that has children.
  const containerOptions = (node, isRoot) => {
    const lo = node.layoutOptions ?? (node.layoutOptions = {});
    if (chance(0.5)) lo['elk.direction'] = pick(DIRECTIONS);
    if (chance(0.5)) lo['elk.padding'] = `[top=${pick([0, 8, 20.5])},left=${pick([0, 8, 12])},bottom=${pick([0, 8, 14])},right=${pick([0, 8, 30.25])}]`;
    if (chance(0.3)) {
      lo['elk.nodeSize.constraints'] = pick(['MINIMUM_SIZE', '[MINIMUM_SIZE, PORT_LABELS]', 'PORT_LABELS', '[PORTS, PORT_LABELS, MINIMUM_SIZE]']);
      if (chance(0.7)) lo['elk.nodeSize.minimum'] = `(${pick([0, 150, 320.5])}, ${pick([0, 120, 260.25])})`;
      if (chance(0.3)) lo['elk.nodeSize.options'] = pick(['[]', 'DEFAULT_MINIMUM_SIZE']);
    }
    if (chance(0.25)) lo['elk.contentAlignment'] = pick(['[H_CENTER, V_CENTER]', '[H_RIGHT, V_BOTTOM]', 'H_CENTER', 'V_BOTTOM', '[H_LEFT, V_CENTER]', '[H_RIGHT, V_TOP]']);
    if (chance(0.2)) lo['elk.layered.mergeHierarchyEdges'] = chance(0.5);
    if (chance(0.15)) lo['elk.layered.mergeEdges'] = true;
    if (chance(0.15)) lo['elk.spacing.portPort'] = pick([0, 4, 22.5]);
    if (chance(0.15)) lo['elk.spacing.edgeEdge'] = pick([3, 14.5]);
    if (chance(0.15)) lo['elk.spacing.nodeNode'] = pick([8, 33.5]);
    if (chance(0.15)) lo['elk.layered.spacing.edgeEdgeBetweenLayers'] = pick([4, 17.25]);
    if (!isRoot && chance(0.1)) lo['elk.layered.unnecessaryBendpoints'] = true;
  };

  // container makes a node with children. region is the region of the graph
  // that has the node, and parentInclude tells that this graph is part of a
  // compound layout.
  const container = (region, parentInclude, depth) => {
    const node = { id: `c${nodeCounter++}`, children: [] };
    if (chance(0.6)) {
      node.width = size(80, 240);
      node.height = size(60, 200);
    }
    node.layoutOptions = {};
    containerOptions(node, false);

    // The hierarchy handling of the node, and the region inside it.
    let include = parentInclude;
    if (kind === 3 && chance(0.6)) include = false;
    if (kind === 5 && chance(0.15)) include = false;
    if (kind === 2 && depth === 1 && chance(0.25)) include = true;
    if (include !== parentInclude) node.layoutOptions['elk.hierarchyHandling'] = include ? 'INCLUDE_CHILDREN' : 'SEPARATE_CHILDREN';
    const border = !parentInclude || !include;
    const inside = border ? node.id : region;

    const withPorts = kind === 1 || kind === 2 || (kind === 3 && border) || (kind === 5 && chance(0.7));
    if (withPorts) {
      // A compound layout gives no node size to the dummy of a port, and
      // fails for the constraint FIXED_RATIO.
      const constraints = pick(border ? PORT_CONSTRAINTS : PORT_CONSTRAINTS.filter((c) => c !== 'FIXED_RATIO'));
      if (chance(0.9)) node.layoutOptions['elk.portConstraints'] = constraints;
      const withLabels = chance(0.5);
      const sides = chance(0.3) ? ['NORTH', 'SOUTH'] : PORT_SIDES;
      node.ports = ports(node, 1 + int(border ? 5 : 4), withLabels, sides);
      if (withLabels && chance(0.7)) {
        node.layoutOptions['elk.portLabels.placement'] = pick(['INSIDE', 'OUTSIDE', '[]', 'NEXT_TO_PORT_IF_POSSIBLE', '[INSIDE, NEXT_TO_PORT_IF_POSSIBLE]']);
      }
      for (const p of node.ports) ends.push({ id: p.id, node: node.id, regions: border ? [region, inside] : [region], port: true, container: true });
    }
    const childCount = 1 + int(3);
    for (let i = 0; i < childCount; i++) node.children.push(leaf(inside));
    if (depth < 2 && chance(0.35)) node.children.push(container(inside, include, depth + 1));
    ends.push({ id: node.id, node: node.id, regions: [region], container: true });
    return node;
  };

  const options = { 'elk.algorithm': 'layered' };
  if (rootInclude) options['elk.hierarchyHandling'] = 'INCLUDE_CHILDREN';
  else if (chance(0.3)) options['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN';
  const rootNode = { id: 'root', layoutOptions: options, children: [] };
  containerOptions(rootNode, true);
  if (chance(0.15)) options['elk.layered.unnecessaryBendpoints'] = true;
  if (chance(0.1)) options['elk.layered.feedbackEdges'] = true;
  if (chance(0.15)) options['elk.edgeRouting'] = 'POLYLINE';
  if (chance(0.2)) options['elk.layered.cycleBreaking.strategy'] = pick(['GREEDY', 'DEPTH_FIRST']);

  const leafCount = kind === 4 ? 3 + int(4) : 1 + int(3);
  for (let i = 0; i < leafCount; i++) rootNode.children.push(leaf('root'));
  if (kind !== 4) {
    const containers = 1 + int(kind === 0 || kind === 5 ? 3 : 2);
    for (let i = 0; i < containers; i++) rootNode.children.push(container('root', rootInclude, 1));
  }

  const label = (text) => {
    const l = { text, width: size(6, 70), height: size(10, 24) };
    const r = rnd();
    if (r < 0.25) l.layoutOptions = { 'elk.edgeLabels.placement': 'HEAD' };
    else if (r < 0.5) l.layoutOptions = { 'elk.edgeLabels.placement': 'TAIL' };
    else if (r < 0.7) l.layoutOptions = { 'elk.edgeLabels.placement': 'CENTER' };
    return l;
  };

  const edges = [];
  const addEdge = (from, to) => {
    const e = { id: `e${edges.length}`, sources: [from.id], targets: [to.id] };
    const r = rnd();
    const count = r < 0.6 ? 0 : r < 0.9 ? 1 : 2;
    const labels = [];
    for (let k = 0; k < count; k++) labels.push(label(`${e.id}.${k}`));
    if (labels.length > 0) e.labels = labels;
    const lo = {};
    if (chance(0.15)) lo['elk.edge.thickness'] = pick([0, 2, 3.5]);
    if (from.node === to.node && from.container && chance(0.5)) lo['elk.insideSelfLoops.yo'] = true;
    if (Object.keys(lo).length > 0) e.layoutOptions = lo;
    edges.push(e);
  };
  // partners has the ends that an end can connect to.
  const partners = (a) => ends.filter((b) => b !== a && b.regions.some((r) => a.regions.includes(r)));
  const containerPorts = ends.filter((e) => e.port && e.container);
  const edgeCount = 3 + int(ends.length + 2);
  for (let k = 0; k < edgeCount; k++) {
    // An edge of a port of a node with children, or of any end.
    const a = containerPorts.length > 0 && chance(0.45) ? pick(containerPorts) : pick(ends);
    const others = partners(a);
    if (others.length === 0) continue;
    const b = pick(others);
    const r = rnd();
    if (r < 0.04) {
      // A self loop.
      addEdge(a, a);
      continue;
    }
    const forward = chance(0.5);
    addEdge(forward ? a : b, forward ? b : a);
    if (r < 0.3) {
      // A second edge of the same end: a hyperedge, or a merged hierarchy
      // edge.
      const c = pick(others);
      addEdge(forward ? a : c, forward ? c : a);
    }
  }
  rootNode.edges = edges;

  if (kind === 5 && chance(0.2)) {
    // Inside self loops of a compound node.
    const c = rootNode.children.find((n) => n.children && n.ports);
    if (c) c.layoutOptions['elk.insideSelfLoops.activate'] = true;
  }
  return rootNode;
}

// specials has graphs that are made by hand for cases that the random
// graphs reach seldom: a port index of 0 with a fixed port order on each
// side (elkjs gives the dummy the position -0 on the south and west sides),
// and a minimum size that is larger than the content, with each content
// alignment and with ports on each side.
function specials() {
  const out = [];
  const port = (id, side, index, extra = {}) => {
    const lo = { 'elk.port.side': side, ...extra };
    if (index !== undefined) lo['elk.port.index'] = index;
    const xy = { WEST: [-6, 10], EAST: [120, 14], NORTH: [20, -6], SOUTH: [30, 80] }[side];
    return { id, width: 6, height: 6, x: xy[0] + (index ?? 0) * 9, y: xy[1] + (index ?? 0) * 7, layoutOptions: lo };
  };
  const box = (id) => ({ id, width: 30.5, height: 24.25 });

  for (const handling of ['INCLUDE_CHILDREN', 'SEPARATE_CHILDREN']) {
    for (const constraints of ['FIXED_ORDER', 'FIXED_POS', 'FIXED_SIDE', 'FREE']) {
      for (const direction of ['RIGHT', 'DOWN']) {
        const c1 = {
          id: 'c1',
          width: 120,
          height: 80,
          layoutOptions: { 'elk.portConstraints': constraints, 'elk.direction': direction },
          ports: [
            port('pw0', 'WEST', 0),
            port('pw1', 'WEST', 1),
            port('pe0', 'EAST', 0),
            port('pe2', 'EAST', 2),
            port('pn0', 'NORTH', 0),
            port('pn1', 'NORTH', 1),
            port('ps0', 'SOUTH', 0),
            port('ps3', 'SOUTH', 3),
          ],
          children: [box('x'), box('y'), box('z')],
        };
        const edges = [
          ['pw0', 'x'],
          ['pw1', 'y'],
          ['x', 'pe0'],
          ['y', 'pe2'],
          ['pn0', 'y'],
          ['pn1', 'z'],
          ['z', 'ps0'],
          ['x', 'ps3'],
          ['x', 'z'],
        ];
        if (handling === 'INCLUDE_CHILDREN') {
          edges.push(['a', 'pw0'], ['a', 'pw1'], ['pe0', 'b'], ['a', 'pn0'], ['ps0', 'b'], ['ps3', 'b'], ['a', 'z'], ['y', 'b']);
        } else {
          c1.layoutOptions['elk.hierarchyHandling'] = 'SEPARATE_CHILDREN';
          edges.push(['a', 'pw0'], ['pe0', 'b'], ['a', 'b']);
        }
        out.push({
          name: `special-index-${handling}-${constraints}-${direction}`,
          input: {
            id: 'root',
            layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN' },
            children: [box('a'), c1, box('b')],
            edges: edges.map(([from, to], i) => ({ id: `e${i}`, sources: [from], targets: [to] })),
          },
        });
      }
    }
  }

  const alignments = ['[H_RIGHT, V_BOTTOM]', '[H_CENTER, V_CENTER]', '[H_LEFT, V_TOP]', 'H_RIGHT', 'V_BOTTOM', 'V_CENTER'];
  alignments.forEach((alignment, i) => {
    for (const withPorts of [false, true]) {
      const c1 = {
        id: 'c1',
        layoutOptions: {
          'elk.nodeSize.constraints': i % 2 === 0 ? 'MINIMUM_SIZE' : '[MINIMUM_SIZE, PORT_LABELS]',
          'elk.nodeSize.minimum': i === 4 ? '(0, 380.5)' : i === 5 ? '(10, 300)' : '(420.25, 310.5)',
          'elk.contentAlignment': alignment,
          'elk.padding': '[top=14,left=9,bottom=11.5,right=7]',
        },
        children: [box('x'), box('y')],
      };
      const edges = [['x', 'y'], ['a', 'x'], ['y', 'b']];
      if (withPorts) {
        c1.layoutOptions['elk.portConstraints'] = i % 3 === 0 ? 'FIXED_SIDE' : 'FIXED_ORDER';
        c1.width = 120;
        c1.height = 80;
        c1.ports = [port('pw', 'WEST'), port('pe', 'EAST'), port('pn', 'NORTH'), port('ps', 'SOUTH')];
        c1.ports[0].labels = [{ text: 'in', width: 14.5, height: 11 }];
        c1.ports[3].labels = [{ text: 'out', width: 18.25, height: 11 }];
        edges.push(['pw', 'x'], ['y', 'pe'], ['pn', 'x'], ['y', 'ps'], ['a', 'pw'], ['pe', 'b'], ['ps', 'b']);
      }
      out.push({
        name: `special-minimum-${i}-${withPorts ? 'ports' : 'plain'}`,
        input: {
          id: 'root',
          layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN', 'elk.direction': i % 2 === 0 ? 'RIGHT' : 'DOWN' },
          children: [box('a'), c1, box('b')],
          edges: edges.map(([from, to], k) => ({ id: `e${k}`, sources: [from], targets: [to] })),
        },
      });
    }
  });

  // Inside self loops: an edge between two ports of a compound node that
  // runs inside the node.
  for (const direction of ['RIGHT', 'DOWN']) {
    const c1 = {
      id: 'c1',
      layoutOptions: { 'elk.insideSelfLoops.activate': true, 'elk.direction': direction },
      ports: [port('pw', 'WEST'), port('pe', 'EAST'), port('pn', 'NORTH')],
      children: [box('x'), box('y')],
    };
    const edges = [
      { id: 'e0', sources: ['pw'], targets: ['pe'], layoutOptions: { 'elk.insideSelfLoops.yo': true } },
      { id: 'e1', sources: ['pn'], targets: ['pe'], layoutOptions: { 'elk.insideSelfLoops.yo': true } },
      { id: 'e2', sources: ['pw'], targets: ['x'] },
      { id: 'e3', sources: ['x'], targets: ['y'] },
      { id: 'e4', sources: ['y'], targets: ['pe'] },
      { id: 'e5', sources: ['a'], targets: ['pw'] },
      { id: 'e6', sources: ['pe'], targets: ['b'] },
      { id: 'e7', sources: ['pe'], targets: ['pw'] },
    ];
    out.push({
      name: `special-insideselfloops-${direction}`,
      input: {
        id: 'root',
        layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN' },
        children: [box('a'), c1, box('b')],
        edges,
      },
    });
  }

  // Port labels of a compound node, for each placement and with labels that
  // are partly inside the node.
  for (const placement of ['INSIDE', 'OUTSIDE', '[]', 'NEXT_TO_PORT_IF_POSSIBLE']) {
    for (const constraints of ['FREE', 'FIXED_SIDE', 'FIXED_POS']) {
      const labelled = (p, x, y) => ({ ...p, labels: [{ text: `${p.id} label`, width: 31.5, height: 12.25, x, y }] });
      const c1 = {
        id: 'c1',
        width: 120,
        height: 80,
        layoutOptions: { 'elk.portConstraints': constraints, 'elk.portLabels.placement': placement, 'elk.nodeSize.constraints': '[PORTS, PORT_LABELS]' },
        ports: [
          labelled(port('pw', 'WEST'), -10, -4),
          labelled(port('pe', 'EAST'), -20.5, 2),
          labelled(port('pn', 'NORTH'), -3, -7.5),
          labelled(port('ps', 'SOUTH'), 2, -9.25),
        ],
        children: [box('x'), box('y')],
      };
      const pairs = [['pw', 'x'], ['x', 'y'], ['y', 'pe'], ['pn', 'x'], ['y', 'ps'], ['a', 'pw'], ['pe', 'b'], ['a', 'pn'], ['ps', 'b']];
      out.push({
        name: `special-portlabels-${placement.replace(/\W/g, '') || 'none'}-${constraints}`,
        input: {
          id: 'root',
          layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN' },
          children: [box('a'), c1, box('b')],
          edges: pairs.map(([from, to], k) => ({ id: `e${k}`, sources: [from], targets: [to] })),
        },
      });
    }
  }

  // Edges that cross two hierarchy borders, with labels at each place, with
  // and without the merge of hierarchy edges.
  for (const merge of [true, false]) {
    for (const unnecessary of [false, true]) {
      const inner = (id) => ({ id, children: [box(`${id}x`), box(`${id}y`)] });
      const outer = (id) => ({ id, layoutOptions: { 'elk.layered.mergeHierarchyEdges': merge }, children: [box(`${id}u`), inner(`${id}i`)] });
      const label = (text, placement) => ({ text, width: 22.5, height: 13, layoutOptions: placement ? { 'elk.edgeLabels.placement': placement } : undefined });
      const edge = (id, from, to, labels, thickness) => {
        const e = { id, sources: [from], targets: [to] };
        if (labels) e.labels = labels;
        if (thickness !== undefined) e.layoutOptions = { 'elk.edge.thickness': thickness };
        return e;
      };
      const edges = [
        edge('e0', 'Aix', 'Bix', [label('head', 'HEAD'), label('tail', 'TAIL'), label('center', 'CENTER')]),
        edge('e1', 'Aix', 'Biy', [label('default')], 3),
        edge('e2', 'Aix', 'b', undefined, 2.5),
        edge('e3', 'a', 'Aiy', [label('down', 'CENTER')]),
        edge('e4', 'Biy', 'Aiy', [label('back', 'HEAD')]),
        edge('e5', 'Au', 'Aix'),
        edge('e6', 'Aiy', 'Au', [label('up', 'CENTER')]),
        edge('e7', 'Aix', 'A', [label('to the parent', 'TAIL')]),
        edge('e8', 'B', 'Bix'),
        edge('e9', 'a', 'b'),
        edge('e10', 'Aix', 'Aiy'),
      ];
      const options = { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN', 'elk.layered.mergeHierarchyEdges': merge };
      if (unnecessary) options['elk.layered.unnecessaryBendpoints'] = true;
      out.push({
        name: `special-deep-${merge ? 'merge' : 'nomerge'}-${unnecessary ? 'unnecessary' : 'plain'}`,
        input: { id: 'root', layoutOptions: options, children: [box('a'), outer('A'), outer('B'), box('b')], edges },
      });
    }
  }

  // A compound layout gives the dummy node of a port no node size. For the
  // port constraint FIXED_RATIO, CompoundGraphPreprocessor then fails with a
  // NullPointerException.
  for (const direction of ['RIGHT', 'DOWN']) {
    const c1 = {
      id: 'c1',
      width: 120,
      height: 80,
      layoutOptions: { 'elk.portConstraints': 'FIXED_RATIO', 'elk.direction': direction },
      ports: [port('pw', 'WEST'), port('pe', 'EAST')],
      children: [box('x'), box('y')],
    };
    const pairs = [['pw', 'x'], ['x', 'y'], ['y', 'pe'], ['a', 'pw'], ['pe', 'b']];
    out.push({
      name: `special-fails-ratio-${direction}`,
      fails: true,
      input: {
        id: 'root',
        layoutOptions: { 'elk.algorithm': 'layered', 'elk.hierarchyHandling': 'INCLUDE_CHILDREN' },
        children: [box('a'), c1, box('b')],
        edges: pairs.map(([from, to], k) => ({ id: `e${k}`, sources: [from], targets: [to] })),
      },
    });
  }
  return out;
}

// dump runs the phase oracle for one graph file.
function dump(file) {
  return new Promise((done) => {
    execFile(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, maxBuffer: 1 << 30, timeout: 60000 }, (err, stdout) => {
      if (err) {
        done({ err: String(err.message).slice(0, 200) });
        return;
      }
      try {
        done(JSON.parse(stdout));
      } catch (e) {
        done({ err: `no JSON: ${String(e.message).slice(0, 100)}` });
      }
    });
  });
}

// trim keeps the steps of the processors of one package and the states that
// they name. It returns null for a layout that has no such step, or that has
// a state that is too large.
//
// For a layout that fails in elkjs, the last step has no state after it: the
// processor of that step threw the error. trim keeps such a layout when that
// processor is one of the package. The step then has no member after, and
// the case has the member error in place of the member out. The port of the
// processor must stop for the state before that step.
function trim(pkg, name, input, r, maxStateBytes) {
  const failed = r.err !== undefined || r.out === undefined;
  let failingStep = false;
  const runs = [];
  for (const run of r.runs) {
    const states = [];
    const indexOf = new Map();
    const keep = (i) => {
      if (!indexOf.has(i)) {
        indexOf.set(i, states.length);
        states.push(run.states[i]);
      }
      return indexOf.get(i);
    };
    const steps = [];
    run.steps.forEach((s, i) => {
      if (!pkg.processors.has(s.proc) || s.before === undefined) return;
      if (s.after === undefined) {
        if (failed && run === r.runs[r.runs.length - 1] && i === run.steps.length - 1) {
          steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: keep(s.before), draws: s.draws });
          failingStep = true;
        }
        return;
      }
      steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: keep(s.before), after: keep(s.after), draws: s.draws, drawsAfter: s.drawsAfter });
    });
    if (steps.length > 0) runs.push({ elk: run.elk, steps, states });
  }
  if (runs.length === 0 || (failed && !failingStep)) return null;
  for (const run of runs) for (const s of run.states) if (JSON.stringify(s).length > maxStateBytes) return null;
  if (failed) return { name, source: pkg.source, in: input, error: String(r.err), runs };
  // The result of elkjs, without the hash codes that it leaves in the graph.
  const out = JSON.parse(r.out.replace(/,"\$H":\d+/g, ''));
  return { name, source: pkg.source, in: input, out, runs };
}

// changed counts the steps of a case that change the state.
const changed = (c) => c.runs.reduce((n, run) => n + run.steps.filter((s) => s.after !== undefined && s.before !== s.after).length, 0);

async function main() {
  const dir = mkdtempSync(join(tmpdir(), 'elk-hierarchy-'));
  // The graphs: the special ones, then the random ones.
  const inputs = specials();
  for (let i = 0; i < GRAPHS; i++) inputs.push({ name: `hierarchy-${i}`, input: graph(i), kind: i % 6 });
  const results = new Array(inputs.length).fill(null);
  let next = 0;
  const worker = async () => {
    while (next < inputs.length) {
      const i = next++;
      const file = join(dir, `g${i}.json`);
      writeFileSync(file, JSON.stringify(inputs[i].input));
      results[i] = { ...inputs[i], r: await dump(file) };
    }
  };
  await Promise.all(Array.from({ length: 8 }, worker));
  rmSync(dir, { recursive: true, force: true });

  const failed = results.filter((x) => x.r.err !== undefined || x.r.out === undefined || !x.r.runs);
  for (const x of results) {
    if (x.fails && !failed.includes(x)) throw new Error(`the special graph ${x.name} must fail in elkjs, and it does not`);
  }
  console.log(`${inputs.length} graphs, ${failed.length} layouts fail in elkjs`);
  const reasons = new Map();
  for (const x of failed) {
    const key = String(x.r.err ?? 'no output').split('\n')[0].slice(0, 120);
    reasons.set(key, (reasons.get(key) ?? 0) + 1);
    if (x.kind === undefined && !x.fails) console.log(`       the special graph ${x.name} fails: ${key}`);
  }
  for (const [reason, n] of reasons) console.log(String(n).padStart(6), 'fail:', reason);

  for (const pkg of PACKAGES) {
    // The special graphs are always kept.
    const kept = [];
    const cases = [];
    for (const x of results) {
      if (!x.r.runs) continue;
      if (x.kind === undefined && pkg.skipSpecials?.test(x.name)) continue;
      // A special graph can have larger states.
      const c = trim(pkg, x.name, x.input, x.r, x.kind === undefined ? 4 * pkg.maxStateBytes : pkg.maxStateBytes);
      if (!c) continue;
      // A layout that fails in a processor of the package is always kept.
      if (x.kind === undefined || c.error !== undefined) kept.push(c);
      else if (JSON.stringify(c.runs).length <= pkg.maxCaseBytes) cases.push({ c, kind: x.kind });
    }
    const special = kept.length;
    // Of the random graphs, keep the cases in which the processors do the
    // most, by kind of graph in turn, so that each kind stays in the goldens.
    const byKind = new Map();
    for (const { c, kind } of cases) {
      if (!byKind.has(kind)) byKind.set(kind, []);
      byKind.get(kind).push(c);
    }
    for (const list of byKind.values()) list.sort((a, b) => changed(b) - changed(a) || JSON.stringify(a.runs).length - JSON.stringify(b.runs).length);
    const random = [];
    for (let round = 0; random.length < pkg.maxCases; round++) {
      let any = false;
      for (const list of byKind.values()) {
        if (round < list.length && random.length < pkg.maxCases) {
          random.push(list[round]);
          any = true;
        }
      }
      if (!any) break;
    }
    random.sort((a, b) => Number(a.name.split('-')[1]) - Number(b.name.split('-')[1]));
    kept.push(...random);

    const count = new Map();
    const changes = new Map();
    const failing = new Map();
    for (const c of kept) {
      for (const run of c.runs) {
        for (const s of run.steps) {
          count.set(s.proc, (count.get(s.proc) ?? 0) + 1);
          if (s.after === undefined) failing.set(s.proc, (failing.get(s.proc) ?? 0) + 1);
          else if (s.before !== s.after) changes.set(s.proc, (changes.get(s.proc) ?? 0) + 1);
        }
      }
    }
    mkdirSync(dirname(pkg.file), { recursive: true });
    const gz = gzipSync(Buffer.from(JSON.stringify(kept)), { level: 9 });
    writeFileSync(pkg.file, gz);
    console.log(`${pkg.source}: ${special} special or failing cases and ${random.length} of ${cases.length} random cases, ${(gz.length / 1e6).toFixed(2)} MB gzip`);
    for (const [proc, n] of [...count].sort()) console.log(String(n).padStart(6), proc, `(${changes.get(proc) ?? 0} change the state, ${failing.get(proc) ?? 0} fail in elkjs)`);
  }
}

await main();
