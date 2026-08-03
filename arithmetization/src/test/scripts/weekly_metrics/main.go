package main

// weekly_metrics turns one run directory into one new row in each markdown table of the
// weekly metrics file. The markdown file IS the ledger — there is no second store.
//
// Rows are inserted directly under each table's header separator, so the newest run is
// always at the top. Tables are located by their `<!-- table:<name> -->` marker, which
// means the surrounding prose can be edited freely without breaking the tool.
//
// Examples, from the repository root:
//   GO111MODULE=off go run ./arithmetization/src/test/scripts/weekly_metrics \
//       -run-dir /tmp/wzm/run -out arithmetization/docs/metrics/zkc-weekly-metrics.md
//
// Input layout, written by the caller (workflow steps or the local script):
//   <run-dir>/meta.env          KEY=VALUE lines: week, started, trigger, runner, versions
//   <run-dir>/<step>/rc         exit code
//   <run-dir>/<step>/wall_s     integer seconds
//   <run-dir>/<step>/rusage     `/usr/bin/time -l` (BSD) or `-v` (GNU) output, optional
//   <run-dir>/<step>/stdout     captured stdout
//   <run-dir>/<step>/timedout   present => the step was killed by its time-box
// with <step> in: compile, guest, trace, check.

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func main() {
	runDir := flag.String("run-dir", "", "run directory holding meta.env and the per-step artifacts (required)")
	out := flag.String("out", "", "markdown file to update; created from a template if absent (required)")
	topN := flag.Int("top", 5, "how many of the most expensive modules to list")
	flag.Parse()
	if *runDir == "" || *out == "" {
		fmt.Fprintln(os.Stderr, "weekly_metrics: -run-dir and -out are required")
		os.Exit(2)
	}

	meta := readMeta(filepath.Join(*runDir, "meta.env"))
	steps := map[string]step{}
	for _, name := range []string{"compile", "guest", "trace", "check"} {
		steps[name] = readStep(*runDir, name)
	}

	cs, csErr := parseCompileStats(steps["compile"].stdout)
	if csErr != nil {
		fmt.Fprintln(os.Stderr, "weekly_metrics: warning: compile stats:", csErr)
	}
	// The trace stats table is printed before constraint checking starts, so the check
	// step's stdout also carries it — use whichever run got that far.
	ts := parseTraceStats(steps["trace"].stdout)
	if ts.cellsRaw == 0 {
		ts = parseTraceStats(steps["check"].stdout)
	}

	rows := buildRows(meta, steps, cs, ts, *topN)
	if err := insertRows(*out, rows); err != nil {
		fmt.Fprintln(os.Stderr, "weekly_metrics:", err)
		os.Exit(1)
	}

	// Echo what was added, so a workflow step summary or a local run shows the numbers
	// without opening the file.
	for _, t := range tableOrder {
		fmt.Printf("%-12s %s\n", t, rows[t])
	}
}

// ── the tables ──────────────────────────────────────────────────────────────────

var tableOrder = []string{"runs", "cost", "stats", "modules", "tracemodules"}

// tableHeaders defines each table's columns. Changing a header here does NOT rewrite
// existing rows, so add columns at the END and accept that older rows render short.
var tableHeaders = map[string][]string{
	"runs":         {"week", "started (UTC)", "trigger", "runner", "arithmetization", "zkc", "run"},
	"cost":         {"week", "compile", "guest build", "trace", "trace + check", "trace cells"},
	"stats":        {"week", "max degree", "constraints", "d1", "d2", "d3", "d4", "d5", "d6", "d7", "d8+", "lookups", "complexity", "static cells"},
	"modules":      {"week", "1", "2", "3", "4", "5"},
	"tracemodules": {"week", "1", "2", "3", "4", "5"},
}

var tableTitles = map[string]string{
	"runs":         "Runs and component versions",
	"cost":         "Cost per task — outcome, wall clock, peak RSS",
	"stats":        "Constraint system (`zkc compile --stats`, `Total | function` row)",
	"modules":      "Most expensive modules by constraint complexity (Σ d²)",
	"tracemodules": "Most expensive modules by trace cells",
}

