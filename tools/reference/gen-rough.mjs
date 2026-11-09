#!/usr/bin/env node
// Emits golden cases for internal/rough from roughjs 4.6.6.
//
// Run it in tools/reference:
//
//   node gen-rough.mjs             write internal/rough/testdata/rough.json.gz
//   node gen-rough.mjs --dump N    print the full result of call number N
//
// The script runs the real roughjs bundle (the file that mermaid imports).
// Math.random is the seeded PRNG of oracle.mjs (SEEDED_RANDOM), and one
// stream runs through all calls, as it does for a page. Each call records
// the number of Math.random draws that it consumes.
//
// Math.sin, Math.cos, Math.tan, Math.atan and Math.asin are the functions of
// rough-fdlibm.mjs: the fdlibm source code of V8 in JavaScript. The native
// functions depend on the build of Node.js (see that file), and the Go test
// needs the same results for each call. The Go test gives the port the same
// functions (internal/rough/v8math).
//
// A golden record holds hashes, not the full numbers: the SHA-256 of the
// operation lists (each number as its 8 IEEE 754 bytes) and of the path
// strings. Use --dump to see the full data of one call.
import { createHash } from 'node:crypto';
import { gzipSync } from 'node:zlib';
import { writeFileSync, mkdirSync } from 'node:fs';
import { spawn } from 'node:child_process';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import rough from './node_modules/roughjs/bundled/rough.esm.js';
import { install as installFdlibm } from './rough-fdlibm.mjs';

installFdlibm();

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT_DIR = path.join(HERE, '../../internal/rough/testdata');

// ---------------------------------------------------------------------------
// Math.random: the PRNG of oracle.mjs, with a call counter.
const RANDOM_SEED = 0x6d6d6467;
let randState = RANDOM_SEED >>> 0;
let randCalls = 0;
Math.random = function () {
  randCalls++;
  randState = (randState + 0x6d2b79f5) >>> 0;
  let t = randState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};

// The PRNG that makes the inputs. It is separate from Math.random.
let genState = 0x726f7567;
function rng() {
  genState = (genState + 0x6d2b79f5) >>> 0;
  let t = genState;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
}
const rnd = (a, b) => a + rng() * (b - a);
const rint = (a, b) => Math.floor(rnd(a, b + 1));
const pick = (arr) => arr[Math.floor(rng() * arr.length)];
const chance = (p) => rng() < p;
const round2 = (x) => Math.round(x * 100) / 100;

// ---------------------------------------------------------------------------
// Encoding of values for JSON.
function enc(v) {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'number') {
    if (Number.isFinite(v) && !Object.is(v, -0)) return v;
    return { $n: Object.is(v, -0) ? '-0' : String(v) };
  }
  if (Array.isArray(v)) return v.map(enc);
  return v;
}
const encOpts = (o) => (o == null ? null : Object.keys(o).map((k) => [k, enc(o[k])]));

const f64 = new DataView(new ArrayBuffer(8));
function hashOps(ops) {
  const h = createHash('sha256');
  const code = { move: 0, bcurveTo: 1, lineTo: 2 };
  for (const op of ops) {
    const b = Buffer.alloc(2 + 8 * op.data.length);
    b[0] = code[op.op];
    b[1] = op.data.length;
    op.data.forEach((v, i) => {
      if (Number.isNaN(v)) {
        b.writeBigUInt64BE(0x7ff8000000000000n, 2 + 8 * i);
      } else {
        b.writeDoubleBE(v, 2 + 8 * i);
      }
    });
    h.update(b);
  }
  return h.digest('hex').slice(0, 20);
}
const hashStr = (s) => createHash('sha256').update(s, 'utf8').digest('hex').slice(0, 20);
function hashPoints(lists) {
  const h = createHash('sha256');
  for (const pts of lists) {
    const b = Buffer.alloc(16 * pts.length + 1);
    pts.forEach((p, i) => {
      for (let k = 0; k < 2; k++) {
        if (Number.isNaN(p[k])) b.writeBigUInt64BE(0x7ff8000000000000n, 16 * i + 8 * k);
        else b.writeDoubleBE(p[k], 16 * i + 8 * k);
      }
    });
    b[16 * pts.length] = 0xff;
    h.update(b);
  }
  return h.digest('hex').slice(0, 20);
}
// tag is the type and the string form of a dynamic value.
const tag = (v) => `${typeof v}:${String(v)}`;
const bitsHex = (v) => {
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};

// ---------------------------------------------------------------------------
// A fake document that records what RoughSVG.draw does.
const fakeDoc = {
  createElementNS(_ns, name) {
    return {
      name,
      attrs: [],
      children: [],
      setAttribute(k, v) {
        this.attrs.push([k, String(v)]);
      },
      appendChild(c) {
        this.children.push(c);
      },
    };
  },
};
const newSvg = (config) => rough.svg({ ownerDocument: fakeDoc }, config ?? undefined);

function callShape(gen, c) {
  const a = c.a;
  const o = c.o ?? undefined;
  switch (c.f) {
    case 'line':
      return gen.line(a[0], a[1], a[2], a[3], o);
    case 'rectangle':
      return gen.rectangle(a[0], a[1], a[2], a[3], o);
    case 'ellipse':
      return gen.ellipse(a[0], a[1], a[2], a[3], o);
    case 'circle':
      return gen.circle(a[0], a[1], a[2], o);
    case 'linearPath':
      return gen.linearPath(a[0], o);
    case 'arc':
      return gen.arc(a[0], a[1], a[2], a[3], a[4], a[5], a[6], o);
    case 'curve':
    case 'curves':
      return gen.curve(a[0], o);
    case 'polygon':
      return gen.polygon(a[0], o);
    case 'path':
      return gen.path(a[0], o);
  }
  throw new Error('unknown shape ' + c.f);
}

// pointLists returns the point lists of a call, for the check that roughjs
// changes them in place in the same way.
function pointLists(c) {
  switch (c.f) {
    case 'linearPath':
    case 'polygon':
    case 'curve':
      return [c.a[0]];
    case 'curves':
      return c.a[0];
  }
  return null;
}

