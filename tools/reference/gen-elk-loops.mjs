// Extra phase goldens for internal/elk/native/loops: the states of the
// LGraph before and after the four self loop processors of ELK layered, for
// graphs with self loops that the shared phase goldens do not have (loops
// with ports on two, three and four sides, hyper loops, loops on nodes with
// each kind of port constraints, each self loop distribution strategy, each
// self loop ordering strategy, several labels on one loop). The first
// graphs are scenarios: each one aims at a part of the self loop code that
// random graphs seldom reach.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-loops.mjs             write the goldens
//   node gen-elk-loops.mjs --graph N   print the ELK graph of the case loops-N
//
// The script makes seeded random ELK graphs and runs
// `node gen-elk-phases.mjs --dump` for each of them, so the states come from
// the same instrumented elkjs 0.9.3 as the shared goldens. It keeps the steps
// of the self loop processors and the states that they name, in the format
// of internal/elk/native/nativetest (one Case for each graph), and writes
// internal/elk/native/loops/loopstest/testdata/loop-steps.json.gz.
import { execFile } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const outFile = resolve(root, 'internal/elk/native/loops/loopstest/testdata/loop-steps.json.gz');

const PROCESSORS = new Set(['SelfLoopPreProcessor', 'SelfLoopPortRestorer', 'SelfLoopRouter', 'SelfLoopPostProcessor']);

const DIRECTIONS = ['RIGHT', 'DOWN', 'LEFT', 'UP'];
const PORT_SIDES = ['NORTH', 'EAST', 'SOUTH', 'WEST'];
const CONSTRAINTS = ['FREE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS', 'FIXED_RATIO', 'UNDEFINED'];
const DISTRIBUTIONS = ['EQUALLY', 'NORTH', 'NORTH_SOUTH'];
const ORDERINGS = ['STACKED', 'REVERSE_STACKED', 'SEQUENCED'];

// The number of scenario graphs and of random graphs, and the largest state
// that a golden keeps.
const SCENARIOS = 220;
const RANDOM_GRAPHS = 260;
const GRAPHS = SCENARIOS + RANDOM_GRAPHS;
const MAX_STATE_BYTES = 45000;

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

// placePorts sorts the ports of a node by their side, gives each port a
// position on its side in clockwise order, and sets the option port.side.
// Each port has its side in the member side when this is called.
function placePorts(node) {
  const order = (p) => PORT_SIDES.indexOf(p.side);
  const ports = node.ports.map((p, i) => [p, i]).sort((a, b) => order(a[0]) - order(b[0]) || a[1] - b[1]).map((x) => x[0]);
  const counts = { NORTH: 0, EAST: 0, SOUTH: 0, WEST: 0 };
  for (const p of ports) counts[p.side]++;
  const seen = { NORTH: 0, EAST: 0, SOUTH: 0, WEST: 0 };
  for (const p of ports) {
    const side = p.side;
    delete p.side;
    const along = (seen[side] + 1) / (counts[side] + 1);
    seen[side]++;
    if (side === 'NORTH') {
      p.x = Math.round(along * node.width * 16) / 16;
      p.y = -p.height;
    } else if (side === 'EAST') {
      p.x = node.width;
      p.y = Math.round(along * node.height * 16) / 16;
    } else if (side === 'SOUTH') {
      p.x = Math.round((1 - along) * node.width * 16) / 16;
      p.y = node.height;
    } else {
      p.x = -p.width;
      p.y = Math.round((1 - along) * node.height * 16) / 16;
    }
    p.layoutOptions = { 'elk.port.side': side };
  }
  node.ports = ports;
}

