# License audit

This document has the findings of a provenance and license audit of this repository. The audit was made on 2026-11-09 on the branch `rewrite/mermaid-parity`, at commit `f6f6519` with uncommitted changes of other work. The branch is not published: it has no upstream, and no remote branch contains its commits.

The companion documents are [PROVENANCE.md](PROVENANCE.md), which says where each package comes from, and [THIRD_PARTY_NOTICES.md](../THIRD_PARTY_NOTICES.md), which has the copyright lines and the license texts.

This document is not legal advice. It says what was found in the files, and what the text of each license says. The decisions are for the owner of the repository. Each finding names them.

A fix pass on 2026-11-09 applied the mechanical fixes of this audit on the same branch. Its counts are those of the working tree at commit `c21c3ec`. Each finding has a paragraph that starts with `State on 2026-11-09:`. It says what is fixed now and what remains for the owner. The section "What the fix pass of 2026-11-09 changed" lists the changes. The fix pass made no decision for the owner.

## Decisions after the audit

This section is newer than the findings. The findings below have the text of the audit, with a note where a decision changed their state.

| Date | Decision | Findings |
|---|---|---|
| 2026-11-09 | The owner chose the MIT license for the repository. The file `LICENSE` in the root has its text. It covers the original code. Ported code keeps the terms of its source, as the section "If the owner chooses MIT or Apache-2.0" says. | F9 |
| 2026-11-09 | The port of DOMPurify is taken under the Apache License 2.0. This follows from the MIT choice (see the same section). `internal/purify` and THIRD_PARTY_NOTICES.md say so, and the text of the MPL is no longer in the repository. | F5 |
| 2026-11-09 | The history of the branch `rewrite/mermaid-parity` is not published. The published branch `bvolpato/mermaid-12-port` has the tree as one commit on top of `main`, so the commits of F2 and F3 are not on GitHub. | F2, F3 |

Still open:

- F1. The Go files that follow LGPL files of Blink are in the tree, with headers and notices that name their sources. The MIT license of the repository does not apply to them. The owner has not chosen between the four options of F1.
- The second point of F3 (the form of one expression in `linux_libm_tanh.go`), and F14, F15 and the other findings whose state says "Open".
- The Homebrew formula has no license field (F9), because no short SPDX expression is true for the binary.

## Summary

The audit found no license file. Its text follows. Before the repository was published, the owner had to decide four things.

1. What to do with the Go code that follows source files of Blink that have the GNU Library General Public License (finding F1). This is the only copyleft code that every render uses.
2. If the history of the branch is published. Old commits have code that was written from the machine code of a closed-source library of Apple, and one file that says it is a port of glibc (findings F2 and F3). The working tree has neither now.
3. Which license DOMPurify is taken under (finding F5), and which license the repository itself gets (see "If the owner chooses MIT or Apache-2.0").
4. How the notices go with a binary release (finding F9).

| ID | Finding | Seriousness | Decision or work | State on 2026-11-09 |
|---|---|---|---|---|
| F1 | `internal/browser` and `internal/raster` follow Blink files that are LGPL-2.0-or-later | high | Decision of the owner | The headers and the notice files say which Go file follows which Blink file, with the license of each. The decision is open |
| F2 | Code written from the machine code of `libsystem_m` of Apple. Deleted from the tree in `41d617f`. It is in 23 commits of the branch | high for the history, low for the tree | Decision about the history | The comments are fixed. The history is not published (see "Decisions after the audit") |
| F3 | glibc: one file in 4 old commits says `Ported from glibc 2.39`. One expression in `linux_libm_tanh.go` has the form of glibc | medium | Decision about the history. Check of one function | The history is not published. The check of the function is open |
| F4 | EPL-2.0 covers `internal/elk/native`, the embedded elkjs worker and the glue in `internal/elk/engine.go` | medium | Mark `internal/elk`. Tell users | `internal/elk/engine.go` is marked. `internal/elk` has the license text and a notice. The README does not tell users yet |
| F5 | DOMPurify is `MPL-2.0 OR Apache-2.0`. The port does not say which one it uses | medium | Decision of the owner | Decided: Apache-2.0. The port and the notices file say so |
| F6 | The conditions of the Apache License 2.0 are not met in the files that port chevrotain, GWT, LLVM libc and emoji-segmenter | medium | Add notices | The notices are in the Go files. A copy of the license is in each directory |
| F7 | `internal/elk/native/NOTICE` puts the ports of GWT code under EPL-2.0, with no Apache text next to them | low | Add the text | `LICENSE-GWT.txt` and a notice are in `internal/elk/native/gwt` |
| F8 | Notices that must stay in the file are missing: fdlibm in `hyp.go`, Python in `sort_dagre.go`, the copyright lines of Blink files with BSD terms | medium | Add notices | The notices are in the files |
| F9 | No license file. The release configuration says MIT and does not put notices in the archives | medium | Decision of the owner | Decided: MIT, in `LICENSE`. The archives get `LICENSE` and `THIRD_PARTY_NOTICES.md`. The formula has no license claim |
| F10 | Embedded files contain code of other projects with no notice next to them | low | Add notices | Notices are next to the elkjs worker, the cytoscape bundle and the grammar files |
| F11 | The sources that prove the provenance are not in git | low | Document how to get them | License texts of nine projects are in git now. The rest is open |
| F12 | Three ports followed a newer version than the mermaid 12.2.0 bundle contains | fixed for parity, low for licenses | Done on 2026-11-09: the ports follow the bundle versions | The notices file names the bundle versions now |
| F13 | 230 Go files have no `Ported from` header. 162 of them are not generated and do not say that they port nothing | low | Add headers in the packages that other work owns | 215 of 1054 files have no `Ported from` header. 105 of them are not generated and do not say that they port nothing. No header is after the imports |
| F14 | Test inputs follow the examples of the CommonMark and GFM specifications (CC-BY-SA-4.0) | low | Decision of the owner | No change. Open |
| F15 | Test screenshots show glyphs of fonts of macOS | low | Decision of the owner | No change. Open |
| F16 | Unicode and ICU data tables need the Unicode notice | low | Done in the notices file | No change |
| F17 | Small parts with their own notice: fmin, heap, JamaJS | low | Done in the notices file, except heap | The license file of heap has no copyright line. Its text is in `internal/cytoscape/NOTICE` |

## Findings

### F1. Go code follows Blink files that have the LGPL

**What.** Blink, the layout engine of Chromium, is not under one license. The files that Google wrote say `Use of this source code is governed by a BSD-style license`. Many older files come from WebKit and KHTML. They have the GNU Library General Public License, version 2 or later (SPDX `LGPL-2.0-or-later`), or BSD terms with the copyright of Apple. Most of the SVG code of Blink is in the first group.

The Go code in `internal/browser` and `internal/raster` names the Blink classes that it follows. Several of those classes are in LGPL files.

**Where.** The license of each Blink file in this table was read from the header of the file at the tag `156.0.8078.4` of https://chromium.googlesource.com/chromium/src on 2026-11-09. The fix pass of 2026-11-09 read each header a second time, and it extended the table to every Go file that names a Blink file. The paths are in `third_party/blink/renderer`. The paths that start with `ui/gfx`, `base` and `cc` are in the root of Chromium. "BSD-3-Clause" with no note means the license of Chromium: the file says `Use of this source code is governed by a BSD-style license`. "In the file" means that the file has the license terms and its own copyright lines in its first comment.

