// Makes the shape cases of internal/ru/shapes/set3 and records them from
// upstream mermaid in Chromium.
//
//   node gen-shape-cases-set3.mjs [shape ...]
//
// With no argument it records every shape. Each shape gets one golden file
// internal/ru/shapes/set3/testdata/<shape>.json.gz, written by
// gen-shapes.mjs (probes/shapes.js).
//
// The node objects follow what the diagrams build: db.getData() of state
// diagrams and flowcharts, getNodeFromBlock of block diagrams, and the
// label nodes of the layouts. probes/dump-data.js prints such nodes.
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outDir = path.join(here, '../../internal/ru/shapes/set3/testdata');

// The look that a diagram type has when the config sets none.
const defaultLook = { 'flowchart-v2': 'neo', stateDiagram: 'neo', block: 'classic', c4: 'classic' };

const longLabel = 'This is a long label that has to wrap over several lines to fit the width';
const brLabel = 'First line<br/>Second line<br>third';
const markdownLabel = 'Some **bold** and *italic* text\nwith a second line';

// builder collects the cases of one shape. add(name, directive, node)
// makes a case from the base node of the shape and the given changes. The
// look of the node follows the directive, as it does in the diagram dbs.
const builder = (shape, type, baseNode) => {
  const cases = [];
  const add = (name, directive, node = {}, extra = {}) => {
    const look = directive.look ?? defaultLook[type];
    const merged = { ...baseNode, look, ...node, shape };
    for (const key of Object.keys(merged)) {
      if (merged[key] === undefined) {
        delete merged[key];
      }
    }
    cases.push({ name: `${shape}/${name}`, type, directive, node: merged, ...extra });
  };
  return { cases, add };
};

const looks = ['classic', 'neo', 'handDrawn'];
const classic = { look: 'classic', theme: 'default' };
const svgLabels = { ...classic, htmlLabels: false };

// The groups of cases that many shapes share. A shape takes the groups
// that reach code of its own: the same label code runs for every shape, so
// a shape that only passes its label on needs few label cases.
const groups = {
  // Every look with two themes, and the look and theme of the diagram type.
  looks: (add) => {
    for (const look of looks) {
      for (const theme of ['default', 'redux-color']) {
        add(`look-${look}-${theme}`, { look, theme });
      }
    }
    add('no-directive', {});
  },
  themes: (add) => {
    add('theme-dark', { look: 'neo', theme: 'dark' });
    add('theme-redux-dark-color', { look: 'neo', theme: 'redux-dark-color' });
    add('theme-dark-handdrawn', { look: 'handDrawn', theme: 'dark' });
    add('handdrawn-seed', { look: 'handDrawn', theme: 'default', handDrawnSeed: 7 });
  },
  // SVG labels: the config key, its old place under flowchart, and the
  // useHtmlLabels property of the node.
  svg: (add) => {
    for (const look of looks) {
      add(`svg-label-${look}`, { look, theme: 'default', htmlLabels: false });
    }
    add('svg-label-flowchart-key', { ...classic, flowchart: { htmlLabels: false } });
    add('svg-label-node-html', svgLabels, { useHtmlLabels: true });
    add('html-label-node-false', classic, { useHtmlLabels: false });
  },
  labels: (add) => {
    for (const [tag, directive] of [['html', classic], ['svg', svgLabels]]) {
      add(`long-${tag}`, directive, { label: longLabel });
      add(`empty-${tag}`, directive, { label: '' });
      add(`br-${tag}`, directive, { label: brLabel });
      add(`markdown-${tag}`, directive, { label: markdownLabel, labelType: 'markdown' });
    }
    add('entities', classic, { label: 'a &amp; b #quot;c#quot; <b>d</b> é ü' });
    add('no-label', classic, { label: undefined });
    add('fa-icon-label', classic, { label: 'fa:fa-bell Ring' });
    add('long-word', classic, { label: 'Supercalifragilisticexpialidocious_and_more_characters' });
    add('wide-wrap', classic, { label: longLabel, wrappingWidth: 400 });
  },
  css: (add) => {
    for (const look of looks) {
      add(`css-styles-${look}`, { look, theme: 'default' }, { cssStyles: ['fill:#f9f', 'stroke:#333', 'stroke-width:4px'] });
    }
    add('css-label-styles', classic, { cssStyles: ['color:red', 'font-weight:bold', 'font-size:20px', 'fill:#ff0'] });
    add('css-label-styles-svg', svgLabels, { cssStyles: ['color:red', 'font-weight:bold', 'font-size:20px', 'fill:#ff0'] });
    add('css-compiled', { look: 'neo', theme: 'redux-color' }, { cssCompiledStyles: ['fill:#f00', 'color:white', 'stroke-width:2px', 'stroke:yellow'], cssClasses: 'cls statediagram-state' });
    add('css-compiled-handdrawn', { look: 'handDrawn', theme: 'default' }, { cssCompiledStyles: ['fill:#f00', 'color:white', 'stroke-width:2px', 'stroke:yellow', 'stroke-dasharray: 5 5'] });
    add('css-dash', classic, { cssStyles: ['stroke-dasharray:3'] });
    add('css-undefined', classic, { cssStyles: undefined, cssCompiledStyles: undefined, cssClasses: undefined });
    add('css-undefined-handdrawn', { look: 'handDrawn', theme: 'default' }, { cssStyles: undefined, cssCompiledStyles: undefined });
    add('css-no-colon', classic, { cssStyles: ['fill', 'stroke:', 'color: blue ; '] });
  },
  padding: (add) => {
    for (const padding of [0, 20, undefined]) {
      add(`padding-${padding}`, classic, { padding });
    }
    add('padding-undefined-neo', { look: 'neo', theme: 'redux-color' }, { padding: undefined });
  },
  size: (add) => {
    add('width', classic, { width: 300 });
    add('height', { look: 'neo', theme: 'redux-color' }, { height: 100 });
    add('width-height', { look: 'handDrawn', theme: 'default' }, { width: 250, height: 90 });
    add('small-size', classic, { width: 10, height: 5 });
  },
  link: (add) => {
    add('link', classic, { link: 'https://example.com/', linkTarget: '_blank', tooltip: 'A tip' });
    add('link-sandbox', { ...classic, securityLevel: 'sandbox' }, { link: 'https://example.com/' });
    add('tooltip-callback', { look: 'neo', theme: 'redux-color' }, { tooltip: 'A tip', haveCallback: true });
  },
  font: (add) => {
    add('font-courier', { ...classic, fontFamily: 'courier' });
    add('font-size', { ...classic, themeVariables: { fontSize: '20px' } });
    add('font-size-svg', { ...svgLabels, themeVariables: { fontSize: '20px' } });
  },
  misc: (add) => {
    add('loose', { ...classic, securityLevel: 'loose' });
    add('no-dom-id', classic, { domId: undefined });
  },
};

