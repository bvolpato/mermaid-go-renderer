// Records what mermaid's ELK layout does for corpus fixtures, as golden data
// for internal/ru/layout/elk.
//
//   node gen-ru-elk.mjs [--raw DIR] [--pack-only] [--batch N] [--keep N]
//
// It renders the fixtures of the diagram families that use the unified
// renderer with the real mermaid in Chromium (oracle.mjs esm-eval with
// probes/ru-elk.js) and writes
// internal/ru/layout/elk/testdata/fixtures.jsonl.gz: one JSON line per
// fixture that ran an ELK layout. See probes/ru-elk.js for the fields.
//
// The browser runs in batches, so that the oracle lock is free between them.
// --raw keeps the output of the batches in a directory, and --pack-only
// builds the golden file from that directory without a browser.
//
// Every fixture that ran an ELK layout is kept: about 1100 fixtures in
// 1.4 MB. --keep bounds the number of fixtures in the golden file. The
// choice is deterministic: every fixture with a rare feature is kept, and
// the rest is thinned evenly per family.
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const corpus = path.join(root, 'testdata/corpus/mermaid-e2e');
const outFile = path.join(root, 'internal/ru/layout/elk/testdata/fixtures.jsonl.gz');

const FAMILIES = [
  'flowchart',
  'class-diagram',
  'state-diagram',
  'state-diagram-v2',
  'er-diagram',
  'requirement',
  'usecase',
  'agentflow',
  'conf-and-directives',
  'katex',
];

const opts = { batch: 100, keep: Infinity };
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--raw') {
    opts.raw = argv[++i];
  } else if (argv[i] === '--pack-only') {
    opts.packOnly = true;
  } else if (argv[i] === '--batch') {
    opts.batch = Number(argv[++i]);
  } else if (argv[i] === '--keep') {
    opts.keep = Number(argv[++i]);
  } else {
    console.error('usage: node gen-ru-elk.mjs [--raw DIR] [--pack-only] [--batch N] [--keep N]');
    process.exit(2);
  }
}

function walk(dir, out) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0))) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      walk(p, out);
    } else if (e.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
}

const files = [];
for (const family of FAMILIES) {
  const dir = path.join(corpus, family);
  if (fs.existsSync(dir)) {
    walk(dir, files);
  }
}
const names = files.map((f) => path.relative(corpus, f).split(path.sep).join('/'));

const rawDir = opts.raw || fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-ru-elk-'));
fs.mkdirSync(rawDir, { recursive: true });
const batches = [];
for (let i = 0; i < names.length; i += opts.batch) {
  batches.push(names.slice(i, i + opts.batch));
}

if (!opts.packOnly) {
  batches.forEach((batch, bi) => {
    const rawFile = path.join(rawDir, `batch-${String(bi).padStart(3, '0')}.json`);
    if (fs.existsSync(rawFile)) {
      return;
    }
    const casesFile = path.join(rawDir, `cases-${bi}.json`);
    fs.writeFileSync(
      casesFile,
      JSON.stringify({ cases: batch.map((name) => ({ name, text: fs.readFileSync(path.join(corpus, name), 'utf8') })) })
    );
    const tmp = rawFile + '.tmp';
    execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/ru-elk.js'), '--args', casesFile, '--out', tmp], {
      stdio: 'inherit',
    });
    fs.renameSync(tmp, rawFile);
    fs.rmSync(casesFile);
    console.error(`batch ${bi + 1}/${batches.length} done`);
  });
}

// Read the batches.
const records = [];
batches.forEach((batch, bi) => {
  const rawFile = path.join(rawDir, `batch-${String(bi).padStart(3, '0')}.json`);
  if (!fs.existsSync(rawFile)) {
    console.error(`missing ${rawFile}`);
    process.exit(1);
  }
  records.push(...JSON.parse(fs.readFileSync(rawFile, 'utf8')));
});

const usable = records.filter((r) => r.renders && r.renders.some((x) => x.elk.length > 0));
const stats = { fixtures: records.length, elk: usable.length, errors: 0, multiCall: 0, random: 0, notSame: 0, configChanged: 0, captureErrors: 0 };
for (const r of usable) {
  const calls = r.renders.flatMap((x) => x.elk);
  if (r.error) {
    stats.errors++;
  }
  if (r.elkCalls !== 1) {
    stats.multiCall++;
  }
  if (calls.some((c) => c.random)) {
    stats.random++;
  }
  if (calls.some((c) => c.same === false)) {
    stats.notSame++;
  }
  if (r.renders.some((x) => x.configSame === false)) {
    stats.configChanged++;
  }
  if (calls.some((c) => c.captureError)) {
    stats.captureErrors++;
  }
}

