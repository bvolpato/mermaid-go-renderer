// Test inputs for gen-marked.mjs and gen-markdown.mjs: label-like strings
// from the fixture corpus, examples that follow the sections of the
// CommonMark and GFM specifications, and strings from a seeded generator.
import fs from 'node:fs';
import path from 'node:path';

const cp = (...points) => String.fromCodePoint(...points);

// Characters that are written as numbers, so that this file stays ASCII.
const NBSP = cp(0xa0);
const LS = cp(0x2028);
const PS = cp(0x2029);
const IDEOGRAPHIC_SPACE = cp(0x3000);
const BOM = cp(0xfeff);
const ZWJ = cp(0x200d);
const COMBINING_ACUTE = cp(0x301);
const E_ACUTE = cp(0xe9);
const CJK = cp(0x4e2d, 0x6587);
const KANA = cp(0x65e5, 0x672c, 0x8a9e, 0x306e, 0x30c6, 0x30ad, 0x30b9, 0x30c8);
const GRIN = cp(0x1f600);
const THUMB_TONE = cp(0x1f44d, 0x1f3fd);
const FAMILY = cp(0x1f468, 0x200d, 0x1f469, 0x200d, 0x1f467);
const FLAG_BR = cp(0x1f1e7, 0x1f1f7);
const HEART = cp(0x2764, 0xfe0f);
const KEYCAP = '1' + cp(0xfe0f, 0x20e3);
const DOTTED_I = cp(0x130);
const DOTLESS_I = cp(0x131);
const SIGMA = cp(0x3a3);
const SHARP_S = cp(0xdf);
const KELVIN = cp(0x212a);
const LONG_S = cp(0x17f);
const MATH_A = cp(0x1d49c);
const ARABIC = cp(0x645, 0x631, 0x62d, 0x628, 0x627);
const DEVANAGARI = cp(0x915, 0x94d, 0x937, 0x93f);
const HANGUL = cp(0x1112, 0x1161, 0x11ab);
const THAI = cp(0x0e01, 0x0e33);
const GUILLEMETS = cp(0xab) + 'x' + cp(0xbb);
const CURRENCY = cp(0x20ac);
const EM_DASH = cp(0x2014);
const NEW_EMOJI = cp(0x1fae9); // Unicode 16
const HIGH = cp(0x1f600)[0];
const LOW = cp(0x1f600)[1];

// mulberry32 is a small seeded generator. The golden files must not change
// between runs.
export const mulberry32 = (seed) => () => {
  seed |= 0;
  seed = (seed + 0x6d2b79f5) | 0;
  let t = Math.imul(seed ^ (seed >>> 15), 1 | seed);
  t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
  return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
};

const walk = (dir, out = []) => {
  for (const entry of fs.readdirSync(dir, { withFileTypes: true }).sort((a, b) => (a.name < b.name ? -1 : 1))) {
    const full = path.join(dir, entry.name);
    if (entry.isDirectory()) walk(full, out);
    else if (entry.name.endsWith('.mmd')) out.push(full);
  }
  return out;
};

// corpusFiles returns the path of each fixture, relative to the repository.
export const corpusFiles = (repo) =>
  walk(path.join(repo, 'testdata/corpus/mermaid-e2e')).map((f) => path.relative(repo, f).split(path.sep).join('/'));

// corpusLabels returns the label-like strings of the fixtures: the text in
// double quotes, the markdown strings in backquotes, the text in brackets,
// braces, parentheses and pipes, the text after a colon, and each line.
export const corpusLabels = (repo) => {
  const seen = new Set();
  const add = (s) => {
    if (s !== undefined && s !== '' && s.length <= 2000) seen.add(s);
  };
  const patterns = [
    /"`([^`]*)`"/g, // markdown string, can have several lines
    /"([^"\n]*)"/g,
    /"([^"]*)"/g,
    /'([^'\n]*)'/g,
    /\[([^\[\]\n]*)\]/g,
    /\(([^()\n]*)\)/g,
    /\{([^{}\n]*)\}/g,
    /\|([^|\n]*)\|/g,
    /:[ \t]*([^\n]+)$/gm,
    /^[ \t]*([^\n]+?)[ \t]*$/gm, // each line without its indentation
    /^([^\n]+)$/gm, // each line
  ];
  for (const file of corpusFiles(repo)) {
    const text = fs.readFileSync(path.join(repo, file), 'utf8');
    for (const re of patterns) {
      for (const m of text.matchAll(re)) add(m[1]);
    }
  }
  return [...seen];
};

