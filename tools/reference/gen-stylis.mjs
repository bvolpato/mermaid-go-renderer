// Emits golden cases for internal/stylis.
//
// Each case is CSS text and a namespace. The expected values come from the
// real stylis and from mermaid's own compileCSS:
//   ast    compile(css), as a tree
//   plain  serialize(compile(css), stringify)
//   out    compileCSS(ns, css) of mermaid (stylis with the namespace middleware)
//   newer  serialize(compile(css), stringify) of the stylis in node_modules,
//          only when it is not equal to plain
//
// Source of stylis: version 4.3.6, the version inside the mermaid 12.2.0
// bundle that the reference renders with. use-bundled.mjs loads its text from
// the source map of the bundle (see extract-bundled-source.mjs), for this
// script and for mermaid.core.mjs. node_modules has stylis 4.4.0. That version
// parses a pseudo-class function character by character, and it keeps a custom
// property that has no value. So the two versions give other results for some
// inputs. The group "version" has cases for those inputs, and "newer" lets the
// Go test say that a port follows the wrong version.
//
// Usage (in tools/reference): node gen-stylis.mjs
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { pathToFileURL } from 'node:url';
import { bundledVersion } from './use-bundled.mjs';
import { HERE, REPO, loadMermaid, loadDiagram, mulberry32 } from './styles-lib.mjs';

// These are dynamic imports, because use-bundled.mjs must run before node
// resolves "stylis".
const { compile, serialize, stringify } = await import('stylis');
// The stylis of node_modules. The import by path does not go through the hook.
const newerStylis = await import(pathToFileURL(path.join(HERE, 'node_modules/stylis/index.js')).href);
const newerVersion = JSON.parse(fs.readFileSync(path.join(HERE, 'node_modules/stylis/package.json'), 'utf8')).version;

const api = await loadMermaid();

// 4.3.6 gives no output for a custom property that has no value. 4.4.0 keeps
// the declaration. This check stops the script when one of the two paths to
// stylis does not load the bundled version.
if (bundledVersion('stylis') !== '4.3.6' || serialize(compile('.a{--x:;}'), stringify) !== '' || api.compileCSS('#id', '.a{--x:;}') !== '') {
  throw new Error('gen-stylis.mjs: stylis is not the version of the mermaid bundle (4.3.6)');
}

