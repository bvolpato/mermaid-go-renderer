// Emits golden cases for internal/d3/sankey from d3-sankey 0.12.3.
//
// Run it in tools/reference:
//
//   node gen-d3-sankey.mjs
//
// Each case is a random graph with the settings of the generator. The result
// has every number that the layout sets, as bits, the order of the links of
// each node, and the path of each link from sankeyLinkHorizontal.
import { prng, floats, writeGolden, pick, int } from './d3-golden-lib.mjs';
import { sankey, sankeyLeft, sankeyRight, sankeyCenter, sankeyJustify, sankeyLinkHorizontal } from 'd3-sankey';

const rnd = prng(0x5a11ce);

const aligns = { left: sankeyLeft, right: sankeyRight, center: sankeyCenter, justify: sankeyJustify };
const nodeSorts = {
  indexDesc: (a, b) => b.index - a.index,
  valueAsc: (a, b) => a.value - b.value,
};
const linkSorts = {
  valueDesc: (a, b) => b.value - a.value,
  indexDesc: (a, b) => b.index - a.index,
};

function linkValue(style) {
  const k = rnd();
  if (style === 'int') return int(rnd, 1, 100);
  if (style === 'ties') return pick(rnd, [1, 2, 5, 10]);
  if (style === 'odd') {
    if (k < 0.2) return 0;
    if (k < 0.3) return NaN;
    if (k < 0.4) return -rnd() * 10;
    if (k < 0.5) return rnd() * 1e-9;
    if (k < 0.6) return rnd() * 1e12;
    if (k < 0.65) return Infinity;
    return rnd() * 50;
  }
  if (k < 0.4) return int(rnd, 1, 500);
  if (k < 0.7) return Math.round(rnd() * 100000) / 100;
  if (k < 0.75) return 0;
  return rnd() * 300;
}

// shuffle returns the numbers 0..n-1 in a random order.
function shuffle(n) {
  const a = Array.from({ length: n }, (_, i) => i);
  for (let i = n - 1; i > 0; i--) {
    const j = Math.floor(rnd() * (i + 1));
    [a[i], a[j]] = [a[j], a[i]];
  }
  return a;
}

// graph returns the node count and the links [source, target, value] of a
// random graph. kind selects the shape.
function randomGraph(kind) {
  const style = pick(rnd, ['mixed', 'mixed', 'mixed', 'int', 'int', 'ties', 'odd']);
  const links = [];
  let n;
  if (kind === 'layered') {
    // Columns of nodes with links mostly to the next column, as the flow
    // diagrams of mermaid have.
    const layers = int(rnd, 2, 6);
    const members = [];
    n = 0;
    for (let l = 0; l < layers; l++) {
      const size = int(rnd, 1, 6);
      members.push(Array.from({ length: size }, () => n++));
    }
    const order = rnd() < 0.5 ? shuffle(n) : Array.from({ length: n }, (_, i) => i);
    for (let l = 1; l < layers; l++) {
      for (const t of members[l]) {
        const count = int(rnd, 1, 3);
        for (let c = 0; c < count; c++) {
          const from = rnd() < 0.85 ? l - 1 : int(rnd, 0, l - 1);
          links.push([order[pick(rnd, members[from])], order[t], linkValue(style)]);
        }
      }
    }
  } else if (kind === 'chain') {
    n = int(rnd, 2, 12);
    const order = shuffle(n);
    for (let i = 1; i < n; i++) links.push([order[i - 1], order[i], linkValue(style)]);
    const extra = int(rnd, 0, 3);
    for (let c = 0; c < extra; c++) {
      const a = int(rnd, 0, n - 2);
      links.push([order[a], order[int(rnd, a + 1, n - 1)], linkValue(style)]);
    }
  } else {
    // Any acyclic graph: links go forward in a random order of the nodes.
    n = int(rnd, 1, pick(rnd, [4, 8, 16, 40]));
    const order = shuffle(n);
    const count = n < 2 ? 0 : int(rnd, 0, Math.min(90, n * 3));
    for (let c = 0; c < count; c++) {
      const a = int(rnd, 0, n - 2);
      links.push([order[a], order[int(rnd, a + 1, n - 1)], linkValue(style)]);
    }
  }
  if (kind === 'cycle' && n >= 1) {
    // A link backward, or a link from a node to itself.
    const k = rnd();
    if (k < 0.3 || links.length === 0) {
      const a = int(rnd, 0, n - 1);
      links.push([a, a, linkValue(style)]);
    } else {
      const [s, t] = pick(rnd, links);
      links.splice(int(rnd, 0, links.length), 0, [t, s, linkValue(style)]);
    }
  }
  if (kind === 'missing') {
    const bad = pick(rnd, [n, n + 5, -1]);
    const link = rnd() < 0.5 ? [bad, int(rnd, 0, n - 1), 1] : [int(rnd, 0, n - 1), bad, 1];
    links.splice(int(rnd, 0, links.length), 0, link);
  }
  return { n, links };
}

