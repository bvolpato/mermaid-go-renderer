# Parity with mermaid-cli in Chromium on macOS

This file is generated. Do not edit it. The command `go run ./tools/paritydoc` writes it.

## chromium-156-macos, corpus `mermaid-e2e`

The corpus `testdata/corpus/mermaid-e2e` has 2145 diagrams from the end-to-end test suite of mermaid. Each diagram was rendered with mermaid-cli 12.0.0 and mermaid 12.2.0 in the reference environment `chromium-156-macos`. `testdata/reference/macos-mmdc.json` has the SHA-256 of the two `.svg` files that mermaid-cli writes for it: the file without the fonts of its page (`--no-font-embed`) and the file with them (the default). The table compares the output of this package in the page mode `mermaid-cli` with those hashes.

- **Byte-exact**: both files have the same bytes as the files of mermaid-cli.
- **Without fonts only**: the file without fonts has the same bytes. The file with fonts differs.
- **Same error**: mermaid-cli fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the file without fonts differs.
- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.

Result: 2131 of 2145 diagrams pass (99.3%). 2067 are byte-exact and 64 give the same error.

| Diagram type | Diagrams | Byte-exact | Without fonts only | Same error | Different | Not supported |
|---|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 408 | 404 | 0 | 0 | 0 | 4 |
| `classDiagram` | 227 | 224 | 0 | 0 | 3 | 0 |
| `er` | 206 | 203 | 0 | 0 | 3 | 0 |
| `sequence` | 193 | 193 | 0 | 0 | 0 | 0 |
| `requirement` | 170 | 167 | 0 | 0 | 3 | 0 |
| `stateDiagram` | 144 | 144 | 0 | 0 | 0 | 0 |
| `gitGraph` | 135 | 135 | 0 | 0 | 0 | 0 |
| (mermaid fails) | 64 | 0 | 0 | 64 | 0 | 0 |
| `block` | 58 | 58 | 0 | 0 | 0 | 0 |
| `gantt` | 52 | 52 | 0 | 0 | 0 | 0 |
| `xychart` | 48 | 48 | 0 | 0 | 0 | 0 |
| `swimlane` | 46 | 46 | 0 | 0 | 0 | 0 |
| `agentflow` | 38 | 38 | 0 | 0 | 0 | 0 |
| `mindmap` | 33 | 33 | 0 | 0 | 0 | 0 |
| `c4` | 32 | 32 | 0 | 0 | 0 | 0 |
| `timeline` | 30 | 30 | 0 | 0 | 0 | 0 |
| `usecase` | 28 | 28 | 0 | 0 | 0 | 0 |
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
| `railroadAbnf` | 1 | 1 | 0 | 0 | 0 | 0 |
| `railroadPeg` | 1 | 1 | 0 | 0 | 0 | 0 |
| `zenuml` | 1 | 0 | 0 | 0 | 0 | 1 |
| **Total** | 2145 | 2067 | 0 | 64 | 9 | 5 |

## chromium-156-macos, corpus `mermaid-docs`

The corpus `testdata/corpus/mermaid-docs` has 596 diagrams from the examples of the mermaid documentation, a corpus that no part of this package was developed against. Each diagram was rendered with mermaid-cli 12.0.0 and mermaid 12.2.0 in the reference environment `chromium-156-macos`. `testdata/reference/macos-mmdc-docs.json` has the SHA-256 of the two `.svg` files that mermaid-cli writes for it: the file without the fonts of its page (`--no-font-embed`) and the file with them (the default). The table compares the output of this package in the page mode `mermaid-cli` with those hashes.

- **Byte-exact**: both files have the same bytes as the files of mermaid-cli.
- **Without fonts only**: the file without fonts has the same bytes. The file with fonts differs.
- **Same error**: mermaid-cli fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the file without fonts differs.
- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.

Result: 578 of 596 diagrams pass (97.0%). 576 are byte-exact and 2 give the same error.

