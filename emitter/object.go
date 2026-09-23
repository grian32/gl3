package emitter

import (
	"fmt"
	"os"

	llvm "tinygo.org/x/go-llvm"
)

func (e *Emitter) WriteObject(path string) error {
	if err := llvm.VerifyModule(e.module, llvm.ReturnStatusAction); err != nil {
		return fmt.Errorf("invalid LLVM module: %w", err)
	}

	if e.optimization > 0 {
		passes := fmt.Sprintf("default<O%d>", e.optimization)
		options := llvm.NewPassBuilderOptions()
		defer options.Dispose()
		if err := e.module.RunPasses(passes, e.target, options); err != nil {
			return fmt.Errorf("optimize LLVM module: %w", err)
		}
	}

	object, err := e.target.EmitToMemoryBuffer(e.module, llvm.ObjectFile)
	if err != nil {
		return fmt.Errorf("generate object: %w", err)
	}
	defer object.Dispose()
	if err := os.WriteFile(path, object.Bytes(), 0644); err != nil {
		return fmt.Errorf("write object %q: %w", path, err)
	}
	return nil
}
