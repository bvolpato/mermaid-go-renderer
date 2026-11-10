#!/usr/bin/env node
// Generate the ZenUML fixtures from the saved Linux mmdc references.
// The SVG is the output of mmdc --no-font-embed. The metadata also records
// the output that mmdc would write with font embedding enabled.
//
// Usage: node tools/reference/gen-zenuml-golden.mjs
// Record the edge cases before running this generator:
// node tools/reference/mmdc-oracle.mjs render --out .parity/zenuml-edge-macos internal/diagrams/zenuml/testdata/emoji.mmd internal/diagrams/zenuml/testdata/entities.mmd
// node tools/reference/oracle-linux.mjs --fonts liberation --script mmdc-oracle.mjs render --out .parity/zenuml-edge-linux internal/diagrams/zenuml/testdata/emoji.mmd internal/diagrams/zenuml/testdata/entities.mmd
import { createHash } from 'node:crypto';
import { mkdir, readFile, writeFile } from 'node:fs/promises';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { gzipSync } from 'node:zlib';

const repoRoot = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '../..');
const referenceRoot = '.parity/ref-mmdc/linux';
const outputPath = path.join(repoRoot, 'internal/diagrams/zenuml/testdata/golden.json.gz');
const docsCases = Array.from({ length: 16 }, (_, index) => {
  const number = String(index + 1).padStart(2, '0');
  return `mermaid-docs/syntax/syntax-zenuml-${number}`;
});
const caseIds = [
  ...docsCases,
  'mermaid-e2e/zenuml/pr-check/basic-zen-uml-diagram',
];

function sha256(text) {
  return createHash('sha256').update(text, 'utf8').digest('hex');
}

function fail(file, message) {
  throw new Error(`${file}: ${message}`);
}

async function loadCase(caseId) {
  const referencePath = path.join(referenceRoot, `${caseId}.svg`);
  const metadataPath = referencePath.replace(/\.svg$/, '.json');
  const metadata = JSON.parse(await readFile(path.join(repoRoot, metadataPath), 'utf8'));
  const svg = await readFile(path.join(repoRoot, referencePath), 'utf8');

  if (metadata.source !== `testdata/corpus/${caseId}.mmd`) {
    fail(metadataPath, `source path does not match ${caseId}`);
  }
  if (metadata.diagramType !== 'zenuml' || metadata.page !== 'mermaid-cli') {
    fail(metadataPath, 'expected a successful ZenUML mmdc reference');
  }
  if (metadata.mermaid !== '12.2.0' || metadata.mmdc !== '12.0.0') {
    fail(metadataPath, 'unexpected Mermaid or mmdc version');
  }
  if (metadata.sha256 !== sha256(svg)) {
    fail(referencePath, 'SVG hash does not match reference metadata');
  }

  const svgBytes = Buffer.byteLength(svg, 'utf8');
  if (metadata.embedSha256 !== metadata.sha256 || metadata.embedBytes !== svgBytes) {
    fail(metadataPath, 'font-embedded output does not match the no-font-embed SVG');
  }
  if (!Array.isArray(metadata.embedStyles) || metadata.embedStyles.length !== 0) {
    fail(metadataPath, 'expected no injected font styles in the embedded output');
  }

  const root = svg.match(/<svg\b[^>]*>/s)?.[0];
  const viewBoxText = root?.match(/\bviewBox="([^"]+)"/)?.[1];
  const viewBox = viewBoxText?.trim().split(/[\s,]+/).map(Number);
  if (!viewBox || viewBox.length !== 4 || !viewBox.every(Number.isFinite)) {
    fail(referencePath, 'could not read four finite values from the SVG viewBox');
  }
  if (viewBox[2] <= 0 || viewBox[3] <= 0) {
    fail(referencePath, 'SVG viewBox has a non-positive width or height');
  }

  const sourcePath = metadata.source;
  const source = await readFile(path.join(repoRoot, sourcePath), 'utf8');
  return {
    sourcePath,
    source,
    referencePath,
    metadataPath,
    referenceMetadata: metadata,
    fontEmbedding: {
      noFontEmbed: { sha256: metadata.sha256, bytes: svgBytes },
      fontEmbed: {
        sha256: metadata.embedSha256,
        bytes: metadata.embedBytes,
        styles: metadata.embedStyles,
      },
      outputsIdentical: true,
    },
    sha256: metadata.sha256,
    dimensions: {
      width: viewBox[2],
      height: viewBox[3],
      viewBox,
    },
    svg,
  };
}

const cases = await Promise.all(caseIds.map(loadCase));
const golden = {
  schemaVersion: 1,
  provenance: {
    mermaid: '12.2.0',
    mmdc: '12.0.0',
    page: 'mermaid-cli',
    referenceSet: referenceRoot,
    svgFormat: 'mmdc --no-font-embed',
    generator: 'tools/reference/gen-zenuml-golden.mjs',
  },
  cases,
};

const json = `${JSON.stringify(golden, null, 2)}\n`;
const compressed = gzipSync(Buffer.from(json, 'utf8'), { level: 9, mtime: 0 });
await mkdir(path.dirname(outputPath), { recursive: true });
await writeFile(outputPath, compressed);
console.log(
  `Wrote ${path.relative(repoRoot, outputPath)}: ${cases.length} cases, ` +
    `${Buffer.byteLength(json)} JSON bytes, ${compressed.byteLength} gzip bytes`,
);

const edgeCases = await Promise.all(['emoji', 'entities'].flatMap(name => ['linux', 'macos'].map(async (environment) => {
  const sourcePath = `internal/diagrams/zenuml/testdata/${name}.mmd`;
  const source = await readFile(path.join(repoRoot, sourcePath), 'utf8');
  const base = `.parity/zenuml-edge-${environment}/${name}`;
  const metadata = JSON.parse(await readFile(path.join(repoRoot, `${base}.json`), 'utf8'));
  const svg = await readFile(path.join(repoRoot, `${base}.svg`), 'utf8');
  if (metadata.source !== sourcePath || metadata.diagramType !== 'zenuml' || metadata.sha256 !== sha256(svg)) {
    fail(base, 'invalid edge-case reference');
  }
  return { name, environment, source, svg, referenceMetadata: metadata };
})));
const edgeOutput = path.join(path.dirname(outputPath), 'edgecases.json.gz');
await writeFile(edgeOutput, gzipSync(Buffer.from(`${JSON.stringify(edgeCases, null, 2)}\n`), { level: 9, mtime: 0 }));
console.log('Wrote internal/diagrams/zenuml/testdata/edgecases.json.gz: Linux and macOS SVG references');
