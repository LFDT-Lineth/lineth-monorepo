// Runner for the bench_zstd_c micro-benchmark.
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
	"sort"
	"strconv"
	"strings"
)

const (
	elfToJSON     = "../../../arithmetization/src/test/scripts/elf_to_json_gen/main.go"
	zkcMain       = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin         = "zig-out/bin/bench-zstd-c"
	r5JSON        = "zig-out/bin/bench-zstd-c.json"
	tailLimit     = 40
	defaultOutput = "bench/bench-zstd-c.csv"
	// Must match decompressed_len in main.zig — update both together.
	decompressedLen = 262_144
	// FNV-1a (64-bit) of the first decompressedLen bytes of
	// ~/linea-blob-corpus/payloads/2026-07-28_recent.payload.bin, the plaintext
	// both fixtures were compressed from. The guest recomputes this over its
	// own output so a decode that is fast because it is wrong cannot be
	// reported as an improvement.
	expectedFNV1a = 7520853736309397853
)

// Marker pairs, matching the IDs main.zig writes.
var phases = []struct {
	name           string
	start, end     uint64
	hash           uint64
	compressedFile string
}{
	{"zstd -19 (C reference v1.5.7)", 10, 11, 12, "zstd_compressed.bin"},
}

var baseline = struct{ start, end uint64 }{0, 1}

var (
	markRE  = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE = regexp.MustCompile(`PC=([0-9]+), clock cycle: ([0-9]+)`)
	// A disassembly line, e.g. "ANDI a3, a3, 0x3f" or "LD a4, 0x10(a0)".
	mnemonicRE = regexp.MustCompile(`^([A-Z][A-Z0-9_.]*)\s`)
)

type marker struct{ cycle, value uint64 }

type traceStats struct {
	totalCycles uint64
	pcCounts    map[uint64]uint64 // nil unless -pcprof
	markers     map[uint64]marker
	// Executed instruction counts by mnemonic. zkc's -vvv trace disassembles
	// every retired instruction, so this is what actually ran -- the only way
	// to confirm on this target whether wide (ld/sd) or byte-at-a-time
	// (lbu/sb) accesses were emitted for the backref copy.
	mnemonics map[string]uint64
	tail      []string
}

type result struct {
	name            string
	net             uint64
	cyclesPerByte   float64
	compressedBytes int64
}

