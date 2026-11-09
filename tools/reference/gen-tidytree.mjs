// Records what the layout of @mermaid-js/layout-tidy-tree 1.0.1 does, as
// golden data for internal/ru/layout/tidytree.
//
// Run it in tools/reference:
//
//   node gen-tidytree.mjs [--random N]
//   node gen-tidytree.mjs --mmdc
//
// With --mmdc it records what mermaid-cli writes for the diagrams of
// internal/ru/layout/tidytree/testdata/mmdc. See the end of this comment.
//
// Without --mmdc it calls the real executeTidyTreeLayout of the package in
// node. No DOM is used: the sizes of the nodes come from the generator. It
// writes internal/ru/layout/tidytree/testdata/layout.json.gz:
//
//   { "package": "...", "version": "...", "cases": [ case, ... ] }
//
// A case is
//
//   name    the kind of the case and its number
//   nodes   data.nodes: [{ id, width?, height?, shape? }]. A missing
//           property is undefined. nodes is null when data.nodes is missing.
//   edges   data.edges: [{ id?, start?, end? }], or null when data.edges
//           is missing
//   random  the number of calls of Math.random
//   error   { name, message } when the promise rejects
//   result  otherwise: the positioned nodes and edges
//             nodes: [id, x, y, section, width, height, orig]
//             edges: [id, source, target, startX, startY, midX, midY,
//                     endX, endY, [x0, y0, x1, y1, ...], sourceSection,
//                     targetSection, sourceWidth, sourceHeight,
//                     targetWidth, targetHeight]
//           orig is the index in nodes of the object that originalNode is.
//
// executeTidyTreeLayout reaches only a part of the branches of three of
// its functions: an edge always leaves a node at the height of its center.
// So the file also has direct calls of them, in "internals":
//
//   intersection   [x, y, width, height, outsideX, outsideY, insideX,
//                   insideY, resultX, resultY]: intersection(node,
//                   outsidePoint, insidePoint)
//   circle         [x, y, width, height, startX, startY, endX, endY,
//                   resultX, resultY]: computeCircleEdgeIntersection(
//                   circle, lineStart, lineEnd)
//   edgePositions  { nodes: [[id, x, y, section, width, height, shape]],
//                    edges: [[id, start, end]], shift, result }:
//                   calculateEdgePositions(edges, positionedNodes, shift).
//                   result has the edges in the form above.
//
// The package does not export these functions. The generator loads the
// same chunk of the package a second time, with an export statement added.
//
// A number is a JSON number, which round-trips exactly, or one of the
// strings "NaN", "Infinity", "-Infinity" and "-0". null is undefined.
//
// --mmdc renders every .mmd file of testdata/mmdc with the code of
// mermaid-cli (mmdc-oracle.mjs) in the Linux image with only the Liberation
// fonts (oracle-linux.mjs --fonts liberation), which is the reference of the
// embedded font library. It needs docker, and it takes the oracle lock for
// one short run. It writes testdata/mmdc.json.gz:
//
//   { "mermaid": "...", "mmdc": "...", "cases": [ { name, text,
//     diagramType, svg | error }, ... ] }
//
// svg is the .svg file of mmdc --no-font-embed, and error is the message
// of the error when mermaid fails. The diagrams are of every diagram type
// that can ask for the layout, with the looks, with graphs that are not
// trees and with graphs for which the layout throws.
import { spawnSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';
import { executeTidyTreeLayout } from '@mermaid-js/layout-tidy-tree';

const here = path.dirname(fileURLToPath(import.meta.url));
const root = path.resolve(here, '../..');
const outFile = path.join(root, 'internal/ru/layout/tidytree/testdata/layout.json.gz');
const pkg = JSON.parse(
  fs.readFileSync(path.join(here, 'node_modules/@mermaid-js/layout-tidy-tree/package.json'), 'utf8')
);

// The chunk that the entry of the package imports has the whole layout and
// no import of its own.
const pkgDir = path.join(here, 'node_modules/@mermaid-js/layout-tidy-tree');
const entry = fs.readFileSync(path.join(pkgDir, pkg.module), 'utf8');
const chunkName = /from "(\.\/chunks\/[^"]+)"/.exec(entry)[1];
const chunkSource = fs.readFileSync(path.join(pkgDir, path.dirname(pkg.module), chunkName), 'utf8');
if (/^\s*import\s/m.test(chunkSource)) {
  throw new Error('the chunk of the layout has imports');
}
const internals = await import(
  'data:text/javascript;base64,' +
    Buffer.from(
      chunkSource + '\nexport { intersection as __intersection, computeCircleEdgeIntersection as __circle, calculateEdgePositions as __edgePositions };\n'
    ).toString('base64')
);

