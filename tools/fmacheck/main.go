// Command fmacheck finds floating-point expressions that the Go compiler
// fuses into one multiply-add instruction (FMA).
//
// On arm64, ppc64le, s390x and riscv64 the compiler turns x*y + z into a
// fused operation with a single rounding. JavaScript and Chromium never do
// that, and amd64 does not do it by default. A fused expression can give a
// result that differs in the last bit, so the output of the renderer would
// depend on the CPU. The Go specification says that an explicit conversion
// prevents fusion: write float64(x*y) + z.
//
// fmacheck asks the compiler itself: it builds the packages for arm64 with
// -gcflags=-d=fmahash=vn, which makes the compiler print every place where
// it would fuse. That covers the cases that are hard to see in the source:
// a product that is added in a later statement, a product that reaches an
// addition through an inlined call, and x/2 + y (the compiler turns /2
// into *0.5). The report lists each place once. Test files are included.
//
// Usage:
//
//	go run ./tools/fmacheck ./internal/dagre ./internal/d3/...
//
// With -fix, fmacheck first wraps the products that it can find in the
// syntax tree in a conversion to their own type. Run gofmt afterwards, and
// fix what the compiler still reports by hand.
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
)

var fix bool

type edit struct {
	start, end int
	typ        string
}

func main() {
	args := os.Args[1:]
	includeTests := false
	var dirs []string
	for _, a := range args {
		if a == "-tests" {
			includeTests = true
			continue
		}
		if a == "-fix" {
			fix = true
			continue
		}
		if strings.HasSuffix(a, "/...") {
			root := strings.TrimSuffix(a, "/...")
			_ = filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
				if err == nil && d.IsDir() && d.Name() != "testdata" && !strings.HasPrefix(d.Name(), ".") {
					dirs = append(dirs, path)
				}
				return nil
			})
			continue
		}
		dirs = append(dirs, a)
	}
	if fix {
		for _, dir := range dirs {
			checkDir(dir, includeTests)
		}
	}
	sites := compilerReport(os.Args[1:])
	for _, site := range sites {
		fmt.Println(site + ": the compiler fuses this expression; wrap the product: float64(x*y)")
	}
	if len(sites) > 0 {
		fmt.Fprintf(os.Stderr, "fmacheck: %d fused floating-point expressions\n", len(sites))
		os.Exit(1)
	}
}

// compilerReport builds the packages for arm64 with the fusion report on
// and returns the reported places inside this module, sorted.
func compilerReport(args []string) []string {
	var pkgs []string
	for _, a := range args {
		if !strings.HasPrefix(a, "-") {
			pkgs = append(pkgs, a)
		}
	}
	seen := map[string]bool{}
	run := func(goArgs ...string) {
		cmd := exec.Command("go", append(goArgs, pkgs...)...)
		cmd.Env = append(os.Environ(), "GOARCH=arm64", "GOOS="+runtime.GOOS)
		out, _ := cmd.CombinedOutput()
		for _, line := range strings.Split(string(out), "\n") {
			_, rest, ok := strings.Cut(line, "fmahash triggered ")
			if !ok {
				continue
			}
			// The first position is the fused expression. Positions after
			// a semicolon are the call sites of inlined functions.
			site := strings.TrimSpace(rest)
			if i := strings.Index(site, " ["); i >= 0 {
				site = site[:i]
			}
			first, _, _ := strings.Cut(site, ";")
			if strings.HasPrefix(first, "/") || strings.Contains(first, "/pkg/mod/") || strings.HasPrefix(first, "$GOROOT") {
				continue // the standard library or a dependency
			}
			seen[site] = true
		}
	}
	run("build", "-gcflags=-d=fmahash=vn")
	run("vet", "-vettool=/usr/bin/true")
	run("test", "-gcflags=-d=fmahash=vn", "-run", "^$", "-count=1")
	sites := make([]string, 0, len(seen))
	for s := range seen {
		sites = append(sites, s)
	}
	sort.Strings(sites)
	return sites
}

