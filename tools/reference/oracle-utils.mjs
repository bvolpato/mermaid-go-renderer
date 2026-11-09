#!/usr/bin/env node
// Golden generator for internal/utils and internal/svgdraw.
//
// The functions of utils.ts and svgDrawCommon.ts are internal to the mermaid
// bundle, and some of them measure text or build DOM. So this script runs the
// real code in headless Chromium: it serves the readable ESM build
// (node_modules/mermaid/dist) to the page from a fake origin, adds an export
// for every top-level name of the chunks that define the functions, calls
// them with generated inputs and writes the results:
//
//   internal/js/testdata/utils.json.gz       toString(radix), escape, unescape
//   internal/utils/testdata/pure.json.gz     functions without DOM
//   internal/utils/testdata/text.json.gz     text measurement, in call order
//   internal/utils/testdata/dom.json.gz      drawSimpleText, as markup
//   internal/svgdraw/testdata/dom.json.gz    svgDrawCommon.ts, as markup
//
// Run it with tools/reference as the working directory:
//
//   node oracle-utils.mjs
//
// The page is the page of oracle.mjs (body > div#container, 800x600, en-US,
// UTC, fixed clock). oracle.mjs runs its main function when it is imported,
// so the lock and launch code is a copy of the code there. Both scripts use
// the same lock directory: only one browser runs at a time.
import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import fsSync from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const REPO = path.resolve(HERE, '../..');
const DIST_DIR = path.join(HERE, 'node_modules/mermaid/dist');
const CHUNK_DIR = path.join(DIST_DIR, 'chunks/mermaid.esm');
const ORIGIN = 'http://mmdg-utils.local';

// The same values as oracle.mjs.
const FIXED_TIME = '2026-01-15T12:00:00Z';
const VIEWPORT = { width: 800, height: 600 };
const PAGE_HTML =
  '<!doctype html><html><head><meta charset="utf-8"></head>' +
  '<body><div id="container"></div></body></html>';

// ---- lock and launch: a copy of the code of oracle.mjs ----

const LOCK_DIR = path.join(HERE, '.oracle.lock');
async function acquireLock() {
  for (;;) {
    try {
      await fs.mkdir(LOCK_DIR);
      await fs.writeFile(path.join(LOCK_DIR, 'pid'), String(process.pid));
      return;
    } catch {
      let stale = false;
      try {
        const pid = Number(await fs.readFile(path.join(LOCK_DIR, 'pid'), 'utf8'));
        process.kill(pid, 0);
      } catch (e) {
        stale = e.code === 'ESRCH' || e.code === 'ENOENT';
      }
      if (stale) {
        await fs.rm(LOCK_DIR, { recursive: true, force: true });
        continue;
      }
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
  }
}

async function withBrowser(fn) {
  await acquireLock();
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
    const context = await browser.newContext({
      viewport: VIEWPORT,
      deviceScaleFactor: 1,
      locale: 'en-US',
      timezoneId: 'UTC',
      colorScheme: 'light',
    });
    try {
      return await fn(context);
    } finally {
      await context.close();
    }
  } finally {
    if (browser) {
      await browser.close();
    }
    await fs.rm(LOCK_DIR, { recursive: true, force: true });
  }
}

// ---- the ESM build, with exports for the internal names ----

// findChunk returns the file name of the chunk whose text has the marker.
function findChunk(marker) {
  const hits = fsSync
    .readdirSync(CHUNK_DIR)
    .filter((f) => f.endsWith('.mjs') && f.startsWith('chunk-'))
    .filter((f) => fsSync.readFileSync(path.join(CHUNK_DIR, f), 'utf8').includes(marker));
  if (hits.length !== 1) {
    throw new Error(`chunk with ${JSON.stringify(marker)}: ${hits.length} files`);
  }
  return hits[0];
}

// The chunks that define the functions under test. The file names are build
// hashes, so each chunk is found by a line of its text.
const CHUNKS = {
  utils: findChunk('var calculateTextDimensions = '),
  svgDraw: findChunk('var drawBackgroundRect = '),
  margins: findChunk('var getSubGraphTitleMargins = '),
  edges: findChunk('var getLineFunctionsWithOffset = '),
  state: findChunk('var ImperativeState = '),
  d3: findChunk('function select_default'),
};

// withExports appends an export statement for every top-level name that the
// module does not export yet. The name x is exported as __x_x.
function withExports(src) {
  const exported = new Set();
  for (const block of src.matchAll(/^export \{([^}]*)\};?/gm)) {
    for (const item of block[1].split(',')) {
      const m = item.trim().match(/^(\S+)(?:\s+as\s+(\S+))?$/);
      if (m) {
        exported.add(m[1]);
      }
    }
  }
  const names = new Set();
  for (const m of src.matchAll(/^(?:async function\*?|function\*?|var|let|const|class) ([A-Za-z_$][\w$]*)/gm)) {
    names.add(m[1]);
  }
  if (names.size === 0) {
    return src;
  }
  return `${src}\nexport { ${[...names].map((n) => `${n} as __x_${n}`).join(', ')} };\n`;
}

async function newPage(context) {
  const page = await context.newPage();
  await page.clock.install({ time: new Date(FIXED_TIME) });
  await page.clock.setFixedTime(new Date(FIXED_TIME));
  const rewrite = new Set(Object.values(CHUNKS));
  await page.route(`${ORIGIN}/**`, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/index.html') {
      return route.fulfill({ contentType: 'text/html', body: PAGE_HTML });
    }
    try {
      let body = await fs.readFile(path.join(DIST_DIR, url.pathname), 'utf8');
      if (rewrite.has(path.basename(url.pathname)) || url.pathname === '/mermaid.esm.mjs') {
        body = withExports(body);
      }
      return route.fulfill({ contentType: 'text/javascript', body });
    } catch {
      return route.fulfill({ status: 404, body: 'not found' });
    }
  });
  page.on('pageerror', (e) => process.stderr.write(`page error: ${e.message}\n`));
  await page.goto(`${ORIGIN}/index.html`);
  return page;
}

// ---- inputs ----

function mulberry32(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// The tagged form of a value that JSON cannot hold: undefined is {"$u":1},
// and NaN, the infinities and -0 are {"$n":"<text>"}.
const U = { $u: 1 };
const num = (text) => ({ $n: text });

function makeRng(seed) {
  const rnd = mulberry32(seed);
  const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));
  const pick = (arr) => arr[int(0, arr.length - 1)];
  const chance = (p) => rnd() < p;
  return { rnd, int, pick, chance };
}

// walk lists the files under dir, sorted.
function walk(dir) {
  const out = [];
  for (const entry of fsSync.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const p = path.join(dir, entry.name);
    if (entry.isDirectory()) {
      out.push(...walk(p));
    } else if (entry.name.endsWith('.mmd')) {
      out.push(p);
    }
  }
  return out;
}

