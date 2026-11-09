// Writes the golden files for the standalone SVG that mmdg writes: what
// Chromium returns from new XMLSerializer().serializeToString(svg) after the
// steps of mermaid-cli (renderMermaid in src/index.js of mermaid-cli).
//
//   internal/dom/testdata/xml-serialize.json   hand-written cases (see
//       xml-serialize-cases.mjs): the markup, the DOM tree of the svg
//       element, and its XML serialization
//   testdata/standalone/backgrounds.json       svg.style.backgroundColor for
//       many values of --backgroundColor and several style attributes
//   testdata/standalone/<environment>.json     for every reference SVG of the
//       corpus, the hash of the input and of the output of each variant
//   testdata/standalone/sample.json.gz         the full input and output for
//       a sample: the smallest reference of every diagram type and of every
//       kind of label
//
// The script runs xml-serialize-page.js with the eval command of the browser
// oracle, in batches of at most 150 inputs, so that the lock of the oracle
// is free between the batches.
//
// Usage (in tools/reference):
//   node gen-xml-serialize.mjs cases
//   node gen-xml-serialize.mjs corpus macos ../../.parity/ref/mermaid-e2e
//   node gen-xml-serialize.mjs corpus linux ../../.parity/ref-linux/mermaid-e2e
//   node gen-xml-serialize.mjs sample ../../.parity/ref/mermaid-e2e
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { cases, backgrounds, backgroundRoots } from './xml-serialize-cases.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const BATCH = 150;

// The CSS file of the "css" variant: characters that XML escapes.
export const CSS_FILE = '.node > rect { fill: #eee; }\n.label::after { content: "<&>"; }\n';
// The variants of the corpus golden. "white" is mermaid-cli with its default
// flags. "css" is the --cssFile step of mermaid-cli 11. "size" is --size 500
// of mermaid-cli 12.
export const VARIANTS = [
  { name: 'plain' },
  { name: 'white', background: 'white' },
  { name: 'transparent', background: 'transparent' },
  { name: 'css', background: 'white', css: CSS_FILE },
  { name: 'size', background: 'white', size: 500 },
];

function evalPage(args) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'xml-serialize-'));
  try {
    const argsFile = path.join(dir, 'args.json');
    const outFile = path.join(dir, 'out.json');
    fs.writeFileSync(argsFile, JSON.stringify(args));
    execFileSync(
      process.execPath,
      ['oracle.mjs', 'eval', 'xml-serialize-page.js', '--bare', '--args', argsFile, '--out', outFile],
      { cwd: here, stdio: ['ignore', 'inherit', 'inherit'] }
    );
    return JSON.parse(fs.readFileSync(outFile, 'utf8'));
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
}

// hash is the first 16 hex digits of the SHA-256 of the UTF-8 text.
const hash = (text) => crypto.createHash('sha256').update(text, 'utf8').digest('hex').slice(0, 16);

function walk(dir, out = []) {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      walk(p, out);
    } else if (entry.name.endsWith('.svg')) {
      out.push(p);
    }
  }
  return out;
}

function writeJSON(file, value) {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify(value, null, 1) + '\n');
  process.stderr.write(`wrote ${path.relative(repoRoot, file)}\n`);
}

function genCases() {
  const results = evalPage({ cases, variants: [{ name: 'plain' }], trees: true });
  const golden = results.map((r, i) => {
    const entry = { name: r.name, svg: cases[i].svg };
    if (cases[i].script) {
      entry.script = cases[i].script;
    }
    entry.tree = r.tree;
    entry.xml = r.out.plain;
    return entry;
  });
  writeJSON(path.join(repoRoot, 'internal/dom/testdata/xml-serialize.json'), golden);

  const variants = backgrounds.map((b, i) => ({ name: String(i), background: b }));
  const bgResults = evalPage({
    cases: backgroundRoots.map((svg, i) => ({ name: String(i), svg })),
    variants,
  });
  const bgGolden = [];
  bgResults.forEach((r, i) => {
    backgrounds.forEach((background, j) => {
      bgGolden.push({ svg: backgroundRoots[i], background, xml: r.out[String(j)] });
    });
  });
  writeJSON(path.join(repoRoot, 'testdata/standalone/backgrounds.json'), bgGolden);
}