// scenario makes a graph for one part of the self loop code:
//
//   0  a node with free ports and the distribution EQUALLY, with five to ten
//      loops: loops on one port, on two ports and on three ports, and ports
//      that have regular edges too
//   1  loops with labels on the north side and on the south side of a node
//      with the ordering SEQUENCED
//   2  a node with fixed port sides whose ports have self loops only: a loop
//      over four sides with loops on one, two and three sides
//   3  several loops whose labels have no width or the same width, and
//      loops side by side on a node with fixed port positions whose labels
//      touch in one point
//   4  a ring of loops around a node with a fixed port order, where each
//      loop wants to be below the next one: the routing slot assigner must
//      break the cycle, with random numbers
function scenario(index) {
  const rnd = mulberry32(0x7a11e0 + index * 104729);
  const int = (n) => Math.floor(rnd() * n);
  const pick = (list) => list[int(list.length)];
  const chance = (p) => rnd() < p;
  const size = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;
  const kind = index % 5;
  const variant = Math.floor(index / 5);

  const options = { 'elk.algorithm': 'layered' };
  if (variant % 5 !== 4) options['elk.direction'] = DIRECTIONS[variant % DIRECTIONS.length];
  if (chance(0.25)) options['elk.spacing.edgeLabel'] = pick([0, 1, 3.5, 6]);
  if (chance(0.25)) options['elk.spacing.labelLabel'] = pick([0, 2, 4.25]);
  if (chance(0.25)) options['elk.spacing.edgeEdge'] = pick([4, 7.5, 15]);
  if (chance(0.25)) options['elk.spacing.nodeSelfLoop'] = pick([0, 5, 12.5, 20]);

  const n0 = { id: 'n0', width: size(60, 180), height: size(40, 120), ports: [] };
  const n1 = { id: 'n1', width: size(30, 80), height: size(30, 60) };
  const lo = {};
  const edges = [];
  let regular = 0;
  const newPort = (side) => {
    const p = { id: `n0_p${n0.ports.length}`, width: pick([0, 0, 6]), height: pick([0, 0, 6]), side };
    n0.ports.push(p);
    return p.id;
  };
  const label = (text, widths, heights) => ({
    text,
    width: widths ? pick(widths) : chance(0.4) ? pick([24, 40.5, 61.25]) : size(6, 90),
    height: heights ? pick(heights) : chance(0.4) ? pick([12, 18.5]) : size(10, 26),
  });
  // addEdge adds an edge with a number of labels from min to max.
  const addEdge = (source, target, min, max, widths, heights) => {
    const e = { id: `e${edges.length}`, sources: [source], targets: [target] };
    const count = min + int(max - min + 1);
    if (count > 0) e.labels = Array.from({ length: count }, (_, k) => label(`${e.id}.${k}`, widths, heights));
    edges.push(e);
  };
  const either = (a, b, min, max) => (chance(0.5) ? addEdge(a, b, min, max) : addEdge(b, a, min, max));
  const regularEdge = (port) => {
    regular++;
    if (chance(0.5)) addEdge(port, 'n1', 0, 0);
    else addEdge('n1', port, 0, 0);
  };
  const side = () => pick(PORT_SIDES);
  const next = (s) => PORT_SIDES[(PORT_SIDES.indexOf(s) + 1) % 4];
  const opposed = (s) => PORT_SIDES[(PORT_SIDES.indexOf(s) + 2) % 4];

  if (kind === 0) {
    if (variant % 2 === 0) lo['elk.portConstraints'] = 'FREE';
    lo['elk.layered.edgeRouting.selfLoopDistribution'] = 'EQUALLY';
    if (chance(0.5)) lo['elk.layered.edgeRouting.selfLoopOrdering'] = pick(ORDERINGS);
    const loops = 5 + int(6);
    for (let k = 0; k < loops; k++) {
      const r = rnd();
      let ports;
      if (r < 0.12) {
        // The importer makes the ports of this loop.
        addEdge('n0', 'n0', 0, 2);
        continue;
      } else if (r < 0.3) {
        ports = [newPort(side())];
        addEdge(ports[0], ports[0], 0, 1);
      } else if (r < 0.6) {
        ports = [newPort(side()), newPort(side())];
        either(ports[0], ports[1], 0, 2);
      } else {
        ports = [newPort(side()), newPort(side()), newPort(side())];
        either(ports[0], ports[1], 0, 1);
        either(ports[0], ports[2], 0, 1);
      }
      // A regular edge on a port of the loop: the port is not hidden then,
      // and it has a net flow.
      if (chance(0.4)) regularEdge(pick(ports));
    }
  } else if (kind === 1) {
    lo['elk.layered.edgeRouting.selfLoopOrdering'] = 'SEQUENCED';
    if (variant % 3 === 2) {
      // Free ports: the distribution puts the loops on the north side, or on
      // the north side and the south side.
      lo['elk.layered.edgeRouting.selfLoopDistribution'] = pick(['NORTH', 'NORTH_SOUTH', 'NORTH_SOUTH']);
      const loops = 3 + int(5);
      for (let k = 0; k < loops; k++) addEdge('n0', 'n0', chance(0.85) ? 1 : 0, 2);
    } else {
      lo['elk.portConstraints'] = ['FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS', 'FIXED_SIDE'][variant % 4];
      const perSide = { SOUTH: 2 + int(3), NORTH: int(4), EAST: int(2), WEST: int(2) };
      for (const s of PORT_SIDES) {
        for (let k = 0; k < perSide[s]; k++) {
          const a = newPort(s);
          const b = newPort(s);
          either(a, b, chance(0.85) ? 1 : 0, 2);
          if (chance(0.2)) either(a, newPort(s), 0, 1);
        }
      }
      if (chance(0.4)) regularEdge(newPort(side()));
    }
  } else if (kind === 2) {
    lo['elk.portConstraints'] = ['FIXED_SIDE', 'FIXED_SIDE', 'FIXED_SIDE', 'FIXED_ORDER', 'FIXED_POS'][variant % 5];
    if (variant % 4 !== 3) lo['elk.layered.edgeRouting.selfLoopOrdering'] = ORDERINGS[variant % 3];
    if (variant % 3 !== 2) {
      // A loop over four sides: a star around one port.
      const hubSide = side();
      const hub = newPort(hubSide);
      for (let s = next(hubSide); s !== hubSide; s = next(s)) either(hub, newPort(s), 0, chance(0.3) ? 1 : 0);
      if (chance(0.3)) either(hub, newPort(side()), 0, 0);
    } else {
      // A loop over three sides, and loops over the corners between them:
      // the loops share the start area and the end area of a side.
      const s = side();
      const hub = newPort(s);
      either(hub, newPort(next(s)), 0, 1);
      either(hub, newPort(opposed(s)), 0, 0);
      either(newPort(next(s)), newPort(opposed(s)), 0, 1);
      if (chance(0.6)) either(newPort(s), newPort(next(s)), 0, 1);
      if (chance(0.4)) {
        const hub2 = newPort(s);
        either(hub2, newPort(next(s)), 0, 0);
        either(hub2, newPort(opposed(s)), 0, 0);
      }
    }
    const more = 1 + int(4);
    for (let k = 0; k < more; k++) {
      const r = rnd();
      const s = side();
      if (r < 0.3) {
        either(newPort(s), newPort(s), 0, 2);
      } else if (r < 0.45) {
        const hub = newPort(s);
        either(hub, newPort(s), 0, 1);
        either(hub, newPort(s), 0, 1);
      } else if (r < 0.65) {
        either(newPort(s), newPort(next(s)), 0, 2);
      } else if (r < 0.8) {
        either(newPort(s), newPort(opposed(s)), 0, 2);
      } else {
        const hub = newPort(s);
        either(hub, newPort(next(s)), 0, 1);
        either(hub, newPort(opposed(s)), 0, 1);
      }
    }
    const others = int(3);
    for (let k = 0; k < others; k++) regularEdge(newPort(side()));
  } else if (kind === 3 && variant % 2 === 0) {
    if (variant % 3 !== 0) lo['elk.layered.edgeRouting.selfLoopDistribution'] = pick(['NORTH', 'NORTH_SOUTH']);
    if (variant % 4 === 1) lo['elk.layered.edgeRouting.selfLoopOrdering'] = pick(ORDERINGS);
    const loops = 2 + int(3);
    for (let k = 0; k < loops; k++) addEdge('n0', 'n0', 1, 1, [0, 0, 24, 24, 40.5], [0, 12, 18.5]);
  } else if (kind === 3) {
    // Three loops side by side on the north side. The label of the left
    // loop ends at the last port of that loop, which is in the middle of
    // the node. The label of the middle loop has no width and is centered,
    // so the two labels touch in one point. The ports are in a random order
    // in the input, so either loop can be the first one of the two.
    // The layout direction keeps the north side at the top.
    options['elk.direction'] = variant % 4 === 1 ? 'RIGHT' : 'LEFT';
    lo['elk.portConstraints'] = 'FIXED_POS';
    lo['elk.layered.edgeRouting.selfLoopOrdering'] = 'SEQUENCED';
    const w = 16 * (6 + int(8));
    n0.width = w;
    const at = [w / 8, w / 2, (5 * w) / 8, (6 * w) / 8, (7 * w) / 8, w - 4];
    const ids = at.map((x, i) => `n0_q${i}`);
    const order = at.map((x, i) => i);
    for (let i = order.length - 1; i > 0; i--) {
      const j = int(i + 1);
      [order[i], order[j]] = [order[j], order[i]];
    }
    for (const i of order) n0.ports.push({ id: ids[i], width: 0, height: 0, x: at[i], y: 0, layoutOptions: { 'elk.port.side': 'NORTH' } });
    const loop = (a, b, widths) => (chance(0.5) ? addEdge(ids[a], ids[b], 1, 1, widths, [0, 12, 18.5]) : addEdge(ids[b], ids[a], 1, 1, widths, [0, 12, 18.5]));
    const loops = [() => loop(0, 1, [0, 24, 40.5]), () => loop(2, 3, [0]), () => loop(4, 5, [0, 24])];
    for (const i of chance(0.5) ? [0, 1, 2] : chance(0.5) ? [1, 0, 2] : [2, 1, 0]) loops[i]();
    n0.placed = true;
  } else {
    // A ring of loops. Each loop has two or three ports at the start of its
    // first side and one port at the end of its last side, after the first
    // ports of the next loop. So a loop has one port inside the next loop,
    // and the next loop has two or three ports inside this loop.
    lo['elk.portConstraints'] = variant % 2 === 0 ? 'FIXED_ORDER' : 'FIXED_POS';
    const ring = variant % 3 === 2 ? [['NORTH', 'EAST'], ['EAST', 'SOUTH'], ['SOUTH', 'WEST', 'NORTH']] : [['NORTH', 'EAST'], ['EAST', 'SOUTH'], ['SOUTH', 'WEST'], ['WEST', 'NORTH']];
    // The ports of a side are placed in the order in which they are made:
    // each first port before the last port of the loop before.
    const firsts = ring.map((sides) => Array.from({ length: 2 + int(2) }, () => newPort(sides[0])));
    const between = ring.map((sides) => (sides.length === 3 ? newPort(sides[1]) : null));
    const lasts = ring.map((sides) => newPort(sides[sides.length - 1]));
    ring.forEach((sides, i) => {
      for (const first of firsts[i]) either(first, lasts[i], 0, chance(0.2) ? 1 : 0);
      if (between[i]) either(between[i], lasts[i], 0, 0);
    });
    if (chance(0.3)) regularEdge(newPort(side()));
  }

  if (n0.placed) delete n0.placed;
  else if (n0.ports.length === 0) delete n0.ports;
  else placePorts(n0);
  if (Object.keys(lo).length > 0) n0.layoutOptions = lo;
  const children = [n0];
  if (regular > 0 || chance(0.4)) {
    children.push(n1);
    if (regular === 0) addEdge('n0', 'n1', 0, 1);
  }
  return { id: 'root', layoutOptions: options, children, edges };
}