// standard adds the cases of the named groups, or of all groups.
const standard = (add, names = Object.keys(groups)) => {
  for (const name of names) {
    groups[name](add);
  }
};

// The node of a state, as stateDb.getData() makes it.
const stateNode = {
  labelStyle: '',
  label: 'Idle state',
  cssClasses: ' statediagram-state',
  cssCompiledStyles: [],
  cssStyles: [],
  id: 'S1',
  domId: 'my-svg-state-S1-3',
  isGroup: false,
  padding: 8,
  rx: 10,
  ry: 10,
  labelType: 'markdown',
  centerLabel: true,
  wrappingWidth: 120,
  minWidth: 120,
};

const shapes = {};

shapes.state = () => {
  // state.ts gives the node to drawRect with no label padding, so the
  // size of its rect is NaN. No diagram uses the shape.
  const { cases, add } = builder('state', 'stateDiagram', stateNode);
  standard(add, ['looks', 'svg', 'css']);
  return cases;
};

shapes.choice = () => {
  const { cases, add } = builder('choice', 'stateDiagram', { ...stateNode, label: 'c1', id: 'c1', domId: 'my-svg-state-c1-2' });
  standard(add, ['looks', 'themes', 'css', 'link', 'misc']);
  add('width-20', classic, { width: 20 });
  add('width-60', classic, { width: 60 });
  add('width-60-handdrawn', { look: 'handDrawn', theme: 'default' }, { width: 60 });
  add('flowchart-type', classic, {}, { type: 'flowchart-v2' });
  return cases;
};

shapes.note = () => {
  const { cases, add } = builder('note', 'stateDiagram', {
    labelStyle: '',
    label: 'A note<br>two lines',
    labelType: 'markdown',
    cssClasses: 'statediagram-note',
    cssStyles: [],
    cssCompiledStyles: [],
    id: 'S1----note-3',
    domId: 'my-svg-state-S1----note-3',
    type: 'node',
    isGroup: false,
    padding: 15,
    position: 'right of',
    parentId: 'S1----parent',
    wrappingWidth: 120,
    minWidth: 120,
  });
  standard(add);
  add('note-colors', { ...classic, themeVariables: { noteBkgColor: '#cfc', noteBorderColor: '#060' } });
  add('note-colors-handdrawn', { look: 'handDrawn', theme: 'default', themeVariables: { noteBkgColor: '#cfc', noteBorderColor: '#060' } });
  add('center-label', classic, { centerLabel: true });
  add('left-of', { look: 'neo', theme: 'redux-color' }, { position: 'left of', label: 'short note' });
  return cases;
};

shapes.labelRect = () => {
  // The label node of an edge, as the layouts make it.
  const { cases, add } = builder('labelRect', 'flowchart-v2', {
    id: 'edge-label-A-B-L_A_B_0',
    label: 'edge label',
    edgeStart: 'A',
    edgeEnd: 'B',
    width: 0,
    height: 0,
    isEdgeLabel: true,
    isDummy: true,
    isGroup: false,
    labelStyle: '',
  });
  standard(add, ['looks', 'svg', 'labels', 'padding', 'link', 'font']);
  // The dummy nodes of a self loop, as the dagre layout makes them.
  const selfLoop = { domId: 'A---A---1', id: 'A---A---1', labelStyle: '', label: '', padding: 0, style: '', width: 10, height: 10, edgeStart: undefined, edgeEnd: undefined, isEdgeLabel: undefined, isDummy: undefined, isGroup: undefined };
  for (const look of looks) {
    add(`self-loop-${look}`, { look, theme: 'default' }, { ...selfLoop, look: undefined });
  }
  add('self-loop-svg', svgLabels, { ...selfLoop, look: undefined });
  add('label-style', classic, { labelStyle: 'color:red;font-size:18px' });
  add('label-style-svg', svgLabels, { labelStyle: 'color:red;font-size:18px' });
  add('label-style-undefined', classic, { labelStyle: undefined });
  add('label-style-undefined-svg', svgLabels, { labelStyle: undefined });
  add('state-type', { look: 'neo', theme: 'redux-color' }, {}, { type: 'stateDiagram' });
  return cases;
};