// runCall runs one call and returns its golden record. full is the complete
// data, for --dump.
function runCall(svg, c, full) {
  const gen = svg.generator;
  const rec = { f: c.f, a: enc(c.a), o: encOpts(c.o) };
  const before = randCalls;
  let drawable;
  try {
    drawable = callShape(gen, c);
  } catch (e) {
    rec.r = randCalls - before;
    rec.e = [e.constructor.name, String(e.message)];
    return rec;
  }
  const g = svg.draw(drawable);
  rec.r = randCalls - before;
  rec.sh = drawable.shape;
  rec.s = drawable.sets.map((s) => [s.type, s.ops.length, hashOps(s.ops)]);
  rec.d = g.children.map((p) => p.attrs.map(([k, v]) => (k === 'd' ? [k, `${v.length}:${hashStr(v)}`] : [k, v])));
  const infos = gen.toPaths(drawable);
  rec.t = hashStr(infos.map((p) => [p.d, tag(p.stroke), tag(p.strokeWidth), tag(p.fill)].join('\u0000')).join('\n'));
  // The strings with 2 decimal places, for a call with at most 200 operations.
  // Number.prototype.toFixed is slow in the Go port.
  if (drawable.sets.reduce((n, s) => n + s.ops.length, 0) <= 200) {
    rec.p2 = hashStr(drawable.sets.map((s) => gen.opsToPath(s, 2)).join('\n'));
  }
  const pts = pointLists(c);
  if (pts) rec.pt = hashPoints(pts);
  if (full) {
    full.sets = drawable.sets.map((s) => ({ type: s.type, ops: s.ops.map((op) => [op.op, ...op.data.map(bitsHex)]) }));
    full.draw = g.children.map((p) => p.attrs);
    full.paths2 = drawable.sets.map((s) => gen.opsToPath(s, 2));
    full.toPaths = infos;
    full.points = pts;
  }
  return rec;
}

// ---------------------------------------------------------------------------
// Inputs.
const COLORS = ['#ECECFF', 'red', '#9370DB', 'rgba(1, 2, 3, 0.5)', 'hsl(240, 100%, 96.2745098039%)', 'black'];
const FILL_STYLES = ['nofill', 'default', 'hachure', 'solid', 'zigzag', 'cross-hatch', 'dots', 'dashed', 'zigzag-line'];
const ROUGHNESS = [0, 0.7, 1, 2.5];
const SHAPES = ['line', 'rectangle', 'ellipse', 'circle', 'linearPath', 'arc', 'arcOpen', 'curve', 'curves', 'polygon', 'path', 'pathMermaid'];

const coord = () => pick([() => rint(-200, 400), () => round2(rnd(-200, 400)), () => rnd(-200, 400)])();
const size = (max = 300) => pick([() => rint(1, max), () => round2(rnd(1, max)), () => rnd(0.5, max)])();
const points = (n, spread = 300) => {
  const cx = coord();
  const cy = coord();
  return Array.from({ length: n }, () => [cx + round2(rnd(-spread, spread)), cy + rnd(-spread, spread)]);
};

// fmtNum writes a number in one of the forms that path data allows.
function fmtNum(v) {
  const k = rint(0, 9);
  if (k === 0) return v.toExponential(3);
  if (k === 1 && v >= 0) return '+' + v;
  if (k === 2 && Number.isInteger(v)) return v + '.';
  if (k === 3 && Math.abs(v) < 1 && v !== 0) return String(v).replace('0.', '.');
  if (k === 4) return v.toFixed(3);
  return String(v);
}
const pathNum = (lo = -150, hi = 250) => pick([() => rint(lo, hi), () => round2(rnd(lo, hi)), () => rnd(lo, hi), () => rint(lo, hi)])();

// randPath makes path data with every command type.
function randPath(opts = {}) {
  const sep = () => pick([' ', ' ', ',', ', ', '  ', '\n', '\t', ' , ']);
  const cmds = opts.cmds ?? 'LlHhVvCcSsQqTtAaZz';
  const parts = [];
  const nums = (arr) => arr.map((v, i) => (i === 0 ? pick(['', ' ']) : sep()) + fmtNum(v)).join('');
  const lo = opts.lo ?? -150;
  const hi = opts.hi ?? 250;
  const start = [pathNum(lo, hi), pathNum(lo, hi)];
  parts.push((chance(0.1) ? 'm' : 'M') + nums(start));
  const count = opts.count ?? rint(2, 9);
  for (let i = 0; i < count; i++) {
    const c = pick(cmds.split(''));
    const rel = c === c.toLowerCase();
    const p = () => (rel ? pathNum(lo / 2, hi / 3) : pathNum(lo, hi));
    const reps = chance(0.15) ? 2 : 1; // implicit repetition of the command
    let args = [];
    for (let r = 0; r < reps; r++) {
      switch (c.toUpperCase()) {
        case 'L':
        case 'T':
          args.push(p(), p());
          break;
        case 'H':
        case 'V':
          args.push(p());
          break;
        case 'C':
          args.push(p(), p(), p(), p(), p(), p());
          break;
        case 'S':
        case 'Q':
          args.push(p(), p(), p(), p());
          break;
        case 'A': {
          const rmax = Math.max(8, Math.round(hi / 2));
          const rx = pick([() => rint(0, rmax), () => round2(rnd(1, rmax)), () => -rint(1, rmax / 2), () => rint(1, 5)])();
          const ry = pick([() => rint(0, rmax), () => round2(rnd(1, rmax)), () => rx, () => rx])();
          args.push(rx, ry, pick([0, 0, 0, 30, 45, 90, rint(-180, 360), round2(rnd(0, 360))]), rint(0, 1), rint(0, 1), p(), p());
          break;
        }
        case 'Z':
          r = reps;
          break;
      }
    }
    parts.push(c + (args.length ? nums(args) : ''));
    if (c.toUpperCase() === 'Z' && chance(0.5)) {
      parts.push((chance(0.5) ? 'M' : 'm') + nums([p(), p()]));
    }
  }
  return parts.join(pick(['', ' ', ' ', '\n']));
}

// Path data in the forms that mermaid makes.
const createRoundedRectPathD = (x, y, totalWidth, totalHeight, radius) =>
  ['M', x + radius, y, 'H', x + totalWidth - radius, 'A', radius, radius, 0, 0, 1, x + totalWidth, y + radius, 'V', y + totalHeight - radius, 'A', radius, radius, 0, 0, 1, x + totalWidth - radius, y + totalHeight, 'H', x + radius, 'A', radius, radius, 0, 0, 1, x, y + totalHeight - radius, 'V', y + radius, 'A', radius, radius, 0, 0, 1, x + radius, y, 'Z'].join(' ');
