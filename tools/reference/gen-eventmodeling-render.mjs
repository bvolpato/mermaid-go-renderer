// Writes internal/diagrams/eventmodeling/testdata/render.json.gz: the SVG
// that mermaid.render of mermaid 12.2.0 returns in Chromium for hand-written
// event models, or the error. The texts have what the six fixtures of the
// corpus do not have: namespaces, reset frames, every entity type, inline
// data of each form, long texts that wrap, themes, theme variables and the
// config options that the renderer reads.
//
// The script runs the render command of the browser oracle one time, with
// one page for each text, as the reference renders do.
//
// Usage (in tools/reference): node gen-eventmodeling-render.mjs
import { renderExtras, withConfig } from './extra-render-lib.mjs';

const em = (body) => 'eventmodeling\n' + body + '\n';
const model = em(
  'tf 01 ui CartScreen\n' +
    'tf 02 cmd AddItem { productId: 7 }\n' +
    'tf 03 evt ItemAdded [[ItemAddedData]]\n' +
    'tf 04 rmo CartItems ->> 03\n' +
    'tf 05 pcr Notifier ->> 04\n' +
    'tf 06 cmd Inventory.Reserve ->> 05\n' +
    'tf 07 evt Inventory.Reserved\n' +
    'rf 08 ui Summary\n\n' +
    'data ItemAddedData\n{\n  productId: 7,\n  quantity: 1\n}'
);
const small = em('tf 01 ui Screen\ntf 02 cmd DoIt { id: 1 }\ntf 03 evt Done');
const long = 'A'.repeat(20) + 'B'.repeat(20) + 'C'.repeat(20) + 'D'.repeat(20);

const cases = [];
const add = (name, text) => cases.push({ name, text });

add('model', model);
add('small', small);
add('empty', 'eventmodeling\n');
add('no-frames', em('entity A\ndata D\n{\n  a: 1\n}\nnote 01 {\n  n\n}\ngwt 01 given ui A then evt A'));
add('one-frame', em('tf 01 ui A'));
add('every-type', em('tf 01 ui A\ntf 02 pcr B\ntf 03 processor C\ntf 04 rmo D\ntf 05 readmodel E\ntf 06 cmd F\ntf 07 command G\ntf 08 evt H\ntf 09 event I'));
add('every-type-reversed', em('tf 09 event I\ntf 08 evt H\ntf 07 command G\ntf 06 cmd F\ntf 05 readmodel E\ntf 04 rmo D\ntf 03 processor C\ntf 02 pcr B\ntf 01 ui A'));
add('one-swimlane', em('tf 01 evt A\ntf 02 evt B\ntf 03 evt C'));
add('upwards', em('tf 01 evt A\ntf 02 cmd B\ntf 03 ui C\ntf 04 evt D ->> 03\ntf 05 ui E ->> 01'));
add('namespaces', em('tf 01 ui Shop.Screen\ntf 02 cmd Shop.Buy\ntf 03 evt Shop.Bought\ntf 04 ui Other.Screen\ntf 05 cmd Pay\ntf 06 evt Billing.Paid'));
add('namespaces-repeat', em('tf 01 ui Shop.A\ntf 02 ui Shop.B\ntf 03 cmd Shop.C\ntf 04 cmd Shop.D\ntf 05 evt Shop.E\ntf 06 evt Shop.F'));
add('three-part-names', em('tf 01 ui A.B.C\ntf 02 cmd A.B.C.D\ntf 03 evt X.Y'));
add('source-frames', em('tf 01 ui A\ntf 02 cmd B\ntf 03 cmd C\ntf 04 evt D ->> 02 ->> 03\ntf 05 rmo E ->> 04 ->> 01\ntf 06 ui F ->> 99'));
add('source-forward', em('tf 01 ui A\ntf 02 cmd B ->> 03 ->> 02\ntf 03 evt C ->> 02'));
add('source-first', em('tf 01 ui A ->> 01 ->> 02\ntf 02 cmd B'));
add('reset-frames', em('tf 01 ui A\nrf 02 cmd B\ntf 03 evt C\nresetframe 04 ui D ->> 01\ntimeframe 05 cmd E'));
add('inline-forms', em('tf 01 cmd A { a: 1 }\ntf 02 evt B "a string"\ntf 03 cmd C \'single\'\ntf 04 evt D {}\ntf 05 cmd E ""\ntf 06 evt F { a: { b: 1 } }'));
add('inline-types', em('tf 01 cmd A `json` { "a": 1, "b": [1, 2] }\ntf 02 evt B `html` "<b>x</b>"\ntf 03 cmd C `md` { **x** }'));
add('inline-long', em('tf 01 cmd A { ' + 'word '.repeat(40) + '}\ntf 02 evt B { ' + long + long + long + ' }'));
add('inline-html', em('tf 01 cmd A { <b>bold</b> <i>it</i> }\ntf 02 evt B { <script>alert(1)</script> }\ntf 03 cmd C { a &amp; b &lt; c }\ntf 04 evt D { a<br/>b<br>c }'));
add('inline-unicode', em('tf 01 cmd A { é 😀 日本語 }\ntf 02 evt B { Ж: 1 }'));
add('data-reference', em('tf 01 evt A [[D]]\ntf 02 evt B [[E]]\ntf 03 evt C [[Missing]]\ndata D\n{\n  a: 1\n}\ndata E `json` {\n  "b": 2,\n  "c": {\n    "d": [1, 2, 3]\n  }\n}'));
add('data-reference-and-inline', em('tf 01 evt A [[D]] { inline: 1 }\ntf 02 cmd B [[Missing]] { inline: 2 }\ndata D\n{\n  block: 2\n}'));
add('data-long', em('tf 01 evt A [[D]]\ndata D\n{\n  first line\n  ' + 'word '.repeat(60) + '\n  third line\n}'));
add('data-empty', em('tf 01 evt A [[D]]\ndata D\n{\n}'));
add('data-not-in-render-tree', em('tf 01 evt A [[D]]\ndata D\n{ \n  a: 1\n}'));
add('long-names', em('tf 01 ui ' + long + '\ntf 02 cmd ' + long + long + long + '\ntf 03 evt ns.' + long + long));
add('comments', em('%% a comment\ntf 01 ui A %% trailing\n  %% indented\ntf 02 cmd B // slashes\ntf 03 evt C /* block */ tf 04 ui D'));
add('title-acc', 'eventmodeling accTitle: An accessible title\ntf 01 ui A accDescr: A description of the model\ntf 02 cmd B title A title\n');
add('acc-multiline', 'eventmodeling accDescr {\n a\n multi line\n}\ntf 01 ui A\n');
add('front-matter-title', '---\ntitle: A model\n---\n' + small);
add('many-frames', em(Array.from({ length: 30 }, (_, i) => `tf ${String(i + 1).padStart(2, '0')} ${['ui', 'cmd', 'evt', 'rmo', 'pcr'][i % 5]} ${i % 7 === 3 ? 'ns.' : ''}Frame${i}${i % 4 === 1 ? ' { n: ' + i + ' }' : ''}`).join('\n')));
add('error-duplicate', em('tf 01 ui A\ntf 02 cmd B\ntf 01 evt C'));
add('error-parse', em('tf 01 ui'));
add('error-lexer', em('tf 01 ui A;'));
add('error-order', em('tf 01 evt A [[D]] ->> 01'));