func buildRows(meta map[string]string, steps map[string]step, cs *compileStats, ts traceStats, topN int) map[string]string {
	week := get(meta, "week")
	rows := map[string]string{}

	rows["runs"] = mdRow(week,
		get(meta, "started"),
		get(meta, "trigger"),
		get(meta, "runner"),
		get(meta, "arithmetization"),
		get(meta, "zkc"),
		get(meta, "run_url"))

	rows["cost"] = mdRow(week,
		steps["compile"].cell(),
		steps["guest"].cell(),
		steps["trace"].cell(),
		steps["check"].cell(),
		grouped(int64(ts.cellsRaw)))

	if cs != nil {
		cells := []string{week, strconv.Itoa(cs.maxDegree), grouped(int64(cs.constraints))}
		for i := 0; i < 8; i++ {
			if i < len(cs.byDegree) {
				cells = append(cells, grouped(int64(cs.byDegree[i])))
			} else {
				cells = append(cells, "–")
			}
		}
		cells = append(cells, grouped(int64(cs.lookups)), grouped(int64(cs.complexity)), grouped(int64(cs.staticCells)))
		rows["stats"] = mdRow(cells...)
		rows["modules"] = mdRow(append([]string{week}, topCells(cs.modules, topN)...)...)
	} else {
		rows["stats"] = mdRow(append([]string{week}, repeat("–", len(tableHeaders["stats"])-1)...)...)
		rows["modules"] = mdRow(append([]string{week}, repeat("–", topN)...)...)
	}
	rows["tracemodules"] = mdRow(append([]string{week}, topCells(ts.modules, topN)...)...)
	return rows
}

// topCells renders the N heaviest modules as "name (value)" cells, padding with "–".
func topCells(mods []module, n int) []string {
	sort.SliceStable(mods, func(i, j int) bool { return mods[i].weight > mods[j].weight })
	out := make([]string, 0, n)
	for i := 0; i < n; i++ {
		if i < len(mods) && mods[i].weight > 0 {
			out = append(out, fmt.Sprintf("`%s` %s", mods[i].name, grouped(mods[i].weight)))
		} else {
			out = append(out, "–")
		}
	}
	return out
}

// ── markdown insertion ──────────────────────────────────────────────────────────

// insertRows adds one row to every table, immediately after that table's header
// separator, so the newest run appears first.
func insertRows(path string, rows map[string]string) error {
	body, err := os.ReadFile(path)
	if err != nil {
		if !os.IsNotExist(err) {
			return err
		}
		body = []byte(template())
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
	}

	lines := strings.Split(string(body), "\n")
	for _, name := range tableOrder {
		row, ok := rows[name]
		if !ok {
			continue
		}
		idx, err := separatorIndex(lines, name)
		if err != nil {
			return err
		}
		lines = append(lines[:idx+1], append([]string{row}, lines[idx+1:]...)...)
	}
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0o644)
}

var sepLine = regexp.MustCompile(`^\s*\|[\s\-:|]+\|\s*$`)

// separatorIndex finds the `|---|---|` line belonging to the table tagged with
// `<!-- table:<name> -->`. Anchoring on the marker rather than on a heading means the
// prose around the tables can be reworded without breaking insertion.
func separatorIndex(lines []string, name string) (int, error) {
	marker := "<!-- table:" + name + " -->"
	for i, l := range lines {
		if !strings.Contains(l, marker) {
			continue
		}
		for j := i + 1; j < len(lines) && j < i+8; j++ {
			if sepLine.MatchString(lines[j]) {
				return j, nil
			}
		}
		return 0, fmt.Errorf("table %q: found %s but no |---| separator within 8 lines", name, marker)
	}
	return 0, fmt.Errorf("table %q: marker %s not found (add it above the table header)", name, marker)
}

func template() string {
	var b strings.Builder
	b.WriteString("# RISC-V arithmetization weekly metrics\n\n" +
		"Generated by `.github/workflows/arithmetization-weekly-zkc-metrics.yml`, which runs\n" +
		"`arithmetization/src/test/scripts/weekly_metrics`. **Newest run first.** Rows are appended\n" +
		"under each table's `<!-- table:… -->` marker; the prose around them is free to edit.\n\n" +
		"The measured workload is the l2-execution guest built with `KECCAK_ACCEL=true`, run against\n" +
		"`riscv-guests/l2-execution/test/testdata/stateless_input.ssz` — the small committed reference\n" +
		"block. Each cell in the cost table reads `outcome wall / peak RSS`.\n")
	for _, name := range tableOrder {
		hdr := tableHeaders[name]
		fmt.Fprintf(&b, "\n## %s\n\n<!-- table:%s -->\n%s\n%s\n",
			tableTitles[name], name, mdRow(hdr...), separatorFor(len(hdr)))
	}
	return b.String()
}