function generateCirclePoints(centerX, centerY, radius, numPoints, startAngle, endAngle) {
  const pts = [];
  const startAngleRad = (startAngle * Math.PI) / 180;
  const endAngleRad = (endAngle * Math.PI) / 180;
  const angleRange = endAngleRad - startAngleRad;
  const angleStep = angleRange / (numPoints - 1);
  for (let i = 0; i < numPoints; i++) {
    const angle = startAngleRad + i * angleStep;
    const x = centerX + radius * Math.cos(angle);
    const y = centerY + radius * Math.sin(angle);
    pts.push({ x: -x, y: -y });
  }
  return pts;
}
function createPathFromPoints(pts) {
  const pointStrings = pts.map((p, i) => `${i === 0 ? 'M' : 'L'}${p.x},${p.y}`);
  pointStrings.push('Z');
  return pointStrings.join(' ');
}
const stadiumPath = (w, h) => {
  const radius = h / 2;
  return createPathFromPoints([{ x: -w / 2 + radius, y: -h / 2 }, { x: w / 2 - radius, y: -h / 2 }, ...generateCirclePoints(-w / 2 + radius, 0, radius, 50, 90, 270), { x: w / 2 - radius, y: h / 2 }, ...generateCirclePoints(w / 2 - radius, 0, radius, 50, 270, 450)]);
};
const cylinderPath = (x, y, width, height, rx, ry) => [`M${x},${y + ry}`, `M${x + width},${y + ry}`, `a${rx},${ry} 0,0,0 ${-width},0`, `l0,${height}`, `a${rx},${ry} 0,0,0 ${width},0`, `l0,${-height}`].join(' ');
const innerCylinderPath = (x, y, width, height, rx, ry) => [`M${x - width / 2},${-height / 2}`, `a${rx},${ry} 0,0,0 ${width},0`].join(' ');
const hexagonPath = (x, y, width, height, m) => [`M${x + m},${y}`, `L${x + width - m},${y}`, `L${x + width},${y - height / 2}`, `L${x + width - m},${y - height}`, `L${x + m},${y - height}`, `L${x},${y - height / 2}`, 'Z'].join(' ');
const edgePath = () => {
  const p = points(rint(3, 6), 120).map(([x, y]) => [round2(x), round2(y)]);
  let d = `M${p[0][0]},${p[0][1]}L${p[1][0]},${p[1][1]}`;
  for (let i = 2; i + 1 < p.length; i += 1) d += `C${p[i][0]},${p[i][1]},${p[i][0] + 5},${p[i][1] - 3},${p[i + 1][0]},${p[i + 1][1]}`;
  return d;
};
function mermaidPath() {
  const w = pick([() => rint(30, 260), () => rnd(30, 260), () => 171.25, () => 85.609375])();
  const h = pick([() => rint(20, 120), () => rnd(20, 120), () => 45, () => 54])();
  const x = -w / 2;
  const y = -h / 2;
  switch (rint(0, 6)) {
    case 0:
      return createRoundedRectPathD(x, y, w, h, pick([0, 3, 5, 10, h / 2]));
    case 1:
      return stadiumPath(w, h);
    case 2: {
      const rx = w / 2;
      const ry = rx / (2.5 + w / 50);
      return cylinderPath(0, 0, w, h, rx, ry);
    }
    case 3: {
      const rx = w / 2;
      const ry = rx / (2.5 + w / 50);
      return innerCylinderPath(0, ry, w, h, rx, ry);
    }
    case 4:
      return hexagonPath(0, 0, w, h, h / 4);
    case 5:
      return edgePath();
    default:
      return createRoundedRectPathD(coord(), coord(), w, h, 0);
  }
}

// args makes the arguments of one shape. small keeps the shape small, for
// fills that make many operations.
function args(shape, small) {
  const max = small ? 45 : 300;
  switch (shape) {
    case 'line':
      return ['line', [coord(), coord(), coord(), coord()]];
    case 'rectangle':
      return ['rectangle', [coord(), coord(), size(max), size(max)]];
    case 'ellipse':
      return ['ellipse', [coord(), coord(), size(max), size(max)]];
    case 'circle':
      return ['circle', [coord(), coord(), size(max)]];
    case 'linearPath':
      return ['linearPath', [points(rint(2, 7), max)]];
    case 'arc':
    case 'arcOpen': {
      const start = pick([() => 0, () => round2(rnd(-6.5, 6.5)), () => rnd(-6.5, 6.5), () => -Math.PI / 2])();
      // The arc has an angle of 0.6 or more. roughjs does not terminate for
      // an angle near 0.
      const stop = chance(0.9) ? start + rnd(0.6, 7.5) : start - rnd(0.6, 3);
      return ['arc', [coord(), coord(), size(max), size(max), start, stop, shape === 'arc']];
    }
    case 'curve':
      return ['curve', [points(pick([1, 2, 3, 3, 4, 5, 6, 8]), max)]];
    case 'curves': {
      const lists = Array.from({ length: rint(1, 3) }, () => points(pick([0, 1, 2, 3, 4, 5, 7]), max));
      if (lists[0].length === 0 && chance(0.7)) lists[0] = points(4, max);
      return ['curves', [lists]];
    }
    case 'polygon':
      return ['polygon', [points(rint(3, 8), max)]];
    case 'path':
      return ['path', [randPath(small ? { count: rint(2, 4), lo: -20, hi: 40 } : {})]];
    case 'pathMermaid':
      return ['path', [mermaidPath()]];
  }
  throw new Error(shape);
}

// ABSENT is a seed that is not a key of the options object. A seed of
// undefined is a key with the value undefined.
const ABSENT = Symbol('absent');
function matrixOpts(roughness, fillStyle, seed) {
  const o = { roughness };
  if (seed !== ABSENT) o.seed = seed;
  if (fillStyle !== 'nofill') {
    o.fill = pick(COLORS);
    if (fillStyle !== 'default') o.fillStyle = fillStyle;
  }
  return o;
}

// sprinkle adds more options at random.
function sprinkle(o, p = 0.12) {
  if (chance(p)) o.strokeWidth = pick([0.5, 1, 1.3, 2, 2.5, 4, '2', '1.5', '3.0', '2.50']);
  if (chance(p)) o.stroke = pick(['none', '#333', 'blue', 'none']);
  if (chance(p)) o.strokeLineDash = pick([[0, 0], [5], [5, 5], [3, 2.5, 1], [], [8, 3]]);
  if (chance(p / 2)) o.strokeLineDashOffset = pick([0, 2, 1.5, '3']);
  if (chance(p)) o.fillLineDash = pick([[0, 0], [4, 2], [1.5]]);
  if (chance(p / 2)) o.fillLineDashOffset = pick([0, 1, 2.5]);
  if (chance(p)) o.bowing = pick([0, 0.5, 2, 3.5, round2(rnd(0, 5))]);
  if (chance(p)) o.curveFitting = pick([0.5, 0.8, 0.95, 1, round2(rnd(0.3, 1))]);
  if (chance(p)) o.curveTightness = pick([0, 0.2, 0.5, -0.3, round2(rnd(-0.5, 0.9))]);
  if (chance(p)) o.curveStepCount = pick([3, 5, 9, 12, 20, 7.5]);
  if (chance(p)) o.maxRandomnessOffset = pick([0, 1, 2, 3.5, 5]);
  if (chance(p)) o.preserveVertices = pick([true, true, false]);
  if (chance(p)) o.disableMultiStroke = pick([true, true, false]);
  if (chance(p)) o.disableMultiStrokeFill = pick([true, true, false]);
  if (chance(p)) o.hachureAngle = pick([-41, 0, 45, 60, 90, 120, -90, 19, round2(rnd(-180, 180))]);
  if (chance(p)) o.hachureGap = pick([1.5, 2, 4, 5, 6, 8, 0.4, round2(rnd(1, 12))]);
  if (chance(p)) o.fillWeight = pick([0, 0.5, 1.5, 2, 3, 4, -1]);
  if (chance(p / 2)) o.dashOffset = pick([2, 5, 8.5]);
  if (chance(p / 2)) o.dashGap = pick([2, 4, 6.5]);
  if (chance(p / 2)) o.zigzagOffset = pick([3, 5, 8.5]);
  if (chance(p / 2)) o.fillShapeRoughnessGain = pick([0, 0.5, 0.8, 2]);
  if (chance(p / 3)) o.simplification = pick([0.1, 0.5, 0.9, 1, 2]);
  if (chance(p / 3)) o.fixedDecimalPlaceDigits = pick([0, 1, 2, 3, 6]);
  return o;
}

