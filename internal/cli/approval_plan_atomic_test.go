package cli

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"runtime"
	"testing"
)

// This platform-independent check protects the Windows-only failure window:
// an old persisted plan must never be removed before its replacement commits.
func TestApprovalPlanNeverDeletesPreviousFileBeforeReplacement(t *testing.T) {
	_, source, _, _ := runtime.Caller(0)
	file, err := parser.ParseFile(token.NewFileSet(), filepath.Join(filepath.Dir(source), "approval_matrix_import.go"), nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Name.Name != "saveApprovalMatrixImportPlan" {
			continue
		}
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			selector, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || selector.Sel.Name != "Remove" {
				return true
			}
			for _, arg := range call.Args {
				if id, ok := arg.(*ast.Ident); ok && id.Name == "path" {
					t.Error("old plan is deleted before replacement")
				}
			}
			return true
		})
		return
	}
	t.Fatal("plan save function missing")
}
