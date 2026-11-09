# Performance

This file is generated. Do not edit it. The command `go run ./tools/perfdoc` writes it.

The numbers compare `mmdg` with mermaid 12.2.0 in Chromium 156.0.8078.4, for the 2076 diagrams of the corpus `testdata/corpus/mermaid-e2e` that both sides render. Both ran on the same machine (darwin/arm64, 18 CPUs), one diagram at a time. Times are in milliseconds.

## Summary

| Measurement | mermaid in Chromium | mmdg | Ratio |
|---|---:|---:|---:|
| Warm render, median | 11.20 | 0.89 | 13x |
| Warm render, 90th percentile | 26.90 | 2.55 | 11x |
| Warm render, all diagrams, in seconds | 29.94 | 2.81 | 11x |
| First render, median | 133.0 | 45.3 | 3x |
| First render, 90th percentile | 198.1 | 60.0 | 3x |

What each measurement is:

- **Warm render, mermaid**: the fastest of 7 `mermaid.render()` calls for the diagram, in a page that stays open and renders many diagrams. V8 has compiled the code.
- **Warm render, mmdg**: the fastest of 7 `mermaid.Render` calls for the diagram, in a process that stays running.
- **First render, mermaid**: the first `mermaid.render()` call in a new page. The browser is running and the page has loaded mermaid. The start of the browser and the load of the script are not in the time.
- **First render, mmdg**: a new process that reads the file, renders it one time and exits. The start and the end of the process are in the time.

The first-render comparison favors mermaid, because it leaves out the start of the browser. A tool that starts a browser for each run, such as mermaid-cli, pays that start each time.

## By diagram type

| Diagram type | Diagrams | mermaid warm | mmdg warm | mmdg warm, 90th percentile | Warm ratio | mermaid first | mmdg first |
|---|---:|---:|---:|---:|---:|---:|---:|
| `flowchart-v2` | 404 | 14.60 | 1.32 | 3.84 | 11x | 165.0 | 49.2 |
| `classDiagram` | 227 | 15.40 | 1.21 | 2.99 | 13x | 165.9 | 52.0 |
| `er` | 206 | 13.60 | 1.39 | 2.86 | 10x | 165.0 | 53.5 |
| `sequence` | 193 | 6.60 | 0.67 | 1.39 | 10x | 33.2 | 44.6 |
| `requirement` | 170 | 12.20 | 1.05 | 2.38 | 12x | 178.0 | 48.6 |
| `stateDiagram` | 144 | 21.20 | 1.18 | 2.68 | 18x | 171.9 | 41.7 |
| `gitGraph` | 135 | 8.40 | 0.23 | 0.41 | 36x | 64.2 | 41.1 |
| `block` | 58 | 10.90 | 0.41 | 0.93 | 27x | 34.9 | 36.0 |
| `gantt` | 52 | 3.50 | 0.21 | 0.54 | 17x | 23.0 | 37.9 |
| `xychart` | 48 | 4.30 | 0.29 | 0.42 | 15x | 18.6 | 35.4 |
| `swimlane` | 46 | 25.90 | 2.77 | 21.07 | 9x | 75.5 | 43.1 |
| `agentflow` | 38 | 18.50 | 1.41 | 3.19 | 13x | 193.2 | 41.1 |
| `mindmap` | 33 | 11.70 | 0.51 | 5.59 | 23x | 73.7 | 51.4 |
| `c4` | 32 | 5.40 | 0.78 | 1.83 | 7x | 29.1 | 43.2 |
| `timeline` | 30 | 6.30 | 1.26 | 1.58 | 5x | 23.1 | 40.0 |
| `usecase` | 28 | 19.70 | 1.52 | 2.13 | 13x | 219.2 | 50.0 |
| `treemap` | 26 | 4.10 | 0.43 | 0.80 | 10x | 55.2 | 48.3 |
| `pie` | 24 | 2.00 | 0.09 | 0.21 | 21x | 50.7 | 38.9 |
| `venn` | 21 | 3.00 | 0.50 | 0.75 | 6x | 18.7 | 45.3 |
| `architecture` | 20 | 15.90 | 1.07 | 3.06 | 15x | 113.8 | 41.9 |
| `journey` | 18 | 4.10 | 0.30 | 0.59 | 13x | 22.1 | 55.0 |
| `sankey` | 18 | 2.00 | 0.12 | 0.18 | 17x | 16.8 | 37.9 |
| `treeView` | 15 | 2.40 | 0.14 | 0.25 | 18x | 55.0 | 40.4 |
| `quadrantChart` | 14 | 3.20 | 0.12 | 0.25 | 26x | 15.6 | 33.3 |
| `ishikawa` | 13 | 2.80 | 0.14 | 0.49 | 21x | 17.6 | 48.4 |
| `cynefin` | 12 | 3.30 | 0.17 | 0.22 | 20x | 46.0 | 38.1 |
| `kanban` | 11 | 4.60 | 0.25 | 0.32 | 19x | 22.8 | 44.7 |
| `wardley` | 10 | 3.10 | 0.21 | 0.31 | 15x | 50.2 | 41.1 |
| `eventmodeling` | 6 | 2.80 | 0.26 | 0.35 | 11x | 55.9 | 43.3 |
| `packet` | 6 | 2.00 | 0.06 | 0.14 | 36x | 45.3 | 32.3 |
| `radar` | 6 | 2.00 | 0.11 | 0.20 | 18x | 45.8 | 40.7 |
| `railroad` | 5 | 4.90 | 0.25 | 0.45 | 20x | 55.4 | 42.2 |
| `info` | 3 | 1.60 | 0.02 | 0.02 | 69x | 31.9 | 31.1 |
| `railroadEbnf` | 2 | 5.00 | 0.25 | 0.25 | 20x | 59.6 | 41.2 |
| `railroadAbnf` | 1 | 4.40 | 0.20 | 0.20 | 22x | 53.6 | 41.8 |
| `railroadPeg` | 1 | 6.60 | 0.31 | 0.31 | 21x | 61.0 | 40.5 |
| **All** | 2076 | 11.20 | 0.89 | 2.55 | 13x | 133.0 | 45.3 |

The columns have the median of the diagrams of the type, except where the heading says otherwise.

## Command-line tools

The table compares `mmdg` with `mmdc` (mermaid-cli 12.0.0). Each run is one new process that reads a diagram file and writes one output file. The time is the wall time of the process in milliseconds: the median of 7 runs, after 3 runs of each tool that do not count. `mmdc` starts a browser in each run.

| Input | Output | mmdc | mmdg | Ratio |
|---|---|---:|---:|---:|
| Flowchart, 8 nodes | SVG | 762 | 28.5 | 27x |
| Flowchart, 8 nodes | PNG | 850 | 50.3 | 17x |
| Sequence diagram, 4 actors | SVG | 583 | 25.9 | 22x |
| Sequence diagram, 4 actors | PNG | 526 | 46.7 | 11x |
| Class diagram, 5 classes | SVG | 730 | 35.3 | 21x |
| Class diagram, 5 classes | PNG | 695 | 45.3 | 15x |
