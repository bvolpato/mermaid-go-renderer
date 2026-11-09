// Emits golden cases for internal/d3/scale from d3-scale.
//
// The time scales depend on the time zone. Run the script once for each zone:
//   TZ=UTC node gen-d3-scale.mjs
//   TZ=America/New_York node gen-d3-scale.mjs
// The run in UTC also writes the cases that do not depend on the zone.
//
// Source of d3-format: tickFormat of a linear scale calls d3-format, which
// d3-scale imports from node_modules. node_modules has d3-format 3.1.2, and
// the mermaid 12.2.0 bundle that the reference renders with has 3.1.0.
// use-bundled.mjs sends the import to the source of 3.1.0, which comes from
// the source map of the bundle. The two versions differ for a tick step of
// zero: precisionFixed(0) is 0 in 3.1.0 and NaN in 3.1.2. The other d3
// modules have the same version in the bundle and in node_modules.
import { prng, hex, floats, nums, writeGolden, mathTable, pauseMath, pick, int, coord } from './d3-golden-lib.mjs';
import { dateSource, tzFileSuffix } from './d3-golden-dates.mjs';
import { bundledVersion } from './use-bundled.mjs';
import { interpolateHcl, interpolateRgb } from 'd3-interpolate';
import * as d3time from 'd3-time';

// This is a dynamic import, because use-bundled.mjs must run before node
// resolves the "d3-format" that d3-scale imports.
const { scaleLinear, scaleBand, scalePoint, scaleOrdinal, scaleTime, scaleUtc } = await import('d3-scale');
// A domain of one point has a tick step of zero. d3-format 3.1.0 gives "0"
// here, and 3.1.2 gives "0.000000". The check is not in the table of Math calls.
if (bundledVersion('d3-format') !== '3.1.0' || pauseMath(() => scaleLinear().domain([0, 0]).tickFormat(5)(0)) !== '0') {
  throw new Error('gen-d3-scale.mjs: d3-format is not the version of the mermaid bundle (3.1.0)');
}

const rnd = prng(0x5ca1e);
const und = (x) => (x === undefined ? NaN : x);

function numbers(n) {
  const k = rnd();
  const v = () => (k < 0.4 ? int(rnd, -100, 500) : k < 0.8 ? Math.round(rnd() * 1e5) / 100 : coord(rnd));
  const out = Array.from({ length: n }, v);
  const m = rnd();
  if (m < 0.5) out.sort((a, b) => a - b);
  else if (m < 0.7) out.sort((a, b) => b - a);
  if (rnd() < 0.05 && n > 1) out[1] = out[0];
  if (rnd() < 0.03) out[int(rnd, 0, n - 1)] = NaN;
  return out;
}

