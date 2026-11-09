// Hostile and odd HTML inputs for the DOMPurify goldens. The list has a
// hand-written part, with one or more cases for each branch of the sanitizer,
// and a generated part from a seeded PRNG.
import fs from 'node:fs';
import path from 'node:path';
import { ROOT, mulberry32, walk } from './purify-oracle-lib.mjs';

const CORPUS = path.join(ROOT, 'testdata/corpus/mermaid-e2e');

const NBSP = '\u00a0';

// Hand-written cases.
export const handWritten = [
  // Trivial.
  '',
  ' ',
  '\n',
  'plain text',
  'a < b',
  'a <b',
  'a > b',
  'a & b',
  'a &amp; b &lt; c',
  '<',
  '<<',
  '< >',
  '<>',
  '</>',
  '</',
  '<a',
  '<a ',
  '<a href',
  '<a href=',
  '<a href="',
  '<a href="x',
  '<b>bold</b>',
  '<B>BOLD</B>',
  '<i>a<b>b</i>c</b>',
  '<p>one<p>two',
  '<br>',
  '<br/>',
  '<br />',
  '</br>',
  'a<br>b<br/>c<br />d</br>e',
  '<hr><img src="a.png"><input value=" v "><wbr>',
  // Leading and trailing white space.
  '  <b>x</b>',
  '\n<b>x</b>\n',
  '\t\r\n <b>x</b> \t\r\n',
  '\r<b>x</b>',
  '\r\n\r\n<i>y</i>',
  ' text first <b>x</b>',
  '\n\n<svg></svg>',
  ' <title>t</title> after',
  ' <style>a{}</style> after',
  ' <script>1</script> after',
  ' <!-- c --> after <b>x</b>',
  '\f<b>x</b>',
  '\v<b>x</b>',
  NBSP + '<b>x</b>',
  // Scripts and other raw text elements.
  '<script>alert(1)</script>',
  'a<script>alert(1)</script>b',
  '<SCRIPT SRC=//x></SCRIPT>',
  '<script>a</script><b>x</b>',
  '<b>x</b><script>a<b>c</script>',
  '<script>',
  '<script>alert(1)',
  '<scr<script>ipt>alert(1)</scr</script>ipt>',
  '<svg><script>alert(1)</script></svg>',
  '<svg><script xlink:href="x"/></svg>',
  '<math><script>1</script></math>',
  '<iframe src="javascript:alert(1)"></iframe>',
  '<iframe>&lt;b&gt;</iframe>',
  '<iframe><b>x</b></iframe>y',
  '<xmp><b>x</b></xmp>',
  '<xmp></xmp><b>x</b>',
  '<noembed><b>x</b></noembed>y',
  '<noframes><b>x</b></noframes>y',
  '<plaintext><b>x</b>',
  'a<plaintext></plaintext><b>x</b>',
  '<textarea><b>x</b></textarea>',
  '<textarea>&lt;/textarea&gt;</textarea>',
  '<textarea>\n\nx</textarea>',
  '<pre>\n\nx</pre>',
  '<pre>x\n</pre>',
  '<listing>\nx</listing>',
  '<title>x</title>y',
  'a<title><b>x</b></title>y',
  '<object data="x"></object>',
  '<embed src="x">',
  '<applet></applet><base href="x"><link rel="x" href="y"><meta charset="x">',
  '<frameset><frame src="x"></frameset>',
  '<frameset></frameset>',
  'x<frameset><frame></frameset>',
  '<html><head><title>t</title></head><body class="b" onload="x">y</body></html>',
  '<!DOCTYPE html><html><body>z</body></html>',
  '<!doctype html>x',
  '<body><b>x</b></body><body id="b2">y',
  '<head><b>x</b></head>',
  '<html lang="en">x',
  // noscript: the DOMParser document has scripting off.
  '<noscript><b>x</b></noscript>y<b>z</b>',
  'a<noscript><b>x</b></noscript>y',
  'a<noscript>&lt;b</noscript>y',
  'a<noscript><style></noscript><b>x</b>',
  'a<noscript><p title="</noscript><img src=x onerror=alert(1)>">',
  'a<noscript><!-- </noscript> --></noscript>z',
  // Event handlers and unknown attributes.
  '<b onclick="alert(1)">x</b>',
  '<img src="x" onerror="alert(1)">',
  '<img src=x onerror=alert(1)//>',
  '<div ONMOUSEOVER="x" Class="A" STYLE="color:red">x</div>',
  '<div foo="1" bar=2 baz>x</div>',
  '<div data-foo="1" data-="2" data-a.b="3" data-A_B-c="4" data="5">x</div>',
  '<div data-\u00d7=" a " data-\u00b7="1" data-\u00b6="2" data-\ud83d\ude00="3">x</div>',
  '<div aria-label="l" aria-="e" aria-foo.bar="2" ARIA-Hidden="true">x</div>',
  '<div title=" spaced " lang=" en " class=" a b ">x</div>',
  '<input value=" keep " placeholder=" trim ">',
  '<div class="a" class="b" CLASS="c">x</div>',
  '<div id="a" id="b">x</div>',
  '<div =x a=b>x</div>',
  '<div a=\'"\' b="\'" c=`x` d=a&amp;b e="a&b" f="a&lt;b" g="a<b>c">x</div>',
  '<div title="a&quot;b&apos;c&#39;d&#x27;e">x</div>',
  '<div title="' + NBSP + 'x' + NBSP + '">a' + NBSP + 'b&nbsp;c&#160;d</div>',
  '<div title="line1\nline2\ttab\r\nx">x</div>',
  '<div title="--&gt;">a</div><div title="--!>">b</div><div title="]>">c</div><div title="x]>y">d</div>',
  '<div title="</style>">a</div><div title="</SCRIPT>">b</div><div title="</title">c</div>',
  '<div title="</xmp></textarea></noscript></iframe></noembed></noframes>">x</div>',
  '<div title="<br/>">a</div><div title="/>">b</div>',
  '<div is="x-y" class="a">t</div>',
  '<div class="a" is="x">t</div><span is>u</span>',
  '<div IS="x" id="i">t</div>',
  '<p align="center" dir="rtl" hidden>x</p>',
  '<font color="red" face="Arial" size="3">x</font>',
  '<table border="1" cellpadding="2"><tr><td colspan="2" rowspan="1" valign="top">x</td></tr></table>',
  '<label for="x">l</label><div for="y">d</div><output for="z">o</output>',
  '<div patchsrc="x">a</div>',
  '<div style="color:red;background:url(javascript:alert(1))">x</div>',
  '<div style="x:expression(alert(1))">x</div>',
  '<a href="x" target="_blank">l</a>',
  '<a target="_blank" href="x">l</a>',
  '<a href="x" target="_self" rel="nofollow">l</a>',
  '<a href="x" target="_blank" rel="nofollow">l</a>',
  '<a href="x" TARGET="_BLANK">l</a>',
  '<a href="x" target>l</a>',
  '<a target="_blank">l</a><a data-temp-href-target="_blank">m</a>',
  '<a href="x" target="_blank" data-temp-href-target="_top">l</a>',
  '<svg><a href="x" target="_blank">l</a></svg>',
  '<span target="_blank">s</span>',
  '<area target="_blank" href="x">',
  // URLs.
  '<a href="javascript:alert(1)">x</a>',
  '<a href="JaVaScRiPt:alert(1)">x</a>',
  '<a href=" javascript:alert(1)">x</a>',
  '<a href="java\tscript:alert(1)">x</a>',
  '<a href="java\nscript:alert(1)">x</a>',
  '<a href="java&#x0A;script:alert(1)">x</a>',
  '<a href="&#106;avascript:alert(1)">x</a>',
  '<a href="java' + NBSP + 'script:alert(1)">x</a>',
  '<a href="\u2028javascript:alert(1)">x</a>',
  '<a href="\u200bjavascript:alert(1)">x</a>',
  '<a href="\u017ftp://x">long s</a><a href="\u212aey:x">kelvin</a>',
  '<a href="vbscript:x">x</a>',
  '<a href="data:text/html,<b>x</b>">x</a>',
  '<a href="data:text/html;base64,PGI+">x</a>',
  '<img src="data:image/png;base64,AAAA">',
  '<img src="DATA:image/png;base64,AAAA">',
  '<img src=" data:image/png;base64,AAAA">',
  '<video src="data:video/mp4;base64,AAAA"></video><audio src="data:x"></audio>',
  '<source src="data:x"><track src="data:y">',
  '<svg><image href="data:image/png;base64,AAAA" xlink:href="data:image/png;base64,BBBB"/></svg>',
  '<svg><a xlink:href="javascript:alert(1)"><text>x</text></a></svg>',
  '<svg><a href="javascript:alert(1)"><text>x</text></a></svg>',
  '<svg><use href="#a" xlink:href="#b"/></svg>',
  '<a href="http://example.com">x</a>',
  '<a href="https://example.com/a?b=c&d=e#f">x</a>',
  '<a href="HTTPS://EXAMPLE.COM">x</a>',
  '<a href="ftp://x">x</a><a href="ftps://x">x</a><a href="mailto:a@b">x</a><a href="tel:+1">x</a>',
  '<a href="callto:x">x</a><a href="sms:1">x</a><a href="cid:1">x</a><a href="xmpp:a@b">x</a><a href="matrix:x">x</a>',
  '<a href="foo:bar">x</a><a href="foo.bar-baz+q:x">x</a><a href="foo/bar:baz">x</a><a href="foo">x</a>',
  '<a href="#frag">x</a><a href="/path">x</a><a href="./rel">x</a><a href="//host/x">x</a><a href="?q">x</a>',
  '<a href="">x</a><a href>y</a><a href=" ">z</a>',
  '<a href=":x">x</a><a href="1:x">x</a><a href="a1:x">x</a><a href="a:">x</a>',
  '<a href="blob:x">x</a><a href="file:///x">x</a><a href="about:blank">x</a><a href="chrome://x">x</a>',
  '<form action="javascript:x"><input formaction="javascript:y"></form>',
  '<img srcset="x 1x, javascript:y 2x" src="a">',
  '<a href="x" xlink:href="y" XLINK:HREF="z">h</a>',
  '<div background="javascript:x" cite="javascript:y" poster="javascript:z">x</div>',
  // Clobbering.
  '<div id="title">a</div><div id="body">b</div><div id="location">c</div>',
  '<img name="cookie"><img id="createElement"><img name="getElementById">',
  '<a id="x" name="y">a</a><a id="attributes">b</a><a name="submit">c</a>',
  '<div id="constructor">a</div><div id="__proto__">b</div><div id="toString">c</div>',
  '<div id="length">a</div><div id="0">b</div><div id="item">c</div><div id="Title">d</div>',
  '<div id=" title ">a</div><div id="">b</div><div id>c</div>',
  '<form><input name="attributes"></form>',
  '<form><input name="nodeName"><b>x</b></form>y',
  '<form><input id="childNodes"></form>',
  '<form><input name="nodeType"></form><form><input name="textContent"></form>',
  '<form><input name="removeChild"></form><form><input name="insertBefore"></form>',
  '<form><input name="hasChildNodes"></form><form><input name="namespaceURI"></form>',
  '<form><input name="setAttribute"></form><form><input name="removeAttribute"></form>',
  '<form><input name="innerHTML"></form><form><input name="tagName"></form>',
  '<form><input name="parentNode"><input name="firstElementChild"></form>',
  '<form><img name="attributes"></form>',
  '<form><img id="nodeName"></form>',
  '<form><input type="image" name="attributes"></form>',
  '<form><input type="IMAGE" name="attributes"></form>',
  '<form><button name="attributes">b</button></form>',
  '<form><select name="childNodes"><option>o</option></select></form>',
  '<form><textarea name="nodeType">t</textarea></form>',
  '<form><fieldset name="attributes"><legend>l</legend></fieldset></form>',
  '<form><output name="attributes">o</output></form>',
  '<form><object name="attributes"></object></form>',
  '<form><label name="attributes">l</label><div id="nodeName">d</div><a name="nodeType">a</a></form>',
  '<form><div><p><input name="attributes"></p></div></form>',
  '<form id="f"></form><input form="f" name="attributes">',
  '<input form="f" name="attributes"><form id="f">x</form>',
  '<form id=" f "></form><input form="f" name="attributes">',
  '<form id="f">a</form><form id="f">b</form><input form="f" name="nodeName">',
  '<form id="f">a</form><div><form id="g"><input form="f" name="nodeName"></form></div>',
  '<form><input form="zzz" name="attributes"></form>',
  '<form><input form="" name="attributes"></form>',
  '<div><form></div><input name="attributes">',
  '<table><form><tr><td><input name="attributes"></td></tr></form></table>',
  '<form><template><input name="attributes"></template></form>',
  '<form action="x" method="post" name="n" id="i"><input name="q" value="v"></form>',
  '<form name="attributes">x</form><form id="nodeName">y</form>',
  '<svg><form><input name="attributes"/></form></svg>',
  '<form><svg><input name="attributes"></svg></form>',
  '<form><form><input name="attributes"></form></form>',
  '<b><form><input name="attributes"></b>x</form>',
  '<form><input name="attributes" id="nodeName"><input name="attributes"></form>',
  '<form><input name="elements"><input name="length"></form>',
  '<form><input name="innerHTML">a&lt;b<!--c--></form>',
  '<form id="f">a&lt;b<!--c--></form><input form="f" name="innerHTML">',
  '<form id="f">a&lt;b<!--c--></form><input form="f" name="firstElementChild">',
  // Templates.
  '<template><b>x</b><script>1</script></template>',
  'a<template><b onclick="x">y</b><script>1</script>z</template>',
  'a<template><template><i onclick="x">y</i><!--c--></template>z</template>',
  'a<div><template><form><input name="attributes"></form></template></div>',
  'a<template><svg><foreignObject><p>x</p></foreignObject></svg></template>',
  'a<template></template>b<template>text only</template>c',
  'a<template><tr><td>x</td></tr></template>',
  'a<template><unknown>keep</unknown></template>',
  'a<template>&lt;b<!--c--></template>',
  'a<b><template><p>x</b>y</template>',
  'a<svg><template><b>x</b></template></svg>',
  'a<math><template><b>x</b></template></math>',
  'a<template id="title" foo="bar" class=" c ">x</template>',
  'a<template shadowrootmode="open"><b>x</b></template>',
  'a<div><template shadowrootmode="open"><slot></slot><b onclick=x>y</b></template>z</div>',
  // Comments, CDATA, processing instructions, doctype.
  '<!-- comment -->',
  'a<!-- comment -->b',
  'a<!---->b<!-->c<!--->d<!--x--!>e',
  'a<!-- <b>x</b> -->b',
  'a<!--',
  'a<!-- x',
  'a<!x>b',
  'a<?php echo 1 ?>b',
  'a<?xml version="1.0"?>b',
  'a<![CDATA[ x<b>y ]]>b',
  'a<svg><![CDATA[ x<b>y ]]></svg>b',
  'a<svg><text><![CDATA[<b>&amp;]]></text></svg>',
  'a<math><mtext><![CDATA[x]]></mtext></math>',
  'a<!DOCTYPE html>b',
  'a<!ENTITY x "y">b',
  '<p>&lt;x<!--c--></p>',
  '<p>a&lt;b</p><p>&lt;/b</p><p>&lt;!x</p><p>&lt;1</p><p>&lt; b</p>',
  '<p>&lt;x</p>',
  '<div>&lt;x<span>y</span></div>',
  // Styles.
  '<style>a{color:red}</style>',
  'a<style>a{color:red}</style>b',
  'a<style>a > b { }</style>b',
  'a<style>a<b{}</style>b',
  'a<style></style',
  'a<style></style ',
  'a<style>x</style x>y',
  'a<style><!-- a{} --></style>b',
  'a<style>/* </style> */ b{}</style>c',
  'a<style>@import url("javascript:x");</style>b',
  'a<style>@import "x.css"; a::before{content:"<b>"}</style>b',
  'a<style>a{content:"&lt;"}</style>b',
  'a<style>&amp;</style>b',
  'a<STYLE type="text/css" media="all" onload="x">a{}</STYLE>b',
  'a<style>a{}</style><style>b{}</style>c',
  'a<svg><style>a{color:red}</style></svg>b',
  'a<svg><style>a > b {} c::after{content:"<"}</style></svg>b',
  'a<svg><style><![CDATA[ a>b{} ]]></style></svg>b',
  'a<svg><style>&lt;b</style></svg>b',
  'a<svg><style><b>x</b></style></svg>b',
  'a<svg><style>a{}</style><g><style>b{}</style></g></svg>b',
  'a<math><style>a{}</style></math>b',
  // Entities.
  '<b>&amp;&lt;&gt;&quot;&apos;&nbsp;&copy;&#169;&#xA9;&notit;&notin;&amp</b>',
  '<b>&unknown; &; &# &#x &#0; &#128; &#xD800; &#x110000; &#9; &#13;</b>',
  '<b>&lt;script&gt;alert(1)&lt;/script&gt;</b>',
  '<b title="&amp;&lt;&gt;&quot;&nbsp;&copy;&notit;&amp">x</b>',
  '<b>a' + NBSP + 'b\u2003c\u200bd\ufeffe</b>',
  '<b>#quot;x#quot; #amp; #35; #9829;</b>',
  '<b>\ufb02\u00b0\u00b035\u00b6\u00df \ufb02\u00b0quot\u00b6\u00df</b>',
  '<b>emoji \ud83d\ude00 \u4e2d\u6587 \u0639\u0631\u0628\u064a</b>',
  '<b>\u0000null\u0000</b>',
  '<b title="\u0000x">a\u0000b</b>',
  '<svg><text>\u0000x</text></svg>',
  '<b\u0000x>y</b>',
  '<b>x\r\ny\rz</b>',
  // SVG and foreign content.
  '<svg></svg>',
  '<svg/>',
  '<svg><g><rect width="1" height="2"/><circle r="3"/><path d="M0 0L1 1"/></g></svg>',
  '<SVG VIEWBOX="0 0 1 1" PreserveAspectRatio="none"><CLIPPATH><RECT/></CLIPPATH><foreignobject>x</foreignobject></SVG>',
  '<svg viewBox="0 0 10 10" xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" xml:space="preserve" xml:lang="en" xml:base="x" xml:id="i">x</svg>',
  '<svg><linearGradient gradientUnits="x" gradientTransform="y"><stop stop-color="red" offset="1"/></linearGradient></svg>',
  '<svg><feBlend/><feGaussianBlur stdDeviation="1"/><filter><feFlood flood-color="red"/></filter></svg>',
  '<svg><textPath startOffset="1" textLength="2" lengthAdjust="x">t</textPath><tspan dominant-baseline="central">u</tspan></svg>',
  '<svg><marker markerWidth="1" markerHeight="2" refX="3" refY="4" markerUnits="x" orient="auto"/></svg>',
  '<svg><animate attributeName="href" values="javascript:x"/><set attributeName="x" to="y"/></svg>',
  '<svg><animate attributeName="x" from="0" to="1"/><animateTransform attributeName="transform"/></svg>',
  '<svg><animateMotion attributeName="xlink:href"/><animateColor attributeName=" href "/></svg>',
  '<svg><use xlink:href="#x"/><image xlink:href="x.png"/><foreignObject><b>x</b></foreignObject></svg>',
  '<svg><foreignObject><div xmlns="http://www.w3.org/1999/xhtml">x<br/>y</div></foreignObject></svg>',
  '<svg><foreignObject width="1" height="2"><p>a<span class="b">c</span></p><svg><rect/></svg></foreignObject></svg>',
  '<svg><foreignObject><math><mi>x</mi></math><table><tr><td>y</td></tr></table></foreignObject></svg>',
  '<svg><foreignObject><script>1</script><style>a{}</style><iframe></iframe></foreignObject></svg>',
  '<svg><desc><b>x</b></desc><title><i>y</i></title></svg>',
  '<svg><title>t</title><desc>d</desc><metadata>m</metadata></svg>',
  '<svg><b>bold</b><rect/></svg>after',
  '<svg><p>para</p><rect/></svg>after',
  '<svg><g><div>d</div></g><circle/></svg>',
  '<svg><table><tr><td>x</td></tr></table></svg>',
  '<svg><img src="x"><br><hr></svg>',
  '<svg><font>f</font><font color="red">g</font><font face="x">h</font></svg>',
  '<svg><a><b>x</b></a></svg>',
  '<svg><svg><svg><rect/></svg></svg></svg>',
  '<svg><g><svg><foreignObject><svg><circle/></svg></foreignObject></svg></g></svg>',
  '<svg><unknown><rect/></unknown><x-y><circle/></x-y></svg>',
  '<svg><html><body><b>x</b></body></html></svg>',
  '<svg><head><title>x</title></head></svg>',
  '<svg><math><mi>x</mi></math></svg>',
  '<svg><annotation-xml><b>x</b></annotation-xml></svg>',
  '<svg><text x="1" y="2" text-anchor="middle" font-family="a,&quot;b c&quot;" font-size="12px">t</text></svg>',
  '<svg><rect id="title" name="n"/><g id="a b"/><g id="children"/></svg>',
  '<svg><rect style="fill:red" class="a b" transform="translate(1,2)" onclick="x" foo="bar"/></svg>',
  '<svg><style>a{}</style><script>b</script><title>c</title><a>d</a><font>e</font></svg>',
  '<svg><textarea>x</textarea><input value="y"><select><option>z</option></select></svg>',
  '<svg><p><rect/></p></svg>',
  '<svg><foreignObject><foreignObject>x</foreignObject></foreignObject></svg>',
  '<svg><switch><foreignObject requiredExtensions="x"><p>a</p></foreignObject><text>b</text></switch></svg>',
  '<svg xmlns:xlink="http://www.w3.org/1999/xlink" xmlns:foo="bar" foo:baz="1" xlink:title=" t " xlink:show="new">x</svg>',
  '<svg><g xmlns=" http://www.w3.org/2000/svg ">x</g></svg>',
  '<svg></p><b>x</b></svg>',
  '<svg></br><b>x</b></svg>',
  '<svg><g></svg><rect/>',
  '<svg><text>a<br>b</text></svg>',
  '<p><svg><p>x</p></svg></p>',
  '<svg><path d="M0,0"></path><PATH D="M1,1"/></svg>',
  '<div><svg viewbox="0 0 1 1" VIEWBOX="x"><g/></svg></div>',
  '<svg><altGlyph/><altGlyphDef/><glyphRef/><textpath/><fecolormatrix/></svg>',
  // MathML.
  '<math><mi>x</mi><mo>+</mo><mn>1</mn></math>',
  '<math><mrow><mfrac><mi>a</mi><mi>b</mi></mfrac></mrow></math>',
  '<math><mtext><b>x</b></mtext><mi><i>y</i></mi></math>',
  '<math><annotation-xml encoding="text/html"><b>x</b></annotation-xml></math>',
  '<math><annotation-xml encoding="application/xhtml+xml"><b>x</b></annotation-xml></math>',
  '<math><annotation-xml><b>x</b></annotation-xml></math>',
  '<math><semantics><mi>x</mi><annotation encoding="x">y</annotation></semantics></math>',
  '<math><mi><svg><rect/></svg></mi><mrow><svg><rect/></svg></mrow></math>',
  '<math><mglyph src="x"/><malignmark/><maction actiontype="x">y</maction></math>',
  '<math href="javascript:x" xlink:href="y" display="block" mathvariant="bold"><mi>x</mi></math>',
  '<math><b>x</b></math>after',
  '<math><mi>a</mi><div>d</div></math>',
  '<math><mtext><svg><style><img src=x onerror=alert(1)></style></svg></mtext></math>',
  '<math><mtext><table><mglyph><style><img src=x onerror=alert(1)>',
  '<math><mtext><option><FAKEFAKE><option></option><mglyph><svg><mtext><style><a title="</style><img src onerror=alert(1)>">',
  // mXSS classics.
  '<svg></p><style><a id="</style><img src=1 onerror=alert(1)>">',
  '<form><math><mtext></form><form><mglyph><style></math><img src onerror=alert(1)>',
  '<svg><style><a title="</style><img src onerror=alert(1)>"></style></svg>',
  '<noscript><p title="</noscript><img src=x onerror=alert(1)>">',
  '<p id="</p><img src=x onerror=alert(1)>">x</p>',
  '<a href="x" title="<img src=x onerror=alert(1)>">y</a>',
  '<listing>&lt;img src=x onerror=alert(1)&gt;</listing>',
  '<img src="x` `<script>alert(1)</script>"` `>',
  '<select><option><style></option></select><img src=x onerror=alert(1)></style>',
  '<table><caption><style></caption></table><img src=x onerror=alert(1)>',
  '<math><mi><table><mglyph><style><!--</style><img title="--&gt;&lt;img src=1 onerror=alert(1)&gt;">',
  '<svg><desc><template><p><style><a title="</style><img src=x onerror=alert(1)>">',
  '<div><xmp></div><img src=x onerror=alert(1)></xmp>',
  '<a><svg><a><title><style></title><img src=x onerror=alert(1)>',
  // Unknown and custom elements.
  '<foo>x</foo>',
  '<foo-bar baz="1">x</foo-bar>',
  '<x-y><b>keep</b> text</x-y>',
  '<blink>b</blink><marquee>m</marquee><center>c</center><nobr>n</nobr>',
  '<unknown><unknown2>deep<b>x</b></unknown2>tail</unknown>',
  '<a:b>x</a:b><svg:rect/><xlink:href>y</xlink:href>',
  '<1>x</1><_a>y</_a><a_b>z</a_b>',
  '<font-face>x</font-face><annotation-xml>y</annotation-xml><color-profile>z</color-profile>',
  '<article><section><header><nav><main><aside><footer><figure><figcaption>x</figcaption></figure></footer></aside></main></nav></header></section></article>',
  '<details open><summary>s</summary>d</details><dialog open>x</dialog>',
  '<ul><li>a<li>b</ul><ol start="2" reversed><li value="3">c</ol><dl><dt>t<dd>d</dl>',
  '<h1>1</h1><h2>2</h2><h3>3</h3><h4>4</h4><h5>5</h5><h6>6</h6><h7>7</h7>',
  '<em>e</em><strong>s</strong><code>c</code><kbd>k</kbd><samp>s</samp><var>v</var><sub>b</sub><sup>p</sup><small>m</small><mark>k</mark><del>d</del><ins>i</ins><s>s</s><u>u</u><q>q</q><cite>c</cite><abbr title="t">a</abbr>',
  '<ruby>a<rt>b</rt><rp>(</rp></ruby><bdi>x</bdi><bdo dir="rtl">y</bdo><time datetime="2020">t</time><data value="1">d</data>',
  '<picture><source srcset="a.png" media="x"><img src="b.png" alt="alt" width="1" height="2" loading="lazy"></picture>',
  '<video controls poster="p.png" width="1"><source src="a.mp4" type="video/mp4"><track kind="captions" src="c.vtt" srclang="en" label="l" default>fallback</video>',
  '<audio controls loop muted autoplay preload="none" src="a.mp3">fallback <b>x</b></audio>',
  '<canvas width="1" height="2">fallback</canvas><meter value="1" min="0" max="2">m</meter><progress value="1" max="2">p</progress>',
  '<select><option value="1" selected>a<option disabled>b<optgroup label="g"><option>c</select>',
  '<select><div>d</div><b>bold</b><option>o</option><hr><span>s</span></select>',
  '<select><option>a<p>b</p></option></select>',
  '<select><selectedcontent>x</selectedcontent><button>b</button><option>o</option></select>',
  '<datalist id="d"><option value="x"></datalist><input list="d" type="text" required readonly disabled>',
  '<button type="submit" name="n" value="v" disabled formaction="x">b</button>',
  '<textarea rows="2" cols="3" name="t" placeholder="p" wrap="hard">x</textarea>',
  '<map name="m"><area shape="rect" coords="0,0,1,1" href="x" alt="a"></map><img usemap="#m" ismap>',
  '<search><p>x</p></search><hgroup><h1>h</h1><p>p</p></hgroup><menu><li>m</menu>',
  '<slot name="s">x</slot><shadow>y</shadow><content>z</content><element>e</element><decorator>d</decorator>',
  '<menuitem>x</menuitem><spacer>y</spacer><acronym>z</acronym><big>b</big><tt>t</tt><strike>s</strike><dir><li>d</dir>',
  // Tables and foster parenting.
  '<table><tr><td>a</td><td>b</td></tr></table>',
  '<table><thead><tr><th>h</th></tr></thead><tbody><tr><td>b</td></tr></tbody><tfoot><tr><td>f</td></tr></tfoot></table>',
  '<table><caption>c</caption><colgroup><col span="2"></colgroup><tr><td>x</td></tr></table>',
  '<table>text<b>bold</b><tr><td>x</td></tr>tail</table>',
  '<table><tr>x<td>y</td>z</tr></table>',
  '<table><script>1</script><style>a{}</style><tr><td>x</td></tr></table>',
  '<table><td>x</td></table><tr><td>y</td></tr>',
  '<table><tbody><tr><td><table><tr><td>inner</td></tr></table></td></tr></tbody></table>',
  '<table><a href="x">a<tr><td>b</td></tr></a></table>',
  '<td>x</td><tr>y</tr><th>z</th><thead>t</thead><caption>c</caption><col><colgroup>',
  '<table><form><input name="x"></form><tr><td><input name="y"></td></tr></table>',
  '<table><template><tr><td>x</td></tr></template></table>',
  '<table><svg><rect/></svg><math><mi>x</mi></math></table>',
  // Unclosed and misnested.
  '<b><i>x',
  '<b>x</i></b>y',
  '<div><span>x</div>y</span>z',
  '<p><div>x</div></p>',
  '<a href="1">a<a href="2">b</a>c</a>',
  '<b><p>x</b>y</p>',
  '<b><b><b><b><b><b><b><b><b><b>x</b></b></b></b></b></b></b></b></b></b>',
  '<a><b><a>x</a></b></a>',
  '<nobr>a<nobr>b</nobr>c</nobr>',
  '<button>a<button>b</button>c</button>',
  '<li>a<li>b<dd>c<dt>d',
  '<h1>a<h2>b</h1>c</h2>',
  '<div <b>x</b>>y</div>',
  '<div class="a"<b>x</b>',
  '<div/><span/>x<b/>y',
  '<br></br><p></p></p><span></span></span>',
  '</b></i></div>text',
  '<b>x</b ignored="1">y',
  '<b>x</B >y',
  '<a b="c"d="e"f>x</a>',
  "<a b='c'd='e'>x</a>",
  '<a b = "c" d = e >x</a>',
  '<a\nb\n=\n"c"\n>x</a>',
  '<a/b/c="d">x</a>',
  '<a "b"=\'c\' <d>x</a>',
  '<img src="a"/><img/src="b"><img src="c"alt="d">',
  '<image src="x"><isindex><keygen><bgsound><basefont><param name="n"><nextid>',
  '<marquee onstart="x" behavior="alternate" loop="1">m</marquee>',
  '<body onload="x" background="y"><b>z</b>',
  '<body><body class="second">x',
  '<html class="h"><body class="b">x</body></html><html id="late">',
  'x</body>y</html>z<b>w</b>',
  'x</body><!-- after body -->y',
  'x</html><!-- after html -->y',
  // Template expressions (SAFE_FOR_TEMPLATES configs).
  '<b>{{ x }}</b>',
  '<b title="{{x}}">a {{b}} c</b>',
  '<b>${x}</b><i><%= y %></i>',
  '<b>a }} b {{ c</b>',
  '<b>a %> b <% c</b>',
  '{{x}}<b>y</b>${z}',
  '<b>a{</b><i>{b}}</i>',
  // Deep nesting.
  '<div>'.repeat(40) + 'x' + '</div>'.repeat(40),
  '<b>'.repeat(60) + 'x',
  '<svg>' + '<g>'.repeat(50) + '<rect/>' + '</g>'.repeat(50) + '</svg>',
  '<foo>'.repeat(30) + 'x' + '</foo>'.repeat(30),
  '<table><tr><td>'.repeat(12) + 'x',
  '<ul><li>'.repeat(25) + 'x',
  '<a>'.repeat(20) + 'x',
  '<p>'.repeat(30) + 'x',
  '<span><template>'.repeat(8) + '<b onclick=x>y</b>',
  '<foo><bar>'.repeat(10) + '<script>1</script>x' + '</bar></foo>'.repeat(10),
  // Mermaid-like labels.
  'Line 1<br>Line 2',
  'Line 1<br/>Line 2<br />Line 3',
  '<b>Bold</b> and <i>italic</i> and <u>underline</u>',
  "<span style='color:red'>red</span>",
  '<div class="label">text</div>',
  'fa:fa-car Car <i class="fa fa-car"></i>',
  '`markdown **bold** _it_`',
  'A -->|text| B',
  'a ==> b <-- c <-> d',
  'x <= y && y >= z',
  'List<String> items',
  'Map<K, V>',
  'std::vector<int>',
  'if (a < b) then',
  '<<interface>>',
  '<<abstract>> Shape',
  '~T~ generic',
  '$$x^2 < y$$',
  '<img src="https://example.com/a.png" width="30" height="30" />',
  "<img src='x.png' onerror='alert(1)'/>",
  '<a href="https://mermaid.js.org" target="_blank">link</a>',
  "<a href='javascript:alert(1)'>click</a>",
  '<center>centered</center>',
  '<sub>sub</sub><sup>sup</sup>',
  '<h1>Title</h1><p>Body</p>',
  '<input type="checkbox" checked disabled> task',
  '<code>x < y</code>',
  '<pre>  two\n  lines</pre>',
  '&lt;div&gt; escaped',
  '#lt;div#gt;',
  'text with "quotes" and \'single\'',
  'unicode: \u00e9\u00e8 \u4e2d \ud83d\ude80',
  '<p>',
  '</p>',
  '<p/>',
  '<span>a</span><span>b</span>',
  '<style>.x{color:red}</style>styled',
  '<script>alert(1)</script>scripted',
  '<iframe src="x"></iframe>framed',
  // Attribute order of formatting elements. golang.org/x/net/html sorts them.
  '<a href="h" class="c" target="t">x</a>',
  '<A Z="1" A="2" HREF=x>caps</A>',
  '<b z="1" a="2">x</b><i z="1" a="2">y</i><em z="1" a="2">z</em><strong z="1" a="2">w</strong>',
  '<font size=1 color=red face=x>f</font><code z=1 a=2>c</code><u z=1 a=2>u</u><s z=1 a=2>s</s>',
  '<small z=1 a=2>s</small><big z=1 a=2>b</big><tt z=1 a=2>t</tt><strike z=1 a=2>k</strike><nobr z=1 a=2>n</nobr>',
  '<span z="1" a="2">not formatting</span><div z="1" a="2">d</div>',
  '<p><b z="1" a="2"><i y="1" b="2">x</p>y',
  '<a z="1" a="2"><table><a y="1" b="2">x</a></table></a>',
  '<b z=1 a=2 z=3>dup</b>',
  '<b a="1" z="2">x</b><b z="2" a="1">y</b><b a="1" z="2">z</b>',
  '<b z="1" a="2"><b z="1" a="2"><b z="1" a="2"><b z="1" a="2">x</p>y',
  '<svg><title><b z="1" a="2">x</b></title></svg>',
  '<svg><desc><i z="1" a="2">x</i></desc><foreignObject><em z="1" a="2">y</em></foreignObject></svg>',
  '<svg><font size="1" color="c">f</font><a z="1" a="2">svg a</a></svg>',
  '<math><mi><b z="1" a="2">x</b></mi><mtext><i z="1" a="2">y</i></mtext></math>',
  'a<noscript><b z="1" a="2">x</b></noscript>',
  'a<textarea><b z="1" a="2"></textarea><b z="2" a="1">y</b>',
  'a<!-- <b z="1" a="2"> --><b a="2" z="1">x</b>',
  'a<svg><![CDATA[<b z="1" a="2">]]></svg><b a="2" z="1">x</b>',
  'a<![CDATA[<b z="1" a="2">]]><b a="2" z="1">x</b>',
  '<b z="&#x31;" a="&amp;">x</b>',
  '<i class="fa fa-car" aria-hidden="true"></i><a class="l" href="#x">y</a>',
  // U+0000 in every tokenizer state.
  '<title>a\u0000b</title>x<b>y</b>',
  'x<!--a\u0000b-->y<b>z</b>',
  'a<textarea>x\u0000y</textarea>',
  'a<xmp>x\u0000</xmp><plaintext>y\u0000',
  '<math><mi>\u0000x</mi></math>',
  '<div a\u0000b="1" c="\u0000">x</div>',
  '<svg><g a\u0000b="1"><![CDATA[x\u0000y]]></g></svg>',
  // Nesting deeper than the limit of 512 of the parsers.
  '<div>'.repeat(600) + 'x',
  '<b>'.repeat(530) + 'x' + '</b>'.repeat(530),
  '<svg>' + '<g>'.repeat(600) + '<rect/>' + '</g>'.repeat(600) + '</svg>y',
  '<foo>'.repeat(511) + 'x',
  '<i>'.repeat(505) + '<span>y</span>',
  // Processing instructions. Chromium keeps them as nodes.
  'a<?x?>b',
  'a<?x y?>b',
  'a<?x y>z?>b',
  'a<?x  y  ?>b',
  'a<?>b',
  'a<??>b',
  'a<? x?>b',
  'a<?x',
  'a<?x y',
  'a<?x y?',
  'a<?1x y?>b',
  'a<?x-y.z:w v?>b',
  'a<?x y?z?>b',
  'a<?X Y?>b',
  '<p>t<?x y?></p>',
  '<svg><?x y?></svg>',
  '<p>&lt;x<?x y?></p>',
  'a<!--?x y?-->b',
  'a<?x y?>b<!--?x y?-->c<?x y?>d',
  '<div><?x y?><b>t</b><?z?></div>',
  'a<?x &amp; y?>b',
  // Formatting elements that the parser reconstructs, with and without an
  // is attribute.
  '<p><b>a</p>b',
  '<p><b is="x">a</p>b',
  '<b>a<p>b</b>c',
  '<b is="x">a<p>b</b>c',
  '<p><i class="k" is="x">a</p>b<p>c',
  '<p><a href="h" is="x">a</p>b',
  '<div><span is="x">a</div>b',
];

