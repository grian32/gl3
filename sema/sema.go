package sema

import (
	"fmt"
	"gl3/hir"
	"gl3/parser"
	"gl3/util"
	"strings"
)

type Scope struct {
	Symbols map[string]hir.Symbol
	Parent  *Scope
}

type Analyzer struct {
	Structs   []hir.Struct
	Functions []hir.Function
	Globals   []hir.Global

	structPositions   []*util.Position
	functionPositions []*util.Position
	globalPositions   []*util.Position

	functionBodies     []*parser.BlockStatement
	globalInitializers []parser.Expression
	importedStructs    int
	importedFunctions  int
	importedGlobals    int

	Symbols         map[string]hir.Symbol
	currentScope    *Scope
	currentFunction *hir.Function
	loopDepth       int

	containsVariadic bool

	diagnostics []Diagnostic
}

func New() *Analyzer {
	return &Analyzer{Symbols: make(map[string]hir.Symbol)}
}

func (a *Analyzer) Analyze(program *parser.Program) (*hir.Program, []Diagnostic) /* todo: diag */ {
	a.importedStructs = len(a.Structs)
	a.importedFunctions = len(a.Functions)
	a.importedGlobals = len(a.Globals)
	a.structPositions = make([]*util.Position, len(a.Structs))
	a.functionPositions = make([]*util.Position, a.importedFunctions)
	a.functionBodies = make([]*parser.BlockStatement, a.importedFunctions)
	a.globalPositions = make([]*util.Position, a.importedGlobals)
	a.globalInitializers = make([]parser.Expression, a.importedGlobals)
	a.containsVariadic = false

	// pass 1.0 assigns ids to Functions/Structs/Globals
	if !a.assignIDs(program) {
		goto end
	}

	// pass 1.1 populates fields and function parameters
	if !a.populateFieldsFunctions(program) {
		goto end
	}

	// pass 1.2 checks that exported declarations do not expose private types
	if !a.checkPublicTypes() {
		goto end
	}

	// pass 1.3 checks for parameters and global types being sized
	if !a.checkSizedDeclarations() {
		goto end
	}

	// pass 2.0 checks all expressions and produces the final hir
	if !a.checkBodies() {
		goto end
	}

	// pass 2.1 checks if all Functions return
	if !a.checkReturns() {
		goto end
	}

end:
	return &hir.Program{
		Structs:          a.Structs,
		Functions:        a.Functions,
		Globals:          a.Globals,
		ContainsVariadic: a.containsVariadic,
	}, a.diagnostics
}

func (a *Analyzer) typeName(t hir.Type) string {
	if t.Base != hir.StructType {
		return t.String()
	}
	return a.Structs[t.Struct].Name + strings.Repeat("*", int(t.Pointer))
}

func (a *Analyzer) resolveSymbol(name string) (hir.Symbol, bool) {
	for scope := a.currentScope; scope != nil; scope = scope.Parent {
		if symbol, ok := scope.Symbols[name]; ok {
			return symbol, true
		}
	}
	return nil, false
}

func (a *Analyzer) isSymbolDuplicate(name string, pos *util.Position) bool {
	if _, exists := a.Symbols[name]; exists {
		a.appendDiagnostic(pos, "duplicate symbol `%s`", name)
		return true
	}

	return false
}

func (a *Analyzer) isScopeSymbolDuplicate(name string, pos *util.Position) bool {
	if _, exists := a.currentScope.Symbols[name]; exists {
		a.appendDiagnostic(pos, "duplicate symbol `%s`", name)
		return true
	}

	return false
}

func (a *Analyzer) appendDiagnostic(pos *util.Position, msg string, v ...any) {
	a.diagnostics = append(a.diagnostics, Diagnostic{
		Message:  fmt.Sprintf(msg, v...),
		Position: pos,
	})
}
