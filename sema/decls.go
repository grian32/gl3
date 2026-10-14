package sema

import (
	"gl3/hir"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
)

func (a *Analyzer) checkPublicTypes() bool {
	valid := true

	for i := a.importedStructs; i < len(a.Structs); i++ {
		decl := &a.Structs[i]
		if decl.Private {
			continue
		}
		for _, field := range decl.Fields {
			if private, ok := a.privateStruct(field.Type); ok {
				a.appendDiagnostic(a.structPositions[i], "public struct `%s` exposes private struct `%s` in field `%s`", decl.Name, private.Name, field.Name)
				valid = false
			}
		}
	}

	for i := a.importedFunctions; i < len(a.Functions); i++ {
		decl := &a.Functions[i]
		if decl.Private {
			continue
		}
		for _, parameter := range decl.Parameters {
			if private, ok := a.privateStruct(parameter.Type); ok {
				a.appendDiagnostic(a.functionPositions[i], "public function `%s` exposes private struct `%s` in parameter `%s`", decl.Name, private.Name, parameter.Name)
				valid = false
			}
		}
		if private, ok := a.privateStruct(decl.ReturnType); ok {
			a.appendDiagnostic(a.functionPositions[i], "public function `%s` exposes private struct `%s` in its return type", decl.Name, private.Name)
			valid = false
		}
	}

	for i := a.importedGlobals; i < len(a.Globals); i++ {
		decl := &a.Globals[i]
		if !decl.Constant {
			continue
		}
		if private, ok := a.privateStruct(decl.Type); ok {
			a.appendDiagnostic(a.globalPositions[i], "public global `%s` exposes private struct `%s`", decl.Name, private.Name)
			valid = false
		}
	}

	return valid
}

