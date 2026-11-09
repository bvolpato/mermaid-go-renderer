// Writes internal/css/testdata/color-cases.json.gz: what Chromium makes of a
// color in the hsl(), hsla(), rgb() or rgba() notation that a script sets on
// a style. The CSSOM prints such a color as rgb() or rgba(), so each case
// shows how Chromium reads the numbers and converts the color to 8-bit sRGB
// channels and an alpha value. The Go test internal/css/color_chromium_test.go
// replays the cases.
//
// The values are
//   - results of khroma (darken, lighten, adjust, transparentize) for random
//     colors: mermaid makes its theme colors this way,
//   - random hsl() and hsla() texts, with hues and percentages inside and
//     outside of their ranges,
//   - gray colors and saturated colors whose channels are close to the
//     middle between two 8-bit values, where the rounding decides,
//   - the colors of css-color-rounding-cases.json, which tell apart the ways
//     in which the conversion can round, also in other notations (a unit,
//     spaces for commas, an exponent),
//   - alpha values in hsla() and rgba(), and rgb() with numbers that are not
//     integers.
//
// The script runs the eval command of the browser oracle on an empty page.
//
// Usage (in tools/reference): node gen-css-color-probe.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { adjust, darken, lighten, transparentize } from 'khroma';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = prng(0x68736c);
const int = (n) => Math.floor(rnd() * n);
const pick = (arr) => arr[int(arr.length)];
const hexColor = () => '#' + [0, 0, 0].map(() => int(256).toString(16).padStart(2, '0')).join('');
// trim prints a number with at most 10 decimals, as khroma does.
const trim = (v, places = 10) => String(Math.round(v * 10 ** places) / 10 ** places);

const values = new Set();
// 1. khroma.
for (let i = 0; i < 2500; i++) {
  const c = hexColor();
  const f = pick([
    () => darken(c, pick([10, 20, 30, 5, 25, 40])),
    () => lighten(c, pick([10, 20, 30, 5, 25, 40])),
    () => adjust(c, { h: pick([60, -60, 120, 180, 30, -30]), l: pick([-30, -40, 10, 0, 25]) }),
    () => adjust(c, { s: pick([-40, 20, -10]), l: pick([-10, 10, 30]) }),
    () => transparentize(darken(c, 30), pick([0.3, 0.7, 0.5, 0.25])),
    () => adjust(c, { h: int(360) - 180, s: int(100) - 50, l: int(100) - 50 }),
  ]);
  values.add(f());
}
// 2. Random texts.
for (let i = 0; i < 1500; i++) {
  const places = pick([0, 1, 2, 4, 10, 10, 10]);
  const h = pick([() => rnd() * 360, () => rnd() * 360, () => rnd() * 2000 - 1000, () => int(13) * 30, () => 360, () => 0])();
  const s = pick([() => rnd() * 100, () => rnd() * 100, () => 100, () => 0, () => rnd() * 300 - 100])();
  const l = pick([() => rnd() * 100, () => rnd() * 100, () => 50, () => 0, () => 100, () => rnd() * 300 - 100])();
  const body = `${trim(h, places)}, ${trim(s, places)}%, ${trim(l, places)}%`;
  values.add(chanceAlpha(body));
}
function chanceAlpha(body) {
  if (rnd() < 0.2) {
    return `hsla(${body}, ${trim(rnd(), pick([1, 2, 3, 10]))})`;
  }
  return `hsl(${body})`;
}
// 3. Channels near the middle between two 8-bit values. A gray has the
// lightness as each channel. A color with the saturation 100% and a
// lightness of at most 50% has the channels 0, 2l and a value between.
for (let k = 0; k < 255; k++) {
  for (const d of [0, 1e-10, -1e-10, 1e-8, -1e-8, 1e-6, -1e-6]) {
    const l = ((k + 0.5) / 255) * 100 + d;
    values.add(`hsl(0, 0%, ${trim(l)}%)`);
    if (d === 0 || Math.abs(d) === 1e-8) {
      values.add(`hsl(${pick([0, 30, 120, 200, 330.5])}, ${pick([0, 100, 50, 33.3333333333])}%, ${trim(l)}%)`);
      values.add(`hsl(${trim(rnd() * 360)}, 100%, ${trim(l / 2)}%)`);
    }
  }
}
for (const v of [
  'hsl(0, 0%, 3.3333333333%)',
  'hsl(187.9411764706, 85.7142857143%, 23.3333333333%)',
  'hsl(30, 100%, 68.3333333333%)',
  'hsl(0, 0%, 50%)',
  'hsl(120deg, 100%, 25%)',
  'hsl(0.5turn, 50%, 50%)',
  'hsl(120 100% 25%)',
  'hsl(120 100% 25% / 0.5)',
  'hsl(none 100% 25%)',
  'hsl(120, 100%, 25%, 0.5)',
  'hsl(120, 100%, 25%, 50%)',
  'hsl(120, 100, 25)',
  'hsl(120 100 25)',
  'hsl(120, 100%)',
  'hsl(a, b, c)',
  'HSL(120, 100%, 25%)',
  'hsl(1e2, 50%, 50%)',
  'hsl(-0, 50%, 50%)',
  'hsl(360, 50%, 50%)',
  'hsl(720.5, 50%, 50%)',
]) {
  values.add(v);
}

