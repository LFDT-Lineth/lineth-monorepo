package main

import (
	"bufio"
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
	arithmetizationDir = "../../../arithmetization"
	zkcMain            = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin              = "zig-out/bin/bench-pcs"
	r5JSON             = "zig-out/bin/bench-pcs.json"
)

var (
	markRE  = regexp.MustCompile(`VERIFIER-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE = regexp.MustCompile(`clock cycle: ([0-9]+)`)
)

type marker struct{ cycle, value uint64 }

func main() {
	zkcBin := "zkc"
	if len(os.Args) > 1 {
		zkcBin = os.Args[1]
	}

	buildArgs := []string{"build", "--release=small"}
	if os.Getenv("DISABLE_ACCELERATORS") == "true" {
		buildArgs = append(buildArgs, "-Ddisable-accelerators=true")
	}
	if err := run("zig", buildArgs...); err != nil {
		fatal(err)
	}
	if err := os.MkdirAll("zig-out/bin", 0o755); err != nil {
		fatal(err)
	}
	out, err := os.Create(r5JSON)
	if err != nil {
		fatal(err)
	}
	r5BinAbsolute, err := filepath.Abs(r5Bin)
	if err != nil {
		fatal(err)
	}
	convert := exec.Command("go", "-C", arithmetizationDir, "tool", "elf_to_json", r5BinAbsolute, "0x00", "0x08800000")
	convert.Stdout = out
	convert.Stderr = os.Stderr
	if err := convert.Run(); err != nil {
		_ = out.Close()
		fatal(err)
	}
	if err := out.Close(); err != nil {
		fatal(err)
	}

	// -vvv is what surfaces the guest's VERIFIER-MARK writes and the per-cycle
	// counter the marks are read against; at lower verbosities zkc prints
	// neither.
	cmd := exec.Command(zkcBin, "exec", "--fast", "-vvv", r5JSON, zkcMain)
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		fatal(err)
	}
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		fatal(err)
	}
	markers, tail, scanErr := parse(stdout)
	waitErr := cmd.Wait()
	if scanErr != nil {
		fatal(scanErr)
	}
	if waitErr != nil {
		fmt.Fprintf(os.Stderr, "zkc exec failed: %v\nlast output:\n%s\n", waitErr, strings.Join(tail, "\n"))
		os.Exit(1)
	}
	start, startOK := markers[10]
	end, endOK := markers[11]
	if !startOK || !endOK || end.cycle < start.cycle {
		fatal(fmt.Errorf("PCS markers missing or out of order; last output:\n%s", strings.Join(tail, "\n")))
	}
	fmt.Printf("pcs.verify RISC-V cycles: %d\n", end.cycle-start.cycle)
}

func parse(r io.Reader) (map[uint64]marker, []string, error) {
	markers := make(map[uint64]marker)
	var total uint64
	var tail []string
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		tail = appendTail(tail, line)
		if match := cycleRE.FindStringSubmatch(line); match != nil {
			total, _ = strconv.ParseUint(match[1], 10, 64)
		}
		if match := markRE.FindStringSubmatch(line); match != nil {
			phase, _ := strconv.ParseUint(match[1], 10, 64)
			value, _ := strconv.ParseUint(match[2], 10, 64)
			markers[phase] = marker{cycle: total, value: value}
		}
	}
	return markers, tail, scanner.Err()
}

func appendTail(tail []string, line string) []string {
	if len(tail) >= 40 {
		copy(tail, tail[1:])
		return append(tail[:39], line)
	}
	return append(tail, line)
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
