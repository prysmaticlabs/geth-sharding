package slicedirect

import (
	"errors"
	"go/ast"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
)

// Doc explaining the tool.
const Doc = "Tool to detect unnecessary slice-to-slice conversion by applying [:] to a slice expression."

// Analyzer runs static analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "slicedirect",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

func run(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	nodeFilter := []ast.Node{
		(*ast.SliceExpr)(nil),
	}

	typeInfo := types.Info{Types: make(map[ast.Expr]types.TypeAndValue)}
	_, err := (&types.Config{}).Check(pass.Pkg.Path(), pass.Fset, pass.Files, &typeInfo)
	if err != nil {
		return nil, err
	}

	inspect.Preorder(nodeFilter, func(node ast.Node) {
		sliceExpr, ok := node.(*ast.SliceExpr)
		if !ok {
			return
		}

		if sliceExpr.Low != nil || sliceExpr.High != nil {
			return
		}

		switch x := typeInfo.Types[sliceExpr.X].Type.(type) {
		case *types.Array, *types.Slice:
			pass.Reportf(sliceExpr.Pos(), "Expression is already a slice.")
		case *types.Basic:
			if x.String() == "string" {
				pass.Reportf(sliceExpr.Pos(), "Expression is already a slice.")
			}
		}
	})

	return nil, nil
}
