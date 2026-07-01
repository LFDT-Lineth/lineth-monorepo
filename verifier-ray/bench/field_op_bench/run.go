// Runner for the field_op_bench micro-benchmark.
// Builds the R5 ELF, converts to zkc JSON, runs zkc, and prints a cycle table.
package main

import (
	"bufio"
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
	r5Bin          = "zig-out/bin/field-op-bench"
	r5JSON         = "zig-out/bin/field-op-bench.json"
	n              = 1000
	traceTailLimit = 40
)

// The baseline phase runs the same loop with an empty body; its delta is the
// loop-counter / branch overhead common to every op phase and is subtracted out.
var baseline = struct {
	start uint64
	end   uint64
}{0, 1}

// Each phase has a start and end marker bracketing only the loop body.
var phases = []struct {
	name  string
	start uint64
	end   uint64
}{
	{"add", 10, 11},
	{"sub", 20, 21},
	{"neg", 30, 31},
	{"double", 40, 41},
	{"mul", 50, 51},
	{"square", 60, 61},
	{"pow(x,2^20)", 70, 71},
	{"powComptime(2^20)", 72, 73},
	{"inverse", 80, 81},
	{"div", 90, 91},
	{"ext/add", 100, 101},
	{"ext/sub", 110, 111},
	{"ext/mul", 120, 121},
	{"ext/square", 130, 131},
	{"ext/inverse", 140, 141},
	{"ext/div", 150, 151},
	{"ext/mulByBase", 160, 161},
}

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
	zkcBin := "zkc"
	if len(os.Args) > 1 {
		zkcBin = os.Args[1]
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

	// Compute the baseline loop overhead (empty-body loop). Subtracting it from
	// each op's delta isolates the pure per-operation cost.
	bStart, bStartOK := stats.markers[baseline.start]
	bEnd, bEndOK := stats.markers[baseline.end]
	if !bStartOK || !bEndOK {
		fmt.Fprintln(os.Stderr, "baseline markers not found in zkc output")
		os.Exit(1)
	}
	baselineDelta := bEnd.cycle - bStart.cycle

	fmt.Printf("\nN = %d\n", n)
	fmt.Printf("baseline (empty loop) = %d cycles (%.2f/iter), subtracted below\n\n",
		baselineDelta, float64(baselineDelta)/n)
	fmt.Printf("%-20s  %12s  %12s  %10s\n", "op", "raw_cycles", "net_cycles", "cycles/op")
	fmt.Printf("%-20s  %12s  %12s  %10s\n", "---", "----------", "----------", "---------")
	for _, p := range phases {
		start, startOK := stats.markers[p.start]
		end, endOK := stats.markers[p.end]
		if !startOK || !endOK {
			fmt.Printf("%-20s  %12s  %12s  %10s\n", p.name, "-", "-", "-")
			continue
		}
		raw := end.cycle - start.cycle
		// Guard against a negative net if an op loop somehow measures below the
		// baseline (e.g. noise on a near-zero op); clamp to 0.
		var net uint64
		if raw > baselineDelta {
			net = raw - baselineDelta
		}
		fmt.Printf("%-20s  %12d  %12d  %10.2f\n", p.name, raw, net, float64(net)/n)
	}
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