// corpusLabels takes label texts from the corpus fixtures: the text in
// quotes, brackets, parentheses, braces and bars, and the text after a colon.
function corpusLabels() {
  const set = new Set();
  const add = (s) => {
    s = s.trim();
    // A lone surrogate does not survive JSON on the Go side.
    if (s && s.length <= 100 && s.isWellFormed()) {
      set.add(s);
    }
  };
  for (const file of walk(path.join(REPO, 'testdata/corpus/mermaid-e2e'))) {
    let text = fsSync.readFileSync(file, 'utf8');
    const fm = text.match(/^---\n[\s\S]*?\n---\n/);
    if (fm) {
      text = text.slice(fm[0].length);
    }
    for (const line of text.split('\n')) {
      for (const re of [/"([^"]{1,100})"/g, /\[([^[\]]{1,100})\]/g, /\(([^()]{1,100})\)/g, /\{([^{}]{1,100})\}/g, /\|([^|]{1,100})\|/g]) {
        for (const m of line.matchAll(re)) {
          add(m[1]);
        }
      }
      const colon = line.indexOf(':');
      if (colon >= 0) {
        add(line.slice(colon + 1));
      }
    }
  }
  return [...set].sort();
}

const WORDS = (
  'the quick brown fox jumps over a lazy dog Alice Bob Carol request response service database ' +
  'authentication authorization configuration internationalization supercalifragilisticexpialidocious ' +
  'OK id API HTTP/2 v1.2.3 user@example.com 100% (optional) [x] a-b_c "quoted" it\'s & < > Zürich naïve ' +
  'Ελληνικά Привет 日本語 中文 한국어 🙂 WWWW iiii MMMMMMMM .... 1234567890 A B I W'
).split(' ');

const FONT_FAMILIES = [
  '"trebuchet ms", verdana, arial, sans-serif',
  '"Recursive Variable", arial, sans-serif',
  '"Open Sans", sans-serif',
  'arial, sans-serif',
];

function sentence(r, minWords, maxWords) {
  const n = r.int(minWords, maxWords);
  const words = [];
  for (let i = 0; i < n; i++) {
    words.push(r.pick(WORDS));
  }
  return words.join(' ');
}

// fontConfig returns { fontSize, fontWeight, fontFamily } in tagged form.
// Most configs are the ones that mermaid really uses.
function fontConfig(r) {
  const cfg = {};
  const size = r.int(12, 24);
  const roll = r.rnd();
  if (roll < 0.8) {
    cfg.fontSize = size;
  } else if (roll < 0.88) {
    cfg.fontSize = `${size}px`;
  } else if (roll < 0.92) {
    cfg.fontSize = String(size);
  } else if (roll < 0.96) {
    cfg.fontSize = size + 0.5;
  } else {
    cfg.fontSize = U;
  }
  cfg.fontWeight = r.pick(['normal', 'bold', 'normal', 'bold', 400, 700, U]);
  cfg.fontFamily = r.chance(0.08) ? U : r.pick([FONT_FAMILIES[0], FONT_FAMILIES[0], FONT_FAMILIES[1], FONT_FAMILIES[1], FONT_FAMILIES[2], FONT_FAMILIES[3]]);
  return cfg;
}

function randomCoord(r) {
  const roll = r.rnd();
  if (roll < 0.3) {
    return r.int(-200, 800);
  }
  if (roll < 0.6) {
    return Math.round((r.rnd() * 1000 - 200) * 100) / 100;
  }
  return r.rnd() * 1000 - 200;
}

function randomPoints(r, min, max) {
  const n = r.int(min, max);
  const pts = [];
  for (let i = 0; i < n; i++) {
    if (i > 0 && r.chance(0.08)) {
      pts.push([...pts[i - 1]]); // a repeated point: a vector of length zero
    } else if (i > 0 && r.chance(0.15)) {
      // a short step, so that the cardinality distance of 25 goes past it
      pts.push([pts[i - 1][0] + r.int(-12, 12), pts[i - 1][1] + r.int(-12, 12)]);
    } else if (i > 0 && r.chance(0.2)) {
      // an axis-aligned step
      pts.push(r.chance(0.5) ? [pts[i - 1][0], randomCoord(r)] : [randomCoord(r), pts[i - 1][1]]);
    } else {
      pts.push([randomCoord(r), randomCoord(r)]);
    }
  }
  return pts;
}

function randomJson(r, depth) {
  const roll = r.rnd();
  if (depth <= 0 || roll < 0.45) {
    return r.pick([
      0, 1, -1, 2.5, 1e21, 12, 'a', '', 'text', 'true', true, false, null, U, U, num('NaN'), num('-0'),
      '#fff', '16px', 3, 4,
    ]);
  }
  if (roll < 0.62) {
    const n = r.int(0, 4);
    return Array.from({ length: n }, () => randomJson(r, depth - 1));
  }
  const o = {};
  const n = r.int(0, 5);
  for (let i = 0; i < n; i++) {
    const key = r.pick(['a', 'b', 'c', 'd', 'x', 'y', 'fontSize', 'length', '0', '1', '2', '10', 'nested', 'list', 'k1', 'k2']);
    o[key] = randomJson(r, depth - 1);
  }
  return o;
}

// mutateJson makes a value that shares its shape with base in most places,
// so that the merge has something to merge.
function mutateJson(r, base, depth) {
  if (r.chance(0.2)) {
    return randomJson(r, depth);
  }
  if (base !== null && typeof base === 'object' && !Array.isArray(base) && !('$u' in base) && !('$n' in base)) {
    const o = {};
    for (const k of Object.keys(base)) {
      if (r.chance(0.7)) {
        o[k] = mutateJson(r, base[k], depth - 1);
      }
    }
    if (r.chance(0.5)) {
      o[r.pick(['a', 'b', 'z', 'new', '3', 'length'])] = randomJson(r, depth - 1);
    }
    return o;
  }
  if (Array.isArray(base)) {
    const n = r.int(0, base.length + 1);
    return Array.from({ length: n }, (_, i) => (i < base.length ? mutateJson(r, base[i], depth - 1) : randomJson(r, depth - 1)));
  }
  return randomJson(r, 1);
}