// specExamples returns markdown that follows the sections of the CommonMark
// 0.31 and GFM specifications. marked does not ship the specification tests.
export const specExamples = () => [
  // Tabs
  '\tfoo\tbaz\t\tbim',
  '  \tfoo\tbaz\t\tbim',
  '    a\ta\n    ' + cp(0x1f50) + '\ta',
  '  - foo\n\n\tbar',
  '- foo\n\n\t\tbar',
  '>\t\tfoo',
  '-\t\tfoo',
  '    foo\n\tbar',
  ' - foo\n   - bar\n\t - baz',
  '#\tFoo',
  '*\t*\t*\t',
  // Backslash escapes
  '\\!\\"\\#\\$\\%\\&\\\'\\(\\)\\*\\+\\,\\-\\.\\/\\:\\;\\<\\=\\>\\?\\@\\[\\\\\\]\\^\\_\\`\\{\\|\\}\\~',
  '\\\t\\A\\a\\ \\3\\' + cp(0x3c6) + '\\' + cp(0xab),
  '\\*not emphasized*\n\\<br/> not a tag\n\\[not a link](/foo)\n\\`not code`\n1\\. not a list\n\\* not a list\n\\# not a heading\n\\[foo]: /url "not a reference"\n\\&ouml; not a character entity',
  '\\\\*emphasis*',
  'foo\\\nbar',
  '`` \\[\\` ``',
  '    \\[\\]',
  '~~~\n\\[\\]\n~~~',
  '<https://example.com?find=\\*>',
  '<a href="/bar\\/)">',
  '[foo](/bar\\* "ti\\*tle")',
  '[foo]\n\n[foo]: /bar\\* "ti\\*tle"',
  '``` foo\\+bar\nfoo\n```',
  // Entity and numeric character references
  '&nbsp; &amp; &copy; &AElig; &Dcaron;\n&frac34; &HilbertSpace; &DifferentialD;\n&ClockwiseContourIntegral; &ngE;',
  '&#35; &#1234; &#992; &#0;',
  '&#X22; &#XD06; &#xcab;',
  '&nbsp &x; &#; &#x;\n&#87654321;\n&#abcdef0;\n&ThisIsNotDefined; &hi?;',
  '&copy',
  '<a href="&ouml;&ouml;.html">',
  '[foo](/f&ouml;&ouml; "f&ouml;&ouml;")',
  '[foo]\n\n[foo]: /f&ouml;&ouml; "f&ouml;&ouml;"',
  '``` f&ouml;&ouml;\nfoo\n```',
  '`f&ouml;&ouml;`',
  '    f&ouml;f&ouml;',
  '&#42;foo&#42;\n*foo*',
  '&#42; foo\n\n* foo',
  'foo&#10;&#10;bar',
  '&#9;foo',
  '[a](url &quot;tit&quot;)',
  "it&#39;s a 'quote' &#39;",
  // Precedence
  '- `one\n- two`',
  // Thematic breaks
  '***\n---\n___',
  '+++',
  '===',
  '--\n**\n__',
  ' ***\n  ***\n   ***',
  '    ***',
  'Foo\n    ***',
  '_____________________________________',
  ' - - -',
  ' **  * ** * ** * **',
  '-     -      -      -',
  '- - - -    ',
  '_ _ _ _ a\n\na------\n\n---a---',
  ' *-*',
  '- foo\n***\n- bar',
  'Foo\n***\nbar',
  'Foo\n---\nbar',
  '* Foo\n* * *\n* Bar',
  '- Foo\n- * * *',
  // ATX headings
  '# foo\n## foo\n### foo\n#### foo\n##### foo\n###### foo',
  '####### foo',
  '#5 bolt\n\n#hashtag',
  '\\## foo',
  '# foo *bar* \\*baz\\*',
  '#                  foo                     ',
  ' ### foo\n  ## foo\n   # foo',
  '    # foo',
  'foo\n    # bar',
  '## foo ##\n  ###   bar    ###',
  '# foo ##################################\n##### foo ##',
  '### foo ###     ',
  '### foo ### b',
  '# foo#',
  '### foo \\###\n## foo #\\##\n# foo \\#',
  '****\n## foo\n****',
  'Foo bar\n# baz\nBar foo',
  '## \n#\n### ###',
  '#',
  '# ',
  '#\n',
  // Setext headings
  'Foo *bar*\n=========\n\nFoo *bar*\n---------',
  'Foo *bar\nbaz*\n====',
  '  Foo *bar\nbaz*\t\n====',
  'Foo\n-------------------------\n\nFoo\n=',
  '   Foo\n---\n\n  Foo\n-----\n\n  Foo\n  ===',
  '    Foo\n    ---\n\n    Foo\n---',
  'Foo\n   ----      ',
  'Foo\n    ---',
  'Foo\n= =\n\nFoo\n--- -',
  'Foo  \n-----',
  'Foo\\\n----',
  '`Foo\n----\n`\n\n<a title="a lot\n---\nof dashes"/>',
  '> Foo\n---',
  '> foo\nbar\n===',
  '- Foo\n---',
  'Foo\nBar\n---',
  '---\nFoo\n---\nBar\n---\nBaz',
  '\n====',
  '---\n---',
  '- foo\n-----',
  '    foo\n---',
  '> foo\n-----',
  '\\> foo\n------',
  'Foo\n\nbar\n---\nbaz',
  'Foo\nbar\n\n---\n\nbaz',
  'Foo\nbar\n* * *\nbaz',
  'Foo\nbar\n\\---\nbaz',
  // Indented code blocks
  '    a simple\n      indented code block',
  '  - foo\n\n    bar',
  '1.  foo\n\n    - bar',
  '    <a/>\n    *hi*\n\n    - one',
  '    chunk1\n\n    chunk2\n  \n \n \n    chunk3',
  '    chunk1\n      \n      chunk2',
  'Foo\n    bar',
  '    foo\nbar',
  '# Heading\n    foo\nHeading\n------\n    foo\n----',
  '        foo\n    bar',
  '\n    \n    foo\n    ',
  '    foo  ',
  // Fenced code blocks
  '```\n<\n >\n```',
  '~~~\n<\n >\n~~~',
  '``\nfoo\n``',
  '```\naaa\n~~~\n```',
  '~~~\naaa\n```\n~~~',
  '````\naaa\n```\n``````',
  '~~~~\naaa\n~~~\n~~~~',
  '```',
  '`````\n\n```\naaa',
  '> ```\n> aaa\n\nbbb',
  '```\n\n  \n```',
  '```\n```',
  ' ```\n aaa\naaa\n```',
  '  ```\naaa\n  aaa\naaa\n  ```',
  '   ```\n   aaa\n    aaa\n  aaa\n   ```',
  '    ```\n    aaa\n    ```',
  '```\naaa\n  ```',
  '   ```\naaa\n  ```',
  '```\naaa\n    ```',
  '``` ```\naaa',
  '~~~~~~\naaa\n~~~ ~~',
  'foo\n```\nbar\n```\nbaz',
  'foo\n---\n~~~\nbar\n~~~\n# baz',
  '```ruby\ndef foo(x)\n  return 3\nend\n```',
  '~~~~    ruby startline=3 $%@#$\ndef foo(x)\n  return 3\nend\n~~~~~~~',
  '````;\n````',
  '``` aa ```\nfoo',
  '~~~ aa ``` ~~~\nfoo\n~~~',
  '```\n``` aaa\n```',
  '\t```js\n\tcode\n\t```',
  '  ```js\n  a\n b\n   c\n  ```',
  // HTML blocks
  '<table><tr><td>\n<pre>\n**Hello**,\n\n_world_.\n</pre>\n</td></tr></table>',
  '<table>\n  <tr>\n    <td>\n           hi\n    </td>\n  </tr>\n</table>\n\nokay.',
  ' <div>\n  *hello*\n         <foo><a>',
  '</div>\n*foo*',
  '<DIV CLASS="foo">\n\n*Markdown*\n\n</DIV>',
  '<div id="foo"\n  class="bar">\n</div>',
  '<div id="foo" class="bar\n  baz">\n</div>',
  '<div>\n*foo*\n\n*bar*',
  '<div id="foo"\n*hi*',
  '<div class\nfoo',
  '<div *???-&&&-<---\n*foo*',
  '<div><a href="bar">*foo*</a></div>',
  '<table><tr><td>\nfoo\n</td></tr></table>',
  '<div></div>\n``` c\nint x = 33;\n```',
  '<a href="foo">\n*bar*\n</a>',
  '<Warning>\n*bar*\n</Warning>',
  '<i class="foo">\n*bar*\n</i>',
  '</ins>\n*bar*',
  '<del>\n*foo*\n</del>',
  '<del>\n\n*foo*\n\n</del>',
  '<del>*foo*</del>',
  '<pre language="haskell"><code>\nimport Text.HTML.TagSoup\n\nmain :: IO ()\nmain = print $ parseTags tags\n</code></pre>\nokay',
  '<script type="text/javascript">\n// JavaScript example\n\ndocument.getElementById("demo").innerHTML = "Hello JavaScript!";\n</script>\nokay',
  '<textarea>\n\n*foo*\n\n_bar_\n\n</textarea>',
  '<style\n  type="text/css">\nh1 {color:red;}\n\np {color:blue;}\n</style>\nokay',
  '<style\n  type="text/css">\n\nfoo',
  '> <div>\n> foo\n\nbar',
  '- <div>\n- foo',
  '<style>p{color:red;}</style>\n*foo*',
  '<!-- foo -->*bar*\n*baz*',
  '<script>\nfoo\n</script>1. *bar*',
  '<!-- Foo\n\nbar\n   baz -->\nokay',
  '<?php\n\n  echo \'>\';\n\n?>\nokay',
  '<!DOCTYPE html>',
  '<![CDATA[\nfunction matchwo(a,b)\n{\n  if (a < b && a < 0) then {\n    return 1;\n\n  } else {\n\n    return 0;\n  }\n}\n]]>\nokay',
  '  <!-- foo -->\n\n    <!-- foo -->',
  '  <div>\n\n    <div>',
  'Foo\n<div>\nbar\n</div>',
  '<div>\nbar\n</div>\n*foo*',
  'Foo\n<a href="bar">\nbaz',
  '<div>\n\n*Emphasized* text.\n\n</div>',
  '<div>\n*Emphasized* text.\n</div>',
  '<table>\n\n<tr>\n\n<td>\nHi\n</td>\n\n</tr>\n\n</table>',
  '<table>\n\n  <tr>\n\n    <td>\n      Hi\n    </td>\n\n  </tr>\n\n</table>',
  '<SCRIPT>x</script>\ny',
  '<Pre>a</PRE>\nb',
  '<scr' + DOTTED_I + 'pt>x</script>\ny',
  '<script>x</scr' + DOTTED_I + 'pt>\ny\n\nz',
  '<' + KELVIN + 'bd>x</kbd>',
  '<' + LONG_S + 'cript>x</script>',
  '<!---->x',
  '<!--->x',
  '<!-->x',
  // Link reference definitions
  '[foo]: /url "title"\n\n[foo]',
  '   [foo]: \n      /url  \n           \'the title\'  \n\n[foo]',
  "[Foo*bar\\]]:my_(url) 'title (with parens)'\n\n[Foo*bar\\]]",
  "[Foo bar]:\n<my url>\n'title'\n\n[Foo bar]",
  "[foo]: /url '\ntitle\nline1\nline2\n'\n\n[foo]",
  "[foo]: /url 'title\n\nwith blank line'\n\n[foo]",
  '[foo]:\n/url\n\n[foo]',
  '[foo]:\n\n[foo]',
  '[foo]: <>\n\n[foo]',
  '[foo]: <bar>(baz)\n\n[foo]',
  '[foo]: /url\\bar\\*baz "foo\\"bar\\baz"\n\n[foo]',
  '[foo]\n\n[foo]: url',
  '[foo]\n\n[foo]: first\n[foo]: second',
  '[FOO]: /url\n\n[Foo]',
  '[' + cp(0x391, 0x393, 0x3a9) + ']: /' + cp(0x3c6, 0x3bf, 0x3c5) + '\n\n[' + cp(0x3b1, 0x3b3, 0x3c9) + ']',
  '[' + SIGMA + 'A' + SIGMA + ']: /sigma\n\n[' + SIGMA + 'A' + SIGMA + '] [' + cp(0x3c3) + 'a' + cp(0x3c2) + ']',
  '[' + DOTTED_I + ']: /dotted\n\n[' + DOTTED_I + '] [i' + cp(0x307) + '] [i]',
  '[' + SHARP_S + ']: /ss\n\n[' + SHARP_S + '] [SS] [' + cp(0x1e9e) + ']',
  '[foo]: /url',
  '[\nfoo\n]: /url\nbar',
  '[foo]: /url "title" ok',
  '[foo]: /url\n"title" ok',
  '    [foo]: /url "title"\n\n[foo]',
  '```\n[foo]: /url\n```\n\n[foo]',
  'Foo\n[bar]: /baz\n\n[bar]',
  '# [Foo]\n[foo]: /url\n> bar',
  '[foo]: /url\nbar\n===\n[foo]',
  '[foo]: /url\n===\n[foo]',
  '[foo]: /foo-url "foo"\n[bar]: /bar-url\n  "bar"\n[baz]: /baz-url\n\n[foo],\n[bar],\n[baz]',
  '[foo]\n\n> [foo]: /url',
  '[a   b\tc]: /url\n\n[A B C] [a b c] [a  b  c]',
  '[toString]: /proto\n\n[toString] [constructor] [__proto__]',
  // Paragraphs
  'aaa\n\nbbb',
  'aaa\nbbb\n\nccc\nddd',
  'aaa\n\n\nbbb',
  '  aaa\n bbb',
  'aaa\n             bbb\n                                       ccc',
  '   aaa\nbbb',
  '    aaa\nbbb',
  'aaa     \nbbb     ',
  // Blank lines
  '  \n\naaa\n  \n\n# aaa\n\n  ',
  '\n',
  '\n\n',
  ' ',
  '   \n   ',
  '\t',
  // Block quotes
  '> # Foo\n> bar\n> baz',
  '># Foo\n>bar\n> baz',
  '   > # Foo\n   > bar\n > baz',
  '    > # Foo\n    > bar\n    > baz',
  '> # Foo\n> bar\nbaz',
  '> bar\nbaz\n> foo',
  '> foo\n---',
  '> - foo\n- bar',
  '>     foo\n    bar',
  '> ```\nfoo\n```',
  '> foo\n    - bar',
  '>',
  '>\n>  \n> ',
  '>\n> foo\n>  ',
  '> foo\n\n> bar',
  '> foo\n> bar',
  '> foo\n>\n> bar',
  'foo\n> bar',
  '> aaa\n***\n> bbb',
  '> bar\nbaz',
  '> bar\n\nbaz',
  '> bar\n>\nbaz',
  '> > > foo\nbar',
  '>>> foo\n> bar\n>>baz',
  '>     code\n\n>    not code',
  '> > a\n> b\nc\n> > d',
  '> - a\n>   b\nc\n> - d',
  '> 1. a\n2. b\n> c',
  // List items
  'A paragraph\nwith two lines.\n\n    indented code\n\n> A block quote.',
  '1.  A paragraph\n    with two lines.\n\n        indented code\n\n    > A block quote.',
  '- one\n\n two',
  '- one\n\n  two',
  ' -    one\n\n     two',
  ' -    one\n\n      two',
  '   > > 1.  one\n>>\n>>     two',
  '>>- one\n>>\n  >  > two',
  '-one\n\n2.two',
  '- foo\n\n\n  bar',
  '1.  foo\n\n    ```\n    bar\n    ```\n\n    baz\n\n    > bam',
  '- Foo\n\n      bar\n\n\n      baz',
  '123456789. ok',
  '1234567890. not ok',
  '0. ok',
  '003. ok',
  '-1. not ok',
  '- foo\n\n      bar',
  '  10.  foo\n\n           bar',
  '    indented code\n\nparagraph\n\n    more code',
  '1.     indented code\n\n   paragraph\n\n       more code',
  '1.      indented code\n\n   paragraph\n\n       more code',
  '   foo\n\nbar',
  '-    foo\n\n  bar',
  '-  foo\n\n   bar',
  '-\n  foo\n-\n  ```\n  bar\n  ```\n-\n      baz',
  '-   \n  foo',
  '-\n\n  foo',
  '- foo\n-\n- bar',
  '- foo\n-   \n- bar',
  '1. foo\n2.\n3. bar',
  '*',
  'foo\n*\n\nfoo\n1.',
  ' 1.  A paragraph\n     with two lines.\n\n         indented code\n\n     > A block quote.',
  '    1.  A paragraph\n        with two lines.\n\n            indented code\n\n        > A block quote.',
  '  1.  A paragraph\nwith two lines.\n\n          indented code\n\n      > A block quote.',
  '  1.  A paragraph\n    with two lines.',
  '> 1. > Blockquote\ncontinued here.',
  '> 1. > Blockquote\n> continued here.',
  '- foo\n  - bar\n    - baz\n      - boo',
  '- foo\n - bar\n  - baz\n   - boo',
  '10) foo\n    - bar',
  '10) foo\n   - bar',
  '- - foo',
  '1. - 2. foo',
  '- # Foo\n- Bar\n  ---\n  baz',
  // Lists
  '- foo\n- bar\n+ baz',
  '1. foo\n2. bar\n3) baz',
  'Foo\n- bar\n- baz',
  'The number of windows in my house is\n14.  The number of doors is 6.',
  'The number of windows in my house is\n1.  The number of doors is 6.',
  '- foo\n\n- bar\n\n\n- baz',
  '- foo\n  - bar\n    - baz\n\n\n      bim',
  '- foo\n- bar\n\n<!-- -->\n\n- baz\n- bim',
  '-   foo\n\n    notcode\n\n-   foo\n\n<!-- -->\n\n    code',
  '- a\n - b\n  - c\n   - d\n  - e\n - f\n- g',
  '1. a\n\n  2. b\n\n   3. c',
  '- a\n - b\n  - c\n   - d\n    - e',
  '1. a\n\n  2. b\n\n    3. c',
  '- a\n- b\n\n- c',
  '* a\n*\n\n* c',
  '- a\n- b\n\n  c\n- d',
  '- a\n- b\n\n  [ref]: /url\n- d',
  '- a\n- ```\n  b\n\n\n  ```\n- c',
  '- a\n  - b\n\n    c\n- d',
  '* a\n  > b\n  >\n* c',
  '- a\n  > b\n  ```\n  c\n  ```\n- d',
  '- a',
  '- a\n  - b',
  '1. ```\n   foo\n   ```\n\n   bar',
  '* foo\n  * bar\n\n  baz',
  '- a\n  - b\n  - c\n\n- d\n  - e\n  - f',
  '- [ ] foo\n- [x] bar',
  '- [x] foo\n  - [ ] bar\n  - [x] baz\n- [ ] bim',
  '- [X] upper\n- [] no\n- [ ]no\n-  [ ] two spaces\n- [ ]  two after',
  '1. [ ] a\n2. [x] b',
  '- a\n\tb\n- \tc\n-\td',
  '- a\n<div>\nb',
  '- a\n# h\n- b',
  '- a\n```\ncode\n```\n- b',
  '- a\n\n\n\n- b',
  '- ' + NBSP + '\n  x',
  '-' + NBSP + 'x',
  // Inlines
  '`hi`lo`',
  // Code spans
  '`foo`',
  '`` foo ` bar ``',
  '` `` `',
  '`  ``  `',
  '` a`',
  '`' + NBSP + 'b' + NBSP + '`',
  '` `\n`  `',
  '``\nfoo\nbar  \nbaz\n``',
  '``\nfoo \n``',
  '`foo   bar \nbaz`',
  '`foo\\`bar`',
  '``foo`bar``',
  '` foo `` bar `',
  '*foo`*`',
  '[not a `link](/foo`)',
  '`<a href="`">`',
  '<a href="`">`',
  '`<https://foo.bar.`baz>`',
  '<https://foo.bar.`baz>`',
  '```foo``',
  '`foo',
  '`foo``bar``',
  // Emphasis and strong emphasis
  '*foo bar*',
  'a * foo bar*',
  'a*"foo"*',
  '*' + NBSP + 'a' + NBSP + '*',
  '*$*alpha.\n\n*' + cp(0xa3) + '*bravo.\n\n*' + CURRENCY + '*charlie.',
  'foo*bar*',
  '5*6*78',
  '_foo bar_',
  '_ foo bar_',
  'a_"foo"_',
  'foo_bar_',
  '5_6_78',
  cp(0x43f, 0x440, 0x438, 0x441, 0x442, 0x430, 0x43d, 0x44f, 0x43c) + '_' + cp(0x441, 0x442, 0x440, 0x435, 0x43c, 0x44f, 0x442, 0x441, 0x44f) + '_',
  'aa_"bb"_cc',
  'foo-_(bar)_',
  '_foo*',
  '*foo bar *',
  '*foo bar\n*',
  '*(*foo)',
  '*(*foo*)*',
  '*foo*bar',
  '_foo bar _',
  '_(_foo)',
  '_(_foo_)_',
  '_foo_bar',
  '_' + cp(0x43f, 0x440, 0x438) + '_' + cp(0x441, 0x442),
  '_foo_bar_baz_',
  '_(bar)_.',
  '**foo bar**',
  '** foo bar**',
  'a**"foo"**',
  'foo**bar**',
  '__foo bar__',
  '__ foo bar__',
  '__\nfoo bar__',
  'a__"foo"__',
  'foo__bar__',
  '5__6__78',
  cp(0x43f, 0x440, 0x438) + '__' + cp(0x441, 0x442) + '__',
  '__foo, __bar__, baz__',
  'foo-__(bar)__',
  '**foo bar **',
  '**(**foo)',
  '*(**foo**)*',
  '**Gomphocarpus (*Gomphocarpus physocarpus*, syn.\n*Asclepias physocarpa*)**',
  '**foo "*bar*" foo**',
  '**foo**bar',
  '__foo bar __',
  '__(__foo)',
  '_(__foo__)_',
  '__foo__bar',
  '__' + cp(0x43f, 0x440, 0x438) + '__' + cp(0x441, 0x442),
  '__foo__bar__baz__',
  '__(bar)__.',
  '*foo [bar](/url)*',
  '*foo\nbar*',
  '_foo __bar__ baz_',
  '_foo _bar_ baz_',
  '__foo_ bar_',
  '*foo *bar**',
  '*foo **bar** baz*',
  '*foo**bar**baz*',
  '*foo**bar*',
  '***foo** bar*',
  '*foo **bar***',
  '*foo**bar***',
  'foo***bar***baz',
  'foo******bar*********baz',
  '*foo **bar *baz* bim** bop*',
  '*foo [*bar*](/url)*',
  '** is not an empty emphasis',
  '**** is not an empty strong emphasis',
  '**foo [bar](/url)**',
  '**foo\nbar**',
  '__foo _bar_ baz__',
  '__foo __bar__ baz__',
  '____foo__ bar__',
  '**foo **bar****',
  '**foo *bar* baz**',
  '**foo*bar*baz**',
  '***foo* bar**',
  '**foo *bar***',
  '**foo *bar **baz**\nbim* bop**',
  '**foo [*bar*](/url)**',
  '__ is not an empty emphasis',
  '____ is not an empty strong emphasis',
  'foo ***',
  'foo *\\**',
  'foo *_*',
  'foo *****',
  'foo **\\***',
  'foo **_**',
  '**foo*',
  '*foo**',
  '***foo**',
  '****foo*',
  '**foo***',
  '*foo****',
  'foo ___',
  'foo _\\__',
  'foo _*_',
  'foo _____',
  'foo __\\___',
  'foo __*__',
  '__foo_',
  '_foo__',
  '___foo__',
  '____foo_',
  '__foo___',
  '_foo____',
  '**foo**',
  '*_foo_*',
  '__foo__',
  '_*foo*_',
  '****foo****',
  '____foo____',
  '******foo******',
  '***foo***',
  '_____foo_____',
  '*foo _bar* baz_',
  '*foo __bar *baz bim__ bam*',
  '**foo **bar baz**',
  '*foo *bar baz*',
  '*[bar*](/url)',
  '_foo [bar_](/url)',
  '*<img src="foo" title="*"/>',
  '**<a href="**">',
  '__<a href="__">',
  '*a `*`*',
  '_a `_`_',
  '**a<https://foo.bar/?q=**>',
  '__a<https://foo.bar/?q=__>',
  '__abc*abc__ *x*',
  '**abc_abc** _x_',
  '*a' + GRIN + '*',
  '*' + GRIN + 'a*',
  '**' + GRIN + '**' + GRIN + '**',
  GRIN + '*a*',
  GRIN + '_a_',
  MATH_A + '_a_',
  'a_' + MATH_A + '_',
  '_' + MATH_A + '_a',
  '*a \\' + GRIN + ' b*',
  '\\' + GRIN + ' *a*',
  '*a* \\' + GRIN + ' *b* \\' + GRIN + ' **c**',
  '\\' + GRIN + '\\' + GRIN + ' _x_ \\' + GRIN,
  '*a\\' + GRIN + '*',
  '**a \\' + GRIN + ' b** c',
  '~*a*~ ~~*b*~~ *~c~* **~~d~~**',
  '*a~b* *~b* *a~*',
  CJK + '*' + CJK + '*' + CJK,
  CJK + '**' + CJK + '**',
  '**' + CJK + '(' + CJK + ')**' + CJK,
  cp(0x300c) + '*x*' + cp(0x300d),
  // Links
  '[link](/uri "title")',
  '[link](/uri)',
  '[](./target.md)',
  '[link]()',
  '[link](<>)',
  '[]()',
  '[link](/my uri)',
  '[link](</my uri>)',
  '[link](foo\nbar)',
  '[link](<foo\nbar>)',
  '[a](<b)c>)',
  '[link](<foo\\>)',
  '[a](<b)c\n[a](<b)c>\n[a](<b>c)',
  '[link](\\(foo\\))',
  '[link](foo(and(bar)))',
  '[link](foo(and(bar))',
  '[link](foo\\(and\\(bar\\))',
  '[link](<foo(and(bar)>)',
  '[link](foo\\)\\:)',
  '[link](#fragment)\n\n[link](https://example.com#fragment)\n\n[link](https://example.com?foo=3#frag)',
  '[link](foo\\bar)',
  '[link](foo%20b&auml;)',
  '[link]("title")',
  '[link](/url "title")\n[link](/url \'title\')\n[link](/url (title))',
  '[link](/url "title \\"&quot;")',
  '[link](/url' + NBSP + '"title")',
  '[link](/url "title "and" title")',
  '[link](/url \'title "and" title\')',
  '[link](   /uri\n  "title"  )',
  '[link] (/uri)',
  '[link [foo [bar]]](/uri)',
  '[link] bar](/uri)',
  '[link [bar](/uri)',
  '[link \\[bar](/uri)',
  '[link *foo **bar** `#`*](/uri)',
  '[![moon](moon.jpg)](/uri)',
  '[foo [bar](/uri)](/uri)',
  '[foo *[bar [baz](/uri)](/uri)*](/uri)',
  '![[[foo](uri1)](uri2)](uri3)',
  '*[foo*](/uri)',
  '[foo *bar](baz*)',
  '*foo [bar* baz]',
  '[foo <bar attr="](baz)">',
  '[foo`](/uri)`',
  '[foo<https://example.com/?search=](uri)>',
  '[foo][bar]\n\n[bar]: /url "title"',
  '[link [foo [bar]]][ref]\n\n[ref]: /uri',
  '[link \\[bar][ref]\n\n[ref]: /uri',
  '[link *foo **bar** `#`*][ref]\n\n[ref]: /uri',
  '[![moon](moon.jpg)][ref]\n\n[ref]: /uri',
  '[foo [bar](/uri)][ref]\n\n[ref]: /uri',
  '[foo *bar [baz][ref]*][ref]\n\n[ref]: /uri',
  '*[foo*][ref]\n\n[ref]: /uri',
  '[foo *bar][ref]*\n\n[ref]: /uri',
  '[foo <bar attr="][ref]">\n\n[ref]: /uri',
  '[foo`][ref]`\n\n[ref]: /uri',
  '[foo<https://example.com/?search=][ref]>\n\n[ref]: /uri',
  '[foo][BaR]\n\n[bar]: /url "title"',
  '[' + cp(0x1e9e) + ']\n\n[SS]: /url',
  '[Foo\n  bar]: /url\n\n[Baz][Foo bar]',
  '[foo] [bar]\n\n[bar]: /url "title"',
  '[foo]\n[bar]\n\n[bar]: /url "title"',
  '[foo]: /url1\n\n[foo]: /url2\n\n[bar][foo]',
  '[bar][foo\\!]\n\n[foo!]: /url',
  '[foo][ref[]\n\n[ref[]: /uri',
  '[foo][ref[bar]]\n\n[ref[bar]]: /uri',
  '[[[foo]]]\n\n[[[foo]]]: /url',
  '[foo][ref\\[]\n\n[ref\\[]: /uri',
  '[bar\\\\]: /uri\n\n[bar\\\\]',
  '[]\n\n[]: /uri',
  '[\n ]\n\n[\n ]: /uri',
  '[foo][]\n\n[foo]: /url "title"',
  '[*foo* bar][]\n\n[*foo* bar]: /url "title"',
  '[Foo][]\n\n[foo]: /url "title"',
  '[foo] \n[]\n\n[foo]: /url "title"',
  '[foo]\n\n[foo]: /url "title"',
  '[*foo* bar]\n\n[*foo* bar]: /url "title"',
  '[[*foo* bar]]\n\n[*foo* bar]: /url "title"',
  '[[bar [foo]\n\n[foo]: /url',
  '[Foo]\n\n[foo]: /url "title"',
  '[foo] bar\n\n[foo]: /url',
  '\\[foo]\n\n[foo]: /url "title"',
  '[foo*]: /url\n\n*[foo*]',
  '[foo][bar]\n\n[foo]: /url1\n[bar]: /url2',
  '[foo][]\n\n[foo]: /url1',
  '[foo]()\n\n[foo]: /url1',
  '[foo](not a link)\n\n[foo]: /url1',
  '[foo][bar][baz]\n\n[baz]: /url',
  '[foo][bar][baz]\n\n[baz]: /url1\n[bar]: /url2',
  '[foo][bar][baz]\n\n[baz]: /url1\n[foo]: /url2',
  '*[ref]* _[ref][]_ **[x][ref]**\n\n[ref]: /u ""',
  '[' + GRIN + ']: /e\n\n*[' + GRIN + ']* [' + GRIN + '][] *x*',
  // Images
  '![foo](/url "title")',
  '![foo *bar*]\n\n[foo *bar*]: train.jpg "train & tracks"',
  '![foo ![bar](/url)](/url2)',
  '![foo [bar](/url)](/url2)',
  '![foo *bar*][]\n\n[foo *bar*]: train.jpg "train & tracks"',
  '![foo *bar*][foobar]\n\n[FOOBAR]: train.jpg "train & tracks"',
  '![foo](train.jpg)',
  'My ![foo bar](/path/to/train.jpg  "title"   )',
  '![foo](<url>)',
  '![](/url)',
  '![foo][bar]\n\n[bar]: /url',
  '![foo][bar]\n\n[BAR]: /url',
  '![foo][]\n\n[foo]: /url "title"',
  '![*foo* bar][]\n\n[*foo* bar]: /url "title"',
  '![Foo][]\n\n[foo]: /url "title"',
  '![foo] \n[]\n\n[foo]: /url "title"',
  '![foo]\n\n[foo]: /url "title"',
  '![*foo* bar]\n\n[*foo* bar]: /url "title"',
  '![[foo]]\n\n[[foo]]: /url "title"',
  '![Foo]\n\n[foo]: /url "title"',
  '!\\[foo]\n\n[foo]: /url "title"',
  '\\![foo]\n\n[foo]: /url "title"',
  '! [a](b)',
  '!',
  // Autolinks
  '<http://foo.bar.baz>',
  '<https://foo.bar.baz/test?q=hello&id=22&boolean>',
  '<irc://foo.bar:2233/baz>',
  '<MAILTO:FOO@BAR.BAZ>',
  '<a+b+c:d>',
  '<made-up-scheme://foo,bar>',
  '<https://../>',
  '<localhost:5001/foo>',
  '<https://foo.bar/baz bim>',
  '<https://example.com/\\[\\>',
  '<foo@bar.example.com>',
  '<foo+special@Bar.baz-bar0.com>',
  '<foo\\+@bar.example.com>',
  '<>',
  '< https://foo.bar >',
  '<m:abc>',
  '<foo.bar.baz>',
  'https://example.com',
  'foo@bar.example.com',
  '<a^b@c.de>',
  '<a@b.c_>',
  // Autolinks (extension)
  'www.commonmark.org',
  'Visit www.commonmark.org/help for more information.',
  'Visit www.commonmark.org.\n\nVisit www.commonmark.org/a.b.',
  'www.google.com/search?q=Markup+(business)\n\nwww.google.com/search?q=Markup+(business)))\n\n(www.google.com/search?q=Markup+(business))\n\n(www.google.com/search?q=Markup+(business)',
  'www.google.com/search?q=(business))+ok',
  'www.google.com/search?q=commonmark&hl=en\n\nwww.google.com/search?q=commonmark&hl;',
  'www.commonmark.org/he<lp',
  'http://commonmark.org\n\n(Visit https://encrypted.google.com/search?q=Markup+(business))',
  'foo@bar.baz',
  "hello@mail+xyz.example isn't valid, but hello+xyz@mail.example is.",
  'a.b-c_d@a.b\n\na.b-c_d@a.b.\n\na.b-c_d@a.b-\n\na.b-c_d@a.b_',
  'mailto:foo@bar.baz\n\nmailto:a.b-c_d@a.b\n\nmailto:a.b-c_d@a.b.\n\nmailto:a.b-c_d@a.b/\n\nmailto:a.b-c_d@a.b-\n\nmailto:a.b-c_d@a.b_\n\nxmpp:foo@bar.baz\n\nxmpp:foo@bar.baz.',
  'xmpp:foo@bar.baz/txt\n\nxmpp:foo@bar.baz/txt@bin\n\nxmpp:foo@bar.baz/txt@bin.com',
  'xmpp:foo@bar.baz/txt/bin',
  'HTTP://EXAMPLE.COM/A WWW.EXAMPLE.COM Www.Example.com FTP://x.y hTTps://a.b',
  'ftp://files.example.com/a.txt, http://a.b/c?d=e&f;.',
  'see http://a.b/c_d_e and http://a.b/*x* now',
  '<a href="x">www.inside.link</a> www.outside.link',
  '<a href="x">www.inside.link',
  '[www.a.b](http://c.d) www.e.f',
  'http://a.b/' + CJK + ' www.' + CJK + '.com',
  'x@y.z' + GRIN,
  // Raw HTML
  '<a><bab><c2c>',
  '<a/><b2/>',
  '<a  /><b2\ndata="foo" >',
  '<a foo="bar" bam = \'baz <em>"</em>\'\n_boolean zoop:33=zoop:33 />',
  'Foo <responsive-image src="foo.jpg" />',
  '<33> <__>',
  '<a h*#ref="hi">',
  "<a href=\"hi'> <a href=hi'>",
  '< a><\nfoo><bar/ >\n<foo bar=baz\nbim!bop />',
  "<a href='bar'title=title>",
  '</a></foo >',
  '</a href="foo">',
  'foo <!-- this is a --\ncomment - with hyphens -->',
  'foo <!--> foo -->\n\nfoo <!---> foo -->',
  'foo <?php echo $a; ?>',
  'foo <!ELEMENT br EMPTY>',
  'foo <![CDATA[>&<]]>',
  'foo <a href="&ouml;">',
  'foo <a href="\\*">',
  '<a href="\\"">',
  'a <pre>*x*</pre> *y* <code>z</code> <kbd>k</kbd> <script>s</script> t',
  'a <PRE>x</pre> <KBD >y</kbd > z',
  'a <pre>x <b>y</b>',
  'a <a href="u">b <a href="v">c</a> d</a> e',
  'a <A HREF="u">www.a.b</A> www.c.d',
  // Hard line breaks
  'foo  \nbaz',
  'foo\\\nbaz',
  'foo       \nbaz',
  'foo  \n     bar',
  'foo\\\n     bar',
  '*foo  \nbar*',
  '*foo\\\nbar*',
  '`code  \nspan`',
  '`code\\\nspan`',
  '<a href="foo  \nbar">',
  '<a href="foo\\\nbar">',
  'foo\\',
  'foo  ',
  '### foo\\',
  '### foo  ',
  'foo  \n',
  'foo  \n \n',
  // Soft line breaks
  'foo\nbaz',
  'foo \n baz',
  // Textual content
  "hello $.;'there",
  'Foo ' + cp(0x3c7, 0x3c1, 0x1fc6, 0x3bd),
  'Multiple     spaces',
  // Tables (extension)
  '| foo | bar |\n| --- | --- |\n| baz | bim |',
  '| abc | defghi |\n:-: | -----------:\nbar | baz',
  '| f\\|oo  |\n| ------ |\n| b `\\|` az |\n| b **\\|** im |',
  '| abc | def |\n| --- | --- |\n| bar | baz |\n> bar',
  '| abc | def |\n| --- | --- |\n| bar | baz |\nbar\n\nbar',
  '| abc | def |\n| --- |\n| bar |',
  '| abc | def |\n| --- | --- |\n| bar |\n| bar | baz | boo |',
  '| abc | def |\n| --- | --- |',
  'a|b\n-|-\nc|d',
  'a\n-\nb',
  'a\n:-\nb',
  'a\n-:\nb',
  'a\n|-\nb',
  '|a|\n|-|\n|b|\n\n|c|',
  '|a|b|\n|:-|-:|\n|:-:|x|\n| | |',
  'x\n|a|b|\n|-|-|\n|c|d|',
  '|a|b|\n|-|-|\n|c|d|\n# h',
  '|a|b|\n|-|-|\n|c|d|\n- l',
  '|a|b|\n|-|-|\n|c|d|\n    code',
  '|a|b|\n|-|-|\n|c|d|\n```\nf\n```',
  '|a|b|\n|-|-|\n|c|d|\n<div>',
  '|a|b|\n|-|-|\n|c|d|\n***',
  '| a \\\\| b |\n|-|-|\n| \\\\\\| c | d |',
  '|\n|-|',
  '| |\n|-|',
  'a | b\n- | -\n  \n',
  '  a|b\n   -|-\n c|d\n\n',
  '|*a*|`b`|\n|-|-|\n|[c](d)|<br>|',
  // Strikethrough (extension)
  '~~Hi~~ Hello, ~there~ world!',
  'This ~~has a\n\nnew paragraph~~.',
  'This will ~~~not~~~ strike.',
  '~~a~b~~ ~a~~b~ ~~ a~~ ~~a ~~',
  '~~\\~~~ ~\\~~ ~~a\\~~',
  // Mermaid label shapes
  'Hello world',
  'Hello **world**',
  '**bold** and *italic*',
  'This is **bold** </br>and <strong>strong</strong>',
  'The dog in **the** hog... a *very long text* about it\nWord!',
  'A\nB\n\nC',
  'line one<br/>line two<br>line three<br />four',
  '<br/>',
  '<br>',
  'a<br/><br/>b',
  'Text with `code` and \\`escaped\\`',
  '  indented\n    more\n  less',
  '\n    dedent me\n      please\n    ',
  '1. first',
  '1. first\n2. second',
  '- item',
  '# heading',
  '> quote',
  'a_b_c and a*b*c',
  'snake_case_name __init__ ___x___',
  '2 * 3 * 4 = 24',
  'a < b && c > d',
  'x<y>z',
  'fa:fa-car Car',
  'https://mermaid.js.org',
  '"quoted" and \'single\'',
  "it's",
  'tab\there',
  'trailing space ',
  ' leading space',
  'multiple   spaces   inside',
  'emoji ' + GRIN + ' ' + THUMB_TONE + ' ' + FAMILY + ' ' + FLAG_BR + ' ' + HEART + ' ' + KEYCAP,
  CJK + ' ' + KANA,
  ARABIC + ' ' + DEVANAGARI + ' ' + HANGUL + ' ' + THAI,
  'e' + COMBINING_ACUTE + ' vs ' + E_ACUTE,
  'a' + NBSP + 'b' + IDEOGRAPHIC_SPACE + 'c' + BOM + 'd',
  'a' + LS + 'b' + PS + 'c',
  '# h' + LS + 'more\n\nx' + LS + '===',
  '    code' + LS + '    more',
  '> q' + LS + '> r',
  '|a|b|' + LS + '|-|-|',
  'a\n-' + LS,
  '[x]: <u' + LS + 'v>\n\n[x]',
  '- a' + LS + '- b',
  'a' + LS + '    b',
  GUILLEMETS + ' ' + CURRENCY + ' ' + EM_DASH + ' ' + NEW_EMOJI,
  '*' + NEW_EMOJI + '*a* _' + NEW_EMOJI + '_a_',
  'a\r\nb\rc\n\r\nd',
  '**a**\r\n*b*\r',
  'a\\\r\nb  \r\nc',
  '',
];

