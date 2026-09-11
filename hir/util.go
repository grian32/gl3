package hir

import (
	"gl3/lexer"
	"strconv"
	"strings"
)

func (t Type) String() string {
	var name string
	switch t.Base {
	case Int:
		name = "int"
	case Int32:
		name = "int32"
	case Int16:
		name = "int16"
	case Int8:
		name = "int8"
	case Char:
		name = "char"
	case Uint:
		name = "uint"
	case Uint32:
		name = "uint32"
	case Uint16:
		name = "uint16"
	case Uint8:
		name = "uint8"
	case Bool:
		name = "bool"
	case Void:
		name = "none"
	case Float:
		name = "float"
	case StructType:
		name = "struct#" + strconv.FormatUint(uint64(t.Struct), 10)
	default:
		name = "invalid"
	}
	return name + strings.Repeat("*", int(t.Pointer))
}

func Info(base BaseType) ExprInfo {
	return ExprInfo{ResultType: Type{Base: base}}
}

func InfoPtr(base BaseType, depth uint8) ExprInfo {
	return ExprInfo{ResultType: Type{Base: base, Pointer: depth}}
}

func ConvertVarType(vt lexer.VarType, symbols map[string]Symbol) (Type, bool) {
	if vt.IsStructType {
		id, ok := symbols[vt.StructName].(StructID)
		if !ok {
			return Type{}, false
		}
		return Type{Base: StructType, Pointer: vt.Pointer, Struct: id}, true
	}

	base := ConvertBaseType(vt.Base)
	if base == Invalid {
		return Type{}, false
	}
	return Type{Base: base, Pointer: vt.Pointer}, true
}

func ConvertBaseType(bvt lexer.BaseVarType) BaseType {
	switch bvt {
	case lexer.Int:
		return Int
	case lexer.Int32:
		return Int32
	case lexer.Int16:
		return Int16
	case lexer.Int8:
		return Int8
	case lexer.Char:
		return Char
	case lexer.Uint:
		return Uint
	case lexer.Uint32:
		return Uint32
	case lexer.Uint16:
		return Uint16
	case lexer.Uint8:
		return Uint8
	case lexer.Bool:
		return Bool
	case lexer.Void:
		return Void
	case lexer.Float:
		return Float
	default:
		return Invalid
	}
}
