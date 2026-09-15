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
		return a.checkPrefix(expr)
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
	case *parser.CallExpression:
		return a.checkCall(expr)
	case *parser.AssignmentExpression:
		return a.checkAssignment(expr)
	case *parser.InfixExpression:
		return a.checkBinaryOp(expr)
	case *parser.DereferenceExpression:
		return a.checkDeref(expr)
	case *parser.ReferenceExpression:
		return a.checkRef(expr)
	case *parser.StructInitializationExpression:
		return a.checkStructLiteral(expr)
	}
	return nil, false
}

func (a *Analyzer) checkStructLiteral(expr *parser.StructInitializationExpression) (*hir.StructLiteral, bool) {
	symbol, ok := a.symbols[expr.Name]
	if !ok {
		a.appendDiagnostic(expr.Position(), "symbol does not exist")
		return nil, false
	}
	structId, ok := symbol.(hir.StructID)
	if !ok {
		a.appendDiagnostic(expr.Position(), "`%s` is not struct", expr.Name)
		return nil, false
	}
	s := a.structs[structId]
	if s.Opaque || !a.isSized(hir.Type{Base: hir.StructType, Struct: structId}, make(map[hir.StructID]struct{})) {
		a.appendDiagnostic(expr.Position(), "struct literals for unsized or opaque structs are not allowed")
		return nil, false
	}

	if len(s.Fields) != len(expr.Values) {
		a.appendDiagnostic(expr.Position(), "wanted %d fields in literal for struct `%s`, got %d", len(s.Fields), s.Name, len(expr.Values))
		return nil, false
	}

	fieldsOk := true
	fields := []hir.Expr{}
	for i, f := range expr.Values {
		fieldExpr, ok := a.checkExpr(f)
		if !ok {
			fieldsOk = false
			continue
		}
		if fieldExpr.Type() != s.Fields[i].Type {
			a.appendDiagnostic(f.Position(), "wanted %s for field %d in literal for struct `%s`, got %s", s.Fields[i].Type, i, s.Name, fieldExpr.Type())
			fieldsOk = false
			continue
		}
		fields = append(fields, fieldExpr)
	}
	if !fieldsOk {
		return nil, false
	}

	return &hir.StructLiteral{
		ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.StructType, Struct: structId}},
		Fields:   fields,
	}, true
}

func (a *Analyzer) checkRef(expr *parser.ReferenceExpression) (*hir.AddressOf, bool) {
	place, ok := a.checkPlace(expr.Var)
	if !ok {
		return nil, false
	}

	t := place.Type()
	// can overflow technically but if you have > 255 pointers you have bigger problems
	t.Pointer++

	return &hir.AddressOf{
		ExprInfo: hir.ExprInfo{ResultType: t},
		Target:   place,
	}, true
}

func (a *Analyzer) checkDeref(expr *parser.DereferenceExpression) (*hir.Dereference, bool) {
	ptr, ok := a.checkExpr(expr.Var)
	if !ok {
		return nil, false
	}

	t := ptr.Type()
	if t.Pointer == 0 {
		a.appendDiagnostic(expr.Position(), "cannot dereference a non pointer")
		return nil, false
	}
	t.Pointer--

	if !a.isSized(t, make(map[hir.StructID]struct{})) {
		a.appendDiagnostic(expr.Position(), "cannot dereference unsized type `%s`", t)
		return nil, false
	}

	return &hir.Dereference{
		ExprInfo: hir.ExprInfo{ResultType: t},
		Pointer:  ptr,
	}, true
}

func (a *Analyzer) checkPrefix(expr *parser.PrefixExpression) (hir.Expr, bool) {
	switch expr.Operator {
	case "-":
		switch right := expr.Right.(type) {
		case *parser.IntegerLiteral:
			return a.convertIntLiteral(right, true)
		case *parser.FloatLiteral:
			return &hir.FloatLiteral{ExprInfo: hir.Info(hir.Float), Value: -right.Value}, true
		}
	case "!":
		if right, ok := expr.Right.(*parser.BooleanExpression); ok {
			return &hir.BooleanLiteral{ExprInfo: hir.Info(hir.Bool), Value: !right.Value}, true
		}
	}

	right, ok := a.checkExpr(expr.Right)
	if !ok {
		return nil, false
	}
	t := right.Type()
	op := hir.InvalidUnaryOp
	if t.Pointer == 0 {
		switch expr.Operator {
		case "-":
			switch t.Base {
			case hir.Int, hir.Int32, hir.Int16, hir.Int8:
				op = hir.IntNegate
			case hir.Float:
				op = hir.FloatNegate
			}
		case "!":
			if t.Base == hir.Bool {
				op = hir.BoolNot
			}
		}
	}
	if op == hir.InvalidUnaryOp {
		if expr.Operator == "!" {
			a.appendDiagnostic(expr.Position(), "operator `!` requires bool, got `%s`", t)
		} else {
			a.appendDiagnostic(expr.Position(), "unsupported prefix op `%s` on type `%s`", expr.Operator, t)
		}
		return nil, false
	}
	return &hir.Unary{
		ExprInfo: hir.ExprInfo{ResultType: t},
		Op:       op,
		Value:    right,
	}, true
}

