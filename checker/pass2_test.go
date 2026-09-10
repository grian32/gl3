package checker

import (
	ast "gl3/checkedast"
	"testing"
)

// These cases specify pass 2 output before its implementation.
var pass2ExpressionTests = []struct {
	name, source string
	want         ast.Expr
}{
	{name: "signed minimum", source: `fnc sample() -> int8 { return -128i8 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int8}}, Value: 128}},
	{name: "int64 minimum", source: `fnc sample() -> int { return -9223372036854775808 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int}}, Value: 1 << 63}},
	{name: "uint64 maximum", source: `fnc sample() -> uint { return 18446744073709551615u64 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint}}, Value: ^uint64(0)}},
	{name: "negative one bits", source: `fnc sample() -> int16 { return -1i16 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int16}}, Value: 65535}},
	{name: "int literal", source: `fnc sample() -> int { return 7 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int}}, Value: 7}},
	{name: "int32 literal", source: `fnc sample() -> int32 { return 7i32 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}},
	{name: "int16 literal", source: `fnc sample() -> int16 { return 7i16 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int16}}, Value: 7}},
	{name: "int8 literal", source: `fnc sample() -> int8 { return 7i8 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int8}}, Value: 7}},
	{name: "uint literal", source: `fnc sample() -> uint { return 7u64 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint}}, Value: 7}},
	{name: "uint32 literal", source: `fnc sample() -> uint32 { return 7u32 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Value: 7}},
	{name: "uint16 literal", source: `fnc sample() -> uint16 { return 7u16 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint16}}, Value: 7}},
	{name: "uint8 literal", source: `fnc sample() -> uint8 { return 7u8 }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint8}}, Value: 7}},
	{name: "boolean literal", source: `fnc sample() -> bool { return true }`, want: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}},
	{name: "float literal", source: `fnc sample() -> float { return 1.5 }`, want: &ast.FloatLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Value: 1.5}},
	{name: "character literal", source: `fnc sample() -> int8 { return 'A' }`, want: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int8}}, Value: 65}},
	{name: "string literal", source: `fnc sample() -> char* { return "hello" }`, want: &ast.StringLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Char, Pointer: 1}}, Value: "hello\x00"}},
	{name: "IntNegate", source: `fnc sample(int32 x) -> int32 { return -x }`, want: &ast.Unary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Op: ast.IntNegate, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}},
	{name: "FloatNegate", source: `fnc sample(float x) -> float { return -x }`, want: &ast.Unary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Op: ast.FloatNegate, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}}},
	{name: "BoolNot", source: `fnc sample(bool x) -> bool { return !x }`, want: &ast.Unary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.BoolNot, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 0}}},
	{name: "int32 IntAdd", source: `fnc sample(int32 a, int32 b) -> int32 { return a + b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Op: ast.IntAdd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 IntSubtract", source: `fnc sample(int32 a, int32 b) -> int32 { return a - b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Op: ast.IntSubtract, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 IntMultiply", source: `fnc sample(int32 a, int32 b) -> int32 { return a * b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Op: ast.IntMultiply, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 SignedDivide", source: `fnc sample(int32 a, int32 b) -> int32 { return a / b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Op: ast.SignedDivide, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 IntEqual", source: `fnc sample(int32 a, int32 b) -> bool { return a == b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.IntEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 IntNotEqual", source: `fnc sample(int32 a, int32 b) -> bool { return a != b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.IntNotEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 SignedLess", source: `fnc sample(int32 a, int32 b) -> bool { return a < b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.SignedLess, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 SignedLessEqual", source: `fnc sample(int32 a, int32 b) -> bool { return a <= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.SignedLessEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 SignedGreater", source: `fnc sample(int32 a, int32 b) -> bool { return a > b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.SignedGreater, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "int32 SignedGreaterEqual", source: `fnc sample(int32 a, int32 b) -> bool { return a >= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.SignedGreaterEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 1}}},
	{name: "uint32 IntAdd", source: `fnc sample(uint32 a, uint32 b) -> uint32 { return a + b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Op: ast.IntAdd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 IntSubtract", source: `fnc sample(uint32 a, uint32 b) -> uint32 { return a - b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Op: ast.IntSubtract, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 IntMultiply", source: `fnc sample(uint32 a, uint32 b) -> uint32 { return a * b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Op: ast.IntMultiply, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 UnsignedDivide", source: `fnc sample(uint32 a, uint32 b) -> uint32 { return a / b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Op: ast.UnsignedDivide, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 IntEqual", source: `fnc sample(uint32 a, uint32 b) -> bool { return a == b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.IntEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 IntNotEqual", source: `fnc sample(uint32 a, uint32 b) -> bool { return a != b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.IntNotEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 UnsignedLess", source: `fnc sample(uint32 a, uint32 b) -> bool { return a < b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.UnsignedLess, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 UnsignedLessEqual", source: `fnc sample(uint32 a, uint32 b) -> bool { return a <= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.UnsignedLessEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 UnsignedGreater", source: `fnc sample(uint32 a, uint32 b) -> bool { return a > b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.UnsignedGreater, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "uint32 UnsignedGreaterEqual", source: `fnc sample(uint32 a, uint32 b) -> bool { return a >= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.UnsignedGreaterEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 1}}},
	{name: "float FloatAdd", source: `fnc sample(float a, float b) -> float { return a + b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Op: ast.FloatAdd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatSubtract", source: `fnc sample(float a, float b) -> float { return a - b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Op: ast.FloatSubtract, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatMultiply", source: `fnc sample(float a, float b) -> float { return a * b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Op: ast.FloatMultiply, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatDivide", source: `fnc sample(float a, float b) -> float { return a / b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Op: ast.FloatDivide, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatEqual", source: `fnc sample(float a, float b) -> bool { return a == b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatNotEqual", source: `fnc sample(float a, float b) -> bool { return a != b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatNotEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatLess", source: `fnc sample(float a, float b) -> bool { return a < b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatLess, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatLessEqual", source: `fnc sample(float a, float b) -> bool { return a <= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatLessEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatGreater", source: `fnc sample(float a, float b) -> bool { return a > b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatGreater, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "float FloatGreaterEqual", source: `fnc sample(float a, float b) -> bool { return a >= b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.FloatGreaterEqual, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 1}}},
	{name: "BoolAnd", source: `fnc sample(bool a, bool b) -> bool { return a && b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.BoolAnd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 1}}},
	{name: "BoolOr", source: `fnc sample(bool a, bool b) -> bool { return a || b }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Op: ast.BoolOr, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 1}}},
	{name: "IdentityCast", source: `fnc sample(int32 x) -> int32 { return x as int32 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Kind: ast.IdentityCast, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}},
	{name: "SignExtend", source: `fnc sample(int8 x) -> int32 { return x as int32 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Kind: ast.SignExtend, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int8}}, ID: 0}}},
	{name: "ZeroExtend", source: `fnc sample(uint8 x) -> uint32 { return x as uint32 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Kind: ast.ZeroExtend, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint8}}, ID: 0}}},
	{name: "Truncate", source: `fnc sample(int32 x) -> int8 { return x as int8 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int8}}, Kind: ast.Truncate, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}},
	{name: "SignedIntToFloat", source: `fnc sample(int32 x) -> float { return x as float }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Kind: ast.SignedIntToFloat, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}},
	{name: "UnsignedIntToFloat", source: `fnc sample(uint32 x) -> float { return x as float }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, Kind: ast.UnsignedIntToFloat, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, ID: 0}}},
	{name: "FloatToSignedInt", source: `fnc sample(float x) -> int32 { return x as int32 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Kind: ast.FloatToSignedInt, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}}},
	{name: "FloatToUnsignedInt", source: `fnc sample(float x) -> uint32 { return x as uint32 }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint32}}, Kind: ast.FloatToUnsignedInt, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Float}}, ID: 0}}},
	{name: "PointerToInt", source: `fnc sample(int32* x) -> uint { return x as uint }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint}}, Kind: ast.PointerToInt, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}}},
	{name: "IntToPointer", source: `fnc sample(uint x) -> int32* { return x as int32* }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, Kind: ast.IntToPointer, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint}}, ID: 0}}},
	{name: "PointerCast", source: `fnc sample(int32* x) -> char* { return x as char* }`, want: &ast.Cast{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Char, Pointer: 1}}, Kind: ast.PointerCast, Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}}},
	{name: "dereference", source: `fnc sample(int32* x) -> int32 { return *x }`, want: &ast.Dereference{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Pointer: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}}},
	{name: "address local", source: `fnc sample(int32 x) -> int32* { return &x }`, want: &ast.AddressOf{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, Target: &ast.LocalPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}},
	{name: "PointerAdd", source: `fnc sample(int32* x, int i) -> int32* { return x + i }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, Op: ast.PointerAdd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int}}, ID: 1}}},
	{name: "PointerSubtract", source: `fnc sample(int32* x, int i) -> int32* { return x - i }`, want: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, Op: ast.PointerSubtract, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int}}, ID: 1}}},
	{name: "index lowers to pointer add and dereference", source: `fnc sample(int32* x, int i) -> int32 { return x[i] }`, want: &ast.Dereference{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Pointer: &ast.Binary{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, Op: ast.PointerAdd, Left: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}, Right: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int}}, ID: 1}}}},
	{name: "sizeof", source: `fnc sample() -> uint { return sizeof int32 }`, want: &ast.Sizeof{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Uint}}, OperandType: ast.Type{Base: ast.Int32}}},
	{name: "array literal", source: `fnc sample() -> int32* { return [int32; 1i32, 2i32] }`, want: &ast.ArrayLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ElementType: ast.Type{Base: ast.Int32}, Items: []ast.Expr{&ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 1}, &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}},
	{name: "struct literal", source: `struct Pair { int32 x bool y } fnc sample() -> Pair { return Pair:{1i32,true} }`, want: &ast.StructLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.StructType}}, Fields: []ast.Expr{&ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 1}, &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}}}},
	{name: "field index follows declaration order", source: `struct Pair { bool x int32 y } fnc sample(Pair p) -> int32 { return p.y }`, want: &ast.FieldAccess{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Base: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.StructType}}, ID: 0}, FieldIndex: 1}},
	{name: "forward call", source: `fnc sample() -> int32 { return later(7i32) } fnc later(int32 x) -> int32 { return x }`, want: &ast.Call{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Function: 1, Args: []ast.Expr{&ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}}}},
	{name: "recursive call", source: `fnc sample(int32 x) -> int32 { return sample(x) }`, want: &ast.Call{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Function: 0, Args: []ast.Expr{&ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}}},
	{name: "assignment expression", source: `fnc sample(int32 x) -> int32 { return x = 7i32 }`, want: &ast.Assignment{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Target: &ast.LocalPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}}},
}

func TestPass2Expressions(t *testing.T) {
	for _, test := range pass2ExpressionTests {
		t.Run(test.name, func(t *testing.T) {
			got, diagnostics := NewChecker().CheckProgram(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, nil)
			if got == nil || len(got.Functions) == 0 {
				t.Fatal("missing checked function")
			}
			want := []ast.Stmt{&ast.Return{Value: test.want}}
			assertSlice(t, "body", got.Functions[0].Body.Statements, want)
		})
	}
}

var pass2DiagnosticTests = []struct {
	name, source string
	diagnostics  []expectedDiagnostic
}{
	{name: "signed underflow", source: `fnc sample() -> int8 { return -129i8 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Int8"}, line: 1}}},
	{name: "int64 overflow", source: `fnc sample() -> int { return 9223372036854775808 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Int"}, line: 1}}},
	{name: "int64 underflow", source: `fnc sample() -> int { return -9223372036854775809 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Int"}, line: 1}}},
	{name: "negative unsigned literal", source: `fnc sample() -> uint8 { return -1u8 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Uint8"}, line: 1}}},
	{name: "calling an opaque return function requires a sized value", source: `extern struct Handle
extern fnc create() -> Handle
fnc use() -> none { create() }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 3}}},
	{name: "opaque pointer arithmetic requires element size", source: `extern struct Handle
fnc next(Handle* p) -> Handle* { return p + 1 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 2}}},
	{name: "opaque dereference cannot load a value", source: `extern struct Handle
fnc read(Handle* p) -> none { *p }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 2}}},
	{name: "sizeof struct containing opaque field requires a sized type", source: `extern struct Handle
struct Wrapper { Handle handle }
fnc size() -> uint { return sizeof Wrapper }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Wrapper", "size"}, line: 3}}},
	{name: "opaque sizeof", source: `extern struct Handle
fnc size() -> uint { return sizeof Handle }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 2}}},
	{name: "opaque field access", source: `extern struct Handle
fnc read(Handle* handle) -> int32 { return (*handle).value }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 2}}},
	{name: "opaque struct literal", source: `extern struct Handle
fnc create() -> none { Handle:{} }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Handle", "opaque"}, line: 2}}},

	{
		name:        "top level expression is forbidden",
		source:      `1i32`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"top level"}, line: 1}},
	},
	{
		name:        "top level local declaration is forbidden",
		source:      `def int32 value = 1i32`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"top level"}, line: 1}},
	},
	{
		name:        "top level control flow is forbidden",
		source:      `if true { }`,
		diagnostics: []expectedDiagnostic{{messageContains: []string{"top level"}, line: 1}},
	},

	{name: "too many arguments", source: `fnc other() -> int32 { return 0i32 }
fnc sample() -> int32 { return other(1i32) }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"argument"}, line: 2}}},
	{name: "void call used as value", source: `fnc other() -> none {}
fnc sample() -> int32 { return other() }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"none", "int32"}, line: 2}}},
	{name: "global runtime initializer", source: `fnc other() -> int32 { return 0i32 }
global int32 x = other()`, diagnostics: []expectedDiagnostic{{messageContains: []string{"constant"}, line: 2}}},
	{name: "global unknown initializer", source: `
global int32 x = missing`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "missing"}, line: 2}}},
	{name: "pointer plus pointer", source: `
fnc sample(int32* a, int32* b) -> int32* { return a + b }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"pointer"}, line: 2}}},
	{name: "float index", source: `
fnc sample(int32* a) -> int32 { return a[1.5] }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"integer"}, line: 2}}},
	{name: "missing return in one branch", source: `
fnc sample(bool x) -> int32 { if x { return 1i32 } }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"return"}, line: 2}}},
	{name: "empty struct initializer", source: `struct S { int32 x }
fnc sample() -> S { return S:{} }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"field"}, line: 2}}},
	{name: "cast unknown type", source: `
fnc sample(int32 x) -> int32 { return x as Missing }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Missing"}, line: 2}}},
	{name: "cast to none", source: `
fnc sample(int32 x) -> none { x as none }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"none"}, line: 2}}},
	{name: "unsigned overflow", source: `
fnc sample() -> uint8 { return 256u8 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Uint8"}, line: 2}}},
	{name: "sizeof unknown type", source: `
fnc sample() -> uint { return sizeof Missing }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Missing"}, line: 2}}},
	{name: "array invalid element type", source: `
fnc sample() -> none { [Missing;] }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"Missing"}, line: 2}}},
	{name: "constant field assignment", source: `struct S { int32 x } global const S value = S:{1i32}
fnc sample() -> none { value.x = 2i32 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"constant", "value"}, line: 2}}},
	{name: "unknown local", source: `fnc sample() -> int32 {
 return missing
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "missing"}, line: 2}}},
	{name: "unknown call", source: `fnc sample() -> int32 {
 return missing()
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "missing"}, line: 2}}},
	{name: "local type", source: `fnc sample() -> int32 {
 def Missing x = 1i32 return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"type", "Missing"}, line: 2}}},
	{name: "void local", source: `fnc sample() -> int32 {
 def none x = 1i32 return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"none"}, line: 2}}},
	{name: "local initializer mismatch", source: `fnc sample() -> int32 {
 def bool x = 1i32 return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool", "int32"}, line: 2}}},
	{name: "return mismatch", source: `fnc sample() -> int32 {
 return true
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"return", "int32", "bool"}, line: 2}}},
	{name: "missing return", source: `fnc sample() -> int32 {
 
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"return"}, line: 1}}},
	{name: "nonboolean if", source: `fnc sample() -> int32 {
 if 1i32 { return 0i32 } return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "nonboolean while", source: `fnc sample() -> int32 {
 while 1i32 { break } return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "break outside loop", source: `fnc sample() -> int32 {
 break return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"break", "loop"}, line: 2}}},
	{name: "continue outside loop", source: `fnc sample() -> int32 {
 continue return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"continue", "loop"}, line: 2}}},
	{name: "wrong binary operands", source: `fnc sample() -> int32 {
 return 1i32 + true
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "arithmetic boolean", source: `fnc sample() -> int32 {
 def bool x = true + false return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "logical integer", source: `fnc sample() -> int32 {
 def bool x = 1i32 && 2i32 return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "negate boolean", source: `fnc sample() -> int32 {
 def bool x = -true return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "not integer", source: `fnc sample() -> int32 {
 def bool x = !1i32 return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"bool"}, line: 2}}},
	{name: "dereference integer", source: `fnc sample() -> int32 {
 return *1i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"pointer"}, line: 2}}},
	{name: "address temporary", source: `fnc sample() -> int32 {
 def int32* x = &(1i32 + 2i32) return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"address"}, line: 2}}},
	{name: "assignment mismatch", source: `fnc sample() -> int32 {
 def int32 x = 1i32 x = true return x
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "nonstruct field", source: `fnc sample() -> int32 {
 def int32 x = 1i32 return x.missing
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"field"}, line: 2}}},
	{name: "array item mismatch", source: `fnc sample() -> int32 {
 def int32* x = [int32; true] return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "integer overflow", source: `fnc sample() -> int32 {
 return 2147483648i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"invalid value", "Int32"}, line: 2}}},
	{name: "branch scope exit", source: `fnc sample() -> int32 {
 if true { def int32 hidden = 1i32 } return hidden
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "hidden"}, line: 2}}},
	{name: "sibling scope isolation", source: `fnc sample() -> int32 {
 if true { def int32 hidden = 1i32 } else { def int32 x = hidden } return 0i32
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "hidden"}, line: 2}}},
	{name: "loop scope exit", source: `fnc sample() -> int32 {
 while false { def int32 hidden = 1i32 } return hidden
}`, diagnostics: []expectedDiagnostic{{messageContains: []string{"unknown", "hidden"}, line: 2}}},
	{name: "call arity", source: `fnc other(int32 x) -> int32 { return x }
fnc sample() -> int32 { return other() }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"argument"}, line: 2}}},
	{name: "call argument type", source: `fnc other(int32 x) -> int32 { return x }
fnc sample() -> int32 { return other(true) }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "call nonfunction", source: `global int32 x = 1i32
fnc sample() -> int32 { return x() }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"x", "function"}, line: 2}}},
	{name: "constant assignment", source: `global const int32 x = 1i32
fnc sample() -> int32 { x = 2i32 return x }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"constant", "x"}, line: 2}}},
	{name: "unknown field", source: `struct S { int32 x }
fnc sample(S s) -> int32 { return s.missing }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"field", "missing"}, line: 2}}},
	{name: "struct initializer type", source: `struct S { int32 x }
fnc sample() -> S { return S:{true} }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "struct initializer count", source: `struct S { int32 x }
fnc sample() -> S { return S:{1i32,2i32} }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"field"}, line: 2}}},
	{name: "global initializer mismatch", source: `
global int32 x = true`, diagnostics: []expectedDiagnostic{{messageContains: []string{"int32", "bool"}, line: 2}}},
	{name: "void function returns value", source: `
fnc sample() -> none { return 1i32 }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"return"}, line: 2}}},
	{name: "loop context reset between functions", source: `fnc first() -> none { while true { break } }
fnc second() -> none { break }`, diagnostics: []expectedDiagnostic{{messageContains: []string{"break", "loop"}, line: 2}}},
}

