// Writes internal/diagrams/timeline/testdata/extra-ref.json.gz and
// internal/diagrams/journey/testdata/extra-ref.json.gz: the SVG that real
// mermaid renders in Chromium for the fixtures in testdata/extra of the two
// packages. These fixtures cover config options and branches of the
// renderers that the corpus does not have: the vertical timeline with the
// redux themes, the margins, the font size, more sections than colors, the
// text placements of the user journey, its colors and sizes, scores that
// are not numbers, and the loose security level.
//
// The script runs the render command of the browser oracle, one page for
// each fixture, as for the reference output of the corpus.
//
// Usage (in tools/reference): node gen-timeline-journey-extra.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

for (const name of ['timeline', 'journey']) {
  const testdata = path.join(repoRoot, 'internal/diagrams', name, 'testdata');
  const dir = path.join(testdata, 'extra');
  const files = fs
    .readdirSync(dir)
    .filter((f) => f.endsWith('.mmd'))
    .sort()
    .map((f) => path.join(dir, f));
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), `${name}-extra-`));
  execFileSync(
    process.execPath,
    // --quiet is the last argument: the oracle reads the word after a flag
    // as the value of the flag.
    [path.join(here, 'oracle.mjs'), 'render', '--out', tmp, '--base', dir, ...files, '--quiet'],
    { stdio: 'inherit' }
  );
  const svgs = {};
  for (const file of files) {
    const base = path.basename(file, '.mmd');
    const svgFile = path.join(tmp, base + '.svg');
    if (!fs.existsSync(svgFile)) {
      throw new Error(`mermaid does not render ${file}`);
    }
    svgs[base] = fs.readFileSync(svgFile, 'utf8');
  }
  fs.rmSync(tmp, { recursive: true, force: true });
  const data = JSON.stringify({ mermaid: '12.2.0', svgs }, null, 1) + '\n';
  const outFile = path.join(testdata, 'extra-ref.json.gz');
  fs.writeFileSync(outFile, zlib.gzipSync(data, { level: 9 }));
  console.error(`${path.relative(repoRoot, outFile)}: ${files.length} fixtures`);
}
