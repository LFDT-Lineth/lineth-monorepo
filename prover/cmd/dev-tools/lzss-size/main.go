// lzss-size compresses a file with the deployed LZSS scheme and its dictionary
// and prints the compressed size in bytes. It exists so a driver can probe
// "does this many blocks fit in a blob?" without linking Go from Python.
//
// With an optional third argument it also writes the compressed bytes, so the
// residual entropy of the LZSS bitstream can be measured -- that residual is
// the ceiling on what adding an entropy coder to the scheme could recover.
//
// Usage: lzss-size <payload-slice> <dict.bin> [out.bin]
package main

import (
	"fmt"
	"os"

	"github.com/consensys/compress/lzss"
)

func main() {
	if len(os.Args) < 3 {
		fmt.Fprintln(os.Stderr, "usage: lzss-size <payload-slice> <dict.bin>")
		os.Exit(2)
	}
	data, err := os.ReadFile(os.Args[1])
	must(err)
	rawDict, err := os.ReadFile(os.Args[2])
	must(err)

	c, err := lzss.NewCompressor(lzss.AugmentDict(rawDict))
	must(err)
	_, err = c.Write(data)
	must(err)
	fmt.Println(c.Len())
	if len(os.Args) > 3 {
		must(os.WriteFile(os.Args[3], c.Bytes(), 0o644))
	}
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}
