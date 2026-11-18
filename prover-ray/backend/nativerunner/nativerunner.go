// Package nativerunner runs the native l2_execution_runner on an extended
// (0x0002) input and returns its output, to check the guest output against it.
package nativerunner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
)

// RunSSZ writes the extended input to a temp file, runs
// `<binPath> <file> --ssz` and returns the raw 0x0003 output. binPath is the
// native l2_execution_runner binary. The output is byte-identical to what the
// guest writes (hash, then schema id and public inputs), so dev-zkvm and
// dev-partial compare the two.
func RunSSZ(ctx context.Context, binPath string, extendedInput []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "l2-exec-input-*.ssz")
	if err != nil {
		return nil, fmt.Errorf("creating temp input: %w", err)
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(extendedInput); err != nil {
		tmp.Close()
		return nil, fmt.Errorf("writing temp input: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("closing temp input: %w", err)
	}

	//nolint:gosec // G204: binPath is an operator-configured trusted path, not user input
	out, err := exec.CommandContext(ctx, binPath, tmp.Name(), "--ssz").Output()
	if err != nil {
		return nil, fmt.Errorf("running native runner %q: %w", binPath, err)
	}
	return out, nil
}
