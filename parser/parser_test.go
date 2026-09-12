package parser

import (
	"fmt"
	"gl3/lexer"
	"strings"
	"testing"
	"time"
)

type InputOutput struct {
	input  string
	output string
}

func TestReturnStatements(t *testing.T) {
	tests := []struct {
		name, source, wantString string
		bare                     bool
		statementCount           int
	}{
		{name: "bare before brace", source: `fnc sample() -> none { return }`, wantString: "return", bare: true, statementCount: 1},
		{name: "bare before semicolon", source: `fnc sample() -> none { return; }`, wantString: "return", bare: true, statementCount: 1},
		{name: "semicolon preserves next statement", source: `fnc sample() -> none { return; return }`, wantString: "return", bare: true, statementCount: 2},
		{name: "value before brace", source: `fnc sample() -> bool { return true }`, wantString: "return true", statementCount: 1},
		{name: "value before semicolon", source: `fnc sample() -> bool { return true; }`, wantString: "return true", statementCount: 1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			fn := parseSingleStatement(t, test.source).(*FunctionStatement)
			if len(fn.Body.Statements) != test.statementCount {
				t.Fatalf("statements: want %d, got %d", test.statementCount, len(fn.Body.Statements))
			}
			stmt := fn.Body.Statements[0].(*ReturnStatement)
			if (stmt.Expr == nil) != test.bare {
				t.Fatalf("bare return: want %t, got expression %T", test.bare, stmt.Expr)
			}
			if got := stmt.String(); got != test.wantString {
				t.Errorf("String(): want %q, got %q", test.wantString, got)
			}
			wantPosition := stmt.Token.Position
			if !test.bare {
				wantPosition.EndLine = stmt.Expr.Position().EndLine
				wantPosition.EndCol = stmt.Expr.Position().EndCol
			}
			if got := *stmt.Position(); got != wantPosition {
				t.Errorf("Position(): want %+v, got %+v", wantPosition, got)
			}
		})
	}
}

func TestLiterals(t *testing.T) {
	tests := map[string]InputOutput{
		"int64": {
			"4",
			"4(Int);",
		},
		"int32": {
			"4i32",
			"4(Int32);",
		},
		"int16": {
			"4i16",
			"4(Int16);",
		},
		"int8": {
			"4i8",
			"4(Int8);",
		},
		"uint32": {
			"4u32",
			"4(Uint32);",
		},
		"uint16": {
			"4u16",
			"4(Uint16);",
		},
		"uint8": {
			"4u8",
			"4(Uint8);",
		},
		"uint64": {
			"4u64",
			"4(Uint);",
		},
		"float": {
			"1.5",
			"1.5(Float);",
		},
		"true bool": {
			"true",
			"true;",
		},
		"false bool": {
			"false",
			"false;",
		},
		"char": {
			"'a'",
			"97(Int8);",
		},
		"string": {
			"\"hello\"",
			"\"hello\000\";",
		},
	}

	runTests(t, tests)
}

func TestIntegerBoundaryValues(t *testing.T) {
	tests := []struct {
		input string
		kind  lexer.VarType
		value uint64
	}{
		{"0", lexer.VarType{Base: lexer.Int}, 0},
		{"9223372036854775807", lexer.VarType{Base: lexer.Int}, 9223372036854775807},
		{"0u64", lexer.VarType{Base: lexer.Uint}, 0},
		{"9223372036854775808u64", lexer.VarType{Base: lexer.Uint}, 9223372036854775808},
		{"18446744073709551615u64", lexer.VarType{Base: lexer.Uint}, 18446744073709551615},
		{"255u8", lexer.VarType{Base: lexer.Uint8}, 255},
		{"65535u16", lexer.VarType{Base: lexer.Uint16}, 65535},
		{"4294967295u32", lexer.VarType{Base: lexer.Uint32}, 4294967295},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			stmt := parseSingleStatement(t, test.input)
			es, ok := stmt.(*ExpressionStatement)
			if !ok {
				t.Fatalf("expected expression, got %T", stmt)
			}
			lit, ok := es.Expression.(*IntegerLiteral)
			if !ok {
				t.Fatalf("expected integer, got %T", es.Expression)
			}
			if lit.Type != test.kind {
				t.Fatalf("want type %v, got %v", test.kind, lit.Type)
			}
			if lit.UValue != test.value {
				t.Fatalf("want value %d, got %d", test.value, lit.UValue)
			}
		})
	}
}

