// Shared code for gen-jison.mjs, gen-jison-regex.mjs and gen-jison-trace.mjs.
//
// It builds the jison grammars of mermaid 12.2.0 the same way that the mermaid
// build does (.build/jisonTransformer.ts: jison 0.4.18, moduleType "js",
// option "token-stack"), reads the generated parsers back out of the mermaid
// dist, and analyzes the lexer regular expressions.
import { createRequire } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const require = createRequire(path.join(here, 'package.json'));
const jison = require('jison');
const ebnfParser = require('ebnf-parser');

export const repoRoot = path.resolve(here, '../..');
export const diagramsRoot = path.join(
  repoRoot,
  '.upstream/mermaid-12.2.0/packages/mermaid/src/diagrams'
);
export const distRoot = path.join(here, 'node_modules/mermaid/dist');
export const corpusRoot = path.join(repoRoot, 'testdata/corpus');
export const jisonVersion = require('jison/package.json').version;

// ---------------------------------------------------------------------------
// Grammars

// packageName is the Go package name of a grammar file.
export function packageName(rel) {
  return path.basename(rel, '.jison').toLowerCase();
}

function evalParser(source) {
  return new Function(source + '\nreturn parser;')();
}

let grammarCache;

// loadGrammars builds every *.jison grammar of the upstream snapshot.
export function loadGrammars() {
  if (grammarCache) {
    return grammarCache;
  }
  const files = fs
    .readdirSync(diagramsRoot, { recursive: true })
    .filter((f) => f.endsWith('.jison'))
    .map((f) => f.split(path.sep).join('/'))
    .sort((a, b) => (packageName(a) < packageName(b) ? -1 : 1));
  grammarCache = files.map((rel) =>
    buildGrammar(packageName(rel), rel, fs.readFileSync(path.join(diagramsRoot, rel), 'utf8'))
  );
  return grammarCache;
}

// buildGrammar builds one grammar from its source text.
export function buildGrammar(name, rel, src) {
  // This is the body of transformJison in mermaid's .build/jisonTransformer.ts.
  const gen = new jison.Generator(src, { moduleType: 'js', 'token-stack': true });
  const source = gen.generate({ moduleMain: '() => {}' });
  const parser = evalParser(source);
  const ast = ebnfParser.parse(src);
  return { name, rel, src, gen, source, parser, ast };
}

// newParser returns a fresh parser object of a grammar. Tests that patch the
// parser use it so that they do not change the shared instance.
export function newParser(grammar) {
  return evalParser(grammar.source);
}

let chunkCache;

// distParser evaluates the jison parser that the mermaid dist embeds for a
// grammar file. It returns undefined when the dist has no such parser.
export function distParser(rel) {
  const dir = path.join(distRoot, 'chunks/mermaid.core');
  chunkCache ??= fs
    .readdirSync(dir)
    .filter((f) => f.endsWith('.mjs'))
    .map((f) => ({ file: f, text: fs.readFileSync(path.join(dir, f), 'utf8') }));
  const marker = `// src/diagrams/${rel}\n`;
  for (const chunk of chunkCache) {
    const start = chunk.text.indexOf(marker);
    if (start < 0) {
      continue;
    }
    const end = chunk.text.indexOf('\nparser.parser = parser;', start);
    const body = chunk.text.slice(start, end);
    const parser = new Function('__name', body + '\nreturn parser;')((f) => f);
    return { chunk: chunk.file, parser };
  }
  return undefined;
}

// sameTables compares the parts of two generated parsers that the Go driver uses.
export function sameTables(a, b) {
  const same = (x, y) => JSON.stringify(x) === JSON.stringify(y);
  const diff = [];
  if (!same(a.table, b.table)) diff.push('table');
  if (!same(a.defaultActions, b.defaultActions)) diff.push('defaultActions');
  if (!same(a.productions_, b.productions_)) diff.push('productions_');
  if (!same(a.symbols_, b.symbols_)) diff.push('symbols_');
  if (!same(a.terminals_, b.terminals_)) diff.push('terminals_');
  if (!same(a.lexer.rules.map(String), b.lexer.rules.map(String))) diff.push('lexer.rules');
  if (!same(a.lexer.conditions, b.lexer.conditions)) diff.push('lexer.conditions');
  if (!same(a.lexer.options, b.lexer.options)) diff.push('lexer.options');
  return diff;
}

// ruleBody strips the "^(?:" and ")" that jison-lex puts around every rule.
export function ruleBody(re) {
  const s = re.source;
  if (!s.startsWith('^(?:') || !s.endsWith(')')) {
    throw new Error('unexpected rule shape: ' + s);
  }
  return s.slice(4, -1);
}

// ---------------------------------------------------------------------------
// Regular expressions
//
// parseRegex parses the JavaScript regular expression syntax that the grammars
// use (no "u" flag, with the Annex B extensions). The result is a tree of:
//   {t:'alt', alts}          disjunction
//   {t:'seq', items}         alternative
//   {t:'set', src}           one code unit: literal, escape, class or dot
//   {t:'group', body}        capturing or non-capturing group
//   {t:'look', neg, body}    lookahead
//   {t:'assert', kind}       ^ $ \b \B
//   {t:'quant', min, max, lazy, body}
// It throws for constructs that no grammar uses, so that a new construct in a
// future grammar is noticed.

