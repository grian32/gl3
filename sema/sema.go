package sema

import (
	"fmt"
	"gl3/hir"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
)

type Scope struct {
	Symbols map[string]hir.Symbol
	Parent  *Scope
}

type Analyzer struct {
	structs   []hir.Struct
	functions []hir.Function
	globals   []hir.Global

	structPositions  []*util.Position
	functionPostions []*util.Position
	globalPositions  []*util.Position

	functionBodies     []*parser.BlockStatement
	globalInitializers []parser.Expression

	symbols         map[string]hir.Symbol
	currentScope    *Scope
	currentFunction *hir.Function

	diagnostics []Diagnostic
}

func New() *Analyzer {
	return &Analyzer{symbols: make(map[string]hir.Symbol)}
}

func (a *Analyzer) Analyze(program *parser.Program) (*hir.Program, []Diagnostic) /* todo: diag */ {
	if !a.assignIDs(program) {
		goto end
	}

	if !a.populateFieldsFunctions(program) {
		goto end
	}

	if !a.checkSizedDeclarations() {
		goto end
	}

	if !a.checkBodies() {
		goto end
	}

end:
	return &hir.Program{
		Structs:   a.structs,
		Functions: a.functions,
		Globals:   a.globals,
	}, a.diagnostics
}

func (a *Analyzer) checkBodies() bool {
	a.currentScope = &Scope{
		Symbols: a.symbols,
		Parent:  nil,
	}

	for i := range a.globals {
		g := &a.globals[i]
		initExpr, ok := a.checkExpr(a.globalInitializers[g.Id])
		if !ok {
			return false
		}

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

	for i := range a.functions {
		f := &a.functions[i]
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
	case *parser.ExpressionStatement:
		expr, ok := a.checkExpr(stmt.Expression)
		if !ok {
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

		defType, ok := hir.ConvertVarType(stmt.Type, a.symbols)
		if !ok {
			a.appendDiagnostic(stmt.Position(), "invalid type on local declaration for variable `%s`.", stmt.Name)
			return nil, false
		}

		if !a.isSized(defType, make(map[hir.StructID]struct{})) {
			a.appendDiagnostic(stmt.Position(), "unsized and none types are not allowed for local declarations for variable `%s`.", stmt.Name)
			return nil, false
		}

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

	return nil, true
}

func (a *Analyzer) checkExpr(expr parser.Expression) (hir.Expr, bool) {
	switch expr := expr.(type) {
	case *parser.IntegerLiteral:
		return a.convertIntLiteral(expr, false)
	case *parser.PrefixExpression:
		if expr.Operator == "-" {
			switch right := expr.Right.(type) {
			case *parser.IntegerLiteral:
				return a.convertIntLiteral(right, true)
			case *parser.FloatLiteral:
				return &hir.FloatLiteral{
					ExprInfo: hir.Info(hir.Float),
					Value:    -right.Value,
				}, true
			}
		}
	case *parser.BooleanExpression:
		return &hir.BooleanLiteral{
			ExprInfo: hir.Info(hir.Bool),
			Value:    expr.Value,
		}, true
	case *parser.StringLiteral:
		return &hir.StringLiteral{
			ExprInfo: hir.InfoPtr(hir.Char, 1),
			Value:    expr.Value,
		}, true
	case *parser.FloatLiteral:
		return &hir.FloatLiteral{
			ExprInfo: hir.Info(hir.Float),
			Value:    expr.Value,
		}, true
	case *parser.IdentifierExpression:
		return a.findIdentifier(expr)
	}
	return nil, false
}

func (a *Analyzer) findIdentifier(identExpr *parser.IdentifierExpression) (hir.Expr, bool) {
	identifier := identExpr.Value

	for scope := a.currentScope; scope != nil; scope = scope.Parent {
		s, ok := scope.Symbols[identifier]
		if !ok {
			continue
		}

		switch s := s.(type) {
		case hir.LocalID:
			return &hir.LocalRef{
				ExprInfo: hir.ExprInfo{
					ResultType: a.currentFunction.Locals[s].Type,
				},
				ID: s,
			}, true
		case hir.GlobalID:
			return &hir.GlobalRef{
				ExprInfo: hir.ExprInfo{
					ResultType: a.globals[s].Type,
				},
				ID: s,
			}, true
		default:
			a.appendDiagnostic(identExpr.Position(), "cannot use `%s` as a variable", identifier)
			return nil, false
		}
	}

	a.appendDiagnostic(identExpr.Position(), "unknown variable `%s`", identifier)
	return nil, false
}

func (a *Analyzer) convertIntLiteral(expr *parser.IntegerLiteral, negative bool) (*hir.IntegerLiteral, bool) {
	bt := hir.ConvertBaseType(expr.Type.Base)
	if bt == hir.Invalid {
		a.appendDiagnostic(expr.Position(), "invalid type on integer literal")
		return nil, false
	}
	value := expr.UValue
	signed := true
	var bits uint8 = 64
	switch expr.Type.Base {
	case lexer.Int32:
		bits = 32
	case lexer.Int16:
		bits = 16
	case lexer.Int8:
		bits = 8
	case lexer.Uint:
		signed = false
	case lexer.Uint32:
		bits = 32
		signed = false
	case lexer.Uint16:
		bits = 16
		signed = false
	case lexer.Uint8:
		bits = 8
		signed = false
	}

	if !util.IntegerInRange(expr.UValue, negative, bits, signed) {
		a.appendDiagnostic(expr.Position(), "invalid value for literal of type %s", expr.Type.Base)
		return nil, false
	}

	if negative {
		value = -value
	}
	if bits < 64 {
		value &= (uint64(1) << bits) - 1
	}

	return &hir.IntegerLiteral{
		ExprInfo: hir.ExprInfo{
			ResultType: hir.Type{Base: bt},
		},
		Value: value,
	}, true
}

func (a *Analyzer) assignIDs(node parser.Node) bool {
	switch node := node.(type) {
	case *parser.Program:
		valid := true
		for _, s := range node.Statements {
			if !a.assignIDs(s) {
				valid = false
			}
		}

		return valid
	case *parser.StructStatement:
		if a.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.StructID(len(a.structs))
		a.structs = append(a.structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  false,
			Private: false,
		})
		a.structPositions = append(a.structPositions, node.Position())
		a.symbols[node.Name] = id
	case *parser.ExternStructStatement:
		if a.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.StructID(len(a.structs))
		a.structs = append(a.structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  true,
			Private: node.Private,
		})
		a.structPositions = append(a.structPositions, node.Position())
		a.symbols[node.Name] = id
	case *parser.FunctionStatement:
		if a.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(a.functions))
		a.functions = append(a.functions, hir.Function{
			Name:     node.Name.Value,
			Id:       id,
			External: false,
			Private:  node.Private,
		})
		a.functionPostions = append(a.functionPostions, node.Position())
		a.functionBodies = append(a.functionBodies, node.Body)
		a.symbols[node.Name.Value] = id
	case *parser.ExternFunctionStatement:
		if a.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(a.functions))
		a.functions = append(a.functions, hir.Function{
			Name:     node.Name,
			Id:       id,
			External: true,
			Private:  node.Private,
		})
		a.functionPostions = append(a.functionPostions, node.Position())
		a.functionBodies = append(a.functionBodies, nil)
		a.symbols[node.Name] = id
	case *parser.DefStatement:
		if !node.Global {
			break
		}

		if a.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.GlobalID(len(a.globals))
		a.globals = append(a.globals, hir.Global{
			Name:     node.Name.Value,
			Id:       id,
			Constant: node.Constant,
		})
		a.globalPositions = append(a.globalPositions, node.Position())
		a.globalInitializers = append(a.globalInitializers, node.Right)
		a.symbols[node.Name.Value] = id
	}

	return true
}

