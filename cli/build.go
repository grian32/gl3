package cli

import (
	"embed"
	"errors"
	"fmt"

	// "gl3/sema"
	"gl3/emitter"
	"gl3/lexer"
	"gl3/parser"
	"gl3/util"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"strings"
)

type BuildOpts struct {
	Dbg         bool
	NoExecBuild bool
	Shared      bool
	Output      string
	O1          bool
	O2          bool
	O3          bool
}

func RunBuildCmd(builtinFs embed.FS, files []string, opts *BuildOpts) error {
	if opts.O1 && opts.O2 || opts.O1 && opts.O3 || opts.O2 && opts.O3 {
		return errors.New("multiple optimization level arguments not allowed, please use either --O1, --O2, --O3")
	}

	var llFiles []string
	builtinModules := map[string]struct{}{}
	for _, file := range files {
		input, err := os.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		llFile, fileModules, err := compileGl3File(input, file, opts)
		if err != nil {
			return err
		}
		llFiles = append(llFiles, llFile)

		for _, builtinModule := range fileModules {
			builtinModules[builtinModule] = struct{}{}
		}
	}

	for mod, _ := range builtinModules {
		builtinOpts := opts
		// enable optis
		builtinOpts.O3 = true
		builtinOpts.O1 = false
		builtinOpts.O2 = false
		input, err := builtinFs.ReadFile("builtins/" + mod + ".gl3")
		if errors.Is(err, fs.ErrNotExist) {
			input, err = builtinFs.ReadFile("builtins/" + mod + ".ll")
			if err != nil {
				log.Fatal(err)
			}

			llFiles = append(llFiles, "builtins/"+mod+".ll")
			continue
		} else if err != nil {
			log.Fatal(err)
		}
		llFile, _, err := compileGl3File(input, "builtins/"+mod+".gl3", builtinOpts)
		if err != nil {
			return err
		}
		llFiles = append(llFiles, llFile)
	}

	llFiles = append(llFiles, "-o", opts.Output)
	if opts.Shared {
		llFiles = append(llFiles, "-shared")
	}
	if opts.O1 {
		llFiles = append(llFiles, "-O1")
	}
	if opts.O2 {
		llFiles = append(llFiles, "-O2")
	}
	if opts.O3 {
		llFiles = append(llFiles, "-O3")
	}
	if !opts.NoExecBuild {
		cmd := exec.Command("clang", llFiles...)
		if opts.Dbg {
			fmt.Printf("executing: %s\n", strings.Join(cmd.Args, " "))
		}
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("error in clang exec, out: %s, err: %w", out, err)
		}
	}

	return nil
}

func safeRun(fn func()) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	fn()
	return nil
}

func compileGl3File(input []byte, file string, opts *BuildOpts) (string, []string, error) {
	l := lexer.New(string(input))
	p := parser.New(l)
	program := p.ParseProgram()
	err := safeRun(func() {
		if opts.Dbg {
			log.Printf("%s: %s\n", file, program)
		}
	})
	if err != nil {
		return "", nil, err
	}
	if len(p.Errors) != 0 {
		for _, err := range p.Errors {
			log.Printf("parser error: %s:%s\n", file, &err)
		}
		return "", nil, fmt.Errorf("%s: exiting after parser errrors\n", file)
	}
	// c := sema.New()
	// c.Check(program)
	// if len(c.Errors) != 0 {
	// for _, err := range c.Errors {
	// log.Printf("checker warning: %s:%s\n", file, &err)
	// }
	// }

	e := emitter.New()
	err = safeRun(func() {
		e.Emit(program)
	})
	if len(e.Errors) != 0 {
		for _, err := range e.Errors {
			log.Printf("compiler error: %s:%s\n", file, &err)
		}
		return "", nil, fmt.Errorf("compiler errors\n")
	}
	if err != nil {
		log.Printf("%s: recovered emitting llvm ir: %s\n", file, err)
		return "", nil, fmt.Errorf("compiler panic\n")
	}
	llvmIr := e.Module()

	fileName := util.GetFileNamePath(fmt.Sprintf("%s-*.ll", file))
	llFile, err := os.CreateTemp("", fileName)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w\n", file, err)
	}
	_, err = fmt.Fprintf(llFile, "%s", llvmIr)
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w\n", file, err)
	}
	if opts.Dbg {
		fmt.Printf("%s llvm ir: %s", file, llvmIr)
	}
	err = llFile.Close()
	if err != nil {
		return "", nil, fmt.Errorf("%s: %w\n", file, err)
	}

	return llFile.Name(), e.BuiltinModules(), nil
}
