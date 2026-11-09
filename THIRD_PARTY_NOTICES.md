# Third-party notices

This repository has code that is ported from other projects, files of other projects that are embedded unchanged, and data from other projects. This document lists each project with its version, its home page, its license, its copyright lines and its place in this repository. Section 7 has the license texts.

[docs/PROVENANCE.md](docs/PROVENANCE.md) says which Go package comes from which source. [docs/LICENSE-AUDIT.md](docs/LICENSE-AUDIT.md) has the findings of the license audit. Some rows of this document depend on a decision of the owner of the repository. The notes of those rows say so.

The original code of the repository is under the MIT license. The file `LICENSE` in the root has its text. That license does not replace the licenses of this document: code that is ported from another project keeps the terms of its source.

A fix pass on 2026-11-09 added license files and notice files to the packages under `internal/`, next to the code that they are for. The tables name them. A text that the fix pass copied into the repository has the letter R, with the place that it was copied from.

## How the lines and the texts were taken

Each copyright line and each license text in this document was read from a file. No line was written from memory. The column "Source of the lines and the text" starts with a letter that says where the file is.

| Letter | Place | In git |
|---|---|---|
| R | A file of this repository | yes |
| N | `tools/reference/node_modules`, the npm packages of the reference tools | no |
| U | `.upstream`, the source snapshots of mermaid 12.2.0 and Eclipse ELK 0.9.1 | no |
| G | The Go module cache (`$GOMODCACHE`), or the Go distribution | no |
| C | `LICENSE.headless_shell`, the license file of the Chromium 156.0.8078.4 build that Playwright 1.64.1 installs. The reference tests use this browser | no |
| F | A file that was fetched from the upstream project on 2026-11-09, for the audit or for the fix pass. The URL is given. No copy is on the developer machine outside of a temporary directory | no |

When a license file has no copyright line and no other local file has one, the cell says "copyright line not found locally". Section 6 lists those projects.

One license text can serve several projects. When two projects have the same license words, section 7 has the text one time, and the copyright lines are in the tables. When the words differ, section 7 has each text.

## 1. Code that is ported to Go

The Go files in the column "Where in this repository" are translations of the upstream code, or follow it closely. They are derivative works of the upstream code.

### 1.1 mermaid and the libraries in its bundle

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| mermaid | 12.2.0 | https://github.com/mermaid-js/mermaid | MIT | Copyright (c) 2014 - 2022 Knut Sveidqvist | `internal/diagrams`, `internal/ru`, `internal/render`, `internal/config`, `internal/styles`, `internal/theme`, `internal/utils`, `internal/sanitize`, `internal/markdown`, `internal/svgdraw`, `internal/grammar`, `internal/langium`, `testdata/corpus` | MIT | N: `tools/reference/node_modules/mermaid/LICENSE` | `internal/grammar/NOTICE` has a copy of the license, for the grammar tables and the files `actions.js.txt`. |
| @mermaid-js/mermaid-cli | 12.0.0 | https://github.com/mermaid-js/mermaid-cli | MIT | Copyright (c) 2017-2026 Tyler Long, Mindaugas Laganeckas, Alois Klink | `internal/mmdc` (`page.go`, `fontembed.go`, `tokenizecss.go`), `cmd/mmdg/main.go` | MIT | N: `tools/reference/node_modules/@mermaid-js/mermaid-cli/LICENSE` |  |
| @mermaid-js/layout-tidy-tree | 1.0.1 | https://github.com/mermaid-js/mermaid | MIT | Copyright (c) 2014 - 2022 Knut Sveidqvist | `internal/ru/layout/tidytree` (`layout.go`, `render.go`) | MIT | N: `tools/reference/node_modules/@mermaid-js/layout-tidy-tree/LICENSE` | mermaid-cli registers this layout. The port is used for the page of mermaid-cli. |
| non-layered-tidy-tree-layout | 2.0.2 | https://github.com/stetrevor/non-layered-tidy-tree-layout | MIT | copyright line not found locally | `internal/ru/layout/tidytree` (`algorithm.go`, `helpers.go`) | MIT | The source map of `@mermaid-js/layout-tidy-tree` 1.0.1 names the package and the version. The npm registry gives the license `MIT` and the author Michael Wong | The package is bundled into `@mermaid-js/layout-tidy-tree`. It is not installed as a package of its own, so no license file of it is local. |
| @mermaid-js/parser | 2.0.1 | https://github.com/mermaid-js/mermaid/tree/develop/packages/mermaid/parser/#readme | MIT | Copyright (c) 2023 Yokozuna59 | `internal/langium`, the grammar files of the Langium diagrams in `internal/diagrams` | MIT | N: `tools/reference/node_modules/@mermaid-js/parser/LICENSE` | The package is `packages/parser` of the mermaid repository. |
| dagre-d3-es | 7.0.14 | https://github.com/tbo47/dagre-es | MIT | Original dagre-d3 copyright: Copyright (c) 2013 Chris Pettitt<br>Original dagre copyright: Copyright (c) 2012-2014 Chris Pettitt<br>Original graphlib copyright: Copyright (c) 2012-2014 Chris Pettitt<br>Copyright (c) 2022-2024 Thibaut Lassalle, David Newell, Alois Klink, Sidharth Vinod and dagre-es contributors | `internal/dagre`, `internal/graphlib` | MIT | N: `tools/reference/node_modules/dagre-d3-es/LICENSE.md` | The license file also has the copyright lines of dagre, dagre-d3 and graphlib. |
| lodash-es | 4.18.1 | https://lodash.com/custom-builds | MIT | Copyright OpenJS Foundation and other contributors <https://openjsf.org/><br>Based on Underscore.js, copyright Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors <http://underscorejs.org/> | `internal/dagre/lodash.go`, `internal/dagre/state.go`. Also inside `internal/cytoscape/cytoscape.bundle.js` | lodash | N: `tools/reference/node_modules/lodash-es/LICENSE` | The license file waives the copyright of sample code in the documentation under CC0. No sample code is in this repository. `internal/cytoscape/NOTICE` has a copy of the license file, for the functions of lodash inside cytoscape. |
| es-toolkit | 1.45.1 | https://es-toolkit.dev | MIT | Copyright (c) 2024 Viva Republica, Inc<br>Copyright OpenJS Foundation and other contributors | `internal/utils/merge.go`, `internal/config/merge.go`, `internal/styles/merge.go` | MIT | F: https://unpkg.com/es-toolkit@1.45.1/LICENSE | The three ports follow 1.45.1, the version in the mermaid bundle. After the two copyright lines, the license file of 1.45.1 says: "Parts of the test suite and compatibility layer in `es-toolkit/compat` are derived from Lodash (https://github.com/lodash/lodash) by the OpenJS Foundation (https://openjsf.org/) and Underscore.js by Jeremy Ashkenas, DocumentCloud and Investigative Reporters & Editors (http://underscorejs.org/)". The lodash notice is in the row above. Version 1.45.1 has no `NOTICE` file. The local package is 1.53.1. Its license file has one copyright line, and it has a `NOTICE` file. |
| khroma | 2.1.0 | https://github.com/fabiospampinato/khroma | MIT | Copyright (c) 2019-present Fabio Spampinato, Andrew Maney | `internal/khroma` | MIT | N: `tools/reference/node_modules/khroma/license` |  |
| stylis | 4.3.6 | https://github.com/thysultan/stylis.js | MIT | Copyright (c) 2016-present Sultan Tarimo | `internal/stylis` | MIT | N: `tools/reference/node_modules/stylis/LICENSE` | The port follows 4.3.6, the version in the mermaid bundle. The local license file is that of 4.4.0. The license file of 4.3.6 (https://unpkg.com/stylis@4.3.6/LICENSE) has the same bytes. |
| roughjs | 4.6.6 | https://roughjs.com | MIT | Copyright (c) 2019 Preet Shihn | `internal/rough` | MIT | N: `tools/reference/node_modules/roughjs/LICENSE` |  |
| hachure-fill | 0.5.2 | https://github.com/pshihn/hachure-fill#readme | MIT | Copyright (c) 2023 Preet Shihn | `internal/rough/hachure_fill.go` | MIT | N: `tools/reference/node_modules/hachure-fill/LICENSE` |  |
| path-data-parser | 0.1.0 | https://github.com/pshihn/path-data-parser#readme | MIT | Copyright (c) 2020 Preet Shihn | `internal/rough/path_data_parser.go` | MIT | N: `tools/reference/node_modules/path-data-parser/LICENSE` |  |
| points-on-curve | 0.2.0 | https://github.com/pshihn/bezier-points#readme | MIT | Copyright (c) 2020 Preet Shihn | `internal/rough/points_on_curve.go` | MIT | N: `tools/reference/node_modules/points-on-curve/LICENSE` |  |
| points-on-path | 0.2.1 | https://github.com/pshihn/points-on-path#readme | MIT | Copyright (c) 2020 Preet | `internal/rough/points_on_path.go` | MIT | N: `tools/reference/node_modules/points-on-path/LICENSE` |  |
| dayjs | 1.11.21 | https://day.js.org | MIT | Copyright (c) 2018-present, iamkun | `internal/dayjs` | MIT | N: `tools/reference/node_modules/dayjs/LICENSE` | The local license file is that of 1.11.23. |
| @upsetjs/venn.js | 2.0.0 | https://github.com/upsetjs/venn.js | MIT | Copyright (c) 2013 Ben Frederickson<br>Copyright (c) 2021 Samuel Gratzl | `internal/venn` | MIT | N: `tools/reference/node_modules/@upsetjs/venn.js/LICENSE` |  |
| @iconify/utils | 3.0.2 | https://iconify.design/docs/libraries/utils/ | MIT | Copyright (c) 2021-PRESENT Vjacheslav Trushkin | `internal/ru/icons_util.go` | MIT | N: `tools/reference/node_modules/@iconify/utils/license.txt` | The local license file is that of 3.1.7. |
| @braintree/sanitize-url | 7.1.2 | https://github.com/braintree/sanitize-url#readme | MIT | Copyright (c) 2017 Braintree | `internal/sanitize/sanitize_url.go` | MIT | N: `tools/reference/node_modules/@braintree/sanitize-url/LICENSE` |  |
| ts-dedent | 2.2.0 | https://github.com/tamino-martinius/node-ts-dedent | MIT | Copyright (c) 2018 Tamino Martinius | `internal/markdown/dedent.go` | MIT | N: `tools/reference/node_modules/ts-dedent/LICENSE` | The local license file is that of 2.3.0. |
| marked | 16.3.0 | https://marked.js.org | MIT | Copyright (c) 2018+, MarkedJS (https://github.com/markedjs/)<br>Copyright (c) 2011-2018, Christopher Jeffrey (https://github.com/chjj/)<br>Copyright © 2004, John Gruber | `internal/marked` | marked | N: `tools/reference/node_modules/marked/LICENSE.md` | The license file also has the license of Markdown by John Gruber (BSD-3-Clause). The whole file is in section 7. The local file is that of 16.4.2. |
| jison | 0.4.18 | http://jison.org | MIT | Copyright (c) 2009-2014 Zachary Carter | `internal/jison`. The parser driver that jison generates is the model of `internal/jison/parser.go`, and jison made the tables in `internal/grammar` | MIT | N: `tools/reference/node_modules/jison/README.md`, section License | The package has no license file. The text is in its README. `internal/grammar/NOTICE` has a copy of that section. |
| jison-lex | 0.3.4 | http://jison.org | MIT | copyright line not found locally | `internal/jison/lexer.go`, `internal/jison/regex.go` | MIT | N: `tools/reference/node_modules/jison-lex/README.md` says `MIT`, and `regexp-lexer.js` says `MIT Licensed` | The package has no license file and no copyright line. `package.json` names the author: Zach Carter. |
| js-yaml | 4.3.0 | https://github.com/nodeca/js-yaml | MIT | Copyright (C) 2011-2015 by Vitaly Puzrin | `internal/diagrams/agentflow/jsyaml`, `internal/config/yaml.go` | MIT | F: https://unpkg.com/js-yaml@4.3.0/LICENSE | Not in `node_modules`. The mermaid bundle contains it. |
| langium | 4.4.0 | https://langium.org | MIT | Copyright 2021 TypeFox GmbH | `internal/langium` | MIT | F: https://unpkg.com/langium@4.4.0/LICENSE | Not in `node_modules`. The parser package of mermaid contains it. |
| d3-array | 3.2.4 | https://d3js.org/d3-array/ | ISC | Copyright 2010-2023 Mike Bostock | `internal/d3/array` | ISC | N: `tools/reference/node_modules/d3-array/LICENSE` |  |
| d3-axis | 3.0.0 | https://d3js.org/d3-axis/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/axis` | ISC | N: `tools/reference/node_modules/d3-axis/LICENSE` |  |
| d3-color | 3.1.0 | https://d3js.org/d3-color/ | ISC | Copyright 2010-2022 Mike Bostock | `internal/d3/color` | ISC | N: `tools/reference/node_modules/d3-color/LICENSE` |  |
| d3-format | 3.1.0 | https://d3js.org/d3-format/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/format` | ISC | F: https://unpkg.com/d3-format@3.1.0/LICENSE | The port follows 3.1.0, the version in the mermaid bundle. The local license file is that of 3.1.2. It has the line `Copyright 2010-2026 Mike Bostock` and the same license text. |
| d3-hierarchy | 3.1.2 | https://d3js.org/d3-hierarchy/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/hierarchy` | ISC | N: `tools/reference/node_modules/d3-hierarchy/LICENSE` |  |
| d3-interpolate | 3.0.1 | https://d3js.org/d3-interpolate/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/interpolate` | ISC | N: `tools/reference/node_modules/d3-interpolate/LICENSE` |  |
| d3-path | 3.1.0 | https://d3js.org/d3-path/ | ISC | Copyright 2015-2022 Mike Bostock | `internal/d3/path` | ISC | N: `tools/reference/node_modules/d3-path/LICENSE` |  |
| d3-scale | 4.0.2 | https://d3js.org/d3-scale/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/scale` | ISC | N: `tools/reference/node_modules/d3-scale/LICENSE` |  |
| d3-scale-chromatic | 3.1.0 | https://d3js.org/d3-scale-chromatic/ | ISC | Copyright 2010-2024 Mike Bostock | `internal/diagrams/sankey/scheme.go` (the Tableau10 colors) | ISC | N: `tools/reference/node_modules/d3-scale-chromatic/LICENSE` | The license file also has the Apache-style license of the ColorBrewer colors. Those colors are not in this repository. |
| d3-selection | 3.0.0 | https://d3js.org/d3-selection/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/dom/node.go`, `internal/d3/axis/axis.go` | ISC | N: `tools/reference/node_modules/d3-selection/LICENSE` |  |
| d3-shape | 3.2.0 | https://d3js.org/d3-shape/ | ISC | Copyright 2010-2022 Mike Bostock | `internal/d3/shape` | ISC | N: `tools/reference/node_modules/d3-shape/LICENSE` |  |
| d3-time | 3.1.0 | https://d3js.org/d3-time/ | ISC | Copyright 2010-2022 Mike Bostock | `internal/d3/time` | ISC | N: `tools/reference/node_modules/d3-time/LICENSE` |  |
| d3-time-format | 4.1.0 | https://d3js.org/d3-time-format/ | ISC | Copyright 2010-2021 Mike Bostock | `internal/d3/timefmt` | ISC | N: `tools/reference/node_modules/d3-time-format/LICENSE` |  |
| internmap | 2.0.3 | https://github.com/mbostock/internmap/ | ISC | Copyright 2021 Mike Bostock | `internal/d3/scale/band.go` | ISC | N: `tools/reference/node_modules/internmap/LICENSE` |  |
| d3-sankey | 0.12.3 | https://github.com/d3/d3-sankey | BSD-3-Clause | Copyright 2015, Mike Bostock | `internal/d3/sankey` | BSD-3-Clause, text 1 | N: `tools/reference/node_modules/d3-sankey/LICENSE` |  |
| d3-shape | 1.3.7 | https://github.com/d3/d3-shape | BSD-3-Clause | Copyright 2010-2015 Mike Bostock | `internal/d3/sankey/link_horizontal.go` | BSD-3-Clause, text 1 | N: `tools/reference/node_modules/d3-sankey/node_modules/d3-shape/LICENSE` | The version that d3-sankey depends on. |
| fmin | version not stated in the build of venn.js (0.0.4 with a patch, by the `package.json` of venn.js) | https://github.com/benfred/fmin | BSD-3-Clause | Copyright 2016, Ben Frederickson | `internal/venn/fmin.go` | BSD-3-Clause, text 1 | F: https://unpkg.com/fmin@0.0.4/LICENSE | The build `venn.esm.js` of @upsetjs/venn.js contains fmin without a notice. |
| chevrotain | 13.2.0 | https://chevrotain.io/docs/ | Apache-2.0 | copyright line not found locally | `internal/diagrams/usecase/chevrotain.go`, `lexer.go`, `grammar_gen.go`, `internal/langium/lexer.go`, `internal/langium/parser.go` | Apache-2.0 | R: `internal/diagrams/usecase/LICENSE-chevrotain.txt` and `internal/langium/LICENSE-chevrotain.txt`, copies of N: `tools/reference/node_modules/chevrotain/LICENSE.txt` | The package has the license text, no copyright line and no `NOTICE` file. Its source files have no copyright line. The Go files say that they are changed versions. |
| DOMPurify | 3.4.12 | https://github.com/cure53/DOMPurify | MPL-2.0 OR Apache-2.0 | (c) Cure53 and other contributors | `internal/purify` | Apache-2.0 | R: `internal/purify/LICENSE-Apache-2.0.txt`, a copy of N: `tools/reference/node_modules/dompurify/LICENSE` (dompurify 3.4.17) | The upstream license is a choice of two. This port takes DOMPurify under the Apache License 2.0 (finding F5 of the audit), which fits the MIT license of the repository. `internal/purify/NOTICE` says the same. The copyright line is from the banner of DOMPurify 3.4.12 in the mermaid bundle. |

The mermaid bundle also contains these packages. No code of them is in this repository: katex, uuid, fastdom, chevrotain-allstar, vscode-jsonrpc, vscode-languageserver-protocol, vscode-languageserver-textdocument, vscode-languageserver-types, and the d3 modules that the table does not list.

