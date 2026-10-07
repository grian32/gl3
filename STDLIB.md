# GL3 Standard Library

## arrays - Dynamic Arrays

Dynamic heap-allocated arrays with automatic resizing.

### Functions

| Function                | Parameters                    | Returns  |
|-------------------------|-------------------------------|----------|
| `arr_new(size)`         | int64 (element size in bytes) | void*    |
| `arr_push(&arr, &elem)` | void*, void*                  | none     |
| `arr_free(&arr)`        | void*                         | none     |

### Important: Hidden Metadata

The arrays implementation hides metadata behind the pointer returned to the user. When working with array pointers directly, be aware that the actual memory layout includes header data before the pointer address. Do not perform manual pointer arithmetic expecting standard contiguous memory.

### Usage

```gl3
import "arrays"

fnc main() -> int32 {
    // create array, cast to appropriate pointer type
    def int32* arr = (arr_new((sizeof int32))) as int32*
    
    // push elements (pass pointers)
    def int32 val1 = 1i32
    def int32 val2 = 2i32
    arr_push(&arr, &val1)
    arr_push(&arr, &val2)
    
    // access via indexing
    def int32 first = arr[0]
    arr[1] = 10i32
    
    // free when done
    arr_free(&arr)
    
    return 0i32
}
```

### Array Literal Syntax

Array literals provide syntactic sugar for array creation and population:

```gl3
def int32* nums = [int32; 1i32, 2i32, 3i32, 4i32, 5i32]
```

This expands to:

```gl3
def int32* nums = (arr_new((sizeof int32))) as int32*
arr_push(&nums, &1i32)
arr_push(&nums, &2i32)
arr_push(&nums, &3i32)
arr_push(&nums, &4i32)
arr_push(&nums, &5i32)
```

Remember to call `arr_free` on array literals when no longer needed.

## strings - String Operations

Functions for working with strings, supporting both static and dynamically allocated strings.

### Functions

| Function              | Parameters       | Returns  | Description                             |
|-----------------------|------------------|----------|-----------------------------------------|
| `dynstr(char*)`       | char*            | char*    | Convert static string to dynamic        |
| `str_append(a, b)`    | char*, char*     | char*    | Concatenate two strings                 |
| `str_len(char*)`      | char*            | int      | Return string length                    |
| `str_free(char*)`     | char*            | none     | Free dynamically allocated string       |

### Static vs Dynamic Strings

- **Static strings** (`"literal"`) are stored in read-only memory and should not be freed
- **Dynamic strings** are heap-allocated and must be freed with `str_free`

### str_append Behavior

`str_append` always returns a newly allocated dynamic string. It is recommended to use dynamic strings as inputs, but static strings will also work. The returned string must be freed.

### Usage

```gl3
import "strings"

fnc main() -> int32 {
    // static string (read-only, do not free)
    def char* stat = "hello"
    
    // convert to dynamic
    def char* dyn = dynstr(stat)
    
    // append returns new dynamic string
    def char* result = str_append(dyn, " world")
    
    // get length
    def int len = str_len(result)  // returns 11
    
    // free all dynamic strings
    str_free(dyn)
    str_free(result)
    
    // stat is static, do not free
    
    return 0i32
}
```

### Memory Management Summary

| String Type | Source                 | Free Required |
|-------------|------------------------|---------------|
| Static      | `"literal"`            | No            |
| Dynamic     | `dynstr()`             | Yes           |
| Dynamic     | `str_append()`         | Yes           |

## ralloc - Raw Allocation

Low-level heap allocation functions that vendor C's `malloc`, `calloc`, and `free`.

### Functions

| Function               | Parameters                       | Returns | Description |
|------------------------|----------------------------------|---------|-------------|
| `malloc(size)`         | uint (bytes)                     | none*   | Allocate `size` bytes (uninitialized) |
| `calloc(count, size)`  | uint (count), uint (bytes each)  | none*   | Allocate `count * size` bytes, zero-initialized |
| `free(ptr)`            | none*                            | none    | Free memory previously allocated by `malloc`/`calloc` |

### Usage

```gl3
import "ralloc"

fnc main() -> int32 {
    def uint count = 4u64

    // malloc: allocate raw bytes, then cast to a typed pointer
    def int32* values = malloc((sizeof int32) * count) as int32*
    values[0] = 10i32
    values[1] = 20i32

    // calloc: same allocation shape, but zero-initialized
    def int32* zeros = calloc(count, sizeof int32) as int32*
    zeros[2] = 7i32

    free(values as none*)
    free(zeros as none*)

    return 0i32
}
```

### Notes

- `ralloc` is intentionally low-level and does not track element counts or types.
- Always cast returned `none*` to the pointer type you want to use, and cast back to `none*` when freeing.
- Sizes are `uint`, matching the type of `sizeof`.
- Pair every successful `malloc`/`calloc` with exactly one `free`.

## io - Formatted Output

Formatted output helpers for writing text to standard output.

### Functions

| Function            | Parameters                 | Returns |
|---------------------|----------------------------|---------|
| `print(fmt, ...)`   | char*, variadic arguments  | none    |
| `println(fmt, ...)` | char*, variadic arguments  | none    |

`print` writes formatted text. `println` does the same and appends a trailing newline.

### Format Specifiers

| Specifier | Description |
|-----------|-------------|
| `%b`      | bool (`true`/`false`) |
| `%c`      | char |
| `%s`      | string (`char*`) |
| `%y`      | int8 |
| `%w`      | int16 |
| `%d`      | int32 |
| `%l`      | int64 |
| `%uy`     | uint8 |
| `%uw`     | uint16 |
| `%ud`     | uint32 |
| `%ul`     | uint64 |
| `%%`      | literal percent sign |

Prefix integer specifiers with `f` to include the type suffix in output (for example, `%fd` prints `7i32`, while `%d` prints `7`).

### Usage

```gl3
import "io"

fnc main() -> int32 {
    def int8 small = -5i8
    def uint32 count = 42u32

    print("value: %d, small: %fy, count: %fud", 123i32, small, count)
    println(" done")
    println("ok=%b, msg=%s", true, "hello")

    return 0i32
}
```

## asm - Low-Level Operations

Functions that generate LLVM IR directly for scenarios requiring precise control, such as OS development before a memory allocator is available.

### Functions

| Function                    | Parameters                    | Returns |
|-----------------------------|-------------------------------|---------|
| `__asm__salloc(count, sz)`  | int64, int64                  | void*   |

### __asm__salloc

Statically allocates an array. Parameters:
- `count`: number of elements
- `sz`: size of each element (must use `sizeof`)

```gl3
import "asm"

fnc main() -> int32 {
    // allocate static array of 10 int32 values
    def int32* arr = __asm__salloc(10, sizeof int32)
    
    arr[0] = 1i32
    arr[1] = 2i32
    
    // no need to free - static allocation
    
    return 0i32
}
```

This function exists for specific low-level use cases such as OS kernel development before an allocator exists. For general programming, use the `arrays` module.
