// Emits golden cases for internal/js/num_d3.go from V8:
// Number.prototype.toExponential and toPrecision.
import { prng, hex, writeGolden, magnitude, pick } from './d3-golden-lib.mjs';

const rnd = prng(0x51ed270b);
const nums = [0, -0, 1, -1, 0.5, 1.5, 2.5, 0.25, 1.25, 1.35, 1.45, 0.045, 1.005, 8.345, 99.99, 99.5, 999.9995, 9.5, 0.95, 0.0095,
  1e21, 1e-7, 1e-6, 123.456, 0.000001234, 0.0000001234, 5e-324, 1.7976931348623157e308, 2 ** 53, 2 ** 53 + 2, 1e100, 123456789012345680000,
  NaN, Infinity, -Infinity, 0.1, 0.2, 0.30000000000000004, 1 / 3, 2 / 3, 4.35, 4.345, 1e15, 1e16, 123456, 5e-7, 5.5e-7, 9.999999e20];
for (let i = 0; i < 2500; i++) {
  nums.push(magnitude(rnd, -25, 25));
  nums.push(Math.round(rnd() * 1e6) / pick(rnd, [1, 2, 4, 8, 10, 100, 1000, 64]));
  if (i % 5 === 0) nums.push(magnitude(rnd, -320, 308));
  // Decimal ties: k + 0.5 scaled by a power of two is exact.
  if (i % 7 === 0) nums.push((Math.floor(rnd() * 1000) + 0.5) / pick(rnd, [1, 2, 4, 8, 16, 1024]));
}
const cases = nums.map((x) => {
  const e = pick(rnd, [0, 1, 2, 3, 5, 8, 12, 17, 20, 21, 40, 100]);
  const p = pick(rnd, [1, 2, 3, 4, 6, 9, 12, 15, 17, 21, 50, 100]);
  return { x: hex(x), e0: x.toExponential(), e, es: x.toExponential(e), p, ps: x.toPrecision(p) };
});
writeGolden('internal/js/testdata/d3_num.json.gz', { groups: { nums: cases } });