### 1.2 Chromium, Skia, V8 and the C libraries

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| Chromium (Blink, `ui/gfx`, `cc`) | 156 (the reference browser is 156.0.8078.4) | https://www.chromium.org | BSD-3-Clause | Copyright 2015 The Chromium Authors | `internal/browser`, `internal/font` (`linux.go`, `linux_scaler.go`, `fontconfig_match.go`, `emoji_segment.go`, `script_run.go`), `internal/raster` (`svg*.go`), `internal/css`, `internal/dom`, `internal/imagesize` | BSD-3-Clause, text 2 | R: `internal/font/LICENSE-Chromium.txt` | This is the license of the files of Chromium that say `Use of this source code is governed by a BSD-style license`. Many files of Blink have another license. See the next two rows. |
| Blink files that come from WebKit and KHTML, under the LGPL | Chromium 156 | https://www.chromium.org/blink | LGPL-2.0-or-later | The copyright lines differ from file to file. `NOTICE-Blink.txt` in `internal/browser`, `internal/raster`, `internal/dom`, `internal/css` and `internal/font` has the first comment of each Blink file, with its copyright lines. See also the list of holders in section 7 ("WebKit: holders and BSD terms"). | `internal/browser` (`boundschange.go`, `engine.go`, `fontmono.go`, `geom.go`, `htmlinline.go`, `norender.go`, `path.go`, `pathquery.go`, `style.go`, `svgroot.go`, `svgviewport.go`, `text.go`, `transform.go`. `linebreak.go` names one file), `internal/raster` (`render.go`, `svg.go`, `svg_effects.go`, `svg_filter.go`, `svg_gradient.go`, `svg_marker.go`, `svg_text.go`), `internal/dom` (`node.go`, `purify_support.go`, `selector.go`), `internal/css/cssom.go`, `internal/font/xheight.go`. These Go files follow or name Blink files that have this license. The file `NOTICE` in the first four directories and finding F1 of the audit have the list | LGPL-2.0 | R: `internal/browser/LICENSE-LGPL-2.0.txt` and `internal/raster/LICENSE-LGPL-2.0.txt`, copies of F: `third_party/blink/renderer/core/LICENSE-LGPL-2` at the tag `156.0.8078.4` of https://chromium.googlesource.com/chromium/src. C: section WebKit of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1) has the same terms | Open finding. The owner must decide if this code stays (finding F1). The first comment of each Go file names its Blink files and their licenses. The section WebKit of the Chromium license file lists the holders. That list is in section 7 with the license text. |
| Blink files that come from WebKit, under BSD terms | Chromium 156 | https://www.chromium.org/blink | BSD-2-Clause, and BSD-3-Clause in files that name Apple or Google | The copyright lines differ from file to file. `NOTICE-Blink.txt` in each directory has the first comment of each of these Blink files, with its copyright lines, its conditions and its disclaimer. See also the list of holders in section 7 ("WebKit: holders and BSD terms"). | `internal/browser` (`geom.go`, `path.go`, `style.go`, `svgroot.go`, `text.go`, `transform.go`), `internal/imagesize/imagesize.go`, `internal/dom/xml.go`, `internal/css/color.go`, `internal/font` (`linux.go`, `linux_scaler.go`, `face.go`), `internal/raster` (`svg.go`, `svg_filter.go`, `svg_marker.go`, `svg_text.go`). Examples of the upstream files: `platform/transforms/affine_transform.cc`, `platform/geometry/path.cc`, `core/svg/graphics/svg_image.cc`, `core/editing/serializers/markup_accumulator.cc`, `platform/graphics/color.cc`, `platform/fonts/font_cache.cc`, `platform/fonts/simple_font_data.cc`, `core/paint/filter_effect_builder.cc` | WebKit: holders and BSD terms | R: `NOTICE-Blink.txt` in `internal/browser`, `internal/raster`, `internal/dom`, `internal/css`, `internal/font` and `internal/imagesize`, copied from F: the Blink files at the tag `156.0.8078.4` of https://chromium.googlesource.com/chromium/src. C: section WebKit of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1) | The first comment of each Go file gives the license of each Blink file that it names. |
| Skia | branch chrome/m156. `internal/font` names revision aa789d9b | https://skia.org | BSD-3-Clause | Copyright (c) 2011 Google Inc. All rights reserved. | `internal/raster`, `internal/browser` (path geometry), `internal/font` (`linux_scaler.go`, `autohint.go`, `linux.go`) | BSD-3-Clause, text 3 | R: `internal/font/LICENSE-Skia.txt` | The text in `internal/font/LICENSE-Skia.txt` is the same as https://skia.googlesource.com/skia/+/refs/heads/chrome/m156/LICENSE. The `DEPS` file of Chromium has Skia at revision `aa789d9bd566` at the tag `156.0.8078.80`, and at revision `95ee33d7ec77` at the tag `156.0.8078.4`. |
| V8 | 15.6.75 (`internal/js/crmath`). Tags 14.8-lkgr and 15.6-lkgr (`internal/js/sort_dagre.go`). No version in `internal/js/date_gantt.go` | https://v8.dev | BSD-3-Clause | Copyright 2014, the V8 project authors. All rights reserved. | `internal/js/crmath` (`pow.go`, `hyp.go`, `misc.go`), `internal/js/date_gantt.go`, `internal/diagrams/usecase/json.go` (`src/json/json-parser.cc`), and the number and date code of `internal/js` that follows `src/numbers/conversions.cc` and `src/date` | BSD-3-Clause, text 4 | R: `internal/js/LICENSE-V8.txt`, a copy of F: the file `LICENSE` of V8 at revision `b976f35da835` of https://chromium.googlesource.com/v8/v8. That revision is V8 15.6.75, and the `DEPS` file of Chromium 156.0.8078.4 names it | `internal/js/LICENSE-V8.txt` is the whole file `LICENSE` of V8. It starts with a list of libraries inside V8 that have other licenses. A copy of the same license text with the same copyright line is in the Go module cache: `$GOMODCACHE/github.com/dop251/goja@.../ftoa/internal/fast/LICENSE_V8`. |
| fdlibm, and the fdlibm code in V8 (`src/base/ieee754.cc`) | fdlibm 5.3. V8 15.6.75 | https://www.netlib.org/fdlibm | SunPro | Copyright (C) 1993-2004 by Sun Microsystems, Inc. All rights reserved.<br>Copyright (C) 1993 by Sun Microsystems, Inc. All rights reserved.<br>Copyright (C) 2004 by Sun Microsystems, Inc. All rights reserved. | `internal/js/crmath/linux_libm_tanh.go` (fdlibm `s_tanh.c`, `s_expm1.c`), `internal/js/crmath/hyp.go` (V8), `internal/browser/linux_trig.go` (the single precision files of fdlibm as NetBSD has them), and the test files `internal/elk/fdlibm_ref_test.go`, `internal/rough/fdlibm_test.go`, `internal/venn/fdlibm_ref_test.go` | SunPro | C: section `V8 fork of fdlibm` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1). The notices in `internal/js/crmath/linux_libm_tanh.go` and `internal/browser/linux_trig.go` are the notices of the fdlibm files | The notice must stay in each file that has this code. `internal/js/crmath/hyp.go` has the notice of `src/base/ieee754.cc` of V8 15.6.75, which names 1993 and says "Developed at SunSoft". |
| CPython list sort, as V8 has it (`third_party/v8/builtins/array-sort.tq`) | V8 tags 14.8-lkgr and 15.6-lkgr | https://www.python.org | PSF-2.0 | Copyright (c) 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010, 2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018, 2019, 2020, 2021, 2022, 2023, 2024, 2025, 2026 Python Software Foundation; All Rights Reserved<br>Copyright (c) 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010, 2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018 Python Software Foundation; All Rights Reserved | `internal/js/sort_dagre.go` | PSF-2.0 | R: `internal/js/LICENSE-PSF.txt`, a copy of F: `third_party/v8/builtins/LICENSE` of V8 15.6.75 (https://chromium.googlesource.com/v8/v8). The file is the same at the tags 14.8-lkgr and 15.6-lkgr | The first copyright line is the first comment of `array-sort.tq` at V8 15.6.75 and at the tag 15.6-lkgr. The second line is the first comment of that file at the tag 14.8-lkgr. The license asks that a derivative work keeps the notice and has a short summary of the changes. `internal/js/sort_dagre.go` has both notices and the summary. |
| LLVM libc | revision b8b28272d32b, the revision in Chromium 156 | https://libc.llvm.org | Apache-2.0 WITH LLVM-exception | none in the upstream files | `internal/js/crmath` (`trig.go`, `asin.go`, `atan.go`, `exp.go`, `log.go`, `cbrt.go`, `dd.go`, `tables.go`) | Apache-2.0 and LLVM-exception | R: `internal/js/crmath/LICENSE-LLVM.txt`, a copy of F: `LICENSE.TXT` at revision `b8b28272d32b` of https://chromium.googlesource.com/external/github.com/llvm/llvm-project/libc. C: section `llvm-libc` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1) | The `DEPS` file of Chromium 156.0.8078.4 names that revision. The Go files have the three license lines of the LLVM files and say that they are changed versions. The files of LLVM have no copyright line. Each file says: `Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions.` The license text is the Apache License 2.0 in section 7 with the exception text. |
| Arm Optimized Routines | v20.02 | https://github.com/ARM-software/optimized-routines | MIT | Copyright (c) 1999-2019, Arm Limited. | `internal/browser/linux_trig.go` (`math/sinf.c`, `cosf.c`, `sincosf.h`, `sincosf_data.c`) | MIT | R: `internal/browser/LICENSE-ArmOptimizedRoutines.txt` | The file header has the line `Copyright (c) 2018, Arm Limited.` of the source files. |
| Arm Optimized Routines | v23.01 | https://github.com/ARM-software/optimized-routines | MIT OR Apache-2.0 WITH LLVM-exception | Copyright (c) 1999-2022, Arm Limited. | `internal/js/crmath/linux_libm_pow.go`, `linux_libm_pow_tables.go` (`math/pow.c`, `math_err.c`, `pow_log_data.c`, `exp_data.c`) | MIT | F: https://github.com/ARM-software/optimized-routines/blob/v23.01/LICENSE | The two Go files have the MIT text and the lines `Copyright (c) 2018-2020, Arm Limited.` and `Copyright (c) 2018, Arm Limited.` of the source files. They use the code under the MIT license. |
| HarfBuzz | The Go file does not state the version. The notices are those of revision `f5efbbef3841`, the revision in Chromium 156.0.8078.4 | https://harfbuzz.github.io | MIT-Modern-Variant | Copyright © 2017  Google, Inc. (`src/hb-kern.hh`)<br>Copyright © 2010-2022  Google, Inc.<br>Copyright © 2015-2020  Ebrahim Byagowi<br>Copyright © 2019,2020  Facebook, Inc.<br>Copyright © 2012,2015  Mozilla Foundation<br>Copyright © 2011  Codethink Limited<br>Copyright © 2008,2010  Nokia Corporation and/or its subsidiary(-ies)<br>Copyright © 2009  Keith Stribley<br>Copyright © 2011  Martin Hosken and SIL International<br>Copyright © 2007  Chris Wilson<br>Copyright © 2005,2006,2020,2021,2022,2023  Behdad Esfahbod<br>Copyright © 2004,2007,2008,2009,2010,2013,2021,2022,2023  Red Hat, Inc.<br>Copyright © 1998-2005  David Turner and Werner Lemberg<br>Copyright © 2016  Igalia S.L.<br>Copyright © 2022  Matthias Clasen<br>Copyright © 2018,2021  Khaled Hosny<br>Copyright © 2018,2019,2020  Adobe, Inc<br>Copyright © 2013-2015  Alexei Podtelezhnikov | `internal/font/kern_split.go` (`hb-kern.hh`) | MIT-Modern-Variant | R: `internal/font/LICENSE-HarfBuzz.txt`, a copy of F: `COPYING` at revision `f5efbbef3841` of https://chromium.googlesource.com/external/github.com/harfbuzz/harfbuzz | The first copyright line is the notice of `src/hb-kern.hh` at that revision. `internal/font/kern_split.go` has that notice with its two paragraphs. The other lines are those of `COPYING`. The Go module go-text/typesetting has the notices of HarfBuzz in `harfbuzz/LICENSE`. The section `harfbuzz` of the Chromium license file has them too. |
| FreeType | 2.13.2 | https://freetype.org | FTL | Copyright 1996-2002, 2006 by David Turner, Robert Wilhelm, and Werner Lemberg<br>Portions of this software are copyright (c) 2024 The FreeType Project (www.freetype.org). All rights reserved. | `internal/font/autohint*.go` (through skrifa), `internal/font/linux_scaler.go`, `autohint_fixed.go`, `autohint_glyf.go` | FTL | R: `internal/font/LICENSE-FreeType.txt` | The FreeType License asks for this credit in the documentation: `Portions of this software are copyright (c) 2024 The FreeType Project (www.freetype.org). All rights reserved.` The line is in `internal/font/LICENSE-FreeType.txt`. |
| skrifa, read-fonts, font-types (Fontations) | skrifa 0.46.2, read-fonts 0.43.3, font-types 0.12.5 | https://github.com/googlefonts/fontations | MIT OR Apache-2.0 | Copyright (c) 2019 Fontations Developers | `internal/font/autohint*.go`, `internal/font/linux_scaler.go` | MIT | R: `internal/font/LICENSE-skrifa.txt` | `internal/font/LICENSE-skrifa.txt` says that the code is used under the MIT license. |
| fontconfig | 2.15.0 | https://www.freedesktop.org/wiki/Software/fontconfig | HPND-sell-variant, with other notices in the same file | Copyright © 2000,2001,2002,2003,2004,2006,2007 Keith Packard<br>Copyright © 2005 Patrick Lam<br>Copyright © 2007 Dwayne Bailey and Translate.org.za<br>Copyright © 2009 Roozbeh Pournader<br>Copyright © 2008,2009,2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020 Red Hat, Inc.<br>Copyright © 2008 Danilo Šegan<br>Copyright © 2012 Google, Inc.<br>© 2019 Unicode®, Inc.<br>Copyright © 2007  Chris Wilson<br>Copyright © 2009,2010  Red Hat, Inc.<br>Copyright © 2011,2012,2013  Google, Inc.<br>Copyright © 2002-2003 by Juliusz Chroboczek | `internal/font/fontconfig.go`, `fontconfig_match.go`, `internal/font/fontconfig-default.xml.gz` | fontconfig | R: `internal/font/LICENSE-fontconfig.txt` | The file `COPYING` of fontconfig has several notices. The whole file is in section 7. |
| emoji-segmenter | revision `955936be8b39`, the revision in Chromium 156.0.8078.4 | https://github.com/google/emoji-segmenter | Apache-2.0 | Copyright 2019 Google LLC | `internal/font/emoji_segment.go` (`emoji_presentation_scanner.rl`) | Apache-2.0 | R: `internal/font/LICENSE-emoji-segmenter.txt`, a copy of F: `LICENSE` at that revision of https://chromium.googlesource.com/external/github.com/google/emoji-segmenter. C: section `Emoji Segmenter` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1) | The copyright line is from the first comment of `emoji_presentation_scanner.rl` at that revision. The license file has no copyright line. The repository has no `NOTICE` file at that revision. The Go file says that it is a changed version. |

Two sources have no license that allows a copy. No file of the working tree has code of them now. Old commits of the branch do.

| Source | Where | Status |
|---|---|---|
| `libsystem_m` of Apple (macOS 26.7.1, arm64). It is closed source | Commits `cb905bd` to `d7c85c3` of the branch `rewrite/mermaid-parity` have Go code that follows the machine code of `pow` and `tanh`, and copies of their tables (`internal/js/crmath/apple_libm_*.go`, `tools/reference/gen-jsmath-apple-libm.py`). Commit `41d617f` deleted these files | See finding F2 of the audit |
| glibc 2.39 (LGPL-2.1-or-later) | Commits `316a4c7` to `67a62c9` of the branch have `internal/browser/linux_libm.go`, which says `Ported from glibc 2.39`. Commit `d27c046` replaced it | See finding F3 of the audit |

### 1.3 Eclipse ELK and elkjs

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| Eclipse Layout Kernel (ELK) | 0.9.1, with the change of pull request 955 | https://eclipse.dev/elk | EPL-2.0 | Copyright (c) Kiel University and others. | `internal/elk/native` and every package under it | EPL-2.0 | R: `internal/elk/native/LICENSE` | The copyright line is from `internal/elk/native/NOTICE`. The Java files of ELK have the line with years, for example `Copyright (c) 2010, 2015 Kiel University and others.` |
| elkjs | 0.9.3 | https://github.com/kieler/elkjs | EPL-2.0 | Copyright (c) 2017 Kiel University and others.<br>Copyright (c) 2021 Kiel University and others. | `internal/elk/elkjs/elk-worker.min.js` (embedded), `internal/elk/engine.go` (it replaces `lib/elk-api.js` and patches the worker script), the golden files of `internal/elk` | EPL-2.0 | R: `internal/elk/elkjs/LICENSE.md` and `internal/elk/LICENSE-EPL-2.0.txt` | The copyright lines are from `lib/elk-api.js` and `lib/main.js` of the npm package. `elk-worker.min.js` has no copyright line. `internal/elk/NOTICE` says which files of `internal/elk` are under this license, and `internal/elk/engine.go` has the notice of `lib/elk-api.js`. |
| GWT (the Java runtime emulation) | the version that elkjs 0.9.3 was compiled with. It is not stated | https://www.gwtproject.org | Apache-2.0 | Copyright 2007 Google Inc.<br>Copyright 2008 Google Inc.<br>Copyright 2009 Google Inc.<br>Copyright 2014 Google Inc. | `internal/elk/native/gwt`, `internal/elk/native/prop/enum.go`. Compiled GWT code is also in `internal/elk/elkjs/elk-worker.min.js` | Apache-2.0 | R: `internal/elk/native/gwt/LICENSE-GWT.txt`, a copy of F: https://www.apache.org/licenses/LICENSE-2.0.txt. R: `internal/elk/native/gwt/NOTICE`. F: the headers of 15 Java files under `user/super/com/google/gwt/emul/java` and `dev/core/super` on the main branch of https://github.com/gwtproject/gwt (commit `3e636995f379`) | `elk-worker.js` has no copyright lines. The lines are from the GWT source files on the main branch. Each Go file names its Java files and their lines, and says that it is a changed version. `java/util/Random.java` of GWT also has a header of the Apache Software Foundation, which names a `NOTICE` file. The root of the GWT repository has no file with that name. The license information of GWT is in `distro-source/core/src/COPYING`. |
| Guava, Eclipse EMF, Xtext (compiled code inside the elkjs worker) | the versions that elkjs 0.9.3 was compiled with. `NOTICE.md` of ELK 0.9.1 names Guava 30.1.0 and 33.0.0 | https://github.com/google/guava, https://eclipse.dev/modeling/emf, https://eclipse.dev/Xtext | Apache-2.0 (Guava). EPL-2.0 (EMF, Xtext) | Copyright (C) 2007 The Guava Authors (the form of the notice in a source file of Guava). The notice files of EMF and of Xtext have no copyright line | `internal/elk/elkjs/elk-worker.min.js` only. No Go code ports them | Apache-2.0, EPL-2.0 | R: `internal/elk/elkjs/NOTICE`. U: `.upstream/elk-0.9.1/NOTICE.md`. F: `LICENSE` and `guava/src/com/google/common/collect/Lists.java` at the tag v33.0.0 of https://github.com/google/guava, and `LICENSE` and `NOTICE.md` on the default branches of https://github.com/eclipse-emf/org.eclipse.emf and https://github.com/eclipse-xtext/xtext | The worker file has class names of `com.google.common`, `org.eclipse.emf` and `org.eclipse.xtext`, and no notices. It does not state the versions. The notice files of EMF and of Xtext say: "All content is the property of the respective authors or their employers." |