func (a *Analyzer) populateFieldsFunctions(node parser.Node) bool {
	switch node := node.(type) {
	case *parser.Program:
		valid := true
		for _, s := range node.Statements {
			if !a.populateFieldsFunctions(s) {
				valid = false
			}
		}

		return valid
	case *parser.FunctionStatement:
		funcSymbol, _ := a.symbols[node.Name.Value]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &a.functions[funcId]

		retType, ok := a.functionRetType(node.Type, node.Position(), node.Name.Value, false)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return a.checkFunctionParams(node.Params, funcAst, node.Name.Value)
	case *parser.ExternFunctionStatement:
		funcSymbol, _ := a.symbols[node.Name]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &a.functions[funcId]

		retType, ok := a.functionRetType(node.ReturnType, node.Position(), node.Name, true)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return a.checkFunctionParams(node.Params, funcAst, node.Name)
	case *parser.DefStatement:
		if !node.Global {
			break
		}
		globalSymbol, _ := a.symbols[node.Name.Value]
		globalAst := &a.globals[globalSymbol.(hir.GlobalID)]

		gt, ok := hir.ConvertVarType(node.Type, a.symbols)
		if !ok {
			a.appendDiagnostic(node.Name.Position(), "invalid type for global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.Void && gt.Pointer == 0 {
			a.appendDiagnostic(node.Name.Position(), "none type is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.StructType && gt.Pointer == 0 && a.structs[gt.Struct].Opaque {
			a.appendDiagnostic(node.Name.Position(), "opaque struct by value is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		globalAst.Type = gt
	case *parser.StructStatement:
		structSymbol, _ := a.symbols[node.Name]
		structAst := &a.structs[structSymbol.(hir.StructID)]

		fieldsSucceded := true

		structAst.FieldNames = make(map[string]int)

		for _, field := range node.Fields {
			if _, exists := structAst.FieldNames[field.Name.Value]; exists {
				a.appendDiagnostic(field.Name.Position(), "duplicate field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			ft, ok := hir.ConvertVarType(field.Type, a.symbols)
			if !ok {
				a.appendDiagnostic(field.Name.Position(), "invalid field type for field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			if ft.Base == hir.Void && ft.Pointer == 0 {
				a.appendDiagnostic(field.Name.Position(), "none type not allowed on fields, field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			structAst.FieldNames[field.Name.Value] = len(structAst.Fields)
			structAst.Fields = append(structAst.Fields, hir.TypedName{
				Name: field.Name.Value,
				Type: ft,
			})
		}

		return fieldsSucceded
	}

	return true
}

func (a *Analyzer) checkSizedDeclarations() bool {
	for i := range a.structs {
		s := &a.structs[i]
		s.Unsized = !a.isSized(hir.Type{Base: hir.StructType, Struct: s.Id}, map[hir.StructID]struct{}{})
	}

	for _, fnc := range a.functions {
		for _, p := range fnc.Parameters {
			if !a.isSized(p.Type, map[hir.StructID]struct{}{}) {
				a.appendDiagnostic(a.functionPostions[fnc.Id], "unsized type is not allowed for parameter `%s` in function `%s`; use a pointer", p.Name, fnc.Name)
				return false
			}
		}
		if !fnc.External && fnc.ReturnType.Base != hir.Void && !a.isSized(fnc.ReturnType, map[hir.StructID]struct{}{}) {
			a.appendDiagnostic(a.functionPostions[fnc.Id], "unsized return type is not allowed for function `%s`; use a pointer", fnc.Name)
			return false
		}
	}

	for _, g := range a.globals {
		if !a.isSized(g.Type, map[hir.StructID]struct{}{}) {
			a.appendDiagnostic(a.globalPositions[g.Id], "unsized type is not allowed for global `%s`; use a pointer", g.Name)
			return false
		}
	}

	return true
}

func (a *Analyzer) checkFunctionParams(params []parser.FunctionParameter, funcAst *hir.Function, funcName string) bool {
	funcAst.ParameterNames = make(map[string]int)
	paramsSucceded := true

	for _, param := range params {
		if _, exists := funcAst.ParameterNames[param.Name.Value]; exists {
			a.appendDiagnostic(param.Name.Position(), "duplicate parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}

		pt, ok := hir.ConvertVarType(param.Type, a.symbols)
		if !ok {
			a.appendDiagnostic(param.Name.Position(), "invalid parameter type for parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == hir.Void && pt.Pointer == 0 {
			a.appendDiagnostic(param.Name.Position(), "none type is not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == hir.StructType && pt.Pointer == 0 {
			stct := a.structs[pt.Struct]
			if stct.Opaque {
				a.appendDiagnostic(param.Name.Position(), "opaque struct value types are not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
				paramsSucceded = false
				continue
			}
		}

		funcAst.ParameterNames[param.Name.Value] = len(funcAst.Parameters)
		funcAst.Parameters = append(funcAst.Parameters, hir.TypedName{
			Name: param.Name.Value,
			Type: pt,
		})
	}

	return paramsSucceded
}

func (a *Analyzer) functionRetType(retType lexer.VarType, position *util.Position, funcName string, extern bool) (hir.Type, bool) {
	rt, ok := hir.ConvertVarType(retType, a.symbols)
	if !ok {
		a.appendDiagnostic(position, "invalid return type for function `%s`", funcName)
		return hir.Type{}, false
	}

	if !extern && rt.Base == hir.StructType && rt.Pointer == 0 && a.structs[rt.Struct].Opaque {
		a.appendDiagnostic(position, "opaque struct value return type is not allowed for function `%s`", funcName)
		return hir.Type{}, false
	}

	return rt, true
}

func (a *Analyzer) isSized(t hir.Type, visitedStructs map[hir.StructID]struct{}) bool {
	if t.Pointer > 0 {
		return true
	}

	// primitive
	if t.Base != hir.StructType && t.Base != hir.Void {
		return true
	}

	if t.Base == hir.StructType {
		if a.structs[t.Struct].Opaque {
			return false
		} else {
			if _, ok := visitedStructs[t.Struct]; ok {
				return false
			}
			visitedStructs[t.Struct] = struct{}{}
			defer delete(visitedStructs, t.Struct)

			for _, f := range a.structs[t.Struct].Fields {
				if !a.isSized(f.Type, visitedStructs) {
					return false
				}
			}

			return true
		}
	}

	return false
}

func isConstantInitializer(expr hir.Expr) bool {
	switch expr.(type) {
	case *hir.IntegerLiteral, *hir.FloatLiteral,
		*hir.BooleanLiteral, *hir.StringLiteral:
		return true
	default:
		return false
	}
}
func (a *Analyzer) isSymbolDuplicate(name string, pos *util.Position) bool {
	if _, exists := a.symbols[name]; exists {
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
