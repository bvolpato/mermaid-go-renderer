#!/usr/bin/env node
// Writes the golden file of internal/render/async_test.go.
//
// A case is a small program of async operations. This script runs it with
// real promises and records the order of the log entries. The Go test runs
// the same program with ctx.All, ctx.Await and ctx.AwaitMeasure, and the
// order must be equal. The order of microtasks is defined by the language,
// so node gives the same order as Chromium.
//
// Operations:
//   {t:'log', v}        a log entry
//   {t:'await'}         await of a value
//   {t:'measure'}       await fastdom.measure(...), with fastdom set up as
//                       mermaid does (flush from queueMicrotask)
//   {t:'call', ops}     await of an async function with this body
//   {t:'sync', ops}     await of the result of a function that is not async
//   {t:'all', tasks}    await Promise.all(tasks.map(async ...))
//   {t:'timeout'}       await of a promise that a setTimeout resolves
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const HERE = path.dirname(fileURLToPath(import.meta.url));

// fastdom as in fastdom.js with the fastdom-promised extension and the raf
// override of mermaid: reads run in one flush, in order.
function makeFastdom() {
  const reads = [];
  let scheduled = false;
  function flush() {
    scheduled = false;
    let task;
    while ((task = reads.shift())) {
      task();
    }
  }
  return {
    measure(fn) {
      return new Promise((resolve) => {
        reads.push(() => resolve(fn()));
        if (!scheduled) {
          scheduled = true;
          queueMicrotask(flush);
        }
      });
    },
  };
}

async function runCase(ops) {
  const log = [];
  const fastdom = makeFastdom();
  function runSync(ops, name) {
    for (const op of ops) {
      log.push(`${name}:${op.v}`);
    }
  }
  async function run(ops, name) {
    for (const op of ops) {
      switch (op.t) {
        case 'log':
          log.push(`${name}:${op.v}`);
          break;
        case 'await':
          await null;
          break;
        case 'measure':
          await fastdom.measure(() => 0);
          break;
        case 'call':
          await run(op.ops, name);
          break;
        case 'sync':
          await runSync(op.ops, name);
          break;
        case 'all':
          await Promise.all(op.tasks.map((task, i) => run(task, `${name}.${i}`)));
          break;
        case 'timeout':
          await new Promise((resolve) => setTimeout(resolve));
          break;
      }
    }
  }
  await run(ops, 'r');
  return log;
}

// mulberry32
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

let counter = 0;
function randomOps(rand, depth, allowAll) {
  const n = 1 + Math.floor(rand() * 5);
  const ops = [];
  for (let i = 0; i < n; i++) {
    const r = rand();
    if (r < 0.3) {
      ops.push({ t: 'log', v: String(counter++) });
    } else if (r < 0.45) {
      ops.push({ t: 'await' });
    } else if (r < 0.6) {
      ops.push({ t: 'measure' });
    } else if (r < 0.75 && depth < 4) {
      ops.push({ t: 'call', ops: randomOps(rand, depth + 1, allowAll) });
    } else if (r < 0.82) {
      ops.push({ t: 'sync', ops: [{ t: 'log', v: String(counter++) }] });
    } else if (r < 0.92 && depth < 3 && allowAll) {
      const tasks = [];
      const m = Math.floor(rand() * 5);
      for (let k = 0; k < m; k++) {
        tasks.push(randomOps(rand, depth + 1, allowAll));
      }
      ops.push({ t: 'all', tasks });
    } else if (r < 0.95) {
      ops.push({ t: 'timeout' });
    } else {
      ops.push({ t: 'log', v: String(counter++) });
    }
  }
  ops.push({ t: 'log', v: String(counter++) });
  return ops;
}

// The shape of the real code: node callbacks of the layouts.
const L = (v) => ({ t: 'log', v });
const htmlLabel = [
  L('g'),
  { t: 'call', ops: [{ t: 'await' }, { t: 'call', ops: [{ t: 'measure' }, L('span')] }] }, // createText
  { t: 'call', ops: [] }, // configureLabelImages
  { t: 'measure' },
  L('bbox'),
];
const svgLabel = [L('g'), { t: 'call', ops: [L('text')] }, { t: 'measure' }, L('bbox')];
const asyncShape = (label) => [{ t: 'call', ops: [{ t: 'call', ops: label }, L('draw')] }, L('inserted')];
const syncShape = [{ t: 'sync', ops: [L('draw')] }, L('inserted')];
const insertNode = (shape) => [{ t: 'call', ops: shape }, L('done')];

const cases = [
  { name: 'empty-all', ops: [L('a'), { t: 'all', tasks: [] }, L('b')] },
  {
    name: 'html-nodes',
    ops: [{ t: 'all', tasks: [insertNode(asyncShape(htmlLabel)), insertNode(asyncShape(htmlLabel)), insertNode(asyncShape(htmlLabel))] }, L('end')],
  },
  {
    name: 'sync-shape-in-the-middle',
    ops: [{ t: 'all', tasks: [insertNode(asyncShape(htmlLabel)), insertNode(syncShape), insertNode(asyncShape(htmlLabel))] }, L('end')],
  },
  {
    name: 'svg-and-html-labels',
    ops: [{ t: 'all', tasks: [insertNode(asyncShape(svgLabel)), insertNode(asyncShape(htmlLabel)), insertNode(syncShape), insertNode(asyncShape(svgLabel))] }, L('end')],
  },
  {
    name: 'nested-cluster',
    ops: [
      {
        t: 'all',
        tasks: [
          insertNode(asyncShape(htmlLabel)),
          [{ t: 'call', ops: [{ t: 'all', tasks: [insertNode(asyncShape(htmlLabel)), insertNode(syncShape)] }, L('sub-end')] }, L('cluster')],
          insertNode(asyncShape(htmlLabel)),
        ],
      },
      L('end'),
    ],
  },
];
const rand = prng(0x6d6d6467);
for (let i = 0; i < 400; i++) {
  counter = 0;
  cases.push({ name: `random-${i}`, ops: [{ t: 'all', tasks: [randomOps(rand, 0, true), randomOps(rand, 0, true), randomOps(rand, 0, i % 2 === 0)] }, L('end')] });
}

for (const c of cases) {
  c.log = await runCase(c.ops);
}
const out = path.join(HERE, '../../internal/render/testdata/async_order.json.gz');
fs.writeFileSync(out, zlib.gzipSync(JSON.stringify(cases)));
console.log(`${cases.length} cases -> ${out}`);