// Hand-written cases. Each one aims at a branch of the tokenizer, the parser,
// the serializer or the namespace middleware.
const HAND = [
  // Plain rules and white space.
  '',
  ' ',
  ';',
  '{}',
  '}',
  '}}',
  '{',
  '.a{}',
  '.a { }',
  '.a{fill:red}',
  '.a { fill: red }',
  '.a { fill: red; }',
  '.a{fill:red;stroke:blue}',
  '.a\n{\n\tfill:\tred;\r\n  stroke : blue ;\n}',
  '.a   .b    >   .c  +  .d  ~  .e { fill : red }',
  '.a,.b{fill:red}',
  '.a , .b , .c { fill: red }',
  '.a,\n.b,\n.c\n{ fill: red }',
  '.a { fill: red } .b { fill: blue }',
  '.a{fill:red}.a{fill:blue}',
  'fill: red;',
  'fill: red',
  'fill:red;stroke:blue;',
  'fill : red',
  'fill :red',
  'stroke-width : 1px;',
  ' : red;',
  ': red;',
  'a:b:c;',
  'a: b: c;',
  'fill:;',
  'fill: ;',
  'fill',
  'fill;',
  '.a { fill }',
  '.a { fill; stroke: blue }',
  '.a { ; ; fill: red ; ; }',
  // Root selector exceptions of the namespace middleware.
  '& { font-family: a; font-size: 1px; fill: red }',
  '& { font-family: a; font-size: 1px; fill: red; stroke: blue }',
  '& { fill: red; .x { fill: blue } }',
  '& { }',
  '& { /* only a comment */ }',
  '& { fill: red; /* a comment */ }',
  '&{fill:red}',
  '& , & { fill: red }',
  // Ampersand forms.
  '& .a { fill: red }',
  '&.a { fill: red }',
  '&:hover { fill: red }',
  '&>.a { fill: red }',
  '& > .a { fill: red }',
  '& + & { fill: red }',
  '& ~ .a, & .b { fill: red }',
  '.a & { fill: red }',
  '.a & .b { fill: red }',
  '.a:not(&) { fill: red }',
  '.nested-class :not(&) { fill: red }',
  '& || td { fill: red }',
  '&||td { fill: red }',
  '.a { & { fill: red } }',
  '.a { &.b { fill: red } & .c { fill: blue } }',
  '.a { .b & { fill: red } }',
  '.a, .b { & .c, &.d { fill: red } }',
  '.a { &, .b { fill: red } }',
  '.a { &:is(.b, .c) { fill: red } }',
  '&& { fill: red }',
  '& & & { fill: red }',
  '.a { fill: red; & { stroke: blue } fill: green }',
  '.a{&{&{&{fill:red}}}}',
  '& svg { fill: red }',
  '&svg { fill: red }',
  'svg& { fill: red }',
  '@media print { & { fill: red } }',
  '@media print { & .a { fill: red } }',
  // Nesting.
  '.a { .b { fill: red } }',
  '.a { fill: red; .b { fill: blue } }',
  '.a { .b { fill: blue } fill: red; }',
  '.a { .b { .c { .d { fill: red } } } }',
  '.a, .b { .c, .d { fill: red } }',
  '.a { > .b { fill: red } }',
  '.a { + .b { fill: red } ~ .c { fill: blue } }',
  '.a { :hover { fill: red } }',
  '.a { p { fill: red } rect { opacity: 0.5 } text-align: center; }',
  '.edgeLabel { background-color: #fff; p { background-color: #fff; } rect { opacity: 0.5; background-color: #fff; fill: #fff; } text-align: center; }',
  'g.classGroup text { fill: #9370DB; stroke: none; font-family: x; font-size: 10px; .title { font-weight: bolder; } }',
  '.state-note { stroke: a; fill: b; text { fill: c; stroke: none; font-size: 10px; } }',
  // Comments.
  '/* a */',
  '/**/',
  '/* a */ .a { fill: red }',
  '.a { /* a */ fill: red }',
  '.a { fill: /* a */ red }',
  '.a { fill: red /* a */ }',
  '.a { fill: red; /* a */ }',
  '.a /* a */ .b { fill: red }',
  '.a/* a */.b { fill: red }',
  '.a /* a */{ fill: red }',
  '.a { fill:/* a */red }',
  '.a { fill: re/* a */d }',
  '/* a { fill: red } */ .b { fill: blue }',
  '/* unterminated',
  '.a { fill: red } /* unterminated',
  '/*/ .a { fill: red }',
  '/* * / */ .a { fill: red }',
  '/* a **/ .a { fill: red }',
  '// line comment\n.a { fill: red }',
  '.a { fill: red; // line comment\n stroke: blue }',
  '.a { // line comment\n fill: red }',
  '.a { fill: red // no newline at the end }',
  '// only a line comment',
  '//',
  '/',
  '.a { fill: red; // background-color:\n }',
  '.a { color: red; // ${x};\n}',
  '.a { // 50%/ odd\n fill: red }',
  '.a { //%/ fill: red }',
  '.a { background: url(a.png) // comment\n ; }',
  '.a { width: calc(1px / 2) }',
  '.a { width: calc(1px/2) }',
  '.a { font: 12px/1.5 a }',
  '.a { font: 12px / 1.5 a }',
  '.a { fill: red; /* \n multi \n line \n */ stroke: blue }',
  '.a { /* .cluster div { color: red; } */ }',
  '.a{/*!important*/fill:red}',
  // At-rules.
  '@media print { .a { fill: red } }',
  '@media (min-width: 100px) and (max-width:200px) { .a { fill: red } }',
  '@media print{.a{fill:red}.b{fill:blue}}',
  '@media print { fill: red }',
  '@media print { fill: red; color: blue }',
  '@media print { fill: red; .a { color: blue } }',
  '@media print { @media screen { .a { fill: red } } }',
  '.a { @media print { fill: red } }',
  '.a { @media print { fill: red; .b { fill: blue } } }',
  '.a, .b { @media print { fill: red } }',
  '.a { fill: red; @media print { fill: blue } stroke: green }',
  '@media print {}',
  '@media print { }',
  '@media',
  '@media;',
  '@media {',
  '@supports (display: grid) { .a { display: grid } }',
  '@supports not (display: grid) { .a { float: left } }',
  '.a { @supports (display: grid) { display: grid } }',
  '@layer base { .a { fill: red } }',
  '@layer base, components;',
  '@layer base;',
  '@layer;',
  '@layer { .a { fill: red } }',
  '.a { @layer base { fill: red } }',
  '@container (min-width: 100px) { .a { fill: red } }',
  '@container sidebar (min-width: 100px) { .a { fill: red } }',
  '.a { @container (min-width: 100px) { fill: red } }',
  '@scope (.a) to (.b) { .c { fill: red } }',
  '@scope (.a) { & { fill: red } }',
  '.a { @scope (.b) { fill: red } }',
  '@starting-style { .a { opacity: 0 } }',
  '.a { @starting-style { opacity: 0 } }',
  '@keyframes spin { from { transform: rotate(0deg) } to { transform: rotate(360deg) } }',
  '@keyframes edge-animation-frame { from { stroke-dashoffset: 0; } }',
  '@keyframes dash { to { stroke-dashoffset: 0; } }',
  '@keyframes k { 0% { opacity: 0 } 50%, 75% { opacity: .5 } 100% { opacity: 1 } }',
  '@keyframes k {}',
  '@keyframes k { from {} }',
  '.a { @keyframes k { from { opacity: 0 } } }',
  '@-webkit-keyframes k { from { opacity: 0 } }',
  '@-moz-document url-prefix() { .a { fill: red } }',
  '@import url("x.css");',
  '@import "x.css";',
  '@import url(x.css) screen;',
  '@import url(x.css); .a { fill: red }',
  '.a { fill: red } @import url(x.css);',
  '@charset "utf-8";',
  '@namespace svg url(http://www.w3.org/2000/svg);',
  '@namespace url(http://www.w3.org/1999/xhtml);',
  '@font-face { font-family: a; src: url(a.woff) }',
  '@page { margin: 1cm }',
  '@page :first { margin: 1cm }',
  '@document url(http://a/) { .a { fill: red } }',
  '@counter-style x { system: cyclic; symbols: "a"; }',
  '@font-feature-values Font One { @styleset { nice-style: 12; } }',
  '@property --x { syntax: "<length>"; inherits: false; initial-value: 0px }',
  '@viewport { width: device-width }',
  '@unknown { .a { fill: red } }',
  '@unknown foo;',
  '@unknown;',
  '@',
  '@;',
  '@{}',
  '@ media print { .a { fill: red } }',
  '@media print { .a { fill: red } } @media screen { .b { fill: blue } }',
  '@media print { @keyframes k { from { opacity: 0 } } }',
  '@media print { @import url(x); .a { fill: red } }',
  '@supports (--css: variables) { .a { --x: 1 } }',
  '@media (min-width:100px){@supports (display:grid){.a{fill:red}}}',
  '@media print { .a { .b { fill: red } } }',
  '@media print { .a, .b { &.c { fill: red } } }',
  '@container style(--x: 1) { .a { fill: red } }',
  '@cantainer x { .a { fill: red } }',
  '@lazyer x { .a { fill: red } }',
  '@layout x { .a { fill: red } }',
  '@color-profile --x { src: url(a) }',
  '@dummy x { .a { fill: red } }',
  '@swash { a: 1 }',
  // An at-rule name of 64 characters makes "{" equal to 59 + offset.
  `@${'m'.repeat(63)} { .a { fill: red } }`,
  `@${'m'.repeat(62)} { .a { fill: red } }`,
  `@${'m'.repeat(64)} { .a { fill: red } }`,
  `@${'m'.repeat(65)} x: y; .a { fill: red }`,
  // !important.
  '.a { fill: red !important }',
  '.a { fill: red!important }',
  '.a { fill: red ! important }',
  '.a { fill: red !important; stroke: blue ! important ; }',
  '.a { stroke-dasharray: 9,5 !important; }',
  '.a{fill:red !important;}',
  '.a { fill: red !IMPORTANT }',
  '.a ! .b { fill: red }',
  // Strings.
  '.a { content: "x" }',
  ".a { content: 'x' }",
  '.a { content: "a;b{c}d" }',
  ".a { content: 'a;b{c}d' }",
  '.a { content: "a\\"b" }',
  ".a { content: 'a\\'b' }",
  '.a { content: "a\'b" }',
  '.a { content: "  spaced   out  " }',
  '.a { content: "unterminated }',
  '.a { content: "a" "b" }',
  '.a { content: "a""b" }',
  '.a { content: "/* not a comment */" }',
  '.a { content: "// not a comment" }',
  '.a { font-family: "trebuchet ms", verdana, arial, sans-serif; }',
  ".a { font-family: 'trebuchet ms', verdana, arial, sans-serif; }",
  '.a { font-family: "trebuchet ms", verdana, arial, sans-serif;; }',
  '.a { grid-template-areas: "a b" "c d" }',
  '.a[data-x="y z"] { fill: red }',
  ".a[data-x='y z'] { fill: red }",
  '.a[data-x="}"] { fill: red }',
  '.a[data-x=";"] { fill: red }',
  '.a[data-x="]"] { fill: red }',
  '.a[data-x] { fill: red }',
  '.a[ data-x = "y" ] { fill: red }',
  '.a[data-x="y" i] { fill: red }',
  '[data-look="neo"].node rect, [data-look="neo"].cluster rect, [data-look="neo"].node polygon { stroke: a; filter: none; }',
  '[data-look="neo"][data-color-id="color-0"].cluster:not(.swimlane) rect { stroke: a; fill: b; }',
  '[id$="-barbEnd"] { fill: red }',
  'defs [id$="-barbEnd"] { fill: red }',
  '[id$="-compositionStart"], .composition { fill: red !important }',
  '[data-look=neo].labelBkg { fill: red }',
  ".x path[stroke='none'] { fill: red }",
  '[unterminated { fill: red }',
  '.a[b { fill: red } .c { fill: blue }',
  // url() and other functions.
  '.a { background: url(a.png) }',
  '.a { background: url( a.png ) }',
  '.a { background: url("a.png") }',
  ".a { background: url('a b.png') }",
  '.a { background: url(a.png) no-repeat, url(b.png) }',
  '.a { background: url(data:image/png;base64,AAAA//BB==) }',
  '.a { background: url("data:image/svg+xml;utf8,<svg xmlns=\'http://www.w3.org/2000/svg\'></svg>") }',
  '.a { background: url(http://a/b?c=d;e={f}) }',
  '.a { stroke: url(#my-svg-gradient); }',
  '.a { filter: url(#my-svg-drop-shadow) }',
  '.a { filter: drop-shadow(1px 2px 2px rgba(185, 185, 185, 1)); }',
  '.a { filter: drop-shadow( 0 0 2px rgba( 0 , 0 , 0 , .5 ) ) }',
  '.a { fill: rgba(0,0,0,0.2) }',
  '.a { fill: rgba( 0, 0, 0, 0.2 ) }',
  '.a { fill: rgb(0 0 0 / 0.4) }',
  '.a { fill: hsl(240, 60%, 86.2745098039%) }',
  '.a { width: calc(100% - (2 * 10px)) }',
  '.a { width: calc( 100%  -  10px ) }',
  '.a { transform: rotate(45deg) scale(0.8,0.8); }',
  '.a { transform: translate( 1px , 2px ) }',
  '.a { font-family: var(--mermaid-font-family); }',
  '.a { font-family: var(--mermaid-usecase-font-family, "trebuchet ms", verdana, arial, sans-serif); }',
  '.a { color: var( --x , red ) }',
  '.a { box-shadow: rgba(50, 50, 93, 0.25) 0px 13px 27px -5px, rgba(0, 0, 0, 0.3) 0px 8px 16px -8px; }',
  '.a { background: url( }',
  '.a { background: url(a.png }',
  '.a { width: calc(1px + }',
  '.a { fill: ) }',
  '.a { fill: red) }',
  '.a { fill: (red }',
  '.a { fill: a(b(c(d))) }',
  '.a { fill: a( b; c ) }',
  '.a { fill: a( { } ) }',
  '.a { fill: a( b } c ) ; stroke: blue }',
  // Pseudo classes and elements.
  '.a:hover { fill: red }',
  '.a::before { content: "" }',
  '.a:not(.b) { fill: red }',
  '.a:not( .b , .c ) { fill: red }',
  '.a:is(.b, .c) .d { fill: red }',
  '.a:where(.b,.c){fill:red}',
  '.a:nth-child(2n + 1) { fill: red }',
  '.a:nth-of-type(2) { fill: red }',
  '.a:first-of-type { fill: red }',
  '.cluster:not(.swimlane) rect { fill: red }',
  '.swimlane.cluster:not([data-color-id]) rect { stroke: red !important; }',
  '.a:has(> .b) { fill: red }',
  ':root { --mermaid-font-family: "trebuchet ms", verdana, arial, sans-serif; }',
  ':root { --mermaid-font-family: courier; }',
  ':root{--x:1}',
  ':host { fill: red }',
  'a:hover, a:focus { fill: red }',
  '.pieCircle.highlightedOnHover:hover{ transition-duration: 250ms; scale: 1.05; }',
  ':global(.a) { fill: red }',
  '.a :global(.b) { fill: red }',
  // Custom properties.
  '.a { --x: 1px }',
  '.a { --x:1px; --y : 2px }',
  '.a { --x: }',
  '.a { --x:; }',
  '.a { --x: ; fill: red }',
  '.a { --x: { a: b } }',
  '.a { --x: { a: b }; fill: red }',
  '.a { --x: {a:b;c:d}; fill: red }',
  '.a { --x: { { } }; fill: red }',
  '.a { --x: }; fill: red }',
  '.a { --x: [a, b]; }',
  '.a { --x: "a;b"; }',
  '.a { --X-y_Z: A b  C; }',
  '.a { -x: 1 }',
  '.a { -webkit-box-orient: vertical; display: -webkit-box; }',
  '.a { --: 1 }',
  '.a { ---x: 1 }',
  '--x: 1;',
  '--x: { a: b };',
  '.a { --x: 1; .b { --y: 2 } }',
  '.a { margin: -1px; --x: -1px }',
  '.a { a--b: 1 }',
  // Escapes.
  '.a\\:b { fill: red }',
  '.a\\.b { fill: red }',
  '.a\\{b { fill: red }',
  '.a\\;b { fill: red }',
  '.\\31 23 { fill: red }',
  '.\\31 { fill: red }',
  '.\\000031 23 { fill: red }',
  '.\\0000311 { fill: red }',
  '.\\31\t23 { fill: red }',
  '.a { content: "\\201C" }',
  '.a { content: \\201C }',
  '.a { content: \\201C  x }',
  '.a { fill: \\ red }',
  '.a\\ b { fill: red }',
  '.a\\',
  '\\',
  '\\{',
  '.a { fill: red\\; stroke: blue }',
  '.a { fill: r\\65 d }',
  '.a { fill: \\g }',
  '.a { fill: \\abcdefg }',
  '.a { fill: \\abcde f }',
  '.a { fill: \\abcdef  g }',
  // Combinators and odd selectors.
  '.a>.b{fill:red}',
  '.a+.b{fill:red}',
  '.a~.b{fill:red}',
  '.a > .b + .c ~ .d { fill: red }',
  '> .a { fill: red }',
  '+ .a { fill: red }',
  '~ .a { fill: red }',
  '* { fill: red }',
  '*.a { fill: red }',
  '.a * { fill: red }',
  'svg|rect { fill: red }',
  '*|* { fill: red }',
  '.a || .b { fill: red }',
  '.label text,span { fill: red }',
  '.rough-node .label text , .node .label text, .image-shape .label, .icon-shape .label { text-anchor: middle; }',
  '.node rect,\n  .node circle,\n  .node ellipse,\n  .node polygon,\n  .node path { fill: a; }',
  'text.actor > tspan { fill: red }',
  '.labelText, .labelText > tspan { fill: red }',
  'g rect.rect { fill: red }',
  '.a,,.b { fill: red }',
  ',.a { fill: red }',
  '.a, { fill: red }',
  ', { fill: red }',
  '.a{fill:red},.b{fill:blue}',
  '#id { fill: red }',
  '#id .a { fill: red }',
  '#id>.a { fill: red }',
  '#id > .a { fill: red }',
  '#id.a { fill: red }',
  '#idx .a { fill: red }',
  '#id || .a { fill: red }',
  '#id, .a { fill: red }',
  '#id { font-size: 1px }',
  '#id { .a { fill: red } }',
  'div.mermaidTooltip { position: absolute; z-index: 100; }',
  '.statediagram-cluster.statediagram-cluster .inner { fill: red }',
  '.a { rx:0; ry:0; }',
  '.a { stroke: red;; stroke-width: 1px; }',
  '.a { fill: red; } ; .b { fill: blue }',
  '.a { fill: red } } .b { fill: blue }',
  '.a { { fill: red } }',
  '{ fill: red }',
  '.a {{ fill: red }}',
  '.a { .b { fill: red }',
  '.a { fill: red; .b {',
  // Values with odd white space and tokens.
  '.a { margin: 0 auto }',
  '.a { margin:0   auto;padding :  1px  2px }',
  '.a { font: italic bold 12px/30px Georgia, serif }',
  '.a { stroke-dasharray: 2, 2; }',
  '.a { stroke-dasharray: 10,10; }',
  '.a { transition: a 1s , b 2s }',
  '.a { fill: red\n}',
  '.a { fill:\nred;\nstroke:\n\tblue\n}',
  '.a { fill : red ; stroke : blue }',
  '.a { width: 100% }',
  '.a { width: 1e3px }',
  '.a { content: "a" attr(x) "b" }',
  '.a { quotes: "<" ">" }',
  '.a { fill: #abc }',
  '.a { fill: red;}.b{fill:blue;}',
  '.a { opacity : 0.5 }',
  '.a { stroke-width : 2 ; opacity : 1 }',
  '.a { x :y }',
  '.a { x\t:\ty }',
  '.a { x\n:\ny }',
  '.a{b:c;d:e}',
  '.a { fill: undefined; stroke: null; width: NaNpx }',
  '.a { fill: ; stroke: }',
  '.a { filter: ; }',
  '.a { ${x} }',
  '.a { fill: ${x}; }',
  '.a { }\n\n\n.b { fill: red }',
  '\ufeff.a { fill: red }',
  '.a\u00a0.b { fill: red }',
  '.\u00e9 { fill: red }',
  '.a { content: "\u4e2d\u6587 \ud83d\ude00" }',
  '.\ud83d\ude00 { fill: red }',
  '.a { fill: red }\u0000.b { fill: blue }',
  '.a { fill: r\u0000ed }',
  '.a\f.b { fill: red }',
  '.a { fill: red\f }',
  '.a\u000b.b { fill: red }',
  '.a\r\n.b\r\n{\r\nfill: red;\r\n}',
  // Replacement patterns of String.prototype.replace in the parent selector.
  ".a$' { & .b { fill: red } }",
  '.a$& { &.b { fill: red } }',
  '.a$$ { & > .b { fill: red } }',
  '.a$` { .b & { fill: red } }',
  '.a$1 { &:hover { fill: red } }',
  '.a$<x> { & .b { fill: red } }',
  '[id$="x"] { &.b { fill: red } }',
  '.a$ { & { fill: red } }',
];

