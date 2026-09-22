package sema

import (
	"gl3/hir"
	"gl3/lexer"
	"gl3/parser"
	"reflect"
	"strings"
	"testing"
)

type expectedDiagnostic struct {
	messageContains []string
	line            uint32
}

var declarationTests = []struct {
	name        string
	source      string
	want        hir.Program
	symbols     map[string]hir.Symbol
	diagnostics []expectedDiagnostic
}{
	{
		name: "struct sizing flags follow forward fields and preserve pointers",
		source: `struct Outer { Inner inner }
struct Inner { Handle handle }
struct PointerHolder { Outer* value }
extern struct Handle`,
		want: hir.Program{Structs: []hir.Struct{
			{Name: "Outer", Id: 0, Unsized: true, Fields: []hir.TypedName{{Name: "inner", Type: hir.Type{Base: hir.StructType, Struct: 1}}}, FieldNames: map[string]int{"inner": 0}},
			{Name: "Inner", Id: 1, Unsized: true, Fields: []hir.TypedName{{Name: "handle", Type: hir.Type{Base: hir.StructType, Struct: 3}}}, FieldNames: map[string]int{"handle": 0}},
			{Name: "PointerHolder", Id: 2, Unsized: false, Fields: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.StructType, Struct: 0, Pointer: 1}}}, FieldNames: map[string]int{"value": 0}},
			{Name: "Handle", Id: 3, Opaque: true, Unsized: true},
		}},
		symbols: map[string]hir.Symbol{"Outer": hir.StructID(0), "Inner": hir.StructID(1), "PointerHolder": hir.StructID(2), "Handle": hir.StructID(3)},
	},

	{name: "unsized nested parameter", source: `extern struct Handle
struct Inner { Handle handle }
struct Outer { Inner inner }
fnc consume(Outer value) -> none {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unsized", "parameter", "consume"}, line: 4}}},
	{name: "unsized nested extern parameter", source: `extern struct Handle
struct Wrapper { Handle handle }
extern fnc consume(Wrapper value) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unsized", "parameter", "consume"}, line: 3}}},
	{name: "unsized nested function return", source: `extern struct Handle
struct Wrapper { Handle handle }
fnc create() -> Wrapper {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"size", "create"}, line: 3}}},
	{name: "unsized return with forward field declarations", source: `fnc create() -> Outer {}
struct Outer { Inner inner }
struct Inner { Handle handle }
extern struct Handle`, diagnostics: []expectedDiagnostic{{messageContains: []string{"size", "create"}, line: 1}}},
	{name: "unsized global after field declarations", source: `extern struct Handle
struct Wrapper { Handle handle }
global Wrapper value = Wrapper:{}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"global", "value"}, line: 3}}},
	{name: "direct value cycle in parameter", source: `struct Node { Node next }
fnc consume(Node value) -> none {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unsized", "parameter", "consume"}, line: 2}}},
	{name: "indirect value cycle in parameter", source: `struct First { Second next }
struct Second { First next }
fnc consume(First value) -> none {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unsized", "parameter", "consume"}, line: 3}}},
	{name: "indirect value cycle in return", source: `struct First { Second next }
struct Second { First next }
fnc create() -> First {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"size", "create"}, line: 3}}},
	{name: "privacy of functions", source: `fnc publicBefore() -> none {}
private fnc hidden() -> none {}
fnc publicAfter() -> none {}`, want: hir.Program{Functions: []hir.Function{{Name: "publicBefore", Id: 0, Private: false, External: false, ReturnType: hir.Type{Base: hir.Void}}, {Name: "hidden", Id: 1, Private: true, External: false, ReturnType: hir.Type{Base: hir.Void}}, {Name: "publicAfter", Id: 2, Private: false, External: false, ReturnType: hir.Type{Base: hir.Void}}}}, symbols: map[string]hir.Symbol{"publicBefore": hir.FunctionID(0), "hidden": hir.FunctionID(1), "publicAfter": hir.FunctionID(2)}},
	{name: "privacy of extern functions", source: `extern fnc publicBefore() -> none
private extern fnc hidden() -> none
extern fnc publicAfter() -> none`, want: hir.Program{Functions: []hir.Function{{Name: "publicBefore", Id: 0, Private: false, External: true, ReturnType: hir.Type{Base: hir.Void}}, {Name: "hidden", Id: 1, Private: true, External: true, ReturnType: hir.Type{Base: hir.Void}}, {Name: "publicAfter", Id: 2, Private: false, External: true, ReturnType: hir.Type{Base: hir.Void}}}}, symbols: map[string]hir.Symbol{"publicBefore": hir.FunctionID(0), "hidden": hir.FunctionID(1), "publicAfter": hir.FunctionID(2)}},
	{
		name: "privacy of extern structs does not leak to regular structs",
		source: `extern struct PublicBefore
private extern struct Hidden
struct Regular { int32 value }
extern struct PublicAfter`,
		want: hir.Program{Structs: []hir.Struct{
			{Name: "PublicBefore", Id: 0, Opaque: true, Unsized: true, Private: false},
			{Name: "Hidden", Id: 1, Opaque: true, Unsized: true, Private: true},
			{Name: "Regular", Id: 2, Opaque: false, Private: false, Fields: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, FieldNames: map[string]int{"value": 0}},
			{Name: "PublicAfter", Id: 3, Opaque: true, Unsized: true, Private: false},
		}},
		symbols: map[string]hir.Symbol{"PublicBefore": hir.StructID(0), "Hidden": hir.StructID(1), "Regular": hir.StructID(2), "PublicAfter": hir.StructID(3)},
	},

	{name: "empty"},
	{
		name: "none field",
		source: `struct Item {
    none value
}`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"none", "field", "value"}, line: 2},
		},
	},
	{
		name: "none parameter",
		source: `fnc consume(
    none value
) -> none { }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"none", "param", "value"}, line: 2},
		},
	},
	{
		name:   "none global",
		source: `global none value = 0`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"none", "global", "value"}, line: 1},
		},
	},
	{
		name:   "none constant global",
		source: `global const none value = 0`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"none", "global", "value"}, line: 1},
		},
	},
	{
		name: "none return and pointer types are allowed",
		source: `struct Handle { none* pointer }
fnc consume(none** pointer) -> none { }
fnc produce() -> none* { return 0 as none* }
global none* handle = 0 as none*`,
		want: hir.Program{
			Structs: []hir.Struct{
				{Name: "Handle", Id: 0,
					Fields:     []hir.TypedName{{Name: "pointer", Type: hir.Type{Base: hir.Void, Pointer: 1}}},
					FieldNames: map[string]int{"pointer": 0}},
			},
			Functions: []hir.Function{
				{Name: "consume", Id: 0,
					Parameters:     []hir.TypedName{{Name: "pointer", Type: hir.Type{Base: hir.Void, Pointer: 2}}},
					ParameterNames: map[string]int{"pointer": 0},
					ReturnType:     hir.Type{Base: hir.Void}},
				{Name: "produce", Id: 1, ReturnType: hir.Type{Base: hir.Void, Pointer: 1}},
			},
			Globals: []hir.Global{
				{Name: "handle", Id: 0, Type: hir.Type{Base: hir.Void, Pointer: 1}},
			},
		},
		symbols: map[string]hir.Symbol{
			"Handle": hir.StructID(0), "consume": hir.FunctionID(0),
			"produce": hir.FunctionID(1), "handle": hir.GlobalID(0),
		},
	},
	{
		name:   "invalid global type",
		source: `global Missing* value = 0`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"invalid type", "global", "value"}, line: 1},
		},
	},
	{
		name: "interleaved declarations",
		source: `
global int32 first = 1i32
fnc alpha() -> int32 { return 1i32 }
struct First { int32 value }
global const int32 second = 2i32
struct Second { First* previous }
private fnc beta(int32 input) -> int32 {
    def int32 local = input
    return local
}
`,
		want: hir.Program{
			Structs: []hir.Struct{
				{Name: "First", Id: 0, Fields: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, FieldNames: map[string]int{"value": 0}},
				{Name: "Second", Id: 1, Fields: []hir.TypedName{{Name: "previous", Type: hir.Type{Base: hir.StructType, Struct: 0, Pointer: 1}}}, FieldNames: map[string]int{"previous": 0}},
			},
			Functions: []hir.Function{
				{Name: "alpha", Id: 0, ReturnType: hir.Type{Base: hir.Int32}},
				{Name: "beta", Id: 1, Private: true, Parameters: []hir.TypedName{{Name: "input", Type: hir.Type{Base: hir.Int32}}}, ParameterNames: map[string]int{"input": 0}, ReturnType: hir.Type{Base: hir.Int32}},
			},
			Globals: []hir.Global{
				{Name: "first", Id: 0, Constant: false, Type: hir.Type{Base: hir.Int32}},
				{Name: "second", Id: 1, Constant: true, Type: hir.Type{Base: hir.Int32}},
			},
		},
		symbols: map[string]hir.Symbol{
			"First": hir.StructID(0), "Second": hir.StructID(1),
			"alpha": hir.FunctionID(0), "beta": hir.FunctionID(1),
			"first": hir.GlobalID(0), "second": hir.GlobalID(1),
		},
	},
	{
		name: "imports and function locals are not global declarations",
		source: `
import "io"
fnc probe(int32 parameter) -> int32 {
    def int32 inside = parameter
    return inside
}
`,
		want: hir.Program{
			Functions: []hir.Function{{Name: "probe", Id: 0, Parameters: []hir.TypedName{{Name: "parameter", Type: hir.Type{Base: hir.Int32}}}, ParameterNames: map[string]int{"parameter": 0}, ReturnType: hir.Type{Base: hir.Int32}}},
		},
		symbols: map[string]hir.Symbol{
			"probe": hir.FunctionID(0),
		},
	},
	{
		name: "ordered members and forward types",
		source: `fnc selectNode(Node** z, uint8 a, bool ready) -> Node* { return 0 as Node* }
global Node* head = 0 as Node*
global const uint8 limit = 7u8
struct Container { Node* z float a char** text }
struct Node { int32 value Node* next }
fnc idle() -> none { }`,
		want: hir.Program{
			Structs: []hir.Struct{
				{
					Name: "Container", Id: 0,
					Fields: []hir.TypedName{
						{Name: "z", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}},
						{Name: "a", Type: hir.Type{Base: hir.Float}},
						{Name: "text", Type: hir.Type{Base: hir.Char, Pointer: 2}},
					},
					FieldNames: map[string]int{"z": 0, "a": 1, "text": 2},
				},
				{
					Name: "Node", Id: 1,
					Fields: []hir.TypedName{
						{Name: "value", Type: hir.Type{Base: hir.Int32}},
						{Name: "next", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}},
					},
					FieldNames: map[string]int{"value": 0, "next": 1},
				},
			},
			Functions: []hir.Function{
				{
					Name: "selectNode", Id: 0,
					Parameters: []hir.TypedName{
						{Name: "z", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 2}},
						{Name: "a", Type: hir.Type{Base: hir.Uint8}},
						{Name: "ready", Type: hir.Type{Base: hir.Bool}},
					},
					ParameterNames: map[string]int{"z": 0, "a": 1, "ready": 2},
					ReturnType:     hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1},
				},
				{Name: "idle", Id: 1, ReturnType: hir.Type{Base: hir.Void}},
			},
			Globals: []hir.Global{
				{Name: "head", Id: 0, Constant: false, Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}},
				{Name: "limit", Id: 1, Constant: true, Type: hir.Type{Base: hir.Uint8}},
			},
		},
		symbols: map[string]hir.Symbol{
			"Container": hir.StructID(0), "Node": hir.StructID(1),
			"selectNode": hir.FunctionID(0), "idle": hir.FunctionID(1),
			"head": hir.GlobalID(0), "limit": hir.GlobalID(1),
		},
	},
	{
		name: "member names are scoped to their declaration",
		source: `global int32 value = 1i32
struct Left { int32 value }
struct Right { int32 value }
fnc first(int32 value) -> int32 { return value }
fnc second(int32 value) -> int32 { return value }`,
		want: hir.Program{
			Structs: []hir.Struct{
				{Name: "Left", Id: 0, Fields: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, FieldNames: map[string]int{"value": 0}},
				{Name: "Right", Id: 1, Fields: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, FieldNames: map[string]int{"value": 0}},
			},
			Functions: []hir.Function{
				{Name: "first", Id: 0, Parameters: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, ParameterNames: map[string]int{"value": 0}, ReturnType: hir.Type{Base: hir.Int32}},
				{Name: "second", Id: 1, Parameters: []hir.TypedName{{Name: "value", Type: hir.Type{Base: hir.Int32}}}, ParameterNames: map[string]int{"value": 0}, ReturnType: hir.Type{Base: hir.Int32}},
			},
			Globals: []hir.Global{{Name: "value", Id: 0, Constant: false, Type: hir.Type{Base: hir.Int32}}},
		},
		symbols: map[string]hir.Symbol{
			"value": hir.GlobalID(0), "Left": hir.StructID(0), "Right": hir.StructID(1),
			"first": hir.FunctionID(0), "second": hir.FunctionID(1),
		},
	},
	{
		name:        "duplicate fields",
		source:      `struct Item { int32 value bool value }`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "value"}, line: 1}},
	},
	{
		name:        "duplicate parameters",
		source:      `fnc repeat(int32 value, bool value) -> none { }`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "value"}, line: 1}},
	},
	{
		name: "multiple member duplicates",
		source: `struct Item { int32 value bool value }
fnc repeat(int32 arg, bool arg) -> none { }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "value"}, line: 1},
			{messageContains: []string{"duplicate", "arg"}, line: 2},
		},
	},

	{
		name: "duplicate struct then struct",
		source: `struct repeated { int32 value }
struct repeated { int32 value }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate struct then function",
		source: `struct repeated { int32 value }
fnc repeated() -> int32 { return 0i32 }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate struct then global",
		source: `struct repeated { int32 value }
global int32 repeated = 0i32`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate function then struct",
		source: `fnc repeated() -> int32 { return 0i32 }
struct repeated { int32 value }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate function then function",
		source: `fnc repeated() -> int32 { return 0i32 }
fnc repeated() -> int32 { return 0i32 }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate function then global",
		source: `fnc repeated() -> int32 { return 0i32 }
global int32 repeated = 0i32`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate global then struct",
		source: `global int32 repeated = 0i32
struct repeated { int32 value }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate global then function",
		source: `global int32 repeated = 0i32
fnc repeated() -> int32 { return 0i32 }`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "duplicate global then global",
		source: `global int32 repeated = 0i32
global int32 repeated = 0i32`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
		},
	},
	{
		name: "multiple duplicate names",
		source: `struct repeated { int32 value }
struct repeated { int32 other }
global int32 counter = 0i32
global int32 counter = 1i32`,
		diagnostics: []expectedDiagnostic{
			{messageContains: []string{"duplicate", "repeated"}, line: 2},
			{messageContains: []string{"duplicate", "counter"}, line: 4},
		},
	},
}

