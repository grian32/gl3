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

	functionBodies []*parser.BlockStatement

	symbols         map[string]hir.Symbol
	currentScope    *Scope
	currentFunction *hir.Function

	diagnostics []Diagnostic
}

func New() *Analyzer {
	return &Analyzer{symbols: make(map[string]hir.Symbol)}
}

func (c *Analyzer) Analyze(program *parser.Program) (*hir.Program, []Diagnostic) /* todo: diag */ {
	if !c.assignIDs(program) {
		goto end
	}

	if !c.populateFieldsFunctions(program) {
		goto end
	}

	if !c.checkSizedDeclarations() {
		goto end
	}

	if !c.checkBodies(program) {
		goto end
	}

end:
	return &hir.Program{
		Structs:   c.structs,
		Functions: c.functions,
		Globals:   c.globals,
	}, c.diagnostics
}

func (c *Analyzer) checkBodies(node parser.Node) bool {
	c.currentScope = &Scope{
		Symbols: c.symbols,
		Parent:  nil,
	}

	for i := range c.functions {
		f := &c.functions[i]
		if body := c.functionBodies[f.Id]; body != nil {
			parentScope := c.currentScope
			c.currentScope = &Scope{
				Symbols: make(map[string]hir.Symbol, len(f.Parameters)),
				Parent:  parentScope,
			}
			c.currentFunction = f
			f.Locals = make([]hir.Local, len(f.Parameters))
			for j, param := range f.Parameters {
				f.Locals[j] = hir.Local{Name: param.Name, Type: param.Type}
				c.currentScope.Symbols[param.Name] = hir.LocalID(j)
			}

			checkedBody, ok := c.checkBlock(body)
			c.currentScope = parentScope
			c.currentFunction = nil
			if !ok {
				return false
			}
			f.Body = checkedBody
		}
	}

	return true
}

func (c *Analyzer) checkBlock(block *parser.BlockStatement) (hir.Block, bool) {
	checkBlock := hir.Block{}

	for _, s := range block.Statements {
		stmt, ok := c.checkStmt(s)
		if !ok {
			return hir.Block{}, false
		}
		checkBlock.Statements = append(checkBlock.Statements, stmt)
	}

	return checkBlock, true
}

func (c *Analyzer) checkStmt(stmt parser.Statement) (hir.Stmt, bool) {
	switch stmt := stmt.(type) {
	case *parser.ExpressionStatement:
		expr, ok := c.checkExpr(stmt.Expression)
		if !ok {
			return nil, false
		}
		return &hir.ExpressionStatement{Expr: expr}, true
	case *parser.ReturnStatement:
		expr, ok := c.checkExpr(stmt.Expr)
		if !ok {
			return nil, false
		}
		return &hir.Return{Value: expr}, true
	}

	return nil, true
}

