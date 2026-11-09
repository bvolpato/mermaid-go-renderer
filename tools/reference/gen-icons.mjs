// Runs icon cases through upstream mermaid in Chromium and writes the
// golden file of internal/ru/icons_test.go.
//
//   node gen-icons.mjs [golden.json.gz]
//
// See probes/icons.js for the format. The cases use a small test pack
// that has the forms of IconifyJSON: plain icons, sizes, offsets,
// transformations, aliases, ids to replace and values of wrong types.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = process.argv[2] || path.join(here, '../../internal/ru/testdata/icons.json.gz');

const pack = {
  prefix: 'tp',
  width: 24,
  height: 24,
  icons: {
    plain: { body: '<path d="M4 4h16v16H4z" fill="currentColor"/>' },
    sized: { body: '<circle cx="16" cy="8" r="7"/>', width: 32, height: 16 },
    tall: { body: '<rect width="10" height="30"/>', width: 10, height: 30 },
    offset: { body: '<rect x="-4" y="2" width="20" height="12"/>', left: -4, top: 2, width: 20, height: 12 },
    frac: { body: '<path d="M0 0h7v9z"/>', width: 7, height: 9 },
    hflip: { body: '<path d="M1 2h3"/>', hFlip: true },
    vflip: { body: '<path d="M1 2h3"/>', vFlip: true, left: 2, top: 3 },
    bothflip: { body: '<path d="M1 2h3"/>', hFlip: true, vFlip: true },
    rot1: { body: '<path d="M1 2h3"/>', rotate: 1, width: 30, height: 10, left: 1, top: 2 },
    rot2: { body: '<path d="M1 2h3"/>', rotate: 2, width: 30, height: 10 },
    rot3: { body: '<path d="M1 2h3"/>', rotate: 3, width: 30, height: 10 },
    rot5: { body: '<path d="M1 2h3"/>', rotate: 5 },
    rotneg: { body: '<path d="M1 2h3"/>', rotate: -1, width: 12 },
    hidden: { body: '<path d="M0 0"/>', hidden: true },
    ids: {
      body: '<defs><linearGradient id="a" x1="0" x2="1"><stop offset="0" stop-color="#f00"/><stop offset="1" stop-color="#00f"/></linearGradient><clipPath id="b12"><rect width="12" height="24"/></clipPath></defs><g clip-path="url(#b12)"><path fill="url(#a)" d="M0 0h24v24H0z"/></g>',
    },
    xlink: {
      body: '<defs><path id="p.q" d="M0 0h4v4z"/></defs><use xlink:href="#p.q"/><use href="#p.q" x="5"/><animate begin="p.q.end;p.q.click" dur="1s"/>',
    },
    idstyle: {
      body: '<style>#s1{fill:red}</style><g id="s1" class="x"><path id="s1b" style="fill:url(#s1b);stroke:url(#s1)" d="M0 0"/></g>',
    },
    idspecial: { body: '<g id="a+b(c)" filter="url(#a+b(c))"><path id="$1" mask="url(#$1)"/></g>' },
    idnospace: { body: '<g id="x"fill="red"><use href="#x"/></g><g\tid="y"/><g  id="z" data-a="#z"/>' },
    idrepeat: { body: '<g id="a"><g id="a"/><use href="#a"/></g>' },
    idnone: { body: '<g data-id="q" xid="r"><path ID="s"/></g>' },
    defsflip: {
      body: '  <defs> <path id="d" d="M0 0"/> </defs>  <use href="#d"/><defs><path id="e" d="M1 1"/></defs><use href="#e"/> ',
      hFlip: true,
    },
    defsopen: { body: '<defs><path d="M0 0"/><use/>', rotate: 1 },
    defsonly: { body: '<defs></defs>', vFlip: true },
    iddot: { body: '<g id="p" class="p.end" data-b="x;p.click;p.Q;p.9;p."><path id="p.q" d="M0 0" class="p.q.end"/></g>' },
    text: { body: '<text x="1" y="12">a &amp; b &lt; c</text>' },
    script: { body: '<script>alert(1)</script><path d="M0 0" onclick="x()"/><a href="javascript:alert(1)"><path/></a>' },
    nobody: { width: 5 },
    emptybody: { body: '' },
    strsize: { body: '<path d="M0 0"/>', width: '20', height: '10' },
    nullsize: { body: '<path d="M0 0"/>', width: null, height: null },
    zerosize: { body: '<path d="M0 0"/>', width: 0, height: 0 },
    strrotate: { body: '<path d="M0 0"/>', rotate: '1' },
    numbody: { body: 5 },
    isnumber: 7,
    isstring: 'text',
    istrue: true,
    isnull: null,
    isarray: [1, 2],
    0: { body: '<path d="M9 9"/>' },
  },
  aliases: {
    alias: { parent: 'sized' },
    aliasflip: { parent: 'sized', hFlip: true },
    aliasrot: { parent: 'rot1', rotate: 1 },
    aliasrot2: { parent: 'aliasrot', rotate: 2, vFlip: true },
    aliasunflip: { parent: 'hflip', hFlip: true },
    aliassize: { parent: 'plain', width: 48, height: 12, left: 3 },
    aliasmissing: { parent: 'nothere' },
    aliasloop1: { parent: 'aliasloop2' },
    aliasloop2: { parent: 'aliasloop1' },
    aliasself: { parent: 'aliasself' },
    aliasnoparent: { hFlip: true },
    aliasnum: { parent: 0 },
    aliasnumber: 5,
    aliasbad: { parent: 'isnumber' },
    aliasproto: { parent: 'constructor' },
    plain: { parent: 'sized' },
  },
};
const noAliases = { prefix: 'na', icons: { one: { body: '<path d="M1 1"/>' } } };
const noIcons = { prefix: 'ni' };
const fa = {
  prefix: 'fa',
  icons: {
    bell: { body: '<path id="bell" d="M224 512c35.32 0 63.97-28.65 63.97-64H160.03c0 35.35 28.65 64 63.97 64z"/><use href="#bell"/>', width: 448, height: 512 },
    'bell-slash': { body: '<path d="M0 0h640v512z"/>', width: 640, height: 512 },
  },
};
const packs = [
  { name: 'tp', icons: pack },
  { name: 'na', icons: noAliases },
  { name: 'ni', icons: noIcons },
  { name: 'fa', icons: fa },
  { name: 'other', icons: pack },
  // Packs with a loader: mermaid loads them on first use.
  { name: 'lazy', loader: { prefix: 'ignored', icons: { one: { body: '<path id="l" d="M2 2"/>' } }, width: 10 } },
  { name: 'broken', loaderFails: true },
];

