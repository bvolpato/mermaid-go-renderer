// Emits golden cases for internal/js/date_gantt.go from V8: Date.parse.
//
// Run it once for each time zone:
//   TZ=UTC node gen-gantt-jsdate.mjs
//   TZ=America/New_York node gen-gantt-jsdate.mjs
import { prng, hex, writeGolden, pick, int } from './d3-golden-lib.mjs';

const tz = process.env.TZ || 'UTC';
const rnd = prng(0x9a277);

const pad = (n, w) => String(n).padStart(w, '0');
const months = ['Jan', 'Feb', 'Mar', 'Apr', 'May', 'Jun', 'Jul', 'Aug', 'Sep', 'Oct', 'Nov', 'Dec'];
const longMonths = ['January', 'February', 'March', 'April', 'May', 'June', 'July', 'August', 'September', 'October', 'November', 'December'];
const days = ['Sun', 'Mon', 'Tue', 'Wed', 'Thu', 'Fri', 'Sat'];
const zones = ['Z', 'UTC', 'GMT', 'UT', 'EST', 'EDT', 'CST', 'CDT', 'MST', 'MDT', 'PST', 'PDT', 'z', 'utc', 'gmt', 'est', 'CET', 'XYZ'];

const year = () => pick(rnd, [int(rnd, 1990, 2035), int(rnd, 1990, 2035), int(rnd, 0, 99), int(rnd, 100, 9999), int(rnd, 1850, 2100)]);
const month = () => (rnd() < 0.9 ? int(rnd, 1, 12) : int(rnd, 0, 20));
const day = () => (rnd() < 0.9 ? int(rnd, 1, 28) : int(rnd, 0, 40));
const hour = () => (rnd() < 0.9 ? int(rnd, 0, 23) : int(rnd, 0, 30));
const minute = () => (rnd() < 0.92 ? int(rnd, 0, 59) : int(rnd, 0, 99));

function isoString() {
  let s = '';
  const k = rnd();
  if (k < 0.08) s += pick(rnd, ['+', '-']) + pad(int(rnd, 0, 300000), 6);
  else if (k < 0.12) s += pad(year(), int(rnd, 1, 6));
  else s += pad(year(), 4);
  if (rnd() < 0.9) {
    s += '-' + pad(month(), rnd() < 0.95 ? 2 : 1);
    if (rnd() < 0.9) s += '-' + pad(day(), rnd() < 0.93 ? 2 : 1);
  }
  if (rnd() < 0.6) {
    s += pick(rnd, ['T', 'T', 'T', 't', ' ', 'T ']);
    s += pad(hour(), rnd() < 0.95 ? 2 : 1) + ':' + pad(minute(), rnd() < 0.95 ? 2 : 1);
    if (rnd() < 0.6) {
      s += ':' + pad(minute(), 2);
      if (rnd() < 0.5) s += '.' + pad(int(rnd, 0, 10 ** int(rnd, 1, 11) - 1), int(rnd, 1, 11));
    }
    const z = rnd();
    if (z < 0.25) s += 'Z';
    else if (z < 0.45) s += pick(rnd, ['+', '-']) + pad(hour(), 2) + ':' + pad(minute(), 2);
    else if (z < 0.55) s += pick(rnd, ['+', '-']) + pad(hour(), 2) + pad(minute(), 2);
    else if (z < 0.6) s += pick(rnd, ['+', '-']) + pad(hour(), 2);
    else if (z < 0.65) s += ' ' + pick(rnd, zones);
  }
  return s;
}

function timePart() {
  let s = String(hour()) + ':' + pad(minute(), 2);
  const k = rnd();
  if (k < 0.5) s += ':' + pad(minute(), 2);
  if (k < 0.15) s += '.' + pad(int(rnd, 0, 999), int(rnd, 1, 5));
  if (rnd() < 0.3) s += pick(rnd, [' PM', ' AM', 'pm', 'am', ' p.m.', ' PM ']);
  return s;
}

