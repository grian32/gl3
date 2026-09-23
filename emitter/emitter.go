package emitter

import (
	"errors"
	"fmt"
	"gl3/hir"
	"sync"

	llvm "tinygo.org/x/go-llvm"
)

type Emitter struct {
	program      *hir.Program
	context      llvm.Context
	module       llvm.Module
	builder      llvm.Builder
	target       llvm.TargetMachine
	optimization int

	structs   []llvm.Type
	functions []llvm.Value
	globals   []llvm.Value

	locals []llvm.Value
	loops  []loopBlocks
}

type loopBlocks struct {
	condition llvm.BasicBlock
	exit      llvm.BasicBlock
}

var ErrNotImplemented = errors.New("LLVM emission is not implemented yet")

var nativeTargetOnce sync.Once
var nativeTargetError error

func New(program *hir.Program, moduleName string, optimization int) (*Emitter, error) {
	if optimization < 0 || optimization > 3 {
		return nil, fmt.Errorf("invalid optimization level %d", optimization)
	}
	nativeTargetOnce.Do(func() {
		if err := llvm.InitializeNativeTarget(); err != nil {
			nativeTargetError = err
			return
		}
		nativeTargetError = llvm.InitializeNativeAsmPrinter()
	})
	if nativeTargetError != nil {
		return nil, fmt.Errorf("initialize LLVM target: %w", nativeTargetError)
	}

	triple := llvm.DefaultTargetTriple()
	target, err := llvm.GetTargetFromTriple(triple)
	if err != nil {
		return nil, fmt.Errorf("LLVM target %q: %w", triple, err)
	}
	level := llvm.CodeGenLevelNone
	switch optimization {
	case 1:
		level = llvm.CodeGenLevelLess
	case 2:
		level = llvm.CodeGenLevelDefault
	case 3:
		level = llvm.CodeGenLevelAggressive
	}
	machine := target.CreateTargetMachine(triple, "generic", "", level, llvm.RelocPIC, llvm.CodeModelDefault)
	if machine.C == nil {
		return nil, fmt.Errorf("create LLVM target machine for %q", triple)
	}
	data := machine.CreateTargetData()
	defer data.Dispose()

	context := llvm.NewContext()
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
		structs:      make([]llvm.Type, len(program.Structs)),
		functions:    make([]llvm.Value, len(program.Functions)),
		globals:      make([]llvm.Value, len(program.Globals)),
	}, nil
}

func (e *Emitter) Close() {
	e.builder.Dispose()
	e.module.Dispose()
	e.context.Dispose()
	e.target.Dispose()
}

func (e *Emitter) Module() llvm.Module {
	return e.module
}

func (e *Emitter) Emit() error {
	return ErrNotImplemented
}

func (e *Emitter) declareStructs() error {
	panic("emitter: declareStructs not implemented")
}

func (e *Emitter) defineStructs() error {
	panic("emitter: defineStructs not implemented")
}

func (e *Emitter) declareFunctions() error {
	panic("emitter: declareFunctions not implemented")
}

func (e *Emitter) declareGlobals() error {
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

func (e *Emitter) emitExpr(expr hir.Expr) (llvm.Value, error) {
	panic("emitter: emitExpr not implemented")
}

func (e *Emitter) emitPlace(place hir.Place) (llvm.Value, error) {
	panic("emitter: emitPlace not implemented")
}

func (e *Emitter) lowerType(t hir.Type) (llvm.Type, error) {
	panic("emitter: lowerType not implemented")
}