function generalCases() {
  const linear = [];
  for (let i = 0; i < 3000; i++) {
    const n = pick(rnd, [2, 2, 2, 2, 3, 4, 1, 0]);
    const m = pick(rnd, [n, n, n, 2, 3]);
    const domain = numbers(n), range = numbers(m);
    const clamp = rnd() < 0.3, round = rnd() < 0.25;
    const niceCount = pick(rnd, [null, null, undefined, 10, 5, 2, 1, 20, 0]);
    const s = scaleLinear().domain(domain);
    if (round) s.rangeRound(range); else s.range(range);
    s.clamp(clamp);
    if (niceCount !== null) s.nice(niceCount);
    const lo = Math.min(...domain.filter((v) => !Number.isNaN(v)), 0), hi = Math.max(...domain.filter((v) => !Number.isNaN(v)), 1);
    const xs = Array.from({ length: 4 }, () => pick(rnd, [lo + rnd() * (hi - lo), lo + rnd() * (hi - lo), lo - 10, hi + 10, pick(rnd, domain.length ? domain : [0]), NaN]));
    const ys = Array.from({ length: 3 }, () => pick(rnd, [coord(rnd), pick(rnd, range.length ? range : [0]), NaN]));
    const count = pick(rnd, [undefined, 10, 5, 2, 20, 1, 0]);
    const spec = pick(rnd, [undefined, undefined, ',f', 's', '.2s', 'e', 'g', '%', '.1%', 'r', 'p', '', '$,.2f', '+.3~g', 'd', '~s', '.0f']);
    const c = {
      domain: nums(domain), range: nums(range), clamp, round, nice: niceCount === null ? null : niceCount === undefined ? 'default' : niceCount,
      niced: nums(s.domain()), xs: nums(xs), out: floats(xs.map((x) => und(s(x)))), ys: nums(ys), inv: floats(ys.map((y) => und(s.invert(y)))),
      count: count === undefined ? null : count, ticks: nums(count === undefined ? s.ticks() : s.ticks(count)),
      spec: spec === undefined ? null : spec, copy: floats(xs.map((x) => und(s.copy()(x)))),
    };
    try {
      const f = s.tickFormat(count === undefined ? 10 : count, spec);
      const tv = [...s.ticks(count === undefined ? 10 : count).slice(0, 6), ...xs.slice(0, 2)];
      c.tv = nums(tv); c.tf = tv.map((v) => f(v));
    } catch { c.tf = null; }
    linear.push(c);
  }

  // A range of colors, as the gantt renderer builds it.
  const colors = [];
  const names = ['#00B9FA', '#F95002', 'red', 'steelblue', 'white', 'black', 'hsl(120,50%,50%)', 'rgba(10,20,30,0.5)', '#abc'];
  for (let i = 0; i < 500; i++) {
    const n = pick(rnd, [2, 2, 3]);
    const domain = numbers(n).filter((v) => !Number.isNaN(v));
    if (domain.length !== n) continue;
    const range = Array.from({ length: n }, () => pick(rnd, names));
    const ip = pick(rnd, ['hcl', 'rgb']);
    const s = scaleLinear().domain(domain).range(range).interpolate(ip === 'hcl' ? interpolateHcl : interpolateRgb);
    const xs = Array.from({ length: 4 }, () => domain[0] + rnd() * (domain[n - 1] - domain[0]));
    colors.push({ domain: nums(domain), range, ip, xs: nums(xs), out: xs.map((x) => s(x)) });
  }

  const words = ['a', 'b', 'c', 'Jan', 'Feb', 'Mar', 'x1', 'x2', 'category one', '', 'é', '10', '2'];
  const bands = [];
  for (let i = 0; i < 2500; i++) {
    const n = pick(rnd, [0, 1, 2, 3, 4, 5, 8, 12]);
    const domain = Array.from({ length: n }, (_, j) => (rnd() < 0.1 ? pick(rnd, words) : pick(rnd, words) + j));
    const r = [coord(rnd), coord(rnd)];
    const ops = [];
    const point = rnd() < 0.3;
    const s = (point ? scalePoint() : scaleBand()).domain(domain);
    if (rnd() < 0.3) { s.rangeRound(r); ops.push(['rangeRound']); } else { s.range(r); ops.push(['range']); }
    for (let k = int(rnd, 0, 3); k > 0; k--) {
      const v = pick(rnd, [0, 0.1, 0.2, 0.5, 1, 1.5, -0.2, rnd(), NaN]);
      const op = pick(rnd, point ? ['padding', 'align', 'round'] : ['padding', 'paddingInner', 'paddingOuter', 'align', 'round']);
      if (op === 'round') { const b = rnd() < 0.5; s.round(b); ops.push([op, b ? 1 : 0]); } else { s[op](v); ops.push([op, v]); }
    }
    const probe = [...domain, 'not in the domain'];
    bands.push({
      point, domain, r: nums(r), ops: ops.map(([name, v]) => [name, v === undefined ? null : nums([v])[0]]),
      out: floats(probe.map((d) => und(s(d)))), bw: hex(s.bandwidth()), step: hex(s.step()),
      copy: floats(probe.map((d) => und(s.copy()(d)))), dom: s.domain(),
      // The setup of the band axis of the xychart.
    });
  }

  const ordinals = [];
  for (let i = 0; i < 1500; i++) {
    const domain = Array.from({ length: pick(rnd, [0, 0, 1, 3, 5]) }, () => pick(rnd, words));
    const range = Array.from({ length: pick(rnd, [0, 1, 2, 3, 10]) }, (_, j) => 'c' + j);
    const unknown = pick(rnd, [undefined, undefined, 'none']);
    const s = scaleOrdinal().domain(domain).range(range);
    if (unknown !== undefined) s.unknown(unknown);
    const lookups = Array.from({ length: int(rnd, 0, 12) }, () => pick(rnd, words));
    const out = lookups.map((d) => { const v = s(d); return v === undefined ? null : v; });
    ordinals.push({ domain, range, unknown: unknown === undefined ? null : unknown, lookups, out, dom: s.domain(), copyDom: s.copy().domain() });
  }
  // Number keys: NaN is one key, and -0 is the key 0.
  const numberKeys = [];
  for (let i = 0; i < 300; i++) {
    const lookups = Array.from({ length: int(rnd, 1, 10) }, () => pick(rnd, [0, -0, NaN, 1, 2, 1.5, Infinity]));
    const s = scaleOrdinal().range(['a', 'b', 'c']);
    numberKeys.push({ lookups: nums(lookups), out: lookups.map((d) => s(d)), dom: nums(s.domain()) });
  }
  return { linear, colors, bands, ordinals, numberKeys };
}