export function parseRegex(body) {
  let groups = 0;
  for (let i = 0; i < body.length; i++) {
    const c = body[i];
    if (c === '\\') {
      i++;
    } else if (c === '[') {
      i = classEnd(body, i) - 1;
    } else if (c === '(' && body[i + 1] !== '?') {
      groups++;
    }
  }
  let pos = 0;
  const peek = () => body[pos];
  const fail = (msg) => {
    throw new Error(`regex /${body}/: ${msg} at ${pos}`);
  };

  function disjunction() {
    const alts = [alternative()];
    while (peek() === '|') {
      pos++;
      alts.push(alternative());
    }
    return alts.length === 1 ? alts[0] : { t: 'alt', alts };
  }

  function alternative() {
    const items = [];
    while (pos < body.length && peek() !== '|' && peek() !== ')') {
      items.push(term());
    }
    return { t: 'seq', items };
  }

  function term() {
    const c = peek();
    if (c === '^' || c === '$') {
      pos++;
      return { t: 'assert', kind: c };
    }
    if (c === '\\' && (body[pos + 1] === 'b' || body[pos + 1] === 'B')) {
      pos += 2;
      return { t: 'assert', kind: body[pos - 1] };
    }
    let atom;
    if (c === '(') {
      atom = group();
    } else if (c === '[') {
      const end = classEnd(body, pos);
      atom = { t: 'set', src: body.slice(pos, end) };
      pos = end;
    } else if (c === '.') {
      pos++;
      atom = { t: 'set', src: '.' };
    } else if (c === '\\') {
      atom = escape();
    } else if (c === '*' || c === '+' || c === '?') {
      fail('nothing to repeat');
    } else {
      if (c === '{' && /^\{\d+(,\d*)?\}/.test(body.slice(pos))) {
        fail('nothing to repeat');
      }
      pos++;
      atom = { t: 'set', src: unitEscape(c.charCodeAt(0)) };
    }
    return quantifier(atom);
  }

  function group() {
    pos++; // (
    let node;
    if (peek() === '?') {
      const kind = body[pos + 1];
      if (kind === ':') {
        pos += 2;
        node = { t: 'group', body: disjunction() };
      } else if (kind === '=' || kind === '!') {
        pos += 2;
        node = { t: 'look', neg: kind === '!', body: disjunction() };
      } else {
        fail('unsupported group');
      }
    } else {
      node = { t: 'group', body: disjunction() };
    }
    if (peek() !== ')') {
      fail('missing )');
    }
    pos++;
    return node;
  }

  function escape() {
    const rest = body.slice(pos);
    let m;
    if ((m = /^\\[dDsSwW]/.exec(rest))) {
      // Character class escape.
    } else if ((m = /^\\c[A-Za-z]/.exec(rest))) {
      // Control letter.
    } else if (/^\\c/.test(rest)) {
      fail('unsupported \\c');
    } else if ((m = /^\\x[0-9a-fA-F]{2}/.exec(rest))) {
      // Hex escape.
    } else if ((m = /^\\u[0-9a-fA-F]{4}/.exec(rest))) {
      // Unicode escape.
    } else if ((m = /^\\[1-9][0-9]*/.exec(rest))) {
      if (Number(m[0].slice(1)) <= groups) {
        fail('unsupported backreference');
      }
      m = /^\\(?:[0-3][0-7]{0,2}|[4-7][0-7]?|[89])/.exec(rest);
    } else if ((m = /^\\0[0-7]{0,2}/.exec(rest))) {
      // NUL or a legacy octal escape.
    } else if (/^\\k</.test(rest)) {
      fail('unsupported named backreference');
    } else if (rest.length < 2) {
      fail('\\ at end of pattern');
    } else {
      m = [rest.slice(0, 2)];
    }
    pos += m[0].length;
    return { t: 'set', src: m[0] };
  }

  function quantifier(atom) {
    const c = peek();
    let min;
    let max;
    if (c === '*') {
      [min, max] = [0, Infinity];
      pos++;
    } else if (c === '+') {
      [min, max] = [1, Infinity];
      pos++;
    } else if (c === '?') {
      [min, max] = [0, 1];
      pos++;
    } else if (c === '{') {
      const m = /^\{(\d+)(,(\d*))?\}/.exec(body.slice(pos));
      if (!m) {
        return atom;
      }
      min = Number(m[1]);
      max = m[2] === undefined ? min : m[3] === '' ? Infinity : Number(m[3]);
      pos += m[0].length;
    } else {
      return atom;
    }
    let lazy = false;
    if (peek() === '?') {
      lazy = true;
      pos++;
    }
    const q = { t: 'quant', min, max, lazy, body: atom };
    if (['*', '+', '?'].includes(peek()) || /^\{\d+(,\d*)?\}/.test(body.slice(pos))) {
      fail('nothing to repeat');
    }
    return q;
  }

  const tree = disjunction();
  if (pos !== body.length) {
    fail('unmatched )');
  }
  return tree;
}

