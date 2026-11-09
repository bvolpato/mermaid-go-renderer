#!/usr/bin/env node
// Writes the probe SVGs for the text rules of Chromium on Linux that the PNG
// rasterizer ports (internal/raster). Each probe isolates one rule: where a
// baseline lands, when text keeps LCD masks, which outline a rotated glyph
// has, and how very large text is drawn.
//
// Usage, from the repository root:
//
//   node tools/reference/gen-raster-linux-probes.mjs [DIR]
//
// DIR defaults to .parity/raster/linux/probes. The screenshots are made with:
//
//   node tools/reference/oracle-linux.mjs --fonts liberation shoot \
//       DIR/*.svg --quiet
//   node tools/reference/oracle-linux.mjs --fonts liberation shoot \
//       --scale 2 --out DIR/s2 DIR/*.svg --quiet
//
// The paths must be absolute or relative to the directory of the script
// call, and --quiet goes last: oracle.mjs reads a path after it as its
// value.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '../..');
const OUT = process.argv[2] ? path.resolve(process.argv[2]) : path.join(ROOT, '.parity/raster/linux/probes');

const FONT = '"trebuchet ms",verdana,arial,sans-serif';

function svg(w, h, body, css = '') {
  return (
    `<svg xmlns="http://www.w3.org/2000/svg" width="${w}" height="${h}" viewBox="0 0 ${w} ${h}">` +
    `<style>text{font-family:${FONT};font-size:16px;}${css}</style>${body}</svg>\n`
  );
}

const files = {};

// ---- baselines of SVG text ----
//
// A row has 16 letters whose y grows by 1/16 px. The rows are the baseline
// keywords in four forms: the text directly in the text element, in a tspan,
// in a tspan below a dy as the timeline diagram writes it, and with an
// alignment-baseline as the sequence diagram writes it.
{
  const types = ['auto', 'middle', 'central', 'hanging', 'text-before-edge', 'text-after-edge', 'mathematical', 'ideographic'];
  const forms = ['direct', 'tspan', 'dy', 'align'];
  for (const size of [16, 13]) {
    let body = '';
    let y = 24;
    for (const type of types) {
      for (const form of forms) {
        for (let k = 0; k < 16; k++) {
          const x = 8 + k * 15;
          const yy = y + k / 16;
          const db = type === 'auto' ? '' : ` dominant-baseline="${type}"`;
          const fs = size === 16 ? '' : ` style="font-size: ${size}px;"`;
          switch (form) {
            case 'direct':
              body += `<text x="${x}" y="${yy}"${db}${fs}>E</text>`;
              break;
            case 'tspan':
              body += `<text x="${x}" y="${yy}"${db}${fs}><tspan x="${x}" dy="0">E</tspan></text>`;
              break;
            case 'dy':
              body += `<g transform="translate(${x}, ${yy - 2 * size})"><text dy="1em"${db}${fs}><tspan x="0" dy="1em">E</tspan></text></g>`;
              break;
            case 'align':
              body += `<text x="${x}" y="${yy}"${db}${type === 'auto' ? '' : ` alignment-baseline="${type}"`}${fs}><tspan x="${x}" dy="0">E</tspan></text>`;
              break;
          }
        }
        y += 34;
      }
    }
    files[`baseline-svg-${size}.svg`] = svg(256, y, body);
  }
}

// ---- baselines of HTML lines ----
//
// Labels as mermaid writes them, at y positions that grow by 1/8 px, with
// the font sizes and line heights that give whole and fractional lines.
{
  let body = '';
  let y = 4;
  const label = (x, yy, w, h, style, html) =>
    `<g transform="translate(${x}, ${yy})"><foreignObject width="${w}" height="${h}">` +
    `<div xmlns="http://www.w3.org/1999/xhtml" style="display: table-cell; white-space: nowrap; line-height: 1.5; text-align: center;${style}">` +
    `${html}</div></foreignObject></g>`;
  const rows = [
    ['', '<span class="nodeLabel"><p>Eg</p></span>', 24],
    ['font-size: 14px;', '<span class="nodeLabel"><p>Eg</p></span>', 21],
    ['font-size: 17px;', '<span class="nodeLabel"><p>Eg</p></span>', 26],
    ['font-size: 12px; line-height: 1.3;', '<span><p>Eg</p></span>', 16],
    ['', '<span style="background-color: #eee;"><p>Eg<br/>Eg</p></span>', 48],
    ['font-size: 17px;', '<p>Eg<br/>Eg<br/>Eg</p>', 78],
    ['', '<p><b>Eg</b> <i>Eg</i> <span style="background-color:#fd8;">Eg</span></p>', 24],
    ['padding-top: 2.3px;', '<p>Eg</p>', 27],
  ];
  for (const [style, html, h] of rows) {
    for (let k = 0; k < 8; k++) {
      body += label(6 + k * 62, y + k / 8, 58, h + 2, style, html);
    }
    y += h + 12;
  }
  const css = `p{margin:0;}div{font-family:${FONT};font-size:16px;color:#333;}`;
  files['baseline-html.svg'] = svg(504, y, body, css);
}