func (a *Analyzer) resolveField(baseType hir.Type, expr *parser.InfixExpression) (int, bool) {
	if baseType.Base != hir.StructType || baseType.Pointer != 0 {
		a.appendDiagnostic(expr.Position(), "cannot access field on non-struct type `%s`", baseType)
		return 0, false
	}
	s := a.structs[baseType.Struct]
	fieldIdent, ok := expr.Right.(*parser.IdentifierExpression)
	if !ok {
		a.appendDiagnostic(expr.Right.Position(), "non identifier is not allowed on rhs of struct access")
		return 0, false
	}
	idx, ok := s.FieldNames[fieldIdent.Value]
	if !ok {
		a.appendDiagnostic(expr.Position(), "field `%s` does not exist on struct `%s`", fieldIdent.Value, s.Name)
		return 0, false
	}
	return idx, true
}

func (a *Analyzer) checkBinaryOp(infixExpr *parser.InfixExpression) (hir.Expr, bool) {
	if infixExpr.Operator == "." {
		left, ok := a.checkExpr(infixExpr.Left)
		if !ok {
			return nil, false
		}
		idx, ok := a.resolveField(left.Type(), infixExpr)
		if !ok {
			return nil, false
		}
		field := a.structs[left.Type().Struct].Fields[idx]

		return &hir.FieldAccess{
			ExprInfo:   hir.ExprInfo{ResultType: field.Type},
			Base:       left,
			FieldIndex: idx,
		}, true
	}

	leftExpr, ok := a.checkExpr(infixExpr.Left)
	if !ok {
		return nil, false
	}
	rightExpr, ok := a.checkExpr(infixExpr.Right)
	if !ok {
		return nil, false
	}
	if leftExpr.Type().Pointer > 0 {
		op := hir.PointerAdd
		if infixExpr.Operator == "-" {
			op = hir.PointerSubtract
		} else if infixExpr.Operator != "+" {
			a.appendDiagnostic(infixExpr.Position(), "unsupported op `%s` on pointer type `%s`", infixExpr.Operator, leftExpr.Type())
			return nil, false
		}

		offsetType := rightExpr.Type()
		integerOffset := false
		if offsetType.Pointer == 0 {
			switch offsetType.Base {
			case hir.Int, hir.Int32, hir.Int16, hir.Int8,
				hir.Uint, hir.Uint32, hir.Uint16, hir.Uint8:
				integerOffset = true
			}
		}
		if !integerOffset {
			a.appendDiagnostic(infixExpr.Right.Position(), "pointer arithmetic requires an integer offset, got `%s`", offsetType)
			return nil, false
		}

		elementType := leftExpr.Type()
		elementType.Pointer--
		if !a.isSized(elementType, make(map[hir.StructID]struct{})) {
			if elementType.Base == hir.StructType && a.structs[elementType.Struct].Opaque {
				a.appendDiagnostic(infixExpr.Position(), "pointer arithmetic requires a sized element type; struct `%s` is opaque", a.structs[elementType.Struct].Name)
			} else {
				a.appendDiagnostic(infixExpr.Position(), "pointer arithmetic requires a sized element type, got `%s`", elementType)
			}
			return nil, false
		}
		return makeBinaryNode(op, leftExpr, rightExpr, leftExpr.Type()), true
	}

	if leftExpr.Type() != rightExpr.Type() {
		a.appendDiagnostic(infixExpr.Position(), "types of operands cannot be different.")
		return nil, false
	}

	boolType := hir.Type{Base: hir.Bool}

	switch leftExpr.Type().Base {
	case hir.Int, hir.Int32, hir.Int16, hir.Int8,
		hir.Uint, hir.Uint32, hir.Uint16, hir.Uint8:
		unsigned := false
		switch leftExpr.Type().Base {
		case hir.Uint, hir.Uint32, hir.Uint16, hir.Uint8:
			unsigned = true
		}
		switch infixExpr.Operator {
		case "+":
			return makeBinaryNode(hir.IntAdd, leftExpr, rightExpr, leftExpr.Type()), true
		case "-":
			return makeBinaryNode(hir.IntSubtract, leftExpr, rightExpr, leftExpr.Type()), true
		case "*":
			return makeBinaryNode(hir.IntMultiply, leftExpr, rightExpr, leftExpr.Type()), true
		case "/":
			op := hir.SignedDivide
			if unsigned {
				op = hir.UnsignedDivide
			}
			return makeBinaryNode(op, leftExpr, rightExpr, leftExpr.Type()), true
		case "==":
			return makeBinaryNode(hir.IntEqual, leftExpr, rightExpr, boolType), true
		case "!=":
			return makeBinaryNode(hir.IntNotEqual, leftExpr, rightExpr, boolType), true
		case "<":
			op := hir.SignedLess
			if unsigned {
				op = hir.UnsignedLess
			}
			return makeBinaryNode(op, leftExpr, rightExpr, boolType), true
		case "<=":
			op := hir.SignedLessEqual
			if unsigned {
				op = hir.UnsignedLessEqual
			}
			return makeBinaryNode(op, leftExpr, rightExpr, boolType), true
		case ">":
			op := hir.SignedGreater
			if unsigned {
				op = hir.UnsignedGreater
			}
			return makeBinaryNode(op, leftExpr, rightExpr, boolType), true
		case ">=":
			op := hir.SignedGreaterEqual
			if unsigned {
				op = hir.UnsignedGreaterEqual
			}
			return makeBinaryNode(op, leftExpr, rightExpr, boolType), true
		}
	case hir.Float:
		switch infixExpr.Operator {
		case "+":
			return makeBinaryNode(hir.FloatAdd, leftExpr, rightExpr, leftExpr.Type()), true
		case "-":
			return makeBinaryNode(hir.FloatSubtract, leftExpr, rightExpr, leftExpr.Type()), true
		case "*":
			return makeBinaryNode(hir.FloatMultiply, leftExpr, rightExpr, leftExpr.Type()), true
		case "/":
			return makeBinaryNode(hir.FloatDivide, leftExpr, rightExpr, leftExpr.Type()), true
		case "==":
			return makeBinaryNode(hir.FloatEqual, leftExpr, rightExpr, boolType), true
		case "!=":
			return makeBinaryNode(hir.FloatNotEqual, leftExpr, rightExpr, boolType), true
		case "<":
			return makeBinaryNode(hir.FloatLess, leftExpr, rightExpr, boolType), true
		case "<=":
			return makeBinaryNode(hir.FloatLessEqual, leftExpr, rightExpr, boolType), true
		case ">":
			return makeBinaryNode(hir.FloatGreater, leftExpr, rightExpr, boolType), true
		case ">=":
			return makeBinaryNode(hir.FloatGreaterEqual, leftExpr, rightExpr, boolType), true
		}
	case hir.Bool:
		switch infixExpr.Operator {
		case "&&":
			return makeBinaryNode(hir.BoolAnd, leftExpr, rightExpr, leftExpr.Type()), true
		case "||":
			return makeBinaryNode(hir.BoolOr, leftExpr, rightExpr, leftExpr.Type()), true
		case "==":
			return makeBinaryNode(hir.BoolEqual, leftExpr, rightExpr, leftExpr.Type()), true
		case "!=":
			return makeBinaryNode(hir.BoolNotEqual, leftExpr, rightExpr, leftExpr.Type()), true
		}
	}

	a.appendDiagnostic(infixExpr.Position(), "unsupported op `%s` on types `%s` and `%s`.", infixExpr.Operator, leftExpr.Type(), rightExpr.Type())
	return nil, false
}

