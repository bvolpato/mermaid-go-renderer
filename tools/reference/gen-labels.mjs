// Builds the label cases of internal/ru/labelprobe and records them with
// gen-shapes.mjs (upstream mermaid in Chromium).
//
// A node of the unified renderer measures its label in the browser:
// labelHelper calls createText, reads the size of the label
// (getBoundingClientRect of the div of an HTML label, getBBox of an SVG
// text) and sizes the shape from it. So the markup of a rect node has the
// measured size of its label in the foreignObject and in the rect. The
// cases put a wide set of label texts through the real insertNode with
// every label setting, and the Go test compares the markup and the size:
// it is the end-to-end check of the measurement engine for labels.
//
// The texts: plain, long (they wrap), with a very long word, with br
// elements, entities, HTML tags, markdown (bold, italic, lines), icons,
// images (data URLs, with and without text), CJK and mixed scripts, tokens
// with punctuation (the break opportunities differ from prose), empty and
// only white space. The settings: HTML labels and SVG labels, markdown
// auto wrap, the wrapping width, a width of the node, the minimum width,
// and the font of each theme.
//
//   node gen-labels.mjs            records the cases
//   node gen-labels.mjs --cases    only prints the number of cases
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = path.join(here, '../../internal/ru/labelprobe/testdata/labels.json.gz');

// png returns a data URL of a gray PNG image.
function png(w, h) {
  const chunk = (type, data) => {
    const len = Buffer.alloc(4);
    len.writeUInt32BE(data.length);
    const td = Buffer.concat([Buffer.from(type), data]);
    const crc = Buffer.alloc(4);
    crc.writeUInt32BE(zlib.crc32(td) >>> 0);
    return Buffer.concat([len, td, crc]);
  };
  const ihdr = Buffer.alloc(13);
  ihdr.writeUInt32BE(w, 0);
  ihdr.writeUInt32BE(h, 4);
  ihdr[8] = 8;
  const raw = Buffer.alloc((w + 1) * h, 0x80);
  for (let y = 0; y < h; y++) {
    raw[y * (w + 1)] = 0;
  }
  const file = Buffer.concat([Buffer.from([0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a]), chunk('IHDR', ihdr), chunk('IDAT', zlib.deflateSync(raw)), chunk('IEND', Buffer.alloc(0))]);
  return 'data:image/png;base64,' + file.toString('base64');
}

// The label texts. `markdown` selects labelType: 'markdown'. An image has
// the mark CASE in its URL: each case gets its own URL, because Chromium
// keeps an image that it loaded, and a label with the same URL would see a
// complete image at its first measure. In a diagram the first measure of a
// label sees a pending image.
const texts = {
  plain: { label: 'Start' },
  words: { label: 'Is it working?' },
  long: { label: 'This is a long label that must wrap over several lines to fit in the node' },
  longer: { label: 'The quick brown fox jumps over the lazy dog, and then the dog jumps over the quick brown fox again and again' },
  'long-word': { label: 'Supercalifragilisticexpialidocious_and_more_unbreakable_text_here' },
  'long-word-in-text': { label: 'A label with Supercalifragilisticexpialidocious in the middle of it' },
  br: { label: 'Line one<br/>the second line' },
  'br-plain': { label: 'Line one<br>the second line<br>three' },
  'br-spaces': { label: 'long line using <br>should work' },
  'br-double': { label: 'x<br/><br/>y' },
  entities: { label: 'Fish &amp; chips &lt;tag&gt; #9829; 100%' },
  quotes: { label: 'Lorem "ipsum" dolor "sit" amet,"consectetur" adipiscing "elit".' },
  html: { label: 'Hello <b>bold</b> and <i>italic</i> text' },
  'html-strong': { label: 'Mixed <strong>strong</strong><br/><em>em</em> end' },
  code: { label: 'call <code>code()</code> now' },
  icon: { label: 'fa:fa-car Car' },
  'icon-only': { label: 'fa:fa-bell' },
  'icon-long': { label: 'fa:fa-cogs Dedicated Test Business Logic Server:Service 1' },
  path: { label: 'C:\\Users\\name\\Documents\\file.txt path' },
  url: { label: 'https://example.com/a/very/long/url/that/does/not/break' },
  dotted: { label: 'SAM.CommonFA.CommonFAFinanceNurseryDays' },
  brackets: { label: '[DBServer\\SharedDbInstance].[SupportDb]' },
  hyphens: { label: 'a-b-c-d-e hyphen-ated words-here and more-text to-wrap around the-line' },
  numbers: { label: 'Total: 1,234.56 (-7%) 2024-01-15 #42' },
  cjk: { label: '负责人审批 强子 你好，世界' },
  'cjk-long': { label: '这是一个很长的标签，它必须换行才能适应节点的宽度，所以我们写了很多字' },
  mixed: { label: 'Mixed 你好 Привет Γειά σου text' },
  accents: { label: 'Égalité naïve façade Ångström' },
  spaces: { label: 'two  spaces   and    more' },
  empty: { label: '' },
  blank: { label: ' ' },
  'md-bold': { label: '**Bold** and *italic* text', markdown: true },
  'md-lines': { label: 'A **bold** word\nand a *second* line', markdown: true },
  'md-long': { label: 'The dog in **the** hog.(1).. a a a a *very long text* about it\nWord!', markdown: true },
  'md-plain': { label: 'This is a label for horizontal-cylinder shape', markdown: true },
  'md-backtick': { label: '`one` and __two__ and _three_', markdown: true },
  // The italic part is not CJK: the fallback font for CJK has no italic
  // face, Chromium slants its glyphs, and the engine does not slant the ink
  // bounds of such glyphs for the box of an SVG text.
  'md-cjk': { label: '**负责人** 审批 强子 *ok*', markdown: true },
  'md-empty': { label: '', markdown: true },
  image: { label: `<img src='${png(40, 20)}'>` },
  'image-text': { label: `<img src='${png(100, 100)}'> some text` },
  'image-two': { label: `<img src="${png(16, 64)}"> more text <img src='${png(16, 64)}'>` },
  'image-broken': { label: "<img src='data:image/png;base64,AAAA'> broken" },
  'image-wide': { label: `<img src='${png(300, 30)}'> a wide image and a text that wraps` },
};

