// Runner for the bench_bitread micro-benchmark.
// Builds the R5 ELF, converts to zkc JSON, runs zkc, and reports per-call cost
// for each bit-reader design, plus the executed instruction mix.
//
// The instruction mix is the point as much as the cycle counts: it shows
// whether this target executes wide (LD) or byte-at-a-time (LBU) loads, which
// is what determines whether a wide-load reload design can help at all.
package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	elfToJSON = "../../../arithmetization/src/test/scripts/elf_to_json_gen/main.go"
	zkcMain   = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin     = "zig-out/bin/bench-bitread"
	r5JSON    = "zig-out/bin/bench-bitread.json"
	tailLimit = 40
	// Must match N in main.zig — update both together.
	n = 4096
)

var readers = []struct {
	name       string
	width      int
	start, end uint64
}{
	{"Legacy (bit at a time)", 8, 10, 11},
	{"Legacy (bit at a time)", 21, 20, 21},
	{"Accum (byte-loop refill)", 8, 30, 31},
	{"Accum (byte-loop refill)", 21, 40, 41},
	{"MsbWide (load + byteswap)", 8, 50, 51},
	{"MsbWide (load + byteswap)", 21, 60, 61},
	{"LsbWide (native load)", 8, 70, 71},
	{"LsbWide (native load)", 21, 80, 81},
}

var (
	markRE     = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE    = regexp.MustCompile(`clock cycle: ([0-9]+)`)
	mnemonicRE = regexp.MustCompile(`^([A-Z][A-Z0-9_.]*)\s`)
)

type marker struct{ cycle, value uint64 }

type traceStats struct {
	totalCycles uint64
	markers     map[uint64]marker
	mnemonics   map[string]uint64
	tail        []string
}

func main() {
	zkcBin := "zkc"
	if len(os.Args) > 1 {
		zkcBin = os.Args[1]
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

	fmt.Fprintln(os.Stderr, "running zkc...")
	// -vvv is required: zkc gates printf output behind verbosity level PRINTF.
	// --fast executes for cycle counts only (see bench_compress/run.go).
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

	bStart, ok1 := stats.markers[0]
	bEnd, ok2 := stats.markers[1]
	if !ok1 || !ok2 {
		fatal(fmt.Errorf("baseline markers not found"))
	}
	baseline := bEnd.cycle - bStart.cycle

	fmt.Printf("\nN = %d reads per reader; one refill/ensure per read, sized for a 41-bit symbol\n", n)
	fmt.Printf("baseline (empty loop) = %d cycles, subtracted below\n\n", baseline)
	fmt.Printf("%-28s %6s %14s %14s\n", "reader", "bits", "cycles/call", "cycles/bit")
	fmt.Printf("%-28s %6s %14s %14s\n", "---", "----", "-----------", "----------")
	for _, r := range readers {
		s, sOK := stats.markers[r.start]
		e, eOK := stats.markers[r.end]
		if !sOK || !eOK {
			fatal(fmt.Errorf("markers %d/%d for %q not found", r.start, r.end, r.name))
		}
		raw := e.cycle - s.cycle
		var net uint64
		if raw > baseline {
			net = raw - baseline
		}
		perCall := float64(net) / n
		fmt.Printf("%-28s %6d %14.2f %14.2f\n", r.name, r.width, perCall, perCall/float64(r.width))
	}

	printMix(stats)
}

func printMix(stats traceStats) {
	type kv struct {
		name  string
		count uint64
	}
	all := make([]kv, 0, len(stats.mnemonics))
	var total uint64
	for k, v := range stats.mnemonics {
		all = append(all, kv{k, v})
		total += v
	}
	sort.Slice(all, func(i, j int) bool { return all[i].count > all[j].count })
	fmt.Printf("\nexecuted instruction mix (%d retired)\n", total)
	for i, e := range all {
		if i >= 10 {
			break
		}
		fmt.Printf("  %-10s %12d  %6.2f%%\n", e.name, e.count, 100*float64(e.count)/float64(total))
	}
	fmt.Printf("\nmemory access width:\n")
	for _, m := range []string{"LD", "SD", "LW", "SW", "LHU", "LBU", "SB"} {
		fmt.Printf("  %-4s %12d\n", m, stats.mnemonics[m])
	}
}

func parseTrace(r io.Reader) (traceStats, error) {
	stats := traceStats{markers: make(map[uint64]marker), mnemonics: make(map[string]uint64)}
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
			continue
		}
		if m := mnemonicRE.FindStringSubmatch(line); m != nil {
			stats.mnemonics[m[1]]++
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