| Go file | Blink files with LGPL-2.0-or-later that the Go file follows or names | Other files of Chromium that the Go file follows or names |
|---|---|---|
| `internal/browser/pathquery.go` | `core/svg/svg_path_query.cc`, `core/svg/svg_path_parser.cc` and `svg_path_parser.h` (`SVGPathNormalizer`), `core/svg/svg_path_string_source.cc`, `core/svg/svg_path_data.h`, `platform/graphics/path_traversal_state.cc` | `ui/gfx/geometry/vector2d_f.cc` (BSD-3-Clause) |
| `internal/browser/path.go` | `core/svg/svg_path_builder.cc`, `core/svg/svg_path_string_source.cc` | `platform/geometry/path.cc` (BSD-2-Clause in the file: Apple, Eric Seidel, Google, Intel). `ui/gfx/geometry/skia_conversions.cc` (BSD-3-Clause). Skia (BSD-3-Clause) |
| `internal/browser/svgroot.go` | `core/layout/svg/layout_svg_root.cc` and `layout_svg_root.h`, `core/svg/svg_preserve_aspect_ratio.cc`, `core/svg/svg_svg_element.cc`, `core/svg/svg_graphics_element.cc`, `core/layout/svg/svg_layout_support.cc`, `core/layout/layout_object.h` (`LocalToAbsoluteQuad`) | `core/layout/geometry/transform_state.cc` (BSD-2-Clause in the file: Apple). `platform/geometry/physical_size.cc` (BSD-3-Clause). `platform/geometry/layout_unit.h` (BSD-3-Clause in the file: Google). `ui/gfx/geometry` and `base/numerics` (BSD-3-Clause) |
| `internal/browser/svgviewport.go` | `core/svg/svg_length_functions.cc` (`SVGViewportResolver::ResolveViewport`, `ValueForLength`), `core/svg/svg_length_context.cc`, `core/css/css_primitive_value.cc` (`kMaxValueForCssLength`) | none |
| `internal/browser/geom.go` | `core/svg/svg_parser_utilities.cc` (`GenericParseNumber`) | `platform/transforms/affine_transform.cc` (BSD-2-Clause in the file: Apple, Google). `ui/gfx/geometry` (BSD-3-Clause) |
| `internal/browser/boundschange.go` | `core/layout/svg/layout_svg_container.cc`, `core/layout/svg/layout_svg_shape.cc` | `core/layout/svg/svg_content_container.cc`, `core/layout/svg/svg_layout_info.h` (BSD-3-Clause) |
| `internal/browser/norender.go` | `core/svg/svg_g_element.cc`, `core/svg/svg_tests.cc`, `core/svg/svg_switch_element.cc`, `core/layout/svg/layout_svg_transformable_container.cc` (`IsChildAllowed`) | none |
| `internal/browser/text.go` | `core/layout/svg/layout_svg_inline_text.cc`, `core/svg/svg_length.cc`, `platform/fonts/font_metrics.h` (`FixedAscent`) | `platform/fonts/shaping/shape_result.cc` (BSD-3-Clause in the file: Google, BlackBerry). `core/layout/inline/fragment_item.cc`, `core/layout/svg/svg_text_layout_algorithm.cc`, `core/css/parser/css_parser_fast_paths.cc` (BSD-3-Clause). `platform/geometry/layout_unit.h` |
| `internal/browser/htmlinline.go` | `core/layout/layout_inline.cc`, `core/layout/layout_object.cc`, `core/dom/element.cc`, `core/style/computed_style.h` and `computed_style.cc`, `platform/fonts/font_metrics.h`, `platform/geometry/length_functions.cc` | `core/layout/inline/inline_box_state.cc`, `inline_items_builder.cc`, `line_info.h`, `platform/fonts/shaping/shape_result_view.h`, `shaping_line_breaker.cc`, `ui/gfx/geometry/rect_f.cc` (BSD-3-Clause) |
| `internal/browser/engine.go` | `core/layout/layout_object.cc`, `core/layout/layout_inline.cc`, `core/dom/element.cc`, `core/style/computed_style.cc` | `core/layout/svg/svg_content_container.cc`, `core/layout/svg/layout_svg_text.cc`, `ui/gfx/geometry` (BSD-3-Clause) |
| `internal/browser/fontmono.go` | `core/css/resolver/font_builder.cc`, `core/css/font_size_functions.cc`, `platform/fonts/font_description.h` | none |
| `internal/browser/style.go` | `platform/fonts/font_description.h` (`IsAbsoluteSize`) | `platform/fonts/font_description.cc` (BSD-3-Clause in the file: Nicholas Shanks, Apple). `core/css/css_to_length_conversion_data.cc` (BSD-3-Clause in the file: Google) |
| `internal/browser/transform.go` | `core/style/computed_style.cc` (`ApplyTransform`) | `platform/transforms/affine_transform.cc` (BSD-2-Clause in the file). `ui/gfx/geometry` and `base/numerics` (BSD-3-Clause) |
| `internal/browser/linebreak.go`, `linebreak_ascii_gen.go` | `platform/text/text_break_iterator.cc` is named. A probe recorded the Go table from the browser. It was not copied from the file | none |
| `internal/browser/html.go`, `image.go` | none | `core/layout/inline` (LayoutNG), `core/layout/length_utils.cc`, `core/html/html_image_fallback_helper.cc` (BSD-3-Clause). `platform/geometry/layout_unit.h` |
| `internal/raster/svg_marker.go` | `core/layout/svg/svg_marker_data.cc` and `svg_marker_data.h`, `core/layout/svg/layout_svg_resource_marker.cc`, `core/layout/svg/svg_layout_support.cc` | `core/paint/svg_shape_painter.cc` (BSD-3-Clause). `platform/transforms/affine_transform.cc` (BSD-2-Clause in the file) |
| `internal/raster/svg_gradient.go` | `core/svg/svg_gradient_element.cc`, `core/layout/svg/layout_svg_resource_gradient.cc` | none |
| `internal/raster/svg_filter.go` | `core/layout/svg/svg_resources.cc`, `platform/graphics/filters/fe_drop_shadow.cc` | `core/paint/filter_effect_builder.cc` (BSD-2-Clause in the file: Apple, Google). `platform/graphics/filters/paint_filter_builder.cc` (BSD-2-Clause in the file: Google). `cc/paint/render_surface_filters.cc` (BSD-3-Clause) |
| `internal/raster/svg.go` | `core/layout/svg/layout_svg_root.cc`, `core/layout/svg/layout_svg_shape.cc`, `core/svg/svg_tests.cc` | `core/paint/svg_root_painter.cc`, `svg_container_painter.cc`, `svg_shape_painter.cc`, `svg_object_painter.cc` (BSD-3-Clause). `platform/geometry/stroke_data.cc` (BSD-3-Clause in the file: Google) |
| `internal/raster/render.go` | `core/svg/svg_preserve_aspect_ratio.cc`, `core/layout/layout_replaced.cc` (`ComputeNaturalSizingInfo`), `core/layout/svg/layout_svg_root.cc` | `core/layout/length_utils.cc`, `platform/geometry/physical_size.cc` (BSD-3-Clause) |
| `internal/raster/svg_effects.go` | `core/layout/svg/layout_svg_resource_clipper.cc`, `core/layout/svg/svg_layout_support.cc`, `core/style/computed_style.h` | `core/paint/paint_property_tree_builder.cc`, `platform/graphics/compositing/paint_chunks_to_cc_layer.cc` (BSD-3-Clause) |
| `internal/raster/svg_text.go` | `platform/fonts/font_metrics.h`, `core/style/computed_style.h` | `platform/fonts/font_metrics.cc` (BSD-3-Clause in the file: Apple, Alexey Proskuryakov). `core/paint`, `core/layout/inline`, `ui/gfx/font_render_params_linux.cc` (BSD-3-Clause) |
| `internal/dom/purify_support.go` | `core/dom/element.cc` (two functions). `core/dom/node_iterator.cc` has the class of the node iterator. The Go code follows the DOM standard for it, and it does not say that it follows the Blink file | none |
| `internal/dom/node.go` | `core/dom/element.cc` (one function) | none. The file also ports parts of d3-selection (ISC) |
| `internal/dom/selector.go` | `core/css/selector_checker.cc` (one comment) | none |
| `internal/css/cssom.go` | `core/css/css_property_value_set.cc` is named in one comment. The CSSOM serializer of Blink is `core/css/style_property_serializer.cc` and `css_primitive_value.cc`. The Go file names no file that it ports, and it does not say if it follows these two | `core/css/css_numeric_literal_value.cc` (BSD-3-Clause) |
| `internal/font/xheight.go` | `platform/fonts/font_metrics.h` (one comment) | none |

**Differences from the first table of this audit.** The second check confirmed every license of the first table. It found these differences.

- The first table did not have these LGPL files: `platform/fonts/font_metrics.h`, `platform/fonts/font_description.h`, `core/dom/element.cc`, `core/style/computed_style.cc` and `computed_style.h`, `platform/geometry/length_functions.cc`, `core/svg/svg_length.cc`, `core/svg/svg_length_functions.cc`, `core/css/resolver/font_builder.cc`, `core/css/font_size_functions.cc`, `core/css/selector_checker.cc`, `core/css/css_property_value_set.cc`, `core/layout/layout_replaced.cc`, `core/layout/svg/layout_svg_transformable_container.cc` and `core/layout/svg/layout_svg_resource_clipper.cc`.
- The first table did not have these Go files: `internal/browser/fontmono.go` (other work added it after the audit), `style.go` and `transform.go`, `internal/dom/node.go` and `selector.go`, `internal/font/xheight.go`, `internal/raster/svg_effects.go` and `svg_text.go`.
- The first table said for `text.go` and `htmlinline.go` that the LayoutNG classes are BSD-3-Clause. That is right for the LayoutNG files. But these two Go files also name `FontMetrics::FixedAscent`, and `platform/fonts/font_metrics.h` has the LGPL.
- A `.cc` file and its `.h` file can have different licenses. `platform/fonts/font_metrics.cc` has BSD terms, and `font_metrics.h` has the LGPL. `platform/graphics/path_traversal_state.cc` has the LGPL, and `path_traversal_state.h` has BSD-2-Clause.
- `internal/browser/path.go` also follows `platform/geometry/path.cc`, which has BSD-2-Clause in the file.
- Some Go comments name a function that Chromium `156.0.8078.4` does not have under that name. `SVGLengthContext::DetermineViewport` is `SVGViewportResolver::ResolveViewport` in `core/svg/svg_length_functions.cc`. `SVGSwitchElement::ChildShouldCreateLayoutObject` is `LayoutSVGTransformableContainer::IsChildAllowed`. `SelectorChecker::TagMatches` is `MatchesTagName` in `core/css/selector_checker.cc`. `LayoutSVGRoot::ComputeNaturalSizingInfo` is a function of `LayoutReplaced`. `platform/text/text_break_iterator.cc` has no `ShouldBreakAfter` and no `kAsciiLineBreakTable`. `core/svg/graphics/svg_image.cc` has no `SVGImage::ConcreteObjectSize`, which `internal/imagesize/imagesize.go` names. `SkPath::arcTo`, which `path.go` names, is in `SkPathBuilder.cpp` of Skia at the revision of that Chromium. So these Go comments follow another version of Blink and of Skia. The Go files do not record which version. The licenses of the files did not depend on these names in any case that the check found.

One function was compared with its source. `viewBoxToViewTransform` in `internal/browser/svgroot.go` has the statements of `SVGPreserveAspectRatio::ComputeTransform` in `core/svg/svg_preserve_aspect_ratio.cc`, in the same order, with the same expressions and the same names (`logical`, `physical`). The other functions were not compared line by line.

A directory with copies of Blink files is on the developer machine: `/tmp/tcw-chromium-src`. Three of its files have the LGPL header: `layout_svg_root.cc`, `layout_replaced.cc` and `core_svg_svg_svg_element.cc`.

The license file of the reference browser confirms the mix. Its section "WebKit" says: "The terms and conditions vary from file to file", and it gives BSD terms, the GNU Library General Public License version 2 and the GNU Lesser General Public License version 2.1.

**Why it matters.** Section 0 of the LGPL says that a "work based on the Library" is a work that contains the library or a part of it, "either verbatim or with modifications and/or translated straightforwardly into another language". The LGPL sets conditions for such a work. Other code of this repository is under licenses that only ask for notices. Before the fix pass, the headers in `internal/font` said that the Blink files they name have the "BSD 3-clause license" of `LICENSE-Chromium.txt`. That was not exact: eight of those files have BSD terms in the file with other copyright holders, and one of the eight has two clauses (finding F8). The headers now give the license of each file. BSD terms are not a rule for all of Blink.

`internal/browser` is the measurement engine. Every render calls it. So this code cannot be left out as the ELK port can.

**What the license says.** These points are from the text of the GNU Library General Public License, version 2. THIRD_PARTY_NOTICES.md has the text.

- Section 2: a changed copy of the library, or a work based on it, must be licensed as a whole at no charge under the same license. The changed files must say that they were changed, and when.
- Section 3: the holder of a copy can choose to apply the GNU General Public License in place of this license.
- Section 5: a program that only uses the library, and is not linked with it, is outside the license. An executable that is linked with the library is a derivative of the library.
- Section 6: a work that is linked with the library can be distributed under other terms, when those terms allow changes for the own use of the customer and reverse engineering to debug such changes. The distributor must say that the library is used and that this license covers it, must give a copy of this license, and must give the source code of the library. For an executable, the distributor must also give the rest of the program as object code or as source code, so that the user can change the library and link the program again.
- Section 0 also says: "Activities other than copying, distribution and modification are not covered by this License."

A Go program is one executable. So a company that links this module into a program and gives that program to others must meet section 6. A service that runs only on the servers of the company is not distributed.

**Options.**

1. Keep the code and follow the LGPL. Put the affected Go files under LGPL-2.0-or-later, each with the copyright lines of its Blink files and a note that it is a changed version. Keep the license text in the repository. Say in the README that a program that contains this module must meet section 6 of the LGPL when it is distributed. The owner cannot give these files an MIT or Apache license.
2. Write the affected functions again without the LGPL files. The sources would be the SVG and CSS specifications and the probe data under `internal/browser/testdata`, which record what Chromium returns. The person or the agent that writes them must not read the Blink files. The probes stay valid as tests, because they record behavior.
3. Move the affected functions into one package with its own license file. This does not change the conditions. It makes the scope clear, and it makes option 2 possible step by step.
4. Ask a lawyer if a Go function that reproduces the behavior of an LGPL function is a derivative work in each case. The files differ: `pathquery.go` and `svg_marker.go` say that they port the files. `cssom.go` only says that it reproduces the behavior.