function timeCases() {
  const randomTime = dateSource(rnd, { nan: 0.01, far: 0 });
  const safe = (t) => Number.isNaN(t) || Math.abs(t) < 8.64e15 - 4e12;
  const S = 1000, M = 60 * S, H = 60 * M, D = 24 * H;
  const intervals = {
    time: { millisecond: d3time.timeMillisecond, second: d3time.timeSecond, minute: d3time.timeMinute, hour: d3time.timeHour, day: d3time.timeDay, monday: d3time.timeMonday, sunday: d3time.timeSunday, month: d3time.timeMonth, year: d3time.timeYear },
    utc: { millisecond: d3time.utcMillisecond, second: d3time.utcSecond, minute: d3time.utcMinute, hour: d3time.utcHour, day: d3time.utcDay, monday: d3time.utcMonday, sunday: d3time.utcSunday, month: d3time.utcMonth, year: d3time.utcYear },
  };
  const durations = { millisecond: 1, second: S, minute: M, hour: H, day: D, monday: 7 * D, sunday: 7 * D, month: 30 * D, year: 365 * D };
  const num = (d) => +d;
  const cases = [];
  for (let i = 0; i < 1800; i++) {
    const fam = pick(rnd, ['time', 'time', 'utc']);
    const start = randomTime();
    if (!safe(start)) continue;
    const k = rnd();
    const span = k < 0.08 ? int(rnd, 1, 5000) : k < 0.2 ? rnd() * 10 * M : k < 0.4 ? rnd() * 3 * D : k < 0.65 ? rnd() * 90 * D : k < 0.9 ? rnd() * 5 * 365 * D : rnd() * 200 * 365 * D;
    const stop = start + (rnd() < 0.1 ? -span : span);
    const domain = rnd() < 0.1 ? [start, start + span / 3, stop] : [start, stop];
    const range = domain.length === 3 ? [0, int(rnd, 100, 400), int(rnd, 500, 1200)] : [pick(rnd, [0, 0, 75, int(rnd, -50, 100)]), pick(rnd, [int(rnd, 200, 1500), 984, rnd() * 1000])];
    const round = rnd() < 0.5, clamp = rnd() < 0.2;
    const s = (fam === 'time' ? scaleTime() : scaleUtc()).domain(domain);
    if (round) s.rangeRound(range); else s.range(range);
    s.clamp(clamp);
    const c = { fam, domain: floats(domain), range: nums(range), round, clamp };
    // nice by count or by interval
    const niceMode = pick(rnd, ['none', 'none', 'default', 'count', 'interval']);
    if (niceMode === 'default') s.nice();
    else if (niceMode === 'count') { c.niceCount = pick(rnd, [10, 5, 2, 20, 1, 0]); s.nice(c.niceCount); } else if (niceMode === 'interval') {
      c.niceIv = pick(rnd, Object.keys(durations)); c.niceK = pick(rnd, [1, 1, 2, 5]);
      const iv = intervals[fam][c.niceIv].every(c.niceK);
      // A filtered interval can take very long on a wide domain.
      if (span / (durations[c.niceIv] * c.niceK) > 5000) continue;
      s.nice(iv);
    }
    c.niceMode = niceMode;
    c.niced = floats(s.domain().map(num));
    const d0 = +s.domain()[0], d1 = +s.domain()[s.domain().length - 1];
    const ts = Array.from({ length: 4 }, () => pick(rnd, [d0 + rnd() * (d1 - d0), d0 + rnd() * (d1 - d0), d0, d1, d0 - span, NaN]));
    c.ts = floats(ts); c.out = floats(ts.map((t) => und(s(t))));
    const ys = Array.from({ length: 3 }, () => pick(rnd, [rnd() * 1200, range[0], range[1], -50]));
    c.ys = nums(ys); c.inv = floats(ys.map((y) => num(s.invert(y))));
    c.copy = floats(ts.map((t) => und(s.copy()(t))));
    // ticks by count, and the default tick format on them
    const count = pick(rnd, [undefined, undefined, 10, 5, 2, 20, 3]);
    const ticks = count === undefined ? s.ticks() : s.ticks(count);
    if (ticks.length > 60) continue;
    c.count = count === undefined ? null : count;
    c.ticks = floats(ticks.map(num));
    const tf = s.tickFormat();
    c.tf = ticks.map((t) => tf(t));
    c.tfx = ts.map((t) => tf(new Date(t)));
    const spec = pick(rnd, ['%Y-%m-%d', '%d', '%H:%M', '%b %d', '%m/%d', '%a %d', '%Y-%m-%d %H:%M:%S', '%I:%M %p', '%x', '%j']);
    c.spec = spec; c.tfs = ticks.slice(0, 5).map((t) => s.tickFormat(0, spec)(t));
    // ticks by interval, as the gantt renderer asks for them
    c.iv = pick(rnd, Object.keys(durations)); c.ivK = pick(rnd, [1, 1, 2, 3, 7]);
    if (Math.abs(d1 - d0) / (durations[c.iv] * c.ivK) < 60) c.ivTicks = floats(s.ticks(intervals[fam][c.iv].every(c.ivK)).map(num));
    else c.ivTicks = null;
    cases.push(c);
  }
  // The default domain.
  const defaults = { time: floats(scaleTime().domain().map(num)), utc: floats(scaleUtc().domain().map(num)) };
  return { cases, defaults };
}

if (tzFileSuffix() === 'utc') {
  writeGolden('internal/d3/scale/testdata/scale.json.gz', { math: mathTable(), groups: generalCases() });
}
const t = timeCases();
writeGolden(`internal/d3/scale/testdata/scale_time_${tzFileSuffix()}.json.gz`, { tz: process.env.TZ || 'UTC', math: mathTable(), defaults: t.defaults, groups: { times: t.cases } });