shapes.anchor = () => {
  // The node of a flowchart vertex with the shape anchor.
  const { cases, add } = builder('anchor', 'flowchart-v2', {
    id: 'n00',
    label: 'This is a label for anchor shape',
    labelType: 'markdown',
    labelStyle: '',
    padding: 15,
    cssStyles: [],
    cssCompiledStyles: [],
    cssClasses: 'default ',
    domId: 'my-svg-flowchart-n00-0',
    isGroup: false,
    minWidth: 120,
  });
  standard(add, ['looks', 'themes', 'css', 'link', 'misc']);
  return cases;
};

shapes.rectWithTitle = () => {
  // A state with a description, as stateDb.getData() makes it.
  const { cases, add } = builder('rectWithTitle', 'stateDiagram', {
    ...stateNode,
    label: 'State title',
    id: 'S2',
    domId: 'my-svg-state-S2-4',
    description: ['here comes the description'],
  });
  standard(add, ['looks', 'svg', 'labels', 'css', 'padding', 'link']);
  for (const [tag, directive] of [['html', classic], ['svg', svgLabels], ['handdrawn', { look: 'handDrawn', theme: 'default' }]]) {
    add(`descr-none-${tag}`, directive, { description: [] });
    add(`descr-undefined-${tag}`, directive, { description: undefined });
    add(`descr-many-${tag}`, directive, { description: ['first row', 'second row is longer than the title', 'third'] });
    add(`descr-br-${tag}`, directive, { description: ['one<br/>two', 'three'] });
    add(`descr-short-${tag}`, directive, { label: 'A title that is much longer than the text', description: ['x'] });
    add(`descr-same-${tag}`, directive, { label: 'same text', description: ['same text'] });
    add(`title-empty-${tag}`, directive, { label: '', description: ['only a description'] });
  }
  add('rx-0-handdrawn', { look: 'handDrawn', theme: 'default' }, { rx: 0, ry: 0 });
  add('rx-undefined-handdrawn', { look: 'handDrawn', theme: 'default' }, { rx: undefined, ry: undefined });
  add('no-classes', classic, { cssClasses: '' });
  return cases;
};

shapes.collapsedGroup = () => {
  // A collapsed subgraph, as flowDb.getData() makes it.
  const { cases, add } = builder('collapsedGroup', 'flowchart-v2', {
    id: 'grp',
    label: 'Collapsed group',
    labelStyle: '',
    labelType: 'text',
    padding: 8,
    cssCompiledStyles: [],
    cssClasses: '',
    isGroup: false,
    colorIndex: 0,
    cssStyles: [],
  });
  standard(add, ['looks', 'themes', 'svg', 'labels', 'css', 'padding', 'link']);
  for (const look of looks) {
    add(`flow-${look}`, { look, theme: 'default' }, { metadata: { containerType: 'flow' } });
    add(`flow-redux-${look}`, { look, theme: 'redux-color' }, { metadata: { containerType: 'flow' } });
    add(`other-container-${look}`, { look, theme: 'default' }, { metadata: { containerType: 'agent', view: 'collapsed' } });
  }
  add('flow-stroke', { ...classic, themeVariables: { flowContainerStroke: '#123456' } }, { metadata: { containerType: 'flow' } });
  add('flow-svg', svgLabels, { metadata: { containerType: 'flow' } });
  add('cluster-colors', { ...classic, themeVariables: { clusterBkg: '#fed', clusterBorder: '#a50' } });
  add('cluster-colors-handdrawn', { look: 'handDrawn', theme: 'default', themeVariables: { clusterBkg: '#fed', clusterBorder: '#a50' } });
  for (const colorIndex of [undefined, 1, 11, 12, 25]) {
    add(`slot-${colorIndex}`, { look: 'neo', theme: 'redux-color' }, { colorIndex });
  }
  add('slot-dark', { look: 'neo', theme: 'redux-dark-color' }, { colorIndex: 2 });
  add('slot-default-theme', classic, { colorIndex: 2 });
  add('parent', classic, { parentId: 'outer', dir: 'TB' });
  add('wide', classic, { width: 200, height: 120 });
  add('markdown-title', classic, { label: 'A **bold** title', labelType: 'markdown' });
  add('label-style', classic, { labelStyle: 'color:red;font-size:18px' });
  add('label-style-undefined', classic, { labelStyle: undefined });
  add('label-style-undefined-handdrawn', { look: 'handDrawn', theme: 'default' }, { labelStyle: undefined });
  return cases;
};

