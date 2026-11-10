// Writes the golden files of cmd/mmdg from real mermaid-cli (mmdc) runs:
//
//   cmd/mmdg/testdata/mmdc/corpus.json   for a sample of the corpus and
//       several sets of options, the hash of every .svg file that mmdc wrote
//   cmd/mmdg/testdata/mmdc/png.json      the size of the PNG files that mmdc
//       wrote for small diagrams, with scales, --size and backgrounds
//   cmd/mmdg/testdata/mmdc/cli.json      the exit code and the messages of
//       mmdc for wrong arguments, and its Markdown output
//
// mmdc runs through mmdc-run.mjs, which holds the lock of the browser
// oracle. Each call of mmdc-run.mjs has a few jobs. A sample of the corpus
// is one Markdown file, so that one browser renders all its diagrams.
//
// mmdc measures text with the fonts of the machine, so the golden files of
// the corpus and of the PNG sizes are valid for the fonts of the machine
// that wrote them. The tests compare with mmdg --fonts system and skip on a
// machine with other fonts.
//
// Usage (in tools/reference):
//   node gen-mmdc-golden.mjs corpus | png | cli [--out DIR] [--filter TEXT]
//
// Use --out to compare a new oracle with historical goldens before
// replacing them. A version change alone does not invalidate a golden.
// --filter selects corpus fixture names and requires --out.
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const [command, ...args] = process.argv.slice(2);
const usage = () => {
  process.stderr.write('usage: gen-mmdc-golden.mjs corpus | png | cli [--out DIR] [--filter TEXT (corpus only, requires --out)]\n');
  process.exit(2);
};
if (!['corpus', 'png', 'cli'].includes(command)) usage();
const values = new Map();
for (let i = 0; i < args.length; i += 2) {
  const flag = args[i];
  if (!['--out', '--filter'].includes(flag) || !args[i + 1] || values.has(flag)) usage();
  values.set(flag, args[i + 1]);
}
if (values.has('--filter') && (command !== 'corpus' || !values.has('--out'))) usage();
const outDir = values.has('--out') ? path.resolve(values.get('--out')) : path.join(repoRoot, 'cmd/mmdg/testdata/mmdc');
const corpusDir = path.join(repoRoot, 'testdata/corpus/mermaid-e2e');
const refDir = path.join(repoRoot, '.parity/ref/mermaid-e2e');
const cli = fileURLToPath(import.meta.resolve('@mermaid-js/mermaid-cli'));
const cliRequire = createRequire(cli);
const versionFrom = (file) => JSON.parse(fs.readFileSync(file, 'utf8')).version;
const packageVersion = (name) => versionFrom(cliRequire.resolve(`${name}/package.json`));
const oracleVersions = {
  mmdc: versionFrom(path.resolve(path.dirname(cli), '../package.json')),
  mermaid: packageVersion('mermaid'),
  puppeteer: packageVersion('puppeteer'),
  playwright: packageVersion('playwright'),
  katex: packageVersion('katex'),
};

const hash = (data) => crypto.createHash('sha256').update(data).digest('hex').slice(0, 16);

// mmdc runs the jobs with the lock of the oracle held, a few jobs per call.
function mmdc(jobs) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-jobs-'));
  const results = [];
  try {
    for (let i = 0; i < jobs.length; i += 6) {
      const jobsFile = path.join(dir, `jobs-${i}.json`);
      const resultsFile = path.join(dir, `results-${i}.json`);
      fs.writeFileSync(jobsFile, JSON.stringify(jobs.slice(i, i + 6)));
      execFileSync(process.execPath, ['mmdc-run.mjs', jobsFile, resultsFile], {
        cwd: here,
        stdio: ['ignore', 'inherit', 'inherit'],
      });
      results.push(...JSON.parse(fs.readFileSync(resultsFile, 'utf8')));
    }
  } finally {
    fs.rmSync(dir, { recursive: true, force: true });
  }
  return results;
}

function writeJSON(name, value) {
  fs.mkdirSync(outDir, { recursive: true });
  fs.writeFileSync(path.join(outDir, name), JSON.stringify(value, null, 1) + '\n');
  process.stderr.write(`wrote ${path.relative(repoRoot, path.join(outDir, name))}\n`);
}

