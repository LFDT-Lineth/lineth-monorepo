package main

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

var timeRE = regexp.MustCompile(`^(real|user|sys)\s+([0-9]+(?:[\.,][0-9]+)?)$`)

type config struct {
	intervals   int
	size        int
	start       int
	makefileDir string
	timeBin     string
}

func main() {
	cfg := parseArgs()
	if cfg.intervals < 1 {
		fatal("--intervals must be >= 1")
	}
	if cfg.size < 1 {
		fatal("--size must be >= 1")
	}

	fmt.Println("| vectors | KECCAK_ACCEL=false real (s) | KECCAK_ACCEL=true real (s) | speedup |")
	fmt.Println("|---------|-----------------------------|----------------------------|---------|")

	for index := 0; index < cfg.intervals; index++ {
		selector := vectorRange(cfg.start, cfg.size, index)
		falseTime := runTimed(cfg.makefileDir, cfg.timeBin, false, selector)
		trueTime := runTimed(cfg.makefileDir, cfg.timeBin, true, selector)
		printRow(selector, falseTime, trueTime)
	}
}

func parseArgs() config {
	cfg := config{
		intervals:   100,
		size:        10,
		start:       0,
		makefileDir: defaultMakefileDir(),
		timeBin:     "/usr/bin/time",
	}

	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Compare keccak-zig-exec timings with KECCAK_ACCEL=false and true.\n\n")
		fmt.Fprintf(flag.CommandLine.Output(), "Usage of %s:\n", os.Args[0])
		flag.PrintDefaults()
	}
	flag.IntVar(&cfg.intervals, "intervals", cfg.intervals, "number of intervals to run")
	flag.IntVar(&cfg.size, "size", cfg.size, "vectors per interval")
	flag.IntVar(&cfg.start, "start", cfg.start, "first vector index")
	flag.StringVar(&cfg.makefileDir, "makefile-dir", cfg.makefileDir, "directory containing the Makefile")
	flag.StringVar(&cfg.timeBin, "time-bin", cfg.timeBin, "path to time executable")
	flag.Parse()

	abs, err := filepath.Abs(cfg.makefileDir)
	if err != nil {
		fatal("resolving --makefile-dir: %v", err)
	}
	cfg.makefileDir = abs
	return cfg
}

func defaultMakefileDir() string {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return "."
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(file)))
}

func vectorRange(start, size, index int) string {
	first := start + index*size
	last := first + size - 1
	return fmt.Sprintf("%d..%d", first, last)
}

func command(timeBin string, accel bool, selector string) []string {
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

func runTimed(makefileDir, timeBin string, accel bool, selector string) float64 {
	args := command(timeBin, accel, selector)
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

func parseRealTime(timeOutput string) (float64, bool) {
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

func printRow(selector string, falseTime, trueTime float64) {
	speedup := falseTime / trueTime
	fmt.Printf("| %s | %.2f | %.2f | %.2fx |\n", selector, falseTime, trueTime, speedup)
}

func fatal(format string, args ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}
