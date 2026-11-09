// Writes the golden files for the tests in internal/grammar.
//
// trace/<grammar>.txt.gz: for every corpus fixture that mermaid parses with
// the grammar, for mutations of those fixtures, and for some extra inputs,
// the text that mermaid gives to the generated parser and the trace of the
// generated JavaScript parser on that text: every token that the lexer gives to the parser, every
// reduction, and the result or the error. The yy object is a recording
// object (see recordingYY in jison-lib.mjs). The parser is the jison build of
// the grammar. The script checks that the parser in the mermaid dist gives
// the same trace.
//
// regex/<grammar>.bin.gz: the match length of every lexer rule, measured in
// V8, on a set of probe strings: the remaining input at every call of
// lexer.next for every trace input, strings that are generated from the
// rules, and one-unit strings.
//
// Usage (in tools/reference): node gen-jison-golden.mjs [grammar ...]
// The directory JISON_GOLDEN_OUT replaces internal/grammar/testdata when it is set.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import {
  analyzeRule,
  bitsOf,
  corpusInputs,
  distParser,
  literalAdmits,
  loadGrammars,
  newParser,
  parserActions,
  prng,
  repoRoot,
  setRanges,
  tracer,
} from './jison-lib.mjs';

const outRoot = process.env.JISON_GOLDEN_OUT ?? path.join(repoRoot, 'internal/grammar/testdata');
const only = process.argv.slice(2);
const probeLength = 240; // code units of the remaining input that a probe keeps
// The two counts can be raised for a one-time larger run, for example
// JISON_MUTANTS=1000 JISON_GENERATED=300. The committed files use the defaults.
const mutantsPerGrammar = Number(process.env.JISON_MUTANTS ?? 100);
const generatedPerRule = Number(process.env.JISON_GENERATED ?? 12);

// ---------------------------------------------------------------------------
// Mutations

const specials = ['"', '`', "'", '%%', '{', '}', '[', ']', '(', ')', ';', ':', '--', '-->', '\r\n', '\t', '\n', ' ', '|', '#', '@', '&', '<', '>', '=', '.', ',', 'end']
  // e-acute, an emoji, LINE SEPARATOR, NO-BREAK SPACE, a lone surrogate, KELVIN SIGN, I with dot.
  .concat([[0xe9], [0xd83d, 0xde00], [0x2028], [0xa0], [0xd83d], [0x212a], [0x130]].map((u) => String.fromCharCode(...u)));

function mutate(rnd, base, pool) {
  let s = base;
  const edits = 1 + Math.floor(rnd() * 3);
  const at = () => Math.floor(rnd() * (s.length + 1));
  for (let i = 0; i < edits; i++) {
    const kind = Math.floor(rnd() * 7);
    const p = at();
    if (kind === 0) {
      s = s.slice(0, p) + s.slice(p + 1 + Math.floor(rnd() * 8));
    } else if (kind === 1) {
      const q = Math.floor(rnd() * pool.length);
      s = s.slice(0, p) + pool.slice(q, q + 1 + Math.floor(rnd() * 3)) + s.slice(p);
    } else if (kind === 2) {
      const q = Math.floor(rnd() * pool.length);
      s = s.slice(0, p) + pool[q] + s.slice(p + 1);
    } else if (kind === 3) {
      s = s.slice(0, p);
    } else if (kind === 4) {
      const lines = s.split('\n');
      const a = Math.floor(rnd() * lines.length);
      lines.splice(Math.floor(rnd() * lines.length), 0, lines[a]);
      s = lines.join('\n');
    } else {
      s = s.slice(0, p) + specials[Math.floor(rnd() * specials.length)] + s.slice(p);
    }
  }
  return s;
}

// ---------------------------------------------------------------------------
// Extra inputs
//
// These inputs go to the parser as they are. They cover behavior that the
// corpus and the mutants reach only by chance.

