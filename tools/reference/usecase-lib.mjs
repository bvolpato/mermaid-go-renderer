// Gives the usecase generators access to the real lexer, parser, visitor and
// db of the usecase diagram of mermaid 12.2.0, in plain node.
//
// The built chunk (node_modules/mermaid/dist/chunks/mermaid.core/
// usecaseDiagram-*.mjs) exports only the diagram definition. A module load
// hook appends one export statement to that file, so that the lexer, the
// parser, the visitor and the db are reachable. No other line of mermaid
// changes. The chunk imports chevrotain from the package, so this module and
// the chunk share one chevrotain instance.
import { registerHooks } from 'node:module';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath, pathToFileURL } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const CHUNKS = path.join(HERE, 'node_modules/mermaid/dist/chunks/mermaid.core');

const chunkFile = fs.readdirSync(CHUNKS).find((f) => /^usecaseDiagram-.*\.mjs$/.test(f));
if (!chunkFile) {
  throw new Error('no usecaseDiagram chunk in ' + CHUNKS);
}

registerHooks({
  load(url, context, nextLoad) {
    const result = nextLoad(url, context);
    if (url.endsWith('/' + chunkFile)) {
      const source =
        result.source.toString() +
        '\nexport { usecaseLexer as __lexer, usecaseParser as __parser, usecaseVisitor as __visitor, db as __db, parser as __parserDefinition, usecaseTokens as __tokens, parseOrderedJsonObject as __parseOrderedJsonObject };\n';
      return { ...result, source };
    }
    return result;
  },
});

let cached;

// usecase loads the chunk once and returns its internals.
export const usecase = async () => {
  if (cached) {
    return cached;
  }
  const chunk = await import(pathToFileURL(path.join(CHUNKS, chunkFile)).href);
  cached = {
    lexer: chunk.__lexer,
    parser: chunk.__parser,
    visitor: chunk.__visitor,
    db: chunk.__db,
    parserDefinition: chunk.__parserDefinition,
    tokens: chunk.__tokens,
    parseOrderedJsonObject: chunk.__parseOrderedJsonObject,
    diagram: chunk.diagram,
  };
  return cached;
};

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
