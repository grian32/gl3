package llvm

import (
	"errors"
	"fmt"
	"gl3/hir"
	"sync"

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
	return ErrNotImplemented
}

func (e *Emitter) declareStruct(s *hir.Struct) error {
	panic("emitter: declareStructs not implemented")
}

func (e *Emitter) defineStruct(s *hir.Struct) error {
	panic("emitter: defineStructs not implemented")
}

func (e *Emitter) declareFunction(function *hir.Function) error {
	panic("emitter: declareFunctions not implemented")
}

func (e *Emitter) declareGlobal(global *hir.Global) error {
	panic("emitter: declareGlobals not implemented")
}

func (e *Emitter) emitFunction(function *hir.Function) error {
	panic("emitter: emitFunction not implemented")
}

func (e *Emitter) emitBlock(block *hir.Block) error {
	panic("emitter: emitBlock not implemented")
}

func (e *Emitter) emitStmt(stmt hir.Stmt) error {
	panic("emitter: emitStmt not implemented")
}

func (e *Emitter) emitExpr(expr hir.Expr) (llvmapi.Value, error) {
	panic("emitter: emitExpr not implemented")
}

func (e *Emitter) emitPlace(place hir.Place) (llvmapi.Value, error) {
	panic("emitter: emitPlace not implemented")
}

func (e *Emitter) lowerType(t hir.Type) (llvmapi.Type, error) {
	panic("emitter: lowerType not implemented")
}
