# Parity with mermaid on large diagrams

This file is generated. Do not edit it. The command `go run ./tools/paritydoc` writes it.

The corpus `testdata/corpus/mermaid-large` has 70 diagrams from a seeded generator (tools/reference/gen-large-corpus.mjs): flowcharts, class, state and ER diagrams with 15 to 150 nodes. Each diagram was rendered with mermaid 12.2.0 in the reference environment `chromium-156-linux-liberation`, and the SHA-256 of the SVG that `mermaid.render()` returned is in `testdata/reference/linux-large.json`. The table compares the output of this package with those hashes.

- **Byte-exact**: the SVG text has the same bytes as the SVG of mermaid.
- **Same error**: mermaid fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the SVG text differs.
- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.

Result: 70 of 70 diagrams pass (100.0%). 70 are byte-exact and 0 give the same error.

| Diagram type | Diagrams | Byte-exact | Same error | Different | Not supported |
|---|---:|---:|---:|---:|---:|
| `flowchart-v2` | 38 | 38 | 0 | 0 | 0 |
| `classDiagram` | 12 | 12 | 0 | 0 | 0 |
| `er` | 10 | 10 | 0 | 0 | 0 |
| `stateDiagram` | 10 | 10 | 0 | 0 | 0 |
| **Total** | 70 | 70 | 0 | 0 | 0 |

The test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.
