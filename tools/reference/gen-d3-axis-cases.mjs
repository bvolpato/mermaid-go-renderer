// Writes the cases for the d3-axis oracle of internal/d3/axis. The real
// d3-axis needs a DOM, so the cases run in the browser oracle:
//
//   node gen-d3-axis-cases.mjs
//   node oracle.mjs eval d3-axis-page.js --args ../../.parity/probe/gantt/axis-args.json \
//     --out ../../.parity/probe/gantt/axis-out.json
//   node gen-d3-axis-cases.mjs --pack
//
// The last command writes the golden file
// internal/d3/axis/testdata/axis_chromium.json.gz. It has the cases and, for
// each case, the outerHTML of the container after each call of the axis.
import { readFileSync, writeFileSync, mkdirSync } from 'node:fs';
import { gzipSync } from 'node:zlib';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { prng, pick, int } from './d3-golden-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const rnd = prng(0xa715);
const DAY = 864e5;

const round = (x, p) => Math.round(x * 10 ** p) / 10 ** p;
const coordinate = () => pick(rnd, [0, 0, 10, 500, 634, 100.5, -20, round(rnd() * 800, 2), int(rnd, -50, 900)]);
const range = () => {
  const a = coordinate();
  let b = coordinate();
  if (rnd() < 0.8) b = a + pick(rnd, [634, 500, 300, 120.5, 61, 1000, 7]);
  return [a, b];
};

function linearScale() {
  const k = rnd();
  let domain;
  if (k < 0.3) domain = [0, pick(rnd, [1, 10, 100, 1000, 5, 0.5, 37])];
  else if (k < 0.6) domain = [int(rnd, -100, 100), int(rnd, -100, 1000)];
  else if (k < 0.8) domain = [round(rnd() * 10 - 5, 3), round(rnd() * 1000, 3)];
  else if (k < 0.9) domain = [1e6 * int(rnd, 0, 5), 1e6 * int(rnd, 6, 50)];
  else domain = [pick(rnd, [5, 0, -3]), pick(rnd, [5, 0, 1e-3])];
  return { type: 'linear', domain, range: range() };
}

function timeScale() {
  const start = Date.UTC(int(rnd, 2010, 2030), int(rnd, 0, 11), int(rnd, 1, 28), pick(rnd, [0, 0, 0, int(rnd, 0, 23)]), pick(rnd, [0, 0, int(rnd, 0, 59)]), pick(rnd, [0, 0, int(rnd, 0, 59)]));
  const span = pick(rnd, [12e3, 90e3, 3600e3, 6 * 3600e3, DAY, 3 * DAY, 7 * DAY, 20 * DAY, 45 * DAY, 100 * DAY, 400 * DAY, 2000 * DAY, 8000 * DAY, 13, 900, 0]);
  const domain = [start, start + span];
  if (rnd() < 0.1) domain.reverse();
  return { type: 'time', domain, range: rnd() < 0.5 ? [0, 634] : range() };
}

const names = ['a', 'b', 'c', 'Section', 'd e', 'x<y', '1', '', 'Últ', 'a'];
function bandScale(type) {
  const n = int(rnd, 0, 6);
  const domain = [];
  for (let i = 0; i < n; i++) domain.push(pick(rnd, names));
  const s = { type, domain: [...new Set(domain)], range: range(), round: rnd() < 0.5, align: pick(rnd, [0.5, 0.5, 0, 1, 0.3]) };
  if (type === 'band') {
    s.paddingInner = pick(rnd, [0, 0, 0.1, 0.5, 1]);
    s.paddingOuter = pick(rnd, [0, 0, 0.1, 0.5, 2]);
  } else {
    s.padding = pick(rnd, [0, 0, 0.5, 1]);
  }
  return s;
}

function scale(type) {
  if (type === 'linear') return linearScale();
  if (type === 'time') return timeScale();
  return bandScale(type);
}

const timeSpecs = ['%Y-%m-%d', '%d/%m', '%H:%M', '%Ss', '%b %d', '%L', '%a %d %B %Y', '%m-%d', '%d', '%Y'];
const numberSpecs = ['.2f', 'd', ',.0f', '.1%', 's', '.3s', '+', 'e', '~g'];
const intervals = ['millisecond', 'second', 'minute', 'hour', 'day', 'week', 'monday', 'thursday', 'month', 'year'];
const intervalMs = { millisecond: 1, second: 1e3, minute: 6e4, hour: 36e5, day: DAY, week: 7 * DAY, monday: 7 * DAY, thursday: 7 * DAY, month: 28 * DAY, year: 365 * DAY };