// The settings: the directive of the diagram and the properties of the node.
const settings = {
  'html-neo': { directive: { look: 'neo', theme: 'redux-color' } },
  'html-classic': { directive: { look: 'classic', theme: 'default' } },
  'svg-neo': { directive: { look: 'neo', theme: 'redux-color', htmlLabels: false } },
  'svg-classic': { directive: { look: 'classic', theme: 'default', htmlLabels: false } },
  'autowrap-off': { directive: { look: 'classic', theme: 'default', markdownAutoWrap: false } },
  'autowrap-off-svg': { directive: { look: 'classic', theme: 'default', markdownAutoWrap: false, htmlLabels: false } },
  wrap100: { directive: { look: 'classic', theme: 'default', flowchart: { wrappingWidth: 100 } } },
  'wrap300-svg': { directive: { look: 'classic', theme: 'default', htmlLabels: false, flowchart: { wrappingWidth: 300 } } },
  'node-width': { directive: { look: 'neo', theme: 'redux-color' }, node: { width: 150 } },
  'node-wrap60': { directive: { look: 'neo', theme: 'default' }, node: { wrappingWidth: 60, minWidth: undefined } },
  'no-min': { directive: { look: 'classic', theme: 'default' }, node: { minWidth: undefined } },
  'no-min-svg': { directive: { look: 'classic', theme: 'default', htmlLabels: false }, node: { minWidth: undefined } },
  dark: { directive: { look: 'classic', theme: 'dark' } },
  forest: { directive: { look: 'classic', theme: 'forest', htmlLabels: false } },
  neutral: { directive: { look: 'neo', theme: 'neutral' } },
  base: { directive: { look: 'classic', theme: 'base', themeVariables: { fontFamily: 'courier', fontSize: '20px' } } },
  'base-svg': { directive: { look: 'classic', theme: 'base', htmlLabels: false, themeVariables: { fontFamily: 'times', fontSize: '13px' } } },
  'neo-dark': { directive: { look: 'neo', theme: 'neo-dark' } },
  'label-style': { directive: { look: 'classic', theme: 'default' }, node: { cssStyles: ['color:#fff', 'font-size:20px', 'fill:#036'] } },
  'label-style-svg': { directive: { look: 'classic', theme: 'default', htmlLabels: false }, node: { cssStyles: ['font-weight:bold', 'font-size:18px'] } },
};

const cases = [];
for (const [textName, text] of Object.entries(texts)) {
  for (const [settingName, setting] of Object.entries(settings)) {
    cases.push({
      name: `label/${textName}/${settingName}`,
      type: 'flowchart-v2',
      directive: setting.directive,
      node: {
        id: 'A',
        label: text.label.replaceAll('data:image/png;', `data:image/png;case=${cases.length};`),
        labelType: text.markdown ? 'markdown' : 'text',
        labelStyle: '',
        padding: 15,
        cssStyles: [],
        cssCompiledStyles: [],
        cssClasses: 'default ',
        domId: 'my-svg-flowchart-A-0',
        look: setting.directive.look,
        isGroup: false,
        shape: 'squareRect',
        minWidth: 120,
        ...(setting.node || {}),
      },
    });
  }
}

if (process.argv.includes('--cases')) {
  console.log(cases.length);
  process.exit(0);
}
const tmp = path.join(os.tmpdir(), `mmdg-labels-${process.pid}.json`);
fs.writeFileSync(tmp, JSON.stringify({ cases }));
try {
  execFileSync('node', [path.join(here, 'gen-shapes.mjs'), tmp, process.env.MMDG_LABELS_OUT || outFile], { stdio: 'inherit' });
} finally {
  fs.rmSync(tmp, { force: true });
}