func (a *Analyzer) privateStruct(t hir.Type) (*hir.Struct, bool) {
	if t.Base != hir.StructType || !a.Structs[t.Struct].Private {
		return nil, false
	}
	return &a.Structs[t.Struct], true
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

		id := hir.StructID(len(a.Structs))
		a.Structs = append(a.Structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  false,
			Private: node.Private,
		})
		a.structPositions = append(a.structPositions, node.Position())
		a.Symbols[node.Name] = id
	case *parser.ExternStructStatement:
		if a.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.StructID(len(a.Structs))
		a.Structs = append(a.Structs, hir.Struct{
			Name:    node.Name,
			Id:      id,
			Opaque:  true,
			Private: node.Private,
		})
		a.structPositions = append(a.structPositions, node.Position())
		a.Symbols[node.Name] = id
	case *parser.FunctionStatement:
		if a.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(a.Functions))
		a.Functions = append(a.Functions, hir.Function{
			Name:     node.Name.Value,
			Id:       id,
			External: false,
			Variadic: node.Variadic,
			Private:  node.Private,
		})
		if node.Variadic {
			a.containsVariadic = true
		}
		a.functionPositions = append(a.functionPositions, node.Position())
		a.functionBodies = append(a.functionBodies, node.Body)
		a.Symbols[node.Name.Value] = id
	case *parser.ExternFunctionStatement:
		if a.isSymbolDuplicate(node.Name, node.Position()) {
			return false
		}

		id := hir.FunctionID(len(a.Functions))
		a.Functions = append(a.Functions, hir.Function{
			Name:     node.Name,
			Id:       id,
			External: true,
			Variadic: node.Variadic,
			Private:  node.Private,
		})
		a.functionPositions = append(a.functionPositions, node.Position())
		a.functionBodies = append(a.functionBodies, nil)
		a.Symbols[node.Name] = id
	case *parser.DefStatement:
		if !node.Global {
			a.appendDiagnostic(node.Position(), "top level expressions are not allowed")
			break
		}

		if a.isSymbolDuplicate(node.Name.Value, node.Position()) {
			return false
		}

		id := hir.GlobalID(len(a.Globals))
		a.Globals = append(a.Globals, hir.Global{
			Name:     node.Name.Value,
			Id:       id,
			Constant: node.Constant,
		})
		a.globalPositions = append(a.globalPositions, node.Position())
		a.globalInitializers = append(a.globalInitializers, node.Right)
		a.Symbols[node.Name.Value] = id
	case *parser.ImportStatement:
		// TODO
		return true
	default:
		a.appendDiagnostic(node.Position(), "top level expressions are not allowed")
		return false
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
		funcSymbol, _ := a.Symbols[node.Name.Value]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &a.Functions[funcId]

		retType, ok := a.functionRetType(node.Type, node.Position(), node.Name.Value, false)
		if !ok {
			return false
		}

		funcAst.ReturnType = retType
		return a.checkFunctionParams(node.Params, funcAst, node.Name.Value)
	case *parser.ExternFunctionStatement:
		funcSymbol, _ := a.Symbols[node.Name]
		funcId := funcSymbol.(hir.FunctionID)
		funcAst := &a.Functions[funcId]

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
		Globalsymbol, _ := a.Symbols[node.Name.Value]
		globalAst := &a.Globals[Globalsymbol.(hir.GlobalID)]

		gt, ok := hir.ConvertVarType(node.Type, a.Symbols)
		if !ok {
			a.appendDiagnostic(node.Name.Position(), "invalid type for global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.Void && gt.Pointer == 0 {
			a.appendDiagnostic(node.Name.Position(), "none type is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		if gt.Base == hir.StructType && gt.Pointer == 0 && a.Structs[gt.Struct].Opaque {
			a.appendDiagnostic(node.Name.Position(), "opaque struct by value is not allowed on global definitions, global `%s`", node.Name.Value)
			return false
		}

		globalAst.Type = gt
	case *parser.StructStatement:
		Structsymbol, _ := a.Symbols[node.Name]
		structAst := &a.Structs[Structsymbol.(hir.StructID)]

		fieldsSucceded := true

		structAst.FieldNames = make(map[string]int)

		for _, field := range node.Fields {
			if _, exists := structAst.FieldNames[field.Name.Value]; exists {
				a.appendDiagnostic(field.Name.Position(), "duplicate field `%s` on struct `%s`", field.Name.Value, node.Name)
				fieldsSucceded = false
				continue
			}

			ft, ok := hir.ConvertVarType(field.Type, a.Symbols)
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
	for i := range a.Structs {
		s := &a.Structs[i]
		s.Unsized = !a.computeSized(hir.Type{Base: hir.StructType, Struct: s.Id}, map[hir.StructID]struct{}{})
	}

	for _, fnc := range a.Functions {
		for _, p := range fnc.Parameters {
			if !a.isSized(p.Type) {
				a.appendDiagnostic(a.functionPositions[fnc.Id], "unsized type is not allowed for parameter `%s` in function `%s`; use a pointer", p.Name, fnc.Name)
				return false
			}
		}
		if !fnc.External && fnc.ReturnType.Base != hir.Void && !a.isSized(fnc.ReturnType) {
			a.appendDiagnostic(a.functionPositions[fnc.Id], "unsized return type is not allowed for function `%s`; use a pointer", fnc.Name)
			return false
		}
	}

	for _, g := range a.Globals {
		if !a.isSized(g.Type) {
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

		pt, ok := hir.ConvertVarType(param.Type, a.Symbols)
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
			stct := a.Structs[pt.Struct]
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
	rt, ok := hir.ConvertVarType(retType, a.Symbols)
	if !ok {
		a.appendDiagnostic(position, "invalid return type for function `%s`", funcName)
		return hir.Type{}, false
	}

	if !extern && rt.Base == hir.StructType && rt.Pointer == 0 && a.Structs[rt.Struct].Opaque {
		a.appendDiagnostic(position, "opaque struct value return type is not allowed for function `%s`", funcName)
		return hir.Type{}, false
	}

	return rt, true
}

func (a *Analyzer) isSized(t hir.Type) bool {
	if t.Pointer > 0 {
		return true
	}
	if t.Base == hir.StructType {
		return !a.Structs[t.Struct].Unsized
	}
	return t.Base != hir.Void
}

func (a *Analyzer) computeSized(t hir.Type, visitedStructs map[hir.StructID]struct{}) bool {
	if t.Pointer > 0 {
		return true
	}

	// primitive
	if t.Base != hir.StructType && t.Base != hir.Void {
		return true
	}

	if t.Base == hir.StructType {
		if a.Structs[t.Struct].Opaque {
			return false
		} else {
			if _, ok := visitedStructs[t.Struct]; ok {
				return false
			}
			visitedStructs[t.Struct] = struct{}{}
			defer delete(visitedStructs, t.Struct)

			for _, f := range a.Structs[t.Struct].Fields {
				if !a.computeSized(f.Type, visitedStructs) {
					return false
				}
			}

			return true
		}
	}

	return false
}

func isConstantInitializer(expr hir.Expr) bool {
	switch expr := expr.(type) {
	case *hir.IntegerLiteral, *hir.FloatLiteral,
		*hir.BooleanLiteral, *hir.StringLiteral, *hir.Sizeof, *hir.NullPointer:
		return true
	case *hir.Cast:
		return isConstantInitializer(expr.Value)
	case *hir.StructLiteral:
		for _, field := range expr.Fields {
			if !isConstantInitializer(field) {
				return false
			}
		}
		return true
	default:
		return false
	}
}
