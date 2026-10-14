package sema

import (
	"gl3/hir"
	"gl3/parser"
	"gl3/util"
)

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

	fncNode := a.Functions[int(fnc.(hir.FunctionID))]
	if fncNode.Variadic {
		if len(callExpr.Params) < len(fncNode.Parameters) {
			a.appendDiagnostic(callExpr.Position(), "function `%s` expects at least %d arguments, got %d.", fncName, len(fncNode.Parameters), len(callExpr.Params))
			return nil, false
		}
		a.containsVariadic = true
	} else if len(callExpr.Params) != len(fncNode.Parameters) {
		a.appendDiagnostic(callExpr.Position(), "function `%s` expects %d arguments, got %d.", fncName, len(fncNode.Parameters), len(callExpr.Params))
		return nil, false
	}
	var args []hir.Expr
	badParam := false
	for i, p := range callExpr.Params {
		hirExpr, ok := a.checkExpr(p)
		if !ok {
			a.appendDiagnostic(p.Position(), "argument %d: invalid expression", i+1)
			badParam = true
			continue
		}
		if i < len(fncNode.Parameters) {
			hirExpr = coerce(hirExpr, fncNode.Parameters[i].Type)
			if hirExpr.Type() != fncNode.Parameters[i].Type {
				a.appendDiagnostic(p.Position(), "argument %d (`%s`): expected `%s`, got `%s`", i+1, fncNode.Parameters[i].Name, fncNode.Parameters[i].Type, hirExpr.Type())
				badParam = true
				continue
			}
		} else if !a.checkVariadicArg(p.Position(), i, hirExpr.Type()) {
			badParam = true
			continue
		} else {
			hirExpr = promoteVariadicArg(hirExpr)
		}
		args = append(args, hirExpr)
	}

	if badParam {
		return nil, false
	}

	if (fncNode.ReturnType.Base != hir.Void || fncNode.ReturnType.Pointer != 0) && !a.isSized(fncNode.ReturnType) {
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

// checkVariadicArg rejects extra arguments that have no type to pass through `...`.
func (a *Analyzer) checkVariadicArg(pos *util.Position, i int, t hir.Type) bool {
	switch {
	case t.Base == hir.Null:
		a.appendDiagnostic(pos, "argument %d: nullptr needs a pointer type from its context", i+1)
	case t.Base == hir.Void && t.Pointer == 0:
		a.appendDiagnostic(pos, "argument %d: cannot pass `none` as a variadic argument", i+1)
	case t.Base == hir.StructType && t.Pointer == 0:
		a.appendDiagnostic(pos, "argument %d: cannot pass struct value `%s` as a variadic argument", i+1, t)
	default:
		return true
	}
	return false
}

// promoteVariadicArg applies C's default argument promotions to an extra argument.
func promoteVariadicArg(expr hir.Expr) hir.Expr {
	t := expr.Type()
	if t.Pointer != 0 {
		return expr
	}
	switch t.Base {
	case hir.Int8, hir.Int16:
		return &hir.Cast{ExprInfo: hir.Info(hir.Int32), Kind: hir.SignExtend, Value: expr}
	case hir.Bool, hir.Uint8, hir.Uint16:
		return &hir.Cast{ExprInfo: hir.Info(hir.Int32), Kind: hir.ZeroExtend, Value: expr}
	case hir.Float32:
		return &hir.Cast{ExprInfo: hir.Info(hir.Float), Kind: hir.FloatExtend, Value: expr}
	}
	return expr
}

// demoteVararg reads the promoted type of t and truncates it back to t.
func demoteVararg(t hir.Type) hir.Expr {
	if t.Pointer == 0 {
		switch t.Base {
		case hir.Bool, hir.Uint8, hir.Uint16, hir.Int8, hir.Int16:
			return &hir.Cast{ExprInfo: hir.ExprInfo{ResultType: t}, Kind: hir.Truncate, Value: &hir.Vararg{ExprInfo: hir.Info(hir.Int32)}}
		case hir.Float32:
			return &hir.Cast{ExprInfo: hir.ExprInfo{ResultType: t}, Kind: hir.FloatTruncate, Value: &hir.Vararg{ExprInfo: hir.Info(hir.Float)}}
		}
	}
	return &hir.Vararg{ExprInfo: hir.ExprInfo{ResultType: t}}
}

// checkVararg reads the promoted type and narrows it back to the requested one.
func (a *Analyzer) checkVararg(expr *parser.VarargExpression) (hir.Expr, bool) {
	if a.currentFunction == nil || !a.currentFunction.Variadic {
		a.appendDiagnostic(expr.Position(), "vararg can only be used in a variadic function")
		return nil, false
	}
	t, ok := hir.ConvertVarType(expr.Type, a.Symbols)
	if !ok {
		a.appendDiagnostic(expr.Position(), "invalid type for vararg")
		return nil, false
	}
	switch {
	case t.Base == hir.Void && t.Pointer == 0:
		a.appendDiagnostic(expr.Position(), "cannot read `none` as a vararg")
		return nil, false
	case t.Base == hir.StructType && t.Pointer == 0:
		a.appendDiagnostic(expr.Position(), "cannot read struct value `%s` as a vararg", t)
		return nil, false
	}

	return demoteVararg(t), true
}