func (c *Analyzer) checkExpr(expr parser.Expression) (hir.Expr, bool) {
	switch expr := expr.(type) {
	case *parser.IntegerLiteral:
		return c.convertIntLiteral(expr, false)
	case *parser.PrefixExpression:
		if expr.Operator == "-" {
			switch right := expr.Right.(type) {
			case *parser.IntegerLiteral:
				return c.convertIntLiteral(right, true)
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
		return c.findIdentifier(expr)
	}
	return nil, true
}

func (c *Analyzer) findIdentifier(identExpr *parser.IdentifierExpression) (hir.Expr, bool) {
	identifier := identExpr.Value

	for scope := c.currentScope; scope != nil; scope = scope.Parent {
		s, ok := scope.Symbols[identifier]
		if !ok {
			continue
		}

		switch s := s.(type) {
		case hir.LocalID:
			return &hir.LocalRef{
				ExprInfo: hir.ExprInfo{
					ResultType: c.currentFunction.Locals[s].Type,
				},
				ID: s,
			}, true
		case hir.GlobalID:
			return &hir.GlobalRef{
				ExprInfo: hir.ExprInfo{
					ResultType: c.globals[s].Type,
				},
				ID: s,
			}, true
		default:
			c.appendDiagnostic(identExpr.Position(), "cannot use `%s` as a variable", identifier)
			return nil, false
		}
	}

	c.appendDiagnostic(identExpr.Position(), "unknown variable `%s`", identifier)
	return nil, false
}

func (c *Analyzer) convertIntLiteral(expr *parser.IntegerLiteral, negative bool) (*hir.IntegerLiteral, bool) {
	bt := hir.ConvertBaseType(expr.Type.Base)
	if bt == hir.Invalid {
		c.appendDiagnostic(expr.Position(), "invalid type on integer literal")
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
		c.appendDiagnostic(expr.Position(), "invalid value for literal of type %s", expr.Type.Base.String())
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

func (c *Analyzer) assignIDs(node parser.Node) bool {
	switch node := node.(type) {
	case *parser.Program:
		valid := true
		for _, s := range node.Statements {
			if !c.assignIDs(s) {
				valid = false
			}
		}

		return valid
	case *parser.StructStatement:
		if c.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.StructID(len(c.structs))
		c.structs = append(c.structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  false,
			Private: false,
		})
		c.structPositions = append(c.structPositions, node.Position())
		c.symbols[node.Name] = id
	case *parser.ExternStructStatement:
		if c.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.StructID(len(c.structs))
		c.structs = append(c.structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  true,
			Private: node.Private,
		})
		c.structPositions = append(c.structPositions, node.Position())
		c.symbols[node.Name] = id
	case *parser.FunctionStatement:
		if c.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(c.functions))
		c.functions = append(c.functions, hir.Function{
			Name:     node.Name.Value,
			Id:       id,
			External: false,
			Private:  node.Private,
		})
		c.functionPostions = append(c.functionPostions, node.Position())
		c.functionBodies = append(c.functionBodies, node.Body)
		c.symbols[node.Name.Value] = id
	case *parser.ExternFunctionStatement:
		if c.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(c.functions))
		c.functions = append(c.functions, hir.Function{
			Name:     node.Name,
			Id:       id,
			External: true,
			Private:  node.Private,
		})
		c.functionPostions = append(c.functionPostions, node.Position())
		c.functionBodies = append(c.functionBodies, nil)
		c.symbols[node.Name] = id
	case *parser.DefStatement:
		if !node.Global {
			break
		}

		if c.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.GlobalID(len(c.globals))
		c.globals = append(c.globals, hir.Global{
			Name:     node.Name.Value,
			Id:       id,
			Constant: node.Constant,
		})
		c.globalPositions = append(c.globalPositions, node.Position())
		c.symbols[node.Name.Value] = id
	}

	return true
}

