// Writes internal/mmdc/cssom.json.gz: the rules of the style sheets of the
// page of mermaid-cli, as Chromium gives them to the font embedder of
// mermaid-cli (src/browser/fontEmbedder.js).
//
// The embedder copies the cssText of the rules of a style sheet into the
// SVG. cssText is the form of the CSS object model of Chromium, not the
// text of the file: Chromium writes each rule again in its own form. The
// style sheets are fixed files (internal/webfonts), so their rules are
// recorded here one time, and the Go port of the embedder reads the record.
//
// For each style sheet of internal/webfonts.Sheets, in that order:
//
//   path    the path of the style sheet in internal/webfonts, without "files/"
//   sha256  the SHA-256 of the source style sheet
//   rules   for every rule of link.sheet.cssRules:
//             kind      "font-face", "style" or "other"
//             text      rule.cssText
//             family    rule.style.fontFamily of a @font-face rule
//             src       rule.style.src of a @font-face rule
//             selector  rule.selectorText of a style rule
//
// The script runs probes/mmdc-sheet-rules.js with the eval command of the
// browser oracle, one time.
//
// Usage (in tools/reference): node gen-mmdc-sheets.mjs
import { execFileSync } from 'node:child_process';
import crypto from 'node:crypto';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');

const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdc-sheets-'));
try {
  const outFile = path.join(dir, 'sheets.json');
  execFileSync(process.execPath, ['oracle.mjs', 'eval', 'probes/mmdc-sheet-rules.js', '--bare', '--webfonts', '--out', outFile], {
    cwd: here,
    stdio: ['ignore', 'inherit', 'inherit'],
  });
  const probe = JSON.parse(fs.readFileSync(outFile, 'utf8'));
  const sheets = probe.sheets.map((sheet) => {
    const sheetPath = new URL(sheet.href).pathname.replace(/^\//, '');
    const source = fs.readFileSync(path.join(repoRoot, 'internal/webfonts/files', sheetPath));
    return {
      path: sheetPath,
      sha256: crypto.createHash('sha256').update(source).digest('hex'),
      rules: sheet.rules,
    };
  });
  const file = path.join(repoRoot, 'internal/mmdc/cssom.json.gz');
  fs.writeFileSync(file, zlib.gzipSync(JSON.stringify({ sheets }), { level: 9 }));
  const rules = sheets.reduce((n, s) => n + s.rules.length, 0);
  process.stderr.write(`wrote ${path.relative(repoRoot, file)}: ${sheets.length} style sheets, ${rules} rules, ${fs.statSync(file).size} bytes\n`);
} finally {
  fs.rmSync(dir, { recursive: true, force: true });
}
