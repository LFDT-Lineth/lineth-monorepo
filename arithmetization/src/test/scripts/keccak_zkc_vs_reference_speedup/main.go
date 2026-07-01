package main

// Examples, from the repository root:
//   GO111MODULE=off go run ./arithmetization/src/test/scripts/keccak_zkc_vs_reference_speedup --intervals 10 --size 10
//   GO111MODULE=off go run ./arithmetization/src/test/scripts/keccak_zkc_vs_reference_speedup --single-input-lengths 512,1024,2048,4096,8192,16384,32768,65536

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
)

const batchedInputBytes = 512

var timeRE = regexp.MustCompile(`^(real|user|sys)\s+([0-9]+(?:[\.,][0-9]+)?)$`)

type config struct {
	intervals          int
	size               int
	start              int
	makefileDir        string
	timeBin            string
	singleInputLengths string
}

func main() {
	// Select the benchmark mode
	cfg := parseArgs()

	if cfg.singleInputLengths != "" {
		lengths := parseInputLengthList(cfg.singleInputLengths)
		fmt.Printf(
			"| %-11s | %-29s | %-28s | %-7s |\n",
			"input bytes", "KECCAK_ACCEL=false real (s)", "KECCAK_ACCEL=true real (s)", "speedup",
		)
		fmt.Println("| ----------- | ----------------------------- | ---------------------------- | ------- |")
		for _, length := range lengths {
			falseTime := runTimedSingle(cfg.makefileDir, cfg.timeBin, false, length)
			trueTime := runTimedSingle(cfg.makefileDir, cfg.timeBin, true, length)
			printSingleRow(length, falseTime, trueTime)
		}
		return
	}

	if cfg.intervals < 1 {
		fatal("--intervals must be >= 1")
	}
	if cfg.size < 1 {
		fatal("--size must be >= 1")
	}
	if cfg.start < 0 {
		fatal("--start must be >= 0")
	}

	fmt.Printf(
		"| %-31s | %-29s | %-28s | %-7s |\n",
		fmt.Sprintf("batched range (%d-byte inputs)", batchedInputBytes),
		"KECCAK_ACCEL=false real (s)", "KECCAK_ACCEL=true real (s)", "speedup",
	)
	fmt.Println("| ------------------------------- | ----------------------------- | ---------------------------- | ------- |")

	for index := 0; index < cfg.intervals; index++ {
		selector := batchedRange(cfg.start, cfg.size, index)
		falseTime := runTimedBatched(cfg.makefileDir, cfg.timeBin, false, selector)
		trueTime := runTimedBatched(cfg.makefileDir, cfg.timeBin, true, selector)
		printBatchedRow(selector, falseTime, trueTime)
	}
}

