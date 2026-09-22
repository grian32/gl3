package cli

import (
	"embed"
	"errors"
	"fmt"
	"maps"
	"os"
	"path/filepath"

	"gl3/hir"
	"gl3/lexer"
	"gl3/parser"
	"gl3/sema"
)

type BuildOpts struct {
	Dbg         bool
	NoExecBuild bool
	Shared      bool
	Output      string
	O1          bool
	O2          bool
	O3          bool
}

func RunBuildCmd(builtinFs embed.FS, files []string, opts *BuildOpts) error {
	if opts.O1 && opts.O2 || opts.O1 && opts.O3 || opts.O2 && opts.O3 {
		return errors.New("multiple optimization level arguments not allowed, please use either --O1, --O2, --O3")
	}

	ctx := &buildContext{
		compiled:  make(map[string]*compiledModule),
		compiling: make(map[string]bool),
	}

	for _, file := range files {
		if _, err := ctx.compileGl3File(file); err != nil {
			return err
		}
	}

	return nil
}

type compiledModule struct {
	Program   *hir.Program
	Interface moduleInterface
}

type moduleInterface struct {
	SupportingStructs []supportingStruct
	ExportedStructs   []hir.StructID
	ExportedFunctions []hir.Function
	ExportedGlobals   []hir.Global
}

type structOrigin struct {
	path string
	id   hir.StructID
}

type supportingStruct struct {
	decl   hir.Struct
	origin structOrigin
}

type moduleImports struct {
	structs   []hir.Struct
	origins   []structOrigin
	structIDs map[structOrigin]hir.StructID
	functions []hir.Function
	globals   []hir.Global
	symbols   map[string]hir.Symbol
}

func newModuleImports() *moduleImports {
	return &moduleImports{
		structIDs: make(map[structOrigin]hir.StructID),
		symbols:   make(map[string]hir.Symbol),
	}
}

type buildContext struct {
	compiled  map[string]*compiledModule
	compiling map[string]bool
}

func (ctx *buildContext) compileGl3File(filePath string) (*compiledModule, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, err
	}

	if module, ok := ctx.compiled[absPath]; ok {
		return module, nil
	}
	if ctx.compiling[absPath] {
		return nil, fmt.Errorf("import cycle involving %s", absPath)
	}
	ctx.compiling[absPath] = true
	defer delete(ctx.compiling, absPath)

	input, err := os.ReadFile(absPath)
	if err != nil {
		return nil, err
	}

	l := lexer.New(string(input))
	p := parser.New(l)
	program := p.ParseProgram()
	if len(p.Errors) != 0 {
		for _, parseErr := range p.Errors {
			fmt.Printf("%s:%s\n", absPath, &parseErr)
		}
		return nil, fmt.Errorf("%s: found parser errors", absPath)
	}

	imports := newModuleImports()

	for _, stmt := range program.Statements {
		imp, ok := stmt.(*parser.ImportStatement)
		if !ok {
			continue
		}

		importPath := filepath.Join(filepath.Dir(absPath), imp.Path)

		compModule, err := ctx.compileGl3File(importPath)
		if err != nil {
			return nil, fmt.Errorf("%s:%d:%d: import %q: %w", absPath, imp.Position().StartLine, imp.Position().StartCol, imp.Path, err)
		}
		if err := imports.add(compModule.Interface); err != nil {
			return nil, fmt.Errorf("%s:%d:%d: import %q: %w", absPath, imp.Position().StartLine, imp.Position().StartCol, imp.Path, err)
		}
	}

	localStructStart := len(imports.structs)
	localFunctionStart := len(imports.functions)
	localGlobalStart := len(imports.globals)

	anal := sema.New()
	anal.Structs = imports.structs
	anal.Functions = imports.functions
	anal.Globals = imports.globals
	anal.Symbols = imports.symbols
	analyzed, diagnostics := anal.Analyze(program)
	if len(diagnostics) != 0 {
		for _, d := range diagnostics {
			fmt.Printf("%s:%d:%d: %s\n", absPath, d.Position.StartLine, d.Position.StartCol, d.Message)
		}
		return nil, fmt.Errorf("%s: found compiler errors", absPath)
	}
	// TODO: emitter

	module := &compiledModule{Program: analyzed}
	for i, decl := range analyzed.Structs {
		if !decl.Private {
			origin := structOrigin{path: absPath, id: decl.Id}
			if i < localStructStart {
				origin = imports.origins[i]
			}
			module.Interface.SupportingStructs = append(module.Interface.SupportingStructs, supportingStruct{decl: decl, origin: origin})
		}
	}
	for i := localStructStart; i < len(analyzed.Structs); i++ {
		decl := analyzed.Structs[i]
		if !decl.Private {
			module.Interface.ExportedStructs = append(module.Interface.ExportedStructs, decl.Id)
		}
	}
	for i := localFunctionStart; i < len(analyzed.Functions); i++ {
		decl := analyzed.Functions[i]
		if !decl.Private {
			module.Interface.ExportedFunctions = append(module.Interface.ExportedFunctions, decl)
		}
	}
	for i := localGlobalStart; i < len(analyzed.Globals); i++ {
		decl := analyzed.Globals[i]
		if decl.Constant {
			module.Interface.ExportedGlobals = append(module.Interface.ExportedGlobals, decl)
		}
	}

	ctx.compiled[absPath] = module
	return module, nil
}

