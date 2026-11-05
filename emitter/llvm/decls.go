package llvm

import (
	"gl3/hir"

	llvmapi "tinygo.org/x/go-llvm"
)

func (e *Emitter) declareStruct(s *hir.Struct) error {
	e.structs[s.Id] = e.context.StructCreateNamed(s.Name)
	return nil
}

func (e *Emitter) defineStruct(s *hir.Struct) error {
	fieldTypes := []llvmapi.Type{}
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
	name := function.Name
	if name == "main" {
		name = "gl3.main"
	}
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

	fn := llvmapi.AddFunction(e.module, name, llvmapi.FunctionType(loweredRet, paramTypes, function.Variadic))
	// a private extern is still defined elsewhere, so it has to stay external
	if function.Private && !function.External {
		fn.SetLinkage(llvmapi.InternalLinkage)
	}
	e.functions[function.Id] = fn
	return nil
}

func (e *Emitter) declareGlobal(global *hir.Global) error {
	lowered, err := e.lowerType(global.Type)
	if err != nil {
		return err
	}
	g := llvmapi.AddGlobal(e.module, lowered, global.Name)
	g.SetGlobalConstant(global.Constant)
	// only global const is visible to importers, so everything else stays local to its module
	if !global.Constant {
		g.SetLinkage(llvmapi.InternalLinkage)
	}

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
