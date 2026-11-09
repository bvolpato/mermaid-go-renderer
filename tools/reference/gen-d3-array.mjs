// Emits golden cases for internal/d3/array from d3-array.
import { prng, hex, floats, writeGolden, mathTable, pick, int, magnitude, coord } from './d3-golden-lib.mjs';
import { ticks, tickIncrement, tickStep, nice, min, max, extent, sum, range, bisectLeft, bisectRight, bisectCenter, ascending, descending, bisector } from 'd3-array';

const rnd = prng(0xa77a1);

function tickArgs() {
  const k = rnd();
  let start, stop;
  if (k < 0.25) { start = int(rnd, -1000, 1000); stop = start + int(rnd, -2000, 2000); }
  else if (k < 0.5) { start = Math.round(rnd() * 1e5) / 100; stop = Math.round(rnd() * 1e5) / 100; }
  else if (k < 0.7) { start = coord(rnd); stop = coord(rnd); }
  else if (k < 0.8) { const e = int(rnd, -12, 12); start = int(rnd, -20, 20) * 10 ** e; stop = int(rnd, -20, 20) * 10 ** e; }
  else if (k < 0.9) { start = magnitude(rnd, -30, 30); stop = start * (1 + (rnd() - 0.5) * pick(rnd, [1e-3, 1e-9, 1, 10])); }
  else if (k < 0.95) { start = magnitude(rnd, -300, 300); stop = magnitude(rnd, -300, 300); }
  else { start = pick(rnd, [0, -0, NaN, 1, Infinity, -Infinity, 1e-320]); stop = pick(rnd, [0, 1, NaN, 10, 100, -Infinity, 1e-320]); }
  const count = pick(rnd, [10, 10, 10, 5, 2, 1, 1.9, 0.5, 0.4, 0.6, 3, 4, 7, 20, 50, 100, int(rnd, 1, 200), rnd() * 30, 0, -1, NaN]);
  return [start, stop, count];
}

const ticksCases = [];
for (let i = 0; i < 7000; i++) {
  const [start, stop, count] = tickArgs();
  let t = null;
  try {
    const r = ticks(start, stop, count);
    if (r.length > 60) continue;
    t = floats(r);
  } catch {
    t = null;
  }
  const c = { a: floats([start, stop, count]), t, inc: hex(tickIncrement(start, stop, count)), step: hex(tickStep(start, stop, count)) };
  if (isFinite(start) && isFinite(stop) && count >= 1 && Math.abs(start) < 1e200 && Math.abs(stop) < 1e200) c.nice = floats(nice(start, stop, count));
  ticksCases.push(c);
}

function values() {
  const n = pick(rnd, [0, 1, 2, 3, 5, 8, 20]);
  const out = [];
  for (let i = 0; i < n; i++) {
    const k = rnd();
    out.push(k < 0.1 ? NaN : k < 0.15 ? pick(rnd, [0, -0, Infinity, -Infinity]) : k < 0.5 ? int(rnd, -50, 50) : coord(rnd));
  }
  return out;
}
const und = (x) => (x === undefined ? NaN : x);
const stats = [];
for (let i = 0; i < 2500; i++) {
  const v = values();
  const e = extent(v);
  stats.push({
    v: floats(v), min: hex(und(min(v))), max: hex(und(max(v))), ok: min(v) !== undefined, e: floats(e.map(und)), sum: hex(sum(v)),
    // With an accessor: twice the value.
    min2: hex(und(min(v, (d) => d * 2))), max2: hex(und(max(v, (d) => d * 2))), sum2: hex(sum(v, (d) => d * 2)),
  });
}

const ranges = [];
for (let i = 0; i < 2500; i++) {
  const k = rnd();
  let a;
  if (k < 0.4) a = [int(rnd, -20, 20), int(rnd, -20, 40), pick(rnd, [1, 1, 2, 3, -1, -2, 0.5, 0.1, 0.25])];
  else if (k < 0.8) a = [coord(rnd) % 100, coord(rnd) % 100, (rnd() - 0.3) * 10];
  else a = [pick(rnd, [0, 1, NaN, -0]), pick(rnd, [0, 5, NaN, Infinity, -Infinity, 10]), pick(rnd, [0, -0, 1, NaN, Infinity, -1, 1e-320])];
  let r = null;
  try {
    const out = range(...a);
    if (out.length > 60) continue;
    r = floats(out);
  } catch {
    r = null;
  }
  ranges.push({ a: floats(a), r });
}

const bisects = [];
const acc = bisector((d) => d.v);
const cmp = bisector((d, x) => d.v - x);
for (let i = 0; i < 4000; i++) {
  const n = pick(rnd, [0, 1, 2, 3, 5, 10, 30]);
  const a = [];
  for (let j = 0; j < n; j++) a.push(rnd() < 0.5 ? int(rnd, -10, 10) : coord(rnd) % 50);
  a.sort((p, q) => p - q);
  const k = rnd();
  const x = k < 0.05 ? NaN : k < 0.5 && n ? pick(rnd, a) : k < 0.9 ? (rnd() - 0.5) * 30 : pick(rnd, [Infinity, -Infinity, 0, -0]);
  let lo = 0, hi = n;
  if (rnd() < 0.3 && n) { lo = int(rnd, 0, n); hi = int(rnd, lo, n); }
  const objs = a.map((v) => ({ v }));
  bisects.push({
    a: floats(a), x: hex(x), lo, hi,
    r: [bisectLeft(a, x, lo, hi), bisectRight(a, x, lo, hi), bisectCenter(a, x, lo, hi),
      acc.left(objs, x, lo, hi), acc.right(objs, x, lo, hi), acc.center(objs, x, lo, hi),
      cmp.left(objs, x, lo, hi), cmp.right(objs, x, lo, hi), cmp.center(objs, x, lo, hi)],
  });
}

const orders = [];
for (let i = 0; i < 2000; i++) {
  const a = pick(rnd, [NaN, 0, -0, 1, Infinity, -Infinity, coord(rnd), int(rnd, -3, 3)]);
  const b = pick(rnd, [NaN, 0, -0, 1, Infinity, -Infinity, coord(rnd), int(rnd, -3, 3), a]);
  orders.push({ a: hex(a), b: hex(b), asc: hex(ascending(a, b)), desc: hex(descending(a, b)) });
}

writeGolden('internal/d3/array/testdata/array.json.gz', { math: mathTable(), groups: { ticks: ticksCases, stats, ranges, bisects, orders } });
