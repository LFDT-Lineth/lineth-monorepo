// Runner for the bench_lzss micro-benchmark.
// Builds the R5 ELF, converts to zkc JSON, runs zkc, and reports decode cycles
// per output byte for both wire formats, plus a CSV report.
//
// zkc's trace is streamed and reduced as it arrives rather than saved: at these
// cycle counts the per-instruction trace runs to hundreds of millions of lines.
package main

import (
	"bufio"
	"encoding/csv"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
)

const (
	elfToJSON     = "../../../arithmetization/src/test/scripts/elf_to_json_gen/main.go"
	zkcMain       = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin         = "zig-out/bin/bench-lzss"
	r5JSON        = "zig-out/bin/bench-lzss.json"
	tailLimit     = 40
	defaultOutput = "bench/bench-lzss.csv"
	// Must match decompressed_len in main.zig — update both together.
	decompressedLen = 780_000
)

// Marker pairs, matching the IDs main.zig writes.
var phases = []struct {
	name           string
	start, end     uint64
	compressedFile string
}{
	{"lzss v0.3.0 (A0)", 10, 11, "a0_compressed.bin"},
	{"lzss + huffman-on-lengths", 20, 21, "huffman_compressed.bin"},
}

var baseline = struct{ start, end uint64 }{0, 1}

var (
	markRE  = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE = regexp.MustCompile(`clock cycle: ([0-9]+)`)
)

type marker struct{ cycle, value uint64 }

type traceStats struct {
	totalCycles uint64
	markers     map[uint64]marker
	tail        []string
}

type result struct {
	name            string
	net             uint64
	cyclesPerByte   float64
	compressedBytes int64
}