func (c *Analyzer) populateFieldsFunctions(node parser.Node) bool {
	switch node := node.(type) {
	case *parser.Program:
		valid := true
		for _, s := range node.Statements {
			if !c.populateFieldsFunctions(s) {
				valid = false
			}
		}

		return valid
	case *parser.FunctionStatement:
		funcSymbol, _ := c.symbols[node.Name.Value]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &c.functions[funcId]

		retType, ok := c.functionRetType(node.Type, node.Position(), node.Name.Value, false)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return c.checkFunctionParams(node.Params, funcAst, node.Name.Value)
	case *parser.ExternFunctionStatement:
		funcSymbol, _ := c.symbols[node.Name]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &c.functions[funcId]

		retType, ok := c.functionRetType(node.ReturnType, node.Position(), node.Name, true)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return c.checkFunctionParams(node.Params, funcAst, node.Name)
	case *parser.DefStatement:
		if !node.Global {
			break
		}
		globalSymbol, _ := c.symbols[node.Name.Value]
		globalAst := &c.globals[globalSymbol.(hir.GlobalID)]

		gt, ok := hir.ConvertVarType(node.Type, c.symbols)
		if !ok {
			c.appendDiagnostic(node.Name.Position(), "invalid type for global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.Void && gt.Pointer == 0 {
			c.appendDiagnostic(node.Name.Position(), "none type is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.StructType && gt.Pointer == 0 && c.structs[gt.Struct].Opaque {
			c.appendDiagnostic(node.Name.Position(), "opaque struct by value is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		globalAst.Type = gt
	case *parser.StructStatement:
		structSymbol, _ := c.symbols[node.Name]
		structAst := &c.structs[structSymbol.(hir.StructID)]

		fieldsSucceded := true

		structAst.FieldNames = make(map[string]int)

		for _, field := range node.Fields {
			if _, exists := structAst.FieldNames[field.Name.Value]; exists {
				c.appendDiagnostic(field.Name.Position(), "duplicate field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			ft, ok := hir.ConvertVarType(field.Type, c.symbols)
			if !ok {
				c.appendDiagnostic(field.Name.Position(), "invalid field type for field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			if ft.Base == hir.Void && ft.Pointer == 0 {
				c.appendDiagnostic(field.Name.Position(), "none type not allowed on fields, field `%s` on struct `%s`", field.Name.Value, node.Name)
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

func (c *Analyzer) checkSizedDeclarations() bool {
	for i := range c.structs {
		s := &c.structs[i]
		s.Unsized = !c.isSized(hir.Type{Base: hir.StructType, Struct: s.Id}, map[hir.StructID]struct{}{})
	}

	for _, fnc := range c.functions {
		for _, p := range fnc.Parameters {
			if !c.isSized(p.Type, map[hir.StructID]struct{}{}) {
				c.appendDiagnostic(c.functionPostions[fnc.Id], "unsized type is not allowed for parameter `%s` in function `%s`; use a pointer", p.Name, fnc.Name)
				return false
			}
		}
		if !fnc.External && fnc.ReturnType.Base != hir.Void && !c.isSized(fnc.ReturnType, map[hir.StructID]struct{}{}) {
			c.appendDiagnostic(c.functionPostions[fnc.Id], "unsized return type is not allowed for function `%s`; use a pointer", fnc.Name)
			return false
		}
	}

	for _, g := range c.globals {
		if !c.isSized(g.Type, map[hir.StructID]struct{}{}) {
			c.appendDiagnostic(c.globalPositions[g.Id], "unsized type is not allowed for global `%s`; use a pointer", g.Name)
			return false
		}
	}

	return true
}

func (c *Analyzer) checkFunctionParams(params []parser.FunctionParameter, funcAst *hir.Function, funcName string) bool {
	funcAst.ParameterNames = make(map[string]int)
	paramsSucceded := true

	for _, param := range params {
		if _, exists := funcAst.ParameterNames[param.Name.Value]; exists {
			c.appendDiagnostic(param.Name.Position(), "duplicate parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}

		pt, ok := hir.ConvertVarType(param.Type, c.symbols)
		if !ok {
			c.appendDiagnostic(param.Name.Position(), "invalid parameter type for parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == hir.Void && pt.Pointer == 0 {
			c.appendDiagnostic(param.Name.Position(), "none type is not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == hir.StructType && pt.Pointer == 0 {
			stct := c.structs[pt.Struct]
			if stct.Opaque {
				c.appendDiagnostic(param.Name.Position(), "opaque struct value types are not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
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

func (c *Analyzer) functionRetType(retType lexer.VarType, position *util.Position, funcName string, extern bool) (hir.Type, bool) {
	rt, ok := hir.ConvertVarType(retType, c.symbols)
	if !ok {
		c.appendDiagnostic(position, "invalid return type for function `%s`", funcName)
		return hir.Type{}, false
	}

	if !extern && rt.Base == hir.StructType && rt.Pointer == 0 && c.structs[rt.Struct].Opaque {
		c.appendDiagnostic(position, "opaque struct value return type is not allowed for function `%s`", funcName)
		return hir.Type{}, false
	}

	return rt, true
}

func (c *Analyzer) isSized(t hir.Type, visitedStructs map[hir.StructID]struct{}) bool {
	if t.Pointer > 0 {
		return true
	}

	// primitive
	if t.Base != hir.StructType && t.Base != hir.Void {
		return true
	}

	if t.Base == hir.StructType {
		if c.structs[t.Struct].Opaque {
			return false
		} else {
			if _, ok := visitedStructs[t.Struct]; ok {
				return false
			}
			visitedStructs[t.Struct] = struct{}{}
			defer delete(visitedStructs, t.Struct)

			for _, f := range c.structs[t.Struct].Fields {
				if !c.isSized(f.Type, visitedStructs) {
					return false
				}
			}

			return true
		}
	}

	return false
}

func (c *Analyzer) isSymbolDuplicate(name string, pos *util.Position) bool {
	if _, exists := c.symbols[name]; exists {
		c.appendDiagnostic(pos, "duplicate symbol `%s`", name)
		return true
	}

	return false
}

func (c *Analyzer) appendDiagnostic(pos *util.Position, msg string, v ...any) {
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Message:  fmt.Sprintf(msg, v...),
		Position: pos,
	})
}
