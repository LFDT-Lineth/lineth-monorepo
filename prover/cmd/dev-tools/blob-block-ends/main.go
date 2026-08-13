// blob-block-ends recovers L2 block boundaries from an encoded blob payload and
// prints the cumulative end offset of each block, one per line.
//
// Blob filling searches over block counts rather than bytes, because production
// cannot split a block across blobs. This uses v1.ScanBlockByteLen -- the same
// routine production uses to recover those boundaries -- so it needs only the
// payload, not the original corpus.
//
// Usage: blob-block-ends <payload.bin>
package main

import (
	"fmt"
	"os"

	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: blob-block-ends <payload.bin>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}

	out := make([]byte, 0, 1<<20)
	for off := 0; off < len(data); {
		n, err := v1.ScanBlockByteLen(data[off:])
		if err != nil || n <= 0 {
			fmt.Fprintf(os.Stderr, "stopped at offset %d after %d blocks: %v\n",
				off, len(out), err)
			break
		}
		off += n
		out = fmt.Appendf(out, "%d\n", off)
	}
	os.Stdout.Write(out)
}
