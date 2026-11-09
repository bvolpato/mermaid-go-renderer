// Writes internal/diagrams/kanban/testdata/render.json.gz: the SVG that
// mermaid.render of mermaid 12.2.0 returns in Chromium for hand-written
// kanban texts, or the error. Every fixture of the corpus has all its nodes
// at one indentation, so each node is a section and no fixture has an item.
// These texts have items, with the shape data and the config options that
// the renderer and the item shape read.
//
// The script runs the render command of the browser oracle one time, with
// one page for each text, as the reference renders do.
//
// Usage (in tools/reference): node gen-kanban-render.mjs
import { renderExtras, withConfig } from './extra-render-lib.mjs';

const kb = (body) => 'kanban\n' + body + '\n';
const board = kb(
  'todo[Todo]\n' +
    '  docs[Create Documentation]\n' +
    "  blog[Create Blog about the new diagram]@{ assigned: 'knsv' }\n" +
    'doing[In progress]\n' +
    "  render[Create renderer so that it works in all cases. We also add some extra text here for testing purposes. And some more just for the extra flare.]@{ ticket: MC-2037, assigned: knsv, priority: 'High' }\n" +
    "  grammar[Design grammar]@{ priority: 'Very Low' }\n" +
    'ready[Ready for deploy]\n' +
    "  db[Update DB function]@{ ticket: MC-2038, priority: 'Very High' }\n" +
    "  tests[Create parsing tests]@{ ticket: MC-2039, assigned: 'K.Sveidqvist', priority: 'Low' }\n" +
    'done[Done]\n' +
    "  get[define getData]@{ priority: 'Medium' }\n" +
    "  title[Title of diagram is more than 100 chars when user duplicates diagram with 100 char]@{ ticket: MC-2036, priority: 'Very High'}\n" +
    'empty[An empty section]'
);
const small = kb("todo[Todo]\n  a[First item]@{ ticket: T-1, assigned: me, priority: High }\n  b[Second item]\ndone[Done]\n  c[Third item]@{ assigned: you }");

const cases = [];
const add = (name, text) => cases.push({ name, text });

add('board', board);
add('small', small);
add('empty', 'kanban\n');
add('one-section', kb('todo[Todo]'));
add('one-item', kb('todo[Todo]\n  a[Item]'));
add('no-brackets', kb('todo\n  first item\n  second item\ndone\n  third item'));
add('nested-levels', kb('todo[Todo]\n  a[Item A]\n      deep[Deeper item]\n    mid[Middle item]\n  b[Item B]'));
add('duplicate-section-ids', kb('docs[First]\n  a[Item A]\ndocs[Second]\n  b[Item B]'));
add('duplicate-item-ids', kb('todo[Todo]\n  a[One]\n  a[Two]\ndoing[Doing]\n  a[Three]'));
add('long-section-titles', kb('todo[A section with a title that is long enough to wrap over more than one line of text]\n  a[Item A]\n  b[Item B]\ndone[Done]\n  c[Item C]'));
add('long-items', kb('todo[Todo]\n  a[An item with a text that is long enough to wrap over more than one line of the item, and then some more words]@{ ticket: MC-1, assigned: knsv }\n  b[Short]\n  c[Another long item text that needs several lines to show all of its words in the column]@{ priority: Low }'));
add('priorities', kb("id2[In progress]\n  vh[Very High]@{ priority: 'Very High' }\n  h[High]@{ priority: 'High' }\n  m[Default priority]\n  md[Medium]@{ priority: 'Medium' }\n  l[Low]@{ priority: 'Low' }\n  vl[Very Low]@{ priority: 'Very Low' }\n  u[Unknown]@{ priority: 'Unknown' }\n  n[Number]@{ priority: 3 }"));
add('tickets', kb('todo[Todo]\n  a[Item A]@{ ticket: MC-2037 }\n  b[Item B]@{ ticket: 2038 }\n  c[Item C]@{ ticket: A very long ticket id that does not fit in one line of an item }'));
add('assigned', kb("todo[Todo]\n  a[Item A]@{ assigned: 'knsv' }\n  b[Item B]@{ assigned: 5 }\n  c[Item C]@{ assigned: A very long name of a person that does not fit in one line }\n  d[Item D]@{ assigned: [a, b] }"));
add('label-data', kb('todo[Todo]@{ label: Section from data }\n  a[Item A]@{ label: Item from data }\n  b[Item B]@{ label: 5 }\n  c[Item C]@{ label: [x, y] }\n  d[Item D]@{ label: "**bold** and *italic*" }'));
add('section-data', kb("todo[Todo]@{ ticket: T-1, assigned: me, priority: High, icon: book }\n  a[Item A]"));
add('icons-classes', kb('todo[Todo]\n  a[Item A]\n  ::icon(fa fa-book)\n  b[Item B]\n  :::urgent large\n  c[Item C]@{ icon: fa fa-fire, assigned: me }'));
add('shapes', kb('todo(Rounded)\n  a((Circle))\n  b))Bang((\n  c)Cloud(\n  d{{Hexagon}}\n  [No id]\n  e["Quoted text"]\n  f["`Markdown **bold** string`"]'));
add('markdown', kb('todo[A **bold** section]\n  a[An *italic* item with `code`]\n  b[line1<br/>line2<br>line3]@{ assigned: "**me**" }\n  c["`A markdown\n  string on two lines`"]'));
add('entities', kb('todo[#35; hash #lt;b#gt; #9829;]\n  a[#quot;quoted#quot; #amp; more]@{ ticket: "#35;1" }\n  b[<b>bold</b> <i>it</i> <script>alert(1)</script>]'));
add('unicode-latin-cjk', kb('todo[Ünïcödé 日本語]\n  a[Ünïcödé item]@{ assigned: 日本語, ticket: Ü-1 }\n  b[日本語の項目]'));
add('unicode-cyrillic', kb('todo[Доска]\n  a[емкость и ещё слова]@{ assigned: Иван, ticket: Ж-1 }'));
add('unicode-emoji', kb('todo[Emoji 😀]\n  a[An item 😀 with 🚀 emoji]@{ assigned: 😀 }'));
add('unicode-hebrew', kb('todo[שלום]\n  a[שלום עולם]@{ assigned: שלום }'));
// Arabic text is not here: the text measurement of the port gives another
// width for it than Chromium (the word "مرحبا" at 16px: 28.92 and 30.19).
// That is a matter of the layout engine and not of the kanban diagram.
add('multiline-data', kb('todo[Todo]\n  a[Item A]@{\n    assigned: knsv\n    ticket: MC-1\n    priority: Low\n  }\n  b[Item B]@{ assigned: "two\n    lines" }'));
add('hyphen-ticket', kb('todo[Todo]\n  a[Item A]@{ ticket: MC-2037-ABCD-1234-5678-EFGH-9012 }\n  b[Item-B with-hyphens-and-1-2-3]'));
add('error-shape', kb('todo[Todo]\n  a[Item A]@{ shape: kanbanItem }'));
add('error-items-without-section', kb('  todo\n    a\nless\nmore'));
add('error-yaml', kb('todo[Todo]\n  a[Item A]@{ a: [ }'));
add('error-grammar', kb('todo[unclosed'));

