// Emits golden cases for internal/d3/color from d3-color.
import { prng, hex, floats, writeGolden, mathTable, pick, int } from './d3-golden-lib.mjs';
import { color, rgb, hsl, lab, hcl, lch, gray, cubehelix } from 'd3-color';

const rnd = prng(0xc0101);
const names = ['aliceblue', 'red', 'steelblue', 'rebeccapurple', 'transparent', 'Red', 'BLUE', 'darkGray', 'grey', 'lightgoldenrodyellow', 'black', 'white', 'tomato', 'hotpink'];
const hexDigit = () => pick(rnd, '0123456789abcdefABCDEF'.split(''));
const sp = () => pick(rnd, ['', '', '', ' ', '  ', '\t', '\n', ' ', ' ', '﻿']);
function num(kind) {
  const k = rnd();
  const sign = k < 0.1 ? '-' : k < 0.2 ? '+' : '';
  let body;
  if (kind === 'int') body = String(int(rnd, 0, pick(rnd, [255, 255, 300, 9])));
  else {
    const m = rnd();
    const v = kind === 'alpha' ? rnd() * 1.2 : kind === 'pct' ? rnd() * 110 : rnd() * 400;
    body = m < 0.3 ? String(Math.round(v)) : m < 0.6 ? v.toFixed(int(rnd, 1, 4)) : m < 0.7 ? '.' + int(rnd, 0, 99) : m < 0.8 ? v.toExponential(2) : m < 0.9 ? int(rnd, 0, 3) + '.' : String(v);
    if (m >= 0.8 && m < 0.9) body += '5';
  }
  return sp() + sign + body + sp();
}
function colorString() {
  const k = rnd();
  let s;
  if (k < 0.2) s = '#' + Array.from({ length: pick(rnd, [3, 4, 6, 8, 6, 6, 3, 5, 7, 2, 9]) }, hexDigit).join('');
  else if (k < 0.32) s = `rgb(${num('int')},${num('int')},${num('int')})`;
  else if (k < 0.42) s = `rgb(${num('pct')}%,${num('pct')}%,${num('pct')}%)`;
  else if (k < 0.52) s = `rgba(${num('int')},${num('int')},${num('int')},${num('alpha')})`;
  else if (k < 0.6) s = `rgba(${num('pct')}%,${num('pct')}%,${num('pct')}%,${num('alpha')})`;
  else if (k < 0.72) s = `hsl(${num('hue')},${num('pct')}%,${num('pct')}%)`;
  else if (k < 0.82) s = `hsla(${num('hue')},${num('pct')}%,${num('pct')}%,${num('alpha')})`;
  else if (k < 0.92) s = pick(rnd, names);
  else s = pick(rnd, ['', 'none', 'rgb(1,2)', 'rgb(1 2 3)', 'hsl(120,50,50)', '#ggg', 'rgb(1.5,2,3)', 'rgba(1,2,3)', 'hsl(120deg,50%,50%)', 'currentColor', 'İndigo', 'tomato ', ' #fff ', 'RGB(10, 20, 30)', 'Hsl(10, 20%, 30%)', 'blacK', 'rgb(1e2,2,3)', 'rgba(1,2,3,1e-1)', 'hsl(1e2,50%,50%)', 'constructor', 'toString', 'rgb(255,0,0);']);
  if (rnd() < 0.1) s = sp() + s + sp();
  if (rnd() < 0.1) s = s.toUpperCase();
  return s;
}
const und = (x) => (x === undefined ? NaN : x);
const ch = (c) => (c instanceof rgb ? [c.r, c.g, c.b, c.opacity] : c instanceof hsl ? [c.h, c.s, c.l, c.opacity]
  : c instanceof lab ? [c.l, c.a, c.b, c.opacity] : c instanceof hcl ? [c.h, c.c, c.l, c.opacity] : [c.h, c.s, c.l, c.opacity]);