// Parts that the generator joins.
const INLINE_PARTS = [
  '*', '**', '***', '_', '__', '___', '~', '~~', '~~~',
  '\\', '\\\\', '\\*', '\\_', '\\[', '\\]', '\\`', '\\<', '\\&', '\\~', '\\|', '\\' + GRIN, '\\' + CURRENCY,
  '`', '``', '```', '` `',
  '<br>', '<br/>', '<br />', '<BR>', '</br>', '<b>', '</b>', '<i>', '</i>', '<a href="x">', '<a >', '</a>', '<A HREF=x>', '</A>',
  '<div>', '</div>', '<span class=\'a\'>', '</span>', '<pre>', '</pre>', '<code>', '</code>', '<kbd>', '</kbd>',
  '<script>', '</script>', '<SCRIPT>', '<style>', '</style>', '<textarea>', '</textarea>',
  '<!-- c -->', '<!--', '-->', '<?php ?>', '<!DOCTYPE x>', '<![CDATA[x]]>', '<x', '<', '>', '<>', '< >', '<x y="z">', '<x/>',
  '&amp;', '&#39;', '&lt;', '&gt;', '&nbsp;', '&#x27;', '&', '&#39', '&quot;',
  '[', ']', '(', ')', '[]', '()', '[a]', '[a](b)', '[a](<b c>)', '[a](b "t")', "[a](b 't')", '[a](b (t))', '![i](s)', '[a][]', '[a][r]', '[r]', '![r]',
  '](', ')[', '[[', ']]', '!', '![',
  '<http://x.y>', '<a@b.co>', 'http://x.y/z?q=1.', 'https://x.y', 'www.x.y', 'WWW.X.Y', 'a@b.co', 'ftp://x.y', 'HTTP://X.Y', 'mailto:a@b.co',
  ' ', ' ', ' ', '  ', '   ', '\t', '\n', '\n', '\n\n', '  \n', '\\\n', '\r\n', '\r', ' \n ',
  'a', 'b', 'foo', 'bar', 'Hello', 'world', 'x1', '42', '3.14', 'A_B', 'a-b',
  '.', ',', '!', '?', ':', ';', '"', "'", '-', '--', '---', '+', '=', '==', '|', '#', '##', '/', '%', '$', '^', '{', '}', '@',
  E_ACUTE, 'e' + COMBINING_ACUTE, CJK, KANA, GRIN, THUMB_TONE, FAMILY, FLAG_BR, HEART, KEYCAP, NBSP, LS, PS, IDEOGRAPHIC_SPACE, BOM, ZWJ,
  DOTTED_I, DOTLESS_I, SIGMA, SHARP_S, KELVIN, LONG_S, MATH_A, ARABIC, DEVANAGARI, HANGUL, THAI, GUILLEMETS, CURRENCY, EM_DASH, NEW_EMOJI,
];

