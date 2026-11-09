// Replays the reference diagrams in Chromium and records what the page
// measures for each element. See probes/replay.js for the values.
//
// The reference SVG files of .parity/ref/mermaid-e2e are what mermaid made.
// Each file goes back into the page at the place where mermaid renders:
// body > div#container > div#dmy-svg > svg#my-svg. The root svg loses its
// viewBox, its height and its style, and its width is 100%, because mermaid
// sets them after it measures. So the page measures the content in the state
// that the code of mermaid saw.
//
// Usage (in tools/reference):
//
//   node gen-browser-replay.mjs measure [--filter text] [--force]
//       Runs Chromium over the whole corpus, at most 150 documents for each
//       run of the oracle, and writes .parity/replay/batch-NNN.json.gz. A
//       batch that exists is not measured again.
//   node gen-browser-replay.mjs golden [--budget bytes]
//       Selects documents from the batches, drops the records that an
//       earlier document has with the same markup and style context, and
//       writes internal/browser/testdata/replay.json.gz. It does not start
//       a browser.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const refDir = path.join(repoRoot, '.parity/ref/mermaid-e2e');
const defaultCacheDir = path.join(repoRoot, '.parity/replay');
const goldenFile = path.join(repoRoot, 'internal/browser/testdata/replay.json.gz');
const BATCH = 150;
const KINDS = ['bbox', 'rect', 'tlen', 'font'];

const argv = process.argv.slice(2);
const command = argv[0];
const flag = (name) => {
  const i = argv.indexOf('--' + name);
  if (i < 0) {
    return undefined;
  }
  const next = argv[i + 1];
  return next === undefined || next.startsWith('--') ? true : next;
};
// --cache names another directory for the batches (for a trial run).
const cacheDir = typeof flag('cache') === 'string' ? path.resolve(flag('cache')) : defaultCacheDir;

function listSvgs(dir, out = []) {
  for (const e of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const p = path.join(dir, e.name);
    if (e.isDirectory()) {
      listSvgs(p, out);
    } else if (e.name.endsWith('.svg')) {
      out.push(p);
    }
  }
  return out;
}

// prepare returns the markup that the page gets for a reference SVG, with
// each style sheet replaced by a reference to the table of style sheets.
function prepare(svg, styles) {
  const end = svg.indexOf('>');
  if (!svg.startsWith('<svg ') || end < 0) {
    return null;
  }
  let root = svg.slice(0, end);
  root = root.replace(/ viewBox="[^"]*"/, '').replace(/ style="[^"]*"/, '').replace(/ height="[^"]*"/, '');
  root = root.replace(/ width="[^"]*"/, ' width="100%"');
  let fontFamily = '"trebuchet ms", verdana, arial, sans-serif';
  const refs = [];
  const body = svg.slice(end).replace(/<style>([\s\S]*?)<\/style>/g, (_, text) => {
    const family = text.match(/#my-svg\{font-family:([^;}]*)[;}]/);
    if (family && refs.length === 0) {
      fontFamily = family[1];
    }
    let idx = styles.indexOf(text);
    if (idx < 0) {
      idx = styles.push(text) - 1;
    }
    refs.push(idx);
    return `<style>/*@${idx}*/</style>`;
  });
  const html = `<div id="dmy-svg" style="font-family: ${fontFamily.replace(/"/g, '&quot;')}">${root}${body}</div>`;
  return { html, style: refs.join(',') };
}

// expand puts the style sheets back.
const expand = (html, styles) => html.replace(/<style>\/\*@(\d+)\*\/<\/style>/g, (_, i) => `<style>${styles[Number(i)]}</style>`);

function readBatches() {
  const files = fs.existsSync(cacheDir) ? fs.readdirSync(cacheDir).filter((f) => /^batch-\d+\.json\.gz$/.test(f)).sort() : [];
  return files.map((f) => JSON.parse(zlib.gunzipSync(fs.readFileSync(path.join(cacheDir, f))).toString('utf8')));
}

function measure() {
  const filter = flag('filter');
  let files = listSvgs(refDir);
  if (typeof filter === 'string') {
    files = files.filter((f) => f.includes(filter));
  }
  fs.mkdirSync(cacheDir, { recursive: true });
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-replay-'));
  for (let start = 0, b = 0; start < files.length; start += BATCH, b++) {
    const outFile = path.join(cacheDir, `batch-${String(b).padStart(3, '0')}.json.gz`);
    if (fs.existsSync(outFile) && !flag('force')) {
      continue;
    }
    const styles = [];
    const docs = [];
    for (const file of files.slice(start, start + BATCH)) {
      const name = path.relative(refDir, file).replace(/\.svg$/, '');
      let type = '';
      try {
        type = JSON.parse(fs.readFileSync(file.replace(/\.svg$/, '.json'), 'utf8')).diagramType || '';
      } catch {
        // No type.
      }
      const p = prepare(fs.readFileSync(file, 'utf8'), styles);
      if (p) {
        docs.push({ name, type, html: p.html, style: p.style });
      }
    }
    const argsFile = path.join(tmp, 'args.json');
    const resFile = path.join(tmp, 'res.json');
    // The key of a record has the text of the style sheets, so that it is
    // the same in every batch.
    const styleKeys = styles.map((s) => zlib.crc32(s).toString(36) + '.' + s.length);
    fs.writeFileSync(
      argsFile,
      JSON.stringify({
        keys: true,
        docs: docs.map((d) => ({ name: d.name, html: expand(d.html, styles), style: d.style.split(',').map((i) => styleKeys[Number(i)]).join(',') })),
      })
    );
    execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'eval', path.join(here, 'probes/replay.js'), '--args', argsFile, '--out', resFile], {
      stdio: 'inherit',
    });
    const results = JSON.parse(fs.readFileSync(resFile, 'utf8'));
    for (let i = 0; i < docs.length; i++) {
      const { name: _, ...rest } = results[i];
      Object.assign(docs[i], rest);
    }
    fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify({ styles, docs }), { level: 9 }));
    console.error(`batch ${b}: ${docs.length} documents`);
  }
  fs.rmSync(tmp, { recursive: true, force: true });
}

