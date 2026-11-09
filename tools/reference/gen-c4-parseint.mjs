// Writes internal/js/testdata/c4_parseint.json: results of the global
// parseInt(string) of V8 with no radix argument, for the test of
// js.ParseIntAuto. Each case is the string and the bits of the result.
//
// Usage (in tools/reference): node gen-c4-parseint.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.resolve(here, '../../internal/js/testdata/c4_parseint.json');

function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}
const rnd = prng(0xc4);
const int = (n) => Math.floor(rnd() * n);
const digits = (n, alphabet) => Array.from({ length: n }, () => alphabet[int(alphabet.length)]).join('');

const inputs = [
  '', ' ', '0', '-0', '+0', '00', '-00', '7', '+7', '-7', '- 7', '+-7', '12abc', 'abc', '1e3', '1.9', '.5', '-.5',
  'Infinity', '-Infinity', 'NaN', '0x', '0X', '0xg', '0x1g', '-0x10', '+0X1f', '0x0', '-0x0', '0x00ff', '0b11', '0o17',
  '08', '010', '  42  ', '\t\n42', ' 42', '﻿42', '​42', '42 ', '4 2', '4,2', '4_2', '1'.repeat(400),
  '9'.repeat(308), '9'.repeat(309), '9'.repeat(310), '9'.repeat(320), '1' + '0'.repeat(308), '1' + '0'.repeat(309),
  '0x' + 'f'.repeat(255), '0x' + 'f'.repeat(256), '0x' + 'f'.repeat(257), '0x1' + '0'.repeat(255), '0x1' + '0'.repeat(256),
  '9007199254740992', '9007199254740993', '9007199254740994', '9007199254740995', '18014398509481985',
  '0x20000000000000', '0x20000000000001', '0x20000000000002', '0x20000000000003', '0x40000000000002',
  '0x40000000000003', '0x200000000000010', '0x200000000000018', '0x2000000000000180', '0x2000000000000181',
  '0x3ffffffffffffe', '0x3fffffffffffff', '0x7ffffffffffffc', '0x7ffffffffffffe', '0x7fffffffffffff',
  '99999999999999999999999', '1e400', '５', '0x５',
];
const dec = '0123456789';
const hex = '0123456789abcdefABCDEF';
for (let i = 0; i < 600; i++) {
  const sign = ['', '', '', '-', '+'][int(5)];
  const lead = ['', '', ' ', '0', '000'][int(5)];
  const tail = ['', '', '', 'px', '.5', ' ', 'e5', 'g'][int(8)];
  if (i % 2 === 0) {
    inputs.push(lead.trim() === '' ? lead + sign + digits(1 + int(40), dec) + tail : sign + lead + digits(1 + int(40), dec) + tail);
  } else {
    inputs.push(sign + '0x' + (lead === ' ' ? '' : lead) + digits(1 + int(30), hex) + tail);
  }
}
// Hexadecimal numbers near a tie of the rounding.
for (let i = 0; i < 200; i++) {
  const head = '1' + digits(13, hex);
  const round = '08'[int(2)] + ['', '0', '00', '1', '01'][int(5)];
  inputs.push('0x' + head + digits(int(2), '7f89') + round + '0'.repeat(int(6)));
}

const buf = new DataView(new ArrayBuffer(8));
const bits = (x) => {
  buf.setFloat64(0, x);
  return buf.getBigUint64(0).toString(16).padStart(16, '0');
};
const cases = inputs.map((s) => [s, bits(parseInt(s))]);
fs.writeFileSync(outFile, JSON.stringify(cases) + '\n');
console.log(`${outFile}: ${cases.length} cases`);