func TestDefStatement(t *testing.T) {
	tests := map[string]InputOutput{
		"int def": {
			"def int x = 7",
			"def Int x = 7(Int);",
		},
		"int32 def": {
			"def int32 x = 7i32",
			"def Int32 x = 7(Int32);",
		},
		"uint32 def": {
			"def uint32 x = 7u32",
			"def Uint32 x = 7(Uint32);",
		},
		"float def": {
			"def float x = 1.5",
			"def Float x = 1.5(Float);",
		},
		"bool def": {
			"def bool x = true",
			"def Bool x = true;",
		},
		"char def": {
			"def char x = 'a'",
			"def Char x = 97(Int8);",
		},
		"string def": {
			"def char* x = \"hello\"",
			"def Char* x = \"hello\000\";",
		},
		"multi pointer": {
			"def char*** x = \"hello\"",
			"def Char*** x = \"hello\000\";",
		},
		"global int def": {
			"global int x = 7",
			"global Int x = 7(Int);",
		},
		"global int32 def": {
			"global int32 x = 7i32",
			"global Int32 x = 7(Int32);",
		},
		"global uint32 def": {
			"global uint32 x = 7u32",
			"global Uint32 x = 7(Uint32);",
		},
		"global float def": {
			"global float x = 1.5",
			"global Float x = 1.5(Float);",
		},
		"global bool def": {
			"global bool x = true",
			"global Bool x = true;",
		},
		"global char def": {
			"global char x = 'a'",
			"global Char x = 97(Int8);",
		},
		"global string def": {
			"global char* x = \"hello\"",
			"global Char* x = \"hello\000\";",
		},
		"global multi pointer": {
			"global char*** x = \"hello\"",
			"global Char*** x = \"hello\000\";",
		},
	}

	runTests(t, tests)
}

func TestFuncStatement(t *testing.T) {
	tests := map[string]InputOutput{
		"basic main func": {
			"fnc main() -> int32 { \n return 0i32 \n }",
			"fnc main() -> Int32 { return 0(Int32) };",
		},
		"mult params": {
			"fnc stuff(int8 x, int32** other) -> none { \n }",
			"fnc stuff(Int8 x, Int32** other) -> Void {  };",
		},
		"no params void": {
			"fnc foo() -> none { \n }",
			"fnc foo() -> Void {  };",
		},
		"non void ret with params": {
			"fnc x(int8 x, int32** other) -> int8 { \n return x; \n }",
			"fnc x(Int8 x, Int32** other) -> Int8 { return x };",
		},
		"bool ret": {
			"fnc isok() -> bool { \n return true; \n }",
			"fnc isok() -> Bool { return true };",
		},
		"char pointer ret": {
			"fnc greet() -> char* { \n return \"hi\"; \n }",
			"fnc greet() -> Char* { return \"hi\000\" };",
		},
		"edge case": {
			`fnc create_item(int32 id) -> Item {
    if id < 0i32 {
        return Item:{ default_id }
    }
    return Item:{ id }
}`,
			"fnc create_item(Int32 id) -> Item { if (id < 0(Int32)) { return Item:{default_id} };return Item:{id} };",
		},
	}

	runTests(t, tests)
}

func TestPrivateAndExternDeclarations(t *testing.T) {
	for _, private := range []bool{false, true} {
		prefix := ""
		if private {
			prefix = "private "
		}
		for _, external := range []bool{false, true} {
			input := prefix
			if external {
				input += "extern "
			}
			input += "fnc f(Node** p, int count) -> char*"
			if !external {
				input += " { return p }"
			}
			t.Run(input, func(t *testing.T) {
				stmt := parseSingleStatement(t, input)
				var params []FunctionParameter
				var ret lexer.VarType
				if external {
					f, ok := stmt.(*ExternFunctionStatement)
					if !ok {
						t.Fatalf("expected extern function, got %T", stmt)
					}
					if f.Name != "f" || f.Private != private {
						t.Fatalf("wrong name/privacy: %+v", f)
					}
					params, ret = f.Params, f.ReturnType
				} else {
					f, ok := stmt.(*FunctionStatement)
					if !ok {
						t.Fatalf("expected function, got %T", stmt)
					}
					if f.Name.Value != "f" || f.Private != private {
						t.Fatalf("wrong name/privacy: %+v", f)
					}
					params, ret = f.Params, f.Type
					if len(f.Body.Statements) != 1 {
						t.Fatalf("expected one body statement")
					}
					if _, ok := f.Body.Statements[0].(*ReturnStatement); !ok {
						t.Fatalf("expected return statement")
					}
				}
				if ret != (lexer.VarType{Base: lexer.Char, Pointer: 1}) {
					t.Errorf("wrong return type: %v", ret)
				}
				if len(params) != 2 {
					t.Fatalf("expected 2 parameters, got %d", len(params))
				}
				if params[0].Name.Value != "p" || params[0].Type != (lexer.VarType{IsStructType: true, StructName: "Node", Pointer: 2}) {
					t.Errorf("wrong first parameter: %+v", params[0])
				}
				if params[1].Name.Value != "count" || params[1].Type != (lexer.VarType{Base: lexer.Int}) {
					t.Errorf("wrong second parameter: %+v", params[1])
				}
			})
		}
		t.Run(prefix+"extern struct", func(t *testing.T) {
			stmt := parseSingleStatement(t, prefix+"extern struct Node")
			s, ok := stmt.(*ExternStructStatement)
			if !ok {
				t.Fatalf("expected extern struct, got %T", stmt)
			}
			if s.Name != "Node" || s.Private != private {
				t.Fatalf("wrong name/privacy: %+v", s)
			}
		})
		t.Run(prefix+"extern empty function", func(t *testing.T) {
			stmt := parseSingleStatement(t, prefix+"extern fnc empty() -> none")
			f, ok := stmt.(*ExternFunctionStatement)
			if !ok {
				t.Fatalf("expected extern function, got %T", stmt)
			}
			if f.Name != "empty" || f.Private != private || len(f.Params) != 0 || f.ReturnType != (lexer.VarType{Base: lexer.Void}) {
				t.Fatalf("wrong empty declaration: %+v", f)
			}
		})
	}
}