const opts = { random: 900, mmdc: false };
const argv = process.argv.slice(2);
for (let i = 0; i < argv.length; i++) {
  if (argv[i] === '--random') {
    opts.random = Number(argv[++i]);
  } else if (argv[i] === '--mmdc') {
    opts.mmdc = true;
  } else {
    console.error('usage: node gen-tidytree.mjs [--random N] | --mmdc');
    process.exit(2);
  }
}

if (opts.mmdc) {
  recordMmdc();
  process.exit(0);
}

// recordMmdc renders the diagrams of testdata/mmdc with mermaid-cli and
// packs the results.
function recordMmdc() {
  const corpus = 'internal/ru/layout/tidytree/testdata/mmdc';
  const work = '.parity/tidytree-mmdc';
  const names = fs
    .readdirSync(path.join(root, corpus))
    .filter((name) => name.endsWith('.mmd'))
    .sort()
    .map((name) => name.replace(/\.mmd$/, ''));
  // The old results go first, so that every result is from this run.
  fs.mkdirSync(path.join(root, work, 'ref'), { recursive: true });
  for (const name of names) {
    fs.rmSync(path.join(root, work, 'ref', `${name}.json`), { force: true });
    fs.rmSync(path.join(root, work, 'ref', `${name}.svg`), { force: true });
  }
  const list = path.join(work, 'list.txt');
  fs.writeFileSync(path.join(root, list), names.map((name) => `${corpus}/${name}.mmd`).join('\n') + '\n');
  // The working directory is the repository root: the image mounts it at
  // the same path.
  const run = spawnSync(
    'node',
    [
      path.join(here, 'oracle-linux.mjs'), '--fonts', 'liberation', '--script', 'mmdc-oracle.mjs',
      'render', '--quiet', '--base', corpus, '--out', path.join(work, 'ref'), '--list', list,
    ],
    { cwd: root, stdio: ['ignore', 'inherit', 'inherit'] }
  );
  if (run.status !== 0) {
    throw new Error(`the oracle stopped with status ${run.status}`);
  }
  const cases = [];
  let mermaid, mmdc;
  for (const name of names) {
    const meta = JSON.parse(fs.readFileSync(path.join(root, work, 'ref', `${name}.json`), 'utf8'));
    mermaid = meta.mermaid;
    mmdc = meta.mmdc;
    const c = { name, text: fs.readFileSync(path.join(root, corpus, `${name}.mmd`), 'utf8') };
    if (meta.error) {
      c.error = String(meta.error);
    } else {
      c.diagramType = meta.diagramType;
      c.svg = fs.readFileSync(path.join(root, work, 'ref', `${name}.svg`), 'utf8');
    }
    cases.push(c);
  }
  const file = path.join(root, 'internal/ru/layout/tidytree/testdata/mmdc.json.gz');
  const json = JSON.stringify({ mermaid, mmdc, cases });
  const gz = zlib.gzipSync(Buffer.from(json), { level: 9 });
  fs.writeFileSync(file, gz);
  const failed = cases.filter((c) => c.error).length;
  console.log(
    `${path.relative(root, file)}: ${cases.length} diagrams, ${failed} with an error, ${(gz.length / 1024).toFixed(0)} KiB gzip, ${(json.length / 1024).toFixed(0)} KiB json`
  );
}

// mulberry32, the same generator as gen-jsnum.mjs.
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

const rnd = prng(0x71d7);
const pick = (list) => list[Math.floor(rnd() * list.length)];
const int = (lo, hi) => lo + Math.floor(rnd() * (hi - lo + 1));