func separatorFor(n int) string {
	return "|" + strings.Repeat("---|", n)
}

func mdRow(cells ...string) string {
	for i, c := range cells {
		if strings.TrimSpace(c) == "" {
			c = "–"
		}
		// A literal pipe would end the cell.
		cells[i] = strings.ReplaceAll(c, "|", `\|`)
	}
	return "| " + strings.Join(cells, " | ") + " |"
}

// ── per-step artifacts ──────────────────────────────────────────────────────────

type step struct {
	present  bool
	rc       int
	haveRC   bool
	timedout bool
	wallS    int
	haveWall bool
	rssBytes int64
	stdout   string
	failures int
}

func readStep(runDir, name string) step {
	dir := filepath.Join(runDir, name)
	s := step{}
	if fi, err := os.Stat(dir); err != nil || !fi.IsDir() {
		return s
	}
	s.present = true
	s.stdout = stripANSI(readText(filepath.Join(dir, "stdout")))
	if v := strings.TrimSpace(readText(filepath.Join(dir, "rc"))); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.rc, s.haveRC = n, true
		}
	}
	if v := strings.TrimSpace(readText(filepath.Join(dir, "wall_s"))); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			s.wallS, s.haveWall = n, true
		}
	}
	s.timedout = exists(filepath.Join(dir, "timedout"))
	s.rssBytes = parsePeakRSS(readText(filepath.Join(dir, "rusage")))
	// A constraint failure does NOT change zkc's exit code: checkConstraints reports the
	// failures and returns, leaving rc at 0. The reported lines are the only signal.
	s.failures = len(failingLine.FindAllString(s.stdout, -1)) +
		len(failingLine.FindAllString(stripANSI(readText(filepath.Join(dir, "stderr"))), -1))
	return s
}

var failingLine = regexp.MustCompile(`(?m)^\s*failing\s+\S`)

// cell renders "outcome wall / peak RSS" for the cost table.
func (s step) cell() string {
	if !s.present {
		return "skipped"
	}
	var outcome string
	switch {
	case s.timedout:
		outcome = "**TIMEOUT**"
	case s.failures > 0:
		outcome = fmt.Sprintf("**FAIL** (%d failing)", s.failures)
	case s.haveRC && s.rc == 0:
		outcome = "ok"
	// 128+signal. SIGKILL on a memory-hungry step is almost always the kernel's OOM
	// killer, and it is worth distinguishing from an ordinary non-zero exit: the peak RSS
	// in the same cell then tells you what it died wanting.
	case s.haveRC && s.rc == 137:
		outcome = "**OOM/killed** (SIGKILL)"
	case s.haveRC && s.rc == 139:
		outcome = "**CRASH** (SIGSEGV)"
	case s.haveRC:
		outcome = fmt.Sprintf("**FAIL** (rc %d)", s.rc)
	default:
		outcome = "**FAIL** (no exit code recorded)"
	}
	parts := outcome
	if s.haveWall {
		parts += " " + fmtDuration(s.wallS)
	}
	if s.rssBytes > 0 {
		parts += " / " + fmtBytes(s.rssBytes)
	}
	return parts
}

var (
	// macOS `time -l`: "          8264237056  maximum resident set size" (bytes).
	// Deliberately NOT "peak memory footprint": despite the name that is a cumulative
	// allocation figure — a run whose true peak was 12 GiB reported 154 GiB.
	bsdMaxRSS = regexp.MustCompile(`(?m)^\s*(\d+)\s+maximum resident set size`)
	// GNU `time -v`: "\tMaximum resident set size (kbytes): 4677872"
	gnuMaxRSSkb = regexp.MustCompile(`(?m)^\s*Maximum resident set size \(kbytes\):\s*(\d+)`)
)

func parsePeakRSS(text string) int64 {
	if m := bsdMaxRSS.FindStringSubmatch(text); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		return n
	}
	if m := gnuMaxRSSkb.FindStringSubmatch(text); m != nil {
		n, _ := strconv.ParseInt(m[1], 10, 64)
		return n * 1024
	}
	return 0
}