// The options that mermaid gives (handDrawnShapeStyles.ts).
const userNodeOverrides = (seed, styles, extra) =>
  Object.assign({ roughness: 0.7, fill: styles.fill || '#ECECFF', fillStyle: 'hachure', fillWeight: 1.5, hachureGap: 1.5, stroke: styles.stroke || '#9370DB', seed, strokeWidth: styles.strokeWidth || 1.3, fillLineDash: [0, 0], strokeLineDash: styles.dash || [0, 0] }, extra);
const solidStateFill = (color, seed) => ({ fill: color, hachureAngle: 120, hachureGap: 1.5, fillWeight: 1.5, roughness: 0.7, stroke: color, seed });

const cases = [];
const add = (name, calls, config) => cases.push({ n: name, c: config ?? null, calls });
const call = (f, a, o) => ({ f, a, o: o ?? null });

// 1. The matrix: every shape with every fill style, roughness and seed kind.
for (const shape of SHAPES) {
  for (const fillStyle of FILL_STYLES) {
    for (const roughness of ROUGHNESS) {
      for (const seedKind of [0, 1]) {
        const reps = 2;
        for (let r = 0; r < reps; r++) {
          const seed = seedKind === 0 ? pick([0, 0, 0, undefined, ABSENT]) : pick([1, 42, 2147483647, rint(1, 2 ** 31 - 1)]);
          const small = fillStyle === 'dots' || fillStyle === 'zigzag-line' || fillStyle === 'dashed';
          const [f, a] = args(shape, small);
          const o = matrixOpts(roughness, fillStyle, seed);
          if (r === 1) sprinkle(o);
          if (small && o.hachureGap !== undefined && o.hachureGap < 2) o.hachureGap = 3;
          add(`matrix ${shape} ${fillStyle} r${roughness} s${seedKind}`, [call(f, a, o)]);
        }
      }
    }
  }
}

// 2. Random options on random shapes.
for (let i = 0; i < 900; i++) {
  const shape = pick(SHAPES);
  const fillStyle = pick(FILL_STYLES);
  const small = fillStyle === 'dots' || fillStyle === 'zigzag-line' || fillStyle === 'dashed';
  const [f, a] = args(shape, small);
  const o = matrixOpts(pick([0, 0.2, 0.3, 0.5, 0.7, 1, 1.5, 2.5, round2(rnd(0, 3))]), fillStyle, pick([0, 1, undefined, ABSENT, rint(1, 1e6), 7]));
  sprinkle(o, 0.3);
  if (small && o.hachureGap !== undefined && o.hachureGap < 2) o.hachureGap = 3;
  add(`random ${shape}`, [call(f, a, o)]);
}

// 3. The calls that mermaid makes.
for (let i = 0; i < 260; i++) {
  const seed = pick([0, 0, 0, 1, undefined]);
  const styles = pick([{}, {}, { fill: '#f9f' }, { stroke: '#333', strokeWidth: '4' }, { strokeWidth: '2', dash: [5, 5] }, { dash: [5, 5] }]);
  const handDrawn = chance(0.5);
  const classic = (o) => {
    if (!handDrawn) {
      o.roughness = 0;
      o.fillStyle = 'solid';
    }
    return o;
  };
  const w = pick([() => rint(30, 260), () => rnd(30, 260)])();
  const h = pick([() => rint(20, 120), () => rnd(20, 120)])();
  const calls = [];
  switch (i % 13) {
    case 0:
      calls.push(call('rectangle', [-w / 2, -h / 2, w, h], classic(userNodeOverrides(seed, styles, {}))));
      break;
    case 1:
      calls.push(call('path', [createRoundedRectPathD(-w / 2, -h / 2, w, h, pick([5, 10, 0, 3]))], classic(userNodeOverrides(seed, styles, {}))));
      break;
    case 2:
      calls.push(call('path', [stadiumPath(w, h)], classic(userNodeOverrides(seed, styles, {}))));
      break;
    case 3: {
      const rx = w / 2;
      const ry = rx / (2.5 + w / 50);
      calls.push(call('path', [cylinderPath(0, 0, w, h, rx, ry)], userNodeOverrides(seed, styles, {})));
      calls.push(call('path', [innerCylinderPath(0, ry, w, h, rx, ry)], userNodeOverrides(seed, styles, { fill: 'none' })));
      break;
    }
    case 4:
      calls.push(call('path', [hexagonPath(0, 0, w, h, h / 4)], classic(userNodeOverrides(seed, styles, {}))));
      break;
    case 5:
      calls.push(call('circle', [0, 0, w], classic(userNodeOverrides(seed, styles, {}))));
      calls.push(call('circle', [0, 0, w - 10], userNodeOverrides(seed, styles, { roughness: 0.2, strokeWidth: 1.5 })));
      break;
    case 6:
      calls.push(call('circle', [0, 0, 14], solidStateFill('#333333', seed)));
      calls.push(call('circle', [0, 0, 5], { ...solidStateFill('#9370DB', seed), fillStyle: 'solid' }));
      break;
    case 7:
      calls.push(call('path', [edgePath()], { roughness: 0.3, seed }));
      break;
    case 8: {
      const pts = [[0, 0], [w, 0], [w, -h], [0, -h], [0, 0], [-8, 0], [w + 8, 0], [w + 8, -h], [-8, -h], [-8, 0]];
      calls.push(call('polygon', [pts], classic(userNodeOverrides(seed, styles, {}))));
      calls.push(call('polygon', [pts], userNodeOverrides(seed, styles, { fillStyle: 'cross-hatch' })));
      break;
    }
    case 9:
      calls.push(call('line', [-w / 2, 0, w / 2, 0], classic(userNodeOverrides(seed, styles, {}))));
      calls.push(call('ellipse', [0, 0, w, h], userNodeOverrides(seed, styles, {})));
      break;
    case 10:
      calls.push(call('rectangle', [-w / 2, -h / 2, w, h], { seed }));
      calls.push(call('rectangle', [-w / 2, -h / 2 + 20, w, h - 20], { fill: '#fff', fillStyle: chance(0.5) ? 'hachure' : 'solid', stroke: '#9370DB', seed }));
      break;
    case 11:
      calls.push(call('path', [createRoundedRectPathD(-w / 2, -h / 2, w, h, 5)], userNodeOverrides(seed, styles, { roughness: 0.7, fill: undefined, stroke: undefined, strokeWidth: undefined, seed, fillStyle: 'solid', strokeLineDash: [5] })));
      break;
    default:
      calls.push(call('circle', [w, h, 2 * w], { roughness: 0.7, seed, fill: 'rgba(1,2,3,0.3)', fillStyle: 'hachure', fillWeight: 2, hachureGap: 8, hachureAngle: -41 + (i % 3) * 60, stroke: '#333', strokeWidth: parseFloat('5') }));
      calls.push(call('path', [createRoundedRectPathD(0, 0, w, h, 8)], { roughness: 0.7, seed, fill: 'red', fillStyle: 'cross-hatch', fillWeight: 2, hachureGap: 6, hachureAngle: 60, stroke: 'none' }));
  }
  add(`mermaid ${i % 13} ${handDrawn ? 'handDrawn' : 'classic'}`, calls);
}

