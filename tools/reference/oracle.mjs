#!/usr/bin/env node
// Reference oracle for mmdg.
//
// Runs upstream mermaid (pinned in package.json) in one headless Chromium and
// writes what a browser produces. The Go renderer is compared against this
// output. The tool has three commands:
//
//   render  .mmd files -> the string that mermaid.render() returns (.svg);
//           --timeout MS limits each diagram (default 60000)
//   shoot   .svg files -> PNG screenshots of the SVG at its natural size
//   eval    a JS file  -> JSON result of running it in a page (calibration)
//   esm-eval  like eval, with access to mermaid internals (see newEsmPage)
//   bench   .mmd files -> one JSON file with the times of mermaid.render()
//           (see cmdBench)
//
// --webfonts loads the web fonts and the style sheets of the page of
// mermaid-cli 12 into the page (see addWebFonts). It works with render,
// shoot, eval and esm-eval.
//
// Determinism: Math.random is a fixed-seed PRNG, the clock is frozen, and the
// timezone and locale are fixed. The Go renderer uses the same values.
import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const MERMAID_JS = path.join(HERE, 'node_modules/mermaid/dist/mermaid.min.js');

// Shared with the Go side (internal/oracle constants). Change both together.
export const FIXED_TIME = '2026-01-15T12:00:00Z';
export const RANDOM_SEED = 0x6d6d6467; // "mmdg"
export const VIEWPORT = { width: 800, height: 600 }; // mmdc defaults

const SEEDED_RANDOM = `(() => {
  let a = ${RANDOM_SEED} >>> 0;
  globalThis.__mmdgRandomCalls = 0;
  globalThis.__mmdgResetRandom = () => {
    a = ${RANDOM_SEED} >>> 0;
    globalThis.__mmdgRandomCalls = 0;
  };
  Math.random = function () {
    globalThis.__mmdgRandomCalls++;
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
})();`;

const PAGE_HTML =
  '<!doctype html><html><head><meta charset="utf-8"></head>' +
  '<body><div id="container"></div></body></html>';

// The page of mermaid-cli 12 has web fonts and style sheets that a plain
// page has not. internal/webfonts has a copy of the files. addWebFonts adds
// them to a page as mermaid-cli does (src/index.js, renderMermaid): one link
// element for each style sheet in the head, then a load of every font face.
// The files come from a fake origin with CORS headers, as in mermaid-cli.
// The order of the list is the order of internal/webfonts.Sheets.
const WEBFONT_ORIGIN = 'http://webfonts.mmdg.local';
const WEBFONT_DIR = path.join(HERE, '../../internal/webfonts/files');
export const WEBFONT_SHEETS = [
  'fontawesome-free/css/brands.css',
  'fontawesome-free/css/regular.css',
  'fontawesome-free/css/solid.css',
  'fontawesome-free/css/fontawesome.css',
  'recursive/index.css',
  'open-sans/400.css',
  'open-sans/400-italic.css',
  'katex/dist/katex.css',
];

export async function addWebFonts(page) {
  await page.route(`${WEBFONT_ORIGIN}/**`, async (route) => {
    const url = new URL(route.request().url());
    const file = path.join(WEBFONT_DIR, decodeURIComponent(url.pathname));
    const headers = { 'access-control-allow-origin': '*' };
    if (!file.startsWith(WEBFONT_DIR + path.sep)) {
      return route.fulfill({ status: 403, headers, body: 'forbidden' });
    }
    try {
      const body = await fs.readFile(file);
      const contentType = file.endsWith('.css') ? 'text/css' : file.endsWith('.woff2') ? 'font/woff2' : 'application/octet-stream';
      return route.fulfill({ status: 200, contentType, headers, body });
    } catch {
      return route.fulfill({ status: 404, headers, body: 'not found' });
    }
  });
  await page.evaluate(
    async ({ origin, sheets }) => {
      for (const sheet of sheets) {
        const link = document.createElement('link');
        link.setAttribute('crossorigin', 'anonymous');
        link.setAttribute('rel', 'stylesheet');
        link.setAttribute('href', `${origin}/${sheet}`);
        link.setAttribute('class', 'mermaid-cli-css');
        const loaded = new Promise((resolve, reject) => {
          link.onload = () => resolve();
          link.onerror = () => reject(new Error(`Failed to load stylesheet: ${sheet}`));
        });
        document.head.appendChild(link);
        await loaded;
      }
      await Promise.all(Array.from(document.fonts, (font) => font.load()));
    },
    { origin: WEBFONT_ORIGIN, sheets: WEBFONT_SHEETS }
  );
}