// The node of a block, as getNodeFromBlock of the block diagram makes it.
// The first pass of the block renderer sizes the block: the node is not
// positioned, it has no width and its wrapping width is Infinity. The
// second pass draws it with the size and the position of the layout.
const blockNode = {
  labelStyle: '',
  label: 'Block label',
  labelText: 'Block label',
  rx: 0,
  ry: 0,
  class: 'default flowchart-label',
  cssClasses: 'default flowchart-label',
  cssStyles: [],
  cssCompiledStyles: [],
  style: '',
  id: 'b1',
  domId: 'my-svg-b1',
  isGroup: false,
  wrappingWidth: 'Infinity',
  x: 0,
  y: 0,
  positioned: false,
  padding: 8,
  widthInColumns: 1,
};
const positionedBlock = (width, height) => ({ width, height, wrappingWidth: width, x: 120, y: 60, positioned: true });

shapes.composite = () => {
  const { cases, add } = builder('composite', 'block', { ...blockNode, label: '', labelText: '', id: 'group1', domId: 'my-svg-group1', padding: 0 });
  standard(add, ['looks', 'svg', 'labels', 'css', 'padding', 'link']);
  // The root block has no label.
  add('root', classic, { label: undefined, labelText: undefined, id: 'root', domId: 'my-svg-root' });
  for (const look of looks) {
    add(`positioned-${look}`, { look, theme: 'default' }, positionedBlock(180, 90));
    add(`positioned-redux-${look}`, { look, theme: 'redux-color' }, { ...positionedBlock(180, 90), colorIndex: 1 });
  }
  add('positioned-no-size', classic, { positioned: true });
  add('positioned-label', classic, { ...positionedBlock(220, 70), label: 'A group', labelText: 'A group' });
  add('positioned-label-svg', svgLabels, { ...positionedBlock(220, 70), label: 'A group', labelText: 'A group' });
  add('label', classic, { label: 'A group', labelText: 'A group' });
  add('label-svg', svgLabels, { label: 'A group', labelText: 'A group' });
  add('label-wrap', classic, { label: longLabel, wrappingWidth: 150 });
  add('style', classic, { style: 'fill:#f9f;stroke:#333', cssStyles: ['fill:#f9f', 'stroke:#333'] });
  add('style-undefined', classic, { style: undefined });
  add('class-undefined', classic, { class: undefined });
  add('class-other', classic, { class: 'blue flowchart-label', cssClasses: 'blue flowchart-label', cssCompiledStyles: ['fill:#00f'] });
  add('classes-empty', classic, { class: '', cssClasses: '' });
  add('rx', classic, { rx: 5, ry: 5 });
  add('rx-undefined', classic, { rx: undefined, ry: undefined });
  add('padding-8', classic, { padding: 8, label: 'padded' });
  add('label-style', classic, { labelStyle: 'color:red;font-size:18px', label: 'styled' });
  add('label-style-undefined', classic, { labelStyle: undefined, label: 'no style' });
  return cases;
};

shapes.block_arrow = () => {
  const { cases, add } = builder('block_arrow', 'block', { ...blockNode, label: 'Arrow', labelText: 'Arrow', id: 'arrow1', domId: 'my-svg-arrow1', directions: ['right'] });
  standard(add, ['looks', 'svg', 'css', 'padding', 'link']);
  const sets = [
    ['right'], ['left'], ['up'], ['down'], ['x'], ['y'], ['x', 'y'], ['right', 'up'], ['right', 'down'], ['left', 'up'], ['left', 'down'],
    ['x', 'up'], ['x', 'down'], ['right', 'y'], ['left', 'y'], ['down', 'up', 'left', 'right'], ['right', 'right', 'x'], [], ['sideways'], ['up', 'sideways'],
  ];
  sets.forEach((directions, i) => {
    const tag = directions.join('-') || 'none';
    add(`dir-${tag}`, classic, { directions });
    if (i % 3 === 0) {
      add(`dir-${tag}-positioned`, classic, { directions, ...positionedBlock(140, 45) });
    }
  });
  add('dir-undefined', classic, { directions: undefined });
  add('dir-x-svg', svgLabels, { directions: ['x'] });
  add('dir-xy-neo', { look: 'neo', theme: 'redux-color' }, { directions: ['x', 'y'] });
  add('dir-xy-handdrawn', { look: 'handDrawn', theme: 'default' }, { directions: ['x', 'y'] });
  // A positioned arrow takes the width of the layout only when it spans
  // more than one column and the layout width is larger.
  add('span', classic, { directions: ['x'], ...positionedBlock(600, 45), widthInColumns: 10, label: 'span 10' });
  add('span-narrow', classic, { directions: ['x'], ...positionedBlock(40, 45), widthInColumns: 10, label: 'span 10' });
  add('span-one-column', classic, { directions: ['x'], ...positionedBlock(600, 45), widthInColumns: 1 });
  add('span-unpositioned', classic, { directions: ['x'], width: 600, widthInColumns: 10 });
  add('span-no-columns', classic, { directions: ['up'], ...positionedBlock(600, 45), widthInColumns: undefined });
  add('span-down', classic, { directions: ['down'], ...positionedBlock(300, 80), widthInColumns: 3 });
  add('style', classic, { style: 'fill:#f9f;stroke:#333', cssStyles: ['fill:#f9f', 'stroke:#333'] });
  add('style-undefined', classic, { style: undefined });
  add('label-wrap', classic, { label: longLabel, wrappingWidth: 150, directions: ['x'] });
  add('label-empty-y', classic, { label: '', directions: ['y'] });
  add('pad0-xy', classic, { padding: 0, directions: ['x', 'y'] });
  add('pad20-left', classic, { padding: 20, directions: ['left'] });
  add('label-style', classic, { labelStyle: 'color:red;font-size:18px' });
  add('label-style-undefined', classic, { labelStyle: undefined });
  return cases;
};

