package cli

import (
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"gl3/parser"
	"gl3/util"
)

const (
	severityError   = "error"
	severityWarning = "warning"
)

type CheckOpts struct {
	Path string
}

type Diagnostic struct {
	File     string     `json:"file"`
	Severity string     `json:"severity"`
	Message  string     `json:"message"`
	Range    *diagRange `json:"range,omitempty"`
}

type diagRange struct {
	Start diagPos `json:"start"`
	End   diagPos `json:"end"`
}

type diagPos struct {
	Line uint32 `json:"line"`
	Col  uint32 `json:"col"`
}

type checkResult struct {
	Diagnostics []Diagnostic `json:"diagnostics"`
}

func newDiagnostic(file string, severity string, pos *util.Position, msg string) Diagnostic {
	d := Diagnostic{File: file, Severity: severity, Message: msg}
	if pos != nil {
		d.Range = &diagRange{
			Start: diagPos{Line: pos.StartLine, Col: pos.StartCol},
			End:   diagPos{Line: pos.EndLine, Col: pos.EndCol},
		}
	}
	return d
}

// RunCheckCmd returns whether any error-severity diagnostics were found.
func RunCheckCmd(builtinFs embed.FS, stdin io.Reader, stdout io.Writer, files []string, opts *CheckOpts) (bool, error) {
	result := checkResult{Diagnostics: []Diagnostic{}}
	hasErrors := false
	ctx := &buildContext{
		builtinFs: builtinFs,
		compiled:  make(map[string]*compiledModule),
		compiling: make(map[string]bool),
		failed:    make(map[string]error),
		report: func(d Diagnostic) {
			if d.Severity == severityError {
				hasErrors = true
			}
			result.Diagnostics = append(result.Diagnostics, d)
		},
	}

	if len(files) == 0 {
		files = []string{"-"}
	}
	for _, file := range files {
		if file == "-" {
			if err := ctx.checkStdin(stdin, opts.Path); err != nil {
				return false, err
			}
			continue
		}
		absPath, err := filepath.Abs(file)
		if err != nil {
			return false, err
		}
		input, err := os.ReadFile(absPath)
		if err != nil {
			ctx.report(newDiagnostic(absPath, severityError, nil, err.Error()))
			continue
		}
		ctx.checkModule(absPath, string(input))
	}

	enc := json.NewEncoder(stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(result); err != nil {
		return false, err
	}
	return hasErrors, nil
}

func (ctx *buildContext) checkStdin(stdin io.Reader, path string) error {
	input, err := io.ReadAll(stdin)
	if err != nil {
		return fmt.Errorf("read stdin: %w", err)
	}
	if path == "" {
		path = "<stdin>"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	ctx.checkModule(absPath, string(input))
	return nil
}

func (ctx *buildContext) checkModule(absPath string, input string) {
	_, err := ctx.compileModule(absPath, input)
	var failed *moduleFailedError
	if err != nil && !errors.As(err, &failed) {
		ctx.report(newDiagnostic(absPath, severityError, nil, err.Error()))
	}
}

func (ctx *buildContext) reportImportError(absPath string, imp *parser.ImportStatement, err error) {
	msg := fmt.Sprintf("import %q: %s", imp.Path, err)
	var failed *moduleFailedError
	if errors.As(err, &failed) {
		msg = fmt.Sprintf("import %q: imported module has errors", imp.Path)
	}
	ctx.report(newDiagnostic(absPath, severityError, imp.Position(), msg))
}