function zonePart() {
  const k = rnd();
  if (k < 0.3) return ' ' + pick(rnd, zones);
  if (k < 0.5) return ' GMT' + pick(rnd, ['+', '-']) + pad(int(rnd, 0, 14), 2) + pad(pick(rnd, [0, 0, 30, 45]), 2);
  if (k < 0.6) return ' ' + pick(rnd, ['+', '-']) + pad(int(rnd, 0, 14), 2) + ':' + pad(pick(rnd, [0, 30, 45, 75]), 2);
  if (k < 0.7) return ' UTC' + pick(rnd, ['+', '-']) + int(rnd, 0, 14);
  if (k < 0.75) return pick(rnd, ['+', '-']) + pad(int(rnd, 0, 99999), int(rnd, 1, 5));
  if (k < 0.8) return ' (Central European Standard Time)';
  return '';
}

function legacyString() {
  const k = rnd();
  const y = year(), m = month(), d = day();
  const mn = months[(m + 11) % 12], lm = longMonths[(m + 11) % 12];
  if (k < 0.1) return `${m}/${d}/${y}`;
  if (k < 0.2) return `${y}/${m}/${d}`;
  if (k < 0.3) return `${mn} ${d}, ${y} ${timePart()}${zonePart()}`;
  if (k < 0.4) return `${d} ${lm} ${y}`;
  if (k < 0.5) return `${pick(rnd, days)} ${mn} ${pad(d, 2)} ${y} ${timePart()}${zonePart()}`;
  if (k < 0.6) return `${pick(rnd, days)}, ${pad(d, 2)} ${mn} ${y} ${timePart()} GMT`;
  if (k < 0.7) return `${y}-${m}-${d}`;
  if (k < 0.75) return `${y}-${pad(m, 2)}-${pad(d, 2)} ${timePart()}${zonePart()}`;
  if (k < 0.8) return `${d}-${mn}-${y}`;
  if (k < 0.85) return `${mn}-${d}-${y} ${timePart()}`;
  if (k < 0.9) return `${lm} ${y}`;
  if (k < 0.95) return `${m}-${d}-${y}`;
  return `${timePart()} ${m}/${d}/${y}`;
}

const vocab = [
  ...months, ...longMonths.slice(0, 4), ...zones, 'T', 't', 'AM', 'PM', 'am', 'pm', 'after', 'until', 'foo', 'x', 'd', 'h', 'ms', 'Janx', 'utcx', 'Ma',
  ' ', ' ', ' ', '  ', '\t', '\n', ' ', '　', '﻿', ' ', ',', '/', '-', '-', '+', '+', ':', ':', '::', '.', '(', ')', '(a)', '((b) c)', '()', ';', '#', '_', 'é', 'ß', '\u0000',
];
function number() {
  const k = rnd();
  if (k < 0.3) return String(int(rnd, 0, 31));
  if (k < 0.5) return pad(int(rnd, 0, 99), 2);
  if (k < 0.7) return String(int(rnd, 1900, 2100));
  if (k < 0.8) return pad(int(rnd, 0, 999), 3);
  if (k < 0.9) return pad(int(rnd, 0, 99999), int(rnd, 1, 6));
  return pad(int(rnd, 0, 10 ** 12), int(rnd, 6, 13));
}
function soup() {
  const n = int(rnd, 1, 9);
  let s = '';
  for (let i = 0; i < n; i++) s += rnd() < 0.45 ? number() : pick(rnd, vocab);
  return s;
}

function mutate(s) {
  const k = rnd();
  const i = int(rnd, 0, s.length);
  if (k < 0.3) return s.slice(0, i) + pick(rnd, vocab) + s.slice(i);
  if (k < 0.6) return s.slice(0, i) + s.slice(i + 1);
  if (k < 0.8) return s.slice(0, i) + number() + s.slice(i);
  return s.slice(0, i) + pick(rnd, ['T', ' ', '-', ':', '.', 'Z', '+']) + s.slice(i + 1);
}