// The layout calls Math.random for the id of each virtual root. The
// generator counts the calls.
let randomCalls = 0;
const pageRandom = prng(0x6d6d6467);
Math.random = () => {
  randomCalls++;
  return pageRandom();
};

// ---- node sizes ----

// The round shapes have another rule for the end of an edge.
const SHAPES = [
  undefined, 'defaultMindmapNode', 'rounded', 'squareRect', 'roundedRect', 'hexagon', 'mindmapCircle',
  'circle', 'cloud', 'bang', 'circle', 'cloud', 'bang',
];

// size returns a width or a height. style says which numbers:
//   text   a size as the browser measures it: a multiple of 1/64 px, as
//          text has, plus a padding
//   int    an integer
//   any    any double
//   same   one size for every node
//   odd    also zero, a negative number, a tiny and a huge number, and
//          undefined
function size(style, wide) {
  const k = rnd();
  switch (style) {
    case 'int':
      return int(1, wide ? 260 : 90);
    case 'any':
      return rnd() * (wide ? 300 : 120);
    case 'same':
      return wide ? 80 : 34;
    case 'odd':
      if (k < 0.12) return 0;
      if (k < 0.22) return undefined;
      if (k < 0.3) return -int(1, 100);
      if (k < 0.34) return -60;
      if (k < 0.38) return -40;
      if (k < 0.42) return -rnd() * 400;
      if (k < 0.47) return rnd() * 1e-9;
      if (k < 0.52) return rnd() * 1e9;
      if (k < 0.56) return -0;
      return Math.round(rnd() * 200 * 64) / 64;
    default: {
      const text = Math.round(rnd() * (wide ? 220 : 72) * 64) / 64;
      const padding = pick([0, 10, 15, 20, 30, 16.5]);
      return text + (k < 0.8 ? padding : 2 * padding * Math.SQRT2);
    }
  }
}

function makeNode(id, style) {
  const node = { id };
  const width = size(style, true);
  const height = size(style, false);
  if (width !== undefined) node.width = width;
  if (height !== undefined) node.height = height;
  const shape = pick(SHAPES);
  if (shape !== undefined) node.shape = shape;
  if (style === 'text' && rnd() < 0.05) {
    // A circle has one size for both sides.
    node.height = node.width;
  }
  return node;
}

// ---- trees ----

// parents returns the parent of each node of a random tree with n nodes.
// Node 0 is the root.
function parents(n, shape) {
  const parent = [-1];
  for (let i = 1; i < n; i++) {
    let p;
    switch (shape) {
      case 'flat':
        p = 0;
        break;
      case 'chain':
        p = i - 1;
        break;
      case 'deep':
        p = rnd() < 0.8 ? i - 1 : int(0, i - 1);
        break;
      case 'wide':
        p = int(0, Math.min(i - 1, 3));
        break;
      case 'binary':
        p = Math.floor((i - 1) / 2);
        break;
      case 'two':
        // Two branches under the root: one on each side.
        p = i <= 2 ? 0 : rnd() < 0.5 ? int(1, i - 1) : i - 1;
        break;
      default:
        p = int(Math.max(0, i - int(1, 12)), i - 1);
    }
    parent.push(p);
  }
  return parent;
}

function shuffle(list) {
  for (let i = list.length - 1; i > 0; i--) {
    const j = int(0, i);
    [list[i], list[j]] = [list[j], list[i]];
  }
  return list;
}

const TREE_SHAPES = ['any', 'any', 'any', 'flat', 'chain', 'deep', 'wide', 'binary', 'two'];
const SIZE_STYLES = ['text', 'text', 'text', 'text', 'int', 'any', 'same', 'odd'];

