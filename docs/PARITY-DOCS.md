# Parity with mermaid on the documentation examples

This file is generated. Do not edit it. The command `go run ./tools/paritydoc` writes it.

The corpus `testdata/corpus/mermaid-docs` has 596 diagrams from the examples of the mermaid documentation, a corpus that no part of this package was developed against. Each diagram was rendered with mermaid 12.2.0 in the reference environment `chromium-156-linux-liberation`, and the SHA-256 of the SVG that `mermaid.render()` returned is in `testdata/reference/linux-docs.json`. The table compares the output of this package with those hashes.

- **Byte-exact**: the SVG text has the same bytes as the SVG of mermaid.
- **Same error**: mermaid fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the SVG text differs.
- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.

Result: 594 of 596 diagrams pass (99.7%). 576 are byte-exact and 18 give the same error.

| Diagram type | Diagrams | Byte-exact | Same error | Different | Not supported |
|---|---:|---:|---:|---:|---:|
| `flowchart-v2` | 139 | 138 | 0 | 0 | 1 |
| `sequence` | 48 | 47 | 0 | 0 | 1 |
| `classDiagram` | 40 | 40 | 0 | 0 | 0 |
| `gitGraph` | 38 | 38 | 0 | 0 | 0 |
| `block` | 30 | 30 | 0 | 0 | 0 |
| `er` | 28 | 28 | 0 | 0 | 0 |
| `usecase` | 25 | 25 | 0 | 0 | 0 |
| `stateDiagram` | 23 | 23 | 0 | 0 | 0 |
| `wardley` | 19 | 19 | 0 | 0 | 0 |
| (mermaid fails) | 18 | 0 | 18 | 0 | 0 |
| `mindmap` | 18 | 18 | 0 | 0 | 0 |
| `gantt` | 14 | 14 | 0 | 0 | 0 |
| `timeline` | 14 | 14 | 0 | 0 | 0 |
| `swimlane` | 13 | 13 | 0 | 0 | 0 |
| `treeView` | 13 | 13 | 0 | 0 | 0 |
| `agentflow` | 12 | 12 | 0 | 0 | 0 |
| `eventmodeling` | 12 | 12 | 0 | 0 | 0 |
| `requirement` | 10 | 10 | 0 | 0 | 0 |
| `railroadEbnf` | 9 | 9 | 0 | 0 | 0 |
| `xychart` | 9 | 9 | 0 | 0 | 0 |
| `sankey` | 8 | 8 | 0 | 0 | 0 |
| `treemap` | 8 | 8 | 0 | 0 | 0 |
| `venn` | 8 | 8 | 0 | 0 | 0 |
| `architecture` | 6 | 6 | 0 | 0 | 0 |
| `c4` | 5 | 5 | 0 | 0 | 0 |
| `pie` | 5 | 5 | 0 | 0 | 0 |
| `packet` | 4 | 4 | 0 | 0 | 0 |
| `cynefin` | 3 | 3 | 0 | 0 | 0 |
| `kanban` | 3 | 3 | 0 | 0 | 0 |
| `quadrantChart` | 3 | 3 | 0 | 0 | 0 |
| `radar` | 3 | 3 | 0 | 0 | 0 |
| `journey` | 2 | 2 | 0 | 0 | 0 |
| `railroadAbnf` | 2 | 2 | 0 | 0 | 0 |
| `railroadPeg` | 2 | 2 | 0 | 0 | 0 |
| `ishikawa` | 1 | 1 | 0 | 0 | 0 |
| `railroad` | 1 | 1 | 0 | 0 | 0 |
| **Total** | 596 | 576 | 18 | 0 | 2 |

The test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.
