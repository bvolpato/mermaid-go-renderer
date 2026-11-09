// Emits golden cases for internal/d3/shape from d3-shape.
import { prng, hex, floats, nums, writeGolden, mathTable, pick, int } from './d3-golden-lib.mjs';
import * as d3 from 'd3-shape';

const rnd = prng(0x5a4e);
const TAU = 2 * Math.PI;

// A coordinate as diagrams have them: mostly an integer or a number with few
// decimals, sometimes any double, sometimes huge or tiny.
function coord() {
  const k = rnd();
  if (k < 0.45) return int(rnd, -50, 600);
  if (k < 0.75) return Math.round((rnd() * 700 - 50) * 100) / 100;
  if (k < 0.9) return Math.round((rnd() * 700 - 50) * 1e6) / 1e6;
  if (k < 0.95) return rnd() * 700 - 50;
  if (k < 0.97) return (rnd() - 0.5) * 10 ** int(rnd, -9, 12);
  return pick(rnd, [0, -0, 1e-7, 1e21, 0.0005, -0.0005, 1.0005, 2.5e-4, 0.1 + 0.2]);
}

const curves = [
  ['linear'], ['linearClosed'], ['basis'], ['basisClosed'], ['basisOpen'], ['bumpX'], ['bumpY'],
  ['bundle'], ['bundle', 0], ['bundle', 0.5], ['bundle', 1],
  ['cardinal'], ['cardinal', 0.5], ['cardinal', 1], ['cardinal', -1], ['cardinalClosed'], ['cardinalClosed', 0.5], ['cardinalOpen'], ['cardinalOpen', 0.5],
  ['catmullRom'], ['catmullRom', 0], ['catmullRom', 1], ['catmullRom', 0.25], ['catmullRomClosed'], ['catmullRomClosed', 1], ['catmullRomClosed', 0],
  ['catmullRomOpen'], ['catmullRomOpen', 1], ['catmullRomOpen', 0],
  ['monotoneX'], ['monotoneY'], ['natural'], ['step'], ['stepAfter'], ['stepBefore'],
];
function factory([name, param]) {
  const base = d3['curve' + name[0].toUpperCase() + name.slice(1)];
  if (param === undefined) return base;
  if (name === 'bundle') return base.beta(param);
  if (name.startsWith('cardinal')) return base.tension(param);
  return base.alpha(param);
}

// points returns n points. Some are equal to the previous point, some share
// a coordinate with it, some are on one line with the two previous points.
function points(n) {
  const pts = [];
  for (let i = 0; i < n; i++) {
    const k = rnd();
    const prev = pts[i - 1], prev2 = pts[i - 2];
    if (prev && k < 0.08) pts.push([prev[0], prev[1]]);
    else if (prev && k < 0.16) pts.push([prev[0], coord()]);
    else if (prev && k < 0.24) pts.push([coord(), prev[1]]);
    else if (prev2 && k < 0.3) pts.push([2 * prev[0] - prev2[0], 2 * prev[1] - prev2[1]]);
    else if (k < 0.33) pts.push([pick(rnd, [NaN, coord()]), pick(rnd, [NaN, coord()])]);
    else pts.push([coord(), coord()]);
  }
  // Sometimes monotone in x, as chart data is.
  if (rnd() < 0.25) pts.sort((a, b) => a[0] - b[0]);
  return pts;
}
const sizes = [0, 1, 2, 3, 3, 4, 4, 5, 6, 8, 12];