// These cases check declaration metadata; pass 2 cases check executable output.
func TestAnalyze(t *testing.T) {
	for _, test := range declarationTests {
		t.Run(test.name, func(t *testing.T) {
			program := parseDeclarations(t, test.source)
			c := New()
			got, diagnostics := c.Analyze(program)

			assertDiagnostics(t, diagnostics, test.diagnostics)
			if len(test.diagnostics) != 0 {
				// Recovery output is not part of the error-case contract.
				return
			}
			assertDeclarationMetadata(t, got, &test.want)
			assertSymbols(t, c.Symbols, test.symbols)
		})
	}
}

func parseDeclarations(t *testing.T, source string) *parser.Program {
	t.Helper()
	p := parser.New(lexer.New(source))
	program := p.ParseProgram()
	if len(p.Errors) != 0 {
		t.Fatalf("invalid test fixture: %v", p.Errors)
	}
	return program
}

func assertDeclarationMetadata(t *testing.T, got, want *hir.Program) {
	t.Helper()
	if got == nil {
		t.Fatal("Analyze returned nil")
	}
	metadata := *got
	metadata.Functions = append([]hir.Function(nil), got.Functions...)
	metadata.Globals = append([]hir.Global(nil), got.Globals...)
	for i := range metadata.Functions {
		metadata.Functions[i].Locals = nil
		metadata.Functions[i].Body = hir.Block{}
	}
	for i := range metadata.Globals {
		metadata.Globals[i].Initializer = nil
	}
	assertDeclarations(t, &metadata, want)
}

