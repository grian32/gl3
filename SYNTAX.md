# GL3 Syntax Reference

## Lexical Elements

### Comments

Single-line comments begin with `//` and extend to the end of the line.

```gl3
// this is a comment
def int32 x = 10i32  // inline comment
```

### Semicolons

Semicolons are optional statement terminators. They may be omitted entirely.

Global variable definitions are not statement terminators and therefore cannot end with `;`.

```gl3
def int32 x = 10i32
def int32 y = 20i32
x = 30i32

// with semicolons (also valid)
def int32 a = 10i32;
def int32 b = 20i32;
```

## Imports

Import statements load standard library modules or other GL3 source files.

For `.gl3` file imports, the imported names are the target file's top-level function definitions, struct definitions, and global constant definitions.
Standard library modules may expose whatever they want, though in practice they usually only import functions.

```gl3
import "module"      // standard library module
import "file.gl3"    // GL3 source file
```

Example:

```gl3
// math.gl3
global int answer = 42

struct Pair {
    int left
    int right
}

func int add(int a, int b) {
    return a + b
}

// main.gl3
import "math.gl3"

def int x = add(answer, 8)
def Pair p = Pair:{ 1, 2 }
```

## Data Types

### Primitive Types

| Type     | Description                  |
| -------- | ---------------------------- |
| `int8`   | 8-bit signed integer         |
| `int16`  | 16-bit signed integer        |
| `int32`  | 32-bit signed integer        |
| `int`    | 64-bit signed integer        |
| `uint8`  | 8-bit unsigned integer       |
| `uint16` | 16-bit unsigned integer      |
| `uint32` | 32-bit unsigned integer      |
| `uint`   | 64-bit unsigned integer      |
| `char`   | 8-bit value (alias for int8) |
| `bool`   | boolean (true or false)      |
| `float`  | 32-bit floating point        |
| `none`   | void type (function returns) |

### Pointer Types

Append `*` to any type to form a pointer type. Multiple indirection levels are supported.

```gl3
def int x = 10
def int* ptr = &x
def int** ptr_to_ptr = &ptr
```

### Struct Types

User-defined composite types.

```gl3
struct Point {
    int32 x
    int32 y
}
```

Struct fields may themselves be struct types or pointers.

```gl3
struct Node {
    int32 value
    Node* next
}
```

## Literals

### Integer Literals

Integer literals specify their type via suffixes. A bare number without suffix is a 64-bit signed integer.

```gl3
1       // int (64-bit signed)
1i8     // int8
1i16    // int16
1i32    // int32
1u8     // uint8
1u16    // uint16
1u32    // uint32
1u64    // uint (64-bit unsigned)
```

### Floating Point Literals

```gl3
1.5     // float (32-bit)
3.14    // float
```

### Character Literals

```gl3
'a'     // char literal (int8 value 97)
'Z'     // char literal (int8 value 90)
```

### String Literals

String literals are null-terminated and stored in read-only memory as `char*`.

```gl3
"hello"     // static string in rodata
```

### Boolean Literals

```gl3
true
false
```

### Array Literals

Array literals are syntactic sugar that expand to dynamic array operations.

```gl3
[int32; 1i32, 2i32, 3i32]
```

This is equivalent to creating a new array and pushing each element. Requires the `arrays` module.

## Variable Declarations

Variables are declared with `def`, followed by type, identifier, and initial value.

```gl3
def int32 x = 10i32
def float pi = 3.14
def bool flag = true
def char* message = "hello"
```

Struct instances:

```gl3
def Point p = Point:{ 10i32, 20i32 }
```

> **Note**: The `:` part of the struct initialization statement is largely redundant from an abstract point of view but the lack of it leads to parsing ambiguity with syntax along the lines of `if x { }`.

Pointers:

```gl3
def int x = 10
def int* ptr = &x
```

### Global Variable Declarations

Global variables are declared with `global`, using the same form as `def` (`type`, name, and initializer).

```gl3
global int32 max_items = 128i32
global char* app_name = "grianlang"
```

> **Note**: Global variable definitions are top-level declarations outside functions, so they cannot end with `;`.

## Assignment

Reassignment uses `=`.

```gl3
x = 20i32
ptr = &x
*ptr = 30i32                    // dereference and assign
struct_instance.field = value   // struct field assignment
```

## Operators

### Prefix Operators

