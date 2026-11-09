// Writes internal/diagrams/state/testdata/draw.json.gz: the SVG that the
// real mermaid.render returns for state diagrams when the layouts are
// replaced by a stub (see state-draw-page.js). The cases test the draw
// function of stateRenderer-v3-unified.ts without the layouts: the layout
// data that it builds, the links that it injects, the title and the view
// box.
//
// Usage (in tools/reference): node gen-state-draw.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/state/testdata/draw.json.gz');
// The seed of Math.random in the oracle page (RANDOM_SEED of oracle.mjs).
const randomSeed = 0x6d6d6467;

const sd = (body, front = '') => front + 'stateDiagram-v2\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const texts = [
  // The layout data.
  sd('[*] --> A\nA --> B: label\nB --> [*]'),
  sd('A: one\nA: two\nstate "Long name" as L\nA --> L'),
  sd('state C {\n  [*] --> A\n  A --> [*]\n}\n[*] --> C\nstate D {\n  state E {\n    F\n  }\n}'),
  sd('state C {\n  A --> B\n  --\n  D --> E\n  --\n  F\n}'),
  sd('state f <<fork>>\nstate j <<join>>\nstate c <<choice>>\nf --> j\nj --> c'),
  sd('A --> B\nnote right of A: a note\nnote left of B\n  two\n  lines\nend note'),
  sd('classDef foo fill:#f00,color:white\nA:::foo --> B\nstyle B stroke:blue,fill:#eee\nstate C {\n  D\n}\nclass C foo'),
  sd('direction LR\nA --> B\nstate C {\n  direction RL\n  D\n}'),
  sd('A --> B', cfg('look: classic')),
  sd('A --> B\nstate C {\n  D\n}\nnote right of A: n', cfg('look: handDrawn')),
  sd('A --> B', cfg('layout: dagre')),
  sd('A --> B', cfg('layout: elk.stress')),
  sd('A --> B', cfg('layout: nosuchlayout')),
  sd('A --> B', cfg('state:\n  nodeSpacing: 10\n  rankSpacing: 20')),
  sd('A --> B', cfg('state:\n  nodeSpacing: 0\n  rankSpacing: 0')),
  sd('A --> B\nnote right of A: n', cfg('state:\n  wrappingWidth: 100\n  minNodeWidth: 80\nflowchart:\n  padding: 20')),
  // The title and the view box.
  sd('A --> B', '---\ntitle: The title\n---\n'),
  sd('A --> B', '---\ntitle: The title\nconfig:\n  state:\n    titleTopMargin: 5\n---\n'),
  sd('A --> B', '---\ntitle: The title\nconfig:\n  state:\n    titleTopMargin: 0\n    useMaxWidth: false\n---\n'),
  sd('A --> B', cfg('state:\n  useMaxWidth: false')),
  sd('accTitle: The accessible title\naccDescr: The accessible description\nA --> B'),
  sd(''),
  // Links.
  sd('A --> B\nclick A "https://example.com" "A tooltip"\n'),
  sd('A --> B\nclick A href "https://example.com"\n'),
  sd('A --> B\nclick A "https://example.com/a" "tip a"\nclick B href "https://example.com/b"\n'),
  sd('A --> B\nclick A "https://example.com/a" "tip a"\nclick A href "https://example.com/again"\n'),
  sd('A --> B\nclick Missing "https://example.com" "tip"\n'),
  sd('state "A long name" as A\nA --> B\nclick A "https://example.com" "tip"\n'),
  sd('state "B" as A\nA --> B\nclick B "https://example.com" "tip"\n'),
  sd('B --> A\nstate "B" as A\nclick B "https://example.com" "tip"\n'),
  sd('A: one\nA: two\nA --> B\nclick A "https://example.com" "tip"\n'),
  sd('state C {\n  A --> B\n}\nclick C "https://example.com" "tip"\nclick A href "https://example.com/a"\n'),
  sd('[*] --> A\nclick [*] href "https://example.com"\n'),
  sd('[*] --> A\nclick root_start href "https://example.com"\n'),
  sd('A --> B\nnote right of A: A\nclick A href "https://example.com"\n'),
  sd('A --> B\nclick A "javascript:alert(1)" "<b>tip</b> & more"\n'),
  sd('A --> B\nclick A "https://example.com/?a=1&b=2" "tip"\n', cfg('look: handDrawn')),
  sd('A --> B\nclick A "https://example.com" "tip"\n', cfg('securityLevel: loose')),
  sd('A --> B\nclick A "https://example.com" "tip"\n', '---\ntitle: With a link\n---\n'),
];
const cases = texts.map((text, i) => ({ name: `draw${i}`, text }));
const sited = [
  { site: { securityLevel: 'loose' }, text: sd('A --> B\nclick A "javascript:alert(1)" "<b>tip</b>"\n') },
  { site: { arrowMarkerAbsolute: true }, text: sd('A --> B') },
  { site: { look: 'handDrawn', layout: 'dagre' }, text: sd('A --> B\nclick B href "https://example.com"\n') },
];
sited.forEach((c, i) => cases.push({ name: `site${i}`, text: c.text, site: c.site }));

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'statedraw-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ seed: randomSeed, cases }));
execFileSync(process.execPath, ['oracle.mjs', 'esm-eval', 'state-draw-page.js', '--args', argsFile, '--out', resultFile], {
  cwd: here,
  stdio: 'inherit',
  maxBuffer: 1 << 30,
});
const result = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (result.cases.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${result.cases.length}`);
}
const golden = result.cases.map((r, i) => {
  const rec = { ...r, text: cases[i].text };
  if (cases[i].site) {
    rec.site = cases[i].site;
  }
  return rec;
});
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases: golden }), { level: 9 }));
fs.rmSync(tmp, { recursive: true, force: true });
const errors = golden.filter((r) => r.error).length;
console.log(`${outFile}: ${golden.length} cases, ${errors} with an error`);