const names = [
  ...Object.keys(pack.icons).map((n) => 'tp:' + n),
  ...Object.keys(pack.aliases).map((n) => 'tp:' + n),
  'tp:nothere', 'tp:constructor', 'tp:toString', 'tp:__proto__', 'tp:hasOwnProperty', 'tp:valueOf',
  'na:one', 'na:two', 'na:constructor', 'ni:one', 'fa:bell', 'fa:bell-slash', 'fa-bell', 'fa-bell-slash',
  'lazy:one', 'lazy:two', 'lazy:one', 'broken:one', 'broken:one',
  'other:plain', 'nopack:plain', 'plain', 'tp-plain', 'tp-aliasrot2', '@prov:tp:plain', '@prov:tp-plain', '@prov',
  'a:tp:plain', 'a:b:tp:plain', 'tp:', ':plain', ':', '', 'tp::plain', 'TP:plain', 'tp:Plain', '-plain', 'tp-',
];
const sizes = [
  undefined,
  {},
  { width: 48, height: 48, fallbackPrefix: '' },
  { height: 48 },
  { width: 48 },
  { width: 30, height: 50 },
  { width: 'auto' },
  { height: 'auto' },
  { width: 'auto', height: 'auto' },
  { width: '2em' },
  { height: '1.5em' },
  { height: '24px' },
  { width: 'calc(1em + 3px)', height: null },
  { height: '-1.5em-.5..em3' },
  { height: '..5em' },
  { width: 'unset', height: 'none' },
  { width: 'undefined' },
  { width: null, height: null },
  { height: 0 },
  { width: 10.005, height: 3.3333 },
];
const transforms = [
  { rotate: 1 }, { rotate: 2 }, { rotate: 3 }, { rotate: 4 }, { rotate: -1 }, { rotate: -6 }, { rotate: 7 },
  { hFlip: true }, { vFlip: true }, { hFlip: true, vFlip: true }, { hFlip: true, rotate: 1 }, { vFlip: true, rotate: 3, height: 20 },
  { rotate: '1' }, { rotate: 1.5 },
];
const steps = [];
for (const name of names) {
  steps.push({ op: 'available', name });
  steps.push({ op: 'svg', name });
  steps.push({ op: 'svg', name, customisations: { height: 48, width: 48, fallbackPrefix: '' } });
  steps.push({ op: 'svg', name, customisations: { height: 14, width: 14, fallbackPrefix: 'tp' } });
  steps.push({ op: 'svg', name, extraAttributes: { class: 'label-icon' } });
}
const sizeNames = ['tp:plain', 'tp:sized', 'tp:tall', 'tp:offset', 'tp:frac', 'tp:rot1', 'tp:aliasrot2', 'tp:strsize', 'tp:nullsize', 'tp:zerosize', 'tp:ids', 'nopack:x'];
for (const name of sizeNames) {
  for (const customisations of sizes) {
    steps.push({ op: 'svg', name, customisations });
  }
  for (const customisations of transforms) {
    steps.push({ op: 'svg', name, customisations });
  }
}
for (const name of ['tp:defsflip', 'tp:defsopen', 'tp:defsonly', 'tp:xlink', 'tp:vflip', 'tp:bothflip']) {
  for (const customisations of transforms) {
    steps.push({ op: 'svg', name, customisations });
  }
}
const extras = [
  { class: 'label-icon' },
  { class: 'a b', 'data-x': '1', 2: 'two', 1: 'one' },
  { width: '5', viewBox: '0 0 1 1', style: 'color: red' },
  { title: 'a "quoted" <value>' },
];
for (const extraAttributes of extras) {
  for (const name of ['tp:plain', 'tp:ids', 'nopack:x', 'fa:bell']) {
    steps.push({ op: 'svg', name, extraAttributes });
    steps.push({ op: 'svg', name, customisations: { width: 20 }, extraAttributes });
  }
}
const short = [];
for (const name of ['tp:plain', 'tp:ids', 'tp:xlink', 'tp:idstyle', 'tp:script', 'tp:text', 'nopack:x', 'fa:bell', 'tp:aliasrot2']) {
  short.push({ op: 'available', name });
  short.push({ op: 'svg', name, customisations: { height: 48, width: 48, fallbackPrefix: '' } });
  short.push({ op: 'svg', name, extraAttributes: { class: 'label-icon' } });
}
const groups = [
  { directive: {}, steps },
  { directive: { securityLevel: 'loose' }, steps: short },
  { directive: { securityLevel: 'antiscript' }, steps: short },
  { directive: { securityLevel: 'sandbox' }, steps: short },
  { directive: { htmlLabels: false }, steps: short },
  { directive: { flowchart: { htmlLabels: false } }, steps: short },
];
// A pack that registerIconPacks refuses.
packs.push({ name: '', icons: pack }, { name: 'neither' });

