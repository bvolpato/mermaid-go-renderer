// Random inputs of the two layouts of internal/cytoscape, for the coverage
// report. rng is a function that returns a number in [0, 1).

function int(rng, lo, hi) {
  return lo + Math.floor(rng() * (hi - lo + 1));
}

// randomMindmap is the input of coseBilkent of the glue: a tree, as the
// mindmap db builds it, or (with general) a graph with more edges.
export function randomMindmap(rng, opts = {}) {
  const max = opts.max || [1, 2, 3, 5, 8, 13, 30, 60, 120, 200][int(rng, 0, 9)];
  const n = int(rng, 1, max);
  const nodes = [];
  const edges = [];
  const depthBias = rng();
  const newTree = opts.forest ? 0.05 + rng() * 0.2 : 0;
  // The first node of the current tree.
  let first = 0;
  for (let i = 0; i < n; i++) {
    let w = rng() < 0.5 ? 30 + Math.floor(rng() * 4000) / 16 : int(rng, 20, 300);
    let h = rng() < 0.7 ? 34 : 34 + Math.floor(rng() * 1600) / 16;
    if (opts.special && rng() < 0.15) {
      w = 0;
    }
    if (opts.special && rng() < 0.15) {
      h = 0;
    }
    nodes.push({ id: String(i), width: w, height: h, x: 0, y: 0 });
    if (i > 0) {
      if (rng() < newTree) {
        first = i;
        continue;
      }
      // A deep tree takes a late node as the parent, a flat tree an early one.
      const p = depthBias < 0.33 ? int(rng, first, Math.min(i - 1, first + 2)) : depthBias < 0.66 ? int(rng, first, i - 1) : Math.max(first, i - 1 - int(rng, 0, 2));
      edges.push({ id: 'edge_' + p + '_' + i, start: String(p), end: String(i) });
    }
  }
  if (opts.general) {
    const extra = int(rng, 1, Math.max(1, n >> 1));
    for (let k = 0; k < extra; k++) {
      const a = int(rng, 0, n - 1);
      const b = int(rng, 0, n - 1);
      edges.push({ id: 'x_' + k, start: String(a), end: String(b) });
    }
  }
  if (opts.special) {
    const kind = int(rng, 0, 3);
    if (kind === 0 && n > 1) {
      // An edge in parallel to another one, in one of the two directions.
      const e = edges[int(rng, 0, edges.length - 1)];
      edges.push(rng() < 0.5 ? { id: 'p', start: e.start, end: e.end } : { id: 'p', start: e.end, end: e.start });
    } else if (kind === 1) {
      const a = String(int(rng, 0, n - 1));
      edges.push({ id: 'self', start: a, end: a });
    } else if (kind === 2) {
      // Nodes without edges.
      const more = int(rng, 1, 3);
      for (let k = 0; k < more; k++) {
        nodes.push({ id: 'alone' + k, width: int(rng, 10, 100), height: int(rng, 10, 60), x: 0, y: 0 });
      }
    }
    // kind 3: only the sizes.
  }
  return { nodes, edges };
}

const opposite = { L: 'R', R: 'L', T: 'B', B: 'T' };
const shift = { L: [-1, 0], R: [1, 0], T: [0, 1], B: [0, -1] };

