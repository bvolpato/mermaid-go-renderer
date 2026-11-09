// The inputs of the golden files of the architecture diagram
// (gen-architecture-db.mjs and gen-architecture-layout.mjs): the fixtures of
// the corpus, seeded mutations of them, and hand-written inputs for the
// branches of architectureDb.ts, architectureRenderer.ts and svgDraw.ts.
//
// A case is { name, text, full, site }. `full` marks a case whose draw
// function runs in the db golden and that is a case of the layout golden.
// `site` is the site config, the argument of mermaid.initialize.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
export const repoRoot = path.resolve(here, '../..');
export const corpusDir = 'testdata/corpus/mermaid-e2e';
const family = 'architecture/';
const mutantsPerFixture = 10;

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

// ---------------------------------------------------------------------------
// The fixtures of the corpus.

const fixtures = fs
  .readdirSync(path.join(repoRoot, corpusDir), { recursive: true })
  .filter((f) => f.endsWith('.mmd'))
  .map((f) => f.split(path.sep).join('/'))
  .filter((f) => f.startsWith(family))
  .sort();

export const cases = [];
for (const f of fixtures) {
  cases.push({ name: `${corpusDir}/${f}`, corpus: f.replace(/\.mmd$/, ''), full: true, text: fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8') });
}

// ---------------------------------------------------------------------------
// Seeded mutations: edits of the lines after the line of the diagram keyword.

const snippets = [
  'service extra(server)[Extra]',
  'service extra2[No icon]',
  'service txt "T" [Icon text]',
  'service in_api(disk)[In api] in api',
  'service dup(server)[Dup]',
  'service dup(server)[Dup again]',
  'group g2(cloud)[Second]',
  'group nested[Nested] in g2',
  'group api(cloud)[API again]',
  'junction jx',
  'junction jy in g2',
  'extra:L -- R:extra2',
  'extra:T --> B:extra2',
  'extra:R <-[lbl]- T:extra2',
  'extra{group}:B -- T:jx',
  'jx:L -- R:jy{group}',
  'extra:L -- L:extra2',
  'nosuch:L -- R:extra',
  'extra:L -- R:nosuch',
  'g2:L -- R:extra',
  'align row extra extra2',
  'align column extra extra2 jx',
  'align row extra extra',
  'align row extra g2',
  'align column nosuch extra',
  'title A title',
  'accTitle: An accessible title',
  'accDescr: A description',
  '%% a comment',
  '',
];

function mutate(rnd, text) {
  const lines = text.split('\n');
  // Skip the front matter: its text can have a line that starts with the keyword.
  let from = 0;
  if (/^\s*---\s*$/.test(lines[0] ?? '')) {
    const close = lines.findIndex((l, i) => i > 0 && /^\s*---\s*$/.test(l));
    from = close < 0 ? 0 : close + 1;
  }
  let start = lines.findIndex((l, i) => i >= from && /^\s*architecture-beta/.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  // Most mutants declare the elements that the snippets name, so that the
  // edges and the align statements of the snippets reach the db.
  if (rnd() < 0.7) {
    lines.splice(start, 0, '  service extra(server)[Extra]', '  service extra2[No icon]', '  junction jx', '  group g2(cloud)[Second]', '  junction jy in g2');
  }
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 8);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind <= 5) {
      lines.splice(p, 0, '  ' + snippets[Math.floor(rnd() * snippets.length)]);
    } else if (kind === 6) {
      // Change one direction or one id of a line.
      const l = lines[p] ?? '';
      lines[p] = rnd() < 0.5 ? l.replace(/:([LRTB])/, () => ':' + 'LRTB'[Math.floor(rnd() * 4)]) : l.replace(/\b(in) (\w+)/, () => 'in ' + ['api', 'g2', 'nosuch', 'extra'][Math.floor(rnd() * 4)]);
    } else {
      lines.push('  ' + snippets[Math.floor(rnd() * snippets.length)]);
    }
  }
  return lines.join('\n');
}

const rnd = prng(0xa4c81db0);
for (const f of fixtures) {
  const text = fs.readFileSync(path.join(repoRoot, corpusDir, f), 'utf8');
  for (let i = 0; i < mutantsPerFixture; i++) {
    cases.push({ name: `${corpusDir}/${f}#mutant${i}`, text: mutate(rnd, text) });
  }
}