function buildPureInputs() {
  const r = makeRng(0x7574696c);
  const inputs = {};

  // interpolateToCurve
  const curves = [
    'basis', 'basisClosed', 'basisOpen', 'bumpX', 'bumpY', 'bundle', 'cardinalClosed', 'cardinalOpen', 'cardinal',
    'catmullRomClosed', 'catmullRomOpen', 'catmullRom', 'linear', 'linearClosed', 'monotoneX', 'monotoneY',
    'natural', 'step', 'stepAfter', 'stepBefore',
  ];
  const curveInputs = [U, '', 'ſtep', 'constructor', '__proto__', 'toString', 'curveBasis', 'Curve', ' basis', 'basis '];
  for (const c of curves) {
    curveInputs.push(c, c[0].toUpperCase() + c.slice(1), c.toUpperCase(), c.toLowerCase(), c + 'x', c.slice(1));
  }
  for (let i = 0; i < 1900; i++) {
    let s = r.pick(curves);
    const k = r.int(0, 3);
    for (let j = 0; j < k; j++) {
      const at = r.int(0, s.length);
      const op = r.int(0, 2);
      const ch = r.pick([...'abcXYlinearStepBasis ſßé']);
      s = op === 0 ? s.slice(0, at) + ch + s.slice(at) : op === 1 ? s.slice(0, at) + s.slice(at + 1) : s.slice(0, at) + ch + s.slice(at + 1);
    }
    curveInputs.push(s);
  }
  inputs.interpolateToCurve = curveInputs.map((s) => ({ interpolate: s, def: r.pick(['curveLinear', 'curveBasis', 'curveNatural']) }));

  // formatUrl
  const urlParts = {
    scheme: ['http://', 'https://', 'HTTPS://', 'javascript:', 'JaVaScRiPt:', 'data:', 'vbscript:', 'mailto:', 'ftp://', '//', '/', './', '#', '', 'http:', 'https:', 'www.', 'java\tscript:', '&#106;avascript:', 'x-custom:'],
    host: ['example.com', 'Example.COM', 'mermaid.js.org', 'localhost:8080', 'user@host', '127.0.0.1', 'a.b/c', ''],
    path: ['', '/', '/a/b', '/a b', '/path?x=1&y=2', '#frag', '/%41', '/a\\b', '?q=<script>', '/é', '/a/../b'],
  };
  const urls = ['', ' ', '   \t', 'about:blank', 'javascript:alert(1)', ' https://mermaid.js.org ', 'https://mermaid.js.org'];
  for (let i = 0; i < 1500; i++) {
    let u = r.pick(urlParts.scheme) + r.pick(urlParts.host) + r.pick(urlParts.path);
    if (r.chance(0.2)) {
      u = r.pick([' ', '\n', '\t', '  ']) + u + r.pick([' ', '', '\n']);
    }
    urls.push(u);
  }
  inputs.formatUrl = urls.map((u) => ({ url: u, config: r.pick([{ securityLevel: 'loose' }, { securityLevel: 'strict' }, { securityLevel: 'antiscript' }, {}]) }));

  // roundNumber
  inputs.roundNumber = [];
  for (let i = 0; i < 2000; i++) {
    const n = r.pick([randomCoord(r), r.rnd(), (r.rnd() - 0.5) * 1e6, r.int(-5, 5) + 0.5, r.int(-5, 5) / 1000 + 0.0005]);
    inputs.roundNumber.push({ num: n, precision: r.pick([U, U, 5, 5, 0, 1, 2, 3, 10]) });
  }
  inputs.roundNumber.push({ num: num('NaN'), precision: U }, { num: num('-0'), precision: 5 }, { num: num('Infinity'), precision: 2 }, { num: -0.004, precision: 2 });

  // calcLabelPosition, calculatePoint, calcCardinalityPosition, calcTerminalLabelPosition
  inputs.calcLabelPosition = [{ points: [] }];
  for (let i = 0; i < 3000; i++) {
    inputs.calcLabelPosition.push({ points: randomPoints(r, 1, 8) });
  }
  inputs.calculatePoint = [{ points: [], distance: 0 }, { points: [[1, 2]], distance: 3 }];
  for (let i = 0; i < 3000; i++) {
    inputs.calculatePoint.push({ points: randomPoints(r, 1, 7), distance: r.pick([0, 25, 35, r.rnd() * 400, r.rnd() * 40, -1, 1e9]) });
  }
  inputs.calcCardinalityPosition = [{ rel: true, points: [], initial: 'other' }, { rel: false, points: [[1, 2]], initial: 'first' }];
  for (let i = 0; i < 3000; i++) {
    inputs.calcCardinalityPosition.push({ rel: r.chance(0.5), points: randomPoints(r, 1, 7), initial: r.pick(['first', 'last', 'first', 'last', 'other']) });
  }
  inputs.calcTerminalLabelPosition = [{ size: 0, position: 'start_left', points: [] }];
  for (let i = 0; i < 3000; i++) {
    inputs.calcTerminalLabelPosition.push({
      size: r.pick([0, 10, 10, 0, 4, 17.25, r.rnd() * 20]),
      position: r.pick(['start_left', 'start_right', 'end_left', 'end_right', 'other']),
      points: randomPoints(r, 1, 7),
    });
  }

  // getStylesFromArray
  const decls = ['color:red', 'color: #fff', 'text-align:center', 'fill:#f9f', 'stroke:#333', 'stroke-width:4px', 'font-size:12px', 'background-color:blue', ' color:red', 'COLOR:red', 'text-align', 'colors:1', '', 'opacity:0.5'];
  inputs.getStylesFromArray = [];
  for (let i = 0; i < 2000; i++) {
    inputs.getStylesFromArray.push({ arr: Array.from({ length: r.int(0, 6) }, () => r.pick(decls)) });
  }

  // generateId and random: the page seeds Math.random with 0 first.
  inputs.generateId = { count: 3000 };
  inputs.random = Array.from({ length: 2000 }, () => ({ length: r.pick([0, 1, 7, 7, 7, 8, 16, 32, r.int(0, 40)]) }));

  // toString(36) of the numbers that generateId can get, with edge cases.
  inputs.toStringRadix = [];
  for (let i = 0; i < 3000; i++) {
    const x = r.pick([r.rnd(), r.rnd() * 1e-5, r.rnd() * 1e6, Math.floor(r.rnd() * 1e9), r.rnd() * 2 ** 70, (r.rnd() - 0.5) * 100, r.int(1, 99) / 4294967296, 2 ** -r.int(1, 60)]);
    inputs.toStringRadix.push({ x, radix: r.pick([36, 36, 36, 2, 16, 8, 3, 7, 32, 10]) });
  }
  inputs.toStringRadix.push({ x: 0, radix: 36 }, { x: num('-0'), radix: 36 }, { x: num('NaN'), radix: 36 }, { x: num('Infinity'), radix: 2 }, { x: 0.5, radix: 36 }, { x: 1, radix: 36 }, { x: 35, radix: 36 }, { x: 36, radix: 36 }, { x: 5e-324, radix: 36 }, { x: 1.7976931348623157e308, radix: 36 });

  // escape and unescape, which entityDecode calls
  const escParts = ['a', 'Z', '0', '@', '*', '_', '+', '-', '.', '/', ' ', '%', '&', '#', ';', '<', '>', '"', "'", '~', '!', '\n', '\t', '\u0000', '\u007f', '\u0080', 'é', 'ÿ', 'Ā', '日', '\uffff', '🙂', '𝒜', '=', '?', ':', ','];
  inputs.escape = escParts.map((x) => ({ s: x }));
  for (let i = 0; i < 1500; i++) {
    let x = '';
    const k = r.int(0, 8);
    for (let j = 0; j < k; j++) {
      x += r.pick(escParts);
    }
    inputs.escape.push({ s: x });
  }
  const unescParts = ['%41', '%7e', '%7E', '%u0041', '%u65E5', '%ud83d%ude42', '%uD83D', '%', '%%', '%4', '%4g', '%u', '%u12', '%u12G4', '%U0041', '%e9', '%E9', '%C3%A9', 'a', 'b', ' ', 'é', '日', '🙂', '%00', '%20', '%2', '%u00e9'];
  inputs.unescape = unescParts.map((x) => ({ s: x }));
  for (let i = 0; i < 1500; i++) {
    let x = '';
    const k = r.int(0, 8);
    for (let j = 0; j < k; j++) {
      x += r.pick(unescParts);
    }
    inputs.unescape.push({ s: x });
  }

  // parseFontSize
  const sizes = [U, null, '', 'abc', '16', '16px', ' 16px', '16 px', '1e3', '12.5', '12.5px', '0x10', '-5', '+7em', '016', '16.0', 'px16', '2rem', '100%', 'NaN', 'Infinity', '9007199254740993', '1e21', true, false, 0, 16, 16.5, -3, num('NaN'), num('-0'), num('Infinity'), 1e21, 1e-7];
  inputs.parseFontSize = sizes.map((v) => ({ v }));
  for (let i = 0; i < 2000; i++) {
    const n = r.pick([r.int(0, 40), r.int(0, 40) + r.int(0, 9) / 10, r.rnd() * 100]);
    inputs.parseFontSize.push({ v: r.pick([n, String(n), `${n}px`, `${n}em`, ` ${n}`, `${n} `, `+${n}`, `-${n}pt`, `${n}`.replace('.', ',')]) });
  }

  // getEdgeId
  const ids = ['A', 'B', 'node 1', 'a_b', '', 'ß', '日本', 'x-y', '1', 'Start', 'End'];
  inputs.getEdgeId = [];
  for (let i = 0; i < 2000; i++) {
    const opts = {};
    if (r.chance(0.7)) opts.counter = r.int(0, 50);
    if (r.chance(0.7)) opts.prefix = r.pick(['L', 'L', '', 'id', 'edge']);
    if (r.chance(0.4)) opts.suffix = r.pick(['', 's', '0', 'end']);
    inputs.getEdgeId.push({ from: r.pick(ids), to: r.pick(ids), opts, id: r.pick([U, U, U, '', 'custom', 'e1']) });
  }

  // handleUndefinedAttr
  inputs.handleUndefinedAttr = [U, null, '', 'a', 0, 1, false, true, num('NaN')].map((v) => ({ v }));
// isLabelCoordinateInPath
  inputs.isLabelCoordinateInPath = [];
  for (let i = 0; i < 2500; i++) {
    const pts = randomPoints(r, 2, 6);
    let d = '';
    pts.forEach((p, j) => {
      d += (j === 0 ? 'M' : r.pick(['L', 'C', ' ', ','])) + p[0] + r.pick([',', ' ']) + p[1];
    });
    const src = r.pick(pts);
    const point = r.chance(0.5) ? { x: src[0] + r.pick([0, 0.2, -0.4, 0.5, 30]), y: src[1] + r.pick([0, 0.3, 0.5, 30]) } : { x: randomCoord(r), y: randomCoord(r) };
    inputs.isLabelCoordinateInPath.push({ point, d });
  }
  inputs.isLabelCoordinateInPath.push({ point: { x: -0.2, y: 5000 }, d: 'M0.4,1L2,3' }, { point: { x: 12, y: 9999 }, d: 'M1.5.5L12.49,3' });

  // isSubstringInArray
  const subs = ['a', 'b', 'ab', 'graph', 'flow', 'a.c', '^a', 'a$', '[ab]', 'a|b', '\\d', '', 'x*', '(', '[', 'a{2}', '.', 'A'];
  const els = ['a', 'abc', 'graph TD', 'flowchart', 'xyz', '', 'aa', 'a1c', 'A', 'b'];
  inputs.isSubstringInArray = [];
  for (let i = 0; i < 1500; i++) {
    inputs.isSubstringInArray.push({ str: r.pick(subs), arr: Array.from({ length: r.int(0, 5) }, () => r.pick(els)) });
  }

  // cleanAndMerge
  inputs.cleanAndMerge = [
    { a: {}, b: U },
    { a: { a: [1, 2, 3] }, b: { a: [null, 9] } },
    { a: { a: { length: 2, 0: 'x', 1: 'y' } }, b: { a: [5] } },
    { a: { a: 1 }, b: { a: U } },
    { a: { a: U }, b: { a: null } },
    { a: { a: [1, [2, 3]] }, b: { a: { 1: { 0: 'z' } } } },
    { a: null, b: { x: 1 } },
    { a: [1, 2], b: 'text' },
    { a: { __proto__x: 1 }, b: { constructor: 2 } },
  ];
  for (let i = 0; i < 3000; i++) {
    let a = randomJson(r, 4);
    if (a === null || typeof a !== 'object' || Array.isArray(a) || '$u' in a || '$n' in a) {
      if (r.chance(0.9)) {
        a = { a: a, b: randomJson(r, 3) };
      }
    }
    inputs.cleanAndMerge.push({ a, b: mutateJson(r, a, 4) });
  }

  // entityDecode
  const entityParts = ['&amp;', '&lt;', '&gt;', '&quot;', '&apos;', '&nbsp;', '&copy;', '&copy', '&amp', '&notit;', '&notin;', '&#35;', '&#x26;', '&#0;', '&#128512;', '&#x1F600;', '&#xD800;', '&#x80;', '&#1114112;', '&#;', '&#x;', '&;', '&', '#', ';', '%', '%41', '%u0041', '%zz', '<b>', '</b>', '<br/>', 'a', 'B', ' ', '\n', 'é', '日', '🙂', '+', '/', '@', '*', '_', '-', '.', '~', "'", '"', '=', '&AMP;', '&Aacute;', '&ensp;', '&zwnj;', '&NotEqualTilde;', '&fjlig;'];
  inputs.entityDecode = entityParts.map((s) => ({ html: s }));
  for (let i = 0; i < 2500; i++) {
    let s = '';
    const n = r.int(1, 6);
    for (let j = 0; j < n; j++) {
      s += r.pick(entityParts);
    }
    inputs.entityDecode.push({ html: s });
  }

  // InitIDGenerator
  inputs.initIDGenerator = [];
  for (const deterministic of [U, false, true]) {
    for (const seed of [U, '', 'a', 'seed', '日本語', '🙂x']) {
      inputs.initIDGenerator.push({ deterministic, seed, calls: 4 });
    }
  }

  // getSubGraphTitleMargins
  inputs.getSubGraphTitleMargins = [
    {}, { flowchart: {} }, { flowchart: { subGraphTitleMargin: {} } }, { flowchart: { subGraphTitleMargin: { top: 5 } } },
    { flowchart: { subGraphTitleMargin: { top: 0, bottom: 0 } } }, { flowchart: { subGraphTitleMargin: { top: 10, bottom: 5 } } },
    { flowchart: { subGraphTitleMargin: { bottom: 2.5 } } }, { flowchart: { subGraphTitleMargin: { top: null, bottom: 7 } } },
  ].map((config) => ({ config }));
  for (let i = 0; i < 500; i++) {
    inputs.getSubGraphTitleMargins.push({ config: { flowchart: { subGraphTitleMargin: { top: r.pick([0, 1, 2.5, r.rnd() * 30]), bottom: r.pick([0, 3, r.rnd() * 30]) } } } });
  }

  // toBase64
  inputs.toBase64 = ['', 'a', 'ab', 'abc', 'é', '日本語', '🙂', '<body style="margin:0">x</body>'].map((s) => ({ s }));
  for (let i = 0; i < 1000; i++) {
    inputs.toBase64.push({ s: sentence(r, 0, 8) });
  }

  // getLineFunctionsWithOffset
  const arrows = ['aggregation', 'extension', 'composition', 'dependency', 'lollipop', 'arrow_point', 'arrow_barb', 'arrow_barb_neo', 'arrow_cross', 'arrow_circle', 'none', U, '', 'constructor'];
  inputs.lineWithOffset = [];
  for (let i = 0; i < 3000; i++) {
    const edge = {};
    const s = r.pick(arrows);
    const e = r.pick(arrows);
    if (s !== U) edge.arrowTypeStart = s;
    if (e !== U) edge.arrowTypeEnd = e;
    let data = randomPoints(r, 1, 6);
    if (r.chance(0.3)) {
      // points close to the ends, for the marker height adjustments
      data = data.map((p, j) => (j === 0 ? p : [data[0][0] + r.int(-20, 20) * r.pick([1, 0.5, 0]), data[0][1] + r.int(-20, 20) * r.pick([1, 0.5, 0])]));
    }
    inputs.lineWithOffset.push({ edge, data });
  }

  // getErrorMessage, isDetailedError
  inputs.errors = { count: 1 };
  return inputs;
}

