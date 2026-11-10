// Records integrated Mermaid 12.2.0 math diagrams through the locked oracle.
// Run from tools/reference: node gen-math-diagrams.mjs.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const cases = [
  { name: 'flowchart-mixed', text: String.raw`flowchart LR
    A["Solve $$x^2$$ now"] -->|"$$\\frac{a}{b}$$"| B["$$\\sqrt{x}$$"]` },
  { name: 'flowchart-lines', text: String.raw`flowchart TD
    A["$$a$$ and $$b$$<br/>tail"] --> B["Plain"]` },
  { name: 'flowchart-svg', text: String.raw`---
config:
  htmlLabels: false
---
flowchart TD
    A["$$x^2$$"] --> B["plain"]` },
  // Mermaid does not await actor math drawing. Header labels finish during
  // message drawing, but footer labels can still be empty at serialization.
  { name: 'sequence-actors', text: String.raw`---
config:
  sequence:
    mirrorActors: false
---
sequenceDiagram
    participant A as $$\alpha$$
    participant B as $$\beta$$
    A->>B: Solve: $$\sqrt{2+2}$$
    B-->>A: Answer: $$2$$
    Note right of B: $$\sqrt{2+2}=2$$` },
  { name: 'sequence-loop', text: String.raw`sequenceDiagram
    participant A
    participant B
    loop $$n^2$$
      A->>B: $$x_i$$
    end
    alt $$a>b$$
      B->>A: yes
    else $$a<b$$
      A->>B: no
    end` },
  { name: 'sequence-notes', text: String.raw`sequenceDiagram
    autonumber
    A->>A: $$x^2$$
    Note left of A: $$\frac{a}{b}$$
    A->>B: plain
    Note over A,B: Total $$a+b$$
    Note over A: $$\alpha$$` },
  { name: 'sequence-shapes', text: String.raw`---
config:
  sequence:
    mirrorActors: false
---
sequenceDiagram
    actor A as $$x^2$$
    participant B@{ "type": "database" } as $$y_i$$
    A->>B: $$\sum_i x_i$$` },
  { name: 'class-math', text: String.raw`classDiagram
    class A["$$x^2$$"]
    class B
    A --> B` },
  { name: 'invalid-math', text: String.raw`sequenceDiagram
    A->>B: $$\unknownMathCommand$$` },
];
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-math-diagrams-'));
const input = path.join(tmp, 'cases.json');
const result = path.join(tmp, 'results.json');
fs.writeFileSync(input, JSON.stringify({ cases }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'probes/math-diagrams.js'), '--args', input, '--out', result], { stdio: 'inherit', timeout: 120000 });
const rows = JSON.parse(fs.readFileSync(result, 'utf8'));
const dest = path.join(here, '../../internal/ru/testdata/math_diagrams.json.gz');
fs.writeFileSync(dest, zlib.gzipSync(JSON.stringify(rows)));
fs.rmSync(input);
fs.rmSync(result);
fs.rmdirSync(tmp);
console.error(`${rows.length} upstream diagrams -> ${dest}`);
