// Image size oracle: what Chromium gives for
//
//   const img = new Image(); img.src = src; await img.decode();
//   img.naturalWidth, img.naturalHeight
//
// which is how the imageSquare shape of mermaid gets the size of its image.
// Run with:
//
//   node oracle.mjs esm-eval probes/image-size.js --out out.json
//
// The sources are data URLs: bitmaps that a canvas encodes, with parts cut
// off or changed, and SVG documents. The result is a list of { name, src }
// with width and height, or with the message of the error of decode().
const mk = (w, h, type, quality, flat) => {
  const c = document.createElement('canvas');
  c.width = w;
  c.height = h;
  const g = c.getContext('2d');
  // A large image has one color, so that its file is small.
  let fill = '#39f';
  if (!flat) {
    fill = g.createLinearGradient(0, 0, w, h);
    fill.addColorStop(0, '#f00');
    fill.addColorStop(0.5, '#0f0');
    fill.addColorStop(1, '#00f');
  }
  g.fillStyle = fill;
  g.fillRect(0, 0, w, h);
  return c.toDataURL(type, quality);
};
const bytesOf = (u) => [...atob(u.slice(u.indexOf(',') + 1))].map((c) => c.charCodeAt(0));
const b64 = (bytes) => btoa(String.fromCharCode(...bytes));
const urlOf = (type, bytes) => 'data:' + type + ';base64,' + b64(bytes);
const svgDoc = (text) => 'data:image/svg+xml,' + encodeURIComponent(text);
const svgUrl = (attrs) => svgDoc(`<svg xmlns="http://www.w3.org/2000/svg" ${attrs}/>`);
const srcs = {};

// Sources that are not an image.
Object.assign(srcs, {
  empty: '',
  'missing-file': 'http://mmdg.local/missing.png',
  relative: 'nothere.png',
  'not-image': 'data:text/plain,hello',
  'no-comma': 'data:image/png;base64',
  'bad-base64': 'data:image/png;base64,AAAA',
  'base64-length': 'data:image/png;base64,AAAAA',
});

// PNG.
const png = bytesOf(mk(20, 10, 'image/png'));
const idat = (() => {
  for (let pos = 8; pos + 8 <= png.length; ) {
    const len = (png[pos] << 24) | (png[pos + 1] << 16) | (png[pos + 2] << 8) | png[pos + 3];
    if (String.fromCharCode(...png.slice(pos + 4, pos + 8)) === 'IDAT') {
      return pos;
    }
    pos += 12 + len;
  }
  return -1;
})();
const changed = (bytes, edit) => {
  const b = bytes.slice();
  edit(b);
  return b;
};
Object.assign(srcs, {
  png: urlOf('image/png', png),
  'png-large': mk(641, 483, 'image/png', undefined, true),
  'png-1x1': mk(1, 1, 'image/png'),
  'png-header': urlOf('image/png', png.slice(0, 33)),
  'png-header-short': urlOf('image/png', png.slice(0, 29)),
  'png-before-idat': urlOf('image/png', png.slice(0, idat)),
  'png-idat-length': urlOf('image/png', png.slice(0, idat + 4)),
  'png-idat-type-short': urlOf('image/png', png.slice(0, idat + 7)),
  'png-idat-type': urlOf('image/png', png.slice(0, idat + 8)),
  'png-idat-data': urlOf('image/png', png.slice(0, idat + 12)),
  'png-drop1': urlOf('image/png', png.slice(0, -1)),
  'png-half': urlOf('image/png', png.slice(0, png.length >> 1)),
  'png-bad-crc': urlOf('image/png', changed(png, (b) => (b[29] ^= 0xff))),
  'png-bad-depth': urlOf('image/png', changed(png, (b) => (b[24] = 3))),
  'png-zero-width': urlOf('image/png', changed(png, (b) => (b[16] = b[17] = b[18] = b[19] = 0))),
  'png-no-ihdr': urlOf('image/png', changed(png, (b) => (b[13] = 0x44))),
  'png-bad-signature': urlOf('image/png', changed(png, (b) => (b[0] = 0))),
  'png-corrupt-data': urlOf('image/png', changed(png, (b) => (b[b.length - 20] ^= 0xff))),
  'png-text-type': 'data:text/plain;base64,' + b64(png),
  'png-no-type': 'data:;base64,' + b64(png),
  'png-svg-type': 'data:image/svg+xml;base64,' + b64(png),
  'png-percent': 'data:image/png,' + png.map((c) => '%' + c.toString(16).padStart(2, '0')).join(''),
  'png-spaces': 'data:image/png;base64,' + b64(png).replace(/(.{20})/g, '$1 \n'),
  'png-no-padding': 'data:image/png;base64,' + b64(png).replace(/=+$/, ''),
  'png-base64-upper': 'data:image/png;BASE64,' + b64(png),
  'png-base64-space': 'data:image/png; base64,' + b64(png),
  'png-charset': 'data:image/png;charset=utf-8;base64,' + b64(png),
  'png-fragment': urlOf('image/png', png) + '#frag',
  'png-scheme-upper': 'DATA:image/png;base64,' + b64(png),
  'png-leading-space': '  ' + urlOf('image/png', png) + ' \n',
});