// The node of a flowchart vertex with an icon, as flowDb.getData() makes
// it. The form of the vertex selects the shape: icon, iconSquare,
// iconRounded or iconCircle.
const iconNode = {
  id: 'B',
  label: 'Notify',
  labelType: 'markdown',
  labelStyle: '',
  padding: 15,
  cssStyles: [],
  cssCompiledStyles: [],
  cssClasses: 'default ',
  domId: 'my-svg-flowchart-B-1',
  icon: 'fa:bell',
  isGroup: false,
  minWidth: 120,
};

// iconCases are the cases of one icon shape. The reference renderer
// registers no icon pack, so the icon is the unknown icon. The four icon
// shapes have the same code but for the box of the icon, so iconRounded,
// which differs from iconSquare in one number and one class, has fewer.
const iconCases = (shape, names) => () => {
  const { cases, add } = builder(shape, 'flowchart-v2', iconNode);
  standard(add, names);
  for (const [tag, directive] of [['html', classic], ['svg', svgLabels]]) {
    add(`pos-t-${tag}`, directive, { pos: 't' });
    add(`pos-b-${tag}`, directive, { pos: 'b' });
    add(`pos-t-no-label-${tag}`, directive, { pos: 't', label: '' });
    add(`no-label-${tag}`, directive, { label: '' });
    add(`size-64-${tag}`, directive, { assetWidth: 64, assetHeight: 64 });
    add(`size-w30-${tag}`, directive, { assetWidth: 30 });
    add(`size-h80-${tag}`, directive, { assetHeight: 80, pos: 't' });
    add(`long-label-t-${tag}`, directive, { label: longLabel, pos: 't' });
  }
  add('pos-t-neo', { look: 'neo', theme: 'redux-color' }, { pos: 't' });
  add('size-wide-neo', { look: 'neo', theme: 'redux-color' }, { assetWidth: 300, assetHeight: 20, label: 'wide' });
  add('pos-t-handdrawn', { look: 'handDrawn', theme: 'default' }, { pos: 't', assetWidth: 24, assetHeight: 24 });
  add('no-icon', classic, { icon: undefined });
  add('no-icon-t', classic, { icon: undefined, pos: 't', label: 'no icon' });
  add('icon-no-prefix', classic, { icon: 'bell' });
  add('icon-bad-name', classic, { icon: 'a:b:c:d' });
  add('fill-stroke', classic, { cssStyles: ['fill:#fc0', 'stroke:#00f'] });
  add('fill-no-value', classic, { cssStyles: ['fill', 'stroke'] });
  add('fill-empty-value', classic, { cssStyles: ['fill:', 'stroke:'] });
  add('compiled-fill', classic, { cssCompiledStyles: ['fill:#0f0', 'stroke:#f0f', 'color:#00f'] });
  add('wrapping-width', { ...classic, flowchart: { wrappingWidth: 60 } }, { label: longLabel });
  add('wrapping-width-large', { ...classic, flowchart: { wrappingWidth: 400 } }, { label: longLabel });
  return cases;
};
const iconGroups = ['looks', 'svg', 'css', 'padding', 'link'];
shapes.icon = iconCases('icon', iconGroups);
shapes.iconSquare = iconCases('iconSquare', iconGroups);
shapes.iconRounded = iconCases('iconRounded', ['looks']);
shapes.iconCircle = iconCases('iconCircle', iconGroups);

// Image sources for the imageSquare shape: data URLs, which need no
// network. The natural size of the image comes from its data.
const svgImage = (attrs) => 'data:image/svg+xml,' + encodeURIComponent(`<svg xmlns="http://www.w3.org/2000/svg" ${attrs}><rect width="10" height="10" fill="#39f"/></svg>`);
const images = {
  svg40x30: svgImage('width="40" height="30"'),
  svg200x80: svgImage('width="200" height="80"'),
  svgViewBox: svgImage('viewBox="0 0 30 60"'),
  svgNoSize: svgImage(''),
  svgZero: svgImage('width="0" height="0"'),
  svgZeroWidth: svgImage('width="0" height="10"'),
  svgZeroHeight: svgImage('width="10" height="0"'),
  gif1x1: 'data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7',
  png1x1: 'data:image/png;base64,iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNkYPhfDwAChwGA60e6kgAAAABJRU5ErkJggg==',
  missing: 'http://mmdg.local/missing.png',
  notImage: 'data:text/plain,hello',
  brokenSvg: 'data:image/svg+xml,' + encodeURIComponent('<svg xmlns="http://www.w3.org/2000/svg"><rect'),
};