| Operator | Description | Example |
| -------- | ----------- | ------- |
| `-`      | Negation    | `-x`    |
| `!`      | Logical NOT | `!flag` |
| `&`      | Address-of  | `&x`    |
| `*`      | Dereference | `*ptr`  |

### Infix Operators

| Operator | Description           | Example    |
| -------- | --------------------- | ---------- |
| `+`      | Addition              | `a + b`    |
| `-`      | Subtraction           | `a - b`    |
| `*`      | Multiplication        | `a * b`    |
| `/`      | Division              | `a / b`    |
| `==`     | Equality              | `a == b`   |
| `!=`     | Inequality            | `a != b`   |
| `<`      | Less than             | `a < b`    |
| `>`      | Greater than          | `a > b`    |
| `<=`     | Less than or equal    | `a <= b`   |
| `>=`     | Greater than or equal | `a >= b`   |
| `&&`     | Logical AND           | `a && b`   |
| `\|\|`   | Logical OR            | `a \|\| b` |

### Operator Precedence (lowest to highest)

1. Assignment (`=`)
2. Logical OR (`||`)
3. Logical AND (`&&`)
4. Equality (`==`, `!=`)
5. Comparison (`<`, `>`, `<=`, `>=`)
6. Cast (`as`)
7. Addition/Subtraction (`+`, `-`)
8. Multiplication/Division (`*`, `/`)
9. Prefix operators (`!`, `-`, `&`, `*`)
10. Function call, member access, struct initialization
11. Array indexing

## Type Casting

Cast between compatible types using `as`.

```gl3
def int32 x = 10i32
def int64 y = x as int
def int8 small = 255u8 as int8
def int32* ptr = (arr_new((sizeof int32))) as int32*
```

## Sizeof Expression

Returns the byte size of a type.

```gl3
def int size = sizeof int32
def int struct_size = sizeof Point
```

## Control Flow

### If Statements

```gl3
if condition {
    // executed when true
}

if condition {
    // executed when true
} else {
    // executed when false
}
```

### While Loops

```gl3
while condition {
    // repeated while condition is true
}
```

`break` and `continue` are statements for `while` loops:

- `break` exits the nearest enclosing `while` loop.
- `continue` skips to the next iteration of the nearest enclosing `while` loop.

```gl3
while i < 10i32 {
    if i == 3i32 {
        i = i + 1i32
        continue
    }

    if i == 8i32 {
        break
    }

    i = i + 1i32
}
```

## Functions

Functions use the `fnc` keyword. Return type is always required.

- Functions returning `none` receive an implicit return
- All other functions require an explicit `return` statement

Functions returning `none` also allow an explicit bare `return` with no value.
Use `return;` when another statement follows; a newline alone does not end the
return statement. The semicolon is optional immediately before `}`.
Functions returning a value, including pointer types such as `none*`, require
a return expression matching their declared return type.

```gl3
fnc add(int32 a, int32 b) -> int32 {
    return a + b
}

fnc greet() -> none {
    // implicit return
}

fnc finish() -> none {
    return
}

fnc main() -> int32 {
    return 0i32
}
```

Parameters are specified as `type name` pairs:

```gl3
fnc process(int32 count, char* data, bool flag) -> none {
    // ...
}
```

## Structs

### Definition

```gl3
struct Person {
    int32 age
    bool employed
}
```

### Initialization

Positional initialization only. Fields are specified in declaration order.

```gl3
def Person p = Person{ 25i32, true }
```

### Field Access

Dot notation works on both values and pointers transparently (unlike C which requires `->` for pointers).

```gl3
def int32 age = p.age
p.age = 26i32

def Person* ptr = &p
def int32 age_from_ptr = ptr.age    // no -> needed
ptr.age = 30i32
```

## Pointers

Traditional pointer operations.

```gl3
def int x = 10
def int* ptr = &x        // address-of
def int value = *ptr     // dereference
*ptr = 20                // assign through pointer
```

### Array Indexing

Syntactic sugar for pointer arithmetic with dereference.

```gl3
arr[i]          // equivalent to *(arr + i)
arr[0] = 5i32   // assign to first element
```

## Program Structure

Typical structure:

1. Import statements
2. Struct definitions
3. Global variable definitions
4. Function definitions
5. `main` function returning `int32`

```gl3
import "arrays"

struct Item {
    int32 id
}

global int32 default_id = 1i32

fnc create_item(int32 id) -> Item {
    return Item:{ id }
}

fnc main() -> int32 {
    def Item i = create_item(1i32)
    return 0i32
}
```
