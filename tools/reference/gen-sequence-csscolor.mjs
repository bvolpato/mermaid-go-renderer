// Writes internal/diagrams/sequence/testdata/css_supports_color.json: the
// result of window.CSS.supports('color', value) in Chromium for the values
// that parseBoxData of sequenceDb.ts can test: one word (\w*), or a text that
// starts with rgb(, rgba(, hsl( or hsla( and ends with ")".
//
// Usage (in tools/reference): node gen-sequence-csscolor.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const outFile = path.join(repoRoot, 'internal/diagrams/sequence/testdata/css_supports_color.json');

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

const words = [
  '', 'transparent', 'Transparent', 'currentcolor', 'currentColor', 'inherit', 'initial', 'unset',
  'revert', 'default', 'none', 'auto', 'red', 'RED', 'Aqua', 'LightYellow', 'rebeccapurple', 'grey',
  'gray', 'darkgrey', 'NotAColor', 'Group', 'box', 'a', '_', '1', '123', 'fff', 'ff0000', '0', 'x11',
  'canvas', 'CanvasText', 'linktext', 'visitedtext', 'activetext', 'buttonface', 'buttontext',
  'buttonborder', 'field', 'fieldtext', 'highlight', 'highlighttext', 'selecteditem',
  'selecteditemtext', 'mark', 'marktext', 'graytext', 'accentcolor', 'accentcolortext',
  'activeborder', 'activecaption', 'appworkspace', 'background', 'buttonhighlight', 'buttonshadow',
  'captiontext', 'inactiveborder', 'inactivecaption', 'inactivecaptiontext', 'infobackground',
  'infotext', 'menu', 'menutext', 'scrollbar', 'threeddarkshadow', 'threedface', 'threedhighlight',
  'threedlightshadow', 'threedshadow', 'window', 'windowframe', 'windowtext', 'text', 'link',
  'rgb', 'rgba', 'hsl', 'hsla', 'black', 'white', 'orange', 'Purple', 'tan', 'teal', 'flavor',
  'lightgoldenrodyellow', 'mediumspringgreen', 'windowText', 'revert_layer', 'Alice', 'true', 'e',
];

const functions = [
  'rgb(1,2,3)', 'rgb(1, 2, 3)', 'rgb( 1 , 2 , 3 )', 'RGB(1,2,3)', 'Rgba(1,2,3,0.5)', 'rgb (1,2,3)',
  'rgb\t(1,2,3)', 'rgb(1,2,3) x', 'rgb(1,2,3) (x)', 'rgb(1,2)', 'rgb(1,2,3,4,5)', 'rgb()', 'rgb(,,)',
  'rgb(1,2,3,)', 'rgb(1,,3)', 'rgb(1 2 3)', 'rgb(1 2 3 4)', 'rgb(1 2 3 / 4)', 'rgb(1 2 3/4)',
  'rgb(1 2 3 / 50%)', 'rgb(1 2 3 / none)', 'rgb(none 2 3)', 'rgb(none none none)',
  'rgb(none, 2, 3)', 'rgb(1, 2, 3, none)', 'rgb(10%,20%,30%)', 'rgb(10%, 20%, 30%, 40%)',
  'rgb(10%,20,30)', 'rgb(10,20%,30%)', 'rgb(10% 20 30)', 'rgb(10 20% 30)', 'rgb(10% 20% 30%)',
  'rgb(1.5,2.5,3.5)', 'rgb(1.,2,3)', 'rgb(.5,.5,.5)', 'rgb(+1,-2,3)', 'rgb(1e2,2,3)', 'rgb(1e,2,3)',
  'rgb(1E2 2 3)', 'rgb(1+2+3)', 'rgb(1-2-3)', 'rgb(1 2,3)', 'rgb(1,2 3)', 'rgb(1/2/3)',
  'rgb(1 2 3 /)', 'rgb(1 2 / 3)', 'rgb(1,2,3/4)', 'rgb(1, 2, 3 / 4)', 'rgb(300,-20,1000)',
  'rgb(1px,2,3)', 'rgb(1deg,2,3)', 'rgb(a,b,c)', 'rgb(red)', 'rgb(#fff)', 'rgb(1,2,3',
  'rgb(1,2,3))', 'rgb((1),2,3)', 'rgb(1,2,3)rgb(1,2,3)', 'rgb(1,2,3) rgb(4,5,6)',
  'rgb(calc(1 + 1), 2, 3)', 'rgb(calc(1 + 1) 2 3)', 'rgb(calc(50%) 2 3)', 'rgb(min(1,2), 2, 3)',
  'rgb(var(--x))', 'rgb(var(--x), 2, 3)', 'rgb(var(x), 2, 3)', 'rgb(var(--x) 2 3 / 4)',
  'rgb(env(x), 2, 3)', 'rgb(attr(x), 2, 3)', 'rgb(from red r g b)', 'rgb(from red r g b / 50%)',
  'rgb(from red 1 2 3)', 'rgb(1 2 3 / 0.5 )', 'rgb(1 2 3 / -1)', 'rgb(1 2 3 / 200%)',
  'rgb(0 0 0 / 0)', 'rgb(1,2,3,0.5)', 'rgb(1,2,3,50%)', 'rgb(1,2,3,+.5e1)', 'rgb(1 , 2 , 3 , 0.5)',
  'rgb(1,2,3)!important', 'rgb(1,2,3);', 'rgb(1,2,3)/**/', 'rgb(/**/1,2,3)', 'rgb(1,/* c */2,3)',
  'rgb(1 /**/ 2 3)', 'rgb(1/**/2/**/3)', 'rgb(1.5.5 2)', 'rgb(1.5.5.5)', 'rgb(10%20%30%)',
  'rgb(1,2,3) ', 'rgb(1\n2\n3)', 'rgb(1\t2\t3)', 'rgb(１,２,３)', 'rgb(1,2,3)\\', 'rgb(Infinity,2,3)',
  'rgb(NaN 2 3)', 'rgb(0x10,2,3)', 'rgb(1 2 3 / 1 2)', 'rgb(1 2 3 / 1 / 2)', 'rgb(1 2 3 4 / 5)',
  'rgba(1,2,3)', 'rgba(1 2 3)', 'rgba(1 2 3 / 0.5)', 'rgba(1,2,3,0.5)', 'rgba(1,2,3,0.5) Title',
  'rgba()', 'rgba(1%,2%,3%,4)', 'rgba(none none none / none)', 'rgbaa(1,2,3)', 'rgbx(1,2,3)',
  'hsl(120,50%,50%)', 'hsl(120, 50%, 50%)', 'HSL(120,50%,50%)', 'hsl(120,50,50)', 'hsl(120 50 50)',
  'hsl(120 50% 50%)', 'hsl(120 50% 50)', 'hsl(120deg,50%,50%)', 'hsl(120DEG,50%,50%)',
  'hsl(1.5turn 50% 50%)', 'hsl(2rad,50%,50%)', 'hsl(100grad,50%,50%)', 'hsl(120px,50%,50%)',
  'hsl(120%,50%,50%)', 'hsl(120% 50% 50%)', 'hsl(none 50% 50%)', 'hsl(none,50%,50%)',
  'hsl(120 none none)', 'hsl(120,50%,50%,0.5)', 'hsl(120,50%,50%,50%)', 'hsl(120 50% 50% / 0.5)',
  'hsl(120 50% 50% / 50%)', 'hsl(120 50% 50% / none)', 'hsl(120,50%,50%/0.5)', 'hsl(120,50%)',
  'hsl(120)', 'hsl()', 'hsl(-120,150%,-50%)', 'hsl(120e0,50%,50%)', 'hsl(120, 50%, 50%) T',
  'hsl(calc(60 * 2), 50%, 50%)', 'hsl(var(--h), 50%, 50%)', 'hsl(from red h s l)',
  'hsl(120deg 50% 50%)', 'hsl(120 deg,50%,50%)', 'hsl(120,50 %,50%)', 'hsl(.5turn,1%,1%)',
  'hsla(120,50%,50%)', 'hsla(120,50%,50%,0.3)', 'hsla(120 50% 50% / 0.3)', 'hsla(0, 100%, 50%, 0.3)',
  'hsla(120,50%,50%,0.3,1)', 'hsla()', 'hslaa(1,2%,3%)', 'hsl(0,0%,0%)', 'hsl(360,100%,100%)',
  'hsl(120, 50%, 50%, none)', 'hsl(120 50 50 / 1)', 'hsl(120 50 50%)', 'hsl(120,50,50%)',
  'hsl(1e2,5e1%,5e1%)', 'hsl(+120,+50%,+50%)', 'hsl(120,50%,50%,)', 'hsl(120 , 50% , 50%)',
  'rgb(1,2,3)a', 'rgb(1,2,3)1', 'hsl(1,2%,3%)_', 'rgb(1 2 3 / 4%)', 'rgb(100% 0% 0% / 100%)',
  'rgb(1,2,3,4%)', 'rgb(0,0,0,0)', 'rgb(00,01,002)', 'rgb(1 2 3/none)', 'rgb(1  2   3)',
];