func assertDeclarations(t *testing.T, got, want *hir.Program) {
	t.Helper()
	if got == nil {
		t.Fatal("Analyze returned nil")
	}
	assertSlice(t, "structs", got.Structs, want.Structs)
	assertSlice(t, "functions", got.Functions, want.Functions)
	assertSlice(t, "globals", got.Globals, want.Globals)
}

func assertSlice[T any](t *testing.T, name string, got, want []T) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("%s: want %d entries, got %d", name, len(want), len(got))
	}
	for i := range want {
		if !reflect.DeepEqual(normalizeEmptyCollections(got[i]), normalizeEmptyCollections(want[i])) {
			t.Errorf("%s[%d]: want %#v, got %#v", name, i, want[i], got[i])
		}
	}
}

// Normalize copies, leaving the actual output and fixture data unchanged.
func normalizeEmptyCollections(declaration any) any {
	switch node := declaration.(type) {
	case hir.Struct:
		if len(node.Fields) == 0 {
			node.Fields = nil
		}
		if len(node.FieldNames) == 0 {
			node.FieldNames = nil
		}
		return node
	case hir.Function:
		if len(node.Locals) == 0 {
			node.Locals = nil
		}
		if len(node.Body.Statements) == 0 {
			node.Body.Statements = nil
		}
		if len(node.Parameters) == 0 {
			node.Parameters = nil
		}
		if len(node.ParameterNames) == 0 {
			node.ParameterNames = nil
		}
		return node
	default:
		return declaration
	}
}