## 2. Files that are embedded unchanged

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| cytoscape | 3.34.0 | http://js.cytoscape.org | MIT | Copyright (c) 2016-2026, The Cytoscape Consortium. | `internal/cytoscape/cytoscape.bundle.js`. Ported in `internal/cytoscape/native` | MIT | R: `internal/cytoscape/LICENSES.txt` |  |
| layout-base | 1.0.2 and 2.0.1 | https://github.com/iVis-at-Bilkent/layout-base#readme | MIT | Copyright (c) 2019 iVis@Bilkent | `internal/cytoscape/cytoscape.bundle.js`. Ported in `internal/cytoscape/native` | MIT | R: `internal/cytoscape/LICENSES.txt` |  |
| cose-base | 1.0.3 and 2.2.0 | https://github.com/iVis-at-Bilkent/cose-base#readme | MIT | Copyright (c) 2019 - present, iVis@Bilkent. | `internal/cytoscape/cytoscape.bundle.js`. Ported in `internal/cytoscape/native` | MIT | R: `internal/cytoscape/LICENSES.txt` |  |
| cytoscape-cose-bilkent | 4.1.0 | https://github.com/cytoscape/cytoscape.js-cose-bilkent | MIT | Copyright (c) 2016-2018, The Cytoscape Consortium. | `internal/cytoscape/cytoscape.bundle.js`. Ported in `internal/cytoscape/native` | MIT | R: `internal/cytoscape/LICENSES.txt` |  |
| cytoscape-fcose | 2.2.0 | https://github.com/iVis-at-Bilkent/cytoscape.js-fcose | MIT | Copyright (c) 2018 - present, iVis-at-Bilkent. | `internal/cytoscape/cytoscape.bundle.js` | MIT | R: `internal/cytoscape/LICENSES.txt` |  |
| Code inside cytoscape and layout-base |  |  | MIT (Promises/A+ thenable, jQuery event code, bezier and spring functions, lodash, heap). Apache-2.0 (JamaJS, in layout-base 2.0.1) | Copyright (c) 2013-2014 Ralf S. Engelschall (http://engelschall.com)<br>Bezier curve function generator. Copyright Gaetan Renaudeau. MIT License: http://en.wikipedia.org/wiki/MIT_License<br>Runge-Kutta spring physics function generator. Adapted from Framer.js, copyright Koen Bok. MIT License: http://en.wikipedia.org/wiki/MIT_License | `internal/cytoscape/cytoscape.bundle.js` | see `internal/cytoscape/LICENSES.txt` | R: `internal/cytoscape/LICENSES.txt`, last section. R: `internal/cytoscape/NOTICE` for lodash and heap | `LICENSES.txt` has the notices that the files contain. The build of cytoscape contains functions of lodash and heap without a notice. `internal/cytoscape/NOTICE` has the license files of the two packages. The license file of heap 0.2.7 (F: https://unpkg.com/heap@0.2.7/LICENSE) has the MIT text and no copyright line. The file `package.json` of heap names the author: Xueqiao Xu. |
| Liberation fonts | 2.1.5 | https://github.com/liberationfonts/liberation-fonts | OFL-1.1 | Digitized data copyright (c) 2010 Google Corporation with Reserved Font Arimo, Tinos and Cousine.<br>Copyright (c) 2012 Red Hat, Inc. with Reserved Font Name Liberation. | `internal/font/liberation/*.ttf.gz` | OFL-1.1 | R: `internal/font/liberation/LICENSE` | The fonts have Reserved Font Names: Liberation, Arimo, Tinos and Cousine. The OFL does not let a changed version keep these names. |
| Recursive (`@fontsource-variable/recursive`) | 5.3.0 | https://fontsource.org/fonts/recursive | OFL-1.1 | Copyright 2019 The Recursive Project Authors (github.com/arrowtype/recursive) | `internal/webfonts/files/recursive`: `index.css` and 4 WOFF2 files. The rules of `index.css` are also in `internal/mmdc/cssom.json.gz` | OFL-1.1, Recursive | R: `internal/webfonts/files/recursive/LICENSE` | A web font of the page of mermaid-cli 12. `package.json` of the package says `OFL-1.1`. An SVG that `mmdg` writes with embedded fonts has the license text and the font data, as an SVG of mermaid-cli has. |
| Open Sans (`@fontsource/open-sans`) | 5.3.0 | https://fontsource.org/fonts/open-sans | OFL-1.1 | Copyright 2020 The Open Sans Project Authors (https://github.com/googlefonts/opensans) | `internal/webfonts/files/open-sans`: `400.css`, `400-italic.css` and 20 WOFF2 files. The rules of the style sheets are also in `internal/mmdc/cssom.json.gz` | OFL-1.1, Open Sans | R: `internal/webfonts/files/open-sans/LICENSE` | A web font of the page of mermaid-cli 12. `package.json` of the package says `OFL-1.1`. |
| Font Awesome Free (`@fortawesome/fontawesome-free`) | 7.3.1 | https://fontawesome.com | OFL-1.1 (the font files). MIT (the style sheets). `package.json` says `(CC-BY-4.0 AND OFL-1.1 AND MIT)` | Copyright (c) 2026 Fonticons, Inc. (https://fontawesome.com) with Reserved Font Name: "Font Awesome".<br>Copyright 2026 Fonticons, Inc. | `internal/webfonts/files/fontawesome-free`: `css/brands.css`, `css/regular.css`, `css/solid.css`, `css/fontawesome.css` and 3 WOFF2 files under `webfonts`. The rules of the style sheets are also in `internal/mmdc/cssom.json.gz` | Font Awesome Free | R: `internal/webfonts/files/fontawesome-free/LICENSE.txt` | Web fonts and style sheets of the page of mermaid-cli 12. The license file gives three licenses. CC BY 4.0 is for the icons that the package has as SVG and JS files: no such file is in this repository. OFL-1.1 is for the font files, with the Reserved Font Name "Font Awesome". MIT is for all other files, which are the style sheets here. The file asks to keep the attribution comments of the files: the style sheets are unchanged. The brand icons are trademarks of their owners. The text of CC BY 4.0 is not in the license file, which gives its URL: https://creativecommons.org/licenses/by/4.0/. |

The embedded file `internal/elk/elkjs/elk-worker.min.js` is in section 1.3, in the rows of elkjs and of the code inside the worker.

## 3. Data tables

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| Unicode data and ICU data | Unicode 17.0 from ICU 78.3 of Node.js v26.12.0 (`emoji_data_gen.go`). The ICU of Chromium 156 (`str_swimlanes_table.go`) | https://www.unicode.org, https://icu.unicode.org | Unicode-3.0 | Copyright © 2016-2025 Unicode, Inc. | `internal/font/emoji_data_gen.go`, `internal/js/str_swimlanes_table.go` | Unicode-3.0 | C: section `icu` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1) | The copyright line is that of the ICU in Chromium 156. The ICU of Node.js v26.12.0 was not checked. The same license is in `$GOMODCACHE/github.com/dlclark/regexp2/v2@v2.11.0/internal/unicodegen/UNICODE-LICENSE.txt`. |

`internal/font/fontconfig-default.xml.gz` has the rules of the configuration files of fontconfig 2.15.0. The row of fontconfig in section 1.2 covers it.

## 4. Go modules that the linker adds to a program

A program that uses this module contains these modules. Their notices must go with a binary.

| Project | Version | Home page | License (SPDX) | Copyright lines | Where in this repository | License text in section 7 | Source of the lines and the text | Notes |
|---|---|---|---|---|---|---|---|---|
| github.com/dop251/goja | v0.0.0-20261029234554-1329d889cf98 | https://pkg.go.dev/github.com/dop251/goja | MIT | Copyright (c) 2016 Dmitry Panov<br>Copyright (c) 2012 Robert Krimen | linked | MIT | G: `$GOMODCACHE/github.com/dop251/goja@v0.0.0-20261029234554-1329d889cf98/LICENSE` |  |
| github.com/dlclark/regexp2 | v1.12.0 | https://pkg.go.dev/github.com/dlclark/regexp2 | MIT | Copyright (c) Doug Clark | linked | MIT | G: `$GOMODCACHE/github.com/dlclark/regexp2@v1.12.0/LICENSE` |  |
| github.com/dlclark/regexp2/v2 | v2.11.0 | https://pkg.go.dev/github.com/dlclark/regexp2/v2 | MIT | Copyright (c) Doug Clark | linked | MIT | G: `$GOMODCACHE/github.com/dlclark/regexp2/v2@v2.11.0/LICENSE` |  |
| golang.org/x/net | v0.55.0 | https://pkg.go.dev/golang.org/x/net | BSD-3-Clause | Copyright 2009 The Go Authors. | linked | BSD-3-Clause, text 5 | G: `$GOMODCACHE/golang.org/x/net@v0.55.0/LICENSE` | The module has a `PATENTS` file. |
| golang.org/x/text | v0.43.0 | https://pkg.go.dev/golang.org/x/text | BSD-3-Clause | Copyright 2009 The Go Authors. | linked | BSD-3-Clause, text 5 | G: `$GOMODCACHE/golang.org/x/text@v0.43.0/LICENSE` | The module has a `PATENTS` file. |
| golang.org/x/image | v0.47.0 | https://pkg.go.dev/golang.org/x/image | BSD-3-Clause | Copyright 2009 The Go Authors. | linked | BSD-3-Clause, text 5 | G: `$GOMODCACHE/golang.org/x/image@v0.47.0/LICENSE` | The module has a `PATENTS` file. |
| github.com/go-sourcemap/sourcemap | v2.1.3+incompatible | https://pkg.go.dev/github.com/go-sourcemap/sourcemap | BSD-2-Clause | Copyright (c) 2016 The github.com/go-sourcemap/sourcemap Contributors. | linked | BSD-2-Clause | G: `$GOMODCACHE/github.com/go-sourcemap/sourcemap@v2.1.3+incompatible/LICENSE` |  |
| github.com/dop251/goja, package `ftoa` | same | https://pkg.go.dev/github.com/dop251/goja/ftoa | a notice of Lucent Technologies, and BSD-3-Clause (V8) | Copyright (C) 1998, 1999 by Lucent Technologies | linked | Lucent | G: `$GOMODCACHE/github.com/dop251/goja@v0.0.0-20261029234554-1329d889cf98/ftoa/LICENSE_LUCENE` | `ftoa/internal/fast/LICENSE_V8` is the license of V8, in the row of V8 in section 1.2. |
| github.com/go-text/typesetting | v0.3.5 | https://pkg.go.dev/github.com/go-text/typesetting | Unlicense OR BSD-3-Clause | Copyright 2021 The go-text authors | linked | go-text | G: `$GOMODCACHE/github.com/go-text/typesetting@v0.3.5/LICENSE` |  |
| github.com/go-text/typesetting, package `harfbuzz` | v0.3.5 | https://pkg.go.dev/github.com/go-text/typesetting/harfbuzz | MIT, with the notices of HarfBuzz | Copyright (c) 2021 B. KUGLER<br>Copyright © 2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020  Google, Inc.<br>Copyright © 2018,2019,2020  Ebrahim Byagowi<br>Copyright © 2019,2020  Facebook, Inc.<br>Copyright © 2012  Mozilla Foundation<br>Copyright © 2011  Codethink Limited<br>Copyright © 2008,2010  Nokia Corporation and/or its subsidiary(-ies)<br>Copyright © 2009  Keith Stribley<br>Copyright © 2009  Martin Hosken and SIL International<br>Copyright © 2007  Chris Wilson<br>Copyright © 2006  Behdad Esfahbod<br>Copyright © 2005  David Turner<br>Copyright © 2004,2007,2008,2009,2010  Red Hat, Inc.<br>Copyright © 1998-2004  David Turner and Werner Lemberg | linked | go-text harfbuzz | G: `$GOMODCACHE/github.com/go-text/typesetting@v0.3.5/harfbuzz/LICENSE` |  |
| gopkg.in/yaml.v3 | v3.0.1 | https://pkg.go.dev/gopkg.in/yaml.v3 | MIT AND Apache-2.0 | Copyright (c) 2006-2010 Kirill Simonov<br>Copyright (c) 2006-2011 Kirill Simonov<br>Copyright (c) 2011-2019 Canonical Ltd<br>Copyright 2011-2016 Canonical Ltd. | linked | yaml.v3 | G: `$GOMODCACHE/gopkg.in/yaml.v3@v3.0.1/LICENSE` | The module has a `NOTICE` file. It is in section 7 after the license file. |
| github.com/google/pprof | v0.0.0-20230207041349-798e818bf904 | https://pkg.go.dev/github.com/google/pprof | Apache-2.0 | copyright line not found locally | linked through goja (package `profile`) | Apache-2.0 | G: `$GOMODCACHE/github.com/google/pprof@v0.0.0-20230207041349-798e818bf904/LICENSE` | The license file has no copyright line. The file `AUTHORS` of the module names Google Inc. |
| The Go standard library and runtime | the version of the Go toolchain that builds the program | https://go.dev | BSD-3-Clause | Copyright 2009 The Go Authors. | linked | BSD-3-Clause, text 5 | G: `LICENSE` of the Go distribution |  |

## 5. Test data and reference data

These files are in the repository. The Go compiler does not put them in a program.

| Data | Path | Source | License | Notes |
|---|---|---|---|---|
| Diagram sources of the end-to-end tests of mermaid | `testdata/corpus/mermaid-e2e` (2145 files) | mermaid 12.2.0 | MIT | The copyright line of mermaid in section 1.1 covers it. The directory has no notice file |
| Diagram sources of the documentation of mermaid | `testdata/corpus/mermaid-docs` (596 files) | mermaid 12.2.0 | MIT | The same |
| Generated diagram sources | `testdata/corpus/mermaid-large` (70 files) | `tools/reference/gen-large-corpus.mjs` | the license of this repository | |
| Golden files: results of upstream code for recorded inputs | `internal/*/testdata`, `testdata/reference` | mermaid 12.2.0 and its libraries, run in Node.js and in Chromium 156 | The results are output of the upstream programs. The inputs are from the corpora or are original | |
| States of the layout graph of elkjs | `internal/elk/native/*/testdata`, `internal/elk/testdata` | elkjs 0.9.3 | See `internal/elk/native/NOTICE` | |
| Markdown examples | `tools/reference/markdown-inputs.mjs` (function `specExamples`), and the golden files `internal/marked/testdata`, `internal/markdown/testdata` | Examples that follow the CommonMark 0.31 specification and the GFM specification | CC-BY-SA-4.0 for the specifications | See finding F14 of the audit. The copyright lines of the specifications were not found locally |
| Screenshots of Chromium on macOS | `internal/raster/testdata` | Chromium 156 with the fonts of macOS | no license statement | The files under `glyph/` show glyphs of Arial, Courier, Helvetica, Times and Trebuchet MS. See finding F15 of the audit |
| Results of the Math functions of Chromium | `internal/js/crmath/testdata` | Chromium 156 on macOS and on Linux | numbers, no license statement | The macOS file has results of `pow` and `tanh` of `libsystem_m` |

## 6. What was not found locally

### Projects with no copyright line in a local file

- jison-lex (0.3.4)
- non-layered-tidy-tree-layout (2.0.2). The npm registry names the author Michael Wong
- chevrotain (13.2.0). Its source files have no copyright line
- Eclipse EMF and Xtext (compiled code inside the elkjs worker). Their notice files have no copyright line
- heap (inside cytoscape). Its license file has no copyright line
- github.com/google/pprof (v0.0.0-20230207041349-798e818bf904)

The fix pass found two lines in upstream files: `Copyright 2019 Google LLC` for emoji-segmenter, and the form of the notice of a source file of Guava.

The copyright lines of the CommonMark and GFM specifications and of the ICU in Node.js v26.12.0 were also not found locally.

### Texts that are not in the repository and not on the developer machine

These texts were fetched and are not in the repository. Add a copy of each to the repository before a release, or check the text in this document against the upstream file.

- js-yaml: https://unpkg.com/js-yaml@4.3.0/LICENSE
- langium: https://unpkg.com/langium@4.4.0/LICENSE
- fmin: https://unpkg.com/fmin@0.0.4/LICENSE
- Arm Optimized Routines: https://github.com/ARM-software/optimized-routines/blob/v23.01/LICENSE. The two Go files that port it have the MIT text in their first comment
- d3-format 3.1.0: https://unpkg.com/d3-format@3.1.0/LICENSE. The text is the ISC text of section 7 with the copyright line of the table
- es-toolkit 1.45.1: https://unpkg.com/es-toolkit@1.45.1/LICENSE. The text is the MIT text of section 7 with the lines of the table

The fix pass of 2026-11-09 put these texts into the repository. They were in this list before.

- V8: `internal/js/LICENSE-V8.txt`
- CPython list sort, as V8 has it: `internal/js/LICENSE-PSF.txt`
- HarfBuzz: `internal/font/LICENSE-HarfBuzz.txt`
- GWT (the Java runtime emulation): the notices of the Java files are in `internal/elk/native/gwt/NOTICE`

## 7. License texts

### MIT

Used for: mermaid 12.2.0, @mermaid-js/mermaid-cli 12.0.0, @mermaid-js/parser 2.0.1, dagre-d3-es 7.0.14, es-toolkit 1.45.1, khroma 2.1.0, stylis 4.3.6, roughjs 4.6.6, hachure-fill 0.5.2, path-data-parser 0.1.0, points-on-curve 0.2.0, points-on-path 0.2.1, dayjs 1.11.21, @upsetjs/venn.js 2.0.0, @iconify/utils 3.0.2, @braintree/sanitize-url 7.1.2, ts-dedent 2.2.0, jison 0.4.18, jison-lex 0.3.4, js-yaml 4.3.0, langium 4.4.0, Arm Optimized Routines v20.02, Arm Optimized Routines v23.01, skrifa, read-fonts, font-types (Fontations), cytoscape 3.34.0, layout-base 1.0.2 and 2.0.1, cose-base 1.0.3 and 2.2.0, cytoscape-cose-bilkent 4.1.0, cytoscape-fcose 2.2.0, github.com/dop251/goja, github.com/dlclark/regexp2 v1.12.0, github.com/dlclark/regexp2/v2 v2.11.0.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/mermaid/LICENSE`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### lodash

The license file of lodash-es.

Used for: lodash-es 4.18.1.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/lodash-es/LICENSE`.

```text
Copyright OpenJS Foundation and other contributors <https://openjsf.org/>

Based on Underscore.js, copyright Jeremy Ashkenas,
DocumentCloud and Investigative Reporters & Editors <http://underscorejs.org/>

This software consists of voluntary contributions made by many
individuals. For exact contribution history, see the revision history
available at https://github.com/lodash/lodash

The following license applies to all parts of this software except as
documented below:

====

Permission is hereby granted, free of charge, to any person obtaining
a copy of this software and associated documentation files (the
"Software"), to deal in the Software without restriction, including
without limitation the rights to use, copy, modify, merge, publish,
distribute, sublicense, and/or sell copies of the Software, and to
permit persons to whom the Software is furnished to do so, subject to
the following conditions:

The above copyright notice and this permission notice shall be
included in all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND
NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT HOLDERS BE
LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION
WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

====

Copyright and related rights for sample code are waived via CC0. Sample
code is defined as all source code displayed within the prose of the
documentation.

CC0: http://creativecommons.org/publicdomain/zero/1.0/

====

Files located in the node_modules and vendor directories are externally
maintained libraries used by this software which have their own
licenses; we recommend you read them, as their terms may differ from the
terms above.
```

### marked

The license file of marked.

Used for: marked 16.3.0.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/marked/LICENSE.md`.

```text
# License information

## Contribution License Agreement

If you contribute code to this project, you are implicitly allowing your code
to be distributed under the MIT license. You are also implicitly verifying that
all code is your original work. `</legalese>`

## Marked

Copyright (c) 2018+, MarkedJS (https://github.com/markedjs/)
Copyright (c) 2011-2018, Christopher Jeffrey (https://github.com/chjj/)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.

## Markdown

Copyright © 2004, John Gruber
http://daringfireball.net/
All rights reserved.

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.
* Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.
* Neither the name “Markdown” nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

This software is provided by the copyright holders and contributors “as is” and any express or implied warranties, including, but not limited to, the implied warranties of merchantability and fitness for a particular purpose are disclaimed. In no event shall the copyright owner or contributors be liable for any direct, indirect, incidental, special, exemplary, or consequential damages (including, but not limited to, procurement of substitute goods or services; loss of use, data, or profits; or business interruption) however caused and on any theory of liability, whether in contract, strict liability, or tort (including negligence or otherwise) arising in any way out of the use of this software, even if advised of the possibility of such damage.
```

### ISC

Used for: d3-array 3.2.4, d3-axis 3.0.0, d3-color 3.1.0, d3-format 3.1.0, d3-hierarchy 3.1.2, d3-interpolate 3.0.1, d3-path 3.1.0, d3-scale 4.0.2, d3-scale-chromatic 3.1.0, d3-selection 3.0.0, d3-shape 3.2.0, d3-time 3.1.0, d3-time-format 4.1.0, internmap 2.0.3.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/d3-array/LICENSE`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Permission to use, copy, modify, and/or distribute this software for any purpose
with or without fee is hereby granted, provided that the above copyright notice
and this permission notice appear in all copies.

THE SOFTWARE IS PROVIDED "AS IS" AND THE AUTHOR DISCLAIMS ALL WARRANTIES WITH
REGARD TO THIS SOFTWARE INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS. IN NO EVENT SHALL THE AUTHOR BE LIABLE FOR ANY SPECIAL, DIRECT,
INDIRECT, OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS
OF USE, DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
THIS SOFTWARE.
```

### BSD-3-Clause, text 1

Used for: d3-sankey 0.12.3, d3-shape 1.3.7, fmin.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/d3-sankey/LICENSE`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Redistribution and use in source and binary forms, with or without modification,
are permitted provided that the following conditions are met:

* Redistributions of source code must retain the above copyright notice, this
  list of conditions and the following disclaimer.

* Redistributions in binary form must reproduce the above copyright notice,
  this list of conditions and the following disclaimer in the documentation
  and/or other materials provided with the distribution.

* Neither the name of the author nor the names of contributors may be used to
  endorse or promote products derived from this software without specific prior
  written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND
ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED
WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE
DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT OWNER OR CONTRIBUTORS BE LIABLE FOR
ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
(INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON
ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS
SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### Apache-2.0

Apache License 2.0.

Used for: chevrotain 13.2.0, DOMPurify 3.4.12, LLVM libc, emoji-segmenter the revision in Chromium 156, GWT (the Java runtime emulation), Guava, Eclipse EMF, Xtext (compiled code inside the elkjs worker), github.com/google/pprof.

Taken from (`node_modules`, not in git): `tools/reference/node_modules/chevrotain/LICENSE.txt`. Copies of this text are in the repository: `internal/diagrams/usecase/LICENSE-chevrotain.txt`, `internal/langium/LICENSE-chevrotain.txt`, `internal/elk/native/gwt/LICENSE-GWT.txt`, `internal/font/LICENSE-emoji-segmenter.txt` and `internal/purify/LICENSE-Apache-2.0.txt`. `internal/js/crmath/LICENSE-LLVM.txt` has the text with the LLVM Exceptions.

```text
                                 Apache License
                           Version 2.0, January 2004
                        http://www.apache.org/licenses/

   TERMS AND CONDITIONS FOR USE, REPRODUCTION, AND DISTRIBUTION

   1. Definitions.

      "License" shall mean the terms and conditions for use, reproduction,
      and distribution as defined by Sections 1 through 9 of this document.

      "Licensor" shall mean the copyright owner or entity authorized by
      the copyright owner that is granting the License.

      "Legal Entity" shall mean the union of the acting entity and all
      other entities that control, are controlled by, or are under common
      control with that entity. For the purposes of this definition,
      "control" means (i) the power, direct or indirect, to cause the
      direction or management of such entity, whether by contract or
      otherwise, or (ii) ownership of fifty percent (50%) or more of the
      outstanding shares, or (iii) beneficial ownership of such entity.

      "You" (or "Your") shall mean an individual or Legal Entity
      exercising permissions granted by this License.

      "Source" form shall mean the preferred form for making modifications,
      including but not limited to software source code, documentation
      source, and configuration files.

      "Object" form shall mean any form resulting from mechanical
      transformation or translation of a Source form, including but
      not limited to compiled object code, generated documentation,
      and conversions to other media types.

      "Work" shall mean the work of authorship, whether in Source or
      Object form, made available under the License, as indicated by a
      copyright notice that is included in or attached to the work
      (an example is provided in the Appendix below).

      "Derivative Works" shall mean any work, whether in Source or Object
      form, that is based on (or derived from) the Work and for which the
      editorial revisions, annotations, elaborations, or other modifications
      represent, as a whole, an original work of authorship. For the purposes
      of this License, Derivative Works shall not include works that remain
      separable from, or merely link (or bind by name) to the interfaces of,
      the Work and Derivative Works thereof.

      "Contribution" shall mean any work of authorship, including
      the original version of the Work and any modifications or additions
      to that Work or Derivative Works thereof, that is intentionally
      submitted to Licensor for inclusion in the Work by the copyright owner
      or by an individual or Legal Entity authorized to submit on behalf of
      the copyright owner. For the purposes of this definition, "submitted"
      means any form of electronic, verbal, or written communication sent
      to the Licensor or its representatives, including but not limited to
      communication on electronic mailing lists, source code control systems,
      and issue tracking systems that are managed by, or on behalf of, the
      Licensor for the purpose of discussing and improving the Work, but
      excluding communication that is conspicuously marked or otherwise
      designated in writing by the copyright owner as "Not a Contribution."

      "Contributor" shall mean Licensor and any individual or Legal Entity
      on behalf of whom a Contribution has been received by Licensor and
      subsequently incorporated within the Work.

   2. Grant of Copyright License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      copyright license to reproduce, prepare Derivative Works of,
      publicly display, publicly perform, sublicense, and distribute the
      Work and such Derivative Works in Source or Object form.

   3. Grant of Patent License. Subject to the terms and conditions of
      this License, each Contributor hereby grants to You a perpetual,
      worldwide, non-exclusive, no-charge, royalty-free, irrevocable
      (except as stated in this section) patent license to make, have made,
      use, offer to sell, sell, import, and otherwise transfer the Work,
      where such license applies only to those patent claims licensable
      by such Contributor that are necessarily infringed by their
      Contribution(s) alone or by combination of their Contribution(s)
      with the Work to which such Contribution(s) was submitted. If You
      institute patent litigation against any entity (including a
      cross-claim or counterclaim in a lawsuit) alleging that the Work
      or a Contribution incorporated within the Work constitutes direct
      or contributory patent infringement, then any patent licenses
      granted to You under this License for that Work shall terminate
      as of the date such litigation is filed.

   4. Redistribution. You may reproduce and distribute copies of the
      Work or Derivative Works thereof in any medium, with or without
      modifications, and in Source or Object form, provided that You
      meet the following conditions:

      (a) You must give any other recipients of the Work or
          Derivative Works a copy of this License; and

      (b) You must cause any modified files to carry prominent notices
          stating that You changed the files; and

      (c) You must retain, in the Source form of any Derivative Works
          that You distribute, all copyright, patent, trademark, and
          attribution notices from the Source form of the Work,
          excluding those notices that do not pertain to any part of
          the Derivative Works; and

      (d) If the Work includes a "NOTICE" text file as part of its
          distribution, then any Derivative Works that You distribute must
          include a readable copy of the attribution notices contained
          within such NOTICE file, excluding those notices that do not
          pertain to any part of the Derivative Works, in at least one
          of the following places: within a NOTICE text file distributed
          as part of the Derivative Works; within the Source form or
          documentation, if provided along with the Derivative Works; or,
          within a display generated by the Derivative Works, if and
          wherever such third-party notices normally appear. The contents
          of the NOTICE file are for informational purposes only and
          do not modify the License. You may add Your own attribution
          notices within Derivative Works that You distribute, alongside
          or as an addendum to the NOTICE text from the Work, provided
          that such additional attribution notices cannot be construed
          as modifying the License.

      You may add Your own copyright statement to Your modifications and
      may provide additional or different license terms and conditions
      for use, reproduction, or distribution of Your modifications, or
      for any such Derivative Works as a whole, provided Your use,
      reproduction, and distribution of the Work otherwise complies with
      the conditions stated in this License.

   5. Submission of Contributions. Unless You explicitly state otherwise,
      any Contribution intentionally submitted for inclusion in the Work
      by You to the Licensor shall be under the terms and conditions of
      this License, without any additional terms or conditions.
      Notwithstanding the above, nothing herein shall supersede or modify
      the terms of any separate license agreement you may have executed
      with Licensor regarding such Contributions.

   6. Trademarks. This License does not grant permission to use the trade
      names, trademarks, service marks, or product names of the Licensor,
      except as required for reasonable and customary use in describing the
      origin of the Work and reproducing the content of the NOTICE file.

   7. Disclaimer of Warranty. Unless required by applicable law or
      agreed to in writing, Licensor provides the Work (and each
      Contributor provides its Contributions) on an "AS IS" BASIS,
      WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or
      implied, including, without limitation, any warranties or conditions
      of TITLE, NON-INFRINGEMENT, MERCHANTABILITY, or FITNESS FOR A
      PARTICULAR PURPOSE. You are solely responsible for determining the
      appropriateness of using or redistributing the Work and assume any
      risks associated with Your exercise of permissions under this License.

   8. Limitation of Liability. In no event and under no legal theory,
      whether in tort (including negligence), contract, or otherwise,
      unless required by applicable law (such as deliberate and grossly
      negligent acts) or agreed to in writing, shall any Contributor be
      liable to You for damages, including any direct, indirect, special,
      incidental, or consequential damages of any character arising as a
      result of this License or out of the use or inability to use the
      Work (including but not limited to damages for loss of goodwill,
      work stoppage, computer failure or malfunction, or any and all
      other commercial damages or losses), even if such Contributor
      has been advised of the possibility of such damages.

   9. Accepting Warranty or Additional Liability. While redistributing
      the Work or Derivative Works thereof, You may choose to offer,
      and charge a fee for, acceptance of support, warranty, indemnity,
      or other liability obligations and/or rights consistent with this
      License. However, in accepting such obligations, You may act only
      on Your own behalf and on Your sole responsibility, not on behalf
      of any other Contributor, and only if You agree to indemnify,
      defend, and hold each Contributor harmless for any liability
      incurred by, or claims asserted against, such Contributor by reason
      of your accepting any such warranty or additional liability.

   END OF TERMS AND CONDITIONS

   APPENDIX: How to apply the Apache License to your work.

      To apply the Apache License to your work, attach the following
      boilerplate notice, with the fields enclosed by brackets "[]"
      replaced with your own identifying information. (Don't include
      the brackets!)  The text should be enclosed in the appropriate
      comment syntax for the file format. We also recommend that a
      file or class name and description of purpose be included on the
      same "printed page" as the copyright notice for easier
      identification within third-party archives.

   Copyright [yyyy] [name of copyright owner]

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
```

### BSD-3-Clause, text 2

Used for: Chromium (Blink, `ui/gfx`, `cc`).

Taken from (the repository): `internal/font/LICENSE-Chromium.txt`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
// Copyright 2015 The Chromium Authors
//
// Redistribution and use in source and binary forms, with or without
// modification, are permitted provided that the following conditions are
// met:
//
//    * Redistributions of source code must retain the above copyright
// notice, this list of conditions and the following disclaimer.
//    * Redistributions in binary form must reproduce the above
// copyright notice, this list of conditions and the following disclaimer
// in the documentation and/or other materials provided with the
// distribution.
//    * Neither the name of Google LLC nor the names of its
// contributors may be used to endorse or promote products derived from
// this software without specific prior written permission.
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
// "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
// LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
// A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
// OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
// LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
// DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
// THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
// (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
// OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### LGPL-2.0

GNU Library General Public License, version 2.

Used for: Blink files that come from WebKit and KHTML, under the LGPL Chromium 156.

Taken from (the license file of the reference Chromium): section WebKit of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1). `internal/browser/LICENSE-LGPL-2.0.txt` and `internal/raster/LICENSE-LGPL-2.0.txt` have the whole license file of Blink (`third_party/blink/renderer/core/LICENSE-LGPL-2` of Chromium 156.0.8078.4). That file has the same terms and conditions as the text below, and the appendix "How to Apply These Terms to Your New Libraries" after them.

```text
                  GNU LIBRARY GENERAL PUBLIC LICENSE
                       Version 2, June 1991

 Copyright (C) 1991 Free Software Foundation, Inc.
 51 Franklin Street, Fifth Floor, Boston, MA  02110-1301  USA
 Everyone is permitted to copy and distribute verbatim copies
 of this license document, but changing it is not allowed.

[This is the first released version of the library GPL.  It is
 numbered 2 because it goes with version 2 of the ordinary GPL.]

                            Preamble

  The licenses for most software are designed to take away your
freedom to share and change it.  By contrast, the GNU General Public
Licenses are intended to guarantee your freedom to share and change
free software--to make sure the software is free for all its users.

  This license, the Library General Public License, applies to some
specially designated Free Software Foundation software, and to any
other libraries whose authors decide to use it.  You can use it for
your libraries, too.

  When we speak of free software, we are referring to freedom, not
price.  Our General Public Licenses are designed to make sure that you
have the freedom to distribute copies of free software (and charge for
this service if you wish), that you receive source code or can get it
if you want it, that you can change the software or use pieces of it
in new free programs; and that you know you can do these things.

  To protect your rights, we need to make restrictions that forbid
anyone to deny you these rights or to ask you to surrender the rights.
These restrictions translate to certain responsibilities for you if
you distribute copies of the library, or if you modify it.

  For example, if you distribute copies of the library, whether gratis
or for a fee, you must give the recipients all the rights that we gave
you.  You must make sure that they, too, receive or can get the source
code.  If you link a program with the library, you must provide
complete object files to the recipients so that they can relink them
with the library, after making changes to the library and recompiling
it.  And you must show them these terms so they know their rights.

  Our method of protecting your rights has two steps: (1) copyright
the library, and (2) offer you this license which gives you legal
permission to copy, distribute and/or modify the library.

  Also, for each distributor's protection, we want to make certain
that everyone understands that there is no warranty for this free
library.  If the library is modified by someone else and passed on, we
want its recipients to know that what they have is not the original
version, so that any problems introduced by others will not reflect on
the original authors' reputations.

  Finally, any free program is threatened constantly by software
patents.  We wish to avoid the danger that companies distributing free
software will individually obtain patent licenses, thus in effect
transforming the program into proprietary software.  To prevent this,
we have made it clear that any patent must be licensed for everyone's
free use or not licensed at all.

  Most GNU software, including some libraries, is covered by the ordinary
GNU General Public License, which was designed for utility programs.  This
license, the GNU Library General Public License, applies to certain
designated libraries.  This license is quite different from the ordinary
one; be sure to read it in full, and don't assume that anything in it is
the same as in the ordinary license.

  The reason we have a separate public license for some libraries is that
they blur the distinction we usually make between modifying or adding to a
program and simply using it.  Linking a program with a library, without
changing the library, is in some sense simply using the library, and is
analogous to running a utility program or application program.  However, in
a textual and legal sense, the linked executable is a combined work, a
derivative of the original library, and the ordinary General Public License
treats it as such.

  Because of this blurred distinction, using the ordinary General
Public License for libraries did not effectively promote software
sharing, because most developers did not use the libraries.  We
concluded that weaker conditions might promote sharing better.

  However, unrestricted linking of non-free programs would deprive the
users of those programs of all benefit from the free status of the
libraries themselves.  This Library General Public License is intended to
permit developers of non-free programs to use free libraries, while
preserving your freedom as a user of such programs to change the free
libraries that are incorporated in them.  (We have not seen how to achieve
this as regards changes in header files, but we have achieved it as regards
changes in the actual functions of the Library.)  The hope is that this
will lead to faster development of free libraries.

  The precise terms and conditions for copying, distribution and
modification follow.  Pay close attention to the difference between a
"work based on the library" and a "work that uses the library".  The
former contains code derived from the library, while the latter only
works together with the library.

  Note that it is possible for a library to be covered by the ordinary
General Public License rather than by this special one.

                  GNU LIBRARY GENERAL PUBLIC LICENSE
   TERMS AND CONDITIONS FOR COPYING, DISTRIBUTION AND MODIFICATION

  0. This License Agreement applies to any software library which
contains a notice placed by the copyright holder or other authorized
party saying it may be distributed under the terms of this Library
General Public License (also called "this License").  Each licensee is
addressed as "you".

  A "library" means a collection of software functions and/or data
prepared so as to be conveniently linked with application programs
(which use some of those functions and data) to form executables.

  The "Library", below, refers to any such software library or work
which has been distributed under these terms.  A "work based on the
Library" means either the Library or any derivative work under
copyright law: that is to say, a work containing the Library or a
portion of it, either verbatim or with modifications and/or translated
straightforwardly into another language.  (Hereinafter, translation is
included without limitation in the term "modification".)

  "Source code" for a work means the preferred form of the work for
making modifications to it.  For a library, complete source code means
all the source code for all modules it contains, plus any associated
interface definition files, plus the scripts used to control compilation
and installation of the library.

  Activities other than copying, distribution and modification are not
covered by this License; they are outside its scope.  The act of
running a program using the Library is not restricted, and output from
such a program is covered only if its contents constitute a work based
on the Library (independent of the use of the Library in a tool for
writing it).  Whether that is true depends on what the Library does
and what the program that uses the Library does.
  
  1. You may copy and distribute verbatim copies of the Library's
complete source code as you receive it, in any medium, provided that
you conspicuously and appropriately publish on each copy an
appropriate copyright notice and disclaimer of warranty; keep intact
all the notices that refer to this License and to the absence of any
warranty; and distribute a copy of this License along with the
Library.

  You may charge a fee for the physical act of transferring a copy,
and you may at your option offer warranty protection in exchange for a
fee.

  2. You may modify your copy or copies of the Library or any portion
of it, thus forming a work based on the Library, and copy and
distribute such modifications or work under the terms of Section 1
above, provided that you also meet all of these conditions:

    a) The modified work must itself be a software library.

    b) You must cause the files modified to carry prominent notices
    stating that you changed the files and the date of any change.

    c) You must cause the whole of the work to be licensed at no
    charge to all third parties under the terms of this License.

    d) If a facility in the modified Library refers to a function or a
    table of data to be supplied by an application program that uses
    the facility, other than as an argument passed when the facility
    is invoked, then you must make a good faith effort to ensure that,
    in the event an application does not supply such function or
    table, the facility still operates, and performs whatever part of
    its purpose remains meaningful.

    (For example, a function in a library to compute square roots has
    a purpose that is entirely well-defined independent of the
    application.  Therefore, Subsection 2d requires that any
    application-supplied function or table used by this function must
    be optional: if the application does not supply it, the square
    root function must still compute square roots.)

These requirements apply to the modified work as a whole.  If
identifiable sections of that work are not derived from the Library,
and can be reasonably considered independent and separate works in
themselves, then this License, and its terms, do not apply to those
sections when you distribute them as separate works.  But when you
distribute the same sections as part of a whole which is a work based
on the Library, the distribution of the whole must be on the terms of
this License, whose permissions for other licensees extend to the
entire whole, and thus to each and every part regardless of who wrote
it.

Thus, it is not the intent of this section to claim rights or contest
your rights to work written entirely by you; rather, the intent is to
exercise the right to control the distribution of derivative or
collective works based on the Library.

In addition, mere aggregation of another work not based on the Library
with the Library (or with a work based on the Library) on a volume of
a storage or distribution medium does not bring the other work under
the scope of this License.

  3. You may opt to apply the terms of the ordinary GNU General Public
License instead of this License to a given copy of the Library.  To do
this, you must alter all the notices that refer to this License, so
that they refer to the ordinary GNU General Public License, version 2,
instead of to this License.  (If a newer version than version 2 of the
ordinary GNU General Public License has appeared, then you can specify
that version instead if you wish.)  Do not make any other change in
these notices.

  Once this change is made in a given copy, it is irreversible for
that copy, so the ordinary GNU General Public License applies to all
subsequent copies and derivative works made from that copy.

  This option is useful when you wish to copy part of the code of
the Library into a program that is not a library.

  4. You may copy and distribute the Library (or a portion or
derivative of it, under Section 2) in object code or executable form
under the terms of Sections 1 and 2 above provided that you accompany
it with the complete corresponding machine-readable source code, which
must be distributed under the terms of Sections 1 and 2 above on a
medium customarily used for software interchange.

  If distribution of object code is made by offering access to copy
from a designated place, then offering equivalent access to copy the
source code from the same place satisfies the requirement to
distribute the source code, even though third parties are not
compelled to copy the source along with the object code.

  5. A program that contains no derivative of any portion of the
Library, but is designed to work with the Library by being compiled or
linked with it, is called a "work that uses the Library".  Such a
work, in isolation, is not a derivative work of the Library, and
therefore falls outside the scope of this License.

  However, linking a "work that uses the Library" with the Library
creates an executable that is a derivative of the Library (because it
contains portions of the Library), rather than a "work that uses the
library".  The executable is therefore covered by this License.
Section 6 states terms for distribution of such executables.

  When a "work that uses the Library" uses material from a header file
that is part of the Library, the object code for the work may be a
derivative work of the Library even though the source code is not.
Whether this is true is especially significant if the work can be
linked without the Library, or if the work is itself a library.  The
threshold for this to be true is not precisely defined by law.

  If such an object file uses only numerical parameters, data
structure layouts and accessors, and small macros and small inline
functions (ten lines or less in length), then the use of the object
file is unrestricted, regardless of whether it is legally a derivative
work.  (Executables containing this object code plus portions of the
Library will still fall under Section 6.)

  Otherwise, if the work is a derivative of the Library, you may
distribute the object code for the work under the terms of Section 6.
Any executables containing that work also fall under Section 6,
whether or not they are linked directly with the Library itself.

  6. As an exception to the Sections above, you may also compile or
link a "work that uses the Library" with the Library to produce a
work containing portions of the Library, and distribute that work
under terms of your choice, provided that the terms permit
modification of the work for the customer's own use and reverse
engineering for debugging such modifications.

  You must give prominent notice with each copy of the work that the
Library is used in it and that the Library and its use are covered by
this License.  You must supply a copy of this License.  If the work
during execution displays copyright notices, you must include the
copyright notice for the Library among them, as well as a reference
directing the user to the copy of this License.  Also, you must do one
of these things:

    a) Accompany the work with the complete corresponding
    machine-readable source code for the Library including whatever
    changes were used in the work (which must be distributed under
    Sections 1 and 2 above); and, if the work is an executable linked
    with the Library, with the complete machine-readable "work that
    uses the Library", as object code and/or source code, so that the
    user can modify the Library and then relink to produce a modified
    executable containing the modified Library.  (It is understood
    that the user who changes the contents of definitions files in the
    Library will not necessarily be able to recompile the application
    to use the modified definitions.)

    b) Accompany the work with a written offer, valid for at
    least three years, to give the same user the materials
    specified in Subsection 6a, above, for a charge no more
    than the cost of performing this distribution.

    c) If distribution of the work is made by offering access to copy
    from a designated place, offer equivalent access to copy the above
    specified materials from the same place.

    d) Verify that the user has already received a copy of these
    materials or that you have already sent this user a copy.

  For an executable, the required form of the "work that uses the
Library" must include any data and utility programs needed for
reproducing the executable from it.  However, as a special exception,
the source code distributed need not include anything that is normally
distributed (in either source or binary form) with the major
components (compiler, kernel, and so on) of the operating system on
which the executable runs, unless that component itself accompanies
the executable.

  It may happen that this requirement contradicts the license
restrictions of other proprietary libraries that do not normally
accompany the operating system.  Such a contradiction means you cannot
use both them and the Library together in an executable that you
distribute.

  7. You may place library facilities that are a work based on the
Library side-by-side in a single library together with other library
facilities not covered by this License, and distribute such a combined
library, provided that the separate distribution of the work based on
the Library and of the other library facilities is otherwise
permitted, and provided that you do these two things:

    a) Accompany the combined library with a copy of the same work
    based on the Library, uncombined with any other library
    facilities.  This must be distributed under the terms of the
    Sections above.

    b) Give prominent notice with the combined library of the fact
    that part of it is a work based on the Library, and explaining
    where to find the accompanying uncombined form of the same work.

  8. You may not copy, modify, sublicense, link with, or distribute
the Library except as expressly provided under this License.  Any
attempt otherwise to copy, modify, sublicense, link with, or
distribute the Library is void, and will automatically terminate your
rights under this License.  However, parties who have received copies,
or rights, from you under this License will not have their licenses
terminated so long as such parties remain in full compliance.

  9. You are not required to accept this License, since you have not
signed it.  However, nothing else grants you permission to modify or
distribute the Library or its derivative works.  These actions are
prohibited by law if you do not accept this License.  Therefore, by
modifying or distributing the Library (or any work based on the
Library), you indicate your acceptance of this License to do so, and
all its terms and conditions for copying, distributing or modifying
the Library or works based on it.

  10. Each time you redistribute the Library (or any work based on the
Library), the recipient automatically receives a license from the
original licensor to copy, distribute, link with or modify the Library
subject to these terms and conditions.  You may not impose any further
restrictions on the recipients' exercise of the rights granted herein.
You are not responsible for enforcing compliance by third parties to
this License.

  11. If, as a consequence of a court judgment or allegation of patent
infringement or for any other reason (not limited to patent issues),
conditions are imposed on you (whether by court order, agreement or
otherwise) that contradict the conditions of this License, they do not
excuse you from the conditions of this License.  If you cannot
distribute so as to satisfy simultaneously your obligations under this
License and any other pertinent obligations, then as a consequence you
may not distribute the Library at all.  For example, if a patent
license would not permit royalty-free redistribution of the Library by
all those who receive copies directly or indirectly through you, then
the only way you could satisfy both it and this License would be to
refrain entirely from distribution of the Library.

If any portion of this section is held invalid or unenforceable under any
particular circumstance, the balance of the section is intended to apply,
and the section as a whole is intended to apply in other circumstances.

It is not the purpose of this section to induce you to infringe any
patents or other property right claims or to contest validity of any
such claims; this section has the sole purpose of protecting the
integrity of the free software distribution system which is
implemented by public license practices.  Many people have made
generous contributions to the wide range of software distributed
through that system in reliance on consistent application of that
system; it is up to the author/donor to decide if he or she is willing
to distribute software through any other system and a licensee cannot
impose that choice.

This section is intended to make thoroughly clear what is believed to
be a consequence of the rest of this License.

  12. If the distribution and/or use of the Library is restricted in
certain countries either by patents or by copyrighted interfaces, the
original copyright holder who places the Library under this License may add
an explicit geographical distribution limitation excluding those countries,
so that distribution is permitted only in or among countries not thus
excluded.  In such case, this License incorporates the limitation as if
written in the body of this License.

  13. The Free Software Foundation may publish revised and/or new
versions of the Library General Public License from time to time.
Such new versions will be similar in spirit to the present version,
but may differ in detail to address new problems or concerns.

Each version is given a distinguishing version number.  If the Library
specifies a version number of this License which applies to it and
"any later version", you have the option of following the terms and
conditions either of that version or of any later version published by
the Free Software Foundation.  If the Library does not specify a
license version number, you may choose any version ever published by
the Free Software Foundation.

  14. If you wish to incorporate parts of the Library into other free
programs whose distribution conditions are incompatible with these,
write to the author to ask for permission.  For software which is
copyrighted by the Free Software Foundation, write to the Free
Software Foundation; we sometimes make exceptions for this.  Our
decision will be guided by the two goals of preserving the free status
of all derivatives of our free software and of promoting the sharing
and reuse of software generally.

                            NO WARRANTY

  15. BECAUSE THE LIBRARY IS LICENSED FREE OF CHARGE, THERE IS NO
WARRANTY FOR THE LIBRARY, TO THE EXTENT PERMITTED BY APPLICABLE LAW.
EXCEPT WHEN OTHERWISE STATED IN WRITING THE COPYRIGHT HOLDERS AND/OR
OTHER PARTIES PROVIDE THE LIBRARY "AS IS" WITHOUT WARRANTY OF ANY
KIND, EITHER EXPRESSED OR IMPLIED, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
PURPOSE.  THE ENTIRE RISK AS TO THE QUALITY AND PERFORMANCE OF THE
LIBRARY IS WITH YOU.  SHOULD THE LIBRARY PROVE DEFECTIVE, YOU ASSUME
THE COST OF ALL NECESSARY SERVICING, REPAIR OR CORRECTION.

  16. IN NO EVENT UNLESS REQUIRED BY APPLICABLE LAW OR AGREED TO IN
WRITING WILL ANY COPYRIGHT HOLDER, OR ANY OTHER PARTY WHO MAY MODIFY
AND/OR REDISTRIBUTE THE LIBRARY AS PERMITTED ABOVE, BE LIABLE TO YOU
FOR DAMAGES, INCLUDING ANY GENERAL, SPECIAL, INCIDENTAL OR
CONSEQUENTIAL DAMAGES ARISING OUT OF THE USE OR INABILITY TO USE THE
LIBRARY (INCLUDING BUT NOT LIMITED TO LOSS OF DATA OR DATA BEING
RENDERED INACCURATE OR LOSSES SUSTAINED BY YOU OR THIRD PARTIES OR A
FAILURE OF THE LIBRARY TO OPERATE WITH ANY OTHER SOFTWARE), EVEN IF
SUCH HOLDER OR OTHER PARTY HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH
DAMAGES.

                     END OF TERMS AND CONDITIONS
```

### WebKit: holders and BSD terms

Used for: Blink files that come from WebKit, under BSD terms Chromium 156.

Taken from (the license file of the reference Chromium): section WebKit of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1).

```text
(WebKit doesn't distribute an explicit license.  This LICENSE is derived from
license text in the source.)

Copyright (c) 1997, 1998, 1999, 2000, 2001, 2002, 2003, 2004, 2005,
2006, 2007 Alexander Kellett, Alexey Proskuryakov, Alex Mathews, Allan
Sandfeld Jensen, Alp Toker, Anders Carlsson, Andrew Wellington, Antti
Koivisto, Apple Inc., Arthur Langereis, Baron Schwartz, Bjoern Graf,
Brent Fulgham, Cameron Zwarich, Charles Samuels, Christian Dywan,
Collabora Ltd., Cyrus Patel, Daniel Molkentin, Dave Maclachlan, David
Smith, Dawit Alemayehu, Dirk Mueller, Dirk Schulze, Don Gibson, Enrico
Ros, Eric Seidel, Frederik Holljen, Frerich Raabe, Friedmann Kleint,
George Staikos, Google Inc., Graham Dennis, Harri Porten, Henry Mason,
Hiroyuki Ikezoe, Holger Hans Peter Freyther, IBM, James G. Speth, Jan
Alonzo, Jean-Loup Gailly, John Reis, Jonas Witt, Jon Shier, Jonas
Witt, Julien Chaffraix, Justin Haygood, Kevin Ollivier, Kevin Watters,
Kimmo Kinnunen, Kouhei Sutou, Krzysztof Kowalczyk, Lars Knoll, Luca
Bruno, Maks Orlovich, Malte Starostik, Mark Adler, Martin Jones,
Marvin Decker, Matt Lilek, Michael Emmel, Mitz Pettel, mozilla.org,
Netscape Communications Corporation, Nicholas Shanks, Nikolas
Zimmermann, Nokia, Oliver Hunt, Opened Hand, Paul Johnston, Peter
Kelly, Pioneer Research Center USA, Rich Moore, Rob Buis, Robin Dunn,
Ronald Tschalär, Samuel Weinig, Simon Hausmann, Staikos Computing
Services Inc., Stefan Schimanski, Symantec Corporation, The Dojo
Foundation, The Karbon Developers, Thomas Boyer, Tim Copperfield,
Tobias Anton, Torben Weis, Trolltech, University of Cambridge, Vaclav
Slavik, Waldo Bastian, Xan Lopez, Zack Rusin

The terms and conditions vary from file to file, but are one of:

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the
   distribution.

*OR*

Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

1. Redistributions of source code must retain the above copyright
   notice, this list of conditions and the following disclaimer.
2. Redistributions in binary form must reproduce the above copyright
   notice, this list of conditions and the following disclaimer in the
   documentation and/or other materials provided with the
   distribution.
3. Neither the name of Apple Computer, Inc. ("Apple") nor the names of
   its contributors may be used to endorse or promote products derived
   from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY APPLE COMPUTER, INC. ``AS IS'' AND ANY
EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR
PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL APPLE COMPUTER, INC. OR
CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR
PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY

OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### BSD-3-Clause, text 3

Used for: Skia.

Taken from (the repository): `internal/font/LICENSE-Skia.txt`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

  * Redistributions of source code must retain the above copyright
    notice, this list of conditions and the following disclaimer.

  * Redistributions in binary form must reproduce the above copyright
    notice, this list of conditions and the following disclaimer in
    the documentation and/or other materials provided with the
    distribution.

  * Neither the name of the copyright holder nor the names of its
    contributors may be used to endorse or promote products derived
    from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### BSD-3-Clause, text 4

Used for: V8.

Taken from (fetched from the upstream project): https://chromium.googlesource.com/v8/v8/+/refs/tags/15.6.75/LICENSE. `internal/js/LICENSE-V8.txt` has the whole file.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

    * Redistributions of source code must retain the above copyright
      notice, this list of conditions and the following disclaimer.
    * Redistributions in binary form must reproduce the above
      copyright notice, this list of conditions and the following
      disclaimer in the documentation and/or other materials provided
      with the distribution.
    * Neither the name of Google Inc. nor the names of its
      contributors may be used to endorse or promote products derived
      from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### SunPro

The notice of fdlibm (SunPro).

Used for: fdlibm, and the fdlibm code in V8 (`src/base/ieee754.cc`) fdlibm 5.3. V8 15.6.75.

Taken from (the license file of the reference Chromium): section `V8 fork of fdlibm` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1). The notices in `internal/js/crmath/linux_libm_tanh.go` and `internal/browser/linux_trig.go` are the notices of the fdlibm files.

```text
Copyright (C) 1993-2004 by Sun Microsystems, Inc. All rights reserved.

Developed at SunSoft, a Sun Microsystems, Inc. business.
Permission to use, copy, modify, and distribute this
software is freely granted, provided that this notice
is preserved.
```

### PSF-2.0

The Python license (`third_party/v8/builtins/LICENSE` of V8).

Used for: CPython list sort, as V8 has it (`third_party/v8/builtins/array-sort.tq`).

Taken from (fetched from the upstream project): https://chromium.googlesource.com/v8/v8/+/refs/tags/15.6.75/third_party/v8/builtins/LICENSE. `internal/js/LICENSE-PSF.txt` is a copy of that file.

```text
A. HISTORY OF THE SOFTWARE
==========================

Python was created in the early 1990s by Guido van Rossum at Stichting
Mathematisch Centrum (CWI, see http://www.cwi.nl) in the Netherlands
as a successor of a language called ABC.  Guido remains Python's
principal author, although it includes many contributions from others.

In 1995, Guido continued his work on Python at the Corporation for
National Research Initiatives (CNRI, see http://www.cnri.reston.va.us)
in Reston, Virginia where he released several versions of the
software.

In May 2000, Guido and the Python core development team moved to
BeOpen.com to form the BeOpen PythonLabs team.  In October of the same
year, the PythonLabs team moved to Digital Creations, which became
Zope Corporation.  In 2001, the Python Software Foundation (PSF, see
https://www.python.org/psf/) was formed, a non-profit organization
created specifically to own Python-related Intellectual Property.
Zope Corporation was a sponsoring member of the PSF.

All Python releases are Open Source (see http://www.opensource.org for
the Open Source Definition).  Historically, most, but not all, Python
releases have also been GPL-compatible; the table below summarizes
the various releases.

    Release         Derived     Year        Owner       GPL-
                    from                                compatible? (1)

    0.9.0 thru 1.2              1991-1995   CWI         yes
    1.3 thru 1.5.2  1.2         1995-1999   CNRI        yes
    1.6             1.5.2       2000        CNRI        no
    2.0             1.6         2000        BeOpen.com  no
    1.6.1           1.6         2001        CNRI        yes (2)
    2.1             2.0+1.6.1   2001        PSF         no
    2.0.1           2.0+1.6.1   2001        PSF         yes
    2.1.1           2.1+2.0.1   2001        PSF         yes
    2.1.2           2.1.1       2002        PSF         yes
    2.1.3           2.1.2       2002        PSF         yes
    2.2 and above   2.1.1       2001-now    PSF         yes

Footnotes:

(1) GPL-compatible doesn't mean that we're distributing Python under
    the GPL.  All Python licenses, unlike the GPL, let you distribute
    a modified version without making your changes open source.  The
    GPL-compatible licenses make it possible to combine Python with
    other software that is released under the GPL; the others don't.

(2) According to Richard Stallman, 1.6.1 is not GPL-compatible,
    because its license has a choice of law clause.  According to
    CNRI, however, Stallman's lawyer has told CNRI's lawyer that 1.6.1
    is "not incompatible" with the GPL.

Thanks to the many outside volunteers who have worked under Guido's
direction to make these releases possible.


B. TERMS AND CONDITIONS FOR ACCESSING OR OTHERWISE USING PYTHON
===============================================================

PYTHON SOFTWARE FOUNDATION LICENSE VERSION 2
--------------------------------------------

1. This LICENSE AGREEMENT is between the Python Software Foundation
("PSF"), and the Individual or Organization ("Licensee") accessing and
otherwise using this software ("Python") in source or binary form and
its associated documentation.

2. Subject to the terms and conditions of this License Agreement, PSF hereby
grants Licensee a nonexclusive, royalty-free, world-wide license to reproduce,
analyze, test, perform and/or display publicly, prepare derivative works,
distribute, and otherwise use Python alone or in any derivative version,
provided, however, that PSF's License Agreement and PSF's notice of copyright,
i.e., "Copyright (c) 2001, 2002, 2003, 2004, 2005, 2006, 2007, 2008, 2009, 2010,
2011, 2012, 2013, 2014, 2015, 2016, 2017, 2018 Python Software Foundation; All
Rights Reserved" are retained in Python alone or in any derivative version
prepared by Licensee.

3. In the event Licensee prepares a derivative work that is based on
or incorporates Python or any part thereof, and wants to make
the derivative work available to others as provided herein, then
Licensee hereby agrees to include in any such work a brief summary of
the changes made to Python.

4. PSF is making Python available to Licensee on an "AS IS"
basis.  PSF MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, PSF MAKES NO AND
DISCLAIMS ANY REPRESENTATION OR WARRANTY OF MERCHANTABILITY OR FITNESS
FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF PYTHON WILL NOT
INFRINGE ANY THIRD PARTY RIGHTS.

5. PSF SHALL NOT BE LIABLE TO LICENSEE OR ANY OTHER USERS OF PYTHON
FOR ANY INCIDENTAL, SPECIAL, OR CONSEQUENTIAL DAMAGES OR LOSS AS
A RESULT OF MODIFYING, DISTRIBUTING, OR OTHERWISE USING PYTHON,
OR ANY DERIVATIVE THEREOF, EVEN IF ADVISED OF THE POSSIBILITY THEREOF.

6. This License Agreement will automatically terminate upon a material
breach of its terms and conditions.

7. Nothing in this License Agreement shall be deemed to create any
relationship of agency, partnership, or joint venture between PSF and
Licensee.  This License Agreement does not grant permission to use PSF
trademarks or trade name in a trademark sense to endorse or promote
products or services of Licensee, or any third party.

8. By copying, installing or otherwise using Python, Licensee
agrees to be bound by the terms and conditions of this License
Agreement.


BEOPEN.COM LICENSE AGREEMENT FOR PYTHON 2.0
-------------------------------------------

BEOPEN PYTHON OPEN SOURCE LICENSE AGREEMENT VERSION 1

1. This LICENSE AGREEMENT is between BeOpen.com ("BeOpen"), having an
office at 160 Saratoga Avenue, Santa Clara, CA 95051, and the
Individual or Organization ("Licensee") accessing and otherwise using
this software in source or binary form and its associated
documentation ("the Software").

2. Subject to the terms and conditions of this BeOpen Python License
Agreement, BeOpen hereby grants Licensee a non-exclusive,
royalty-free, world-wide license to reproduce, analyze, test, perform
and/or display publicly, prepare derivative works, distribute, and
otherwise use the Software alone or in any derivative version,
provided, however, that the BeOpen Python License is retained in the
Software, alone or in any derivative version prepared by Licensee.

3. BeOpen is making the Software available to Licensee on an "AS IS"
basis.  BEOPEN MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, BEOPEN MAKES NO AND
DISCLAIMS ANY REPRESENTATION OR WARRANTY OF MERCHANTABILITY OR FITNESS
FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF THE SOFTWARE WILL NOT
INFRINGE ANY THIRD PARTY RIGHTS.

4. BEOPEN SHALL NOT BE LIABLE TO LICENSEE OR ANY OTHER USERS OF THE
SOFTWARE FOR ANY INCIDENTAL, SPECIAL, OR CONSEQUENTIAL DAMAGES OR LOSS
AS A RESULT OF USING, MODIFYING OR DISTRIBUTING THE SOFTWARE, OR ANY
DERIVATIVE THEREOF, EVEN IF ADVISED OF THE POSSIBILITY THEREOF.

5. This License Agreement will automatically terminate upon a material
breach of its terms and conditions.

6. This License Agreement shall be governed by and interpreted in all
respects by the law of the State of California, excluding conflict of
law provisions.  Nothing in this License Agreement shall be deemed to
create any relationship of agency, partnership, or joint venture
between BeOpen and Licensee.  This License Agreement does not grant
permission to use BeOpen trademarks or trade names in a trademark
sense to endorse or promote products or services of Licensee, or any
third party.  As an exception, the "BeOpen Python" logos available at
http://www.pythonlabs.com/logos.html may be used according to the
permissions granted on that web page.

7. By copying, installing or otherwise using the software, Licensee
agrees to be bound by the terms and conditions of this License
Agreement.


CNRI LICENSE AGREEMENT FOR PYTHON 1.6.1
---------------------------------------

1. This LICENSE AGREEMENT is between the Corporation for National
Research Initiatives, having an office at 1895 Preston White Drive,
Reston, VA 20191 ("CNRI"), and the Individual or Organization
("Licensee") accessing and otherwise using Python 1.6.1 software in
source or binary form and its associated documentation.

2. Subject to the terms and conditions of this License Agreement, CNRI
hereby grants Licensee a nonexclusive, royalty-free, world-wide
license to reproduce, analyze, test, perform and/or display publicly,
prepare derivative works, distribute, and otherwise use Python 1.6.1
alone or in any derivative version, provided, however, that CNRI's
License Agreement and CNRI's notice of copyright, i.e., "Copyright (c)
1995-2001 Corporation for National Research Initiatives; All Rights
Reserved" are retained in Python 1.6.1 alone or in any derivative
version prepared by Licensee.  Alternately, in lieu of CNRI's License
Agreement, Licensee may substitute the following text (omitting the
quotes): "Python 1.6.1 is made available subject to the terms and
conditions in CNRI's License Agreement.  This Agreement together with
Python 1.6.1 may be located on the Internet using the following
unique, persistent identifier (known as a handle): 1895.22/1013.  This
Agreement may also be obtained from a proxy server on the Internet
using the following URL: http://hdl.handle.net/1895.22/1013".

3. In the event Licensee prepares a derivative work that is based on
or incorporates Python 1.6.1 or any part thereof, and wants to make
the derivative work available to others as provided herein, then
Licensee hereby agrees to include in any such work a brief summary of
the changes made to Python 1.6.1.

4. CNRI is making Python 1.6.1 available to Licensee on an "AS IS"
basis.  CNRI MAKES NO REPRESENTATIONS OR WARRANTIES, EXPRESS OR
IMPLIED.  BY WAY OF EXAMPLE, BUT NOT LIMITATION, CNRI MAKES NO AND
DISCLAIMS ANY REPRESENTATION OR WARRANTY OF MERCHANTABILITY OR FITNESS
FOR ANY PARTICULAR PURPOSE OR THAT THE USE OF PYTHON 1.6.1 WILL NOT
INFRINGE ANY THIRD PARTY RIGHTS.

5. CNRI SHALL NOT BE LIABLE TO LICENSEE OR ANY OTHER USERS OF PYTHON
1.6.1 FOR ANY INCIDENTAL, SPECIAL, OR CONSEQUENTIAL DAMAGES OR LOSS AS
A RESULT OF MODIFYING, DISTRIBUTING, OR OTHERWISE USING PYTHON 1.6.1,
OR ANY DERIVATIVE THEREOF, EVEN IF ADVISED OF THE POSSIBILITY THEREOF.

6. This License Agreement will automatically terminate upon a material
breach of its terms and conditions.

7. This License Agreement shall be governed by the federal
intellectual property law of the United States, including without
limitation the federal copyright law, and, to the extent such
U.S. federal law does not apply, by the law of the Commonwealth of
Virginia, excluding Virginia's conflict of law provisions.
Notwithstanding the foregoing, with regard to derivative works based
on Python 1.6.1 that incorporate non-separable material that was
previously distributed under the GNU General Public License (GPL), the
law of the Commonwealth of Virginia shall govern this License
Agreement only as to issues arising under or with respect to
Paragraphs 4, 5, and 7 of this License Agreement.  Nothing in this
License Agreement shall be deemed to create any relationship of
agency, partnership, or joint venture between CNRI and Licensee.  This
License Agreement does not grant permission to use CNRI trademarks or
trade name in a trademark sense to endorse or promote products or
services of Licensee, or any third party.

8. By clicking on the "ACCEPT" button where indicated, or by copying,
installing or otherwise using Python 1.6.1, Licensee agrees to be
bound by the terms and conditions of this License Agreement.

        ACCEPT


CWI LICENSE AGREEMENT FOR PYTHON 0.9.0 THROUGH 1.2
--------------------------------------------------

Copyright (c) 1991 - 1995, Stichting Mathematisch Centrum Amsterdam,
The Netherlands.  All rights reserved.

Permission to use, copy, modify, and distribute this software and its
documentation for any purpose and without fee is hereby granted,
provided that the above copyright notice appear in all copies and that
both that copyright notice and this permission notice appear in
supporting documentation, and that the name of Stichting Mathematisch
Centrum or CWI not be used in advertising or publicity pertaining to
distribution of the software without specific, written prior
permission.

STICHTING MATHEMATISCH CENTRUM DISCLAIMS ALL WARRANTIES WITH REGARD TO
THIS SOFTWARE, INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS, IN NO EVENT SHALL STICHTING MATHEMATISCH CENTRUM BE LIABLE
FOR ANY SPECIAL, INDIRECT OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER IN AN
ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION, ARISING OUT
OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THIS SOFTWARE.
```

### MIT-Modern-Variant

The license of HarfBuzz ("Old MIT").

Used for: HarfBuzz.

Taken from (fetched from the upstream project): https://github.com/harfbuzz/harfbuzz/blob/main/COPYING. `internal/font/LICENSE-HarfBuzz.txt` has the file `COPYING` at revision `f5efbbef3841`, the revision in Chromium 156.0.8078.4.

```text
Permission is hereby granted, without written agreement and without
license or royalty fees, to use, copy, modify, and distribute this
software and its documentation for any purpose, provided that the
above copyright notice and the following two paragraphs appear in
all copies of this software.

IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE TO ANY PARTY FOR
DIRECT, INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES
ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS DOCUMENTATION, EVEN
IF THE COPYRIGHT HOLDER HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH
DAMAGE.

THE COPYRIGHT HOLDER SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING,
BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
FITNESS FOR A PARTICULAR PURPOSE.  THE SOFTWARE PROVIDED HEREUNDER IS
ON AN "AS IS" BASIS, AND THE COPYRIGHT HOLDER HAS NO OBLIGATION TO
PROVIDE MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, OR MODIFICATIONS.
```

### FTL

The FreeType Project LICENSE.

Used for: FreeType 2.13.2.

Taken from (the repository): `internal/font/LICENSE-FreeType.txt`.

```text
                    The FreeType Project LICENSE
                    ----------------------------

                            2006-Jan-27

                    Copyright 1996-2002, 2006 by
          David Turner, Robert Wilhelm, and Werner Lemberg



Introduction
============

  The FreeType  Project is distributed in  several archive packages;
  some of them may contain, in addition to the FreeType font engine,
  various tools and  contributions which rely on, or  relate to, the
  FreeType Project.

  This  license applies  to all  files found  in such  packages, and
  which do not  fall under their own explicit  license.  The license
  affects  thus  the  FreeType   font  engine,  the  test  programs,
  documentation and makefiles, at the very least.

  This  license   was  inspired  by  the  BSD,   Artistic,  and  IJG
  (Independent JPEG  Group) licenses, which  all encourage inclusion
  and  use of  free  software in  commercial  and freeware  products
  alike.  As a consequence, its main points are that:

    o We don't promise that this software works. However, we will be
      interested in any kind of bug reports. (`as is' distribution)

    o You can  use this software for whatever you  want, in parts or
      full form, without having to pay us. (`royalty-free' usage)

    o You may not pretend that  you wrote this software.  If you use
      it, or  only parts of it,  in a program,  you must acknowledge
      somewhere  in  your  documentation  that  you  have  used  the
      FreeType code. (`credits')

  We  specifically  permit  and  encourage  the  inclusion  of  this
  software, with  or without modifications,  in commercial products.
  We  disclaim  all warranties  covering  The  FreeType Project  and
  assume no liability related to The FreeType Project.


  Finally,  many  people  asked  us  for  a  preferred  form  for  a
  credit/disclaimer to use in compliance with this license.  We thus
  encourage you to use the following text:

   """
    Portions of this software are copyright © <year> The FreeType
    Project (www.freetype.org).  All rights reserved.
   """

  Please replace <year> with the value from the FreeType version you
  actually use.


Legal Terms
===========

0. Definitions
--------------

  Throughout this license,  the terms `package', `FreeType Project',
  and  `FreeType  archive' refer  to  the  set  of files  originally
  distributed  by the  authors  (David Turner,  Robert Wilhelm,  and
  Werner Lemberg) as the `FreeType Project', be they named as alpha,
  beta or final release.

  `You' refers to  the licensee, or person using  the project, where
  `using' is a generic term including compiling the project's source
  code as  well as linking it  to form a  `program' or `executable'.
  This  program is  referred to  as  `a program  using the  FreeType
  engine'.

  This  license applies  to all  files distributed  in  the original
  FreeType  Project,   including  all  source   code,  binaries  and
  documentation,  unless  otherwise  stated   in  the  file  in  its
  original, unmodified form as  distributed in the original archive.
  If you are  unsure whether or not a particular  file is covered by
  this license, you must contact us to verify this.

  The FreeType  Project is copyright (C) 1996-2000  by David Turner,
  Robert Wilhelm, and Werner Lemberg.  All rights reserved except as
  specified below.

1. No Warranty
--------------

  THE FREETYPE PROJECT  IS PROVIDED `AS IS' WITHOUT  WARRANTY OF ANY
  KIND, EITHER  EXPRESS OR IMPLIED,  INCLUDING, BUT NOT  LIMITED TO,
  WARRANTIES  OF  MERCHANTABILITY   AND  FITNESS  FOR  A  PARTICULAR
  PURPOSE.  IN NO EVENT WILL ANY OF THE AUTHORS OR COPYRIGHT HOLDERS
  BE LIABLE  FOR ANY DAMAGES CAUSED  BY THE USE OR  THE INABILITY TO
  USE, OF THE FREETYPE PROJECT.

2. Redistribution
-----------------

  This  license  grants  a  worldwide, royalty-free,  perpetual  and
  irrevocable right  and license to use,  execute, perform, compile,
  display,  copy,   create  derivative  works   of,  distribute  and
  sublicense the  FreeType Project (in  both source and  object code
  forms)  and  derivative works  thereof  for  any  purpose; and  to
  authorize others  to exercise  some or all  of the  rights granted
  herein, subject to the following conditions:

    o Redistribution of  source code  must retain this  license file
      (`FTL.TXT') unaltered; any  additions, deletions or changes to
      the original  files must be clearly  indicated in accompanying
      documentation.   The  copyright   notices  of  the  unaltered,
      original  files must  be  preserved in  all  copies of  source
      files.

    o Redistribution in binary form must provide a  disclaimer  that
      states  that  the software is based in part of the work of the
      FreeType Team,  in  the  distribution  documentation.  We also
      encourage you to put an URL to the FreeType web page  in  your
      documentation, though this isn't mandatory.

  These conditions  apply to any  software derived from or  based on
  the FreeType Project,  not just the unmodified files.   If you use
  our work, you  must acknowledge us.  However, no  fee need be paid
  to us.

3. Advertising
--------------

  Neither the  FreeType authors and  contributors nor you  shall use
  the name of the  other for commercial, advertising, or promotional
  purposes without specific prior written permission.

  We suggest,  but do not require, that  you use one or  more of the
  following phrases to refer  to this software in your documentation
  or advertising  materials: `FreeType Project',  `FreeType Engine',
  `FreeType library', or `FreeType Distribution'.

  As  you have  not signed  this license,  you are  not  required to
  accept  it.   However,  as  the FreeType  Project  is  copyrighted
  material, only  this license, or  another one contracted  with the
  authors, grants you  the right to use, distribute,  and modify it.
  Therefore,  by  using,  distributing,  or modifying  the  FreeType
  Project, you indicate that you understand and accept all the terms
  of this license.

4. Contacts
-----------

  There are two mailing lists related to FreeType:

    o freetype@nongnu.org

      Discusses general use and applications of FreeType, as well as
      future and  wanted additions to the  library and distribution.
      If  you are looking  for support,  start in  this list  if you
      haven't found anything to help you in the documentation.

    o freetype-devel@nongnu.org

      Discusses bugs,  as well  as engine internals,  design issues,
      specific licenses, porting, etc.

  Our home page can be found at

    https://www.freetype.org


--- end of FTL.TXT ---
```

### fontconfig

The file COPYING of fontconfig 2.15.0.

Used for: fontconfig 2.15.0.

Taken from (the repository): `internal/font/LICENSE-fontconfig.txt`.

```text
fontconfig/COPYING

Copyright © 2000,2001,2002,2003,2004,2006,2007 Keith Packard
Copyright © 2005 Patrick Lam
Copyright © 2007 Dwayne Bailey and Translate.org.za
Copyright © 2009 Roozbeh Pournader
Copyright © 2008,2009,2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020 Red Hat, Inc.
Copyright © 2008 Danilo Šegan
Copyright © 2012 Google, Inc.


Permission to use, copy, modify, distribute, and sell this software and its
documentation for any purpose is hereby granted without fee, provided that
the above copyright notice appear in all copies and that both that
copyright notice and this permission notice appear in supporting
documentation, and that the name of the author(s) not be used in
advertising or publicity pertaining to distribution of the software without
specific, written prior permission.  The authors make no
representations about the suitability of this software for any purpose.  It
is provided "as is" without express or implied warranty.

THE AUTHOR(S) DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS, IN NO
EVENT SHALL THE AUTHOR(S) BE LIABLE FOR ANY SPECIAL, INDIRECT OR
CONSEQUENTIAL DAMAGES OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE,
DATA OR PROFITS, WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER
TORTIOUS ACTION, ARISING OUT OF OR IN CONNECTION WITH THE USE OR
PERFORMANCE OF THIS SOFTWARE.


--------------------------------------------------------------------------------
fontconfig/fc-case/CaseFolding.txt

© 2019 Unicode®, Inc.
Unicode and the Unicode Logo are registered trademarks of Unicode, Inc. in the U.S. and other countries.
For terms of use, see http://www.unicode.org/terms_of_use.html


--------------------------------------------------------------------------------
fontconfig/src/fcatomic.h

/*
 * Mutex operations.  Originally copied from HarfBuzz.
 *
 * Copyright © 2007  Chris Wilson
 * Copyright © 2009,2010  Red Hat, Inc.
 * Copyright © 2011,2012,2013  Google, Inc.
 *
 * Permission is hereby granted, without written agreement and without
 * license or royalty fees, to use, copy, modify, and distribute this
 * software and its documentation for any purpose, provided that the
 * above copyright notice and the following two paragraphs appear in
 * all copies of this software.
 *
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE TO ANY PARTY FOR
 * DIRECT, INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES
 * ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS DOCUMENTATION, EVEN
 * IF THE COPYRIGHT HOLDER HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH
 * DAMAGE.
 *
 * THE COPYRIGHT HOLDER SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING,
 * BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
 * FITNESS FOR A PARTICULAR PURPOSE.  THE SOFTWARE PROVIDED HEREUNDER IS
 * ON AN "AS IS" BASIS, AND THE COPYRIGHT HOLDER HAS NO OBLIGATION TO
 * PROVIDE MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, OR MODIFICATIONS.
 *
 * Contributor(s):
 *	Chris Wilson <chris@chris-wilson.co.uk>
 * Red Hat Author(s): Behdad Esfahbod
 * Google Author(s): Behdad Esfahbod
 */


--------------------------------------------------------------------------------
fontconfig/src/fcfoundry.h

/*
  Copyright © 2002-2003 by Juliusz Chroboczek

  Permission is hereby granted, free of charge, to any person obtaining a copy
  of this software and associated documentation files (the "Software"), to deal
  in the Software without restriction, including without limitation the rights
  to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
  copies of the Software, and to permit persons to whom the Software is
  furnished to do so, subject to the following conditions:

  The above copyright notice and this permission notice shall be included in
  all copies or substantial portions of the Software.

  THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
  IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
  FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.  IN NO EVENT SHALL THE
  AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
  LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
  OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
  THE SOFTWARE.
*/


--------------------------------------------------------------------------------
fontconfig/src/fcmd5.h

/*
 * This code implements the MD5 message-digest algorithm.
 * The algorithm is due to Ron Rivest.  This code was
 * written by Colin Plumb in 1993, no copyright is claimed.
 * This code is in the public domain; do with it what you wish.
 *
 * Equivalent code is available from RSA Data Security, Inc.
 * This code has been tested against that, and is equivalent,
 * except that you don't need to include two pages of legalese
 * with every copy.
 *
 * To compute the message digest of a chunk of bytes, declare an
 * MD5Context structure, pass it to MD5Init, call MD5Update as
 * needed on buffers full of bytes, and then call MD5Final, which
 * will fill a supplied 16-byte array with the digest.
 */


--------------------------------------------------------------------------------
fontconfig/src/fcmutex.h

/*
 * Atomic int and pointer operations.  Originally copied from HarfBuzz.
 *
 * Copyright © 2007  Chris Wilson
 * Copyright © 2009,2010  Red Hat, Inc.
 * Copyright © 2011,2012,2013  Google, Inc.
 *
 * Permission is hereby granted, without written agreement and without
 * license or royalty fees, to use, copy, modify, and distribute this
 * software and its documentation for any purpose, provided that the
 * above copyright notice and the following two paragraphs appear in
 * all copies of this software.
 *
 * IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE TO ANY PARTY FOR
 * DIRECT, INDIRECT, SPECIAL, INCIDENTAL, OR CONSEQUENTIAL DAMAGES
 * ARISING OUT OF THE USE OF THIS SOFTWARE AND ITS DOCUMENTATION, EVEN
 * IF THE COPYRIGHT HOLDER HAS BEEN ADVISED OF THE POSSIBILITY OF SUCH
 * DAMAGE.
 *
 * THE COPYRIGHT HOLDER SPECIFICALLY DISCLAIMS ANY WARRANTIES, INCLUDING,
 * BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND
 * FITNESS FOR A PARTICULAR PURPOSE.  THE SOFTWARE PROVIDED HEREUNDER IS
 * ON AN "AS IS" BASIS, AND THE COPYRIGHT HOLDER HAS NO OBLIGATION TO
 * PROVIDE MAINTENANCE, SUPPORT, UPDATES, ENHANCEMENTS, OR MODIFICATIONS.
 *
 * Contributor(s):
 *	Chris Wilson <chris@chris-wilson.co.uk>
 * Red Hat Author(s): Behdad Esfahbod
 * Google Author(s): Behdad Esfahbod
 */


--------------------------------------------------------------------------------
fontconfig/src/ftglue.[ch]

/* ftglue.c: Glue code for compiling the OpenType code from
 *           FreeType 1 using only the public API of FreeType 2
 *
 * By David Turner, The FreeType Project (www.freetype.org)
 *
 * This code is explicitely put in the public domain
 *
 * ==========================================================================
 *
 * the OpenType parser codes was originally written as an extension to
 * FreeType 1.x. As such, its source code was embedded within the library,
 * and used many internal FreeType functions to deal with memory and
 * stream i/o.
 *
 * When it was 'salvaged' for Pango and Qt, the code was "ported" to FreeType 2,
 * which basically means that some macro tricks were performed in order to
 * directly access FT2 _internal_ functions.
 *
 * these functions were never part of FT2 public API, and _did_ change between
 * various releases. This created chaos for many users: when they upgraded the
 * FreeType library on their system, they couldn't run Gnome anymore since
 * Pango refused to link.
 *
 * Very fortunately, it's possible to completely avoid this problem because
 * the FT_StreamRec and FT_MemoryRec structure types, which describe how
 * memory and stream implementations interface with the rest of the font
 * library, have always been part of the public API, and never changed.
 *
 * What we do thus is re-implement, within the OpenType parser, the few
 * functions that depend on them. This only adds one or two kilobytes of
 * code, and ensures that the parser can work with _any_ version
 * of FreeType installed on your system. How sweet... !
 *
 * Note that we assume that Pango doesn't use any other internal functions
 * from FreeType. It used to in old versions, but this should no longer
 * be the case. (crossing my fingers).
 *
 *  - David Turner
 *  - The FreeType Project  (www.freetype.org)
 *
 * PS: This "glue" code is explicitely put in the public domain
 */
```

### EPL-2.0

Eclipse Public License - v 2.0.

Used for: Eclipse Layout Kernel (ELK), elkjs 0.9.3, Guava, Eclipse EMF, Xtext (compiled code inside the elkjs worker).

Taken from (the repository): `internal/elk/native/LICENSE`. `internal/elk/LICENSE-EPL-2.0.txt` and `internal/elk/elkjs/LICENSE.md` have the same text.

```text
# Eclipse Public License - v 2.0

THE ACCOMPANYING PROGRAM IS PROVIDED UNDER THE TERMS OF THIS ECLIPSE
PUBLIC LICENSE (“AGREEMENT”). ANY USE, REPRODUCTION OR DISTRIBUTION OF
THE PROGRAM CONSTITUTES RECIPIENT'S ACCEPTANCE OF THIS AGREEMENT.

## 1. DEFINITIONS

“Contribution” means:

-   a\) in the case of the initial Contributor, the initial content
    Distributed under this Agreement, and
-   b\) in the case of each subsequent Contributor:
    -   i\) changes to the Program, and
    -   ii\) additions to the Program;

    where such changes and/or additions to the Program originate from
    and are Distributed by that particular Contributor. A Contribution
    “originates” from a Contributor if it was added to the Program by
    such Contributor itself or anyone acting on such Contributor's
    behalf. Contributions do not include changes or additions to the
    Program that are not Modified Works.

“Contributor” means any person or entity that Distributes the Program.

“Licensed Patents” mean patent claims licensable by a Contributor which
are necessarily infringed by the use or sale of its Contribution alone
or when combined with the Program.

“Program” means the Contributions Distributed in accordance with this
Agreement.

“Recipient” means anyone who receives the Program under this Agreement
or any Secondary License (as applicable), including Contributors.

“Derivative Works” shall mean any work, whether in Source Code or other
form, that is based on (or derived from) the Program and for which the
editorial revisions, annotations, elaborations, or other modifications
represent, as a whole, an original work of authorship.

“Modified Works” shall mean any work in Source Code or other form that
results from an addition to, deletion from, or modification of the
contents of the Program, including, for purposes of clarity any new file
in Source Code form that contains any contents of the Program. Modified
Works shall not include works that contain only declarations,
interfaces, types, classes, structures, or files of the Program solely
in each case in order to link to, bind by name, or subclass the Program
or Modified Works thereof.

“Distribute” means the acts of a) distributing or b) making available in
any manner that enables the transfer of a copy.

“Source Code” means the form of a Program preferred for making
modifications, including but not limited to software source code,
documentation source, and configuration files.

“Secondary License” means either the GNU General Public License, Version
2.0, or any later versions of that license, including any exceptions or
additional permissions as identified by the initial Contributor.

## 2. GRANT OF RIGHTS

-   a\) Subject to the terms of this Agreement, each Contributor hereby
    grants Recipient a non-exclusive, worldwide, royalty-free copyright
    license to reproduce, prepare Derivative Works of, publicly display,
    publicly perform, Distribute and sublicense the Contribution of such
    Contributor, if any, and such Derivative Works.
-   b\) Subject to the terms of this Agreement, each Contributor hereby
    grants Recipient a non-exclusive, worldwide, royalty-free patent license
    under Licensed Patents to make, use, sell, offer to sell, import and
    otherwise transfer the Contribution of such Contributor, if any, in
    Source Code or other form. This patent license shall apply to the
    combination of the Contribution and the Program if, at the time the
    Contribution is added by the Contributor, such addition of the
    Contribution causes such combination to be covered by the
    Licensed Patents. The patent license shall not apply to any other
    combinations which include the Contribution. No hardware per se is
    licensed hereunder.
