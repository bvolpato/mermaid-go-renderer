// Writes two golden files of internal/marked from the marked that the
// mermaid 12.2.0 bundle inlines (marked 16.3.0).
//
//   cd tools/reference && node gen-marked-rules.mjs
//
// rules.json holds the source and the flags of each regular expression of
// marked, and the character sets that the rules name, as V8 matches them.
//
// rules_exec.json.gz holds the matches of each rule that the Go port
// compiles, on probe strings. A match is the text "index,length" and then,
// for each capture group, ";start,end" or ";-" for a group that did not
// take part. The indexes count UTF-16 code units. A rule has up to three
// matches for one string, with "|" between them: the search continues
// after a match, as it does for a regex with the g flag.
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { REPO, markedLexer, markedOther, versions } from './markdown-lib.mjs';
import { inputGroups, surrogateInputs } from './markdown-inputs.mjs';

const dump = (rules) => {
  const out = {};
  for (const [name, re] of Object.entries(rules)) {
    if (re instanceof RegExp) out[name] = { source: re.source, flags: re.flags };
  }
  return out;
};

// The bullets that Tokenizer.list gives to listItemRegex.
const bullets = ['\\d{1,9}\\.', '\\d{1,9}\\)', '\\*', '\\+', '\\-'];
const indents = [1, 2, 3, 4, 5, 9];
const dynamic = {};
for (const bull of bullets) {
  const re = markedOther.listItemRegex(bull);
  dynamic[`listItemRegex ${bull}`] = { source: re.source, flags: re.flags };
}
for (const name of ['nextBulletRegex', 'hrRegex', 'fencesBeginRegex', 'headingBeginRegex', 'htmlBeginRegex']) {
  for (const indent of indents) {
    const re = markedOther[name](indent);
    dynamic[`${name} ${indent}`] = { source: re.source, flags: re.flags };
  }
}

// The character sets that the rules name. Each set is a list of inclusive
// code point ranges, as V8 matches them.
const ranges = (re) => {
  const out = [];
  let start = -1;
  for (let cp = 0; cp <= 0x110000; cp++) {
    const hit = cp <= 0x10ffff && re.test(String.fromCodePoint(cp));
    if (hit && start < 0) start = cp;
    if (!hit && start >= 0) {
      out.push([start, cp - 1]);
      start = -1;
    }
  }
  return out;
};
const sets = {
  space: ranges(/^\s$/u),
  punctuation: ranges(/^\p{P}$/u),
  symbol: ranges(/^\p{S}$/u),
  letter: ranges(/^\p{L}$/u),
  number: ranges(/^\p{N}$/u),
  dot: ranges(/^.$/u),
};

const out = {
  versions: { ...versions, unicode: process.versions.unicode },
  other: dump(markedOther),
  block: dump(markedLexer.rules.block.gfm),
  inline: dump(markedLexer.rules.inline.gfm),
  dynamic,
  sets,
};
const file = path.join(REPO, 'internal/marked/testdata/rules.json');
fs.writeFileSync(file, JSON.stringify(out, null, 1) + '\n');
console.log(`wrote ${file}`);

// --- rules_exec.json.gz -----------------------------------------------------

// The rules that the Go port compiles, with the names that the Go test uses.
const goOther = [
  'codeRemoveIndent', 'outputLinkReplace', 'indentCodeCompensation', 'beginningSpace', 'multipleSpaceGlobal',
  'blankLine', 'doubleBlankLine', 'blockquoteStart', 'blockquoteSetextReplace', 'blockquoteSetextReplace2',
  'listReplaceTabs', 'listIsTask', 'listReplaceTask', 'anyLine', 'hrefBrackets', 'tableDelimiter',
  'tableAlignChars', 'tableRowBlankLine', 'tableAlignRight', 'tableAlignCenter', 'tableAlignLeft', 'startATag',
  'endATag', 'startPreScriptTag', 'endPreScriptTag', 'unicodeAlphaNumeric',
];
const rules = {};
for (const name of goOther) rules[`other.${name}`] = markedOther[name];
for (const [name, re] of Object.entries(markedLexer.rules.block.gfm)) rules[`block.${name}`] = re;
for (const [name, re] of Object.entries(markedLexer.rules.inline.gfm)) rules[`inline.${name}`] = re;
for (const bull of bullets) rules[`other.listItemRegex.${bull.at(-1)}`] = markedOther.listItemRegex(bull);
for (const name of ['nextBulletRegex', 'hrRegex', 'fencesBeginRegex', 'headingBeginRegex', 'htmlBeginRegex']) {
  // The Go port has one regex for each value of Math.min(3, indent - 1).
  for (let indent = 1; indent <= 4; indent++) rules[`other.${name}.${indent - 1}`] = markedOther[name](indent);
}

// The probe strings: parts of the inputs of lexer.json.gz, which
// gen-marked.mjs writes with the same order of cases. A probe is the index
// of a case and an offset in UTF-16 code units: the string is the rest of
// the input from the offset. The probes are each specification example and
// one in forty of the other inputs, cut after their first line feeds and
// after some other characters.
const probes = [];
const probeTexts = [];
{
  const seen = new Set();
  const add = (index, input, offset) => {
    const text = input.slice(offset);
    if (text.length <= 300 && !seen.has(text)) {
      seen.add(text);
      probes.push([index, offset]);
      probeTexts.push(text);
    }
  };
  const groups = inputGroups(REPO);
  const all = [...groups.written, ...groups.generated];
  all.forEach((input, i) => {
    if (i >= 829 && i % 40 !== 0) return;
    add(i, input, 0);
    let cuts = 0;
    for (let j = 0; j < input.length && cuts < 3; j++) {
      // A probe must not start with one half of a surrogate pair.
      const low = input.charCodeAt(j + 1) >= 0xdc00 && input.charCodeAt(j + 1) <= 0xdfff;
      if (!low && (input[j] === '\n' || (j * 7 + i) % 23 === 0)) {
        add(i, input, j + 1);
        cuts++;
      }
    }
  });
  // The inputs with a surrogate that has no partner follow in the file.
  surrogateInputs().forEach((input, k) => add(all.length + k, input, 0));
}

const execAll = (re, text) => {
  const flags = re.flags.replace('g', '') + 'gd';
  const probe = new RegExp(re.source, flags);
  const out = [];
  for (let k = 0; k < 3; k++) {
    const m = probe.exec(text);
    if (!m) break;
    let item = `${m.index},${m[0].length}`;
    for (let g = 1; g < m.length; g++) item += m.indices[g] ? `;${m.indices[g][0]},${m.indices[g][1]}` : ';-';
    out.push(item);
    if (m[0] === '') {
      // AdvanceStringIndex after an empty match.
      const c = text.codePointAt(probe.lastIndex);
      probe.lastIndex += probe.unicode && c > 0xffff ? 2 : 1;
    }
  }
  return out.join('|');
};

const exec = {};
let matches = 0;
for (const [name, re] of Object.entries(rules)) {
  const results = {};
  probeTexts.forEach((text, i) => {
    const result = execAll(re, text);
    if (result) results[i] = result;
  });
  matches += Object.keys(results).length;
  exec[name] = { flags: re.flags, results };
}
{
  const execFile = path.join(REPO, 'internal/marked/testdata/rules_exec.json.gz');
  const data = zlib.gzipSync(JSON.stringify({ probes, rules: exec }), { level: 9 });
  fs.writeFileSync(execFile, data);
  console.log(
    `wrote ${execFile}: ${Object.keys(rules).length} rules, ${probes.length} strings, ${matches} matches, ${data.length} bytes`
  );
}