// ---- LCD masks ----
function lcdBody(extra) {
  let b = '';
  b += `<rect x="150" y="0" width="150" height="150" fill="#ececff"/>`;
  b += `<text x="6" y="16">Plain text Eagle</text><text x="156" y="16">Plain text Eagle</text>`;
  // A group with an opacity, with a line and a text, as the axis of a gantt
  // diagram.
  b += `<g opacity="0.8"><line x1="6" y1="22" x2="140" y2="22" stroke="#000"/><text x="6" y="38">Group opacity 0.8</text></g>`;
  b += `<g opacity="0.8"><line x1="156" y1="22" x2="290" y2="22" stroke="#000"/><text x="156" y="38">Group opacity 0.8</text></g>`;
  // A group with only a text.
  b += `<g opacity="0.5"><text x="6" y="58">Only text 0.5</text></g><g opacity="0.5"><text x="156" y="58">Only text 0.5</text></g>`;
  // Nested groups.
  b += `<g opacity="0.8"><g opacity="0.7"><text x="6" y="78">Nested opacity</text><rect x="120" y="66" width="12" height="12" fill="#06c"/></g></g>`;
  b += `<g opacity="0.8"><g opacity="0.7"><text x="156" y="78">Nested opacity</text><rect x="270" y="66" width="12" height="12" fill="#06c"/></g></g>`;
  // The opacity on the text, and a fill opacity.
  b += `<text x="6" y="98" opacity="0.6">Text opacity</text><text x="156" y="98" fill-opacity="0.6">Fill opacity</text>`;
  // CSS opacity.
  b += `<g style="opacity: 0.8"><text x="6" y="118">CSS opacity</text><text x="156" y="118">CSS opacity</text></g>`;
  // An HTML label with an opacity.
  b += `<g transform="translate(6, 124)"><foreignObject width="140" height="24"><div xmlns="http://www.w3.org/1999/xhtml" style="opacity: 0.7; line-height: 1.5;">Html opacity</div></foreignObject></g>`;
  b += `<g transform="translate(156, 124)" opacity="0.7"><foreignObject width="140" height="24"><div xmlns="http://www.w3.org/1999/xhtml" style="line-height: 1.5;">Html in group</div></foreignObject></g>`;
  return b + extra;
}
const lcdCss = `div{font-family:${FONT};font-size:16px;color:#333;}`;
files['lcd-opacity.svg'] = svg(300, 150, lcdBody(''), lcdCss);
files['lcd-opacity-filter.svg'] = svg(
  300,
  180,
  `<defs><filter id="ds" height="130%" width="130%"><feDropShadow dx="4" dy="4" stdDeviation="0" flood-opacity="0.3" flood-color="#000000"/></filter></defs>` +
    lcdBody(`<rect x="10" y="156" width="60" height="14" fill="#9370db" filter="url(#ds)"/><text x="90" y="170">After a filter</text>`),
  lcdCss
);
files['lcd-opacity-blend.svg'] = svg(
  300,
  180,
  lcdBody(`<rect x="10" y="156" width="60" height="14" fill="#9370db" style="mix-blend-mode: multiply;"/><text x="90" y="170">After a blend</text>`),
  lcdCss
);
{
  let b = `<defs>`;
  b += `<clipPath id="c1"><rect width="100" height="20"/></clipPath>`;
  b += `<clipPath id="c2"><rect x="0.4" y="0.3" width="99.3" height="19.5"/></clipPath>`;
  b += `<clipPath id="c3"><circle cx="50" cy="10" r="30"/></clipPath>`;
  b += `<clipPath id="c4"><rect width="60" height="20"/><rect x="70" width="50" height="20"/></clipPath>`;
  b += `<clipPath id="c5"><rect x="0.05" y="0.06" width="100" height="20"/></clipPath>`;
  b += `</defs>`;
  b += `<rect x="150" y="0" width="150" height="230" fill="#ececff"/>`;
  const one = (x, y, id, where) =>
    where === 'text'
      ? `<g transform="translate(${x},${y})"><text x="2" y="15" clip-path="url(#${id})">Clipped text Eagle</text></g>`
      : `<g transform="translate(${x},${y})" clip-path="url(#${id})"><text x="2" y="15">Clipped group Eagle</text></g>`;
  let y = 4;
  for (const id of ['c1', 'c2', 'c3', 'c4', 'c5']) {
    b += one(6, y, id, 'text') + one(156, y, id, 'group');
    y += 24;
  }
  // A clip at a fractional position, and under a group opacity.
  b += one(6.5, y + 0.5, 'c1', 'text') + one(156.3, y + 0.25, 'c1', 'group');
  y += 24;
  b += `<g opacity="0.8">${one(6, y, 'c1', 'text')}${one(156, y, 'c2', 'group')}</g>`;
  y += 24;
  // The viewport of an inner svg element and a foreignObject clip too.
  b += `<svg x="6" y="${y}" width="100" height="20"><text x="2" y="15">Inner svg viewport</text></svg>`;
  b += `<svg x="156.4" y="${y + 0.3}" width="100" height="20"><text x="2" y="15">Inner svg viewport</text></svg>`;
  y += 24;
  b += `<g transform="translate(6.4, ${y + 0.3})"><foreignObject width="90.5" height="20.5"><div xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; line-height: 1.25;">Html clipped by box</div></foreignObject></g>`;
  b += `<g transform="translate(156, ${y})"><foreignObject width="90" height="20"><div xmlns="http://www.w3.org/1999/xhtml" style="white-space: nowrap; line-height: 1.25;">Html clipped by box</div></foreignObject></g>`;
  y += 26;
  files['lcd-clip.svg'] = svg(300, y, b, lcdCss);
}
{
  let b = `<defs><filter id="ds" height="130%" width="130%"><feDropShadow dx="2" dy="2" stdDeviation="0" flood-opacity="0.3" flood-color="#000000"/></filter></defs>`;
  b += `<rect x="150" y="0" width="150" height="100" fill="#ececff"/>`;
  b += `<text x="6" y="16">Outside a filter</text><text x="156" y="16">Outside a filter</text>`;
  b += `<g filter="url(#ds)"><rect x="4" y="24" width="130" height="22" fill="#fff" stroke="#9370db"/><text x="8" y="40">In a shadow group</text></g>`;
  b += `<g style="filter: brightness(120%);"><rect x="154" y="24" width="130" height="22" fill="#ccf"/><text x="158" y="40">In a brightness</text></g>`;
  b += `<g transform="translate(6.3, 50.6)" style="filter: brightness(120%);"><text x="2" y="16.21875">Fraction brightness</text></g>`;
  b += `<g transform="translate(156.3, 50.6)"><text x="2" y="16.21875">Fraction plain text</text></g>`;
  b += `<text x="6" y="92" filter="url(#ds)">Text with a shadow</text>`;
  files['lcd-filter.svg'] = svg(300, 100, b, lcdCss);
}