// Features that decide which fixtures are always kept.
function features(r) {
  const f = new Set();
  for (const x of r.renders) {
    if (x.layout !== 'elk') {
      f.add('layout:' + x.layout);
    }
    for (const c of x.elk) {
      if (c.error) {
        f.add('elk-error');
      }
      if (!c.data || !c.in) {
        continue;
      }
      const nodes = c.data.nodes;
      const groups = nodes.filter((n) => n.isGroup);
      if (groups.some((n) => n.parentId)) {
        f.add('nested-group');
      }
      if (groups.some((n) => n.dir)) {
        f.add('group-dir');
      }
      if (groups.some((n) => n.metadata && n.metadata.algorithm)) {
        f.add('group-algorithm');
      }
      if (nodes.some((n) => n.spreadPorts)) {
        f.add('spread-ports');
      }
      if (c.data.edges.some((e) => e.terminalLabelSizes)) {
        f.add('terminal-labels');
      }
      if (c.data.edges.some((e) => e.start === e.end)) {
        f.add('self-loop');
      }
      if (c.in.includes('"layoutReversed":true')) {
        f.add('feedback-edge');
      }
      if (c.in.includes('layerConstraint')) {
        f.add('entry-constraint');
      }
      if (c.out && !c.out.includes('"sections"')) {
        f.add('no-sections');
      }
      const elkCfg = (c.data.config && c.data.config.elk) || {};
      for (const k of ['preset', 'nodePlacementStrategy', 'nodePlacementAlignment', 'cycleBreakingStrategy', 'layeringStrategy', 'lineHops', 'straightenEdges', 'keepEntryNodeOnTop', 'orientFeedbackEdges']) {
        if (elkCfg[k] !== undefined) {
          f.add('elk.' + k + '=' + elkCfg[k]);
        }
      }
      if (elkCfg.mergeEdges) {
        f.add('elk.mergeEdges');
      }
      if (c.data.direction && c.data.direction !== 'TB' && c.data.direction !== 'TD') {
        f.add('dir:' + c.data.direction);
      }
    }
  }
  return f;
}

// Keep every fixture that has a feature which fewer than `rare` fixtures
// have. Thin the others evenly per family, by the hash of the name.
const rare = 12;
const featureCount = new Map();
const featureOf = new Map();
for (const r of usable) {
  const f = features(r);
  featureOf.set(r.name, f);
  for (const k of f) {
    featureCount.set(k, (featureCount.get(k) || 0) + 1);
  }
}
const hash = (s) => crypto.createHash('sha1').update(s).digest().readUInt32BE(0);
const always = usable.filter((r) => [...featureOf.get(r.name)].some((k) => featureCount.get(k) < rare));
const rest = usable.filter((r) => !always.includes(r));
const want = Number.isFinite(opts.keep) ? Math.max(0, opts.keep - always.length) : rest.length;
const byFamily = new Map();
for (const r of rest) {
  const fam = r.name.split('/')[0];
  if (!byFamily.has(fam)) {
    byFamily.set(fam, []);
  }
  byFamily.get(fam).push(r);
}
const kept = new Set(always.map((r) => r.name));
for (const [, list] of byFamily) {
  const n = Number.isFinite(opts.keep) ? Math.min(list.length, Math.max(8, Math.round((want * list.length) / Math.max(1, rest.length)))) : list.length;
  const ranked = [...list].sort((a, b) => hash(a.name) - hash(b.name));
  for (const r of ranked.slice(0, n)) {
    kept.add(r.name);
  }
}

const lines = usable.filter((r) => kept.has(r.name)).map((r) => JSON.stringify(r));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
const gz = zlib.gzipSync(lines.join('\n') + '\n', { level: 9 });
fs.writeFileSync(outFile, gz);
const perFamily = {};
for (const r of usable) {
  const fam = r.name.split('/')[0];
  perFamily[fam] ||= { elk: 0, kept: 0 };
  perFamily[fam].elk++;
  if (kept.has(r.name)) {
    perFamily[fam].kept++;
  }
}
console.error(JSON.stringify(stats));
console.error(JSON.stringify(perFamily));
console.error([...featureCount.entries()].sort((a, b) => a[1] - b[1]).map(([k, n]) => `${k}:${n}`).join(' '));
console.error(`${lines.length} fixtures (${always.length} with a rare feature), ${gz.length} bytes -> ${path.relative(root, outFile)}`);
