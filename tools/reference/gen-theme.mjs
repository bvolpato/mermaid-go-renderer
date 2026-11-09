// Emits golden cases for internal/theme from the real mermaid themes:
// theme[name].getThemeVariables(overrides) for every theme and many overrides.
//
// Usage (from tools/reference):
//   node gen-theme.mjs > ../../internal/theme/testdata/themes.json.gz
import zlib from 'node:zlib';
import { encode, internals } from './lib-mermaid-internals.mjs';

const { themes } = await internals();

let seed = 0x7468656d >>> 0; // "them"
const rnd = () => {
  seed = (seed + 0x6d2b79f5) >>> 0;
  let t = seed;
  t = Math.imul(t ^ (t >>> 15), t | 1);
  t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};
const int = (n) => Math.floor(rnd() * n);
const pick = (a) => a[int(a.length)];

const overrides = [
  undefined,
  {},
  { primaryColor: '#ff0000' },
  { primaryColor: 'rgb(10, 200, 30)' },
  { primaryColor: 'hsl(210, 60%, 40%)' },
  { primaryColor: 'tomato' },
  { primaryColor: '#12345678' },
  { primaryColor: 'HSL(120DEG, 50%, 50%)' },
  { primaryColor: 'rgb(300, -5, 12.5)' },
  { primaryColor: 'hsla(30, 100%, 50%, 0.3)', darkMode: true },
  { secondaryColor: '#00ff00' },
  { tertiaryColor: 'rgba(0,0,255,0.5)' },
  { background: '#000000' },
  { background: 'navy', darkMode: true },
  { background: 'transparent' },
  { lineColor: 'hsl(0, 100%, 50%)' },
  { fontFamily: 'Comic Sans MS' },
  { fontFamily: '"Fira Code", monospace', fontSize: '12px' },
  { fontSize: '20px' },
  { fontSize: 18 },
  { darkMode: true },
  { darkMode: false },
  { darkMode: 'false' },
  { primaryColor: '#abc', secondaryColor: '#def', tertiaryColor: '#123' },
  { primaryTextColor: 'white', primaryBorderColor: 'black' },
  { mainBkg: '#fa8072', nodeBorder: '#333' },
  { nodeBorder: 'red', useGradient: true },
  { nodeBorder: 'red' },
  { useGradient: false, gradientStart: 'red', gradientStop: 'blue' },
  { THEME_COLOR_LIMIT: 3 },
  { THEME_COLOR_LIMIT: 20 },
  { THEME_COLOR_LIMIT: '4' },
  { THEME_COLOR_LIMIT: 'many' },
  { xyChart: { backgroundColor: 'red', plotColorPalette: '#000,#fff' } },
  { xyChart: { extraKey: 1 }, cynefin: { domainFontSize: 20 }, radar: { axisColor: 'blue' }, wardley: { gridColor: 'pink' } },
  { xyChart: 'text' },
  { xyChart: null, radar: 5, cynefin: [], wardley: true },
  { cScale0: '#ff0000', cScale1: '#00ff00', cScale11: 'hsl(10, 20%, 30%)' },
  { cScaleInv2: 'black', cScalePeer3: 'white', cScaleLabel4: 'grey' },
  { git0: 'purple', gitInv0: 'white', gitBranchLabel0: 'black' },
  { pie1: 'red', pieOpacity: '0.5', pieStrokeWidth: '5px', pie12: '#abcdef' },
  { quadrant1Fill: '#101010' },
  { quadrant1Fill: '#fafafa', quadrantPointFill: 'green' },
  { noteBkgColor: 'yellow', noteTextColor: 'black', noteBorderColor: '' },
  { primaryColor: '' },
  { primaryColor: '', secondaryColor: '', tertiaryColor: '', background: '', mainBkg: '' },
  { textColor: 'rgb(50%, 50%, 50%)' },
  { actorBkg: 'red', actorBorder: 'blue', signalColor: 'green', labelBoxBkgColor: '#eee' },
  { borderColorArray: ['#111', '#222'], bkgColorArray: ['#eee'] },
  { borderColorArray: 'abc' },
  { bkgColorArray: [] },
  { borderColorArray: [] },
  { venn1: null, venn2: 0, venn3: '', venn4: 'red' },
  { titleColor: 'red', edgeLabelBackground: 'blue', clusterBkg: 'grey', clusterBorder: 'black' },
  { darkMode: true, primaryColor: '#1f2020', secondaryColor: '#333', tertiaryColor: '#444' },
  { unknownVar: 'x', 10: 'ten', 0: 'zero', nested: { a: [1, 2, { b: null }] } },
  { labelTextColor: 'calculated', scaleLabelColor: 'calculated' },
  { labelTextColor: 'red', scaleLabelColor: 'blue' },
  { activeTaskBkgColor: 'not a color', gridColor: 5, taskTextColor: false },
  { stateBkg: '#123456', compositeBackground: '', altBackground: 'white' },
  { rowOdd: 'red', rowEven: 'blue', attributeBackgroundColorOdd: '#000' },
  { archEdgeColor: 'red', archGroupBorderColor: 'blue' },
  { emUiFill: 'red', emSwimlaneBackgroundStroke: '', tagBorder: 'pink' },
  {
    fontFamily: 'verdana',
    fontSize: '16px',
    primaryColor: '#BB2528',
    primaryTextColor: '#fff',
    primaryBorderColor: '#7C0000',
    lineColor: '#F8B229',
    secondaryColor: '#006100',
    tertiaryColor: '#fff',
  },
  // Overrides that make the upstream code throw.
  { primaryColor: 'notacolor' },
  { background: 'rgb(1,2)' },
  { primaryColor: 5 },
  { primaryColor: null },
  { primaryColor: true },
  { quadrant1Fill: 5 },
  { mainBkg: 0 },
  { updateColors: 'x' },
  { lineColor: 'constructor' },
  // Overrides that are not plain objects.
  null,
  'abc',
  5,
  true,
  ['#fff', '#000'],
  [],
];