const strings = (c) => [c.formatHex(), c.formatHex8(), c.formatRgb(), c.formatHsl(), c + '', String(c.displayable())];

const parse = [];
for (let i = 0; i < 4000; i++) {
  const s = colorString();
  const c = color(s);
  if (c === null) { parse.push({ s, k: null }); continue; }
  parse.push({ s, k: c instanceof rgb ? 'rgb' : 'hsl', c: floats(ch(c)), o: strings(c), rgb: floats(ch(rgb(s))), hsl: floats(ch(hsl(s))) });
}

function channel(range) {
  const k = rnd();
  if (k < 0.06) return NaN;
  if (k < 0.1) return pick(rnd, [0, -0, range, -range, range * 2, 1e9, -1e9, Infinity, 1e-9]);
  if (k < 0.4) return Math.round(rnd() * range);
  if (k < 0.5) return (rnd() * 1.4 - 0.2) * range;
  return rnd() * range;
}
const opacity = () => pick(rnd, [1, 1, 1, 0.5, 0, rnd(), rnd() * 1.5 - 0.2, NaN]);
function randomColor() {
  switch (pick(rnd, ['rgb', 'rgb', 'hsl', 'lab', 'hcl', 'cubehelix'])) {
    case 'rgb': return ['rgb', rgb(channel(255), channel(255), channel(255), opacity())];
    case 'hsl': return ['hsl', hsl(channel(360), channel(1), channel(1), opacity())];
    case 'lab': return ['lab', lab(channel(100), channel(160) - 80, channel(160) - 80, opacity())];
    case 'hcl': return ['hcl', hcl(channel(360), channel(130), channel(100), opacity())];
    default: return ['cubehelix', cubehelix(channel(360), channel(2), channel(1), opacity())];
  }
}

const convert = [];
for (let i = 0; i < 1300; i++) {
  const [k, c] = randomColor();
  convert.push({
    k, c: floats(ch(c)), o: strings(c),
    to: floats([...ch(c.rgb()), ...ch(hsl(c)), ...ch(lab(c)), ...ch(hcl(c)), ...ch(cubehelix(c))]),
  });
}
// Grays and exact colors take the branches for zero chroma.
for (const s of ['#000', '#fff', '#808080', '#7f7f7f', 'rgb(1,1,1)', 'red', 'lime', 'blue', 'hsl(0,0%,50%)', 'hsl(120,100%,50%)', '#00B9FA', '#F95002', 'transparent']) {
  const c = color(s);
  convert.push({
    k: c instanceof rgb ? 'rgb' : 'hsl', c: floats(ch(c)), o: strings(c),
    to: floats([...ch(c.rgb()), ...ch(hsl(c)), ...ch(lab(c)), ...ch(hcl(c)), ...ch(cubehelix(c))]),
  });
}

const ops = [];
for (let i = 0; i < 1500; i++) {
  const [k, c] = randomColor();
  const kk = pick(rnd, [1, 2, 0.5, 0, -1, 3, rnd() * 4, NaN]);
  const o = { k, c: floats(ch(c)), kk: hex(kk), b0: floats(ch(c.brighter())), d0: floats(ch(c.darker())), b: floats(ch(c.brighter(kk))), d: floats(ch(c.darker(kk))) };
  if (k === 'rgb' || k === 'hsl') o.clamp = floats(ch(c.clamp()));
  ops.push(o);
}
const misc = { gray: floats(ch(gray(40, 0.5))), lch: floats(ch(lch(50, 30, 120, 1))), nullRgb: floats(ch(rgb('nope'))), nullHsl: floats(ch(hsl('nope'))), nullLab: floats(ch(lab('nope'))), nullHcl: floats(ch(hcl('nope'))), nullCubehelix: floats(ch(cubehelix('nope'))) };

void und;
writeGolden('internal/d3/color/testdata/color.json.gz', { math: mathTable(), misc, groups: { parse, convert, ops } });