const LINE_STARTS = [
  '', '', '', '', ' ', '  ', '   ', '    ', '     ', '\t', ' \t', '  \t',
  '- ', '* ', '+ ', '-', '1. ', '2) ', '10. ', '1.', '- [ ] ', '- [x] ', '-  ', '-    ', '-     ', '  - ', '    - ', '   1. ',
  '# ', '## ', '###### ', '####### ', '#', ' # ', '#\t',
  '> ', '>', '>> ', '> > ', ' > ', '   > ', '    > ', '> - ', '> 1. ', '> # ', '>     ',
  '```', '~~~', '````', '```js', '   ```', '~~~ x',
  '===', '---', '***', '___', '- - -', '=', '-', '--', ' ---', '    ---', '* * *',
  '|', '| a | b |', '|-|-|', '|:-|-:|', '-|-', 'a|b', ':-:|:-', '| --- |', '|a|',
  '[r]: /u', '[r]: /u "t"', '[a]: <u v> \'t\'', '[R  r]: u', '   [r]: u',
  '<div>', '</div>', '<pre>', '</pre>', '<script>', '</script>', '<!--', '-->', '<?', '?>', '<x>', '<table>', '<br>', '<p>', '</p>', '<details>', '<hr/>',
];

const pick = (rand, list) => list[Math.floor(rand() * list.length)];