// 4. One generator for several calls. The random number generator of the
// default options continues from call to call.
for (let i = 0; i < 40; i++) {
  const config = pick([null, { seed: rint(1, 1000) }, { seed: 0, roughness: 2 }, { seed: 5, roughness: 0.7, fill: 'red' }, { roughness: 0 }, { fill: 'blue', fillStyle: 'dots', hachureGap: 6 }]);
  const calls = [];
  for (let k = 0; k < 6; k++) {
    const [f, a] = args(pick(SHAPES), true);
    calls.push(call(f, a, chance(0.6) ? null : pick([{}, { roughness: 1.5 }, { seed: 9 }, { fill: 'green', fillStyle: 'zigzag' }])));
  }
  add(`reuse ${i}`, calls, config);
}

// 5. Path data: every command and number form, and the text rewrites of
// RoughGenerator.path.
const PATHS = [
  'M10 10',
  '   ',
  ' \n ',
  'M10 10 M20 20 L30 30 L40 20',
  'M10 10 M20 20',
  'M0 0 L10 0 L10 10 Z M20 20 L30 20 L30 30 Z',
  'M0 0 L10 0 L10 10 z m20 20 l10 0 l0 10 z',
  'L 5 5 10 10',
  'Z',
  'z',
  'm5 5 10 10 20 0z',
  'M 1 2 3 4 5 6',
  'm 1 2 3 4 5 6',
  'M10-5L20-30',
  'M10 - 5 L20 - 30',
  'M10 -\t5 L20 -\n30',
  'M1.5.5L.5.25',
  'M1e1 2E1 L1e-1 +2.5e+1',
  'M-0 -0 L-0 10 L10 -0',
  'M 5. 5. L 10. 20.',
  'M0,0 H10 V10 H0 Z',
  'M0,0 h10 v10 h-10 z',
  'M0 0 C 10 0 20 10 20 20 S 30 40 40 40 S 60 20 70 30',
  'M0 0 S 30 40 40 40',
  'M0 0 Q 10 20 30 0 T 60 0 T 90 30',
  'M0 0 T 60 0',
  'M0 0 L 10 10 T 60 0 S 5 5 9 9',
  'M0 0 q 10 20 30 0 t 30 0 t 30 30',
  'M0 0 c 10 0 20 10 20 20 s 10 20 20 20',
  'M10 10 A 20 20 0 0 1 50 10',
  'M10 10 A 20 20 0 1 1 50 10',
  'M10 10 A 20 20 0 1 0 50 10',
  'M10 10 A 20 20 0 0 0 50 10',
  'M10 10 A 20 10 30 0 1 50 30',
  'M10 10 A 5 5 0 0 1 100 10',
  'M10 10 A 0 20 0 0 1 50 10',
  'M10 10 A 20 0 0 0 1 50 10',
  'M10 10 A -20 -20 0 0 1 50 10',
  'M10 10 A 20 20 0 0 1 10 10',
  'M10 10 A 20 20 0 2 2 50 10',
  'M10 10 A 20 20 0 0 5 50 10',
  'M10 10 A 20 20 720.5 1 1 50 40',
  'M10 10 a 20 20 0 0 1 40 0 a 20 20 0 0 1 -40 0',
  'M10 10 A 25 25 0 1 1 9.99 10 Z',
  'M0 0 A 1e3 1e3 0 0 1 5 5',
  'M0 0 A 30 50 -45 0 1 215.1 109.9 L 315 10',
  'M100,100 a50,50 0 1,0 100,0 a50,50 0 1,0 -100,0',
  'M0 0 L 100 0 /(ss)/g',
  'M0 0\nL 100 0\nL 100 100',
  'M0 0L1e-320 1e-320L5e-324 0',
  'M 0 0 L 0 0 L 0 0 Z',
  'M5 5 Z',
  'M5 5 L5 5',
  'M0 0 C 0 0 0 0 0 0',
  'M0 0 L10 10 M0 0 L10 10 M0 0 Z',
  'M0 0 l 0.1 0.1 l 0.1 -0.1 l 0.1 0.1 l 0.1 -0.1 z',
];
for (const d of PATHS) {
  for (const o of [
    { roughness: 0, seed: 0, fill: 'red', fillStyle: 'solid' },
    { roughness: 1, seed: 1, fill: 'red' },
    { roughness: 0.7, seed: 0, fill: 'red', fillStyle: 'cross-hatch', hachureGap: 3 },
    { roughness: 2.5, seed: 3, fill: 'red', fillStyle: 'solid', simplification: 0.5 },
    { roughness: 1, seed: 0, fill: 'red', simplification: 0.3, hachureAngle: 19 },
    { roughness: 0.7, seed: 0, fill: 'transparent' },
    null,
  ]) {
    add(`pathdata ${JSON.stringify(d)}`, [call('path', [d], o)]);
  }
}
for (let i = 0; i < 150; i++) {
  const o = matrixOpts(pick(ROUGHNESS), pick(FILL_STYLES.filter((s) => s !== 'dots')), pick([0, 1, 77]));
  if (chance(0.3)) o.simplification = pick([0.2, 0.5, 0.8]);
  add('pathdata random', [call('path', [randPath({ cmds: pick(['LlHhVv', 'CcSs', 'QqTt', 'Aa', 'AaZz', 'LlHhVvCcSsQqTtAaZz']) })], o)]);
}

