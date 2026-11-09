// Writes internal/browser/testdata/linux-libm.json.gz: results of sinf,
// cosf, tanf and atan2f of the C library of the reference Linux environment
// (glibc 2.39 for arm64 in the Playwright image). Skia and Blink call these
// functions for arcs and for points on a path, and glibc does not always
// round to the nearest float32. internal/browser/linux_trig.go computes the
// same results, and its test compares every bit with this file.
//
// The script runs python3 in the image, which calls the functions through
// ctypes. It starts no browser.
//
// Usage (in tools/reference): node gen-browser-linux-libm.mjs
import { execFileSync } from 'node:child_process';
import fs from 'node:fs';
import path from 'node:path';
import zlib from 'node:zlib';
import { fileURLToPath } from 'node:url';

const here = path.dirname(fileURLToPath(import.meta.url));
const repoRoot = path.resolve(here, '../..');
const pkg = JSON.parse(fs.readFileSync(path.join(here, 'node_modules/playwright/package.json'), 'utf8'));
const image = `mcr.microsoft.com/playwright:v${pkg.version}-noble`;

const python = `
import ctypes, json, random, struct, sys
m = ctypes.CDLL("libm.so.6")
for name in ("sinf", "cosf", "tanf"):
    f = getattr(m, name)
    f.restype = ctypes.c_float
    f.argtypes = [ctypes.c_float]
m.atan2f.restype = ctypes.c_float
m.atan2f.argtypes = [ctypes.c_float, ctypes.c_float]
def bits(x):
    return struct.unpack(">I", struct.pack(">f", x))[0]
def f32(x):
    return struct.unpack(">f", struct.pack(">f", x))[0]
rnd = random.Random(0x6c69626d)
def angles():
    out = [0.0, -0.0, 1e-30, -1e-30, 1e-10, 0.25, 0.5, 0.6744, 0.675, 0.7853981, 0.7853982, 1.0, 1.5707963, 1.5707964,
           3.1415925, 3.1415927, 3.1415929, 6.2831855, 100.0, 119.99, 120.0, 120.01, 1e5, 3e7, 1e20, -1e20]
    for _ in range(6000):
        out.append(rnd.uniform(-6.2831855, 6.2831855))
    for _ in range(3000):
        out.append(rnd.uniform(-0.8, 0.8))
    for _ in range(1500):
        out.append(rnd.uniform(-130.0, 130.0))
    for _ in range(1000):
        out.append(rnd.uniform(-1.0, 1.0) * 10 ** rnd.uniform(-12, 9))
    for d in range(-720, 721):
        out.append(d * 3.14159265358979323846 / 180)
        out.append(f32(f32(d) * f32(0.017453292519943295)))
    return [f32(v) for v in out]
res = {}
for name in ("sinf", "cosf", "tanf"):
    f = getattr(m, name)
    res[name] = [[bits(v), bits(f(v))] for v in angles()]
pairs = [(0.0, 1.0), (0.0, -1.0), (-0.0, -1.0), (1.0, 0.0), (-1.0, 0.0), (1.0, 1.0), (1.0, -1.0), (1e-30, 1.0), (1.0, 1e-30), (1e30, 1.0)]
for _ in range(9000):
    a = rnd.uniform(-3.1415927, 3.1415927)
    r = 10 ** rnd.uniform(-3, 3)
    import math
    pairs.append((r * math.sin(a), r * math.cos(a)))
for _ in range(3000):
    pairs.append((rnd.uniform(-1, 1), rnd.uniform(-1, 1)))
for _ in range(1000):
    pairs.append((rnd.uniform(-1, 1) * 10 ** rnd.uniform(-20, 20), rnd.uniform(-1, 1) * 10 ** rnd.uniform(-20, 20)))
res["atan2f"] = [[bits(f32(y)), bits(f32(x)), bits(m.atan2f(f32(y), f32(x)))] for (y, x) in pairs]
json.dump(res, sys.stdout)
`;

const out = execFileSync('docker', ['run', '--rm', '-i', image, 'python3', '-'], { input: python, maxBuffer: 1 << 28 });
const data = JSON.parse(out.toString());
const file = path.join(repoRoot, 'internal/browser/testdata/linux-libm.json.gz');
fs.writeFileSync(file, zlib.gzipSync(JSON.stringify(data), { level: 9 }));
console.error(`${file}: ${Object.entries(data).map(([k, v]) => `${k} ${v.length}`).join(', ')}`);