const TAGS = [
  'b', 'i', 'u', 'p', 'div', 'span', 'a', 'img', 'br', 'hr', 'em', 'strong', 'code', 'pre', 'ul', 'li',
  'table', 'tr', 'td', 'th', 'tbody', 'thead', 'h1', 'h3', 'center', 'font', 'small', 'sub', 'sup',
  'form', 'input', 'button', 'select', 'option', 'textarea', 'label', 'fieldset', 'output', 'object',
  'script', 'style', 'iframe', 'noscript', 'noembed', 'noframes', 'xmp', 'title', 'template',
  'svg', 'g', 'rect', 'circle', 'path', 'text', 'tspan', 'foreignObject', 'defs', 'marker', 'use',
  'image', 'desc', 'clipPath', 'linearGradient', 'animate', 'set', 'switch', 'symbol', 'feBlend',
  'math', 'mi', 'mo', 'mn', 'ms', 'mtext', 'mrow', 'annotation-xml', 'semantics', 'mglyph',
  'foo', 'x-custom', 'unknown', 'FoO', 'DIV', 'SVG', 'A', 'Style', 'body', 'head', 'html', 'frameset',
  'video', 'audio', 'source', 'picture', 'details', 'summary', 'dialog', 'canvas', 'embed', 'base',
  'link', 'meta', 'caption', 'col', 'colgroup', 'dl', 'dt', 'dd', 'nobr', 'plaintext', 'listing',
  'marquee', 'applet', 'area', 'map', 'selectedcontent', 'search', 'slot',
];