func main() {
	outFlag := flag.String("out", defaultOutput, "CSV output path")
	flag.Parse()
	zkcBin := "zkc"
	if args := flag.Args(); len(args) > 0 {
		zkcBin = args[0]
	}

	fmt.Fprintln(os.Stderr, "building R5 ELF...")
	if err := run("zig", "build", "--release=small"); err != nil {
		fatal(err)
	}

	fmt.Fprintln(os.Stderr, "converting ELF to zkc JSON...")
	if err := os.MkdirAll("zig-out/bin", 0o755); err != nil {
		fatal(err)
	}
	out, err := os.Create(r5JSON)
	if err != nil {
		fatal(err)
	}
	cmd := exec.Command("go", "run", elfToJSON, r5Bin, "0x00", "0x08800000")
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = out.Close()
		fatal(err)
	}
	if err := out.Close(); err != nil {
		fatal(err)
	}

	fmt.Fprintln(os.Stderr, "running zkc (this decodes 2 x 780,000 bytes; expect several minutes)...")
	// -vvv is required: zkc gates printf output behind verbosity level PRINTF,
	// and both the "clock cycle:" trace and the guest's VERIFIER-MARK writes go
	// through printf. --fast executes for cycle counts only; the tracing path
	// lowers to a field machine for AIR constraints, which currently panics
	// under KOALABEAR_16 (see bench_compress/run.go for the full note).
	zkcCmd := exec.Command(zkcBin, "exec", "--fast", "-vvv", r5JSON, zkcMain)
	stdout, err := zkcCmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	zkcCmd.Stderr = os.Stderr
	if err := zkcCmd.Start(); err != nil {
		fatal(err)
	}
	stats, scanErr := parseTrace(stdout)
	waitErr := zkcCmd.Wait()
	if scanErr != nil || waitErr != nil {
		if scanErr != nil {
			fmt.Fprintln(os.Stderr, scanErr)
		}
		if waitErr != nil {
			fmt.Fprintf(os.Stderr, "zkc exec failed: %v\n", waitErr)
		}
		if len(stats.tail) != 0 {
			fmt.Fprintf(os.Stderr, "last zkc output:\n%s\n", strings.Join(stats.tail, "\n"))
		}
		os.Exit(1)
	}
	if stats.totalCycles == 0 {
		fmt.Fprintf(os.Stderr, "no cycles recorded; last zkc output:\n%s\n", strings.Join(stats.tail, "\n"))
		os.Exit(1)
	}

	bStart, ok1 := stats.markers[baseline.start]
	bEnd, ok2 := stats.markers[baseline.end]
	if !ok1 || !ok2 {
		fatal(fmt.Errorf("baseline markers not found in zkc output"))
	}
	baselineDelta := bEnd.cycle - bStart.cycle

	results := make([]result, 0, len(phases))
	for _, p := range phases {
		start, sOK := stats.markers[p.start]
		end, eOK := stats.markers[p.end]
		if !sOK || !eOK {
			fatal(fmt.Errorf("markers %d/%d for %q not found in zkc output", p.start, p.end, p.name))
		}
		// The end marker carries the decoded length; a short decode would make
		// the cycle count meaningless, so it is checked rather than reported.
		if end.value != decompressedLen {
			fatal(fmt.Errorf("%s decoded %d bytes, want %d", p.name, end.value, decompressedLen))
		}
		raw := end.cycle - start.cycle
		var net uint64
		if raw > baselineDelta {
			net = raw - baselineDelta
		}
		var size int64
		if fi, err := os.Stat(p.compressedFile); err == nil {
			size = fi.Size()
		}
		results = append(results, result{
			name:            p.name,
			net:             net,
			cyclesPerByte:   float64(net) / decompressedLen,
			compressedBytes: size,
		})
	}

	fmt.Printf("\ndecompressed bytes = %d per variant\n", decompressedLen)
	fmt.Printf("baseline (empty loop) = %d cycles, subtracted below\n\n", baselineDelta)
	fmt.Printf("%-28s  %14s  %14s  %12s  %10s\n", "variant", "compressed_B", "net_cycles", "cycles/byte", "ratio")
	fmt.Printf("%-28s  %14s  %14s  %12s  %10s\n", "---", "------------", "----------", "-----------", "-----")
	for _, r := range results {
		ratio := float64(decompressedLen) / float64(r.compressedBytes)
		fmt.Printf("%-28s  %14d  %14d  %12.2f  %10.3f\n",
			r.name, r.compressedBytes, r.net, r.cyclesPerByte, ratio)
	}

	if err := writeCSV(*outFlag, results); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write CSV: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "\nCSV written to %s\n", *outFlag)
	}
}

func writeCSV(path string, results []result) error {
	if err := os.MkdirAll("bench", 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"variant", "compressed_bytes", "decompressed_bytes", "net_cycles", "cycles_per_byte"}); err != nil {
		return err
	}
	for _, r := range results {
		if err := w.Write([]string{
			r.name,
			strconv.FormatInt(r.compressedBytes, 10),
			strconv.Itoa(decompressedLen),
			strconv.FormatUint(r.net, 10),
			strconv.FormatFloat(r.cyclesPerByte, 'f', 2, 64),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func parseTrace(r io.Reader) (traceStats, error) {
	stats := traceStats{markers: make(map[uint64]marker)}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		stats.tail = appendTail(stats.tail, line)
		if m := cycleRE.FindStringSubmatch(line); m != nil {
			stats.totalCycles, _ = strconv.ParseUint(m[1], 10, 64)
			continue
		}
		if m := markRE.FindStringSubmatch(line); m != nil {
			phase, _ := strconv.ParseUint(m[1], 10, 64)
			value, _ := strconv.ParseUint(m[2], 10, 64)
			stats.markers[phase] = marker{cycle: stats.totalCycles, value: value}
		}
	}
	return stats, scanner.Err()
}

func appendTail(tail []string, line string) []string {
	tail = append(tail, line)
	if len(tail) > tailLimit {
		tail = tail[len(tail)-tailLimit:]
	}
	return tail
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func fatal(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