func assertSymbols(t *testing.T, got, want map[string]hir.Symbol) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("symbols: want %v, got %v", want, got)
	}
	for name, expected := range want {
		actual, ok := got[name]
		if !ok || actual != expected {
			t.Errorf("symbol %q: want %T(%v), got %T(%v), present=%v",
				name, expected, expected, actual, actual, ok)
		}
	}
}

// Match diagnostics without requiring a particular reporting order. Fragments
// keep these tests independent of punctuation and surrounding explanation.
func assertDiagnostics(t *testing.T, got []Diagnostic, want []expectedDiagnostic) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("diagnostics: want %d, got %d: %+v", len(want), len(got), got)
	}
	matched := make([]bool, len(got))
	for _, expected := range want {
		found := false
		for i, actual := range got {
			if matched[i] || actual.Position == nil || actual.Position.StartLine != expected.line {
				continue
			}
			matches := true
			for _, fragment := range expected.messageContains {
				if !strings.Contains(actual.Message, fragment) {
					matches = false
					break
				}
			}
			if matches {
				matched[i] = true
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing diagnostic containing %q at line %d; got %+v", expected.messageContains, expected.line, got)
		}
	}
}

var externDeclarationTests = []struct {
	name, source string
	want         hir.Program
	symbols      map[string]hir.Symbol
	diagnostics  []expectedDiagnostic
}{
	{
		name:   "extern function signature and privacy",
		source: `private extern fnc read(int32 count, char* buffer) -> int32`,
		want: hir.Program{Functions: []hir.Function{{
			Name: "read", Id: 0, Private: true, External: true,
			Parameters: []hir.TypedName{
				{Name: "count", Type: hir.Type{Base: hir.Int32}},
				{Name: "buffer", Type: hir.Type{Base: hir.Char, Pointer: 1}},
			},
			ParameterNames: map[string]int{"count": 0, "buffer": 1},
			ReturnType:     hir.Type{Base: hir.Int32},
		}}},
		symbols: map[string]hir.Symbol{"read": hir.FunctionID(0)},
	},
	{
		name:    "extern void function without parameters",
		source:  `extern fnc release() -> none`,
		want:    hir.Program{Functions: []hir.Function{{Name: "release", Id: 0, External: true, ReturnType: hir.Type{Base: hir.Void}}}},
		symbols: map[string]hir.Symbol{"release": hir.FunctionID(0)},
	},
	{
		name: "extern structs and functions share declaration ID tables",
		source: `struct First {}
fnc first() -> none {}
extern struct Handle
extern fnc open() -> Handle*
extern struct Other
extern fnc close(Handle* handle) -> none`,
		want: hir.Program{
			Structs: []hir.Struct{{Name: "First", Id: 0}, {Name: "Handle", Id: 1, Opaque: true, Unsized: true}, {Name: "Other", Id: 2, Opaque: true, Unsized: true}},
			Functions: []hir.Function{
				{Name: "first", Id: 0, ReturnType: hir.Type{Base: hir.Void}},
				{Name: "open", Id: 1, External: true, ReturnType: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}},
				{Name: "close", Id: 2, External: true, ReturnType: hir.Type{Base: hir.Void}, Parameters: []hir.TypedName{{Name: "handle", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}}}, ParameterNames: map[string]int{"handle": 0}},
			},
		},
		symbols: map[string]hir.Symbol{"First": hir.StructID(0), "Handle": hir.StructID(1), "Other": hir.StructID(2), "first": hir.FunctionID(0), "open": hir.FunctionID(1), "close": hir.FunctionID(2)},
	},
	{
		name: "forward opaque pointers in fields signatures and globals",
		source: `extern fnc lookup(Handle** handles) -> Handle*
struct Wrapper { Handle* handle }
global Handle* handle = 0 as Handle*
extern struct Handle`,
		want: hir.Program{
			Structs: []hir.Struct{
				{Name: "Wrapper", Id: 0, Fields: []hir.TypedName{{Name: "handle", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}}}, FieldNames: map[string]int{"handle": 0}},
				{Name: "Handle", Id: 1, Opaque: true, Unsized: true},
			},
			Functions: []hir.Function{{Name: "lookup", Id: 0, External: true, Parameters: []hir.TypedName{{Name: "handles", Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 2}}}, ParameterNames: map[string]int{"handles": 0}, ReturnType: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}}},
			Globals:   []hir.Global{{Name: "handle", Id: 0, Type: hir.Type{Base: hir.StructType, Struct: 1, Pointer: 1}}},
		},
		symbols: map[string]hir.Symbol{"lookup": hir.FunctionID(0), "Wrapper": hir.StructID(0), "Handle": hir.StructID(1), "handle": hir.GlobalID(0)},
	},
	{name: "duplicate extern parameters", source: `extern fnc read(int32 value,
bool value) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "param", "value"}, line: 2}}},
	{name: "unknown extern parameter type", source: `extern fnc read(
Missing value) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"parameter", "value"}, line: 2}}},
	{name: "none extern parameter", source: `extern fnc read(
none value) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"none", "param", "value"}, line: 2}}},
	{name: "unknown extern return type", source: `extern fnc read() -> Missing`, diagnostics: []expectedDiagnostic{{messageContains: []string{"return type", "read"}, line: 1}}},
	{name: "function name is not an extern parameter type", source: `fnc Named() -> none {}
extern fnc read(Named value) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"parameter", "value"}, line: 2}}},
	{
		name: "opaque field is allowed in a type declaration",
		source: `extern struct Handle
struct Wrapper { Handle handle }`,
		want: hir.Program{Structs: []hir.Struct{
			{Name: "Handle", Id: 0, Opaque: true, Unsized: true},
			{Name: "Wrapper", Id: 1, Unsized: true, Fields: []hir.TypedName{{Name: "handle", Type: hir.Type{Base: hir.StructType, Struct: 0}}}, FieldNames: map[string]int{"handle": 0}},
		}},
		symbols: map[string]hir.Symbol{"Handle": hir.StructID(0), "Wrapper": hir.StructID(1)},
	},
	{name: "opaque parameter by value", source: `extern struct Handle
extern fnc consume(Handle handle) -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"opaque", "param", "handle", "consume"}, line: 2}}},
	{
		name: "opaque return is allowed in an extern declaration",
		source: `extern struct Handle
extern fnc create() -> Handle`,
		want: hir.Program{
			Structs:   []hir.Struct{{Name: "Handle", Id: 0, Opaque: true, Unsized: true}},
			Functions: []hir.Function{{Name: "create", Id: 0, External: true, ReturnType: hir.Type{Base: hir.StructType, Struct: 0}}},
		},
		symbols: map[string]hir.Symbol{"Handle": hir.StructID(0), "create": hir.FunctionID(0)},
	},
	{name: "opaque global by value", source: `extern struct Handle
global Handle handle = Handle:{}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"opaque", "global", "handle"}, line: 2}}},
	{
		name: "global containing opaque field indirectly is unsized",
		source: `global Outer value = Outer:{}
struct Outer { Inner inner }
struct Inner { Handle handle }
extern struct Handle`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"global", "value"}, line: 1}},
	},
	{name: "duplicate extern functions", source: `extern fnc repeated() -> none
extern fnc repeated() -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern function then defined function", source: `extern fnc repeated() -> none
fnc repeated() -> none {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "defined function then extern function", source: `fnc repeated() -> none {}
extern fnc repeated() -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern function then global", source: `extern fnc repeated() -> none
global int32 repeated = 0i32`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "global then extern function", source: `global int32 repeated = 0i32
extern fnc repeated() -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "duplicate extern structs", source: `extern struct repeated
extern struct repeated`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern struct then defined struct", source: `extern struct repeated
struct repeated {}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "defined struct then extern struct", source: `struct repeated {}
extern struct repeated`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern struct then global", source: `extern struct repeated
global int32 repeated = 0i32`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "global then extern struct", source: `global int32 repeated = 0i32
extern struct repeated`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern struct then extern function", source: `extern struct repeated
extern fnc repeated() -> none`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
	{name: "extern function then extern struct", source: `extern fnc repeated() -> none
extern struct repeated`, diagnostics: []expectedDiagnostic{{messageContains: []string{"duplicate", "repeated"}, line: 2}}},
}

func TestExternDeclarations(t *testing.T) {
	for _, test := range externDeclarationTests {
		t.Run(test.name, func(t *testing.T) {
			c := New()
			got, diagnostics := c.Analyze(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, test.diagnostics)
			if len(test.diagnostics) != 0 {
				return
			}
			assertDeclarationMetadata(t, got, &test.want)
			assertSymbols(t, c.Symbols, test.symbols)
			for i, want := range test.want.Functions {
				if want.External {
					if len(got.Functions[i].Locals) != 0 || len(got.Functions[i].Body.Statements) != 0 {
						t.Errorf("extern function %q must have no locals or body", want.Name)
					}
				}
			}
		})
	}
}

func TestExternDeclarationFixturesParse(t *testing.T) {
	for _, test := range externDeclarationTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
}