// generated returns strings that join markdown parts at random. The first
// kind is one line of inline parts. The second kind is a document of lines,
// each with a block start and inline parts.
export const generated = (seed, count) => {
  const rand = mulberry32(seed);
  const out = [];
  const inline = (max) => {
    let s = '';
    const n = 1 + Math.floor(rand() * max);
    for (let i = 0; i < n; i++) s += pick(rand, INLINE_PARTS);
    return s;
  };
  for (let i = 0; i < count; i++) {
    const kind = rand();
    if (kind < 0.5) {
      out.push(inline(10));
    } else {
      const lines = 1 + Math.floor(rand() * 7);
      let s = '';
      for (let j = 0; j < lines; j++) {
        const body = rand() < 0.25 ? '' : inline(5);
        const eol = j === lines - 1 ? (rand() < 0.2 ? '\n' : '') : rand() < 0.1 ? '\n\n' : rand() < 0.05 ? '\r\n' : '\n';
        s += pick(rand, LINE_STARTS) + body + eol;
      }
      out.push(s);
    }
  }
  return out;
};

// Parts for emphasisStrings: the delimiters, and the characters that decide
// whether a delimiter run can open or close.
const EMPHASIS_PARTS = [
  '*', '*', '*', '**', '**', '***', '_', '_', '__', '___', '~', '~~',
  ' ', ' ', ' ', 'a', 'a', 'b', 'c', 'foo', '1', '.', ',', '(', ')', '"', '!', '&', '\\', '`', '[', ']', '<', '>', '\n',
  GRIN, CJK[0], NBSP, E_ACUTE, CURRENCY, MATH_A, cp(0x3002), cp(0x300c), cp(0x300d),
];