// Cases where stylis 4.3.6 (the mermaid bundle) and stylis 4.4.0 (node_modules)
// differ. See the diff of src/Parser.js between the two versions.
const VERSION = [
  // A custom property that has no value. 4.3.6 makes the declaration only when
  // a comment is the last thing before the end of the value.
  '.a { --x:; }',
  '.a{--x:}',
  '.a { --x: ; }',
  '.a { --x: ; fill: red }',
  '.a{--x:;--y:1}',
  '.a { --x:/**/; }',
  '.a { --x:/* c */; fill: red }',
  '.a { --x: /* c */ ; fill: red }',
  '.a { --x:/* c */ ; fill: red }',
  '.a { --x:// c\n; fill: red }',
  '.a { --x:/; }',
  '.a { --x:/**/ }',
  '.a { --x:/**/',
  '--x:;',
  '--x:/**/;',
  ':root{--mermaid-font-family:;}',
  '& { --x:; fill: red }',
  '@media print { .a { --x:; } }',
  '.a { .b { --x:; } stroke: blue }',
  '.a { --:; }',
  '.a { --x:;;; }',
  // A pseudo-class function. 4.3.6 takes the text up to the closing parenthesis
  // as it is, and marks only the first "&" in it as a reference to the parent.
  '.a:not(/* c */.b) { fill: red }',
  '.a:not(.b /* c */) { fill: red }',
  '.a:not(.b/**/) { fill: red }',
  '.a:not(.b // c\n) { fill: red }',
  '.a:is(.b, /* c */ .c) .d { fill: red }',
  '.a:not(&, &.b) { fill: red }',
  '.a:not(&&) { fill: red }',
  '.a:is(& &) { fill: red }',
  '.x { &:not(&.y, &.z) { fill: red } }',
  '.x { &:not(&.y, &.z), &:is(&, &) { fill: red } }',
  '.x { .a:is(&, & .b) { fill: red } }',
  '.x, .y { &:is(&.a, &.b) > & { fill: red } }',
  '.x { .a:not(.b &, .c &) { fill: red } }',
  '.x { :is(&, &) :is(&, &) { fill: red } }',
  '.x { .a, .b:not(&, &) { fill: red } }',
  '.x { .a:not(.b), &:not(&, &) { fill: red } }',
  '.x { &:not(.a), .b:not(&, &) { fill: red } }',
  '@media print { .x { &:not(&.y, &.z) { fill: red } } }',
  '.x$& { &:not(&.y, &.z) { fill: red } }',
  // The text in the parentheses does not end.
  '.a:not(',
  '.a:not(.b',
  '.a:not(.b { fill: red }',
  '.a:not(.b, .c { fill: red } .d { stroke: blue }',
  ':is(:not(',
  '.a:not(.b;',
  '.a:not(.b}',
  '.a { .b:is(.c { fill: red } }',
  '.a:not(\r',
  '.a:not( ',
  '.a:is(\n\t',
  // White space, strings, escapes and blocks in the parentheses.
  '.a:not(  .b  ,\n\t.c  ) { fill: red }',
  '.a:not(.b;.c) { fill: red }',
  '.a:not(.b{.c}) { fill: red }',
  '.a:not({) { fill: red }',
  '.a:not(}) { fill: red }',
  '.a:not(")") { fill: red }',
  '.a:not([x=")"]) { fill: red }',
  '.a:not(.b\\).c) { fill: red }',
  '.a:not(:not(:not(&, &))) { fill: red }',
  '.a:is(.b:hover, .c:focus) { fill: red }',
  '.a:is(.b:hover, .c:focus) { fill: red; .d { stroke: blue } }',
  '.a:is(@media) { fill: red }',
  '.a:is(--x) { fill: red }',
  '.a:is(.b, .c)) { fill: red }',
  '.a) { fill: red }',
  '.a { fill: ) ; stroke: blue }',
  // "l(" is not a pseudo-class function for stylis: the "l" is that of url(.
  '.a:nth-col(&, &) { fill: red }',
  '.x { .a:nth-col(&, &) { fill: red } }',
  '.a { background:url(a.png) }',
  '.a { background: url( a /* c */ ) }',
  // A value that is a pseudo-class function.
  '.a { fill:not(/* c */ red) }',
  '.a { fill:is(&, &) }',
  '.a { fill: not(/* c */ red) }',
  '.a { width:calc(1px ',
  '.a { width: calc(1px + ',
];

