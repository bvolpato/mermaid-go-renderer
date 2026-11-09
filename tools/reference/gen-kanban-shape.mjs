// Makes the cases of the node shape kanbanItem and records them from
// upstream mermaid in Chromium:
// internal/diagrams/kanban/testdata/kanbanItem.json.gz, written by
// gen-shapes.mjs (probes/shapes.js).
//
//   node gen-kanban-shape.mjs
//
// The base node is what kanbanDb.getData() makes for an item, with the
// properties that kanbanRenderer.ts adds before it calls insertNode.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.join(here, '../../internal/diagrams/kanban/testdata/kanbanItem.json.gz');

const baseNode = {
  id: 'docs',
  parentId: 'todo',
  label: 'Create Documentation',
  labelType: 'markdown',
  isGroup: false,
  shape: 'kanbanItem',
  level: 4,
  rx: 5,
  ry: 5,
  cssStyles: ['text-align: left'],
  domId: 'my-svg-docs',
  x: 200,
  width: 185,
};

const longLabel = 'Create renderer so that it works in all cases. We also add some extra text here for testing purposes.';
const cases = [];
const add = (name, directive, node = {}) => {
  const merged = { ...baseNode, ...node };
  for (const key of Object.keys(merged)) {
    if (merged[key] === undefined) {
      delete merged[key];
    }
  }
  cases.push({ name: `kanbanItem/${name}`, type: 'kanban', directive, node: merged });
};

const classic = { look: 'classic', theme: 'default' };
const svgLabels = { ...classic, htmlLabels: false };
const meta = { ticket: 'MC-2037', assigned: 'knsv', priority: 'High' };
const url = { kanban: { ticketBaseUrl: 'https://example.com/browse/#TICKET#' } };

