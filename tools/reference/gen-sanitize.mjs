// Generates the goldens of internal/sanitize from mermaid 12.2.0 running in
// Chromium (oracle.mjs eval, one browser run). The page script loads the ESM
// build of mermaid and calls the real functions of common.ts, utils.ts and
// @braintree/sanitize-url. No function body is copied.
//
//   internal/sanitize/testdata/sanitize_text.json.gz  sanitizeText
//   internal/sanitize/testdata/helpers.json.gz        the other helpers
//   internal/sanitize/testdata/urls.json.gz           sanitizeUrl, formatUrl, new URL
//
// A result is the JSON value that the function returned, {"throw": message}
// when it threw, and {"value": text} for undefined, NaN and the infinities.
//
// Usage: node gen-sanitize.mjs
import path from 'node:path';
import { ROOT, mulberry32, runJobs, writeGolden } from './purify-oracle-lib.mjs';
import { corpusLabels, generated, handWritten, unique } from './purify-hostile.mjs';

const TESTDATA = path.join(ROOT, 'internal/sanitize/testdata');
const NBSP = String.fromCharCode(0xa0);
const LS = String.fromCharCode(0x2028);
const ZWSP = String.fromCharCode(0x200b);
const BOM = String.fromCharCode(0xfeff);

// ---- sanitizeText ----