function golden() {
  const budget = Number(flag('budget') || 2_600_000);
  const all = [];
  for (const batch of readBatches()) {
    for (const d of batch.docs) {
      all.push({ ...d, styleTexts: d.style.split(',').map((i) => batch.styles[Number(i)]) });
    }
  }
  if (all.length === 0) {
    throw new Error('no batches in .parity/replay: run the measure command first');
  }
  // A document with MathML (a label with KaTeX) is not in the file: the
  // engine does not lay out MathML.
  for (let i = all.length - 1; i >= 0; i--) {
    if (all[i].html.includes('<math')) {
      all.splice(i, 1);
    }
  }
  // The cost of a document is the size of its markup without the style
  // sheets, compressed. Its gain is the number of records with a new key,
  // where a record of an HTML label counts more than the others.
  for (const d of all) {
    d.cost = zlib.gzipSync(d.html, { level: 9 }).length + 200;
    d.html5 = d.html.includes('<foreignObject');
  }
  const seen = new Set();
  const weight = { bbox: 1, rect: 6, tlen: 2, font: 1 };
  const gain = (d) => {
    let g = 0;
    for (const kind of KINDS) {
      for (const key of d.keys[kind]) {
        if (!seen.has(kind + key)) {
          g += weight[kind];
        }
      }
    }
    return g;
  };
  const take = (d) => {
    d.taken = true;
    d.keep = {};
    for (const kind of KINDS) {
      d.keep[kind] = d.keys[kind].map((key) => {
        if (seen.has(kind + key)) {
          return false;
        }
        seen.add(kind + key);
        return true;
      });
    }
  };
  let used = 0;
  const picked = [];
  const pick = (d) => {
    take(d);
    picked.push(d);
    // The records cost about a third of the markup.
    used += Math.round(d.cost * 1.35);
  };
  // First the smallest document of each diagram type, so that each type is
  // in the file. Then the document with the best gain for its cost.
  const types = [...new Set(all.map((d) => d.type))].sort();
  for (const type of types) {
    const docs = all.filter((d) => d.type === type).sort((a, b) => a.cost - b.cost);
    pick(docs[Math.floor(docs.length / 4)]);
  }
  for (;;) {
    let best = null;
    let bestScore = 0;
    for (const d of all) {
      if (d.taken || used + d.cost > budget) {
        continue;
      }
      const score = gain(d) / d.cost;
      if (score > bestScore) {
        best = d;
        bestScore = score;
      }
    }
    if (!best) {
      break;
    }
    pick(best);
  }
  picked.sort((a, b) => (a.name < b.name ? -1 : 1));
  const styles = [];
  const docs = picked.map((d) => {
    const refs = d.styleTexts.map((text) => {
      let idx = styles.indexOf(text);
      if (idx < 0) {
        idx = styles.push(text) - 1;
      }
      return idx;
    });
    let k = 0;
    const html = d.html.replace(/<style>\/\*@(\d+)\*\/<\/style>/g, () => `<style>/*@${refs[k++]}*/</style>`);
    const doc = { name: d.name, type: d.type, html, n: d.n, sig: d.sig };
    for (const kind of KINDS) {
      doc[kind] = d[kind].filter((_, i) => d.keep[kind][i]);
    }
    return doc;
  });
  const data = zlib.gzipSync(JSON.stringify({ styles, docs }), { level: 9 });
  fs.writeFileSync(goldenFile, data);
  const count = (kind) => docs.reduce((n, d) => n + d[kind].length, 0);
  console.error(
    `${docs.length} of ${all.length} documents, ${styles.length} style sheets, ${KINDS.map((k) => `${k} ${count(k)}`).join(', ')}, ${data.length} bytes`
  );
  const perType = {};
  for (const d of docs) {
    perType[d.type] = (perType[d.type] || 0) + 1;
  }
  console.error(Object.entries(perType).map(([t, n]) => `${t}:${n}`).join(' '));
}

if (command === 'measure') {
  measure();
} else if (command === 'golden') {
  golden();
} else {
  console.error('usage: gen-browser-replay.mjs <measure|golden>');
  process.exit(2);
}