// Direct calls of the @iconify/utils functions, with handmade and random
// inputs. The random numbers come from a seeded generator.
let seed = 0x1c0f1;
const random = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const pick = (list) => list[Math.floor(random() * list.length)];
const join = (tokens, max) => Array.from({ length: Math.floor(random() * (max + 1)) }, () => pick(tokens)).join('');
const units = [];
for (const value of [...names, 'a-b-c', 'a:b-c', '@:a:b', '@p:a', '@p:a-b', '@p', '@', 'a:b:c:d', '::', ':a:', 'x']) {
  for (const validate of [true, false]) {
    for (const allowSimpleName of [true, false]) {
      units.push({ fn: 'stringToIcon', value, validate, allowSimpleName });
    }
  }
}
const protoNames = ['constructor', 'toString', '__proto__', 'hasOwnProperty', 'valueOf', 'isPrototypeOf', 'toLocaleString', 'nothere'];
for (const name of [...Object.keys(pack.icons), ...Object.keys(pack.aliases), ...protoNames]) {
  units.push({ fn: 'getIconData', pack: 0, name });
}
for (const name of ['one', 'two', ...protoNames]) {
  units.push({ fn: 'getIconData', pack: 1, name });
  units.push({ fn: 'getIconData', pack: 2, name });
}
const body = '<path d="M0 0"/>';
const boxValues = [0, 1, -3, 7.5, 16, 24, 100, 0.1, 1e21];
const rotations = [0, 1, 2, 3, 4, 5, 7, -1, -2, -3, -5, 1.5, 0.5];
const customSizes = [undefined, null, 'auto', 48, 14, 0, 2.5, '1em', '2em', '1.25em', '24px', '10%', 'unset', 'none', 'undefined', '', '3', 'calc(100% - 2.5px)', '-0.5em', '1e3px'];
const randomIcon = () => {
  const icon = { body };
  for (const key of ['left', 'top', 'width', 'height']) {
    if (random() < 0.6) {
      icon[key] = pick(boxValues);
    }
  }
  if (random() < 0.5) {
    icon.rotate = pick(rotations);
  }
  if (random() < 0.3) {
    icon.hFlip = random() < 0.8;
  }
  if (random() < 0.3) {
    icon.vFlip = random() < 0.8;
  }
  if (random() < 0.2) {
    icon.body = pick(['<defs><a/></defs><b/>', ' <defs> x </defs> y ', '<g/><defs/>', '', 'text']);
  }
  return icon;
};
const randomCustomisations = () => {
  if (random() < 0.15) {
    return undefined;
  }
  const c = {};
  for (const key of ['width', 'height']) {
    const v = pick(customSizes);
    if (v !== undefined) {
      c[key] = v;
    }
  }
  if (random() < 0.4) {
    c.rotate = pick(rotations);
  }
  if (random() < 0.25) {
    c.hFlip = random() < 0.8;
  }
  if (random() < 0.25) {
    c.vFlip = random() < 0.8;
  }
  return c;
};
for (let i = 0; i < 1500; i++) {
  units.push({ fn: 'iconToSVG', icon: randomIcon(), customisations: randomCustomisations() });
}
// Values of types that IconifyIcon does not allow.
const oddIcons = [
  {}, { body: 5 }, { body: null }, { body: true }, { body: {} }, { body, width: '20', height: '10' },
  { body, width: null, height: null }, { body, left: '1', top: '2', rotate: 1 }, { body, rotate: '1' }, { body, rotate: '3x' },
  { body, rotate: null }, { body, rotate: true, hFlip: 1, vFlip: 'yes' }, { body, hFlip: 'x', left: 'a', width: 'b' },
  { body, vFlip: 1, top: 't', height: 'h' }, { body, width: [5], height: [1, 2] }, { body, width: {}, height: 3 },
  { body, width: true, height: false }, { body, left: null, top: null, hFlip: true }, { body, rotate: 2, width: 'w', left: 1 },
  { body, rotate: 1, left: '0', top: 0 }, { body, rotate: 1, width: 16, height: '16' }, { body: 5, rotate: 1 }, { rotate: 1 },
];
const oddCustomisations = [undefined, {}, { width: 48, height: 48 }, { rotate: 1 }, { hFlip: true }, { vFlip: true }, { width: 'auto' }, { height: 'auto' }, { rotate: '2' }, { width: true }, { width: 5, height: [7] }, { hFlip: 1, vFlip: 1 }];
for (const icon of oddIcons) {
  for (const customisations of oddCustomisations) {
    units.push({ fn: 'iconToSVG', icon, customisations });
  }
}
const ratios = [[1, 1], [1, 2], [2, 1], [1, 3], [24, 7], [0, 1], [0, 0], [1, 0], [448, 512], [512, 448], [7, 9], [-1, 2], [10000001, 10000000]];
const sizeTokens = ['-', '.', '0', '1', '2', '5', '9', 'e', 'm', 'px', ' ', '%', '+', '(', ')'];
const sizeList = [48, 14, 0, 2.5, -3, 1e21, 0.1, '1em', '', '.', '-', '-.', '..5', '5..', '1.2.3', '-1-2', '--1', 'a1b2c3', '1e5', '.5.5', '0', '00.10', null, true];
for (const [num, den] of ratios) {
  for (const size of sizeList) {
    units.push({ fn: 'calculateSize', size, num, den });
  }
  for (let i = 0; i < 40; i++) {
    units.push({ fn: 'calculateSize', size: join(sizeTokens, 8), num, den });
  }
}
// wrapSVGContent of upstream does not end for some bodies (a closing tag
// before the opening tag). Those would hang the page, so they are left out.
const wrapEnds = (content) => {
  const index = content.indexOf('<defs');
  for (let pass = 0; index >= 0; pass++) {
    if (pass > 100) {
      return false;
    }
    const start = content.indexOf('>', index);
    const end = content.indexOf('</defs');
    if (start === -1 || end === -1) {
      break;
    }
    const endEnd = content.indexOf('>', end);
    if (endEnd === -1) {
      break;
    }
    content = content.slice(0, index).trim() + content.slice(endEnd + 1);
  }
  return true;
};
const defsTokens = ['<defs>', '<defs', '</defs>', '</defs', '>', ' ', 'a', '<g/>', '\n', '<defs id="x">', '\u00a0', 'é', '   ', '</defs>', ' '];
const wrapBodies = ['', 'plain', '<defs></defs>', '<defs>a</defs>b', ' <defs> a </defs> b ', '<defs>a</defs><defs>b</defs>c', 'x<defs>a</defs>y</defs>z>', '<defs><defs>a</defs></defs>', '</defs  <defs>x</defs>', '<defs>a', '<defs', 'a</defs>', '<defs>a</defs', 'é<defs>ü</defs>ß'];
// A closing tag before the opening tag: the loop of upstream ends only
// when it trims enough white space.
wrapBodies.push('      </defs><defs>', '  </defs> <defs>a', ' </defs>   <defs></defs>  ', '\n\n</defs>\n\n\n<defs>\n</defs>', '   </defs></defs>   <defs>x</defs>');
for (let i = 0; i < 1500; i++) {
  wrapBodies.push(join(defsTokens, 9));
}
for (const b of wrapBodies) {
  if (wrapEnds(b)) {
    units.push({ fn: 'wrapSVGContent', body: b, start: '<g>', end: '</g>' });
  }
}
const idTokens = [' id="a"', ' id="a1"', '\tid="b.c"', ' id="x y"', 'id="q"', ' id="a"x"', '#a)', '"a"', ';a.b', '#a1"', 'url(#a)', '"b.c"', '#b.c.d', ' ', '<g', '/>', '>', ';a;', '"a.B', '#x', ' id=""', ' id="é"', '#é"', '\u00a0id="n"', '#n)', '\n id="$&"', '#$&)', ' id="a*"', '"a*"', '"aa"', ' ID="a"', " id='a'", ' id="suffix"', '#suffix)', ' id="IconifyId"'];
const idBodies = ['', 'none', ...Object.values(pack.icons).map((i) => i && i.body).filter((b) => typeof b === 'string')];
for (let i = 0; i < 700; i++) {
  idBodies.push(join(idTokens, 10));
}
for (const b of idBodies) {
  units.push({ fn: 'replaceIDs', body: b });
}
for (const b of ['', body, '<use xlink:href="#a"/>', 'xlink:', 'XLINK:href']) {
  for (const attributes of [{}, { width: '1em', height: '1em', viewBox: '0 0 24 24' }, { 2: 'b', 1: 'a', class: 'x "y"', viewBox: '' }]) {
    units.push({ fn: 'iconToHTML', body: b, attributes });
  }
}

const args = { packs, units, groups };
const tmpIn = path.join(os.tmpdir(), `mmdg-icons-in-${process.pid}.json`);
const tmpOut = path.join(os.tmpdir(), `mmdg-icons-out-${process.pid}.json`);
fs.writeFileSync(tmpIn, JSON.stringify(args));
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/icons.js'), '--args', tmpIn, '--out', tmpOut], { stdio: 'inherit' });
const result = JSON.parse(fs.readFileSync(tmpOut, 'utf8'));
fs.rmSync(tmpIn);
fs.rmSync(tmpOut);
// JSON has no undefined: a step without customisations has no key.
const golden = {
  packs,
  ...result,
  units: units.map((u, i) => ({ ...u, ...result.units[i] })),
  groups: groups.map((g, i) => ({ ...result.groups[i], steps: g.steps.map((s, j) => ({ ...s, ...result.groups[i].steps[j] })) })),
};
const json = JSON.stringify(golden);
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json) : json);
const all = golden.groups.flatMap((g) => g.steps);
console.error(`${golden.units.length} units (${golden.units.filter((u) => u.error).length} errors), ${all.length} steps (${all.filter((s) => s.error).length} errors) -> ${outFile}`);
