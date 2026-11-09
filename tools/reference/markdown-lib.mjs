// Shared code for gen-marked.mjs and gen-markdown.mjs.
//
// It loads the chunk of the built mermaid 12.2.0 that holds the label
// markdown code. The chunk is from dist/chunks/mermaid.esm, the build that
// inlines its dependencies as dist/mermaid.min.js does. So the oracle runs
// marked 16.3.0 and ts-dedent 2.2.0, the versions that the browser bundle
// has. node_modules/marked is 16.4.2 and has different rules.
//
// The chunk does not export the internal functions. A module load hook adds
// one export statement in memory. No other line of mermaid changes. Run with
// tools/reference as the working directory.
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

export const HERE = path.dirname(fileURLToPath(import.meta.url));
export const REPO = path.resolve(HERE, '../..');
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.esm');

// findChunk returns the chunk that defines markdownToLines. The chunk file
// names are build hashes.
const findChunk = () => {
  for (const file of fs.readdirSync(CHUNKS)) {
    if (!file.endsWith('.mjs')) continue;
    const source = fs.readFileSync(path.join(CHUNKS, file), 'utf8');
    if (source.includes('\nfunction markdownToLines(')) return path.join(CHUNKS, file);
  }
  throw new Error('no mermaid chunk defines markdownToLines');
};

const CHUNK = findChunk();
const SOURCE = fs.readFileSync(CHUNK, 'utf8');

// The marked section of the chunk is minified, so the names of its bindings
// are read from the code that uses them.
const pick = (re, what) => {
  const m = re.exec(SOURCE);
  if (!m) throw new Error(`cannot find ${what} in ${CHUNK}`);
  return m[1];
};
const markedVersion = pick(/\/marked@([\d.]+)\/node_modules\/marked\/lib\/marked\.esm\.js\n/, 'the marked version');
const dedentVersion = pick(/\/ts-dedent@([\d.]+)\/node_modules\/ts-dedent\//, 'the ts-dedent version');
const markedName = pick(/const nodes = (\w+)\.lexer\(preprocessedMarkdown\);/, 'the marked instance');
const otherName = pick(/\nvar (\w+) = \{ codeRemoveIndent: /, 'the marked "other" rules');
const lexerName = pick(/\nvar \w+ = (\w+)\.lex;\n/, 'the marked Lexer class');

const INTERNALS = {
  marked: markedName,
  markedOther: otherName,
  markedLexer: lexerName,
  dedent: 'dedent',
  markdownToLines: 'markdownToLines',
  markdownToHTML: 'markdownToHTML',
  nonMarkdownToLines: 'nonMarkdownToLines',
  nonMarkdownToHTML: 'nonMarkdownToHTML',
  splitTextToChars: 'splitTextToChars',
  splitWordToFitWidth: 'splitWordToFitWidth',
  splitLineToFitWidth: 'splitLineToFitWidth',
};

registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url === pathToFileURL(CHUNK).href) {
      const names = Object.entries(INTERNALS)
        .map(([as, name]) => `${name} as __${as}`)
        .join(', ');
      return { ...result, source: `${result.source.toString()}\nexport { ${names} };\n` };
    }
    return result;
  },
});

const mod = await import(pathToFileURL(CHUNK).href);

export const versions = { marked: markedVersion, tsDedent: dedentVersion };
export const marked = mod.__marked;
export const markedOther = mod.__markedOther;
export const markedLexer = mod.__markedLexer;
export const dedent = mod.__dedent;
export const markdownToLines = mod.__markdownToLines;
export const markdownToHTML = mod.__markdownToHTML;
export const nonMarkdownToLines = mod.__nonMarkdownToLines;
export const nonMarkdownToHTML = mod.__nonMarkdownToHTML;
export const splitTextToChars = mod.__splitTextToChars;
export const splitWordToFitWidth = mod.__splitWordToFitWidth;
export const splitLineToFitWidth = mod.__splitLineToFitWidth;