-   c\) Recipient understands that although each Contributor grants the
    licenses to its Contributions set forth herein, no assurances are
    provided by any Contributor that the Program does not infringe the
    patent or other intellectual property rights of any other entity. Each
    Contributor disclaims any liability to Recipient for claims brought by
    any other entity based on infringement of intellectual property rights
    or otherwise. As a condition to exercising the rights and licenses
    granted hereunder, each Recipient hereby assumes sole responsibility to
    secure any other intellectual property rights needed, if any. For
    example, if a third party patent license is required to allow Recipient
    to Distribute the Program, it is Recipient's responsibility to acquire
    that license before distributing the Program.
-   d\) Each Contributor represents that to its knowledge it has sufficient
    copyright rights in its Contribution, if any, to grant the copyright
    license set forth in this Agreement.
-   e\) Notwithstanding the terms of any Secondary License, no Contributor
    makes additional grants to any Recipient (other than those set forth in
    this Agreement) as a result of such Recipient's receipt of the Program
    under the terms of a Secondary License (if permitted under the terms of
    Section 3).

## 3. REQUIREMENTS

3.1 If a Contributor Distributes the Program in any form, then:

-   a\) the Program must also be made available as Source Code, in accordance
    with section 3.2, and the Contributor must accompany the Program with a
    statement that the Source Code for the Program is available under this
    Agreement, and informs Recipients how to obtain it in a reasonable
    manner on or through a medium customarily used for software exchange;
    and
