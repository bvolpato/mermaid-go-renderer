#!/usr/bin/env node
// Writes the golden set of the PNG rasterizer for Chromium on Linux:
// internal/raster/testdata/linux (TestGoldenLinux of internal/raster).
//
// The set has the probes of gen-raster-linux-probes.mjs, and corpus diagrams
// whose text has the cases that the rasterizer ports: an opacity group, a
// clip path, a blend mode, a tspan with a dominant baseline, rotated text,
// large text, a decoration, HTML labels. A corpus file is the Linux
// reference SVG of the fixture (.parity/ref-linux/mermaid-e2e).
//
// Usage, from the repository root:
//
//   node tools/reference/gen-raster-linux-golden.mjs          the SVG files
//   node tools/reference/gen-raster-linux-golden.mjs --shoot  and the PNGs
//
// With --shoot the script runs oracle-linux.mjs with only the Liberation
// fonts, at scale 1 for every file (NAME.png) and at scale 2 for the files
// of SCALE2 (NAME@2x.png).
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));
const ROOT = path.resolve(HERE, '../..');
const REF = path.join(ROOT, '.parity/ref-linux/mermaid-e2e');
const OUT = path.join(ROOT, 'internal/raster/testdata/linux');

// The corpus diagrams, with the case that each one covers.
const CORPUS = {
  // An axis in groups with an opacity: LCD text in a layer.
  'corpus-gantt': 'gantt/nightly-check/should-render-a-gantt-diagram-excluding-friday-and-saturday',
  // Labels under a clip path.
  'corpus-treemap': 'treemap/nightly-check/4-should-handle-long-text-that-wraps',
  // Lines in a tspan with dominant-baseline: middle, and a filter.
  'corpus-timeline': 'timeline/pr-check/14-should-render-a-vertical-timeline-diagram-with-sections',
  // Labels in a tspan with dominant-baseline: central.
  'corpus-sequence': 'sequence/nightly-check/should-prioritize-external-alias-over-inline-alias',
  // Labels under a rotation by 45 degrees.
  'corpus-git': 'git/pr-check/3-should-render-a-simple-gitgraph-with-different-committypes-on-main-branch',
  // An underline that an SVG group declares for an HTML label.
  'corpus-class': 'class-diagram/nightly-check/6-should-render-a-simple-class-diagram-with-static-method-3',
  // Labels under a quarter turn, and a fill that is not a color.
  'corpus-quadrant': 'quadrant-chart/pr-check/should-use-all-the-config',
  // Text above 48 px in an inner svg element.
  'corpus-architecture': 'architecture/nightly-check/should-render-an-architecture-diagram-with-the-fallback-icon',
  // HTML labels.
  'corpus-block': 'block/nightly-check/block-types-1-square-rounded-and-circle',
  'corpus-kanban': 'kanban/pr-check/1-should-render-a-kanban-with-a-single-section',
  // An axis title under a quarter turn.
  'corpus-xychart': 'xychart/nightly-check/should-render-a-single-bar-with-label-for-a-vertical-xy-chart',
  // Links with a blend mode: the page is an isolation group.
  'corpus-sankey': 'sankey/nightly-check/links-same-color-as-target-node',
  // HTML labels with a background at a fraction of a pixel.
  'corpus-er': 'er-diagram/visual-check/er-subgraphs-default',
};

// The files that also have a screenshot at scale 2: at a device scale
// factor above 1 Chromium on Linux turns the hinting off and has other font
// metrics.
const SCALE2 = ['baseline-svg-16', 'baseline-html', 'lcd-opacity', 'corpus-block', 'corpus-sequence', 'corpus-kanban'];

function run(args) {
  const r = spawnSync(process.execPath, args, { stdio: 'inherit' });
  if (r.status !== 0) {
    process.exit(r.status || 1);
  }
}

fs.mkdirSync(OUT, { recursive: true });
run([path.join(HERE, 'gen-raster-linux-probes.mjs'), OUT]);
for (const [name, rel] of Object.entries(CORPUS)) {
  fs.copyFileSync(path.join(REF, rel + '.svg'), path.join(OUT, name + '.svg'));
}
const svgs = fs
  .readdirSync(OUT)
  .filter((f) => f.endsWith('.svg'))
  .sort();
process.stderr.write(`${svgs.length} SVG files in ${OUT}\n`);

if (process.argv.includes('--shoot')) {
  const oracle = path.join(HERE, 'oracle-linux.mjs');
  // --quiet goes last: oracle.mjs reads a path after it as its value.
  run([oracle, '--fonts', 'liberation', 'shoot', ...svgs.map((f) => path.join(OUT, f)), '--quiet']);
  const tmp = path.join(OUT, '.s2');
  fs.rmSync(tmp, { recursive: true, force: true });
  run([oracle, '--fonts', 'liberation', 'shoot', '--scale', '2', '--out', tmp, ...SCALE2.map((n) => path.join(OUT, n + '.svg')), '--quiet']);
  for (const name of SCALE2) {
    fs.renameSync(path.join(tmp, name + '.png'), path.join(OUT, name + '@2x.png'));
  }
  fs.rmSync(tmp, { recursive: true, force: true });
}
