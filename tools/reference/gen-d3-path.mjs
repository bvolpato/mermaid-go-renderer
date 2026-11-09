// Emits golden cases for internal/d3/path from d3-path.
import { prng, floats, writeGolden, mathTable, pick, int, coord } from './d3-golden-lib.mjs';
import { path, pathRound } from 'd3-path';

const rnd = prng(0x9a7b);
const TAU = 2 * Math.PI;
const radius = () => pick(rnd, [rnd() * 100, rnd() * 100, int(rnd, 1, 50), 0, -0, 1e-7, 1e-12, -5, NaN, 1e9, 6.8181818181818175, 7.5]);
const angle = () => pick(rnd, [rnd() * TAU, rnd() * TAU, (rnd() - 0.5) * 4 * TAU, 0, Math.PI, Math.PI / 2, TAU, -Math.PI, 3 * Math.PI / 2, TAU - 1e-7, TAU + 1e-7, 1e-7, NaN, 100]);

function sequence() {
  const ops = [];
  const n = pick(rnd, [1, 1, 2, 3, 4, 6, 10]);
  let last = [coord(rnd), coord(rnd)];
  const pt = () => {
    const k = rnd();
    if (k < 0.15) return last; // a coincident point
    if (k < 0.25) return [last[0] + (rnd() - 0.5) * 1e-6, last[1] + (rnd() - 0.5) * 1e-6]; // a point very close
    last = [coord(rnd), coord(rnd)];
    return last;
  };
  for (let i = 0; i < n; i++) {
    const k = rnd();
    if (k < 0.2) ops.push(['moveTo', pt()]);
    else if (k < 0.4) ops.push(['lineTo', pt()]);
    else if (k < 0.48) ops.push(['quadraticCurveTo', [...pt(), ...pt()]]);
    else if (k < 0.58) ops.push(['bezierCurveTo', [...pt(), ...pt(), ...pt()]]);
    else if (k < 0.72) {
      // Sometimes three collinear points.
      const p1 = pt(), p2 = rnd() < 0.2 ? [p1[0] * 2 - last[0], p1[1] * 2 - last[1]] : pt();
      ops.push(['arcTo', [...p1, ...p2, radius()]]);
    } else if (k < 0.88) ops.push(['arc', [...pt(), radius(), angle(), angle(), rnd() < 0.5 ? 1 : 0]]);
    else if (k < 0.93) ops.push(['rect', [...pt(), coord(rnd), coord(rnd)]]);
    else ops.push(['closePath', []]);
  }
  return ops;
}

function run(p, ops) {
  const errors = [];
  ops.forEach(([name, args], i) => {
    try {
      if (name === 'arc') p.arc(args[0], args[1], args[2], args[3], args[4], !!args[5]);
      else p[name](...args);
    } catch {
      errors.push(i);
    }
  });
  return { s: p + '', errors };
}

const paths = [];
for (let i = 0; i < 4500; i++) {
  const ops = sequence();
  const digits = pick(rnd, [3, 3, 0, 1, 2, 5, 15, 16, 2.9, 20]);
  const plain = run(path(), ops), rounded = run(pathRound(digits), ops);
  paths.push({ ops: ops.map(([name, args]) => [name, floats(args)]), digits, s: plain.s, r: rounded.s, e: plain.errors });
}
const digits = [];
for (const d of [0, 1, 3, 15, 16, 2.5, -1, NaN, -0.5, Infinity, 1e9]) {
  let ok = true;
  try { pathRound(d); } catch { ok = false; }
  digits.push({ d: String(d), ok });
}
writeGolden('internal/d3/path/testdata/path.json.gz', { math: mathTable(), digits, groups: { paths } });