-   b\) the Contributor may Distribute the Program under a license different
    than this Agreement, provided that such license:
    -   i\) effectively disclaims on behalf of all other Contributors all
        warranties and conditions, express and implied, including warranties or
        conditions of title and non-infringement, and implied warranties or
        conditions of merchantability and fitness for a particular purpose;
    -   ii\) effectively excludes on behalf of all other Contributors all
        liability for damages, including direct, indirect, special, incidental
        and consequential damages, such as lost profits;
    -   iii\) does not attempt to limit or alter the recipients' rights in the
        Source Code under section 3.2; and
    -   iv\) requires any subsequent distribution of the Program by any party to
        be under a license that satisfies the requirements of this section 3.

3.2 When the Program is Distributed as Source Code:

-   a\) it must be made available under this Agreement, or if the Program (i)
    is combined with other material in a separate file or files made
    available under a Secondary License, and (ii) the initial Contributor
    attached to the Source Code the notice described in Exhibit A of this
    Agreement, then the Program may be made available under the terms of
    such Secondary Licenses, and
-   b\) a copy of this Agreement must be included with each copy of
    the Program.

3.3 Contributors may not remove or alter any copyright, patent,
trademark, attribution notices, disclaimers of warranty, or limitations
of liability (‘notices’) contained within the Program from any copy of
the Program which they Distribute, provided that Contributors may add
their own appropriate notices.

