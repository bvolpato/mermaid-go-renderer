// Writes internal/dom/testdata/style_sync.json: where Chromium puts the style
// attribute of an element whose inline style was written through the CSSOM
// (element.style.setProperty, as d3 selection.style does).
//
// Blink keeps such a style in the element and adds the style attribute to the
// attribute list only when something reads the attributes: serialization,
// getAttribute('style'), a [style] selector, cloneNode. The attribute then
// goes to the end of the list. Each case is a list of operations on one
// element, and the result is its outerHTML after the last operation.
//
// Usage (in tools/reference): node gen-dom-style-sync.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/dom/testdata/style_sync.json');

function prng(seed) {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

// An operation is [name, ...arguments]:
//   attr name value   el.setAttribute(name, value)
//   remove name       el.removeAttribute(name)
//   get name          el.getAttribute(name)
//   has name          el.hasAttribute(name)
//   style prop value  el.style.setProperty(prop, value)
//   unstyle prop      el.style.removeProperty(prop)
//   outer             el.outerHTML
//   inner             parent.innerHTML
//   query selector    parent.querySelector(selector)
//   matches selector  el.matches(selector)
//   clone             el.cloneNode(true)
//   bbox              el.getBBox()
//   computed prop     getComputedStyle(el).getPropertyValue(prop)
//   text value        el.textContent = value
//   child tag         el.appendChild(new element)
const fixed = [
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'class', 'a']],
  [['style', 'fill', 'red'], ['attr', 'x', '1'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['get', 'style'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['get', 'x'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['get', 'class'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['has', 'style'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['has', 'x'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['outer'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['inner'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['query', '[style]'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['query', '[x]'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['query', '.a'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['query', '[style*="red"]'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['matches', '[style]'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['matches', '[x="1"]'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['matches', ':not([style])'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['clone'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['bbox'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['computed', 'fill'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['text', 'hello'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['child', 'tspan'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'style', 'stroke: blue'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['attr', 'style', 'stroke: blue'], ['attr', 'class', 'a'], ['style', 'fill', 'red']],
  [['attr', 'style', 'stroke: blue'], ['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['remove', 'style'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['remove', 'style'], ['attr', 'class', 'a'], ['style', 'fill', 'blue'], ['attr', 'y', '2']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['remove', 'x'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['unstyle', 'fill'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'not a color'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['unstyle', 'fill'], ['attr', 'class', 'a']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['style', 'fill', 'blue'], ['attr', 'class', 'a'], ['style', 'stroke', 'green'], ['attr', 'y', '2']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'class', 'a'], ['get', 'style'], ['style', 'stroke', 'green'], ['attr', 'y', '2']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'class', 'a'], ['outer'], ['style', 'stroke', 'green'], ['attr', 'y', '2']],
  [['attr', 'x', '1'], ['style', 'fill', 'red'], ['attr', 'class', 'a'], ['outer'], ['unstyle', 'fill'], ['attr', 'y', '2']],
  [['style', 'fill', 'red'], ['unstyle', 'fill'], ['attr', 'x', '1']],
  [['style', 'fill', 'red'], ['outer'], ['unstyle', 'fill'], ['attr', 'x', '1']],
  [['style', 'text-anchor', 'middle'], ['style', 'font-size', '16px'], ['style', 'font-family', '"trebuchet ms", verdana;'], ['attr', 'y', '5'], ['attr', 'dominant-baseline', 'central'], ['attr', 'class', 'actor']],
  [['attr', 'x', '1'], ['attr', 'xlink:href', '#a'], ['style', 'fill', 'red'], ['attr', 'xml:space', 'preserve']],
];

const rnd = prng(0x7374796c);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const randomOps = () => {
  const n = 2 + Math.floor(rnd() * 9);
  const ops = [];
  for (let i = 0; i < n; i++) {
    const kind = Math.floor(rnd() * 16);
    if (kind < 4) {
      ops.push(['attr', pick(['x', 'y', 'class', 'id', 'dy', 'data-a', 'style']), pick(['1', 'a b', 'fill: red', 'stroke: blue;'])]);
    } else if (kind < 8) {
      ops.push(['style', pick(['fill', 'stroke', 'font-size', 'display']), pick(['red', 'blue', '16px', 'none', 'bogus value!', '#fff'])]);
    } else if (kind === 8) {
      ops.push(['unstyle', pick(['fill', 'stroke', 'font-size'])]);
    } else if (kind === 9) {
      ops.push(['get', pick(['x', 'style', 'class'])]);
    } else if (kind === 10) {
      ops.push(['has', pick(['x', 'style'])]);
    } else if (kind === 11) {
      ops.push(['remove', pick(['x', 'style', 'class'])]);
    } else if (kind === 12) {
      ops.push([pick(['outer', 'inner', 'clone', 'bbox'])]);
    } else if (kind === 13) {
      ops.push(['query', pick(['[style]', '[x]', '.a', 'text', '#t', '[style^="fill"]'])]);
    } else if (kind === 14) {
      ops.push(['matches', pick(['[style]', '[x]', '.a', ':not([style])'])]);
    } else {
      ops.push(['computed', 'fill']);
    }
  }
  return ops;
};

const cases = [];
for (const tag of ['text', 'div']) {
  fixed.forEach((ops, i) => cases.push({ name: `${tag}-fixed-${i}`, tag, ops }));
  for (let i = 0; i < 150; i++) {
    cases.push({ name: `${tag}-random-${i}`, tag, ops: randomOps() });
  }
}

const page = `
const container = document.getElementById('container');
const out = [];
for (const c of args.cases) {
  container.innerHTML = c.tag === 'div' ? '<section></section>' : '<svg xmlns="http://www.w3.org/2000/svg"><g></g></svg>';
  const parent = container.querySelector(c.tag === 'div' ? 'section' : 'g');
  const el = c.tag === 'div'
    ? document.createElement('div')
    : document.createElementNS('http://www.w3.org/2000/svg', c.tag);
  parent.appendChild(el);
  for (const [op, a, b] of c.ops) {
    switch (op) {
      case 'attr': {
        const i = a.indexOf(':');
        if (i >= 0) {
          const ns = { xlink: 'http://www.w3.org/1999/xlink', xml: 'http://www.w3.org/XML/1998/namespace' }[a.slice(0, i)];
          el.setAttributeNS(ns, a, b);
        } else {
          el.setAttribute(a, b);
        }
        break;
      }
      case 'remove': el.removeAttribute(a); break;
      case 'get': el.getAttribute(a); break;
      case 'has': el.hasAttribute(a); break;
      case 'style': el.style.setProperty(a, b); break;
      case 'unstyle': el.style.removeProperty(a); break;
      case 'outer': void el.outerHTML; break;
      case 'inner': void parent.innerHTML; break;
      case 'query': parent.querySelector(a); break;
      case 'matches': el.matches(a); break;
      case 'clone': el.cloneNode(true); break;
      case 'bbox': if (el.getBBox) { el.getBBox(); } else { el.getBoundingClientRect(); } break;
      case 'computed': getComputedStyle(el).getPropertyValue(a); break;
      case 'text': el.textContent = a; break;
      case 'child': el.appendChild(document.createElementNS(el.namespaceURI, a)); break;
    }
  }
  out.push(el.outerHTML);
}
container.innerHTML = '';
return out;
`;

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'stylesync-'));
const scriptFile = path.join(tmp, 'page.js');
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, page);
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', scriptFile, '--bare', '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== cases.length) {
  throw new Error(`expected ${cases.length} results, got ${results.length}`);
}
const golden = cases.map((c, i) => ({ ...c, outer: results[i] }));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, '[\n' + golden.map((g) => JSON.stringify(g)).join(',\n') + '\n]\n');
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`${outFile}: ${golden.length} cases`);
