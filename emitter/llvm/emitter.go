package llvm

import (
	"errors"
	"fmt"
	"gl3/hir"
	"sync"

	"tinygo.org/x/go-llvm"
	llvmapi "tinygo.org/x/go-llvm"
)

type Emitter struct {
	program      *hir.Program
	context      llvmapi.Context
	module       llvmapi.Module
	builder      llvmapi.Builder
	target       llvmapi.TargetMachine
	targetData   llvmapi.TargetData
	optimization int

	structs   []llvmapi.Type
	functions []llvmapi.Value
	globals   []llvmapi.Value

	locals []llvmapi.Value
	loops  []loopBlocks
}

type loopBlocks struct {
	condition llvmapi.BasicBlock
	exit      llvmapi.BasicBlock
}

var ErrNotImplemented = errors.New("LLVM emission is not implemented yet")

var nativeTargetOnce sync.Once
var nativeTargetError error

func New(program *hir.Program, moduleName string, optimization int) (*Emitter, error) {
	if optimization < 0 || optimization > 3 {
		return nil, fmt.Errorf("invalid optimization level %d", optimization)
	}
	nativeTargetOnce.Do(func() {
		if err := llvmapi.InitializeNativeTarget(); err != nil {
			nativeTargetError = err
			return
		}
		nativeTargetError = llvmapi.InitializeNativeAsmPrinter()
	})

	if nativeTargetError != nil {
		return nil, fmt.Errorf("initialize LLVM target: %w", nativeTargetError)
	}

	triple := llvmapi.DefaultTargetTriple()
	target, err := llvmapi.GetTargetFromTriple(triple)
	if err != nil {
		return nil, fmt.Errorf("LLVM target %q: %w", triple, err)
	}

	level := llvmapi.CodeGenLevelNone
	switch optimization {
	case 1:
		level = llvmapi.CodeGenLevelLess
	case 2:
		level = llvmapi.CodeGenLevelDefault
	case 3:
		level = llvmapi.CodeGenLevelAggressive
	}

	machine := target.CreateTargetMachine(triple, "generic", "", level, llvmapi.RelocPIC, llvmapi.CodeModelDefault)
	if machine.C == nil {
		return nil, fmt.Errorf("create LLVM target machine for %q", triple)
	}

	data := machine.CreateTargetData()

	context := llvmapi.NewContext()
	module := context.NewModule(moduleName)
	module.SetTarget(triple)
	module.SetDataLayout(data.String())

	return &Emitter{
		program:      program,
		context:      context,
		module:       module,
		builder:      context.NewBuilder(),
		target:       machine,
		targetData:   data,
		optimization: optimization,
		structs:      make([]llvmapi.Type, len(program.Structs)),
		functions:    make([]llvmapi.Value, len(program.Functions)),
		globals:      make([]llvmapi.Value, len(program.Globals)),
	}, nil
}

func (e *Emitter) Close() {
	e.builder.Dispose()
	e.module.Dispose()
	e.context.Dispose()
	e.target.Dispose()
	e.targetData.Dispose()
}

func (e *Emitter) Module() llvmapi.Module {
	return e.module
}