// Fragments for the random cases of the group "version".
const VERSION_FRAGMENTS = [
  ':not(', ':not(', ':is(', ':is(', ':where(', ':has(', '::part(', ':', ':', '(', ')', ')', ')', '&', '&', '& ', ',',
  ' ', ' ', '\n', '\t', '/* c */', '/**/', '// c\n', '/', '--x:', '--y: ', '--', ';', ';', '{', '{', '}', '}', '.a',
  '.b', '#id', 'fill:red', 'fill: red', 'fill', '"', "'", '")"', '[x=")"]', '[', ']', '\\)', '\\', '@media print',
  '@supports (a:b)', 'url(', 'l(', '\f', '$&', '>', '+',
];

// Namespaces. The second group makes the namespace a replacement string.
const NAMESPACES = ['#id', '#my-svg', '#mermaid-0'];
const ODD_NAMESPACES = ['#a$&b', "#a$'b", '#a$$b', '#a$`b', '.ns', '#id > .x', '#\u00e9', ''];

// Fragments for the random cases.
const FRAGMENTS = [
  '{', '}', '{', '}', ';', ';', ':', ':', ',', '&', '&', '(', ')', '[', ']', '"', "'", '\\', '/', '*',
  '/*', '*/', '//', '\n', ' ', ' ', ' ', '\t', '@', '@media', '@media print', '@supports', '@layer', '@container',
  '@scope', '@keyframes k', '@import', '@font-face', '@starting-style', '@-webkit-keyframes', '!important', '!',
  'url(', 'url(a.png)', 'var(--x)', 'calc(1px + 2px)', '--x', '--', '-', '.a', '.b', '#id', '#x', 'rect', 'fill',
  'stroke', 'red', '1px', '0.5', '100%', '>', '+', '~', '||', '|', '=', '$', "$'", '$&', '::before', ':hover',
  ':not(', ':is(', 'from', 'to', '50%', '\f', '\r', '\u00e9', '\ud83d\ude00', '\\31 ', '\\{', 'l(', 'a(', 'and',
  '(min-width: 1px)', 'font-size', 'font-family', 'x', 'y', '0', '9',
];