// emphasisStrings returns short strings of delimiter runs between letters,
// punctuation and spaces. They cover the rules of emphasis.
export const emphasisStrings = (seed, count) => {
  const rand = mulberry32(seed);
  const out = [];
  for (let i = 0; i < count; i++) {
    let s = '';
    const n = 2 + Math.floor(rand() * 11);
    for (let j = 0; j < n; j++) s += pick(rand, EMPHASIS_PARTS);
    out.push(s);
  }
  return out;
};

const CELLS = ['a', 'b c', '', ' ', '*e*', '**s**', '`c`', '`a|b`', 'a\\|b', '\\\\|', '[l](u)', '<br>', GRIN, CJK, ':-', '---', 'x|y', '1', ' pad ', '\\'];
const ALIGNS = ['-', '--', ':-', '-:', ':-:', ' --- ', ':--:', '', ' ', 'x', '- -', ':', '=', '-|'];

// tableDocs returns documents that are tables, or that are near to a table.
export const tableDocs = (seed, count) => {
  const rand = mulberry32(seed);
  const row = (cells, edge) => {
    const body = cells.join(rand() < 0.2 ? ' | ' : '|');
    return (edge < 0.5 ? '|' : '') + body + (edge < 0.5 || rand() < 0.2 ? '|' : '') + (rand() < 0.1 ? '  ' : '');
  };
  const out = [];
  for (let i = 0; i < count; i++) {
    const cols = 1 + Math.floor(rand() * 4);
    const cells = (list, n) => Array.from({ length: n }, () => pick(rand, list));
    const edge = rand();
    const lines = [];
    if (rand() < 0.3) lines.push(pick(rand, ['text', '# h', '- l', '', '> q', '    code']));
    lines.push(pick(rand, ['', '', '', ' ', '   ', '    ']) + row(cells(CELLS, cols), edge));
    const alignCols = rand() < 0.15 ? cols + (rand() < 0.5 ? 1 : -1) : cols;
    lines.push(pick(rand, ['', '', '', ' ', '   ', '    ']) + row(cells(ALIGNS.slice(0, rand() < 0.85 ? 7 : ALIGNS.length), Math.max(alignCols, 1)), edge));
    const rows = Math.floor(rand() * 4);
    for (let r = 0; r < rows; r++) {
      const n = rand() < 0.2 ? Math.max(1, cols + Math.floor(rand() * 3) - 1) : cols;
      lines.push(row(cells(CELLS, n), rand() < 0.8 ? edge : rand()));
    }
    if (rand() < 0.4) lines.push(pick(rand, ['', 'text', '# h', '- l', '> q', '    code', '```', '<div>', '***', ' ', '1. x', '2. x', 'a|b']));
    if (rand() < 0.2) lines.push('after');
    out.push(lines.join('\n') + (rand() < 0.2 ? '\n' : ''));
  }
  return out;
};