function classEnd(body, start) {
  for (let i = start + 1; i < body.length; i++) {
    if (body[i] === '\\') {
      i++;
    } else if (body[i] === ']') {
      return i + 1;
    }
  }
  throw new Error(`regex /${body}/: unterminated class`);
}

const hex4 = (c) => c.toString(16).padStart(4, '0');

function unitEscape(c) {
  return '\\u' + hex4(c);
}

const setCache = new Map();

// unitSet returns the code units that a one-unit matcher accepts, as a
// Uint8Array of 65536 flags. V8 itself decides the membership, so that
// character classes, escapes and case folding are exact.
export function unitSet(src, flags) {
  const key = flags + ' ' + src;
  let set = setCache.get(key);
  if (!set) {
    const re = new RegExp('^(?:' + src + ')$', flags.replace(/[gy]/g, ''));
    set = new Uint8Array(65536);
    for (let c = 0; c < 65536; c++) {
      set[c] = re.test(String.fromCharCode(c)) ? 1 : 0;
    }
    setCache.set(key, set);
  }
  return set;
}

// bitsOf returns the code units of a set node. A node that simplify made
// carries its units. Other nodes get them from V8.
export function bitsOf(node, flags) {
  return node.bits ?? unitSet(node.src, flags);
}

// simplify rewrites a parsed regex into an equivalent one that is cheaper for
// a backtracking engine. It has one rule: a lookahead for one code unit that
// stands in front of a one-unit matcher is the same as one matcher for the
// intersection of the two sets, or for the difference when the lookahead is
// negative. For example (?!\n)\s is the set of \s without \n. The check in
// gen-jison-golden.mjs runs the result in V8 against the original rule.
export function simplify(node, flags) {
  const single = (n) => {
    // The one set that a group or a sequence wraps, or undefined.
    while (n.t === 'group' || (n.t === 'seq' && n.items.length === 1)) {
      n = n.t === 'group' ? n.body : n.items[0];
    }
    return n.t === 'set' ? n : undefined;
  };
  const walk = (n) => {
    switch (n.t) {
      case 'alt':
        return { ...n, alts: n.alts.map(walk) };
      case 'seq': {
        const items = [];
        for (let i = 0; i < n.items.length; i++) {
          const item = n.items[i];
          const next = n.items[i + 1];
          const body = item.t === 'look' ? single(item.body) : undefined;
          if (body && next?.t === 'set') {
            const a = bitsOf(body, flags);
            const b = bitsOf(next, flags);
            const bits = new Uint8Array(65536);
            for (let c = 0; c < 65536; c++) {
              bits[c] = b[c] && (item.neg ? !a[c] : a[c]) ? 1 : 0;
            }
            items.push({ t: 'set', bits });
            i++;
          } else {
            items.push(walk(item));
          }
        }
        return { ...n, items };
      }
      case 'group':
      case 'look':
      case 'quant':
        return { ...n, body: walk(n.body) };
    }
    return n;
  };
  return walk(node);
}

// setRanges returns the members of a set as a flat list [lo, hi, lo, hi, ...].
export function setRanges(set) {
  const out = [];
  for (let c = 0; c < 65536; c++) {
    if (set[c]) {
      const lo = c;
      while (c + 1 < 65536 && set[c + 1]) {
        c++;
      }
      out.push(lo, c);
    }
  }
  return out;
}

const plain = (c) => (c >= 48 && c <= 57) || (c >= 65 && c <= 90) || (c >= 97 && c <= 122);
const unitText = (c) => (plain(c) ? String.fromCharCode(c) : unitEscape(c));

function rangesText(ranges) {
  let s = '';
  for (let i = 0; i < ranges.length; i += 2) {
    const [lo, hi] = [ranges[i], ranges[i + 1]];
    s += lo === hi ? unitText(lo) : unitText(lo) + (hi === lo + 1 ? '' : '-') + unitText(hi);
  }
  return s;
}

// classText prints a set as a pattern that has the same meaning in V8 (without
// flags) and in regexp2: one literal, or a class of explicit code unit ranges.
export function classText(set) {
  const ranges = setRanges(set);
  let count = 0;
  for (let i = 0; i < ranges.length; i += 2) {
    count += ranges[i + 1] - ranges[i] + 1;
  }
  if (count === 1) {
    return unitText(ranges[0]);
  }
  if (count === 0) {
    return '[^\\u0000-\\uffff]';
  }
  if (count > 32768) {
    const inverse = new Uint8Array(65536);
    for (let c = 0; c < 65536; c++) {
      inverse[c] = set[c] ? 0 : 1;
    }
    return '[^' + rangesText(setRanges(inverse)) + ']';
  }
  return '[' + rangesText(ranges) + ']';
}