const cp = (...points) => String.fromCodePoint(...points);
const emoji = cp(0x1f600);
const commonExtras = ['', '\n', ' \n\t ', '\r\n', '%%', '%% comment\n', emoji, cp(0x2028), 'x'.repeat(60), '"', ';', '9'];
const extras = {
  flow: [
    // A rule that matches one code unit splits a surrogate pair.
    `graph TD\n A -- ${emoji} text --> B\n`,
    `graph TD\n A == ${emoji} ==> B\n`,
    `graph TD\n A -. ${emoji} .-> B\n`,
    `graph TD\n A(-${emoji}-)\n`,
    `graph TD\n A[${emoji}] --> B{{${emoji}${emoji}}}\n ${emoji} --> A\n`,
    // Line breaks: \r\n, a lone \r, and LINE SEPARATOR after the last line break of a token.
    'graph TD\r\n A-->B\r\n B-->C\r\n',
    'graph TD\r A-->B\r',
    `graph TD\n A["x\n y${cp(0x2028)}z w"] --> B\n`,
    'graph TD\n A["never closed',
    'graph TD\n accDescr {\n several\n lines\n }\n A-->B\n',
    'graph TD\n A@{ shape: rect, label: "a\n   b\n c" }\n',
    // The second graph keyword is not followed by a direction.
    'graph TD\n graph LR\n A-->B\n',
    'flowchart\n A-->B\n',
    'graph TD\n A-->B\n click A call cb("x", 1)\n click B href "http://x" _blank\n',
    'graph TD\n ' + 'a'.repeat(30) + ' --> ' + 'b'.repeat(30) + ' ]\n',
  ],
  agentflow: [`agentflow-beta\n A -- ${emoji} --> B\n`, 'agentflow-beta\r\n A-->B\r\n'],
  sequencediagram: [
    `sequenceDiagram\n A->>B: ${emoji} hi\n Note over A,B: x${emoji}\n`,
    'sequenceDiagram\r\n participant A as Alice\r\n A->>B: hi\r\n',
    'sequenceDiagram\n participant A@{ "type": "boundary" } as Alice\n A->>B: hi\n',
    'sequenceDiagram\n A->>B hi\n',
    'sequenceDiagram\n loop every minute\n A->>B: x\n',
  ],
  classdiagram: [
    'classDiagram\n class A~T~ {\n +foo() int\n }\n namespace N {\n class B\n }\n',
    'classDiagram\n namespace N {\n }\n class C\n',
    `classDiagram\n class A["${emoji}"]\n A : +${emoji}()\n`,
    'classDiagram\r\n class A\r\n A <|-- B\r\n',
    'classDiagram\n class `a b` {\n',
  ],
  statediagram: [
    'stateDiagram-v2\n state A B {\n }\n',
    'stateDiagram-v2\n s1%%c --> s2\n state foo%% comment\n s2 --> [*]\n',
    'stateDiagram-v2\n %% only\n s1 --> s2 %% tail\n',
    'stateDiagram-v2\n note right of s1\n several\n lines\n end note\n s1 --> s2: x\n',
    'stateDiagram\r\n [*] --> s1\r\n',
    `stateDiagram-v2\n s1: ${emoji}\n`,
  ],
  erdiagram: [
    // The lexer returns a character as the token name. A digit is then the symbol with that number.
    'erDiagram\n    CUSTOMER {\n        string name\n        BOO 9__float price\n    }\n',
    'erDiagram\n    A {\n        string name 1\n    }\n',
    'erDiagram\n    A {\n        string name 2 3 4 5 6 7 8\n    }\n',
    'erDiagram\n    A ||--o{ B : has\n    ? \n',
    `erDiagram\n    A ||--o{ B : "${emoji}"\n    A {\n        string ${emoji}\n    }\n`,
    'erDiagram\r\n    A ||--o{ B : has\r\n',
  ],
  gantt: [
    // begin('open_directive') names a start condition that does not exist.
    'gantt\n%%{init: {}}%%\n title x\n',
    'gantt\n %%{init: {}}%%\n title x\n',
    'gantt\n dateFormat YYYY-MM-DD\n section A\n task :a1, 2024-01-01, 3d\n click a1 call f("x")\n click a1 href "http://x"\n',
    'gantt\r\n title x\r\n',
  ],
  c4diagram: ['C4Context\n Person(a, "b") %% comment\n', 'C4Context\n title x\n %% c\n', 'C4Context\n%%{x}%%\n%% c\r\n\n title x\n'],
  block: [
    'block-beta\n columns auto\n a:2 space:3 b\n blockArrowId<["Label"]>(right, down)\n',
    'block-beta\n columns 3\n a b c\n',
  ],
  kanban: ['kanban\n todo\n  id1[Task]@{ assigned: "a\n   b" }\n'],
  venn: ['venn-beta\n set A\n   text "x"\n union A,B\n text t\n', 'venn-beta\n set A\n\ttext "x"\n'],
  sankey: ['sankey-beta\n\na,b,1\n"c ""d""",e,2\n', 'sankey\r\na,b,1\r\n'],
};

