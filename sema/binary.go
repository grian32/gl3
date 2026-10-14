package sema

import (
	"gl3/hir"
	"gl3/parser"
	"gl3/util"
)

func resolveNullptr(expr hir.Expr, want hir.Type) {
	if null, ok := expr.(*hir.NullPointer); ok && null.ResultType.Base == hir.Null && want.Pointer > 0 {
		null.ResultType = want
	}
}

// coerce applies the implicit conversions allowed wherever a value flows into a slot of type want:
// nullptr takes its type from want, and any pointer converts to none*.
func coerce(expr hir.Expr, want hir.Type) hir.Expr {
	resolveNullptr(expr, want)
	have := expr.Type()
	if want == (hir.Type{Base: hir.Void, Pointer: 1}) && have.Pointer > 0 && have != want {
		return &hir.Cast{ExprInfo: hir.ExprInfo{ResultType: want}, Kind: hir.PointerCast, Value: expr}
	}
	return expr
}

func (a *Analyzer) checkBinaryOp(infixExpr *parser.InfixExpression) (hir.Expr, bool) {
	if infixExpr.Operator == "." {
		left, ok := a.checkExpr(infixExpr.Left)
		if !ok {
			return nil, false
		}
		if left.Type().Base == hir.StructType && left.Type().Pointer == 1 {
			pointee := left.Type()
			pointee.Pointer--
			left = &hir.Dereference{ExprInfo: hir.ExprInfo{ResultType: pointee}, Pointer: left}
		}
		idx, ok := a.resolveField(left.Type(), infixExpr)
		if !ok {
			return nil, false
		}
		field := a.Structs[left.Type().Struct].Fields[idx]

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
	resolveNullptr(leftExpr, rightExpr.Type())
	resolveNullptr(rightExpr, leftExpr.Type())
	op, rightExpr, resultType, ok := a.binaryOp(infixExpr.Position(), infixExpr.Right.Position(), infixExpr.Operator, leftExpr.Type(), rightExpr)
	if !ok {
		return nil, false
	}
	return makeBinaryNode(op, leftExpr, rightExpr, resultType), true
}

// returns hir op, right expr, result type, success
func (a *Analyzer) binaryOp(pos, rightPos *util.Position, operator string, leftType hir.Type, rightExpr hir.Expr) (hir.BinaryOp, hir.Expr, hir.Type, bool) {
	boolType := hir.Type{Base: hir.Bool}

	if leftType.Pointer > 0 {
		// ptr eq ne
		if operator == "==" || operator == "!=" {
			if leftType != rightExpr.Type() {
				a.appendDiagnostic(pos, "types of operands cannot be different: left is `%s`, right is `%s`", leftType, rightExpr.Type())
				return hir.InvalidBinaryOp, nil, hir.Type{}, false
			}
			op := hir.IntEqual
			if operator == "!=" {
				op = hir.IntNotEqual
			}
			return op, rightExpr, boolType, true
		}

		op := hir.PointerAdd
		if operator == "-" {
			op = hir.PointerSubtract
		} else if operator != "+" {
			a.appendDiagnostic(pos, "unsupported op `%s` on pointer type `%s`", operator, leftType)
			return hir.InvalidBinaryOp, nil, hir.Type{}, false
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
			a.appendDiagnostic(rightPos, "pointer arithmetic requires an integer offset, got `%s`", offsetType)
			return hir.InvalidBinaryOp, nil, hir.Type{}, false
		}

		elementType := leftType
		elementType.Pointer--
		if !a.isSized(elementType) {
			if elementType.Base == hir.StructType && a.Structs[elementType.Struct].Opaque {
				a.appendDiagnostic(pos, "pointer arithmetic requires a sized element type; struct `%s` is opaque", a.Structs[elementType.Struct].Name)
			} else {
				a.appendDiagnostic(pos, "pointer arithmetic requires a sized element type, got `%s`", elementType)
			}
			return hir.InvalidBinaryOp, nil, hir.Type{}, false
		}
		return op, rightExpr, leftType, true
	}

	if operator == "<<" || operator == ">>" {
		return a.checkShift(pos, operator, leftType, rightExpr)
	}

	if leftType != rightExpr.Type() {
		a.appendDiagnostic(pos, "types of operands cannot be different: left is `%s`, right is `%s`", leftType, rightExpr.Type())
		return hir.InvalidBinaryOp, nil, hir.Type{}, false
	}

	switch leftType.Base {
	case hir.Int, hir.Int32, hir.Int16, hir.Int8,
		hir.Uint, hir.Uint32, hir.Uint16, hir.Uint8:
		unsigned := false
		switch leftType.Base {
		case hir.Uint, hir.Uint32, hir.Uint16, hir.Uint8:
			unsigned = true
		}
		switch operator {
		case "+":
			return hir.IntAdd, rightExpr, leftType, true
		case "-":
			return hir.IntSubtract, rightExpr, leftType, true
		case "*":
			return hir.IntMultiply, rightExpr, leftType, true
		case "/":
			op := hir.SignedDivide
			if unsigned {
				op = hir.UnsignedDivide
			}
			return op, rightExpr, leftType, true
		case "%":
			op := hir.SignedRemainder
			if unsigned {
				op = hir.UnsignedRemainder
			}
			return op, rightExpr, leftType, true
		case "&":
			return hir.IntAnd, rightExpr, leftType, true
		case "|":
			return hir.IntOr, rightExpr, leftType, true
		case "^":
			return hir.IntXor, rightExpr, leftType, true
		case "==":
			return hir.IntEqual, rightExpr, boolType, true
		case "!=":
			return hir.IntNotEqual, rightExpr, boolType, true
		case "<":
			op := hir.SignedLess
			if unsigned {
				op = hir.UnsignedLess
			}
			return op, rightExpr, boolType, true
		case "<=":
			op := hir.SignedLessEqual
			if unsigned {
				op = hir.UnsignedLessEqual
			}
			return op, rightExpr, boolType, true
		case ">":
			op := hir.SignedGreater
			if unsigned {
				op = hir.UnsignedGreater
			}
			return op, rightExpr, boolType, true
		case ">=":
			op := hir.SignedGreaterEqual
			if unsigned {
				op = hir.UnsignedGreaterEqual
			}
			return op, rightExpr, boolType, true
		}
	case hir.Float, hir.Float32:
		switch operator {
		case "+":
			return hir.FloatAdd, rightExpr, leftType, true
		case "-":
			return hir.FloatSubtract, rightExpr, leftType, true
		case "*":
			return hir.FloatMultiply, rightExpr, leftType, true
		case "/":
			return hir.FloatDivide, rightExpr, leftType, true
		case "==":
			return hir.FloatEqual, rightExpr, boolType, true
		case "!=":
			return hir.FloatNotEqual, rightExpr, boolType, true
		case "<":
			return hir.FloatLess, rightExpr, boolType, true
		case "<=":
			return hir.FloatLessEqual, rightExpr, boolType, true
		case ">":
			return hir.FloatGreater, rightExpr, boolType, true
		case ">=":
			return hir.FloatGreaterEqual, rightExpr, boolType, true
		}
	case hir.Bool:
		switch operator {
		case "&&":
			return hir.BoolAnd, rightExpr, leftType, true
		case "||":
			return hir.BoolOr, rightExpr, leftType, true
		case "==":
			return hir.BoolEqual, rightExpr, leftType, true
		case "!=":
			return hir.BoolNotEqual, rightExpr, leftType, true
		}
	}

	a.appendDiagnostic(pos, "unsupported op `%s` on types `%s` and `%s`.", operator, leftType, rightExpr.Type())
	return hir.InvalidBinaryOp, nil, hir.Type{}, false
}

// converts shift type to same operand as left for llvm
func (a *Analyzer) checkShift(pos *util.Position, operator string, leftType hir.Type, rightExpr hir.Expr) (hir.BinaryOp, hir.Expr, hir.Type, bool) {
	valueBits, valueSigned := castIntegerInfo(leftType)
	amountBits, amountSigned := castIntegerInfo(rightExpr.Type())
	if valueBits == 0 || amountBits == 0 {
		a.appendDiagnostic(pos, "unsupported op `%s` on types `%s` and `%s`.", operator, leftType, rightExpr.Type())
		return hir.InvalidBinaryOp, nil, hir.Type{}, false
	}

	amount := rightExpr
	if amountBits != valueBits {
		kind := hir.ZeroExtend
		if amountBits > valueBits {
			kind = hir.Truncate
		} else if amountSigned {
			kind = hir.SignExtend
		}
		amount = &hir.Cast{ExprInfo: hir.ExprInfo{ResultType: leftType}, Kind: kind, Value: rightExpr}
	}

	op := hir.ShiftLeft
	if operator == ">>" {
		op = hir.LogicalShiftRight
		if valueSigned {
			op = hir.ArithmeticShiftRight
		}
	}
	return op, amount, leftType, true
}

func makeBinaryNode(op hir.BinaryOp, left, right hir.Expr, resultType hir.Type) *hir.Binary {
	return &hir.Binary{
		ExprInfo: hir.ExprInfo{ResultType: resultType},
		Op:       op,
		Left:     left,
		Right:    right,
	}
}

func (a *Analyzer) checkAssignment(assignExpr *parser.AssignmentExpression) (hir.Expr, bool) {
	place, ok := a.checkPlace(assignExpr.Left)
	if !ok {
		return nil, false
	}
	if !a.isSized(place.Type()) {
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
	if gp, ok := root.(*hir.GlobalPlace); ok && a.Globals[gp.ID].Constant {
		a.appendDiagnostic(assignExpr.Position(), "modifying constant globals is not permitted.")
		return nil, false
	}

	rightExpr, ok := a.checkExpr(assignExpr.Right)
	if !ok {
		a.appendDiagnostic(assignExpr.Position(), "invalid expr on rhs of assignment.")
		return nil, false
	}

	if assignExpr.Operator != "" {
		op, rightExpr, resultType, ok := a.binaryOp(assignExpr.Position(), assignExpr.Right.Position(), assignExpr.Operator, place.Type(), rightExpr)
		if !ok {
			return nil, false
		}
		if resultType != place.Type() {
			a.appendDiagnostic(assignExpr.Position(), "result of `%s` is `%s`, cannot assign it to `%s`.", assignExpr.Operator, resultType, place.Type())
			return nil, false
		}
		return &hir.CompoundAssignment{
			ExprInfo: hir.ExprInfo{ResultType: place.Type()},
			Target:   place,
			Op:       op,
			Value:    rightExpr,
		}, true
	}

	rightExpr = coerce(rightExpr, place.Type())
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
		left, ok := a.checkExpr(fieldExpr.Left)
		if !ok {
			return nil, false
		}
		var base hir.Place
		if left.Type().Base == hir.StructType && left.Type().Pointer == 1 {
			pointee := left.Type()
			pointee.Pointer--
			base = &hir.DerefPlace{ExprInfo: hir.ExprInfo{ResultType: pointee}, Pointer: left}
		} else if base, ok = a.checkPlace(fieldExpr.Left); !ok {
			return nil, false
		}
		idx, ok := a.resolveField(base.Type(), fieldExpr)
		if !ok {
			return nil, false
		}
		field := a.Structs[base.Type().Struct].Fields[idx]
		return &hir.FieldPlace{
			ExprInfo:   hir.ExprInfo{ResultType: field.Type},
			Base:       base,
			FieldIndex: idx,
		}, true
	}

	a.appendDiagnostic(expr.Position(), "cannot address lhs of assignment")
	return nil, false
}