func TestImportStatement(t *testing.T) {
	tests := map[string]InputOutput{
		"std module": {
			"import \"arrays\"",
			"import \"arrays\";",
		},
		"gl3 file": {
			"import \"stuff.gl3\"",
			"import \"stuff.gl3\";",
		},
	}

	runTests(t, tests)
}

func TestIfStatement(t *testing.T) {
	tests := map[string]InputOutput{
		"basic true": {
			"if true { \n  \n }",
			"if true {  };",
		},
		"basic false": {
			"if false { \n  \n }",
			"if false {  };",
		},
		"expr cond": {
			"if x > 5i32 { \n \n }",
			"if (x > 5(Int32)) {  };",
		},
		"expr cond with body": {
			"if x == 7u32 { \n stuff()\n }",
			"if (x == 7(Uint32)) { stuff() };",
		},
		"with else empty": {
			"if true { \n \n } else { \n \n }",
			"if true {  } else {  };",
		},
		"with else body": {
			"if x < 1 { \n return 0 \n } else { \n return 1 \n }",
			"if (x < 1(Int)) { return 0(Int) } else { return 1(Int) };",
		},
		"nested if": {
			"if true { \n if false { \n \n } \n }",
			"if true { if false {  } };",
		},
		"logical condition": {
			"if x > 5 && y < 2 { \n \n }",
			"if ((x > 5(Int)) && (y < 2(Int))) {  };",
		},
	}

	runTests(t, tests)
}

func TestWhileStatement(t *testing.T) {
	tests := map[string]InputOutput{
		"basic true": {
			"while true { \n  \n }",
			"while true {  };",
		},
		"basic false": {
			"while false { \n  \n }",
			"while false {  };",
		},
		"expr cond": {
			"while x > 5i32 { \n \n }",
			"while (x > 5(Int32)) {  };",
		},
		"expr cond with body": {
			"while x == 7u32 { \n stuff()\n }",
			"while (x == 7(Uint32)) { stuff() };",
		},
		"nested while": {
			"while true { \n while false { \n \n } \n }",
			"while true { while false {  } };",
		},
		"logical condition": {
			"while x > 5 && y < 2 { \n \n }",
			"while ((x > 5(Int)) && (y < 2(Int))) {  };",
		},
	}

	runTests(t, tests)
}

func TestLoopControlStatements(t *testing.T) {
	for _, separator := range []string{"\n", ";"} {
		t.Run(separator, func(t *testing.T) {
			stmt := parseSingleStatement(t, "while true { continue"+separator+"break"+separator+"sentinel() }")
			loop, ok := stmt.(*WhileStatement)
			if !ok {
				t.Fatalf("expected while, got %T", stmt)
			}
			if len(loop.Body.Statements) != 3 {
				t.Fatalf("expected 3 statements, got %d", len(loop.Body.Statements))
			}
			if _, ok := loop.Body.Statements[0].(*ContinueStatement); !ok {
				t.Errorf("expected continue, got %T", loop.Body.Statements[0])
			}
			if _, ok := loop.Body.Statements[1].(*BreakStatement); !ok {
				t.Errorf("expected break, got %T", loop.Body.Statements[1])
			}
			if got := statementExpressionShape(t, loop.Body.Statements[2]); got != "call(sentinel)" {
				t.Errorf("following statement changed: %s", got)
			}
		})
	}
}

func TestInfixExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"plus": {
			"5i32 + 3i32",
			"(5(Int32) + 3(Int32));",
		},
		"minus": {
			"8u32 - 2u32",
			"(8(Uint32) - 2(Uint32));",
		},
		"asterisk": {
			"x * 4i16",
			"(x * 4(Int16));",
		},
		"slash": {
			"12i32 / 3i32",
			"(12(Int32) / 3(Int32));",
		},
		"logical and": {
			"ok && ready",
			"(ok && ready);",
		},
		"logical or": {
			"ok || ready",
			"(ok || ready);",
		},
		"equals": {
			"x == 7i8",
			"(x == 7(Int8));",
		},
		"not equals": {
			"x != 7i8",
			"(x != 7(Int8));",
		},
		"less than": {
			"x < 5i32",
			"(x < 5(Int32));",
		},
		"greater than": {
			"x > 5i32",
			"(x > 5(Int32));",
		},
		"less than or equal": {
			"x <= 5i32",
			"(x <= 5(Int32));",
		},
		"greater than or equal": {
			"x >= 5i32",
			"(x >= 5(Int32));",
		},
		"dot": {
			"player.health",
			"(player . health);",
		},
		"array index literal": {
			"a[1]",
			"*(a + 1(Int));",
		},
		"array index identifier": {
			"items[i]",
			"*(items + i);",
		},
		"mismatched types": {
			"1 == true",
			"(1(Int) == true);",
		},
	}

	runTests(t, tests)
}

func TestPrecedenceExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"product before sum": {
			"1 + 2 * 3",
			"(1(Int) + (2(Int) * 3(Int)));",
		},
		"grouped sum": {
			"(1 + 2) * 3",
			"((1(Int) + 2(Int)) * 3(Int));",
		},
		"logical and equality": {
			"1 + 2 == 3 && 4 > 5",
			"(((1(Int) + 2(Int)) == 3(Int)) && (4(Int) > 5(Int)));",
		},
		"prefix then sum": {
			"-1 + 2",
			"((-1(Int)) + 2(Int));",
		},
		"comparison before equality": {
			"1 + 2 < 3 == 4",
			"(((1(Int) + 2(Int)) < 3(Int)) == 4(Int));",
		},
		// Cast, sizeof, and postfix interactions have structural assertions below.
	}

	runTests(t, tests)
}

func TestExpressionAssociativityAndPostfix(t *testing.T) {
	tests := []InputOutput{
		{"a - b - c", "-(-(a, b), c)"},
		{"a / b * c", "*(/(a, b), c)"},
		{"a = b = c", "assign(a, assign(b, c))"},
		{"a || b && c", "||(a, &&(b, c))"},
		{"a && b || c", "||(&&(a, b), c)"},
		{"-f(a)[i]", "prefix(-, deref(+(call(f, a), i)))"},
		{"items[i][j]", "deref(+(deref(+(items, i)), j))"},
		{"items[i].field", ".(deref(+(items, i)), field)"},
	}
	runExpressionShapeTests(t, tests)
}

func TestAssignmentExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"identifier assign": {
			"x = 1i32",
			"x = 1(Int32);",
		},
		"deref assign": {
			"*ptr = 3i8",
			"*ptr = 3(Int8);",
		},
		"dot assign": {
			"player.health = 0",
			"(player . health) = 0(Int);",
		},
		"array index assign": {
			"items[i] = 2u16",
			"*(items + i) = 2(Uint16);",
		},
	}

	runTests(t, tests)
}

func TestCastExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"int to float": {
			"1i32 as float",
			"1(Int32) as Float;",
		},
		"ident to pointer": {
			"x as int8*",
			"x as Int8*;",
		},
	}

	runTests(t, tests)
}

func TestCastPrecedence(t *testing.T) {
	tests := []InputOutput{
		{"a + b as float", "cast(Float, +(a, b))"},
		{"a + (b as float)", "+(a, cast(Float, b))"},
		{"a as float + b", "+(cast(Float, a), b)"},
		{"a as float + b as float", "cast(Float, +(cast(Float, a), b))"},
		{"a as float + (b as float)", "+(cast(Float, a), cast(Float, b))"},
		{"(a + b) as float", "cast(Float, +(a, b))"},
		{"a * b as float", "cast(Float, *(a, b))"},
		{"a * (b as float)", "*(a, cast(Float, b))"},
		{"(a as float) * b", "*(cast(Float, a), b)"},
		{"a as float / b", "/(cast(Float, a), b)"},
		{"a / b as float", "cast(Float, /(a, b))"},
		{"a < b as int", "<(a, cast(Int, b))"},
		{"a as int < b", "<(cast(Int, a), b)"},
		{"a == b as int", "==(a, cast(Int, b))"},
		{"a as int == b", "==(cast(Int, a), b)"},
		{"a && b as bool", "&&(a, cast(Bool, b))"},
		{"a as bool || b", "||(cast(Bool, a), b)"},
		{"-a as float", "cast(Float, prefix(-, a))"},
		{"-(a as float)", "prefix(-, cast(Float, a))"},
		{"!a as bool", "cast(Bool, prefix(!, a))"},
		{"!(a as bool)", "prefix(!, cast(Bool, a))"},
		{"*p as int", "cast(Int, deref(p))"},
		{"*p as int*", "cast(Int*, deref(p))"},
		{"*(p as int*)", "deref(cast(Int*, p))"},
		{"(&a) as int", "cast(Int, ref(a))"},
		{"a as int32 as float", "cast(Float, cast(Int32, a))"},
		{"p as Node**", "cast(Node**, p)"},
		{"f(a) as int", "cast(Int, call(f, a))"},
		{"f(a as int, b)", "call(f, cast(Int, a), b)"},
		{"items[i] as int", "cast(Int, deref(+(items, i)))"},
		{"(p as int*)[i]", "deref(+(cast(Int*, p), i))"},
		{"p as int*[i]", "deref(+(cast(Int*, p), i))"},
		{"p.field as int", "cast(Int, .(p, field))"},
		{"(p as Node*).field", ".(cast(Node*, p), field)"},
		{"p as Node*.field", ".(cast(Node*, p), field)"},
		{"f(a)[i] as int", "cast(Int, deref(+(call(f, a), i)))"},
		{"p = malloc(sizeof Node) as Node*", "assign(p, cast(Node*, call(malloc, sizeof(Node))))"},
		{"a = b as int + c", "assign(a, +(cast(Int, b), c))"},
	}
	runExpressionShapeTests(t, tests)
}

func TestPrefixExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"not true": {
			"!true",
			"(!true);",
		},
		"not identifier": {
			"!ready",
			"(!ready);",
		},
		"not infix": {
			"!(x == 1i8)",
			"(!(x == 1(Int8)));",
		},
		"neg int": {
			"-5i32",
			"(-5(Int32));",
		},
		"neg identifier": {
			"-count",
			"(-count);",
		},
		"neg infix": {
			"-(x + 2i32)",
			"(-(x + 2(Int32)));",
		},
	}

	runTests(t, tests)
}

func TestCallExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"empty args": {
			"foo()",
			"foo();",
		},
		"single arg": {
			"foo(1i32)",
			"foo(1(Int32));",
		},
		"multi args": {
			"foo(1i8, x, true)",
			"foo(1(Int8), x, true);",
		},
		"infix arg": {
			"sum(1i32 + 2i32)",
			"sum((1(Int32) + 2(Int32)));",
		},
		"nested call": {
			"outer(inner(2u16))",
			"outer(inner(2(Uint16)));",
		},
		"string and char args": {
			"print(\"hi\", 'a')",
			"print(\"hi\000\", 97(Int8));",
		},
		"sizeof": {
			"arr_new(sizeof int32)",
			"arr_new(sizeof Int32);",
		},
	}

	runTests(t, tests)
}

func TestSizeofExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"sizeof builtin type": {
			"sizeof int32",
			"sizeof Int32;",
		},
		"sizeof pointer": {
			"sizeof char*",
			"sizeof Char*;",
		},
	}

	runTests(t, tests)
}

func TestSizeofPrecedence(t *testing.T) {
	tests := []InputOutput{
		{"sizeof Node", "sizeof(Node)"},
		{"sizeof Node**", "sizeof(Node**)"},
		{"sizeof int32**", "sizeof(Int32**)"},
		// Whitespace does not turn pointer stars into multiplication operators.
		{"sizeof int32 *", "sizeof(Int32*)"},
		{"sizeof int32 * *", "sizeof(Int32**)"},
		{"(sizeof int32) * n", "*(sizeof(Int32), n)"},
		{"(sizeof int32*) * n", "*(sizeof(Int32*), n)"},
		{"n * sizeof int32", "*(n, sizeof(Int32))"},
		{"sizeof int32 / n", "/(sizeof(Int32), n)"},
		{"sizeof int32 + n", "+(sizeof(Int32), n)"},
		{"n + sizeof int32", "+(n, sizeof(Int32))"},
		{"sizeof int32 < n", "<(sizeof(Int32), n)"},
		{"-sizeof int32", "prefix(-, sizeof(Int32))"},
		{"sizeof int32 as uint", "cast(Uint, sizeof(Int32))"},
		{"sizeof int32 + n as uint", "cast(Uint, +(sizeof(Int32), n))"},
		{"malloc((sizeof Node) * n) as Node*", "cast(Node*, call(malloc, *(sizeof(Node), n)))"},
		{"f(sizeof Node*, n as int, sizeof char)", "call(f, sizeof(Node*), cast(Int, n), sizeof(Char))"},
	}
	runExpressionShapeTests(t, tests)
}

func TestTypeStarsConsumeFollowingAsterisks(t *testing.T) {
	// A star following a type is a pointer suffix, even with spaces. These
	// inputs currently leave n as a separate statement; they do not multiply.
	// This characterizes token consumption, not a recommended way to write code.
	tests := []InputOutput{
		{"sizeof int32 * n", "sizeof(Int32*)"},
		{"sizeof int32* * n", "sizeof(Int32**)"},
		{"p as int32 * n", "cast(Int32*, p)"},
	}
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			p := New(lexer.New(test.input))
			program := p.ParseProgram()
			if len(p.Errors) != 0 {
				t.Fatalf("parser errors: %v", p.Errors)
			}
			if len(program.Statements) != 2 {
				t.Fatalf("expected type expression and trailing identifier, got %d statements", len(program.Statements))
			}
			if got := statementExpressionShape(t, program.Statements[0]); got != test.output {
				t.Fatalf("want: %s; got: %s", test.output, got)
			}
			if got := statementExpressionShape(t, program.Statements[1]); got != "n" {
				t.Fatalf("expected trailing n, got %s", got)
			}
		})
	}
}

func TestStructInitializationExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"basic init": {
			"Player:{1i32, true}",
			"Player:{1(Int32),true};",
		},
		"single elem init": {
			"Cooked:{1i32}",
			"Cooked:{1(Int32)};",
		},
		"empty init": {
			"Empty:{}",
			"Empty:{};",
		},
	}

	runTests(t, tests)
}

func TestArrayLiteralExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"int array": {
			"[int32; 1i32, 2i32]",
			"[Int32;1(Int32),2(Int32)];",
		},
		"pointer array": {
			"[char**; x]",
			"[Char**;x];",
		},
		"struct array": {
			"[Player; current]",
			"[Player;current];",
		},
		"struct pointer array": {
			"[Player**; current]",
			"[Player**;current];",
		},
		"empty array": {
			"[uint8;]",
			"[Uint8;];",
		},
	}

	runTests(t, tests)
}

func TestReferenceAndDereferenceExpression(t *testing.T) {
	tests := map[string]InputOutput{
		"reference identifier": {
			"&x",
			"&x;",
		},
		"dereference identifier": {
			"*ptr",
			"*ptr;",
		},
		"multi dereference identifier": {
			"***ptr",
			"***ptr;",
		},
	}

	runTests(t, tests)
}

func TestPointerPrefixPrecedence(t *testing.T) {
	runExpressionShapeTests(t, []InputOutput{
		{"&x as int", "cast(Int, ref(x))"},
		{"(&x) as int", "cast(Int, ref(x))"},
		{"&x + n", "+(ref(x), n)"},
		{"(&x) + n", "+(ref(x), n)"},
		{"&p.field", "ref(.(p, field))"},
		{"&items[i]", "ref(deref(+(items, i)))"},
		{"&*p", "ref(deref(p))"},
		{"*p[i]", "deref(deref(+(p, i)))"},
		{"*(p[i])", "deref(deref(+(p, i)))"},
		{"(*p)[i]", "deref(+(deref(p), i))"},
		{"*p.field", "deref(.(p, field))"},
		{"*(p.field)", "deref(.(p, field))"},
		{"(*p).field", ".(deref(p), field)"},
	})
}

func TestStructStatement(t *testing.T) {
	tests := map[string]struct {
		input  string
		name   string
		fields map[string]lexer.VarType
	}{
		"single field struct": {
			input: "struct Player { int32 health }",
			name:  "Player",
			fields: map[string]lexer.VarType{
				"health": {Base: lexer.Int32},
			},
		},
		"multi field struct": {
			input: "struct Player { int32 health char* name bool alive }",
			name:  "Player",
			fields: map[string]lexer.VarType{
				"health": {Base: lexer.Int32},
				"name":   {Base: lexer.Char, Pointer: 1},
				"alive":  {Base: lexer.Bool},
			},
		},
		"empty struct": {
			input:  "struct Empty { }",
			name:   "Empty",
			fields: map[string]lexer.VarType{},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			defer func() {
				if r := recover(); r != nil {
					t.Errorf("panicked: %v", r)
				}
			}()
			statement := parseSingleStatement(t, test.input)
			stmt, ok := statement.(*StructStatement)
			if !ok {
				t.Fatalf("expected StructStatement, got %T", statement)
			}
			if stmt.Name != test.name {
				t.Fatalf("expected struct name %q, got %q", test.name, stmt.Name)
			}
			if len(stmt.Fields) != len(test.fields) {
				t.Fatalf("expected %d fields, got %d", len(test.fields), len(stmt.Fields))
			}
			seen := make(map[string]bool)
			for _, field := range stmt.Fields {
				fieldName := field.Name.Value
				expectedType, ok := test.fields[fieldName]
				if !ok || seen[fieldName] {
					t.Fatalf("unexpected or repeated field %q", fieldName)
				}
				seen[fieldName] = true
				if field.Type != expectedType {
					t.Errorf("field %q expected type %v, got %v", fieldName, expectedType, field.Type)
				}
			}
		})
	}
}

func TestStructDeclarationFieldOrder(t *testing.T) {
	stmt := parseSingleStatement(t, "struct Ordered { int z char* a Node** middle }")
	s, ok := stmt.(*StructStatement)
	if !ok {
		t.Fatalf("expected struct, got %T", stmt)
	}
	names := []string{"z", "a", "middle"}
	types := []lexer.VarType{{Base: lexer.Int}, {Base: lexer.Char, Pointer: 1}, {IsStructType: true, StructName: "Node", Pointer: 2}}
	if len(s.Fields) != len(names) {
		t.Fatalf("wrong field count: %+v", s)
	}
	for i, name := range names {
		if s.Fields[i].Name.Value != name {
			t.Errorf("field %d: want name %s, got %s", i, name, s.Fields[i].Name.Value)
		}
		if s.Fields[i].Type != types[i] {
			t.Errorf("field %d: want %v, got %v", i, types[i], s.Fields[i].Type)
		}
	}
}

func TestUnterminatedStringLiteralDoesNotTimeoutRegression(t *testing.T) {
	tests := map[string]string{
		"parser nil expr stmt loop regression": `
fnc main() -> int32 {
				println("Hello, World!;
`,
		"lexer loop regression": `
import "io"

fnc main() -> int32 {
	println("Hello, World!;

	return 0i32;
}
`,
	}

	runTestCheckForTimeout(t, tests)
}

func TestLineCommentWithoutTrailingNewlineRegression(t *testing.T) {
	tests := map[string]InputOutput{
		"parses empty program": {
			input:  "// no new line",
			output: "",
		},
	}

	runTests(t, tests)
}

