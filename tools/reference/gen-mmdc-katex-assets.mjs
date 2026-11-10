// Copies the CSS, license and WOFF2 fonts that mermaid-cli 12.2.0 loads
// from KaTeX 0.18.10.
// Resolve from mermaid-cli so that this script copies the page assets.
//
// Usage (in tools/reference): node gen-mmdc-katex-assets.mjs
// Then run gen-webfonts-sfnt.py, gen-mmdc-sheets.mjs and gen-mmdc-embed.mjs
// to update the font data and browser records.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { katexDirectory as source, katexVersion as version } from './mmdc-assets.mjs';

const here = path.dirname(fileURLToPath(import.meta.url));
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
