#!/usr/bin/env node
// Runs mermaid-cli (mmdc) as the oracle for cmd/mmdg.
//
// mmdc starts a browser with puppeteer. This script points it at the
// headless shell of the Chromium that Playwright has installed, the binary
// that oracle.mjs launches, so that no other browser is downloaded and no
// window opens. It holds the lock of the browser oracle while mmdc runs, so
// that only one browser runs at a time, and it runs the jobs one after the
// other.
//
//   node mmdc-run.mjs jobs.json results.json
//
//   jobs.json:    [{ args: [...], cwd?, stdin?, timeout? }]   at most 150 jobs
//   results.json: [{ code, signal, stdout, stderr, ms }]
//
// The args are the arguments of mmdc. The script adds -p with its puppeteer
// config when a job has none. A relative cwd is relative to this directory.
import { spawn } from 'node:child_process';
import { chromium } from 'playwright';
import fs from 'node:fs/promises';
import fsSync from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const MMDC = path.join(HERE, 'node_modules/@mermaid-js/mermaid-cli/src/cli.js');
const MAX_JOBS = 150;

// headlessShellPath is the chrome-headless-shell of the Chromium revision
// of Playwright. chromium.executablePath() is the full browser of the same
// revision, which is the fallback.
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

function runJob(job, puppeteerConfig) {
  const args = [...job.args];
  if (!args.includes('-p') && !args.includes('--puppeteerConfigFile')) {
    args.push('-p', puppeteerConfig);
  }
  const started = Date.now();
  return new Promise((resolve) => {
    const child = spawn(process.execPath, [MMDC, ...args], {
      cwd: job.cwd ? path.resolve(HERE, job.cwd) : HERE,
      // No color codes in the messages, and no browser download.
      env: { ...process.env, NO_COLOR: '1', FORCE_COLOR: '0', PUPPETEER_SKIP_DOWNLOAD: '1', ...(job.env || {}) },
      stdio: ['pipe', 'pipe', 'pipe'],
    });
    const out = [];
    const err = [];
    child.stdout.on('data', (d) => out.push(d));
    child.stderr.on('data', (d) => err.push(d));
    // A job that does not end must not hold the lock. SIGKILL of mmdc can
    // leave its browser: kill the whole group is not possible without a
    // new session, so ask politely first.
    const timer = setTimeout(() => {
      child.kill('SIGTERM');
      setTimeout(() => child.kill('SIGKILL'), 5000).unref();
    }, job.timeout || 180000);
    child.on('error', (e) => {
      clearTimeout(timer);
      resolve({ code: null, signal: null, stdout: '', stderr: String(e), ms: Date.now() - started });
    });
    child.on('close', (code, signal) => {
      clearTimeout(timer);
      resolve({
        code,
        signal,
        stdout: Buffer.concat(out).toString('utf8'),
        stderr: Buffer.concat(err).toString('utf8'),
        ms: Date.now() - started,
      });
    });
    child.stdin.on('error', () => {});
    child.stdin.end(job.stdin === undefined ? '' : job.stdin);
  });
}

async function main() {
  const [jobsFile, resultsFile] = process.argv.slice(2);
  if (!jobsFile || !resultsFile) {
    process.stderr.write('usage: mmdc-run.mjs jobs.json results.json\n');
    process.exit(2);
  }
  const jobs = JSON.parse(await fs.readFile(jobsFile, 'utf8'));
  if (jobs.length > MAX_JOBS) {
    process.stderr.write(`mmdc-run: ${jobs.length} jobs, at most ${MAX_JOBS} in one run\n`);
    process.exit(2);
  }
  const dir = await fs.mkdtemp(path.join(os.tmpdir(), 'mmdc-run-'));
  const puppeteerConfig = path.join(dir, 'puppeteer.json');
  await fs.writeFile(puppeteerConfig, JSON.stringify(browserExecutable()));
  const results = [];
  await acquireLock();
  try {
    for (const job of jobs) {
      results.push(await runJob(job, puppeteerConfig));
    }
  } finally {
    await fs.rm(LOCK_DIR, { recursive: true, force: true });
    await fs.rm(dir, { recursive: true, force: true });
  }
  await fs.writeFile(resultsFile, JSON.stringify(results, null, 1) + '\n');
}

await main();