function genCorpus(environment, refDir) {
  const files = walk(path.resolve(refDir)).sort();
  const fixtures = {};
  for (let i = 0; i < files.length; i += BATCH) {
    const batch = files.slice(i, i + BATCH);
    const inputs = batch.map((file) => ({
      name: path.relative(path.resolve(refDir), file).replace(/\.svg$/, '').split(path.sep).join('/'),
      svg: fs.readFileSync(file, 'utf8'),
    }));
    const results = evalPage({ cases: inputs, variants: VARIANTS });
    results.forEach((r, j) => {
      const entry = { in: hash(inputs[j].svg) };
      for (const v of VARIANTS) {
        entry[v.name] = hash(r.out[v.name]);
      }
      fixtures[r.name] = entry;
    });
    process.stderr.write(`${environment}: ${Math.min(i + BATCH, files.length)}/${files.length}\n`);
  }
  const golden = {
    environment,
    hash: 'the first 16 hex digits of the SHA-256',
    variants: VARIANTS,
    fixtures,
  };
  fs.mkdirSync(path.join(repoRoot, 'testdata/standalone'), { recursive: true });
  const file = path.join(repoRoot, `testdata/standalone/${environment}.json`);
  // One fixture per line keeps the file small and the diffs readable.
  const lines = Object.keys(fixtures).map((name) => ` ${JSON.stringify(name)}: ${JSON.stringify(fixtures[name])}`);
  const head = JSON.stringify({ environment: golden.environment, hash: golden.hash, variants: golden.variants });
  fs.writeFileSync(file, `${head.slice(0, -1)},\n"fixtures": {\n${lines.join(',\n')}\n}}\n`);
  process.stderr.write(`wrote ${path.relative(repoRoot, file)}: ${files.length} fixtures\n`);
}

// The kinds of label and content that the sample must cover: a name and a
// test of the reference SVG.
const FEATURES = [
  ['html-label', (s) => s.includes('<foreignObject')],
  ['svg-label', (s) => /<text[^>]*>\s*<tspan/.test(s)],
  ['br', (s) => s.includes('<br>')],
  ['img', (s) => s.includes('<img ')],
  ['icon', (s) => /<i class="fa/.test(s)],
  ['nbsp', (s) => s.includes('&nbsp;')],
  ['markdown', (s) => s.includes('<strong>') || s.includes('<em>')],
  ['math', (s) => s.includes('<math')],
  ['xlink', (s) => s.includes('xlink:href=')],
  ['link', (s) => /<a [^>]*href=/.test(s)],
  ['image', (s) => s.includes('<image ')],
  ['nested-svg', (s) => s.indexOf('<svg', 4) > 0],
  ['entity', (s) => s.includes('&amp;') || s.includes('&lt;')],
  ['non-bmp', (s) => /[\u{10000}-\u{10FFFF}]/u.test(s)],
  ['comment', (s) => s.includes('<!--')],
  ['title', (s) => /<title[ >]/.test(s)],
  ['empty-html', (s) => /<(span|div|p)[^>]*><\/\1>/.test(s)],
];

function genSample(refDir) {
  const files = walk(path.resolve(refDir)).sort();
  const smallest = new Map(); // key -> { file, size }
  const consider = (key, file, size) => {
    const best = smallest.get(key);
    if (!best || size < best.size) {
      smallest.set(key, { file, size });
    }
  };
  for (const file of files) {
    const svg = fs.readFileSync(file, 'utf8');
    const meta = JSON.parse(fs.readFileSync(file.replace(/\.svg$/, '.json'), 'utf8'));
    consider(`type:${meta.diagramType}`, file, svg.length);
    for (const [name, test] of FEATURES) {
      if (test(svg)) {
        consider(`label:${name}`, file, svg.length);
      }
    }
  }
  const chosen = new Map(); // file -> [keys]
  for (const [key, { file }] of smallest) {
    chosen.set(file, [...(chosen.get(file) || []), key]);
  }
  const inputs = [...chosen.keys()].sort().map((file) => ({
    name: path.relative(path.resolve(refDir), file).replace(/\.svg$/, '').split(path.sep).join('/'),
    svg: fs.readFileSync(file, 'utf8'),
    covers: chosen.get(file).sort(),
  }));
  const sample = [];
  for (let i = 0; i < inputs.length; i += BATCH) {
    const batch = inputs.slice(i, i + BATCH);
    const results = evalPage({ cases: batch.map(({ name, svg }) => ({ name, svg })), variants: VARIANTS });
    results.forEach((r, j) => {
      sample.push({ name: r.name, covers: batch[j].covers, svg: batch[j].svg, out: r.out });
    });
  }
  const file = path.join(repoRoot, 'testdata/standalone/sample.json.gz');
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify({ variants: VARIANTS, sample }), { level: 9 }));
  const keys = [...smallest.keys()];
  process.stderr.write(
    `wrote ${path.relative(repoRoot, file)}: ${sample.length} references, ` +
      `${keys.filter((k) => k.startsWith('type:')).length} diagram types, ` +
      `${keys.filter((k) => k.startsWith('label:')).length} of ${FEATURES.length} label kinds\n`
  );
}

const [command, a, b] = process.argv.slice(2);
if (command === 'cases') {
  genCases();
} else if (command === 'corpus' && a && b) {
  genCorpus(a, b);
} else if (command === 'sample' && a) {
  genSample(a);
} else {
  process.stderr.write('usage: gen-xml-serialize.mjs cases | corpus <environment> <ref dir> | sample <ref dir>\n');
  process.exit(2);
}