// ---- corpus ----

// The fonts and style sheets that the page of mmdc has and a plain page has
// not. A diagram that uses one of them has another layout in mmdc.
const usesPageFonts = (svg) =>
  svg.includes('Recursive Variable') || svg.includes('Open Sans') || /class="fa[ bsrlk]/.test(svg) || svg.includes('katex');

// The sets of options of the comparison.
const CSS_FILE = '.node rect { fill: #fdd !important; }\n.messageText { fill: #00f; }\n';
const SETS = [
  { name: 'default', args: [] },
  { name: 'no-font-embed', args: ['--no-font-embed'] },
  { name: 'transparent', args: ['-b', 'transparent'] },
  { name: 'css', args: ['-C', 'style.css'] },
  { name: 'id', args: ['-I', 'diagram'] },
  { name: 'dark', args: ['-t', 'dark'] },
];

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

function selectFixtures() {
  const chosen = [];
  for (const dir of fs.readdirSync(refDir).sort()) {
    // gantt shows the time of the run. zenuml is an external diagram.
    if (dir === 'gantt' || dir === 'zenuml') {
      continue;
    }
    const plain = [];
    const pageFonts = [];
    for (const file of walk(path.join(refDir, dir)).sort()) {
      const name = path.relative(refDir, file).slice(0, -4).split(path.sep).join('/');
      const meta = JSON.parse(fs.readFileSync(path.join(refDir, `${name}.json`), 'utf8'));
      const svg = fs.readFileSync(path.join(refDir, `${name}.svg`), 'utf8');
      const src = fs.readFileSync(path.join(corpusDir, `${name}.mmd`), 'utf8');
      // No random numbers, no images from the network, no icon packs, and
      // no text that ends the code block of the Markdown file.
      if (meta.randomCalls !== 0 || /<img |<image |(?:src|href)="https?:\/\//.test(svg) || /[`:]{3}[^\S\n]*$/m.test(src)) {
        continue;
      }
      (usesPageFonts(svg) ? pageFonts : plain).push({ name, size: svg.length });
    }
    const bySize = (a, b) => a.size - b.size || (a.name < b.name ? -1 : 1);
    chosen.push(...plain.sort(bySize).slice(0, 3).map((f) => ({ name: f.name, pageFonts: false })));
    chosen.push(...pageFonts.sort(bySize).slice(0, 1).map((f) => ({ name: f.name, pageFonts: true })));
  }
  return chosen;
}

function genCorpus() {
  const fixtures = selectFixtures().filter(f => !values.has('--filter') || f.name.includes(values.get('--filter')));
  if (!fixtures.length) throw new Error('No corpus fixtures matched the filter');
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-corpus-'));
  let markdown = '';
  for (const f of fixtures) {
    const src = fs.readFileSync(path.join(corpusDir, `${f.name}.mmd`), 'utf8');
    markdown += `## ${f.name}\n\n\`\`\`mermaid\n${src}${src.endsWith('\n') ? '' : '\n'}\`\`\`\n\n`;
  }
  fs.writeFileSync(path.join(dir, 'doc.md'), markdown);
  fs.writeFileSync(path.join(dir, 'style.css'), CSS_FILE);
  const jobs = SETS.map((set) => {
    fs.mkdirSync(path.join(dir, set.name));
    return { args: ['-i', 'doc.md', '-o', `${set.name}/out.svg`, '-q', '-j', '6', ...set.args], cwd: dir, timeout: 300000 };
  });
  const results = mmdc(jobs);
  const sets = {};
  SETS.forEach((set, i) => {
    if (results[i].code !== 0) {
      process.stderr.write(`mmdc failed for ${set.name}: ${results[i].stderr.slice(0, 2000)}\n`);
      return;
    }
    const files = {};
    fixtures.forEach((f, k) => {
      const data = fs.readFileSync(path.join(dir, set.name, `out-${k + 1}.svg`), 'utf8');
      // mmdc adds a style element with a license and font data after the
      // style element of mermaid when the diagram uses a font of its page.
      files[f.name] = { hash: hash(data), fontEmbed: data.includes('</style><style>/*') };
    });
    sets[set.name] = { args: set.args, files };
  });
  writeJSON('corpus.json', {
    ...oracleVersions,
    ...(values.has('--filter') ? { filter: values.get('--filter') } : {}),
    note: 'hash is the first 16 hex digits of the SHA-256 of the .svg file of mmdc',
    cssFile: CSS_FILE,
    fixtures,
    sets,
  });
}