// 4. Colors for which two plausible ways to compute the conversion differ.
const rounding = JSON.parse(fs.readFileSync(path.join(here, 'css-color-rounding-cases.json'), 'utf8'));
for (const v of [...rounding.fused, ...rounding.pct]) {
  values.add(v);
}

// 5. The same colors in other notations. Chromium has a fast parser for
// the plain legacy notation, which reads at most 7 digits of a fraction, and
// a general parser for all other notations, which reads every digit.
rounding.pct.forEach((v, i) => {
  const m = v.match(/^hsl\(([^,]+), ([^,]+)%, ([^,]+)%\)$/);
  const [h, sat, lig] = m.slice(1);
  const forms = [
    `hsl(${h}deg, ${sat}%, ${lig}%)`,
    `hsl(${h} ${sat}% ${lig}%)`,
    `hsl(${h},${sat}%,${lig}%)`,
    `hsl( ${h} , ${sat}% , ${lig}% )`,
    `HSL(${h}, ${sat}%, ${lig}%)`,
    `hsla(${h}, ${sat}%, ${lig}%, 1)`,
    `hsla(${h}, ${sat}%, ${lig}%, 0.5)`,
    `hsl(${h}, ${sat}%, ${lig}%, 0.5)`,
    `hsl(+${h}, ${sat}%, ${lig}%)`,
    `hsl(${h}e0, ${sat}%, ${lig}%)`,
    `hsl(${h} ${sat}% ${lig}% / 0.5)`,
    `hsl(${h}, ${sat}%, ${lig}%, 50%)`,
  ];
  values.add(forms[i % forms.length]);
  values.add(forms[(i + 5) % forms.length]);
});
// 6. The alpha value, in hsla() and in rgba().
for (let i = 0; i < 400; i++) {
  const a = pick([() => trim(rnd(), pick([1, 2, 3, 4, 10])), () => trim(int(256) / 255, 10), () => trim(rnd() * 100, pick([0, 1, 4])) + '%', () => pick(['0', '1', '.5', '0.5', '1.0', '0.0', '2', '-1', '0.50', '.25', '1e-1'])])();
  const rgb = `${int(256)}, ${int(256)}, ${int(256)}`;
  values.add(pick([`rgba(${rgb}, ${a})`, `rgba(${rgb},${a})`, `rgb(${rgb}, ${a})`, `hsla(${int(360)}, ${int(101)}%, ${int(101)}%, ${a})`, `rgba(${rgb.replaceAll(',', '')} / ${a})`]));
}
// 7. rgb() with numbers that are not integers, and with percentages.
for (let i = 0; i < 300; i++) {
  const n = () => pick([() => trim(rnd() * 255, pick([1, 2, 7, 10])), () => String(int(256)), () => String(int(255)) + '.5', () => String(int(255)) + '.4999999', () => String(int(255)) + '.50000001'])();
  const pct = () => pick([() => trim(rnd() * 100, pick([0, 1, 2, 7, 10])), () => trim(((int(255) + 0.5) / 255) * 100, 10)])() + '%';
  values.add(pick([() => `rgb(${n()}, ${n()}, ${n()})`, () => `rgb(${pct()}, ${pct()}, ${pct()})`, () => `rgba(${n()}, ${n()}, ${n()}, 0.5)`, () => `rgb(${n()} ${n()} ${n()})`])());
}

const list = [...values];
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'css-color-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
const scriptFile = path.join(tmp, 'page.js');
fs.writeFileSync(argsFile, JSON.stringify({ values: list }));
fs.writeFileSync(
  scriptFile,
  `const el = document.createElementNS('http://www.w3.org/2000/svg', 'text');
   return args.values.map((v) => { el.style.fill = ''; el.style.fill = v; return el.style.fill; });`
);
execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'eval', scriptFile, '--args', argsFile, '--out', resultFile, '--bare'], {
  stdio: 'inherit',
});
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

const cases = list.map((v, i) => [v, results[i]]);
const outFile = path.join(repoRoot, 'internal/css/testdata/color-cases.json.gz');
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ engine: 'chromium', cases }) + '\n', { level: 9 }));
console.error(`${path.relative(repoRoot, outFile)}: ${cases.length} cases, ${cases.filter((c) => c[1] === '').length} invalid`);
