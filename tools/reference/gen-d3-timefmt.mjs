// Emits golden cases for internal/d3/timefmt from d3-time-format.
//
// Run it once for each time zone:
//   TZ=UTC node gen-d3-timefmt.mjs
//   TZ=America/New_York node gen-d3-timefmt.mjs
import { prng, hex, floats, writeGolden, mathTable, pick, int } from './d3-golden-lib.mjs';
import { dateSource, tzFileSuffix } from './d3-golden-dates.mjs';
import { timeFormat, timeParse, utcFormat, utcParse, timeFormatLocale } from 'd3-time-format';

const rnd = prng(0x7f37);
const randomTime = dateSource(rnd, { nan: 0.02, far: 0.03 });
const directives = 'aAbBcdefgGHIjLmMpqQsSuUVwWxXyYZ%'.split('');
const literals = ['-', '/', ':', ' ', ', ', 'T', '.', 'Week ', ' of ', 'é', '日', '😀', 'Z', '%%', 'k', '(', ')'];

function specifier() {
  let s = '';
  const n = int(rnd, 1, 6);
  for (let i = 0; i < n; i++) {
    const k = rnd();
    if (k < 0.7) s += '%' + pick(rnd, ['', '', '', '-', '_', '0']) + pick(rnd, directives);
    else if (k < 0.95) s += pick(rnd, literals);
    else s += pick(rnd, ['%k', '%E', '%', '%-', '%_', '%é', '%😀', '%1']);
  }
  return s;
}
const common = ['%Y-%m-%d', '%d', '%H:%M', '%b %d', '%a %d', '%B', '%I:%M', '%I %p', '.%L', ':%S', '%Y', '%m/%d', '%Y-%m-%d %H:%M', '%x', '%X', '%c',
  '%Y-%m-%dT%H:%M:%S.%LZ', '%e %b %Y', '%A, %B %-d, %Y', '%j', '%U', '%W', '%V', '%G-W%V-%u', '%y%m%d', '%s', '%Q', '%f', '%Z', '%q', '%-I:%M %p', '%_d', '%H:%M:%S.%L'];

const formats = [];
for (let i = 0; i < 5000; i++) {
  const spec = i < common.length * 12 ? common[i % common.length] : specifier();
  const t = [randomTime(), randomTime()];
  const f = timeFormat(spec), u = utcFormat(spec);
  formats.push({ s: spec, t: floats(t), time: t.map((x) => f(x)), utc: t.map((x) => u(x)) });
}

// Parsing: text that a format made (which mostly parses back), and changed text.
const parseSpecs = ['%Y-%m-%d', '%Y-%m-%dT%H:%M:%S.%LZ', '%m/%d/%Y', '%d %b %Y', '%A, %B %d, %Y', '%H:%M', '%I:%M %p', '%Y-%m-%d %H:%M:%S', '%x', '%X', '%c',
  '%y-%m-%d', '%j %Y', '%Y %U %w', '%Y %W %u', '%G-W%V-%u', '%Y-W%V', '%s', '%Q', '%s.%L', '%Y-%m-%d %Z', '%Y-%m-%dT%H:%M:%S%Z', '%B %Y', '%b %e', '%Y q%q',
  '%H:%M:%S.%f', '%e/%m/%Y', '%a %d %b', '%p %I', '%d%%', '%-m/%-d/%y', '%Y%m%d', '%Y %U', '%Y %W', '%Y %V %a', '%Y %U %A', '%y', '%Y', '%d'];
const mutate = (s) => {
  const k = rnd();
  if (k < 0.5 || s.length === 0) return s;
  const i = int(rnd, 0, s.length - 1);
  if (k < 0.6) return s.slice(0, i);
  if (k < 0.68) return s + pick(rnd, [' ', 'x', '0', 'Z']);
  if (k < 0.76) return s.slice(0, i) + ' ' + s.slice(i);
  if (k < 0.82) return s.toLowerCase();
  if (k < 0.88) return s.toUpperCase();
  if (k < 0.94) return s.slice(0, i) + pick(rnd, ['9', '0', '-', '+05', ':', 'a', '99', '']) + s.slice(i + 1);
  return pick(rnd, ['', ' ', '2020', 'ab+05:30', '+0530', 'Z', '12/31/1999', 'Monday', 'jan 5', '1e3', '٣']);
};
const und = (d) => (d === null ? null : hex(+d));
const parses = [];
for (let i = 0; i < 5000; i++) {
  const spec = rnd() < 0.8 ? pick(rnd, parseSpecs) : specifier();
  const t = randomTime();
  const text = mutate(rnd() < 0.5 ? timeFormat(spec)(t) : utcFormat(spec)(t));
  parses.push({ s: spec, text, time: und(timeParse(spec)(text)), utc: und(utcParse(spec)(text)) });
}

// A locale with other names and formats.
const def = {
  dateTime: '%A %e %B %Y à %X', date: '%d/%m/%Y', time: '%H:%M:%S', periods: ['AM', 'PM'],
  days: ['dimanche', 'lundi', 'mardi', 'mercredi', 'jeudi', 'vendredi', 'samedi'], shortDays: ['dim.', 'lun.', 'mar.', 'mer.', 'jeu.', 'ven.', 'sam.'],
  months: ['janvier', 'février', 'mars', 'avril', 'mai', 'juin', 'juillet', 'août', 'septembre', 'octobre', 'novembre', 'décembre'],
  shortMonths: ['janv.', 'févr.', 'mars', 'avr.', 'mai', 'juin', 'juil.', 'août', 'sept.', 'oct.', 'nov.', 'déc.'],
};
const fr = timeFormatLocale(def);
const locales = [];
for (let i = 0; i < 1200; i++) {
  const spec = pick(rnd, ['%c', '%x', '%X', '%A %d %B', '%a %e %b %Y', '%B', '%b', specifier()]);
  const t = randomTime();
  const text = fr.format(spec)(t);
  const changed = mutate(text);
  locales.push({ s: spec, t: hex(t), time: text, utc: fr.utcFormat(spec)(t), text: changed, ptime: und(fr.parse(spec)(changed)), putc: und(fr.utcParse(spec)(changed)) });
}

writeGolden(`internal/d3/timefmt/testdata/timefmt_${tzFileSuffix()}.json.gz`, { tz: process.env.TZ || 'UTC', math: mathTable(), groups: { formats, parses, locales } });
