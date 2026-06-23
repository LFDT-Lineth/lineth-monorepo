// Tiny CPU-cache eviction tool used between benchmark iterations.
//
// Allocates a buffer of <size-mb> MB (default 256) and stream-touches it with
// a non-trivial xor pattern, comfortably evicting L3 (>= 32 MB on shared-host
// CPUs). Runs in well under a second. Output is suppressed; only the
// exit-code matters.
package main

import (
	"fmt"
	"os"
	"strconv"
)

const defaultMB = 256

func main() {
	mb := defaultMB
	if len(os.Args) >= 2 {
		v, err := strconv.Atoi(os.Args[1])
		if err != nil || v <= 0 {
			fmt.Fprintf(os.Stderr, "usage: cache_thrasher [size-mb]   (got %q)\n", os.Args[1])
			os.Exit(1)
		}
		mb = v
	}
	size := mb * 1024 * 1024
	buf := make([]byte, size)
	var acc byte = 0xA5
	for i := 0; i < size; i++ {
		acc ^= byte(i)
		buf[i] = acc
	}
	acc = 0
	for i := 0; i < size; i += 64 {
		acc ^= buf[i]
	}
	fmt.Fprintf(os.Stderr, "thrashed %d MB (acc=%#02x)\n", mb, acc)
}