// graph makes the ELK graph with the given index: a scenario, or a random
// graph.
function graph(index) {
  return index < SCENARIOS ? scenario(index) : randomGraph(index - SCENARIOS);
}

// randomGraph makes a random ELK graph. The first graphs walk through each
// port constraint, each distribution strategy and each ordering strategy in
// each direction. The later ones choose everything at random.
function randomGraph(index) {
  const rnd = mulberry32(0x5e1f100b + index * 7919);
  const int = (n) => Math.floor(rnd() * n);
  const pick = (list) => list[int(list.length)];
  const chance = (p) => rnd() < p;
  // Sizes with fractions, as text measurement gives them.
  const size = (lo, hi) => Math.round((lo + rnd() * (hi - lo)) * 64) / 64;

  const options = { 'elk.algorithm': 'layered' };
  const direction = DIRECTIONS[index % DIRECTIONS.length];
  if (index % 9 !== 8) options['elk.direction'] = direction;
  if (chance(0.3)) options['elk.spacing.edgeLabel'] = pick([0, 1, 3.5, 6]);
  if (chance(0.3)) options['elk.spacing.labelLabel'] = pick([0, 2, 4.25]);
  if (chance(0.3)) options['elk.spacing.edgeEdge'] = pick([4, 7.5, 15]);
  if (chance(0.3)) options['elk.spacing.nodeSelfLoop'] = pick([0, 5, 12.5, 20]);
  if (chance(0.2)) options['elk.layered.mergeEdges'] = true;
  // The graph options that mermaid sets. ELK reads both strategies from the
  // node, so they change nothing here.
  if (chance(0.3)) options['elk.layered.edgeRouting.selfLoopDistribution'] = pick(DISTRIBUTIONS);
  if (chance(0.2)) options['elk.layered.edgeRouting.selfLoopOrdering'] = pick(ORDERINGS);

  const fixedConstraints = index < 108 ? CONSTRAINTS[Math.floor(index / 4) % CONSTRAINTS.length] : null;
  const fixedDistribution = index < 108 ? DISTRIBUTIONS[Math.floor(index / 24) % DISTRIBUTIONS.length] : null;
  const fixedOrdering = index < 108 ? ORDERINGS[Math.floor(index / 12) % ORDERINGS.length] : null;

  const nodeCount = 1 + int(4);
  const children = [];
  for (let i = 0; i < nodeCount; i++) {
    const node = { id: `n${i}`, width: size(30, 140), height: size(30, 100) };
    const lo = {};
    const withPorts = i === 0 ? index % 3 !== 2 : chance(0.5);
    if (withPorts) {
      const constraints = i === 0 && fixedConstraints ? fixedConstraints : pick(CONSTRAINTS);
      if (constraints !== 'UNDEFINED') lo['elk.portConstraints'] = constraints;
      node.ports = [];
      const portCount = 2 + int(7);
      // The ports of a side are in one run, in the order of the sides, so
      // that an order that is fixed is a possible order.
      const sides = [];
      for (let p = 0; p < portCount; p++) sides.push(int(PORT_SIDES.length));
      sides.sort((a, b) => a - b);
      const counts = [0, 0, 0, 0];
      for (const s of sides) counts[s]++;
      const seen = [0, 0, 0, 0];
      for (let p = 0; p < portCount; p++) {
        const s = sides[p];
        const side = PORT_SIDES[s];
        const pw = pick([0, 0, 6, 8]);
        const ph = pick([0, 0, 6, 8]);
        const port = { id: `n${i}_p${p}`, width: pw, height: ph };
        // The position along the side, clockwise.
        const along = (seen[s] + 1) / (counts[s] + 1);
        seen[s]++;
        if (side === 'NORTH') {
          port.x = Math.round(along * node.width * 16) / 16;
          port.y = -ph;
        } else if (side === 'EAST') {
          port.x = node.width;
          port.y = Math.round(along * node.height * 16) / 16;
        } else if (side === 'SOUTH') {
          port.x = Math.round((1 - along) * node.width * 16) / 16;
          port.y = node.height;
        } else {
          port.x = -pw;
          port.y = Math.round((1 - along) * node.height * 16) / 16;
        }
        port.layoutOptions = { 'elk.port.side': side };
        if (constraints === 'FIXED_ORDER' && chance(0.5)) port.layoutOptions['elk.port.index'] = p;
        if (chance(0.15)) port.labels = [{ text: `p${p}`, width: size(6, 30), height: size(8, 16) }];
        node.ports.push(port);
      }
    }
    if (i === 0 && fixedDistribution) {
      lo['elk.layered.edgeRouting.selfLoopDistribution'] = fixedDistribution;
    } else if (chance(0.6)) {
      lo['elk.layered.edgeRouting.selfLoopDistribution'] = pick(DISTRIBUTIONS);
    }
    if (i === 0 && fixedOrdering) {
      lo['elk.layered.edgeRouting.selfLoopOrdering'] = fixedOrdering;
    } else if (chance(0.6)) {
      lo['elk.layered.edgeRouting.selfLoopOrdering'] = pick(ORDERINGS);
    }
    if (Object.keys(lo).length > 0) node.layoutOptions = lo;
    children.push(node);
  }

  // Label sizes: some from a short list, so that two labels have the same
  // size, and some labels without a width or a height, as the label of an
  // edge without text has in mermaid.
  const label = (text) => {
    const l = { text, width: chance(0.4) ? pick([24, 40.5, 61.25]) : size(6, 90), height: chance(0.4) ? pick([12, 18.5]) : size(10, 26) };
    if (chance(0.12)) l.width = 0;
    if (chance(0.12)) l.height = 0;
    return l;
  };
  const edges = [];
  const addEdge = (source, target, maxLabels) => {
    const e = { id: `e${edges.length}`, sources: [source], targets: [target] };
    const labels = [];
    const r = rnd();
    const count = Math.min(maxLabels, r < 0.35 ? 0 : r < 0.75 ? 1 : r < 0.92 ? 2 : 3);
    for (let k = 0; k < count; k++) labels.push(label(`${e.id}.${k}`));
    if (labels.length > 0) e.labels = labels;
    edges.push(e);
  };

  // Self loops.
  for (let i = 0; i < nodeCount; i++) {
    const node = children[i];
    const loops = i === 0 ? 1 + int(5) : int(4);
    for (let k = 0; k < loops; k++) {
      if (!node.ports || chance(0.2)) {
        // A loop from the node to the node. The importer makes its ports.
        addEdge(node.id, node.id, 3);
        continue;
      }
      const kind = rnd();
      const from = pick(node.ports);
      if (kind < 0.45) {
        // A loop between two ports.
        addEdge(from.id, pick(node.ports).id, 3);
      } else if (kind < 0.6) {
        // A loop from a port to the node, or from the node to a port.
        if (chance(0.5)) addEdge(from.id, node.id, 2);
        else addEdge(node.id, from.id, 2);
      } else {
        // A hyper loop: a star around one port. A star keeps the object
        // graph of the self loop holder flat enough for the state dump.
        const rays = 2 + int(3);
        for (let r = 0; r < rays; r++) {
          const other = pick(node.ports);
          if (chance(0.5)) addEdge(from.id, other.id, 1);
          else addEdge(other.id, from.id, 1);
        }
      }
    }
  }

  // Regular edges.
  const end = (i) => {
    const node = children[i];
    return node.ports && chance(0.6) ? pick(node.ports).id : node.id;
  };
  for (let i = 0; i + 1 < nodeCount; i++) if (chance(0.8)) addEdge(end(i), end(i + 1), 1);
  const extra = int(nodeCount + 1);
  for (let k = 0; k < extra; k++) {
    const from = int(nodeCount);
    const to = int(nodeCount);
    if (from !== to) addEdge(end(from), end(to), 1);
  }

  const out = { id: 'root', layoutOptions: options, children, edges };

  // A compound node with a self loop, and an edge that leaves it: the graph
  // inside the node then has external ports.
  if (index % 7 === 6) {
    options['elk.hierarchyHandling'] = 'INCLUDE_CHILDREN';
    const inner = [
      { id: 'c0', width: size(20, 60), height: size(20, 40) },
      { id: 'c1', width: size(20, 60), height: size(20, 40) },
    ];
    const compound = { id: 'comp', children: inner, edges: [{ id: 'ce0', sources: ['c0'], targets: ['c1'] }] };
    if (chance(0.5)) compound.layoutOptions = { 'elk.layered.edgeRouting.selfLoopDistribution': pick(DISTRIBUTIONS) };
    children.push(compound);
    edges.push({ id: `e${edges.length}`, sources: ['comp'], targets: ['comp'], labels: chance(0.5) ? [label('comp')] : undefined });
    if (chance(0.7)) edges.push({ id: `e${edges.length}`, sources: ['c1'], targets: ['n0'] });
    if (chance(0.5)) edges.push({ id: `e${edges.length}`, sources: ['n0'], targets: ['comp'] });
  }
  return out;
}

