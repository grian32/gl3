package llvm

import (
	"fmt"
	"gl3/hir"

	llvmapi "tinygo.org/x/go-llvm"
)

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
		lowered = e.context.DoubleType()
	case hir.Float32:
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
