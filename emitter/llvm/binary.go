package llvm

import (
	"fmt"
	"gl3/hir"

	llvmapi "tinygo.org/x/go-llvm"
)

func (e *Emitter) emitBinary(expr *hir.Binary) (llvmapi.Value, error) {
	if expr.Op == hir.BoolAnd || expr.Op == hir.BoolOr {
		return e.emitShortCircuit(expr)
	}

	l, err := e.emitExpr(expr.Left)
	if err != nil {
		return llvmapi.Value{}, err
	}
	r, err := e.emitExpr(expr.Right)
	if err != nil {
		return llvmapi.Value{}, err
	}

	return e.emitBinaryOp(expr.Op, l, r, expr.Left.Type(), expr.Right.Type())
}

func (e *Emitter) emitBinaryOp(op hir.BinaryOp, l, r llvmapi.Value, leftType, rightType hir.Type) (llvmapi.Value, error) {
	switch op {
	case hir.IntAdd:
		return e.builder.CreateAdd(l, r, ""), nil
	case hir.IntSubtract:
		return e.builder.CreateSub(l, r, ""), nil
	case hir.IntMultiply:
		return e.builder.CreateMul(l, r, ""), nil
	case hir.SignedDivide:
		return e.builder.CreateSDiv(l, r, ""), nil
	case hir.UnsignedDivide:
		return e.builder.CreateUDiv(l, r, ""), nil
	case hir.SignedRemainder:
		return e.builder.CreateSRem(l, r, ""), nil
	case hir.UnsignedRemainder:
		return e.builder.CreateURem(l, r, ""), nil
	case hir.IntAnd:
		return e.builder.CreateAnd(l, r, ""), nil
	case hir.IntOr:
		return e.builder.CreateOr(l, r, ""), nil
	case hir.IntXor:
		return e.builder.CreateXor(l, r, ""), nil
	case hir.ShiftLeft, hir.ArithmeticShiftRight, hir.LogicalShiftRight:
		// shifting by bit width or more is bad in llvm cuz poison s othe amount is maskd to width-1 as x86 does in hw
		mask := llvmapi.ConstInt(r.Type(), uint64(r.Type().IntTypeWidth()-1), false)
		r = e.builder.CreateAnd(r, mask, "")
		switch op {
		case hir.ShiftLeft:
			return e.builder.CreateShl(l, r, ""), nil
		case hir.ArithmeticShiftRight:
			return e.builder.CreateAShr(l, r, ""), nil
		default:
			return e.builder.CreateLShr(l, r, ""), nil
		}
	case hir.FloatAdd:
		return e.builder.CreateFAdd(l, r, ""), nil
	case hir.FloatSubtract:
		return e.builder.CreateFSub(l, r, ""), nil
	case hir.FloatMultiply:
		return e.builder.CreateFMul(l, r, ""), nil
	case hir.FloatDivide:
		return e.builder.CreateFDiv(l, r, ""), nil
	case hir.IntEqual, hir.BoolEqual:
		return e.builder.CreateICmp(llvmapi.IntEQ, l, r, ""), nil
	case hir.IntNotEqual, hir.BoolNotEqual:
		return e.builder.CreateICmp(llvmapi.IntNE, l, r, ""), nil
	case hir.SignedLess:
		return e.builder.CreateICmp(llvmapi.IntSLT, l, r, ""), nil
	case hir.SignedLessEqual:
		return e.builder.CreateICmp(llvmapi.IntSLE, l, r, ""), nil
	case hir.SignedGreater:
		return e.builder.CreateICmp(llvmapi.IntSGT, l, r, ""), nil
	case hir.SignedGreaterEqual:
		return e.builder.CreateICmp(llvmapi.IntSGE, l, r, ""), nil
	case hir.UnsignedLess:
		return e.builder.CreateICmp(llvmapi.IntULT, l, r, ""), nil
	case hir.UnsignedLessEqual:
		return e.builder.CreateICmp(llvmapi.IntULE, l, r, ""), nil
	case hir.UnsignedGreater:
		return e.builder.CreateICmp(llvmapi.IntUGT, l, r, ""), nil
	case hir.UnsignedGreaterEqual:
		return e.builder.CreateICmp(llvmapi.IntUGE, l, r, ""), nil
	// NaN compares unequal to everything, so != is the one unordered predicate.
	case hir.FloatEqual:
		return e.builder.CreateFCmp(llvmapi.FloatOEQ, l, r, ""), nil
	case hir.FloatNotEqual:
		return e.builder.CreateFCmp(llvmapi.FloatUNE, l, r, ""), nil
	case hir.FloatLess:
		return e.builder.CreateFCmp(llvmapi.FloatOLT, l, r, ""), nil
	case hir.FloatLessEqual:
		return e.builder.CreateFCmp(llvmapi.FloatOLE, l, r, ""), nil
	case hir.FloatGreater:
		return e.builder.CreateFCmp(llvmapi.FloatOGT, l, r, ""), nil
	case hir.FloatGreaterEqual:
		return e.builder.CreateFCmp(llvmapi.FloatOGE, l, r, ""), nil
	case hir.PointerAdd, hir.PointerSubtract:
		elementType := leftType
		elementType.Pointer--
		element, err := e.lowerType(elementType)
		if err != nil {
			return llvmapi.Value{}, err
		}
		// GEP sign-extends narrow indices, so unsigned offsets are widened first.
		offset := r
		if r.Type().IntTypeWidth() < 64 {
			switch rightType.Base {
			case hir.Uint32, hir.Uint16, hir.Uint8:
				offset = e.builder.CreateZExt(r, e.context.Int64Type(), "")
			default:
				offset = e.builder.CreateSExt(r, e.context.Int64Type(), "")
			}
		}
		if op == hir.PointerSubtract {
			offset = e.builder.CreateNeg(offset, "")
		}
		return e.builder.CreateGEP(element, l, []llvmapi.Value{offset}, ""), nil
	default:
		panic(fmt.Sprintf("unexpected hir.BinaryOp: %#v", op))
	}
}

func (e *Emitter) emitShortCircuit(expr *hir.Binary) (llvmapi.Value, error) {
	// basically follows clang here
	l, err := e.emitExpr(expr.Left)
	if err != nil {
		return llvmapi.Value{}, err
	}
	leftEnd := e.builder.GetInsertBlock()
	f := leftEnd.Parent()
	rhs := llvmapi.AddBasicBlock(f, "logic.rhs")
	end := llvmapi.AddBasicBlock(f, "logic.end")

	// && skips the right side when the left is false, || when it is true.
	var skipped uint64
	if expr.Op == hir.BoolAnd {
		e.builder.CreateCondBr(l, rhs, end)
	} else {
		skipped = 1
		e.builder.CreateCondBr(l, end, rhs)
	}

	e.builder.SetInsertPointAtEnd(rhs)
	r, err := e.emitExpr(expr.Right)
	if err != nil {
		return llvmapi.Value{}, err
	}
	// The right side may have added blocks of its own.
	rightEnd := e.builder.GetInsertBlock()
	e.builder.CreateBr(end)

	e.builder.SetInsertPointAtEnd(end)
	phi := e.builder.CreatePHI(e.context.Int1Type(), "")
	phi.AddIncoming(
		[]llvmapi.Value{llvmapi.ConstInt(e.context.Int1Type(), skipped, false), r},
		[]llvmapi.BasicBlock{leftEnd, rightEnd},
	)
	return phi, nil
}