const cases = [];
const seen = new Set();
function add(group, ns, css, withAst) {
  const key = `${ns}\u0001${css}`;
  if (seen.has(key)) return;
  seen.add(key);
  cases.push({ group, ns, css, withAst });
}

for (const css of HAND) {
  add('hand', '#id', css, true);
}
HAND.forEach((css, i) => {
  add('hand', NAMESPACES[1 + (i % 2)], css, false);
});
ODD_NAMESPACES.forEach((ns) => {
  for (const css of ['.a { fill: red }', '& { fill: red }', '&.a, & > .b { fill: red; .c & { stroke: blue } }', '.a { &.b { fill: red } }', '@media print { .a { fill: red } }', '& { font-size: 1px }']) {
    add('namespace', ns, css, false);
  }
});

const rnd = mulberry32(0x7374796c);
const pick = (list) => list[Math.floor(rnd() * list.length)];
for (let i = 0; i < 4000; i++) {
  const n = 1 + Math.floor(rnd() * 24);
  let css = '';
  for (let j = 0; j < n; j++) css += pick(FRAGMENTS);
  add('fuzz', '#id', css, true);
}
// Random edits of valid CSS reach the states that pure noise does not.
for (let i = 0; i < 3000; i++) {
  let css = pick(HAND);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let j = 0; j < edits; j++) {
    const at = Math.floor(rnd() * (css.length + 1));
    css = rnd() < 0.5 ? css.slice(0, at) + pick(FRAGMENTS) + css.slice(at) : css.slice(0, at) + css.slice(at + 1 + Math.floor(rnd() * 3));
  }
  add('mutation', '#id', css, true);
}