function parseArgs(argv) {
  const opts = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (next === undefined || next.startsWith('--')) {
        opts[key] = true;
      } else {
        opts[key] = next;
        i++;
      }
    } else {
      opts._.push(a);
    }
  }
  return opts;
}

async function readList(opts) {
  const files = [...opts._];
  if (opts.list) {
    const text = await fs.readFile(opts.list, 'utf8');
    files.push(...text.split('\n').map((s) => s.trim()).filter(Boolean));
  }
  return files;
}

// outPath maps an input file to its output location. With --base, the path
// relative to the base directory is kept under --out.
function outPath(file, opts, ext) {
  const rel = opts.base ? path.relative(opts.base, file) : path.basename(file);
  return path.join(opts.out, rel.replace(/\.[^./]+$/, '') + ext);
}

async function newContext(browser, scale = 1) {
  return browser.newContext({
    viewport: VIEWPORT,
    deviceScaleFactor: scale,
    locale: 'en-US',
    timezoneId: 'UTC',
    colorScheme: 'light',
  });
}

async function newMermaidPage(context, opts = {}) {
  const page = await context.newPage();
  await page.clock.install({ time: new Date(FIXED_TIME) });
  await page.clock.setFixedTime(new Date(FIXED_TIME));
  await page.setContent(PAGE_HTML);
  // setContent does not navigate, so an init script would not run. Install
  // the seeded Math.random in the page directly, before mermaid loads.
  await page.evaluate(SEEDED_RANDOM);
  await page.addScriptTag({ path: MERMAID_JS });
  if (opts.webfonts) {
    await addWebFonts(page);
  }
  return page;
}

async function cmdRender(browser, opts) {
  const files = await readList(opts);
  const id = opts.id || 'my-svg';
  const config = opts.config ? JSON.parse(await fs.readFile(opts.config, 'utf8')) : {};
  const timeoutMs = Number(opts.timeout || 60000);
  const context = await newContext(browser);
  let ok = 0;
  let failed = 0;
  try {
    for (const file of files) {
      const text = await fs.readFile(file, 'utf8');
      // One page per diagram: mermaid keeps module-level counters and config.
      const page = await newMermaidPage(context, opts);
      let result;
      let timer;
      try {
        // A diagram that never finishes must not hold the lock: other
        // callers wait for it. The page is closed after the time limit.
        const limit = new Promise((resolve) => {
          timer = setTimeout(
            () => resolve({ error: `oracle: no result after ${timeoutMs} ms` }),
            timeoutMs
          );
        });
        const evaluated = page.evaluate(
          async ({ text, id, config }) => {
            const mermaid = globalThis.mermaid;
            mermaid.initialize({ startOnLoad: false, ...config });
            const container = document.getElementById('container');
            const started = performance.now();
            try {
              const randomBefore = globalThis.__mmdgRandomCalls;
              const { svg, diagramType } = await mermaid.render(id, text, container);
              return {
                svg,
                diagramType,
                ms: performance.now() - started,
                randomBefore,
                randomCalls: globalThis.__mmdgRandomCalls - randomBefore,
                now: new Date().toISOString(),
              };
            } catch (e) {
              return { error: String((e && e.message) || e) };
            }
          },
          { text, id, config }
        );
        // The evaluation of a page that is closed after the limit rejects.
        evaluated.catch(() => {});
        result = await Promise.race([evaluated, limit]);
      } catch (e) {
        result = { error: 'oracle: ' + String((e && e.message) || e) };
      } finally {
        clearTimeout(timer);
        // A page that is busy in a loop does not close by itself.
        await Promise.race([
          page.close({ runBeforeUnload: false }),
          new Promise((resolve) => setTimeout(resolve, 5000)),
        ]);
      }
      const svgFile = outPath(file, opts, '.svg');
      await fs.mkdir(path.dirname(svgFile), { recursive: true });
      const meta = { source: file, mermaid: opts.mermaidVersion };
      if (result.error) {
        failed++;
        meta.error = result.error;
        await fs.rm(svgFile, { force: true });
      } else {
        ok++;
        meta.diagramType = result.diagramType;
        meta.renderMs = Math.round(result.ms * 100) / 100;
        meta.randomBefore = result.randomBefore;
        meta.randomCalls = result.randomCalls;
        meta.now = result.now;
        await fs.writeFile(svgFile, result.svg);
      }
      await fs.writeFile(outPath(file, opts, '.json'), JSON.stringify(meta, null, 1) + '\n');
      if (!opts.quiet) {
        process.stderr.write(`${result.error ? 'ERR ' : 'ok  '} ${file}\n`);
      }
    }
  } finally {
    await context.close();
  }
  process.stderr.write(`render: ${ok} ok, ${failed} failed\n`);
}