const VOID = new Set(['img', 'br', 'hr', 'input', 'embed', 'base', 'link', 'meta', 'col', 'source', 'area']);

const ATTR_NAMES = [
  'id', 'name', 'class', 'style', 'title', 'href', 'src', 'xlink:href', 'alt', 'target', 'rel',
  'onclick', 'onerror', 'onload', 'ONMOUSEOVER', 'data-x', 'data-foo-bar', 'aria-label', 'aria-hidden',
  'width', 'height', 'x', 'y', 'd', 'fill', 'stroke', 'transform', 'viewBox', 'viewbox', 'xmlns',
  'xmlns:xlink', 'xml:space', 'xml:lang', 'is', 'for', 'form', 'type', 'value', 'action', 'method',
  'srcset', 'poster', 'background', 'cite', 'lang', 'dir', 'role', 'tabindex', 'hidden', 'open',
  'foo', 'x-y', 'attributeName', 'dominant-baseline', 'text-anchor', 'marker-end', 'font-family',
  'encoding', 'shadowrootmode', 'slot', 'part', 'itemprop', 'srcdoc', 'formaction', 'patchsrc',
  'data-temp-href-target', 'colspan', 'checked', 'disabled', 'placeholder', 'label', 'summary',
  'pattern', 'requiredExtensions', 'preserveAspectRatio', 'gradientUnits', 'stdDeviation',
];

