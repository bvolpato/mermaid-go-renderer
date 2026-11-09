// Emits golden cases for internal/d3/format from d3-format.
//
// Source of d3-format: version 3.1.0, the version inside the mermaid 12.2.0
// bundle that the reference renders with. use-bundled.mjs loads its text from
// the source map of the bundle (see extract-bundled-source.mjs). node_modules
// has d3-format 3.1.2, which changed formatDecimal.js, formatPrefixAuto.js and
// locale.js. The two versions differ for these inputs:
//   - Zero with the types "r" and "p". 3.1.0 keeps the digits of the
//     precision ("0.00"), and 3.1.2 gives "0".
//   - NaN and the infinities with the type "s". 3.1.0 adds the SI prefix of
//     the number that the page formatted last, or the text "undefined".
//   - formatPrefix. 3.1.0 adds the prefix after the padding and the closing
//     parenthesis, and it takes a reference value of zero as 1.
//   - precisionFixed, precisionPrefix and precisionRound with a zero.
//
// Each case records "pe", the value of the module variable prefixExponent of
// formatPrefixAuto.js before the case (null for undefined). A format of the
// type "s" reads that variable for NaN and for an infinity, so the result of
// a case depends on the cases before it. The Go test starts each case with a
// state that has this value.
//
// Each case also records "n", the outputs of the d3-format in node_modules,
// when they are not equal to the outputs of the bundled version. The Go test
// uses them to say that a port follows the wrong version.
//
// Usage (in tools/reference): node gen-d3-format.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';
import { prng, hex, floats, writeGolden, mathTable, pick, int, magnitude, coord } from './d3-golden-lib.mjs';
import { bundledVersion } from './use-bundled.mjs';

const HERE = path.dirname(fileURLToPath(import.meta.url));

// These are dynamic imports, because use-bundled.mjs must run before node
// resolves "d3-format".
const { format, formatPrefix, formatSpecifier, formatLocale, precisionFixed, precisionPrefix, precisionRound } = await import('d3-format');
// The module that owns the variable prefixExponent. It is the module instance
// that the d3-format above uses, so the binding is live.
const auto = await import(new URL('./formatPrefixAuto.js', import.meta.resolve('d3-format')).href);
// The d3-format of node_modules. The import by path does not go through the hook.
const newerDir = path.join(HERE, 'node_modules/d3-format');
const newer = await import(pathToFileURL(path.join(newerDir, 'src/index.js')).href);
const newerVersion = JSON.parse(fs.readFileSync(path.join(newerDir, 'package.json'), 'utf8')).version;

// 3.1.0 gives "0.00" here, and 3.1.2 gives "0". This check stops the script
// when d3-format is not the bundled version.
if (bundledVersion('d3-format') !== '3.1.0' || format('.3r')(0) !== '0.00' || auto.prefixExponent !== undefined) {
  throw new Error('gen-d3-format.mjs: d3-format is not the version of the mermaid bundle (3.1.0)');
}

const rnd = prng(0xf0a3a7);

// pe is the module variable prefixExponent, with null for undefined.
const pe = () => (auto.prefixExponent === undefined ? null : auto.prefixExponent);

let differ = 0;
// withNewer adds the outputs of the newer version to a record when they are
// not equal to the outputs of the bundled version.
function withNewer(rec, outputs, newerOutputs) {
  let other;
  try {
    other = newerOutputs();
  } catch (e) {
    other = [`throws ${e.message}`];
  }
  if (JSON.stringify(other) !== JSON.stringify(outputs)) {
    rec.n = other;
    differ++;
  }
  return rec;
}

// formatCase runs one specifier on a list of values with a format function of
// the bundled version, made by make, and one of the newer version.
function formatCase(spec, vals, make, makeNewer) {
  const before = pe();
  const f = make(spec);
  const o = vals.map((v) => f(v));
  return withNewer({ s: spec, pe: before, v: floats(vals), o }, o, () => {
    const g = makeNewer(spec);
    return vals.map((v) => g(v));
  });
}

