// Emits golden cases for internal/dayjs from the real dayjs with the plugins
// that the gantt diagram of mermaid loads.
//
// Run it once for each time zone:
//   TZ=UTC node gen-dayjs.mjs
//   TZ=America/New_York node gen-dayjs.mjs
import { prng, hex, writeGolden, pick, int } from './d3-golden-lib.mjs';

// The clock of the reference browser. dayjs reads it with new Date().
const FIXED_TIME = Date.parse('2026-01-15T12:00:00Z');
const RealDate = Date;
class FixedDate extends RealDate {
  constructor(...args) {
    if (args.length === 0) super(FIXED_TIME);
    else super(...args);
  }
  static now() {
    return FIXED_TIME;
  }
}
globalThis.Date = FixedDate;

const { default: dayjs } = await import('dayjs');
const { default: isoWeek } = await import('dayjs/plugin/isoWeek.js');
const { default: customParseFormat } = await import('dayjs/plugin/customParseFormat.js');
const { default: advancedFormat } = await import('dayjs/plugin/advancedFormat.js');
const { default: duration } = await import('dayjs/plugin/duration.js');
dayjs.extend(isoWeek);
dayjs.extend(customParseFormat);
dayjs.extend(advancedFormat);
dayjs.extend(duration);

const tz = process.env.TZ || 'UTC';
const rnd = prng(0xda7715);
const DAY = 864e5;

// Local time changes: the instants at which the offset changes.
const transitions = [];
for (let t = Date.UTC(1900, 0, 1), prev = new Date(t).getTimezoneOffset(); t < Date.UTC(2040, 0, 1); t += DAY) {
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
  if (transitions.length && k < 0.2) return pick(rnd, transitions) + int(rnd, -30 * 3600e3, 30 * 3600e3);
  if (k < 0.6) return Date.UTC(int(rnd, 2010, 2030), int(rnd, 0, 11), int(rnd, 1, 31), int(rnd, 0, 23), int(rnd, 0, 59), int(rnd, 0, 59), int(rnd, 0, 999));
  if (k < 0.75) return Date.UTC(int(rnd, 1900, 2100), int(rnd, 0, 11), int(rnd, 1, 31));
  if (k < 0.8) return Date.UTC(int(rnd, 1990, 2030), int(rnd, 0, 11), pick(rnd, [28, 29, 30, 31, 1]), pick(rnd, [0, 0, 12, 23]), pick(rnd, [0, 0, 59]), pick(rnd, [0, 59]));
  if (k < 0.85) return new Date(Date.UTC(2000, 0, 1)).setUTCFullYear(int(rnd, -50, 150), int(rnd, 0, 11), int(rnd, 1, 28)) + int(rnd, 0, DAY - 1);
  if (k < 0.9) return Math.floor((rnd() * 2 - 1) * 4000 * 365 * DAY);
  if (k < 0.93) return Math.floor((rnd() * 2 - 1) * 8.64e15);
  if (k < 0.96) return pick(rnd, [0, -1, 1, 8.64e15, -8.64e15, FIXED_TIME, 951782400000, 1709164800000]);
  if (k < 0.98) return NaN;
  return int(rnd, 0, 4e12);
}
const usualTime = () => Date.UTC(int(rnd, 1995, 2035), int(rnd, 0, 11), int(rnd, 1, 31), int(rnd, 0, 23), int(rnd, 0, 59), int(rnd, 0, 59), pick(rnd, [0, 0, int(rnd, 0, 999)]));

