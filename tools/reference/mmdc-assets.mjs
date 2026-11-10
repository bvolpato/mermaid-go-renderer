// Resolves page assets from mermaid-cli, including hoisted npm dependencies.
// gen-webfonts-sfnt.py runs this file to use the same KaTeX package.
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const require = createRequire(import.meta.url);
const cliRequire = createRequire(require.resolve('@mermaid-js/mermaid-cli'));
export const katexDirectory = path.dirname(cliRequire.resolve('katex/package.json'));
export const katexVersion = JSON.parse(fs.readFileSync(path.join(katexDirectory, 'package.json'), 'utf8')).version;
if (katexVersion !== '0.18.10') {
  throw new Error(`Expected mermaid-cli's KaTeX 0.18.10, got ${katexVersion}`);
}

if (process.argv[1] && path.resolve(process.argv[1]) === fileURLToPath(import.meta.url)) {
  process.stdout.write(JSON.stringify({ directory: katexDirectory, version: katexVersion }) + '\n');
}
