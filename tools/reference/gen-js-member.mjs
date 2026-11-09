#!/usr/bin/env node
// Writes internal/js/testdata/member_treeview.json: goldens for
// js.GetMember, the property access v?.[name] with the properties of the
// prototypes, from V8.
//
// The names of the prototype properties in the Go file are the ones of the
// V8 of the oracle browser (Chromium 156). The script fails when the V8 of
// node has other names, so that the golden never holds another list.
//
// Run it with tools/reference as the working directory:
//
//   node gen-js-member.mjs
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const OUT = path.join(HERE, '../../internal/js/testdata/member_treeview.json');

// The number of own property names of each prototype in Chromium 156.
const expected = [
  [Object.prototype, 12],
  [String.prototype, 52],
  [Array.prototype, 40],
  [Number.prototype, 7],
  [Boolean.prototype, 3],
];
for (const [proto, count] of expected) {
  if (Object.getOwnPropertyNames(proto).length !== count) {
    throw new Error(`this V8 has ${Object.getOwnPropertyNames(proto).length} properties on a prototype, Chromium 156 has ${count}`);
  }
}

const values = [
  undefined,
  null,
  {},
  { a: 1, toString: 'own', length: 3, 0: 'zero', constructor: null },
  { 'README.md': 'fa:bell', '.ts': 'typescript', ts: '', none: 'none' },
  [],
  ['x', 5, null, undefined, { a: 1 }, ['n']],
  '',
  'text',
  '😀a',
  0,
  5,
  -1.5,
  NaN,
  true,
  false,
];
const names = [
  ...new Set([
    ...expected.flatMap(([proto]) => Object.getOwnPropertyNames(proto)),
    'a', 'README.md', '.ts', 'ts', 'none', '0', '1', '2', '3', '4', '5', '6', '00', '01', '-1', '1.5', '4294967294',
    '4294967295', '', ' ', 'Length', 'tostring', 'ToString', 'prototype', 'name', 'x', 'é', 'size',
  ]),
];

// encode keeps what JSON loses: undefined is {"$u":1}, a function is
// {"$f":1}, an object that is not plain data (a prototype) is {"$p":1}, and
// NaN is {"$n":"NaN"}.
const prototypes = new Set(expected.map(([proto]) => proto));
const encode = (v) => {
  if (v === undefined) return { $u: 1 };
  if (typeof v === 'function') return { $f: 1 };
  if (prototypes.has(v)) return { $p: 1 };
  if (typeof v === 'number' && Number.isNaN(v)) return { $n: 'NaN' };
  if (v === null || typeof v !== 'object') return v;
  if (Array.isArray(v)) return Array.from(v, encode);
  const o = {};
  for (const k of Object.keys(v)) o[k] = encode(v[k]);
  return o;
};

const cases = [];
for (const value of values) {
  for (const name of names) {
    cases.push({ value: encode(value), name, result: encode(value?.[name]) });
  }
}
fs.writeFileSync(OUT, '[\n' + cases.map((c) => JSON.stringify(c)).join(',\n') + '\n]\n');
console.log(`${path.relative(path.join(HERE, '../..'), OUT)}: ${cases.length} cases`);
