// Command verifier_profile profiles verifier-ray's real R5 verifier entrypoint
// through zkc and renders a compact markdown report.
//
// The command intentionally avoids a benchmark-only guest program. For each
// selected generated verifier case it:
//   - builds `src/main.zig` for the R5 target with the selected typed fixture
//     embedded at comptime;
//   - converts the ELF to the JSON input shape consumed by zkc;
//   - runs the shared RISC-V interpreter without `-q`;
//   - streams stdout line-by-line, extracting cycle counts, verifier phase
//     markers, Poseidon2 compression counts, and instruction frequencies;
//   - writes only the compact markdown report, not the full zkc trace.
//
// `raw` mode reports the least-instrumented total cycle count. `profiled` mode
// enables verifier profiling counters and R5 marker syscalls, which add a small
// amount of overhead but give phase-level attribution.
package main

import (
	"bufio"
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	defaultInput       = "valid"
	defaultInputOrigin = "0x08800000"
	defaultOutput      = "bench/verifier-profile.md"
	defaultTopCount    = 10
	traceTailLimit     = 40
	r5Bin              = "zig-out/bin/verifier-ray"
	r5JSON             = "zig-out/bin/verifier-ray.json"
	elfToJSON          = "../arithmetization/src/test/examples/scripts/elf_to_json_gen/main.go"
	zkcMain            = "../arithmetization/src/main/riscv/main.zkc"
	verifyFixture      = "testdata/generated/verify.zig"
	// Marker IDs must match verifier-ray/src/profiling.zig profiling.Mark.
	markVerifyStart    = 1
	markTranscriptDone = 2
	markVanishingStart = 3
	markVanishingDone  = 4
	markVerifyDone     = 5
)

var (
	cycleRE    = regexp.MustCompile(`clock cycle: ([0-9]+)`)
	markerRE   = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	metadataRE = regexp.MustCompile(`\.\{ \.name = "([^"]+)", \.module_count = ([0-9]+), \.dynamic_module_count = ([0-9]+), \.round_count = ([0-9]+), \.expression_count = ([0-9]+), \.bucket_count = ([0-9]+), \.vanishing_count = ([0-9]+), \.total_witness_claims = ([0-9]+), \.total_quotient_claims = ([0-9]+) \}`)
)

type caseMetadata struct {
	name                string
	moduleCount         uint64
	dynamicModuleCount  uint64
	roundCount          uint64
	expressionCount     uint64
	bucketCount         uint64
	vanishingCount      uint64
	totalWitnessClaims  uint64
	totalQuotientClaims uint64
}

// marker is one parsed `VERIFIER-MARK <phase> <value>` line. `cycle` is the
// latest zkc clock-cycle line seen before the marker was printed.
type marker struct {
	phase uint64
	value uint64
	cycle uint64
}

// traceStats is the compact summary recovered from the streamed zkc output.
type traceStats struct {
	totalCycles  uint64
	markers      map[uint64]marker
	instructions map[string]uint64
	tail         []string
}

type result struct {
	caseIndex int
	mode      string
	input     string
	metadata  caseMetadata
	stats     traceStats
}

type instructionCount struct {
	mnemonic string
	count    uint64
}

