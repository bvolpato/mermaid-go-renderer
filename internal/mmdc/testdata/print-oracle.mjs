// Records print geometry from the installed mermaid-cli and Puppeteer.
// The historical print.json used mermaid-cli 12.0.0 and Puppeteer 25.16.0.
// Run from the repository root: node internal/mmdc/testdata/print-oracle.mjs.
// mmdc-run.mjs holds the oracle lock and launches one headless browser.
import fs from 'node:fs/promises';
import os from 'node:os';
import path from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawnSync } from 'node:child_process';
import { createRequire } from 'node:module';

const here = path.dirname(fileURLToPath(import.meta.url));
const reference = path.resolve(here, '../../../tools/reference');
const cli = path.join(reference, 'node_modules/@mermaid-js/mermaid-cli/src/index.js');
const cliRequire = createRequire(cli);
const versionFrom = async file => JSON.parse(await fs.readFile(file, 'utf8')).version;
const versions = {
  mmdc: await versionFrom(path.resolve(path.dirname(cli), '../package.json')),
  mermaid: await versionFrom(cliRequire.resolve('mermaid/package.json')),
  puppeteer: await versionFrom(cliRequire.resolve('puppeteer/package.json')),
};
const scratch = await fs.mkdtemp(path.join(os.tmpdir(), 'mmdc-print-'));
const hook = `
import fs from 'node:fs/promises';
import zlib from 'node:zlib';
import { CdpPage } from ${JSON.stringify(new URL('node_modules/puppeteer-core/lib/puppeteer/cdp/Page.js', 'file://' + reference + '/').href)};
import { parsePDFOptions } from ${JSON.stringify(new URL('node_modules/puppeteer-core/lib/puppeteer/common/util.js', 'file://' + reference + '/').href)};
const pdf = CdpPage.prototype.pdf;
CdpPage.prototype.pdf = async function(options) {
  const svg = await this.$eval('svg', svg => svg.outerHTML);
  const result = await pdf.call(this, options);
  const sampleRect = await this.$eval('svg', svg => svg.getBoundingClientRect().toJSON());
  const sampleMedia = Buffer.from(result).toString('latin1').match(/\\/MediaBox\\s*\\[([^\\]]+)/)[1].trim().split(/\\s+/).map(Number);
  const records = [{...${JSON.stringify(versions)}, name: 'real mmdc flowchart', svg, size: 0, format: '', viewport: 800,
    width: Math.fround(sampleMedia[2])*96/72, height: Math.fround(sampleMedia[3])*96/72,
    box: sampleRect, pages: 1, mediaBox: sampleMedia, browser: await this.browser().version()}];
  const cases = [
    ['small', 358, 70], ['tall', 168, 504], ['wide', 1896, 64],
    ['fixed wide', 1896, 64, false], ['fractional', 358.3125, 70.75],
    ['narrow viewport', 1896, 64, true, 0, '', 400],
    ['mixed case format', 1896, 2000, true, 0, 'a4'],
    ['A1 layout rounding', 4000, 4000, true, 0, 'A1'],
    ['A6 layout rounding', 4000, 4000, true, 0, 'A6'],
    ['fractional shrink', 816.25, 100, false, 0, 'Letter'],
    ['size small', 358, 70, true, 300], ['size tall', 168, 504, true, 300],
    ['size tiny', 168, 504, true, 1], ['short', 358, 2],
    ...['Letter','Legal','Tabloid','Ledger','A0','A1','A2','A3','A4','A5','A6'].map(format => [format, 1896, 2000, true, 0, format]),
    ['Letter tall', 168, 2500, true, 0, 'Letter'],
    ['Letter fixed wide', 1896, 64, false, 0, 'Letter'],
    ['Letter size tall', 168, 2500, true, 300, 'Letter'],
    ['Letter fixed tall', 100, 2200, false, 0, 'Letter'],
    ...[1040,1044,1045,1047,1048,1049,1052,1055,1056,1057,2108,2112].map(h => ['boundary '+h,100,h,false,0,'Letter']),
    ...[800,808,816,900,1000,1200,1216,1224].map(w => ['shrink '+w,w,100,false,0,'Letter']),
  ];
  for (const [name, width, height, responsive = true, size = 0, format = '', viewport = size ? size + 16 : 800] of cases) {
    await this.setViewport({width: viewport, height: 600, deviceScaleFactor: 1});
    const input = await this.evaluate(({width,height,responsive,size}) => {
      const attrs = responsive ? 'width="100%" style="max-width: '+width+'px;"' : 'width="'+width+'" height="'+height+'"';
      const input = '<svg '+attrs+' xmlns="http://www.w3.org/2000/svg" viewBox="0 0 '+width+' '+height+'"><rect width="'+width+'" height="'+height+'" fill="cyan"/></svg>';
      document.querySelector('#container').innerHTML = input;
      document.body.style.background = 'red';
      const svg = document.querySelector('svg');
      svg.style.backgroundColor = 'magenta';
      if (size) {svg.style.maxWidth = size+'px'; svg.style.maxHeight = size+'px'; svg.removeAttribute('height');}
      return {svg: input, screen: svg.getBoundingClientRect().toJSON()};
    }, {width,height,responsive,size});
    const opts = {omitBackground: false, printBackground: true};
    if (format) opts.format = format;
    else Object.assign(opts, {width: Math.ceil(input.screen.width)+input.screen.x*2+'px', height: Math.ceil(input.screen.height)+input.screen.y*2+'px', pageRanges: '1-1'});
    const data = Buffer.from(await pdf.call(this, opts));
    const text = data.toString('latin1');
    const streams = [];
    for (const match of text.matchAll(/stream\\r?\\n/g)) {
      const start = match.index + match[0].length;
      try {
        const stream = zlib.inflateSync(data.subarray(start,text.indexOf('endstream',start))).toString();
        if (stream.includes(' cm')) streams.push(stream);
      } catch {}
    }
    const parsed = parsePDFOptions(opts);
    const pageWidth = Math.ceil(Math.fround(parsed.width*72))*96/72;
    const pageHeight = Math.ceil(Math.fround(parsed.height*72))*96/72;
    const layout = await this.evaluate(({pageWidth,pageHeight}) => {
      const w = Math.ceil(pageWidth);
      document.querySelector('#container').style.width = (w-16)+'px';
      return document.querySelector('svg').getBoundingClientRect().toJSON();
    }, {pageWidth,pageHeight});
    await this.evaluate(() => document.querySelector('#container').style.removeProperty('width'));
    let first = -1, matrix;
    for (let i=0;i<streams.length;i++) {
      const index = streams[i].indexOf('1 0 1 RG');
      if (index < 0) continue;
      first = i;
      matrix = streams[i].slice(0,index).split('\\n').filter(s => s.endsWith(' cm')).at(-1).split(' ').slice(0,6).map(Number);
      break;
    }
    if (first < 0) throw new Error('SVG background missing from ' + name);
    const scale = matrix[0]/3.125;
    if (name.endsWith('layout rounding')) {
      const stream = streams.find(s => s.includes('0 1 1 RG'));
      const index = stream.indexOf('0 1 1 RG');
      const transform = stream.slice(0,index).split('\\n').filter(s => s.endsWith(' cm')).at(-1).split(' ').map(Number);
      const expected = Math.fround(3.125*Math.fround(layout.width/width));
      if (Math.abs(transform[0]-expected)>0.000001) throw new Error('PDF layout transform differs for '+name+': '+transform[0]+' vs '+expected);
    }
    const mediaBox = text.match(/\\/MediaBox\\s*\\[([^\\]]+)/)[1].trim().split(/\\s+/).map(Number);
    const finalWidth = Math.fround(mediaBox[2])*96/72;
    const finalHeight = Math.fround(mediaBox[3])*96/72;
    const box = {x: layout.x*scale, y: first ? first*finalHeight : layout.y*scale,
      width: layout.width*scale, height: layout.height*scale};
    records.push({name, svg: input.svg, size, format, viewport,
      width: finalWidth, height: finalHeight, box, pages: (text.match(/\\/Type \\/Page\\b/g)||[]).length,
      matrix, mediaBox});
  }
  await fs.writeFile(${JSON.stringify(path.join(scratch, 'sample.json'))}, JSON.stringify(records,null,2));
  return result;
};
`;
try {
  await fs.writeFile(path.join(scratch, 'hook.mjs'), hook);
  await fs.writeFile(path.join(scratch, 'jobs.json'), JSON.stringify([{
    args: ['-o', path.join(scratch, 'mmdc.pdf')],
    stdin: 'flowchart LR\nA --> B\n',
    env: { NODE_OPTIONS: '--import=' + path.join(scratch, 'hook.mjs') },
    timeout: 60000,
  }]));
  const run = spawnSync(process.execPath, ['mmdc-run.mjs', path.join(scratch, 'jobs.json'), path.join(scratch, 'results.json')], {cwd: reference, stdio: 'inherit', timeout: 90000});
  if (run.error || run.status !== 0) throw run.error || new Error('mmdc-run failed');
  const results = JSON.parse(await fs.readFile(path.join(scratch, 'results.json'), 'utf8'));
  if (results.some(result => result.code !== 0)) throw new Error(JSON.stringify(results));
  const records = JSON.parse(await fs.readFile(path.join(scratch, 'sample.json'), 'utf8'));
  await fs.writeFile(path.join(here, 'print.json'), JSON.stringify(records, null, 2)+'\n');
  console.log('Recorded ' + records.length + ' cases from one locked headless mmdc browser.');
} catch (error) {
  console.error(error);
  process.exitCode = 1;
} finally {
  await fs.rm(scratch, {recursive: true, force: true});
}