async function cmdShoot(browser, opts) {
  const files = await readList(opts);
  const scale = Number(opts.scale || 1);
  const context = await newContext(browser, scale);
  try {
    const page = await context.newPage();
    for (const file of files) {
      const svg = await fs.readFile(file, 'utf8');
      const pngFile = opts.out ? outPath(file, opts, '.png') : file.replace(/\.svg$/, '') + '.png';
      await fs.mkdir(path.dirname(pngFile), { recursive: true });
      try {
        // A wide viewport first, so "width: 100%; max-width" resolves to the
        // natural width. Then shrink the viewport to the measured size.
        await page.setViewportSize({ width: 16000, height: 600 });
        await page.setContent(
          '<!doctype html><html><head><meta charset="utf-8">' +
            '<style>html,body{margin:0;padding:0;background:white}svg{display:block}</style>' +
            `</head><body>${svg}</body></html>`
        );
        if (opts.webfonts) {
          // The page of mermaid-cli: its style sheets and its web fonts.
          await addWebFonts(page);
        }
        const box = await page.evaluate(() => {
          const r = document.querySelector('body > svg').getBoundingClientRect();
          return { width: r.width, height: r.height };
        });
        const width = Math.max(1, Math.ceil(box.width));
        const height = Math.max(1, Math.ceil(box.height));
        await page.setViewportSize({ width, height });
        await page.screenshot({
          path: pngFile,
          clip: { x: 0, y: 0, width, height },
          animations: 'disabled',
          caret: 'hide',
        });
        if (!opts.quiet) {
          process.stderr.write(`ok   ${pngFile} ${width}x${height}@${scale}\n`);
        }
      } catch (e) {
        process.stderr.write(`ERR  ${file}: ${String((e && e.message) || e)}\n`);
      }
    }
  } finally {
    await context.close();
  }
}

// eval runs a script file inside a page that has mermaid loaded. The script is
// the body of an async function with `args` in scope and returns JSON data.
async function cmdEval(browser, opts) {
  const [scriptFile] = opts._;
  const body = await fs.readFile(scriptFile, 'utf8');
  const args = opts.args ? JSON.parse(await fs.readFile(opts.args, 'utf8')) : null;
  const context = await newContext(browser, Number(opts.scale || 1));
  try {
    const page = opts.bare ? await context.newPage() : await newMermaidPage(context, opts);
    if (opts.bare) {
      await page.setContent(PAGE_HTML);
      if (opts.webfonts) {
        await addWebFonts(page);
      }
    }
    const result = await page.evaluate(
      ({ body, args }) => new Function('args', `return (async () => {${body}\n})();`)(args),
      { body, args }
    );
    const json = JSON.stringify(result, null, 1) + '\n';
    if (opts.out) {
      await fs.mkdir(path.dirname(opts.out), { recursive: true });
      await fs.writeFile(opts.out, json);
    } else {
      process.stdout.write(json);
    }
  } finally {
    await context.close();
  }
}