// JPEG. An Exif segment with an orientation goes after the first marker.
const jpg = bytesOf(mk(20, 10, 'image/jpeg', 0.5));
const markers = {};
for (let i = 2; i + 3 < jpg.length; ) {
  const len = (jpg[i + 2] << 8) | jpg[i + 3];
  markers[jpg[i + 1]] = [i, len];
  if (jpg[i + 1] === 0xda) {
    break;
  }
  i += 2 + len;
}
const [sof, sofLen] = markers[0xc0];
const [sos, sosLen] = markers[0xda];
const exif = (orientation, littleEndian) => {
  const tiff = littleEndian
    ? [0x49, 0x49, 0x2a, 0x00, 0x08, 0x00, 0x00, 0x00, 0x01, 0x00, 0x12, 0x01, 0x03, 0x00, 0x01, 0x00, 0x00, 0x00, orientation, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00]
    : [0x4d, 0x4d, 0x00, 0x2a, 0x00, 0x00, 0x00, 0x08, 0x00, 0x01, 0x01, 0x12, 0x00, 0x03, 0x00, 0x00, 0x00, 0x01, 0x00, orientation, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00];
  const body = [0x45, 0x78, 0x69, 0x66, 0x00, 0x00, ...tiff];
  const len = body.length + 2;
  return [0xff, 0xe1, len >> 8, len & 0xff, ...body];
};
Object.assign(srcs, {
  jpeg: urlOf('image/jpeg', jpg),
  'jpeg-large': mk(641, 483, 'image/jpeg', 0.3, true),
  'jpeg-to-sof': urlOf('image/jpeg', jpg.slice(0, sof + 2 + sofLen)),
  'jpeg-before-sos': urlOf('image/jpeg', jpg.slice(0, sos)),
  'jpeg-sos-marker': urlOf('image/jpeg', jpg.slice(0, sos + 2)),
  'jpeg-sos-short': urlOf('image/jpeg', jpg.slice(0, sos + 1 + sosLen)),
  'jpeg-sos-header': urlOf('image/jpeg', jpg.slice(0, sos + 2 + sosLen)),
  'jpeg-drop1': urlOf('image/jpeg', jpg.slice(0, -1)),
  'jpeg-corrupt-data': urlOf('image/jpeg', changed(jpg, (b) => (b[b.length - 10] ^= 0xff))),
  'jpeg-text-type': 'data:text/plain;base64,' + b64(jpg),
});
for (let o = 0; o <= 9; o++) {
  srcs[`jpeg-exif-${o}`] = urlOf('image/jpeg', [...jpg.slice(0, 2), ...exif(o, false), ...jpg.slice(2)]);
}
srcs['jpeg-exif-6-little-endian'] = urlOf('image/jpeg', [...jpg.slice(0, 2), ...exif(6, true), ...jpg.slice(2)]);

// GIF: a file of 1 by 1, cut after each byte, and with other screen sizes.
const gif = bytesOf('data:image/gif;base64,R0lGODlhAQABAIAAAAAAAP///yH5BAEAAAAALAAAAAABAAEAAAIBRAA7');
for (let n = gif.length; n >= 6; n--) {
  srcs[`gif-${n}`] = urlOf('image/gif', gif.slice(0, n));
}
srcs['gif-screen-5x7'] = urlOf('image/gif', changed(gif, (b) => ((b[6] = 5), (b[8] = 7))));
srcs['gif-screen-0x0'] = urlOf('image/gif', changed(gif, (b) => ((b[6] = 0), (b[8] = 0))));
srcs['gif-screen-0x7'] = urlOf('image/gif', changed(gif, (b) => ((b[6] = 0), (b[8] = 7))));
srcs['gif-87a'] = urlOf('image/gif', changed(gif, (b) => (b[4] = 0x37)));
srcs['gif-no-image'] = urlOf('image/gif', [...gif.slice(0, 27), 0x3b]);
srcs['gif-comment'] = urlOf('image/gif', [...gif.slice(0, 19), 0x21, 0xfe, 0x03, 0x61, 0x62, 0x63, 0x00, ...gif.slice(19)]);

