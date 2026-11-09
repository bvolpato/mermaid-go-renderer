// Records how V8 in Chromium answers for the parts of JavaScript that the
// cytoscape layouts depend on and that goja implements in its own way, as
// golden data for internal/cytoscape (testdata/engine.json.gz). The script
// gen-cytoscape-engine.mjs runs it:
//
//   node oracle.mjs eval probes/cytoscape-js.js --bare --out raw.json
//
//   sort     Array.prototype.sort with comparators that are not consistent.
//            Each case has the source of the comparator, the input and the
//            result. "u" stands for undefined and "h" for a hole.
//   numbers  String(x) for doubles: [the bits of x as hex, the text].
//   keys     Object.keys of objects that mix integer keys and other keys:
//            [the keys in insertion order, the keys in enumeration order].
//   parse    parseInt and parseFloat of number texts, as the layouts call
//            parseInt(Math.ceil(x)): [the text, parseInt, parseFloat].
//
// The cases come from a fixed generator, so that the file is stable.
let seed = 0x636f7365;
const rnd = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const int = (n) => Math.floor(rnd() * n);

const comparators = [
  // Consistent.
  '(function (a, b) { return a - b; })',
  '(function (a, b) { return (a % 5) - (b % 5); })',
  // The area comparators of cose-base, on numbers.
  '(function (a, b) { if (a > b) return -1; if (a < b) return 1; return 0; })',
  // Not consistent: a boolean, a result that depends on both in a cycle, NaN.
  '(function (a, b) { return a < b; })',
  '(function (a, b) { return ((a * 7 + b * 13) % 5) - 2; })',
  '(function (a, b) { return (a % 3 === 0 || b % 3 === 0) ? NaN : a - b; })',
  '(function (a, b) { return (a + b) % 2 === 0 ? 1 : -1; })',
  '(function (a, b) { return 0.3 * (a - b); })',
];

const sort = [];
for (const src of comparators) {
  const cmp = eval(src);
  for (const n of [0, 1, 2, 3, 5, 8, 15, 16, 17, 31, 32, 33, 64, 100, 257, 600]) {
    for (let rep = 0; rep < 3; rep++) {
      const input = [];
      for (let i = 0; i < n; i++) {
        const r = rnd();
        // The third repetition has undefined elements and holes.
        input.push(rep === 2 && r < 0.08 ? 'u' : rep === 2 && r < 0.16 ? 'h' : int(rep === 0 ? 1000 : 12));
      }
      const arr = [];
      arr.length = n;
      input.forEach((v, i) => {
        if (v === 'u') {
          arr[i] = undefined;
        } else if (v !== 'h') {
          arr[i] = v;
        }
      });
      let calls = 0;
      arr.sort((a, b) => {
        calls++;
        return cmp(a, b);
      });
      const output = [];
      for (let i = 0; i < n; i++) {
        output.push(!(i in arr) ? 'h' : arr[i] === undefined ? 'u' : arr[i]);
      }
      sort.push({ cmp: src, input, output, calls });
    }
  }
}

const hex = (x) => {
  const v = new DataView(new ArrayBuffer(8));
  v.setFloat64(0, x);
  return v.getBigUint64(0).toString(16).padStart(16, '0');
};
const doubles = [0, -0, 1, -1, 0.1, 1e21, 1e-7, 123456789012345680000, 5e-324, 1.7976931348623157e308, NaN, Infinity, -Infinity, 0.000001, 1e-6, 1.5e-7, 4.35, 0.30000000000000004, 2 ** 53, 2 ** 53 + 2, 1e300, 123.456, 100, 1e20, 999999999999999900000];
for (let i = 0; i < 400; i++) {
  const v = new DataView(new ArrayBuffer(8));
  v.setUint32(0, int(2 ** 32));
  v.setUint32(4, int(2 ** 32));
  doubles.push(v.getFloat64(0));
  doubles.push((rnd() - 0.5) * 10 ** (int(40) - 20));
  doubles.push(Math.round(rnd() * 1e6) / 10 ** int(7));
}
const numbers = doubles.map((x) => [hex(x), String(x)]);

const keys = [];
const pool = ['a', 'b', '10', '2', '0', 'x1', '-1', '1.5', '4294967294', '4294967295', '007', 'left', 'top', '3', '11'];
for (let i = 0; i < 60; i++) {
  const order = [];
  const o = {};
  for (let k = 0; k < 2 + int(8); k++) {
    const key = pool[int(pool.length)];
    if (!(key in o)) {
      order.push(key);
    }
    o[key] = k;
  }
  keys.push([order, Object.keys(o)]);
}

const parse = [];
for (const x of [0, 1, 12.7, -3.2, 1e21, 1.5e21, 1e-7, 123456789012345680000, NaN, Infinity, -0, 0.5, 15, 1e6, 2 ** 53]) {
  const text = String(x);
  parse.push([text, hex(parseInt(x)), hex(parseFloat(text))]);
}
for (const text of ['40px', '0px', '12.5px', ' 7 ', '1e3px', '-0', '.5', '0x10', 'abc', '', '16', '3.14abc']) {
  parse.push([text, hex(parseInt(text)), hex(parseFloat(text))]);
}

return { userAgent: navigator.userAgent, sort, numbers, keys, parse };
