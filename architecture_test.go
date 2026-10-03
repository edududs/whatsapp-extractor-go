package extractor_test

import (
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestCoreDependencyDirection(t *testing.T) {
	for _, layer := range []string{"domain", "application"} {
		err := filepath.WalkDir(layer, func(path string, d fs.DirEntry, err error) error {
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
			for _, imp := range f.Imports {
				name, err := strconv.Unquote(imp.Path.Value)
				if err != nil {
					return err
				}
				if layer == "application" && name == "github.com/edududs/whatsapp-extractor-go/domain" {
					continue
				}
				first, _, _ := strings.Cut(name, "/")
				if strings.Contains(first, ".") {
					t.Errorf("%s imports forbidden dependency %s", path, name)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
