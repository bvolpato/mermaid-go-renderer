// Writes internal/font/fontconfig-default.xml.gz: the fontconfig
// configuration of the reference environment (the Playwright image) as one
// file. A Linux font library uses it when the machine has no fontconfig
// configuration of its own.
//
// The script reads fonts.conf of a configuration directory and puts the
// content of each file of conf.d in the place of the include element, in
// the order in which fontconfig reads the files. Only the alias and the
// match elements are kept: they are the rules that select fonts.
//
// Only the files of fontconfig itself are read (the package
// fontconfig-config: fonts.conf and the files in the list below). They have
// the license of fontconfig (internal/font/LICENSE-fontconfig.txt). The
// rule files that font packages add to the image (WenQuanYi Zen Hei,
// IPAGothic) have other licenses and are left out.
//
// Usage (in tools/reference): node gen-fontconfig-default.mjs [config directory]
// The default directory is ../../.parity/linux/image/etc-fonts, a copy of
// /etc/fonts of the image mcr.microsoft.com/playwright:v1.64.1-noble.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const dir = process.argv[2] || path.join(repoRoot, '.parity/linux/image/etc-fonts');

// The rule files of the package fontconfig-config 2.15.0.
const fontconfigFiles = new Set([
  '10-hinting-slight.conf', '10-scale-bitmap-fonts.conf', '10-sub-pixel-rgb.conf', '10-yes-antialias.conf',
  '11-lcdfilter-default.conf', '20-unhint-small-vera.conf', '30-metric-aliases.conf', '40-nonlatin.conf',
  '45-generic.conf', '45-latin.conf', '48-spacing.conf', '49-sansserif.conf', '50-user.conf', '51-local.conf',
  '60-generic.conf', '60-latin.conf', '65-fonts-persian.conf', '65-nonlatin.conf', '69-unifont.conf',
  '70-no-bitmaps-except-emoji.conf', '80-delicious.conf', '90-synthetic.conf',
]);

// rules returns the alias and the match elements of a file, in order. An
// include of a directory is replaced by the rules of its files.
function rules(file, depth = 0) {
  let text = fs.readFileSync(file, 'utf8').replace(/<!--[\s\S]*?-->/g, '');
  const out = [];
  const element = /<(alias|match|include)\b([^>]*)>([\s\S]*?)<\/\1>/g;
  for (const m of text.matchAll(element)) {
    if (m[1] !== 'include') {
      out.push(m[0].replace(/\s+/g, ' ').replace(/> </g, '><'));
      continue;
    }
    if (/prefix="xdg"/.test(m[2]) || depth > 4) {
      continue;
    }
    const target = path.resolve(path.dirname(file), m[3].trim());
    if (!fs.existsSync(target)) {
      continue;
    }
    if (fs.statSync(target).isDirectory()) {
      for (const name of fs.readdirSync(target).filter((n) => /^[0-9].*\.conf$/.test(n) && fontconfigFiles.has(n)).sort()) {
        out.push(...rules(path.join(target, name), depth + 1));
      }
    } else {
      out.push(...rules(target, depth + 1));
    }
  }
  return out;
}

const all = rules(path.join(dir, 'fonts.conf'));
const xml = `<?xml version="1.0"?>\n<fontconfig>\n${all.join('\n')}\n</fontconfig>\n`;
const outFile = path.join(repoRoot, 'internal/font/fontconfig-default.xml.gz');
fs.writeFileSync(outFile, zlib.gzipSync(xml, { level: 9 }));
console.error(`${outFile}: ${all.length} rules, ${xml.length} bytes of XML`);