// ---- png ----

const DIAGRAMS = {
  small: 'flowchart LR\n  A[Hello] --> B(World)\n',
  wide:
    'flowchart LR\n  A[Alpha node] --> B[Bravo node] --> C[Charlie node] --> D[Delta node] --> E[Echo node] --> F[Foxtrot node] --> G[Golf node] --> H[Hotel node] --> I[India node] --> J[Juliet node]\n',
  tall: 'flowchart TD\n  A[Alpha node] --> B[Bravo node] --> C[Charlie node] --> D[Delta node] --> E[Echo node] --> F[Foxtrot node]\n',
  fixed:
    '%%{init: {"flowchart": {"useMaxWidth": false}}}%%\nflowchart LR\n  A[Alpha node] --> B[Bravo node] --> C[Charlie node] --> D[Delta node] --> E[Echo node] --> F[Foxtrot node] --> G[Golf node] --> H[Hotel node] --> I[India node] --> J[Juliet node]\n',
  pie: 'pie title Pets\n  "Dogs" : 386\n  "Cats" : 85\n',
  info: 'info\n',
};

// The theme "default" has no font of the page of mmdc for these diagrams. A
// sequence diagram is not in the list: it measures the names of its actors
// with Open Sans, which the page of mmdc has.
const PNG_CASES = [
  ...Object.keys(DIAGRAMS).map((diagram) => ({ diagram, args: [] })),
  ...['2', '1.5', '1.25', '3'].flatMap((s) => [
    { diagram: 'small', args: ['-s', s] },
    { diagram: 'wide', args: ['-s', s] },
  ]),
  ...['small', 'wide', 'tall', 'pie'].map((diagram) => ({ diagram, args: ['--size', '300'] })),
  { diagram: 'small', args: ['-b', 'transparent'] },
  { diagram: 'small', args: ['-b', 'red', '-s', '2'] },
];

function pngSize(file) {
  const data = fs.readFileSync(file);
  return { width: data.readUInt32BE(16), height: data.readUInt32BE(20), colorType: data[25] };
}

function genPNG() {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-png-'));
  for (const [name, text] of Object.entries(DIAGRAMS)) {
    fs.writeFileSync(path.join(dir, `${name}.mmd`), text);
  }
  const jobs = PNG_CASES.map((c, i) => ({
    args: ['-i', `${c.diagram}.mmd`, '-o', `out-${i}.png`, '-q', '-t', 'default', ...c.args],
    cwd: dir,
  }));
  const results = mmdc(jobs);
  const cases = PNG_CASES.map((c, i) => {
    if (results[i].code !== 0) {
      throw new Error(`mmdc failed for ${JSON.stringify(c)}: ${results[i].stderr}`);
    }
    return { ...c, args: ['-t', 'default', ...c.args], ...pngSize(path.join(dir, `out-${i}.png`)) };
  });
  writeJSON('png.json', { ...oracleVersions, diagrams: DIAGRAMS, cases });
}

// ---- cli ----

const MARKDOWN =
  '# Title\n\n```mermaid\nflowchart LR\n  accTitle: The "title" [x]\n  accDescr: A description [with] back\\slash\n  A --> B\n```\n\n' +
  'text\n\n  :::mermaid\npie title Pets\n  "Dogs" : 386\n  "Cats" : 85\n:::  \n\n```mermaid\ninfo\n```\nend\n';