// Config.
for (const theme of ['default', 'dark', 'forest', 'neutral', 'base', 'redux', 'redux-dark', 'redux-color', 'redux-dark-color', 'neo', 'neo-dark']) {
  add(`config-theme-${theme}`, withConfig(`theme: ${theme}`, model));
}
add('config-theme-variables', withConfig('theme: base\nthemeVariables:\n  emUiFill: "#111111"\n  emUiStroke: "#222222"\n  emProcessorFill: red\n  emReadModelStroke: blue\n  emCommandFill: "rgb(1,2,3)"\n  emEventStroke: "#abc"\n  emArrowhead: "#ff00ff"\n  emRelationStroke: green\n  emSwimlaneBackgroundOdd: "#eeeeff"\n  emSwimlaneBackgroundStroke: "#0000ff"', model));
add('config-line-color', withConfig('themeVariables:\n  lineColor: "#ff0000"', model));
add('config-padding', withConfig('eventmodeling:\n  padding: 5', model));
add('config-padding-0', withConfig('eventmodeling:\n  padding: 0', small));
add('config-max-width-false', withConfig('eventmodeling:\n  useMaxWidth: false', model));
add('config-padding-string', withConfig('eventmodeling:\n  padding: "12"', small));
add('config-eventmodeling-null', withConfig('eventmodeling: null', small));
add('config-eventmodeling-false', withConfig('eventmodeling: false', small));
add('config-eventmodeling-empty', withConfig('eventmodeling: {}', small));
add('config-font', withConfig('fontFamily: courier\nfontSize: 20', model));
add('config-security-loose', withConfig('securityLevel: loose', em('tf 01 cmd A { <b>bold</b> <i>it</i> }\ntf 02 evt B [[D]]\ndata D\n{\n  <u>u</u> <script>x</script>\n}')));
add('config-html-labels-false', withConfig('htmlLabels: false', model));
add('config-look-handdrawn', withConfig('look: handDrawn', small));
add('directive', '%%{init: {"theme": "dark", "eventmodeling": {"padding": 8}}}%%\n' + small);

renderExtras(cases, 'internal/diagrams/eventmodeling/testdata/render.json.gz');
