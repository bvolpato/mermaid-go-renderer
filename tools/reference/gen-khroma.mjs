// Emits golden cases for internal/khroma from the real khroma in V8.
//
// Usage (from tools/reference):
//   node gen-khroma.mjs > ../../internal/khroma/testdata/khroma.json.gz
import * as khroma from 'khroma';
import zlib from 'node:zlib';

let seed = 0x6b68726f >>> 0; // "khro"
const rnd = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const int = (n) => Math.floor(rnd() * n);
const pick = (a) => a[int(a.length)];
const KEYWORDS = Object.keys(
  (await import(process.cwd() + '/node_modules/khroma/dist/color/keyword.js')).default.colors
);

const randCase = (s) => {
  const mode = int(4);
  if (mode === 0) return s;
  if (mode === 1) return s.toUpperCase();
  return [...s].map((c) => (rnd() < 0.5 ? c.toUpperCase() : c)).join('');
};
const hexDigits = '0123456789abcdef';
const hex = () => {
  const len = pick([3, 4, 6, 6, 6, 8]);
  let s = '#';
  for (let i = 0; i < len; i++) s += hexDigits[int(16)];
  return rnd() < 0.3 ? s.toUpperCase() : s;
};
const num = (max, allowOdd = true) => {
  const mode = int(allowOdd ? 10 : 4);
  if (mode < 4) return String(int(max + 1));
  if (mode < 7) return (rnd() * max).toFixed(int(4) + 1);
  if (mode === 7) return String(-int(max));
  if (mode === 8) return '.' + String(int(1000));
  return String(int(9) + 1) + 'e' + String(int(3));
};
const sep = () => pick([', ', ',', ' ', ' , ', ',  ']);
const rgb = () => {
  const pct = rnd() < 0.2;
  const c = () => (pct || rnd() < 0.05 ? num(100) + '%' : num(300));
  let s = (rnd() < 0.5 ? 'rgb' : 'rgba') + '(' + pick(['', ' ']) + c() + sep() + c() + sep() + c();
  if (rnd() < 0.5) {
    s += pick([', ', ',', ' / ', '/', ' ,']) + pick(['', '', '+']);
    s += rnd() < 0.3 ? num(100) + '%' : pick(['0', '1', '0.5', '.25', '0.' + int(1000), '1.5', '-0.2']);
  }
  s += pick(['', ' ']) + ')';
  return rnd() < 0.1 ? s.toUpperCase() : s;
};
const hsl = () => {
  let h = num(720);
  if (rnd() < 0.4) h += pick(['deg', 'grad', 'rad', 'turn', 'DEG', 'Grad', 'RAD', 'TURN']);
  let s = (rnd() < 0.5 ? 'hsl' : 'hsla') + '(' + pick(['', ' ']) + h + sep() + num(120) + '%' + sep() + num(120) + '%';
  if (rnd() < 0.5) {
    s += pick([', ', ',', ' / ', '/']) + pick(['', '', '+']);
    s += rnd() < 0.3 ? num(100) + '%' : pick(['0', '1', '0.5', '.25', '0.' + int(1000), '1.5', '-0.2']);
  }
  s += pick(['', ' ']) + ')';
  return rnd() < 0.1 ? s.toUpperCase() : s;
};
const keyword = () => randCase(pick(KEYWORDS));