func (imports *moduleImports) add(iface moduleInterface) error {
	ids, err := imports.addStructs(iface.SupportingStructs)
	if err != nil {
		return err
	}

	for _, sourceID := range iface.ExportedStructs {
		id, ok := ids[sourceID]
		if !ok {
			return fmt.Errorf("exported struct has invalid id %d", sourceID)
		}
		if err := imports.addSymbol(imports.structs[id].Name, id); err != nil {
			return err
		}
	}
	if err := imports.addFunctions(iface.ExportedFunctions, ids); err != nil {
		return err
	}
	return imports.addGlobals(iface.ExportedGlobals, ids)
}

func (imports *moduleImports) addStructs(supporting []supportingStruct) (map[hir.StructID]hir.StructID, error) {
	ids := make(map[hir.StructID]hir.StructID, len(supporting))
	var added []hir.Struct

	for _, item := range supporting {
		decl := item.decl
		if id, exists := imports.structIDs[item.origin]; exists {
			ids[decl.Id] = id
			continue
		}

		id := hir.StructID(len(imports.structs))
		ids[decl.Id] = id
		imports.structIDs[item.origin] = id
		imports.origins = append(imports.origins, item.origin)
		added = append(added, decl)

		decl.Id = id
		decl.Fields = nil
		decl.FieldNames = maps.Clone(decl.FieldNames)
		imports.structs = append(imports.structs, decl)
	}

	// All IDs must exist before remapping fields, including self references.
	for _, decl := range added {
		fields := make([]hir.TypedName, len(decl.Fields))
		for i, field := range decl.Fields {
			fieldType, err := remapImportedType(field.Type, ids)
			if err != nil {
				return nil, err
			}
			fields[i] = hir.TypedName{Name: field.Name, Type: fieldType}
		}
		imports.structs[ids[decl.Id]].Fields = fields
	}
	return ids, nil
}

func (imports *moduleImports) addFunctions(functions []hir.Function, structIDs map[hir.StructID]hir.StructID) error {
	for _, source := range functions {
		returnType, err := remapImportedType(source.ReturnType, structIDs)
		if err != nil {
			return err
		}
		parameters := make([]hir.TypedName, len(source.Parameters))
		for i, parameter := range source.Parameters {
			parameterType, err := remapImportedType(parameter.Type, structIDs)
			if err != nil {
				return err
			}
			parameters[i] = hir.TypedName{Name: parameter.Name, Type: parameterType}
		}

		id := hir.FunctionID(len(imports.functions))
		imports.functions = append(imports.functions, hir.Function{
			Name:           source.Name,
			Id:             id,
			Parameters:     parameters,
			ParameterNames: maps.Clone(source.ParameterNames),
			ReturnType:     returnType,
			External:       true,
		})
		if err := imports.addSymbol(source.Name, id); err != nil {
			return err
		}
	}
	return nil
}

func (imports *moduleImports) addGlobals(globals []hir.Global, structIDs map[hir.StructID]hir.StructID) error {
	for _, source := range globals {
		globalType, err := remapImportedType(source.Type, structIDs)
		if err != nil {
			return err
		}
		id := hir.GlobalID(len(imports.globals))
		imports.globals = append(imports.globals, hir.Global{
			Name:     source.Name,
			Id:       id,
			Constant: source.Constant,
			Type:     globalType,
		})
		if err := imports.addSymbol(source.Name, id); err != nil {
			return err
		}
	}
	return nil
}

func (imports *moduleImports) addSymbol(name string, symbol hir.Symbol) error {
	if _, exists := imports.symbols[name]; exists {
		return fmt.Errorf("duplicate imported symbol `%s`", name)
	}
	imports.symbols[name] = symbol
	return nil
}

func remapImportedType(t hir.Type, ids map[hir.StructID]hir.StructID) (hir.Type, error) {
	if t.Base != hir.StructType {
		return t, nil
	}
	id, ok := ids[t.Struct]
	if !ok {
		return hir.Type{}, fmt.Errorf("imported type refers to invalid struct id %d", t.Struct)
	}
	t.Struct = id
	return t, nil
}
