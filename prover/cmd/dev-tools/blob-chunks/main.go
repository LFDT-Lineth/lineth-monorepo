// blob-chunks splits a corpus into blob-sized payload chunks and compresses
// each with the deployed LZSS scheme and its production dictionary.
//
// Chunking matters. Production compresses one blob at a time, so the matcher
// only ever sees ~780 kB of context (MaxUncompressedBytes). Compressing a
// whole multi-megabyte window in one go lets LZ find matches across far more
// history than it will ever have in practice, which inflates every ratio.
// The chunks written here are the unit the economics actually operate on, and
// they are emitted to disk so the same units can be fed to zstd and lz4.
//
// Usage: blob-chunks <payload.bin> <dict.bin> <outDir>
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/consensys/compress/lzss"
	v1 "github.com/consensys/linea-monorepo/prover/lib/compressor/blob/v1"
)

func main() {
	if len(os.Args) < 4 {
		fmt.Fprintln(os.Stderr, "usage: blob-chunks <payload.bin> <dict.bin> <outDir>")
		os.Exit(2)
	}
	payloadPath, dictPath, outDir := os.Args[1], os.Args[2], os.Args[3]

	full, err := os.ReadFile(payloadPath)
	must(err)

	rawDict, err := os.ReadFile(dictPath)
	must(err)
	dict := lzss.AugmentDict(rawDict)

	must(os.MkdirAll(outDir, 0o755))

	const chunkSize = v1.MaxUncompressedBytes // 780_000
	fmt.Printf("payload=%d bytes  chunkSize=%d  chunks=%d\n", len(full), chunkSize, (len(full)+chunkSize-1)/chunkSize)
	fmt.Printf("%-6s %10s %10s %8s\n", "chunk", "raw", "lzss", "ratio")

	var totRaw, totLzss int
	for i := 0; i*chunkSize < len(full); i++ {
		lo := i * chunkSize
		hi := min(lo+chunkSize, len(full))
		chunk := full[lo:hi]

		// Only full-size chunks are representative; a short tail would flatter
		// or penalise arbitrarily depending on where the window happens to end.
		if len(chunk) < chunkSize {
			fmt.Printf("%-6d %10d %10s %8s  (partial tail, skipped)\n", i, len(chunk), "-", "-")
			break
		}

		must(os.WriteFile(filepath.Join(outDir, fmt.Sprintf("chunk_%03d.bin", i)), chunk, 0o644))

		c, err := lzss.NewCompressor(dict)
		must(err)
		_, err = c.Write(chunk)
		must(err)
		n := c.Len()

		totRaw += len(chunk)
		totLzss += n
		fmt.Printf("%-6d %10d %10d %7.2fx\n", i, len(chunk), n, float64(len(chunk))/float64(n))
	}
	if totLzss > 0 {
		fmt.Printf("%-6s %10d %10d %7.2fx\n", "TOTAL", totRaw, totLzss, float64(totRaw)/float64(totLzss))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