function randomOptions() {
  const opt = {};
  if (rnd() < 0.9) opt.align = pick(rnd, ['left', 'right', 'center', 'justify']);
  if (rnd() < 0.8) opt.width = pick(rnd, [10, 10, 24, 15, 1, 0, 5.5, rnd() * 60, 700]);
  if (rnd() < 0.8) opt.padding = pick(rnd, [12, 27, 8, 0, 3.3, rnd() * 40, 100, 1000]);
  const k = rnd();
  if (k < 0.45) opt.extent = [0, 0, 600, 400];
  else if (k < 0.7) {
    const x0 = Math.round((rnd() - 0.3) * 200), y0 = Math.round((rnd() - 0.3) * 200);
    opt.extent = [x0, y0, x0 + int(rnd, 50, 1500), y0 + int(rnd, 50, 1000)];
  } else if (k < 0.8) opt.extent = [(rnd() - 0.5) * 100, (rnd() - 0.5) * 100, rnd() * 1000, rnd() * 800];
  else if (k < 0.9) opt.size = [pick(rnd, [600, 960, 100.5, 1]), pick(rnd, [400, 500, 33.3, 1])];
  else if (k < 0.93) opt.extent = pick(rnd, [[0, 0, 0, 0], [10, 10, 5, 300], [0, 100, 300, 100], [0, 300, 300, 0]]);
  if (rnd() < 0.5) opt.iterations = pick(rnd, [0, 1, 2, 3, 6, 10, 32, 64, 2.5]);
  const s = rnd();
  if (s < 0.08) opt.nodeSort = 'null';
  else if (s < 0.2) opt.nodeSort = pick(rnd, Object.keys(nodeSorts));
  const l = rnd();
  if (l < 0.08) opt.linkSort = 'null';
  else if (l < 0.2) opt.linkSort = pick(rnd, Object.keys(linkSorts));
  // How the links name their nodes: by a string id, by the index (the
  // default id), or by the node object.
  opt.ids = pick(rnd, ['string', 'string', 'index', 'object']);
  return opt;
}

function generator(opt) {
  const s = sankey();
  if (opt.ids === 'string') s.nodeId((d) => d.id);
  if (opt.align) s.nodeAlign(aligns[opt.align]);
  if (opt.width !== undefined) s.nodeWidth(opt.width);
  if (opt.padding !== undefined) s.nodePadding(opt.padding);
  if (opt.extent) s.extent([[opt.extent[0], opt.extent[1]], [opt.extent[2], opt.extent[3]]]);
  if (opt.size) s.size(opt.size);
  if (opt.iterations !== undefined) s.iterations(opt.iterations);
  if (opt.nodeSort) s.nodeSort(opt.nodeSort === 'null' ? null : nodeSorts[opt.nodeSort]);
  if (opt.linkSort) s.linkSort(opt.linkSort === 'null' ? null : linkSorts[opt.linkSort]);
  return s;
}