const INDENTS = ['', '', '', ' ', '  ', '  ', '   ', '    ', '    ', '     ', '      ', '        ', '\t', ' \t', '  \t', '\t\t'];
const BLOCK_STARTS = [
  '', '', '', '', '- ', '- ', '* ', '+ ', '1. ', '2. ', '3) ', '10. ', '-', '1.', '- [ ] ', '- [x] ', '-   ', '-\t', '1.\t',
  '> ', '> ', '>', '>> ', '> - ', '> > ', '> 1. ', '>\t',
  '# ', '## ', '```', '~~~', '```x', '---', '***', '===', '- - -', '<div>', '</div>', '<!-- c -->', '[r]: /u', '|a|b|', '|-|-|',
];
const LINE_BODIES = ['', '', 'a', 'b', 'foo bar', 'text', '*e*', '**s**', '`c`', 'x  ', 'y\\', '[r]', '<br>', '# h', '- n', '1. n', '> q', '```', '---', '===', '    c', GRIN, NBSP, ' '];

// blockDocs returns documents of lines with random indentation and block
// starts. They cover lists, block quotes, code blocks and their nesting.
export const blockDocs = (seed, count) => {
  const rand = mulberry32(seed);
  const out = [];
  for (let i = 0; i < count; i++) {
    const n = 2 + Math.floor(rand() * 7);
    const lines = [];
    for (let j = 0; j < n; j++) {
      lines.push(rand() < 0.15 ? pick(rand, ['', '', ' ', '  ', '\t']) : pick(rand, INDENTS) + pick(rand, BLOCK_STARTS) + pick(rand, LINE_BODIES));
    }
    out.push(lines.join('\n') + (rand() < 0.2 ? '\n' : ''));
  }
  return out;
};

