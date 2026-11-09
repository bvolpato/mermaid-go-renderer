// Emits internal/elk/native/gwt/testdata/p2layers_priorityqueue.json: runs
// of java.util.PriorityQueue of the GWT runtime in elk-worker.js (elkjs
// 0.9.3), for the test of the Go port.
//
// Usage, with tools/reference as the working directory:
//
//   node gen-elk-gwt-priorityqueue.mjs
//
// An element is [key, id]. The comparator compares the keys only, so the
// order of elements with one key shows the heap algorithm. A run has a list
// of operations:
//
//   ["o", key, id]            offer
//   ["a", [[key, id], ...]]   addAll
//   ["p"]                     poll
//   ["c"]                     clear
//
// and for each operation the result: the id that poll returns (-1 for an
// empty queue, null for another operation) and the ids of the heap list
// after the operation.
import { mkdirSync, writeFileSync } from 'node:fs';
import { dirname, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';
import { loadElk } from './elk-phases-lib.mjs';

const here = dirname(fileURLToPath(import.meta.url));
const outFile = resolve(here, '../../internal/elk/native/gwt/testdata/p2layers_priorityqueue.json');

const elk = loadElk();
const S = elk.hook.scope;
const PriorityQueue = S('PriorityQueue');
const ArrayList = S('ArrayList');
const offer = S('$offer');
const poll = S('$poll_0');
const addAll = S('$addAll_4');
const listAdd = S('$add_3');

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

const runs = [];
for (let seed = 1; seed <= 120; seed++) {
  const r = prng(seed);
  const int = (lo, hi) => lo + Math.floor(r() * (hi - lo + 1));
  // Few keys give many elements that the comparator calls equal.
  const keys = [1, 2, 3, 5, 8, 20, 1000][int(0, 6)];
  const steps = int(1, 90);
  const pollShare = [0.2, 0.4, 0.6][int(0, 2)];
  const cmp = { compare_1: (x, y) => (x.key < y.key ? -1 : x.key > y.key ? 1 : 0) };
  const q = new PriorityQueue(cmp);
  let nextId = 0;
  const elem = () => ({ key: int(0, keys - 1), id: nextId++ });
  const ops = [];
  const results = [];
  for (let i = 0; i < steps; i++) {
    const x = r();
    let op;
    let polled = null;
    if (x < pollShare) {
      op = ['p'];
      const v = poll(q);
      polled = v === null ? -1 : v.id;
    } else if (x < pollShare + 0.06) {
      const list = new ArrayList();
      const items = [];
      const k = int(0, 9);
      for (let j = 0; j < k; j++) {
        const e = elem();
        listAdd(list, e);
        items.push([e.key, e.id]);
      }
      op = ['a', items];
      addAll(q, list);
    } else if (x < pollShare + 0.07) {
      op = ['c'];
      q.clear_0();
    } else {
      const e = elem();
      op = ['o', e.key, e.id];
      offer(q, e);
    }
    ops.push(op);
    results.push([polled, q.heap.array.map((e) => e.id)]);
  }
  // Empty the queue: the order of all elements that are left.
  while (q.heap.array.length > 0) {
    ops.push(['p']);
    const v = poll(q);
    results.push([v.id, q.heap.array.map((e) => e.id)]);
  }
  runs.push({ seed, ops, results });
}

mkdirSync(dirname(outFile), { recursive: true });
writeFileSync(outFile, JSON.stringify(runs) + '\n');
console.log(`${runs.length} runs, ${runs.reduce((a, x) => a + x.ops.length, 0)} operations`);
