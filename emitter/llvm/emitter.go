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
	defer data.Dispose()

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
		e.declareFunction(&f)
	}

	for _, g := range e.program.Globals {
		err := e.declareGlobal(&g)
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
	e.globals[global.Id] = llvmapi.AddGlobal(e.module, lowered, global.Name)
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
			return false, nil
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
	case *hir.While:
	case *hir.Break:
	case *hir.Continue:
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
	case *hir.Unary:
	case *hir.Assignment:
	case *hir.AddressOf:
	case *hir.Dereference:
	case *hir.FieldAccess:
	case *hir.StructLiteral:
	case *hir.ArrayLiteral:
	case *hir.Sizeof:
	}
	panic("emitter: emitExpr not implemented")
}

func (e *Emitter) emitPlace(place hir.Place) (llvmapi.Value, error) {
	switch place := place.(type) {
	case *hir.LocalPlace:
		return e.locals[place.ID], nil
	case *hir.GlobalPlace:
	case *hir.DerefPlace:
	case *hir.FieldPlace:
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
	case hir.Int8, hir.Char, hir.Uint8:
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