const colors = [
  '', ' ', '#', '#1', '#12', '#12345', '#1234567', '#123456789', '#ggg', '#fff', '#FFFF', '#0000', '#00000000',
  'red', 'RED', ' red', 'red ', 'transparent', 'notacolor', 'constructor', '__proto__', 'toString',
  'hasOwnProperty', 'valueOf', 'WH\u0130TE', 'DAR\u212aKHAKI', 'dark\u212ahaki', 'h\u017fl(1, 2%, 3%)',
  'rgb(1,2)', 'rgb(1,2,3,4,5)', 'rgb(1 2 3)', 'rgb(1,2,3', 'rgb (1,2,3)', 'rgb(1,,3)', 'rgb(255, 255, 255)',
  'rgb(100%, 0%, 50%)', 'rgb(1e2, 1e1, 1e0)', 'rgb(1E2, 1e1, 1e0)', 'rgb(1e-2, 1, 1)', 'rgb(.5, .5, .5)',
  'rgb(1., 2, 3)', 'rgb(\u00a01,\u20282,\u30003)', 'rgb(1\n,2\t,3\r)', 'rgb(1\u200b,2,3)', 'rgba(1, 2, 3, 50%)',
  'rgba(1, 2, 3, +0.5)', 'rgba(1, 2, 3 / 0.5)', 'rgb(-1, 256, 1000)', 'rgba(0,0,0,0)', 'rgba(0, 0, 0, 1)',
  'hsl(0, 0%, 0%)', 'hsl(360, 100%, 100%)', 'hsl(120, 50%)', 'hsl(120, 50, 50)', 'hsl(120deg, 50%, 50%)',
  'hsl(0.5turn, 50%, 50%)', 'hsl(3.14rad, 50%, 50%)', 'hsl(100grad, 50%, 50%)', 'hsl(100GRAD, 50%, 50%)',
  'hsl(1e2, 50%, 50%)', 'hsl(1e-1, 5e1%, 50%)', 'hsl(-120, 50%, 50%)', 'hsl(480, 150%, -5%)',
  'hsla(120, 50%, 50%, 0.3)', 'hsla(120, 50%, 50%, 30%)', 'hsl(120 50% 50% / 0.3)', 'hsl(240, 100%, 46.2745098039%)',
  'HSL(240, 100%, 50%)', 'hsl(1turn,1%,1%)', 'hsl(10\n,10%\n,10%)', 'rgb(0, 0, 0)', 'rgb(127.5, 127.5, 127.5)',
  '#fff4dd', '#ECECFF', '#ffffde', '#f4f4f4', '#333', '#000', 'white', 'black', 'lightgrey', 'grey', 'navy',
];
while (colors.length < 2100) {
  const k = int(20);
  if (k < 6) colors.push(hex());
  else if (k < 11) colors.push(rgb());
  else if (k < 16) colors.push(hsl());
  else if (k < 19) colors.push(keyword());
  else colors.push(pick([hex, rgb, hsl, keyword])().slice(0, -1 - int(2)) + pick(['', 'x', ' ', ')']));
}

const encNum = (x) => (Object.is(x, -0) ? '-0' : String(x));
const enc = (v) => {
  if (v === undefined) return null;
  if (typeof v === 'number') return encNum(v);
  return v;
};
const run = (fn) => {
  try {
    return enc(fn());
  } catch (e) {
    return { e: String(e.message), t: e instanceof TypeError };
  }
};
const amount = () => {
  const k = int(12);
  if (k < 6) return pick([0, 5, 10, 20, 23, 25, 30, 75, 100, -10, 150]);
  if (k < 10) return (rnd() - 0.3) * 120;
  if (k === 10) return rnd();
  return null; // an omitted argument
};
const arg = (a) => (a === null ? undefined : a);
const ci = () => int(colors.length);

const out = { colors, unary: {}, amount: {}, channels: {} };
const unary = [
  'toHex', 'toRgba', 'toHsla', 'toKeyword', 'red', 'green', 'blue', 'hue', 'saturation', 'lightness', 'alpha',
  'luminance', 'isDark', 'isLight', 'isTransparent', 'isValid', 'invert', 'complement', 'grayscale',
];
for (const name of unary) out.unary[name] = colors.map((c) => run(() => khroma[name](c)));

const withAmount = ['darken', 'lighten', 'saturate', 'desaturate', 'opacify', 'transparentize', 'invert', 'rgba'];
for (const name of withAmount) {
  const cases = [];
  // Every color once, so that each parser meets each function, then random pairs.
  for (let i = 0; i < 3200; i++) {
    const c = i < colors.length ? i : ci();
    const a = name === 'rgba' ? pick([0, 0.5, 1, rnd(), 1.5, -1]) : amount();
    cases.push([c, a, run(() => khroma[name](colors[c], arg(a)))]);
  }
  out.amount[name] = cases;
}