function build(g, opt, fixed) {
  const nodes = Array.from({ length: g.n }, (_, i) => ({ id: 'n' + i }));
  for (const [i, v] of fixed) nodes[i].fixedValue = v;
  const end = (i) => (opt.ids === 'string' ? 'n' + i : opt.ids === 'object' ? nodes[i] ?? i : i);
  const links = g.links.map(([s, t, v]) => ({ source: end(s), target: end(t), value: v }));
  return { nodes, links };
}

const link = sankeyLinkHorizontal();

// result reads everything that the layout set.
function result(graph) {
  const ni = [], nf = [], sl = [], tl = [], lf = [], paths = [];
  for (const d of graph.nodes) {
    ni.push(d.index, d.depth, d.height, d.layer);
    nf.push(d.value, d.x0, d.x1, d.y0, d.y1);
    sl.push(d.sourceLinks.map((l) => l.index));
    tl.push(d.targetLinks.map((l) => l.index));
  }
  for (const l of graph.links) {
    lf.push(l.width, l.y0, l.y1);
    paths.push(link(l));
  }
  return { ni, nf: floats(nf), sl, tl, lf: floats(lf), paths };
}

function makeCase(kind) {
  const g = randomGraph(kind);
  const opt = randomOptions();
  const fixed = [];
  if (rnd() < 0.1) {
    const count = int(rnd, 1, Math.max(1, g.n >> 1));
    for (let c = 0; c < count; c++) fixed.push([int(rnd, 0, g.n - 1), pick(rnd, [0, 5, 100, rnd() * 400])]);
  }
  const c = {
    n: g.n,
    links: g.links.map(([s, t]) => [s, t]),
    values: floats(g.links.map((l) => l[2])),
    fixed: fixed.map(([i]) => i),
    fixedValues: floats(fixed.map((f) => f[1])),
    opt,
  };
  const graph = build(g, opt, fixed);
  const s = generator(opt);
  try {
    s(graph);
    Object.assign(c, result(graph));
  } catch (e) {
    c.err = e.message;
    return c;
  }
  // Lay out the same objects again: the links now hold node objects.
  if (rnd() < 0.15) {
    s(graph);
    c.again = result(graph);
  }
  // Move some nodes and update the links.
  if (rnd() < 0.15) {
    const moves = [];
    const count = int(rnd, 1, 3);
    for (let m = 0; m < count; m++) {
      const i = int(rnd, 0, g.n - 1), dy = Math.round((rnd() - 0.5) * 2000) / 10;
      graph.nodes[i].y0 += dy;
      graph.nodes[i].y1 += dy;
      moves.push([i, dy]);
    }
    s.update(graph);
    c.moves = moves;
    c.updated = result(graph);
  }
  return c;
}

const layout = [];
for (let i = 0; i < 520; i++) layout.push(makeCase(pick(rnd, ['layered', 'layered', 'layered', 'dag', 'dag', 'chain'])));
const errors = [];
for (let i = 0; i < 80; i++) errors.push(makeCase(pick(rnd, ['cycle', 'cycle', 'missing'])));
// No nodes, and nodes without links.
for (const [n, links] of [[0, []], [1, []], [3, []], [2, [[0, 1, 0]]], [2, [[0, 1, NaN]]], [3, [[0, 1, 5], [0, 1, 5], [1, 2, 0]]]]) {
  for (const align of ['left', 'right', 'center', 'justify']) {
    const opt = { align, extent: [0, 0, 600, 400], width: 10, padding: 27, ids: 'string' };
    const c = { n, links: links.map(([s, t]) => [s, t]), values: floats(links.map((l) => l[2])), fixed: [], fixedValues: floats([]), opt };
    const graph = build({ n, links }, opt, []);
    try {
      generator(opt)(graph);
      Object.assign(c, result(graph));
    } catch (e) {
      c.err = e.message;
    }
    errors.push(c);
  }
}

writeGolden('internal/d3/sankey/testdata/sankey.json.gz', { groups: { layout, errors } });
const failed = [...layout, ...errors].filter((c) => c.err);
const messages = new Map();
for (const c of failed) messages.set(c.err, (messages.get(c.err) ?? 0) + 1);
console.log([...messages].map(([m, k]) => `${k} x "${m}"`).join('; '));