// bench measures the time of mermaid.render() for each diagram and writes
// one JSON file. tools/perfdoc compares these times with the Go renderer.
//
//   node oracle.mjs bench --base DIR --list FILE --out FILE [--repeat N] [--fresh]
//
// Without --fresh one page stays open for all diagrams and renders each
// diagram N times (default 7). This is mermaid in a warm page: V8 has
// compiled the code. With --fresh each diagram has a new page that has
// mermaid loaded, so the first time is the first render of that page.
//
// The clock of the page is the real clock, because the fake clock of the
// render command counts in whole milliseconds. Math.random has the fixed
// seed. mermaid.initialize is not in the measured time.
async function cmdBench(browser, opts) {
  const files = await readList(opts);
  const repeat = Number(opts.repeat || 7);
  const timeoutMs = Number(opts.timeout || 60000);
  const context = await newContext(browser);
  const openPage = async () => {
    const page = await context.newPage();
    await page.setContent(PAGE_HTML);
    await page.evaluate(SEEDED_RANDOM);
    await page.addScriptTag({ path: MERMAID_JS });
    return page;
  };
  const closePage = (page) =>
    Promise.race([
      page.close({ runBeforeUnload: false }),
      new Promise((resolve) => setTimeout(resolve, 5000)),
    ]);
  const results = {};
  let page = null;
  let ok = 0;
  let failed = 0;
  try {
    for (const file of files) {
      const text = await fs.readFile(file, 'utf8');
      if (opts.fresh && page) {
        await closePage(page);
        page = null;
      }
      if (!page) {
        page = await openPage();
      }
      let result;
      let timer;
      try {
        const limit = new Promise((resolve) => {
          timer = setTimeout(() => resolve({ error: `oracle: no result after ${timeoutMs} ms`, hung: true }), timeoutMs);
        });
        const evaluated = page.evaluate(
          async ({ text, repeat }) => {
            const mermaid = globalThis.mermaid;
            const container = document.getElementById('container');
            const times = [];
            let diagramType;
            for (let i = 0; i < repeat; i++) {
              mermaid.initialize({ startOnLoad: false });
              globalThis.__mmdgResetRandom();
              const started = performance.now();
              try {
                const r = await mermaid.render('my-svg', text, container);
                times.push(Math.round((performance.now() - started) * 100) / 100);
                diagramType = r.diagramType;
              } catch (e) {
                return { error: String((e && e.message) || e) };
              }
            }
            return { diagramType, times };
          },
          { text, repeat }
        );
        evaluated.catch(() => {});
        result = await Promise.race([evaluated, limit]);
      } catch (e) {
        result = { error: 'oracle: ' + String((e && e.message) || e), hung: true };
      } finally {
        clearTimeout(timer);
      }
      if (result.hung) {
        // The page can be busy in a loop. The next diagram has a new page.
        delete result.hung;
        await closePage(page);
        page = null;
      }
      if (result.error) {
        failed++;
      } else {
        ok++;
      }
      const rel = opts.base ? path.relative(opts.base, file) : path.basename(file);
      results[rel.split(path.sep).join('/')] = result;
      if (!opts.quiet && (ok + failed) % 100 === 0) {
        process.stderr.write(`bench: ${ok + failed} of ${files.length}\n`);
      }
    }
  } finally {
    await context.close();
  }
  const out = {
    mermaid: opts.mermaidVersion,
    chromium: browser.version(),
    platform: `${process.platform}/${process.arch}`,
    mode: opts.fresh ? 'fresh' : 'shared',
    repeat,
    results,
  };
  await fs.mkdir(path.dirname(opts.out), { recursive: true });
  await fs.writeFile(opts.out, JSON.stringify(out) + '\n');
  process.stderr.write(`bench: ${ok} ok, ${failed} failed\n`);
}