const ATTR_VALUES = [
  '', 'x', ' x ', 'a b', 'title', 'body', 'attributes', 'nodeName', 'location', 'cookie', 'f', 'my-id',
  'javascript:alert(1)', ' JaVaScRiPt:alert(1)', 'java\tscript:x', 'data:text/html,x', 'data:image/png;base64,AA',
  'http://example.com/', 'https://example.com/a?b=c&d=e', '//example.com', '#anchor', '/path', 'mailto:a@b.c',
  'vbscript:x', 'foo:bar', 'url(#a)', 'url(javascript:x)', 'color:red', 'color:red;background:url(x)',
  '_blank', '_self', 'noopener', 'href', 'xlink:href', 'x-y', 'text/html', 'application/xhtml+xml',
  '0 0 10 10', 'translate(1,2)', 'M0 0L1 1', '100%', '12px', 'true', 'false', 'image', 'open',
  '</style>', '</script>', '-->', '--!>', ']>', 'a>b', 'a<b', '<b>x</b>', '"', "'", 'a"b', "a'b", '`',
  '&amp;', '&lt;x', '&#x6A;avascript:x', 'a&nbsp;b', '\u00a0x\u00a0', '{{x}}', '${x}', '<%x%>', '/>',
  '\n', 'a\nb', ' \t\n', 'x\u0000y', '\u00e9', '\ud83d\ude00', 'user-content-x', 'constructor', '__proto__',
];