// randomCase returns the nodes and the edges of one random case. kind says
// how the graph differs from a tree whose root is the first node.
function randomCase(kind) {
  const shape = pick(TREE_SHAPES);
  const style = pick(SIZE_STYLES);
  let n = pick([1, 2, 3, 4, 5, 7, 9, 12, 16, 24, 40, 70]);
  if (shape === 'chain') n = Math.min(n, 40);
  if (kind === 'dag') n = Math.min(n, 16);
  const ids = Array.from({ length: n }, (_, i) => (rnd() < 0.1 ? String(i) : `n${i}`));
  const parent = parents(n, shape);
  let nodes = ids.map((id) => makeNode(id, style));
  let edges = [];
  for (let i = 1; i < n; i++) {
    edges.push({ id: `edge_${ids[parent[i]]}_${ids[i]}`, start: ids[parent[i]], end: ids[i] });
  }

  switch (kind) {
    case 'tree':
      break;
    case 'shuffled':
      // The root is the first node that no edge ends at, and the order of
      // the children is the order of the edges.
      shuffle(nodes);
      shuffle(edges);
      break;
    case 'forest': {
      // More roots: the layout takes the first one.
      for (let k = int(1, 3); k > 0 && edges.length > 0; k--) {
        edges.splice(int(0, edges.length - 1), 1);
      }
      if (rnd() < 0.5) shuffle(nodes);
      break;
    }
    case 'dag': {
      // A node with more parents is a branch of each of them.
      for (let k = int(1, 4); k > 0 && n > 2; k--) {
        const to = int(2, n - 1);
        const from = int(0, to - 1);
        edges.splice(int(0, edges.length), 0, { id: `extra_${k}`, start: ids[from], end: ids[to] });
      }
      // The same edge two times.
      if (rnd() < 0.4 && edges.length > 0) {
        const e = pick(edges);
        edges.push({ id: `${e.id}_again`, start: e.start, end: e.end });
      }
      break;
    }
    case 'cyclic': {
      // An edge back to an ancestor, to the node itself, or to the root.
      for (let k = int(1, 2); k > 0 && n > 1; k--) {
        const from = int(0, n - 1);
        const how = rnd();
        let to = from;
        if (how < 0.5) {
          for (let up = int(0, 3); up > 0 && parent[to] >= 0; up--) to = parent[to];
        } else if (how < 0.8) {
          to = 0;
        } else {
          to = int(0, n - 1);
        }
        edges.splice(int(0, edges.length), 0, { id: `back_${k}`, start: ids[from], end: ids[to] });
      }
      if (rnd() < 0.3) shuffle(nodes);
      break;
    }
    case 'dangling': {
      // Edges with an unknown end, with a missing end and with no id.
      for (let k = int(1, 4); k > 0; k--) {
        const known = pick(ids);
        const how = int(0, 6);
        const at = int(0, edges.length);
        if (how === 0) edges.splice(at, 0, { id: `d${k}`, start: known, end: `missing${k}` });
        else if (how === 1) edges.splice(at, 0, { id: `d${k}`, start: `missing${k}`, end: known });
        else if (how === 2) edges.splice(at, 0, { id: `d${k}`, start: known });
        else if (how === 3) edges.splice(at, 0, { id: `d${k}`, end: known });
        else if (how === 4) edges.splice(at, 0, { id: `d${k}`, start: '', end: known });
        else if (how === 5) edges.splice(at, 0, { id: `d${k}` });
        else edges.splice(at, 0, { start: known, end: `missing${k}` });
      }
      if (rnd() < 0.3 && edges.length > 0) delete pick(edges).id;
      break;
    }
    case 'duplicate': {
      // Two nodes with one id: the later one is the node of the id.
      for (let k = int(1, 2); k > 0; k--) {
        const copy = makeNode(pick(ids), style);
        nodes.splice(int(0, nodes.length), 0, copy);
      }
      break;
    }
    case 'noedges':
      edges = rnd() < 0.5 ? null : [];
      break;
    default:
      throw new Error(`unknown kind ${kind}`);
  }
  return { nodes, edges };
}

// ---- fixed cases, for the branches ----

const N = (id, width, height, shape) => {
  const node = { id };
  if (width !== undefined) node.width = width;
  if (height !== undefined) node.height = height;
  if (shape !== undefined) node.shape = shape;
  return node;
};
const E = (start, end, id = `edge_${start}_${end}`) => ({ id, start, end });