## 4. COMMERCIAL DISTRIBUTION

Commercial distributors of software may accept certain responsibilities
with respect to end users, business partners and the like. While this
license is intended to facilitate the commercial use of the Program, the
Contributor who includes the Program in a commercial product offering
should do so in a manner which does not create potential liability for
other Contributors. Therefore, if a Contributor includes the Program in
a commercial product offering, such Contributor (“Commercial
Contributor”) hereby agrees to defend and indemnify every other
Contributor (“Indemnified Contributor”) against any losses, damages and
costs (collectively “Losses”) arising from claims, lawsuits and other
legal actions brought by a third party against the Indemnified
Contributor to the extent caused by the acts or omissions of such
Commercial Contributor in connection with its distribution of the
Program in a commercial product offering. The obligations in this
section do not apply to any claims or Losses relating to any actual or
alleged intellectual property infringement. In order to qualify, an
Indemnified Contributor must: a) promptly notify the Commercial
Contributor in writing of such claim, and b) allow the Commercial
Contributor to control, and cooperate with the Commercial Contributor
in, the defense and any related settlement negotiations. The Indemnified
Contributor may participate in any such claim at its own expense.

For example, a Contributor might include the Program in a commercial
product offering, Product X. That Contributor is then a Commercial
Contributor. If that Commercial Contributor then makes performance
claims, or offers warranties related to Product X, those performance
claims and warranties are such Commercial Contributor's responsibility
alone. Under this section, the Commercial Contributor would have to
defend claims against the other Contributors related to those
performance claims and warranties, and if a court requires any other
Contributor to pay any damages as a result, the Commercial Contributor
must pay those damages.

