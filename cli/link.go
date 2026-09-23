package cli

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func linkObjects(objects []string, output string, shared, debug bool) error {
	args := make([]string, 0, len(objects)+3)
	if shared {
		args = append(args, "-shared")
	}
	args = append(args, objects...)
	args = append(args, "-o", output)
	if debug {
		fmt.Fprintf(os.Stderr, "clang %q\n", args)
	}

	result, err := exec.Command("clang", args...).CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(result))
		if message != "" {
			return fmt.Errorf("clang link failed: %w: %s", err, message)
		}
		return fmt.Errorf("clang link failed: %w", err)
	}
	return nil
}