const FIXED = {
  'no-nodes': { nodes: [], edges: [] },
  'nodes-missing': { nodes: null, edges: [] },
  'one-node': { nodes: [N('a', 80, 40)], edges: [] },
  'one-node-no-size': { nodes: [N('a')], edges: [] },
  'one-node-edges-missing': { nodes: [N('a', 80, 40)], edges: null },
  'one-child': { nodes: [N('a', 80, 40), N('b', 60, 30)], edges: [E('a', 'b')] },
  'two-children': {
    nodes: [N('a', 90.05201721191406, 90.05201721191406, 'circle'), N('b', 50.671875, 34), N('c', 50.671875, 34)],
    edges: [E('a', 'b'), E('a', 'c')],
  },
  'no-sizes': { nodes: [N('a'), N('b'), N('c'), N('d')], edges: [E('a', 'b'), E('a', 'c'), E('b', 'd')] },
  'zero-sizes': {
    nodes: [N('a', 0, 0), N('b', 0, 30), N('c', 40, 0), N('d', 0, 0)],
    edges: [E('a', 'b'), E('a', 'c'), E('b', 'd')],
  },
  'round-shapes': {
    nodes: [N('a', 100, 100, 'circle'), N('b', 120, 80, 'cloud'), N('c', 90, 70, 'bang'), N('d', 60, 60, 'circle'), N('e', 50, 30)],
    edges: [E('a', 'b'), E('a', 'c'), E('b', 'd'), E('c', 'e')],
  },
  'round-no-size': {
    nodes: [N('a', undefined, undefined, 'circle'), N('b', undefined, undefined, 'cloud'), N('c', 20, undefined, 'bang')],
    edges: [E('a', 'b'), E('a', 'c')],
  },
  // The second point of the edge is at the center of the round node: the
  // line has no length.
  'round-width-minus-60': {
    nodes: [N('a', -60, 50, 'circle'), N('b', -60, 40, 'bang'), N('c', -60, 40, 'cloud'), N('d', -60, 20, 'circle')],
    edges: [E('a', 'b'), E('a', 'c'), E('b', 'd')],
  },
  'root-not-first': {
    nodes: [N('c', 50, 30), N('b', 60, 30), N('a', 80, 40), N('d', 30, 30)],
    edges: [E('a', 'b'), E('a', 'c'), E('b', 'd')],
  },
  'two-roots': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('x', 80, 40), N('y', 60, 30)],
    edges: [E('a', 'b'), E('x', 'y')],
  },
  'cycle-two': { nodes: [N('a', 80, 40), N('b', 60, 30)], edges: [E('a', 'b'), E('b', 'a')] },
  'cycle-self': { nodes: [N('a', 80, 40)], edges: [E('a', 'a')] },
  'cycle-self-child': { nodes: [N('a', 80, 40), N('b', 60, 30)], edges: [E('a', 'b'), E('b', 'b')] },
  'cycle-three-below-root': {
    nodes: [N('r', 80, 40), N('a', 60, 30), N('b', 60, 30), N('c', 60, 30)],
    edges: [E('r', 'a'), E('a', 'b'), E('b', 'c'), E('c', 'a')],
  },
  // Every node has a parent, and the first node has no child: no cycle is
  // reached.
  'cycle-not-reached': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('c', 60, 30)],
    edges: [E('b', 'c'), E('c', 'b'), E('b', 'a')],
  },
  'cycle-apart-from-root': {
    nodes: [N('r', 80, 40), N('k', 50, 30), N('b', 60, 30), N('c', 60, 30)],
    edges: [E('r', 'k'), E('b', 'c'), E('c', 'b')],
  },
  'diamond': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('c', 60, 30), N('d', 70, 36)],
    edges: [E('a', 'b'), E('a', 'c'), E('b', 'd'), E('c', 'd')],
  },
  'same-edge-twice': {
    nodes: [N('a', 80, 40), N('b', 60, 30)],
    edges: [E('a', 'b', 'e1'), E('a', 'b', 'e2')],
  },
  'same-edge-id': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('c', 50, 30)],
    edges: [E('a', 'b', 'e'), E('a', 'c', 'e')],
  },
  'edge-to-unknown': {
    nodes: [N('a', 80, 40), N('b', 60, 30)],
    edges: [E('a', 'b'), E('a', 'zz'), E('zz', 'b'), E('zz', 'yy')],
  },
  'edge-without-ends': {
    nodes: [N('a', 80, 40), N('b', 60, 30)],
    edges: [{ id: 'e0' }, { id: 'e1', start: 'a' }, { id: 'e2', end: 'b' }, { start: 'a', end: 'b' }, { id: 'e4', start: '', end: '' }],
  },
  'duplicate-node-id': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('b', 200, 90, 'circle'), N('c', 50, 30)],
    edges: [E('a', 'b'), E('a', 'c')],
  },
  'duplicate-root-id': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('a', 300, 120, 'cloud')],
    edges: [E('a', 'b')],
  },
  'numeric-ids': {
    nodes: [N('10', 80, 40), N('2', 60, 30), N('1', 50, 30), N('03', 50, 30)],
    edges: [E('10', '2'), E('10', '1'), E('10', '03')],
  },
  'same-level-sizes': {
    nodes: [N('a', 80, 40), N('b', 60, 30), N('c', 60, 90), N('d', 60, 10), N('e', 60, 50), N('f', 60, 30), N('g', 60, 70)],
    edges: [E('a', 'b'), E('a', 'c'), E('a', 'd'), E('a', 'e'), E('a', 'f'), E('a', 'g')],
  },
  // A tall child beside a short one with a deep subtree: the threads of
  // the contours.
  'threads': {
    nodes: [
      N('r', 80, 40), N('a', 200, 30), N('b', 20, 30), N('c', 20, 30), N('a1', 30, 30), N('b1', 30, 20), N('b2', 30, 20),
      N('b3', 300, 20), N('c1', 30, 20), N('c2', 30, 200), N('b31', 30, 20), N('b32', 30, 20),
    ],
    edges: [
      E('r', 'a'), E('r', 'x'), E('r', 'b'), E('r', 'y'), E('r', 'c'), E('a', 'a1'), E('b', 'b1'), E('b', 'b2'), E('b', 'b3'),
      E('c', 'c1'), E('c', 'c2'), E('b3', 'b31'), E('b3', 'b32'),
    ],
  },
  // Many siblings between two large subtrees: the extra space is spread.
  'distribute-extra': {
    nodes: [
      N('r', 80, 40), N('p', 60, 30),
      N('a', 40, 30), N('b', 40, 20), N('c', 40, 20), N('d', 40, 20), N('e', 40, 30),
      N('a1', 40, 200), N('a2', 40, 200), N('e1', 40, 200), N('e2', 40, 200),
    ],
    edges: [
      E('r', 'p'), E('p', 'a'), E('p', 'b'), E('p', 'c'), E('p', 'd'), E('p', 'e'),
      E('a', 'a1'), E('a', 'a2'), E('e', 'e1'), E('e', 'e2'),
    ],
  },
};