// randomArchitecture is the input of architecture of the glue. The nodes of
// a component sit on a grid, and the constraints follow from the grid, as in
// mermaid.
export function randomArchitecture(rng, opts = {}) {
  const max = opts.max || [1, 2, 3, 5, 8, 12, 20, 40][int(rng, 0, 7)];
  const n = int(rng, 1, max);
  const groupCount = rng() < 0.3 ? 0 : int(rng, 1, Math.max(1, Math.min(8, n >> 1)));
  const groups = [];
  for (let g = 0; g < groupCount; g++) {
    const group = { id: 'g' + g, title: rng() < 0.8 ? 'Group ' + g : undefined, icon: rng() < 0.3 ? 'cloud' : undefined };
    if (g > 0 && rng() < 0.4) {
      group.in = 'g' + int(rng, 0, g - 1);
    }
    groups.push(group);
  }
  const services = [];
  const junctions = [];
  const ids = [];
  for (let i = 0; i < n; i++) {
    const inGroup = groupCount > 0 && rng() < 0.7 ? 'g' + int(rng, 0, groupCount - 1) : undefined;
    if (rng() < 0.15) {
      junctions.push({ id: 'j' + i, in: inGroup });
      ids.push('j' + i);
    } else {
      services.push({ id: 's' + i, title: rng() < 0.85 ? 'Service ' + i + (rng() < 0.2 ? ' with a long title' : '') : undefined, icon: 'server', in: inGroup });
      ids.push('s' + i);
    }
  }
  const groupOf = {};
  for (const s of services.concat(junctions)) {
    groupOf[s.id] = s.in === undefined ? 'default' : s.in;
  }
  // Components on grids.
  const edges = [];
  const used = new Set();
  const addEdge = (a, b, da, db) => {
    const key = a + '-' + b;
    if (a === b || used.has(key)) {
      return;
    }
    used.add(key);
    edges.push({ lhsId: a, rhsId: b, lhsDir: da, rhsDir: db, title: rng() < 0.15 ? 'edge ' + edges.length : undefined });
  };
  const horizontal = [];
  const vertical = [];
  const relative = [];
  const gap = (opts.iconSize || 80) * 1.5;
  let at = 0;
  while (at < n) {
    const size = Math.min(n - at, int(rng, 1, 9));
    const cells = new Map();
    const pos = {};
    cells.set('0,0', ids[at]);
    pos[ids[at]] = [0, 0];
    for (let k = 1; k < size; k++) {
      const id = ids[at + k];
      for (let tries = 0; tries < 50; tries++) {
        const from = ids[at + int(rng, 0, k - 1)];
        const dir = 'LRTB'[int(rng, 0, 3)];
        const p = [pos[from][0] + shift[dir][0], pos[from][1] + shift[dir][1]];
        if (!cells.has(p.join(','))) {
          cells.set(p.join(','), id);
          pos[id] = p;
          addEdge(from, id, dir, opposite[dir]);
          break;
        }
      }
    }
    const placed = ids.slice(at, at + size).filter((id) => pos[id]);
    // Alignments: the nodes of a row and of a column, by group.
    const rows = new Map();
    const cols = new Map();
    for (const id of placed) {
      const [x, y] = pos[id];
      for (const [m, key] of [[rows, y], [cols, x]]) {
        const k = key + '/' + (rng() < 0.8 ? 'all' : groupOf[id]);
        if (!m.has(k)) {
          m.set(k, []);
        }
        m.get(k).push(id);
      }
    }
    for (const list of rows.values()) {
      if (list.length > 1) {
        horizontal.push(list);
      }
    }
    for (const list of cols.values()) {
      if (list.length > 1) {
        vertical.push(list);
      }
    }
    // Relative placements: a search from the first cell.
    const queue = ['0,0'];
    const visited = new Set();
    while (queue.length > 0) {
      const curr = queue.shift();
      visited.add(curr);
      const currId = cells.get(curr);
      const [cx, cy] = curr.split(',').map(Number);
      for (const dir of 'LRTB') {
        const np = cx + shift[dir][0] + ',' + (cy + shift[dir][1]);
        const newId = cells.get(np);
        if (newId && !visited.has(np)) {
          queue.push(np);
          if (dir === 'L') {
            relative.push({ left: newId, right: currId, gap });
          } else if (dir === 'R') {
            relative.push({ right: newId, left: currId, gap });
          } else if (dir === 'T') {
            relative.push({ top: newId, bottom: currId, gap });
          } else {
            relative.push({ bottom: newId, top: currId, gap });
          }
        }
      }
    }
    // Edges that bend, and edges between cells that are not neighbours.
    const extra = rng() < 0.5 ? int(rng, 0, 3) : 0;
    for (let k = 0; k < extra; k++) {
      const a = placed[int(rng, 0, placed.length - 1)];
      const b = placed[int(rng, 0, placed.length - 1)];
      addEdge(a, b, 'LRTB'[int(rng, 0, 3)], 'LRTB'[int(rng, 0, 3)]);
    }
    at += size;
  }
  // Edges between components.
  if (n > 1 && rng() < 0.4) {
    const extra = int(rng, 1, 3);
    for (let k = 0; k < extra; k++) {
      addEdge(ids[int(rng, 0, n - 1)], ids[int(rng, 0, n - 1)], 'LRTB'[int(rng, 0, 3)], 'LRTB'[int(rng, 0, 3)]);
    }
  }
  const iconSize = opts.iconSize || 80;
  return {
    config: {
      iconSize,
      fontSize: 16,
      padding: rng() < 0.8 ? 40 : int(rng, 0, 60),
      idealEdgeLengthMultiplier: 1.5,
      edgeElasticity: 0.45,
      randomize: rng() < 0.2,
      nodeSeparation: 75,
      numIter: 2500,
      seed: rng() < 0.9 ? 1 : int(rng, 0, 1000),
    },
    groups,
    services,
    junctions,
    edges,
    alignmentConstraint: { horizontal, vertical },
    relativePlacementConstraint: relative,
  };
}