// ---------------------------------------------------------------------------
// Probe strings from the rules

const interesting = [
  0x09, 0x0a, 0x0b, 0x0c, 0x0d, 0x20, 0x21, 0x22, 0x23, 0x25, 0x27, 0x28, 0x29, 0x2a, 0x2c, 0x2d, 0x2e, 0x2f,
  0x30, 0x39, 0x3a, 0x3b, 0x3c, 0x3d, 0x3e, 0x40, 0x41, 0x5a, 0x5b, 0x5c, 0x5d, 0x5f, 0x60, 0x61, 0x7a, 0x7b,
  0x7c, 0x7d, 0x7e, 0x7f, 0x85, 0xa0, 0xaa, 0xb5, 0xc0, 0xd7, 0xdf, 0xe9, 0xf7, 0xff, 0x130, 0x131, 0x17f,
  0x1680, 0x180e, 0x2000, 0x200a, 0x200b, 0x2028, 0x2029, 0x202f, 0x205f, 0x212a, 0x212b, 0x3000, 0x4e2d,
  0xd83d, 0xde00, 0xfeff, 0xff21, 0xffff, 0x00,
];

function generator(rnd, flags) {
  const pick = (list) => list[Math.floor(rnd() * list.length)];
  const fromSet = (set) => {
    const ranges = setRanges(set);
    if (ranges.length === 0 || rnd() < 0.12) {
      // A unit that is near the set or interesting, in the set or not.
      if (ranges.length && rnd() < 0.5) {
        const i = 2 * Math.floor(rnd() * (ranges.length / 2));
        const c = rnd() < 0.5 ? ranges[i] - 1 : ranges[i + 1] + 1;
        return c >= 0 && c <= 0xffff ? c : pick(interesting);
      }
      return pick(interesting);
    }
    const i = 2 * Math.floor(rnd() * (ranges.length / 2));
    const [lo, hi] = [ranges[i], ranges[i + 1]];
    const r = rnd();
    if (r < 0.3) return lo;
    if (r < 0.6) return hi;
    return lo + Math.floor(rnd() * (hi - lo + 1));
  };
  const gen = (n, out, tail) => {
    switch (n.t) {
      case 'alt':
        gen(pick(n.alts), out, tail);
        break;
      case 'seq':
        for (const item of n.items) gen(item, out, tail);
        break;
      case 'set':
        out.push(fromSet(bitsOf(n, flags)));
        break;
      case 'group':
        gen(n.body, out, tail);
        break;
      case 'look': {
        // The text of a lookahead follows the match.
        const body = [];
        gen(n.body, body, []);
        if (!n.neg || rnd() < 0.5) tail.push(...body);
        break;
      }
      case 'assert':
        break;
      case 'quant': {
        const max = Math.min(n.max, n.min + (rnd() < 0.2 ? 6 : 2));
        const count = n.min + Math.floor(rnd() * (max - n.min + 1));
        for (let i = 0; i < count; i++) gen(n.body, out, tail);
        break;
      }
    }
  };
  return (tree) => {
    const out = [];
    const tail = [];
    gen(tree, out, tail);
    const units = out.concat(tail);
    const extra = rnd() < 0.3 ? 0 : 1 + Math.floor(rnd() * 5);
    for (let i = 0; i < extra; i++) units.push(pick(interesting));
    if (rnd() < 0.3 && units.length) {
      const p = Math.floor(rnd() * units.length);
      const kind = Math.floor(rnd() * 3);
      if (kind === 0) units.splice(p, 1);
      else if (kind === 1) units[p] = pick(interesting);
      else units.splice(p, 0, pick(interesting));
    }
    return String.fromCharCode(...units.slice(0, 400));
  };
}