// WebP.
const webp = bytesOf(mk(20, 10, 'image/webp', 0.5));
const webpLossless = bytesOf(mk(21, 11, 'image/webp', 1));
Object.assign(srcs, {
  webp: urlOf('image/webp', webp),
  'webp-lossless': urlOf('image/webp', webpLossless),
  'webp-large': mk(641, 483, 'image/webp', 0.3, true),
  'webp-drop1': urlOf('image/webp', webp.slice(0, -1)),
  'webp-half': urlOf('image/webp', webp.slice(0, webp.length >> 1)),
  'webp-header': urlOf('image/webp', webp.slice(0, 30)),
  'webp-extra': urlOf('image/webp', [...webp, 0, 0, 0, 0]),
  'webp-corrupt-data': urlOf('image/webp', changed(webp, (b) => (b[b.length - 10] ^= 0xff))),
});

// BMP: 24 bits, made by hand.
const bmp = (w, h) => {
  const row = Math.ceil((w * 3) / 4) * 4;
  const size = 54 + row * Math.abs(h);
  const b = new Array(size).fill(0);
  const le = (o, v, n) => {
    for (let i = 0; i < n; i++) {
      b[o + i] = (v >>> (8 * i)) & 0xff;
    }
  };
  b[0] = 0x42;
  b[1] = 0x4d;
  le(2, size, 4);
  le(10, 54, 4);
  le(14, 40, 4);
  le(18, w, 4);
  le(22, h, 4);
  le(26, 1, 2);
  le(28, 24, 2);
  le(34, row * Math.abs(h), 4);
  return b;
};
Object.assign(srcs, {
  'bmp-2x3': urlOf('image/bmp', bmp(2, 3)),
  'bmp-top-down': urlOf('image/bmp', bmp(4, -5)),
  'bmp-headers': urlOf('image/bmp', bmp(2, 3).slice(0, 54)),
  'bmp-headers-short': urlOf('image/bmp', bmp(2, 3).slice(0, 53)),
  'bmp-file-header': urlOf('image/bmp', bmp(2, 3).slice(0, 14)),
});