func makeBinaryNode(op hir.BinaryOp, left, right hir.Expr, resultType hir.Type) *hir.Binary {
	return &hir.Binary{
		ExprInfo: hir.ExprInfo{ResultType: resultType},
		Op:       op,
		Left:     left,
		Right:    right,
	}
}

func (a *Analyzer) checkAssignment(assignExpr *parser.AssignmentExpression) (*hir.Assignment, bool) {
	place, ok := a.checkPlace(assignExpr.Left)
	if !ok {
		return nil, false
	}
	if !a.isSized(place.Type(), make(map[hir.StructID]struct{})) {
		a.appendDiagnostic(assignExpr.Position(), "cannot assign to unsized type `%s`.", place.Type())
		return nil, false
	}

	root := place
	for {
		field, ok := root.(*hir.FieldPlace)
		if !ok {
			break
		}
		root = field.Base
	}
	if gp, ok := root.(*hir.GlobalPlace); ok && a.globals[gp.ID].Constant {
		a.appendDiagnostic(assignExpr.Position(), "modifying constant globals is not permitted.")
		return nil, false
	}

	rightExpr, ok := a.checkExpr(assignExpr.Right)
	if !ok {
		a.appendDiagnostic(assignExpr.Position(), "invalid expr on rhs of assignment.")
		return nil, false
	}

	if rightExpr.Type() != place.Type() {
		a.appendDiagnostic(assignExpr.Position(), "expected type `%s` in assignment but got type `%s`.", place.Type(), rightExpr.Type())
		return nil, false
	}

	return &hir.Assignment{
		ExprInfo: hir.ExprInfo{
			// this should be safe? since place. type == rightexpr.type
			ResultType: place.Type(),
		},
		Target: place,
		Value:  rightExpr,
	}, true
}

