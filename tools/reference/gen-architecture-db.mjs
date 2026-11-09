// Writes internal/diagrams/architecture/testdata/db.json.gz: what the db of
// the architecture diagram of real mermaid holds when the draw function
// starts (the nodes with their edges, the services, the junctions, the
// groups, the edges, the layout hints, the registered ids, the data
// structures that the renderer reads, the config), for
//   - every fixture of the corpus,
//   - seeded mutations of those fixtures,
//   - hand-written inputs for the branches of architectureDb.ts.
// A case that mermaid rejects has the error message instead. A case with
// `full` also ran the real draw function: it has the size that drawServices
// gives to each service, or the error of the draw.
//
// The db needs a browser (DOMPurify), so the script runs
// architecture-db-page.js with the esm-eval command of the browser oracle,
// one time.
//
// Usage (in tools/reference): node gen-architecture-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { cases, repoRoot } from './architecture-cases.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.join(repoRoot, 'internal/diagrams/architecture/testdata/db.json.gz');

// ---------------------------------------------------------------------------
// Run the cases in the browser.

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-architecture-db-'));
const argsFile = path.join(tmp, 'args.json');
const rawFile = path.join(tmp, 'raw.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'architecture-db-page.js'), '--args', argsFile, '--out', rawFile], {
  stdio: 'inherit',
});
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));
fs.rmSync(tmp, { recursive: true });

const stats = { cases: 0, dumps: 0, full: 0, errors: 0, drawErrors: 0, otherType: [] };
const records = raw.cases.map((rec, i) => {
  const c = cases[i];
  if (rec.name !== c.name) {
    throw new Error(`case ${i}: ${rec.name} is not ${c.name}`);
  }
  stats.cases++;
  if (rec.error) {
    stats.errors++;
  } else if (rec.type !== 'architecture') {
    stats.otherType.push(`${c.name}: ${rec.type}`);
  }
  if (rec.db) {
    stats.dumps++;
  }
  if (rec.sizes) {
    stats.full++;
  }
  if (rec.drawError) {
    stats.drawErrors++;
  }
  return { ...rec, text: c.text, site: c.site, full: !!c.full };
});
console.error(JSON.stringify(stats));
if (stats.otherType.length > 0) {
  console.error('some cases are not architecture diagrams');
  process.exitCode = 1;
}
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ cases: records }), { level: 9 }));
console.error(`${path.relative(repoRoot, outFile)}: ${records.length} cases, ${fs.statSync(outFile).size} bytes`);