const TEXTS = [
  'text', ' ', '\n', 'a b', 'x < y', 'x > y', 'a & b', '&amp;', '&lt;b&gt;', '&nbsp;', '\u00a0', '&copy;',
  '&#60;', '&unknown;', '<', '>', '&', '{{x}}', '${x}', '<%x%>', '}}', '--', '-->', ']]>', '</', '<!',
  'a{color:red}', 'a>b{}', '@import "x";', '/* c */', 'alert(1)', '\u4e2d\u6587', '\ud83d\ude00', '\r\n',
  '#quot;', 'x\u0000y', '  lead', 'trail  ', '\t', '</style', '</script', '<br>', '<br/>',
];

const SNIPPETS = [
  '<!-- c -->', '<!---->', '<!-- <b>x</b> -->', '<!--x', '<![CDATA[x<y]]>', '<?pi x?>', '<!DOCTYPE html>',
  '</p>', '</br>', '</div>', '</svg>', '</math>', '</b>', '</form>', '</table>', '</template>', '</select>',
  '</style>', '</script>', '</title>', '</foreignObject>', '</x>', '<', '<<', '>', '</', '<a ', '<a b=',
];

function pick(rnd, list) {
  return list[Math.floor(rnd() * list.length)];
}

function quoteAttr(rnd, value) {
  const r = rnd();
  if (r < 0.7) {
    return '"' + value.replace(/"/g, r < 0.35 ? '&quot;' : "'") + '"';
  }
  if (r < 0.85) {
    return "'" + value.replace(/'/g, '&#39;') + "'";
  }
  // Unquoted: only when the value has no character that ends it.
  if (value !== '' && !/[\s"'`=<>]/.test(value)) {
    return value;
  }
  return '"' + value.replace(/"/g, '&quot;') + '"';
}

function genAttrs(rnd) {
  let out = '';
  const n = rnd() < 0.45 ? 0 : 1 + Math.floor(rnd() * 4);
  for (let i = 0; i < n; i++) {
    const name = pick(rnd, ATTR_NAMES);
    const sep = rnd() < 0.9 ? ' ' : pick(rnd, ['\n', '\t', '  ', '/']);
    if (rnd() < 0.08) {
      out += sep + name;
    } else {
      out += sep + name + '=' + quoteAttr(rnd, pick(rnd, ATTR_VALUES));
    }
  }
  return out;
}

function genNode(rnd, depth) {
  const r = rnd();
  if (depth <= 0 || r < 0.28) {
    return pick(rnd, TEXTS);
  }
  if (r < 0.36) {
    return pick(rnd, SNIPPETS);
  }
  const tag = pick(rnd, TAGS);
  let out = '<' + tag + genAttrs(rnd);
  const lower = tag.toLowerCase();
  if (VOID.has(lower)) {
    return out + (rnd() < 0.3 ? '/>' : '>');
  }
  if (rnd() < 0.06) {
    return out + '/>';
  }
  out += '>';
  const kids = Math.floor(rnd() * 4);
  for (let i = 0; i < kids; i++) {
    out += genNode(rnd, depth - 1);
  }
  // Some elements stay open.
  if (rnd() < 0.88) {
    out += '</' + tag + '>';
  }
  return out;
}

// generated returns n generated inputs.
export function generated(n, seed) {
  const rnd = mulberry32(seed);
  const out = [];
  while (out.length < n) {
    let s = '';
    if (rnd() < 0.12) {
      s += pick(rnd, [' ', '\n', '\t', '\r\n', '  \n', '\r']);
    }
    const parts = 1 + Math.floor(rnd() * 3);
    for (let i = 0; i < parts; i++) {
      s += genNode(rnd, 1 + Math.floor(rnd() * 4));
    }
    if (rnd() < 0.08) {
      s += pick(rnd, [' ', '\n', '\t']);
    }
    if (rnd() < 0.05) {
      // Truncate, to get an unfinished tag or attribute.
      s = s.slice(0, Math.floor(rnd() * s.length));
    }
    out.push(s);
  }
  return out;
}

// svgMutations returns hostile variants of a clean SVG: each variant has one
// hostile piece injected after a tag of the input.
export function svgMutations(svg, n, seed) {
  const rnd = mulberry32(seed);
  const cuts = [];
  for (let i = svg.indexOf('>'); i >= 0 && i < svg.length - 1; i = svg.indexOf('>', i + 1)) {
    cuts.push(i + 1);
  }
  const out = [];
  for (let k = 0; k < n && cuts.length > 0; k++) {
    const at = pick(rnd, cuts);
    out.push(svg.slice(0, at) + genNode(rnd, 2) + svg.slice(at));
  }
  return out;
}

// unique removes duplicates and keeps the first occurrence.
export function unique(list) {
  return [...new Set(list)];
}

// corpusLabels extracts label-like strings from the corpus fixtures: quoted
// strings, text in brackets, and the text after a colon.
export function corpusLabels() {
  const set = new Set();
  const patterns = [
    /"([^"\n]*)"/g,
    /\[([^\]\n]*)\]/g,
    /\(([^)\n]*)\)/g,
    /\{([^}\n]*)\}/g,
    /\|([^|\n]*)\|/g,
    /:[ \t]*([^\n]*)$/gm,
  ];
  for (const file of walk(CORPUS, '.mmd')) {
    const text = fs.readFileSync(file, 'utf8').replace(/^---\n[\s\S]*?\n---\n/, '');
    for (const re of patterns) {
      for (const m of text.matchAll(re)) {
        if (m[1]) {
          set.add(m[1]);
        }
      }
    }
  }
  return [...set];
}