// 6. Options of other types and other values.
{
  const rect = () => ['rectangle', [coord(), coord(), size(120), size(120)]];
  const optionSets = [
    {},
    { fill: undefined },
    { fill: null },
    { fill: '' },
    { fill: 'none' },
    { fill: 'none', fillStyle: 'solid' },
    { fill: 'transparent' },
    { fill: 5 },
    { fill: true, fillStyle: 'solid' },
    { stroke: undefined },
    { stroke: null, fill: 'red' },
    { stroke: 5 },
    { stroke: 'none' },
    { stroke: 'none', fill: 'red', fillStyle: 'solid' },
    { strokeWidth: '2' },
    { strokeWidth: '2.50', fill: 'red' },
    { strokeWidth: ' 3 ', fill: 'red' },
    { strokeWidth: 'abc', fill: 'red' },
    { strokeWidth: undefined, fill: 'red' },
    { strokeWidth: null, fill: 'red' },
    { strokeWidth: 0, fill: 'red' },
    { strokeWidth: -2, fill: 'red' },
    { strokeWidth: 1e21, fill: 'red', hachureGap: 4 },
    { strokeWidth: 1e-7, fill: 'red', hachureGap: 4 },
    { fillWeight: undefined, fill: 'red' },
    { fillWeight: '2.0', fill: 'red' },
    { fillWeight: 0, fill: 'red' },
    { fillWeight: -3, strokeWidth: 3, fill: 'red' },
    { fillWeight: 0.1 + 0.2, fill: 'red' },
    { fillStyle: 'nope', fill: 'red' },
    { fillStyle: '', fill: 'red' },
    { fillStyle: undefined, fill: 'red' },
    { fillStyle: 5, fill: 'red' },
    { fillStyle: null, fill: 'red' },
    { seed: undefined, fill: 'red' },
    { seed: null, fill: 'red' },
    { seed: 0.5, fill: 'red' },
    { seed: -3, fill: 'red' },
    { seed: 2 ** 31, fill: 'red' },
    { seed: 2 ** 32 + 7, fill: 'red' },
    { seed: 2 ** 53, fill: 'red' },
    { seed: NaN, fill: 'red' },
    { seed: '7', fill: 'red' },
    { seed: true, fill: 'red' },
    { seed: 1.9, fill: 'red' },
    { seed: 44488, fill: 'red' },
    { roughness: undefined, fill: 'red', fillStyle: 'solid' },
    { roughness: NaN, fill: 'red', fillStyle: 'solid' },
    { roughness: 1, hachureGap: 0, fill: 'red' },
    { roughness: 1, hachureGap: 0.3, fill: 'red' },
    { roughness: 2, hachureGap: NaN, fill: 'red' },
    { roughness: 2, hachureGap: Infinity, fill: 'red' },
    { hachureAngle: NaN, fill: 'red' },
    { hachureAngle: -90, fill: 'red' },
    { hachureAngle: 270, fill: 'red' },
    { hachureAngle: 0, fill: 'red' },
    { hachureGap: 2.5, fill: 'red' },
    { hachureGap: 1.5, fill: 'red' },
    { hachureGap: 0.5, fill: 'red' },
    { hachureGap: 0.49, fill: 'red' },
    { maxRandomnessOffset: 0, fill: 'red', fillStyle: 'solid' },
    { maxRandomnessOffset: NaN, fill: 'red', fillStyle: 'solid' },
    { maxRandomnessOffset: -2, fill: 'red' },
    { bowing: 0 },
    { bowing: -3 },
    { bowing: NaN },
    { roughness: -0.5, fill: 'red' },
    { roughness: 10, fill: 'red' },
    { disableMultiStroke: 1, disableMultiStrokeFill: 'yes', preserveVertices: 1, fill: 'red' },
    { disableMultiStroke: 0, disableMultiStrokeFill: '', preserveVertices: null, fill: 'red' },
    { strokeLineDash: [], fillLineDash: [], fill: 'red' },
    { strokeLineDash: [5, null, undefined, 2], fill: 'red' },
    { strokeLineDash: [' 5', '5 '], fillLineDash: ['a'], fill: 'red' },
    { strokeLineDash: null, strokeLineDashOffset: null, fill: 'red' },
    { strokeLineDash: [0, 0], strokeLineDashOffset: 0, fillLineDash: [0, 0], fillLineDashOffset: 0, fill: 'red' },
    { strokeLineDash: [1.5, 2.25], strokeLineDashOffset: 1.25, fillLineDash: [3], fillLineDashOffset: '2', fill: 'red' },
    { fixedDecimalPlaceDigits: 0, fill: 'red' },
    { fixedDecimalPlaceDigits: 2, fill: 'red', fillStyle: 'solid' },
    { fixedDecimalPlaceDigits: 1.9, fill: 'red' },
    { fixedDecimalPlaceDigits: 20, fill: 'red' },
    { fixedDecimalPlaceDigits: 100, fill: 'red', hachureGap: 30 },
    { fixedDecimalPlaceDigits: -1, fill: 'red' },
    { fixedDecimalPlaceDigits: NaN, fill: 'red' },
    { fixedDecimalPlaceDigits: '2', fill: 'red' },
    { fixedDecimalPlaceDigits: null, fill: 'red' },
    { fixedDecimalPlaceDigits: 3, roughness: 0, fill: 'red', fillStyle: 'solid' },
    { dashOffset: 0, dashGap: 3, fill: 'red', fillStyle: 'dashed' },
    { dashOffset: 3, dashGap: 0, fill: 'red', fillStyle: 'dashed' },
    { dashOffset: 300, dashGap: 300, fill: 'red', fillStyle: 'dashed' },
    { dashOffset: -3, dashGap: 1, fill: 'red', fillStyle: 'dashed' },
    { zigzagOffset: 500, fill: 'red', fillStyle: 'zigzag-line' },
    { zigzagOffset: 2.5, hachureGap: 6, fill: 'red', fillStyle: 'zigzag-line' },
    { hachureGap: 0, fill: 'red', fillStyle: 'dots' },
    { hachureGap: 9, fillWeight: 3, fill: 'red', fillStyle: 'dots' },
    { hachureGap: 9, fillWeight: undefined, fill: 'red', fillStyle: 'dots' },
    { hachureGap: 0.01, fill: 'red', fillStyle: 'zigzag' },
    { curveStepCount: 0 },
    { curveStepCount: NaN },
    { curveStepCount: 0, roughness: 0 },
    { curveStepCount: 1, roughness: 0, fill: 'red' },
    { curveFitting: 0, fill: 'red' },
    { curveFitting: 2, fill: 'red' },
    { curveTightness: 1 },
    { curveTightness: 5 },
    { unknownOption: 5, randomizerX: 1, fill: 'red' },
  ];
  for (const o of optionSets) {
    for (const seedKind of [0, 1]) {
      const opts = { ...o };
      if (!('seed' in opts)) opts.seed = seedKind === 0 ? 0 : 13;
      const calls = [];
      const [f, a] = rect();
      calls.push(call(f, a, opts));
      calls.push(call('ellipse', [coord(), coord(), size(90), size(90)], opts));
      calls.push(call('path', ['M0 0 L 40 0 C 60 10 60 30 40 40 L 0 40 Z'], opts));
      add(`options ${JSON.stringify(encOpts(o))}`, calls);
    }
  }
}

