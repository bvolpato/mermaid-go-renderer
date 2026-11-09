// The scripts of the page: the depth gauge, the charts and the playground.
// The data of the charts is in script elements of the page, which the site
// generator writes (tools/sitegen).

(() => {
  'use strict';

  const $ = (sel, root = document) => root.querySelector(sel);
  const $$ = (sel, root = document) => Array.from(root.querySelectorAll(sel));
  const json = (id) => JSON.parse(document.getElementById(id).textContent);
  const css = (name) => getComputedStyle(document.documentElement).getPropertyValue(name).trim();
  const reduced = matchMedia('(prefers-reduced-motion: reduce)').matches;

  const fmtMs = (v) => (v >= 100 ? v.toFixed(0) : v >= 10 ? v.toFixed(1) : v.toFixed(2));

  // ---- Labels in another browser ----

  // A diagram has HTML labels in boxes of a fixed width: the width that the
  // renderer measured, with the rules of Chromium on Linux. A browser on
  // another platform can measure the same text a little wider. A label of
  // one line then wraps, and its second line is cut. This keeps such a
  // label on one line. mermaid in a browser has no such case, because it
  // measures and shows in the same browser.
  function fitLabels(root) {
    for (const fo of root.querySelectorAll('foreignObject')) {
      const box = fo.firstElementChild;
      if (!(box instanceof HTMLElement)) continue;
      const height = fo.height.baseVal.value;
      if (box.offsetHeight <= height + 1) continue;
      const line = parseFloat(getComputedStyle(box).lineHeight) || 21;
      if (height <= line * 1.2) box.style.whiteSpace = 'nowrap';
      fo.style.overflow = 'visible';
    }
  }

  // ---- Reveal on scroll ----

  const revealTargets = $$('.panel, .tile, .card, .features article, .kpis, .pipeline');
  if ('IntersectionObserver' in window && !reduced) {
    const io = new IntersectionObserver(
      (entries) => {
        for (const e of entries) {
          if (e.isIntersecting) {
            e.target.classList.add('in');
            io.unobserve(e.target);
          }
        }
      },
      { rootMargin: '0px 0px -8% 0px' }
    );
    for (const el of revealTargets) {
      el.classList.add('reveal');
      io.observe(el);
    }
  } else {
    for (const el of revealTargets) el.classList.add('in');
  }

  // ---- The depth gauge: the scroll position as a depth in meters ----

  const depthFill = $('#depth-fill');
  const depthValue = $('#depth-value');
  const onScroll = () => {
    const max = document.documentElement.scrollHeight - innerHeight;
    const p = max > 0 ? Math.min(1, Math.max(0, scrollY / max)) : 0;
    depthFill.style.height = `${(p * 100).toFixed(1)}%`;
    depthValue.textContent = Math.round(p * 4000).toLocaleString('en-US');
  };
  addEventListener('scroll', onScroll, { passive: true });
  onScroll();

  // ---- A tooltip that follows a mark ----

  function showTip(tip, host, x, y, build) {
    tip.textContent = '';
    build(tip);
    tip.classList.add('on');
    const w = tip.offsetWidth;
    const h = tip.offsetHeight;
    const left = Math.min(Math.max(0, x - w / 2), host.clientWidth - w);
    const top = y - h - 12 < 0 ? y + 16 : y - h - 12;
    tip.style.left = `${left}px`;
    tip.style.top = `${top}px`;
  }
  const hideTip = (tip) => tip.classList.remove('on');

  function tipLine(tip, color, value, label) {
    const row = document.createElement('div');
    const key = document.createElement('span');
    key.className = 'key';
    key.style.background = color;
    const strong = document.createElement('b');
    strong.textContent = value;
    strong.style.color = css('--ink-0');
    row.append(key, strong, document.createTextNode(` ${label}`));
    tip.append(row);
  }

  // ---- The unit chart: one square for each diagram ----

  const parity = json('parity-data');
  const grid = $('#unit-grid');
  const gridTip = $('#unit-tip');
  const gridState = { set: 'plain', cols: 0, pitch: 0, cell: 0, hover: -1 };
  const statusText = {
    e: 'Same bytes as the reference',
    s: 'The reference fails, and mmdg fails too',
    x: 'Not supported',
  };

  const currentSet = () => parity.sets.find((s) => s.id === gridState.set);
  const e2e = () => currentSet().corpora[0];
  const names = () => parity.fixtures[e2e().corpus];

  function drawGrid() {
    const corpus = e2e();
    const n = corpus.total;
    const width = grid.parentElement.clientWidth;
    const gap = 2;
    const cell = width < 520 ? 6 : width < 760 ? 8 : 9;
    const pitch = cell + gap;
    const cols = Math.max(1, Math.floor((width + gap) / pitch));
    const rows = Math.ceil(n / cols);
    const dpr = window.devicePixelRatio || 1;
    const height = rows * pitch - gap;
    grid.width = Math.round(width * dpr);
    grid.height = Math.round(height * dpr);
    grid.style.height = `${height}px`;
    Object.assign(gridState, { cols, pitch, cell });
    const ctx = grid.getContext('2d');
    ctx.setTransform(dpr, 0, 0, dpr, 0, 0);
    ctx.clearRect(0, 0, width, height);
    const colors = { e: css('--mark-aqua'), s: css('--mark-blue'), x: css('--mark-orange') };
    for (let i = 0; i < n; i++) {
      ctx.fillStyle = colors[corpus.status[i]];
      ctx.fillRect((i % cols) * pitch, Math.floor(i / cols) * pitch, cell, cell);
    }
    if (gridState.hover >= 0 && gridState.hover < n) {
      const i = gridState.hover;
      ctx.strokeStyle = css('--ink-0');
      ctx.lineWidth = 2;
      ctx.strokeRect((i % cols) * pitch - 1, Math.floor(i / cols) * pitch - 1, cell + 2, cell + 2);
    }
  }

  function gridIndex(event) {
    const rect = grid.getBoundingClientRect();
    const x = event.clientX - rect.left;
    const y = event.clientY - rect.top;
    const col = Math.floor(x / gridState.pitch);
    const row = Math.floor(y / gridState.pitch);
    if (col < 0 || col >= gridState.cols || row < 0) return -1;
    const i = row * gridState.cols + col;
    return i < e2e().total ? i : -1;
  }

  grid.addEventListener('pointermove', (event) => {
    const i = gridIndex(event);
    if (i === gridState.hover) return;
    gridState.hover = i;
    drawGrid();
    if (i < 0) return hideTip(gridTip);
    const corpus = e2e();
    const st = corpus.status[i];
    const fixtures = names();
    const x = (i % gridState.cols) * gridState.pitch + gridState.cell / 2;
    const y = Math.floor(i / gridState.cols) * gridState.pitch;
    showTip(gridTip, grid.parentElement, x, y, (tip) => {
      const title = document.createElement('strong');
      title.textContent = st === 'x' && corpus.reasons ? corpus.reasons[i] || statusText.x : statusText[st];
      const name = document.createElement('div');
      name.textContent = fixtures.names[i];
      const type = document.createElement('div');
      type.textContent = fixtures.types[i] ? `Type: ${fixtures.types[i]}` : 'mermaid rejects this diagram';
      tip.append(title, name, type);
    });
  });
  grid.addEventListener('pointerleave', () => {
    gridState.hover = -1;
    drawGrid();
    hideTip(gridTip);
  });

  function showSet(id) {
    gridState.set = id;
    gridState.hover = -1;
    const set = currentSet();
    for (const tab of $$('.seg [data-set]')) tab.setAttribute('aria-selected', String(tab.dataset.set === id));
    $('#set-note').textContent = set.note;
    const corpus = set.corpora[0];
    for (const b of $$('#unit-legend [data-k]')) b.textContent = corpus[b.dataset.k];
    const tiles = $('#corpus-tiles');
    tiles.textContent = '';
    for (const c of set.corpora) {
      const pass = c.exact + c.sameError;
      const tile = document.createElement('div');
      tile.className = 'tile in';
      const label = document.createElement('p');
      label.className = 'tile-label';
      label.textContent = c.label;
      const value = document.createElement('p');
      value.className = 'tile-value';
      value.textContent = pass;
      const small = document.createElement('small');
      small.textContent = ` of ${c.total}`;
      value.append(small);
      const meter = document.createElement('div');
      meter.className = 'meter';
      meter.setAttribute('role', 'img');
      meter.setAttribute('aria-label', `${((100 * pass) / c.total).toFixed(1)} percent pass`);
      const fill = document.createElement('span');
      fill.style.width = `${(100 * pass) / c.total}%`;
      meter.append(fill);
      tile.append(label, value, meter);
      tiles.append(tile);
    }
    grid.setAttribute('aria-label', `One square for each of the ${corpus.total} diagrams. ${corpus.exact} have the same bytes as the reference.`);
    drawGrid();
  }
  for (const tab of $$('.seg [data-set]')) tab.addEventListener('click', () => showSet(tab.dataset.set));

  // ---- The dumbbell chart: render time by diagram type ----

  const perf = json('perf-data');
  const dumbbell = $('#dumbbell');
  const dumbbellTip = $('#dumbbell-tip');
  const NS = 'http://www.w3.org/2000/svg';
  const el = (name, attrs = {}, text) => {
    const node = document.createElementNS(NS, name);
    for (const [k, v] of Object.entries(attrs)) node.setAttribute(k, v);
    if (text !== undefined) node.textContent = text;
    return node;
  };

  function drawDumbbell() {
    const types = [...perf.types].sort((a, b) => b.diagrams - a.diagrams).slice(0, 18);
    const width = dumbbell.parentElement.clientWidth;
    const narrow = width < 560;
    const left = narrow ? 104 : 132;
    const right = 52;
    const top = 26;
    const rowH = 26;
    const height = top + types.length * rowH + 8;
    dumbbell.textContent = '';
    dumbbell.setAttribute('viewBox', `0 0 ${width} ${height}`);
    dumbbell.style.height = `${height}px`;
    const lo = Math.log10(0.04);
    const hi = Math.log10(60);
    const x = (v) => left + ((Math.log10(v) - lo) / (hi - lo)) * (width - left - right);
    for (const tick of [0.1, 1, 10]) {
      dumbbell.append(el('line', { class: 'gridline', x1: x(tick), x2: x(tick), y1: top - 6, y2: height - 6 }));
      dumbbell.append(el('text', { class: 'tick', x: x(tick), y: 12, 'text-anchor': 'middle' }, `${tick} ms`));
    }
    types.forEach((t, i) => {
      const y = top + i * rowH + rowH / 2;
      const row = el('g', { class: 'row', tabindex: 0, role: 'img', 'aria-label': `${t.type}: mermaid ${fmtMs(t.mermaid)} ms, mmdg ${fmtMs(t.mmdg)} ms` });
      row.append(el('rect', { class: 'hit', x: 0, y: y - rowH / 2, width, height: rowH }));
      row.append(el('text', { x: left - 14, y: y + 4, 'text-anchor': 'end' }, t.type));
      row.append(el('line', { class: 'stem', x1: x(t.mmdg), x2: x(t.mermaid), y1: y, y2: y }));
      row.append(el('circle', { class: 'pt', cx: x(t.mermaid), cy: y, r: 6, fill: css('--mark-gray') }));
      row.append(el('circle', { class: 'pt', cx: x(t.mmdg), cy: y, r: 6, fill: css('--mark-aqua') }));
      row.append(el('text', { class: 'tick', x: width - 4, y: y + 4, 'text-anchor': 'end' }, `${Math.round(t.mermaid / t.mmdg)}×`));
      const show = () =>
        showTip(dumbbellTip, dumbbell.parentElement, (x(t.mmdg) + x(t.mermaid)) / 2, y - rowH / 2, (tip) => {
          const title = document.createElement('strong');
          title.textContent = `${t.type}, ${t.diagrams} diagrams`;
          tip.append(title);
          tipLine(tip, css('--mark-gray'), `${fmtMs(t.mermaid)} ms`, 'mermaid in Chromium');
          tipLine(tip, css('--mark-aqua'), `${fmtMs(t.mmdg)} ms`, 'mmdg');
        });
      row.addEventListener('pointerenter', show);
      row.addEventListener('focus', show);
      row.addEventListener('pointerleave', () => hideTip(dumbbellTip));
      row.addEventListener('blur', () => hideTip(dumbbellTip));
      dumbbell.append(row);
    });
  }

  // ---- The tooltips of the two bar pairs ----

  const barTip = document.createElement('div');
  barTip.className = 'tip';
  barTip.style.position = 'fixed';
  document.body.append(barTip);
  for (const bar of $$('.bar[data-tip]')) {
    const show = () => {
      const r = bar.getBoundingClientRect();
      barTip.textContent = bar.dataset.tip;
      barTip.classList.add('on');
      const w = barTip.offsetWidth;
      barTip.style.left = `${Math.min(Math.max(8, r.left), innerWidth - w - 8)}px`;
      barTip.style.top = `${r.top - barTip.offsetHeight - 10}px`;
    };
    bar.addEventListener('pointerenter', show);
    bar.addEventListener('focus', show);
    bar.addEventListener('pointerleave', () => hideTip(barTip));
    bar.addEventListener('blur', () => hideTip(barTip));
  }

  // ---- Copy buttons ----

  for (const button of $$('.copy')) {
    button.addEventListener('click', async () => {
      const text = $(button.dataset.copy).textContent;
      try {
        await navigator.clipboard.writeText(text);
        button.textContent = 'Copied';
        button.classList.add('done');
        setTimeout(() => {
          button.textContent = 'Copy';
          button.classList.remove('done');
        }, 1600);
      } catch {
        button.textContent = 'Press Ctrl+C';
      }
    });
  }

  // ---- The playground ----

  const sources = json('gallery-data');
  const src = $('#play-src');
  const out = $('#play-out');
  const status = $('#play-status');
  const loadButton = $('#play-load');
  const example = $('#play-example');
  const page = $('#play-page');
  let worker = null;
  let ready = false;
  let pending = 0;
  let timer = 0;

  const setStatus = (text, error = false) => {
    status.textContent = text;
    status.classList.toggle('err', error);
  };

  function render() {
    if (!ready) return;
    pending++;
    worker.postMessage({ id: pending, src: src.value, page: page.value });
  }

  function loadRenderer() {
    if (worker) return;
    loadButton.disabled = true;
    loadButton.textContent = 'Loading…';
    // Icons such as fa:fa-car need the style sheets of Font Awesome.
    for (const sheet of ['brands', 'regular', 'solid', 'fontawesome']) {
      const link = document.createElement('link');
      link.rel = 'stylesheet';
      link.href = `fonts/fontawesome-free/css/${sheet}.css`;
      document.head.append(link);
    }
    worker = new Worker('play-worker.js');
    worker.addEventListener('message', ({ data }) => {
      if (data.type === 'progress') {
        setStatus(`Loading the renderer: ${data.percent}%`);
      } else if (data.type === 'ready') {
        ready = true;
        loadButton.hidden = true;
        setStatus(`Ready in ${(data.ms / 1000).toFixed(1)} s. Edit the diagram.`);
        render();
      } else if (data.type === 'failed') {
        setStatus(`The renderer did not load: ${data.error}`, true);
        loadButton.disabled = false;
        loadButton.textContent = 'Try again';
        worker = null;
      } else if (data.type === 'result' && data.id === pending) {
        if (data.error) {
          setStatus(data.error, true);
        } else {
          out.innerHTML = data.svg;
          fitLabels(out);
          setStatus(`${data.diagramType} rendered in ${fmtMs(data.ms)} ms, in your browser`);
        }
      }
    });
  }

  loadButton.addEventListener('click', loadRenderer);
  src.addEventListener('input', () => {
    clearTimeout(timer);
    if (!ready) return setStatus('Load the renderer to render your changes.');
    timer = setTimeout(render, 180);
  });
  page.addEventListener('change', render);

  function openExample(name, scroll) {
    if (!(name in sources)) return;
    example.value = name;
    src.value = sources[name];
    if (ready) {
      render();
    } else {
      // The page has this diagram already, rendered when the site was built.
      const built = document.getElementById(`d-${name}`);
      if (built) {
        out.textContent = '';
        out.append(built.cloneNode(true));
        fitLabels(out);
      }
    }
    if (scroll) $('#play').scrollIntoView({ behavior: reduced ? 'auto' : 'smooth' });
  }
  example.addEventListener('change', () => openExample(example.value, false));
  for (const button of $$('[data-play]')) button.addEventListener('click', () => openExample(button.dataset.play, true));

  // ---- First paint and resize ----

  let resizeTimer = 0;
  const drawAll = () => {
    drawGrid();
    drawDumbbell();
  };
  addEventListener('resize', () => {
    clearTimeout(resizeTimer);
    resizeTimer = setTimeout(drawAll, 120);
  });
  const fitAll = () => {
    for (const paper of $$('.paper')) fitLabels(paper);
  };
  if (document.fonts && document.fonts.ready) {
    document.fonts.ready.then(() => {
      drawAll();
      fitAll();
    });
  }
  drawAll();
  fitAll();
})();