function value() {
  const k = rnd();
  if (k < 0.2) return int(rnd, -2000, 2000);
  if (k < 0.4) return Math.round((rnd() - 0.4) * 1e7) / pick(rnd, [10, 100, 1000, 1e4]);
  if (k < 0.55) return coord(rnd);
  if (k < 0.7) return magnitude(rnd, -30, 30);
  if (k < 0.78) return magnitude(rnd, -320, 308);
  if (k < 0.84) return (int(rnd, -50, 50) + 0.5) / pick(rnd, [1, 10, 100, 1000, 2, 4, 8]);
  if (k < 0.9) return pick(rnd, [0, -0, NaN, Infinity, -Infinity, 1e21, -1e21, 1e-7, 0.5, -0.5, 1.5, 2.5, 0.05, -0.0001, 999.5, 0.9995, 1e24, 1e-24, 1e27, 1e-27, 123456789, 2 ** 53, 2 ** 64, 1.5e300]);
  if (k < 0.95) return int(rnd, 0, 2 ** 31) * pick(rnd, [1, 1, 2 ** 20, 2 ** 40, 2 ** 70]);
  return int(rnd, -9, 9) * 10 ** int(rnd, -12, 24);
}

const types = ['e', 'f', 'g', 'r', 's', '%', 'p', 'b', 'o', 'd', 'x', 'X', 'c', '', 'n', 'f', 'd', 's', 'g', 'a', 'G', 'E', 'z'];
function specifier(type) {
  let s = '';
  const k = rnd();
  if (k < 0.25) s += pick(rnd, ['<', '>', '=', '^']);
  else if (k < 0.5) s += pick(rnd, ['0', 'x', '*', ' ', '_', '.', '-', '+', 'é', '€', '<', '0', '#']) + pick(rnd, ['<', '>', '=', '^']);
  if (rnd() < 0.4) s += pick(rnd, ['+', '-', '(', ' ']);
  if (rnd() < 0.25) s += pick(rnd, ['$', '#']);
  if (rnd() < 0.2) s += '0';
  if (rnd() < 0.45) s += pick(rnd, [int(rnd, 1, 30), int(rnd, 0, 12), 0, 1, 20]);
  if (rnd() < 0.35) s += ',';
  if (rnd() < 0.55) s += '.' + pick(rnd, [int(rnd, 0, 8), int(rnd, 0, 25), 0, 1, 2, 3, 20, 21, 22]);
  if (rnd() < 0.2) s += '~';
  s += type ?? pick(rnd, types);
  return s;
}

// A locale with other separators, digits and group sizes.
const localeDef = { decimal: ',', thousands: '.', grouping: [3, 2], currency: ['', ' €'], numerals: ['٠', '١', '٢', '٣', '٤', '٥', '٦', '٧', '٨', '٩'], percent: ' %', minus: '-', nan: 'n/a' };
const loc = formatLocale(localeDef);
const newerLoc = newer.formatLocale(localeDef);

const formats = [];
const locales = [];

// The variable prefixExponent is undefined until a format of the type "s"
// formats a finite number. These cases run first, so that they have that
// state: the results have the text "undefined" in the place of the SI prefix.
const NOT_FINITE = [NaN, Infinity, -Infinity];
for (const spec of ['s', '.3s', '~s', '+s', '(s', '$s', '010s', ',s', '^12s', '.0s', '#s', ' s', '=+8.2s', '<8s', '020,.1s', 'x^+14,.5~s']) {
  formats.push(formatCase(spec, NOT_FINITE, format, newer.format));
}
for (const spec of ['s', '.3s', '$s', '(s', '012,s']) {
  locales.push(formatCase(spec, NOT_FINITE, loc.format, newerLoc.format));
}
if (pe() !== null) throw new Error('gen-d3-format.mjs: a value that is not finite wrote prefixExponent');