// 7. Degenerate geometry that roughjs handles.
{
  const fills = [
    { roughness: 0, seed: 0, fill: 'red', fillStyle: 'solid' },
    { roughness: 1, seed: 0, fill: 'red' },
    { roughness: 0.7, seed: 4, fill: 'red', fillStyle: 'zigzag' },
    { roughness: 1, seed: 0 },
  ];
  const geo = [
    call('line', [1, 1, 1, 1]),
    call('line', [0, 0, 0, 0]),
    call('line', [-0, -0, -0, -0]),
    call('line', [0, 0, 199.99, 0]),
    call('line', [0, 0, 200, 0]),
    call('line', [0, 0, 350, 0]),
    call('line', [0, 0, 500, 0]),
    call('line', [0, 0, 500.01, 0]),
    call('line', [0, 0, 5000, 3000]),
    call('line', [0, 0, 19.9, 0]),
    call('line', [0, 0, 20, 0]),
    call('line', [1e-300, 0, 2e-300, 0]),
    call('line', [1e300, 0, -1e300, 0]),
    call('line', [NaN, 1, 1, 1]),
    call('line', [0, Infinity, 1, 1]),
    call('rectangle', [0, 0, 0, 0]),
    call('rectangle', [0, 0, 10, 0]),
    call('rectangle', [0, 0, 0, 10]),
    call('rectangle', [5, 5, -10, -20]),
    call('rectangle', [-0, -0, 3, 3]),
    call('rectangle', [0, 0, 0.001, 0.001]),
    call('rectangle', [0.5, 0.5, 1, 1]),
    call('rectangle', [1e6, 1e6, 30, 30]),
    call('ellipse', [0, 0, 0, 0]),
    call('ellipse', [0, 0, -10, -20]),
    call('ellipse', [0, 0, 10, 0]),
    call('ellipse', [0, 0, 1e-3, 1e-3]),
    call('ellipse', [1e5, -1e5, 50, 20]),
    call('circle', [0, 0, 0]),
    call('circle', [0, 0, -14]),
    call('circle', [0, 0, 14]),
    call('circle', [0, 0, 600]),
    call('linearPath', [[]]),
    call('linearPath', [[[1, 1]]]),
    call('linearPath', [[[1, 1], [1, 1]]]),
    call('linearPath', [[[1, 1], [2, 2], [1, 1]]]),
    call('polygon', [[[1, 2]]]),
    call('polygon', [[[1, 2], [3, 4]]]),
    call('polygon', [[[1, 2], [1, 2], [1, 2]]]),
    call('polygon', [[[0, 0], [10, 0], [10, 10], [0, 0]]]),
    call('polygon', [[[0, 0], [10, 0], [20, 0], [30, 0]]]),
    call('polygon', [[[0, 0], [0, 10], [0, 20], [0, 30]]]),
    call('polygon', [[[0, 0], [10, 10], [0, 10], [10, 0]]]),
    call('polygon', [[[0, 0], [10, 0], [10, 10], [0, 10], [0, 0], [3, 3], [7, 3], [7, 7], [3, 7], [3, 3]]]),
    call('curve', [[]]),
    call('curve', [[[1, 2]]]),
    call('curve', [[[1, 2], [3, 4]]]),
    call('curve', [[[1, 2], [3, 4], [9, 1]]]),
    call('curve', [[[1, 2], [1, 2], [1, 2], [1, 2]]]),
    call('curve', [[[0, 0], [100, 0], [100, 100], [0, 100], [0, 0]]]),
    call('curves', [[]]),
    call('curves', [[[]]]),
    call('curves', [[[], [[1, 2], [3, 4], [5, 0], [7, 7]]]]),
    call('curves', [[[[1, 2], [3, 4], [9, 1], [4, 4]], [], [[5, 5]]]]),
    call('curves', [[[[1, 2]], [[3, 4]], [[9, 1], [4, 4]]]]),
    call('arc', [0, 0, 10, 10, 0, Math.PI * 2, true]),
    call('arc', [0, 0, 10, 10, 0, Math.PI * 4, true]),
    call('arc', [0, 0, 10, 10, -Math.PI, Math.PI, false]),
    call('arc', [0, 0, 10, 10, -20, -18, true]),
    call('arc', [0, 0, 0, 0, 0, 2, true]),
    call('arc', [0, 0, -30, -20, 0.5, 2.5, true]),
    call('arc', [0, 0, 30, 20, 3, 1, true]),
    call('arc', [0, 0, 30, 20, 3, 1, false]),
    call('arc', [0, 0, 30, 20, NaN, 1, true]),
    call('arc', [0, 0, 30, 20, 0, Infinity, false]),
    call('path', ['M0 0 L 10 10 L 20 0 L 30 10']),
    call('path', ['']),
    call('path', ['M0 0 L 1e150 0 L 1e150 1e150']),
    call('path', ['M0 0 L 1e150 0 L 1e150 1e150 Z M 5 5 L 6 6 L 7 5 Z']),
  ];
  for (const g of geo) {
    for (const o of fills) {
      // roughjs does not terminate for a pattern fill of a shape with a
      // coordinate that is not finite. Those inputs are in the hang list.
      const nonFinite = g.a.flat(3).some((v) => (typeof v === 'number' && !Number.isFinite(v)) || (typeof v === 'string' && v.includes('1e150')));
      if (nonFinite && o.fill && o.fillStyle !== 'solid' && g.f !== 'line') continue;
      // Each call gets its own points, because roughjs changes them in place.
      add(`degenerate ${g.f} ${JSON.stringify(enc(g.a))}`, [call(g.f, clonePoints(g.a), o)]);
    }
  }
}
function clonePoints(a) {
  return a.map((v) => (Array.isArray(v) ? clonePoints(v) : v));
}

// 8. Inputs for which roughjs throws.
{
  const fill = { fill: 'red', seed: 0 };
  const errs = [
    call('polygon', [[]], fill),
    call('polygon', [[]], { fill: 'red', seed: 3, fillStyle: 'dots' }),
    call('path', ['M10 10 X'], fill),
    call('path', ['M10 10 L NaN NaN'], fill),
    call('path', ['M10 10 L Infinity 5'], fill),
    call('path', ['M10 10 L 5'], fill),
    call('path', ['M 1 2 3'], fill),
    call('path', ['M 1'], null),
    call('path', ['5 5 L 1 1'], fill),
    call('path', ['M10 10 L 5 5 C 1 2 3 L 4 5'], fill),
    call('path', ['M10 10 A 20 20 0 01 50 10'], fill),
    call('path', ['M10 10 L 5 5e'], fill),
    call('path', ['M10 10 L +-5 5'], fill),
    call('path', ['M10 10\fL 5 5'], fill),
    call('path', ['M10 10 L 5 5 #'], fill),
    call('path', ['M10 10 L 1.2.3'], fill),
    call('path', ['é'], fill),
    call('path', ['M0 0 L 10 10 L 20 0'], { roughness: -2, seed: 0 }),
    call('path', ['M0 0 L 10 10 L 20 0'], { roughness: -1.5, seed: 4, fill: 'red' }),
    call('path', ['M0 0 C 1e999 0 5 5 10 10'], { seed: 0 }),
    call('path', ['M0 0 C 1e308 1e308 -1e308 1e308 10 10'], { seed: 0 }),
    call('curve', [[[0, 0], [NaN, 5], [10, 0], [5, 5]]], fill),
    call('curve', [[[0, 0], [Infinity, 5], [10, 0], [5, 5]]], { fill: 'red', seed: 5 }),
  ];
  for (const c of errs) add(`throws ${c.f} ${JSON.stringify(enc(c.a))}`, [c]);
}

