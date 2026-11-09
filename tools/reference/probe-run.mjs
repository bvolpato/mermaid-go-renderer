// Runs the cases of a DOM probe in the browser oracle and writes the two
// golden files of the probe: <name>-cases.json and <name>-results.json.
//
// The oracle can wait a long time for its lock, and the Go tests read the
// two files as a pair. So the cases go to the oracle through a temporary
// file, and the two golden files are written only after the oracle
// returned, one right after the other.
//
// MMDG_PROBE_OUT names another directory for the golden files. Use it to
// try new cases before the engine passes them: the tests of the engine read
// internal/browser/testdata.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

// runProbe runs `cases` with the probe script (the default is the generic
// DOM probe) and writes the golden files. It returns the results.
export function runProbe(name, cases, probe = path.join(repoRoot, '.parity/probe/dom.js')) {
  const outDir = process.env.MMDG_PROBE_OUT ? path.resolve(process.env.MMDG_PROBE_OUT) : path.join(repoRoot, 'internal/browser/testdata');
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-probe-'));
  const casesText = JSON.stringify({ cases }, null, 1) + '\n';
  const tmpCases = path.join(tmp, 'cases.json');
  const tmpResults = path.join(tmp, 'results.json');
  fs.writeFileSync(tmpCases, casesText);
  try {
    execFileSync(process.execPath, [path.join(here, 'oracle.mjs'), 'eval', probe, '--args', tmpCases, '--out', tmpResults], { stdio: 'inherit' });
    const resultsText = fs.readFileSync(tmpResults, 'utf8');
    fs.mkdirSync(outDir, { recursive: true });
    fs.writeFileSync(path.join(outDir, `${name}-cases.json`), casesText);
    fs.writeFileSync(path.join(outDir, `${name}-results.json`), resultsText);
    console.error(`${cases.length} cases in ${outDir}`);
    return JSON.parse(resultsText);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}
