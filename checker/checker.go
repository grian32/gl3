package checker

import (
	"fmt"
	"gl3/checkedast"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
)

type Scope struct {
	Symbols map[string]checkedast.Symbol
	Parent  *Scope
}

type Checker struct {
	structs   []checkedast.Struct
	functions []checkedast.Function
	globals   []checkedast.Global

	structPositions  []*util.Position
	functionPostions []*util.Position
	globalPositions  []*util.Position

	functionBodies []*parser.BlockStatement

	symbols         map[string]checkedast.Symbol
	currentScope    *Scope
	currentFunction *checkedast.Function

	diagnostics []Diagnostic
}

func NewChecker() *Checker {
	return &Checker{symbols: make(map[string]checkedast.Symbol)}
}

func (c *Checker) CheckProgram(program *parser.Program) (*checkedast.Program, []Diagnostic) /* todo: diag */ {
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
	return &checkedast.Program{
		Structs:   c.structs,
		Functions: c.functions,
		Globals:   c.globals,
	}, c.diagnostics
}

func (c *Checker) checkBodies(node parser.Node) bool {
	c.currentScope = &Scope{
		Symbols: c.symbols,
		Parent:  nil,
	}

	for i := range c.functions {
		f := &c.functions[i]
		if body := c.functionBodies[f.Id]; body != nil {
			parentScope := c.currentScope
			c.currentScope = &Scope{
				Symbols: make(map[string]checkedast.Symbol, len(f.Parameters)),
				Parent:  parentScope,
			}
			c.currentFunction = f
			f.Locals = make([]checkedast.Local, len(f.Parameters))
			for j, param := range f.Parameters {
				f.Locals[j] = checkedast.Local{Name: param.Name, Type: param.Type}
				c.currentScope.Symbols[param.Name] = checkedast.LocalID(j)
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

func (c *Checker) checkBlock(block *parser.BlockStatement) (checkedast.Block, bool) {
	checkBlock := checkedast.Block{}

	for _, s := range block.Statements {
		stmt, ok := c.checkStmt(s)
		if !ok {
			return checkedast.Block{}, false
		}
		checkBlock.Statements = append(checkBlock.Statements, stmt)
	}

	return checkBlock, true
}

func (c *Checker) checkStmt(stmt parser.Statement) (checkedast.Stmt, bool) {
	switch stmt := stmt.(type) {
	case *parser.ExpressionStatement:
		expr, ok := c.checkExpr(stmt.Expression)
		if !ok {
			return nil, false
		}
		return &checkedast.ExpressionStatement{Expr: expr}, true
	case *parser.ReturnStatement:
		expr, ok := c.checkExpr(stmt.Expr)
		if !ok {
			return nil, false
		}
		return &checkedast.Return{Value: expr}, true
	}

	return nil, true
}

func (c *Checker) checkExpr(expr parser.Expression) (checkedast.Expr, bool) {
	switch expr := expr.(type) {
	case *parser.IntegerLiteral:
		return c.convertIntLiteral(expr, false)
	case *parser.PrefixExpression:
		if expr.Operator == "-" {
			switch right := expr.Right.(type) {
			case *parser.IntegerLiteral:
				return c.convertIntLiteral(right, true)
			case *parser.FloatLiteral:
				return &checkedast.FloatLiteral{
					ExprInfo: checkedast.Info(checkedast.Float),
					Value:    -right.Value,
				}, true
			}
		}
	case *parser.BooleanExpression:
		return &checkedast.BooleanLiteral{
			ExprInfo: checkedast.Info(checkedast.Bool),
			Value:    expr.Value,
		}, true
	case *parser.StringLiteral:
		return &checkedast.StringLiteral{
			ExprInfo: checkedast.InfoPtr(checkedast.Char, 1),
			Value:    expr.Value,
		}, true
	case *parser.FloatLiteral:
		return &checkedast.FloatLiteral{
			ExprInfo: checkedast.Info(checkedast.Float),
			Value:    expr.Value,
		}, true
	case *parser.IdentifierExpression:
		return c.findIdentifier(expr)
	}
	return nil, true
}

func (c *Checker) findIdentifier(identExpr *parser.IdentifierExpression) (checkedast.Expr, bool) {
	identifier := identExpr.Value

	for scope := c.currentScope; scope != nil; scope = scope.Parent {
		s, ok := scope.Symbols[identifier]
		if !ok {
			continue
		}

		switch s := s.(type) {
		case checkedast.LocalID:
			return &checkedast.LocalRef{
				ExprInfo: checkedast.ExprInfo{
					ResultType: c.currentFunction.Locals[s].Type,
				},
				ID: s,
			}, true
		case checkedast.GlobalID:
			return &checkedast.GlobalRef{
				ExprInfo: checkedast.ExprInfo{
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

func (c *Checker) convertIntLiteral(expr *parser.IntegerLiteral, negative bool) (*checkedast.IntegerLiteral, bool) {
	bt := checkedast.ConvertBaseType(expr.Type.Base)
	if bt == checkedast.Invalid {
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

	return &checkedast.IntegerLiteral{
		ExprInfo: checkedast.ExprInfo{
			ResultType: checkedast.Type{Base: bt},
		},
		Value: value,
	}, true
}

func (c *Checker) assignIDs(node parser.Node) bool {
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

		id := checkedast.StructID(len(c.structs))
		c.structs = append(c.structs, checkedast.Struct{
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

		id := checkedast.StructID(len(c.structs))
		c.structs = append(c.structs, checkedast.Struct{
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

		id := checkedast.FunctionID(len(c.functions))
		c.functions = append(c.functions, checkedast.Function{
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

		id := checkedast.FunctionID(len(c.functions))
		c.functions = append(c.functions, checkedast.Function{
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

		id := checkedast.GlobalID(len(c.globals))
		c.globals = append(c.globals, checkedast.Global{
			Name:     node.Name.Value,
			Id:       id,
			Constant: node.Constant,
		})
		c.globalPositions = append(c.globalPositions, node.Position())
		c.symbols[node.Name.Value] = id
	}

	return true
}

func (c *Checker) populateFieldsFunctions(node parser.Node) bool {
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
		funcId := funcSymbol.(checkedast.FunctionID)
		funcAst := &c.functions[funcId]

		retType, ok := c.functionRetType(node.Type, node.Position(), node.Name.Value, false)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return c.checkFunctionParams(node.Params, funcAst, node.Name.Value)
	case *parser.ExternFunctionStatement:
		funcSymbol, _ := c.symbols[node.Name]
		funcId := funcSymbol.(checkedast.FunctionID)
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
		globalAst := &c.globals[globalSymbol.(checkedast.GlobalID)]

		gt, ok := checkedast.ConvertVarType(node.Type, c.symbols)
		if !ok {
			c.appendDiagnostic(node.Name.Position(), "invalid type for global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == checkedast.Void && gt.Pointer == 0 {
			c.appendDiagnostic(node.Name.Position(), "none type is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == checkedast.StructType && gt.Pointer == 0 && c.structs[gt.Struct].Opaque {
			c.appendDiagnostic(node.Name.Position(), "opaque struct by value is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		globalAst.Type = gt
	case *parser.StructStatement:
		structSymbol, _ := c.symbols[node.Name]
		structAst := &c.structs[structSymbol.(checkedast.StructID)]

		fieldsSucceded := true

		structAst.FieldNames = make(map[string]int)

		for _, field := range node.Fields {
			if _, exists := structAst.FieldNames[field.Name.Value]; exists {
				c.appendDiagnostic(field.Name.Position(), "duplicate field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			ft, ok := checkedast.ConvertVarType(field.Type, c.symbols)
			if !ok {
				c.appendDiagnostic(field.Name.Position(), "invalid field type for field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			if ft.Base == checkedast.Void && ft.Pointer == 0 {
				c.appendDiagnostic(field.Name.Position(), "none type not allowed on fields, field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			structAst.FieldNames[field.Name.Value] = len(structAst.Fields)
			structAst.Fields = append(structAst.Fields, checkedast.TypedName{
				Name: field.Name.Value,
				Type: ft,
			})
		}

		return fieldsSucceded
	}

	return true
}

func (c *Checker) checkSizedDeclarations() bool {
	for i := range c.structs {
		s := &c.structs[i]
		s.Unsized = !c.isSized(checkedast.Type{Base: checkedast.StructType, Struct: s.Id}, map[checkedast.StructID]struct{}{})
	}

	for _, fnc := range c.functions {
		for _, p := range fnc.Parameters {
			if !c.isSized(p.Type, map[checkedast.StructID]struct{}{}) {
				c.appendDiagnostic(c.functionPostions[fnc.Id], "unsized type is not allowed for parameter `%s` in function `%s`; use a pointer", p.Name, fnc.Name)
				return false
			}
		}
		if !fnc.External && fnc.ReturnType.Base != checkedast.Void && !c.isSized(fnc.ReturnType, map[checkedast.StructID]struct{}{}) {
			c.appendDiagnostic(c.functionPostions[fnc.Id], "unsized return type is not allowed for function `%s`; use a pointer", fnc.Name)
			return false
		}
	}

	for _, g := range c.globals {
		if !c.isSized(g.Type, map[checkedast.StructID]struct{}{}) {
			c.appendDiagnostic(c.globalPositions[g.Id], "unsized type is not allowed for global `%s`; use a pointer", g.Name)
			return false
		}
	}

	return true
}

func (c *Checker) checkFunctionParams(params []parser.FunctionParameter, funcAst *checkedast.Function, funcName string) bool {
	funcAst.ParameterNames = make(map[string]int)
	paramsSucceded := true

	for _, param := range params {
		if _, exists := funcAst.ParameterNames[param.Name.Value]; exists {
			c.appendDiagnostic(param.Name.Position(), "duplicate parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}

		pt, ok := checkedast.ConvertVarType(param.Type, c.symbols)
		if !ok {
			c.appendDiagnostic(param.Name.Position(), "invalid parameter type for parameter `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == checkedast.Void && pt.Pointer == 0 {
			c.appendDiagnostic(param.Name.Position(), "none type is not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
			paramsSucceded = false
			continue
		}
		if pt.Base == checkedast.StructType && pt.Pointer == 0 {
			stct := c.structs[pt.Struct]
			if stct.Opaque {
				c.appendDiagnostic(param.Name.Position(), "opaque struct value types are not allowed on parameters, param `%s` on function `%s`", param.Name.Value, funcName)
				paramsSucceded = false
				continue
			}
		}

		funcAst.ParameterNames[param.Name.Value] = len(funcAst.Parameters)
		funcAst.Parameters = append(funcAst.Parameters, checkedast.TypedName{
			Name: param.Name.Value,
			Type: pt,
		})
	}

	return paramsSucceded
}

func (c *Checker) functionRetType(retType lexer.VarType, position *util.Position, funcName string, extern bool) (checkedast.Type, bool) {
	rt, ok := checkedast.ConvertVarType(retType, c.symbols)
	if !ok {
		c.appendDiagnostic(position, "invalid return type for function `%s`", funcName)
		return checkedast.Type{}, false
	}

	if !extern && rt.Base == checkedast.StructType && rt.Pointer == 0 && c.structs[rt.Struct].Opaque {
		c.appendDiagnostic(position, "opaque struct value return type is not allowed for function `%s`", funcName)
		return checkedast.Type{}, false
	}

	return rt, true
}

func (c *Checker) isSized(t checkedast.Type, visitedStructs map[checkedast.StructID]struct{}) bool {
	if t.Pointer > 0 {
		return true
	}

	// primitive
	if t.Base != checkedast.StructType && t.Base != checkedast.Void {
		return true
	}

	if t.Base == checkedast.StructType {
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

func (c *Checker) isSymbolDuplicate(name string, pos *util.Position) bool {
	if _, exists := c.symbols[name]; exists {
		c.appendDiagnostic(pos, "duplicate symbol `%s`", name)
		return true
	}

	return false
}

func (c *Checker) appendDiagnostic(pos *util.Position, msg string, v ...any) {
	c.diagnostics = append(c.diagnostics, Diagnostic{
		Message:  fmt.Sprintf(msg, v...),
		Position: pos,
	})
}