// ---------------------------------------------------------------------------
// Binary output

class Writer {
  constructor() {
    this.bytes = [];
  }
  uvarint(n) {
    if (!Number.isInteger(n) || n < 0) throw new Error('bad uvarint ' + n);
    while (n >= 0x80) {
      this.bytes.push((n & 0x7f) | 0x80);
      n = Math.floor(n / 128);
    }
    this.bytes.push(n);
  }
  buffer() {
    return Buffer.from(this.bytes);
  }
}

// ---------------------------------------------------------------------------

const grammars = loadGrammars();
const { inputs, skipped, types } = await corpusInputs(grammars);
fs.mkdirSync(path.join(outRoot, 'trace'), { recursive: true });
fs.mkdirSync(path.join(outRoot, 'regex'), { recursive: true });

const refError = (file) => {
  const ref = path.join(repoRoot, '.parity/ref', file.replace(/^testdata\/corpus\//, '').replace(/\.mmd$/, '.json'));
  if (!fs.existsSync(ref)) return undefined;
  try {
    return JSON.parse(fs.readFileSync(ref, 'utf8')).error ?? null;
  } catch {
    return undefined;
  }
};

const report = [];
let failed = false;
for (const grammar of grammars) {
  if (only.length && !only.includes(grammar.name)) continue;
  const run = tracer(grammar, newParser(grammar));
  const runDist = tracer(grammar, distParser(grammar.rel).parser);
  const productions = parserActions(grammar);
  const halts = productions.filter((p) => p.halts).map((p) => p.id);
  const rules = grammar.parser.lexer.rules;
  const infos = rules.map((re) => analyzeRule(re));
  const normalized = infos.map((a) => new RegExp('^(?:' + a.jsPattern + ')'));

  // trace runs one input and returns the trace lines and the probe offsets.
  const trace = (input) => {
    const offsets = new Set();
    let semantic = false;
    const events = run(input, {
      onNext: (lexer) => {
        if (input.endsWith(lexer._input)) offsets.add(input.length - lexer._input.length);
      },
    });
    const last = events.findLast((l) => l.startsWith('OK') || l.startsWith('ERR'));
    if (last.startsWith('ERR')) {
      // An error without a hash that follows a reduction comes from a semantic action.
      const i = events.indexOf(last);
      const fromParser = events[i + 1]?.startsWith('H ');
      semantic = !fromParser && events[i - 1]?.startsWith('R ');
    }
    if (last.startsWith('OK return')) {
      if (!halts.includes(Number(last.slice(10)))) throw new Error(`${grammar.name}: unexpected ${last}`);
    }
    return { events, offsets: [...offsets].sort((a, b) => a - b), semantic, last };
  };

  const mine = inputs.filter((x) => x.grammar === grammar.name);
  const fixtures = [];
  const stats = { ok: 0, parseErrors: 0, lexErrors: 0, otherErrors: 0, semantic: 0, refErrors: 0, refAgree: 0, distMismatch: 0 };
  const classify = (t) => {
    if (t.last.startsWith('OK')) stats.ok++;
    else if (t.last.includes('"Parse error on line')) stats.parseErrors++;
    else if (t.last.includes('"Lexical error on line')) stats.lexErrors++;
    else stats.otherErrors++;
  };
  for (const x of mine) {
    const t = trace(x.input);
    if (runDist(x.input).join('\n') !== t.events.join('\n')) {
      stats.distMismatch++;
      failed = true;
      console.error(`${grammar.name}: the dist parser gives another trace for ${x.file}`);
    }
    if (t.semantic) stats.semantic++;
    classify(t);
    const ref = refError(x.file);
    if (t.last.startsWith('ERR')) {
      const message = JSON.parse(t.last.split('\t')[1]);
      if (typeof ref === 'string') {
        stats.refErrors++;
        if (ref === message) stats.refAgree++;
        else console.error(`${grammar.name}: ${x.file}: the browser reference has another error:\n  ${ref}\n  ${message}`);
      } else if (ref === null) {
        console.error(`${grammar.name}: ${x.file}: error here, but none in the browser reference: ${message}`);
      }
    }
    fixtures.push({ name: x.file, input: x.input, ...t });
  }

  // Mutants: deterministic edits of the corpus inputs.
  const rnd = prng(0x6a09e667 ^ grammar.name.length ^ (grammar.name.charCodeAt(0) << 8));
  const pool = mine.map((x) => x.input).join('\n');
  const bases = mine.filter((x) => x.input.length <= 1500);
  const seen = new Set(mine.map((x) => x.input));
  let mutants = 0;
  for (let i = 0; mutants < mutantsPerGrammar && i < mutantsPerGrammar * 6 && bases.length; i++) {
    const base = bases[Math.floor(rnd() * bases.length)];
    const input = mutate(rnd, base.input, pool);
    if (seen.has(input)) continue;
    seen.add(input);
    const t = trace(input);
    if (t.semantic) continue; // The semantic actions are not part of this test.
    if (runDist(input).join('\n') !== t.events.join('\n')) {
      stats.distMismatch++;
      failed = true;
      console.error(`${grammar.name}: the dist parser gives another trace for a mutant of ${base.file}`);
    }
    classify(t);
    mutants++;
    fixtures.push({ name: `mutant ${mutants} of ${base.file}`, input, ...t });
  }

  // Extra inputs.
  let extraCount = 0;
  for (const input of [...commonExtras, ...(extras[grammar.name] ?? [])]) {
    if (seen.has(input)) continue;
    seen.add(input);
    const t = trace(input);
    if (t.semantic) {
      console.error(`${grammar.name}: an extra input ends in a semantic action: ${JSON.stringify(input)} ${t.last}`);
      continue;
    }
    if (runDist(input).join('\n') !== t.events.join('\n')) {
      stats.distMismatch++;
      failed = true;
      console.error(`${grammar.name}: the dist parser gives another trace for ${JSON.stringify(input)}`);
    }
    classify(t);
    extraCount++;
    fixtures.push({ name: `extra ${extraCount}`, input, ...t });
  }

  // The trace file.
  const lines = [`jison-trace 1 ${grammar.name}`, `halts ${halts.join(' ')}`];
  let tokens = 0;
  let reductions = 0;
  fixtures.forEach((f, index) => {
    // The location of a reduction (@$) is in the trace of every fifth input,
    // and of every agentflow input: that grammar reads locations in its actions.
    const withLoc = grammar.name === 'agentflow' || index % 5 === 0;
    lines.push(`F ${f.name}`, `I ${JSON.stringify(f.input)}`);
    if (withLoc) lines.push('LOC');
    for (const e of f.events) {
      if (e.startsWith('T ')) tokens++;
      else if (e.startsWith('R ')) reductions++;
      lines.push(withLoc || !e.startsWith('R ') ? e : e.split('\t')[0]);
    }
  });
  const traceGz = zlib.gzipSync(lines.join('\n') + '\n', { level: 9 });
  fs.writeFileSync(path.join(outRoot, 'trace', grammar.name + '.txt.gz'), traceGz);

  // The probes.
  const matchAll = (probe) => {
    const out = [];
    for (let r = 0; r < rules.length; r++) {
      const m = rules[r].exec(probe);
      const n = m ? m[0].length : -1;
      // The pattern for Go, run in V8, must agree with the rule of the grammar.
      const m2 = normalized[r].exec(probe);
      if ((m2 ? m2[0].length : -1) !== n) {
        throw new Error(`${grammar.name}: rule ${r}: the normalized pattern differs on ${JSON.stringify(probe)}`);
      }
      // The first-unit filter must admit every match.
      if (n >= 0 && !infos[r].nullable) {
        const c = probe.charCodeAt(0);
        const first = infos[r].first;
        let ok = false;
        for (let i = 0; i < first.length; i += 2) ok ||= c >= first[i] && c <= first[i + 1];
        if (!ok) throw new Error(`${grammar.name}: rule ${r}: the first set misses ${JSON.stringify(probe)}`);
      }
      // The literal filter must admit every match as well.
      if (n >= 0 && infos[r].need && !literalAdmits(infos[r].need, probe)) {
        throw new Error(`${grammar.name}: rule ${r}: the literal filter rejects ${JSON.stringify(probe)}`);
      }
      if (n >= 0) out.push(r, n);
    }
    return out;
  };
  const w = new Writer();
  // A probe that was already tested is written as 0. Other probes are the
  // number of matches plus one, then the matches.
  const seenProbes = new Set();
  const writeProbe = (probe) => {
    if (seenProbes.has(probe)) {
      w.uvarint(0);
      return;
    }
    seenProbes.add(probe);
    const matches = matchAll(probe);
    writeMatches(matches);
    probes++;
    positives += matches.length / 2;
  };
  const writeMatches = (matches) => {
    w.uvarint(matches.length / 2 + 1);
    let prev = -1;
    for (let i = 0; i < matches.length; i += 2) {
      w.uvarint(matches[i] - prev - 1);
      w.uvarint(matches[i + 1]);
      prev = matches[i];
    }
  };
  let probes = 0;
  let tests = 0;
  let positives = 0;
  w.uvarint(fixtures.length);
  for (const f of fixtures) {
    w.uvarint(f.offsets.length);
    let prev = 0;
    for (const off of f.offsets) {
      w.uvarint(off - prev);
      prev = off;
      writeProbe(f.input.slice(off, off + probeLength));
    }
  }
  // Generated strings, and one-unit strings for the units at the edges of the sets.
  const grnd = prng(0xbb67ae85 ^ grammar.name.length ^ (grammar.name.charCodeAt(1) << 8));
  const generated = new Set();
  for (const info of infos) {
    const make = generator(grnd, info.flags);
    for (let i = 0; i < generatedPerRule; i++) generated.add(make(info.tree));
  }
  const edge = new Set(interesting);
  for (let c = 0; c < 0x250; c++) edge.add(c);
  for (const info of infos) {
    for (const c of info.first) {
      for (const d of [c - 1, c, c + 1]) if (d >= 0 && d <= 0xffff) edge.add(d);
    }
  }
  for (const c of edge) generated.add(String.fromCharCode(c));
  generated.delete('');
  generated.add('');
  w.uvarint(generated.size);
  for (const probe of generated) {
    w.uvarint(probe.length);
    for (let i = 0; i < probe.length; i++) w.uvarint(probe.charCodeAt(i));
    writeProbe(probe);
  }
  tests = probes * rules.length;
  const header = JSON.stringify({
    format: 'jison-regex 1',
    grammar: grammar.name,
    probeLength,
    rules: infos.map((a) => ({ source: a.source, flags: a.flags })),
  });
  const regexGz = zlib.gzipSync(Buffer.concat([Buffer.from(header + '\n'), w.buffer()]), { level: 9 });
  fs.writeFileSync(path.join(outRoot, 'regex', grammar.name + '.bin.gz'), regexGz);

  report.push({
    grammar: grammar.name,
    corpus: mine.length,
    mutants,
    extras: extraCount,
    tokens,
    reductions,
    ok: stats.ok,
    parseErr: stats.parseErrors,
    lexErr: stats.lexErrors,
    otherErr: stats.otherErrors,
    semantic: stats.semantic,
    refErr: `${stats.refAgree}/${stats.refErrors}`,
    distDiff: stats.distMismatch,
    probes,
    regexTests: tests,
    matches: positives,
    traceKB: Math.round(traceGz.length / 1024),
    regexKB: Math.round(regexGz.length / 1024),
  });
}
console.table(report);
const total = (k) => report.reduce((a, r) => a + r[k], 0);
console.log(
  `corpus fixtures with a jison grammar: ${total('corpus')}, skipped: ${skipped.size}, mutants: ${total('mutants')}, extra inputs: ${total('extras')}, ` +
    `tokens: ${total('tokens')}, reductions: ${total('reductions')}, regex tests: ${total('regexTests')}`
);
const other = Object.entries(types).filter(([, v]) => v === 'other' || v.startsWith('not loadable'));
console.log('diagram types without a jison grammar: ' + other.map(([k]) => k).join(', '));
process.exit(failed ? 1 : 0);