// dump runs the phase oracle for one graph file.
function dump(file) {
  return new Promise((done, fail) => {
    execFile(process.execPath, ['gen-elk-phases.mjs', '--dump', file], { cwd: here, maxBuffer: 1 << 30 }, (err, stdout) => {
      if (err) fail(err);
      else done(JSON.parse(stdout));
    });
  });
}

// cut tells that a state has an object without its fields: the dump stops
// below a fixed depth, and the Go codec cannot read such a self loop holder.
function cut(v) {
  if (Array.isArray(v)) return v.some(cut);
  if (v && typeof v === 'object') {
    if (v.$o !== undefined && v.c !== undefined && v.f === undefined) return true;
    for (const k of Object.keys(v)) if (cut(v[k])) return true;
  }
  return false;
}

// trim keeps the steps of the self loop processors and the states that they
// name. It returns null for a layout that failed or that has no such step.
function trim(name, input, r) {
  if (r.err !== undefined || r.out === undefined) return null;
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
    for (const s of run.steps) {
      if (!PROCESSORS.has(s.proc) || s.before === undefined || s.after === undefined) continue;
      steps.push({ kind: s.kind, proc: s.proc, slot: s.slot, graph: s.graph, comp: s.comp, before: keep(s.before), after: keep(s.after), draws: s.draws, drawsAfter: s.drawsAfter });
    }
    if (steps.length > 0) runs.push({ elk: run.elk, steps, states });
  }
  if (runs.length === 0) return null;
  for (const run of runs) for (const s of run.states) if (JSON.stringify(s).length > MAX_STATE_BYTES || cut(s)) return null;
  return { name, source: 'loops', in: input, runs };
}