const lines = [];
for (const curve of curves) {
  for (let i = 0; i < 170; i++) {
    const pts = points(pick(rnd, sizes));
    // A mask of the defined points makes gaps.
    const gaps = rnd() < 0.2;
    const mask = pts.map(() => (gaps && rnd() < 0.25 ? 0 : 1));
    const digits = pick(rnd, [3, 3, 3, null, null, 0, 1, 5]);
    const line = d3.line().x((d) => d[0]).y((d) => d[1]).curve(factory(curve)).digits(digits);
    if (gaps) line.defined((d, j) => mask[j] === 1);
    lines.push({ c: curve[0], p: curve[1] === undefined ? null : curve[1], pts: nums(pts.flat()), mask: gaps ? mask.join('') : null, digits, d: line(pts) });
  }
}
// The default accessors and constant accessors.
const defaults = [];
for (let i = 0; i < 200; i++) {
  const pts = points(pick(rnd, sizes));
  const cx = coord();
  defaults.push({ pts: nums(pts.flat()), cx: hex(cx), d: d3.line()(pts), dx: d3.line().x(cx)(pts), dy: d3.line().y(cx)(pts) });
}

const areas = [];
for (const curve of curves) {
  if (curve[0] === 'bundle') continue; // the bundle curve has no area methods
  for (let i = 0; i < 42; i++) {
    const n = pick(rnd, sizes);
    const rows = Array.from({ length: n }, () => [coord(), coord(), coord(), coord()]);
    if (rnd() < 0.5) rows.sort((a, b) => a[0] - b[0]);
    const gaps = rnd() < 0.2;
    const mask = rows.map(() => (gaps && rnd() < 0.25 ? 0 : 1));
    const mode = pick(rnd, ['y0y1', 'y0y1', 'const', 'x0x1']);
    const base = coord();
    const area = d3.area().curve(factory(curve));
    if (mode === 'y0y1') area.x((d) => d[0]).y0((d) => d[1]).y1((d) => d[2]);
    else if (mode === 'const') area.x((d) => d[0]).y0(base).y1((d) => d[2]);
    else area.x0((d) => d[0]).x1((d) => d[3]).y0((d) => d[1]).y1((d) => d[2]);
    if (gaps) area.defined((d, j) => mask[j] === 1);
    areas.push({
      c: curve[0], p: curve[1] === undefined ? null : curve[1], rows: nums(rows.flat()), mask: gaps ? mask.join('') : null, mode, base: hex(base),
      d: area(rows), ...(i % 3 === 0 ? { lx0: area.lineX0()(rows), ly1: area.lineY1()(rows), lx1: area.lineX1()(rows) } : {}),
    });
  }
}

function radius() {
  return pick(rnd, [int(rnd, 0, 300), int(rnd, 0, 300), Math.round(rnd() * 3e4) / 100, rnd() * 300, 0, -0, 1e-13, 1e-9, -10, NaN, 185, 6.8181818181818175, 7.5]);
}
function arcAngle() {
  return pick(rnd, [rnd() * TAU, rnd() * TAU, rnd() * TAU, (rnd() - 0.5) * 3 * TAU, 0, Math.PI / 2, Math.PI, 3 * (Math.PI / 2), TAU, -Math.PI / 2, TAU + 1e-13, 1e-13, NaN, Math.round(rnd() * 628) / 100]);
}
const arcs = [];
for (let i = 0; i < 2800; i++) {
  const inner = radius(), outer = radius();
  const start = arcAngle();
  const end = pick(rnd, [arcAngle(), arcAngle(), start + rnd() * 0.5, start + TAU, start - 0.3, start]);
  const pad = pick(rnd, [0, 0, 0, 0.01, 0.05, rnd() * 0.3, 1, NaN, undefined, -0.1, 1e-13]);
  const corner = pick(rnd, [0, 0, 0, 2, 5, rnd() * 30, 500, NaN, -3, 1e-13]);
  const padRadius = pick(rnd, [null, null, null, int(rnd, 0, 300), 0, NaN]);
  const arc = d3.arc().innerRadius(inner).outerRadius(outer).startAngle(start).endAngle(end).cornerRadius(corner).padRadius(padRadius);
  if (pad !== undefined) arc.padAngle(pad);
  const d = arc();
  const c = arc.centroid();
  arcs.push({ a: nums([inner, outer, start, end, pad === undefined ? NaN : pad, corner, padRadius === null ? NaN : padRadius]), padUndefined: pad === undefined, padRadiusNull: padRadius === null, d, raw: arc.digits(null)(), c: floats(c) });
}
// The arcs of mermaid: the faces of the journey and timeline diagrams, and
// the default accessors on a datum.
const fixed = [];
for (const [s, e, ri, ro] of [[Math.PI / 2, 3 * (Math.PI / 2), 7.5, 6.8181818181818175], [3 * (Math.PI / 2), 5 * (Math.PI / 2), 7.5, 6.8181818181818175], [0, TAU, 0, 185], [0, Math.PI, 20, 40]]) {
  const arc = d3.arc().startAngle(s).endAngle(e).innerRadius(ri).outerRadius(ro);
  fixed.push({ a: nums([ri, ro, s, e]), d: arc(), c: floats(arc.centroid()) });
}
const datumArcs = [];
for (let i = 0; i < 300; i++) {
  const datum = { innerRadius: radius(), outerRadius: radius(), startAngle: arcAngle(), endAngle: arcAngle(), padAngle: pick(rnd, [0, 0.02, NaN]) };
  datumArcs.push({ a: nums([datum.innerRadius, datum.outerRadius, datum.startAngle, datum.endAngle, datum.padAngle]), d: d3.arc()(datum), c: floats(d3.arc().centroid(datum)), empty: d3.arc()({}) });
}

