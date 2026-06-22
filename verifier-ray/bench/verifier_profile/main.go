// Command verifier_profile profiles verifier-ray's real R5 verifier entrypoint
// through zkc and renders a compact CSV report.
//
// The command intentionally avoids a benchmark-only guest program. For each
// generated verifier case it:
//   - builds `src/main.zig` for the R5 target with the selected typed fixture
//     embedded at comptime;
//   - converts the ELF to the JSON input shape consumed by zkc;
//   - runs the shared zkc RISC-V interpreter;
//   - streams the verbose trace and keeps only total cycles, verifier phase
//     cycles, and Poseidon2 compression counts;
//   - writes the compact CSV report.
package main

import (
	"bufio"
	"bytes"
	"encoding/csv"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	defaultInput       = "valid"
	defaultInputOrigin = "0x08800000"
	defaultOutput      = "bench/verifier-profile.csv"
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

// marker is one verifier phase checkpoint recovered from the shared zkc trace.
type marker struct {
	phase uint64
	value uint64
	cycle uint64
}

// traceStats is the compact summary recovered from the zkc stdout stream.
type traceStats struct {
	totalCycles uint64
	markers     map[uint64]marker
	tail        []string
}

type result struct {
	caseIndex int
	metadata  caseMetadata
	stats     traceStats
}

func main() {
	outFlag := flag.String("out", defaultOutput, "CSV output path")
	flag.Parse()

	if err := run(*outFlag); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(outPath string) error {
	metadata, err := readMetadata(verifyFixture)
	if err != nil {
		return err
	}

	results := make([]result, 0, len(metadata))
	for caseIndex, caseMetadata := range metadata {
		fmt.Fprintf(os.Stderr, "profiling case %d (%s)\n", caseIndex, caseMetadata.name)
		stats, err := runCase(caseIndex)
		if err != nil {
			return fmt.Errorf("case %d (%s): %w", caseIndex, caseMetadata.name, err)
		}
		results = append(results, result{
			caseIndex: caseIndex,
			metadata:  caseMetadata,
			stats:     stats,
		})
	}

	report, err := renderCSV(results)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(outPath, []byte(report), 0o644); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "wrote %s\n", outPath)
	return nil
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

// runCase builds one selected fixture, converts the R5 ELF to zkc JSON, and
// parses the shared zkc interpreter trace back into traceStats.
func runCase(caseIndex int) (traceStats, error) {
	buildArgs := []string{
		"build",
		"--release=small",
		"-Dstrip=true",
		"-Dr5=true",
		"-Dembedded-input=" + defaultInput,
		fmt.Sprintf("-Dembedded-spec=%d", caseIndex),
		"-Dverifier-profiling=true",
		"-Dr5-marks=true",
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

// runZKC uses the shared RISC-V runner rather than a benchmark-specific zkc
// wrapper. The runner is verbose, so parseTrace streams stdout and keeps only
// the lines needed for the compact report.
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
		return traceStats{}, errors.New("zkc output did not contain a total cycle count")
	}
	return stats, nil
}

// parseTrace consumes the shared zkc RISC-V trace. It intentionally ignores the
// instruction trace and records only the latest clock cycle plus verifier marker
// writes. A small tail is kept for useful error messages when zkc fails.
func parseTrace(stdout io.Reader) (traceStats, error) {
	stats := traceStats{
		markers: make(map[uint64]marker),
	}
	scanner := bufio.NewScanner(stdout)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)

	for scanner.Scan() {
		line := scanner.Text()
		stats.tail = appendTail(stats.tail, line)

		if match := cycleRE.FindStringSubmatch(line); len(match) == 2 {
			cycle, err := strconv.ParseUint(match[1], 10, 64)
			if err != nil {
				return traceStats{}, err
			}
			stats.totalCycles = cycle
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

func renderCSV(results []result) (string, error) {
	var out bytes.Buffer
	writer := csv.NewWriter(&out)
	if err := writer.Write([]string{
		"case",
		"name",
		"total_cycles",
		"verifier_cycles",
		"transcript_cycles",
		"vanishing_cycles",
		"poseidon2_compressions",
		"module_count",
		"dynamic_module_count",
		"round_count",
		"expression_count",
		"bucket_count",
		"vanishing_count",
		"total_witness_claims",
		"total_quotient_claims",
	}); err != nil {
		return "", err
	}
	for _, result := range results {
		if err := writer.Write([]string{
			strconv.Itoa(result.caseIndex),
			result.metadata.name,
			strconv.FormatUint(result.stats.totalCycles, 10),
			cycleDelta(result.stats.markers, markVerifyStart, markVerifyDone),
			cycleDelta(result.stats.markers, markVerifyStart, markTranscriptDone),
			cycleDelta(result.stats.markers, markVanishingStart, markVanishingDone),
			markerValue(result.stats.markers, markVerifyDone),
			strconv.FormatUint(result.metadata.moduleCount, 10),
			strconv.FormatUint(result.metadata.dynamicModuleCount, 10),
			strconv.FormatUint(result.metadata.roundCount, 10),
			strconv.FormatUint(result.metadata.expressionCount, 10),
			strconv.FormatUint(result.metadata.bucketCount, 10),
			strconv.FormatUint(result.metadata.vanishingCount, 10),
			strconv.FormatUint(result.metadata.totalWitnessClaims, 10),
			strconv.FormatUint(result.metadata.totalQuotientClaims, 10),
		}); err != nil {
			return "", err
		}
	}
	writer.Flush()
	if err := writer.Error(); err != nil {
		return "", err
	}
	return out.String(), nil
}

func cycleDelta(markers map[uint64]marker, startPhase, endPhase uint64) string {
	value, ok := cycleDeltaValue(markers, startPhase, endPhase)
	if !ok {
		return ""
	}
	return strconv.FormatUint(value, 10)
}

func cycleDeltaValue(markers map[uint64]marker, startPhase, endPhase uint64) (uint64, bool) {
	start, ok := markers[startPhase]
	if !ok {
		return 0, false
	}
	end, ok := markers[endPhase]
	if !ok {
		return 0, false
	}
	if end.cycle < start.cycle {
		return 0, false
	}
	return end.cycle - start.cycle, true
}

func markerValue(markers map[uint64]marker, phase uint64) string {
	value, ok := markerValueRaw(markers, phase)
	if !ok {
		return ""
	}
	return strconv.FormatUint(value, 10)
}

func markerValueRaw(markers map[uint64]marker, phase uint64) (uint64, bool) {
	marker, ok := markers[phase]
	if !ok {
		return 0, false
	}
	return marker.value, true
}