func (a *Analyzer) checkPlace(expr parser.Expression) (hir.Place, bool) {
	if identExpr, ok := expr.(*parser.IdentifierExpression); ok {
		found, ok := a.findIdentifier(identExpr)
		if !ok {
			// diag emitted by findidentifier
			return nil, false
		}
		switch found := found.(type) {
		case *hir.LocalRef:
			return &hir.LocalPlace{
				ExprInfo: found.ExprInfo,
				ID:       found.ID,
			}, true
		case *hir.GlobalRef:
			return &hir.GlobalPlace{
				ExprInfo: found.ExprInfo,
				ID:       found.ID,
			}, true
		}
	} else if derefExpr, ok := expr.(*parser.DereferenceExpression); ok {
		expr, ok := a.checkExpr(derefExpr.Var)
		if !ok {
			return nil, false
		}
		t := expr.Type()
		if t.Pointer == 0 {
			a.appendDiagnostic(derefExpr.Position(), "cannot dereference a non pointer")
			return nil, false
		}
		t.Pointer--
		return &hir.DerefPlace{
			ExprInfo: hir.ExprInfo{ResultType: t},
			Pointer:  expr,
		}, true
	} else if fieldExpr, ok := expr.(*parser.InfixExpression); ok && fieldExpr.Operator == "." {
		base, ok := a.checkPlace(fieldExpr.Left)
		if !ok {
			return nil, false
		}
		idx, ok := a.resolveField(base.Type(), fieldExpr)
		if !ok {
			return nil, false
		}
		field := a.structs[base.Type().Struct].Fields[idx]
		return &hir.FieldPlace{
			ExprInfo:   hir.ExprInfo{ResultType: field.Type},
			Base:       base,
			FieldIndex: idx,
		}, true
	}

	a.appendDiagnostic(expr.Position(), "cannot address lhs of assignment")
	return nil, false
}

func (a *Analyzer) checkCall(callExpr *parser.CallExpression) (*hir.Call, bool) {
	fncName := callExpr.Function.Value
	fnc, ok := a.resolveSymbol(fncName)
	if !ok {
		a.appendDiagnostic(callExpr.Position(), "could not find symbol `%s`.", fncName)
		return nil, false
	}

	if _, ok := fnc.(hir.FunctionID); !ok {
		a.appendDiagnostic(callExpr.Position(), "`%s` is not a function.", fncName)
		return nil, false
	}

	fncNode := a.functions[int(fnc.(hir.FunctionID))]
	if len(callExpr.Params) != len(fncNode.Parameters) {
		a.appendDiagnostic(callExpr.Position(), "function `%s` expects %d arguments, got %d.", fncName, len(fncNode.Parameters), len(callExpr.Params))
		return nil, false
	}

	var args []hir.Expr
	badParam := false
	for i, p := range callExpr.Params {
		hirExpr, ok := a.checkExpr(p)
		if !ok {
			a.appendDiagnostic(p.Position(), "argument %d (`%s`): invalid expression", i+1, fncNode.Parameters[i].Name)
			badParam = true
			continue
		}
		if hirExpr.Type() != fncNode.Parameters[i].Type {
			a.appendDiagnostic(p.Position(), "argument %d (`%s`): expected `%s`, got `%s`", i+1, fncNode.Parameters[i].Name, fncNode.Parameters[i].Type, hirExpr.Type())
			badParam = true
			continue
		}
		args = append(args, hirExpr)
	}

	if badParam {
		return nil, false
	}

	if (fncNode.ReturnType.Base != hir.Void || fncNode.ReturnType.Pointer != 0) && !a.isSized(fncNode.ReturnType, make(map[hir.StructID]struct{})) {
		a.appendDiagnostic(callExpr.Position(), "cannot call function with unsized return type.")
		return nil, false
	}

	return &hir.Call{
		ExprInfo: hir.ExprInfo{
			ResultType: fncNode.ReturnType,
		},
		Function: fncNode.Id,
		Args:     args,
	}, true
}

func (a *Analyzer) resolveSymbol(name string) (hir.Symbol, bool) {
	for scope := a.currentScope; scope != nil; scope = scope.Parent {
		if symbol, ok := scope.Symbols[name]; ok {
			return symbol, true
		}
	}
	return nil, false
}

func (a *Analyzer) findIdentifier(identExpr *parser.IdentifierExpression) (hir.Expr, bool) {
	identifier := identExpr.Value

	s, ok := a.resolveSymbol(identifier)
	if ok {
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
	switch expr := expr.(type) {
	case *hir.IntegerLiteral, *hir.FloatLiteral,
		*hir.BooleanLiteral, *hir.StringLiteral:
		return true
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