shapes.imageSquare = () => {
  // The node of a flowchart vertex with an image, as flowDb.getData()
  // makes it.
  const { cases, add } = builder('imageSquare', 'flowchart-v2', {
    id: 'A',
    label: 'Image label',
    labelType: 'markdown',
    labelStyle: '',
    padding: 15,
    cssStyles: [],
    cssCompiledStyles: [],
    cssClasses: 'default ',
    domId: 'my-svg-flowchart-A-0',
    img: images.svg40x30,
    isGroup: false,
    minWidth: 120,
  });
  standard(add, ['looks', 'svg', 'css', 'link']);
  for (const [tag, directive] of [['html', classic], ['svg', svgLabels]]) {
    add(`pos-t-${tag}`, directive, { pos: 't' });
    add(`no-label-${tag}`, directive, { label: '' });
    add(`no-label-t-${tag}`, directive, { label: '', pos: 't' });
    add(`asset-size-${tag}`, directive, { assetWidth: 60, assetHeight: 60 });
    add(`constraint-${tag}`, directive, { assetWidth: 60, assetHeight: 60, constraint: 'on', pos: 't' });
    add(`long-label-${tag}`, directive, { label: longLabel, pos: 't' });
  }
  add('constraint-no-height', classic, { assetWidth: 90, constraint: 'on' });
  add('constraint-height-only', classic, { assetHeight: 90, constraint: 'on', label: '' });
  add('constraint-off', classic, { assetWidth: 60, assetHeight: 45, constraint: 'off' });
  add('asset-width-only', classic, { assetWidth: 250 });
  add('asset-height-only', classic, { assetHeight: 100, label: '' });
  add('asset-zero', classic, { assetWidth: 0, assetHeight: 0, constraint: 'on' });
  add('wrapping-width', { ...classic, flowchart: { wrappingWidth: 60 } });
  add('wrapping-width-no-label', { ...classic, flowchart: { wrappingWidth: 400 } }, { label: '' });
  for (const [name, img] of Object.entries(images)) {
    add(`img-${name}`, classic, { img });
    add(`img-${name}-constraint`, classic, { img, constraint: 'on', assetHeight: 50, label: '', pos: 't' });
  }
  add('img-empty', classic, { img: '' });
  add('img-undefined', classic, { img: undefined });
  add('handdrawn-large', { look: 'handDrawn', theme: 'default' }, { img: images.svg200x80, pos: 't' });
  return cases;
};

// The node of a C4 element, as buildC4Node of the C4 diagram makes it. A
// node with a stereotype gets its label from c4LabelHelper. The cases use
// shapes of this package that call labelHelper in different ways.
const c4Node = {
  id: 'customerA',
  label: 'Banking Customer A',
  stereotype: '[Person]',
  description: ['A customer of the bank, with personal bank accounts.'],
  labelType: 'string',
  isGroup: false,
  cssClasses: 'c4-shape c4-person',
  cssStyles: ['fill:#08427B', 'stroke:#073B6F', 'color:#FFFFFF'],
  padding: 16,
  useHtmlLabels: false,
  width: 216,
};

// c4LabelCases are the cases of c4LabelHelper. Their names start with
// the shape that calls labelHelper.
const c4LabelCases = () => {
  const all = [];
  const samples = { c4: 'C4Context\n  Person(a, "A")' };
  for (const shape of ['labelRect', 'composite', 'note', 'block_arrow', 'collapsedGroup', 'iconSquare']) {
    const { cases, add } = builder(shape, 'c4', { ...c4Node, ...(shape === 'iconSquare' ? { icon: 'fa:bell' } : {}), ...(shape === 'block_arrow' ? { directions: ['x'] } : {}) });
    for (const look of looks) {
      add(`c4-look-${look}`, { look, theme: 'default' });
    }
    add('c4-redux', { look: 'neo', theme: 'redux-color' });
    add('c4-no-directive', {});
    add('c4-no-wrap', { ...classic, c4: { wrap: false } }, { description: [longLabel] });
    add('c4-wrap', classic, { description: [longLabel, 'second line', '', 'after an empty line'] });
    add('c4-no-width', classic, { width: undefined, description: [longLabel] });
    add('c4-no-width-wrapping', { ...classic, flowchart: { wrappingWidth: 90 } }, { width: undefined, description: [longLabel] });
    add('c4-narrow', classic, { width: 40, padding: 30 });
    add('c4-no-padding', classic, { padding: undefined });
    add('c4-no-description', classic, { description: undefined, stereotype: '[Software System]' });
    add('c4-empty-stereotype', classic, { stereotype: '' });
    add('c4-no-name', classic, { label: '' });
    add('c4-name-undefined', classic, { label: undefined, description: [] });
    add('c4-nothing', classic, { label: '', stereotype: '', description: [] });
    add('c4-entities', classic, { label: 'a &amp; b <b>c</b> #quot;d#quot;', stereotype: '[Container: Java, Spring MVC]', description: ['x<br/>y', 'fa:fa-bell z'] });
    add('c4-label-style', classic, { labelStyle: 'color:red;font-size:18px', cssStyles: ['color:red', 'font-size:18px', 'fill:#ff0'] });
    add('c4-html-config', { ...classic, htmlLabels: true }, { useHtmlLabels: true });
    add('c4-flowchart-type', classic, {}, { type: 'flowchart-v2' });
    add('c4-font', { ...classic, fontFamily: 'courier', themeVariables: { fontSize: '20px' } });
    all.push(...cases.map((c) => ({ ...c, name: c.name.replace(`${shape}/`, `c4LabelHelper/${shape}-`) })));
  }
  return { cases: all, samples };
};