function buildTextInputs(labels) {
  const r = makeRng(0x74657874);
  const ops = [];
  const label = () => {
    const roll = r.rnd();
    if (roll < 0.6) {
      return r.pick(labels);
    }
    if (roll < 0.9) {
      return sentence(r, 1, 7);
    }
    // texts with line breaks, in the forms that lineBreakRegex accepts
    const br = r.pick(['<br>', '<br/>', '<br />', '<BR>', '</br>', '<br\t/>', '<br />']);
    return sentence(r, 0, 3) + br + sentence(r, 0, 3) + (r.chance(0.3) ? br + sentence(r, 0, 2) : '');
  };

  // The texts and fonts of real use. The same text is measured again with
  // another font, and some calls repeat, so that the caches take part.
  for (let i = 0; i < 4200; i++) {
    const fn = r.pick(['dims', 'dims', 'dims', 'dims', 'width', 'height']);
    ops.push({ fn, text: label(), config: fontConfig(r) });
    if (r.chance(0.1)) {
      ops.push({ ...ops[r.int(0, ops.length - 1)] });
    }
  }
  // Edge cases of the arguments.
  const edge = [
    { text: '', config: { fontSize: 16 } },
    { text: 'x', config: {} },
    { text: 'x', config: { fontSize: null, fontWeight: null, fontFamily: null } },
    { text: 'x', config: { fontSize: 'big', fontWeight: 'heavy', fontFamily: 'a;b' } },
    { text: 'x', config: { fontSize: '1.5em', fontWeight: 900, fontFamily: 'monospace' } },
    { text: 'x', config: { fontSize: '14pt', fontWeight: 'lighter', fontFamily: 'serif' } },
    { text: '<br>', config: { fontSize: 14 } },
    { text: 'a<br><br>b', config: { fontSize: 14 } },
    { text: '<br/>tail', config: { fontSize: 14 } },
    { text: ' ', config: { fontSize: 14 } },
    { text: '   leading and trailing   ', config: { fontSize: 14 } },
    { text: 'tab\there', config: { fontSize: 14 } },
    { text: 'line\nbreak', config: { fontSize: 14 } },
    { text: '​', config: { fontSize: 14 } },
    // A collision of cache keys: 'Hello' at 16 and 'Hello1' at 6 have the
    // same key text. The second call gets the result of the first.
    { text: 'Hello', config: { fontSize: 16, fontWeight: 400, fontFamily: 'arial' } },
    { text: 'Hello1', config: { fontSize: 6, fontWeight: 400, fontFamily: 'arial' } },
    { text: 'Hello1', config: { fontSize: 6, fontWeight: 400, fontFamily: 'arial, sans-serif' } },
    { text: 'x', config: { fontSize: 0 } },
    { text: 'after the error', config: { fontSize: 13 } },
  ];
  for (const e of edge) {
    ops.push({ fn: 'dims', ...e });
  }

  // wrapLabel and breakString
  for (let i = 0; i < 1500; i++) {
    const config = fontConfig(r);
    if (r.chance(0.1)) {
      config.joinWith = r.pick(['<br/>', '\n', ' ', '']);
    }
    const text = r.chance(0.5) ? r.pick(labels) : sentence(r, 1, 14);
    ops.push({ fn: 'wrap', text, maxWidth: r.pick([r.int(20, 300), r.int(60, 200), 100, 150, 200]), config });
    if (r.chance(0.08)) {
      ops.push({ ...ops[ops.length - 1] });
    }
  }
  for (const e of [
    { text: '', maxWidth: 100, config: {} },
    { text: 'no config at all here', maxWidth: 60, config: {} },
    { text: 'a  b   c', maxWidth: 30, config: { fontSize: 14 } },
    { text: 'has<br>breaks here and more words', maxWidth: 40, config: { fontSize: 14 } },
    { text: 'has<br>breaks here and more words', maxWidth: 40, config: { fontSize: 14 } },
    { text: 'x y z', maxWidth: 0, config: { fontSize: 14 } },
    { text: 'x y z', maxWidth: -5, config: { fontSize: 14 } },
    { text: 'join with undefined', maxWidth: 50, config: { fontSize: 14, joinWith: null } },
    { text: 'astral 🙂🙂🙂🙂🙂🙂🙂🙂🙂🙂 word', maxWidth: 40, config: { fontSize: 14 } },
  ]) {
    ops.push({ fn: 'wrap', ...e });
  }
  for (let i = 0; i < 600; i++) {
    const word = r.chance(0.5) ? r.pick(labels).replace(/ /g, '') : r.pick(WORDS) + r.pick(WORDS) + r.pick(WORDS);
    ops.push({ fn: 'break', text: word, maxWidth: r.pick([r.int(5, 120), 30, 50, 80]), hyphen: r.pick(['-', '-', '-', U, '', '~', '--']), config: fontConfig(r) });
  }
  return ops;
}

