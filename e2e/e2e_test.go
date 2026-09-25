package e2e_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"
)

// Run with: go test ./e2e -count=1
// Each fixture starts with // e2e: followed by a JSON expectation. Supporting
// modules live under support/ and are compiled only through fixture imports or
// inputs. No compiler packages, AST shapes, or generated IR are test APIs.
type expectation struct {
	Mode          string   `json:"mode"` // run, reject, shared, help
	Exit          int      `json:"exit"`
	Stdout        string   `json:"stdout"`
	Stderr        string   `json:"stderr"`
	Error         []string `json:"error"` // stable diagnostic fragments, not whole snapshots
	Line          int      `json:"line"`  // source diagnostic must identify this line and a column
	Flags         []string `json:"flags"`
	Inputs        []string `json:"inputs"` // additional CLI inputs, relative to the fixture
	NoInput       bool     `json:"no_input"`
	DefaultOutput bool     `json:"default_output"`
	OutputFlag    string   `json:"output_flag"` // -o (default) or --output
	OutputPath    string   `json:"output_path"` // relative to the isolated work directory
	BuildOutput   []string `json:"build_output"`
	Consumer      string   `json:"consumer"` // C caller for shared-library ABI checks
}

func TestFixtures(t *testing.T) {
	root, err := filepath.Abs("..")
	if err != nil {
		t.Fatal(err)
	}
	compiler := filepath.Join(t.TempDir(), "gl3")
	// Prepare the embedded builtin IR on clean checkouts, then build once from
	// current sources; never accidentally test a stale ./gl3.
	runCommand(t, root, time.Minute, "make", "stdlib").success(t)
	runCommand(t, root, 2*time.Minute, "go", "build", "-tags=llvm22", "-o", compiler, ".").success(t)
	fixtures := filepath.Join(root, "e2e", "testdata")
	count := 0
	err = filepath.WalkDir(fixtures, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "support" {
			return filepath.SkipDir
		}
		if entry.IsDir() || filepath.Ext(path) != ".gl3" {
			return nil
		}
		count++
		name, err := filepath.Rel(fixtures, path)
		if err != nil {
			return err
		}
		t.Run(strings.TrimSuffix(filepath.ToSlash(name), ".gl3"), func(t *testing.T) {
			data, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			header := strings.SplitN(string(data), "\n", 2)[0]
			const prefix = "// e2e: "
			if !strings.HasPrefix(header, prefix) {
				t.Fatal("missing // e2e: JSON expectation")
			}
			var want expectation
			decoder := json.NewDecoder(strings.NewReader(strings.TrimPrefix(header, prefix)))
			decoder.DisallowUnknownFields()
			if err := decoder.Decode(&want); err != nil {
				t.Fatal(err)
			}
			if want.Mode != "run" && want.Mode != "reject" && want.Mode != "shared" && want.Mode != "help" {
				t.Fatalf("invalid mode %q", want.Mode)
			}
			if want.Mode == "reject" && len(want.Error) == 0 {
				t.Fatal("rejection needs diagnostic fragments")
			}
			if want.Mode == "shared" && want.Consumer == "" {
				t.Fatal("shared fixture needs a consumer")
			}
			if want.OutputFlag == "" {
				want.OutputFlag = "-o"
			}
			if want.OutputFlag != "-o" && want.OutputFlag != "--output" {
				t.Fatal("output_flag must be -o or --output")
			}
			if want.OutputPath != "" && (!filepath.IsLocal(want.OutputPath) || want.DefaultOutput) {
				t.Fatal("output_path must stay inside the work directory and cannot use default_output")
			}
			levels := []string{"default", "O1", "O2", "O3"}
			if want.Mode == "reject" || want.Mode == "help" {
				levels = levels[:1]
			}
			for _, level := range levels {
				t.Run(level, func(t *testing.T) {
					work := t.TempDir()
					output := filepath.Join(work, "program with spaces")
					if want.OutputPath != "" {
						output = filepath.Join(work, want.OutputPath)
					}
					args := []string{"build"}
					if !want.NoInput {
						args = append(args, path)
					}
					for _, input := range want.Inputs {
						args = append(args, filepath.Join(filepath.Dir(path), input))
					}
					if want.DefaultOutput {
						output = filepath.Join(work, "out")
					} else {
						args = append(args, want.OutputFlag, output)
					}
					if level != "default" {
						args = append(args, "--"+level)
					}
					if want.Mode == "shared" {
						args = append(args, "--shared")
					}
					args = append(args, want.Flags...)
					built := runCommand(t, work, 10*time.Second, compiler, args...)
					diagnostics := built.stdout + built.stderr
					if want.Mode == "reject" {
						if built.code != 1 {
							t.Fatalf("expected clean compiler rejection (exit 1), got %d\n%s", built.code, diagnostics)
						}
						for _, bad := range []string{"panic:", "SIGSEGV", "LLVM ERROR", "fatal error:"} {
							if strings.Contains(diagnostics, bad) {
								t.Fatalf("compiler crashed instead of diagnosing input:\n%s", diagnostics)
							}
						}
						for _, fragment := range want.Error {
							// The CLI renderer capitalizes the first word of errors.
							if !strings.Contains(strings.ToLower(diagnostics), strings.ToLower(fragment)) {
								t.Errorf("missing diagnostic %q:\n%s", fragment, diagnostics)
							}
						}
						if want.Line > 0 {
							location := regexp.MustCompile(regexp.QuoteMeta(path) + fmt.Sprintf(":%d:[1-9][0-9]*:", want.Line))
							if !location.MatchString(diagnostics) {
								t.Errorf("diagnostic must locate %s:%d:<column>:\n%s", path, want.Line, diagnostics)
							}
						}
						if _, err := os.Stat(output); !errors.Is(err, os.ErrNotExist) {
							t.Errorf("rejected build left an output artifact: %v", err)
						}
						return
					}
					built.success(t)
					for _, fragment := range want.BuildOutput {
						if !strings.Contains(diagnostics, fragment) {
							t.Errorf("missing build output %q:\n%s", fragment, diagnostics)
						}
					}
					if want.Mode == "help" {
						for _, artifact := range []string{output, filepath.Join(work, "out")} {
							if _, err := os.Stat(artifact); !errors.Is(err, os.ErrNotExist) {
								t.Errorf("help created a build artifact: %s: %v", artifact, err)
							}
						}
						return
					}
					if info, err := os.Stat(output); err != nil || info.Size() == 0 {
						t.Fatalf("missing or empty build artifact: %v", err)
					}
					if want.Mode == "shared" {
						library := output
						output = filepath.Join(work, "consumer")
						runCommand(t, work, 10*time.Second, "clang", filepath.Join(filepath.Dir(path), want.Consumer), library, "-Wl,-rpath,"+work, "-o", output).success(t)
					}
					got := runCommand(t, work, 3*time.Second, output)
					if got.code != want.Exit || got.stdout != want.Stdout || got.stderr != want.Stderr {
						t.Fatalf("program exit=%d stdout=%q stderr=%q; want exit=%d stdout=%q stderr=%q", got.code, got.stdout, got.stderr, want.Exit, want.Stdout, want.Stderr)
					}
				})
			}
		})
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if count == 0 {
		t.Fatal("no fixtures discovered")
	}
}

type result struct {
	command        string
	code           int
	stdout, stderr string
}

func (r result) success(t *testing.T) {
	t.Helper()
	if r.code != 0 {
		t.Fatalf("%s exited %d\n%s%s", r.command, r.code, r.stdout, r.stderr)
	}
}

// Keep malformed-input loops from consuming unbounded memory while preserving
// the first useful diagnostic. Even expected failures must finish before timeout.
type limitedOutput struct{ bytes.Buffer }

func (b *limitedOutput) Write(p []byte) (int, error) {
	n := len(p)
	if left := 64*1024 - b.Len(); left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		_, _ = b.Buffer.Write(p)
	}
	return n, nil
}
func runCommand(t *testing.T, dir string, timeout time.Duration, name string, args ...string) result {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "NO_COLOR=1", "TERM=dumb")
	cmd.WaitDelay = time.Second
	var stdout, stderr limitedOutput
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	r := result{command: fmt.Sprintf("%s %q", name, args), stdout: stdout.String(), stderr: stderr.String()}
	if ctx.Err() != nil {
		t.Fatalf("command timed out: %s\n%s%s", r.command, r.stdout, r.stderr)
	}
	if err != nil {
		var exit *exec.ExitError
		if !errors.As(err, &exit) {
			t.Fatalf("cannot run %s: %v", r.command, err)
		}
		r.code = exit.ExitCode()
	}
	return r
}