## 5. NO WARRANTY {#warranty}

EXCEPT AS EXPRESSLY SET FORTH IN THIS AGREEMENT, AND TO THE EXTENT
PERMITTED BY APPLICABLE LAW, THE PROGRAM IS PROVIDED ON AN “AS IS”
BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, EITHER EXPRESS OR
IMPLIED INCLUDING, WITHOUT LIMITATION, ANY WARRANTIES OR CONDITIONS OF
TITLE, NON-INFRINGEMENT, MERCHANTABILITY OR FITNESS FOR A PARTICULAR
PURPOSE. Each Recipient is solely responsible for determining the
appropriateness of using and distributing the Program and assumes all
risks associated with its exercise of rights under this Agreement,
including but not limited to the risks and costs of program errors,
compliance with applicable laws, damage to or loss of data, programs or
equipment, and unavailability or interruption of operations.

## 6. DISCLAIMER OF LIABILITY {#disclaimer}

EXCEPT AS EXPRESSLY SET FORTH IN THIS AGREEMENT, AND TO THE EXTENT
PERMITTED BY APPLICABLE LAW, NEITHER RECIPIENT NOR ANY CONTRIBUTORS
SHALL HAVE ANY LIABILITY FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL,
EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING WITHOUT LIMITATION LOST
PROFITS), HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN
CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE)
ARISING IN ANY WAY OUT OF THE USE OR DISTRIBUTION OF THE PROGRAM OR THE
EXERCISE OF ANY RIGHTS GRANTED HEREUNDER, EVEN IF ADVISED OF THE
POSSIBILITY OF SUCH DAMAGES.

## 7. GENERAL

If any provision of this Agreement is invalid or unenforceable under
applicable law, it shall not affect the validity or enforceability of
the remainder of the terms of this Agreement, and without further action
by the parties hereto, such provision shall be reformed to the minimum
extent necessary to make such provision valid and enforceable.

If Recipient institutes patent litigation against any entity (including
a cross-claim or counterclaim in a lawsuit) alleging that the Program
itself (excluding combinations of the Program with other software or
hardware) infringes such Recipient's patent(s), then such Recipient's
rights granted under Section 2(b) shall terminate as of the date such
litigation is filed.

All Recipient's rights under this Agreement shall terminate if it fails
to comply with any of the material terms or conditions of this Agreement
and does not cure such failure in a reasonable period of time after
becoming aware of such noncompliance. If all Recipient's rights under
this Agreement terminate, Recipient agrees to cease use and distribution
of the Program as soon as reasonably practicable. However, Recipient's
obligations under this Agreement and any licenses granted by Recipient
relating to the Program shall continue and survive.

Everyone is permitted to copy and distribute copies of this Agreement,
but in order to avoid inconsistency the Agreement is copyrighted and may
only be modified in the following manner. The Agreement Steward reserves
the right to publish new versions (including revisions) of this
Agreement from time to time. No one other than the Agreement Steward has
the right to modify this Agreement. The Eclipse Foundation is the
initial Agreement Steward. The Eclipse Foundation may assign the
responsibility to serve as the Agreement Steward to a suitable separate
entity. Each new version of the Agreement will be given a distinguishing
version number. The Program (including Contributions) may always be
Distributed subject to the version of the Agreement under which it was
received. In addition, after a new version of the Agreement is
published, Contributor may elect to Distribute the Program (including
its Contributions) under the new version.

Except as expressly stated in Sections 2(a) and 2(b) above, Recipient
receives no rights or licenses to the intellectual property of any
Contributor under this Agreement, whether expressly, by implication,
estoppel or otherwise. All rights in the Program not expressly granted
under this Agreement are reserved. Nothing in this Agreement is intended
to be enforceable by any entity that is not a Contributor or Recipient.
No third-party beneficiary rights are created under this Agreement.

## Exhibit A – Form of Secondary Licenses Notice {#exhibit-a}

“This Source Code may also be made available under the following
Secondary Licenses when the conditions for such availability set forth
in the Eclipse Public License, v. 2.0 are satisfied: {name license(s),
version(s), and exceptions or additional permissions here}.”

> Simply including a copy of this Agreement, including this Exhibit A is
> not sufficient to license the Source Code under Secondary Licenses.
>
> If it is not possible or desirable to put the notice in a particular
> file, then You may include the notice in a location (such as a LICENSE
> file in a relevant directory) where a recipient would be likely to
> look for such a notice.
>
> You may add additional accurate notices of copyright ownership.
```

### OFL-1.1

The license file of the Liberation fonts (SIL Open Font License 1.1).

Used for: Liberation fonts 2.1.5.

Taken from (the repository): `internal/font/liberation/LICENSE`.

```text
Liberation Fonts 2.1.5
Source: https://github.com/liberationfonts

The font files in this directory are the files of the Ubuntu package
fonts-liberation 1:2.1.5-3, compressed with gzip. The text below is the
copyright notice and the license from the copyright file of that package.

Copyright:
Digitized data copyright (c) 2010 Google Corporation with Reserved Font Arimo, Tinos and Cousine.
Copyright (c) 2012 Red Hat, Inc. with Reserved Font Name Liberation.

License: SIL Open Font License, Version 1.1

This Font Software is licensed under the SIL Open Font License,
Version 1.1.

This license is copied below, and is also available with a FAQ at:
http://scripts.sil.org/OFL

SIL OPEN FONT LICENSE Version 1.1 - 26 February 2007

PREAMBLE The goals of the Open Font License (OFL) are to stimulate
worldwide development of collaborative font projects, to support the font
creation efforts of academic and linguistic communities, and to provide
a free and open framework in which fonts may be shared and improved in
partnership with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves.
The fonts, including any derivative works, can be bundled, embedded,
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works.  The fonts and derivatives,
however, cannot be released under any other type of license.  The
requirement for fonts to remain under this license does not apply to
any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such.
This may include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components
as distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting ? in part or in whole ?
any of the components of the Original Version, by changing formats or
by porting the Font Software to a new environment.

"Author" refers to any designer, engineer, programmer, technical writer
or other person who contributed to the Font Software.

PERMISSION & CONDITIONS

Permission is hereby granted, free of charge, to any person obtaining a
copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,in
   Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
   redistributed and/or sold with any software, provided that each copy
   contains the above copyright notice and this license. These can be
   included either as stand-alone text files, human-readable headers or
   in the appropriate machine-readable metadata fields within text or
   binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
   Name(s) unless explicit written permission is granted by the
   corresponding Copyright Holder. This restriction only applies to the
   primary font name as presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
   Software shall not be used to promote, endorse or advertise any
   Modified Version, except to acknowledge the contribution(s) of the
   Copyright Holder(s) and the Author(s) or with their explicit written
   permission.

5) The Font Software, modified or unmodified, in part or in whole, must
   be distributed entirely under this license, and must not be distributed
   under any other license. The requirement for fonts to remain under
   this license does not apply to any document created using the Font
   Software.

TERMINATION
This license becomes null and void if any of the above conditions are not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT.  IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM OTHER
DEALINGS IN THE FONT SOFTWARE.
```

### OFL-1.1, Recursive

The license file of the package `@fontsource-variable/recursive` (SIL Open Font License 1.1).

Used for: Recursive, `@fontsource-variable/recursive` 5.3.0.

Taken from (the repository): `internal/webfonts/files/recursive/LICENSE`.

```text
Copyright 2019 The Recursive Project Authors (github.com/arrowtype/recursive)

This Font Software is licensed under the SIL Open Font License, Version 1.1.
This license is copied below, and is also available with a FAQ at:
http://scripts.sil.org/OFL


-----------------------------------------------------------
SIL OPEN FONT LICENSE Version 1.1 - 26 February 2007
-----------------------------------------------------------

PREAMBLE
The goals of the Open Font License (OFL) are to stimulate worldwide
development of collaborative font projects, to support the font creation
efforts of academic and linguistic communities, and to provide a free and
open framework in which fonts may be shared and improved in partnership
with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves. The
fonts, including any derivative works, can be bundled, embedded,
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works. The fonts and derivatives,
however, cannot be released under any other type of license. The
requirement for fonts to remain under this license does not apply
to any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such. This may
include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components as
distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting -- in part or in whole -- any of the components of the
Original Version, by changing formats or by porting the Font Software to a
new environment.

