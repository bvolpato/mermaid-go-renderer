// Shared part of the gen-*-render.mjs scripts: it renders hand-written
// texts with the render command of the browser oracle, one page for each
// text as in the reference renders, and writes a golden file for a Go test.
//
// The golden file is gzip-compressed JSON: a list of { name, text } with
// `svg` and `type` (the string that mermaid.render returns and the diagram
// type), or `error` when mermaid fails.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const repoRoot = path.resolve(here, '../..');

// renderExtras renders cases ([{ name, text }]) and writes outFile, a path
// from the repository root.
export function renderExtras(cases, outFile) {
  const names = new Set(cases.map((c) => c.name));
  if (names.size !== cases.length) {
    throw new Error('duplicate case names');
  }
  const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'mmdg-extra-render-'));
  const corpus = path.join(tmp, 'corpus');
  const ref = path.join(tmp, 'ref');
  fs.mkdirSync(corpus);
  const files = cases.map((c) => {
    const file = path.join(corpus, c.name + '.mmd');
    fs.writeFileSync(file, c.text);
    return file;
  });
  const list = path.join(tmp, 'list.txt');
  fs.writeFileSync(list, files.join('\n') + '\n');
  try {
    execFileSync('node', [path.join(here, 'oracle.mjs'), 'render', '--base', corpus, '--out', ref, '--quiet', '--list', list], { stdio: 'inherit' });
    const out = cases.map((c) => {
      const meta = JSON.parse(fs.readFileSync(path.join(ref, c.name + '.json'), 'utf8'));
      if (meta.error !== undefined) {
        return { name: c.name, text: c.text, error: meta.error };
      }
      return { name: c.name, text: c.text, type: meta.diagramType, svg: fs.readFileSync(path.join(ref, c.name + '.svg'), 'utf8') };
    });
    const target = path.join(repoRoot, outFile);
    fs.mkdirSync(path.dirname(target), { recursive: true });
    fs.writeFileSync(target, zlib.gzipSync(JSON.stringify(out), { level: 9 }));
    console.error(`${outFile}: ${out.length} cases, ${out.filter((c) => c.error !== undefined).length} that mermaid rejects, ${fs.statSync(target).size} bytes`);
  } finally {
    fs.rmSync(tmp, { recursive: true, force: true });
  }
}

// withConfig puts a config in front of a text as front matter. yaml is the
// text of the config key, without indentation.
export const withConfig = (yaml, body) =>
  '---\nconfig:\n' +
  yaml
    .split('\n')
    .map((l) => '  ' + l)
    .join('\n') +
  '\n---\n' +
  body;