// normalize prints a parsed regex with every one-unit matcher replaced by its
// explicit code unit set. The result needs no flags: case folding is in the
// sets. Capturing groups become non-capturing groups, because no lexer action
// reads this.matches. opts.end is the text for "$".
export function normalize(node, flags, opts = {}) {
  const end = opts.end ?? '$';
  const print = (n) => {
    switch (n.t) {
      case 'alt':
        return n.alts.map(print).join('|');
      case 'seq':
        return n.items.map(print).join('');
      case 'set':
        return classText(bitsOf(n, flags));
      case 'group':
        return '(?:' + print(n.body) + ')';
      case 'look':
        return (n.neg ? '(?!' : '(?=') + print(n.body) + ')';
      case 'assert':
        return n.kind === '^' ? '^' : n.kind === '$' ? end : '\\' + n.kind;
      case 'quant': {
        let q;
        if (n.min === 0 && n.max === Infinity) q = '*';
        else if (n.min === 1 && n.max === Infinity) q = '+';
        else if (n.min === 0 && n.max === 1) q = '?';
        else if (n.max === Infinity) q = `{${n.min},}`;
        else if (n.max === n.min) q = `{${n.min}}`;
        else q = `{${n.min},${n.max}}`;
        let inner = print(n.body);
        if (n.body.t === 'look') {
          inner = '(?:' + inner + ')';
        }
        return inner + q + (n.lazy ? '?' : '');
      }
    }
    throw new Error('unknown node ' + n.t);
  };
  return print(node);
}

// firstSet computes which code units can start a match of a parsed regex, and
// whether the regex can match without consuming a code unit. Lookaheads and
// assertions do not restrict the set, so the result is a superset.
export function firstSet(node, flags) {
  const walk = (n) => {
    switch (n.t) {
      case 'alt': {
        const set = new Uint8Array(65536);
        let nullable = false;
        for (const a of n.alts) {
          const r = walk(a);
          or(set, r.set);
          nullable ||= r.nullable;
        }
        return { set, nullable };
      }
      case 'seq': {
        const set = new Uint8Array(65536);
        for (const item of n.items) {
          const r = walk(item);
          or(set, r.set);
          if (!r.nullable) {
            return { set, nullable: false };
          }
        }
        return { set, nullable: true };
      }
      case 'set':
        return { set: bitsOf(n, flags), nullable: false };
      case 'group':
        return walk(n.body);
      case 'look':
      case 'assert':
        return { set: new Uint8Array(65536), nullable: true };
      case 'quant': {
        if (n.max === 0) {
          return { set: new Uint8Array(65536), nullable: true };
        }
        const r = walk(n.body);
        return { set: r.set, nullable: r.nullable || n.min === 0 };
      }
    }
    throw new Error('unknown node ' + n.t);
  };
  return walk(node);
}

function or(dst, src) {
  for (let c = 0; c < 65536; c++) {
    if (src[c]) {
      dst[c] = 1;
    }
  }
}

// requiredLiteral finds a text that every match of a rule contains, for rules
// that start with an unbounded repetition. Such a rule, for example
// .*direction\s+TB[^\n]*, makes a backtracking engine scan the line for every
// token. The result describes the rule as: units of the set "before", then a
// literal. The literal is a list of positions, and each position has one or
// two code units (two for a letter of a case-insensitive grammar).
//
// The lexer uses it as follows. Let e be the first position at or after the
// current position whose unit is not in "before". The rule can match only if
// the literal starts at a position q with q <= e. The reason: the items in
// front of the literal consume only units of "before", so the literal of a
// match starts at or before e, and the first occurrence of the literal is not
// later than that.
export function requiredLiteral(node, flags) {
  while (node.t === 'group' || (node.t === 'seq' && node.items.length === 1)) {
    node = node.t === 'group' ? node.body : node.items[0];
  }
  if (node.t !== 'seq') {
    return undefined;
  }
  const small = (n) => {
    if (n.t !== 'set') return undefined;
    const ranges = setRanges(bitsOf(n, flags));
    const units = [];
    for (let i = 0; i < ranges.length; i += 2) {
      for (let c = ranges[i]; c <= ranges[i + 1]; c++) {
        units.push(c);
        if (units.length > 2) return undefined;
      }
    }
    return units.length ? units : undefined;
  };
  // Collect the units that the items in front of the literal can consume.
  const before = new Uint8Array(65536);
  const collect = (n) => {
    switch (n.t) {
      case 'alt':
        n.alts.forEach(collect);
        break;
      case 'seq':
        n.items.forEach(collect);
        break;
      case 'set':
        or(before, bitsOf(n, flags));
        break;
      case 'group':
      case 'quant':
        collect(n.body);
        break;
      // A lookahead and an assertion consume nothing.
    }
  };
  let unbounded = false;
  for (let k = 0; k < node.items.length; k++) {
    const item = node.items[k];
    if (unbounded && small(item)) {
      const literal = [];
      for (let j = k; j < node.items.length && small(node.items[j]); j++) {
        const units = small(node.items[j]);
        literal.push(units[0], units[units.length - 1]);
      }
      return { before: setRanges(before), literal };
    }
    collect(item);
    // Only a rule that starts with the repetition is worth the test.
    unbounded ||= k === 0 && item.t === 'quant' && item.max === Infinity;
    if (!unbounded) {
      return undefined;
    }
  }
  return undefined;
}

