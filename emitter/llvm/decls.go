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

	e.functions[function.Id] = llvmapi.AddFunction(e.module, function.Name, llvmapi.FunctionType(loweredRet, paramTypes, function.Variadic))
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