State on 2026-11-09: The first comment of each Go file of the table names the Blink files, the version `156.0.8078.4` and the license of each file, and it says which part follows which file. `internal/browser/LICENSE-LGPL-2.0.txt` and `internal/raster/LICENSE-LGPL-2.0.txt` have the text of the GNU Library General Public License, version 2. The text is the file `third_party/blink/renderer/core/LICENSE-LGPL-2` of Chromium `156.0.8078.4`. Its terms are the same as the text in the license file of the reference browser. The file `NOTICE` in `internal/browser`, `internal/raster`, `internal/dom` and `internal/css` lists the Go files that follow LGPL files. The file `NOTICE-Blink.txt` in those four directories, in `internal/font` and in `internal/imagesize` has the first comment of each Blink file that has its terms in the file, with the copyright lines. The headers and the notices state where the code comes from. They put no Go file under a license. What remains for the owner: the choice between the four options. If the owner keeps the code (option 1), each affected Go file still needs the statement of its license and the note that it was changed, with the date, as section 2 of the LGPL asks. The README does not name the LGPL yet. In `internal/raster`, which other work was editing, the seven files got the license block, and their first line does not have the form `Ported from` yet (finding F13).

### F2. Code written from the machine code of a closed-source library of Apple

**What.** V8 calls `pow` and `tanh` of the C library of the system for `Math.pow` and `Math.tanh`. On macOS that library is `libsystem_m` of Apple. It is closed source. To get the same bits, the package `internal/js/crmath` had Go functions that followed the machine code of the two functions step by step, and copies of their data tables.

**What was there.** At the start of the audit the Apple-derived code was in four files, apart from the rest of the package, and one script.

| File | Content |
|---|---|
| `internal/js/crmath/apple_libm_pow.go` | `applePow`: "follows the machine code of that function (macOS 26.7.1, arm64) step by step, with the same tables and the same fused operations" |
| `internal/js/crmath/apple_libm_pow_tables.go` | 129 and 128 table entries and the polynomial coefficients of `pow` |
| `internal/js/crmath/apple_libm_tanh.go` | `appleTanh`, written the same way |
| `internal/js/crmath/apple_libm_tanh_tables.go` | the tables of `tanh` |
| `tools/reference/gen-jsmath-apple-libm.py` | Starts a program under `lldb` and reads the tables from the memory of the process at fixed offsets of the library |

The four Go files set two function variables in their `init` functions (`darwinLibmPow`, `darwinLibmTanh`). The package compiled without them.

**What is there now.** Commit `41d617f` ("Use the Math functions of Chromium on Linux, and drop the macOS pow") deleted the five files during the audit. `internal/js/crmath/platform.go` now says: "The module has no implementation of the pow and tanh of macOS." With the platform `darwin`, `Pow` and `Tanh` return the Linux results. `HasDarwinLibm()` reports false.

Three things are left.

- The history. Commits `cb905bd` to `d7c85c3` of the branch (23 commits) contain the Apple-derived code. In the first commits it was in `internal/js/crmath/pow.go` and `tanh.go`.
- Comments. At the time of the audit, `internal/js/crmath/dd.go`, `pow.go` and `tanh.go` still named the files `apple_libm_*.go`.
- Data. `internal/js/crmath/testdata/chromium_math.bin.gz` has the results of the Math functions of Chromium on macOS, with those of `pow` and `tanh`. These are numbers that a browser returned for given arguments.

**Why it matters.** No license allows a copy of the code or the tables of `libsystem_m`. Code written from a disassembly, with copied tables, can be a derivative work of the library. The audit did not read the license terms of macOS.

**Options.**

1. Publish the branch without its history: one new commit, or a history that is written again without the 23 commits. Then no published commit has the code.
2. Publish the history as it is, and accept that the old commits have the code.
3. Fix the three comments that name the deleted files.

State on 2026-11-09: The comments are fixed (option 3). `dd.go`, `pow.go`, `tanh.go`, `platform.go`, `platform_test.go` and `golden_test.go` of `internal/js/crmath` no longer name the deleted files. They say that the module has no port of the pow and the tanh of macOS, and that the platform Darwin returns the results of Linux. A search for `apple_libm` and `gen-jsmath-apple-libm` finds these names only in this document and in THIRD_PARTY_NOTICES.md, where they describe the history. The function variables `darwinLibmPow` and `darwinLibmTanh` are still in the code. No file sets them. What remains for the owner: the decision about the 23 commits of the history (option 1 or 2).

Other parts of the repository model closed-source behavior of macOS without code of Apple. They were made from results and screenshots: `internal/raster/glyph.go`, `glyph_raster.go` and `glyph_zones.go` (CoreText and CoreGraphics, "This file is not a port"), `internal/browser/skatan2.go` (one result of `atan2f`), `internal/font/emoji.go` (the size of Apple Color Emoji). The audit found no sign that they come from a disassembly.

### F3. glibc

**What.** glibc has the GNU Lesser General Public License, version 2.1 or later. The reference browser on Linux uses the `pow`, `tanh`, `sinf`, `cosf`, `tanf` and `atan2f` of glibc 2.39.

**The working tree.** No file of the working tree names glibc as its source.

- `internal/js/crmath/linux_libm_pow.go` ports `math/pow.c` of Arm Optimized Routines v23.01, which glibc also uses. Its header says that it "reads nothing of glibc". The license is MIT OR Apache-2.0 WITH LLVM-exception, and the file has the MIT text.
- `internal/js/crmath/linux_libm_tanh.go` ports `s_tanh.c` and `s_expm1.c` of fdlibm 5.3 and has their notices.
- `internal/browser/linux_trig.go` ports Arm Optimized Routines v20.02 (MIT) and the single precision fdlibm files of NetBSD.

**One expression.** The polynomial of `linuxExpm1` in `linux_libm_tanh.go` is not the Horner form of fdlibm. It is `R1 + h2*R2 + h4*R3`, with `R1 = 1 + hxs*Q1`, `R2 = Q2 + hxs*Q3` and `R3 = Q4 + hxs*Q5`. This is the form of `sysdeps/ieee754/dbl-64/s_expm1.c` of glibc 2.39, which changed the fdlibm code. The Go header says that comparisons with the C library of the reference image selected this form, and the Go code does not name the glibc file. The audit cannot check how the form was found. It is four lines.

**The history.** Commits `316a4c7` to `67a62c9` (4 commits) have `internal/browser/linux_libm.go`. Its header says: "Ported from glibc 2.39 (sysdeps/ieee754/flt-32/s_sinf.c, s_cosf.c, s_sincosf.h, sincosf_poly.h, s_sincosf_data.c, s_tanf.c, k_tanf.c, s_atanf.c, e_atan2f.c)". Commit `d27c046` replaced it with `linux_trig.go`.

**Why it matters.** A port of a glibc file is under the LGPL, with the conditions that F1 describes for a linked program. The glibc files for `sinf` and `cosf` come from Arm, and those for `tanf` and `atan2f` come from fdlibm, so the same code exists under permissive licenses. But a file that says it was ported from glibc is evidence of its source.

**Options.**

1. Decide about the history together with F2.
2. For `linuxExpm1`: keep it with a note of how the form was found, or write the polynomial in the Horner form of fdlibm and accept a different last bit of `tanh` on Linux for some arguments.
3. Keep `tools/reference/check-jsmath-glibc.mjs` and `jsmath-glibc.c`. They call the C library and compare results. They contain no glibc code.

State on 2026-11-09: Nothing changed. Both points remain for the owner: the 4 commits of the history, and the form of `linuxExpm1`.

### F4. The Eclipse Public License 2.0 covers the ELK code

**What and where.**

| Path | What it is | Marked as EPL-2.0 |
|---|---|---|
| `internal/elk/native` (45 packages) | A Go port of the layered algorithm of Eclipse ELK 0.9.1 | Yes: `LICENSE`, `NOTICE`, and the header of each ported file |
| `internal/elk/elkjs/elk-worker.min.js` | The worker of elkjs 0.9.3, unchanged | Yes: `LICENSE.md` next to it |
| `internal/elk/engine.go` | Go code that replaces `lib/elk-api.js` of elkjs, and seven text patches that replace functions of the worker when the script is compiled | No |
| `internal/elk/elk.go`, `graph.go` | The Go API of the engine | No |
| `internal/elk/testdata`, `internal/elk/native/*/testdata` | Records of the results of elkjs | `NOTICE` names the goldens under `native` |

**What the license says.** These points are from the text in `internal/elk/native/LICENSE`.

- Section 3.1: when the program is distributed in any form, its source code must also be available under the EPL, and the distributor must say so and say how to get it. The distributor can use another license for the program as a whole, when that license disclaims warranties and liability for the contributors and does not limit the rights in the source code.
- Section 3.2: the source code is distributed under the EPL, with a copy of the license.
- Section 3.3: notices in the program must not be removed.
- The definition of "Modified Works" covers a file that contains contents of the program. It does not cover works that only link to the program or bind to it by name.