func TestTypedIdentifierErrorReporting(t *testing.T) {
	tests := map[string]struct {
		input       string
		errorSubstr string
	}{
		"def missing name": {
			input:       "def int = 1",
			errorSubstr: "expected identifier after type in def statement",
		},
		"function param missing name": {
			input:       "fnc foo(int32) -> int32 { }",
			errorSubstr: "expected identifier after type in function definition params",
		},
		"struct field missing name": {
			input:       "struct Player { int32 }",
			errorSubstr: "expected identifer after type in struct definition",
		},
	}

	runErrorTests(t, tests)
}

func TestTypeParseErrorReporting(t *testing.T) {
	tests := map[string]struct {
		input       string
		errorSubstr string
	}{
		"sizeof missing type": {
			input:       "sizeof",
			errorSubstr: "expected type after sizeof keyword",
		},
		"cast missing type": {
			input:       "x as",
			errorSubstr: "expected type after as token in cast expr",
		},
		"array missing type": {
			input:       "[; 1]",
			errorSubstr: "expected type after [ in array literal expr",
		},
	}

	runErrorTests(t, tests)
}

func TestMalformedParserInput(t *testing.T) {
	tests := map[string]struct {
		input       string
		errorSubstr string
	}{
		"missing index bracket":       {"a[1", "]"},
		"missing call paren":          {"f(a", ")"},
		"missing grouping paren":      {"(a + b", ""},
		"missing function brace":      {"fnc f() -> none { f()", ""},
		"missing if brace":            {"if true { f()", ""},
		"missing while brace":         {"while true { f()", ""},
		"missing struct brace":        {"struct S { int x", ""},
		"missing initializer brace":   {"S:{a", ""},
		"missing array bracket":       {"[int; a", "]"},
		"missing binary operand":      {"a +", "no prefix"},
		"missing prefix operand":      {"!", "no prefix"},
		"missing assignment operand":  {"a =", "no prefix"},
		"missing reference operand":   {"&", ""},
		"missing dereference operand": {"*", "no prefix"},
		"literal assignment target":   {"1 = x", "lhs of assignment"},
		"sum assignment target":       {"(a + b) = x", "lhs of assignment"},
		"call assignment target":      {"f() = x", "lhs of assignment"},
		"unknown integer suffix":      {"1u128", ""},
		"misspelled integer suffix":   {"1i33", ""},
		"unsuffixed overflow":         {"18446744073709551616", `could not parse "18446744073709551616" as integer`},
		"unsigned overflow":           {"18446744073709551616u64", `could not parse "18446744073709551616" as integer`},
	}

	runErrorTests(t, tests)
}

func runTestCheckForTimeout(t *testing.T, tests map[string]string) {
	for name, input := range tests {
		t.Run(name, func(t *testing.T) {
			timeout := time.After(1 * time.Second)
			done := make(chan bool)

			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panicked: %v", r)
					}
					done <- true
				}()
				l := lexer.New(input)
				p := New(l)
				_ = p.ParseProgram()
			}()

			select {
			case <-timeout:
				t.Fatalf("timed out after 1 second while parsing input")
			case <-done:
			}
		})
	}
}

func runErrorTests(t *testing.T, tests map[string]struct {
	input       string
	errorSubstr string
}) {
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			timeout := time.After(1 * time.Second)
			done := make(chan bool)
			var p *Parser

			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panicked: %v", r)
					}
					done <- true
				}()

				l := lexer.New(test.input)
				p = New(l)
				_ = p.ParseProgram()

				if len(p.Errors) == 0 {
					t.Errorf("expected parser error containing %q, got none", test.errorSubstr)
					return
				}

				for _, err := range p.Errors {
					if strings.Contains(err.Msg, test.errorSubstr) {
						return
					}
				}

				var msgs []string
				for _, err := range p.Errors {
					msgs = append(msgs, err.Msg)
				}
				t.Errorf("expected parser error containing %q, got %v", test.errorSubstr, msgs)
			}()

			select {
			case <-timeout:
				t.Fatalf("timed out after 1 second while parsing input")
			case <-done:
			}
		})
	}
}

func runTests(t *testing.T, tests map[string]InputOutput) {
	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			timeout := time.After(1 * time.Second)
			done := make(chan bool)
			var p *Parser
			go func() {
				defer func() {
					if r := recover(); r != nil {
						t.Errorf("panicked: %v", r)
						done <- true
					}
				}()
				l := lexer.New(test.input)
				p = New(l)
				program := p.ParseProgram()
				if len(p.Errors) != 0 {
					t.Errorf("got parser errors: %v", p.Errors)
				}
				if program.String() != test.output {
					t.Errorf("wanted: %s, got: %s", test.output, program)
				}
				done <- true
			}()

			select {
			case <-timeout:
				var trimmedErrors []string
				if p != nil {
					if len(p.Errors) > 5 {
						for _, err := range p.Errors[:5] {
							trimmedErrors = append(trimmedErrors, err.String())
						}
					} else {
						for _, err := range p.Errors {
							trimmedErrors = append(trimmedErrors, err.String())
						}
					}
				}
				t.Fatalf("timed out after 1 second with parser errors: %v", trimmedErrors)
			case <-done:
			}
		})
	}
}