// literalAdmits is the test that the Go lexer makes with requiredLiteral: it
// returns false only when the rule cannot match at the start of input.
export function literalAdmits(need, input) {
  const inBefore = (c) => {
    for (let i = 0; i < need.before.length; i += 2) {
      if (c >= need.before[i] && c <= need.before[i + 1]) return true;
    }
    return false;
  };
  let e = 0;
  while (e < input.length && inBefore(input.charCodeAt(e))) e++;
  const n = need.literal.length / 2;
  for (let q = 0; q <= e && q + n <= input.length; q++) {
    let ok = true;
    for (let i = 0; i < n && ok; i++) {
      const c = input.charCodeAt(q + i);
      ok = c === need.literal[2 * i] || c === need.literal[2 * i + 1];
    }
    if (ok) return true;
  }
  return false;
}

// re2Compatible reports whether a parsed regex uses only constructs that Go's
// regexp package has with the same meaning once the sets are explicit.
export function re2Compatible(node) {
  switch (node.t) {
    case 'alt':
      return node.alts.every(re2Compatible);
    case 'seq':
      return node.items.every(re2Compatible);
    case 'set':
      return true;
    case 'group':
      return re2Compatible(node.body);
    case 'look':
      return false;
    case 'assert':
      return true;
    case 'quant':
      return node.max === Infinity || node.max <= 1000 ? re2Compatible(node.body) : false;
  }
  return false;
}

// analyzeRule returns everything that the Go driver needs for one lexer rule.
export function analyzeRule(re) {
  const body = ruleBody(re);
  const flags = re.flags;
  const tree = simplify(parseRegex(body), flags);
  const first = firstSet(tree, flags);
  return {
    source: body,
    flags,
    tree,
    // The Go pattern. "$" is a lookahead for "no code unit follows", which has
    // the meaning of the JavaScript "$" without the "m" flag in every engine.
    pattern: normalize(tree, flags, { end: '(?![\\u0000-\\uffff])' }),
    // The same pattern in a form that V8 accepts, for the self-check.
    jsPattern: normalize(tree, flags),
    first: setRanges(first.set),
    nullable: first.nullable,
    need: requiredLiteral(tree, flags),
    re2: re2Compatible(tree),
  };
}

// ---------------------------------------------------------------------------
// Seeded random numbers (mulberry32).

