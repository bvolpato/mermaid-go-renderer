// Records the natural size of images from Chromium and writes the golden
// file of internal/ru/shapes/set3/imagesize_test.go.
//
//   node gen-image-size.mjs [golden.json.gz]
//
// See probes/image-size.js: the page makes the image sources and gives,
// for each one, the natural size or the error of img.decode().
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import os from 'node:os';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const outFile = process.argv[2] || path.join(here, '../../internal/ru/shapes/set3/testdata/images/sizes.json.gz');
const tmp = path.join(os.tmpdir(), `mmdg-image-size-${process.pid}.json`);
execFileSync('node', [path.join(here, 'oracle.mjs'), 'esm-eval', path.join(here, 'probes/image-size.js'), '--out', tmp], { stdio: 'inherit' });
const results = JSON.parse(fs.readFileSync(tmp, 'utf8'));
fs.rmSync(tmp);
fs.mkdirSync(path.dirname(outFile), { recursive: true });
const json = JSON.stringify(results);
fs.writeFileSync(outFile, outFile.endsWith('.gz') ? zlib.gzipSync(json) : json);
console.error(`${results.length} images, ${results.filter((r) => r.error).length} errors -> ${outFile}`);
