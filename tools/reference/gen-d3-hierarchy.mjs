// Emits golden cases for internal/d3/hierarchy from d3-hierarchy 3.1.2.
//
// Run it in tools/reference:
//
//   node gen-d3-hierarchy.mjs
//
// A case is a random tree. The data of a node is {id, value, children}, and
// the id is the index of the node in the lists of the case. The group
// "hierarchy" has the traversals and the queries of a node. The group
// "treemap" has the bounds that the treemap layout sets, as bits.
import { prng, floats, writeGolden, pick, int } from './d3-golden-lib.mjs';
import {
  hierarchy, treemap, treemapBinary, treemapDice, treemapSlice, treemapSliceDice, treemapSquarify, treemapResquarify,
} from 'd3-hierarchy';

const rnd = prng(0x7ee5);

function randomValue(style) {
  const k = rnd();
  if (style === 'int') return int(rnd, 1, 100);
  if (style === 'ties') return pick(rnd, [1, 2, 2, 5, 10]);
  if (style === 'odd') {
    if (k < 0.25) return 0;
    if (k < 0.35) return NaN;
    if (k < 0.45) return -rnd() * 10;
    if (k < 0.55) return rnd() * 1e-9;
    if (k < 0.65) return rnd() * 1e12;
    return rnd() * 50;
  }
  if (k < 0.45) return int(rnd, 1, 500);
  if (k < 0.75) return Math.round(rnd() * 100000) / 100;
  if (k < 0.8) return 0;
  return rnd() * 300;
}

// randomTree returns the lists of a tree: kids[i] has the ids of the
// children of node i, values[i] is the value, and has[i] says whether the
// datum has a value at all.
function randomTree() {
  const shape = pick(rnd, ['any', 'any', 'any', 'flat', 'deep', 'wide', 'single']);
  const style = pick(rnd, ['mixed', 'mixed', 'mixed', 'int', 'int', 'ties', 'odd']);
  let n;
  if (shape === 'single') n = 1;
  else n = int(rnd, 2, pick(rnd, [5, 12, 30, 70]));
  const kids = Array.from({ length: n }, () => []);
  for (let i = 1; i < n; i++) {
    let parent;
    if (shape === 'flat') parent = 0;
    else if (shape === 'deep') parent = rnd() < 0.8 ? i - 1 : int(rnd, 0, i - 1);
    else if (shape === 'wide') parent = int(rnd, 0, Math.min(i - 1, 3));
    else parent = int(rnd, Math.max(0, i - int(rnd, 1, 12)), i - 1);
    kids[parent].push(i);
  }
  const values = [], has = [];
  // Mostly only leaves have a value, as in a treemap of mermaid.
  const inner = rnd() < 0.3;
  for (let i = 0; i < n; i++) {
    const leaf = kids[i].length === 0;
    const h = leaf ? rnd() < 0.95 : inner && rnd() < 0.5;
    has.push(h ? 1 : 0);
    values.push(h ? randomValue(style) : 0);
  }
  return { n, kids, values, has, emptyKids: rnd() < 0.3 };
}

// data builds the objects of a tree.
function data(t, values = t.values) {
  const nodes = Array.from({ length: t.n }, (_, id) => {
    const d = { id };
    if (t.has[id]) d.value = values[id];
    return d;
  });
  for (let i = 0; i < t.n; i++) {
    if (t.kids[i].length > 0) nodes[i].children = t.kids[i].map((k) => nodes[k]);
    else if (t.emptyKids) nodes[i].children = [];
  }
  return nodes[0];
}

const valueFns = {
  value: (d) => d.value,
  leaf: (d) => (d.children && d.children.length > 0 ? 0 : d.value),
  one: () => 1,
};
function applyValues(root, mode) {
  if (mode === 'count') root.count();
  else if (mode !== 'none') root.sum(valueFns[mode]);
}

const sortFns = {
  valueDesc: (a, b) => b.value - a.value,
  valueAsc: (a, b) => a.value - b.value,
  heightValue: (a, b) => b.height - a.height || b.value - a.value,
  idDesc: (a, b) => b.data.id - a.data.id,
};

