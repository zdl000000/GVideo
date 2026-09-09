package httpapi

import (
	"go/parser"
	"go/token"
	"os"
	"strconv"
	"strings"
	"testing"
)

func TestProductionHTTPAPIDoesNotImportPersistencePackages(t *testing.T) {
	fileSet := token.NewFileSet()
	packages, err := parser.ParseDir(fileSet, ".", func(info os.FileInfo) bool {
		return strings.HasSuffix(info.Name(), ".go") && !strings.HasSuffix(info.Name(), "_test.go")
	}, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}

	for _, pkg := range packages {
		for fileName, file := range pkg.Files {
			for _, spec := range file.Imports {
				importPath, err := strconv.Unquote(spec.Path.Value)
				if err != nil {
					t.Fatalf("parse import in %s: %v", fileName, err)
				}
				if forbiddenHTTPAPIImport(importPath) {
					t.Errorf("%s imports forbidden persistence package %q; handlers must depend on service-layer contracts", fileSet.Position(spec.Pos()), importPath)
				}
			}
		}
	}
}

func forbiddenHTTPAPIImport(importPath string) bool {
	if importPath == "database/sql" || importPath == "gvideo/backend/internal/repository" {
		return true
	}
	return strings.HasPrefix(importPath, "gvideo/backend/internal/repository/")
}