const formats = [
  'YYYY-MM-DD', 'YYYY-MM-DD', 'YYYY-MM-DD HH:mm', 'YYYY-MM-DD HH:mm:ss', 'YYYY-MM-DD HH:mm:ss.SSS', 'DD/MM/YYYY', 'MM-DD-YYYY', 'M/D/YYYY', 'YYYY.MM.DD', 'YYYYMMDD',
  'HH:mm', 'HH:mm:ss', 'H:m:s', 'hh:mm a', 'h:mm A', 'ss', 'mm', 's', 'D', 'DD', 'M', 'YY-M-D', 'YY', 'YYYY', 'Y', 'MMM D, YYYY', 'MMMM Do YYYY', 'Do MMM', 'Do',
  'dddd, MMMM D YYYY h:mm A', 'ddd D', 'dd', 'd', 'YYYY-MM-DDTHH:mm:ssZ', 'YYYY-MM-DDTHH:mm:ss.SSSZZ', 'YYYY-MM-DD[T]HH', 'Z', 'ZZ', 'x', 'X', 'Q YYYY', 'Q',
  'YYYY [week] W', 'GGGG-WW', 'W', 'k', 'kk', 'SSS', 'SS', 'S', 'HH:mm:ss.S', '[literal] YYYY', 'DD MMM YY', 'HH[h]mm', '[YYYY]', '[]YYYY', '[a]b]YYYY', '[a[b]YYYY',
  'YYY', 'YYYYY', 'MMMMM', 'DDD', 'hhh', 'mmm', 'sss', 'SSSS', 'ZZZ', 'gggg', 'w', 'ww', 'wo', 'z', 'zzz', 'L', 'LT', 'LTS', 'll', 'YYYY-MM-DD l', '[L] YYYY', '',
  ' YYYY-MM-DD ', 'YYYY年MM月DD日', 'YYYY-MM-DD dddd', 'MMMM', 'MMM', 'A', 'a', 'hh A', 'YYYY-MM', 'MM/YY', 'DD.MM.YYYY HH:mm', 'D MMMM YYYY', 'YYYY/M/D H:m', 'YYYY-DDD',
  'X.SSS', 'x x', 'xX', 'YYYY-MM-DD HH:mm:ss Z', 'YYYY-MM-DDTHH:mm:ss[Z]', 'e', '###', '-', ' ', 'YYYY-MM-DD\nHH', 'Do [of] MMMM', 'dddd', 'M-D', 'H', 'h', 'm',
];
const tokens = ['YYYY', 'YY', 'Y', 'M', 'MM', 'MMM', 'MMMM', 'D', 'DD', 'Do', 'd', 'dd', 'ddd', 'dddd', 'H', 'HH', 'h', 'hh', 'a', 'A', 'm', 'mm', 's', 'ss', 'S', 'SS', 'SSS',
  'Z', 'ZZ', 'Q', 'W', 'WW', 'GGGG', 'k', 'kk', 'X', 'x', '[at]', '[T]'];
const seps = ['-', '/', ' ', ':', '.', ',', '', '', 'T', '_', '(', ')', '  '];
function randomFormat() {
  const n = int(rnd, 1, 6);
  let s = '';
  for (let i = 0; i < n; i++) s += (i ? pick(rnd, seps) : '') + pick(rnd, tokens);
  return s;
}
const anyFormat = () => (rnd() < 0.7 ? pick(rnd, formats) : randomFormat());

function tryFormat(d, f) {
  try {
    return { s: d.format(f) };
  } catch {
    return { err: true };
  }
}

// format: [time, format] -> string or error.
const format = [];
for (let i = 0; i < 9000; i++) {
  const t = rnd() < 0.5 ? usualTime() : randomTime();
  const f = anyFormat();
  format.push({ t: hex(t), f, ...tryFormat(dayjs(new Date(t)), f) });
}

const junk = ['', ' ', '3d', '24h', 'after a', 'until b c', 'abc', '2014-01-06', '2019-09-1', '202-12-01', '06', '00', '1', '17:49', '2014-01-06 12:00', '12/31/2020',
  'Invalid Date', '0', '-1', '1e3', '1700000000000', '1700000000', ' 1700000000000', '12abc', 'PM', '12 PM', 'January 5th 2020', 'Jan 5, 2020', '5th', '15st', '0th', '31st', '32nd',
  '+05:30', 'Z', '2020-01-01T10:00:00+05:30', '2020-01-01T10:00:00Z', '2020-W05', '１２', '2020‐01‐01', '99', '9999-12-31', '0000-01-01', '0099-01-01', '2020-00-10', '2020-13-10', '2020-02-30'];