// The node as the kanban diagram makes it, with the config of the diagram.
add('no-directive', {});
add('no-directive-meta', {}, meta);
for (const theme of ['default', 'redux-color', 'dark', 'neo', 'redux-dark-color', 'forest']) {
  add(`theme-${theme}`, { theme }, meta);
}
// The look of the node. The kanban db sets no look on an item.
for (const look of ['classic', 'neo', 'handDrawn']) {
  for (const theme of ['default', 'redux-color']) {
    add(`look-${look}-${theme}`, { look, theme }, { look });
    add(`look-${look}-${theme}-meta`, { look, theme }, { look, ...meta });
  }
}
// The hand-drawn look: a path with a radius, and a rectangle without.
const handDrawn = { look: 'handDrawn', theme: 'default' };
add('handdrawn-no-radius', handDrawn, { look: 'handDrawn', rx: 0, ry: 0 });
add('handdrawn-rx-undefined', handDrawn, { look: 'handDrawn', rx: undefined, ry: undefined });
add('handdrawn-ry-only', handDrawn, { look: 'handDrawn', rx: 0, ry: 7 });
add('handdrawn-rx-only', handDrawn, { look: 'handDrawn', rx: 9, ry: undefined });
add('handdrawn-css-undefined', handDrawn, { look: 'handDrawn', cssStyles: undefined });
add('handdrawn-css-empty', handDrawn, { look: 'handDrawn', cssStyles: [] });
add('handdrawn-css-styles', handDrawn, { look: 'handDrawn', cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px', 'stroke-dasharray: 5 5', 'color:red'] });
add('handdrawn-seed', { ...handDrawn, handDrawnSeed: 7 }, { look: 'handDrawn', ...meta });
add('handdrawn-priority', handDrawn, { look: 'handDrawn', priority: 'Very High' });
add('handdrawn-svg-label', { ...handDrawn, htmlLabels: false }, { look: 'handDrawn', ...meta });
add('handdrawn-no-width', handDrawn, { look: 'handDrawn', width: undefined });
// The ticket, the assigned person and the priority.
add('ticket', classic, { ticket: 'MC-2037' });
add('assigned', classic, { assigned: 'knsv' });
add('ticket-assigned', classic, { ticket: 'MC-2037', assigned: 'K.Sveidqvist' });
add('ticket-empty', classic, { ticket: '', assigned: '' });
add('ticket-long', classic, { ticket: 'A very long ticket id that does not fit in the width of an item at all', assigned: 'knsv' });
add('assigned-long', classic, { ticket: 'MC-1', assigned: 'A very long name of a person that does not fit in the width of an item' });
add('ticket-markdown', classic, { ticket: '**MC**-*2037*', assigned: '`knsv` <b>bold</b>' });
add('ticket-entities', classic, { ticket: 'a &amp; b #35; <i>c</i>', assigned: 'é ü 日本語 😀' });
add('ticket-number', classic, { ticket: 2037 });
add('assigned-number', classic, { assigned: 7 });
add('ticket-svg', svgLabels, { ticket: 'MC-2037' });
add('assigned-svg', svgLabels, { assigned: 'knsv' });
add('meta-svg', svgLabels, meta);
add('ticket-long-svg', svgLabels, { ticket: 'A very long ticket id that does not fit in the width of an item at all', assigned: 'A long name of a person' });
for (const priority of ['Very High', 'High', 'Medium', 'Low', 'Very Low', 'Unknown', 'very high', '', 3, true, ['High']]) {
  add(`priority-${JSON.stringify(priority)}`, classic, { priority });
}
add('priority-rx-undefined', classic, { priority: 'Low', rx: undefined, ry: undefined });
add('priority-rx-9', classic, { priority: 'Low', rx: 9, ry: 3 });
add('priority-rx-0', classic, { priority: 'Low', rx: 0, ry: 0 });
add('priority-rx-2.5', classic, { priority: 'Very Low', rx: 2.5, ry: 2.5 });
add('priority-svg', svgLabels, { priority: 'High', assigned: 'knsv' });
// The link of the ticket.
add('url', { ...classic, ...url }, { ticket: 'MC-2037' });
add('url-meta', { ...classic, ...url }, meta);
add('url-no-ticket', { ...classic, ...url }, { assigned: 'knsv' });
add('url-no-placeholder', { ...classic, kanban: { ticketBaseUrl: 'https://example.com/browse/' } }, { ticket: 'MC-2037' });
add('url-two-placeholders', { ...classic, kanban: { ticketBaseUrl: 'https://example.com/#TICKET#/#TICKET#' } }, { ticket: 'MC-2037' });
add('url-patterns', { ...classic, ...url }, { ticket: "a$&b$$c$`d$'e$1" });
add('url-svg', { ...svgLabels, ...url }, meta);
add('url-handdrawn', { ...handDrawn, ...url }, { look: 'handDrawn', ...meta });
add('url-number', { ...classic, kanban: { ticketBaseUrl: 5 } }, { ticket: 'MC-2037' });
add('url-script', { ...classic, kanban: { ticketBaseUrl: 'javascript:alert(1)//#TICKET#' } }, { ticket: 'MC-2037' });
// Labels.
for (const [tag, directive] of [['html', classic], ['svg', svgLabels]]) {
  add(`long-${tag}`, directive, { label: longLabel, ...meta });
  add(`empty-${tag}`, directive, { label: '' });
  add(`empty-meta-${tag}`, directive, { label: '', ...meta });
  add(`br-${tag}`, directive, { label: 'First line<br/>Second line<br>third' });
  add(`markdown-${tag}`, directive, { label: 'Some **bold** and *italic* text\nwith a second line', ...meta });
  add(`text-label-${tag}`, directive, { label: 'Some **bold** and *italic* text', labelType: 'text' });
}
add('entities', classic, { label: 'a &amp; b #quot;c#quot; <b>d</b> é ü' });
add('no-label', classic, { label: undefined });
add('long-word', classic, { label: 'Supercalifragilisticexpialidocious_and_more_characters' });
add('fa-icon-label', classic, { label: 'fa:fa-bell Ring' });
// The icon and the image make a background behind the title.
add('icon', classic, { icon: 'fa fa-book', ...meta });
add('icon-svg', svgLabels, { icon: 'fa fa-book', ...meta });
add('img', classic, { img: 'https://example.com/a.png', ...meta });
add('img-svg', svgLabels, { img: 'https://example.com/a.png', ...meta });
// Sizes.
add('width-undefined', classic, { width: undefined, ...meta });
add('width-null', classic, { width: null });
add('width-0', classic, { width: 0 });
add('width-300', classic, { width: 300, ...meta });
add('width-40', classic, { width: 40, ...meta });
add('height-100', classic, { height: 100, ...meta });
add('height-10', classic, { height: 10 });
add('wrapping-width', classic, { width: undefined, wrappingWidth: 120, label: longLabel });
for (const padding of [0, 20, undefined]) {
  add(`padding-${padding}`, classic, { padding, ...meta });
}
add('padding-svg', svgLabels, { padding: 30, ...meta });
// Styles and classes.
add('css-undefined', classic, { cssStyles: undefined, ...meta });
add('css-styles', classic, { cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'], ...meta });
add('css-label-styles', classic, { cssStyles: ['color:red', 'font-weight:bold', 'font-size:20px', 'fill:#ff0', 'text-align: right'], ...meta });
add('css-label-styles-svg', svgLabels, { cssStyles: ['color:red', 'font-weight:bold', 'font-size:20px', 'fill:#ff0'], ...meta });
add('css-compiled', classic, { cssCompiledStyles: ['fill:#f00', 'color:white', 'stroke-width:2px', 'stroke:yellow'], cssClasses: 'cls urgent', ...meta });
add('css-classes', classic, { cssClasses: 'urgent' });
add('label-style', classic, { labelStyle: 'color: blue', ...meta });
// The useHtmlLabels property of the node.
add('node-html-in-svg', svgLabels, { useHtmlLabels: true, ...meta });
add('node-svg-in-html', classic, { useHtmlLabels: false, ...meta });
add('flowchart-html-false', { ...classic, flowchart: { htmlLabels: false } }, meta);
// What insertNode adds.
add('link', classic, { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'A tip', ...meta });
add('tooltip-callback', classic, { tooltip: 'A tip', haveCallback: true });
add('no-dom-id', classic, { domId: undefined });
// Fonts and security.
add('font-courier', { ...classic, fontFamily: 'courier' }, meta);
add('font-size', { ...classic, themeVariables: { fontSize: '20px' } }, meta);
add('font-size-svg', { ...svgLabels, themeVariables: { fontSize: '20px' } }, meta);
add('loose', { ...classic, securityLevel: 'loose' }, { label: 'a <b>b</b>', ticket: '<i>t</i>', assigned: '<u>k</u>' });
add('section-width', { ...classic, kanban: { sectionWidth: 300 } }, { width: 285, ...meta });

const names = new Set(cases.map((c) => c.name));
if (names.size !== cases.length) {
  throw new Error('duplicate case names');
}
const tmp = path.join(os.tmpdir(), `mmdg-kanban-shape-cases-${process.pid}.json`);
fs.writeFileSync(tmp, JSON.stringify({ cases }));
try {
  execFileSync('node', [path.join(here, 'gen-shapes.mjs'), tmp, outFile], { stdio: 'inherit' });
} finally {
  fs.rmSync(tmp);
}
