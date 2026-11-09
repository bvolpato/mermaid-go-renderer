// Writes internal/diagrams/architecture/testdata/layout.json.gz: what the
// architecture renderer of real mermaid gives to cytoscape and what it
// reads back, in Chromium, for every fixture of the corpus and for the
// hand-written inputs of architecture-cases.mjs that are marked `full`, and
// the SVG that mermaid.render returns for each of them.
//
//   node gen-architecture-layout.mjs [--raw DIR] [--pack-only]
//
// The script runs the browser oracle two times:
//
//   1. `oracle.mjs render` renders the hand-written inputs, each in a new
//      page, as the reference renders of the corpus were made. This gives
//      the reference SVG of each input, or the error of mermaid.
//   2. `oracle.mjs esm-eval probes/cytoscape.js` renders all cases in one
//      page with hooks on cytoscape, and records for each cytoscape instance
//      the style sheet, the cy.add calls, the layout options, the
//      measureText calls of the canvas and the values that mermaid reads
//      after the layout. See probes/cytoscape.js for the fields.
//
// The hooks must not change a result. The script proves it: the SVG of each
// render with hooks must be the bytes of the reference SVG. The only
// difference that is allowed is the page counter in the ids of icon parts.
//
// The probe evaluates the function options of the layout for every edge. For
// an edge from a node to itself that call fails, and the render of mermaid
// without hooks does not make it. Such a case keeps its reference SVG and has
// no record of the layout.
//
// --raw keeps the files of the browser in a directory, and --pack-only
// builds the golden file from that directory without a browser.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { cases as allCases, repoRoot } from './architecture-cases.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const refDir = path.join(repoRoot, '.parity/ref/mermaid-e2e');
const outFile = path.join(repoRoot, 'internal/diagrams/architecture/testdata/layout.json.gz');

let rawDir;
let packOnly = false;
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--raw') {
    rawDir = argv[++i];
  } else if (argv[i] === '--pack-only') {
    packOnly = true;
  } else {
    console.error('usage: node gen-architecture-layout.mjs [--raw DIR] [--pack-only]');
    process.exit(2);
  }
}
if (packOnly && !rawDir) {
  console.error('--pack-only needs --raw DIR');
  process.exit(2);
}
const tmp = rawDir || fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-architecture-layout-'));
fs.mkdirSync(tmp, { recursive: true });

// The probe has no site config.
const cases = allCases.filter((c) => c.full && !c.site);

// ---------------------------------------------------------------------------
// 1. The reference renders of the hand-written inputs.

const srcDir = path.join(tmp, 'src');
const renderDir = path.join(tmp, 'ref');
if (!packOnly) {
  fs.rmSync(srcDir, { recursive: true, force: true });
  fs.rmSync(renderDir, { recursive: true, force: true });
  fs.mkdirSync(srcDir, { recursive: true });
  const extraFiles = [];
  for (const c of cases) {
    if (!c.corpus) {
      const file = path.join(srcDir, c.name + '.mmd');
      fs.writeFileSync(file, c.text);
      extraFiles.push(file);
    }
  }
  const listFile = path.join(tmp, 'list.txt');
  fs.writeFileSync(listFile, extraFiles.join('\n') + '\n');
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'render', '--out', renderDir, '--base', srcDir, '--list', listFile, '--quiet'], {
    stdio: 'inherit',
  });
}

function reference(c) {
  const base = c.corpus ? path.join(refDir, c.corpus) : path.join(renderDir, c.name);
  const meta = JSON.parse(fs.readFileSync(base + '.json', 'utf8'));
  if (meta.error) {
    return { error: meta.error };
  }
  return { svg: fs.readFileSync(base + '.svg', 'utf8'), randomBefore: meta.randomBefore, randomCalls: meta.randomCalls };
}

// ---------------------------------------------------------------------------
// 2. The renders with hooks on cytoscape.

const casesFile = path.join(tmp, 'cases.json');
const rawFile = path.join(tmp, 'raw.json');
if (!packOnly) {
  fs.writeFileSync(casesFile, JSON.stringify({ cases: cases.map(({ name, text }) => ({ name, text, randomBefore: 1 })) }));
  execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/cytoscape.js'), '--args', casesFile, '--out', rawFile], {
    stdio: 'inherit',
  });
}
const raw = JSON.parse(fs.readFileSync(rawFile, 'utf8'));

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

const stats = { cases: 0, corpus: 0, layouts: 0, errors: 0, probeFails: [], svgDiffers: [], errorDiffers: [], noResult: [] };
const records = [];
raw.cases.forEach((rec, i) => {
  const c = cases[i];
  if (rec.name !== c.name) {
    throw new Error(`case ${i}: ${rec.name} is not ${c.name}`);
  }
  stats.cases++;
  if (c.corpus) {
    stats.corpus++;
  }
  const ref = reference(c);
  const out = { name: c.name, text: c.text, corpus: !!c.corpus };
  const selfEdge = (rec.instances[0]?.adds ?? []).some((a) => a.group === 'edges' && a.data.source === a.data.target);
  if (!ref.error && rec.error && selfEdge) {
    // The call of the probe fails, not mermaid.
    stats.probeFails.push(c.name);
    out.svg = ref.svg;
    out.randomBefore = ref.randomBefore;
    out.randomCalls = ref.randomCalls;
    out.probeError = rec.error;
    records.push(out);
    return;
  }
  if (ref.error || rec.error) {
    stats.errors++;
    // A TypeError of V8 quotes the source text, which differs between the
    // two builds of mermaid that the two commands of the oracle load.
    const plain = (msg) => (msg === undefined ? msg : msg.replace(/ of '[^']*' as it is undefined/, " of '...' as it is undefined"));
    if (plain(ref.error) !== plain(rec.error)) {
      stats.errorDiffers.push(`${c.name}: ${rec.error} is not ${ref.error}`);
    }
    out.error = ref.error;
    records.push(out);
    return;
  }
  if (rec.type !== 'architecture') {
    throw new Error(`${c.name}: the type is ${rec.type}`);
  }
  out.sameSvg = iconIds(rec.svg) === iconIds(ref.svg);
  if (!out.sameSvg) {
    stats.svgDiffers.push(c.name);
  }
  out.svg = ref.svg;
  out.randomBefore = ref.randomBefore;
  out.randomCalls = ref.randomCalls;
  if (rec.instances.length !== 1) {
    throw new Error(`${c.name}: ${rec.instances.length} cytoscape instances`);
  }
  const inst = rec.instances[0];
  if (!inst.result) {
    stats.noResult.push(`${c.name}: ${inst.resultError || 'no read'}`);
  }
  if (!rec.architecture) {
    stats.noResult.push(`${c.name}: ${rec.architectureError || 'no architecture input'}`);
  }
  stats.layouts++;
  Object.assign(out, {
    style: inst.style,
    initial: inst.initial,
    adds: inst.adds,
    layouts: inst.layouts,
    measure: inst.measure,
    random: inst.random,
    result: inst.result,
    architecture: rec.architecture,
  });
  records.push(out);
});

console.error(JSON.stringify(stats, null, 1));
if (stats.svgDiffers.length > 0 || stats.errorDiffers.length > 0 || stats.noResult.length > 0) {
  console.error('the capture is not a faithful copy of the reference renders');
  process.exitCode = 1;
}
if (!rawDir) {
  fs.rmSync(tmp, { recursive: true });
}
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ env: raw.env, records }), { level: 9 }));
console.error(`${path.relative(repoRoot, outFile)}: ${records.length} records, ${fs.statSync(outFile).size} bytes`);
