package sema

import (
	"gl3/hir"
	"gl3/parser"
)

func (a *Analyzer) checkReturns() bool {
	valid := true
	for i, f := range a.Functions {
		if f.External || (f.ReturnType.Base == hir.Void && f.ReturnType.Pointer == 0) {
			continue
		}
		if blockFallsThrough(f.Body) {
			a.appendDiagnostic(a.functionPositions[i], "function `%s` can reach the end without returning `%s`", f.Name, f.ReturnType)
			valid = false
		}
	}
	return valid
}

func blockFallsThrough(block hir.Block) bool {
	for _, stmt := range block.Statements {
		if !statementFallsThrough(stmt) {
			return false
		}
	}
	return true
}

func statementFallsThrough(stmt hir.Stmt) bool {
	switch stmt := stmt.(type) {
	case *hir.Return:
		return false
	case *hir.If:
		if stmt.Else == nil {
			return true
		}
		return blockFallsThrough(stmt.Then) || blockFallsThrough(*stmt.Else)
	default:
		return true
	}
}

func (a *Analyzer) checkBodies() bool {
	a.currentScope = &Scope{
		Symbols: a.Symbols,
		Parent:  nil,
	}

	for i := a.importedGlobals; i < len(a.Globals); i++ {
		g := &a.Globals[i]
		initExpr, ok := a.checkExpr(a.globalInitializers[g.Id])
		if !ok {
			return false
		}

		initExpr = coerce(initExpr, g.Type)
		if g.Type != initExpr.Type() {
			a.appendDiagnostic(a.globalPositions[g.Id], "invalid initializer of type `%s` for global `%s` with declared type `%s`", initExpr.Type(), g.Name, g.Type)
			return false
		}

		if !isConstantInitializer(initExpr) {
			a.appendDiagnostic(a.globalPositions[g.Id], "global `%s` must have constant initializer", g.Name)
			return false
		}

		g.Initializer = initExpr
	}

	for i := range a.Functions {
		f := &a.Functions[i]
		if body := a.functionBodies[f.Id]; body != nil {
			parentScope := a.currentScope
			a.currentScope = &Scope{
				Symbols: make(map[string]hir.Symbol, len(f.Parameters)),
				Parent:  parentScope,
			}
			a.currentFunction = f
			f.Locals = make([]hir.Local, len(f.Parameters))
			for j, param := range f.Parameters {
				f.Locals[j] = hir.Local{Name: param.Name, Type: param.Type}
				a.currentScope.Symbols[param.Name] = hir.LocalID(j)
			}

			checkedBody, ok := a.checkBlock(body)
			a.currentScope = parentScope
			a.currentFunction = nil
			if !ok {
				return false
			}
			f.Body = checkedBody
		}
	}

	return true
}

func (a *Analyzer) checkBlock(block *parser.BlockStatement) (hir.Block, bool) {
	checkBlock := hir.Block{}

	for _, s := range block.Statements {
		stmt, ok := a.checkStmt(s)
		if !ok {
			return hir.Block{}, false
		}
		checkBlock.Statements = append(checkBlock.Statements, stmt)
	}

	return checkBlock, true
}