// byId lists a property of every node in the order of the ids.
function byId(root, n, f) {
  const out = new Array(n).fill(null);
  root.each((d) => (out[d.data.id] = f(d)));
  return out;
}
const ids = (nodes) => nodes.map((d) => (d ? d.data.id : -1));

function hierarchyCase() {
  const t = randomTree();
  const c = { ...t, values: floats(t.values) };
  const root = hierarchy(data(t));
  const n = t.n;
  c.depth = byId(root, n, (d) => d.depth);
  c.height = byId(root, n, (d) => d.height);
  c.parent = byId(root, n, (d) => (d.parent ? d.parent.data.id : -1));
  c.valueMode = pick(rnd, ['value', 'value', 'leaf', 'one', 'count', 'none']);
  applyValues(root, c.valueMode);
  c.value = floats(byId(root, n, (d) => (d.value === undefined ? NaN : d.value)));
  c.sortMode = pick(rnd, ['none', 'valueDesc', 'valueDesc', 'valueAsc', 'heightValue', 'idDesc']);
  if (c.sortMode !== 'none') root.sort(sortFns[c.sortMode]);

  // Traversals, each with the index that the callback gets.
  const record = (method) => {
    const out = [];
    let ok = true;
    root[method]((d, i, r) => {
      if (i !== out.length || r !== root) ok = false;
      out.push(d.data.id);
    });
    if (!ok) throw new Error('unexpected callback arguments');
    return out;
  };
  c.each = record('each');
  c.eachBefore = record('eachBefore');
  c.eachAfter = record('eachAfter');
  c.descendants = ids(root.descendants());
  c.leaves = ids(root.leaves());
  c.links = root.links().flatMap((l) => [l.source.data.id, l.target.data.id]);

  // Queries on some nodes.
  const all = root.descendants();
  c.queries = [];
  for (let q = 0; q < 4; q++) {
    const a = pick(rnd, all), b = pick(rnd, all);
    const m = int(rnd, 2, 7), r = int(rnd, 0, m - 1);
    const found = a.find((d) => d.data.id % m === r);
    const copy = a.copy();
    const copied = [];
    copy.eachBefore((d) => copied.push(d.data.id, d.depth, d.height, d.parent ? d.parent.data.id : -1));
    const copyValues = [];
    copy.eachBefore((d) => copyValues.push(d.value === undefined ? NaN : d.value));
    c.queries.push({
      a: a.data.id,
      b: b.data.id,
      ancestors: ids(a.ancestors()),
      path: ids(a.path(b)),
      descendants: ids(a.descendants()),
      leaves: ids(a.leaves()),
      links: a.links().flatMap((l) => [l.source.data.id, l.target.data.id]),
      find: [m, r, found ? found.data.id : -1],
      copy: copied,
      copyValues: floats(copyValues),
    });
  }
  return c;
}

// The padding settings. "mermaid" is the setting of the treemap diagram.
function randomPadding() {
  const k = rnd();
  if (k < 0.15) return { kind: 'none' };
  if (k < 0.3) return { kind: 'inner', v: [pick(rnd, [1, 2, 5, 10, 3.5])] };
  if (k < 0.45) return { kind: 'all', v: [pick(rnd, [1, 2, 5, 10, 0.5, 40])] };
  if (k < 0.6) return { kind: 'sides', v: [int(rnd, 0, 30), int(rnd, 0, 12), int(rnd, 0, 12), int(rnd, 0, 12), pick(rnd, [0, 1, 4, 10])] };
  if (k < 0.85) return { kind: 'mermaid', v: [pick(rnd, [10, 10, 5, 0, 20])] };
  return { kind: 'depth', v: [] };
}

function applyPadding(layout, p) {
  const branch = (d) => d.children && d.children.length > 0;
  switch (p.kind) {
    case 'inner': layout.paddingInner(p.v[0]); break;
    case 'all': layout.padding(p.v[0]); break;
    case 'sides': layout.paddingTop(p.v[0]).paddingRight(p.v[1]).paddingBottom(p.v[2]).paddingLeft(p.v[3]).paddingInner(p.v[4]); break;
    case 'mermaid':
      layout
        .paddingTop((d) => (branch(d) ? 35 : 0))
        .paddingInner(p.v[0])
        .paddingLeft((d) => (branch(d) ? 10 : 0))
        .paddingRight((d) => (branch(d) ? 10 : 0))
        .paddingBottom((d) => (branch(d) ? 10 : 0));
      break;
    case 'depth':
      layout.paddingInner((d) => d.depth * 2 + 1).paddingOuter((d) => d.height);
      break;
  }
}