func checkDir(dir string, includeTests bool) int {
	fset := token.NewFileSet()
	pkgs, err := parser.ParseDir(fset, dir, func(fi os.FileInfo) bool {
		return includeTests || !strings.HasSuffix(fi.Name(), "_test.go")
	}, 0)
	if err != nil || len(pkgs) == 0 {
		return 0
	}
	count := 0
	for _, pkg := range pkgs {
		var files []*ast.File
		for _, f := range pkg.Files {
			files = append(files, f)
		}
		sort.Slice(files, func(i, j int) bool {
			return fset.Position(files[i].Pos()).Filename < fset.Position(files[j].Pos()).Filename
		})
		info := &types.Info{Types: map[ast.Expr]types.TypeAndValue{}}
		conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(error) {}}
		_, _ = conf.Check(pkg.Name, fset, files, info)
		for _, f := range files {
			n, edits := checkFile(fset, f, info)
			count += n
			if fix && len(edits) > 0 {
				name := fset.Position(f.Pos()).Filename
				src, err := os.ReadFile(name)
				if err != nil {
					continue
				}
				// Apply the edits from the end of the file to the start. An
				// edit that overlaps one that is already applied is left for
				// the next run, because its offsets are stale.
				sort.Slice(edits, func(i, j int) bool { return edits[i].start > edits[j].start })
				limit := len(src) + 1
				for _, e := range edits {
					if e.end > limit {
						continue
					}
					src = []byte(string(src[:e.start]) + e.typ + "(" + string(src[e.start:e.end]) + ")" + string(src[e.end:]))
					limit = e.start
					count--
				}
				_ = os.WriteFile(name, src, 0o644)
			}
		}
	}
	return count
}

func isFloat(info *types.Info, e ast.Expr) bool {
	tv, ok := info.Types[e]
	if !ok || tv.Type == nil {
		return false
	}
	if tv.Value != nil {
		return false // a constant expression is folded at compile time
	}
	b, ok := tv.Type.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsFloat != 0
}

func isFloatConversion(info *types.Info, call *ast.CallExpr) bool {
	if len(call.Args) != 1 {
		return false
	}
	tv, ok := info.Types[call.Fun]
	if !ok || !tv.IsType() {
		return false
	}
	b, ok := tv.Type.Underlying().(*types.Basic)
	return ok && b.Info()&(types.IsFloat|types.IsInteger) != 0
}

func checkFile(fset *token.FileSet, file *ast.File, info *types.Info) (int, []edit) {
	count := 0
	var edits []edit
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return true
		}
		stack = append(stack, n)
		mul, ok := n.(*ast.BinaryExpr)
		if !ok || mul.Op != token.MUL || !isFloat(info, mul) {
			return true
		}
		// Find the consumer of the product, looking through parentheses and
		// unary minus (negation does not stop fusion).
		i := len(stack) - 2
		for i >= 0 {
			switch p := stack[i].(type) {
			case *ast.ParenExpr:
				i--
				continue
			case *ast.UnaryExpr:
				if p.Op == token.SUB || p.Op == token.ADD {
					i--
					continue
				}
			}
			break
		}
		if i < 0 {
			return true
		}
		switch p := stack[i].(type) {
		case *ast.CallExpr:
			if isFloatConversion(info, p) {
				return true
			}
			// An argument of math.Sqrt, math.Abs and similar cannot fuse.
			if sel, ok := p.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && id.Name == "math" {
					return true
				}
			}
		case *ast.BinaryExpr:
			switch p.Op {
			case token.MUL, token.QUO, token.LSS, token.GTR, token.LEQ, token.GEQ, token.EQL, token.NEQ:
				return true
			}
		}
		pos := fset.Position(mul.Pos())
		count++
		if fix {
			if b, ok := info.Types[mul].Type.(*types.Basic); ok && (b.Kind() == types.Float32 || b.Kind() == types.Float64) {
				edits = append(edits, edit{pos.Offset, fset.Position(mul.End()).Offset, b.Name()})
				return true
			}
		}
		fmt.Printf("%s:%d:%d: floating-point product can be fused; write float64(x*y)\n", pos.Filename, pos.Line, pos.Column)
		return true
	})
	return count, edits
}
