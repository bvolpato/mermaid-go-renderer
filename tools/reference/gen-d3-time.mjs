// Emits golden cases for internal/d3/time from d3-time.
//
// Run it once for each time zone:
//   TZ=UTC node gen-d3-time.mjs
//   TZ=America/New_York node gen-d3-time.mjs
import { prng, hex, floats, writeGolden, mathTable, pick, int } from './d3-golden-lib.mjs';
import { dateSource, tzFileSuffix } from './d3-golden-dates.mjs';
import * as d3 from 'd3-time';

const rnd = prng(0x71e5);
const randomTime = dateSource(rnd);
const S = 1000, M = 60 * S, H = 60 * M, D = 24 * H;
const durations = { millisecond: 1, second: S, minute: M, hour: H, day: D, unixDay: D, sunday: 7 * D, monday: 7 * D, tuesday: 7 * D, wednesday: 7 * D, thursday: 7 * D, friday: 7 * D, saturday: 7 * D, month: 30 * D, year: 365 * D };
const families = {
  time: { millisecond: d3.timeMillisecond, second: d3.timeSecond, minute: d3.timeMinute, hour: d3.timeHour, day: d3.timeDay, sunday: d3.timeSunday, monday: d3.timeMonday, tuesday: d3.timeTuesday, wednesday: d3.timeWednesday, thursday: d3.timeThursday, friday: d3.timeFriday, saturday: d3.timeSaturday, month: d3.timeMonth, year: d3.timeYear },
  utc: { millisecond: d3.utcMillisecond, second: d3.utcSecond, minute: d3.utcMinute, hour: d3.utcHour, day: d3.utcDay, unixDay: d3.unixDay, sunday: d3.utcSunday, monday: d3.utcMonday, tuesday: d3.utcTuesday, wednesday: d3.utcWednesday, thursday: d3.utcThursday, friday: d3.utcFriday, saturday: d3.utcSaturday, month: d3.utcMonth, year: d3.utcYear },
};
const num = (d) => +d;
// d3 loops forever when a filtered interval reaches the end of the range of
// the Date object: the date becomes invalid and no test passes. The cases for
// every, filter and ticks stay away from the ends.
const safe = (t) => Number.isNaN(t) || Math.abs(t) < 8.64e15 - 4e12;
const step = () => pick(rnd, [1, 1, 2, 3, -1, -2, 5, 7, 12, 0, 2.7, -2.7, int(rnd, -40, 40), NaN]);
const everyStep = (name) => pick(rnd, name === 'year' ? [1, 2, 5, 10, 100, 3, 0, -1, NaN, 2.5, Infinity]
  : name === 'millisecond' ? [1, 2, 5, 10, 100, 250, 1000, 0, NaN, 3.9]
  : [1, 2, 3, 4, 5, 6, 7, 10, 12, 15, 20, 30, 0, -3, NaN, 2.5, Infinity]);

const intervals = [];
for (const [family, set] of Object.entries(families)) {
  for (const [name, interval] of Object.entries(set)) {
    for (let i = 0; i < 110; i++) {
      const t = randomTime();
      const st = step();
      const t2 = rnd() < 0.5 ? randomTime() : t + (rnd() - 0.3) * durations[name] * pick(rnd, [3, 30, 300]);
      const rs = pick(rnd, [1, 1, 2, 3, 0, -1, 2.5, NaN]);
      const stop = t + rnd() * durations[name] * 12 * (rs > 1 ? rs : 1);
      const c = {
        fam: family, iv: name, t: hex(t), st: hex(st), t2: hex(t2), rs: hex(rs), stop: hex(stop),
        r: floats([num(interval.floor(t)), num(interval.ceil(t)), num(interval.round(t)), num(interval.offset(t, st)), interval.count(t, t2)]),
        range: floats(interval.range(t, stop, rs).map(num)),
      };
      // interval.every(k), and the same operations on the result.
      const k = everyStep(name);
      const ev = safe(t) ? interval.every(k) : null;
      c.k = hex(k);
      c.evSkip = !safe(t);
      if (ev === null) c.ev = null;
      else {
        const evStop = t + rnd() * durations[name] * 8 * (k > 1 && isFinite(k) ? Math.floor(k) : 1);
        c.evStop = hex(evStop);
        c.ev = floats([num(ev.floor(t)), num(ev.ceil(t)), num(ev.round(t)), num(ev.offset(t, pick(rnd, [1, -1, 2, st])))]);
        c.evSt = hex(0);
        c.evRange = floats(ev.range(t, evStop).map(num));
        c.evHasCount = typeof ev.count === 'function';
      }
      intervals.push(c);
    }
  }
}
// The offset step of the every result is recorded separately, because pick consumed it above.
// Regenerate deterministically: the Go test reads the step from evOff.
// (See the loop below, which fills evOff for every case with ev.)
{
  const rnd2 = prng(0x0ff5e7);
  for (const c of intervals) {
    if (c.ev === null) continue;
    const interval = families[c.fam][c.iv];
    const view = new DataView(new ArrayBuffer(8));
    const f = (h) => { view.setBigUint64(0, BigInt('0x' + h)); return view.getFloat64(0); };
    const ev = interval.every(f(c.k));
    const st = pick(rnd2, [1, -1, 2, 3, -4, 0, 1.5]);
    const t = f(c.t);
    c.evSt = hex(st);
    c.ev = floats([num(ev.floor(t)), num(ev.ceil(t)), num(ev.round(t)), num(ev.offset(t, st))]);
  }
}

