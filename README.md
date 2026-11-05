# GL3

GL3 is an LLVM-based compiler for a systems programming language. The compiler is written in Go and uses the [go-llvm](https://github.com/tinygo-org/go-llvm) bindings for code generation.

## Philosophy

GL3 makes deliberate design choices that differ from many modern languages:

**No Type Inference** - All types must be explicitly specified. Variables require type annotations, and literals carry type suffixes. This eliminates ambiguity and makes the programmer's intentions explicit in the code.

**No Static Arrays** - Static arrays complicate the type system and offer little practical benefit when heap memory is readily available. Use dynamic arrays from the `arrays` standard library module instead. The `__asm__salloc` function in the `asm` module exists for extreme low-level scenarios (such as OS development before an allocator exists), but its use is discouraged outside of those specific cases.

## Building

The compiler uses LLVM to generate an object file for each module, then links them into a native executable using `clang`. Clang is required for linking.

### Requirements

- Go >=1.25.5
- LLVM 22 development libraries
- Clang (tested with version 22.1.1)

### Building the compiler

```bash
make
```

This builds the `gl3` compiler binary, with the stdlib builtin modules embedded in it. You can remove the binary using `make clean`.

## Example Program

```gl3
import "io"
import "ralloc"
import "strings"

struct Node {
    char* name
    Node* next
}

fnc main() -> int32 {
    def Node* a = malloc(sizeof Node) as Node*
    def Node* b = malloc(sizeof Node) as Node*

    a.name = dynstr("alice")
    a.next = b
    b.name = dynstr("bob")
    b.next = (0 as Node*)

    def Node* cur = a
    while (cur as int) != 0 {
        println("hello, %s", cur.name)
        cur = cur.next
    }

    str_free(a.name)
    str_free(b.name)
    free(a)
    free(b)
    return 0i32
}
```

### Compile & run the example

```bash
./gl3 build example.gl3 -o output
./output
```

## Documentation

- [SYNTAX.md](SYNTAX.md) - Complete language syntax reference
- [STDLIB.md](STDLIB.md) - Standard library documentation
- [CLI.md](CLI.md) - Command line interface reference
