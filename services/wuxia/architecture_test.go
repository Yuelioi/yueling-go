package wuxia

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestRuleModulesStayIndependentOfIO(t *testing.T) {
	err := filepath.WalkDir("internal", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, im := range f.Imports {
			name, _ := strconv.Unquote(im.Path.Value)
			if strings.Contains(name, "gorm") || strings.Contains(name, "/bot") || strings.Contains(name, "/plugins/") || name == "database/sql" || strings.HasPrefix(name, "net") || name == "os" {
				t.Errorf("pure module %s imports IO %s", path, name)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestCardsNeverReadLiveContent(t *testing.T) {
	paths, err := filepath.Glob("cards/*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if s, ok := n.(*ast.SelectorExpr); ok && (s.Sel.Name == "DefaultCatalog" || s.Sel.Name == "ParseCatalog") {
				t.Errorf("card queries live content: %s", path)
			}
			return true
		})
	}
}