const digitsOrSeps = '0123456789-/: .,';
function mutate(s) {
  if (s.length === 0) return pick(rnd, junk);
  const i = int(rnd, 0, s.length - 1);
  const k = rnd();
  if (k < 0.3) return s.slice(0, i) + s.slice(i + 1);
  if (k < 0.6) return s.slice(0, i) + pick(rnd, [...digitsOrSeps]) + s.slice(i + 1);
  if (k < 0.8) return s.slice(0, i) + pick(rnd, [...digitsOrSeps, 'a', 'T', 'Z', '+']) + s.slice(i);
  if (k < 0.9) return ' ' + s;
  return s + pick(rnd, [' ', '0', 'x', 'Z']);
}

function parseCase(input, f, strict) {
  const c = { i: input, f, strict };
  try {
    const d = strict ? dayjs(input, f, true) : dayjs(input, f);
    c.t = hex(d.valueOf());
    // The fields that a strict failure leaves behind are observable.
    c.y = hex(d.$y);
  } catch {
    c.err = true;
  }
  return c;
}

// parse: [input, format, strict] -> time value or error.
const parse = [];
for (let i = 0; i < 12000; i++) {
  const f = anyFormat();
  const k = rnd();
  let input;
  if (k < 0.1) input = pick(rnd, junk);
  else {
    const r = tryFormat(dayjs(new Date(rnd() < 0.8 ? usualTime() : randomTime())), f);
    input = r.err ? pick(rnd, junk) : r.s;
    if (k < 0.4) input = mutate(input);
    if (k < 0.15) input = mutate(input);
  }
  parse.push(parseCase(input, f, rnd() < 0.8));
}
for (const input of junk) {
  for (const f of ['YYYY-MM-DD', 'x', 'X', '', 'ss', 'HH:mm', 'Do', 'YYYY-MM-DD HH:mm:ss']) {
    parse.push(parseCase(input, f, true));
    parse.push(parseCase(input, f, false));
  }
}

// fromString: dayjs(string) without a format.
const fromString = [];
for (let i = 0; i < 2500; i++) {
  const k = rnd();
  let input;
  if (k < 0.1) input = pick(rnd, junk);
  else {
    const f = pick(rnd, ['YYYY-MM-DD', 'YYYY-MM-DD HH:mm:ss', 'YYYY-MM-DDTHH:mm:ss', 'YYYY-MM-DDTHH:mm:ss.SSS', 'YYYY/MM/DD', 'YYYYMMDD', 'YYYY-M-D', 'YYYY-MM-DD HH:mm', 'YYYY-MM-DDTHH:mm:ssZ',
      'YYYY-MM-DDTHH:mm:ss.SSS[Z]', 'YYYY', 'YYYY-MM', 'MMM D, YYYY', 'YYYY-MM-DD HH', 'YYYYMMDDHHmmss', 'YYYY-MM-DD HH:mm:ss.S', 'YYYY-MM-DD HH:mm:ss:SSS']);
    input = dayjs(new Date(rnd() < 0.8 ? usualTime() : randomTime())).format(f);
    if (k < 0.4) input = mutate(input);
  }
  fromString.push({ i: input, t: hex(dayjs(input).valueOf()) });
}

const units = ['ms', 'millisecond', 'milliseconds', 's', 'second', 'seconds', 'm', 'minute', 'h', 'hour', 'hours', 'd', 'day', 'days', 'D', 'date', 'w', 'week', 'weeks',
  'M', 'month', 'months', 'Q', 'quarter', 'y', 'year', 'years', 'isoWeek', 'foo', '', 'Day', 'YEARS'];
const amount = () => {
  const k = rnd();
  if (k < 0.5) return int(rnd, -40, 40);
  if (k < 0.65) return pick(rnd, [0.5, 1.5, 2.5, -0.5, -1.5, 2.7, 0.1, 1.49, 3.999]);
  if (k < 0.8) return int(rnd, -500, 500);
  if (k < 0.9) return pick(rnd, [10000, -10000, 365, 1000000, 99999999, 1e12, -1e12]);
  if (k < 0.93) return NaN;
  return (rnd() * 2 - 1) * 100;
};