const cases = [];
function add(type) {
  const c = { orient: pick(rnd, ['top', 'right', 'bottom', 'left']), scale: scale(type) };
  const numeric = type === 'linear' || type === 'time';
  if (numeric) {
    const k = rnd();
    if (k < 0.25) c.ticks = [pick(rnd, [5, 10, 2, 20, 3, 0, 1])];
    else if (k < 0.4 && type === 'linear') c.ticks = [pick(rnd, [5, 10, 4]), pick(rnd, numberSpecs)];
    else if (k < 0.4 && type === 'time') c.ticks = [pick(rnd, [5, 10, 4]), pick(rnd, timeSpecs)];
    else if (k < 0.6 && type === 'time') {
      // An interval that gives at most 300 ticks: more would be slow.
      const span = Math.abs(c.scale.domain[1] - c.scale.domain[0]);
      for (let tries = 0; tries < 50; tries++) {
        const interval = pick(rnd, intervals);
        const every = pick(rnd, [1, 1, 2, 3, 6, 15]);
        if (span / (intervalMs[interval] * every) <= 300) {
          c.ticks = [{ interval, every }];
          break;
        }
      }
    }
    else if (k < 0.65) c.ticks = [null];
    else if (k < 0.7) c.ticks = [];
    if (rnd() < 0.15) {
      const [a, b] = c.scale.domain;
      c.tickValues = [];
      const n = int(rnd, 0, 5);
      for (let i = 0; i < n; i++) c.tickValues.push(type === 'time' ? Math.round(a + (b - a) * rnd()) : round(a + (b - a) * (rnd() * 1.4 - 0.2), 2));
      if (n > 1 && rnd() < 0.3) c.tickValues.push(c.tickValues[0]);
    }
    const f = rnd();
    if (f < 0.35) c.tickFormat = type === 'time' ? { kind: 'time', spec: pick(rnd, timeSpecs) } : { kind: 'number', spec: pick(rnd, numberSpecs) };
    else if (f < 0.45) c.tickFormat = { kind: 'wrap' };
  } else if (rnd() < 0.2) {
    c.tickFormat = { kind: 'wrap' };
  }
  const k = rnd();
  if (k < 0.3) c.tickSize = pick(rnd, [6, 0, -87, -150.5, 10, 3]);
  else if (k < 0.45) c.tickSizeInner = pick(rnd, [0, -40, 12, 2.5]);
  else if (k < 0.6) c.tickSizeOuter = pick(rnd, [0, -40, 12, 2.5]);
  if (rnd() < 0.25) c.tickPadding = pick(rnd, [0, 3, 8, -2, 1.5]);
  if (rnd() < 0.2) c.offset = pick(rnd, [0, 0, 0.5, 1, -0.5]);
  // Later calls of the same axis on the same container, with a new scale.
  if (rnd() < 0.35) {
    c.steps = [];
    const n = int(rnd, 1, 2);
    for (let i = 0; i < n; i++) {
      const next = scale(type);
      if (numeric && rnd() < 0.6) {
        // A domain that overlaps the previous one, so that ticks stay.
        const prev = (c.steps.at(-1) ?? c).scale;
        const [a, b] = prev.domain;
        const shift = (b - a) * pick(rnd, [0.25, 0.5, -0.5, 1, 0]);
        next.domain = type === 'time' ? [Math.round(a + shift), Math.round(b + shift * pick(rnd, [1, 2]))] : [a + shift, b + shift * pick(rnd, [1, 2])];
        next.range = prev.range;
      } else if (!numeric && rnd() < 0.7) {
        const prev = (c.steps.at(-1) ?? c).scale;
        next.domain = [...new Set([...prev.domain.filter(() => rnd() < 0.6), ...next.domain.filter(() => rnd() < 0.5)])];
        if (rnd() < 0.3) next.domain.reverse();
        next.range = prev.range;
      }
      c.steps.push({ scale: next });
    }
  }
  cases.push(c);
}
for (let i = 0; i < 70; i++) add('linear');
for (let i = 0; i < 90; i++) add('time');
for (let i = 0; i < 45; i++) add('band');
for (let i = 0; i < 35; i++) add('point');

// The axes of the gantt renderer.
for (const [d0, d1, h, every, interval, spec] of [
  [Date.UTC(2014, 0, 6), Date.UTC(2014, 1, 14, 20), 772, null, null, '%d/%m'],
  [Date.UTC(2022, 9, 1), Date.UTC(2022, 11, 9), 196, 1, 'day', '%m-%d'],
  [Date.UTC(2026, 0, 15), Date.UTC(2026, 0, 15, 0, 0, 12), 172, 2, 'second', '%Ss'],
  [Date.UTC(2022, 9, 1), Date.UTC(2022, 11, 9), 196, 1, 'monday', '%Y-%m-%d'],
  [Date.UTC(2022, 9, 1), Date.UTC(2023, 11, 9), 196, 1, 'month', '%b'],
]) {
  for (const orient of ['bottom', 'top']) {
    const c = { orient, scale: { type: 'time', domain: [d0, d1], range: [0, 634], rangeRound: true }, tickSize: -h + 50 + 35, tickFormat: { kind: 'time', spec } };
    if (interval) c.ticks = [{ interval, every }];
    cases.push(c);
  }
}

const probe = resolve(here, '../../.parity/probe/gantt');
if (!process.argv.includes('--pack')) {
  mkdirSync(probe, { recursive: true });
  const out = resolve(probe, 'axis-args.json');
  writeFileSync(out, JSON.stringify({ d3: readFileSync(resolve(here, 'node_modules/d3/dist/d3.min.js'), 'utf8'), cases }));
  console.log(`${cases.length} cases -> ${out}`);
} else {
  const result = JSON.parse(readFileSync(resolve(probe, 'axis-out.json'), 'utf8'));
  if (JSON.stringify(result.cases) !== JSON.stringify(cases)) throw new Error('the oracle output does not match the cases: run the oracle again');
  const failed = result.results.filter((r) => r.error);
  if (failed.length) throw new Error(`${failed.length} cases failed in the oracle: ${failed[0].error}`);
  const file = resolve(here, '../../internal/d3/axis/testdata/axis_chromium.json.gz');
  mkdirSync(dirname(file), { recursive: true });
  const gz = gzipSync(Buffer.from(JSON.stringify(result)), { level: 9 });
  writeFileSync(file, gz);
  console.log(`${cases.length} cases -> ${file} (${(gz.length / 1024).toFixed(0)} KiB)`);
}
