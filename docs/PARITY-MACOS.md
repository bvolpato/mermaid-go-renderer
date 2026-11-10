# Parity with mermaid in Chromium on macOS

This file is generated. Do not edit it. `tools/update-docs.sh` writes it with the command `go run ./tools/paritydoc`.

The corpus `testdata/corpus/mermaid-e2e` has 2145 diagrams from the end-to-end test suite of mermaid. Each diagram was rendered with mermaid 12.2.0 in the reference environment `chromium-156-macos`, and the SHA-256 of the SVG that `mermaid.render()` returned is in `testdata/reference/macos.json`. The table compares the output of this package with those hashes.

- **Byte-exact**: the SVG text has the same bytes as the SVG of mermaid.
- **Same error**: mermaid fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the SVG text differs.
- **Not supported**: the diagram type or a feature that the diagram uses is unavailable.
- **Error**: rendering or comparison fails for another reason.

Result: 2141 of 2145 diagrams pass (99.8%). 2140 are byte-exact and 1 give the same error.

| Diagram type | Diagrams | Byte-exact | Same error | Different | Not supported | Error |
|---|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 404 | 0 | 4 | 0 | 0 |
| `classDiagram` | 285 | 285 | 0 | 0 | 0 | 0 |
| `er` | 206 | 206 | 0 | 0 | 0 | 0 |
| `sequence` | 193 | 193 | 0 | 0 | 0 | 0 |
| `requirement` | 172 | 172 | 0 | 0 | 0 | 0 |
| `stateDiagram` | 144 | 144 | 0 | 0 | 0 | 0 |
| `gitGraph` | 135 | 135 | 0 | 0 | 0 | 0 |
| `block` | 58 | 58 | 0 | 0 | 0 | 0 |
| `gantt` | 52 | 52 | 0 | 0 | 0 | 0 |
| `xychart` | 48 | 48 | 0 | 0 | 0 | 0 |
| `swimlane` | 46 | 46 | 0 | 0 | 0 | 0 |
| `agentflow` | 38 | 38 | 0 | 0 | 0 | 0 |
| `mindmap` | 33 | 33 | 0 | 0 | 0 | 0 |
| `c4` | 32 | 32 | 0 | 0 | 0 | 0 |
| `usecase` | 32 | 32 | 0 | 0 | 0 | 0 |
| `timeline` | 30 | 30 | 0 | 0 | 0 | 0 |
| `treemap` | 26 | 26 | 0 | 0 | 0 | 0 |
| `pie` | 24 | 24 | 0 | 0 | 0 | 0 |
| `venn` | 21 | 21 | 0 | 0 | 0 | 0 |
| `architecture` | 20 | 20 | 0 | 0 | 0 | 0 |
| `journey` | 18 | 18 | 0 | 0 | 0 | 0 |
| `sankey` | 18 | 18 | 0 | 0 | 0 | 0 |
| `treeView` | 15 | 15 | 0 | 0 | 0 | 0 |
| `quadrantChart` | 14 | 14 | 0 | 0 | 0 | 0 |
| `ishikawa` | 13 | 13 | 0 | 0 | 0 | 0 |
| `cynefin` | 12 | 12 | 0 | 0 | 0 | 0 |
| `kanban` | 11 | 11 | 0 | 0 | 0 | 0 |
| `wardley` | 10 | 10 | 0 | 0 | 0 | 0 |
| `eventmodeling` | 6 | 6 | 0 | 0 | 0 | 0 |
| `packet` | 6 | 6 | 0 | 0 | 0 | 0 |
| `radar` | 6 | 6 | 0 | 0 | 0 | 0 |
| `railroad` | 5 | 5 | 0 | 0 | 0 | 0 |
| `info` | 3 | 3 | 0 | 0 | 0 | 0 |
| `railroadEbnf` | 2 | 2 | 0 | 0 | 0 | 0 |
| (mermaid fails) | 1 | 0 | 1 | 0 | 0 | 0 |
| `railroadAbnf` | 1 | 1 | 0 | 0 | 0 | 0 |
| `railroadPeg` | 1 | 1 | 0 | 0 | 0 | 0 |
| **Total** | 2145 | 2140 | 1 | 4 | 0 | 0 |

The test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.