// The inputs where the bundled stylis and the stylis of node_modules differ.
for (const css of VERSION) {
  add('version', '#id', css, true);
}
for (let i = 0; i < 2500; i++) {
  const n = 2 + Math.floor(rnd() * 14);
  let css = '';
  for (let j = 0; j < n; j++) css += pick(VERSION_FRAGMENTS);
  add('version', '#id', css, true);
}
for (let i = 0; i < 1500; i++) {
  let css = pick(VERSION);
  const edits = 1 + Math.floor(rnd() * 3);
  for (let j = 0; j < edits; j++) {
    const at = Math.floor(rnd() * (css.length + 1));
    css = rnd() < 0.5 ? css.slice(0, at) + pick(VERSION_FRAGMENTS) + css.slice(at) : css.slice(0, at) + css.slice(at + 1 + Math.floor(rnd() * 3));
  }
  add('version', '#id', css, true);
}

// The style text that mermaid makes for every diagram type, with several
// themes and looks. This is the input that stylis gets in a real render.
const TYPES = [
  'error', 'flowchart-elk', 'mindmap', 'architecture', 'agentflow', 'c4', 'kanban', 'classDiagram', 'er', 'gantt',
  'info', 'pie', 'requirement', 'sequence', 'swimlane', 'flowchart-v2', 'timeline', 'gitGraph', 'stateDiagram',
  'journey', 'quadrantChart', 'sankey', 'packet', 'xychart', 'block', 'eventmodeling', 'treeView', 'radar',
  'ishikawa', 'treemap', 'railroad', 'railroadEbnf', 'railroadAbnf', 'railroadPeg', 'venn', 'wardley', 'cynefin',
  'usecase',
];
const THEMES = Object.keys(api.themes_default);
const LOOKS = ['classic', 'neo', 'handDrawn'];
let templateCount = 0;
for (const type of TYPES) {
  await loadDiagram(api, type);
  for (const theme of THEMES) {
    for (const look of LOOKS) {
      const text = `---\nconfig:\n  theme: ${theme}\n  look: ${look}\n---\n`;
      api.processAndSetConfigs(text);
      const config = api.getConfig();
      const userStyles = api.createCssStyles(config, undefined);
      const css = api.styles_default(type, userStyles, { ...config.themeVariables, theme: config.theme, look: config.look }, '#my-svg');
      const before = cases.length;
      add('template', '#my-svg', css, false);
      templateCount += cases.length - before;
    }
  }
}

