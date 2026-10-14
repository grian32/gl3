package sema

import (
	"gl3/hir"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
)

func (a *Analyzer) checkExpr(expr parser.Expression) (hir.Expr, bool) {
	switch expr := expr.(type) {
	case *parser.IntegerLiteral:
		return a.convertIntLiteral(expr, false)
	case *parser.PrefixExpression:
		return a.checkPrefix(expr)
	case *parser.NullptrLiteral:
		return &hir.NullPointer{ExprInfo: hir.Info(hir.Null)}, true
	case *parser.BooleanExpression:
		return &hir.BooleanLiteral{
			ExprInfo: hir.Info(hir.Bool),
			Value:    expr.Value,
		}, true
	case *parser.StringLiteral:
		return &hir.StringLiteral{
			ExprInfo: hir.InfoPtr(hir.Int8, 1),
			Value:    expr.Value,
		}, true
	case *parser.FloatLiteral:
		return &hir.FloatLiteral{
			ExprInfo: hir.Info(hir.ConvertBaseType(expr.Type.Base)),
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
	case *parser.ArrayLiteral:
		return a.checkArrayLiteral(expr)
	case *parser.CastExpression:
		return a.checkCast(expr)
	case *parser.SizeofExpression:
		return a.checkSizeof(expr)
	case *parser.VarargExpression:
		return a.checkVararg(expr)
	}
	return nil, false
}

func (a *Analyzer) checkArrayLiteral(expr *parser.ArrayLiteral) (*hir.ArrayLiteral, bool) {
	elementType, ok := hir.ConvertVarType(expr.Type, a.Symbols)
	if !ok {
		a.appendDiagnostic(expr.Position(), "invalid array element type `%s`", expr.Type)
		return nil, false
	}
	if !a.isSized(elementType) {
		a.appendDiagnostic(expr.Position(), "array element type `%s` must be sized", elementType)
		return nil, false
	}

	var items []hir.Expr
	itemsOk := true
	for i, item := range expr.Items {
		value, ok := a.checkExpr(item)
		if !ok {
			itemsOk = false
			continue
		}
		value = coerce(value, elementType)
		if value.Type() != elementType {
			a.appendDiagnostic(item.Position(), "array element %d: expected `%s`, got `%s`", i+1, elementType, value.Type())
			itemsOk = false
			continue
		}
		items = append(items, value)
	}
	if !itemsOk {
		return nil, false
	}
	none := hir.Type{Base: hir.Void, Pointer: 1}
	if !a.checkArraysFunction(expr, "arr_new", []hir.Type{{Base: hir.Uint}}, none) {
		return nil, false
	}
	if !a.checkArraysFunction(expr, "arr_push", []hir.Type{none, none}, hir.Type{Base: hir.Void}) {
		return nil, false
	}

	resultType := elementType
	resultType.Pointer++
	return &hir.ArrayLiteral{
		ExprInfo:    hir.ExprInfo{ResultType: resultType},
		ElementType: elementType,
		Items:       items,
	}, true
}

func (a *Analyzer) checkArraysFunction(expr *parser.ArrayLiteral, name string, params []hir.Type, ret hir.Type) bool {
	symbol, ok := a.Symbols[name]
	id, isFunction := symbol.(hir.FunctionID)
	if !ok || !isFunction {
		a.appendDiagnostic(expr.Position(), "array literals require import \"arrays\" (could not find `%s`)", name)
		return false
	}
	fn := a.Functions[id]
	matches := fn.ReturnType == ret && len(fn.Parameters) == len(params) && !fn.Variadic
	for i := 0; matches && i < len(params); i++ {
		matches = fn.Parameters[i].Type == params[i]
	}
	if !matches {
		a.appendDiagnostic(expr.Position(), "array literals need `%s` from the arrays stdlib, but `%s` has a different signature", name, name)
		return false
	}
	return true
}

func (a *Analyzer) checkCast(expr *parser.CastExpression) (*hir.Cast, bool) {
	value, ok := a.checkExpr(expr.Expr)
	if !ok {
		return nil, false
	}
	target, ok := hir.ConvertVarType(expr.Type, a.Symbols)
	if !ok {
		a.appendDiagnostic(expr.Position(), "invalid cast target type `%s`", expr.Type)
		return nil, false
	}
	source := value.Type()
	if (target.Base == hir.Void && target.Pointer == 0) || (source.Base == hir.Void && source.Pointer == 0) {
		a.appendDiagnostic(expr.Position(), "cannot cast from `%s` to `%s`", source, target)
		return nil, false
	}

	sourceBits, sourceSigned := castIntegerInfo(source)
	targetBits, targetSigned := castIntegerInfo(target)
	kind := hir.InvalidCastKind
	switch {
	case source == target:
		kind = hir.IdentityCast
	case source.Base == hir.Bool && source.Pointer == 0 && targetBits != 0:
		kind = hir.ZeroExtend
	case source.Pointer > 0 && target.Pointer > 0:
		kind = hir.PointerCast
	case source.Pointer > 0 && targetBits != 0:
		kind = hir.PointerToInt
	case sourceBits != 0 && target.Pointer > 0:
		kind = hir.IntToPointer
	case sourceBits != 0 && targetBits != 0:
		switch {
		case sourceBits == targetBits:
			kind = hir.IdentityCast
		case sourceBits > targetBits:
			kind = hir.Truncate
		case sourceSigned:
			kind = hir.SignExtend
		default:
			kind = hir.ZeroExtend
		}
	case isFloat(source) && isFloat(target):
		kind = hir.FloatTruncate
		if target.Base == hir.Float {
			kind = hir.FloatExtend
		}
	case sourceBits != 0 && isFloat(target):
		kind = hir.UnsignedIntToFloat
		if sourceSigned {
			kind = hir.SignedIntToFloat
		}
	case isFloat(source) && targetBits != 0:
		kind = hir.FloatToUnsignedInt
		if targetSigned {
			kind = hir.FloatToSignedInt
		}
	}
	if kind == hir.InvalidCastKind {
		a.appendDiagnostic(expr.Position(), "cannot cast from `%s` to `%s`", source, target)
		return nil, false
	}
	return &hir.Cast{
		ExprInfo: hir.ExprInfo{ResultType: target},
		Kind:     kind,
		Value:    value,
	}, true
}

func isFloat(t hir.Type) bool {
	return t.Pointer == 0 && (t.Base == hir.Float || t.Base == hir.Float32)
}

func castIntegerInfo(t hir.Type) (bits uint8, signed bool) {
	if t.Pointer != 0 {
		return 0, false
	}
	switch t.Base {
	case hir.Int:
		return 64, true
	case hir.Int32:
		return 32, true
	case hir.Int16:
		return 16, true
	case hir.Int8:
		return 8, true
	case hir.Uint:
		return 64, false
	case hir.Uint32:
		return 32, false
	case hir.Uint16:
		return 16, false
	case hir.Uint8:
		return 8, false
	}
	return 0, false
}

func (a *Analyzer) checkSizeof(expr *parser.SizeofExpression) (*hir.Sizeof, bool) {
	t, ok := hir.ConvertVarType(expr.Type, a.Symbols)
	if !ok {
		a.appendDiagnostic(expr.Position(), "invalid type for sizeof")
		return nil, false
	}
	if !a.isSized(t) {
		a.appendDiagnostic(expr.Position(), "sizeof unsized type is not allowed")
		return nil, false
	}

	return &hir.Sizeof{
		ExprInfo:    hir.Info(hir.Uint),
		OperandType: t,
	}, true
}

func (a *Analyzer) checkStructLiteral(expr *parser.StructInitializationExpression) (*hir.StructLiteral, bool) {
	symbol, ok := a.Symbols[expr.Name]
	if !ok {
		a.appendDiagnostic(expr.Position(), "struct `%s` does not exist", expr.Name)
		return nil, false
	}
	structId, ok := symbol.(hir.StructID)
	if !ok {
		a.appendDiagnostic(expr.Position(), "`%s` is not struct", expr.Name)
		return nil, false
	}
	s := a.Structs[structId]
	if s.Unsized {
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
		fieldExpr = coerce(fieldExpr, s.Fields[i].Type)
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

	if !a.isSized(t) {
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
			return &hir.FloatLiteral{ExprInfo: hir.Info(hir.ConvertBaseType(right.Type.Base)), Value: -right.Value}, true
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
			case hir.Float, hir.Float32:
				op = hir.FloatNegate
			}
		case "!":
			if t.Base == hir.Bool {
				op = hir.BoolNot
			}
		case "~":
			if bits, _ := castIntegerInfo(t); bits != 0 {
				op = hir.IntNot
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
		a.appendDiagnostic(expr.Position(), "cannot access field on non-struct type `%s`", a.typeName(baseType))
		return 0, false
	}
	s := a.Structs[baseType.Struct]
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
					ResultType: a.Globals[s].Type,
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