// ---- run ----

const num = (x) =>
  x === undefined || x === null
    ? null
    : Number.isNaN(x)
      ? 'NaN'
      : x === Infinity
        ? 'Infinity'
        : x === -Infinity
          ? '-Infinity'
          : Object.is(x, -0)
            ? '-0'
            : x;
const opt = (x) => (x === undefined ? null : x);

const edgeRow = (e) => [
  opt(e.id), e.source, e.target, num(e.startX), num(e.startY), num(e.midX), num(e.midY), num(e.endX), num(e.endY),
  e.points.flatMap((p) => [num(p.x), num(p.y)]),
  opt(e.sourceSection), opt(e.targetSection), num(e.sourceWidth), num(e.sourceHeight), num(e.targetWidth), num(e.targetHeight),
];

async function run(name, { nodes, edges }) {
  // The golden file has the input as the layout gets it. The layout gets
  // objects of its own, as the render makes a copy of each node.
  const input = {
    name,
    nodes: nodes === null ? null : nodes.map((n) => ({ id: n.id, width: num(n.width), height: num(n.height), shape: opt(n.shape) })),
    edges: edges === null ? null : edges.map((e) => ({ id: opt(e.id), start: opt(e.start), end: opt(e.end) })),
  };
  const data = { config: {} };
  if (nodes !== null) data.nodes = nodes;
  if (edges !== null) data.edges = edges;
  randomCalls = 0;
  try {
    const result = await executeTidyTreeLayout(data);
    input.random = randomCalls;
    input.result = {
      nodes: result.nodes.map((n) => [
        n.id, num(n.x), num(n.y), opt(n.section), num(n.width), num(n.height), nodes.indexOf(n.originalNode),
      ]),
      edges: result.edges.map(edgeRow),
    };
  } catch (error) {
    input.random = randomCalls;
    input.error = { name: error?.name ?? null, message: String(error?.message ?? error) };
  }
  return input;
}

