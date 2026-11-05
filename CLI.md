# CLI

`gl3` provides subcommands for building programs and generating `.gl3` definitions from C headers.

## Overview

```text
gl3 [command] [--flags]
```

### Top-level commands

| Command | Description |
| ------- | ----------- |
| `build` | Compile GL3 files to an executable |
| `exdef` | Extract `#define` values from a C header into a `.gl3` file |
| `completion` | Generate the autocompletion script for a shell |
| `help` | Show help for a command |

### Top-level flags

| Flag | Description |
| ---- | ----------- |
| `-h`, `--help` | Show help for `gl3` |
| `-v`, `--version` | Show the compiler version |

## `build`

Compile GL3 files to an executable.

### Usage

```text
gl3 build [--flags] <files>
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `--dbg` | Print the LLVM IR for all compiled files, along with the `clang` command used for linking |
| `--O1` | Compiles the code with optimization level 1 |
| `--O2` | Requests optimization level 2 |
| `--O3` | Requests optimization level 3 |
| `-o`, `--output` | Path of the output executable (default `./out`) |
| `--shared` | Compile the code as a shared library |
| `--ldflag` | Pass one argument to `clang` when linking. Repeat it for each argument, for example `--ldflag -lGL` or `--ldflag -framework --ldflag OpenGL` |
| `--emitter` | Code generation backend (default `llvm`, currently the only option) |
| `-h`, `--help` | Show help for `build` |

Builtin modules are always compiled at optimization level 3. The optimization flags only apply to your own files.

### Example

```bash
./gl3 build example.gl3 -o output
./output
```

## `exdef`

Extract `#define` statements from a C header file and emit them as constants in a `.gl3` file.

### Usage

```text
gl3 exdef [--flags]
```

### Flags

| Flag | Description |
| ---- | ----------- |
| `-i`, `--input` | Path to the input header file |
| `-o`, `--output` | Path to the output `.gl3` file |
| `-h`, `--help` | Show help for `exdef` |

### Example

```bash
./gl3 exdef -i example.h -o example.gl3
```