function buildDomInputs(labels) {
  const r = makeRng(0x646f6d);
  const inputs = {};
  const color = () => r.pick(['#fff', 'red', '#EDF2AE', 'rgb(1, 2, 3)', 'none', '', 'hsl(80, 50%, 50%)', 'url(#grad)', U]);
  const cls = () => r.pick([U, U, '', 'note', 'actor actor-top', 'rect', 'journey-section section-type-0']);
  const n = () => r.pick([0, 1, 10, 100, 150.5, -20, r.int(-50, 500), Math.round(r.rnd() * 50000) / 100, r.rnd() * 500]);
  const text = () => (r.chance(0.6) ? r.pick(labels) : sentence(r, 0, 5) + r.pick(['', '<br>', '<br/>x', ' <BR /> y']));
  const links = ['', 'https://example.com/a.png', 'http://example.com', 'javascript:alert(1)', 'img.png', './img.png', '/abs/img.svg', 'data:image/png;base64,AAAA', 'actor', 'computer', 'database', ' https://mermaid.js.org/x.png ', 'HTTPS://EXAMPLE.COM/A B', '#id', 'https://example.com/?a=1&b=<2>"'];

  inputs.drawRect = [];
  for (let i = 0; i < 1500; i++) {
    const d = { x: n(), y: n(), fill: color(), stroke: color(), width: n(), height: n() };
    const c = cls();
    if (c !== U) d.class = c;
    if (r.chance(0.4)) d.rx = r.pick([0, 3, 6, 2.5]);
    if (r.chance(0.4)) d.ry = r.pick([0, 3, 6, 2.5]);
    if (r.chance(0.3)) d.name = r.pick(['', 'Alice', 'Bob <b>', 'a"b']);
    if (r.chance(0.3)) {
      d.attrs = {};
      const k = r.int(0, 4);
      for (let j = 0; j < k; j++) {
        d.attrs[r.pick(['stroke-width', 'opacity', 'data-id', 'rx', 'x', 'xlink:href', 'class', 'filter', '2', '1'])] = r.pick([0.5, 1, 'a', '', 'url(#drop-shadow)', 7]);
      }
    }
    if (r.chance(0.1)) d.anchor = 'start';
    if (r.chance(0.1)) d.color = 'blue';
    inputs.drawRect.push({ data: d });
  }
  inputs.drawBackgroundRect = [];
  for (let i = 0; i < 800; i++) {
    inputs.drawBackgroundRect.push({ bounds: { startx: n(), stopx: n(), starty: n(), stopy: n(), fill: color(), stroke: color() }, before: r.int(0, 3) });
  }
  inputs.drawText = [];
  for (let i = 0; i < 1500; i++) {
    const d = { x: n(), y: n(), anchor: r.pick(['start', 'middle', 'end', 'left', '', U, 'inherit', 'MIDDLE']), text: text(), textMargin: r.pick([0, 5, 10, 2.5, r.rnd() * 10]) };
    const c = cls();
    if (c !== U) d.class = c;
    inputs.drawText.push({ data: d });
  }
  inputs.drawImage = [];
  inputs.drawEmbeddedImage = [];
  for (let i = 0; i < 400; i++) {
    inputs.drawImage.push({ x: n(), y: n(), link: r.pick(links) });
    inputs.drawEmbeddedImage.push({ x: n(), y: n(), link: r.pick(links) });
  }
  inputs.createTooltip = { count: 1 };

  // drawSimpleText of utils.ts
  inputs.drawSimpleText = [];
  for (let i = 0; i < 1200; i++) {
    const cfg = fontConfig(r);
    const d = { text: text(), x: n(), y: n(), anchor: r.pick(['start', 'middle', 'end', U, '']), textMargin: r.pick([0, 5, 2.5]) };
    if (r.chance(0.8)) d.fontFamily = cfg.fontFamily;
    if (r.chance(0.8)) d.fontSize = cfg.fontSize;
    if (r.chance(0.8)) d.fontWeight = cfg.fontWeight;
    if (r.chance(0.7)) d.fill = color();
    const c = cls();
    if (c !== U || r.chance(0.3)) d.class = c;
    inputs.drawSimpleText.push({ data: d });
  }
  return inputs;
}

