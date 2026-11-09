// Records the cytoscape layouts of real mermaid in Chromium for every mindmap
// and architecture fixture of the corpus, as golden data for
// internal/cytoscape.
//
//   node gen-cytoscape.mjs [--raw FILE] [--pack-only]
//
// It finds the fixtures by the diagramType of the reference render
// (.parity/ref/mermaid-e2e/**/*.json), renders them in one browser run
// (oracle.mjs esm-eval with probes/cytoscape.js) and writes
// internal/cytoscape/testdata/layouts.json.gz: the environment values of the
// page and one record per cytoscape instance. See probes/cytoscape.js for the
// fields.
//
// The probe changes the page: it wraps methods of cytoscape and resets
// module state between the cases. The script proves that this does not
// change a result: the SVG of each render must be the bytes of the reference
// SVG, which a page without hooks produced, one page per diagram. The only
// difference that is allowed is the page counter in the ids of icon parts.
//
// --raw keeps the output of the browser in a file, and --pack-only builds
// the golden file from that file without a browser.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e');
const ref = path.join(root, '.parity/ref/mermaid-e2e');
const outFile = path.join(root, 'internal/cytoscape/testdata/layouts.json.gz');

const opts = {};
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--raw') {
    opts.raw = argv[++i];
  } else if (argv[i] === '--pack-only') {
    opts.packOnly = true;
  } else {
    console.error('usage: node gen-cytoscape.mjs [--raw FILE] [--pack-only]');
    process.exit(2);
  }
}

function walk(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      walk(p, out);
    } else if (e.name.endsWith('.json')) {
      out.push(p);
    }
  }
  return out;
}

const cases = [];
for (const file of walk(ref, [])) {
  const meta = JSON.parse(fs.readFileSync(file, 'utf8'));
  if (meta.diagramType !== 'mindmap' && meta.diagramType !== 'architecture') {
    continue;
  }
  const name = path.relative(ref, file).split(path.sep).join('/').replace(/\.json$/, '');
  cases.push({
    name,
    type: meta.diagramType,
    text: fs.readFileSync(path.join(corpus, name + '.mmd'), 'utf8'),
    randomBefore: meta.randomBefore,
    randomCalls: meta.randomCalls,
  });
}

// iconIds renumbers the ids that @iconify/utils gives to the parts of an
// icon. They end with a counter of the page, and the probe renders all
// cases in one page.
function iconIds(svg) {
  const seen = new Map();
  return svg.replace(/IconifyId[0-9a-f]+/g, (id) => {
    if (!seen.has(id)) {
      seen.set(id, `IconifyId#${seen.size}`);
    }
    return seen.get(id);
  });
}

const rawFile = opts.raw || path.join(fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-cytoscape-')), 'raw.json');
if (!opts.packOnly) {
  const casesFile = rawFile + '.cases.json';
  fs.writeFileSync(casesFile, JSON.stringify({ cases: cases.map(({ name, text, randomBefore }) => ({ name, text, randomBefore })) }));
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/cytoscape.js'), '--args', casesFile, '--out', rawFile], {
    stdio: 'inherit',
  });
  fs.rmSync(casesFile);
}
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));

const stats = { mindmap: 0, architecture: 0, instances: { 'cose-bilkent': 0, fcose: 0, none: 0 }, svgDiffers: [], errors: [], noResult: [] };
const records = [];
raw.cases.forEach((rec, i) => {
  const c = cases[i];
  if (rec.name !== c.name) {
    throw new Error(`case ${i}: ${rec.name} is not ${c.name}`);
  }
  stats[c.type]++;
  if (rec.error) {
    stats.errors.push(`${c.name}: ${rec.error}`);
    return;
  }
  const refSvg = fs.readFileSync(path.join(ref, c.name + '.svg'), 'utf8');
  const sameSvg = iconIds(rec.svg) === iconIds(refSvg);
  if (!sameSvg) {
    stats.svgDiffers.push(c.name);
  }
  rec.instances.forEach((inst, k) => {
    const layout = (inst.layouts.find((l) => l.options.name !== undefined) || { options: {} }).options.name || 'none';
    stats.instances[layout] = (stats.instances[layout] || 0) + 1;
    if (!inst.result) {
      stats.noResult.push(`${c.name}#${k}: ${inst.resultError || 'no read'}`);
    }
    if (c.type === 'architecture' && !rec.architecture) {
      stats.noResult.push(`${c.name}#${k}: ${rec.architectureError || 'no architecture input'}`);
    }
    records.push({
      name: rec.instances.length > 1 ? `${c.name}#${k}` : c.name,
      type: c.type,
      layout,
      sameSvg,
      randomBefore: c.randomBefore,
      architecture: rec.architecture,
      ...inst,
    });
  });
  if (rec.instances.length === 0) {
    stats.instances.none++;
  }
});

console.error(JSON.stringify(stats, null, 1));
if (stats.svgDiffers.length > 0 || stats.errors.length > 0 || stats.noResult.length > 0) {
  console.error('the capture is not a faithful copy of the reference renders');
  process.exitCode = 1;
}
const golden = { env: raw.env, seedState: raw.seedState, records };
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden), { level: 9 }));
console.error(`${path.relative(root, outFile)}: ${records.length} records, ${fs.statSync(outFile).size} bytes`);