func (e *Emitter) Emit() error {
	for _, s := range e.program.Structs {
		err := e.declareStruct(&s)
		if err != nil {
			return err
		}
	}

	for _, f := range e.program.Functions {
		err := e.declareFunction(&f)
		if err != nil {
			return err
		}
	}

	for _, s := range e.program.Structs {
		if !s.Opaque {
			err := e.defineStruct(&s)
			if err != nil {
				return err
			}
		}
	}

	for _, g := range e.program.Globals {
		err := e.declareGlobal(&g)
		if err != nil {
			return err
		}
	}

	for _, f := range e.program.Functions {
		if !f.External {
			err := e.emitFunction(&f)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

func (e *Emitter) declareStruct(s *hir.Struct) error {
	e.structs[s.Id] = e.context.StructCreateNamed(s.Name)
	return nil
}

func (e *Emitter) defineStruct(s *hir.Struct) error {
	fieldTypes := []llvm.Type{}
	for _, f := range s.Fields {
		lowered, err := e.lowerType(f.Type)
		if err != nil {
			return err
		}
		fieldTypes = append(fieldTypes, lowered)
	}
	e.structs[s.Id].StructSetBody(fieldTypes, false)
	return nil
}

func (e *Emitter) declareFunction(function *hir.Function) error {
	paramTypes := []llvmapi.Type{}
	for _, p := range function.Parameters {
		lowered, err := e.lowerType(p.Type)
		if err != nil {
			return err
		}
		paramTypes = append(paramTypes, lowered)
	}

	loweredRet, err := e.lowerType(function.ReturnType)
	if err != nil {
		return err
	}

	e.functions[function.Id] = llvmapi.AddFunction(e.module, function.Name, llvmapi.FunctionType(loweredRet, paramTypes, false))
	return nil
}

func (e *Emitter) declareGlobal(global *hir.Global) error {
	lowered, err := e.lowerType(global.Type)
	if err != nil {
		return err
	}
	g := llvmapi.AddGlobal(e.module, lowered, global.Name)
	g.SetGlobalConstant(global.Constant)

	if global.Initializer != nil {
		init, err := e.emitExpr(global.Initializer)
		if err != nil {
			return err
		}
		g.SetInitializer(init)
	}

	e.globals[global.Id] = g

	return nil
}

func (e *Emitter) emitFunction(function *hir.Function) error {
	f := e.functions[function.Id]
	block := llvmapi.AddBasicBlock(f, "body")
	e.builder.SetInsertPointAtEnd(block)

	e.locals = make([]llvmapi.Value, len(function.Locals))
	for i, local := range function.Locals {
		t, err := e.lowerType(local.Type)
		if err != nil {
			return err
		}
		e.locals[i] = e.builder.CreateAlloca(t, local.Name)
	}

	for i := range function.Parameters {
		e.builder.CreateStore(f.Param(i), e.locals[i])
	}

	fallsThrough, err := e.emitBlock(&function.Body)
	if err != nil {
		return err
	}

	if fallsThrough && function.ReturnType.Base == hir.Void && function.ReturnType.Pointer == 0 {
		e.builder.CreateRetVoid()
	}

	return nil
}

func (e *Emitter) emitBlock(block *hir.Block) (bool, error) {
	for _, stmt := range block.Statements {
		fallsThrough, err := e.emitStmt(stmt)
		if err != nil || !fallsThrough {
			return fallsThrough, err
		}
	}

	return true, nil
}

func (e *Emitter) emitStmt(stmt hir.Stmt) (bool, error) {
	switch stmt := stmt.(type) {
	case *hir.LocalDeclaration:
		value, err := e.emitExpr(stmt.Initializer)
		if err != nil {
			return false, err
		}
		e.builder.CreateStore(value, e.locals[stmt.ID])
		return true, nil
	case *hir.Return:
		if stmt.Value == nil {
			e.builder.CreateRetVoid()
			return false, nil
		}
		value, err := e.emitExpr(stmt.Value)
		if err != nil {
			return false, err
		}
		e.builder.CreateRet(value)
		return false, nil
	case *hir.ExpressionStatement:
		_, err := e.emitExpr(stmt.Expr)
		if err != nil {
			return false, err
		}
		return true, nil
	case *hir.If:
		cond, err := e.emitExpr(stmt.Condition)
		if err != nil {
			return false, err
		}
		f := e.builder.GetInsertBlock().Parent()
		thenBlock := llvmapi.AddBasicBlock(f, "if.then")
		endBlock := llvmapi.AddBasicBlock(f, "if.end")
		falseBlock := endBlock
		if stmt.Else != nil {
			falseBlock = llvmapi.AddBasicBlock(f, "if.else")
		}
		e.builder.CreateCondBr(cond, thenBlock, falseBlock)

		e.builder.SetInsertPointAtEnd(thenBlock)
		thenFalls, err := e.emitBlock(&stmt.Then)
		if err != nil {
			return false, err
		}
		if thenFalls {
			e.builder.CreateBr(endBlock)
		}

		if stmt.Else != nil {
			e.builder.SetInsertPointAtEnd(falseBlock)
			elseFalls, err := e.emitBlock(stmt.Else)
			if err != nil {
				return false, err
			}
			if elseFalls {
				e.builder.CreateBr(endBlock)
			}
			if !thenFalls && !elseFalls {
				endBlock.EraseFromParent()
				return false, nil
			}
		}

		e.builder.SetInsertPointAtEnd(endBlock)
		return true, nil
	case *hir.While:
		f := e.builder.GetInsertBlock().Parent()
		cond := llvmapi.AddBasicBlock(f, "while.cond")
		body := llvmapi.AddBasicBlock(f, "while.body")
		end := llvmapi.AddBasicBlock(f, "while.end")

		e.builder.CreateBr(cond)

		e.builder.SetInsertPointAtEnd(cond)
		cExpr, err := e.emitExpr(stmt.Condition)
		if err != nil {
			return false, err
		}
		e.builder.CreateCondBr(cExpr, body, end)

		e.loops = append(e.loops, loopBlocks{
			condition: cond,
			exit:      end,
		})
		e.builder.SetInsertPointAtEnd(body)
		falls, err := e.emitBlock(&stmt.Body)
		if err != nil {
			return false, err
		}
		if falls {
			e.builder.CreateBr(cond)
		}
		e.loops = e.loops[:len(e.loops)-1]
		e.builder.SetInsertPointAtEnd(end)

		return true, nil
	case *hir.Break:
		e.builder.CreateBr(e.loops[len(e.loops)-1].exit)
		return false, nil
	case *hir.Continue:
		e.builder.CreateBr(e.loops[len(e.loops)-1].condition)
		return false, nil
	}
	panic("emitter: emitStmt not implemented")
}

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
		return llvmapi.ConstFloat(e.context.FloatType(), float64(expr.Value)), nil
	case *hir.StringLiteral:
		data := e.context.ConstString(expr.Value, false)
		global := llvmapi.AddGlobal(e.module, data.Type(), ".str")
		global.SetInitializer(data)
		global.SetGlobalConstant(true)
		global.SetUnnamedAddr(true)

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
		panic("emitter: ArrayLiteral not implemented until the stdlib is ready")
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
	}
	panic("emitter: emitExpr not implemented")
}

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

func (e *Emitter) lowerType(t hir.Type) (llvmapi.Type, error) {
	var lowered llvmapi.Type
	switch t.Base {
	case hir.Int, hir.Uint:
		lowered = e.context.Int64Type()
	case hir.Int32, hir.Uint32:
		lowered = e.context.Int32Type()
	case hir.Int16, hir.Uint16:
		lowered = e.context.Int16Type()
	case hir.Int8, hir.Uint8:
		lowered = e.context.Int8Type()
	case hir.Bool:
		lowered = e.context.Int1Type()
	case hir.Float:
		lowered = e.context.FloatType()
	case hir.Void:
		lowered = e.context.VoidType()
	case hir.StructType:
		if int(t.Struct) >= len(e.structs) {
			return llvmapi.Type{}, fmt.Errorf("unknown struct ID %d", t.Struct)
		}
		lowered = e.structs[t.Struct]
		if lowered.IsNil() {
			return llvmapi.Type{}, fmt.Errorf("struct %q has not been declared", e.program.Structs[t.Struct].Name)
		}
	default:
		return llvmapi.Type{}, fmt.Errorf("unsupported HIR type %s", t)
	}

	if t.Pointer != 0 {
		if t.Base == hir.Void {
			lowered = e.context.Int8Type()
		}
		return llvmapi.PointerType(lowered, 0), nil
	}
	return lowered, nil
}
