// Runner for the bench_compress micro-benchmark.
// Builds the R5 ELF, converts to zkc JSON, runs zkc, prints cycles/compress,
// and writes a CSV report.
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
	elfToJSON      = "../../../arithmetization/src/test/scripts/elf_to_json_gen/main.go"
	zkcMain        = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin          = "zig-out/bin/bench-compress"
	r5JSON         = "zig-out/bin/bench-compress.json"
	traceTailLimit = 40
	defaultOutput  = "bench/bench-compress.csv"
	// n must match const N in main.zig — update both together.
	n = 10
)

var baseline = struct {
	start uint64
	end   uint64
}{0, 1}

var compressPhase = struct {
	start uint64
	end   uint64
}{10, 11}

var (
	markRE  = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE = regexp.MustCompile(`clock cycle: ([0-9]+)`)
)

type marker struct {
	cycle uint64
	value uint64
}

type traceStats struct {
	totalCycles uint64
	markers     map[uint64]marker
	tail        []string
}

func main() {
	outFlag := flag.String("out", defaultOutput, "CSV output path")
	flag.Parse()
	args := flag.Args()
	zkcBin := "zkc"
	if len(args) > 0 {
		zkcBin = args[0]
	}

	fmt.Fprintln(os.Stderr, "building R5 ELF...")
	if err := run("zig", "build", "--release=small"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "converting ELF to zkc JSON...")
	if err := os.MkdirAll("zig-out/bin", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := os.Create(r5JSON)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	cmd := exec.Command("go", "run", elfToJSON, r5Bin, "0x00", "0x08800000")
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = out.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := out.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	fmt.Fprintln(os.Stderr, "running zkc...")
	// --fast: execute for cycle counts only. The default (tracing) mode lowers
	// the word machine to a field machine for AIR constraints, which currently
	// panics under KOALABEAR_16 — the 32-bit `instruction` register exceeds the
	// 16-bit field register width and register splitting (--split) is incomplete
	// for multi-limb arithmetic. The benchmark only needs cycle counts, so the
	// trace/AIR path is unnecessary.
	zkcCmd := exec.Command(zkcBin, "exec", "--fast", r5JSON, zkcMain)
	stdout, err := zkcCmd.StdoutPipe()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	zkcCmd.Stderr = os.Stderr
	if err := zkcCmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
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

	bStart, bStartOK := stats.markers[baseline.start]
	bEnd, bEndOK := stats.markers[baseline.end]
	if !bStartOK || !bEndOK {
		fmt.Fprintln(os.Stderr, "baseline markers not found in zkc output")
		os.Exit(1)
	}
	baselineDelta := bEnd.cycle - bStart.cycle

	start, startOK := stats.markers[compressPhase.start]
	end, endOK := stats.markers[compressPhase.end]
	if !startOK || !endOK {
		fmt.Fprintln(os.Stderr, "compression markers not found in zkc output")
		os.Exit(1)
	}

	raw := end.cycle - start.cycle
	var net uint64
	if raw > baselineDelta {
		net = raw - baselineDelta
	}

	cyclesPerCall := float64(net) / n

	fmt.Printf("\nN = %d\n", n)
	fmt.Printf("baseline (empty loop) = %d cycles (%.2f/iter), subtracted below\n\n",
		baselineDelta, float64(baselineDelta)/n)
	fmt.Printf("%-28s  %12s  %12s  %12s\n", "operation", "raw_cycles", "net_cycles", "cycles/call")
	fmt.Printf("%-28s  %12s  %12s  %12s\n", "---", "----------", "----------", "-----------")
	fmt.Printf("%-28s  %12d  %12d  %12.2f\n", "poseidon2 compress", raw, net, cyclesPerCall)

	if err := writeCSV(*outFlag, cyclesPerCall); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write CSV: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "CSV written to %s\n", *outFlag)
	}
}

func writeCSV(path string, cyclesPerCall float64) error {
	if err := os.MkdirAll("bench", 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"operation", "cycles_per_call"}); err != nil {
		return err
	}
	if err := w.Write([]string{"poseidon2 compress", strconv.FormatFloat(cyclesPerCall, 'f', 2, 64)}); err != nil {
		return err
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
	if len(tail) == traceTailLimit {
		copy(tail, tail[1:])
		tail[len(tail)-1] = line
		return tail
	}
	return append(tail, line)
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