// Random strings from the tokens of the color functions.
const rnd = prng(0x636f6c72);
const pick = (a) => a[Math.floor(rnd() * a.length)];
const tokens = [
  '0', '1', '25', '255', '300', '-5', '+7', '1.5', '.5', '5.', '1e1', '50%', '100%', '-10%', '0.5',
  'none', '120deg', '1turn', '2rad', '90grad', '10px', 'red', 'x',
];
const seps = [',', ', ', ' , ', ' ', '  ', '/', ' / ', '', '\t'];
for (let i = 0; i < 400; i++) {
  const fn = pick(['rgb', 'rgba', 'hsl', 'hsla']);
  const n = 2 + Math.floor(rnd() * 4);
  const mode = Math.floor(rnd() * 3);
  let body = '';
  for (let k = 0; k < n; k++) {
    if (k > 0) {
      if (mode === 0) {
        body += pick([',', ', ']);
      } else if (mode === 1) {
        body += k === n - 1 && n === 4 ? pick([' / ', '/']) : ' ';
      } else {
        body += pick(seps);
      }
    }
    body += pick(tokens);
  }
  functions.push(`${fn}(${body})`);
}

const values = [...new Set([...words, ...functions])];
const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'seqcss-'));
const scriptFile = path.join(tmp, 'page.js');
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(scriptFile, "return args.values.map((v) => CSS.supports('color', v));\n");
fs.writeFileSync(argsFile, JSON.stringify({ values }));
execFileSync(
  process.execPath,
  ['oracle.mjs', 'eval', scriptFile, '--bare', '--args', argsFile, '--out', resultFile],
  { cwd: here, stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
if (results.length !== values.length) {
  throw new Error(`expected ${values.length} results, got ${results.length}`);
}
const golden = values.map((value, i) => ({ value, supports: results[i] }));
fs.mkdirSync(path.dirname(outFile), { recursive: true });
fs.writeFileSync(outFile, JSON.stringify(golden, null, 0).replace(/\},\{/g, '},\n{') + '\n');
fs.rmSync(tmp, { recursive: true, force: true });
console.log(`${outFile}: ${golden.length} values, ${golden.filter((g) => g.supports).length} supported`);