// Only one browser runs at a time. A lock directory serializes callers.
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
        // Only one waiter can take the stale lock away: rename is atomic.
        // Removing it directly would let two waiters each remove the lock
        // and then each create a new one.
        const grave = `${LOCK_DIR}.stale.${process.pid}`;
        try {
          await fs.rename(LOCK_DIR, grave);
          await fs.rm(grave, { recursive: true, force: true });
        } catch {
          // Another waiter took it.
        }
        continue;
      }
      await new Promise((resolve) => setTimeout(resolve, 250));
    }
  }
}

// ---- esm-eval: access to mermaid internals ----
//
// The IIFE bundle hides everything but the public API. The ESM build has the
// same code as readable modules with real names. esm-eval serves that build
// to the page from a fake origin and adds an export for every top-level
// function and variable of every chunk, so that a script can call internal
// functions such as insertNode, createText or a diagram db:
//
//   const { insertNode, getConfig } = await internals('insertNode', 'getConfig');
//   const draw = await internalsFrom('flowDiagram', 'draw');   // pick the chunk by name
//
// `mermaid` is the default export of the ESM build. State (config, registered
// diagrams) is shared between `mermaid` and the internals, because they are
// the same module instances.
const ESM_ORIGIN = 'http://mmdg.local';
const DIST_DIR = path.join(HERE, 'node_modules/mermaid/dist');

async function buildEsmIndex() {
  const dir = path.join(DIST_DIR, 'chunks/mermaid.esm');
  // No prototype: internal names such as `constructor` must not collide.
  const index = Object.create(null); // name -> [{ file, key }]
  const extra = Object.create(null); // file -> [names to export]
  // The entry module holds mermaidAPI (createUserStyles, renderDiagram).
  const files = [...(await fs.readdir(dir)).filter((f) => f.endsWith('.mjs')), 'mermaid.esm.mjs'];
  for (const file of files) {
    const isEntry = file === 'mermaid.esm.mjs';
    const src = await fs.readFile(isEntry ? path.join(DIST_DIR, file) : path.join(dir, file), 'utf8');
    const exported = new Set();
    for (const block of src.matchAll(/^export \{([^}]*)\};?/gm)) {
      for (const item of block[1].split(',')) {
        const m = item.trim().match(/^(\S+)(?:\s+as\s+(\S+))?$/);
        if (m) {
          const key = m[2] || m[1];
          exported.add(m[1]);
          (index[m[1]] ||= []).push({ file, key });
          if (m[2]) {
            (index[m[2]] ||= []).push({ file, key });
          }
        }
      }
    }
    const names = new Set();
    for (const m of src.matchAll(/^(?:async function\*?|function\*?|var|let|const|class) ([A-Za-z_$][\w$]*)/gm)) {
      if (!exported.has(m[1])) {
        names.add(m[1]);
      }
    }
    extra[file] = [...names];
    for (const name of names) {
      (index[name] ||= []).push({ file, key: `__x_${name}` });
    }
  }
  return { index, extra };
}