// ---- the code that runs in the page ----

async function pageMain({ chunks, pure, text, dom }) {
  const enc = (v) => {
    if (v === undefined) return { $u: 1 };
    if (typeof v === 'number' && (!Number.isFinite(v) || Object.is(v, -0))) {
      return { $n: Object.is(v, -0) ? '-0' : String(v) };
    }
    if (v === null || typeof v !== 'object') return v;
    if (Array.isArray(v)) return Array.from(v, enc);
    const o = {};
    for (const k of Object.keys(v)) {
      Object.defineProperty(o, k, { value: enc(v[k]), enumerable: true, writable: true, configurable: true });
    }
    return o;
  };
  const dec = (v) => {
    if (v === null || typeof v !== 'object') return v;
    if (Array.isArray(v)) return v.map(dec);
    const keys = Object.keys(v);
    if (keys.length === 1 && keys[0] === '$u') return undefined;
    if (keys.length === 1 && keys[0] === '$n') return v.$n === '-0' ? -0 : Number(v.$n);
    const o = {};
    for (const k of keys) {
      Object.defineProperty(o, k, { value: dec(v[k]), enumerable: true, writable: true, configurable: true });
    }
    return o;
  };
  // attempt records the result of fn, or the message of the error it throws.
  const attempt = (fn) => {
    try {
      return { out: fn() };
    } catch (e) {
      return { error: String((e && e.message) || e) };
    }
  };
  const seedRandom = (seed) => {
    let a = seed >>> 0;
    Math.random = function () {
      a = (a + 0x6d2b79f5) >>> 0;
      let t = a;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
  };

  const load = async (file) => {
    const mod = await import(`/chunks/mermaid.esm/${file}`);
    return new Proxy(mod, {
      get(target, name) {
        if (typeof name === 'symbol' || name === 'then' || name in target) return target[name];
        const alt = `__x_${String(name)}`;
        if (alt in target) return target[alt];
        throw new Error(`${file} has no ${String(name)}`);
      },
    });
  };
  const utils = await load(chunks.utils);
  const svgDraw = await load(chunks.svgDraw);
  const margins = await load(chunks.margins);
  const edges = await load(chunks.edges);
  const state = await load(chunks.state);
  const d3 = await load(chunks.d3);
  const mermaidMod = await import('/mermaid.esm.mjs');
  const utilsDefault = utils.utils_default;
  const select = d3.select_default;
  const container = document.getElementById('container');

  const result = { pure: {}, text: null, dom: {} };
  const P = result.pure;

  // ---- pure functions ----
  if (pure) {
    P.zeroWidthSpace = utils.ZERO_WIDTH_SPACE;

    // generateId first: the counter of the module starts at 1.
    seedRandom(0);
    P.generateId = [];
    for (let i = 0; i < pure.generateId.count; i++) {
      P.generateId.push(utils.generateId());
    }
    P.random = pure.random.map((c) => ({ ...c, out: utils.random({ length: c.length }) }));

    P.toStringRadix = pure.toStringRadix.map((c) => ({ ...c, out: dec(c.x).toString(c.radix) }));
    // A string with a lone surrogate does not survive JSON on the Go side,
    // so unescape results are recorded as UTF-16 code units.
    const units = (str) => Array.from({ length: str.length }, (_, i) => str.charCodeAt(i));
    P.escape = pure.escape.map((c) => ({ ...c, out: escape(c.s) }));
    P.unescape = pure.unescape.map((c) => ({ ...c, out: units(unescape(c.s)) }));

    const curveNames = new Map();
    for (const [name, key] of [
      ['curveBasis', 'basis_default'], ['curveBasisClosed', 'basisClosed_default'], ['curveBasisOpen', 'basisOpen_default'],
      ['curveBumpX', 'bumpX'], ['curveBumpY', 'bumpY'], ['curveBundle', 'bundle_default'],
      ['curveCardinalClosed', 'cardinalClosed_default'], ['curveCardinalOpen', 'cardinalOpen_default'], ['curveCardinal', 'cardinal_default'],
      ['curveCatmullRomClosed', 'catmullRomClosed_default'], ['curveCatmullRomOpen', 'catmullRomOpen_default'], ['curveCatmullRom', 'catmullRom_default'],
      ['curveLinear', 'linear_default'], ['curveLinearClosed', 'linearClosed_default'], ['curveMonotoneX', 'monotoneX'],
      ['curveMonotoneY', 'monotoneY'], ['curveNatural', 'natural_default'], ['curveStep', 'step_default'],
      ['curveStepAfter', 'stepAfter'], ['curveStepBefore', 'stepBefore'],
    ]) {
      curveNames.set(d3[key], name);
    }
    const curveByName = new Map([...curveNames].map(([fn, name]) => [name, fn]));
    P.interpolateToCurve = pure.interpolateToCurve.map((c) => {
      const got = utils.interpolateToCurve(dec(c.interpolate), curveByName.get(c.def));
      return { ...c, out: curveNames.get(got) };
    });

    P.formatUrl = pure.formatUrl.map((c) => ({ ...c, out: enc(utilsDefault.formatUrl(c.url, c.config)) }));

    P.roundNumber = pure.roundNumber.map((c) => {
      const precision = dec(c.precision);
      const out = precision === undefined ? utils.roundNumber(dec(c.num)) : utils.roundNumber(dec(c.num), precision);
      return { ...c, out: enc(out) };
    });

    const toPoints = (pts) => pts.map(([x, y]) => ({ x, y }));
    const fromPoints = (pts) => pts.map((p) => [p.x, p.y]);
    P.calcLabelPosition = pure.calcLabelPosition.map((c) => ({ ...c, ...attempt(() => enc(utilsDefault.calcLabelPosition(toPoints(c.points)))) }));
    P.calculatePoint = pure.calculatePoint.map((c) => ({ ...c, ...attempt(() => enc(utils.calculatePoint(toPoints(c.points), c.distance))) }));
    P.calcCardinalityPosition = pure.calcCardinalityPosition.map((c) => {
      const points = toPoints(c.points);
      const initial = c.initial === 'first' ? points[0] : c.initial === 'last' ? points[points.length - 1] : { x: 0, y: 0 };
      const res = attempt(() => enc(utilsDefault.calcCardinalityPosition(c.rel, points, initial)));
      // The function reverses the array of the caller in place.
      return { ...c, ...res, after: fromPoints(points) };
    });
    P.calcTerminalLabelPosition = pure.calcTerminalLabelPosition.map((c) => {
      const points = toPoints(c.points);
      const res = attempt(() => enc(utilsDefault.calcTerminalLabelPosition(c.size, c.position, points)));
      return { ...c, ...res, after: fromPoints(points) };
    });

    P.getStylesFromArray = pure.getStylesFromArray.map((c) => ({ ...c, out: utils.getStylesFromArray(c.arr) }));

    P.parseFontSize = pure.parseFontSize.map((c) => ({ ...c, out: enc(utils.parseFontSize(dec(c.v))) }));

    P.getEdgeId = pure.getEdgeId.map((c) => ({ ...c, out: utils.getEdgeId(c.from, c.to, c.opts, dec(c.id)) }));

    P.handleUndefinedAttr = pure.handleUndefinedAttr.map((c) => ({ ...c, out: enc(utils.handleUndefinedAttr(dec(c.v))) }));
    // isString is not in the bundle: the build drops it as unused.

    P.isLabelCoordinateInPath = pure.isLabelCoordinateInPath.map((c) => ({ ...c, out: utils.isLabelCoordinateInPath(c.point, c.d) }));

    P.isSubstringInArray = pure.isSubstringInArray.map((c) => ({ ...c, ...attempt(() => utils.isSubstringInArray(c.str, c.arr)) }));

    // A merge can throw: a `length` property that lands on an array must
    // be a valid array length.
    P.cleanAndMerge = pure.cleanAndMerge.map((c) => ({
      ...c,
      ...attempt(() => enc(utils.cleanAndMerge(dec(c.a), dec(c.b)))),
    }));

    P.entityDecode = pure.entityDecode.map((c) => ({ ...c, out: utils.entityDecode(c.html) }));

    P.initIDGenerator = pure.initIDGenerator.map((c) => {
      const g = new utils.InitIDGenerator(dec(c.deterministic), dec(c.seed));
      const out = [];
      for (let i = 0; i < c.calls; i++) {
        out.push(g.next());
      }
      return { ...c, out };
    });

    P.getSubGraphTitleMargins = pure.getSubGraphTitleMargins.map((c) => ({ ...c, out: enc(margins.getSubGraphTitleMargins(c.config)) }));

    P.toBase64 = pure.toBase64.map((c) => ({ ...c, out: mermaidMod.__x_toBase64(c.s) }));

    P.markerOffsets = { ...edges.markerOffsets };
    P.markerOffsets2 = { ...edges.markerOffsets2 };
    P.lineWithOffset = pure.lineWithOffset.map((c) => {
      const data = toPoints(c.data);
      const fns = edges.getLineFunctionsWithOffset(c.edge);
      return { ...c, out: data.map((d, i) => [enc(fns.x(d, i, data)), enc(fns.y(d, i, data))]) };
    });

    P.errors = {
      getErrorMessage: [utils.getErrorMessage(new Error('boom')), utils.getErrorMessage('text'), utils.getErrorMessage(12), utils.getErrorMessage(undefined), utils.getErrorMessage(null)],
      isDetailedError: [utils.isDetailedError({ str: 'x', hash: {} }), utils.isDetailedError({ message: 'x' }), utils.isDetailedError(new Error('x'))],
    };

    const st = new state.ImperativeState(() => ({ foo: 1, bar: [] }));
    st.records.foo = 2;
    st.records.bar.push(1);
    const beforeReset = JSON.parse(JSON.stringify(st.records));
    st.reset();
    P.imperativeState = { beforeReset, afterReset: st.records };
  }

  // ---- text measurement, in call order ----
  if (text) {
    result.text = text.map((op) => {
      const config = dec(op.config);
      switch (op.fn) {
        case 'dims':
          return { ...op, ...attempt(() => enc(utils.calculateTextDimensions(op.text, config))) };
        case 'width':
          return { ...op, ...attempt(() => enc(utils.calculateTextWidth(op.text, config))) };
        case 'height':
          return { ...op, ...attempt(() => enc(utils.calculateTextHeight(op.text, config))) };
        case 'wrap':
          return { ...op, ...attempt(() => enc(utils.wrapLabel(op.text, op.maxWidth, config))) };
        case 'break': {
          const hyphen = dec(op.hyphen);
          return { ...op, ...attempt(() => enc(utils.breakString(op.text, op.maxWidth, hyphen, config))) };
        }
      }
      throw new Error(`unknown op ${op.fn}`);
    });
    // The svg of a call that threw stays in the body. Report what is left.
    result.textLeftInBody = [...document.body.children].map((el) => el.tagName.toLowerCase() + (el.id ? '#' + el.id : ''));
    for (const el of [...document.body.children]) {
      if (el.id !== 'container') el.remove();
    }
  }

  // ---- DOM builders ----
  if (dom) {
    const D = result.dom;
    // run calls fn with a selection of a new svg in the container and
    // returns the markup that it leaves there.
    const run = (fn, before = 0) => {
      const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
      container.appendChild(svg);
      for (let i = 0; i < before; i++) {
        svg.appendChild(document.createElementNS('http://www.w3.org/2000/svg', i % 2 ? 'g' : 'circle'));
      }
      const res = attempt(() => fn(select(svg)));
      const html = svg.innerHTML;
      svg.remove();
      return res.error ? { error: res.error, html } : { html, ret: res.out };
    };
    const outer = (sel) => (sel && sel.node ? sel.node().outerHTML : undefined);

    D.getNoteRect = enc(svgDraw.getNoteRect());
    D.getTextObj = enc(svgDraw.getTextObj());
    D.utilsGetTextObj = enc(utils.getTextObj());

    D.drawRect = dom.drawRect.map((c) => {
      const r = run((el) => outer(svgDraw.drawRect(el, dec(c.data))));
      return { ...c, ...r };
    });
    D.drawBackgroundRect = dom.drawBackgroundRect.map((c) => {
      const r = run((el) => enc(svgDraw.drawBackgroundRect(el, dec(c.bounds))), c.before);
      return { ...c, ...r };
    });
    D.drawText = dom.drawText.map((c) => {
      const r = run((el) => outer(svgDraw.drawText(el, dec(c.data))));
      return { ...c, ...r };
    });
    D.drawImage = dom.drawImage.map((c) => ({ ...c, ...run((el) => enc(svgDraw.drawImage(el, c.x, c.y, c.link))) }));
    D.drawEmbeddedImage = dom.drawEmbeddedImage.map((c) => ({ ...c, ...run((el) => enc(svgDraw.drawEmbeddedImage(el, c.x, c.y, c.link))) }));
    D.drawSimpleText = dom.drawSimpleText.map((c) => {
      const r = run((el) => outer(utils.drawSimpleText(el, dec(c.data))));
      return { ...c, ...r };
    });

    // createTooltip: the first call appends the element to the body, and
    // the second call returns the same element.
    const first = svgDraw.createTooltip();
    const firstHtml = first.node().outerHTML;
    const second = svgDraw.createTooltip();
    D.createTooltip = {
      html: firstHtml,
      same: first.node() === second.node(),
      parent: first.node().parentNode.tagName.toLowerCase(),
      bodyChildren: [...document.body.children].map((el) => el.tagName.toLowerCase()),
    };
    first.remove();
  }

  return JSON.stringify(result);
}

// ---- main ----

async function writeGz(file, value) {
  const json = JSON.stringify(value);
  await fs.mkdir(path.dirname(file), { recursive: true });
  await fs.writeFile(file, zlib.gzipSync(json, { level: 9 }));
  process.stderr.write(`${path.relative(REPO, file)}: ${(json.length / 1024).toFixed(0)} KB of JSON\n`);
}

async function main() {
  const labels = corpusLabels();
  process.stderr.write(`${labels.length} label texts from the corpus\n`);
  const pure = buildPureInputs();
  const text = buildTextInputs(labels);
  const dom = buildDomInputs(labels);

  const result = await withBrowser(async (context) => {
    const page = await newPage(context);
    const version = await page.evaluate(() => navigator.userAgent);
    const json = await page.evaluate(pageMain, { chunks: CHUNKS, pure, text, dom });
    const parsed = JSON.parse(json);
    parsed.userAgent = version;
    return parsed;
  });

  const meta = { mermaid: JSON.parse(await fs.readFile(path.join(HERE, 'node_modules/mermaid/package.json'), 'utf8')).version, userAgent: result.userAgent };
  const { toStringRadix, escape, unescape, ...pureRest } = result.pure;
  await writeGz(path.join(REPO, 'internal/js/testdata/utils.json.gz'), { meta, toStringRadix, escape, unescape });
  await writeGz(path.join(REPO, 'internal/utils/testdata/pure.json.gz'), { meta, ...pureRest });
  await writeGz(path.join(REPO, 'internal/utils/testdata/text.json.gz'), { meta, ops: result.text, leftInBody: result.textLeftInBody });
  const { drawSimpleText, utilsGetTextObj, ...svgdraw } = result.dom;
  await writeGz(path.join(REPO, 'internal/utils/testdata/dom.json.gz'), { meta, drawSimpleText, getTextObj: utilsGetTextObj });
  await writeGz(path.join(REPO, 'internal/svgdraw/testdata/dom.json.gz'), { meta, ...svgdraw });
}

await main();