const CH = ['r', 'g', 'b', 'h', 's', 'l', 'a'];
const channelSet = () => {
  // Mostly sets that khroma accepts (RGB or HSL, never both), sometimes a mix.
  const mode = int(10);
  const pool = mode < 4 ? ['h', 's', 'l', 'a'] : mode < 8 ? ['r', 'g', 'b', 'a'] : CH;
  const n = 1 + int(3);
  const chosen = [];
  while (chosen.length < n) {
    const c = pick(pool);
    if (!chosen.some((x) => x[0] === c)) {
      const range = c === 'a' ? 1 : c === 'h' ? 360 : c === 's' || c === 'l' ? 100 : 255;
      const v = rnd() < 0.15 ? 0 : rnd() < 0.5 ? Math.round((rnd() - 0.5) * 2 * range) : (rnd() - 0.5) * 2 * range;
      chosen.push([c, v]);
    }
  }
  return chosen;
};
for (const name of ['adjust', 'change', 'scale']) {
  const cases = [];
  for (let i = 0; i < 3200; i++) {
    const c = i < colors.length ? i : ci();
    const set = channelSet();
    cases.push([c, set, run(() => khroma[name](colors[c], Object.fromEntries(set)))]);
  }
  out.channels[name] = cases;
}

out.mix = [];
for (let i = 0; i < 3200; i++) {
  const a = ci();
  const b = ci();
  const w = rnd() < 0.3 ? null : pick([0, 25, 50, 75, 100, rnd() * 100, -20, 130]);
  out.mix.push([a, b, w, run(() => khroma.mix(colors[a], colors[b], arg(w)))]);
}
out.contrast = [];
for (let i = 0; i < 2500; i++) {
  const a = ci();
  const b = ci();
  out.contrast.push([a, b, run(() => khroma.contrast(colors[a], colors[b]))]);
}
const component = (max) => {
  const k = int(10);
  if (k < 5) return int(max + 1);
  if (k < 8) return rnd() * max;
  if (k === 8) return -rnd() * max;
  return max + rnd() * max;
};
out.rgba = [];
out.hsla = [];
for (let i = 0; i < 3000; i++) {
  const a = pick([1, 1, 1, 0, 0.5, rnd(), 2, -1]);
  const r = [component(255), component(255), component(255), a];
  out.rgba.push([r, run(() => khroma.rgba(...r))]);
  const h = [component(360), component(100), component(100), a];
  out.hsla.push([h, run(() => khroma.hsla(...h))]);
}

// Values that are not strings. khroma returns them unchanged from Color.parse.
// Objects are left out of the functions that write a channel: khroma mutates
// the object there, and the Go port returns a TypeError instead.
const foreign = [undefined, null, 5, 0, true, false, NaN];
const foreignRead = [...foreign, {}, [], { r: 255, g: 128, b: 0, a: 1 }, { r: '10', g: null, b: 3 }];
const tag = (v) => (v === undefined ? { u: 1 } : typeof v === 'number' && Number.isNaN(v) ? { nan: 1 } : v);
out.foreign = [];
for (const name of ['toHex', 'toRgba', 'red', 'alpha', 'luminance', 'isDark', 'isLight', 'isTransparent', 'isValid']) {
  for (const v of foreignRead) out.foreign.push([name, tag(v), run(() => khroma[name](v))]);
}
for (const name of ['invert', 'complement', 'grayscale', 'darken', 'lighten', 'adjust']) {
  for (const v of foreign) {
    out.foreign.push([name, tag(v), run(() => (name === 'adjust' ? khroma.adjust(v, { h: 10 }) : khroma[name](v, 10)))]);
  }
}
for (const v of foreignRead) out.foreign.push(['mix', tag(v), run(() => khroma.mix(v, '#102030'))]);

process.stdout.write(zlib.gzipSync(JSON.stringify(out), { level: 9 }));