const cases = [];
for (const [name, c] of Object.entries(FIXED)) {
  cases.push(await run(`fixed/${name}`, c));
}

const KINDS = [
  'tree', 'tree', 'tree', 'tree', 'tree', 'tree', 'shuffled', 'shuffled', 'forest', 'dag', 'cyclic', 'dangling',
  'duplicate', 'noedges',
];
for (let i = 0; i < opts.random; i++) {
  const kind = KINDS[i % KINDS.length];
  cases.push(await run(`${kind}/${i}`, randomCase(kind)));
}

// A few large trees, for the paths that small trees do not reach.
for (const [shape, n] of [['any', 400], ['deep', 300], ['wide', 300], ['binary', 255], ['flat', 120], ['chain', 150]]) {
  const parent = parents(n, shape);
  const nodes = Array.from({ length: n }, (_, i) => makeNode(`n${i}`, 'text'));
  const edges = [];
  for (let i = 1; i < n; i++) edges.push({ id: `edge_${parent[i]}_${i}`, start: `n${parent[i]}`, end: `n${i}` });
  cases.push(await run(`large/${shape}-${n}`, { nodes, edges }));
}

// ---- direct calls of the functions that place the ends of an edge ----

// coordinate returns a number from a small set, so that two coordinates
// are often equal, or any double.
function coordinate() {
  const k = rnd();
  if (k < 0.55) return pick([0, 0, 10, -10, 20, 25, 30, -30, 40, 50, 60, 100, -100, 12.5, 37.5]);
  if (k < 0.75) return int(-200, 200);
  if (k < 0.95) return (rnd() - 0.5) * 600;
  return pick([-0, 1e-12, -1e9, 1e9, 0.1 + 0.2]);
}

// side returns a width or a height of a positioned node.
function side(odd) {
  const k = rnd();
  if (odd) {
    if (k < 0.15) return 0;
    if (k < 0.25) return undefined;
    if (k < 0.35) return -pick([20, 40, 60, 80]);
    if (k < 0.4) return -rnd() * 300;
    if (k < 0.45) return NaN;
  }
  if (k < 0.6) return pick([20, 40, 60, 80, 100, 200]);
  if (k < 0.8) return int(1, 300);
  return rnd() * 300;
}

const intersections = [];
for (let i = 0; i < 4000; i++) {
  const odd = i % 4 === 0;
  const node = { x: coordinate(), y: coordinate(), width: side(odd), height: side(odd) };
  const outside = { x: coordinate(), y: coordinate() };
  const inside = { x: coordinate(), y: coordinate() };
  const how = i % 5;
  if (how === 1) {
    // The inside point is the center of the node, as the layout calls it.
    inside.x = node.x;
    inside.y = node.y;
  } else if (how === 2) {
    // The outside point is on the border of the node, or at its height.
    outside.y = rnd() < 0.5 ? node.y : node.y + (rnd() < 0.5 ? 1 : -1) * (node.height ?? 0) / 2;
    if (rnd() < 0.5) outside.x = node.x + (rnd() < 0.5 ? 1 : -1) * (node.width ?? 0) / 2;
  } else if (how === 3) {
    outside.x = node.x;
    outside.y = node.y;
  }
  const r = internals.__intersection(node, outside, inside);
  intersections.push([node.x, node.y, node.width, node.height, outside.x, outside.y, inside.x, inside.y, r.x, r.y].map(num));
}

