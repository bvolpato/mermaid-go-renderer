// Emits golden cases for internal/d3/internal/jsm from V8:
// - Math.pow(10, n) and Math.log10(10 ** n) for every integer n, which the
//   tick computation of d3 depends on;
// - random samples of each transcendental Math function. The Go test uses
//   them to measure how often internal/js differs from V8 in the last bit.
import { prng, floats, writeGolden, mathTable, pauseMath } from './d3-golden-lib.mjs';

const rnd = prng(0x315a);
const pow10 = [], log10 = [];
for (let n = -330; n <= 310; n++) {
  const p = Math.pow(10, n);
  pow10.push(n, p);
  log10.push(Number('1e' + n), Math.log10(Number('1e' + n)));
}
mathTable();

const N = 1500;
const samples = {};
const sample = (name, gen, f) => {
  const flat = [];
  for (let i = 0; i < N; i++) {
    const args = gen(i);
    flat.push(...args, f(...args));
  }
  samples[name] = floats(flat);
};
const angle = (i) => [(rnd() * 2 - 1) * (i % 4 === 0 ? 1000 : 2 * Math.PI)];
const unit = () => [rnd() * 2 - 1];
const positive = () => [pauseMath(() => Math.pow(10, rnd() * 20 - 10))];
sample('sin', angle, Math.sin);
sample('cos', angle, Math.cos);
sample('tan', angle, Math.tan);
sample('asin', unit, Math.asin);
sample('acos', unit, Math.acos);
sample('atan', () => [(rnd() * 2 - 1) * 50], Math.atan);
sample('atan2', () => [(rnd() * 2 - 1) * 500, (rnd() * 2 - 1) * 500], Math.atan2);
sample('exp', () => [rnd() * 40 - 20], Math.exp);
sample('log', positive, Math.log);
sample('log10', positive, Math.log10);
sample('log2', positive, Math.log2);
sample('cbrt', positive, Math.cbrt);
sample('pow', (i) => [rnd() * 1000, [1 / 3, 2.4, 1 / 2.4, 0.25, 0.75, 2, 3, rnd() * 4 - 2][i % 8]], Math.pow);
// Special values, compared exactly.
const special = [];
for (const [x, y] of [[1, Infinity], [1, -Infinity], [1, NaN], [-1, Infinity], [-1, -Infinity], [NaN, 0], [0, 0], [2, NaN], [0, -1], [-0, -1], [-8, 1 / 3],
  [4, 0.5], [2, 10], [2, -1074], [10, 0.5], [0.7, 1], [10, 308.5], [10, -400], [10, 400]]) special.push(x, y, Math.pow(x, y));
writeGolden('internal/d3/internal/jsm/testdata/v8math.json.gz', { pow10: floats(pow10), log10: floats(log10), special: floats(special), samples });
