// Writes internal/diagrams/timeline/testdata/db.json.gz and
// internal/diagrams/journey/testdata/db.json.gz: the content of the timeline
// db and of the journey db of real mermaid (sections, tasks, actors, title,
// accessibility values, direction) after it parses
//   - every fixture of testdata/corpus/mermaid-e2e/timeline and journey,
//   - seeded mutations of the body of those fixtures,
//   - seeded random documents made of statement snippets,
//   - hand-written inputs for the branches of the grammars and the dbs.
// The dbs sanitize their texts with DOMPurify, which needs a browser, so the
// script runs timeline-journey-db-page.js with the eval command of the
// browser oracle, one time for both diagram types.
//
// Usage (in tools/reference): node gen-timeline-journey-db.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const mutantsPerFixture = 4;
const randomDocuments = 220;

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

const types = {
  timeline: {
    family: 'testdata/corpus/mermaid-e2e/timeline',
    out: 'internal/diagrams/timeline/testdata/db.json.gz',
    header: /^\s*timeline/i,
    headers: ['timeline', 'timeline', 'timeline', 'timeline LR', 'timeline TD', 'timeline  \tTD', 'TimeLine td'],
    snippets: [
      '2002 : LinkedIn',
      '2004 : Facebook : Google',
      '2005 : YouTube',
      '     : a continued event',
      '          : New styles of pottery and ways of burying the dead appear.',
      'Period only',
      'Industry 1.0 : Machinery, Water power, Steam <br>power',
      'Bronze Age : text : with: colon:inside : end',
      '6000 BC : Sea levels rise.<br> The people are hunter-gatherers.',
      'section Stone Age',
      'section 17th-20th century',
      'section Two words : and an event',
      'section',
      'title History of Social Media Platform',
      'title  spaced   title ',
      'title',
      'accTitle: Timeline acc title',
      'accTitle   :    spaced acc title   ',
      'accDescr: Timeline acc descr',
      'accDescr {\n  multi line\n     description\n  }',
      '%% a comment',
      '# a hash comment',
      'Period # trailing hash : event',
      'A <b>bold</b> period : <i>event</i>',
      'Ünïcödé pèriod : évènt ☃ 😀',
      '  2010 :   spaced   event   ',
      'title <script>alert(1)</script> T & <b>b</b> a < b',
      'accDescr: <img src=x onerror=alert(1)> d',
      ';',
      ':',
      ': lone event',
      'x:y',
      'a :b',
      '2020\t:\ttabbed event',
      'Quoted "period" : \'event\'',
      'timeline',
      'title ﬂ°°35¶ß entity #35; and #quot;',
    ],
    extras: [
      'timeline',
      'timeline\n',
      'timeline TD',
      'timeline LR\n  2002 : LinkedIn',
      'timeline TD\n  2002 : LinkedIn',
      'timeline TDx\n  2002 : LinkedIn',
      'timeline\n  : event before any period',
      'timeline\n  section S\n  : event before any period',
      'timeline\n  P1\n  : e1\n  : e2\n  P2 : e3',
      'timeline\n  title T\n  title Second title\n  P',
      'timeline\n  accTitle: A\n  accDescr: D\n  P : e',
      'timeline\n  accDescr {\n    line one\n    line two\n  }\n  P : e',
      'timeline\n  accDescr { unterminated\n  P : e',
      'timeline\n  section A\n  P1 : e\n  section B\n  P2 : e\n  section A\n  P3 : e',
      'timeline\n  P0 : before sections\n  section A\n  P1 : e',
      'timeline\n  section Only',
      'timeline\n  P : e ::: f',
      'timeline\n  P :e',
      'timeline\n  P:e',
      'timeline\n  P : a:b : c',
      'timeline\n  P :\n  Q',
      'timeline\n  P : \n  Q',
      'timeline\n%% c\n  P : e %% not a comment?\n  Q',
      'timeline\n  P } %% odd\n  Q',
      'timeline\n  P # hash\n  Q # : r',
      'timeline\n  sectionX\n  section\tTabbed\n  P',
      'timeline\n  titleX\n  title\tTabbed\n  P',
      'timeline\n\n\n  P\n\n\n  : e\n\n',
      'timeline\r\n  title Windows\r\n  P : e\r\n',
      'timeline\n  P\u00a0nbsp : e\u2003em space',
      'timeline\n  😀 : 🎉',
      '---\ntitle: Front matter title\n---\ntimeline\n  P : e',
      '---\ntitle: Front matter title\n---\ntimeline\n  title Body title\n  P : e',
      '---\nconfig:\n  securityLevel: loose\n---\ntimeline\n  title <b>bold</b> a < b\n  P : e',
      '---\nconfig:\n  securityLevel: antiscript\n---\ntimeline\n  title <b>bold</b><script>x</script>\n  P : e',
      '---\nconfig:\n  htmlLabels: false\n---\ntimeline\n  title <b>bold</b> a < b\n  P : e',
      '%%{init: {"theme": "dark"}}%%\ntimeline\n  title T\n  P : e',
      'timeline\n  INVALID {\n  P',
      'timeline nonsense\n  P',
      'timeline\n' + '  P : e\n'.repeat(60),
    ],
  },
  journey: {
    family: 'testdata/corpus/mermaid-e2e/journey',
    out: 'internal/diagrams/journey/testdata/db.json.gz',
    header: /^\s*journey/i,
    headers: ['journey', 'journey', 'journey', 'JOURNEY', 'journey  '],
    snippets: [
      'title My working day',
      'title',
      'title  spaced   title ',
      'section Go to work',
      'section Go home',
      'section',
      'Make tea: 5: Me',
      'Go upstairs: 3: Me',
      'Do work: 1: Me, Cat',
      'Go downstairs: 5: Me',
      'Sit down: 3: Me',
      'Task only score: 5',
      'Task:5:A,B , C',
      'Sign Up: 5:',
      'size: 2rem',
      'T: x: Y',
      'T: 3: a:b',
      'T: 4.5: Zoë, 1, 10, 2, 😀',
      'Only a name',
      ': 5: orphan data',
      'accTitle: Journey acc title',
      'accTitle   :    spaced acc title   ',
      'accDescr: Journey acc descr',
      'accDescr {\n  multi line\n     description\n  }',
      '%% a comment',
      '# a hash comment',
      'Task # hash: 5: X',
      'Task; 5: X',
      'title with ; semicolon',
      'A <b>b</b>: 2: <i>P</i>',
      'T: 0x10: hex',
      'T: 1e1: exp',
      'T: -3: neg',
      'T:  : blank',
      'T: Infinity: inf',
      'T: 3: , ,A,,A',
      'T: 3: constructor, toString, __proto__',
      'T::',
      ':',
      ';',
      'journey',
      'title <script>alert(1)</script> T & <b>b</b> a < b',
      'accDescr: <img src=x onerror=alert(1)> d',
      'Wide\ttabs:\t5\t:\tMe\t,\tYou',
    ],
    extras: [
      'journey',
      'journey\n',
      'journey\n  title T',
      'journey\n  section S',
      'journey\n  Task: 5: Me',
      'journey\n  Task: 5',
      'journey\n  Task: 5:',
      'journey\n  Task: 5: ',
      'journey\n  Task:',
      'journey\n  Task',
      'journey\n  Task: 5: A: B: C',
      'journey\n  Task: : A',
      'journey\n  Task: abc: A',
      'journey\n  Task: 5 5: A',
      'journey\n  Task: 5: A, B,C ,  D  ',
      'journey\n  T1: 1: b, a\n  T2: 2: c, a, 10, 9, B, é, Z',
      'journey\n  title T\n  title Second\n  Task: 1: A',
      'journey\n  accTitle: A\n  accDescr: D\n  Task: 1: A',
      'journey\n  accDescr {\n    line one\n    line two\n  }\n  Task: 1: A',
      'journey\n  section A\n  T1: 1: x\n  section B\n  T2: 2: y\n  section A\n  T3: 3: z',
      'journey\n  T0: 0: before\n  section A\n  T1: 1: x',
      'journey\n  section With : colon\n  T1: 1: x',
      'journey\n  section With # hash\n  T1: 1: x',
      'journey\n  section With ; semi\n  T1: 1: x',
      'journey\n  title With # hash\n  T1: 1: x',
      'journey\n%% c\n  T1: 1: x %% not a comment?\n  T2: 2: y',
      'journey\n  T1: 1: x # c\n  T2: 2: y',
      'journey\n  T1: 1: x; T2: 2: y',
      'journey\n\n\n  T1: 1: x\n\n\n',
      'journey\r\n  title Windows\r\n  T1: 1: x\r\n',
      'journey\n  T\u00a0nbsp: 5: A\u2003B',
      'journey\n  😀: 5: 🎉, 😀',
      'journey\n  T: 5: __proto__',
      'journey\n  T: 5: 2, 1, 10, b, a, B',
      '---\ntitle: Front matter title\n---\njourney\n  T: 5: A',
      '---\ntitle: Front matter title\n---\njourney\n  title Body title\n  T: 5: A',
      '---\nconfig:\n  securityLevel: loose\n---\njourney\n  title <b>bold</b> a < b\n  T: 5: A',
      '---\nconfig:\n  securityLevel: antiscript\n---\njourney\n  title <b>bold</b><script>x</script>\n  T: 5: A',
      '---\nconfig:\n  htmlLabels: false\n---\njourney\n  title <b>bold</b> a < b\n  T: 5: A',
      '%%{init: {"theme": "dark"}}%%\njourney\n  title T\n  T: 5: A',
      'journey\n  INVALID {\n  T: 5: A',
      'journey nonsense\n  T: 5: A',
      'journey\n' + '  T: 5: A, B\n'.repeat(60),
    ],
  },
};

