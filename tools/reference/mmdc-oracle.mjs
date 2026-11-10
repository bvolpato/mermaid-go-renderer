#!/usr/bin/env node
// Reference of the files that mermaid-cli (mmdc) writes.
//
// oracle.mjs runs mermaid in a plain page. mmdc renders in a page of its
// own, which has web fonts and style sheets, and it changes the SVG after
// mermaid.render(). This script runs the code of mmdc itself: the exported
// renderMermaid of @mermaid-js/mermaid-cli (src/index.js), which the mmdc
// command calls for every diagram, with the options that the command passes
// when it has no arguments. One browser renders all the files of a run.
//
//   node mmdc-oracle.mjs render --out DIR [--base DIR] [--list FILE]
//        [--timeout MS] [--id ID] [--scale N] [--check-embed] [--race-links] files...
//
// For every .mmd file it writes, as oracle.mjs render does:
//
//   <rel>.svg    the .svg file of mmdc --no-font-embed
//   <rel>.json   diagramType or error, and what describes the run
//
// mmdc embeds the fonts of its page into the SVG by default: it adds style
// elements with the font data after the style element of mermaid. Such a
// file is large, so the script does not store it. The .json has its
// SHA-256 (embedSha256) and the text of the added style elements
// (embedStyles), where each font is "sha256:" and the hash of its bytes in
// place of the base64 text. Both variants come from one render: the script
// takes the XML of the svg element right before mmdc runs its font
// embedder. --check-embed renders each file a second time with
// fontEmbed: false and fails when that file differs from the .svg.
//
// --scale N renders with the device scale factor N, as mmdc --scale N.
//
// --race-links keeps the order of the style sheets in the head as mmdc
// makes it, which changes from page to page (see orderLinks). Without the
// option the order is the order of the source of mmdc.
//
// Determinism, as in oracle.mjs: Math.random is the same seeded generator,
// Date gives the same fixed time, the timezone is UTC and the locale is
// en-US. The script installs them in every page before a script of the page
// runs. mmdc itself has none of this. The window is 800 by 600, which is
// what mmdc has.
//
// The browser is the headless shell of the Chromium of Playwright, the
// binary that oracle.mjs launches. The script takes the oracle lock, so
// that only one browser runs at a time. oracle-linux.mjs --script
// mmdc-oracle.mjs runs it in the Linux image.
import puppeteer from 'puppeteer';
import { chromium } from 'playwright';
import { renderMermaid } from '@mermaid-js/mermaid-cli';
import crypto from 'node:crypto';
import fs from 'node:fs/promises';
import fsSync from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { createRequire } from 'node:module';

const HERE = path.dirname(fileURLToPath(import.meta.url));

// The runner uses this contract to reject results from another install or
// another version of this oracle. Resolve Mermaid from the CLI itself.
export async function referenceContract() {
  const cli = fileURLToPath(import.meta.resolve('@mermaid-js/mermaid-cli'));
  const require = createRequire(cli);
  const version = async (file) => JSON.parse(await fs.readFile(file, 'utf8')).version;
  return {
    mermaid: await version(require.resolve('mermaid/package.json')),
    mmdc: await version(path.resolve(path.dirname(cli), '../package.json')),
    puppeteer: await version(require.resolve('puppeteer/package.json')),
    playwright: await version(path.join(HERE, 'node_modules/playwright/package.json')),
    packageLockSha256: sha256(await fs.readFile(path.join(HERE, 'package-lock.json'))),
    oracleSha256: sha256(await fs.readFile(fileURLToPath(import.meta.url))),
  };
}

// The same values as oracle.mjs.
const FIXED_TIME = '2026-01-15T12:00:00Z';
const RANDOM_SEED = 0x6d6d6467; // "mmdg"

