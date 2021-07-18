// analyzer for checking for recursive or nested mutex read locks
package recursivelock

import (
	"errors"
	"fmt"

	"go/ast"
	"go/token"
	"go/types"

	"golang.org/x/tools/go/analysis"
	"golang.org/x/tools/go/analysis/passes/inspect"
	"golang.org/x/tools/go/ast/inspector"
	"golang.org/x/tools/go/types/typeutil"
)

// Doc explaining the tool
const Doc = "Tool to check for recursive or nested mutex read lock calls"

// Analyzer runs static analysis.
var Analyzer = &analysis.Analyzer{
	Name:     "recursivelock",
	Doc:      Doc,
	Requires: []*analysis.Analyzer{inspect.Analyzer},
	Run:      run,
}

var errNestedRLock = errors.New("found recursive read lock call")

func run(pass *analysis.Pass) (interface{}, error) {
	inspect, ok := pass.ResultOf[inspect.Analyzer].(*inspector.Inspector)
	if !ok {
		return nil, errors.New("analyzer is not type *inspector.Inspector")
	}

	// filters out other pieces of source code except for function/method calls, defers, func declartions, and return statements
	nodeFilter := []ast.Node{
		(*ast.CallExpr)(nil),
		(*ast.DeferStmt)(nil),
		(*ast.FuncDecl)(nil),
		(*ast.ReturnStmt)(nil),
	}

	foundRLock := 0
	deferredRLock := false
	endPos := token.NoPos
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		if _, isRet := node.(*ast.ReturnStmt); deferredRLock && (node.Pos() > endPos || isRet) {
			deferredRLock = false
			foundRLock--
		}
		switch stmt := node.(type) {
		case *ast.CallExpr:
			call := getCallInfo(pass.TypesInfo, stmt)
			if call == nil {
				break
			}
			name := call.id
			if name == "RLock" { // if the method found is an RLock method
				if foundRLock > 0 { // if we have already seen an RLock method without seeing a corresponding RUnlock
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v",
							errNestedRLock,
						),
					)
				}
				foundRLock++
			} else if name == "RUnlock" && !deferredRLock {
				foundRLock--
			} else if name != "RUnlock" && foundRLock > 0 {
				if stack := hasNestedRLock(call, inspect, pass, make(map[string]bool)); stack != "" {
					pass.Reportf(
						node.Pos(),
						fmt.Sprintf(
							"%v\n%v",
							errNestedRLock,
							stack, // printing stack may cause error when running go tests
						),
					)
				}
			}
		case *ast.DeferStmt:
			call := getCallInfo(pass.TypesInfo, stmt.Call)
			if call != nil && call.id == "RUnlock" {
				deferredRLock = true
			}
		case *ast.FuncDecl:
			endPos = stmt.End()
		}
	})

	return nil, nil
}

type callInfo struct {
	id  string     // type ID [either the name (if the function is exported) or the package/name if otherwise] of the function/method
	typ types.Type // type of the method receiver (nil if a function)
}

// returns true if callInfo represents a method, false if it is a function
func (c *callInfo) isMethod() bool {
	return c.typ != nil
}

func (c *callInfo) String() string {
	if c.isMethod() {
		return fmt.Sprintf("%v: %v", c.id, c.typ.String())
	}
	return c.id
}

// returns a *callInfo struct with call info (ID and type)
func getCallInfo(tInfo *types.Info, call *ast.CallExpr) (c *callInfo) {
	c = &callInfo{}
	f := typeutil.StaticCallee(tInfo, call)
	if f == nil {
		return nil
	}
	c.id = f.Id()
	s, ok := f.Type().(*types.Signature)
	if r := s.Recv(); ok && r != nil {
		c.typ = r.Type()
	}
	return c
}

// hasNestedRLock takes a call expression represented by callInfo as input and returns a stack trace of the nested or recursive RLock within
// that call expression. If the call expression does not contain a nested or recursive RLock, hasNestedRLock returns an empty string.
// hasNestedRLock finds a nested or recursive RLock by recursively calling itself on any functions called by the function/method represented
// by callInfo.
func hasNestedRLock(call *callInfo, inspect *inspector.Inspector, pass *analysis.Pass, hist map[string]bool) (retStack string) {
	f := pass.Fset
	tInfo := pass.TypesInfo
	node := findCallDeclarationNode(call, inspect, pass.TypesInfo)
	if node == nil {
		return ""
	}
	addition := fmt.Sprintf("\t%q at %v\n", call.id, f.Position(node.Pos()))
	ast.Inspect(node, func(iNode ast.Node) bool {
		switch stmt := iNode.(type) {
		case *ast.CallExpr:
			c := getCallInfo(tInfo, stmt)
			if c == nil {
				return false
			}
			name := c.id

			if name == "RLock" { // if the method found is an RLock method
				retStack += addition + fmt.Sprintf("\t%q at %v\n", name, f.Position(iNode.Pos()))
			} else if name != "RUnlock" { // name should not equal the previousName to prevent infinite recursive loop
				nt := c.String()
				if !hist[nt] { // make sure we are not in an infinite recursive loop
					hist[nt] = true
					stack := hasNestedRLock(c, inspect, pass, hist)
					delete(hist, nt)
					if stack != "" {
						retStack += addition + stack
					}
				}
			}
		}
		return true
	})
	return retStack
}

// findCallDeclarationNode takes a callInfo struct and inspects the AST of the package
// to find a matching method or function declaration. It returns this declaration of type *ast.FuncDecl
func findCallDeclarationNode(c *callInfo, inspect *inspector.Inspector, tInfo *types.Info) *ast.FuncDecl {
	var retNode *ast.FuncDecl = nil
	nodeFilter := []ast.Node{
		(*ast.FuncDecl)(nil),
	}
	inspect.Preorder(nodeFilter, func(node ast.Node) {
		funcDec, ok := node.(*ast.FuncDecl)
		if !ok {
			return
		}
		name := tInfo.ObjectOf(funcDec.Name).Id()
		if c.isMethod() { // are we looking for a method of a specific type?
			if funcDec.Recv == nil { // if this particular call declaration isn't even a method, we can move on
				return
			}
			if t := tInfo.TypeOf(funcDec.Recv.List[0].Type); !types.Identical(t, c.typ) { // if the found type does not equal the target type, we can move on
				return
			}
		} else if funcDec.Recv != nil { // if we are looking for a function, ignore methods
			return
		}
		if c.id == name {
			retNode = funcDec
		}
	})
	return retNode
}