**What a user of this module must do.** A company that links this Go module into a closed-source service that runs only on its own servers does not distribute the program. The EPL sets no condition for that. A company that gives a binary to others must tell the recipients that the binary contains ELK code under EPL-2.0 and where the source code is (this repository, and https://github.com/eclipse/elk). A change to a file under `internal/elk/native`, or to the worker, must be published under EPL-2.0. The EPL does not extend to the code of the company that calls the module.

**What is missing.**

- `internal/elk/engine.go` is not marked. It follows `lib/elk-api.js` (EPL-2.0), and its patches change EPL code at run time.
- The Go headers name the Java file, the version and the license. They do not repeat the copyright line of the Java file, for example `Copyright (c) 2010, 2015 Kiel University and others.` `NOTICE` has one line for all files: `Copyright (c) Kiel University and others.`
- The worker contains compiled code of Guava and GWT (Apache-2.0) and of EMF and Xtext (EPL). The npm package of elkjs gives no notices for them. See F10.

**Options.** Mark `internal/elk/engine.go` as EPL-2.0, or extend `NOTICE` to `internal/elk`. Decide if the per-file copyright lines of the Java files go into the Go headers. Say in the README which packages are under EPL-2.0. A build without ELK is possible in principle, because only diagrams with `layout: elk` need it, but the default layout of mermaid 12.2.0 is elk.

State on 2026-11-09: `internal/elk/engine.go` has the EPL-2.0 statement and the notice of `lib/elk-api.js` in its first comment. The comment also says that the seven patches replace helpers of the Java runtime that GWT emits. `internal/elk/NOTICE` says which files of `internal/elk` are under EPL-2.0, and `internal/elk/LICENSE-EPL-2.0.txt` has the license text. `elk.go` and `graph.go` say that they port no upstream file. `internal/elk/elkjs/NOTICE` names the code of other projects in the worker (finding F10). What remains: the README does not say which packages are under EPL-2.0. The owner decides if the copyright line of each Java file goes into the Go headers.

### F5. DOMPurify has a choice of two licenses

**What.** DOMPurify is under `MPL-2.0 OR Apache-2.0`. The package has both texts (`LICENSE` is the Apache License 2.0, `LICENSE-MPL` is the Mozilla Public License 2.0). `internal/purify` is a port of DOMPurify 3.4.12. No file says which of the two licenses the port uses.

**Why it matters.** The two licenses ask for different things.

- Apache-2.0: keep the notices, give a copy of the license, and mark changed files. See F6.
- MPL-2.0: the files that contain code of DOMPurify, changed or not, stay under the MPL, and their source code must be available to each recipient of a binary. The license covers files, not the program.

**Options.** Choose Apache-2.0 and say so in `internal/purify` and in THIRD_PARTY_NOTICES.md. The notices file already lists Apache-2.0 as a proposal. The other option is MPL-2.0, which would add a third copyleft license with file scope to the repository.

The audit added `(MPL-2.0 OR Apache-2.0)` to the headers of five files of `internal/purify`. That states the license of the upstream. It is not a choice.

State on 2026-11-09: The choice is open. The package comment of `internal/purify/purify.go` and the file `internal/purify/NOTICE` say that DOMPurify is `MPL-2.0 OR Apache-2.0`, that the owner has not chosen, and that the Go files are changed versions. `internal/purify` has both license texts: `LICENSE-Apache-2.0.txt` and `LICENSE-MPL-2.0.txt`. THIRD_PARTY_NOTICES.md lists both licenses and has both texts. What remains for the owner: the choice. After the choice, remove the other license text and write the chosen license in the headers. The generated files `attrs.go` and `tags.go` have no license in their `Ported from` line, because `tools/reference/gen-purify-lists.mjs` writes that line.

### F6. The conditions of the Apache License 2.0 are not met in the ported files

**What.** Section 4 of the Apache License 2.0 has four conditions for a copy or a derivative work: give a copy of the license, put a prominent notice in each changed file that says that the file was changed, keep the copyright, patent, trademark and attribution notices of the source, and pass on the content of a `NOTICE` file when the work has one.

**Where.**

| Go files | Upstream | What is missing |
|---|---|---|
| `internal/diagrams/usecase/chevrotain.go`, `lexer.go`, `internal/langium/lexer.go`, `parser.go` | chevrotain 13.2.0 | No copy of the license in the repository before this audit. No statement that the files are changed versions. chevrotain has no `NOTICE` file |
| `internal/elk/native/gwt/*.go` (6 files), `internal/elk/native/prop/enum.go` | The Java runtime emulation of GWT | The same. The source files of GWT have the notice `Copyright 2008 Google Inc.` and the Apache header. See F7 |
| `internal/js/crmath/trig.go`, `asin.go`, `atan.go`, `exp.go`, `log.go`, `cbrt.go`, `dd.go`, `tables.go` | LLVM libc | The files of LLVM start with "Part of the LLVM Project, under the Apache License v2.0 with LLVM Exceptions" and an SPDX line. The Go files do not have these lines. Only `tables.go` names the revision |
| `internal/font/emoji_segment.go` | emoji-segmenter (`emoji_presentation_scanner.rl`) | No license statement in the header. No version |

The ports of Arm Optimized Routines v23.01 and of skrifa use the MIT side of their dual license, and the files say so.

**Why it matters.** The license is permissive, but its conditions are specific. THIRD_PARTY_NOTICES.md now has the license text and names the projects. The notices inside the Go files are still missing.

**Options.** Add to each of these Go files the license line of its source and one sentence that the file is a Go translation of the source. Other work owns `internal/elk/native`, `internal/js/crmath` and `internal/font`, so this audit did not edit them. It added `(Apache-2.0)` to the four headers in `internal/diagrams/usecase` and `internal/langium`.

State on 2026-11-09: Each Go file of the table has the license of its source in its first comment, the statement that it is a Go translation and a changed version, and the name of a copy of the license in its directory. The copies are `LICENSE-chevrotain.txt` in `internal/diagrams/usecase` and `internal/langium`, `LICENSE-GWT.txt` in `internal/elk/native/gwt`, `LICENSE-LLVM.txt` in `internal/js/crmath` and `LICENSE-emoji-segmenter.txt` in `internal/font`. The Go files that port LLVM libc have the three license lines of the LLVM files and the revision `b8b28272d32b`, which is the revision in the `DEPS` file of Chromium `156.0.8078.4`. `NOTICE` files: chevrotain 13.2.0, LLVM libc and emoji-segmenter have none. GWT has none in the root of its repository (finding F7). The copyright line of emoji-segmenter was found in the upstream file: `Copyright 2019 Google LLC`. `internal/js/crmath/tables.go` is a generated file. Its header got the same lines, and the header text in `tools/reference/gen-jsmath-tables.py` got them too, so that the generator writes the same file. Not changed: `internal/diagrams/usecase/grammar_gen.go`, a generated file with tables that chevrotain computes from the grammar of mermaid.

### F7. The ports of GWT code are in the EPL tree

**What.** `internal/elk/native/NOTICE` says: "The files in this directory tree are made available under the Eclipse Public License 2.0." It also says that the package `gwt` ports parts of GWT, which has the Apache License 2.0. The directory has the text of the EPL only.

**Why it matters.** The Apache License 2.0 lets a derivative work have other license terms, but its own conditions stay (F6). So the seven files need the Apache notice and a copy of the Apache text within reach.

**Options.** Add `LICENSE-GWT.txt` with the Apache License 2.0 to `internal/elk/native/gwt`, and name the license in the seven headers. The copyright lines of GWT were not found locally. The notices file has the lines of two GWT files that were fetched.

State on 2026-11-09: `internal/elk/native/gwt/LICENSE-GWT.txt` has the Apache License 2.0. `internal/elk/native/gwt/NOTICE` has the notices of the GWT source files. The first comment of each of the seven Go files names the license, the Java source files of GWT and their copyright lines, and says that the Go file is a changed version. `internal/elk/native/NOTICE` says that the conditions of the Apache License 2.0 stay in force for the code from GWT. The copyright lines are from 15 Java files on the main branch of https://github.com/gwtproject/gwt (commit `3e636995f379`). `java/util/Random.java` has a second header of the Apache Software Foundation that names a `NOTICE` file. The root of the GWT repository has no file with that name. What remains: the version of GWT that compiled elkjs 0.9.3 is not known.

### F8. Notices that must stay in the file are missing

Some licenses ask that their notice stays in the file, or in each derivative work.

| Go file | Source | What the license asks | State |
|---|---|---|---|
| `internal/js/crmath/hyp.go` | `src/base/ieee754.cc` of V8, which is fdlibm code | "Permission to use, copy, modify, and distribute this software is freely granted, provided that this notice is preserved." | The Sun notice is not in the file. `linux_libm_tanh.go`, `internal/browser/linux_trig.go` and the three fdlibm test files have it |
| `internal/js/sort_dagre.go` | `third_party/v8/builtins/array-sort.tq` of V8 | The file has the copyright notice of the Python Software Foundation. Its license is the Python license (`third_party/v8/builtins/LICENSE` of V8). The license asks that the notice and the license stay in a derivative work, and that the work has a short summary of the changes | The header named V8 only. The audit added the license and the origin to the header. The copyright notice and a summary of changes are not in the file |
| `internal/font/linux.go`, `linux_scaler.go` | `platform/fonts/font_cache.cc`, `font_fallback_list.cc`, `font_metrics.cc`, `simple_font_data.cc`, `alternate_font_family.h`, `shaping/harfbuzz_face.cc`, `skia/font_cache_skia.cc`, `linux/font_cache_linux.cc` of Blink | These files have BSD terms in the file, with the copyright of Apple, Google and others. The terms ask that the copyright notice of the file is kept | The headers point to `LICENSE-Chromium.txt`, which has the line `Copyright 2015 The Chromium Authors`. The lines of the files are not in the repository |
| `internal/dom/xml.go` | `core/editing/serializers/markup_accumulator.cc`, `markup_formatter.cc` of Blink | BSD-2-Clause in the file, copyright Apple and Google | The header names the files. It has no version and no license |
| `internal/font/kern_split.go` | `hb-kern.hh` of HarfBuzz | The copyright notice and two paragraphs must be in all copies | The header has no license. The Go module go-text/typesetting, which the package links, has the notices |
| `internal/browser/transform.go`, `internal/imagesize/imagesize.go`, `internal/raster/svg_filter.go` | `platform/transforms/affine_transform.cc`, `core/svg/graphics/svg_image.cc`, `core/paint/filter_effect_builder.cc` of Blink | BSD-2-Clause in the file, copyright Apple and others | No header |

**Options.** Add the notice to each file. For `sort_dagre.go` add the copyright line of the Python Software Foundation from `array-sort.tq` and one sentence about the changes (a translation from Torque to Go). THIRD_PARTY_NOTICES.md has the texts and the holder list of WebKit.

State on 2026-11-09: Each row of the table is fixed.

- `internal/js/crmath/hyp.go` has the notice of `src/base/ieee754.cc` of V8 15.6.75: the Sun notice and the line of the V8 authors.
- `internal/js/sort_dagre.go` has the copyright notices of `array-sort.tq` at the tags 14.8-lkgr and 15.6-lkgr, and a summary of the changes. `internal/js/LICENSE-PSF.txt` has the license, and `internal/js/LICENSE-V8.txt` has the license of V8.
- `internal/font/linux.go` and `linux_scaler.go` give the license and the copyright lines of each Blink file. Seven of the eight files of the row have BSD-3-Clause in the file, and `linux/font_cache_linux.cc` has BSD-2-Clause in the file. The other four Blink files that the two headers name have the license of Chromium: `font_selector.cc`, `font_fallback_iterator.cc`, `skia/skia_text_metrics.cc` and `web_font_render_style.cc`. `internal/font/NOTICE-Blink.txt` has the first comments of the files, with the conditions and the disclaimers. The headers of these files are the same at the tags `156.0.8078.80` and `156.0.8078.4`.
- `internal/dom/xml.go` names the license and the copyright lines, and it says that the port does not record the version of Chromium.
- `internal/font/kern_split.go` has its header at the top of the file, with the notice of `src/hb-kern.hh`. `internal/font/LICENSE-HarfBuzz.txt` has the file `COPYING` of HarfBuzz. Both are from the revision of HarfBuzz in Chromium `156.0.8078.4` (`f5efbbef3841`). The port does not record its own version of HarfBuzz.
- `internal/browser/transform.go`, `internal/imagesize/imagesize.go` and `internal/raster/svg_filter.go` name their Blink files and the licenses. `NOTICE-Blink.txt` in each of the three directories has the first comments of the files.

### F9. No license file, and the release has no notices

**What.**

- The repository has no `LICENSE` file. Without one, nobody has a license for the original code.
- `.goreleaser.yaml` has `license: "MIT"` for the Homebrew formula.
- The archives of a release contain the binary `mmdg`. GoReleaser adds files with names like `LICENSE*` and `README*` to an archive. It would not add `THIRD_PARTY_NOTICES.md` or the license files under `internal/`.

**Why it matters.** The binary contains all ported code, the elkjs worker, the cytoscape bundle and the Liberation fonts. Several licenses set conditions for a binary: the BSD licenses ask that the notice is in the documentation or other materials of the distribution, the Apache License asks for a copy of the license, the EPL asks for a statement that the source is available and where, the OFL asks that the copyright notice and the license go with the fonts, the FreeType License asks for a credit in the documentation, and the LGPL has the conditions of F1.

**Options.** Choose the license of the repository (see below). Add `THIRD_PARTY_NOTICES.md` to the `files` of the archives in `.goreleaser.yaml`. Change the license field of the formula to an SPDX expression that is true for the binary, for example the license of the repository `AND` the licenses of the ported code. Add one paragraph to the README about the EPL and, if it stays, the LGPL code.

State on 2026-11-09: `.goreleaser.yaml` puts `THIRD_PARTY_NOTICES.md` into each archive. The `files` list of the archive has `LICENSE*`, `README*`, `CHANGELOG*` and `THIRD_PARTY_NOTICES.md`, so the license file goes into the archives when the owner adds it. The line `license: "MIT"` of the Homebrew formula is removed. A comment in its place says that the owner must set the field. The file is valid YAML. GoReleaser was not run. What remains for the owner: the choice of the license, the file `LICENSE`, the license field of the formula, and the paragraph in the README.

### F10. Embedded files contain code of other projects

| File | Content with its own license | Notice next to the file |
|---|---|---|
| `internal/elk/elkjs/elk-worker.min.js` | Compiled classes of Guava (`com.google.common`) and GWT (`com.google.gwt`, the Java runtime), both Apache-2.0, and of EMF and Xtext (EPL). `NOTICE.md` of ELK 0.9.1 lists Guava 30.1.0 and 33.0.0. It also lists libavoid (LGPL-2.1), but the worker has no libavoid code: the string does not occur in it | `LICENSE.md` (EPL-2.0) only |
| `internal/cytoscape/cytoscape.bundle.js` | Functions of lodash and heap inside cytoscape, without a notice in the build. JamaJS (Apache-2.0) inside layout-base 2.0.1 | `LICENSES.txt` has the texts and says what is missing |
| `internal/grammar/*/actions.js.txt` (19 files) | JavaScript of the grammars of mermaid (MIT) | none |
| `internal/font/fontconfig-default.xml.gz` | Rules of the configuration files of fontconfig 2.15.0. The generator leaves out the rule files of other font packages of the image | `LICENSE-fontconfig.txt` |
| `internal/font/liberation/*.ttf.gz` | Liberation fonts 2.1.5, with Reserved Font Names | `LICENSE`. The package also has the text in the variable `License` |

**Options.** Add a short notice file to `internal/elk/elkjs` that names Guava, GWT, EMF and Xtext with their licenses. THIRD_PARTY_NOTICES.md names them. Their copyright lines were not found locally.

The Liberation fonts are compressed with gzip and not changed. The OFL allows that the fonts are "bundled, embedded, redistributed and/or sold with any software", when each copy has the copyright notice and the license. A changed font must not use the names Liberation, Arimo, Tinos or Cousine.

State on 2026-11-09: Three notice files are new. `internal/elk/elkjs/NOTICE` names GWT, Guava, Eclipse EMF and Eclipse Xtext with their licenses and says where each fact was read. EMF and Xtext have the Eclipse Public License 2.0 on the default branches of their repositories. Their notices have no copyright line. `internal/cytoscape/NOTICE` has the license texts of lodash and heap (finding F17). `internal/grammar/NOTICE` has the license of mermaid for the 19 files `actions.js.txt` and for the tables, and the license of jison. The other two rows had a license file before. Not changed: `internal/cytoscape/LICENSES.txt`, because `tools/reference/gen-cytoscape-bundle.mjs` writes it.

### F11. The sources that prove the provenance are not in git

**What.** The headers point to upstream files. The copies of those files are in `.upstream/` and `tools/reference/node_modules/`, and `.gitignore` excludes both. The source of the versions that the mermaid bundle contains (js-yaml 4.3.0, langium 4.4.0, marked 16.3.0, DOMPurify 3.4.12, @iconify/utils 3.0.2) exists only inside source maps in `node_modules`. The sources of Chromium, Skia, V8, LLVM libc, FreeType, skrifa and fontconfig are not on the machine at all.

**Why it matters.** A reader of a clone cannot check a header. THIRD_PARTY_NOTICES.md has texts from six kinds of places, and four of them are not in git.

**Options.** `tools/reference/package-lock.json` is in git, so `npm ci` gives `node_modules` again. Add a script or a paragraph that says how `.upstream` was made (the tag of mermaid, the tag of ELK, the pull request 955). Add the license texts that were fetched for this audit to the repository (section 6 of the notices file lists them).

State on 2026-11-09: License texts that were not in git are in git now: the LGPL-2.0 text of Blink, the licenses of V8, of the CPython list sort, of LLVM libc, of HarfBuzz, of emoji-segmenter, of chevrotain and of GWT, and both licenses of DOMPurify. Section 6 of THIRD_PARTY_NOTICES.md lists the texts that are still not in git: js-yaml, langium, fmin and Arm Optimized Routines v23.01. The Go files that port Arm Optimized Routines v23.01 have the MIT text in their first comment. What remains: no script or paragraph says how `.upstream` was made.

### F12. Ports that follow another version than the mermaid bundle

**State.** Fixed on 2026-11-09. The three ports in the first table now follow the versions in the bundle, and their golden files come from the bundled source.

The reference is `mermaid.min.js` of mermaid 12.2.0. Its source map names the version of each library inside, and it has the source text of each bundled module. `npm install` put newer versions of some libraries into `node_modules`, and three ports followed those.

| Port | Version that the port named | Version in the bundle, which the port follows now | Difference between the two versions | An input that shows the difference |
|---|---|---|---|---|
| `internal/stylis` | stylis 4.4.0 | 4.3.6 | `parse` in `src/Parser.js`. For a parenthesis after a colon, 4.3.6 takes the text up to the closing parenthesis as it is, and marks only the first `&` in it as the parent selector. 4.4.0 has a counter `parens` and reads the text character by character. 4.3.6 makes no declaration for a custom property that has no value, unless a comment is the last thing in the value. `indexof` in `src/Utility.js` has a third parameter in 4.3.6. The other four bundled files are the same | `.a { --x:; }` gives no output in 4.3.6 and `.a{--x:;}` in 4.4.0. `.a:not(/* c */.b) { fill: red }` keeps the comment in 4.3.6. `.x { &:not(&.y, &.z) { fill: red } }` gives `.x:not(.x.y, &.z){fill:red;}` in 4.3.6 and `.x:not(.x.y, .x.z){fill:red;}` in 4.4.0. A user reaches this code with `themeCSS` |
| `internal/d3/format` | d3-format 3.1.2 | 3.1.0 | `formatDecimalParts` in `formatDecimal.js`: 3.1.0 returns a coefficient of zeros and the exponent 0 for zero, 3.1.2 returns null. `formatPrefixAuto.js`: for an infinity, 3.1.0 leaves the module variable `prefixExponent` as it is. `locale.js`: with the type `s`, 3.1.0 adds the SI prefix of that variable also for NaN and for an infinity, and `formatPrefix` adds the prefix after the padding. The other 13 bundled files are the same | `format(".3r")(0)` is `0.00` in 3.1.0 and `0` in 3.1.2. `format("s")(Infinity)` is `Infinityundefined` in a new page and `Infinityk` after `format("s")(1234)` in 3.1.0. It is `Infinity` in 3.1.2. `formatPrefix("10.2", 1e3)(1234)` has 11 characters in 3.1.0 and 10 in 3.1.2. `precisionFixed(0)` is 0 in 3.1.0 and NaN in 3.1.2, so `scaleLinear().domain([0, 0]).tickFormat(5)(0)` is `0` and not `0.000000`. A user reaches `format` with the option `valueFormat` of a treemap |
| `internal/config/merge.go`, `internal/styles/merge.go` | es-toolkit 1.53.1 | 1.45.1 | `mergeWithDeep` in `dist/compat/object/mergeWith.mjs`: 1.45.1 reads a null element of a source array as undefined (`source[i] = source[i] ?? undefined`). 1.53.1 keeps the null. `isEmpty` differs only for a function value. The text of all 26 bundled files differs, because 1.53.1 comes from another build tool. For the other functions that mermaid reaches (`clone`, `memoize`, the predicates), the code has the same behavior. `internal/utils/merge.go` is a third port of the same functions, and it followed 1.45.1 before | `merge({}, {a: [1, 2]}, {a: [null, 3]})` is `{a: [1, 3]}` in 1.45.1 and `{a: [null, 3]}` in 1.53.1. In a diagram: the front matter `config: {flowchart: [1, 2, 3]}` with the directive `%%{init: {"flowchart": [null, 9]}}%%` |

**How the facts were found.** `tools/reference/extract-bundled-source.mjs` reads the source maps of the mermaid build and of the parser package. The command `list` prints each bundled package with its version, the version in `node_modules`, and the number of bundled files that have the same text in `node_modules`. The command `extract` writes the bundled source of a package to `.upstream/bundled`. The differences in behavior came from a comparison of the two source texts and from runs of both versions on the same inputs in Node.js.

**What was done.** The Go code of the three ports follows the bundled source. `tools/reference/use-bundled.mjs` makes Node.js load stylis, d3-format and es-toolkit from the bundled source, also for the imports inside `mermaid.core.mjs` and d3-scale. The generators `gen-stylis.mjs`, `gen-styles.mjs`, `gen-config.mjs`, `gen-d3-format.mjs` and `gen-d3-scale.mjs` use it, and each one stops when it does not get the bundled version. The golden files record the version that made them. They have cases for the inputs where the versions differ, with the result of the newer version next to the expected result, so a test fails when a port follows the newer version. The corpus proof gives the same result before and after the change: no corpus fixture has an input for which the versions differ.

The same comparison for the other libraries:

| Library | State |
|---|---|
| @iconify/utils | `internal/ru/icons_util.go` names 3.0.2, the bundle version, and says that it is not the version in `node_modules`. All 13 bundled files differ from 3.1.7 in `node_modules`. The golden file comes from the bundle in Chromium |
| cytoscape | The embedded bundle is 3.34.0, taken from the mermaid chunk. `cytoscape.bundle.json` records it |
| marked | `internal/marked` names 16.3.0, the bundle version. The bundled files are TypeScript sources that the npm package of 16.4.2 does not have. The golden files come from the mermaid ESM chunk, which has 16.3.0 |
| DOMPurify | `internal/purify` names 3.4.12, the bundle version. 3 of the 5 bundled files differ from 3.4.17 in `node_modules`. The golden files come from the mermaid ESM chunk |
| dayjs | `internal/dayjs` names 1.11.21, the bundle version. The headers point to `dayjs/esm/...`, which exists only in `node_modules` (1.11.23). The nine ported files are the same in 1.11.21 and 1.11.23 (compared with the npm files of 1.11.21). The five files that the bundle has (`dayjs.min.js` and four plugins) have the same text in both versions |
| ts-dedent, uuid | The bundled files of ts-dedent 2.2.0 and uuid 14.0.0 have the same text in the newer versions in `node_modules` |
| Libraries with one version | Every bundled file has the same text in `node_modules`. This includes lodash-es 4.18.1, dagre-d3-es 7.0.14, khroma 2.1.0, roughjs 4.6.6 (patched by mermaid, but the bundled file is that of npm), chevrotain 13.2.0, elkjs 0.9.3 and the d3 modules |
| d3-array and d3-path for d3-sankey | The bundle has d3-array 2.12.1 and d3-path 1.0.9 for d3-sankey. `internal/d3/sankey` uses the ports of d3-array 3.2.4 and d3-path 3.1.0. `max.js`, `min.js` and `sum.js` have the same text in both versions of d3-array. `moveTo` and `bezierCurveTo` of the two versions of d3-path gave the same text for 200,000 random paths |
| langium 4.4.0, js-yaml 4.3.0, chevrotain-allstar 0.5.0 | Not in `node_modules`. The headers name the bundle versions |

No other Go header names a version that is not the version in the bundle.

Smaller points of the same kind:

- Three headers in `internal/font` name Chromium 156.0.8078.80. The reference browser is 156.0.8078.4.
- `internal/raster` names the Skia branch `chrome/m156`, and `internal/font` names the Skia revision `aa789d9b`.
- `internal/js/date_gantt.go` names files of V8 with no version. `internal/js/crmath` names V8 15.6.75, `internal/js/sort_dagre.go` names the tags 14.8-lkgr and 15.6-lkgr, and `tools/reference/elk-fdlibm.mjs` and `rough-fdlibm.mjs` name V8 14.6.
- `internal/d3/sankey` does not say that d3-sankey uses d3-array 2.12.1 in the bundle. `link_horizontal.go` names d3-shape 1.3.7, which is right.
- `internal/font/kern_split.go` and `emoji_segment.go` name HarfBuzz and emoji-segmenter with no version.

**Why it matters.** For licenses, the version decides which copyright line and which license file apply. For parity, a port of another version can return another result. Before the fix, the tests of `internal/stylis` compared with stylis 4.4.0 of `node_modules`, so they did not show a difference to the bundle.

**What remains.**

- `use-bundled.mjs` covers the generators that load mermaid through `lib-mermaid-internals.mjs` or `styles-lib.mjs`. A generator that imports `mermaid.core.mjs` or its chunks without them gets stylis, d3-format and es-toolkit from `node_modules`: `gen-agentflow-yaml.mjs`, `gen-cynefin-boundaries.mjs`, `gen-dagre-index.mjs`, `gen-dagre-graphlib.mjs`, `gen-ru-elk-random.mjs`, `gen-treeview-units.mjs`, `usecase-lib.mjs` and `jison-lib.mjs`. Each generator that runs mermaid in Node.js from `mermaid.core.mjs` also gets @iconify/utils, DOMPurify and marked from `node_modules`. It was not checked if their golden files have a case for which those versions differ.
- `merge` still has three ports: `internal/utils/merge.go`, `internal/config/merge.go` and `internal/styles/merge.go`. `internal/config` cannot import `internal/utils`.

State on 2026-11-09: THIRD_PARTY_NOTICES.md names the bundle versions now: stylis 4.3.6, d3-format 3.1.0 and es-toolkit 1.45.1. The license file of stylis 4.3.6 is the same as that of 4.4.0. The license file of d3-format 3.1.0 has the line `Copyright 2010-2021 Mike Bostock`. The license file of es-toolkit 1.45.1 has two copyright lines and a paragraph about lodash, and that version has no `NOTICE` file. The fix pass of the notices also found these facts about versions. The tag `156.0.8078.80` of Chromium exists, and its `DEPS` file has Skia at `aa789d9bd566`, the revision that the headers of `internal/font` name. The tag `156.0.8078.4`, the reference browser, has Skia at `95ee33d7ec77`. Both tags have the same revisions of LLVM libc, HarfBuzz and emoji-segmenter, and the same first comment in the 17 files of Blink and `ui/gfx` that `internal/font` names. Several Go comments name functions that Chromium `156.0.8078.4` does not have under that name (finding F1). The other two points of "What remains" are open.

### F13. Go files without a provenance header

**Counts.** At the time of the audit the tree had 1027 Go files under `internal/` that are not tests. The numbers after the fix pass are in the paragraph "State" at the end of this finding.

| Group | Files |
|---|---:|
| `Ported from` in the first comment | 748 |
| `Ported from` in a comment after the imports | 49 |
| No `Ported from` | 230 |

Of the 230 files, 46 are generated and name their generator, 22 say that they port no upstream file, and 162 are other files. Most of the 162 are original: package comments, glue, test support and commands. The files below follow upstream code and have no header, or a header with no version.

**Packages that other work owns. This audit did not edit them.**

| Package | Files that follow upstream code and have no `Ported from` header | Files with a header that has no version |
|---|---|---|
| `internal/browser` | `boundschange.go`, `cmath.go`, `cssvar.go`, `engine.go`, `export_raster.go`, `geom.go`, `html.go`, `htmlinline.go`, `image.go`, `linebreak.go`, `norender.go`, `path.go`, `pathquery.go`, `skatan2.go`, `style.go`, `svgroot.go`, `svgviewport.go`, `table.go`, `text.go`, `transform.go` (20 files: every file but `linux_trig.go` and the generated table) | none |
| `internal/raster` | `canvas.go`, `clip.go`, `nonaa.go`, `render.go`, `rpblitter.go`, `svg.go`, `svg_effects.go`, `svg_filter.go`, `svg_gradient.go`, `svg_marker.go`, `svg_text.go`. Their first comment names the Skia or Blink files in other words. `glyph.go`, `glyph_raster.go` and `glyph_zones.go` say that they are not ports | none. The directory has no license file for Skia |
| `internal/font` | `fontconfig.go` and `fontconfig_match.go` say `Follows`. `concurrent.go`, `emoji.go`, `face.go`, `library.go`, `measure.go`, `platform.go`, `xheight.go` and `liberation/liberation.go` have no header | `emoji_segment.go`, `script_run.go` (Chromium with no version), `kern_split.go` (HarfBuzz with no version, and the header is after the imports) |
| `internal/js/crmath` | `dd.go` (the package comment names LLVM libc in its last paragraph) | `asin.go`, `atan.go`, `cbrt.go`, `exp.go`, `log.go`, `trig.go`: LLVM libc with no revision and no license. `hyp.go`, `misc.go`, `pow.go` name V8 15.6.75 with no license |
| `internal/elk/native` | `elknodespacing/elknodespacing.go` (the sentence starts with other words), `lgraph/processor.go`, `lgraph/props.go` | none. `gwt/*.go` and `prop/enum.go` have no license in the header (F7) |
| `internal/diagrams/architecture` | none | all 8 files: `architecture.go`, `db.go`, `grammar.go`, `icons.go`, `parser.go`, `renderer.go`, `svgdraw.go`, `types.go`. They name the file and not mermaid 12.2.0 |

**Other packages.**

| Package | Files | What they follow |
|---|---|---|
| `internal/css` | `color.go`, `cssom.go`, `decl.go`, `range.go` | The behavior of Chromium. `color.go` names `css_parser_fast_paths.cc` |
| `internal/dom` | `parse.go`, `purify_support.go`, `selector.go`, `serialize.go`, `style.go` | The HTML and DOM standards and the behavior of Chromium. `xml.go` has a header that names two Blink files with no version and no license |
| `internal/js` | `num.go`, `obj.go`, `str.go`, `value.go`, `math.go` and six more with no statement. `date_gantt.go` has a header with no version | ECMAScript and V8 |
| `internal/elk` | `elk.go`, `engine.go`, `graph.go` | elkjs 0.9.3 (F4) |
| `internal/cytoscape` | `cytoscape.go`, `runtime.go`, `mode.go`, `doc.go` | The first comments name the files of mermaid and cytoscape in other words |
| `internal/imagesize` | `imagesize.go` | The behavior of Chromium |
| `internal/render` | `async.go`, `context.go`, `registry.go`, `stylesheet.go` | Original. `stylesheet.go` reproduces the CSSOM of the browser |

**Headers after the imports.** 49 files have the `Ported from` comment after the import block: 24 in `internal/ru/shapes/set1`, 11 in `internal/ru`, 6 in `internal/render`, 4 in `internal/ru/graph`, 2 in `internal/ru/clusters`, 1 in `internal/ru/edges` and `internal/font/kern_split.go`. The audit completed these headers in place and did not move them.

**Paths in headers.** A script checked each path in a header against `.upstream` and `node_modules`. After the fixes of the audit, no header names a file that does not exist upstream. The paths that the script could not find are explained: `marked/src/*.ts` and `d3-shape/src/link/index.js` are in the source map and not in `node_modules`, and `elk-worker.js` is a file of elkjs.

State on 2026-11-09: The tree has 1054 Go files under `internal/` that are not tests. Other work added 27 files since the audit.

| Group | Files at the audit | Files after the fix pass |
|---|---:|---:|
| `Ported from` in the first comment | 748 | 839 |
| `Ported from` in a comment after the imports | 49 | 0 |
| No `Ported from` | 230 | 215 |
| Total | 1027 | 1054 |

Of the 215 files, 46 are generated and name their generator, 64 say that they port no upstream file, and 105 are other files. `internal/css/cssom.go` is one of the 64. It says that it names no upstream file that it ports. The counts came from the two commands of PROVENANCE.md and from a script that reads the comment before the `package` clause of each file.

A header of the form `Ported from <project> <version>, in part` says that parts of the Go file follow the upstream functions that the header or the comments name. It does not say that each line is a translation. A file whose comments name no upstream function has the statement `This file ports no upstream file`, and one sentence that says what the file is.

| Package | State |
|---|---|
| `internal/browser` | Done. 16 files have a `Ported from` header that names the files of Chromium and their licenses. `cmath.go`, `cssvar.go`, `export_raster.go`, `linebreak.go`, `skatan2.go` and `table.go` say that they port no upstream file. `linebreak_ascii_gen.go` is generated |
| `internal/font` | Done. `fontconfig.go` and `fontconfig_match.go` say `Ported from fontconfig 2.15.0, in part`. `face.go` names two Blink files. `concurrent.go`, `emoji.go`, `library.go`, `measure.go`, `platform.go`, `xheight.go` and `liberation/liberation.go` say that they port no upstream file. `emoji_segment.go` and `script_run.go` have the license, and they say that the port does not record the version. `kern_split.go` has its header at the top |
| `internal/js/crmath` | Done. The files that port LLVM libc name the revision and the license. `hyp.go`, `misc.go` and `pow.go` name the license of V8. `dd.go` has a header before the package comment |
| `internal/elk/native` | Done. `elknodespacing/elknodespacing.go`, `lgraph/processor.go` and `lgraph/props.go` have a `Ported from` header. `native.go` and `processors.go` say that they port no upstream file. `gwt/*.go` and `prop/enum.go` name the license (F7) |
| `internal/diagrams/architecture` | Done. All 8 headers name mermaid 12.2.0 and the license |
| `internal/css` | Done. `color.go` has a `Ported from` header. `decl.go` and `range.go` say that they port no upstream file. `cssom.go` says that it names no upstream file that it ports (F1) |
| `internal/dom` | Done for the files of the audit. `style.go` has a `Ported from` header for d3-selection. `parse.go`, `purify_support.go`, `selector.go` and `serialize.go` say that they port no upstream file, and they name the Blink files of their comments. `xml.go` has the license. Other work added `parse_simple.go`, `parse_simple_check.go`, `parse_simple_off.go` and `parse_simple_on.go` during the fix pass. They have no header yet |
| `internal/js` | Done. 12 files say that they port no upstream file. `date_gantt.go` names the license and says that the port does not record the version of V8 |
| `internal/elk` | Done. `engine.go` has a `Ported from` header with the EPL-2.0 statement. `elk.go` and `graph.go` say that they port no upstream file |
| `internal/imagesize` | Done. `imagesize.go` says that it ports no upstream file, and it names the Blink file of its comment |
| `internal/render` | Done. `async.go`, `context.go`, `registry.go` and `stylesheet.go` say that they port no upstream file |
| `internal/raster` | Not done. Other work edits the package. The seven files that name Blink files got a license block (F1), and nothing else was changed. Still needed: a first line of the form `Ported from Skia ...` in `canvas.go`, `clip.go` and `nonaa.go`, which name their Skia files in other words. A first line of the form `Ported from Chromium ..., in part` in `render.go`, `svg.go`, `svg_effects.go`, `svg_filter.go`, `svg_gradient.go`, `svg_marker.go` and `svg_text.go`. A statement that the file ports no upstream file in `rpblitter.go` and `glyph.go`. The other Skia headers name the branch `chrome/m156` and no license. `internal/raster/NOTICE` names the license file of Skia |
| `internal/cytoscape` | Not done. Other work edits the package. Still needed: a header in `cytoscape.go`, `runtime.go`, `mode.go`, `doc.go` and `observe.go`, and in 12 files under `native` that have no `Ported from` line: `native.go`, `architecture.go`, `architecture_types.go`, `cosebilkent.go`, `cosebilkent/adapter.go`, `cosebilkent/trace.go`, `cy/math.go`, `fcose/adapter.go`, `fcose/trace.go`, `internal/bail/bail.go` and the two files of `nativetest` |

**Headers after the imports.** All 49 headers are at the top of their file now. A script moved 48 of them. It moved a comment only when the comment starts with `// Ported from`, has a blank line before and after it, and only the import block is between the `package` clause and the comment. Each move changed no other line. `internal/font/kern_split.go` was moved by hand with its new notice.

**The 105 other files.** 32 are the `main.go` files of the `paritycmd` commands. 22 are test support packages and the packages `internal/parity`. 17 are in `internal/cytoscape` and 12 in `internal/raster` (see the table). The other 22 are `internal/config` (`errors.go`, `resolve.go`, `template.go`), `internal/styles` (`color.go`, `jsutil.go`, `theme_default.go`), the four new files of `internal/dom`, `internal/ru` (`jsconv.go`, `layout/dagre/ops.go`, `layout/elk/jsobj.go`, `layout/swimlanes/util.go`, `shapes/set1/helpers.go`), `internal/sanitize/whatwg_url.go`, `internal/marked/regexp.go`, `internal/memo/memo.go`, `internal/d3/internal/jsm/jsm.go`, `internal/diagrams/all.go`, `internal/grammar/doc.go` and `internal/langium/doc.go`. The audit did not list these files as files that follow upstream code, and the fix pass did not edit them.

### F14. Markdown test inputs follow the CommonMark and GFM specifications

**What.** `tools/reference/markdown-inputs.mjs` has a function `specExamples`. Its comment says that it returns "markdown that follows the sections of the CommonMark 0.31 and GFM specifications". Many inputs are the examples of the specifications, for example the tab examples of the first section. The golden files `internal/marked/testdata/lexer.json.gz` and `internal/markdown/testdata/markdown.json.gz` contain these inputs.

**Why it matters.** The CommonMark specification and the GFM specification are under CC-BY-SA-4.0. That license asks for attribution, and for the same license for adapted material. These are test files. They are not in a program.

**Options.** Add an attribution for the specifications to the notices file and to the script. Or write the examples again in own words. The copyright lines of the specifications were not found locally.

State on 2026-11-09: Nothing changed. The decision remains for the owner.

### F15. Screenshots with glyphs of the fonts of macOS

**What.** `internal/raster/testdata/glyph` has PNG files that Chromium made on macOS. They show single glyphs and words of Arial, Courier, Helvetica, Times and Trebuchet MS at many sizes. `internal/raster/testdata/golden` has screenshots of diagrams with the same fonts. The golden files of other packages have text widths that were measured with these fonts.

**Why it matters.** The fonts are not free. Their licenses decide what a user can do with rendered text. Images of documents are a normal use. A sheet of glyphs at many sizes is closer to a copy of the design. Text widths are numbers.

**Options.** Keep the files. Or make the calibration sheets again with the Liberation fonts, which are in the repository under the OFL, and keep the macOS sheets out of git.

State on 2026-11-09: Nothing changed. The decision remains for the owner.

### F16. Unicode and ICU data

`internal/font/emoji_data_gen.go` has the code points of three Unicode emoji properties, read from the ICU 78.3 of Node.js v26.12.0. `internal/js/str_swimlanes_table.go` has collation weights from the ICU collator of Chromium. The Unicode License V3 asks that its copyright and permission notice goes with all copies of the data, or is in the documentation. THIRD_PARTY_NOTICES.md now has the notice. Its copyright line is from the ICU of Chromium 156. The ICU of Node.js was not checked.

State on 2026-11-09: Nothing changed. The ICU of Node.js v26.12.0 is still not checked.

### F17. Small parts with their own notice

| Part | Where | License | State |
|---|---|---|---|
| fmin | `internal/venn/fmin.go`. The build `venn.esm.js` of @upsetjs/venn.js contains fmin with no notice | BSD-3-Clause, `Copyright 2016, Ben Frederickson` (fetched) | In the notices file |
| d3-shape 1.3.7 | `internal/d3/sankey/link_horizontal.go` | BSD-3-Clause. The newer d3 modules are ISC | In the notices file |
| heap, lodash | Inside cytoscape in `cytoscape.bundle.js` | MIT by the `package.json` of heap. lodash is MIT | lodash is in the notices file. The copyright line of heap was not found locally |
| JamaJS | Inside layout-base 2.0.1 in `cytoscape.bundle.js` | Apache-2.0. The text is in the bundle | `internal/cytoscape/LICENSES.txt` says so |
| Markdown by John Gruber | The license file of marked has its BSD-3-Clause notice | BSD-3-Clause | In the notices file |
| goja, package `ftoa` | Linked into every program | A notice of Lucent Technologies, and the BSD license of V8 | In the notices file |

State on 2026-11-09: The row of heap is closed. The file `LICENSE` of the npm package heap 0.2.7 has the MIT text and no copyright line. The file `package.json` of that package names the author. So no copyright line of heap exists that the notices could repeat. `internal/cytoscape/NOTICE` has the license file of heap and the license file of lodash, and THIRD_PARTY_NOTICES.md says the same. cytoscape names `heap ^0.2.7` and `lodash ^4.17.21` as build dependencies. The bundle does not say which versions the build used.

## What each license asks

The table says what the text of each license asks from someone who uses this Go module. "Service" means a closed-source program that runs only on the servers of its owner. "Binary" means a closed-source program that is given to others.

| License | Code in this repository | Service | Binary |
|---|---|---|---|
| MIT, ISC | mermaid and most of its libraries, the d3 modules, cytoscape, Arm Optimized Routines, skrifa | Nothing beyond keeping the notices in the source | The copyright notice and the permission notice go with the copies |
| BSD-3-Clause, BSD-2-Clause | Chromium, Skia, V8, d3-sankey, fmin, the Go libraries | The same | The notice, the conditions and the disclaimer are in the documentation or other materials. The names of the holders are not used for promotion |
| Apache-2.0 | chevrotain, GWT, LLVM libc, emoji-segmenter, DOMPurify if the owner chooses it | Keep the notices. Changed files say that they are changed | A copy of the license goes with the copies. The patent license ends for someone who starts a patent suit about the work |
| EPL-2.0 | `internal/elk/native`, the elkjs worker | Nothing for the own code. Changes to the EPL files that are distributed must be under the EPL | Say that the binary has EPL code and where its source is. Do not limit the rights in that source |
| LGPL-2.0-or-later | The Blink code of F1, if it stays | No condition without distribution | Section 6: give the source of the library part, and give the rest of the program as object code or source so that the user can link it again. Allow reverse engineering to debug changes |
| MPL-2.0 | DOMPurify if the owner chooses it | No condition without distribution | The source of the MPL files is available to each recipient |
| OFL-1.1 | The Liberation fonts | Nothing | The copyright notice and the license go with the fonts. The fonts are not sold by themselves |
| FTL | The automatic hinter, through skrifa and FreeType | Nothing | The documentation says that the software is based in part on the work of the FreeType Team |
| SunPro | fdlibm code | Keep the notice in the file | Nothing more |
| PSF-2.0 | `internal/js/sort_dagre.go` | Keep the notice and the license. Say what was changed | The same |
| Unicode-3.0 | The emoji and collation tables | Nothing | The notice is with the copies or in the documentation |
| HPND-sell-variant, MIT-Modern-Variant | fontconfig, HarfBuzz | Nothing | The notices go with the copies |

## If the owner chooses MIT or Apache-2.0

A license file in the root gives a license for what the owner wrote. It does not change the license of code that was ported from another project. A port keeps the conditions of its source. So the repository will have one license for the original code and the licenses of the sources for the ported files, whichever license the owner chooses.

**What is the same for both choices.**

- The ports of MIT, ISC and BSD code can be given out under the license of the repository, when the notices of the sources stay. THIRD_PARTY_NOTICES.md has them.
- The files under `internal/elk/native`, the elkjs worker and `internal/elk/engine.go` stay under EPL-2.0. The EPL allows that they are in a larger program under another license.
- The Liberation fonts stay under OFL-1.1.
- The files of F1 stay under LGPL-2.0-or-later, if they stay.
- The files that port Apache-2.0 code keep the conditions of F6.
- The notices of fdlibm, Python, FreeType, fontconfig, HarfBuzz and Unicode stay.

**MIT.**

- The original code and the ports of mermaid have the same license as mermaid. That is the simplest reading for users.
- The ports of chevrotain, GWT, LLVM libc and emoji-segmenter cannot be given out under the MIT terms alone, because the Apache License has conditions that MIT does not have: the notice of changes, the copy of the license, and the end of the patent license. These files need a header that says Apache-2.0.
- For DOMPurify, choose Apache-2.0 (F5), and mark `internal/purify` the same way.
- MIT has no patent grant. The owner gives none for the original code.

**Apache-2.0.**

- MIT, ISC and BSD code can be part of an Apache-2.0 work, with the notices kept.
- The ports of Apache-2.0 code have the same license as the original code, so they need no separate marking of the license. They still need the notice of changes and the notices of their sources.
- The owner gives a patent license for the original code, and the license asks contributors for one.
- The Apache License asks for a `NOTICE` file to be passed on when a work has one. The owner can use a `NOTICE` file in the root for the attributions, with THIRD_PARTY_NOTICES.md for the texts.
- EPL-2.0 files and Apache-2.0 files can be in one repository as separate files. The EPL files stay under the EPL.

**For both.** The SPDX expression of a binary of `mmdg` is long: the license of the repository, `AND` MIT, ISC, BSD-2-Clause, BSD-3-Clause, Apache-2.0, EPL-2.0, OFL-1.1, FTL, PSF-2.0, Unicode-3.0, and more. A file `LICENSE` in the root should name the directories that have another license: `internal/elk`, `internal/font/liberation`, and the files of F1, F5 and F6.

## What this audit changed

The audit wrote three documents: this file, `docs/PROVENANCE.md` and `THIRD_PARTY_NOTICES.md`.

The audit changed comment lines in 482 Go files. It changed no code. `gofmt -l` reports none of the files, and `go build ./...` succeeds. Commit `9f63dc0`, made by other work, contains these edits. One later change to the wording of the header of `internal/dom/node.go` is not committed.

| Change | Files |
|---|---:|
| A script added the project, the version and the license to a `Ported from` header that had only a file path. The script made the change only when every path in the header exists in `.upstream/mermaid-12.2.0` or in the npm package that the header names. Example: `// Ported from mermaid 12.2.0, diagrams/pie/pieDb.ts and pieTypes.ts (MIT).` | 455 |
| Headers that the script could not classify, completed by hand: `internal/jison/grammar.go`, `internal/langium/convert.go`, `lexer.go`, `parser.go`, `internal/d3/scale/band.go`, `internal/d3/axis/axis.go`, `internal/utils/helpers.go`, `internal/diagrams/venn/venn.go`, `internal/diagrams/agentflow/jsyaml/jsyaml.go` | 9 |
| The license added to a header that had a version: `internal/diagrams/usecase/chevrotain.go`, `lexer.go` (Apache-2.0), five files of `internal/purify` (MPL-2.0 OR Apache-2.0), `internal/js/sort_dagre.go` (PSF-2.0, with the origin in CPython) | 8 |
| A new header for a file that had none: `internal/diagrams/mindmap/shapes.go` (two shape files of mermaid), `internal/dom/node.go` (the parts of d3-selection 3.0.0) | 2 |
| The first comment of the hand-written lexer actions changed to a `Ported from` header: `internal/grammar/*/lexactions_manual.go` | 8 |

The 455 files are in `internal/diagrams` (without `architecture`), `internal/ru`, `internal/styles`, `internal/d3`, `internal/dagre`, `internal/graphlib`, `internal/rough`, `internal/render`, `internal/utils`, `internal/dayjs`, `internal/jison`, `internal/langium`, `internal/markdown`, `internal/sanitize`, `internal/stylis` and `internal/svgdraw`.

The audit did not edit `internal/font`, `internal/browser`, `internal/raster`, `internal/elk/native`, `internal/js/crmath` and `internal/diagrams/architecture`, because other work was changing them. F13 lists what they need. The audit did not edit generated files, because their generators would write the old header again.

## What the fix pass of 2026-11-09 changed

The fix pass changed comments, added license and notice files, and updated the three documents. It changed no Go code. `gofmt -l` reports none of the changed files, and `go build ./...` succeeds. Other work committed five times during the fix pass (`b473cdc`, `9f8a121`, `d2ffbb3`, `12b26bb` and `c21c3ec`). The fix pass made no commit. Those commits contain most of its edits.

| Change | Files |
|---|---|
| First comments that name the files of Chromium, the version `156.0.8078.4` and the license of each file (F1, F8, F13) | 21 files of `internal/browser`, 7 of `internal/dom`, 4 of `internal/css`, `internal/imagesize/imagesize.go`, 7 of `internal/raster`, `internal/utils/entity_decode.go` |
| Comments that named the deleted Apple files (F2) | `dd.go`, `pow.go`, `tanh.go`, `platform.go`, `platform_test.go`, `golden_test.go` of `internal/js/crmath` |
| License lines and notices of LLVM libc, V8 and fdlibm (F6, F8) | 11 files of `internal/js/crmath`, and the header text in `tools/reference/gen-jsmath-tables.py` |
| The Python notice, the license of V8 and the statement that a file ports no upstream file (F8, F13) | 14 files of `internal/js` |
| Licenses, copyright lines and provenance statements (F6, F8, F13) | 14 files of `internal/font`, `internal/font/liberation/liberation.go` |
| The EPL-2.0 statement and provenance statements (F4, F13) | `engine.go`, `elk.go`, `graph.go` of `internal/elk`, 5 files of `internal/elk/native` |
| The Apache-2.0 notices (F6, F7) | 6 files of `internal/elk/native/gwt`, `internal/elk/native/prop/enum.go`, `chevrotain.go` and `lexer.go` of `internal/diagrams/usecase`, `lexer.go` and `parser.go` of `internal/langium` |
| The license of the file of V8 that the header names | `internal/diagrams/usecase/json.go` |
| The statement that the license choice is open (F5) | `internal/purify/purify.go` |
| mermaid 12.2.0 and the license in the header (F13) | 8 files of `internal/diagrams/architecture` |
| The statement that a file ports no upstream file (F13) | `async.go`, `context.go`, `registry.go`, `stylesheet.go` of `internal/render` |
| The header moved from after the imports to the top of the file (F13) | 42 files of `internal/ru` and its packages, 6 of `internal/render`, `internal/font/kern_split.go` |
| The release configuration (F9) | `.goreleaser.yaml` |

New license and notice files:

| File | Content | Source |
|---|---|---|
| `internal/browser/LICENSE-LGPL-2.0.txt`, `internal/raster/LICENSE-LGPL-2.0.txt` | The GNU Library General Public License, version 2 | `third_party/blink/renderer/core/LICENSE-LGPL-2` of Chromium `156.0.8078.4` |
| `NOTICE` in `internal/browser`, `internal/raster`, `internal/dom`, `internal/css` | The Go files that follow Blink files with the LGPL | Written by the fix pass |
| `NOTICE-Blink.txt` in `internal/browser`, `internal/raster`, `internal/dom`, `internal/css`, `internal/font`, `internal/imagesize` | The first comment of each Blink file that has its license terms in the file | The Blink files at the tag `156.0.8078.4` |
| `internal/elk/LICENSE-EPL-2.0.txt`, `internal/elk/NOTICE` | The Eclipse Public License 2.0, and the files that it covers | `internal/elk/native/LICENSE`. The notice is written by the fix pass |
| `internal/elk/elkjs/NOTICE` | The code of other projects in the elkjs worker | Written by the fix pass from the files that it names |
| `internal/elk/native/gwt/LICENSE-GWT.txt`, `internal/elk/native/gwt/NOTICE` | The Apache License 2.0, and the notices of the GWT files | https://www.apache.org/licenses/LICENSE-2.0.txt, and the Java files of GWT |
| `internal/diagrams/usecase/LICENSE-chevrotain.txt`, `internal/langium/LICENSE-chevrotain.txt` | The Apache License 2.0 | `LICENSE.txt` of the npm package chevrotain 13.2.0 |
| `internal/js/crmath/LICENSE-LLVM.txt` | The Apache License 2.0 with LLVM Exceptions | `LICENSE.TXT` of LLVM libc at revision `b8b28272d32b` |
| `internal/js/LICENSE-V8.txt`, `internal/js/LICENSE-PSF.txt` | The license of V8, and the Python license | `LICENSE` and `third_party/v8/builtins/LICENSE` of V8 15.6.75 |
| `internal/font/LICENSE-emoji-segmenter.txt`, `internal/font/LICENSE-HarfBuzz.txt` | The Apache License 2.0, and the license of HarfBuzz | `LICENSE` of emoji-segmenter at revision `955936be8b39`, and `COPYING` of HarfBuzz at revision `f5efbbef3841` |
| `internal/purify/LICENSE-Apache-2.0.txt`, `internal/purify/LICENSE-MPL-2.0.txt`, `internal/purify/NOTICE` | Both licenses of DOMPurify, and the statement that the choice is open | `LICENSE` and `LICENSE-MPL` of the npm package dompurify 3.4.17 |
| `internal/cytoscape/NOTICE` | The licenses of lodash and heap | `LICENSE` of heap 0.2.7 and of lodash-es 4.18.1 |
| `internal/grammar/NOTICE` | The licenses of mermaid and jison | `LICENSE` of mermaid 12.2.0, and `README.md` of jison 0.4.18 |

Files that the fix pass did not edit, with the reason:

- Generated Go files, because their generators write the header: `internal/browser/linebreak_ascii_gen.go`, `internal/purify/attrs.go` and `tags.go`, `internal/diagrams/usecase/grammar_gen.go`, `internal/font/emoji_data_gen.go`. The one exception is `internal/js/crmath/tables.go`, where the generator got the same header text.
- `internal/cytoscape` and most of `internal/raster`, because other work was editing them. F13 lists what they need.
- `internal/dom/parse_simple.go`, `parse_simple_check.go`, `parse_simple_off.go` and `parse_simple_on.go`, which other work added during the fix pass.
- `internal/cytoscape/LICENSES.txt`, because `tools/reference/gen-cytoscape-bundle.mjs` writes it.
- `README.md` and `go.mod`.

## How the audit was made, and its limits

- A script read the first comment of each Go file and the first `Ported from` paragraph, and checked each path in it against `.upstream` and `tools/reference/node_modules`.
- The versions inside the mermaid bundle came from the `sources` of `mermaid.min.js.map` and of the source maps of the parser package. The source text of those versions came from `sourcesContent`. It was compared with `node_modules` file by file for stylis, d3-format, es-toolkit, dayjs, ts-dedent, roughjs, khroma, dagre-d3-es, lodash-es, d3-sankey, venn.js and sanitize-url.
- The license header of 104 source files of Chromium was read at the tag `156.0.8078.4`. The license files of Skia, V8, LLVM libc, Arm Optimized Routines, HarfBuzz, FreeType, fontconfig, Fontations, emoji-segmenter and the Liberation fonts were fetched and compared with the copies in `internal/font` and `internal/browser`. Each local copy contains the text of the upstream file.
- The linked Go modules came from `go list -deps`.
- The fix pass of 2026-11-09 read the first comment of 346 source files of Chromium at the tag `156.0.8078.4`, and of 17 files at the tag `156.0.8078.80`. It read the `DEPS` file of both tags for the revisions of Skia, V8, LLVM libc, HarfBuzz and emoji-segmenter. For each function of Blink that a Go comment names, it searched the files of the tag for the definition. It read the files of V8 at 15.6.75, of LLVM libc at `b8b28272d32b`, of HarfBuzz at `f5efbbef3841`, of emoji-segmenter at `955936be8b39`, of Skia at `95ee33d7ec77`, of fontconfig at the tag 2.15.0, and the Java files of GWT on its main branch.

Limits:

- The audit did not compare each Go function with its source. It read headers and comments, and compared one function for F1.
- The tree changed during the audit. Other work added packages (`internal/cytoscape/native`, `internal/mmdc`, `internal/fonttest`), deleted the Apple-derived files, and added license files to `internal/font` and `internal/browser`. The counts are those of the working tree at commit `f6f6519`, with the uncommitted files of other work.
- The history was checked only for the files of F2 and F3.
- The audit did not read the license terms of macOS or of its fonts.
- The fix pass did not compare Go functions with their sources. It matched the names in the Go comments with the files of Chromium. A Go file can follow a Blink file that its comments do not name.
