package sema

import (
	"gl3/hir"
	"testing"
)

var scopeTests = []struct {
	name        string
	source      string
	want        hir.Program
	symbols     map[string]hir.Symbol
	diagnostics []expectedDiagnostic
}{
	{
		name: "parameters and locals resolve to local IDs",
		source: `fnc copy(int32 input) -> int32 {
    def int32 output = input
    return output
}`,
		want: hir.Program{Functions: []hir.Function{{
			Name: "copy", Id: 0,
			Parameters:     []hir.TypedName{{Name: "input", Type: hir.Type{Base: hir.Int32}}},
			ParameterNames: map[string]int{"input": 0},
			ReturnType:     hir.Type{Base: hir.Int32},
			Locals: []hir.Local{
				{Name: "input", Type: hir.Type{Base: hir.Int32}},
				{Name: "output", Type: hir.Type{Base: hir.Int32}},
			},
			Body: hir.Block{Statements: []hir.Stmt{
				&hir.LocalDeclaration{ID: 1, Initializer: &hir.LocalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 0}},
				&hir.Return{Value: &hir.LocalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 1}},
			}},
		}}},
		symbols: map[string]hir.Symbol{"copy": hir.FunctionID(0)},
	},
	{
		name: "local initializer resolves outer name before shadowing",
		source: `global int32 value = 7i32
fnc copy() -> int32 {
    def int32 value = value
    return value
}
fnc read() -> int32 { return value }`,
		want: hir.Program{
			Globals: []hir.Global{{Name: "value", Id: 0, Type: hir.Type{Base: hir.Int32},
				Initializer: &hir.IntegerLiteral{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, Value: 7}}},
			Functions: []hir.Function{
				{Name: "copy", Id: 0, ReturnType: hir.Type{Base: hir.Int32},
					Locals: []hir.Local{{Name: "value", Type: hir.Type{Base: hir.Int32}}},
					Body: hir.Block{Statements: []hir.Stmt{
						&hir.LocalDeclaration{ID: 0, Initializer: &hir.GlobalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 0}},
						&hir.Return{Value: &hir.LocalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 0}},
					}}},
				{Name: "read", Id: 1, ReturnType: hir.Type{Base: hir.Int32},
					Body: hir.Block{Statements: []hir.Stmt{
						&hir.Return{Value: &hir.GlobalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 0}},
					}}},
			},
		},
		symbols: map[string]hir.Symbol{"value": hir.GlobalID(0), "copy": hir.FunctionID(0), "read": hir.FunctionID(1)},
	},
	{
		name: "parameter names and IDs are function local",
		source: `fnc first(int32 value) -> int32 { return value }
fnc second(bool value) -> bool { return value }`,
		want: hir.Program{Functions: []hir.Function{
			{Name: "first", Id: 0,
				Parameters:     []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}},
				ParameterNames: map[string]int{"value": 0}, ReturnType: hir.Type{Base: hir.Int32},
				Locals: []hir.Local{{Name: "value", Type: hir.Type{Base: hir.Int32}}},
				Body: hir.Block{Statements: []hir.Stmt{
					&hir.Return{Value: &hir.LocalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Int32}}, ID: 0}},
				}}},
			{Name: "second", Id: 1,
				Parameters:     []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Bool}}},
				ParameterNames: map[string]int{"value": 0}, ReturnType: hir.Type{Base: hir.Bool},
				Locals: []hir.Local{{Name: "value", Type: hir.Type{Base: hir.Bool}}},
				Body: hir.Block{Statements: []hir.Stmt{
					&hir.Return{Value: &hir.LocalRef{ExprInfo: hir.ExprInfo{ResultType: hir.Type{Base: hir.Bool}}, ID: 0}},
				}}},
		}},
		symbols: map[string]hir.Symbol{"first": hir.FunctionID(0), "second": hir.FunctionID(1)},
	},
	{
		name: "local names cannot leak between functions",
		source: `fnc first() -> int32 {
    def int32 hidden = 1i32
    return hidden
}
fnc second() -> int32 {
    return hidden
}`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "hidden"}, line: 6}},
	},
	{
		name: "parameter names cannot leak between functions",
		source: `fnc first(int32 hidden) -> int32 { return hidden }
fnc second() -> int32 { return hidden }`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "hidden"}, line: 2}},
	},
	{
		name: "local is unavailable in its own initializer",
		source: `fnc sample() -> int32 {
    def int32 value = value
    return 0i32
}`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "value"}, line: 2}},
	},
	{
		name: "local is unavailable before its declaration",
		source: `fnc sample() -> int32 {
    def int32 first = later
    def int32 later = 1i32
    return 0i32
}`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "later"}, line: 2}},
	},
	{
		name: "duplicate locals in one scope",
		source: `fnc sample() -> int32 {
    def int32 value = 1i32
    def int32 value = 2i32
    return 0i32
}`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "value"}, line: 3}},
	},
}

func TestAnalyzeScopes(t *testing.T) {
	for _, test := range scopeTests {
		t.Run(test.name, func(t *testing.T) {
			program := parseDeclarations(t, test.source)
			c := New()
			got, diagnostics := c.Analyze(program)
			assertDiagnostics(t, diagnostics, test.diagnostics)
			if len(test.diagnostics) != 0 {
				return
			}
			assertDeclarations(t, got, &test.want)
			assertSymbols(t, c.symbols, test.symbols)
		})
	}
}