shapes.c4LabelHelper = c4LabelCases;

// The cases of these shapes have values that JSON cannot hold (Infinity)
// or need a set-up in the page, so they run with a preamble in front of
// probes/shapes.js.
const needsPreamble = new Set(['composite', 'block_arrow']);

// The preamble turns the text 'Infinity' of a node property into the
// number, and registers the icon packs of the cases.
const preamble = `
for (const c of args.cases) {
  for (const [key, value] of Object.entries(c.node)) {
    if (value === 'Infinity') {
      c.node[key] = Infinity;
    }
  }
}
if (args.iconPacks) {
  mermaid.registerIconPacks(args.iconPacks);
}
`;

// runWithPreamble records cases as gen-shapes.mjs does, with the preamble
// in front of the probe. The golden file has the same form.
const runWithPreamble = (name, cases, extra = {}) => {
  const tmpScript = path.join(os.tmpdir(), `mmdg-set3-probe-${process.pid}.js`);
  const tmpIn = path.join(os.tmpdir(), `mmdg-set3-in-${process.pid}.json`);
  const tmpOut = path.join(os.tmpdir(), `mmdg-set3-out-${process.pid}.json`);
  fs.writeFileSync(tmpScript, preamble + fs.readFileSync(path.join(here, 'probes/shapes.js'), 'utf8'));
  fs.writeFileSync(tmpIn, JSON.stringify({ cases, ...extra }));
  try {
    execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', tmpScript, '--args', tmpIn, '--out', tmpOut], { stdio: 'inherit' });
    const results = JSON.parse(fs.readFileSync(tmpOut, 'utf8'));
    const golden = cases.map((c, i) => ({ ...c, directive: undefined, ...results[i] }));
    const outFile = path.join(outDir, `${name}.json.gz`);
    fs.mkdirSync(outDir, { recursive: true });
    fs.writeFileSync(outFile, zlib.gzipSync(JSON.stringify(golden)));
    console.error(`${golden.length} cases, ${golden.filter((g) => g.error).length} errors -> ${outFile}`);
  } finally {
    for (const f of [tmpScript, tmpIn, tmpOut]) {
      fs.rmSync(f, { force: true });
    }
  }
};

// A small icon pack for the cases with registered icons. The icons have
// the forms that change the size and the place of the icon in the shape:
// a view box that is not square, an offset, transformations, and ids.
const testIconPack = {
  prefix: 'tp',
  width: 24,
  height: 24,
  icons: {
    box: { body: '<path d="M4 4h16v16H4z" fill="currentColor"/>' },
    full: { body: '<rect width="24" height="24" fill="currentColor"/>' },
    wide: { body: '<path d="M2 3h44v10H2z"/>', width: 48, height: 16 },
    tall: { body: '<circle cx="5" cy="15" r="4"/><rect x="1" y="1" width="8" height="6"/>', width: 10, height: 30 },
    offset: { body: '<rect x="-4" y="2" width="20" height="12"/>', left: -4, top: 2, width: 20, height: 12 },
    rotated: { body: '<path d="M1 2h20v4H1z"/>', rotate: 1, width: 30, height: 10 },
    flipped: { body: '<path d="M1 2h6v9z"/><defs><path id="unused" d="M0 0"/></defs>', hFlip: true },
    gradient: {
      body: '<defs><linearGradient id="a" x1="0" x2="1"><stop offset="0" stop-color="#f00"/><stop offset="1" stop-color="#00f"/></linearGradient></defs><path fill="url(#a)" d="M0 0h24v24H0z"/>',
    },
    curve: { body: '<path d="M224 512c35.32 0 63.97-28.65 63.97-64H160.03c0 35.35 28.65 64 63.97 64zm215.39-149.71c-19.32-20.76-55.47-51.99-55.47-154.29 0-77.7-54.48-139.9-127.94-155.16V32c0-17.67-14.32-32-31.98-32s-31.98 14.33-31.98 32v20.84C118.56 68.1 64.08 130.3 64.08 208c0 102.3-36.15 133.53-55.47 154.29-6 6.45-8.66 14.16-8.61 21.71.11 16.4 12.98 32 32.1 32h383.8c19.12 0 32-15.6 32.1-32 .05-7.55-2.61-15.27-8.61-21.71z"/>', width: 448, height: 512 },
    empty: { body: '' },
  },
  aliases: {
    turned: { parent: 'wide', rotate: 1 },
  },
};
const testIconPacks = [
  { name: 'tp', icons: testIconPack },
  { name: 'fa', icons: { prefix: 'fa', icons: { bell: testIconPack.icons.curve } } },
];