// Config.
const ticketUrl = "kanban:\n  ticketBaseUrl: 'https://example.com/browse/#TICKET#'";
add('config-ticket-url', withConfig(ticketUrl, board));
add('config-ticket-url-small', withConfig(ticketUrl, small));
add('config-ticket-url-no-placeholder', withConfig("kanban:\n  ticketBaseUrl: 'https://example.com/browse/'", small));
add('config-ticket-url-patterns', withConfig(ticketUrl, kb("todo[Todo]\n  a[Item A]@{ ticket: \"a$&b$$c\" }")));
add('config-ticket-url-script', withConfig("kanban:\n  ticketBaseUrl: 'javascript:alert(1)//#TICKET#'", small));
add('config-section-width-300', withConfig('kanban:\n  sectionWidth: 300', board));
add('config-section-width-120', withConfig('kanban:\n  sectionWidth: 120', board));
add('config-section-width-0', withConfig('kanban:\n  sectionWidth: 0', small));
add('config-kanban-padding', withConfig('kanban:\n  padding: 30\n  useMaxWidth: false', small));
add('config-mindmap-padding', withConfig('mindmap:\n  padding: 30\n  maxNodeWidth: 100', small));
add('config-mindmap-max-width-false', withConfig('mindmap:\n  useMaxWidth: false', small));
for (const look of ['classic', 'neo', 'handDrawn']) {
  add(`config-look-${look}`, withConfig(`look: ${look}`, board));
}
add('config-handdrawn-seed', withConfig('look: handDrawn\nhandDrawnSeed: 7', small));
for (const theme of ['default', 'dark', 'forest', 'neutral', 'base', 'redux', 'redux-dark', 'redux-color', 'redux-dark-color', 'neo', 'neo-dark']) {
  add(`config-theme-${theme}`, withConfig(`theme: ${theme}`, small));
}
add('config-theme-variables', withConfig('theme: base\nthemeVariables:\n  primaryColor: "#ff0000"\n  fontSize: 20px', small));
add('config-neo-redux', withConfig('look: neo\ntheme: redux', board));
add('config-html-labels-false', withConfig('htmlLabels: false', board));
add('config-html-labels-false-url', withConfig('htmlLabels: false\n' + ticketUrl, small));
add('config-flowchart-html-labels-false', withConfig('flowchart:\n  htmlLabels: false', small));
add('config-security-loose', withConfig('securityLevel: loose', kb('todo[<b>bold</b> section]\n  a[<i>it</i> <u>item</u>]@{ assigned: "<b>me</b>" }')));
add('config-security-antiscript', withConfig('securityLevel: antiscript', small));
add('config-font', withConfig('fontFamily: courier\nfontSize: 20', small));
add('config-wrapping-width', withConfig('flowchart:\n  wrappingWidth: 80', small));
add('config-markdown-auto-wrap', withConfig('markdownAutoWrap: false', kb('todo[A section with a title that is long enough to wrap over more than one line]\n  a[An item with a text that is long enough to wrap over more than one line of the item]')));
add('directive', '%%{init: {"kanban": {"sectionWidth": 250}, "look": "neo"}}%%\n' + small);
add('title-acc', '---\ntitle: A board\n---\n' + small);

renderExtras(cases, 'internal/diagrams/kanban/testdata/render.json.gz');