| Diagram type | Diagrams | Byte-exact | Without fonts only | Same error | Different | Not supported |
|---|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 139 | 138 | 0 | 0 | 0 | 1 |
| `sequence` | 48 | 47 | 0 | 0 | 0 | 1 |
| `classDiagram` | 40 | 40 | 0 | 0 | 0 | 0 |
| `gitGraph` | 38 | 38 | 0 | 0 | 0 | 0 |
| `block` | 30 | 30 | 0 | 0 | 0 | 0 |
| `er` | 28 | 28 | 0 | 0 | 0 | 0 |
| `usecase` | 25 | 25 | 0 | 0 | 0 | 0 |
| `stateDiagram` | 23 | 23 | 0 | 0 | 0 | 0 |
| `wardley` | 19 | 19 | 0 | 0 | 0 | 0 |
| `mindmap` | 18 | 18 | 0 | 0 | 0 | 0 |
| `zenuml` | 16 | 0 | 0 | 0 | 0 | 16 |
| `gantt` | 14 | 14 | 0 | 0 | 0 | 0 |
| `timeline` | 14 | 14 | 0 | 0 | 0 | 0 |
| `swimlane` | 13 | 13 | 0 | 0 | 0 | 0 |
| `treeView` | 13 | 13 | 0 | 0 | 0 | 0 |
| `agentflow` | 12 | 12 | 0 | 0 | 0 | 0 |
| `eventmodeling` | 12 | 12 | 0 | 0 | 0 | 0 |
| `requirement` | 10 | 10 | 0 | 0 | 0 | 0 |
| `railroadEbnf` | 9 | 9 | 0 | 0 | 0 | 0 |
| `xychart` | 9 | 9 | 0 | 0 | 0 | 0 |
| `sankey` | 8 | 8 | 0 | 0 | 0 | 0 |
| `treemap` | 8 | 8 | 0 | 0 | 0 | 0 |
| `venn` | 8 | 8 | 0 | 0 | 0 | 0 |
| `architecture` | 6 | 6 | 0 | 0 | 0 | 0 |
| `c4` | 5 | 5 | 0 | 0 | 0 | 0 |
| `pie` | 5 | 5 | 0 | 0 | 0 | 0 |
| `packet` | 4 | 4 | 0 | 0 | 0 | 0 |
| `cynefin` | 3 | 3 | 0 | 0 | 0 | 0 |
| `kanban` | 3 | 3 | 0 | 0 | 0 | 0 |
| `quadrantChart` | 3 | 3 | 0 | 0 | 0 | 0 |
| `radar` | 3 | 3 | 0 | 0 | 0 | 0 |
| (mermaid fails) | 2 | 0 | 0 | 2 | 0 | 0 |
| `journey` | 2 | 2 | 0 | 0 | 0 | 0 |
| `railroadAbnf` | 2 | 2 | 0 | 0 | 0 | 0 |
| `railroadPeg` | 2 | 2 | 0 | 0 | 0 | 0 |
| `ishikawa` | 1 | 1 | 0 | 0 | 0 | 0 |
| `railroad` | 1 | 1 | 0 | 0 | 0 | 0 |
| **Total** | 596 | 576 | 0 | 2 | 0 | 18 |

## chromium-156-macos, corpus `mermaid-large`

The corpus `testdata/corpus/mermaid-large` has 70 diagrams from a seeded generator (tools/reference/gen-large-corpus.mjs): flowcharts, class, state and ER diagrams with 15 to 150 nodes. Each diagram was rendered with mermaid-cli 12.0.0 and mermaid 12.2.0 in the reference environment `chromium-156-macos`. `testdata/reference/macos-mmdc-large.json` has the SHA-256 of the two `.svg` files that mermaid-cli writes for it: the file without the fonts of its page (`--no-font-embed`) and the file with them (the default). The table compares the output of this package in the page mode `mermaid-cli` with those hashes.

- **Byte-exact**: both files have the same bytes as the files of mermaid-cli.
- **Without fonts only**: the file without fonts has the same bytes. The file with fonts differs.
- **Same error**: mermaid-cli fails on the diagram, and this package fails too.
- **Different**: the diagram renders, and the file without fonts differs.
- **Not supported**: the diagram type, or a feature that the diagram uses, is not ported.

Result: 70 of 70 diagrams pass (100.0%). 70 are byte-exact and 0 give the same error.

| Diagram type | Diagrams | Byte-exact | Without fonts only | Same error | Different | Not supported |
|---|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 38 | 38 | 0 | 0 | 0 | 0 |
| `classDiagram` | 12 | 12 | 0 | 0 | 0 | 0 |
| `er` | 10 | 10 | 0 | 0 | 0 | 0 |
| `stateDiagram` | 10 | 10 | 0 | 0 | 0 | 0 |
| **Total** | 70 | 70 | 0 | 0 | 0 | 0 |


The test `TestCorpusParity` makes the same comparison. It fails when a diagram that passed stops passing.