const labelTexts = unique([
  '',
  'plain',
  'a<br>b',
  'a<br/>b',
  'a<br />b',
  'a</br>b',
  'a<BR>b<Br/>c<bR  />d<br\t/>e<br\n>f',
  'a<br' + NBSP + '/>b<br' + LS + '>c',
  'a#br#b',
  'a = b',
  'x<y>z',
  '1 < 2 > 0 = false',
  '#quot;quoted#quot;',
  '#35; #amp; #lt;b#gt;',
  'a &amp; b &lt;c&gt; &equals;',
  '<a href="https://mermaid.js.org" target="_blank">link</a>',
  '<a href="x" target="_self">self</a>',
  '<a target="_blank" href="x" rel="nofollow">rel</a>',
  '<a href="javascript:alert(1)" target="_blank">js</a>',
  '<A HREF="x" TARGET="_blank">upper</A>',
  '<svg><a href="x" target="_blank">svg link</a></svg>',
  '<span target="_blank">not a link</span>',
  '<a href="x" target="_blank" data-temp-href-target="_top">both</a>',
  '<a data-temp-href-target="_blank">temp only</a>',
  '**bold** and _italic_ and `code`',
  '`markdown **string**`',
  '# heading\n- item\n- item',
  '[link](https://example.com) ![img](x.png)',
  '<b>**mixed**</b> <i>_md_</i>',
  '> quote\n\n1. one\n2. two',
  '<style>.x{color:red}</style>styled',
  'a<style>b</style>c<STYLE>d</STYLE>e',
  '<svg><style>x{}</style><text>t</text></svg>',
  '<script>alert(1)</script>after',
  '<img src=x onerror=alert(1)>',
  '<div class="a" style="color:red" onclick="x">d</div>',
  '<p>one</p><p>two</p>',
  ' leading and trailing ',
  '\n<b>x</b>\n',
  '$$x^2 < y$$',
  'fa:fa-car <i class="fa fa-car"></i>',
  ...corpusLabels().filter((s) => /[<>=&#]/.test(s)),
  ...corpusLabels().filter((_, i) => i % 9 === 0),
  ...handWritten,
  ...generated(450, 0x7374786c),
]);

const htmlLabelVariants = [
  {},
  { htmlLabels: true },
  { htmlLabels: false },
  { htmlLabels: 'false' },
  { htmlLabels: ' FALSE ' },
  { htmlLabels: 'null' },
  { htmlLabels: 0 },
  { htmlLabels: '0' },
  { htmlLabels: 'no' },
  { htmlLabels: null },
  { flowchart: { htmlLabels: false } },
  { flowchart: { htmlLabels: true } },
  { htmlLabels: true, flowchart: { htmlLabels: false } },
  { htmlLabels: false, flowchart: { htmlLabels: true } },
  { htmlLabels: null, flowchart: { htmlLabels: 'false' } },
  { flowchart: {} },
];
const securityLevels = ['strict', 'loose', 'antiscript', 'sandbox', undefined, 'other', ''];
const purifyConfigs = [
  undefined,
  { FORBID_TAGS: ['b'] },
  { ADD_ATTR: ['target'] },
  { ALLOWED_TAGS: ['b', 'i', 'br'] },
  { USE_PROFILES: { html: true } },
  { KEEP_CONTENT: false },
  {},
  false,
  null,
  0,
  '',
  true,
  'text',
  1,
  [],
  { RETURN_DOM: true },
  { RETURN_DOM_FRAGMENT: true },
  { RETURN_DOM: true, WHOLE_DOCUMENT: true },
  { TRUSTED_TYPES_POLICY: {} },
];

// The main set: every security level and htmlLabels variant, with no
// dompurifyConfig, for every text.
const mainConfigs = [];
for (const level of securityLevels) {
  for (const labels of htmlLabelVariants) {
    const config = { ...labels };
    if (level !== undefined) {
      config.securityLevel = level;
    }
    mainConfigs.push(config);
  }
}
// The dompurifyConfig set: a smaller list of texts.
const purifyConfigCases = [];
for (const level of ['strict', 'loose', 'other']) {
  for (const labels of [{}, { htmlLabels: false }]) {
    for (const dompurifyConfig of purifyConfigs) {
      purifyConfigCases.push({ ...labels, securityLevel: level, dompurifyConfig });
    }
  }
}
const smallTexts = labelTexts.filter((_, i) => i < 60 || i % 11 === 0);

// triggers reports whether sanitizeText with the config calls removeScript,
// which installs the hooks. It only selects which cases run on a new module
// instance. The expected results come from the real functions.
const triggers = (config) => {
  const v = config.htmlLabels ?? config.flowchart?.htmlLabels ?? true;
  const on = !(v === false || ['false', 'null', '0'].includes(String(v).trim().toLowerCase()));
  return on && ['antiscript', 'strict', 'sandbox'].includes(config.securityLevel);
};

const rnd = mulberry32(0x66726573);
const groups = [];
// Only the result of getEffectiveHtmlLabels matters to sanitizeText. So the
// two plain variants run for every text, and the others for the small list.
const isPlainVariant = (config) => {
  const keys = Object.keys(config).filter((k) => k !== 'securityLevel');
  return keys.length === 0 || (keys.length === 1 && config.htmlLabels === false);
};
for (const config of mainConfigs) {
  groups.push({ config, texts: isPlainVariant(config) ? labelTexts : smallTexts });
}
for (const config of purifyConfigCases) {
  groups.push({ config, texts: smallTexts });
}
// Each group runs with the hooks installed. A group whose config does not
// install the hooks also runs on an instance that never had them. For a
// group that installs them, a sample of the texts runs on a new instance for
// each call: the state of a page whose first call it is.
for (const g of groups) {
  if (triggers(g.config)) {
    g.fresh = g.texts.map((_, i) => i).filter(() => rnd() < 0.004);
  }
}

// ---- helpers ----

const breakTexts = unique([
  ...labelTexts.filter((s) => /<\/?br/i.test(s) || s.includes('\\n') || s.includes('#br#')),
  'a\\nb\\nc',
  '\\n',
  'a<br>b\\nc#br#d',
  '<br><br>',
  '<br',
  'br>',
  '<br/ >',
  '< br>',
  '<br//>',
  '</ br>',
  '<br>'.repeat(5),
  ...labelTexts.filter((_, i) => i % 15 === 0),
]);
const genericTexts = [
  '',
  '~',
  '~~',
  '~~~',
  'Array~string~',
  'Array~Array~string~~',
  'Map~K, V~',
  'Map~K,V~',
  'Map~K, List~V~~',
  'Map~K,List~V~~',
  'a~b~c~d',
  'a~b~c~d~',
  '~a~b~',
  '~T~ x',
  'List~int~ ~',
  'x, y',
  ',',
  ',,',
  '~,~',
  '~a,b~,~c,d~',
  'f(a~b~, c~d~)',
  'Pair~Map~K,V~,List~T~~',
  'emoji~' + String.fromCodePoint(0x1f600) + '~',
  'a~' + NBSP + '~b',
  ...corpusLabels().filter((s) => s.includes('~')),
];
const occurrenceCalls = [
  ['', ''],
  ['abc', ''],
  ['a' + String.fromCodePoint(0x1f600) + 'b', ''],
  ['', '~'],
  ['~', '~'],
  ['a~b~c', '~'],
  ['aaaa', 'aa'],
  ['aaaaa', 'aa'],
  ['abcabc', 'bc'],
  ['abc', 'abcd'],
  ['a.b.c', '.'],
];
const katexTexts = [
  '',
  '$',
  '$$',
  '$$$$',
  '$$x$$',
  'a $$x$$ b $$y$$',
  '$$x\ny$$',
  '$$x' + LS + 'y$$',
  '$x$',
  '$$x$',
  '$$$x$$$',
  'cost: $5 and $$6',
  ...labelTexts.filter((s) => s.includes('$')),
];
const evaluateValues = [
  true, false, null, 'true', 'false', 'null', '0', 0, 1, '', ' false ', 'FALSE', 'Null', ' 0 ',
  '0.0', 'no', 'off', [], [false], ['false'], [0], {}, 'False' + NBSP, NBSP + 'null', '\tfalse\n',
  -0, 'undefined',
];
const entityTexts = unique([
  '',
  '#quot;',
  'A["#quot;hi#quot;"]',
  '#35;',
  '#+35;',
  '#9829; #x; #a_b; #;',
  'style A fill:#f9f,stroke:#333;',
  'style A fill:#f9f;stroke:#333; B',
  'classDef x fill:#fff;',
  'classDef x fill: #fff;',
  'style x color:#000;\nclassDef y fill:#f00;stroke:#0f0;',
  'style A fill:#f9f' + LS + ';',
  'linkStyle 0 stroke:#f00;',
  'a:#b;c',
  'style: #a;',
  'style:' + NBSP + '#a;',
  '##a;;',
  '#a;#b;#1;#+2;#-3;',
  String.fromCharCode(0xfb02, 0xb0, 0xb0) + '35' + String.fromCharCode(0xb6, 0xdf),
  String.fromCharCode(0xfb02, 0xb0) + 'quot' + String.fromCharCode(0xb6, 0xdf),
  String.fromCharCode(0xfb02, 0xb0, 0xb0, 0xb0) + String.fromCharCode(0xb6, 0xdf, 0xb6, 0xdf),
  ...corpusLabels().filter((s) => s.includes('#')),
  ...labelTexts.filter((_, i) => i % 13 === 0),
]);
const cssEscapeTexts = [
  '', 'a', '-', '--', '-a', '-1', '1', '12', 'a1', '_', 'a b', 'a.b', 'a#b', 'about://blank',
  'http://localhost:8080/a/b?q=1&r=2', String.fromCharCode(0) + 'a', String.fromCharCode(1, 0x1f, 0x7f) + 'x',
  String.fromCharCode(0xe9) + String.fromCodePoint(0x1f600), 'A-Z_0-9', '-' + String.fromCharCode(0) , '0', '-0',
  'a\nb', '~!@$%^&*()+=,/;:<>?[]{}|`\'"\\',
];

// ---- URLs ----

function urlInputs() {
  const hand = [
    '', ' ', 'about:blank', 'https://mermaid.js.org', 'https://mermaid.js.org/', 'http://example.com',
    'HTTP://EXAMPLE.COM/Path?Q=1#Frag', 'https://example.com/a b', 'https://example.com/a%20b',
    'https://example.com/%7Euser', 'https://example.com/a/../b/./c', 'https://example.com/a/%2e%2e/b',
    'https://example.com/..', 'https://example.com/.', 'https://example.com/a/..', 'https://example.com/a/.',
    'https://example.com//a//b', 'https://example.com/?', 'https://example.com/#', 'https://example.com/?#',
    'https://example.com?a=b&c=d', "https://example.com?a='b'", 'https://example.com#a b', 'https://example.com/a?b#c?d#e',
    'https://example.com/a"b<c>d`e{f}g|h^i', 'https://example.com/?a"b<c>d`e{f}g|h^i', 'https://example.com/#a"b<c>d`e{f}g|h^i',
    'https://example.com/' + String.fromCharCode(0xe9) + '?' + String.fromCharCode(0xe9) + '#' + String.fromCharCode(0xe9),
    'https://example.com/' + String.fromCodePoint(0x1f600), 'https://user@example.com', 'https://user@example.com/',
    'https://us er@example.com/', 'https://u@v@example.com/', 'https://@example.com/', 'https://example.com./',
    'https://EXAMPLE.com', 'https://ex_ample.com', 'https://exa mple.com', 'https://exa%mple.com', 'https://exa<mple.com',
    'https://example..com', 'https://.example.com', 'https://-example.com', 'https://xn--bcher-kva.example',
    'https://XN--BCHER-KVA.example', 'https://xn--a.example', 'https://xn--.example', 'https://b' + String.fromCharCode(0xfc) + 'cher.example',
    'https://B' + String.fromCharCode(0xdc) + 'CHER.example', 'https://fa' + String.fromCharCode(0xdf) + '.example',
    'https://' + String.fromCharCode(0x4f8b, 0x3048) + '.jp', 'https://example' + String.fromCharCode(0x3002) + 'com',
    'https://' + String.fromCharCode(0xff45, 0xff58) + 'ample.com', 'https://a' + String.fromCharCode(0xad) + 'b.com',
    'https://' + String.fromCodePoint(0x1f600) + '.com', 'https://a' + String.fromCharCode(0x200d) + 'b.com',
    'https://' + String.fromCharCode(0x5d0) + 'a.com', 'https://a.' + String.fromCharCode(0x5d0, 0x5d1) + '.com',
    'https://127.0.0.1', 'https://127.1', 'https://0x7f.1', 'https://017700000001', 'https://2130706433', 'https://1.2.3.4.5',
    'https://256.1.1.1', 'https://1.2.3.256', 'https://1.2.3', 'https://1..2', 'https://0x.0x.0', 'https://1.2.3.4.',
    'https://09', 'https://1.2.3.08', 'https://a.1', 'https://a.0x1g', 'https://4294967296', 'https://0xffffffff',
    'https://example.com/path/to/page.html?query=string&other=1#section-2', 'http:example.com', 'http:/example.com',
    'http:///example.com', 'http:\\\\example.com\\a\\b', 'http:', 'http:/', 'http://', 'http:///', 'https:?q', 'https:#f',
    'http://?q', 'http:a/b', 'https:a?b#c', 'HtTpS:Example.COM', ' https://example.com ', 'https://example.com ',
    'ftp://example.com', 'ftp:example.com', 'ws://example.com', 'file:///etc/passwd', 'mailto:a@b.c', 'MAILTO:a@b.c',
    'mailto:a@b.c?subject=x', 'tel:+123', 'sms:123', 'custom://deep/link', 'custom:thing', 'a:b:c', 'a:b://c',
    'www.example.com', 'example.com/path', 'example.com:8080/path', 'localhost:3000', '/relative/path', './relative', '../up',
    '.hidden', '//protocol-relative.com/x', '#anchor', '?query', 'path with spaces', 'path/with:colon', 'x&colon;y',
    'javascript&colon;alert(1)', 'javascript:alert(1)', 'JAVASCRIPT:alert(1)', 'JaVaScRiPt:alert(1)', ' javascript:alert(1)',
    'java\nscript:alert(1)', 'java\tscript:alert(1)', 'java\rscript:alert(1)', 'java' + String.fromCharCode(0) + 'script:alert(1)',
    'javascript' + ZWSP + ':alert(1)', BOM + 'javascript:alert(1)', 'java&#115;cript:alert(1)', 'java&#x73;cript:alert(1)',
    '&#106;&#97;&#118;&#97;&#115;&#99;&#114;&#105;&#112;&#116;&#58;alert(1)', '&#106avascript:alert(1)',
    '&#0000106&#0000097&#0000118&#0000097&#0000115&#0000099&#0000114&#0000105&#0000112&#0000116&#0000058alert(1)',
    '&#x6A&#x61&#x76&#x61&#x73&#x63&#x72&#x69&#x70&#x74&#x3A&#x61&#x6C&#x65&#x72&#x74&#x28&#x27&#x58&#x53&#x53&#x27&#x29',
    'java&Tab;script:alert(1)', 'java&NewLine;script:alert(1)', 'java&tab;script&newline;:alert(1)', 'jav&#x09;ascript:alert(1)',
    'javascript%3Aalert(1)', 'java%0ascript:alert(1)', 'java%09script:alert(1)', '%6A%61%76%61%73%63%72%69%70%74%3Aalert(1)',
    'java\\nscript:alert(1)', 'java\\tscript:alert(1)', 'java\\rscript:alert(1)', 'java%5cnscript:alert(1)', 'java%5C%6escript:alert(1)',
    'java%5c%72script:x', 'java%5c%74script:x', '\\njavascript:x', 'data:text/html,<b>', 'DATA:text/html;base64,AAAA',
    ' data:x', 'vbscript:msgbox(1)', 'VBSCRIPT:x', '!!!javascript:x', '(javascript:x', '..javascript:x', '/javascript:x',
    '_javascript:x', '1javascript:x', 'xjavascript:x', 'java script:x', 'javascripts:x', 'javascript', 'javascript:', 'datafoo:x',
    'x:javascript:y', 'x-javascript:y', 'https://javascript:x', 'not_javascript:alert(1)', 'http://x/javascript:y',
    'http://x/a:b', 'http://x:80/', 'http://x:8080/', 'https://x:443/a', 'https://x:0443/a', 'https://x:65535', 'https://x:65536',
    'https://x:/a', 'https://x:8a/', 'https://[::1]/', 'https://[::1]:8080/', 'https://[2001:db8::1]/', 'https://[::ffff:1.2.3.4]/',
    'https://[1:2:3:4:5:6:7:8]/', 'https://[1::8]/', 'https://[::]/', 'https://[1:2:3:4:5:6:7::]/', 'https://[1:0:0:2:0:0:0:3]/',
    'https://[::1', 'https://[:1]/', 'https://[1:2]/', 'https://[g::1]/', 'https://[::1.2.3]/', 'https://[1::2::3]/',
    'https://%65xample.com/', 'https://example.com/%zz', 'https://example.com/%', 'https://example.com/%e9', 'https://example.com/%C3%A9',
    'https://example.com/%ff%fe', '%', '%zz', '%e9', 'a%20b', 'a%2520b', 'a%252520b', '%25%32%35', 'x%00y', 'x%0', '&#', '&#;', '&#x;',
    '&#1;', '&#65;', '&#65', '&#0x41;', '&#x41;', '&#1e2;', '&#0b11;', '&#0o17;', '&#65601;', '&#128512;', '&#55357;&#56832;', '&#55357;',
    '&#Infinity;', '&#_;', '&#65a;', '&#65;&#66;&#67;', '&#38;#65;', '&amp;#65;', '&#x26;#x41;', '&#0x' + 'f'.repeat(20) + ';',
    'a&#58;b', 'a&#x3a;//b', 'http&#58;//example.com', 'http&colon;//example.com', 'HTTP&COLON;//example.com',
    'https://example.com/a&#47;b', 'line1' + LS + 'line2:x', 'line1' + LS + 'http://example.com', LS + 'javascript:x',
    'foo' + LS + 'javascript:x', 'http://a' + LS + 'b/', 'https://example.com/' + LS + 'x:y', 'a' + NBSP + ':b', NBSP + 'https://example.com',
    'https://example.com/' + NBSP, 'https://ex' + ZWSP + 'ample.com', 'ht' + String.fromCharCode(0x80) + 'tp://x', 'http' + String.fromCharCode(0x7f) + '://x',
    'https://example.com/a\\b\\c', 'https:\\\\example.com', '\\\\example.com\\share', 'C:\\Users\\x', 'c:/x', 'a\\b:c',
    'https://example.com:8080/a?b=c#d', 'https://example.com/a?x=1:2', 'https://example.com/a#b:c', 'https://example.com/a;b=c',
    'https://example.com/~user/', "https://example.com/a'b", 'https://example.com/a(b)c*d!e$f,g', 'https://example.com/[a]',
    'https://user:pass@example.com/', 'https://exam[ple.com/', 'https://exam]ple.com/', 'https://exam|ple.com/', 'https://exam^ple.com/',
    'https://exam@ple.com/', 'https://exa#mple.com/', 'https://exa?mple.com/', 'https://a,b.com/', 'https://a!b.com/', "https://a'b.com/",
    'https://a(b).com/', 'https://a*b.com/', 'https://a+b.com/', 'https://a;b.com/', 'https://a=b.com/', 'https://a~b.com/', 'https://a$b.com/',
    'https://a&b.com/', 'https://a{b}.com/', 'https://a`b.com/', 'https://a"b.com/', 'https://x' + 'y'.repeat(300) + '.com/',
    'https://' + 'a.'.repeat(130) + 'com/', 'https://www.xn--80ak6aa92e.com/', 'https://xn--ls8h.la/', 'https://xn--zca.example/',
  ];
  const rndUrl = mulberry32(0x75726c73);
  const pick = (list) => list[Math.floor(rndUrl() * list.length)];
  const schemes = ['http:', 'https:', 'HTTP:', 'Https:', 'ftp:', 'mailto:', 'javascript:', 'data:', 'custom:', 'x-y.z+w:', '', '', ''];
  const slashes = ['//', '//', '//', '/', '', '///', '\\\\', '/\\'];
  const hosts = [
    'example.com', 'EXAMPLE.org', 'sub.example.co.uk', 'localhost', '127.0.0.1', '0x7f.0.0.1', '1.2.3', 'a_b.example', 'xn--bcher-kva.de',
    'b' + String.fromCharCode(0xfc) + 'cher.de', String.fromCharCode(0x4e2d, 0x6587) + '.cn', 'a b.com', 'a%41b.com', 'a..b', 'exa\tmple.com',
    'user@example.com', 'u:p@example.com', '', '.', 'a.', '999', '1.1.1.1.1', 'x-y.z',
  ];
  const ports = ['', '', '', '', ':80', ':443', ':8080', ':', ':99999', ':abc', ':080'];
  const paths = [
    '', '/', '/a', '/a/b/c', '/a/../b', '/./a', '/a b', '/a%20b', '/a%2Fb', '/%2e%2e/x', '/a//b', '/a\\b', '/' + String.fromCharCode(0xe9),
    '/a;b', '/a:b', '/a|b', '/a"b', '/<a>', '/{a}', '/a^b', '/`', '/~a', "/a'b", '/a&amp;b', '/a&#x2f;b', '/%', '/%zz', '/..', '/.',
  ];
  const queries = ['', '', '?', '?a=b', '?a=b&c=d', "?q='x'", '?q="x"', '?q=<x>', '?q=a b', '?q=%20', '?q=' + String.fromCharCode(0xe9), '?a#b', '?a:b', '?a=b?c'];
  const fragments = ['', '', '#', '#a', '#a b', '#a"b', '#<a>', '#a`b', '#a#b', '#a:b', '#' + String.fromCharCode(0xe9), '#a{b}'];
  const noise = ['', '', '', '', ' ', '\t', '\n', NBSP, ZWSP, '&#9;', '&Tab;', '%0a', '\\n', '&#x0A;', String.fromCharCode(0), BOM, '%09'];
  const out = [...hand];
  for (let i = 0; i < 1400; i++) {
    let s = pick(noise) + pick(schemes);
    if (rndUrl() < 0.15) {
      // Put noise inside the scheme.
      const at = Math.floor(rndUrl() * (s.length + 1));
      s = s.slice(0, at) + pick(noise) + s.slice(at);
    }
    s += pick(slashes) + pick(hosts) + pick(ports) + pick(paths) + pick(queries) + pick(fragments) + pick(noise);
    out.push(s);
  }
  return unique(out);
}

const urls = urlInputs();
const formatUrlCalls = [];
for (const url of urls.filter((_, i) => i % 4 === 0)) {
  for (const config of [{ securityLevel: 'strict' }, { securityLevel: 'loose' }, {}, { securityLevel: 'sandbox' }]) {
    formatUrlCalls.push([url, config]);
  }
}
formatUrlCalls.push([' ', {}], ['  x  ', { securityLevel: 'loose' }], ['', { securityLevel: 'loose' }]);
// new URL() is the part of sanitizeUrl that needs a URL parser. It is also
// recorded alone, for inputs that sanitizeUrl would not pass to it unchanged.
// Every ASCII character in each component of a URL. Chromium does not follow
// the URL Standard for all of them: it escapes some characters of a host
// that the standard rejects or keeps.
const componentProbes = [];
for (let c = 0x20; c < 0x7f; c++) {
  const ch = String.fromCharCode(c);
  componentProbes.push(
    'https://a' + ch + 'b.test/',
    'https://h.test/a' + ch + 'b',
    'https://h.test/?a' + ch + 'b',
    'https://h.test/#a' + ch + 'b',
    'https://a' + ch + 'b@h.test/'
  );
}
for (let c = 0; c < 0x80; c++) {
  componentProbes.push('https://a%' + c.toString(16).padStart(2, '0') + 'b.test/');
}
const urlHrefInputs = unique([
  ...urls.filter((s) => /^\s*https?:/i.test(s)),
  ...urls.filter((_, i) => i % 3 === 0),
  ...componentProbes,
]);

// ---- jobs ----

const jobs = [{ kind: 'env' }];
const add = (job) => jobs.push(job) - 1;

// Jobs that cannot install the hooks run first on the instance without hooks.
const cleanGroups = groups.filter((g) => !triggers(g.config));
const jClean = add({
  kind: 'sanitizeText',
  hooks: false,
  cases: cleanGroups.flatMap((g) => g.texts.map((t) => [t, g.config])),
});
const jOn = add({
  kind: 'sanitizeText',
  hooks: true,
  cases: groups.flatMap((g) => g.texts.map((t) => [t, g.config])),
});
const freshCases = groups.flatMap((g) => (g.fresh || []).map((i) => [g.texts[i], g.config]));
const jFresh = add({ kind: 'sanitizeTextFresh', cases: freshCases });

const helperJobs = [];
const helper = (name, calls, hooks = false) => {
  helperJobs.push({ name, calls, job: add({ kind: 'fn', name, calls, hooks }) });
};
helper('getRows', [...breakTexts.map((t) => [t]), []]);
helper('hasBreaks', breakTexts.map((t) => [t]));
helper('splitBreaks', breakTexts.map((t) => [t]));
helper('breakToPlaceholder', breakTexts.map((t) => [t]));
helper('placeholderToBreak', breakTexts.map((t) => [t]));
helper('parseGenericTypes', genericTexts.map((t) => [t]));
helper('processSet', genericTexts.map((t) => [t]));
helper('countOccurrence', occurrenceCalls);
helper(
  'shouldCombineSets',
  genericTexts.slice(0, 14).flatMap((a) => genericTexts.slice(0, 14).map((b) => [a, b]))
);
helper('hasKatex', katexTexts.map((t) => [t]));
helper('evaluate', [...evaluateValues.map((v) => [v]), []]);
helper('getEffectiveHtmlLabels', mainConfigs.map((c) => [c]));
// A number that JSON cannot hold is passed as a string. The page script
// converts every argument with Number().
helper('getMax', [[], [1], [1, 2, 3], [3, 'NaN', 5], ['NaN'], ['NaN', 'NaN'], [-1, '-Infinity'], ['Infinity', 1], [-5, -7]]);
helper('getMin', [[], [1], [1, 2, 3], [3, 'NaN', 5], ['NaN'], ['NaN', 'NaN'], [1, 'Infinity'], ['-Infinity', 1], [-5, -7]]);
helper('getUrl', [[true], [false]]);
helper('cssEscape', cssEscapeTexts.map((t) => [t]));
helper('encodeEntities', entityTexts.map((t) => [t]));
helper('decodeEntities', entityTexts.map((t) => [t]));
// The functions below install the hooks, so they run on the instance that
// has them. Their results do not depend on the state before the call.
helper('removeScript', smallTexts.map((t) => [t]), true);
helper('sanitizeMore', mainConfigs.flatMap((c) => smallTexts.filter((_, i) => i % 3 === 0).map((t) => [t, c])), true);
helper(
  'sanitizeTextOrArray',
  [
    ['a<b>b</b>', { securityLevel: 'strict' }],
    [['a<script>x</script>', '<b>c</b>'], { securityLevel: 'strict' }],
    [[['a', '<i>b</i>'], ['<style>c</style>d']], { securityLevel: 'loose' }],
    [[], {}],
    [['x', ['y', '<b>z</b>'], 'w<br>'], { securityLevel: 'other' }],
  ],
  true
);
// The tree that DOMParser builds for every text that has markup. The tests
// use it to tell a difference of the HTML parser from a difference of the
// sanitizer.
const parseInputs = labelTexts.filter((t) => t.includes('<'));
const jParse = add({ kind: 'parse', inputs: parseInputs });
const jSanitizeUrl = add({ kind: 'fn', name: 'sanitizeUrl', calls: urls.map((u) => [u]) });
const jUrlHref = add({ kind: 'fn', name: 'urlHref', calls: urlHrefInputs.map((u) => [u]) });
const jFormatUrl = add({ kind: 'fn', name: 'formatUrl', calls: formatUrlCalls });

const { ua, results } = runJobs(jobs);
const chrome = /HeadlessChrome\/[\d.]+/.exec(ua)[0];
const meta = { mermaid: '12.2.0', browser: chrome };

// ---- goldens ----

{
  // Results by group. off is null where the config installs the hooks: the
  // result on a new page is then the result with hooks.
  let cleanAt = 0;
  let onAt = 0;
  let freshAt = 0;
  const out = groups.map((g) => {
    const n = g.texts.length;
    const entry = { config: g.config, texts: g.texts === labelTexts ? 'all' : 'small' };
    entry.on = results[jOn].slice(onAt, onAt + n);
    onAt += n;
    if (!triggers(g.config)) {
      const off = results[jClean].slice(cleanAt, cleanAt + n);
      cleanAt += n;
      // Store only the differences from the result with hooks.
      entry.off = off.map((v, i) => (JSON.stringify(v) === JSON.stringify(entry.on[i]) ? null : v));
    } else {
      entry.fresh = (g.fresh || []).map((i) => [i, results[jFresh][freshAt++]]);
    }
    // Most results equal the input.
    entry.on = entry.on.map((v, i) => (v === g.texts[i] ? 0 : v));
    return entry;
  });
  writeGolden(path.join(TESTDATA, 'sanitize_text.json.gz'), {
    ...meta,
    location: results[0].location,
    texts: { all: labelTexts, small: smallTexts },
    groups: out,
    parse: { inputs: parseInputs, trees: results[jParse] },
  });
  console.log(
    `sanitizeText: ${results[jOn].length} cases with hooks, ${results[jClean].length} without, ${freshCases.length} on a new instance`
  );
}

writeGolden(path.join(TESTDATA, 'helpers.json.gz'), {
  ...meta,
  location: results[0].location,
  functions: helperJobs.map((h) => ({ name: h.name, calls: h.calls, out: results[h.job] })),
});
console.log('helpers: ' + helperJobs.map((h) => `${h.name} ${h.calls.length}`).join(', '));

writeGolden(path.join(TESTDATA, 'urls.json.gz'), {
  ...meta,
  sanitizeUrl: { inputs: urls, out: results[jSanitizeUrl] },
  urlHref: { inputs: urlHrefInputs, out: results[jUrlHref] },
  formatUrl: { calls: formatUrlCalls, out: results[jFormatUrl] },
});
console.log(`urls: sanitizeUrl ${urls.length}, new URL ${urlHrefInputs.length}, formatUrl ${formatUrlCalls.length}`);