// ---------------------------------------------------------------------------
// Hand-written inputs.

const ab = (body, front = '') => front + 'architecture-beta\n' + body + '\n';
const cfg = (yaml) => '---\nconfig:\n' + yaml.split('\n').map((l) => '  ' + l).join('\n') + '\n---\n';
const small = 'group api(cloud)[API]\nservice db(database)[Database] in api\nservice server(server)[Server] in api\nservice gw(internet)[Gateway]\ndb:L -- R:server\nserver:T -- B:gw';
const extras = [
  // Empty diagrams and headers.
  { text: 'architecture-beta' },
  { text: 'architecture-beta\n' },
  { text: '\n\n   architecture-beta   \n\n\n' },
  { text: 'architecture-beta\n%% only a comment\n', full: true },
  { text: 'architecture\nservice a\n' },
  { text: 'architecture-betax\nservice a\n' },
  // One element of each kind.
  { text: ab('service a'), full: true },
  { text: ab('service a(server)'), full: true },
  { text: ab('service a[Title]'), full: true },
  { text: ab('service a(server)[Title]'), full: true },
  { text: ab('service a "Text"'), full: true },
  { text: ab('service a "Text" [Title]'), full: true },
  { text: ab('service a "" [Empty icon text]'), full: true },
  { text: ab('service a "A longer icon text that must wrap in the icon"'), full: true },
  { text: ab('service a[""]'), full: true },
  { text: ab('service a[ ]'), full: true },
  { text: ab('service a(nosuchicon)[Unknown]'), full: true },
  { text: ab('service a(logos:aws)[Other prefix]'), full: true },
  { text: ab('service a(mermaid-architecture:cloud)[Own prefix]'), full: true },
  { text: ab('service a(unknown)[The unknown icon]'), full: true },
  { text: ab('service a(blank)[The blank icon]'), full: true },
  { text: ab('service a(database)[One]\nservice b(database)[Two]\nservice c(disk)[Three]\nservice d(internet)[Four]\nservice e(cloud)[Five]'), full: true },
  { text: ab('service a(server)[A very long title that has to wrap over several lines below the icon]'), full: true },
  { text: ab('service a(server)[**bold** and *italic* `code`]'), full: true },
  { text: ab('service a(server)[line one<br/>line two<br>three]'), full: true },
  { text: ab('service a(server)[#35; hash #lt;b#gt; #9829;]'), full: true },
  { text: ab('service a(server)[<b>bold</b> <script>alert(1)</script>]'), full: true },
  { text: ab('service a(server)[Ünïcödé 日本語 😀]'), full: true },
  { text: ab('service a(server)["quoted [title] with \\" quote"]'), full: true },
  { text: ab('junction j'), full: true },
  { text: ab('group g'), full: true },
  { text: ab('group g(cloud)'), full: true },
  { text: ab('group g[Title]'), full: true },
  { text: ab('group g(cloud)[Title]'), full: true },
  { text: ab('group g(nosuchicon)[Unknown icon]\nservice a(server)[A] in g'), full: true },
  { text: ab('group g(database)[Icon with ids]\nservice a(database)[A] in g'), full: true },
  { text: ab('group g[""]\nservice a(server)[A] in g'), full: true },
  { text: ab('group g(cloud)[A very long group title that is wider than the group itself]\nservice a(server)[A] in g'), full: true },
  { text: ab('group default[Default]\nservice a(server)[A] in default\nservice b(server)[B]\na:R -- L:b'), full: true },
  // Ids.
  { text: ab('service 1(server)[One]\nservice 2(server)[Two]\nservice 10(server)[Ten]\n2:R -- L:1\n10:T -- B:2'), full: true },
  { text: ab('service a-b(server)\nservice c_d(server)\na-b:L -- R:c_d'), full: true },
  { text: ab('service a\nservice b\nservice a-b\na:L -- R:b'), full: true },
  // Parents.
  { text: ab('group g\nservice a in g'), full: true },
  { text: ab('service a in g\ngroup g'), full: true },
  { text: ab('group g\ngroup h in g\ngroup i in h\nservice a in i\njunction j in h'), full: true },
  { text: ab('group h in g\ngroup g') },
  { text: ab('service a in nosuch') },
  { text: ab('junction j in nosuch') },
  { text: ab('group g in nosuch') },
  { text: ab('service a in a') },
  { text: ab('junction j in j') },
  { text: ab('group g in g') },
  { text: ab('service a\nservice b in a') },
  { text: ab('service a\njunction j in a') },
  { text: ab('junction j\nservice a in j') },
  { text: ab('junction j\njunction k in j') },
  { text: ab('service a\ngroup g in a') },
  // Ids in use.
  { text: ab('service a\nservice a') },
  { text: ab('group a\nservice a') },
  { text: ab('service a\njunction a') },
  { text: ab('junction a\njunction a') },
  { text: ab('group a\njunction a') },
  { text: ab('group a\ngroup a') },
  { text: ab('junction a\nservice a\ngroup a') },
  // Edges.
  { text: ab('service a\nservice b\na:L -- R:b'), full: true },
  { text: ab('service a\nservice b\na:R --> L:b'), full: true },
  { text: ab('service a\nservice b\na:T <-- B:b'), full: true },
  { text: ab('service a\nservice b\na:B <--> T:b'), full: true },
  { text: ab('service a\nservice b\na:L -[Label]- R:b'), full: true },
  { text: ab('service a\nservice b\na:T -[Vertical label]- B:b'), full: true },
  { text: ab('service a\nservice b\na:L -[ ]- R:b'), full: true },
  { text: ab('service a\nservice b\na:L -- L:b'), full: true },
  { text: ab('service a\nservice b\na:T -- T:b'), full: true },
  { text: ab('service a\nservice b\na:L -- T:b'), full: true },
  { text: ab('service a\nservice b\na:L -[XY label]- T:b\na:R <-[Other]-> B:b'), full: true },
  { text: ab('service a\na:L -- R:a'), full: true },
  { text: ab('service a\nservice b\na:L -- R:b\na:L -- R:b'), full: true },
  { text: ab('service a\nservice b\na:L -- R:b\nb:L -- R:a'), full: true },
  { text: ab('service a\nservice b\na:L -- R:b\na:T -- B:b'), full: true },
  { text: ab('service a\nservice b\nservice c\na:R -- L:b\nb:R -- L:c\nc:B -- T:a'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\na:R -- L:b\nc:R -- L:d'), full: true },
  { text: ab('service a\njunction j\nservice b\na:R -- L:j\nj:R -- L:b\nj:B --> T:a'), full: true },
  { text: ab('junction j\njunction k\nj:R <--> L:k\nj:T -[J]- B:k'), full: true },
  { text: ab('a:L -- R:b\nservice a\nservice b'), full: true },
  { text: ab('service a\na:L -- R:nosuch') },
  { text: ab('service a\nnosuch:L -- R:a') },
  { text: ab('nosuch:L -- R:other') },
  { text: ab('group g\nservice a\ng:L -- R:a') },
  { text: ab('group g\nservice a\na:L -- R:g') },
  { text: ab('group g\ngroup h\ng:L -- R:h') },
  { text: ab('group g\ng:L -- R:nosuch') },
  // The group modifier.
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na{group}:R -- L:b{group}'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na{group}:B --> T:b'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na:T <-- B:b{group}'), full: true },
  { text: ab('group g\nservice a in g\nservice b\na{group}:R -- L:b{group}'), full: true },
  { text: ab('service a\nservice b\na{group}:R -- L:b{group}'), full: true },
  { text: ab('group g\nservice a in g\nservice b in g\na{group}:R -- L:b') },
  { text: ab('group g\nservice a in g\nservice b in g\na:R -- L:b{group}') },
  { text: ab('group g\nservice a in g\nservice b in g\na{group}:R -- L:b{group}') },
  { text: ab('group g\ngroup h in g\nservice a in g\nservice b in h\na{group}:R -- L:b{group}'), full: true },
  { text: ab('group g\njunction j in g\njunction k in g\nj{group}:R -- L:k') },
  { text: ab('group g\ngroup h\njunction j in g\njunction k in h\nj{group}:R -- L:k{group}\nj:T -- B:k'), full: true },
  // Group alignments.
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na:R -- L:b'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na:B -- T:b'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\na:R -- T:b'), full: true },
  { text: ab('group h\ngroup g\nservice a in g\nservice b in h\nb:L -- R:a'), full: true },
  { text: ab('group g\ngroup h\ngroup i\nservice a in g\nservice b in h\nservice c in i\na:R -- L:b\nb:R -- L:c'), full: true },
  { text: ab('group g\ngroup h\ngroup i\nservice a in g\nservice b in h\nservice c in i\na:R -- L:b\nb:B -- T:c\nc:L -- R:a'), full: true },
  { text: ab('group g\ngroup h\ngroup i\nservice a in g\nservice a2 in g\nservice b in h\nservice c in i\na:R -- L:a2\na2:R -- L:b\nb:R -- L:c\nservice d\nc:R -- L:d'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\nservice c in g\nservice d in h\na:R -- L:b\nc:R -- L:d\na:B -- T:c\nb:B -- T:d'), full: true },
  { text: ab('group "g"\nservice a in g') },
  // Layout hints.
  { text: ab('service a\nservice b\nalign row a b'), full: true },
  { text: ab('service a\nservice b\nalign column a b'), full: true },
  { text: ab('service a\nservice b\nservice c\na:R -- L:b\nalign row a b c'), full: true },
  { text: ab('service a\nservice b\nservice c\na:R -- L:b\nb:R -- L:c\nalign column a c'), full: true },
  { text: ab('service a\njunction j\nalign row a j'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nalign row a b\nalign row c d\nalign column a c\nalign column b d'), full: true },
  { text: ab('align row a b\nservice a\nservice b'), full: true },
  { text: ab('service a\nservice b\na:R -- L:b\nalign row b a'), full: true },
  { text: ab('service a\nservice b\na:R -- L:b\nalign column a b'), full: true },
  { text: ab('service a\nservice b\nservice c\nalign row a b\nalign row b c'), full: true },
  { text: ab('service a\nservice b\nservice c\nalign row a b c\nalign row c b a'), full: true },
  { text: ab('service a\nservice b\nalign row a a') },
  { text: ab('service a\nservice b\nalign row a b a') },
  { text: ab('service a\ngroup g\nalign row a g') },
  { text: ab('service a\nalign row a nosuch') },
  { text: ab('service a\nalign column nosuch a') },
  { text: ab('service a\nalign row a') },
  // More shapes of the layout and of the drawing.
  { text: ab('group g(cloud)[G]\nservice a(server)[A] in g\nservice b(server)[B] in g\nservice c(disk)[C]\na:R -[in group]- L:b\nb:R -[out]-> L:c\na:B <-[down]- T:c'), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(cloud)[H]\nservice a(server)[A] in g\nservice b(server)[B] in h\na{group}:R -[Group edge]-> L:b{group}'), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(cloud)[H]\nservice a(server)[A] in g\nservice b(server)[B] in h\na{group}:B <-[Vertical]-> T:b{group}'), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(cloud)[H]\nservice a(server)[A] in g\nservice b(server)[B] in h\na{group}:T --> B:b{group}'), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(cloud)[H]\nservice a(server)[A] in g\nservice b(server)[B] in h\na{group}:L <-- R:b{group}'), full: true },
  { text: ab('group g\ngroup h\njunction j in g\njunction k in h\nservice a in g\nservice b in h\na:R -- L:j\nj{group}:R --> L:k{group}\nk:R -- L:b'), full: true },
  { text: ab('junction j\nservice a\nservice b\nservice c\nservice d\na:R --> L:j\nb:B --> T:j\nc:L <-- R:j\nd:T <-- B:j'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\na:L -[LT]- T:b\na:R -[RT]- T:c\na:L -[LB]- B:d\na:R -[RB]- B:e'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\na:T -[TL]- L:b\na:T <-[TR]-> R:c\na:B -[BL]- L:d\na:B -[BR]-> R:e'), full: true },
  { text: ab('service a\nservice b\na:L <-[A long label on a short edge that wraps]-> R:b'), full: true },
  { text: ab('service a\nservice b\na:T -[A long vertical label that wraps as well]- B:b'), full: true },
  { text: ab('service a\nservice b\na:L -[**bold** *label*]- T:b'), full: true },
  { text: ab('service a(server)["two\nlines"]\nservice b(server)[B]\na:R -- L:b'), full: true },
  { text: ab('group g(cloud)["two\nlines"]\nservice a(server)["a  b   c"] in g\nservice b(server)[ wide   spaces ] in g\na:R -- L:b'), full: true },
  { text: ab('group g\nservice a(server)[""] in g\nservice b(server)[ ] in g\nservice c in g\na:R -- L:b\nb:R -- L:c'), full: true },
  { text: ab('group g\nservice a(server)[A wide title under the icon] in g\nservice b(server)[B] in g\na:B -- T:b'), full: true },
  { text: ab('group g\ngroup h in g\ngroup i in h\nservice a(server)[A] in i\nservice b(server)[B] in h\nservice c(server)[C] in g\nservice d(server)[D]\na:R -- L:b\nb:R -- L:c\nc:R -- L:d'), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(disk)[H] in g\ngroup i(server)[I] in g\nservice a(server)[A] in h\nservice b(server)[B] in i\na:B -- T:b'), full: true },
  { text: ab('group g(cloud)[Empty group]\nservice a(server)[A]'), full: true },
  { text: ab('group g(cloud)[Empty group]'), full: true },
  { text: ab('group g\ngroup h in g'), full: true },
  { text: ab('group g\njunction j in g'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\nservice f\na:R -- L:b\nb:R -- L:c\nd:R -- L:e\ne:R -- L:f\na:B -- T:d\nb:B -- T:e\nc:B -- T:f'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\na:R -- L:b\nb:B -- T:c\nc:L -- R:d\nd:T -- B:a'), full: true },
  { text: ab('service a\nservice b\nservice c\na:R -- L:b\na:R -- L:c'), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\na:R -- L:b\nc:B -- T:d\nservice lone'), full: true },
  { text: ab('service a "Text"\nservice b "Other"\ngroup g\nservice c "In group" [Titled] in g\na:R -- L:b\nb:R -- L:c'), full: true },
  { text: ab('service a "<b>bold</b> #35; <script>x</script>"\nservice b "é 日本語"'), full: true },
  { text: ab('group g\nservice a in g\nservice b in g\nservice c in g\nservice d in g\na:R -- L:b\nc:R -- L:d\nalign row a b c d'), full: true },
  { text: ab('group g\ngroup h\nservice a in g\nservice b in h\nalign column a b'), full: true },
  { text: ab('service a\njunction j\njunction k\nalign row a j k\nalign column a j'), full: true },
  // The order of the icons with ids, in services and in groups.
  { text: ab('group g(database)[G]\ngroup h(database)[H]\ngroup i(database)[I] in h\ngroup j[No icon] in g\nservice a(database)[A] in g\nservice b(database)[B] in i\nservice c(server)[C] in j\na:R -- L:b'), full: true },
  // Layouts that draw from Math.random.
  { text: ab(small, cfg('architecture:\n  seed: 0\n  randomize: true')), full: true },
  { text: ab('service hub(database)[Hub]\nservice a(database)[A]\nservice b(server)[B]\nservice c(server)[C]\nservice d(server)[D]\nservice e(server)[E]\nservice f(server)[F]\nhub:L -- R:a\nhub:R -- L:b\nhub:T -- B:c\nhub:B -- T:d\nd:B -- T:e\ne:R -- L:f', cfg('architecture:\n  seed: 0\n  randomize: true')), full: true },
  { text: ab('group g(database)[G]\nservice a(database)[A] in g\nservice b(database)[B] in g\nservice c(database)[C]\nservice d(server)[D]\na:R -- L:b\nb:R -- L:c\nc:B -- T:d', cfg('architecture:\n  seed: 0\n  randomize: true')), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\na:R -- L:b\nb:R -- L:c\nc:B -- T:d\nd:L -- R:e', cfg('architecture:\n  seed: 0')), full: true },
  { text: ab('service a\nservice b\nservice c\nservice d\nservice e\na:R -- L:b\nb:R -- L:c\nc:B -- T:d\nd:L -- R:e', cfg('architecture:\n  seed: 123456789\n  randomize: true')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: -1\n  randomize: true')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: 1.5\n  randomize: true')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: 4294967296\n  randomize: true')), full: true },
  // Config values of another type than the schema names.
  { text: ab(small, cfg('architecture:\n  iconSize: "60"')), full: true },
  { text: ab(small, cfg('architecture:\n  fontSize: "20"')), full: true },
  { text: ab(small, cfg('architecture:\n  padding: "20"')), full: true },
  { text: ab('group g(cloud)[G]\ngroup h(cloud)[H]\nservice a(server)[A] in g\nservice b(server)[B] in h\na{group}:R --> L:b{group}', cfg('architecture:\n  padding: "20"')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: "0"\n  randomize: true')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: true\n  randomize: 1')), full: true },
  { text: ab(small, cfg('architecture:\n  randomize: "false"')), full: true },
  { text: ab(small, cfg('architecture:\n  useMaxWidth: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  useMaxWidth: "no"')), full: true },
  { text: ab(small, cfg('architecture:\n  iconSize: null')), full: true },
  { text: ab(small, cfg('architecture:\n  padding: null\n  fontSize: null')), full: true },
  { text: ab(small, cfg('architecture:\n  iconSize: -40')), full: true },
  { text: ab(small, cfg('architecture:\n  iconSize: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  fontSize: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  padding: -10')), full: true },
  { text: ab(small, cfg('architecture:\n  numIter: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  nodeSeparation: 0\n  idealEdgeLengthMultiplier: 0\n  edgeElasticity: 0')), full: true },
  // Title and accessibility.
  { text: ab('title A title\naccTitle: An accessible title\naccDescr: A description\nservice a'), full: true },
  { text: ab('accDescr {\n  A description\n    on two lines\n}\nservice a'), full: true },
  { text: ab('title   spaced    title  \nservice a') },
  { text: ab('title <b>bold</b> <script>alert(1)</script> title\naccTitle: <i>acc</i>\nservice a') },
  { text: ab('title #35; entity\nservice a') },
  { text: ab('service a\ntitle Late title\ntitle Later title') },
  { text: ab('service a', '---\ntitle: Front matter title\n---\n'), full: true },
  { text: ab('title Diagram title\nservice a', '---\ntitle: Front matter title\n---\n') },
  // Config.
  { text: ab(small, cfg('architecture:\n  iconSize: 40')), full: true },
  { text: ab(small, cfg('architecture:\n  iconSize: 120\n  fontSize: 24\n  padding: 10')), full: true },
  { text: ab(small, cfg('architecture:\n  padding: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  fontSize: 10.5')), full: true },
  { text: ab(small, cfg('architecture:\n  useMaxWidth: false')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: 0')), full: true },
  { text: ab(small, cfg('architecture:\n  seed: 7\n  randomize: true')), full: true },
  { text: ab(small, cfg('architecture:\n  nodeSeparation: 150\n  idealEdgeLengthMultiplier: 3\n  edgeElasticity: 0.1\n  numIter: 100')), full: true },
  { text: ab(small, cfg('architecture:\n  unknownKey: 5\n  nested:\n    a: 1')) },
  { text: ab(small, cfg('fontFamily: courier\nfontSize: 20')), full: true },
  { text: ab(small, cfg('theme: dark')), full: true },
  { text: ab(small, cfg('theme: forest\nthemeVariables:\n  archEdgeColor: "#ff0000"\n  archGroupBorderColor: "#00ff00"')), full: true },
  { text: ab(small, cfg('htmlLabels: false')), full: true },
  { text: ab(small, cfg('securityLevel: loose')), full: true },
  { text: ab(small, '%%{init: {"architecture": {"iconSize": 60, "padding": 20}}}%%\n'), full: true },
  { text: ab('service a "Text" [Title]', cfg('architecture:\n  iconSize: 40\n  fontSize: 30')), full: true },
  { text: ab('service a "Text" [Title]', cfg('fontSize: 14.5')), full: true },
];
extras.forEach((c, i) => cases.push({ name: `extra-${i}`, ...c }));

// Site configs: the argument of mermaid.initialize.
const sites = [
  { architecture: { iconSize: 50, padding: 30 } },
  { architecture: { seed: 3, fontSize: 12 } },
  { theme: 'neutral' },
  { fontFamily: 'Georgia, serif' },
  { securityLevel: 'loose' },
];
sites.forEach((site, i) => {
  cases.push({ name: `site-${i}`, site, full: true, text: ab(small) });
  cases.push({ name: `site-${i}-directive`, site, full: true, text: ab(small, cfg('architecture:\n  iconSize: 64')) });
});