func (a *Analyzer) checkStmt(stmt parser.Statement) (hir.Stmt, bool) {
	switch stmt := stmt.(type) {
	case *parser.IfStatement:
		return a.checkIf(stmt)
	case *parser.WhileStatement:
		return a.checkWhile(stmt)
	case *parser.BreakStatement:
		if a.loopDepth == 0 {
			a.appendDiagnostic(stmt.Position(), "break is only allowed inside a loop")
			return nil, false
		}
		return &hir.Break{}, true
	case *parser.ContinueStatement:
		if a.loopDepth == 0 {
			a.appendDiagnostic(stmt.Position(), "continue is only allowed inside a loop")
			return nil, false
		}
		return &hir.Continue{}, true
	case *parser.ExpressionStatement:
		expr, ok := a.checkExpr(stmt.Expression)
		if !ok {
			return nil, false
		}
		if expr.Type().Base == hir.Null {
			a.appendDiagnostic(stmt.Position(), "nullptr needs a pointer type from its context")
			return nil, false
		}
		return &hir.ExpressionStatement{Expr: expr}, true
	case *parser.ReturnStatement:
		retType := a.currentFunction.ReturnType

		if stmt.Expr == nil {
			if retType.Base != hir.Void || retType.Pointer != 0 {
				a.appendDiagnostic(stmt.Position(), "return statement with no expression can only be used in functions with return type `none`")
				return nil, false
			} else {
				return &hir.Return{Value: nil}, true
			}
		}

		expr, ok := a.checkExpr(stmt.Expr)
		if !ok {
			return nil, false
		}

		expr = coerce(expr, retType)
		if expr.Type() != retType {
			a.appendDiagnostic(stmt.Position(), "value with type `%s` is not allowed to be returned for function `%s` of type `%s`", expr.Type(), a.currentFunction.Name, retType)
			return nil, false
		}

		return &hir.Return{Value: expr}, true
	case *parser.DefStatement:
		if stmt.Global || stmt.Constant {
			a.appendDiagnostic(stmt.Position(), "global declarations are not allowed inside function bodies.")
			return nil, false
		}

		if a.isScopeSymbolDuplicate(stmt.Name.Value, stmt.Name.Position()) {
			return nil, false
		}

		expr, ok := a.checkExpr(stmt.Right)
		if !ok {
			return nil, false
		}

		defType, ok := hir.ConvertVarType(stmt.Type, a.Symbols)
		if !ok {
			a.appendDiagnostic(stmt.Position(), "invalid type on local declaration for variable `%s`.", stmt.Name)
			return nil, false
		}

		if !a.isSized(defType) {
			a.appendDiagnostic(stmt.Position(), "unsized and none types are not allowed for local declarations for variable `%s`.", stmt.Name)
			return nil, false
		}

		expr = coerce(expr, defType)
		if defType != expr.Type() {
			a.appendDiagnostic(stmt.Position(), "initializer for local variable `%s` of type `%s` does not match declared type `%s`.", stmt.Name, expr.Type(), defType)
			return nil, false
		}

		id := hir.LocalID(len(a.currentFunction.Locals))
		a.currentFunction.Locals = append(a.currentFunction.Locals, hir.Local{
			Name: stmt.Name.Value,
			Type: defType,
		})
		a.currentScope.Symbols[stmt.Name.Value] = id
		return &hir.LocalDeclaration{
			ID:          id,
			Initializer: expr,
		}, true
	}

	a.appendDiagnostic(stmt.Position(), "unsupported statement `%s`", stmt.TokenLiteral())
	return nil, false
}

func (a *Analyzer) checkIf(stmt *parser.IfStatement) (*hir.If, bool) {
	condition, ok := a.checkExpr(stmt.Condition)
	if !ok {
		return nil, false
	}
	if condition.Type().Base != hir.Bool || condition.Type().Pointer != 0 {
		a.appendDiagnostic(stmt.Condition.Position(), "if condition must be bool, got `%s`", condition.Type())
		return nil, false
	}
	thenBlock, ok := a.checkScopedBlock(stmt.Success)
	if !ok {
		return nil, false
	}
	var elseBlock *hir.Block
	if stmt.Fail != nil {
		block, ok := a.checkScopedBlock(stmt.Fail)
		if !ok {
			return nil, false
		}
		elseBlock = &block
	}
	return &hir.If{Condition: condition, Then: thenBlock, Else: elseBlock}, true
}

func (a *Analyzer) checkWhile(stmt *parser.WhileStatement) (*hir.While, bool) {
	condition, ok := a.checkExpr(stmt.Condition)
	if !ok {
		return nil, false
	}
	if condition.Type().Base != hir.Bool || condition.Type().Pointer != 0 {
		a.appendDiagnostic(stmt.Condition.Position(), "while condition must be bool, got `%s`", condition.Type())
		return nil, false
	}
	a.loopDepth++
	body, ok := a.checkScopedBlock(stmt.Body)
	a.loopDepth--
	if !ok {
		return nil, false
	}
	return &hir.While{Condition: condition, Body: body}, true
}

func (a *Analyzer) checkScopedBlock(block *parser.BlockStatement) (hir.Block, bool) {
	parent := a.currentScope
	a.currentScope = &Scope{Symbols: make(map[string]hir.Symbol), Parent: parent}
	checked, ok := a.checkBlock(block)
	a.currentScope = parent
	return checked, ok
}
