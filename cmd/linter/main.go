// Command linter is a static analyzer that reports:
//   - any use of the built-in panic;
//   - calls to os.Exit or log.Fatal/Fatalf/Fatalln outside the main
//     function of the main package.
//
// Usage:
//
//	linter ./...
package main

import (
	"golang.org/x/tools/go/analysis/singlechecker"

	"github.com/arsykor/go-url-shortener/cmd/linter/safeterm"
)

func main() {
	singlechecker.Main(safeterm.Analyzer)
}