export function prng(seed) {
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
// Corpus

// listCorpus returns the paths of all corpus fixtures, relative to the repository root.
export function listCorpus() {
  return fs
    .readdirSync(corpusRoot, { recursive: true })
    .filter((f) => f.endsWith('.mmd'))
    .map((f) => 'testdata/corpus/' + f.split(path.sep).join('/'))
    .sort();
}

const stop = Symbol('stop');

// corpusInputs runs every corpus fixture through mermaid.parse of the mermaid
// dist until the jison lexer receives its input, and returns that input. So
// the text is exactly what mermaid gives to the generated parser: after
// preprocessDiagram (frontmatter, directives, comments), encodeEntities, the
// trailing newline, and the wrapper of the diagram parser (flow, sankey,
// agentflow). Fixtures of diagram types that do not use jison are skipped.
export async function corpusInputs(grammars) {
  const mermaid = (await import(pathToFileURL(path.join(distRoot, 'mermaid.core.mjs')).href))
    .default;
  const registry = await import(
    pathToFileURL(path.join(distRoot, 'chunks/mermaid.core/chunk-DSNLSHJZ.mjs')).href
  );
  mermaid.initialize({ startOnLoad: false });
  const bySymbols = new Map(grammars.map((g) => [JSON.stringify(g.parser.symbols_), g]));
  let captured;
  // Load every diagram and replace setInput of each jison lexer.
  const types = {};
  for (const type of Object.keys(registry.detectors)) {
    let jisonParser;
    try {
      const loader = registry.getDiagramLoader(type);
      if (loader) {
        const { id, diagram } = await loader();
        registry.registerDiagram(id, diagram);
      }
      jisonParser = registry.getDiagram(type).parser.parser;
    } catch (e) {
      types[type] = 'not loadable: ' + String(e?.message ?? e).split('\n')[0];
      continue;
    }
    if (!jisonParser?.symbols_) {
      types[type] = 'other';
      continue;
    }
    const grammar = bySymbols.get(JSON.stringify(jisonParser.symbols_));
    if (!grammar) {
      throw new Error(`no grammar matches the jison parser of diagram type ${type}`);
    }
    types[type] = grammar.name;
    jisonParser.lexer.setInput = function (input) {
      captured = { grammar, type, input };
      throw stop;
    };
  }
  const out = [];
  const skipped = new Map();
  for (const file of listCorpus()) {
    const text = fs.readFileSync(path.join(repoRoot, file), 'utf8');
    captured = undefined;
    let error;
    try {
      await mermaid.parse(text);
    } catch (e) {
      if (e !== stop) {
        error = String(e?.message ?? e).split('\n')[0];
      }
    }
    if (!captured) {
      let type = '?';
      try {
        type = mermaid.detectType(text);
      } catch {
        // No diagram type.
      }
      skipped.set(file, `type ${type}` + (error ? `: ${error}` : ''));
      continue;
    }
    out.push({ file, type: captured.type, grammar: captured.grammar.name, input: captured.input });
  }
  return { inputs: out, skipped, types };
}

// ---------------------------------------------------------------------------
// Recording yy

// harmless is the value that every recorded yy method returns: a callable
// object whose properties are again harmless values. It converts to the empty
// string and iterates as an empty list.
function makeHarmless() {
  const target = function () {};
  const proxy = new Proxy(target, {
    get(_, prop) {
      if (prop === Symbol.toPrimitive) return () => '';
      if (prop === Symbol.iterator) return function* () {};
      if (prop === 'then' || prop === 'toJSON') return undefined;
      if (prop === 'length') return 0;
      if (prop === 'toString') return () => '';
      return proxy;
    },
    set() {
      return true;
    },
    apply() {
      return proxy;
    },
    construct() {
      return proxy;
    },
    has() {
      return true;
    },
  });
  return proxy;
}

export const harmless = makeHarmless();

// yyNames lists the properties of yy that the actions of a grammar read.
export function yyNames(grammar) {
  const text = grammar.gen.performAction + '\n' + grammar.gen.lexer.performAction;
  const names = new Set();
  for (const m of text.matchAll(/\byy\.([A-Za-z_$][\w$]*)/g)) {
    names.add(m[1]);
  }
  names.delete('lexer');
  names.delete('parser');
  return [...names].sort();
}

// recordingYY returns the yy object for a trace run. The generated parser
// copies the own properties of parser.yy, so every name must be an own
// property. Every method returns the harmless value, with these exceptions,
// where the lexer depends on the result:
//   flow, agentflow: yy.lex.firstGraph() is true for the first call only, as
//     in FlowDB.firstGraph.
//   venn: getIndentMode and setIndentMode keep a flag, as in vennDB.
export function recordingYY(grammar) {
  const yy = {};
  for (const name of yyNames(grammar)) {
    yy[name] = harmless;
  }
  if ('lex' in yy) {
    let first = true;
    yy.lex = {
      firstGraph() {
        if (first) {
          first = false;
          return true;
        }
        return false;
      },
    };
  }
  if ('getIndentMode' in yy) {
    let indentMode = false;
    yy.getIndentMode = () => indentMode;
    yy.setIndentMode = (v) => {
      indentMode = v;
    };
    yy.consumeIndentText = false;
  }
  return yy;
}

// ---------------------------------------------------------------------------
// Traces

// quote prints a string as JSON.stringify does. A lone surrogate becomes a
// \udXXX escape, so the text is valid UTF-8 and loses nothing.
export const quote = (s) => JSON.stringify(String(s));

const locText = (l) => `${l.first_line}:${l.first_column}-${l.last_line}:${l.last_column}`;

// symbolName gives the name that the parser prints for a token in an error message.
export function symbolName(parser, token) {
  const symbol = typeof token === 'number' ? token : parser.symbols_[token] || token;
  return String(parser.terminals_[symbol] || symbol);
}

// tracer patches a parser object so that parse() records every token that the
// lexer gives to the parser, and every reduction. lexer.lex calls itself when
// a rule gives no token, so only the outermost call records.
export function tracer(grammar, parser) {
  const lexer = parser.lexer;
  const next = lexer.next;
  const lex = lexer.lex;
  const perform = parser.performAction;
  let events;
  let onNext;
  let halt;
  let depth = 0;
  lexer.next = function () {
    onNext?.(this);
    return next.call(this);
  };
  lexer.lex = function () {
    depth++;
    let r;
    try {
      r = lex.call(this);
    } finally {
      depth--;
    }
    if (depth === 0) {
      events.push(
        `T ${symbolName(parser, r)}\t${this.conditionStack.join(' ')}\t${this.yylineno}\t${this.yyleng}\t${locText(this.yylloc)}\t${quote(this.yytext)}`
      );
    }
    return r;
  };
  parser.performAction = function (yytext, yyleng, yylineno, yy, yystate) {
    events.push(`R ${yystate}\t${locText(this._$)}`);
    const r = perform.apply(this, arguments);
    if (r !== undefined) {
      halt = yystate;
    }
    return r;
  };
  // run parses one input and returns the trace lines. hooks.yy replaces the
  // recording yy object, and hooks.result adds the value that parse returns.
  return function run(input, hooks = {}) {
    events = [];
    halt = undefined;
    depth = 0;
    onNext = hooks.onNext;
    parser.yy = hooks.yy ? hooks.yy(events) : recordingYY(grammar);
    try {
      const result = parser.parse(input);
      if (halt === undefined && result !== true) {
        throw new Error('parse returned ' + String(result) + ' without an action');
      }
      events.push(halt === undefined ? 'OK accept' : `OK return ${halt}`);
      if (hooks.result) {
        events.push(`V ${quote(result)}`);
      }
    } catch (e) {
      const message = e instanceof Error ? e.message : String(e);
      events.push(`ERR ${e?.name ?? 'throw'}\t${quote(message)}`);
      if (e?.hash) {
        const h = e.hash;
        events.push(
          `H ${quote(h.token ?? '')}\t${h.line}\t${quote(h.text ?? '')}\t${h.loc ? locText(h.loc) : '-'}\t${quote((h.expected ?? []).join(','))}`
        );
      }
    }
    return events;
  };
}

// parserActions lists the semantic action of every production, in the form
// that the grammar file has (with named references resolved to numbers). It
// repeats the steps of buildProductions in jison/lib/jison.js and checks the
// result against the generated performAction.
export function parserActions(grammar) {
  const { ast, gen } = grammar;
  const out = [{ id: 0, lhs: '$accept', rhs: [gen.startSymbol, '$end'], action: null }];
  const groups = new Map();
  for (const symbol of Object.keys(ast.bnf)) {
    const prods =
      typeof ast.bnf[symbol] === 'string' ? ast.bnf[symbol].split(/\s*\|\s*/g) : ast.bnf[symbol];
    for (let handle of prods) {
      const id = out.length;
      let rhs;
      let action = null;
      let processed = null;
      if (Array.isArray(handle)) {
        rhs = typeof handle[0] === 'string' ? handle[0].trim().split(' ') : handle[0].slice(0);
        if (typeof handle[1] === 'string' || handle.length === 3) {
          action = handle[1];
          if (action.match(/[$@][a-zA-Z][a-zA-Z0-9_]*/)) {
            const count = {};
            const names = {};
            for (let i = 0; i < rhs.length; i++) {
              let alias = rhs[i].match(/\[[a-zA-Z][a-zA-Z0-9_-]*\]/);
              if (alias) {
                alias = alias[0].substr(1, alias[0].length - 2);
                rhs[i] = rhs[i].substr(0, rhs[i].indexOf('['));
              } else {
                alias = rhs[i];
              }
              if (names[alias]) {
                names[alias + ++count[alias]] = i + 1;
              } else {
                names[alias] = i + 1;
                names[alias + '1'] = i + 1;
                count[alias] = 1;
              }
            }
            action = action
              .replace(/\$([a-zA-Z][a-zA-Z0-9_]*)/g, (str, pl) => (names[pl] ? '$' + names[pl] : str))
              .replace(/@([a-zA-Z][a-zA-Z0-9_]*)/g, (str, pl) => (names[pl] ? '@' + names[pl] : str));
          }
          processed = action
            .replace(/([^'"])\$\$|^\$\$/g, '$1this.$')
            .replace(/@[0$]/g, 'this._$')
            .replace(/\$(-?\d+)/g, (_, n) => '$$[$0' + (parseInt(n, 10) - rhs.length || '') + ']')
            .replace(/@(-?\d+)/g, (_, n) => '_$[$0' + (n - rhs.length || '') + ']');
          const label = 'case ' + id + ':';
          if (groups.has(processed)) groups.get(processed).push(label);
          else groups.set(processed, [label]);
        }
        rhs = rhs.map((e) => e.replace(/\[[a-zA-Z_][a-zA-Z0-9_-]*\]/g, ''));
      } else {
        handle = handle.replace(/\[[a-zA-Z_][a-zA-Z0-9_-]*\]/g, '');
        rhs = handle.trim().split(' ');
      }
      if (rhs.length === 1 && rhs[0] === '') {
        rhs = [];
      }
      out.push({ id, lhs: symbol, rhs, action, processed });
    }
  }
  // Check against the generator.
  const lines = ['/* this == yyval */', gen.actionInclude || '', 'var $0 = $$.length - 1;', 'switch (yystate) {'];
  for (const [action, labels] of groups) {
    lines.push(labels.join(' '), action, 'break;');
  }
  lines.push('}');
  const text = lines.join('\n').replace(/YYABORT/g, 'return false').replace(/YYACCEPT/g, 'return true');
  let parameters = 'yytext, yyleng, yylineno, yy, yystate /* action[1] */, $$ /* vstack */, _$ /* lstack */';
  if (gen.parseParams) parameters += ', ' + gen.parseParams.join(', ');
  if ('function anonymous(' + parameters + ') {\n' + text + '\n}' !== gen.performAction) {
    throw new Error(`${grammar.name}: the derived semantic actions differ from jison's performAction`);
  }
  for (const p of out) {
    const g = gen.productions[p.id];
    const handle = g.handle[0] === '' ? [] : g.handle;
    if (g.symbol !== p.lhs || JSON.stringify(handle) !== JSON.stringify(p.rhs)) {
      throw new Error(`${grammar.name}: production ${p.id} differs from jison's production`);
    }
    if (gen.productions_[p.id] && (gen.productions_[p.id][0] !== gen.symbols_[p.lhs] || gen.productions_[p.id][1] !== p.rhs.length)) {
      throw new Error(`${grammar.name}: production ${p.id} differs from productions_`);
    }
    // An action that has a return statement ends the parse.
    p.halts = p.action !== null && /\breturn\b|YYACCEPT|YYABORT/.test(p.action);
  }
  return out;
}

// lexerRules lists the lexer rules of a grammar with their action text and
// their start conditions.
export function lexerRules(grammar) {
  const { ast, parser } = grammar;
  const lexer = parser.lexer;
  if (ast.lex.rules.length !== lexer.rules.length) {
    throw new Error(`${grammar.name}: lex rule count differs`);
  }
  return lexer.rules.map((re, i) => {
    const raw = ast.lex.rules[i];
    const conditions = Object.keys(lexer.conditions).filter((c) => lexer.conditions[c].rules.includes(i));
    return { index: i, re, action: raw[raw.length - 1], conditions };
  });
}

// ---------------------------------------------------------------------------
// Go output

export const goString = (s) =>
  JSON.stringify(s).replace(/[\u2028\u2029]/g, (c) => '\\u' + c.charCodeAt(0).toString(16));

function numberLines(nums, perLine = 24, indent = '\t\t') {
  const lines = [];
  for (let i = 0; i < nums.length; i += perLine) {
    lines.push(indent + nums.slice(i, i + perLine).join(', ') + ',');
  }
  return lines.join('\n');
}

const oneLine = (jsText) => jsText.replace(/\s+/g, ' ').trim();

// grammarTables converts the tables of a generated parser to the form of
// jison.Grammar in Go. It returns the symbol names, the terminal numbers and
// the lines of the composite literal, without the type name.
export function grammarTables(grammar, ruleInfos) {
  const { parser, name, rel, gen } = grammar;
  const symbols = [];
  for (const [sym, id] of Object.entries(parser.symbols_)) {
    symbols[id] = sym;
  }
  for (let i = 0; i < symbols.length; i++) {
    if (symbols[i] === undefined) {
      throw new Error(`${name}: symbol numbers are not dense`);
    }
  }
  const terminalIDs = Object.keys(parser.terminals_)
    .map(Number)
    .sort((a, b) => a - b);
  for (const id of terminalIDs) {
    if (parser.terminals_[id] !== symbols[id]) {
      throw new Error(`${name}: terminals_ and symbols_ differ for ${id}`);
    }
  }
  const productions = [parser.symbols_.$accept, 2];
  for (let i = 1; i < parser.productions_.length; i++) {
    productions.push(parser.productions_[i][0], parser.productions_[i][1]);
  }
  const table = [];
  for (const state of parser.table) {
    const keys = Object.keys(state)
      .map(Number)
      .sort((a, b) => a - b);
    table.push(keys.length);
    for (const sym of keys) {
      const act = state[sym];
      let enc;
      if (typeof act === 'number') {
        enc = act << 2;
      } else if (Array.isArray(act) && typeof act[0] === 'number' && act.length <= 2) {
        if (act[0] < 1 || act[0] > 3) {
          throw new Error(`${name}: unknown action ${JSON.stringify(act)}`);
        }
        enc = act[0] === 3 ? 3 : (act[1] << 2) | act[0];
      } else {
        throw new Error(`${name}: conflict or unknown action ${JSON.stringify(act)}`);
      }
      if (enc <= 0 || enc > 0xffff || sym > 0xffff) {
        throw new Error(`${name}: table entry out of range`);
      }
      table.push(sym, enc);
    }
  }
  const defaults = [];
  for (const [state, act] of Object.entries(parser.defaultActions)) {
    if (act[0] !== 2) {
      throw new Error(`${name}: default action is not a reduction`);
    }
    defaults.push(Number(state), act[1]);
  }
  const lexer = parser.lexer;
  const options = lexer.options ?? {};
  for (const key of Object.keys(options)) {
    if (!['case-insensitive', 'flex', 'backtrack_lexer', 'ranges'].includes(key)) {
      throw new Error(`${name}: unknown lexer option ${key}`);
    }
  }
  const out = [];
  out.push(`\tName: ${goString(path.basename(rel, '.jison'))},`, '\tSymbols: []string{');
  for (let i = 0; i < symbols.length; i += 8) {
    out.push('\t\t' + symbols.slice(i, i + 8).map(goString).join(', ') + ',');
  }
  out.push('\t},');
  out.push('\tTerminals: []uint16{', numberLines(terminalIDs), '\t},');
  out.push('\tProductions: []uint16{', numberLines(productions), '\t},');
  out.push('\tTable: []uint16{', numberLines(table), '\t},');
  out.push('\tDefaultActions: []uint16{', numberLines(defaults), '\t},');
  out.push('\tRules: []jison.Rule{');
  ruleInfos.forEach((info, index) => {
    out.push(`\t\t// ${index}: /${oneLine(info.source)}/${info.flags}`);
    if (info.pattern.includes('`')) {
      throw new Error(`${name}: pattern with a backtick`);
    }
    let line = `\t\t{Pattern: \`${info.pattern}\``;
    if (info.first.length) {
      line += `, First: []uint16{${info.first.join(', ')}}`;
    }
    if (info.nullable) {
      line += ', Nullable: true';
    }
    if (info.need) {
      line += `, Before: []uint16{${info.need.before.join(', ')}}, Literal: []uint16{${info.need.literal.join(', ')}}`;
    }
    out.push(line + '},');
  });
  out.push('\t},');
  out.push('\tConditions: []jison.Condition{');
  for (const [cond, c] of Object.entries(lexer.conditions)) {
    out.push(
      `\t\t{Name: ${goString(cond)}, Inclusive: ${c.inclusive}, Rules: []uint16{${c.rules.join(', ')}}},`
    );
  }
  out.push('\t},');
  if (options['case-insensitive']) out.push('\tCaseInsensitive: true,');
  if (options.flex) out.push('\tFlex: true,');
  if (options.backtrack_lexer) out.push('\tBacktrackLexer: true,');
  if (options.ranges) out.push('\tRanges: true,');
  if (gen.hasErrorRecovery) out.push('\tErrorRecovery: true,');
  return { symbols, terminalIDs, lines: out };
}