// INIT runs in every document before its scripts. The random generator is
// the generator of oracle.mjs. Date is the Date of the browser with a fixed
// current time: Date.now(), new Date() and Date() give FIXED_TIME, and a
// Date with arguments is not changed. Timers keep running.
const INIT = `(() => {
  let a = ${RANDOM_SEED} >>> 0;
  globalThis.__mmdgRandomCalls = 0;
  Math.random = function () {
    globalThis.__mmdgRandomCalls++;
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
  const fixed = ${Date.parse(FIXED_TIME)};
  const RealDate = Date;
  function FixedDate(...args) {
    if (!new.target) {
      return new RealDate(fixed).toString();
    }
    const date = args.length === 0 ? new RealDate(fixed) : new RealDate(...args);
    Object.setPrototypeOf(date, new.target.prototype);
    return date;
  }
  FixedDate.prototype = RealDate.prototype;
  FixedDate.prototype.constructor = FixedDate;
  FixedDate.now = () => fixed;
  FixedDate.parse = RealDate.parse;
  FixedDate.UTC = RealDate.UTC;
  globalThis.Date = FixedDate;
})();`;

// The options that the mmdc command passes to renderMermaid when it has no
// arguments (cli in src/index.js). The window size 0 by 0 keeps the size of
// the window of the browser.
const MMDC_OPTIONS = {
  mermaidConfig: { theme: undefined },
  backgroundColor: 'white',
  fontEmbed: true,
  pdfPaperFormat: undefined,
  size: undefined,
  viewport: { width: 0, height: 0, deviceScaleFactor: 1 },
  iconPacks: {},
};

// The options that take no value.
const FLAGS = new Set(['quiet', 'check-embed', 'race-links']);

