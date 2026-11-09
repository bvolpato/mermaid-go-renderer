// The go command leaves a directory that has a go.mod file out of the module
// that contains it. This file keeps the test data out of the download of the
// module. The tests read the files from a clone of the repository.
module github.com/bvolpato/mermaid-go-renderer/internal/elk/native/p3order/testdata

go 1.26.0