// registeredIconCases are cases of the four icon shapes with registered
// icon packs. They run with the preamble, which registers the packs in the
// page. The golden file is in testdata/registered, with the packs beside
// it: the test registers the same packs.
const registeredIconCases = () => {
  const all = [];
  const names = [...Object.keys(testIconPack.icons), 'turned', 'missing'];
  for (const shape of ['icon', 'iconSquare', 'iconRounded', 'iconCircle']) {
    const { cases, add } = builder(shape, 'flowchart-v2', iconNode);
    // Every icon for the square and the circle, some for the others.
    const full = shape === 'iconSquare' || shape === 'iconCircle';
    names.forEach((name, i) => {
      if (full || i % 3 === 0) {
        add(`tp-${name}`, classic, { icon: `tp:${name}` });
      }
      if (full && i % 2 === 0) {
        add(`tp-${name}-t`, { look: 'neo', theme: 'redux-color' }, { icon: `tp:${name}`, pos: 't' });
      }
    });
    add('fa-bell', classic, { icon: 'fa:bell' });
    add('fa-bell-handdrawn', { look: 'handDrawn', theme: 'default' }, { icon: 'fa:bell', assetWidth: 24, assetHeight: 24 });
    add('fa-bell-svg', svgLabels, { icon: 'fa:bell', pos: 't' });
    add('fa-bell-size', classic, { icon: 'fa:bell', assetWidth: 64, assetHeight: 20 });
    add('fa-bell-no-label', classic, { icon: 'fa:bell', label: '' });
    add('fa-bell-stroke', classic, { icon: 'fa:bell', cssStyles: ['stroke:#f00', 'fill:#ff0'] });
    add('dash-name', classic, { icon: 'tp-wide' });
    add('no-prefix', classic, { icon: 'box' });
    add('label-icon-svg', svgLabels, { icon: 'tp:box', label: 'fa:fa-bell Ring' });
    add('loose', { ...classic, securityLevel: 'loose' }, { icon: 'tp:gradient' });
    all.push(...cases.map((c) => ({ ...c, name: c.name.replace(`${shape}/`, `${shape}/registered-`) })));
  }
  return all;
};

// labelIconCases are cases with a registered icon in the text of an HTML
// label: createText puts the svg element of the icon in the label. They
// are in a file of their own, because the layout engine of the Go
// renderer does not give an svg element in an HTML label its size yet.
const labelIconCases = () => {
  const all = [];
  for (const shape of ['icon', 'iconSquare', 'iconRounded', 'iconCircle', 'labelRect', 'note']) {
    const base = shape === 'note' ? { ...stateNode, cssClasses: 'statediagram-note', padding: 15 } : iconNode;
    const { cases, add } = builder(shape, 'flowchart-v2', { ...base, icon: shape.startsWith('icon') ? 'tp:box' : undefined });
    add('label-icon', classic, { label: 'fa:fa-bell Ring fa:fa-nothere' });
    add('label-icon-only', classic, { label: 'fa:fa-bell' });
    add('label-icon-two', { look: 'neo', theme: 'redux-color' }, { label: 'Ring fa:fa-bell twice fa:fa-bell' });
    all.push(...cases.map((c) => ({ ...c, name: c.name.replace(`${shape}/`, `${shape}/registered-`) })));
  }
  return all;
};

// run records the cases of one shape with gen-shapes.mjs.
const run = (shape) => {
  const made = shapes[shape]();
  const cases = Array.isArray(made) ? made : made.cases;
  const names = new Set(cases.map((c) => c.name));
  if (names.size !== cases.length) {
    throw new Error(`${shape}: duplicate case names`);
  }
  if (needsPreamble.has(shape)) {
    runWithPreamble(shape, cases);
    return;
  }
  const tmp = path.join(os.tmpdir(), `mmdg-set3-cases-${process.pid}-${shape}.json`);
  fs.writeFileSync(tmp, JSON.stringify({ cases, samples: made.samples }));
  try {
    execFileSync('node', [path.join(here, 'gen-shapes.mjs'), tmp, path.join(outDir, `${shape}.json.gz`)], { stdio: 'inherit' });
  } finally {
    fs.rmSync(tmp);
  }
};

// runRegistered records the cases with registered icon packs.
const runRegistered = () => {
  fs.mkdirSync(path.join(outDir, 'registered'), { recursive: true });
  fs.writeFileSync(path.join(outDir, 'registered/icon-packs.json'), JSON.stringify(testIconPacks, null, 1) + '\n');
  runWithPreamble('registered/icons', registeredIconCases(), { iconPacks: testIconPacks });
  runWithPreamble('registered/label-icons', labelIconCases(), { iconPacks: testIconPacks });
};

const wanted = process.argv.slice(2);
for (const shape of wanted.length > 0 ? wanted : [...Object.keys(shapes), 'registered']) {
  if (shape === 'registered') {
    runRegistered();
    continue;
  }
  if (!shapes[shape]) {
    console.error(`unknown shape ${shape}; known: ${Object.keys(shapes).join(', ')}`);
    process.exit(2);
  }
  run(shape);
}
