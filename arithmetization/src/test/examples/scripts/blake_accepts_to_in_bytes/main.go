// Helper for the Blake-2f `IN_BYTES` per-vector layout expected by
// rust/src/blake/blake_with_in_bytes.rs.
//
// Reads a plain text `.all` file where every line is one `0x<hex>` `IN_BYTES`
// blob for a single Blake-2f test vector (213 input bytes + 64 expected output
// bytes = 277 bytes = 554 hex chars + the `0x` prefix). Emits the first N
// validated lines to stdout, one IN_BYTES per line — the natural shape for
// Blake's per-vector loop (unlike Keccak's single batched blob).
//
// Usage:
//
//	blake_accepts_to_in_bytes <accepts-file> <n-vectors>
//
// `n-vectors == 0` means "all lines". Exits non-zero if any line fails
// validation or fewer than N vectors are available.
package main

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	rawBytes      = 277
	hexCharsBody  = rawBytes * 2 // 554
	hexCharsTotal = hexCharsBody + 2
)

func validateLine(line string, lineno int) (string, error) {
	if !strings.HasPrefix(line, "0x") && !strings.HasPrefix(line, "0X") {
		return "", fmt.Errorf("line %d: missing 0x prefix", lineno)
	}
	if len(line) != hexCharsTotal {
		return "", fmt.Errorf("line %d: length is %d chars, expected %d", lineno, len(line), hexCharsTotal)
	}
	if _, err := hex.DecodeString(line[2:]); err != nil {
		return "", fmt.Errorf("line %d: hex decode: %w", lineno, err)
	}
	return line, nil
}

func run(args []string) error {
	if len(args) != 2 {
		return fmt.Errorf("usage: blake_accepts_to_in_bytes <accepts-file> <n-vectors>  (n=0 means all lines)")
	}
	path := args[0]
	n, err := strconv.Atoi(args[1])
	if err != nil || n < 0 {
		return fmt.Errorf("n-vectors must be a non-negative integer, got %q", args[1])
	}

	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 4<<10), 1<<20)
	out := bufio.NewWriter(os.Stdout)
	defer out.Flush()

	lineno := 0
	emitted := 0
	for scanner.Scan() {
		lineno++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		validated, err := validateLine(line, lineno)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, validated); err != nil {
			return fmt.Errorf("write stdout: %w", err)
		}
		emitted++
		if n > 0 && emitted >= n {
			break
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("scan %s: %w", path, err)
	}
	if n > 0 && emitted < n {
		return fmt.Errorf("%s: only %d vector(s) available, requested %d", path, emitted, n)
	}

	fmt.Fprintf(os.Stderr, "emitted %d Blake vector(s) from %s\n", emitted, path)
	return nil
}

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
