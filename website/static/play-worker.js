// The worker of the playground. It loads the renderer, which is the Go code
// of the module compiled to WebAssembly (cmd/wasm), and renders the diagrams
// that the page sends. The page stays responsive while the module loads.

importScripts('play/wasm_exec.js');

const started = performance.now();

async function load() {
  const response = await fetch('play/mmdg.wasm.gz');
  if (!response.ok) throw new Error(`HTTP ${response.status}`);
  const total = Number(response.headers.get('content-length')) || 0;
  const reader = response.body.getReader();
  const chunks = [];
  let received = 0;
  let last = -1;
  for (;;) {
    const { done, value } = await reader.read();
    if (done) break;
    chunks.push(value);
    received += value.length;
    const percent = total ? Math.min(99, Math.floor((100 * received) / total)) : 0;
    if (percent !== last) {
      last = percent;
      postMessage({ type: 'progress', percent });
    }
  }
  let blob = new Blob(chunks);
  // The file is gzip. A server can also send it with a content encoding,
  // and then the browser has expanded it already.
  const head = new Uint8Array(await blob.slice(0, 2).arrayBuffer());
  if (head[0] === 0x1f && head[1] === 0x8b) {
    blob = await new Response(blob.stream().pipeThrough(new DecompressionStream('gzip'))).blob();
  }
  const go = new Go();
  const { instance } = await WebAssembly.instantiate(await blob.arrayBuffer(), go.importObject);
  go.run(instance);
  if (typeof self.mmdgRender !== 'function') throw new Error('the module did not start');
}

const loaded = load().then(
  () => postMessage({ type: 'ready', ms: performance.now() - started }),
  (error) => postMessage({ type: 'failed', error: String((error && error.message) || error) })
);

addEventListener('message', async ({ data }) => {
  await loaded;
  if (typeof self.mmdgRender !== 'function') return;
  const t0 = performance.now();
  const result = self.mmdgRender(data.src, data.page);
  postMessage({ type: 'result', id: data.id, ms: performance.now() - t0, error: result.error, svg: result.svg, diagramType: result.type });
});