// ── `zkc compile --stats` parsing ───────────────────────────────────────────────

type module struct {
	name   string
	weight int64
}

type compileStats struct {
	maxDegree   int
	byDegree    []int
	constraints int
	lookups     int
	complexity  int
	staticCells int
	modules     []module // by complexity
}

// degreeLabel matches "d1"…"d7", the open final bucket "d8+", and the closed final
// bucket "d8-d8" / "d8-d11".
var degreeLabel = regexp.MustCompile(`^d(\d+)(?:\+|-d(\d+))?$`)

// parseCompileStats reads the wide `--stats` table. Column indices come from the
// sub-header row starting "Module | type" — never hard-coded, and never from the row
// above it, which uses spanning cells and so has fewer fields than the data rows.
func parseCompileStats(text string) (*compileStats, error) {
	if strings.TrimSpace(text) == "" {
		return nil, fmt.Errorf("no stats output captured")
	}
	var (
		degCols, degMax []int
		degLabels       []string
		colCx, colLook  = -1, -1
		colCells        = -1
		haveHdr, sawFn  bool
		st              = &compileStats{}
	)
	for _, line := range strings.Split(text, "\n") {
		if !strings.Contains(line, "|") {
			continue
		}
		f := strings.Split(line, "|")
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		if !haveHdr {
			if len(f) > 2 && f[0] == "Module" && f[1] == "type" {
				for i, label := range f {
					if m := degreeLabel.FindStringSubmatch(label); m != nil {
						lo, _ := strconv.Atoi(m[1])
						hi := lo
						if m[2] != "" {
							hi, _ = strconv.Atoi(m[2])
						}
						degCols, degMax, degLabels = append(degCols, i), append(degMax, hi), append(degLabels, label)
					} else if label == "(= sum d^2)" {
						colCx = i
					} else if label == "(static tables)" {
						colCells = i
					}
				}
				// The lookups column's sub-header cell is blank; it always sits
				// immediately before the complexity column.
				colLook = colCx - 1
				haveHdr = true
			}
			continue
		}
		if len(f) < 3 {
			continue
		}
		if f[0] == "Total" {
			switch f[1] {
			case "function":
				sawFn = true
				st.lookups, st.complexity = cell(f, colLook), cell(f, colCx)
				for _, c := range degCols {
					st.byDegree = append(st.byDegree, cell(f, c))
				}
			case "static":
				st.staticCells = cell(f, colCells)
			}
			continue
		}
		// Per-module rows: only function modules carry a complexity figure.
		if f[1] == "function" && f[0] != "" {
			if w := cell(f, colCx); w > 0 {
				st.modules = append(st.modules, module{name: f[0], weight: int64(w)})
			}
		}
	}
	if !haveHdr {
		return nil, fmt.Errorf("no 'Module | type' header row")
	}
	if len(degCols) == 0 || colCx < 0 || colCells < 0 || colLook <= degCols[len(degCols)-1] {
		return nil, fmt.Errorf("header found but columns did not map (degrees=%d complexity=%d cells=%d lookups=%d)",
			len(degCols), colCx, colCells, colLook)
	}
	if !sawFn {
		return nil, fmt.Errorf("no 'Total | function' row")
	}

	for _, c := range st.byDegree {
		st.constraints += c
	}
	// zkc closes the final bucket to "d<lo>-d<max>" as soon as any module reaches degree
	// lo; while it still reads "d<lo>+", nothing got there and the true maximum is the
	// last non-empty single-degree bucket.
	if last := degLabels[len(degLabels)-1]; strings.Contains(last, "-d") {
		st.maxDegree = degMax[len(degMax)-1]
	} else {
		for i := len(st.byDegree) - 1; i >= 0; i-- {
			if st.byDegree[i] > 0 {
				st.maxDegree = degMax[i]
				break
			}
		}
	}
	// Σ cₖ·k² must equal the reported complexity while every bucket spans one degree.
	// It is a free check that the columns were mapped correctly.
	if sum, ok := sumOfSquares(st.byDegree, degLabels, degMax); ok && sum != st.complexity {
		return st, fmt.Errorf("column mapping suspect: Σ cₖ·k² = %d but the table reports complexity %d", sum, st.complexity)
	}
	return st, nil
}

