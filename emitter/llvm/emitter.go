package llvm

import (
	"errors"
	"fmt"
	"gl3/hir"
	"strings"
	"sync"

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

	arrNew  hir.FunctionID
	arrPush hir.FunctionID

	containsVariadic bool
	vaStart          llvmapi.Value
	vaEnd            llvmapi.Value
	vaList           llvmapi.Type
	currVaList       llvmapi.Value
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

	var arrNew, arrPush hir.FunctionID
	for _, f := range program.Functions {
		switch f.Name {
		case "arr_new":
			arrNew = f.Id
		case "arr_push":
			arrPush = f.Id
		}
	}

	return &Emitter{
		program:          program,
		context:          context,
		module:           module,
		builder:          context.NewBuilder(),
		target:           machine,
		targetData:       data,
		optimization:     optimization,
		containsVariadic: program.ContainsVariadic,
		structs:          make([]llvmapi.Type, len(program.Structs)),
		functions:        make([]llvmapi.Value, len(program.Functions)),
		globals:          make([]llvmapi.Value, len(program.Globals)),
		arrNew:           arrNew,
		arrPush:          arrPush,
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
	if e.containsVariadic {
		vaType := llvmapi.FunctionType(e.context.VoidType(), []llvmapi.Type{llvmapi.PointerType(e.context.Int8Type(), 0)}, false)
		e.vaEnd = llvmapi.AddFunction(e.module, "llvm.va_end.p0", vaType)
		e.vaStart = llvmapi.AddFunction(e.module, "llvm.va_start.p0", vaType)

		triple := e.module.Target()
		ptr := llvmapi.PointerType(e.context.Int8Type(), 0)
		i32 := e.context.Int32Type()

		// targets not listed use a plain pointer
		vaListFields := []llvmapi.Type{ptr}
		if !strings.Contains(triple, "windows") && !strings.Contains(triple, "apple") && !strings.Contains(triple, "darwin") {
			switch {
			case strings.HasPrefix(triple, "x86_64"):
				vaListFields = []llvmapi.Type{i32, i32, ptr, ptr}
			case strings.HasPrefix(triple, "aarch64"), strings.HasPrefix(triple, "arm64"):
				vaListFields = []llvmapi.Type{ptr, ptr, ptr, i32, i32}
			}
		}

		e.vaList = e.context.StructCreateNamed("va_list")
		e.vaList.StructSetBody(vaListFields, false)
	}

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
	if mainFn := e.module.NamedFunction("gl3.main"); !mainFn.IsNil() {
		wrapperMain := llvmapi.AddFunction(e.module, "main", llvmapi.FunctionType(e.context.Int32Type(), nil, false))
		block := llvmapi.AddBasicBlock(wrapperMain, "body")
		e.builder.SetInsertPointAtEnd(block)
		call := e.builder.CreateCall(mainFn.GlobalValueType(), mainFn, nil, "")
		if mainFn.GlobalValueType().ReturnType().TypeKind() == llvmapi.VoidTypeKind {
			e.builder.CreateRet(llvmapi.ConstInt(e.context.Int32Type(), 0, false))
			return nil
		}
		if strings.Contains(e.module.Target(), "windows") {
			call = e.builder.CreateTrunc(call, e.context.Int8Type(), "")
			call = e.builder.CreateZExt(call, e.context.Int32Type(), "")
		}
		e.builder.CreateRet(call)
	}

	return nil
}