// node -> plain tree. Line and column are kept, so the Go test can check them
// for ASCII input.
function tree(elements) {
  return elements.map((e) => ({
    t: e.type,
    v: e.value,
    p: e.props,
    c: Array.isArray(e.children) ? tree(e.children) : e.children,
    n: e.length,
    l: e.line,
    k: e.column,
  }));
}

let failures = 0;
let differ = 0;
const out = [];
for (const c of cases) {
  const rec = { group: c.group, ns: c.ns, css: c.css };
  try {
    if (c.withAst) {
      rec.ast = tree(compile(c.css));
      rec.plain = serialize(compile(c.css), stringify);
      const newer = newerStylis.serialize(newerStylis.compile(c.css), newerStylis.stringify);
      if (newer !== rec.plain) {
        rec.newer = newer;
        differ++;
      }
    }
    rec.out = api.compileCSS(c.ns, c.css);
  } catch (e) {
    failures++;
    rec.error = String(e && e.message);
  }
  out.push(rec);
}

const file = path.join(REPO, 'internal/stylis/testdata/stylis.json.gz');
fs.mkdirSync(path.dirname(file), { recursive: true });
const json = JSON.stringify({ stylis: bundledVersion('stylis'), newer: newerVersion, cases: out });
fs.writeFileSync(file, zlib.gzipSync(json, { level: 9 }));
const groups = {};
for (const c of out) groups[c.group] = (groups[c.group] || 0) + 1;
console.error(
  `wrote ${file}: ${out.length} cases ${JSON.stringify(groups)}, ${failures} threw, ${json.length} bytes of JSON; ` +
    `stylis ${bundledVersion('stylis')}, ${differ} cases differ from stylis ${newerVersion}`
);