func parseArgs() config {
	// Parse flags
	cfg := config{
		intervals:   100,
		size:        10,
		start:       0,
		makefileDir: defaultMakefileDir(),
		timeBin:     "/usr/bin/time",
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Compare keccak-zig-exec timings with KECCAK_ACCEL=false and true.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Modes:\n")
		fmt.Fprintf(flag.CommandLine.Output(), "  default: benchmark batched ranges of %d-byte inputs from common_inputs/keccak.all\n", batchedInputBytes)
		fmt.Fprintf(flag.CommandLine.Output(), "  --single-input-lengths: benchmark generated all-zero inputs at the requested byte lengths\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.IntVar(&cfg.intervals, "intervals", cfg.intervals, "number of intervals to run")
	flag.IntVar(&cfg.size, "size", cfg.size, "inputs per batched interval")
	flag.IntVar(&cfg.start, "start", cfg.start, "first input index")
	flag.StringVar(&cfg.makefileDir, "makefile-dir", cfg.makefileDir, "directory containing the Makefile")
	flag.StringVar(&cfg.timeBin, "time-bin", cfg.timeBin, "path to time executable")
	flag.StringVar(&cfg.singleInputLengths, "single-input-lengths", cfg.singleInputLengths, "comma-separated byte lengths for generated all-zero input mode")
	flag.Parse()

	abs, err := filepath.Abs(cfg.makefileDir)
	if err != nil {
		fatal("resolving --makefile-dir: %v", err)
	}
	cfg.makefileDir = abs
	return cfg
}

func defaultMakefileDir() string {
	// Locate the test Makefile
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func batchedRange(start, size, index int) string {
	// Build an inclusive batched range
	first := start + index*size
	last := first + size - 1
	return fmt.Sprintf("%d..%d", first, last)
}

func batchedCommand(timeBin string, accel bool, selector string) []string {
	// Build the batched command
	accelValue := "false"
	if accel {
		accelValue = "true"
	}
	return []string{
		timeBin,
		"-p",
		"make",
		"keccak-zig-exec",
		"KECCAK_ACCEL=" + accelValue,
		"KECCAK_N_VECTORS=" + selector,
	}
}

func runTimedBatched(makefileDir, timeBin string, accel bool, selector string) float64 {
	// Time batched execution
	return runCommand(makefileDir, batchedCommand(timeBin, accel, selector))
}

func singleCommand(timeBin string, accel bool, vectorFile, jsonDir string) []string {
	// Build the single-input command
	accelValue := "false"
	if accel {
		accelValue = "true"
	}
	return []string{
		timeBin,
		"-p",
		"make",
		"vector-build",
		"vector-json",
		"vector-exec",
		"TEST=keccak/keccak_with_provider_single_input.zig",
		"VECTOR_FILE=" + vectorFile,
		"VECTOR_N_VECTORS=1",
		"VECTOR_JSON_MODE=per-vector",
		"VECTOR_JSON_DIR=" + jsonDir,
		"KECCAK_ACCEL=" + accelValue,
	}
}

func runTimedSingle(makefileDir, timeBin string, accel bool, inputBytes int) float64 {
	// Time one generated input
	vectorFile, cleanupVector := writeSingleInputVector(inputBytes)
	defer cleanupVector()

	jsonDir, err := os.MkdirTemp("", "keccak-single-json-*")
	if err != nil {
		fatal("creating temporary JSON directory: %v", err)
	}
	defer os.RemoveAll(jsonDir)

	return runCommand(makefileDir, singleCommand(timeBin, accel, vectorFile, jsonDir))
}

func runCommand(makefileDir string, args []string) float64 {
	// Run and parse timing output
	cmd := exec.Command(args[0], args[1:]...)
	cmd.Dir = makefileDir
	cmd.Env = append(os.Environ(), "GO111MODULE=on")

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "command failed: %s\n", strings.Join(args, " "))
		fmt.Fprint(os.Stderr, stdout.String())
		fmt.Fprint(os.Stderr, stderr.String())
		if exitErr, ok := err.(*exec.ExitError); ok {
			os.Exit(exitErr.ExitCode())
		}
		os.Exit(1)
	}

	realTime, ok := parseRealTime(stderr.String())
	if !ok {
		fmt.Fprintf(os.Stderr, "could not parse /usr/bin/time output for: %s\n", strings.Join(args, " "))
		fmt.Fprint(os.Stderr, stderr.String())
		os.Exit(1)
	}
	return realTime
}

func parseInputLengthList(arg string) []int {
	// Parse comma-separated lengths
	var lengths []int
	for _, part := range strings.Split(arg, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			fatal("empty input length in %q", arg)
		}
		length, err := strconv.Atoi(part)
		if err != nil {
			fatal("parsing input length %q: %v", part, err)
		}
		if length < 0 {
			fatal("input length must be >= 0, got %d", length)
		}
		lengths = append(lengths, length)
	}
	return lengths
}

func writeSingleInputVector(inputBytes int) (string, func()) {
	// Create a temporary zero-input vector
	file, err := os.CreateTemp("", "keccak-single-*.all")
	if err != nil {
		fatal("creating temporary vector file: %v", err)
	}

	// Hex inputs are reversed by elf_to_json_gen, so write payload first and length last
	line := fmt.Sprintf("0x%s%016x\n", strings.Repeat("00", inputBytes), inputBytes)
	if _, err := file.WriteString(line); err != nil {
		_ = file.Close()
		_ = os.Remove(file.Name())
		fatal("writing temporary vector file: %v", err)
	}
	if err := file.Close(); err != nil {
		_ = os.Remove(file.Name())
		fatal("closing temporary vector file: %v", err)
	}

	return file.Name(), func() {
		_ = os.Remove(file.Name())
	}
}

func parseRealTime(timeOutput string) (float64, bool) {
	// Extract the real time
	timings := make(map[string]float64)
	for _, line := range strings.Split(timeOutput, "\n") {
		match := timeRE.FindStringSubmatch(strings.TrimSpace(line))
		if match == nil {
			continue
		}
		value, err := strconv.ParseFloat(strings.ReplaceAll(match[2], ",", "."), 64)
		if err != nil {
			return 0, false
		}
		timings[match[1]] = value
	}

	realTime, ok := timings["real"]
	return realTime, ok
}

func printBatchedRow(selector string, falseTime, trueTime float64) {
	// Print one batched row
	speedup := falseTime / trueTime
	fmt.Printf(
		"| %-31s | %29.2f | %28.2f | %7s |\n",
		selector, falseTime, trueTime, fmt.Sprintf("%.2fx", speedup),
	)
}

func printSingleRow(inputBytes int, falseTime, trueTime float64) {
	// Print one single-input row
	speedup := falseTime / trueTime
	fmt.Printf(
		"| %11d | %29.2f | %28.2f | %7s |\n",
		inputBytes, falseTime, trueTime, fmt.Sprintf("%.2fx", speedup),
	)
}

func fatal(format string, args ...any) {
	// Print an error and exit
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