async function newEsmPage(context) {
  const { index, extra } = await buildEsmIndex();
  const page = await context.newPage();
  await page.clock.install({ time: new Date(FIXED_TIME) });
  await page.clock.setFixedTime(new Date(FIXED_TIME));
  await page.route(`${ESM_ORIGIN}/**`, async (route) => {
    const url = new URL(route.request().url());
    if (url.pathname === '/index.html') {
      return route.fulfill({ contentType: 'text/html', body: PAGE_HTML });
    }
    try {
      let body = await fs.readFile(path.join(DIST_DIR, url.pathname), 'utf8');
      const names = extra[path.basename(url.pathname)];
      if (names && names.length > 0) {
        body += `\nexport { ${names.map((n) => `${n} as __x_${n}`).join(', ')} };\n`;
      }
      return route.fulfill({ contentType: 'text/javascript', body });
    } catch {
      return route.fulfill({ status: 404, body: 'not found' });
    }
  });
  await page.goto(`${ESM_ORIGIN}/index.html`);
  await page.evaluate(SEEDED_RANDOM);
  await page.evaluate(async (index) => {
    globalThis.mermaid = (await import('/mermaid.esm.mjs')).default;
    const load = async (name, chunkHint) => {
      let hits = index[name] || [];
      if (chunkHint) {
        hits = hits.filter((h) => h.file.includes(chunkHint));
      }
      if (hits.length === 0) {
        throw new Error(`internals: ${name} not found${chunkHint ? ' in ' + chunkHint : ''}`);
      }
      const files = [...new Set(hits.map((h) => h.file))];
      if (files.length > 1) {
        throw new Error(`internals: ${name} is defined in several chunks (${files.join(', ')}); use internalsFrom(chunk, name)`);
      }
      const url = hits[0].file === 'mermaid.esm.mjs' ? '/mermaid.esm.mjs' : `/chunks/mermaid.esm/${hits[0].file}`;
      const mod = await import(url);
      return mod[hits[0].key];
    };
    globalThis.internals = async (...names) => Object.fromEntries(await Promise.all(names.map(async (n) => [n, await load(n)])));
    globalThis.internalsFrom = (chunkHint, name) => load(name, chunkHint);
    globalThis.internalsWhere = (name) => (index[name] || []).map((h) => h.file);
  }, index);
  return page;
}

async function cmdEsmEval(browser, opts) {
  const [scriptFile] = opts._;
  const body = await fs.readFile(scriptFile, 'utf8');
  const args = opts.args ? JSON.parse(await fs.readFile(opts.args, 'utf8')) : null;
  const context = await newContext(browser, Number(opts.scale || 1));
  try {
    const page = await newEsmPage(context);
    if (opts.webfonts) {
      await addWebFonts(page);
    }
    const result = await page.evaluate(
      ({ body, args }) => new Function('args', `return (async () => {${body}\n})();`)(args),
      { body, args }
    );
    const json = JSON.stringify(result, null, 1) + '\n';
    if (opts.out) {
      await fs.mkdir(path.dirname(opts.out), { recursive: true });
      await fs.writeFile(opts.out, json);
    } else {
      process.stdout.write(json);
    }
  } finally {
    await context.close();
  }
}

async function main() {
  const [command, ...rest] = process.argv.slice(2);
  const opts = parseArgs(rest);
  const pkg = JSON.parse(
    await fs.readFile(path.join(HERE, 'node_modules/mermaid/package.json'), 'utf8')
  );
  opts.mermaidVersion = pkg.version;
  const commands = { render: cmdRender, shoot: cmdShoot, eval: cmdEval, 'esm-eval': cmdEsmEval, bench: cmdBench };
  if (!commands[command]) {
    process.stderr.write('usage: oracle.mjs <render|shoot|eval|esm-eval|bench> [--out DIR] [--base DIR] files...\n');
    process.exit(2);
  }
  if ((command === 'render' || command === 'bench') && !opts.out) {
    process.stderr.write(`${command}: --out is required\n`);
    process.exit(2);
  }
  // oracle-linux.mjs holds the lock on the host and runs this file in a
  // container, where the process ids of the host do not exist.
  const lockHeld = process.env.MMDG_ORACLE_LOCK_HELD === '1';
  if (!lockHeld) {
    await acquireLock();
  }
  let browser;
  try {
    browser = await chromium.launch({ headless: true });
    await commands[command](browser, opts);
  } finally {
    if (browser) {
      await browser.close();
    }
    if (!lockHeld) {
      await fs.rm(LOCK_DIR, { recursive: true, force: true });
    }
  }
}

await main();