func TestPass2Diagnostics(t *testing.T) {
	for _, test := range pass2DiagnosticTests {
		t.Run(test.name, func(t *testing.T) {
			_, diagnostics := NewChecker().CheckProgram(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, test.diagnostics)
		})
	}
}

func TestPass2FixturesParse(t *testing.T) {
	for _, test := range pass2ProgramTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
	for _, test := range pass2GlobalTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
	for _, test := range pass2ExpressionTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
	for _, test := range pass2DiagnosticTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
	for _, test := range pass2BodyTests {
		t.Run(test.name, func(t *testing.T) { parseDeclarations(t, test.source) })
	}
}

func TestPass2Bodies(t *testing.T) {
	for _, test := range pass2BodyTests {
		t.Run(test.name, func(t *testing.T) {
			got, diagnostics := NewChecker().CheckProgram(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, nil)
			if got == nil || len(got.Functions) == 0 {
				t.Fatal("missing checked function")
			}
			assertSlice(t, "locals", got.Functions[0].Locals, test.locals)
			assertSlice(t, "body", got.Functions[0].Body.Statements, test.body)
		})
	}
}

var pass2BodyTests = []struct {
	name, source string
	locals       []ast.Local
	body         []ast.Stmt
}{
	{name: "if else returns", source: `fnc sample(bool x) -> int32 { if x { return 1i32 } else { return 2i32 } }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.Bool}}}, body: []ast.Stmt{&ast.If{Condition: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, ID: 0}, Then: ast.Block{Statements: []ast.Stmt{&ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 1}}}}, Else: &ast.Block{Statements: []ast.Stmt{&ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}}}}},
	{name: "while break", source: `fnc sample() -> int32 { while true { break } return 0i32 }`, locals: []ast.Local{}, body: []ast.Stmt{&ast.While{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}, Body: ast.Block{Statements: []ast.Stmt{&ast.Break{}}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "while continue", source: `fnc sample() -> int32 { while false { continue } return 0i32 }`, locals: []ast.Local{}, body: []ast.Stmt{&ast.While{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: false}, Body: ast.Block{Statements: []ast.Stmt{&ast.Continue{}}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "nested loop contexts", source: `fnc sample() -> int32 { while true { while true { break } continue } return 0i32 }`, locals: []ast.Local{}, body: []ast.Stmt{&ast.While{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}, Body: ast.Block{Statements: []ast.Stmt{&ast.While{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}, Body: ast.Block{Statements: []ast.Stmt{&ast.Break{}}}}, &ast.Continue{}}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "void call statement", source: `fnc sample() -> int32 { other() return 0i32 } fnc other() -> none {}`, locals: []ast.Local{}, body: []ast.Stmt{&ast.ExpressionStatement{Expr: &ast.Call{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Void}}, Function: 1}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "local assignment", source: `fnc sample(int32 x) -> int32 { x = 2i32 return x }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.Int32}}}, body: []ast.Stmt{&ast.ExpressionStatement{Expr: &ast.Assignment{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Target: &ast.LocalPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}, &ast.Return{Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}}},
	{name: "pointer assignment", source: `fnc sample(int32* x) -> int32 { *x = 2i32 return 0i32 }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.Int32, Pointer: 1}}}, body: []ast.Stmt{&ast.ExpressionStatement{Expr: &ast.Assignment{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Target: &ast.DerefPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Pointer: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32, Pointer: 1}}, ID: 0}}, Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "global assignment", source: `global int32 x = 1i32 fnc sample() -> int32 { x = 2i32 return 0i32 }`, locals: []ast.Local{}, body: []ast.Stmt{&ast.ExpressionStatement{Expr: &ast.Assignment{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Target: &ast.GlobalPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}, Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "field assignment", source: `struct S { bool a int32 b } fnc sample(S x) -> int32 { x.b = 2i32 return 0i32 }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.StructType}}}, body: []ast.Stmt{&ast.ExpressionStatement{Expr: &ast.Assignment{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Target: &ast.FieldPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Base: &ast.LocalPlace{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.StructType}}, ID: 0}, FieldIndex: 1}, Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
	{name: "nested shadow restores outer binding", source: `fnc sample(int32 x) -> int32 { if true { def bool x = false } return x }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.Int32}}, {Name: "x", Type: ast.Type{Base: ast.Bool}}}, body: []ast.Stmt{&ast.If{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}, Then: ast.Block{Statements: []ast.Stmt{&ast.LocalDeclaration{ID: 1, Initializer: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: false}}}}}, &ast.Return{Value: &ast.LocalRef{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, ID: 0}}}},
	{name: "sibling declarations have distinct local IDs", source: `fnc sample() -> int32 { if true { def int32 x = 1i32 } else { def int32 x = 2i32 } return 0i32 }`, locals: []ast.Local{{Name: "x", Type: ast.Type{Base: ast.Int32}}, {Name: "x", Type: ast.Type{Base: ast.Int32}}}, body: []ast.Stmt{&ast.If{Condition: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}, Then: ast.Block{Statements: []ast.Stmt{&ast.LocalDeclaration{ID: 0, Initializer: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 1}}}}, Else: &ast.Block{Statements: []ast.Stmt{&ast.LocalDeclaration{ID: 1, Initializer: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 2}}}}}, &ast.Return{Value: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 0}}}},
}

var pass2GlobalTests = []struct {
	name, source string
	want         []ast.Global
}{
	{name: "mutable integer", source: `global int32 value = 7i32`, want: []ast.Global{{Name: "value", Id: 0, Constant: false, Type: ast.Type{Base: ast.Int32}, Initializer: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}}}},
	{name: "constant integer", source: `global const int32 value = 7i32`, want: []ast.Global{{Name: "value", Id: 0, Constant: true, Type: ast.Type{Base: ast.Int32}, Initializer: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}}}},
	{name: "boolean", source: `global bool value = true`, want: []ast.Global{{Name: "value", Id: 0, Constant: false, Type: ast.Type{Base: ast.Bool}, Initializer: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}}}},
	{name: "struct", source: `struct S { int32 x } global S value = S:{7i32}`, want: []ast.Global{{Name: "value", Id: 0, Constant: false, Type: ast.Type{Base: ast.StructType}, Initializer: &ast.StructLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.StructType}}, Fields: []ast.Expr{&ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 7}}}}}},
}

func TestPass2Globals(t *testing.T) {
	for _, test := range pass2GlobalTests {
		t.Run(test.name, func(t *testing.T) {
			got, diagnostics := NewChecker().CheckProgram(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, nil)
			if got == nil {
				t.Fatal("missing checked program")
			}
			assertSlice(t, "globals", got.Globals, test.want)
		})
	}
}

var pass2ProgramTests = []struct {
	name, source string
	want         ast.Program
}{

	{
		name:   "global initializers preserve declaration IDs and types",
		source: `global int32 first = 1i32 global const bool second = true`,
		want: ast.Program{Globals: []ast.Global{
			{Name: "first", Id: 0, Type: ast.Type{Base: ast.Int32}, Initializer: &ast.IntegerLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Int32}}, Value: 1}},
			{Name: "second", Id: 1, Constant: true, Type: ast.Type{Base: ast.Bool}, Initializer: &ast.BooleanLiteral{ExprInfo: ast.ExprInfo{ResultType: ast.Type{Base: ast.Bool}}, Value: true}},
		}},
	},
}

func TestPass2Program(t *testing.T) {
	for _, test := range pass2ProgramTests {
		t.Run(test.name, func(t *testing.T) {
			got, diagnostics := NewChecker().CheckProgram(parseDeclarations(t, test.source))
			assertDiagnostics(t, diagnostics, nil)
			assertDeclarations(t, got, &test.want)
		})
	}
}