// One format of the type "s" on a run of values. NaN and an infinity take
// the SI prefix of the finite number before them.
function stateValue() {
  const k = rnd();
  if (k < 0.3) return pick(rnd, NOT_FINITE);
  if (k < 0.4) return pick(rnd, [0, -0]);
  return value();
}
for (let i = 0; i < 800; i++) {
  const spec = pick(rnd, ['s', '.3s', '~s', '.1s', '+.2s', '(,.4s', '$.2s', '012.3s', specifier('s'), specifier('s')]);
  formats.push(formatCase(spec, Array.from({ length: 6 }, stateValue), format, newer.format));
}
for (let i = 0; i < 200; i++) {
  const spec = pick(rnd, ['s', '.3s', '$~s', specifier('s')]);
  locales.push(formatCase(spec, Array.from({ length: 6 }, stateValue), loc.format, newerLoc.format));
}

// Zero with the types "r" and "p". 3.1.0 formats it with the digits of the
// precision, 3.1.2 gives "0".
for (const spec of ['r', '.1r', '.3r', '.0r', '.21r', '~r', '.3~r', '+.3r', '(.2r', '$.3r', '08.3r', ',.3r', '^10.4r', 'p', '.0p', '.1p', '.2p', '.5~p', '+p', '(.3p', '$.2p', '010.2p', ',.4p', '.2s', 's', '.1s', '~s', '.2e', '.2g', '.2f', '.2%', 'd', '']) {
  formats.push(formatCase(spec, [0, -0, 1e-320, 5e-324, 1, 0], format, newer.format));
  locales.push(formatCase(spec, [0, -0, 0.5], loc.format, newerLoc.format));
}
for (let i = 0; i < 300; i++) {
  const spec = specifier(pick(rnd, ['r', 'p']));
  formats.push(formatCase(spec, [0, value(), -0], format, newer.format));
}

for (let i = 0; i < 6000; i++) {
  formats.push(formatCase(specifier(), [value(), value(), value()], format, newer.format));
}
// The specifiers that mermaid builds for treemap values.
for (const spec of [',', ',.2f', '$,.2f', '.1%', ',.0f', '$0,0', '.3s', '$.2s', '.2~f', ',d', '~s', '.0%', '$,', '=+,.3f']) {
  try {
    format(spec);
  } catch {
    continue;
  }
  formats.push(formatCase(spec, Array.from({ length: 40 }, value), format, newer.format));
}

const invalid = [];
for (const spec of ['.', 'q5', '5.', '..2f', ',,', '😀<5', '<<<', '5 5', 'ff', 'é', '\n<5', '2.2.2', '0x', '~~', '%%', 'ſ', 'K', '+-', '$$', '.f', '-.5', ' ', 'f ', '10,0']) {
  let ok = true, str = null;
  try { str = formatSpecifier(spec) + ''; } catch { ok = false; }
  invalid.push({ s: spec, ok, str });
}

const parses = [];
for (let i = 0; i < 1500; i++) {
  const spec = specifier();
  const p = formatSpecifier(spec);
  parses.push({
    s: spec, fill: p.fill, align: p.align, sign: p.sign, symbol: p.symbol, zero: p.zero, width: hex(p.width === undefined ? NaN : p.width),
    comma: p.comma, precision: hex(p.precision === undefined ? NaN : p.precision), trim: p.trim, type: p.type, str: p + '',
  });
}
// A specifier object goes through toString and is parsed again.
const objects = [];
for (let i = 0; i < 1500; i++) {
  const p = formatSpecifier(specifier());
  if (rnd() < 0.5) p.precision = pick(rnd, [undefined, 0, 1, 2.7, -3, 25, 1e10, int(rnd, 0, 10)]);
  if (rnd() < 0.3) p.width = pick(rnd, [undefined, 0, 1, 7.9, -2, 12]);
  if (rnd() < 0.3) p.type = pick(rnd, types);
  const before = pe();
  const f = format(p);
  const vals = [value(), value()];
  const o = vals.map((v) => f(v));
  objects.push(withNewer({
    s: { fill: p.fill, align: p.align, sign: p.sign, symbol: p.symbol, zero: p.zero, width: hex(p.width === undefined ? NaN : p.width), comma: p.comma, precision: hex(p.precision === undefined ? NaN : p.precision), trim: p.trim, type: p.type },
    str: p + '', pe: before, v: floats(vals), o,
  }, o, () => {
    const g = newer.format(p + '');
    return vals.map((v) => g(v));
  }));
}

