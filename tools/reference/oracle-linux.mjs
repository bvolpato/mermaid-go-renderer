#!/usr/bin/env node
// Runs oracle.mjs in the Playwright Linux image.
//
// The result is what Chromium on Linux produces with the fonts of that image
// (Liberation, Noto Color Emoji, WenQuanYi and others). The arguments are
// the arguments of oracle.mjs. Paths must be inside the repository, which is
// mounted at the same absolute path in the container. The working directory
// in the container is the working directory of this script when that is
// inside the repository, so relative paths mean the same on both sides.
//
// The option `--fonts liberation`, as the first two arguments, or the
// environment variable MMDG_LINUX_FONTS=liberation, selects the reference
// environment of the embedded font library: the container deletes every font
// file that is not a Liberation font and rebuilds the fontconfig cache
// before oracle.mjs starts. The default, `--fonts image`, keeps the fonts of
// the image.
//
// This script takes the oracle lock on the host, so only one browser runs at
// a time, as with oracle.mjs.
import { spawn } from 'node:child_process';
import fs from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '../..');
const LOCK_DIR = path.join(HERE, '.oracle.lock');

async function playwrightVersion() {
  const pkg = JSON.parse(
    await fs.readFile(path.join(HERE, 'node_modules/playwright/package.json'), 'utf8')
  );
  return pkg.version;
}

const PID_FILE = path.join(LOCK_DIR, 'pid');
const sleep = (ms) => new Promise((resolve) => setTimeout(resolve, ms));

async function lockOwner() {
  try {
    return Number(await fs.readFile(PID_FILE, 'utf8'));
  } catch {
    return 0;
  }
}

// acquireLock takes the lock that oracle.mjs uses: a directory with the
// process id of the holder. The directory and the file are two steps, so a
// waiter can see the directory of a new holder without the file. This
// script then looks again before it calls the lock stale, and it checks
// after a moment that the lock it took is still its own.
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
          await sleep(300);
          try {
            await fs.readFile(PID_FILE, 'utf8');
          } catch (again) {
            stale = again.code === 'ENOENT';
          }
        }
      }
      if (stale) {
        await fs.rm(LOCK_DIR, { recursive: true, force: true });
        continue;
      }
      await sleep(250);
    }
  }
}

// releaseLock removes the lock when this process still holds it.
async function releaseLock() {
  if ((await lockOwner()) === process.pid) {
    await fs.rm(LOCK_DIR, { recursive: true, force: true });
  }
}

// LIBERATION_ONLY is the shell prelude that leaves only the Liberation fonts
// in the container. fontconfig reads /usr/share/fonts and
// /usr/local/share/fonts in this image.
const LIBERATION_ONLY =
  'find /usr/share/fonts /usr/local/share/fonts -type f ! -path "*/truetype/liberation/*" -delete && ' +
  'fc-cache -f >/dev/null 2>&1 && ';

// workDir is the working directory in the container: the current directory
// when it is inside the repository, and the directory of this script
// otherwise.
function workDir() {
  const cwd = process.cwd();
  const rel = path.relative(ROOT, cwd);
  return rel.startsWith('..') || path.isAbsolute(rel) ? HERE : cwd;
}

function run(image, fonts, args) {
  const prelude = fonts === 'liberation' ? LIBERATION_ONLY : '';
  return new Promise((resolve) => {
    const child = spawn(
      'docker',
      [
        'run', '--rm', '--ipc=host',
        '-v', `${ROOT}:${ROOT}`,
        '-w', workDir(),
        '-e', 'MMDG_ORACLE_LOCK_HELD=1',
        '-e', 'TZ=UTC',
        image,
        'sh', '-c', `${prelude}exec node "$0" "$@"`, path.join(HERE, 'oracle.mjs'), ...args,
      ],
      { stdio: 'inherit' }
    );
    child.on('exit', (code) => resolve(code ?? 1));
  });
}

let args = process.argv.slice(2);
let fonts = process.env.MMDG_LINUX_FONTS || 'image';
if (args[0] === '--fonts') {
  fonts = args[1];
  args = args.slice(2);
}
if (fonts !== 'image' && fonts !== 'liberation') {
  process.stderr.write('oracle-linux.mjs: --fonts must be image or liberation\n');
  process.exit(2);
}

const image = `mcr.microsoft.com/playwright:v${await playwrightVersion()}-noble`;
const queued = Date.now();
await acquireLock();
const started = Date.now();
let code = 1;
try {
  code = await run(image, fonts, args);
  if (process.env.MMDG_ORACLE_TIMING) {
    const sec = (ms) => (ms / 1000).toFixed(1);
    process.stderr.write(`oracle-linux: waited ${sec(started - queued)}s for the lock, ran ${sec(Date.now() - started)}s\n`);
  }
} finally {
  await releaseLock();
}
process.exit(code);
