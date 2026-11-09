// Emits golden cases for internal/d3/interpolate from d3-interpolate.
import { prng, hex, floats, writeGolden, mathTable, pick, int, coord } from './d3-golden-lib.mjs';
import * as d3 from 'd3-interpolate';
import { rgb, hsl, lab, hcl, cubehelix } from 'd3-color';

const rnd = prng(0x1e7e2901);
const tValue = () => pick(rnd, [0, 1, 0.5, rnd(), rnd(), rnd(), 0.25, 0.1, -0.5, 1.5, NaN]);
const tList = (n) => Array.from({ length: n }, tValue);

const numbers = [];
for (let i = 0; i < 3000; i++) {
  const a = pick(rnd, [coord(rnd), coord(rnd), int(rnd, -100, 100), NaN, 0, -0, Infinity]);
  const b = pick(rnd, [coord(rnd), coord(rnd), int(rnd, -100, 100), NaN, 0, a]);
  const t = tList(3);
  const n = d3.interpolateNumber(a, b), r = d3.interpolateRound(a, b), h = d3.interpolateHue(a, b), d = d3.interpolateDate(a, b);
  numbers.push({ a: hex(a), b: hex(b), t: floats(t), n: floats(t.map(n)), r: floats(t.map(r)), h: floats(t.map(h)), d: floats(t.map((x) => +d(x))) });
}

const names = ['red', 'steelblue', 'transparent', 'white', 'black', 'tomato', 'gray', 'nope', '#00B9FA', '#F95002', '#fff', '#12345678', 'hsl(120,50%,50%)', 'hsla(300,100%,25%,0.3)', 'rgba(10,20,30,0.5)', 'rgb(50%,10%,0%)'];
function colorSpec() {
  const k = rnd();
  if (k < 0.4) return { s: pick(rnd, names) };
  if (k < 0.6) return { s: '#' + int(rnd, 0, 0xffffff).toString(16).padStart(6, '0') };
  const kind = pick(rnd, ['rgb', 'hsl', 'lab', 'hcl', 'cubehelix']);
  const nan = () => (rnd() < 0.05 ? NaN : 1);
  const o = pick(rnd, [1, 1, rnd(), 0, NaN]);
  switch (kind) {
    case 'rgb': return { k: kind, c: [int(rnd, 0, 255) * nan(), rnd() * 255, int(rnd, -20, 280), o] };
    case 'hsl': return { k: kind, c: [rnd() * 360 * nan(), rnd() * nan(), rnd(), o] };
    case 'lab': return { k: kind, c: [rnd() * 100, (rnd() * 160 - 80) * nan(), rnd() * 160 - 80, o] };
    case 'hcl': return { k: kind, c: [(rnd() * 720 - 180) * nan(), rnd() * 100 * nan(), rnd() * 100, o] };
    default: return { k: kind, c: [rnd() * 360 * nan(), rnd() * 2 * nan(), rnd(), o] };
  }
}
const make = { rgb, hsl, lab, hcl, cubehelix };
const toColor = (spec) => (spec.s !== undefined ? spec.s : make[spec.k](...spec.c));
const enc = (spec) => (spec.s !== undefined ? { s: spec.s } : { k: spec.k, c: floats(spec.c) });

const interpolators = {
  rgb: d3.interpolateRgb, rgbGamma: d3.interpolateRgb.gamma(2.2), hsl: d3.interpolateHsl, hslLong: d3.interpolateHslLong, lab: d3.interpolateLab,
  hcl: d3.interpolateHcl, hclLong: d3.interpolateHclLong, cubehelix: d3.interpolateCubehelix, cubehelixLong: d3.interpolateCubehelixLong,
  cubehelixGamma: d3.interpolateCubehelix.gamma(3), cubehelixLongGamma: d3.interpolateCubehelixLong.gamma(0.5),
};
const colors = [];
const kinds = Object.keys(interpolators);
for (let i = 0; i < 2600; i++) {
  const f = i < 600 ? 'hcl' : i < 1000 ? 'rgb' : pick(rnd, kinds);
  const a = colorSpec(), b = colorSpec();
  const t = tList(3);
  const ip = interpolators[f](toColor(a), toColor(b));
  colors.push({ f, a: enc(a), b: enc(b), t: floats(t), o: t.map((x) => ip(x)) });
}

