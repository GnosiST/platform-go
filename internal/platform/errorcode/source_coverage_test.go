package errorcode

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// TestProductionErrorConstructionCoverage keeps the public error envelope behind
// the registry-backed writer and prevents production code from inventing codes.
func TestProductionErrorConstructionCoverage(t *testing.T) {
	root := repositoryRoot(t)
	fset := token.NewFileSet()
	var violations []string

	for _, directory := range []string{"internal", "cmd"} {
		directory := filepath.Join(root, directory)
		err := filepath.Walk(directory, func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return err
			}
			if info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			file, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				return fmt.Errorf("parse %s: %w", path, err)
			}
			allowedWriter := filepath.Clean(path) == filepath.Join(root, "internal/platform/httpapi/error_response.go")
			ast.Inspect(file, func(node ast.Node) bool {
				switch value := node.(type) {
				case *ast.CompositeLit:
					if selector, ok := value.Type.(*ast.SelectorExpr); ok && selector.Sel.Name == "ErrorBody" && !allowedWriter {
						violations = append(violations, fmt.Sprintf("%s:%d constructs ErrorBody directly; use writePlatformError", path, fset.Position(value.Pos()).Line))
					}
				case *ast.CallExpr:
					if selector, ok := value.Fun.(*ast.SelectorExpr); ok {
						if selector.Sel.Name == "AbortWithStatusJSON" && !allowedWriter {
							violations = append(violations, fmt.Sprintf("%s:%d writes an error response directly; use writePlatformError", path, fset.Position(value.Pos()).Line))
						}
						if packageName, ok := selector.X.(*ast.Ident); ok && packageName.Name == "errorcode" && selector.Sel.Name == "Code" {
							if len(value.Args) != 1 {
								violations = append(violations, fmt.Sprintf("%s:%d errorcode.Code requires one literal registered code", path, fset.Position(value.Pos()).Line))
								return true
							}
							literal, ok := value.Args[0].(*ast.BasicLit)
							if !ok || literal.Kind != token.STRING {
								violations = append(violations, fmt.Sprintf("%s:%d errorcode.Code must use a literal registered code", path, fset.Position(value.Pos()).Line))
								return true
							}
							code := strings.Trim(literal.Value, "\"")
							if _, registered := Lookup(Code(code)); !registered {
								violations = append(violations, fmt.Sprintf("%s:%d errorcode.Code(%q) is not registered", path, fset.Position(value.Pos()).Line, code))
							}
						}
					}
				}
				return true
			})
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}

	sort.Strings(violations)
	if len(violations) > 0 {
		t.Fatal(strings.Join(violations, "\n"))
	}
}

func repositoryRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	root := workingDirectory
	for {
		if _, err := os.Stat(filepath.Join(root, "go.mod")); err == nil {
			return root
		}
		parent := filepath.Dir(root)
		if parent == root {
			t.Fatalf("could not locate repository root from %s", workingDirectory)
		}
		root = parent
	}
}