func main() {
	outFlag := flag.String("out", defaultOutput, "CSV output path")
	pcprofFlag := flag.Bool("pcprof", false,
		"attribute cycles to functions via the trace's PC stream and the ELF symbol table")
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

	fmt.Fprintln(os.Stderr, "running zkc (this decodes 262,144 bytes)...")
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
	var pcCounts map[uint64]uint64
	if *pcprofFlag {
		pcCounts = make(map[uint64]uint64)
	}
	stats, scanErr := parseTrace(stdout, pcCounts)
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
		h, hOK := stats.markers[p.hash]
		if !hOK {
			fatal(fmt.Errorf("hash marker %d for %q not found in zkc output", p.hash, p.name))
		}
		if h.value != expectedFNV1a {
			fatal(fmt.Errorf("%s decoded to the wrong bytes: FNV-1a %d, want %d",
				p.name, h.value, expectedFNV1a))
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

	printMix(stats)

	if *pcprofFlag {
		// Persist the raw counts so attribution can be re-derived offline
		// (e.g. after fixing symbolization) without another zkc run.
		if err := writePCCounts("bench/pc-counts.csv", stats.pcCounts); err != nil {
			fmt.Fprintf(os.Stderr, "warning: could not write pc counts: %v\n", err)
		}
		if err := printPCProfile(stats.pcCounts); err != nil {
			fmt.Fprintf(os.Stderr, "warning: pc profile failed: %v\n", err)
		}
	}

	if err := writeCSV(*outFlag, results); err != nil {
		fmt.Fprintf(os.Stderr, "warning: could not write CSV: %v\n", err)
	} else {
		fmt.Fprintf(os.Stderr, "\nCSV written to %s\n", *outFlag)
	}
}

// printMix reports the executed instruction mix: the top mnemonics, and the
// wide-vs-byte access counts that determine whether the backref copy moved
// words or bytes.
// printPCProfile attributes executed-instruction counts to functions by
// binary-searching each PC against the ELF symbol table (via `go tool nm`,
// which reads ELF). Requires the guest to be built with strip = false.
func writePCCounts(path string, pcCounts map[uint64]uint64) error {
	if err := os.MkdirAll("bench", 0o755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	w := csv.NewWriter(f)
	if err := w.Write([]string{"pc", "count"}); err != nil {
		return err
	}
	pcs := make([]uint64, 0, len(pcCounts))
	for pc := range pcCounts {
		pcs = append(pcs, pc)
	}
	sort.Slice(pcs, func(i, j int) bool { return pcs[i] < pcs[j] })
	for _, pc := range pcs {
		if err := w.Write([]string{
			"0x" + strconv.FormatUint(pc, 16),
			strconv.FormatUint(pcCounts[pc], 10),
		}); err != nil {
			return err
		}
	}
	w.Flush()
	return w.Error()
}

func printPCProfile(pcCounts map[uint64]uint64) error {
	nmOut, err := exec.Command("go", "tool", "nm", r5Bin).Output()
	if err != nil {
		return fmt.Errorf("go tool nm: %w", err)
	}
	type sym struct {
		addr uint64
		name string
	}
	var syms []sym
	for _, line := range strings.Split(string(nmOut), "\n") {
		f := strings.Fields(line)
		// "addr T name" -- keep text symbols only.
		if len(f) != 3 || (f[1] != "T" && f[1] != "t") {
			continue
		}
		// Zig's symtab interleaves `.L*` local labels and `$x`/`$d` mapping
		// symbols at nearly every address; only real function symbols are
		// useful attribution targets.
		if strings.HasPrefix(f[2], ".L") || strings.HasPrefix(f[2], "$") {
			continue
		}
		addr, err := strconv.ParseUint(f[0], 16, 64)
		if err != nil {
			continue
		}
		syms = append(syms, sym{addr, f[2]})
	}
	if len(syms) == 0 {
		return fmt.Errorf("no text symbols; is the guest built with strip = false?")
	}
	sort.Slice(syms, func(i, j int) bool { return syms[i].addr < syms[j].addr })

	perFunc := make(map[string]uint64)
	var total uint64
	for pc, n := range pcCounts {
		i := sort.Search(len(syms), func(i int) bool { return syms[i].addr > pc }) - 1
		name := "???"
		if i >= 0 {
			name = syms[i].name
		}
		perFunc[name] += n
		total += n
	}
	type fc struct {
		name  string
		count uint64
	}
	var funcs []fc
	for k, v := range perFunc {
		funcs = append(funcs, fc{k, v})
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].count > funcs[j].count })
	fmt.Printf("\ncycles by function (whole run, including both marker regions and setup):\n")
	for i, f := range funcs {
		if i >= 20 || f.count*1000 < total {
			break
		}
		fmt.Printf("  %-64s %12d  %6.2f%%\n", f.name, f.count, 100*float64(f.count)/float64(total))
	}
	return nil
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

	fmt.Printf("\nexecuted instruction mix (%d retired, %d distinct mnemonics)\n", total, len(all))
	fmt.Printf("%-12s  %14s  %8s\n", "mnemonic", "count", "share")
	for i, e := range all {
		if i >= 15 {
			break
		}
		fmt.Printf("%-12s  %14d  %7.2f%%\n", e.name, e.count, 100*float64(e.count)/float64(total))
	}
	fmt.Printf("\nmemory access width:\n")
	for _, m := range []string{"LD", "SD", "LW", "SW", "LHU", "SH", "LBU", "LB", "SB"} {
		if c, ok := stats.mnemonics[m]; ok {
			fmt.Printf("  %-4s %12d\n", m, c)
		}
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

func parseTrace(r io.Reader, pcCounts map[uint64]uint64) (traceStats, error) {
	stats := traceStats{markers: make(map[uint64]marker), mnemonics: make(map[string]uint64), pcCounts: pcCounts}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		stats.tail = appendTail(stats.tail, line)
		if m := cycleRE.FindStringSubmatch(line); m != nil {
			stats.totalCycles, _ = strconv.ParseUint(m[2], 10, 64)
			if stats.pcCounts != nil {
				pc, _ := strconv.ParseUint(m[1], 10, 64)
				stats.pcCounts[pc]++
			}
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