const prefixCases = [];
function prefixCase(spec, ref, vals) {
  const f = formatPrefix(spec, ref);
  const o = vals.map((v) => f(v));
  prefixCases.push(withNewer({ s: spec, ref: hex(ref), v: floats(vals), o }, o, () => {
    const g = newer.formatPrefix(spec, ref);
    return vals.map((v) => g(v));
  }));
}
// 3.1.0 adds the SI prefix after the padding and the closing parenthesis.
// 3.1.2 adds it inside. A reference value of zero is 1 in 3.1.0, and NaN and
// the infinities give the text "undefined".
for (const spec of ['10.2', '(.1', '(10.2', '<10.2', '^10.2', '=10.2', '010.2', '(,.1', '$.2', '+.0', 'x>12,.3', '(010,.2', '.2', '']) {
  for (const ref of [1e3, 1e-6, 1e6, 1, 0, -0, NaN, Infinity, -Infinity, 1e30, 1e-30, -2500]) {
    prefixCase(spec, ref, [1234, -1234, 0, 0.000012, NaN, Infinity]);
  }
}
for (let i = 0; i < 2500; i++) {
  const spec = pick(rnd, [',.0', '.2', ',.3', '', '+.1', '$.2', '08.2', '.0', '(,.1', '.1~', '^12.2', specifier()]);
  const ref = pick(rnd, [value(), value(), 1e-6, 1e6, 1e3, 0.001, 1, 0, NaN, 1e24, 1e-24, 1e30, 1e-30]);
  prefixCase(spec, ref, [value(), value()]);
}

const precisions = [];
function precisionCase(step, v) {
  const r = [precisionFixed(step), precisionPrefix(step, v), precisionRound(step, v)];
  const rec = { a: floats([step, v]), r: floats(r) };
  const other = [newer.precisionFixed(step), newer.precisionPrefix(step, v), newer.precisionRound(step, v)];
  if (floats(other) !== rec.r) {
    rec.n = floats(other);
    differ++;
  }
  precisions.push(rec);
}
// The exponent of zero is 0 in 3.1.0 and NaN in 3.1.2.
for (const step of [0, -0, 1, 0.1, 0.25, 10, 1000, 1e-7, NaN, Infinity]) {
  for (const v of [0, -0, 1, 0.1, 999, 1e6, 1e-6, NaN, Infinity]) {
    precisionCase(step, v);
  }
}
for (let i = 0; i < 2500; i++) {
  const step = pick(rnd, [value(), magnitude(rnd, -10, 10), 10 ** int(rnd, -10, 10), 0, NaN, 0.5, 0.25, 2, 5]);
  const v = pick(rnd, [value(), magnitude(rnd, -10, 10), 10 ** int(rnd, -10, 10), 0, NaN]);
  precisionCase(step, v);
}

for (let i = 0; i < 1500; i++) {
  locales.push(formatCase(specifier(), [value(), value()], loc.format, newerLoc.format));
}

const total = formats.length + objects.length + prefixCases.length + precisions.length + locales.length;
if (bundledVersion('d3-format') !== newerVersion && differ < 500) {
  throw new Error(`gen-d3-format.mjs: only ${differ} cases tell d3-format ${bundledVersion('d3-format')} from ${newerVersion}`);
}
writeGolden('internal/d3/format/testdata/format.json.gz', {
  // The version that made the expected values, and the version that made "n".
  d3Format: bundledVersion('d3-format'),
  newer: newerVersion,
  math: mathTable(),
  groups: { formats, invalid, parses, objects, prefixCases, precisions, locales },
});
console.log(`d3-format ${bundledVersion('d3-format')}: ${differ} of ${total} cases with outputs differ from d3-format ${newerVersion}`);