async function main() {
  const dir = mkdtempSync(join(tmpdir(), 'elk-loops-'));
  const cases = new Array(GRAPHS).fill(null);
  let next = 0;
  let failed = 0;
  const worker = async () => {
    while (next < GRAPHS) {
      const i = next++;
      const input = graph(i);
      const file = join(dir, `g${i}.json`);
      writeFileSync(file, JSON.stringify(input));
      const r = await dump(file);
      if (r.err !== undefined || r.out === undefined) failed++;
      cases[i] = trim(`loops-${i}`, input, r);
    }
  };
  await Promise.all(Array.from({ length: 6 }, worker));
  rmSync(dir, { recursive: true, force: true });

  const kept = cases.filter(Boolean);
  const count = new Map();
  for (const c of kept) for (const run of c.runs) for (const s of run.steps) count.set(s.proc, (count.get(s.proc) ?? 0) + 1);
  mkdirSync(dirname(outFile), { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify(kept)), { level: 9 });
  writeFileSync(outFile, gz);
  console.log(`${kept.length} cases of ${GRAPHS} graphs (${failed} layouts failed), ${(gz.length / 1e6).toFixed(2)} MB gzip`);
  for (const [proc, n] of [...count].sort()) console.log(String(n).padStart(6), proc);
}

if (process.argv.includes('--graph')) {
  console.log(JSON.stringify(graph(Number(process.argv[process.argv.indexOf('--graph') + 1]))));
} else {
  await main();
}