// Each case runs in a directory that has small.mmd, doc.md, none.md,
// config.json and style.css.
const CLI_CASES = [
  { args: ['-i', 'missing.mmd'] },
  { args: ['-i', 'small.mmd', '-o', 'x.txt'] },
  { args: ['-i', 'small.mmd', '-o', 'nodir/x.svg'] },
  { args: ['-i', 'small.mmd', '-t', 'nope'] },
  { args: ['-i', 'small.mmd', '-s', '0'] },
  { args: ['-i', 'small.mmd', '-s', 'abc'] },
  { args: ['-i', 'small.mmd', '--size', '0'] },
  { args: ['-i', 'small.mmd', '-e', 'gif'] },
  { args: ['-i', 'small.mmd', '-j', '0'] },
  { args: ['-i', 'small.mmd', '--bogus'] },
  { args: ['-i', 'small.mmd', '-x'] },
  { args: ['-i', 'small.mmd', 'extra'] },
  { args: ['-i', 'small.mmd', '-c', 'missing.json'] },
  { args: ['-i', 'small.mmd', '-C', 'missing.css'] },
  { args: ['-i', 'small.mmd', '-a', 'arte'] },
  { args: ['-i', 'doc.md', '-o', '-', '-e', 'svg'] },
  { args: ['-i', 'none.md', '-o', 'none-out.md'], files: ['none-out.md'] },
  { args: ['-i', 'doc.md', '-o', 'out.md', '-t', 'default'], files: ['out.md'], exists: ['out-1.svg', 'out-2.svg', 'out-3.svg'] },
  { args: ['-i', 'doc.md', '-o', 'art.md', '-a', 'images/deep', '-e', 'png', '-t', 'default'], files: ['art.md'], exists: ['images/deep/art-1.png', 'images/deep/art-3.png'] },
  { args: ['-i', 'doc.md', '-o', 'tpl.png', '-t', 'default', '-q'], exists: ['tpl-1.png', 'tpl-2.png', 'tpl-3.png'] },
  { args: ['-i', 'doc.md', '-t', 'default', '-q'], exists: ['doc.md-1.svg', 'doc.md-3.svg'] },
  { args: ['-i', 'small.mmd', '-t', 'default'], exists: ['small.mmd.svg'] },
  { args: ['-i', 'small.mmd', '-e', 'png', '-t', 'default'], exists: ['small.mmd.png'] },
  { args: ['-t', 'default'], stdin: 'info\n', exists: ['out.svg'] },
  { args: ['-i', '-', '-o', '-', '-t', 'default'], stdin: 'info\n', stdoutHash: true },
];

function genCLI() {
  const jobs = CLI_CASES.map((c) => {
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-cli-'));
    fs.writeFileSync(path.join(dir, 'small.mmd'), DIAGRAMS.small);
    fs.writeFileSync(path.join(dir, 'doc.md'), MARKDOWN);
    fs.writeFileSync(path.join(dir, 'none.md'), '# no charts\n\ntext\n');
    fs.writeFileSync(path.join(dir, 'config.json'), '{"theme":"forest"}');
    fs.writeFileSync(path.join(dir, 'style.css'), CSS_FILE);
    return { args: c.args, cwd: dir, stdin: c.stdin };
  });
  const results = mmdc(jobs);
  const cases = CLI_CASES.map((c, i) => {
    const r = results[i];
    const out = { args: c.args, code: r.code };
    if (c.stdin !== undefined) {
      out.stdin = c.stdin;
    }
    if (c.stdoutHash) {
      out.stdoutHash = hash(r.stdout);
    } else {
      out.stdout = r.stdout;
    }
    // The stack of a JavaScript error is not part of the contract: keep the
    // first line of each message.
    out.stderr = r.stderr.replace(/\n {4}at [^\n]*/g, '');
    if (c.files) {
      out.files = Object.fromEntries(c.files.map((f) => [f, fs.readFileSync(path.join(jobs[i].cwd, f), 'utf8')]));
    }
    if (c.exists) {
      out.exists = Object.fromEntries(c.exists.map((f) => [f, fs.existsSync(path.join(jobs[i].cwd, f))]));
    }
    return out;
  });
  writeJSON('cli.json', { ...oracleVersions, small: DIAGRAMS.small, markdown: MARKDOWN, cases });
}

if (command === 'corpus') {
  genCorpus();
} else if (command === 'png') {
  genPNG();
} else if (command === 'cli') {
  genCLI();
}