const ops = [];
for (let i = 0; i < 16000; i++) {
  const t = rnd() < 0.5 ? usualTime() : randomTime();
  const d = dayjs(new Date(t));
  const u = pick(rnd, units);
  const k = rnd();
  if (k < 0.25) {
    const n = amount();
    ops.push({ op: 'add', t: hex(t), u, n: hex(n), r: hex(d.add(n, u).valueOf()) });
  } else if (k < 0.32) {
    const n = amount();
    ops.push({ op: 'subtract', t: hex(t), u, n: hex(n), r: hex(d.subtract(n, u).valueOf()) });
  } else if (k < 0.45) {
    ops.push({ op: 'startOf', t: hex(t), u, r: hex(d.startOf(u).valueOf()) });
  } else if (k < 0.58) {
    ops.push({ op: 'endOf', t: hex(t), u, r: hex(d.endOf(u).valueOf()) });
  } else if (k < 0.7) {
    const n = pick(rnd, [int(rnd, 0, 60), int(rnd, -100, 3000), amount()]);
    ops.push({ op: 'set', t: hex(t), u, n: hex(n), r: hex(d.set(u, n).valueOf()) });
  } else if (k < 0.85) {
    const t2 = rnd() < 0.7 ? usualTime() : randomTime();
    const fl = rnd() < 0.5;
    ops.push({ op: 'diff', t: hex(t), t2: hex(t2), u, fl, r: hex(d.diff(dayjs(new Date(t2)), u, fl)) });
  } else if (k < 0.93) {
    const t2 = rnd() < 0.5 ? t + int(rnd, -40 * DAY, 40 * DAY) : usualTime();
    const o = dayjs(new Date(t2));
    ops.push({ op: 'cmp', t: hex(t), t2: hex(t2), u, r: [d.isSame(o, u), d.isBefore(o, u), d.isAfter(o, u)] });
  } else if (k < 0.97) {
    const n = int(rnd, -3, 12);
    ops.push({ op: 'isoWeekday', t: hex(t), n: hex(n), r: hex(d.isoWeekday(n).valueOf()) });
  } else {
    const n = int(rnd, 1, 53);
    ops.push({ op: 'isoWeek', t: hex(t), n: hex(n), r: hex(d.isoWeek(n).valueOf()) });
  }
}

// info: the getters of one date.
const info = [];
for (let i = 0; i < 3500; i++) {
  const t = rnd() < 0.5 ? usualTime() : randomTime();
  const d = dayjs(new Date(t));
  info.push({
    t: hex(t),
    r: [d.year(), d.month(), d.date(), d.day(), d.hour(), d.minute(), d.second(), d.millisecond(), d.isoWeekday(), d.daysInMonth(), d.utcOffset(), d.unix(),
      d.isoWeek(), d.isoWeekYear(), d.valueOf(), d.toDate().getTime()].map(hex),
    valid: d.isValid(),
  });
}
// now: dayjs() reads the clock.
const now = hex(dayjs().valueOf());

// walk: the loop of the gantt code, one day at a time.
const walk = [];
for (let i = 0; i < 150; i++) {
  const t = rnd() < 0.5 && transitions.length ? pick(rnd, transitions) + int(rnd, -5 * DAY, 5 * DAY) : usualTime();
  const n = pick(rnd, [1, 1, 1, 7, -1]);
  const u = pick(rnd, ['d', 'd', 'd', 'w', 'M', 'h']);
  let d = dayjs(new Date(t));
  const r = [];
  for (let j = 0; j < 40; j++) {
    d = d.add(n, u);
    r.push(hex(d.valueOf()));
  }
  walk.push({ t: hex(t), n: hex(n), u, r });
}