// SVG: the type, and the form of the document.
const svgText = '<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><rect fill="#f00" width="10" height="10"/></svg>';
Object.assign(srcs, {
  'svg-raw-hash': 'data:image/svg+xml,' + svgText,
  'svg-encoded': svgDoc(svgText),
  'svg-utf8': 'data:image/svg+xml;utf8,' + encodeURIComponent(svgText),
  'svg-charset': 'data:image/svg+xml;charset=utf-8,' + encodeURIComponent(svgText),
  'svg-base64': 'data:image/svg+xml;base64,' + btoa(svgText),
  'svg-type-upper': 'data:IMAGE/SVG+XML,' + encodeURIComponent(svgText),
  'svg-png-type': 'data:image/png,' + encodeURIComponent(svgText),
  'svg-xml-type': 'data:text/xml,' + encodeURIComponent(svgText),
  'svg-no-type': 'data:,' + encodeURIComponent(svgText),
  'svg-declaration': svgDoc('<?xml version="1.0"?><!-- c --><!DOCTYPE svg>' + svgText),
  'svg-prefix': svgDoc('<s:svg xmlns:s="http://www.w3.org/2000/svg" width="7" height="9"/>'),
  'svg-white-space': svgDoc('  \n' + svgText + '\n '),
  'svg-comment-after': svgDoc(svgText + '<!-- end -->'),
  'svg-text-after': svgDoc(svgText + 'junk'),
  'svg-text-before': svgDoc('junk' + svgText),
  'svg-two-roots': svgDoc(svgText + svgText),
  'svg-unclosed-child': svgDoc('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><rect></svg>'),
  'svg-unclosed-root': svgDoc('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><rect/>'),
  'svg-cut': svgDoc('<svg xmlns="http://www.w3.org/2000/svg"><rect'),
  'svg-no-namespace': svgDoc('<svg width="10" height="20"></svg>'),
  'svg-html-root': svgDoc('<html xmlns="http://www.w3.org/1999/xhtml"/>'),
  'svg-other-root': svgDoc('<g xmlns="http://www.w3.org/2000/svg"/>'),
  'svg-entity': svgDoc('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><text>&nbsp;</text></svg>'),
  'svg-known-entity': svgDoc('<svg xmlns="http://www.w3.org/2000/svg" width="40" height="30"><text>&amp;&#160;</text></svg>'),
  'svg-empty': 'data:image/svg+xml,',
  'svg-style-size': svgUrl('style="width:60px;height:20px"'),
  'svg-xlink-attr': svgDoc('<svg xmlns="http://www.w3.org/2000/svg" xmlns:x="urn:x" x:width="5" width="41" height="31"/>'),
});
// The width and the height.
const sizes = {
  wh: 'width="40" height="30"',
  'wh-px': 'width="40px" height="30.5px"',
  'wh-fraction': 'width="10.6" height="20.4"',
  'wh-half': 'width="0.5" height="1.5"',
  'wh-half2': 'width="2.5" height="3.49"',
  'wh-small': 'width="0.4" height="0.6"',
  'wh-zero': 'width="0" height="0"',
  'w-zero': 'width="0" height="10"',
  'h-zero': 'width="10" height="0"',
  'w-negative': 'width="-40" height="30"',
  'wh-negative': 'width="-1" height="-1"',
  'wh-exponent': 'width="1e2" height=" 3.5e1 "',
  'wh-plus': 'width="+12" height="1.5E+1"',
  'wh-dot': 'width=".5e2" height="5."',
  'wh-space-unit': 'width="12 px" height="12"',
  'wh-percent': 'width="100%" height="100%"',
  'w-percent': 'width="50%"',
  'h-percent': 'width="40" height="50%"',
  'wh-auto': 'width="auto" height="20"',
  'wh-invalid': 'width="abc" height="20px;"',
  'wh-empty': 'width="" height=""',
  'wh-big': 'width="100000" height="50000"',
  'wh-huge': 'width="1e9" height="1e9"',
  'w-only': 'width="77"',
  'h-only': 'height="77"',
  none: '',
  'w-e-unit': 'width="2em" height="2e1em"',
  'w-ex': 'width="3ex" height="1ex"',
};
for (const unit of ['ex', 'ch', 'em', 'rem', 'pt', 'pc', 'in', 'cm', 'mm', 'Q', 'px', 'vw', 'vh', 'vmin', 'vmax', 'lh', 'cap', 'ic', 'EM', 'Px', 'PT', 'q', 'foo']) {
  sizes[`unit-${unit}`] = `width="1000${unit}" height="7${unit}"`;
}
// The view box and preserveAspectRatio.
Object.assign(sizes, {
  vb: 'viewBox="0 0 200 100"',
  'vb-fraction': 'viewBox="0 0 10.5 20.5"',
  'vb-comma': 'viewBox="0,0,50,200"',
  'vb-space': 'viewBox=" 0  0 , 20   60 "',
  'vb-wide': 'viewBox="0 0 1000 10"',
  'vb-tall': 'viewBox="0 0 10 1000"',
  'vb-zero': 'viewBox="0 0 0 100"',
  'vb-negative': 'viewBox="0 0 -10 100"',
  'vb-three': 'viewBox="0 0 10"',
  'vb-five': 'viewBox="0 0 20 60 9"',
  'vb-text': 'viewBox="0 0 a b"',
  'vb-w': 'width="50" viewBox="0 0 200 100"',
  'vb-h': 'height="50" viewBox="0 0 200 100"',
  'vb-w-ratio': 'width="77" viewBox="0 0 3 7"',
  'vb-h-ratio': 'height="77" viewBox="0 0 3 7"',
  'vb-w-zero': 'width="0" viewBox="0 0 2 7"',
  'vb-h-zero': 'height="0" viewBox="0 0 2 7"',
  'vb-percent': 'width="100%" height="100%" viewBox="0 0 30 60"',
  'vb-w-percent-h': 'width="50%" height="80" viewBox="0 0 10 40"',
  'vb-wh': 'width="20" height="30" viewBox="0 0 3 7"',
  'par-slice': 'width="30" viewBox="0 0 3 7" preserveAspectRatio="xMidYMid slice"',
  'par-none-w': 'width="30" viewBox="0 0 3 7" preserveAspectRatio="none"',
  'par-none-h': 'height="30" viewBox="0 0 3 7" preserveAspectRatio="none"',
  'par-none-wh': 'width="20" height="30" viewBox="0 0 3 7" preserveAspectRatio="none"',
  'par-none-vb': 'viewBox="0 0 3 7" preserveAspectRatio="none"',
  'par-none-w-no-vb': 'width="30" preserveAspectRatio="none"',
  'par-none-space': 'width="30" viewBox="0 0 3 7" preserveAspectRatio=" none "',
  'par-none-meet': 'width="30" viewBox="0 0 3 7" preserveAspectRatio="none meet"',
  'par-invalid': 'width="30" viewBox="0 0 3 7" preserveAspectRatio="nonsense"',
});
for (const [name, attrs] of Object.entries(sizes)) {
  srcs[`svg-${name}`] = svgUrl(attrs);
}

const out = [];
for (const [name, src] of Object.entries(srcs)) {
  const img = new Image();
  img.src = src;
  const r = { name, src };
  try {
    await img.decode();
    r.width = img.naturalWidth;
    r.height = img.naturalHeight;
  } catch (e) {
    r.error = String(e && e.message);
  }
  out.push(r);
}
return out;
