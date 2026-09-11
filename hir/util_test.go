package hir

import (
	"gl3/lexer"
	"testing"
)

func TestConvertVarType(t *testing.T) {
	symbols := map[string]Symbol{
		"First":  StructID(0),
		"Player": StructID(3),
		"update": FunctionID(0),
		"count":  GlobalID(0),
	}
	tests := []struct {
		name  string
		input lexer.VarType
		want  Type
		ok    bool
	}{
		{"int", lexer.VarType{Base: lexer.Int}, Type{Base: Int}, true},
		{"int32", lexer.VarType{Base: lexer.Int32}, Type{Base: Int32}, true},
		{"int16", lexer.VarType{Base: lexer.Int16}, Type{Base: Int16}, true},
		{"int8", lexer.VarType{Base: lexer.Int8}, Type{Base: Int8}, true},
		{"char pointer", lexer.VarType{Base: lexer.Char, Pointer: 2}, Type{Base: Char, Pointer: 2}, true},
		{"uint", lexer.VarType{Base: lexer.Uint}, Type{Base: Uint}, true},
		{"uint32", lexer.VarType{Base: lexer.Uint32}, Type{Base: Uint32}, true},
		{"uint16", lexer.VarType{Base: lexer.Uint16}, Type{Base: Uint16}, true},
		{"uint8", lexer.VarType{Base: lexer.Uint8}, Type{Base: Uint8}, true},
		{"bool", lexer.VarType{Base: lexer.Bool}, Type{Base: Bool}, true},
		{"void", lexer.VarType{Base: lexer.Void}, Type{Base: Void}, true},
		{"float", lexer.VarType{Base: lexer.Float}, Type{Base: Float}, true},
		{"zero struct ID", lexer.VarType{IsStructType: true, StructName: "First"}, Type{Base: StructType, Struct: 0}, true},
		{"struct overrides base", lexer.VarType{Base: lexer.Int, IsStructType: true, StructName: "Player", Pointer: 2}, Type{Base: StructType, Struct: 3, Pointer: 2}, true},
		{"unknown struct", lexer.VarType{IsStructType: true, StructName: "Missing"}, Type{}, false},
		{"function is not a type", lexer.VarType{IsStructType: true, StructName: "update"}, Type{}, false},
		{"global is not a type", lexer.VarType{IsStructType: true, StructName: "count"}, Type{}, false},
		{"invalid", lexer.VarType{Base: lexer.None}, Type{}, false},
		{"unknown base", lexer.VarType{Base: lexer.BaseVarType(255)}, Type{}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, ok := ConvertVarType(test.input, symbols)
			if got != test.want || ok != test.ok {
				t.Fatalf("want (%+v, %v), got (%+v, %v)", test.want, test.ok, got, ok)
			}
		})
	}
}
