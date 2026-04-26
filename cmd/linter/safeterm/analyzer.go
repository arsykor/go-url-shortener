package safeterm

import (
	"go/ast"
	"go/token"

	"golang.org/x/tools/go/analysis"
)

// Analyzer reports panic usage and os.Exit/log.Fatal calls outside main.
var Analyzer = &analysis.Analyzer{
	Name: "safeterm",
	Doc:  "reports panic usage and os.Exit/log.Fatal calls outside the main function of the main package",
	Run:  run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	isMainPkg := pass.Pkg.Name() == "main"

	for _, file := range pass.Files {
		// Locate the body of the top-level main() function, if present
		mainBodyStart, mainBodyEnd := mainFuncRange(file)

		ast.Inspect(file, func(node ast.Node) bool {
			call, ok := node.(*ast.CallExpr)
			if !ok {
				return true
			}

			switch fn := call.Fun.(type) {
			case *ast.Ident:
				// Built-in panic is forbidden everywhere
				if fn.Name == "panic" {
					pass.Reportf(call.Pos(), "use of built-in panic")
				}
			case *ast.SelectorExpr:
				pkg, ok := fn.X.(*ast.Ident)
				if !ok {
					break
				}
				if isExitCall(pkg.Name, fn.Sel.Name) {
					inMain := isMainPkg &&
						mainBodyStart != token.NoPos &&
						call.Pos() >= mainBodyStart &&
						call.Pos() <= mainBodyEnd
					if !inMain {
						pass.Reportf(call.Pos(), "must not be called outside the main function of main package")
					}
				}
			}
			return true
		})
	}
	return nil, nil
}

func mainFuncRange(file *ast.File) (start, end token.Pos) {
	for _, decl := range file.Decls {
		fd, ok := decl.(*ast.FuncDecl)
		if !ok {
			continue
		}
		if fd.Name.Name == "main" && fd.Recv == nil && fd.Body != nil {
			return fd.Body.Pos(), fd.Body.End()
		}
	}
	return token.NoPos, token.NoPos
}

func isExitCall(pkg, fn string) bool {
	switch pkg {
	case "os":
		return fn == "Exit"
	case "log":
		return fn == "Fatal" || fn == "Fatalf" || fn == "Fatalln"
	}
	return false
}