// mutate changes the lines after the header line.
function mutate(rnd, text, spec) {
  const lines = text.split('\n');
  let start = lines.findIndex((l) => spec.header.test(l));
  if (start < 0) {
    return text;
  }
  start++;
  const pick = () => start + Math.floor(rnd() * Math.max(1, lines.length - start));
  const edits = 1 + Math.floor(rnd() * 3);
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 6);
    const p = pick();
    if (kind === 0) {
      lines.splice(p, 1);
    } else if (kind === 1) {
      lines.splice(p, 0, lines[pick()] ?? '');
    } else if (kind === 2) {
      const q = pick();
      [lines[p], lines[q]] = [lines[q] ?? '', lines[p] ?? ''];
    } else if (kind === 3) {
      lines.splice(p, 0, '  ' + spec.snippets[Math.floor(rnd() * spec.snippets.length)]);
    } else if (kind === 4) {
      const l = lines[p] ?? '';
      const at = Math.floor(rnd() * (l.length + 1));
      lines[p] = l.slice(0, at) + l.slice(at + 1 + Math.floor(rnd() * 4));
    } else {
      lines.length = Math.max(start, p);
    }
  }
  return lines.join('\n');
}

// randomDocument joins a header and random snippets with random indents.
function randomDocument(rnd, spec) {
  const pick = (arr) => arr[Math.floor(rnd() * arr.length)];
  const lines = [pick(spec.headers)];
  const n = Math.floor(rnd() * 9);
  for (let i = 0; i < n; i++) {
    lines.push(pick(['', '  ', '    ', '\t', ' ']) + pick(spec.snippets));
  }
  return lines.join(pick(['\n', '\n', '\n', '\n\n'])) + pick(['', '\n']);
}