func sumOfSquares(counts []int, labels []string, degMax []int) (int, bool) {
	sum := 0
	for i, label := range labels {
		lo, _ := strconv.Atoi(degreeLabel.FindStringSubmatch(label)[1])
		if degMax[i] != lo || strings.HasSuffix(label, "+") {
			if counts[i] != 0 {
				return 0, false // a multi-degree bucket makes the sum incomparable
			}
			continue
		}
		sum += counts[i] * lo * lo
	}
	return sum, true
}

// ── `zkc trace --stats` parsing ─────────────────────────────────────────────────

type traceStats struct {
	cellsRaw int
	modules  []module // by cells
}

// "Cells (raw) | 7908271799 |" — the plain-integer field. The neighbouring "Cells" row
// is human-formatted ("7.9G") and deliberately ignored.
var traceCellsRaw = regexp.MustCompile(`(?m)^\s*Cells \(raw\)\s*\|\s*(\d+)`)

func parseTraceStats(text string) traceStats {
	ts := traceStats{}
	if strings.TrimSpace(text) == "" {
		return ts
	}
	if m := traceCellsRaw.FindStringSubmatch(text); m != nil {
		ts.cellsRaw, _ = strconv.Atoi(m[1])
	}
	// Per-module table: "<name> | columns | lines | bitwidth | cells | nonzero | bytes |"
	inTable, cellsCol := false, -1
	for _, line := range strings.Split(text, "\n") {
		f := strings.Split(line, "|")
		for i := range f {
			f[i] = strings.TrimSpace(f[i])
		}
		if !inTable {
			for i, label := range f {
				if label == "cells" {
					cellsCol, inTable = i, true
				}
			}
			continue
		}
		if len(f) <= cellsCol || f[0] == "" || strings.HasPrefix(f[0], "-") {
			continue
		}
		if w := cell(f, cellsCol); w > 0 {
			ts.modules = append(ts.modules, module{name: f[0], weight: int64(w)})
		}
	}
	return ts
}

// ── small helpers ───────────────────────────────────────────────────────────────

func readMeta(path string) map[string]string {
	out := map[string]string{}
	f, err := os.Open(path)
	if err != nil {
		return out
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if i := strings.Index(line, "="); i > 0 {
			out[strings.TrimSpace(line[:i])] = strings.TrimSpace(line[i+1:])
		}
	}
	return out
}

func get(m map[string]string, k string) string {
	if v, ok := m[k]; ok && v != "" {
		return v
	}
	return "–"
}

// cell reads an integer cell; blank means zero, because the renderer elides zeros.
func cell(fields []string, idx int) int {
	if idx < 0 || idx >= len(fields) {
		return 0
	}
	var digits strings.Builder
	for _, r := range fields[idx] {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	if digits.Len() == 0 {
		return 0
	}
	n, _ := strconv.Atoi(digits.String())
	return n
}

func fmtDuration(s int) string {
	switch {
	case s < 60:
		return fmt.Sprintf("%ds", s)
	case s < 3600:
		return fmt.Sprintf("%dm%02ds", s/60, s%60)
	default:
		return fmt.Sprintf("%dh%02dm", s/3600, (s%3600)/60)
	}
}

func fmtBytes(n int64) string {
	f, units, i := float64(n), []string{"B", "KiB", "MiB", "GiB", "TiB"}, 0
	for f >= 1024 && i < len(units)-1 {
		f /= 1024
		i++
	}
	if i == 0 {
		return fmt.Sprintf("%.0f B", f)
	}
	return fmt.Sprintf("%.2f %s", f, units[i])
}

// grouped renders 12627 as "12 627" — readable in a table without commas.
func grouped(n int64) string {
	if n == 0 {
		return "–"
	}
	s := strconv.FormatInt(n, 10)
	var parts []string
	for len(s) > 3 {
		parts = append([]string{s[len(s)-3:]}, parts...)
		s = s[:len(s)-3]
	}
	return strings.Join(append([]string{s}, parts...), " ")
}

func repeat(s string, n int) []string {
	out := make([]string, n)
	for i := range out {
		out[i] = s
	}
	return out
}

var ansiRE = regexp.MustCompile(`\x1b\[[0-9;]*[A-Za-z]`)

func stripANSI(s string) string { return ansiRE.ReplaceAllString(s, "") }

func readText(path string) string {
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return string(b)
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}