const words = ['translate(', ')', ' ', ',', 'px', 'M', 'L', 'rotate(', 'deg) ', 'a', 'e', '-', '+', '.', '%', ' solid ', 'x', '1e', 'E2'];
function template() {
  let s = '';
  const n = int(rnd, 0, 6);
  for (let i = 0; i < n; i++) {
    const k = rnd();
    if (k < 0.5) s += pick(rnd, words);
    else if (k < 0.8) s += pick(rnd, [int(rnd, -50, 300), (rnd() * 100).toFixed(int(rnd, 0, 3)), '.5', '5.', '1e3', '2.5e-2', '-.25', '+7', '007', '1.2.3']);
    else s += pick(rnd, words) + int(rnd, 0, 99);
  }
  return s;
}
const strings = [];
for (let i = 0; i < 3000; i++) {
  const a = template();
  const b = rnd() < 0.3 ? a.replace(/\d/g, () => String(int(rnd, 0, 9))) : template();
  const t = tList(2);
  const ip = d3.interpolateString(a, b);
  strings.push({ a, b, t: floats(t), o: t.map((x) => ip(x)) });
}

// Dynamic values: numbers, strings, colors, arrays, objects, null and booleans.
function dyn(depth) {
  const k = rnd();
  if (k < 0.3) return int(rnd, -50, 50) + (rnd() < 0.3 ? 0.5 : 0);
  if (k < 0.45) return pick(rnd, names);
  if (k < 0.6) return template();
  if (k < 0.65) return null;
  if (k < 0.7) return rnd() < 0.5;
  if (depth > 1) return int(rnd, 0, 9);
  if (k < 0.85) return Array.from({ length: int(rnd, 0, 4) }, () => dyn(depth + 1));
  const o = {};
  for (const key of ['x', 'y', 'fill', '10', '2', 'label'].filter(() => rnd() < 0.6)) o[key] = dyn(depth + 1);
  return o;
}
function sameShape(v, depth) {
  // A second value of the same kind, so that the interpolation is not trivial.
  if (typeof v === 'number') return int(rnd, -50, 50);
  if (typeof v === 'string') return rnd() < 0.5 ? pick(rnd, names) : template();
  if (Array.isArray(v)) return [...v.map((e) => sameShape(e, depth + 1)), ...(rnd() < 0.3 ? [dyn(2)] : [])].slice(0, rnd() < 0.2 ? 1 : 9);
  if (v && typeof v === 'object') { const o = {}; for (const k of Object.keys(v)) if (rnd() < 0.8) o[k] = sameShape(v[k], depth + 1); if (rnd() < 0.3) o.extra = dyn(2); return o; }
  return dyn(depth);
}
function compatible(a, b) {
  if (b === null || typeof b !== 'object') return true;
  if (a === null || typeof a !== 'object' || Array.isArray(a) !== Array.isArray(b)) return false;
  if (Array.isArray(b)) return b.every((e, i) => i >= a.length || compatible(a[i], e));
  return Object.keys(b).every((k) => !(k in a) || compatible(a[k], b[k]));
}
const values = [];
for (let i = 0; i < 2500; i++) {
  const a = dyn(0);
  const b = rnd() < 0.8 ? sameShape(a, 0) : dyn(0);
  // The port has no interpolator for an array or object b with an a of another kind.
  if (!compatible(a, b)) continue;
  const t = tList(2);
  const ip = d3.interpolate(a, b);
  values.push({ a: JSON.stringify(a), b: JSON.stringify(b), t: floats(t), o: t.map((x) => JSON.stringify(ip(x))) });
}

const splines = [];
for (let i = 0; i < 1500; i++) {
  const n = pick(rnd, [1, 2, 3, 4, 6, 9]);
  const v = Array.from({ length: n }, () => (rnd() < 0.05 ? NaN : coord(rnd)));
  const cols = Array.from({ length: n }, () => pick(rnd, names));
  const t = tList(3);
  const bs = d3.interpolateBasis(v), bc = d3.interpolateBasisClosed(v), rb = d3.interpolateRgbBasis(cols), rc = d3.interpolateRgbBasisClosed(cols);
  const dis = d3.interpolateDiscrete(v);
  const q = int(rnd, 1, 6);
  const c = { v: floats(v), cols, t: floats(t), bs: floats(t.map(bs)), bc: floats(t.map(bc)), rb: t.map((x) => rb(x)), rc: t.map((x) => rc(x)), dis: floats(t.map((x) => dis(x))), q, quant: floats(d3.quantize(d3.interpolateNumber(v[0], v[n - 1]), q)) };
  if (n >= 2) {
    const pw = d3.piecewise(d3.interpolateNumber, v), pc = d3.piecewise(d3.interpolateRgb, cols);
    const tt = t.filter((x) => !Number.isNaN(x));
    c.pt = floats(tt); c.pw = floats(tt.map(pw)); c.pc = tt.map((x) => pc(x));
  }
  splines.push(c);
}

writeGolden('internal/d3/interpolate/testdata/interpolate.json.gz', { math: mathTable(), groups: { numbers, colors, strings, values, splines } });