func parseSingleStatement(t *testing.T, input string) Statement {
	t.Helper()
	p := New(lexer.New(input))
	program := p.ParseProgram()
	if len(p.Errors) != 0 {
		t.Fatalf("parser errors: %v", p.Errors)
	}
	if len(program.Statements) != 1 {
		t.Fatalf("expected one statement, got %d", len(program.Statements))
	}
	return program.Statements[0]
}

func runExpressionShapeTests(t *testing.T, tests []InputOutput) {
	t.Helper()
	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			// Exercise EOF and block/semicolon boundaries, including the next
			// statement so a parser cannot silently swallow trailing tokens.
			for _, inFunction := range []bool{false, true} {
				name, input := "standalone", test.input
				if inFunction {
					name = "function_body"
					input = "fnc probe() -> none { " + input + "; sentinel() }"
				}
				t.Run(name, func(t *testing.T) {
					p := New(lexer.New(input))
					program := p.ParseProgram()
					if len(p.Errors) != 0 {
						t.Fatalf("parser errors: %v", p.Errors)
					}
					if len(program.Statements) != 1 {
						t.Fatalf("expected one top-level statement, got %d", len(program.Statements))
					}
					stmt := program.Statements[0]
					if inFunction {
						fn, ok := stmt.(*FunctionStatement)
						if !ok {
							t.Fatalf("expected function, got %T", stmt)
						}
						if len(fn.Body.Statements) != 2 {
							t.Fatalf("expected expression and sentinel, got %d statements", len(fn.Body.Statements))
						}
						if got := statementExpressionShape(t, fn.Body.Statements[1]); got != "call(sentinel)" {
							t.Fatalf("following statement changed: %s", got)
						}
						stmt = fn.Body.Statements[0]
					}
					if got := statementExpressionShape(t, stmt); got != test.output {
						t.Fatalf("input: %s\nwant: %s\n got: %s", input, test.output, got)
					}
				})
			}
		})
	}
}

func statementExpressionShape(t *testing.T, stmt Statement) string {
	t.Helper()
	expr, ok := stmt.(*ExpressionStatement)
	if !ok {
		t.Fatalf("expected expression statement, got %T", stmt)
	}
	return expressionShape(t, expr.Expression)
}

// Preserve node grouping that Expression.String() omits for casts and dereferences.
func expressionShape(t *testing.T, expr Expression) string {
	t.Helper()
	shape := func(e Expression) string { return expressionShape(t, e) }
	switch e := expr.(type) {
	case *IdentifierExpression:
		return e.Value
	case *InfixExpression:
		return fmt.Sprintf("%s(%s, %s)", e.Operator, shape(e.Left), shape(e.Right))
	case *CastExpression:
		return fmt.Sprintf("cast(%s, %s)", e.Type, shape(e.Expr))
	case *SizeofExpression:
		return "sizeof(" + e.Type.String() + ")"
	case *PrefixExpression:
		return fmt.Sprintf("prefix(%s, %s)", e.Operator, shape(e.Right))
	case *DereferenceExpression:
		return "deref(" + shape(e.Var) + ")"
	case *ReferenceExpression:
		if e.Var == nil {
			t.Fatal("reference has no operand")
		}
		return "ref(" + shape(e.Var) + ")"
	case *AssignmentExpression:
		return fmt.Sprintf("assign(%s, %s)", shape(e.Left), shape(e.Right))
	case *CallExpression:
		parts := []string{shape(e.Function)}
		for _, param := range e.Params {
			parts = append(parts, shape(param))
		}
		return "call(" + strings.Join(parts, ", ") + ")"
	default:
		t.Fatalf("unexpected expression node %T", expr)
		return ""
	}
}

func TestStructFieldsPreserveDuplicates(t *testing.T) {
	stmt := parseSingleStatement(t, "struct Item {\nint32 z\nbool a\nchar* z\n}").(*StructStatement)
	wantNames := []string{"z", "a", "z"}
	wantTypes := []lexer.VarType{{Base: lexer.Int32}, {Base: lexer.Bool}, {Base: lexer.Char, Pointer: 1}}
	if len(stmt.Fields) != len(wantNames) {
		t.Fatalf("want 3 fields, got %d", len(stmt.Fields))
	}
	for i, field := range stmt.Fields {
		if field.Name.Value != wantNames[i] || field.Type != wantTypes[i] {
			t.Errorf("field %d: want %s %v, got %s %v", i, wantNames[i], wantTypes[i], field.Name.Value, field.Type)
		}
		if field.Name.Position().StartLine != uint32(i+2) {
			t.Errorf("field %d: wrong name position: %+v", i, field.Name.Position())
		}
	}
	if got, want := stmt.String(), "struct Item{Int32 z;Bool a;Char* z;}"; got != want {
		t.Errorf("want %q, got %q", want, got)
	}
}
