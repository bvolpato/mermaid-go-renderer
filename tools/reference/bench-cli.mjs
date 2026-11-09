#!/usr/bin/env node
// Compares the run time of the command-line tools mmdc (mermaid-cli) and
// mmdg, and writes the result as JSON. tools/perfdoc puts it into
// docs/PERFORMANCE.md and docs/performance.json. tools/reference/bench.mjs
// runs this script as one step of the whole measurement.
//
//   node bench-cli.mjs --mmdg PATH_OF_MMDG --out FILE.json [--runs 9] [--warmup 3]
//
// Each run is one process that reads a diagram file and writes an SVG or a
// PNG file. mmdc starts a browser in each run. The time is the wall time of
// the process. mmdc runs through mmdc-run.mjs, which uses the Chromium of
// Playwright and holds the lock of the browser oracle.
//
// Each tool first has warm-up runs that do not count. The operating system
// checks a new binary in its first runs, and reads the files of a tool from
// the disk. mmdg runs first: many starts of a browser leave the machine
// busy for a while. The fastest run of each tool counts, because other work
// on the machine can only add time to a run.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

const INPUTS = [
  {
    name: 'Flowchart, 8 nodes',
    file: 'flowchart.mmd',
    text: `flowchart TD
  A[Request] --> B{Cached?}
  B -->|yes| C[Read cache]
  B -->|no| D[Query database]
  D --> E[Render template]
  C --> F[Response]
  E --> G[Write cache]
  G --> F
  F --> H([Done])
`,
  },
  {
    name: 'Sequence diagram, 4 actors',
    file: 'sequence.mmd',
    text: `sequenceDiagram
  participant U as User
  participant W as Web
  participant A as API
  participant D as Database
  U->>W: Open page
  W->>A: GET /items
  A->>D: SELECT
  D-->>A: rows
  A-->>W: JSON
  W-->>U: HTML
  Note over W,A: The page is cached for one minute
`,
  },
  {
    name: 'Class diagram, 5 classes',
    file: 'class.mmd',
    text: `classDiagram
  class Shape {
    +float area()
    +draw()
  }
  class Circle {
    +float radius
  }
  class Square {
    +float side
  }
  class Canvas {
    +add(Shape s)
  }
  class Renderer
  Shape <|-- Circle
  Shape <|-- Square
  Canvas o-- Shape
  Renderer --> Canvas
`,
  },
];

function parseArgs(argv) {
  const opts = {};
  for (let i = 0; i < argv.length; i += 2) {
    opts[argv[i].replace(/^--/, '')] = argv[i + 1];
  }
  return opts;
}

// The fastest run counts, for both tools. Other work on the machine can
// only add time to a run, so the fastest run is the one with the least of
// it.
function fastest(values) {
  return Math.min(...values);
}

async function main() {
  const opts = parseArgs(process.argv.slice(2));
  if (!opts.mmdg || !opts.out) {
    process.stderr.write('usage: bench-cli.mjs --mmdg PATH_OF_MMDG --out FILE.json [--runs 9] [--warmup 3]\n');
    process.exit(2);
  }
  const mmdg = path.resolve(opts.mmdg);
  const runs = Number(opts.runs || 9);
  const warmup = Number(opts.warmup || 3);
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'mmdg-bench-cli-'));
  const cases = [];
  for (const input of INPUTS) {
    await fs.writeFile(path.join(dir, input.file), input.text);
    for (const format of ['svg', 'png']) {
      cases.push({ input, format });
    }
  }

  // mmdg runs first. Many starts of a browser leave the machine busy for a
  // while, and that must not count against the other tool.
  for (let r = 0; r < warmup; r++) {
    spawnSync(mmdg, ['-i', INPUTS[0].file, '-o', path.join(dir, `mmdg-warmup-${r}.svg`), '-q'], { cwd: dir });
  }
  const mmdgTimes = [];
  for (const [c, { input, format }] of cases.entries()) {
    const times = [];
    for (let r = 0; r < runs; r++) {
      const out = path.join(dir, `mmdg-${c}-${r}.${format}`);
      const started = process.hrtime.bigint();
      const result = spawnSync(mmdg, ['-i', input.file, '-o', out, '-q'], { cwd: dir, encoding: 'utf8' });
      const ms = Number(process.hrtime.bigint() - started) / 1e6;
      if (result.status !== 0) {
        throw new Error(`mmdg failed for ${input.file} (${format}): ${result.stderr}`);
      }
      times.push(ms);
    }
    mmdgTimes.push(times);
  }

  // mmdc: all runs in one call of mmdc-run.mjs. The first jobs are the
  // warm-up runs.
  const jobs = [];
  for (let r = 0; r < warmup; r++) {
    jobs.push({ args: ['-i', INPUTS[0].file, '-o', `mmdc-warmup-${r}.svg`, '-q'], cwd: dir });
  }
  for (const [c, { input, format }] of cases.entries()) {
    for (let r = 0; r < runs; r++) {
      jobs.push({ args: ['-i', input.file, '-o', `mmdc-${c}-${r}.${format}`, '-q'], cwd: dir });
    }
  }
  const jobsFile = path.join(dir, 'jobs.json');
  const resultsFile = path.join(dir, 'results.json');
  await fs.writeFile(jobsFile, JSON.stringify(jobs));
  const run = spawnSync(process.execPath, [path.join(HERE, 'mmdc-run.mjs'), jobsFile, resultsFile], { stdio: 'inherit' });
  if (run.status !== 0) {
    throw new Error(`mmdc-run.mjs ended with status ${run.status}`);
  }
  const results = JSON.parse(await fs.readFile(resultsFile, 'utf8')).slice(warmup);

  const rows = [];
  for (const [c, { input, format }] of cases.entries()) {
    const mmdcMs = [];
    for (let r = 0; r < runs; r++) {
      const result = results[c * runs + r];
      if (result.code !== 0) {
        throw new Error(`mmdc failed for ${input.file} (${format}): ${result.stderr}`);
      }
      mmdcMs.push(result.ms);
    }
    rows.push({ input: input.name, output: format.toUpperCase(), mmdc: fastest(mmdcMs), mmdg: Math.round(fastest(mmdgTimes[c]) * 10) / 10 });
  }
  await fs.rm(dir, { recursive: true, force: true });

  const mmdcVersion = JSON.parse(
    await fs.readFile(path.join(HERE, 'node_modules/@mermaid-js/mermaid-cli/package.json'), 'utf8')
  ).version;
  await fs.writeFile(opts.out, JSON.stringify({ mmdc: mmdcVersion, runs, warmup, rows }, null, 1) + '\n');
  for (const row of rows) {
    process.stderr.write(`${row.input}, ${row.output}: mmdc ${row.mmdc} ms, mmdg ${row.mmdg} ms\n`);
  }
}

await main();
