// Copies the CSS, license and WOFF2 fonts that mermaid-cli 12.0.0 loads
// from KaTeX 0.16.47. Mermaid itself uses a different KaTeX version.
// Resolve from mermaid-cli so that this script copies the page assets.
//
// Usage (in tools/reference): node gen-mmdc-katex-assets.mjs
// After changing fonts, the font owner must also run gen-webfonts-sfnt.py.
import fs from 'node:fs';
import { createRequire } from 'node:module';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(import.meta.url);
const cliRequire = createRequire(require.resolve('@mermaid-js/mermaid-cli'));
const source = path.dirname(cliRequire.resolve('katex/package.json'));
const { version } = JSON.parse(fs.readFileSync(path.join(source, 'package.json'), 'utf8'));
if (version !== '0.16.47') {
  throw new Error(`Expected mermaid-cli's KaTeX 0.16.47, got ${version}`);
}
const target = path.resolve(here, '../../internal/webfonts/files/katex');
const css = fs.readFileSync(path.join(source, 'dist/katex.css'), 'utf8');
const fonts = [...new Set([...css.matchAll(/url\((fonts\/[^)]+\.woff2)\)/g)].map((m) => m[1]))];
fs.mkdirSync(path.join(target, 'dist/fonts'), { recursive: true });
for (const file of ['LICENSE', 'dist/katex.css', ...fonts.map((font) => `dist/${font}`)]) {
  fs.copyFileSync(path.join(source, file), path.join(target, file));
}
// The separate OFL font notice is kept in files/katex/fonts-OFL.txt.
// It is not the package LICENSE that mmdc includes in each style element.
process.stderr.write(`copied KaTeX ${version}: CSS, MIT license and ${fonts.length} WOFF2 files\n`);
