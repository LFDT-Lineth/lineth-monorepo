// Count register-x0 access markers emitted by read_register/write_register in
// arithmetization RISC-V zkc (REG_READ_0 / REG_WRITE_0).
//
// Markers are printed via zkc printf, which is suppressed when zkc is run with
// -q / --quiet. Omit that flag when capturing counts.
//
// Usage:
//
//	make -C arithmetization install-zkc
//	zkc exec -v --field KOALABEAR_16 /tmp/keccak.json arithmetization/src/main/riscv/main.zkc 2>&1 \
//	  | go run ./count_reg0
//	go run ./count_reg0 -log run.log
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
)

const (
	markerRead       = "REG_READ_0"
	markerWrite      = "REG_WRITE_0"
	clockCycleMarker = "clock cycle:"
)

func countMarkers(r io.Reader) (reads, writes, clockCycles int, err error) {
	scanner := bufio.NewScanner(r)
	// Keccak runs can emit very long lines (e.g. instruction traces).
	scanner.Buffer(make([]byte, 1024), 10*1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, markerRead) {
			reads++
		}
		if strings.Contains(line, markerWrite) {
			writes++
		}
		if strings.Contains(line, clockCycleMarker) {
			clockCycles++
		}
	}
	return reads, writes, clockCycles, scanner.Err()
}

func main() {
	logPath := flag.String("log", "", "path to zkc exec log (default: stdin)")
	flag.Parse()

	var input io.Reader = os.Stdin
	if *logPath != "" {
		f, err := os.Open(*logPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		defer f.Close()
		input = f
	}

	reads, writes, clockCycles, err := countMarkers(input)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("clock cycles:       %d\n", clockCycles)
	fmt.Printf("register x0 reads:  %d\n", reads)
	fmt.Printf("register x0 writes: %d\n", writes)
	if clockCycles > 0 {
		fmt.Printf("writes - cycles:    %d  (instruction writes to x0 where rd=0)\n", writes-clockCycles)
	}
	if reads == 0 && writes == 0 {
		fmt.Fprintln(os.Stderr, "warning: no markers found; zkc printf is suppressed by -q/--quiet")
	}
}