"Author" refers to any designer, engineer, programmer, technical
writer or other person who contributed to the Font Software.

PERMISSION & CONDITIONS
Permission is hereby granted, free of charge, to any person obtaining
a copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,
in Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
redistributed and/or sold with any software, provided that each copy
contains the above copyright notice and this license. These can be
included either as stand-alone text files, human-readable headers or
in the appropriate machine-readable metadata fields within text or
binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
Name(s) unless explicit written permission is granted by the corresponding
Copyright Holder. This restriction only applies to the primary font name as
presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
Software shall not be used to promote, endorse or advertise any
Modified Version, except to acknowledge the contribution(s) of the
Copyright Holder(s) and the Author(s) or with their explicit written
permission.

5) The Font Software, modified or unmodified, in part or in whole,
must be distributed entirely under this license, and must not be
distributed under any other license. The requirement for fonts to
remain under this license does not apply to any document created
using the Font Software.

TERMINATION
This license becomes null and void if any of the above conditions are
not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
OTHER DEALINGS IN THE FONT SOFTWARE.
```

### OFL-1.1, Open Sans

The license file of the package `@fontsource/open-sans` (SIL Open Font License 1.1).

Used for: Open Sans, `@fontsource/open-sans` 5.3.0.

Taken from (the repository): `internal/webfonts/files/open-sans/LICENSE`.

```text
Copyright 2020 The Open Sans Project Authors (https://github.com/googlefonts/opensans) OpenSans-Italic[wdth,wght].ttf: Copyright 2020 The Open Sans Project Authors (https://github.com/googlefonts/opensans)

This Font Software is licensed under the SIL Open Font License, Version 1.1.
This license is copied below, and is also available with a FAQ at:
http://scripts.sil.org/OFL


-----------------------------------------------------------
SIL OPEN FONT LICENSE Version 1.1 - 26 February 2007
-----------------------------------------------------------

PREAMBLE
The goals of the Open Font License (OFL) are to stimulate worldwide
development of collaborative font projects, to support the font creation
efforts of academic and linguistic communities, and to provide a free and
open framework in which fonts may be shared and improved in partnership
with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves. The
fonts, including any derivative works, can be bundled, embedded,
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works. The fonts and derivatives,
however, cannot be released under any other type of license. The
requirement for fonts to remain under this license does not apply
to any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such. This may
include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components as
distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting -- in part or in whole -- any of the components of the
Original Version, by changing formats or by porting the Font Software to a
new environment.

"Author" refers to any designer, engineer, programmer, technical
writer or other person who contributed to the Font Software.

PERMISSION & CONDITIONS
Permission is hereby granted, free of charge, to any person obtaining
a copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,
in Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
redistributed and/or sold with any software, provided that each copy
contains the above copyright notice and this license. These can be
included either as stand-alone text files, human-readable headers or
in the appropriate machine-readable metadata fields within text or
binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
Name(s) unless explicit written permission is granted by the corresponding
Copyright Holder. This restriction only applies to the primary font name as
presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
Software shall not be used to promote, endorse or advertise any
Modified Version, except to acknowledge the contribution(s) of the
Copyright Holder(s) and the Author(s) or with their explicit written
permission.

5) The Font Software, modified or unmodified, in part or in whole,
must be distributed entirely under this license, and must not be
distributed under any other license. The requirement for fonts to
remain under this license does not apply to any document created
using the Font Software.

TERMINATION
This license becomes null and void if any of the above conditions are
not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
OTHER DEALINGS IN THE FONT SOFTWARE.
```

### Font Awesome Free

The license file of the package `@fortawesome/fontawesome-free`. It has the SIL Open Font License 1.1 for the fonts, the MIT license for the code, and it names CC BY 4.0 for the icons.

Used for: Font Awesome Free 7.3.1.

Taken from (the repository): `internal/webfonts/files/fontawesome-free/LICENSE.txt`.

```text
Fonticons, Inc. (https://fontawesome.com)

--------------------------------------------------------------------------------

Font Awesome Free License

Font Awesome Free is free, open source, and GPL friendly. You can use it for
commercial projects, open source projects, or really almost whatever you want.
Full Font Awesome Free license: https://fontawesome.com/license/free.

--------------------------------------------------------------------------------

# Icons: CC BY 4.0 License (https://creativecommons.org/licenses/by/4.0/)

The Font Awesome Free download is licensed under a Creative Commons
Attribution 4.0 International License and applies to all icons packaged
as SVG and JS file types.

--------------------------------------------------------------------------------

# Fonts: SIL OFL 1.1 License

In the Font Awesome Free download, the SIL OFL license applies to all icons
packaged as web and desktop font files.

Copyright (c) 2026 Fonticons, Inc. (https://fontawesome.com)
with Reserved Font Name: "Font Awesome".

This Font Software is licensed under the SIL Open Font License, Version 1.1.
This license is copied below, and is also available with a FAQ at:
http://scripts.sil.org/OFL

SIL OPEN FONT LICENSE
Version 1.1 - 26 February 2007

PREAMBLE
The goals of the Open Font License (OFL) are to stimulate worldwide
development of collaborative font projects, to support the font creation
efforts of academic and linguistic communities, and to provide a free and
open framework in which fonts may be shared and improved in partnership
with others.

The OFL allows the licensed fonts to be used, studied, modified and
redistributed freely as long as they are not sold by themselves. The
fonts, including any derivative works, can be bundled, embedded,
redistributed and/or sold with any software provided that any reserved
names are not used by derivative works. The fonts and derivatives,
however, cannot be released under any other type of license. The
requirement for fonts to remain under this license does not apply
to any document created using the fonts or their derivatives.

DEFINITIONS
"Font Software" refers to the set of files released by the Copyright
Holder(s) under this license and clearly marked as such. This may
include source files, build scripts and documentation.

"Reserved Font Name" refers to any names specified as such after the
copyright statement(s).

"Original Version" refers to the collection of Font Software components as
distributed by the Copyright Holder(s).

"Modified Version" refers to any derivative made by adding to, deleting,
or substituting — in part or in whole — any of the components of the
Original Version, by changing formats or by porting the Font Software to a
new environment.

"Author" refers to any designer, engineer, programmer, technical
writer or other person who contributed to the Font Software.

PERMISSION & CONDITIONS
Permission is hereby granted, free of charge, to any person obtaining
a copy of the Font Software, to use, study, copy, merge, embed, modify,
redistribute, and sell modified and unmodified copies of the Font
Software, subject to the following conditions:

1) Neither the Font Software nor any of its individual components,
in Original or Modified Versions, may be sold by itself.

2) Original or Modified Versions of the Font Software may be bundled,
redistributed and/or sold with any software, provided that each copy
contains the above copyright notice and this license. These can be
included either as stand-alone text files, human-readable headers or
in the appropriate machine-readable metadata fields within text or
binary files as long as those fields can be easily viewed by the user.

3) No Modified Version of the Font Software may use the Reserved Font
Name(s) unless explicit written permission is granted by the corresponding
Copyright Holder. This restriction only applies to the primary font name as
presented to the users.

4) The name(s) of the Copyright Holder(s) or the Author(s) of the Font
Software shall not be used to promote, endorse or advertise any
Modified Version, except to acknowledge the contribution(s) of the
Copyright Holder(s) and the Author(s) or with their explicit written
permission.

5) The Font Software, modified or unmodified, in part or in whole,
must be distributed entirely under this license, and must not be
distributed under any other license. The requirement for fonts to
remain under this license does not apply to any document created
using the Font Software.

TERMINATION
This license becomes null and void if any of the above conditions are
not met.

DISCLAIMER
THE FONT SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO ANY WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT
OF COPYRIGHT, PATENT, TRADEMARK, OR OTHER RIGHT. IN NO EVENT SHALL THE
COPYRIGHT HOLDER BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY,
INCLUDING ANY GENERAL, SPECIAL, INDIRECT, INCIDENTAL, OR CONSEQUENTIAL
DAMAGES, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING
FROM, OUT OF THE USE OR INABILITY TO USE THE FONT SOFTWARE OR FROM
OTHER DEALINGS IN THE FONT SOFTWARE.

--------------------------------------------------------------------------------

# Code: MIT License (https://opensource.org/licenses/MIT)

In the Font Awesome Free download, the MIT license applies to all non-font and
non-icon files.

Copyright 2026 Fonticons, Inc.

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in the
Software without restriction, including without limitation the rights to use, copy,
modify, merge, publish, distribute, sublicense, and/or sell copies of the Software,
and to permit persons to whom the Software is furnished to do so, subject to the
following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR IMPLIED,
INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY, FITNESS FOR A
PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE AUTHORS OR COPYRIGHT
HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER LIABILITY, WHETHER IN AN ACTION
OF CONTRACT, TORT OR OTHERWISE, ARISING FROM, OUT OF OR IN CONNECTION WITH THE
SOFTWARE OR THE USE OR OTHER DEALINGS IN THE SOFTWARE.

--------------------------------------------------------------------------------

# Attribution

Attribution is required by MIT, SIL OFL, and CC BY licenses. Downloaded Font
Awesome Free files already contain embedded comments with sufficient
attribution, so you shouldn't need to do anything additional when using these
files normally.

We've kept attribution comments terse, so we ask that you do not actively work
to remove them from files, especially code. They're a great way for folks to
learn about Font Awesome.

--------------------------------------------------------------------------------

# Brand Icons

All brand icons are trademarks of their respective owners. The use of these
trademarks does not indicate endorsement of the trademark holder by Font
Awesome, nor vice versa. **Please do not use brand logos for any purpose except
to represent the company, product, or service to which they refer.**
```

### Unicode-3.0

UNICODE LICENSE V3.

Used for: Unicode data and ICU data.

Taken from (the license file of the reference Chromium): section `icu` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1).

```text
UNICODE LICENSE V3

COPYRIGHT AND PERMISSION NOTICE

Copyright © 2016-2025 Unicode, Inc.

NOTICE TO USER: Carefully read the following legal agreement. BY
DOWNLOADING, INSTALLING, COPYING OR OTHERWISE USING DATA FILES, AND/OR
SOFTWARE, YOU UNEQUIVOCALLY ACCEPT, AND AGREE TO BE BOUND BY, ALL OF THE
TERMS AND CONDITIONS OF THIS AGREEMENT. IF YOU DO NOT AGREE, DO NOT
DOWNLOAD, INSTALL, COPY, DISTRIBUTE OR USE THE DATA FILES OR SOFTWARE.

Permission is hereby granted, free of charge, to any person obtaining a
copy of data files and any associated documentation (the "Data Files") or
software and any associated documentation (the "Software") to deal in the
Data Files or Software without restriction, including without limitation
the rights to use, copy, modify, merge, publish, distribute, and/or sell
copies of the Data Files or Software, and to permit persons to whom the
Data Files or Software are furnished to do so, provided that either (a)
this copyright and permission notice appear with all copies of the Data
Files or Software, or (b) this copyright and permission notice appear in
associated Documentation.

THE DATA FILES AND SOFTWARE ARE PROVIDED "AS IS", WITHOUT WARRANTY OF ANY
KIND, EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT OF
THIRD PARTY RIGHTS.

IN NO EVENT SHALL THE COPYRIGHT HOLDER OR HOLDERS INCLUDED IN THIS NOTICE
BE LIABLE FOR ANY CLAIM, OR ANY SPECIAL INDIRECT OR CONSEQUENTIAL DAMAGES,
OR ANY DAMAGES WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS,
WHETHER IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF THE DATA
FILES OR SOFTWARE.

Except as contained in this notice, the name of a copyright holder shall
not be used in advertising or otherwise to promote the sale, use or other
dealings in these Data Files or Software without prior written
authorization of the copyright holder.

SPDX-License-Identifier: Unicode-3.0
```

### BSD-3-Clause, text 5

Used for: golang.org/x/net v0.55.0, golang.org/x/text v0.43.0, golang.org/x/image v0.47.0, The Go standard library and runtime.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/golang.org/x/net@v0.55.0/LICENSE`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.
   * Neither the name of Google LLC nor the names of its
contributors may be used to endorse or promote products derived from
this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### BSD-2-Clause

Used for: github.com/go-sourcemap/sourcemap v2.1.3+incompatible.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/github.com/go-sourcemap/sourcemap@v2.1.3+incompatible/LICENSE`.

The copyright lines of each project are in the tables above. They go in the place of the copyright line of this text.

```text
Redistribution and use in source and binary forms, with or without
modification, are permitted provided that the following conditions are
met:

   * Redistributions of source code must retain the above copyright
notice, this list of conditions and the following disclaimer.
   * Redistributions in binary form must reproduce the above
copyright notice, this list of conditions and the following disclaimer
in the documentation and/or other materials provided with the
distribution.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS
"AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT
LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR
A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT
OWNER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT
LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE,
DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY
THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
(INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE
OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
```

### Lucent

The notice of Lucent Technologies (goja `ftoa/LICENSE_LUCENE`).

Used for: github.com/dop251/goja, package `ftoa` same.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/github.com/dop251/goja@v0.0.0-20261029234554-1329d889cf98/ftoa/LICENSE_LUCENE`.

```text
Copyright (C) 1998, 1999 by Lucent Technologies
All Rights Reserved

Permission to use, copy, modify, and distribute this software and
its documentation for any purpose and without fee is hereby
granted, provided that the above copyright notice appear in all
copies and that both that the copyright notice and this
permission notice and warranty disclaimer appear in supporting
documentation, and that the name of Lucent or any of its entities
not be used in advertising or publicity pertaining to
distribution of the software without specific, written prior
permission.

LUCENT DISCLAIMS ALL WARRANTIES WITH REGARD TO THIS SOFTWARE,
INCLUDING ALL IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS.
IN NO EVENT SHALL LUCENT OR ANY OF ITS ENTITIES BE LIABLE FOR ANY
SPECIAL, INDIRECT OR CONSEQUENTIAL DAMAGES OR ANY DAMAGES
WHATSOEVER RESULTING FROM LOSS OF USE, DATA OR PROFITS, WHETHER
IN AN ACTION OF CONTRACT, NEGLIGENCE OR OTHER TORTIOUS ACTION,
ARISING OUT OF OR IN CONNECTION WITH THE USE OR PERFORMANCE OF
THIS SOFTWARE.
```

### go-text

The license file of go-text/typesetting.

Used for: github.com/go-text/typesetting v0.3.5.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/github.com/go-text/typesetting@v0.3.5/LICENSE`.

```text
This project is provided under the terms of the UNLICENSE or
the BSD license denoted by the following SPDX identifier:

SPDX-License-Identifier: Unlicense OR BSD-3-Clause

You may use the project under the terms of either license.

Both licenses are reproduced below.

----
The BSD 3 Clause License

Copyright 2021 The go-text authors

Redistribution and use in source and binary forms, with or without modification, are permitted provided that the following conditions are met:

1. Redistributions of source code must retain the above copyright notice, this list of conditions and the following disclaimer.

2. Redistributions in binary form must reproduce the above copyright notice, this list of conditions and the following disclaimer in the documentation and/or other materials provided with the distribution.

3. Neither the name of the copyright holder nor the names of its contributors may be used to endorse or promote products derived from this software without specific prior written permission.

THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
---



---
The UNLICENSE

This is free and unencumbered software released into the public domain.

Anyone is free to copy, modify, publish, use, compile, sell, or
distribute this software, either in source code form or as a compiled
binary, for any purpose, commercial or non-commercial, and by any
means.

In jurisdictions that recognize copyright laws, the author or authors
of this software dedicate any and all copyright interest in the
software to the public domain. We make this dedication for the benefit
of the public at large and to the detriment of our heirs and
successors. We intend this dedication to be an overt act of
relinquishment in perpetuity of all present and future rights to this
software under copyright law.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND,
EXPRESS OR IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF
MERCHANTABILITY, FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT.
IN NO EVENT SHALL THE AUTHORS BE LIABLE FOR ANY CLAIM, DAMAGES OR
OTHER LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE,
ARISING FROM, OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR
OTHER DEALINGS IN THE SOFTWARE.

For more information, please refer to <https://unlicense.org/>
---
```

### go-text harfbuzz

The license file of the package harfbuzz of go-text/typesetting.

Used for: github.com/go-text/typesetting, package `harfbuzz` v0.3.5.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/github.com/go-text/typesetting@v0.3.5/harfbuzz/LICENSE`.

```text
MIT License

Copyright (c) 2021 B. KUGLER
Copyright © 2010,2011,2012,2013,2014,2015,2016,2017,2018,2019,2020  Google, Inc.
Copyright © 2018,2019,2020  Ebrahim Byagowi
Copyright © 2019,2020  Facebook, Inc. 
Copyright © 2012  Mozilla Foundation
Copyright © 2011  Codethink Limited
Copyright © 2008,2010  Nokia Corporation and/or its subsidiary(-ies)
Copyright © 2009  Keith Stribley
Copyright © 2009  Martin Hosken and SIL International
Copyright © 2007  Chris Wilson
Copyright © 2006  Behdad Esfahbod
Copyright © 2005  David Turner
Copyright © 2004,2007,2008,2009,2010  Red Hat, Inc.
Copyright © 1998-2004  David Turner and Werner Lemberg

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.
```

### yaml.v3

The license file and the NOTICE file of gopkg.in/yaml.v3.

Used for: gopkg.in/yaml.v3 v3.0.1.

Taken from (the Go module cache or the Go distribution): `$GOMODCACHE/gopkg.in/yaml.v3@v3.0.1/LICENSE`.

```text
This project is covered by two different licenses: MIT and Apache.

#### MIT License ####

The following files were ported to Go from C files of libyaml, and thus
are still covered by their original MIT license, with the additional
copyright staring in 2011 when the project was ported over:

    apic.go emitterc.go parserc.go readerc.go scannerc.go
    writerc.go yamlh.go yamlprivateh.go

Copyright (c) 2006-2010 Kirill Simonov
Copyright (c) 2006-2011 Kirill Simonov

Permission is hereby granted, free of charge, to any person obtaining a copy of
this software and associated documentation files (the "Software"), to deal in
the Software without restriction, including without limitation the rights to
use, copy, modify, merge, publish, distribute, sublicense, and/or sell copies
of the Software, and to permit persons to whom the Software is furnished to do
so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in all
copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN THE
SOFTWARE.

### Apache License ###

All the remaining project files are covered by the Apache license:

Copyright (c) 2011-2019 Canonical Ltd

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.

---- NOTICE ----

Copyright 2011-2016 Canonical Ltd.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
```

### LLVM-exception

LLVM Exceptions to the Apache 2.0 License.

Used for: LLVM libc.

Taken from (the license file of the reference Chromium): section `llvm-libc` of `LICENSE.headless_shell` of Chromium 156.0.8078.4 (Playwright 1.64.1).

```text
---- LLVM Exceptions to the Apache 2.0 License ----

As an exception, if, as a result of your compiling your source code, portions
of this Software are embedded into an Object form of such source code, you
may redistribute such embedded portions in such Object form without complying
with the conditions of Sections 4(a), 4(b) and 4(d) of the License.

In addition, if you combine or link compiled forms of this Software with
software that is licensed under the GPLv2 ("Combined Software") and if a
court of competent jurisdiction determines that the patent provision (Section
3), the indemnity provision (Section 9) or other Section of the License
conflicts with the conditions of the GPLv2, you may retroactively and
prospectively choose to deem waived or otherwise exclude such Section(s) of
the License, but only in their entirety and only with respect to the Combined
Software.
```