// 9. Inputs for which roughjs does not terminate. Each one runs in a child
// process with a time limit and a memory limit.
const hangs = [
  call('rectangle', [NaN, 0, 10, 10], { fill: 'red', seed: 0 }),
  call('rectangle', [0, NaN, 10, 10], { fill: 'red', seed: 1 }),
  call('rectangle', [0, 0, 10, Infinity], { fill: 'red', seed: 0 }),
  call('rectangle', [0, 0, Infinity, 10], { fill: 'red', seed: 0, fillStyle: 'cross-hatch' }),
  call('rectangle', [0, 0, 1e300, 1e300], { fill: 'red', seed: 0 }),
  call('ellipse', [NaN, 0, 10, 10], { fill: 'red', seed: 0 }),
  call('ellipse', [0, 0, Infinity, 10], { seed: 0 }),
  call('ellipse', [0, 0, 10, 10], { seed: 0, curveStepCount: -5 }),
  call('ellipse', [0, 0, 10, 10], { seed: 0, curveStepCount: -5, roughness: 0 }),
  call('circle', [0, 0, Infinity], { seed: 2 }),
  call('arc', [0, 0, 10, 10, 1, 1, true], { seed: 0, roughness: 0 }),
  call('arc', [0, 0, 10, 10, 0, 0, true], { seed: 0, roughness: 0, fill: 'red' }),
  call('arc', [0, 0, 10, 10, -Infinity, 1, false], { seed: 0 }),
  call('arc', [0, 0, 10, 10, -1e300, 1, false], { seed: 0 }),
  call('arc', [0, 0, 10, 10, 0, 1, true], { seed: 0, roughness: 0, curveStepCount: -3 }),
  call('rectangle', [0, 0, 10, 10], { fill: 'red', seed: 0, fillStyle: 'zigzag-line', zigzagOffset: 0 }),
  call('rectangle', [0, 0, 10, 10], { fill: 'red', seed: 0, fillStyle: 'dashed', dashOffset: 0, dashGap: 0 }),
  call('polygon', [[[0, 0], [10, NaN], [10, 10]]], { fill: 'red', seed: 0 }),
  call('path', ['M 1 2 z 3 4'], { seed: 0 }),
  call('path', ['M 1 2 L 3 4 Z 5'], { seed: 0 }),
  call('path', ['M0 0 L 1e200 1e200 L 0 1e200 z'], { fill: 'red', seed: 0 }),
  call('path', ['M0 0 L 1e999 5 L 5 5 z'], { fill: 'red', seed: 0 }),
];

// ---------------------------------------------------------------------------
const decode = (v) => {
  if (Array.isArray(v)) return v.map(decode);
  if (v && typeof v === 'object') {
    if ('$n' in v) return Number(v.$n);
    if ('$u' in v) return undefined;
  }
  return v;
};
if (process.argv[2] === '--hang-child') {
  const c = JSON.parse(process.argv[3]);
  const o = c.o ? Object.fromEntries(c.o.map(([k, v]) => [k, decode(v)])) : null;
  const svg = newSvg(null);
  callShape(svg.generator, { f: c.f, a: decode(c.a), o });
  process.stdout.write('finished');
  process.exit(0);
}

// runHang reports how the call ends in a child process: 'hang' when the
// process reaches the time limit or the memory limit, or 'finished'.
function runHang(c) {
  const rec = { f: c.f, a: enc(c.a), o: encOpts(c.o) };
  return new Promise((resolve) => {
    const child = spawn(process.execPath, ['--max-old-space-size=192', fileURLToPath(import.meta.url), '--hang-child', JSON.stringify(rec)], { stdio: ['ignore', 'pipe', 'ignore'] });
    let out = '';
    let timedOut = false;
    child.stdout.on('data', (d) => (out += d));
    const timer = setTimeout(() => {
      timedOut = true;
      child.kill('SIGKILL');
    }, 5000);
    child.on('close', () => {
      clearTimeout(timer);
      rec.outcome = out === 'finished' && !timedOut ? 'finished' : 'hang';
      resolve(rec);
    });
  });
}

const dumpIndex = process.argv[2] === '--dump' ? Number(process.argv[3]) : -1;
const trace = process.argv.includes('--trace');
let callIndex = 0;
let total = 0;
const outCases = [];
for (const cs of cases) {
  const svg = newSvg(cs.c ? { options: cs.c } : null);
  const calls = [];
  for (const c of cs.calls) {
    if (trace) process.stderr.write(`call ${callIndex} ${cs.n}\n`);
    const full = callIndex === dumpIndex ? {} : null;
    const rec = runCall(svg, c, full);
    rec.i = callIndex;
    rec.rb = total; // the number of draws before this call
    total += rec.r;
    if (full) {
      console.log(JSON.stringify({ name: cs.n, config: cs.c, input: { f: c.f, a: enc(c.a), o: encOpts(c.o) }, rec, full }, null, 1));
      process.exit(0);
    }
    calls.push(rec);
    callIndex++;
  }
  outCases.push({ n: cs.n, c: encOpts(cs.c), calls });
}

const hangRecs = await Promise.all(hangs.map(runHang));
const notHung = hangRecs.filter((h) => h.outcome === 'finished');
if (notHung.length) {
  console.error('these inputs are in the hang list, but roughjs terminates for them:');
  for (const h of notHung) console.error(JSON.stringify(h));
  process.exit(1);
}

const golden = {
  meta: {
    roughjs: '4.6.6',
    node: process.versions.node,
    v8: process.versions.v8,
    platform: `${process.platform}/${process.arch}`,
    randomSeed: RANDOM_SEED,
    calls: callIndex,
    draws: total,
  },
  cases: outCases,
  hangs: hangRecs,
};
mkdirSync(OUT_DIR, { recursive: true });
const json = JSON.stringify(golden);
const gz = gzipSync(json, { level: 9 });
writeFileSync(path.join(OUT_DIR, 'rough.json.gz'), gz);
const errors = outCases.flatMap((c) => c.calls).filter((r) => r.e).length;
console.error(`${outCases.length} cases, ${callIndex} calls (${errors} throw), ${hangRecs.length} hangs, ${total} Math.random draws, ${json.length} bytes of JSON, ${gz.length} bytes gzipped`);