function parseArgs(argv) {
  const opts = { _: [] };
  for (let i = 0; i < argv.length; i++) {
    const a = argv[i];
    if (a.startsWith('--')) {
      const key = a.slice(2);
      const next = argv[i + 1];
      if (FLAGS.has(key) || next === undefined || next.startsWith('--')) {
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

function outPath(file, opts, ext) {
  const rel = opts.base ? path.relative(opts.base, file) : path.basename(file);
  return path.join(opts.out, rel.replace(/\.[^./]+$/, '') + ext);
}

// browserExecutable is the chrome-headless-shell of the Chromium revision
// of Playwright. The full browser of the same revision is the fallback.
function browserExecutable() {
  const full = chromium.executablePath();
  const m = full.match(/^(.*)[\\/]chromium-(\d+)[\\/]/);
  if (m) {
    const dir = path.join(m[1], `chromium_headless_shell-${m[2]}`);
    if (fsSync.existsSync(dir)) {
      for (const sub of fsSync.readdirSync(dir)) {
        for (const name of ['chrome-headless-shell', 'headless_shell', 'chrome-headless-shell.exe']) {
          const candidate = path.join(dir, sub, name);
          if (fsSync.existsSync(candidate)) {
            return { executablePath: candidate, headless: 'shell' };
          }
        }
      }
    }
  }
  return { executablePath: full, headless: true };
}

// ---- lock: a copy of the code of oracle.mjs ----

const LOCK_DIR = path.join(HERE, '.oracle.lock');
const PID_FILE = path.join(LOCK_DIR, 'pid');
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function lockOwner() {
  try {
    return Number(await fs.readFile(PID_FILE, 'utf8'));
  } catch {
    return 0;
  }
}

async function acquireLock() {
  for (;;) {
    try {
      await fs.mkdir(LOCK_DIR);
      await fs.writeFile(PID_FILE, String(process.pid));
      await sleep(60);
      if ((await lockOwner()) === process.pid) {
        return;
      }
      continue;
    } catch {
      let stale = false;
      try {
        const pid = Number(await fs.readFile(PID_FILE, 'utf8'));
        process.kill(pid, 0);
      } catch (e) {
        if (e.code === 'ESRCH') {
          stale = true;
        } else if (e.code === 'ENOENT') {
          // mkdir and the PID write are separate operations. Give a new
          // owner time to write its PID before treating the lock as stale.
          await sleep(300);
          try {
            await fs.readFile(PID_FILE, 'utf8');
          } catch (again) {
            stale = again.code === 'ENOENT';
          }
        }
      }
      if (stale) {
        // Only one waiter can take the stale lock away: rename is atomic.
        const grave = `${LOCK_DIR}.stale.${process.pid}`;
        try {
          await fs.rename(LOCK_DIR, grave);
          await fs.rm(grave, { recursive: true, force: true });
        } catch {
          // Another waiter took it.
        }
        continue;
      }
      await sleep(250);
    }
  }
}

// CSS_IMPORTS is the cssImports list of renderMermaid: the style sheets of
// the page of mmdc, in the order of the source.
const CSS_IMPORTS = [
  '@fortawesome/fontawesome-free/css/brands.css',
  '@fortawesome/fontawesome-free/css/regular.css',
  '@fortawesome/fontawesome-free/css/solid.css',
  '@fortawesome/fontawesome-free/css/fontawesome.css',
  '@fontsource-variable/recursive/index.css',
  '@fontsource/open-sans/400.css',
  '@fontsource/open-sans/400-italic.css',
  'katex/dist/katex.css',
];

// orderLinks makes the order of the style sheets in the head of the page
// the order of CSS_IMPORTS. mmdc adds the link elements from calls that run
// at the same time (Promise.all over cssImports, each call waits for a
// realpath of the file system first), so their order in the head changes
// from page to page. The order decides the cascade between the style
// sheets and the order of the style elements that the font embedder adds.
// A reference needs one order: the order of the source. orderLinks holds
// the calls of page.evaluate that add a link until all have come, and then
// runs them one after the other.
function orderLinks(page) {
  const evaluate = page.evaluate.bind(page);
  const pending = [];
  let timer;
  const flush = async () => {
    clearTimeout(timer);
    const batch = pending.splice(0).sort((a, b) => a.order - b.order);
    for (const call of batch) {
      try {
        call.resolve(await evaluate(call.fn, ...call.args));
      } catch (e) {
        call.reject(e);
      }
    }
  };
  page.evaluate = (fn, ...args) => {
    const href = args[0] && typeof args[0] === 'object' ? args[0].href : undefined;
    if (typeof href !== 'string' || !href.endsWith('.css')) {
      return evaluate(fn, ...args);
    }
    return new Promise((resolve, reject) => {
      const order = CSS_IMPORTS.findIndex((name) => href.endsWith(`/node_modules/${name}`));
      pending.push({ fn, args, resolve, reject, order: order < 0 ? CSS_IMPORTS.length : order });
      clearTimeout(timer);
      if (pending.length >= CSS_IMPORTS.length) {
        flush();
      } else {
        // Not all calls came: do not wait forever.
        timer = setTimeout(flush, 5000);
      }
    });
  };
}

// instrument makes every page of the browser deterministic, and takes two
// records from the page of a render right before mmdc runs its font
// embedder: the XML of the svg element, which is the file of mmdc
// --no-font-embed, and facts about the page. state gets the records.
function instrument(browser, state, opts) {
  const newPage = browser.newPage.bind(browser);
  browser.newPage = async (...args) => {
    const page = await newPage(...args);
    state.page = page;
    await page.evaluateOnNewDocument(INIT);
    await page.emulateTimezone('UTC');
    await page.emulateMediaFeatures([{ name: 'prefers-color-scheme', value: 'light' }]);
    const client = await page.createCDPSession();
    await client.send('Emulation.setLocaleOverride', { locale: 'en-US' });
    if (!opts['race-links']) {
      orderLinks(page);
    }

    const $eval = page.$eval.bind(page);
    page.$eval = async (selector, fn, ...rest) => {
      // renderMermaid calls page.$eval("svg", ..., { browserUtilsUrl, fontEmbed })
      // to embed the fonts and to serialize the svg element.
      const isSerialize = selector === 'svg' && rest[0] && typeof rest[0] === 'object' && 'fontEmbed' in rest[0];
      if (!isSerialize) {
        return $eval(selector, fn, ...rest);
      }
      const before = await $eval('svg', (svg) => {
        // The style elements that follow the style element of mermaid.
        let styles = 0;
        for (let s = svg.querySelector('style')?.nextElementSibling; s && s.localName === 'style'; s = s.nextElementSibling) {
          styles++;
        }
        return {
          xml: new XMLSerializer().serializeToString(svg),
          styles,
          diagramType: svg.getAttribute('aria-roledescription'),
          randomCalls: globalThis.__mmdgRandomCalls,
          now: new Date().toISOString(),
          innerWidth: globalThis.innerWidth,
          innerHeight: globalThis.innerHeight,
          devicePixelRatio: globalThis.devicePixelRatio,
          timeZone: Intl.DateTimeFormat().resolvedOptions().timeZone,
          language: navigator.language,
          // The style sheets of the head, in the order of the document.
          links: Array.from(document.head.querySelectorAll('link'), (link) => link.href),
          fontFaces: document.fonts.size,
          fontFacesLoaded: Array.from(document.fonts).filter((f) => f.status === 'loaded').length,
        };
      });
      const xml = await $eval(selector, fn, ...rest);
      // The font embedder adds its style elements right after the style
      // element of mermaid, the last one first.
      const added = await $eval(
        'svg',
        (svg, before) => {
          const texts = [];
          for (let s = svg.querySelector('style')?.nextElementSibling; s && s.localName === 'style'; s = s.nextElementSibling) {
            texts.push(s.textContent);
          }
          return texts.slice(0, texts.length - before);
        },
        before.styles
      );
      Object.assign(state, { plain: before.xml, info: before, embedStyles: added });
      return xml;
    };
    return page;
  };
}

const sha256 = (data) => crypto.createHash('sha256').update(data).digest('hex');

// hashFonts replaces the base64 text of every font in a style text with
// "sha256:" and the hash of the bytes of the font.
const hashFonts = (text) =>
  text.replace(/(data:[^;,"]*;base64,)([A-Za-z0-9+/=]+)/g, (_, head, data) => `${head}sha256:${sha256(Buffer.from(data, 'base64'))}`);

// linkName is the path of a style sheet below node_modules.
const linkName = (href) => href.replace(/^.*\/node_modules\//, '');

async function renderOne(browser, state, text, opts, timeoutMs, fontEmbed) {
  for (const key of Object.keys(state)) {
    delete state[key];
  }
  let timer;
  const limit = new Promise((resolve) => {
    timer = setTimeout(() => resolve({ timedOut: true }), timeoutMs);
  });
  const options = { ...MMDC_OPTIONS, fontEmbed, svgId: opts.id || undefined };
  if (opts.scale) {
    // The option --scale of mmdc is the device scale factor of the page.
    options.viewport = { ...options.viewport, deviceScaleFactor: Number(opts.scale) };
  }
  const rendered = renderMermaid(browser, text, 'svg', options).then(
    (result) => ({ result }),
    (error) => ({ error })
  );
  const outcome = await Promise.race([rendered, limit]);
  clearTimeout(timer);
  if (outcome.timedOut) {
    // A page that is busy in a loop does not close by itself.
    if (state.page) {
      await Promise.race([state.page.close().catch(() => {}), new Promise((resolve) => setTimeout(resolve, 5000))]);
    }
    return { error: `oracle: no result after ${timeoutMs} ms` };
  }
  if (outcome.error) {
    const message = String((outcome.error && outcome.error.message) || outcome.error);
    return { error: message.split('\n')[0] };
  }
  return { ...outcome.result, xml: Buffer.from(outcome.result.data).toString('utf8') };
}

async function cmdRender(browser, opts) {
  const files = await readList(opts);
  const timeoutMs = Number(opts.timeout || 60000);
  const contract = await referenceContract();
  const browserVersion = await browser.version();
  const state = {};
  instrument(browser, state, opts);
  let ok = 0;
  let failed = 0;
  for (const file of files) {
    const text = await fs.readFile(file, 'utf8');
    const started = Date.now();
    const result = await renderOne(browser, state, text, opts, timeoutMs, true);
    const svgFile = outPath(file, opts, '.svg');
    await fs.mkdir(path.dirname(svgFile), { recursive: true });
    if (!result.error && state.plain === undefined) {
      throw new Error(`oracle: mmdc did not serialize the svg element for ${file}`);
    }
    const meta = {
      source: file, mermaid: contract.mermaid, mmdc: contract.mmdc, page: 'mermaid-cli',
      contract, platform: process.platform, browserVersion, sourceSha256: sha256(text),
    };
    if (result.error) {
      failed++;
      meta.error = result.error;
      await fs.rm(svgFile, { force: true });
    } else {
      ok++;
      const info = state.info;
      meta.diagramType = info.diagramType;
      meta.renderMs = Date.now() - started;
      // The draws of Math.random in the page, with those before the render.
      meta.randomTotal = info.randomCalls;
      meta.now = info.now;
      meta.window = `${info.innerWidth}x${info.innerHeight}@${info.devicePixelRatio}`;
      meta.timeZone = info.timeZone;
      meta.language = info.language;
      meta.links = info.links.map(linkName);
      meta.fontFaces = `${info.fontFacesLoaded}/${info.fontFaces}`;
      meta.title = result.title;
      meta.desc = result.desc;
      meta.sha256 = sha256(state.plain);
      // The file of mmdc with its default options: with the fonts.
      meta.embedSha256 = sha256(result.xml);
      meta.embedBytes = Buffer.byteLength(result.xml);
      meta.embedStyles = state.embedStyles.map(hashFonts);
      if (state.embedStyles.length === 0 && result.xml !== state.plain) {
        throw new Error(`oracle: the file with fonts differs and has no added style element for ${file}`);
      }
      const plain = state.plain;
      if (opts['check-embed']) {
        const again = await renderOne(browser, state, text, opts, timeoutMs, false);
        meta.checkEmbed = again.xml === plain;
        if (!meta.checkEmbed) {
          throw new Error(`oracle: fontEmbed: false gives another file for ${file}`);
        }
      }
      await fs.writeFile(svgFile, plain);
    }
    await fs.writeFile(outPath(file, opts, '.json'), JSON.stringify(meta, null, 1) + '\n');
    if (!opts.quiet) {
      process.stderr.write(`${meta.error ? 'ERR ' : 'ok  '} ${file}\n`);
    }
  }
  process.stderr.write(`render: ${ok} ok, ${failed} failed\n`);
}

async function main() {
  const [command, ...rest] = process.argv.slice(2);
  const opts = parseArgs(rest);
  if (command !== 'render' || !opts.out) {
    process.stderr.write('usage: mmdc-oracle.mjs render --out DIR [--base DIR] [--list FILE] [--timeout MS] [--id ID] [--scale N] [--check-embed] [--race-links] files...\n');
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
    const args = ['--lang=en-US'];
    if (process.platform === 'linux') {
      // The container runs as root, where Chromium has no sandbox.
      args.push('--no-sandbox');
    }
    browser = await puppeteer.launch({
      ...browserExecutable(),
      args,
      env: { ...process.env, TZ: 'UTC', LANG: 'en_US.UTF-8', LC_ALL: 'en_US.UTF-8' },
    });
    // mmdc prints the console of the page. A missing license file of a font
    // package is a normal 404 there.
    const warn = console.warn;
    console.warn = () => {};
    try {
      await cmdRender(browser, opts);
    } finally {
      console.warn = warn;
    }
  } finally {
    if (browser) {
      await Promise.race([browser.close().catch(() => {}), new Promise((resolve) => setTimeout(resolve, 10000))]);
    }
    if (!lockHeld && (await lockOwner()) === process.pid) {
      await fs.rm(LOCK_DIR, { recursive: true, force: true });
    }
  }
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  await main();
  process.exit(0);
}