// ---- rotated and scaled text ----
{
  let b = '';
  const word = 'Eagle 0123';
  const angles = [0, 90, -90, 180, 270, 45, -45, 30, 10, 1, 89, -135];
  let i = 0;
  for (const a of angles) {
    const cx = 60 + (i % 4) * 120;
    const cy = 60 + Math.floor(i / 4) * 120;
    b += `<g transform="translate(${cx + 0.3}, ${cy + 0.6}) rotate(${a})"><text x="-36" y="5">${word}</text><text x="-36" y="-14" style="font-size: 11px;">${word} small</text></g>`;
    i++;
  }
  let y = 380;
  const tr = [
    'scale(-1, 1)',
    'scale(1, -1)',
    'scale(1.5, 1)',
    'scale(1, 1.5)',
    'scale(1.5)',
    'scale(0.75)',
    'skewX(20)',
    'matrix(0 1 -1 0 0 0) scale(-1, 1)',
  ];
  i = 0;
  for (const t of tr) {
    const cx = 80 + (i % 4) * 120;
    const cy = y + Math.floor(i / 4) * 70;
    b += `<g transform="translate(${cx}, ${cy}) ${t}"><text x="-30" y="5">${word}</text></g>`;
    i++;
  }
  files['rotate.svg'] = svg(480, 520, b);
}
// The label of an xychart axis and of a gitgraph commit.
{
  let b = '';
  for (let k = 0; k < 8; k++) {
    b += `<text x="0" y="0" transform="translate(${20 + k * 24.37}, ${130 + k * 0.13}) rotate(270)" style="font-size: 14px;" dominant-baseline="middle" text-anchor="start">Revenue ${k}</text>`;
    b += `<g transform="translate(${230 + k * 26.21}, ${44 + k * 0.29}) rotate(-45, 0, 0)"><text x="0" y="0" style="font-size: 10px;">${k}-a1b2c3d</text></g>`;
    b += `<g transform="translate(${230 + k * 26.21}, ${170 + k * 0.29}) rotate(45)"><text x="-30" y="0" style="font-size: 12px;" font-weight="bold">tag ${k}</text></g>`;
  }
  files['rotate-labels.svg'] = svg(460, 200, b);
}

// ---- very large text ----
{
  let b = '';
  let x = 6;
  for (const size of [100, 200, 255, 256, 257, 300]) {
    b += `<text x="${x}" y="300.4" style="font-size: ${size}px;">a</text>`;
    x += size * 0.58 + 8;
  }
  b += `<g transform="translate(6, 420) scale(3)"><text x="0" y="0" style="font-size: 90px;">g</text><text x="60" y="0" style="font-size: 85px;">g</text></g>`;
  b += `<g transform="translate(420, 520) rotate(-90)"><text x="0" y="0" style="font-size: 260px;">e</text></g>`;
  b += `<g opacity="0.8"><text x="640" y="520" style="font-size: 270px;">s</text></g>`;
  files['large.svg'] = svg(900, 560, b);
}

fs.mkdirSync(OUT, { recursive: true });
for (const [name, text] of Object.entries(files)) {
  fs.writeFileSync(path.join(OUT, name), text);
}
process.stderr.write(`wrote ${Object.keys(files).length} probes to ${OUT}\n`);