// durations.
const durationUnits = ['milliseconds', 'seconds', 'minutes', 'hours', 'days', 'weeks', 'months', 'years', 'millisecond', 'second', 'minute', 'hour', 'day', 'week', 'month', 'year',
  'ms', 's', 'm', 'h', 'd', 'w', 'M', 'y', 'Q', 'foo'];
function durationResult(d) {
  return {
    ms: hex(d.asMilliseconds()),
    as: durationUnits.map((u) => hex(d.as(u))),
    get: durationUnits.map((u) => hex(d.get(u))),
    iso: d.toISOString(),
    fmt: [d.format(), d.format('Y-M-D H:m:s.SSS'), d.format('YYYY YY [x] MM DD HH mm ss YYY')],
  };
}
const durations = [];
for (let i = 0; i < 1000; i++) {
  const k = rnd();
  let c;
  let d;
  if (k < 0.3) {
    const ms = pick(rnd, [int(rnd, -1e9, 1e9), int(rnd, 0, 1e12), (rnd() - 0.5) * 1e10, int(rnd, 0, 5000), 0, NaN]);
    c = { kind: 'ms', ms: hex(ms) };
    d = dayjs.duration(ms);
  } else if (k < 0.55) {
    const v = pick(rnd, [int(rnd, 1, 100), int(rnd, -50, 50), rnd() * 20, 0, 1]);
    const u = pick(rnd, durationUnits);
    c = { kind: 'unit', v: hex(v), u };
    d = dayjs.duration(v, u);
  } else if (k < 0.85) {
    const obj = {};
    const keys = [];
    const n = int(rnd, 1, 4);
    for (let j = 0; j < n; j++) {
      const u = pick(rnd, durationUnits);
      if (u in obj) continue;
      obj[u] = pick(rnd, [int(rnd, 1, 100), int(rnd, -50, 50), rnd() * 20, 0]);
      keys.push([u, hex(obj[u])]);
    }
    c = { kind: 'obj', keys };
    d = dayjs.duration(obj);
  } else {
    const parts = [['Y', 0.3], ['M', 0.3], ['W', 0.2], ['D', 0.5]].map(([s, p]) => (rnd() < p ? pick(rnd, [int(rnd, 0, 40), '1.5', '-2', '+3', '']) + s : '')).join('');
    const time = [['H', 0.5], ['M', 0.5], ['S', 0.5]].map(([s, p]) => (rnd() < p ? pick(rnd, [int(rnd, 0, 99), '1.5', '0.25', '-7']) + s : '')).join('');
    const s = pick(rnd, ['', '', '-', '+']) + 'P' + parts + (time || rnd() < 0.2 ? 'T' + time : '');
    c = { kind: 'iso', s: rnd() < 0.05 ? pick(rnd, ['', 'P', 'PT', 'x', '1d', 'P1', 'p1d']) : s };
    d = dayjs.duration(c.s);
  }
  c.r = durationResult(d);
  if (rnd() < 0.3) {
    const v = int(rnd, -100, 100);
    const u = pick(rnd, durationUnits.slice(0, 8));
    const sub = rnd() < 0.5;
    c.add = { v: hex(v), u, sub, r: durationResult(sub ? d.subtract(v, u) : d.add(v, u)) };
  }
  if (rnd() < 0.2) {
    const t = usualTime();
    const sub = rnd() < 0.5;
    const base = dayjs(new Date(t));
    c.apply = { t: hex(t), sub, r: hex((sub ? base.subtract(d) : base.add(d)).valueOf()) };
  }
  durations.push(c);
}

const file = 'dayjs_' + (tz === 'UTC' ? 'utc' : tz.replace(/.*\//, '').toLowerCase()) + '.json.gz';
writeGolden('internal/dayjs/testdata/' + file, { tz, now, groups: { format, parse, fromString, ops, info, walk, durations } });
console.log('format errors:', format.filter((c) => c.err).length, 'parse errors:', parse.filter((c) => c.err).length,
  'parse valid:', parse.filter((c) => !c.err && c.t !== hex(NaN)).length, 'fromString valid:', fromString.filter((c) => c.t !== hex(NaN)).length);