// interval.filter with a test on a local field.
const filters = [];
for (let i = 0; i < 300; i++) {
  const family = pick(rnd, ['time', 'utc']);
  const name = pick(rnd, ['day', 'hour', 'month']);
  const interval = families[family][name];
  const modulus = pick(rnd, [2, 3, 5]);
  // The test is on the day of the month, or on the month for the month interval.
  const test = name === 'month'
    ? (family === 'time' ? (d) => d.getMonth() % modulus === 0 : (d) => d.getUTCMonth() % modulus === 0)
    : (family === 'time' ? (d) => d.getDate() % modulus === 0 : (d) => d.getUTCDate() % modulus === 0);
  const f = interval.filter(test);
  const t = randomTime();
  if (!safe(t)) continue;
  const st = pick(rnd, [1, 2, -1, -3, 0]);
  const stop = t + rnd() * durations[name] * 20;
  filters.push({ fam: family, iv: name, mod: modulus, t: hex(t), st: hex(st), stop: hex(stop), r: floats([num(f.floor(t)), num(f.ceil(t)), num(f.offset(t, st))]), range: floats(f.range(t, stop).map(num)) });
}

// timeTicks and utcTicks.
const ticks = [];
for (let i = 0; i < 2200; i++) {
  const start = randomTime();
  const k = rnd();
  const span = k < 0.1 ? int(rnd, 1, 5000) : k < 0.25 ? rnd() * 10 * M : k < 0.4 ? rnd() * 3 * D : k < 0.6 ? rnd() * 90 * D : k < 0.8 ? rnd() * 5 * 365 * D : k < 0.95 ? rnd() * 300 * 365 * D : 0;
  const stop = rnd() < 0.1 ? randomTime() : start + (rnd() < 0.15 ? -span : span);
  const count = pick(rnd, [10, 10, 10, 5, 2, 1, 20, 7, 3, 0, NaN, 2.5, int(rnd, 1, 40)]);
  if (!safe(start) || !safe(stop)) continue;
  let time, utc;
  try { time = d3.timeTicks(start, stop, count).map(num); utc = d3.utcTicks(start, stop, count).map(num); } catch { continue; }
  if (time.length > 60 || utc.length > 60) continue;
  // The interval that the tick computation selects, observed through its floor and offset.
  const ti = d3.timeTickInterval(start, stop, count), ui = d3.utcTickInterval(start, stop, count);
  let probe = randomTime();
  while (!safe(probe)) probe = randomTime();
  ticks.push({
    a: floats([start, stop, count]), time: floats(time), utc: floats(utc), probe: hex(probe),
    ti: ti ? floats([num(ti.floor(probe)), num(ti.offset(probe, 1))]) : null,
    ui: ui ? floats([num(ui.floor(probe)), num(ui.offset(probe, 1))]) : null,
  });
}
// Ticks with an interval in place of the count, as the gantt renderer uses them.
const intervalTicks = [];
for (let i = 0; i < 700; i++) {
  const family = pick(rnd, ['time', 'time', 'utc']);
  const name = pick(rnd, ['millisecond', 'second', 'minute', 'hour', 'day', 'monday', 'sunday', 'thursday', 'month']);
  const k = pick(rnd, [1, 1, 2, 3, 5, 7, 10, 15]);
  const interval = families[family][name].every(k);
  const start = randomTime();
  if (!safe(start)) continue;
  const stop = start + (rnd() < 0.1 ? -1 : 1) * rnd() * durations[name] * k * 14;
  const r = (family === 'time' ? d3.timeTicks : d3.utcTicks)(start, stop, interval).map(num);
  intervalTicks.push({ fam: family, iv: name, k, a: floats([start, stop]), r: floats(r) });
}

writeGolden(`internal/d3/time/testdata/time_${tzFileSuffix()}.json.gz`, { tz: process.env.TZ || 'UTC', math: mathTable(), groups: { intervals, filters, ticks, intervalTicks } });
