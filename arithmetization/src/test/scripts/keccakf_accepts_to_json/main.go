// Converts a native-zkc keccakf `.accepts` fixture into the input JSON that
// `zkc exec` expects, slicing it to the first N vectors.
//
// The `.accepts` fixture is a single JSON object with four hex-string fields:
//   - "n_vectors"     : ROM input — number of independent hashes (u64 BE)
//   - "block_counts"  : ROM input — per-vector block count, flat concat of
//     n_vectors × u64 BE
//   - "blocks"        : ROM input — flat concat of sum(block_counts) Keccak
//     rate-1088 padded blocks (136 bytes each)
//   - "result"        : WOM output — expected per-vector 256-bit digest
//
// in arithmetization/src/test/Makefile.
// `zkc exec` accepts only ROM inputs; passing the WOM `result` field along
// with the inputs makes it fail. This helper slices the fixture to the first
// N vectors and drops `result`: `n_vectors` is re-encoded to N,
// `block_counts` is truncated to the first N u64 entries, and `blocks` is
// truncated to the first `sum(block_counts[0..N])` blocks. The output is a
// valid input for `keccakf_batched.zkc` at workload size N.

// Usage:
//
//	keccakf_accepts_to_json <in.accepts> <out.json> <n-vectors>
package main

import (
	"encoding/json"
	"fmt"
	"os"
	"strconv"
)

const (
	// One u64, big-endian, encoded as 16 hex chars.
	u64HexLen = 16
	// One Keccak rate-1088 block: 1088 bits = 136 bytes = 272 hex chars.
	blockHexLen = 272
)

func main() {
	if len(os.Args) != 4 {
		fmt.Fprintln(os.Stderr, "usage: keccakf_accepts_to_json <in.accepts> <out.json> <n-vectors>")
		os.Exit(1)
	}
	inPath, outPath := os.Args[1], os.Args[2]
	nWant, err := strconv.ParseUint(os.Args[3], 10, 64)
	if err != nil || nWant == 0 {
		fmt.Fprintf(os.Stderr, "error: invalid n-vectors %q: must be a positive integer\n", os.Args[3])
		os.Exit(1)
	}

	data, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: read %s: %v\n", inPath, err)
		os.Exit(1)
	}

	var raw struct {
		NVectors    string `json:"n_vectors"`
		BlockCounts string `json:"block_counts"`
		Blocks      string `json:"blocks"`
	}
	if err := json.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "error: parse %s: %v\n", inPath, err)
		os.Exit(1)
	}

	nvHex, err := stripHexPrefix(raw.NVectors)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: n_vectors: %v\n", err)
		os.Exit(1)
	}
	bcHex, err := stripHexPrefix(raw.BlockCounts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: block_counts: %v\n", err)
		os.Exit(1)
	}
	blocksHex, err := stripHexPrefix(raw.Blocks)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: blocks: %v\n", err)
		os.Exit(1)
	}

	if len(nvHex) != u64HexLen {
		fmt.Fprintf(os.Stderr, "error: n_vectors: expected %d hex chars, got %d\n", u64HexLen, len(nvHex))
		os.Exit(1)
	}
	nHave, err := strconv.ParseUint(nvHex, 16, 64)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: n_vectors: parse: %v\n", err)
		os.Exit(1)
	}
	if uint64(len(bcHex)) != nHave*u64HexLen {
		fmt.Fprintf(os.Stderr, "error: block_counts: expected %d hex chars for %d vectors, got %d\n",
			nHave*u64HexLen, nHave, len(bcHex))
		os.Exit(1)
	}
	if nWant > nHave {
		fmt.Fprintf(os.Stderr, "error: requested n-vectors=%d exceeds fixture size %d\n", nWant, nHave)
		os.Exit(1)
	}

	bcHexOut := bcHex[:nWant*u64HexLen]
	var totalBlocks uint64
	for i := uint64(0); i < nWant; i++ {
		chunk := bcHexOut[i*u64HexLen : (i+1)*u64HexLen]
		c, err := strconv.ParseUint(chunk, 16, 64)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: block_counts[%d]: parse %q: %v\n", i, chunk, err)
			os.Exit(1)
		}
		totalBlocks += c
	}
	if uint64(len(blocksHex)) < totalBlocks*blockHexLen {
		fmt.Fprintf(os.Stderr, "error: blocks: need %d hex chars for %d blocks, have %d\n",
			totalBlocks*blockHexLen, totalBlocks, len(blocksHex))
		os.Exit(1)
	}
	blocksHexOut := blocksHex[:totalBlocks*blockHexLen]

	out := map[string]string{
		"n_vectors":    fmt.Sprintf("0x%016x", nWant),
		"block_counts": "0x" + bcHexOut,
		"blocks":       "0x" + blocksHexOut,
	}
	// json.Marshal sorts map keys alphabetically (block_counts < blocks <
	// n_vectors), which is fine for `zkc exec` — it doesn't care about key
	// order, only that the three ROM inputs are present and `result` is not.
	buf, err := json.Marshal(out)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: marshal: %v\n", err)
		os.Exit(1)
	}
	if err := os.WriteFile(outPath, buf, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "error: write %s: %v\n", outPath, err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "wrote %s (%d bytes, sliced first %d of %d vectors, %d total blocks)\n",
		outPath, len(buf), nWant, nHave, totalBlocks)
}

func stripHexPrefix(s string) (string, error) {
	if len(s) < 2 || s[0] != '0' || (s[1] != 'x' && s[1] != 'X') {
		return "", fmt.Errorf("expected 0x-prefixed hex, got %q (len=%d)", trunc(s, 16), len(s))
	}
	return s[2:], nil
}

func trunc(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