const ascending = (a, b) => (a < b ? -1 : a > b ? 1 : a >= b ? 0 : NaN);
const pies = [];
for (let i = 0; i < 1100; i++) {
  const n = pick(rnd, [0, 1, 2, 3, 4, 5, 6, 8, 12, 20, 70]);
  const values = Array.from({ length: n }, () => pick(rnd, [int(rnd, 1, 100), int(rnd, 1, 100), int(rnd, 1, 100), Math.round(rnd() * 1e4) / 100, rnd() * 50, 0, -5, NaN, 42, 1e-9]));
  const sort = pick(rnd, ['default', 'default', 'none', 'none', 'ascending', 'data']);
  const startAngle = pick(rnd, [0, 0, 0, -0, Math.PI / 2, rnd() * TAU, -Math.PI, NaN]);
  const endAngle = pick(rnd, [TAU, TAU, TAU, Math.PI, startAngle + rnd() * 10 - 5, -TAU, 100, NaN]);
  const padAngle = pick(rnd, [0, 0, 0, 0.02, 0.1, 1, -0.05, NaN]);
  const data = values.map((v, j) => ({ v, key: (j * 7) % 5 }));
  const pie = d3.pie().value((d) => d.v).startAngle(startAngle).endAngle(endAngle).padAngle(padAngle);
  if (sort === 'none') pie.sort(null);
  else if (sort === 'ascending') pie.sortValues(ascending);
  else if (sort === 'data') pie.sort((a, b) => a.key - b.key);
  const result = pie(data);
  // The pie chart of mermaid: an arc path and a label position for each slice.
  const arc = d3.arc().innerRadius(0).outerRadius(185);
  const label = d3.arc().innerRadius(185 * 0.75).outerRadius(185 * 0.75);
  const withPaths = n <= 6;
  pies.push({
    v: nums(values), sort, a: nums([startAngle, endAngle, padAngle]),
    r: nums(result.flatMap((x) => [x.index, x.value, x.startAngle, x.endAngle, x.padAngle])),
    d: withPaths ? result.map((x) => arc(x)) : null,
    c: withPaths ? floats(result.flatMap((x) => label.centroid(x))) : null,
  });
}
// The default value accessor: the datum is the value.
const plainPies = [];
for (let i = 0; i < 200; i++) {
  const values = Array.from({ length: int(rnd, 0, 8) }, () => pick(rnd, [int(rnd, 1, 100), rnd() * 50, 0, NaN]));
  plainPies.push({ v: nums(values), r: nums(d3.pie()(values).flatMap((x) => [x.index, x.value, x.startAngle, x.endAngle, x.padAngle])) });
}

writeGolden('internal/d3/shape/testdata/shape.json.gz', { math: mathTable(), groups: { lines, defaults, areas, arcs, fixed, datumArcs, pies, plainPies } });
