#!/usr/bin/env node
// Writes internal/js/testdata/math_sankey_chromium.json: the results of
// Math.pow(0.99, i) in the Chromium of the browser oracle, for the calls that
// d3-sankey makes. The Go test compares them with js.PowIntCR.
//
// The script runs the eval command of the browser oracle one time.
//
// Usage (in tools/reference): node gen-sankey-pow-chromium.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.join(here, '../../internal/js/testdata/math_sankey_chromium.json');

const page = `
const f64 = new DataView(new ArrayBuffer(8));
const bits = (v) => {
  f64.setFloat64(0, v);
  return f64.getBigUint64(0).toString(16).padStart(16, '0');
};
const pow = [];
for (let i = 0; i < 1000; i++) pow.push(bits(Math.pow(0.99, i)));
return { userAgent: navigator.userAgent, pow };
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'sankeypow-'));
const scriptFile = path.join(tmp, 'page.js');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, page);
execFileSync(process.execPath, ['oracle.mjs', 'eval', scriptFile, '--bare', '--out', resultFile], { cwd: here, stdio: 'inherit' });
const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.writeFileSync(outFile, JSON.stringify(result) + '\n');
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`${outFile}: ${result.pow.length} values from ${result.userAgent}`);
