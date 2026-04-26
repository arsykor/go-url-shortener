package safeterm_test

import (
	"testing"

	"github.com/arsykor/go-url-shortener/cmd/linter/safeterm"
	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), safeterm.Analyzer, "sample", "mainpkg")
}
