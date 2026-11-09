// Emits golden cases for internal/js from V8.
let a = 0x9e3779b9 >>> 0;
const rnd = () => { a = (a + 0x6d2b79f5) >>> 0; let t = a; t = Math.imul(t ^ (t >>> 15), t | 1); t ^= t + Math.imul(t ^ (t >>> 7), t | 61); return ((t ^ (t >>> 14)) >>> 0) / 4294967296; };
const nums = [0, -0, 1, -1, 0.1, 0.5, 1.5, 2.5, -2.5, -0.5, 1e21, 1e-7, 123456789012345680000, 1.7976931348623157e308, 5e-324, 0.000001, 1e-6, 100, 1e20, 123.456, 0.30000000000000004, 230.24998092651367, 257.6717529296875, 1/3, 2/3, 1.005, 8.345, 1.45, -1.45, 0.045, 10.235, 1.255, 999.9995];
for (let i = 0; i < 1500; i++) {
  const mag = Math.pow(10, Math.floor(rnd() * 40) - 20);
  nums.push((rnd() - 0.5) * mag, Math.fround(rnd() * 1000), Math.round(rnd() * 1e6) / 64);
}
const buf = new DataView(new ArrayBuffer(8));
const hex = (x) => { buf.setFloat64(0, x); return buf.getBigUint64(0).toString(16).padStart(16, '0'); };
const strs = ['12px', ' 3.5em', '.5', '-.5e2x', '1e', '1e+', 'abc', '', '  ', '0x1f', '1_000', 'Infinity', '-Infinity', '12.', '+7', '1e400', '-0', '00012', '5.5.5', '\n42\n', '0b101', '0o17', '1e3', '12e-1px'];
console.log(JSON.stringify({
  nums: nums.map((x) => ({ bits: hex(x), str: String(x), round: hex(Math.round(x)), f0: Math.abs(x) < 1e21 ? x.toFixed(0) : null, f2: Math.abs(x) < 1e21 ? x.toFixed(2) : null, f5: Math.abs(x) < 1e21 ? x.toFixed(5) : null })),
  strs: strs.map((s) => ({ s, pf: hex(parseFloat(s)), pi: hex(parseInt(s)), pi16: hex(parseInt(s, 16)), num: hex(Number(s)) })),
  hypot: Array.from({ length: 300 }, () => { const x = (rnd() - 0.5) * 2000, y = (rnd() - 0.5) * 2000; return { x: hex(x), y: hex(y), h: hex(Math.hypot(x, y)) }; }),
}));