function tileOf(name) {
  const [kind, ratio] = name.split(':');
  switch (kind) {
    case 'squarify': return ratio === undefined ? treemapSquarify : treemapSquarify.ratio(+ratio);
    case 'resquarify': return ratio === undefined ? treemapResquarify : treemapResquarify.ratio(+ratio);
    case 'binary': return treemapBinary;
    case 'dice': return treemapDice;
    case 'slice': return treemapSlice;
    case 'sliceDice': return treemapSliceDice;
  }
  throw new Error('unknown tile ' + name);
}

const bounds = (root, n) => floats(byId(root, n, (d) => [d.x0, d.y0, d.x1, d.y1]).flatMap((b) => b ?? [NaN, NaN, NaN, NaN]));

function treemapCase(forced = {}) {
  const t = randomTree();
  const c = { ...t, values: floats(t.values) };
  const n = t.n;
  const root = hierarchy(data(t));
  c.valueMode = forced.valueMode ?? pick(rnd, ['value', 'value', 'value', 'leaf', 'leaf', 'one', 'count', 'none']);
  applyValues(root, c.valueMode);
  c.sortMode = forced.sortMode ?? pick(rnd, ['none', 'valueDesc', 'valueDesc', 'valueDesc', 'valueAsc', 'heightValue']);
  if (c.sortMode !== 'none') root.sort(sortFns[c.sortMode]);

  c.size = forced.size ?? pick(rnd, [[960, 500], [960, 500], [600, 400], [1, 1], [100.5, 33.3], [int(rnd, 10, 2000), int(rnd, 10, 2000)], [0, 100], null]);
  c.round = forced.round ?? rnd() < 0.5;
  c.padding = forced.padding ?? randomPadding();
  c.tile = forced.tile ?? pick(rnd, ['default', 'default', 'squarify', 'squarify:1', 'squarify:2', 'squarify:0.5', 'squarify:3.7', 'binary', 'binary', 'dice', 'slice', 'sliceDice', 'resquarify', 'resquarify', 'resquarify:2']);

  const layout = treemap();
  if (c.size) layout.size(c.size);
  if (c.round) layout.round(true);
  applyPadding(layout, c.padding);
  if (c.tile !== 'default') layout.tile(tileOf(c.tile));
  layout(root);
  c.bounds = bounds(root, n);

  // A second layout of the same nodes with other values. Resquarify keeps
  // the rows of the first layout.
  if (c.valueMode !== 'count' && c.valueMode !== 'none' && rnd() < 0.5) {
    const style = pick(rnd, ['mixed', 'int']);
    const next = t.values.map(() => randomValue(style));
    root.each((d) => {
      if (t.has[d.data.id]) d.data.value = next[d.data.id];
    });
    applyValues(root, c.valueMode);
    c.second = { values: floats(next), resort: rnd() < 0.3 && c.sortMode !== 'none' };
    if (c.second.resort) root.sort(sortFns[c.sortMode]);
    layout(root);
    c.second.bounds = bounds(root, n);
  }

  // The layout of a node that is not the root.
  if (rnd() < 0.1) {
    const sub = pick(rnd, root.descendants());
    layout(sub);
    c.subtree = { id: sub.data.id, bounds: bounds(root, n) };
  }
  return c;
}

const hier = [];
for (let i = 0; i < 300; i++) hier.push(hierarchyCase());
const tm = [];
for (let i = 0; i < 700; i++) tm.push(treemapCase());
// The settings of the treemap diagram of mermaid.
for (let i = 0; i < 200; i++) {
  tm.push(treemapCase({
    valueMode: 'value', sortMode: 'valueDesc', size: [960, 500], round: true,
    padding: { kind: 'mermaid', v: [pick(rnd, [10, 10, 10, 5, 0])] }, tile: 'default',
  }));
}

writeGolden('internal/d3/hierarchy/testdata/hierarchy.json.gz', { groups: { hierarchy: hier, treemap: tm } });