func main() {
	var (
		casesFlag = flag.String("cases", "0", "case selector: all, N, A-B, or comma-separated selectors")
		inputFlag = flag.String("input", defaultInput, "embedded input kind: valid or invalid")
		modeFlag  = flag.String("mode", "profiled", "run mode: raw, profiled, or both")
		outFlag   = flag.String("out", defaultOutput, "markdown output path")
		topFlag   = flag.Int("top-instructions", defaultTopCount, "number of top RISC-V instructions to print")
	)
	flag.Parse()

	if err := run(*casesFlag, *inputFlag, *modeFlag, *outFlag, *topFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(caseSelector, inputKind, mode, outPath string, topCount int) error {
	if inputKind != "valid" && inputKind != "invalid" {
		return fmt.Errorf("unsupported input %q: expected valid or invalid", inputKind)
	}
	modes, err := parseModes(mode)
	if err != nil {
		return err
	}
	metadata, err := readMetadata(verifyFixture)
	if err != nil {
		return err
	}
	cases, err := parseCaseSelector(caseSelector, len(metadata))
	if err != nil {
		return err
	}

	results := make([]result, 0, len(cases)*len(modes))
	for _, caseIndex := range cases {
		for _, runMode := range modes {
			fmt.Fprintf(os.Stderr, "profiling case %d (%s), mode=%s, input=%s\n", caseIndex, metadata[caseIndex].name, runMode, inputKind)
			stats, err := runCase(caseIndex, inputKind, runMode)
			if err != nil {
				return fmt.Errorf("case %d (%s), mode=%s: %w", caseIndex, metadata[caseIndex].name, runMode, err)
			}
			results = append(results, result{
				caseIndex: caseIndex,
				mode:      runMode,
				input:     inputKind,
				metadata:  metadata[caseIndex],
				stats:     stats,
			})
		}
	}

	report := renderMarkdown(results, topCount)
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	return nil
}

func parseModes(mode string) ([]string, error) {
	switch mode {
	case "raw":
		return []string{"raw"}, nil
	case "profiled":
		return []string{"profiled"}, nil
	case "both":
		return []string{"raw", "profiled"}, nil
	default:
		return nil, fmt.Errorf("unsupported mode %q: expected raw, profiled, or both", mode)
	}
}

func readMetadata(path string) ([]caseMetadata, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	matches := metadataRE.FindAllSubmatch(data, -1)
	if len(matches) == 0 {
		return nil, fmt.Errorf("no verifier case metadata found in %s", path)
	}
	metadata := make([]caseMetadata, 0, len(matches))
	for _, match := range matches {
		fields := make([]uint64, 0, 8)
		for _, raw := range match[2:] {
			value, err := strconv.ParseUint(string(raw), 10, 64)
			if err != nil {
				return nil, err
			}
			fields = append(fields, value)
		}
		metadata = append(metadata, caseMetadata{
			name:                string(match[1]),
			moduleCount:         fields[0],
			dynamicModuleCount:  fields[1],
			roundCount:          fields[2],
			expressionCount:     fields[3],
			bucketCount:         fields[4],
			vanishingCount:      fields[5],
			totalWitnessClaims:  fields[6],
			totalQuotientClaims: fields[7],
		})
	}
	return metadata, nil
}

func parseCaseSelector(selector string, caseCount int) ([]int, error) {
	selector = strings.TrimSpace(selector)
	if selector == "" {
		return nil, errors.New("empty case selector")
	}
	if selector == "all" {
		cases := make([]int, caseCount)
		for i := range cases {
			cases[i] = i
		}
		return cases, nil
	}

	seen := make(map[int]bool)
	var cases []int
	for _, part := range strings.Split(selector, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.Contains(part, "-") {
			parts := strings.Split(part, "-")
			if len(parts) != 2 {
				return nil, fmt.Errorf("invalid case range %q", part)
			}
			start, err := parseCaseIndex(strings.TrimSpace(parts[0]), caseCount)
			if err != nil {
				return nil, err
			}
			end, err := parseCaseIndex(strings.TrimSpace(parts[1]), caseCount)
			if err != nil {
				return nil, err
			}
			if start > end {
				return nil, fmt.Errorf("invalid case range %q: start is greater than end", part)
			}
			for i := start; i <= end; i++ {
				if !seen[i] {
					seen[i] = true
					cases = append(cases, i)
				}
			}
			continue
		}
		index, err := parseCaseIndex(part, caseCount)
		if err != nil {
			return nil, err
		}
		if !seen[index] {
			seen[index] = true
			cases = append(cases, index)
		}
	}
	if len(cases) == 0 {
		return nil, errors.New("case selector did not select any cases")
	}
	return cases, nil
}

func parseCaseIndex(raw string, caseCount int) (int, error) {
	index, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("invalid case index %q", raw)
	}
	if index < 0 || index >= caseCount {
		return 0, fmt.Errorf("case index %d out of range [0,%d)", index, caseCount)
	}
	return index, nil
}

// runCase builds one selected fixture, converts the R5 ELF to zkc JSON, and
// streams the zkc interpreter output back into traceStats.
func runCase(caseIndex int, inputKind, mode string) (traceStats, error) {
	buildArgs := []string{
		"build",
		"--release=small",
		"-Dstrip=true",
		"-Dr5=true",
		"-Dembedded-input=" + inputKind,
		fmt.Sprintf("-Dembedded-spec=%d", caseIndex),
	}
	if mode == "profiled" {
		buildArgs = append(buildArgs, "-Dverifier-profiling=true", "-Dr5-marks=true")
	}
	if err := runCommand("zig", buildArgs...); err != nil {
		return traceStats{}, err
	}
	if err := writeJSONInput(); err != nil {
		return traceStats{}, err
	}
	return runZKC()
}

func runCommand(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// writeJSONInput keeps using the existing ELF-to-JSON helper. The verifier input
// is already embedded into the binary, so the JSON input bytes are just a
// one-byte placeholder required by the helper.
func writeJSONInput() error {
	if err := os.MkdirAll(filepath.Dir(r5JSON), 0o755); err != nil {
		return err
	}
	out, err := os.Create(r5JSON)
	if err != nil {
		return err
	}
	defer out.Close()

	cmd := exec.Command("go", "run", elfToJSON, r5Bin, "0x00", defaultInputOrigin)
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runZKC intentionally does not pass `-q`: the cycle lines, instruction
// mnemonics, and marker writes are all printed on stdout by the shared zkc
// RISC-V runner.
func runZKC() (traceStats, error) {
	cmd := exec.Command("zkc", "exec", r5JSON, zkcMain)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return traceStats{}, err
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return traceStats{}, err
	}

	stats, scanErr := parseTrace(stdout)
	waitErr := cmd.Wait()
	if scanErr != nil {
		return traceStats{}, scanErr
	}
	if waitErr != nil {
		return traceStats{}, fmt.Errorf("%w\nlast zkc output:\n%s", waitErr, strings.Join(stats.tail, "\n"))
	}
	if stats.totalCycles == 0 {
		return traceStats{}, errors.New("zkc trace did not contain a clock cycle line")
	}
	return stats, nil
}

// parseTrace consumes zkc output incrementally. It keeps only the current
// summary plus a small tail for useful error messages when zkc fails.
func parseTrace(stdout io.Reader) (traceStats, error) {
	stats := traceStats{
		markers:      make(map[uint64]marker),
		instructions: make(map[string]uint64),
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	awaitingMnemonic := false
	for scanner.Scan() {
		line := scanner.Text()
		stats.tail = appendTail(stats.tail, line)
		if match := cycleRE.FindStringSubmatch(line); len(match) == 2 {
			cycle, err := strconv.ParseUint(match[1], 10, 64)
			if err != nil {
				return traceStats{}, err
			}
			stats.totalCycles = cycle
			awaitingMnemonic = true
			continue
		}
		if match := markerRE.FindStringSubmatch(line); len(match) == 3 {
			phase, err := strconv.ParseUint(match[1], 10, 64)
			if err != nil {
				return traceStats{}, err
			}
			value, err := strconv.ParseUint(match[2], 10, 64)
			if err != nil {
				return traceStats{}, err
			}
			stats.markers[phase] = marker{phase: phase, value: value, cycle: stats.totalCycles}
			continue
		}
		if awaitingMnemonic {
			if mnemonic := parseMnemonic(line); mnemonic != "" {
				stats.instructions[mnemonic]++
				awaitingMnemonic = false
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return traceStats{}, err
	}
	return stats, nil
}

func appendTail(tail []string, line string) []string {
	if len(tail) == traceTailLimit {
		copy(tail, tail[1:])
		tail[len(tail)-1] = line
		return tail
	}
	return append(tail, line)
}

func parseMnemonic(line string) string {
	fields := strings.Fields(strings.TrimSpace(line))
	if len(fields) == 0 {
		return ""
	}
	mnemonic := strings.TrimRight(fields[0], ":")
	if knownMnemonics[mnemonic] {
		return mnemonic
	}
	return ""
}

func renderMarkdown(results []result, topCount int) string {
	var out bytes.Buffer
	fmt.Fprintln(&out, "# Verifier R5 Profiling")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "This report is generated from the shared `src/main.zig` R5 verifier path and the shared `arithmetization/src/main/riscv/main.zkc` interpreter. The parser streams `zkc exec` output directly and does not store the full instruction trace.")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Case | Name | Mode | Input | Total cycles | Verifier cycles | Transcript cycles | Vanishing cycles | Poseidon2 compressions | Top instructions |")
	fmt.Fprintln(&out, "| ---: | --- | --- | --- | ---: | ---: | ---: | ---: | ---: | --- |")
	for _, result := range results {
		fmt.Fprintf(
			&out,
			"| %d | %s | %s | %s | %d | %s | %s | %s | %s | %s |\n",
			result.caseIndex,
			escapeCell(result.metadata.name),
			result.mode,
			result.input,
			result.stats.totalCycles,
			cycleDelta(result.stats.markers, markVerifyStart, markVerifyDone),
			cycleDelta(result.stats.markers, markVerifyStart, markTranscriptDone),
			cycleDelta(result.stats.markers, markVanishingStart, markVanishingDone),
			markerValue(result.stats.markers, markVerifyDone),
			escapeCell(topInstructions(result.stats.instructions, topCount)),
		)
	}
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "## Fixture Metadata")
	fmt.Fprintln(&out)
	fmt.Fprintln(&out, "| Case | Modules | Dynamic modules | Rounds | Expressions | Buckets | Vanishings | Witness claims | Quotient claims |")
	fmt.Fprintln(&out, "| ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: | ---: |")
	for _, result := range results {
		fmt.Fprintf(
			&out,
			"| %d | %d | %d | %d | %d | %d | %d | %d | %d |\n",
			result.caseIndex,
			result.metadata.moduleCount,
			result.metadata.dynamicModuleCount,
			result.metadata.roundCount,
			result.metadata.expressionCount,
			result.metadata.bucketCount,
			result.metadata.vanishingCount,
			result.metadata.totalWitnessClaims,
			result.metadata.totalQuotientClaims,
		)
	}
	return out.String()
}

func cycleDelta(markers map[uint64]marker, startPhase, endPhase uint64) string {
	start, ok := markers[startPhase]
	if !ok {
		return "-"
	}
	end, ok := markers[endPhase]
	if !ok {
		return "-"
	}
	if end.cycle < start.cycle {
		return "-"
	}
	return strconv.FormatUint(end.cycle-start.cycle, 10)
}

func markerValue(markers map[uint64]marker, phase uint64) string {
	marker, ok := markers[phase]
	if !ok {
		return "-"
	}
	return strconv.FormatUint(marker.value, 10)
}

func topInstructions(counts map[string]uint64, limit int) string {
	if len(counts) == 0 || limit <= 0 {
		return "-"
	}
	items := make([]instructionCount, 0, len(counts))
	for mnemonic, count := range counts {
		items = append(items, instructionCount{mnemonic: mnemonic, count: count})
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].count == items[j].count {
			return items[i].mnemonic < items[j].mnemonic
		}
		return items[i].count > items[j].count
	})
	if len(items) > limit {
		items = items[:limit]
	}
	parts := make([]string, len(items))
	for i, item := range items {
		parts[i] = fmt.Sprintf("%s %d", item.mnemonic, item.count)
	}
	return strings.Join(parts, ", ")
}

func escapeCell(value string) string {
	return strings.ReplaceAll(value, "|", "\\|")
}

var knownMnemonics = map[string]bool{
	"ADD":    true,
	"ADDI":   true,
	"ADDIW":  true,
	"ADDW":   true,
	"AND":    true,
	"ANDI":   true,
	"AUIPC":  true,
	"BEQ":    true,
	"BGE":    true,
	"BGEU":   true,
	"BLT":    true,
	"BLTU":   true,
	"BNE":    true,
	"DIV":    true,
	"DIVU":   true,
	"DIVUW":  true,
	"DIVW":   true,
	"ECALL":  true,
	"JAL":    true,
	"JALR":   true,
	"KECCAK": true,
	"LB":     true,
	"LBU":    true,
	"LD":     true,
	"LH":     true,
	"LHU":    true,
	"LUI":    true,
	"LW":     true,
	"LWU":    true,
	"MUL":    true,
	"MULH":   true,
	"MULHSU": true,
	"MULHU":  true,
	"MULW":   true,
	"OR":     true,
	"ORI":    true,
	"REM":    true,
	"REMU":   true,
	"REMUW":  true,
	"REMW":   true,
	"SB":     true,
	"SD":     true,
	"SH":     true,
	"SLL":    true,
	"SLLI":   true,
	"SLLIW":  true,
	"SLLW":   true,
	"SLT":    true,
	"SLTI":   true,
	"SLTIU":  true,
	"SLTU":   true,
	"SRA":    true,
	"SRAI":   true,
	"SRAIW":  true,
	"SRAW":   true,
	"SRL":    true,
	"SRLI":   true,
	"SRLIW":  true,
	"SRLW":   true,
	"SUB":    true,
	"SUBW":   true,
	"SW":     true,
	"XOR":    true,
	"XORI":   true,
}