const cases = [];
let seed = 0x7461736b;
for (const [type, spec] of Object.entries(types)) {
  const rnd = prng(seed++);
  const fixtures = fs
    .readdirSync(path.join(repoRoot, spec.family), { recursive: true })
    .filter((f) => f.endsWith('.mmd'))
    .map((f) => spec.family + '/' + f.split(path.sep).join('/'))
    .sort();
  for (const f of fixtures) {
    const text = fs.readFileSync(path.join(repoRoot, f), 'utf8');
    const name = f.slice('testdata/corpus/mermaid-e2e/'.length);
    cases.push({ name, type, text });
    for (let i = 0; i < mutantsPerFixture; i++) {
      cases.push({ name: `${name}#mutant${i}`, type, text: mutate(rnd, text, spec) });
    }
  }
  for (let i = 0; i < randomDocuments; i++) {
    cases.push({ name: `${type}/random-${i}`, type, text: randomDocument(rnd, spec) });
  }
  spec.extras.forEach((text, i) => cases.push({ name: `${type}/extra-${i}`, type, text }));
}

const tmp = fs.mkdtempSync(path.join(os.tmpdir(), 'timeline-journey-db-'));
const argsFile = path.join(tmp, 'args.json');
const resultFile = path.join(tmp, 'result.json');
fs.writeFileSync(argsFile, JSON.stringify({ cases }));
execFileSync(
  process.execPath,
  [
    path.join(here, 'oracle.mjs'),
    'eval',
    path.join(here, 'timeline-journey-db-page.js'),
    '--args',
    argsFile,
    '--out',
    resultFile,
  ],
  { stdio: 'inherit' }
);
const results = JSON.parse(fs.readFileSync(resultFile, 'utf8'));
fs.rmSync(tmp, { recursive: true, force: true });

for (const [type, spec] of Object.entries(types)) {
  const own = results.filter((r) => r.type === type);
  const data = JSON.stringify({ mermaid: '12.2.0', cases: own }, null, 1) + '\n';
  const outFile = path.join(repoRoot, spec.out);
  fs.mkdirSync(path.dirname(outFile), { recursive: true });
  fs.writeFileSync(outFile, zlib.gzipSync(data, { level: 9 }));
  const failed = own.filter((r) => r.error !== undefined).length;
  console.error(`${spec.out}: ${own.length} cases, ${failed} with an error`);
}