// Random subsets of the color variables with random colors.
const colorKeys = [
  'primaryColor', 'secondaryColor', 'tertiaryColor', 'background', 'mainBkg', 'lineColor', 'textColor',
  'primaryTextColor', 'primaryBorderColor', 'secondaryBorderColor', 'tertiaryBorderColor', 'noteBkgColor',
  'nodeBkg', 'nodeBorder', 'clusterBkg', 'secondBkg', 'border1', 'border2', 'arrowheadColor', 'labelBackground',
  'actorBkg', 'actorBorder', 'activationBkgColor', 'sectionBkgColor', 'taskBkgColor', 'quadrant1Fill', 'cScale0',
  'cScale5', 'git0', 'git3', 'pie1', 'stateBkg', 'edgeLabelBackground', 'titleColor', 'labelTextColor',
];
const randomColor = () => {
  const k = int(6);
  const h2 = () => int(256).toString(16).padStart(2, '0');
  if (k < 2) return '#' + h2() + h2() + h2();
  if (k === 2) return `rgb(${int(256)}, ${int(256)}, ${int(256)})`;
  if (k === 3) return `hsl(${int(360)}, ${int(101)}%, ${int(101)}%)`;
  if (k === 4) return `rgba(${int(256)}, ${int(256)}, ${int(256)}, ${(rnd()).toFixed(2)})`;
  return pick(['red', 'navy', 'white', 'black', 'LightGrey', 'rebeccapurple', 'transparent', 'gold', 'teal']);
};
for (let i = 0; i < 40; i++) {
  const o = {};
  const n = 1 + int(6);
  for (let j = 0; j < n; j++) o[pick(colorKeys)] = randomColor();
  if (rnd() < 0.3) o.darkMode = rnd() < 0.7;
  if (rnd() < 0.2) o.fontFamily = pick(['arial', 'monospace', '"Open Sans", sans-serif']);
  overrides.push(o);
}

const names = Object.keys(themes);
const keysets = new Map();
const cases = [];
const record = (theme, ov, label) => {
  // The overrides are cloned first: upstream stores nested values by reference.
  const input = ov === undefined ? undefined : structuredClone(ov);
  let result;
  try {
    const vars = themes[theme].getThemeVariables(input);
    const keys = Object.keys(vars);
    const sig = JSON.stringify(keys);
    if (!keysets.has(sig)) keysets.set(sig, keysets.size);
    result = { ks: keysets.get(sig), v: keys.map((k) => encode(vars[k])) };
  } catch (e) {
    result = { e: String(e.message), t: e instanceof TypeError };
  }
  cases.push({ theme, label, overrides: encode(ov), result });
};
for (const theme of names) {
  for (const ov of overrides) record(theme, ov);
  // initialize() and setSiteConfig() feed the variables of one theme back in
  // as the overrides of the next call.
  for (const other of [theme, 'default', 'base', 'redux-color']) {
    record(theme, themes[other].getThemeVariables(), `output of ${other}`);
    record(theme, themes[other].getThemeVariables({ primaryColor: '#123456', darkMode: true }), `output of ${other} with overrides`);
  }
}

const out = { names, keysets: [...keysets.keys()].map((s) => JSON.parse(s)), cases };
process.stdout.write(zlib.gzipSync(JSON.stringify(out), { level: 9 }));
process.stderr.write(`${cases.length} cases, ${cases.filter((c) => c.result.e).length} throw\n`);
