// Records Mermaid 12.2.0 math label strings through the locked browser oracle.
// Run from tools/reference: node gen-math-labels.mjs.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const cases = [
  ['plain', 'Plain &amp; <b>bold</b>'],
  ['power', '$$x^2$$'],
  ['mixed', 'Solve: $$x^2$$ now'],
  ['consecutive', '$$a$$ and $$b$$'],
  ['fraction', '$$\\frac{a+b}{c}$$'],
  ['radical', '$$\\sqrt{2+2}$$'],
  ['greek', '$$\\alpha + \\beta = \\gamma$$'],
  ['line-break', 'before<br/>$$x^2$$<br>after'],
  ['empty-line', '$$a$$<br/><br/>$$b$$'],
  ['newline', '$$a$$\ntext $$b$$'],
  ['literal-newline', '$$a\\nb$$'],
  ['escaped', '$$\\\\alpha$$'],
  ['text', '$$\\text{A < B & C}$$'],
  ['matrix', '$$\\begin{matrix}a&b\\\\c&d\\end{matrix}$$'],
  ['nested', '$$\\sum_{i=0}^{n}\\frac{x_i}{\\sqrt{y}}$$'],
  ['empty-math', '$$$$'],
  ['script', '<script>alert(1)</script>$$x$$'],
  ['error', '$$\\notACommand$$'],
  ['unclosed', '$$\\frac{a$$'],
].map(([name, text]) => ({ name, text }));
for (const [name, text] of [['legacy-power', '$$x^2$$'], ['legacy-mixed', 'A $$\\frac{a}{b}$$ B']]) {
  cases.push({ name, text, directive: { forceLegacyMathML: true } });
}
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-math-labels-'));
const input = path.join(tmp, 'cases.json');
const result = path.join(tmp, 'results.json');
fs.writeFileSync(input, JSON.stringify({ cases }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/math-labels.js'), '--args', input, '--out', result], { stdio: 'inherit', timeout: 120000 });
const rows = JSON.parse(fs.readFileSync(result, 'utf8'));
const dest = path.join(here, '../../internal/ru/testdata/math_labels.json.gz');
fs.writeFileSync(dest, zlib.gzipSync(JSON.stringify(rows)));
fs.rmSync(input);
fs.rmSync(result);
fs.rmdirSync(tmp);
console.error(`${rows.length} upstream cases -> ${dest}`);
