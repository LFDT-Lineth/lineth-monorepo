// Parses STEP_COUNT lines emitted by printsteps in ZKC programs and aggregates
// decode-region micro-step counts.
//
// Label convention (see arithmetization/src/main/riscv/):
//
//	printsteps "<Prefix> before decode"
//	... decode work ...
//	printsteps "<Prefix> after decode"
//
// Each matched before/after pair contributes (after - before) VM micro-steps to
// that prefix. Prefixes are free-form ("Interpreter", "I-type", "I-type OP-IMM",
// etc.) so nested detail is preserved.
//
// Usage:
//
//	zkc exec -v -q input.json program.bin 2>&1 | go run ./parse_decode_steps/main.go
//	go run ./parse_decode_steps -log bench.log
//
// Output: Markdown table on stdout (suitable for CI summaries).
package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

const (
	suffixBefore = " before decode"
	suffixAfter  = " after decode"
)

var (
	reStepCount  = regexp.MustCompile(`^STEP_COUNT (.+) \((\d+) steps\)$`)
	reMachineExec = regexp.MustCompile(`Machine execution \((\d+) steps\)`)
)

// Top-level decode prefixes counted toward the non-overlapping decode total.
// Nested sub-regions (e.g. "I-type OP-IMM") are listed in the detail table
// but excluded here to avoid double-counting.
var topLevelDecodePrefixes = map[string]bool{
	"Interpreter": true,
	"R-type":      true,
	"I-type":      true,
	"S-type":      true,
	"B-type":      true,
	"U-type":      true,
	"J-type":      true,
}

type aggregate struct {
	invocations uint64
	totalSteps  uint64
}

type pending struct {
	steps uint64
	line  int
}

type result struct {
	aggregates   map[string]*aggregate
	machineSteps uint64
	warnings     []string
}

func parseLabel(label string) (prefix, phase string, ok bool) {
	switch {
	case strings.HasSuffix(label, suffixBefore):
		return strings.TrimSuffix(label, suffixBefore), "before", true
	case strings.HasSuffix(label, suffixAfter):
		return strings.TrimSuffix(label, suffixAfter), "after", true
	default:
		return "", "", false
	}
}

func consume(reader io.Reader) (result, error) {
	var (
		open       = make(map[string]pending)
		aggregates = make(map[string]*aggregate)
		warnings   []string
		scanner    = bufio.NewScanner(reader)
		lineNo     int
		out        result
	)
	//
	for scanner.Scan() {
		lineNo++
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		//
		if mm := reMachineExec.FindStringSubmatch(line); mm != nil {
			steps, err := strconv.ParseUint(mm[1], 10, 64)
			if err != nil {
				return out, fmt.Errorf("line %d: parse machine steps: %w", lineNo, err)
			}
			out.machineSteps = steps
			continue
		}
		//
		mm := reStepCount.FindStringSubmatch(line)
		if mm == nil {
			continue
		}
		//
		prefix, phase, ok := parseLabel(mm[1])
		if !ok {
			warnings = append(warnings, fmt.Sprintf("line %d: unrecognised STEP_COUNT label %q", lineNo, mm[1]))
			continue
		}
		//
		steps, err := strconv.ParseUint(mm[2], 10, 64)
		if err != nil {
			return out, fmt.Errorf("line %d: parse steps: %w", lineNo, err)
		}
		//
		switch phase {
		case "before":
			if prev, exists := open[prefix]; exists {
				warnings = append(warnings, fmt.Sprintf(
					"line %d: duplicate %q before decode (previous at line %d)", lineNo, prefix, prev.line))
			}
			open[prefix] = pending{steps: steps, line: lineNo}
		case "after":
			prev, exists := open[prefix]
			if !exists {
				warnings = append(warnings, fmt.Sprintf("line %d: %q after decode without matching before", lineNo, prefix))
				continue
			}
			delete(open, prefix)
			if steps < prev.steps {
				warnings = append(warnings, fmt.Sprintf(
					"line %d: %q after (%d) < before (%d at line %d)", lineNo, prefix, steps, prev.steps, prev.line))
				continue
			}
			delta := steps - prev.steps
			agg := aggregates[prefix]
			if agg == nil {
				agg = &aggregate{}
				aggregates[prefix] = agg
			}
			agg.invocations++
			agg.totalSteps += delta
		}
	}
	//
	if err := scanner.Err(); err != nil {
		return out, err
	}
	for prefix, prev := range open {
		warnings = append(warnings, fmt.Sprintf("unclosed %q before decode (line %d)", prefix, prev.line))
	}
	//
	out.aggregates = aggregates
	out.warnings = warnings
	return out, nil
}

func topLevelDecodeTotal(aggregates map[string]*aggregate) uint64 {
	var total uint64
	for prefix, agg := range aggregates {
		if topLevelDecodePrefixes[prefix] {
			total += agg.totalSteps
		}
	}
	return total
}

func formatThousands(n uint64) string {
	s := strconv.FormatUint(n, 10)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	first := len(s) % 3
	if first == 0 {
		first = 3
	}
	b.WriteString(s[:first])
	for i := first; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

func pct(part, whole uint64) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

func render(res result) {
	aggregates := res.aggregates
	warnings := res.warnings

	prefixes := make([]string, 0, len(aggregates))
	for prefix := range aggregates {
		prefixes = append(prefixes, prefix)
	}
	sort.Strings(prefixes)

	fmt.Println("## Decode step profile")
	fmt.Println()
	fmt.Println("| prefix | invocations | total decode steps | avg steps / invocation |")
	fmt.Println("| --- | ---: | ---: | ---: |")

	var grandTotal uint64
	for _, prefix := range prefixes {
		agg := aggregates[prefix]
		grandTotal += agg.totalSteps
		avg := float64(agg.totalSteps) / float64(agg.invocations)
		fmt.Printf("| %s | %s | %s | %.2f |\n",
			prefix, formatThousands(agg.invocations), formatThousands(agg.totalSteps), avg)
	}
	fmt.Printf("| **TOTAL (all prefixes)** | | **%s** | |\n", formatThousands(grandTotal))
	fmt.Println()
	fmt.Println("> Detail TOTAL sums every prefix independently. Nested regions (e.g. `I-type OP-IMM` inside `I-type`) appear in both rows.")
	fmt.Println("> Run totals use top-level prefixes only (`Interpreter`, `*-type`) for decode vs execution split.")

	if len(warnings) > 0 {
		fmt.Println()
		fmt.Println("### Warnings")
		fmt.Println()
		for _, w := range warnings {
			fmt.Printf("- %s\n", w)
		}
	}
}

func main() {
	logPath := flag.String("log", "", "log file to parse (default: stdin)")
	flag.Parse()

	var (
		reader io.Reader = os.Stdin
		err    error
	)
	if *logPath != "" {
		var f *os.File
		f, err = os.Open(*logPath)
		if err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		defer f.Close()
		reader = f
	}

	res, err := consume(reader)
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
	if len(res.aggregates) == 0 {
		fmt.Fprintln(os.Stderr, "error: no STEP_COUNT before/after decode pairs found")
		os.Exit(1)
	}
	render(res)
}