const circles = [];
for (let i = 0; i < 1500; i++) {
  const circle = { x: coordinate(), y: coordinate(), width: side(false), height: side(false) };
  if (i % 6 === 0) circle.height = circle.width;
  if (i % 50 === 0) circle.width = NaN;
  const start = { x: coordinate(), y: coordinate() };
  const end = { x: coordinate(), y: coordinate() };
  if (i % 7 === 0) {
    // A line with no length.
    end.x = start.x;
    end.y = start.y;
  } else if (i % 7 === 1) {
    end.x = circle.x;
    end.y = circle.y;
  }
  const r = internals.__circle(circle, start, end);
  circles.push([circle.x, circle.y, circle.width, circle.height, start.x, start.y, end.x, end.y, r.x, r.y].map(num));
}

// Positioned nodes with any section: the layout itself never has an edge
// that ends at the root, and never has a node without a section.
const edgePositions = [];
for (let i = 0; i < 600; i++) {
  const n = int(1, 6);
  const odd = i % 5 === 0;
  const nodes = [];
  for (let k = 0; k < n; k++) {
    const node = { id: `n${k}`, x: coordinate(), y: coordinate() };
    const section = pick(['root', 'left', 'right', 'left', 'right', undefined]);
    if (section !== undefined) node.section = section;
    // No NaN here: in the port a NaN size is an undefined size, and the
    // layout never has a NaN size, as it reads the sizes with `??`.
    const width = side(odd);
    const height = side(odd);
    if (width !== undefined) node.width = Number.isNaN(width) ? 0 : width;
    if (height !== undefined) node.height = Number.isNaN(height) ? 0 : height;
    const shape = pick(SHAPES);
    // A node sometimes has no original node.
    if (shape !== undefined || rnd() < 0.7) node.originalNode = shape === undefined ? {} : { shape };
    nodes.push(node);
  }
  if (rnd() < 0.2 && n > 1) {
    // Two nodes at one place.
    nodes[1].x = nodes[0].x;
    nodes[1].y = nodes[0].y;
  }
  const edges = [];
  for (let k = int(1, 5); k > 0; k--) {
    const edge = { id: `e${k}` };
    const start = rnd() < 0.93 ? `n${int(0, n - 1)}` : pick([undefined, 'missing']);
    const end = rnd() < 0.93 ? `n${int(0, n - 1)}` : pick([undefined, 'missing']);
    if (start !== undefined) edge.start = start;
    if (end !== undefined) edge.end = end;
    edges.push(edge);
  }
  const shift = pick([30, 30, 30, 50, 0]);
  const input = {
    nodes: nodes.map((p) => [
      p.id, num(p.x), num(p.y), opt(p.section), num(p.width), num(p.height),
      p.originalNode === undefined ? null : p.originalNode.shape === undefined ? '' : p.originalNode.shape,
    ]),
    edges: edges.map((e) => [e.id, opt(e.start), opt(e.end)]),
    shift,
  };
  input.result = internals.__edgePositions(edges, nodes, shift).map(edgeRow);
  edgePositions.push(input);
}

const tally = {};
for (const c of cases) {
  const kind = c.name.split('/')[0] + (c.error ? `:${c.error.name}` : '');
  tally[kind] = (tally[kind] ?? 0) + 1;
}

fs.mkdirSync(path.dirname(outFile), { recursive: true });
const json = JSON.stringify({
  package: pkg.name,
  version: pkg.version,
  cases,
  internals: { intersection: intersections, circle: circles, edgePositions },
});
const gz = zlib.gzipSync(Buffer.from(json), { level: 9 });
fs.writeFileSync(outFile, gz);
console.log(
  `${path.relative(root, outFile)}: ${cases.length} cases, ${(gz.length / 1024).toFixed(0)} KiB gzip, ${(json.length / 1024).toFixed(0)} KiB json`
);
console.log(Object.entries(tally).map(([k, v]) => `${k}=${v}`).join(' '));
console.log(`internals: intersection=${intersections.length} circle=${circles.length} edgePositions=${edgePositions.length}`);