// Strings as gantt diagrams have them in a date position.
const fixed = [
  '', ' ', '2014-01-06', '2019-09-1', '2019-9-01', '202-12-01', '2024-12-01', '202304', '06', '00', '1', '12', '24', '2022', '17:49', '00:00', '18:00:00',
  '3d', '24h', '1w', 'after a', 'until b', '2014-01-06 12:00', '2014-01-06T12:00', '2014-01-06T24:00', '2014-01-06T24:00:00.000', '2014-01-06T24:00:01',
  '2014-01-06T12:00Z', '2014-01-06T12:00:00+01:00', '2014-01-06T12:00:00+0100', '2014-01-06T12:00:00+01', '2014-02-30', '2014-02-31', '2014-13-01', '2014-00-10',
  '+002014-01-06', '-000001-01-01', '-000000-01-01', '+275760-09-13T00:00:00.000Z', '+275760-09-13T00:00:00.001Z', '-271821-04-20T00:00:00.000Z', '-271821-04-19T23:59:59.999Z',
  '1/2/3', '1/2', '13/1/2020', '32/1/2020', '2020/13/1', '0/1/2020', 'Jan', 'Jan 5', '5 Jan', 'Jan 2020', 'Jan 32', 'January 5, 49', 'January 5, 50', 'January 5, 99', 'January 5, 100',
  '12:00', '12:00 PM', '12:00 AM', '13:00 PM', '0:00 PM', '1:2:3.4', '1:2:3.4567', '10::30', '10:30:', '10:', ':10', '10:30.5', '10.30', '2020 10.30', 'Jan 1 2020 10.30.20',
  'Thu, 01 Jan 1970 00:00:00 GMT', 'Thu Jan 01 1970 00:00:00 GMT+0000 (Coordinated Universal Time)', 'Mon Jan 15 2026 12:00:00 GMT-0500 (Eastern Standard Time)',
  '2020-01-01 (comment) 10:00', '(2020-01-01)', '2020-01-01)', 'x2020-01-01', 'x 2020-01-01', 'foo bar 2020-01-01', '2020-01-01 foo', '2020-01-01x',
  '2020-01-01 10:00 +', '2020-01-01 10:00 -5', '2020-01-01 10:00 +530', '2020-01-01 10:00 +05:', '2020-01-01 10:00 +05:30', '2020-01-01 10:00 +12345',
  '2020-01-01 UTC', '2020-01-01 UTC+2', '2020-01-01 Z', '2020-01-01Z', '2020-01-01T', '2020-01-01T1', '2020-01-01T10', '2020-01-01T10:0', '2020-01-01 T10:00',
  '1e3', '0x10', '１２', '2020‐01‐01', '2020-01-01\u000010:00', '2020-01-01 10:00', '2020-01-01 10:00', '99999999999999', '0000-01-01', '0001-01-01', '0099-12-31',
];

const cases = new Map();
function add(s) {
  // V8 with pointer compression (Chromium) and without it (Node.js) accept
  // different ranges for an offset of 298262 hours and more. Skip these.
  if (/[+-]\d{6,}:/.test(s)) return;
  if (!cases.has(s)) cases.set(s, Date.parse(s));
}
for (const s of fixed) add(s);
for (let i = 0; i < 6000; i++) add(isoString());
for (let i = 0; i < 6000; i++) add(legacyString());
for (let i = 0; i < 6000; i++) add(soup());
for (let i = 0; i < 3000; i++) add(mutate(isoString()));
for (let i = 0; i < 3000; i++) add(mutate(legacyString()));
for (let i = 0; i < 1500; i++) add(mutate(mutate(pick(rnd, fixed))));

const parse = [...cases].map(([s, t]) => ({ s, t: hex(t) }));
const valid = parse.filter((c) => c.t !== hex(NaN)).length;
const file = tz === 'UTC' ? 'gantt_dateparse_utc.json.gz' : 'gantt_dateparse_' + tz.replace(/.*\//, '').toLowerCase() + '.json.gz';
writeGolden('internal/js/testdata/' + file, { tz, valid, groups: { parse } });
console.log(`valid: ${valid}`);
