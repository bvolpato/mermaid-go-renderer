// Emits golden cases for internal/js/date_d3.go from V8: the Date getters,
// setters, the constructor with fields and Date.UTC.
//
// Run it once for each time zone:
//   TZ=UTC node gen-d3-jsdate.mjs
//   TZ=America/New_York node gen-d3-jsdate.mjs
import { prng, hex, floats, writeGolden, pick, int } from './d3-golden-lib.mjs';

const tz = process.env.TZ || 'UTC';
const rnd = prng(0x0da7e5);
const DAY = 864e5;

// Local time changes: scan for the instants at which the offset changes.
const transitions = [];
for (let t = Date.UTC(1880, 0, 1), prev = new Date(t).getTimezoneOffset(); t < Date.UTC(2040, 0, 1); t += DAY) {
  const off = new Date(t).getTimezoneOffset();
  if (off !== prev) {
    let lo = t - DAY, hi = t;
    while (hi - lo > 1000) {
      const mid = Math.floor((lo + hi) / 2000) * 1000;
      if (new Date(mid).getTimezoneOffset() === prev) lo = mid; else hi = mid;
    }
    transitions.push(hi);
    prev = off;
  }
}

function randomTime() {
  const k = rnd();
  if (transitions.length && k < 0.3) return pick(rnd, transitions) + int(rnd, -4 * 3600e3, 4 * 3600e3);
  if (k < 0.7) return Date.UTC(1970, 0, 1) + Math.floor((rnd() * 2 - 0.5) * 100 * 365 * DAY);
  if (k < 0.8) return Math.floor((rnd() * 2 - 1) * 8.64e15);
  if (k < 0.9) return Math.floor((rnd() * 2 - 1) * 400 * 365 * DAY);
  if (k < 0.93) return pick(rnd, [8.64e15, -8.64e15, 8.64e15 - 1, -8.64e15 + 1, 0, -1, 1, 8.64e15 - 3600e3 * 5, -8.64e15 + 3600e3 * 5]);
  if (k < 0.96) return Date.UTC(int(rnd, -10, 120), int(rnd, 0, 11), int(rnd, 1, 28), int(rnd, 0, 23), int(rnd, 0, 59));
  if (k < 0.98) return NaN;
  return Date.UTC(int(rnd, 1990, 2030), int(rnd, 0, 11), int(rnd, 1, 31));
}

function fields(d) {
  return [d.getFullYear(), d.getMonth(), d.getDate(), d.getDay(), d.getHours(), d.getMinutes(), d.getSeconds(), d.getMilliseconds(),
    d.getTimezoneOffset(), d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCDay(), d.getUTCHours(), d.getUTCMinutes(),
    d.getUTCSeconds(), d.getUTCMilliseconds()];
}

const get = [];
for (let i = 0; i < 3500; i++) {
  const t = randomTime();
  get.push({ t: hex(t), f: floats(fields(new Date(t))) });
}
// new Date(value) clips the value.
const clip = [];
for (const v of [0, -0, -0.9, 0.9, 1.9, -1.9, 8.64e15, 8.64e15 + 0.5, 8.64e15 + 1, -8.64e15 - 1, NaN, Infinity, -Infinity, 1e300, 1234567.891, -1234567.891]) {
  clip.push({ v: hex(v), t: hex(+new Date(v)) });
}

function arg(kind) {
  const k = rnd();
  if (k < 0.03) return NaN;
  if (k < 0.05) return pick(rnd, [Infinity, -Infinity, 1e21, -1e21, 1e9, -1e9, 2 ** 31, 2 ** 53]);
  if (k < 0.15) return (rnd() * 2 - 1) * 100;
  switch (kind) {
    case 'year': return pick(rnd, [int(rnd, 1900, 2100), int(rnd, 1900, 2100), int(rnd, -5, 105), int(rnd, -300000, 300000), int(rnd, 1960, 2030)]);
    case 'month': return pick(rnd, [int(rnd, 0, 11), int(rnd, 0, 11), int(rnd, -30, 40), int(rnd, -5000, 5000)]);
    case 'date': return pick(rnd, [int(rnd, 1, 31), int(rnd, 1, 31), int(rnd, -40, 70), int(rnd, -100000, 100000)]);
    case 'hours': return pick(rnd, [int(rnd, 0, 23), int(rnd, 0, 23), int(rnd, -50, 80), int(rnd, -1e6, 1e6)]);
    case 'minutes': case 'seconds': return pick(rnd, [int(rnd, 0, 59), int(rnd, 0, 59), int(rnd, -200, 200), int(rnd, -1e7, 1e7)]);
    default: return pick(rnd, [int(rnd, 0, 999), int(rnd, 0, 999), int(rnd, -5000, 5000), int(rnd, -1e10, 1e10)]);
  }
}
const kinds = ['year', 'month', 'date', 'hours', 'minutes', 'seconds', 'ms'];

const ctor = [], utc = [];
for (let i = 0; i < 2500; i++) {
  const n = int(rnd, 2, 7);
  const a = kinds.slice(0, n).map(arg);
  ctor.push({ a: floats(a), t: hex(+new Date(...a)) });
  const m = int(rnd, 1, 7);
  const b = kinds.slice(0, m).map(arg);
  utc.push({ a: floats(b), t: hex(Date.UTC(...b)) });
}
// Local times in and around the hours that a transition skips or repeats.
for (const tr of transitions) {
  for (let i = 0; i < 8; i++) {
    const d = new Date(tr + int(rnd, -3 * 3600e3, 3 * 3600e3));
    const a = [d.getUTCFullYear(), d.getUTCMonth(), d.getUTCDate(), d.getUTCHours() + int(rnd, -6, 6), d.getUTCMinutes(), d.getUTCSeconds()];
    ctor.push({ a: floats(a), t: hex(+new Date(...a)) });
  }
}

const setters = {
  setFullYear: ['year', 'month', 'date'], setMonth: ['month', 'date'], setDate: ['date'],
  setHours: ['hours', 'minutes', 'seconds', 'ms'], setMinutes: ['minutes', 'seconds', 'ms'], setSeconds: ['seconds', 'ms'],
  setMilliseconds: ['ms'], setTime: ['time'],
};
for (const [name, k] of Object.entries(setters)) if (name !== 'setTime') setters[name.replace('set', 'setUTC')] = k;
const names = Object.keys(setters);
const set = [];
for (let i = 0; i < 7000; i++) {
  const name = pick(rnd, names);
  const t = randomTime();
  const d = new Date(t);
  const n = int(rnd, 1, setters[name].length);
  const a = name === 'setTime' ? [pick(rnd, [randomTime(), (rnd() - 0.5) * 2e16, rnd() * 1e6 - 0.5])] : setters[name].slice(0, n).map(arg);
  const r = d[name](...a);
  if (!Object.is(r, +d) && !(Number.isNaN(r) && Number.isNaN(+d))) throw new Error('setter return value');
  set.push({ t: hex(t), m: name, a: floats(a), r: hex(r) });
}

const file = tz === 'UTC' ? 'd3_date_utc.json.gz' : 'd3_date_' + tz.replace(/.*\//, '').toLowerCase() + '.json.gz';
writeGolden('internal/js/testdata/' + file, { tz, transitions: transitions.length, groups: { get, clip, ctor, utc, set } });
