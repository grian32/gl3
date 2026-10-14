package llvm

import (
	"fmt"
	"gl3/hir"

	llvmapi "tinygo.org/x/go-llvm"
)

func (e *Emitter) emitExpr(expr hir.Expr) (llvmapi.Value, error) {
	switch expr := expr.(type) {
	case *hir.LocalRef:
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateLoad(t, e.locals[expr.ID], ""), nil
	case *hir.GlobalRef:
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateLoad(t, e.globals[expr.ID], ""), nil
	case *hir.IntegerLiteral:
		typ, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return llvmapi.ConstInt(typ, expr.Value, false), nil
	case *hir.BooleanLiteral:
		var value uint64 = 0
		if expr.Value {
			value = 1
		}
		return llvmapi.ConstInt(e.context.Int1Type(), value, false), nil
	case *hir.NullPointer:
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return llvmapi.ConstPointerNull(t), nil
	case *hir.FloatLiteral:
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return llvmapi.ConstFloat(t, expr.Value), nil
	case *hir.StringLiteral:
		data := e.context.ConstString(expr.Value, false)
		global := llvmapi.AddGlobal(e.module, data.Type(), ".str")
		global.SetInitializer(data)
		global.SetGlobalConstant(true)
		global.SetUnnamedAddr(true)
		global.SetLinkage(llvmapi.PrivateLinkage)

		zero := llvmapi.ConstInt(e.context.Int32Type(), 0, false)
		return llvmapi.ConstInBoundsGEP(data.Type(), global, []llvmapi.Value{zero, zero}), nil
	case *hir.Binary:
		return e.emitBinary(expr)
	case *hir.Call:
		f := e.functions[expr.Function]
		params := []llvmapi.Value{}
		for _, p := range expr.Args {
			pe, err := e.emitExpr(p)
			if err != nil {
				return llvmapi.Value{}, err
			}
			params = append(params, pe)
		}
		return e.builder.CreateCall(f.GlobalValueType(), f, params, ""), nil
	case *hir.Cast:
		v, err := e.emitExpr(expr.Value)
		if err != nil {
			return llvmapi.Value{}, err
		}
		t, err := e.lowerType(expr.ResultType)
		if err != nil {
			return llvmapi.Value{}, err
		}
		switch expr.Kind {
		case hir.IdentityCast, hir.PointerCast:
			return v, nil
		case hir.SignExtend:
			return e.builder.CreateSExt(v, t, ""), nil
		case hir.ZeroExtend:
			return e.builder.CreateZExt(v, t, ""), nil
		case hir.Truncate:
			return e.builder.CreateTrunc(v, t, ""), nil
		case hir.SignedIntToFloat:
			return e.builder.CreateSIToFP(v, t, ""), nil
		case hir.UnsignedIntToFloat:
			return e.builder.CreateUIToFP(v, t, ""), nil
		case hir.FloatToSignedInt:
			return e.builder.CreateFPToSI(v, t, ""), nil
		case hir.FloatToUnsignedInt:
			return e.builder.CreateFPToUI(v, t, ""), nil
		case hir.FloatExtend:
			return e.builder.CreateFPExt(v, t, ""), nil
		case hir.FloatTruncate:
			return e.builder.CreateFPTrunc(v, t, ""), nil
		case hir.PointerToInt:
			return e.builder.CreatePtrToInt(v, t, ""), nil
		case hir.IntToPointer:
			return e.builder.CreateIntToPtr(v, t, ""), nil
		default:
			panic(fmt.Sprintf("unexpected hir.CastKind: %#v", expr.Kind))
		}
	case *hir.Unary:
		v, err := e.emitExpr(expr.Value)
		if err != nil {
			return llvmapi.Value{}, err
		}
		switch expr.Op {
		case hir.BoolNot, hir.IntNot:
			return e.builder.CreateNot(v, ""), nil
		case hir.FloatNegate:
			return e.builder.CreateFNeg(v, ""), nil
		case hir.IntNegate:
			return e.builder.CreateNeg(v, ""), nil
		default:
			panic(fmt.Sprintf("unexpected hir.UnaryOp: %#v", expr.Op))
		}
	case *hir.Assignment:
		p, err := e.emitPlace(expr.Target)
		if err != nil {
			return llvmapi.Value{}, err
		}
		ex, err := e.emitExpr(expr.Value)
		if err != nil {
			return llvmapi.Value{}, err
		}
		e.builder.CreateStore(ex, p)
		return ex, nil
	case *hir.AddressOf:
		p, err := e.emitPlace(expr.Target)
		if err != nil {
			return llvmapi.Value{}, err
		}
		return p, nil
	case *hir.Dereference:
		ptr, err := e.emitExpr(expr.Pointer)
		if err != nil {
			return llvmapi.Value{}, err
		}
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateLoad(t, ptr, ""), nil
	case *hir.FieldAccess:
		base, err := e.emitExpr(expr.Base)
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateExtractValue(base, expr.FieldIndex, ""), nil
	case *hir.StructLiteral:
		t, err := e.lowerType(expr.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		// Constant fields fold to a constant struct, so this also serves global initializers.
		value := llvmapi.Undef(t)
		for i, field := range expr.Fields {
			fv, err := e.emitExpr(field)
			if err != nil {
				return llvmapi.Value{}, err
			}
			value = e.builder.CreateInsertValue(value, fv, i, "")
		}
		return value, nil
	case *hir.ArrayLiteral:
		newFnc := e.functions[e.arrNew]
		push := e.functions[e.arrPush]

		elemType, err := e.lowerType(expr.ElementType)
		if err != nil {
			return llvmapi.Value{}, err
		}
		sizeInt := llvmapi.ConstInt(e.context.Int64Type(), e.targetData.TypeAllocSize(elemType), false)
		newCall := e.builder.CreateCall(newFnc.GlobalValueType(), newFnc, []llvmapi.Value{sizeInt}, "")
		ptrType := llvmapi.PointerType(e.context.Int8Type(), 0)
		ptr := e.builder.CreateAlloca(ptrType, "")
		e.builder.CreateStore(newCall, ptr)
		elem := e.builder.CreateAlloca(elemType, "")
		for _, item := range expr.Items {
			v, err := e.emitExpr(item)
			if err != nil {
				return llvmapi.Value{}, err
			}
			e.builder.CreateStore(v, elem)
			e.builder.CreateCall(push.GlobalValueType(), push, []llvmapi.Value{ptr, elem}, "")
		}

		return e.builder.CreateLoad(ptrType, ptr, ""), nil
	case *hir.Sizeof:
		lowered, err := e.lowerType(expr.OperandType)
		if err != nil {
			return llvmapi.Value{}, err
		}
		return llvmapi.ConstInt(e.context.Int64Type(), e.targetData.TypeAllocSize(lowered), false), nil
	case *hir.CompoundAssignment:
		addr, err := e.emitPlace(expr.Target)
		if err != nil {
			return llvmapi.Value{}, err
		}
		t, err := e.lowerType(expr.Target.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		old := e.builder.CreateLoad(t, addr, "")
		rhs, err := e.emitExpr(expr.Value)
		if err != nil {
			return llvmapi.Value{}, err
		}
		result, err := e.emitBinaryOp(expr.Op, old, rhs, expr.Target.Type(), expr.Value.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		e.builder.CreateStore(result, addr)
		return result, nil
	case *hir.Vararg:
		t, err := e.lowerType(expr.ResultType)
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateVAArg(e.currVaList, t, ""), nil
	}
	panic("emitter: emitExpr not implemented")
}

func (e *Emitter) emitPlace(place hir.Place) (llvmapi.Value, error) {
	switch place := place.(type) {
	case *hir.LocalPlace:
		return e.locals[place.ID], nil
	case *hir.GlobalPlace:
		return e.globals[place.ID], nil
	case *hir.DerefPlace:
		return e.emitExpr(place.Pointer)
	case *hir.FieldPlace:
		base, err := e.emitPlace(place.Base)
		if err != nil {
			return llvmapi.Value{}, err
		}
		t, err := e.lowerType(place.Base.Type())
		if err != nil {
			return llvmapi.Value{}, err
		}
		return e.builder.CreateStructGEP(t, base, place.FieldIndex, ""), nil
	}
	panic("emitter: emitPlace not implemented")
}
