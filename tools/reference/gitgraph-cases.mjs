// Shared input generator for the gitGraph golden files: the corpus fixtures
// and seeded mutations of them. gen-gitgraph-parser.mjs and
// gen-gitgraph-db.mjs use it.
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
export const ROOT = path.resolve(HERE, '../..');
export const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e/git');
export const TESTDATA = path.join(ROOT, 'internal/diagrams/git/testdata');

// mulberry32 is the seeded PRNG that the generators share.
export const mulberry32 = (seed) => {
  let a = seed >>> 0;
  return () => {
    a = (a + 0x6d2b79f5) >>> 0;
    let t = a;
    t = Math.imul(t ^ (t >>> 15), t | 1);
    t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
};

// corpusFiles returns the fixtures as { name, text }, sorted by name.
export const corpusFiles = () => {
  const out = [];
  const walk = (dir) => {
    for (const entry of fs.readdirSync(dir, { withFileTypes: true })) {
      const full = path.join(dir, entry.name);
      if (entry.isDirectory()) walk(full);
      else if (entry.name.endsWith('.mmd')) {
        out.push({ name: path.relative(CORPUS, full), text: fs.readFileSync(full, 'utf8') });
      }
    }
  };
  walk(CORPUS);
  out.sort((a, b) => (a.name < b.name ? -1 : a.name > b.name ? 1 : 0));
  return out;
};

// stripFrontMatter removes a leading YAML front matter block.
export const stripFrontMatter = (text) => text.replace(/^-{3}\s*[\n\r](.*?)[\n\r]-{3}\s*[\n\r]+/s, '');

// The words that the mutations insert: every keyword of the grammar, and
// texts that sit on the edges of the terminal rules.
export const VOCABULARY = [
  'gitGraph', 'gitGraph:', 'gitGraph LR:', 'gitGraph TB:', 'gitGraph BT:', 'gitGraph :',
  'commit', 'branch', 'merge', 'checkout', 'switch', 'cherry-pick',
  'id:', 'msg:', 'tag:', 'type:', 'order:', 'parent:', ':',
  'NORMAL', 'REVERSE', 'HIGHLIGHT', 'LR', 'TB', 'BT',
  '"a"', "'b'", '"x\\"y"', '"two\nlines"', '"\\n\\t\\q\\\\"', '"un', "'un", '"', "'", '""', "''",
  '"a b  c"', '"é ü 日本"', '"😀"', '"\\😀"', '"a=b"', '"x>y"', '"a & b"', '"#quot;"', '"ﬂ°°35¶ß"',
  '0', '1', '12', '12.5', '1.', '007', '10.', '99999999999999999999', '3.14.15',
  'main', 'develop', 'feature/x', 'a.b', 'a-', 'a.', 'a/', '_x', 'x-y_z', '1abc', 'abc1', 'a..b', 'a/.-',
  'commit1', 'commits', 'mergex', 'LRx', 'gitGraphx', 'gitGraph-x', 'cherry-picked', 'cherry', 'id', 'tag',
  'é', 'ünï', '日本', 'x:y', 'id:"z"', 'tag:"t"', 'type:REVERSE', 'order: 3', 'order:0', 'order: 01',
  '%% comment', '%%', '%%{init: {"theme":"dark"}}%%', '%%{x}%% y', '%%{', '}%%',
  '---', '---\na: b\n---', '---\n---', '--- \nx\n--- y',
  'title', 'title x', ' title  a  b', 'titlex', 'title\tT', 'title a %% c',
  'accTitle: t', 'accTitle : t %% c', 'accTitle', 'accTitle:', 'accTitle:  a   b  ',
  'accDescr: d', 'accDescr:  a   b %%c', 'accDescr { a\n   b  c\n\n\n d }', 'accDescr{', 'accDescr\n{x}',
  'accDescr {}', 'accDescr { a \n b \n}', 'accDescr',
  ' ', '  ', '\t', '\n', '\n\n', '\r\n', '\r', ' ', ' ', '\f', '\v', '﻿',
  ';', ',', '(', ')', '-', '.', '/', '#', '\\', '=', '>', '&', '@', '!', '{', '}', '[', ']', '*',
];

const pick = (rand, list) => list[Math.floor(rand() * list.length)];

// mutate applies one random edit to the text.
export const mutate = (rand, text) => {
  const lines = text.split('\n');
  const op = Math.floor(rand() * 12);
  switch (op) {
    case 0: {
      // Delete a short range of characters.
      const at = Math.floor(rand() * (text.length + 1));
      const n = 1 + Math.floor(rand() * 5);
      return text.slice(0, at) + text.slice(at + n);
    }
    case 1:
    case 2: {
      // Insert a word at a random place.
      const at = Math.floor(rand() * (text.length + 1));
      return text.slice(0, at) + pick(rand, VOCABULARY) + text.slice(at);
    }
    case 3:
    case 4: {
      // Replace one token by a word.
      const parts = text.split(/(\s+)/);
      const tokens = [];
      parts.forEach((p, i) => {
        if (i % 2 === 0 && p !== '') tokens.push(i);
      });
      if (tokens.length === 0) return text + pick(rand, VOCABULARY);
      parts[pick(rand, tokens)] = pick(rand, VOCABULARY);
      return parts.join('');
    }
    case 5: {
      // Insert a word as a token of its own, with spaces around it.
      const parts = text.split(/(\s+)/);
      const at = Math.floor(rand() * (parts.length + 1));
      parts.splice(at, 0, ' ' + pick(rand, VOCABULARY) + ' ');
      return parts.join('');
    }
    case 6: {
      // Duplicate a line.
      const i = Math.floor(rand() * lines.length);
      lines.splice(i, 0, lines[i]);
      return lines.join('\n');
    }
    case 7: {
      // Swap two lines.
      const i = Math.floor(rand() * lines.length);
      const j = Math.floor(rand() * lines.length);
      [lines[i], lines[j]] = [lines[j], lines[i]];
      return lines.join('\n');
    }
    case 8: {
      // Delete a line.
      lines.splice(Math.floor(rand() * lines.length), 1);
      return lines.join('\n');
    }
    case 9: {
      // Join two lines, or change the line breaks.
      const i = Math.floor(rand() * lines.length);
      const sep = pick(rand, [' ', '', '\r\n', '\n\n', ' %% c\n', '\t']);
      return lines.slice(0, i + 1).join('\n') + sep + lines.slice(i + 1).join('\n');
    }
    case 10: {
      // Add a new statement line.
      const stmt = pick(rand, [
        'commit', 'commit id: "dup"', 'commit id: "dup" tag: "v1" type: HIGHLIGHT', 'commit "msg"',
        'commit msg: "m" tag: "a" tag: "b"', 'branch dup', 'branch dup order: 2', 'branch "quoted name"',
        'checkout main', 'checkout dup', 'switch develop', 'checkout nothere', 'merge main', 'merge dup',
        'merge develop', 'merge develop id: "dup"', 'merge develop tag: "m" type: REVERSE', 'merge nothere',
        'cherry-pick id: "dup"', 'cherry-pick id: "nothere"', 'cherry-pick id: "dup" tag: ""',
        'cherry-pick id: "dup" parent: "dup"', 'cherry-pick id: "dup" tag: "t1" tag: ""', 'cherry-pick',
        'cherry-pick parent: "dup"', 'title new title', 'accTitle: acc', 'accDescr: descr',
      ]);
      const i = Math.floor(rand() * (lines.length + 1));
      lines.splice(i, 0, '  ' + stmt);
      return lines.join('\n');
    }
    default: {
      // Cut the text.
      return text.slice(0, Math.floor(rand() * (text.length + 1)));
    }
  }
};

// mutations returns n mutated variants of a text. Each has one to three
// edits.
export const mutations = (rand, text, n) => {
  const out = [];
  for (let i = 0; i < n; i++) {
    let t = text;
    const edits = 1 + Math.floor(rand() * 3);
    for (let k = 0; k < edits; k++) t = mutate(rand, t);
    out.push(t);
  }
  return out;
};