const LABELS = ['a', 'A', 'a b', 'A  B', 'a\nb', '*a*', 'a]', 'a\\]', '[a]', '!', 'x`y`', SIGMA + 'a' + SIGMA, DOTTED_I, GRIN, '', ' ', 'a[b]c', 'toString'];
const DESTS = ['/u', '<u v>', '<>', 'u(v)', 'u(v', 'u)v', 'u\\)v', '<u>v', 'http://a.b/c', '', '#f', 'u v', '\\<u\\>', '<u\\>', '/' + GRIN, '&amp;', 'a\\*b'];
const TITLES = ['', '', ' "t"', " 't'", ' (t)', ' "a \\" b"', ' "t', ' "a"b"', '\n"t"', ' "a\nb"', ' ""', '  "t"  ', ' "\\*"', ' (a(b))'];

// linkDocs returns inline links, images, reference links and definitions.
export const linkDocs = (seed, count) => {
  const rand = mulberry32(seed);
  const out = [];
  for (let i = 0; i < count; i++) {
    const parts = [];
    const n = 1 + Math.floor(rand() * 4);
    for (let j = 0; j < n; j++) {
      const label = pick(rand, LABELS);
      const kind = rand();
      const bang = rand() < 0.2 ? '!' : '';
      if (kind < 0.4) parts.push(`${bang}[${label}](${pick(rand, DESTS)}${pick(rand, TITLES)})`);
      else if (kind < 0.55) parts.push(`${bang}[${label}][${pick(rand, LABELS)}]`);
      else if (kind < 0.7) parts.push(`${bang}[${label}]${rand() < 0.5 ? '[]' : ''}`);
      else if (kind < 0.8) parts.push(pick(rand, ['*', '**', '_', ' ', 'x', '\\', '`', '<a href="u">', '</a>', 'www.a.b', '<http://a.b>']));
      else parts.push(`\n\n${pick(rand, ['', ' ', '   ', '    '])}[${pick(rand, LABELS)}]:${pick(rand, ['', ' ', '\n', ' \n  '])}${pick(rand, DESTS)}${pick(rand, TITLES)}${pick(rand, ['', '', ' x', '\n', '\n\n'])}`);
      if (rand() < 0.3) parts.push(pick(rand, [' ', '\n', '\n\n', ' text ', '*']));
    }
    out.push(parts.join(''));
  }
  return out;
};

// surrogateInputs returns strings that hold a surrogate without its partner.
// A Go string cannot hold them in JSON, so the generators write them as
// lists of UTF-16 code units.
export const surrogateInputs = () => [
  HIGH,
  LOW,
  'a' + HIGH + 'b',
  'a' + LOW + 'b',
  '*a' + HIGH + '*',
  '*' + LOW + 'a*',
  '_a_' + LOW,
  HIGH + '_a_',
  LOW + HIGH + ' **x** ' + LOW + HIGH,
  '\\' + HIGH + ' *a*',
  '[' + HIGH + ']: /u\n\n[' + HIGH + ']',
  '- ' + LOW + '\n- ' + HIGH,
  '`' + HIGH + '`' + LOW,
  HIGH + '\n' + LOW,
  '<a ' + HIGH + '>' + LOW + '</a>',
];

// inputGroups returns each input once, in a stable order: the inputs that
// come from the specifications and from the corpus, and the inputs of the
// generators.
export const inputGroups = (repo) => {
  const seen = new Set();
  const unique = (list) =>
    list.filter((s) => {
      if (seen.has(s)) return false;
      seen.add(s);
      return true;
    });
  const written = unique([...specExamples(), ...corpusLabels(repo)]);
  const generatedInputs = unique([
    ...generated(20261108, 6000),
    ...emphasisStrings(11, 5000),
    ...blockDocs(12, 3000),
    ...linkDocs(13, 1500),
    ...tableDocs(14, 600),
  ]);
  return { written, generated: generatedInputs };
};

// allInputs returns each input once, in a stable order.
export const allInputs = (repo) => {
  const groups = inputGroups(repo);
  return [...groups.written, ...groups.generated];
};
