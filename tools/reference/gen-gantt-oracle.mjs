// Writes the arguments for the gantt oracle of internal/diagrams/gantt: the
// texts of the gantt fixtures of the corpus and of the extra cases in
// internal/diagrams/gantt/testdata/cases. The oracle needs the browser:
//
//   node gen-gantt-oracle.mjs
//   node oracle.mjs eval gantt-oracle-page.js --args ../../.parity/probe/gantt/oracle-args.json \
//     --out ../../.parity/probe/gantt/oracle-out.json
//   node gen-gantt-oracle.mjs --pack
//
// The last command checks the run and writes the golden file
// internal/diagrams/gantt/testdata/oracle.json.gz. For each case it holds
// the dump of the db after the parse and, for the extra cases, the SVG that
// mermaid.render returns (or its error). The corpus fixtures have their SVG
// in .parity/ref; the pack step requires that the oracle page gave the same
// SVG for them, which proves that the page of this oracle renders as the
// reference page does.
import { readFileSync, writeFileSync, mkdirSync, readdirSync, existsSync } from 'node:fs';
import { dirname, resolve, relative, join } from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const here = dirname(fileURLToPath(import.meta.url));
const root = resolve(here, '../..');
const probe = resolve(root, '.parity/probe/gantt');
const corpus = resolve(root, 'testdata/corpus/mermaid-e2e');
const casesDir = resolve(root, 'internal/diagrams/gantt/testdata/cases');

function walk(dir) {
  const out = [];
  for (const e of readdirSync(dir, { withFileTypes: true })) {
    const p = join(dir, e.name);
    if (e.isDirectory()) out.push(...walk(p));
    else if (e.name.endsWith('.mmd')) out.push(p);
  }
  return out.sort();
}

const cases = [];
for (const file of walk(join(corpus, 'gantt'))) {
  cases.push({ name: 'corpus/' + relative(corpus, file).replace(/\.mmd$/, ''), text: readFileSync(file, 'utf8'), corpus: true });
}
for (const file of walk(casesDir)) {
  cases.push({ name: 'cases/' + relative(casesDir, file).replace(/\.mmd$/, ''), text: readFileSync(file, 'utf8') });
}

if (!process.argv.includes('--pack')) {
  mkdirSync(probe, { recursive: true });
  writeFileSync(join(probe, 'oracle-args.json'), JSON.stringify({ cases }));
  console.log(`${cases.length} cases -> ${join(probe, 'oracle-args.json')}`);
} else {
  const out = JSON.parse(readFileSync(join(probe, 'oracle-out.json'), 'utf8'));
  if (out.results.length !== cases.length) throw new Error('the oracle output does not match the cases: run the oracle again');
  const packed = { now: out.now, timezone: out.timezone, cases: {} };
  let same = 0;
  let errors = 0;
  out.results.forEach((r, i) => {
    const c = cases[i];
    if (r.name !== c.name) throw new Error(`case ${i}: ${r.name} != ${c.name}`);
    if (r.now !== '2026-01-15T12:00:00.000Z') throw new Error(`${c.name}: the clock of the frame is ${r.now}`);
    if (r.width !== 784) throw new Error(`${c.name}: the container is ${r.width} px wide`);
    const entry = { db: r.db };
    if (c.corpus) {
      const ref = join(root, '.parity/ref/mermaid-e2e', c.name.replace(/^corpus\//, ''));
      const refSvg = existsSync(ref + '.svg') ? readFileSync(ref + '.svg', 'utf8') : null;
      if (refSvg !== (r.svg ?? null)) throw new Error(`${c.name}: the oracle page and the reference render differ`);
      same++;
    } else if (r.error !== undefined) {
      entry.error = r.error;
      errors++;
    } else {
      entry.svg = r.svg;
    }
    packed.cases[c.name] = entry;
  });
  const file = resolve(root, 'internal/diagrams/gantt/testdata/oracle.json.gz');
  const gz = gzipSync(Buffer.from(JSON.stringify(packed)), { level: 9 });
  writeFileSync(file, gz);
  console.log(`${cases.length} cases, ${same} corpus renders equal to the reference, ${errors} render errors -> ${file} (${(gz.length / 1024).toFixed(0)} KiB)`);
}
