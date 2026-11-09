#!/bin/sh
# Writes again the documents, the tables and the images that come from the
# data of the repository. Nothing in them is written by hand.
#
#   tools/update-docs.sh
#
# The data, and what updates it:
#
#   testdata/reference/*.json     the hashes of the reference files (tools/manifest)
#   testdata/reference/*.txt      which diagrams pass (go test -run TestCorpusParity -update-parity .)
#   docs/performance.json         the performance (node tools/reference/bench.mjs)
#   website/diagrams              the diagrams of the gallery
#
# What this script writes from the data:
#
#   docs/PARITY*.md               the tables by diagram type (tools/paritydoc)
#   README.md, docs/CLI.md        the blocks between "generated" comments (tools/sitegen)
#   docs/assets                   the charts and the gallery of the README (tools/sitegen)
#
# The website has the same data. The pages workflow builds it after a push to
# main (go run ./tools/sitegen -out dist), so it needs no step here.
#
# A document for the macOS reference needs the system fonts of the reference
# machine. On another machine the script leaves that document as it is.
set -eu
cd "$(dirname "$0")/.."
ref=testdata/reference

go run ./tools/paritydoc -manifest $ref/linux.json -title "Parity with mermaid" -o docs/PARITY.md
go run ./tools/paritydoc -manifest $ref/linux-docs.json -title "Parity with mermaid on the documentation examples" -o docs/PARITY-DOCS.md
go run ./tools/paritydoc -manifest $ref/linux-large.json -title "Parity with mermaid on large diagrams" -o docs/PARITY-LARGE.md
go run ./tools/paritydoc -manifest $ref/macos.json -title "Parity with mermaid in Chromium on macOS" -o docs/PARITY-MACOS.md
go run ./tools/paritydoc -manifest $ref/linux-mmdc.json,$ref/linux-mmdc-docs.json,$ref/linux-mmdc-large.json -title "Parity with mermaid-cli" -o docs/PARITY-MMDC.md
go run ./tools/paritydoc -manifest $ref/macos-mmdc.json,$ref/macos-mmdc-docs.json,$ref/macos-mmdc-large.json -title "Parity with mermaid-cli in Chromium on macOS" -o docs/PARITY-MMDC-MACOS.md

go run ./tools/sitegen -readme docs/assets
