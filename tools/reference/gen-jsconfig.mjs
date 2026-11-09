// Emits golden cases for internal/js/config.go from V8: the operators and the
// property access on dynamic values that the config and theme ports use.
//
// Usage (from tools/reference):
//   node gen-jsconfig.mjs > ../../internal/js/testdata/v8_config.json
import { encode } from './lib-mermaid-internals.mjs';

const ch = (cp) => String.fromCodePoint(cp);
// One instance of each value. A pair (i, i) compares an instance with itself.
const values = [
  undefined, null, true, false, 0, -0, 1, -1, 1.5, NaN, Infinity, -Infinity, 10, 2,
  '', ' ', 'a', 'b', 'abc', 'ab', '10', '2', '1.5', ' 12 ', '0x10', '1e3', 'true', 'null', 'NaN', 'Infinity',
  ch(0x1f600), 'a' + ch(0x1f600), ch(0xff5e),
  [], [1], [1, 2], ['a'], ['a', 'b'], [[1]], [null], [undefined], ['10'], [[]], [{}],
  {}, { a: 1 }, { length: 2, 0: 'x' },
];
const keys = ['length', '0', '1', '2', 'a', 'x', '-1', '01'];

const attempt = (fn) => {
  try {
    return { v: encode(fn()) };
  } catch (e) {
    return { e: e instanceof TypeError ? 'TypeError' : String(e) };
  }
};

const pairs = [];
for (let i = 0; i < values.length; i++) {
  for (let j = 0; j < values.length; j++) {
    const a = values[i];
    const b = values[j];
    pairs.push([
      i,
      j,
      encode(a + b),
      a < b,
      a > b,
      a <= b,
      a >= b,
      a === b,
      [a].includes(b),
    ]);
  }
}

const props = values.map((v) => ({
  get: keys.map((k) => attempt(() => v[k])),
  keys: attempt(() => Object.keys(v)),
  hasOwn: keys.map((k) => attempt(() => Object.hasOwn(v, k))),
  isArray: Array.isArray(v),
  isObject: typeof v === 'object' && v !== null,
}));

// Every code point that /\s/ matches, and what trimStart and trimEnd remove.
const space = [];
for (let cp = 0; cp <= 0xffff; cp++) {
  if (/\s/.test(String.fromCharCode(cp))) space.push(cp);
}
const trims = [' a ', '\n\t a b \r\n', ch(0xfeff) + 'a' + ch(0xa0), ch(0x2028) + 'x' + ch(0x2029), ch(0x200b) + 'x' + ch(0x200b), ch(0x85) + 'x', '', '   ', 'x'].map(
  (s) => [s, s.trimStart(), s.trimEnd()]
);

console.log(JSON.stringify({ values: values.map(encode), keys, pairs, props, space, trims }));
