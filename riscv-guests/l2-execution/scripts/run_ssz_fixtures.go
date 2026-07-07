package main

// Examples, from the repository root:
// Run up to 100 SSZ inputs from each selected fixture path:
//   make -C riscv-guests/l2-execution run-execution-specs-ssz-fixtures
// Run all inputs in each selected fixture path:
//   make -C riscv-guests/l2-execution run-execution-specs-ssz-fixtures EXECUTION_SPECS_RUN_SSZ_LIMIT=0
// Run Lineth zkevm-fixtures release inputs:
//   make -C riscv-guests/l2-execution run-zkevm-fixtures-ssz-fixtures

import (
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

const (
	fixturePathColumnWidth = 40
	testColumnWidth        = 108
	formatExecutionSpecs   = "execution-specs"
	formatZkevmFixtures    = "zkevm-fixtures"
)

type fixtureCase struct {
	Blocks []fixtureBlock `json:"blocks"`
}

type fixtureBlock struct {
	StatelessInputBytes  string `json:"statelessInputBytes"`
	StatelessOutputBytes string `json:"statelessOutputBytes"`
}

type zkevmFixture struct {
	Name string `json:"name"`
}

type statelessInput struct {
	testName      string
	blockIndex    int
	input         []byte
	expectedValid bool
}

type selectedInput struct {
	fixturePath   string
	jsonFile      string
	testName      string
	blockIndex    int
	file          string
	size          int
	expectedValid bool
}

// Runs selected fixtures.
func main() {
	fixturesDir := flag.String("fixtures-dir", filepath.Join(os.TempDir(), "execution-specs-json-fixtures", "fixtures"), "directory containing zkevm fixture JSON files")
	sszDir := flag.String("ssz-dir", filepath.Join(os.TempDir(), "execution-specs-ssz-fixtures"), "directory for selected temporary raw SSZ inputs")
	fixturePathsFlag := flag.String("fixture-paths", "blockchain_tests/for_amsterdam/amsterdam,blockchain_tests/for_amsterdam/osaka", "comma-separated fixture paths under fixtures-dir; . means all")
	sszLimit := flag.Int("ssz-limit", 0, "maximum SSZ inputs to run per fixture path; 0 means all")
	fixtureFormat := flag.String("fixture-format", formatExecutionSpecs, "fixture JSON format: execution-specs or zkevm-fixtures")
	zesuConvert := flag.String("zesu-convert", "", "path to zesu-convert; required for zkevm-fixtures")
	zkcFlags := flag.String("zkc-flags", "--gogen --fast -q", "flags forwarded to zkc exec")
	flag.Parse()

	if *sszLimit < 0 {
		must(fmt.Errorf("ssz-limit must be non-negative"))
	}
	if *fixtureFormat != formatExecutionSpecs && *fixtureFormat != formatZkevmFixtures {
		must(fmt.Errorf("fixture-format must be %q or %q", formatExecutionSpecs, formatZkevmFixtures))
	}

	root, err := repoRoot()
	must(err)

	guestDir := filepath.Join(root, "riscv-guests", "l2-execution")
	fixturePaths := splitList(*fixturePathsFlag)
	if len(fixturePaths) == 0 {
		must(fmt.Errorf("fixture-paths must not be empty"))
	}

	must(run(os.Stderr, "make", "-C", guestDir, "compile"))

	var inputs []selectedInput
	hadError := false
	for _, fixturePath := range fixturePaths {
		fixturePath, targetDir, err := resolveFixturePath(*fixturesDir, fixturePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skip %s: %v\n", fixturePath, err)
			hadError = true
			continue
		}

		jsonPaths, err := jsonFiles(targetDir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "list JSON fixtures %s: %v\n", targetDir, err)
			hadError = true
			continue
		}
		if len(jsonPaths) == 0 {
			fmt.Fprintf(os.Stderr, "no JSON fixtures found in %s\n", targetDir)
			hadError = true
			continue
		}

		selected := 0
		for _, jsonPath := range jsonPaths {
			remaining := 0
			if *sszLimit > 0 {
				remaining = *sszLimit - selected
				if remaining <= 0 {
					break
				}
			}

			newInputs, err := writeSSZInputs(*sszDir, fixturePath, targetDir, jsonPath, remaining, *fixtureFormat, *zesuConvert)
			if err != nil {
				fmt.Fprintf(os.Stderr, "prepare %s: %v\n", jsonPath, err)
				hadError = true
				continue
			}
			inputs = append(inputs, newInputs...)
			selected += len(newInputs)
			if *sszLimit > 0 && selected >= *sszLimit {
				break
			}
		}
	}

	printTableHeader()

	passed := 0
	for _, input := range inputs {
		success, userTime := runGuest(guestDir, input.file, *zkcFlags)
		ok := success == input.expectedValid
		if ok {
			passed++
		} else {
			hadError = true
		}
		testName := fmt.Sprintf("%s:%s[%d]", filepath.ToSlash(input.jsonFile), input.testName, input.blockIndex)
		printTableRow(input.fixturePath, testName, input.size, userTime, ok)
	}

	fmt.Fprintf(os.Stderr, "summary: %d/%d passed\n", passed, len(inputs))
	if len(inputs) == 0 {
		fmt.Fprintln(os.Stderr, "no tests ran")
		os.Exit(1)
	}
	if hadError || passed != len(inputs) {
		os.Exit(1)
	}
}

// Prints the table header.
func printTableHeader() {
	fmt.Printf("| %-*s | %-*s | %8s | %8s | %-6s |\n",
		fixturePathColumnWidth, "fixture path",
		testColumnWidth, "test",
		"size (B)", "time (s)", "result")
	fmt.Printf("| %s | %s | -------- | -------- | ------ |\n",
		strings.Repeat("-", fixturePathColumnWidth),
		strings.Repeat("-", testColumnWidth))
}

// Prints one table row.
func printTableRow(fixturePath, testName string, size int, userTime time.Duration, ok bool) {
	result := "fail"
	if ok {
		result = "pass"
	}
	fmt.Printf("| %-*s | %-*s | %8d | %8.3f | %-6s |\n",
		fixturePathColumnWidth,
		escapeCell(fixturePath),
		testColumnWidth,
		omitMiddle(escapeCell(testName), testColumnWidth),
		size,
		userTime.Seconds(),
		result,
	)
}

// Finds the repo root.
func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// Splits a comma list.
func splitList(s string) []string {
	var out []string
	for _, item := range strings.Split(s, ",") {
		item = strings.TrimSpace(item)
		if item != "" {
			out = append(out, item)
		}
	}
	return out
}

// Validates a fixture path.
func resolveFixturePath(rootDir, fixturePath string) (string, string, error) {
	cleanPath := filepath.Clean(filepath.FromSlash(fixturePath))
	if cleanPath == "." {
		return ".", rootDir, nil
	}
	if cleanPath == ".." || filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("invalid fixture path")
	}
	return filepath.ToSlash(cleanPath), filepath.Join(rootDir, cleanPath), nil
}

// Lists JSON files.
func jsonFiles(dir string) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".json") && !strings.HasPrefix(filepath.Base(path), "._") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	return files, nil
}

// Writes selected SSZ inputs from one JSON file.
func writeSSZInputs(sszDir, fixturePath, targetDir, jsonPath string, limit int, fixtureFormat, zesuConvert string) ([]selectedInput, error) {
	jsonRel, err := filepath.Rel(targetDir, jsonPath)
	if err != nil {
		return nil, err
	}
	if jsonRel == ".." || strings.HasPrefix(jsonRel, ".."+string(os.PathSeparator)) {
		return nil, fmt.Errorf("JSON path is outside target dir: %s", jsonPath)
	}

	if fixtureFormat == formatZkevmFixtures {
		return writeZkevmSSZInput(sszDir, fixturePath, jsonRel, jsonPath, zesuConvert)
	}

	blocks, err := statelessInputs(jsonPath, fixtureFormat)
	if err != nil {
		return nil, err
	}
	if len(blocks) == 0 {
		return nil, nil
	}

	jsonStem := strings.TrimSuffix(jsonRel, filepath.Ext(jsonRel))
	outDir := filepath.Join(sszDir, filepath.FromSlash(fixturePath), jsonStem)
	if err := os.RemoveAll(outDir); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, err
	}

	var out []selectedInput
	for i, block := range blocks {
		if limit > 0 && len(out) >= limit {
			break
		}
		outPath := filepath.Join(outDir, fmt.Sprintf("%04d.ssz", i))
		if err := os.WriteFile(outPath, block.input, 0o644); err != nil {
			return nil, err
		}
		out = append(out, selectedInput{
			fixturePath:   fixturePath,
			jsonFile:      jsonRel,
			testName:      block.testName,
			blockIndex:    block.blockIndex,
			file:          outPath,
			size:          len(block.input),
			expectedValid: block.expectedValid,
		})
	}
	return out, nil
}

// Converts one zkevm-fixtures JSON file to one SSZ input.
func writeZkevmSSZInput(sszDir, fixturePath, jsonRel, jsonPath, zesuConvert string) ([]selectedInput, error) {
	if zesuConvert == "" {
		return nil, fmt.Errorf("zesu-convert is required for %s", formatZkevmFixtures)
	}

	jsonStem := strings.TrimSuffix(jsonRel, filepath.Ext(jsonRel))
	outPath := filepath.Join(sszDir, filepath.FromSlash(fixturePath), jsonStem+".ssz")
	if err := os.MkdirAll(filepath.Dir(outPath), 0o755); err != nil {
		return nil, err
	}
	if err := run(os.Stderr, zesuConvert, jsonPath, outPath); err != nil {
		return nil, err
	}

	info, err := os.Stat(outPath)
	if err != nil {
		return nil, err
	}

	return []selectedInput{{
		fixturePath:   fixturePath,
		jsonFile:      jsonRel,
		testName:      zkevmFixtureName(jsonPath),
		blockIndex:    0,
		file:          outPath,
		size:          int(info.Size()),
		expectedValid: true,
	}}, nil
}

// Extracts stateless inputs from one fixture JSON file.
func statelessInputs(path, fixtureFormat string) ([]statelessInput, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	switch fixtureFormat {
	case formatExecutionSpecs:
		return statelessInputsFromMap(data)
	default:
		return nil, fmt.Errorf("unsupported fixture format %q", fixtureFormat)
	}
}

// Extracts inputs from an execution-specs map of named test cases.
func statelessInputsFromMap(data []byte) ([]statelessInput, error) {
	var cases map[string]json.RawMessage
	if err := json.Unmarshal(data, &cases); err != nil {
		return nil, err
	}

	names := make([]string, 0, len(cases))
	for name := range cases {
		names = append(names, name)
	}
	sort.Strings(names)

	var out []statelessInput
	for _, name := range names {
		inputs, err := statelessInputsFromCase(name, cases[name])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		out = append(out, inputs...)
	}
	return out, nil
}

// Extracts inputs from one test-case object.
func statelessInputsFromCase(name string, data []byte) ([]statelessInput, error) {
	var testCase fixtureCase
	if err := json.Unmarshal(data, &testCase); err != nil {
		return nil, err
	}

	var out []statelessInput
	for i, block := range testCase.Blocks {
		if block.StatelessInputBytes == "" || block.StatelessOutputBytes == "" {
			continue
		}
		input, err := hexBytes(block.StatelessInputBytes)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		output, err := hexBytes(block.StatelessOutputBytes)
		if err != nil {
			return nil, fmt.Errorf("%s[%d]: %w", name, i, err)
		}
		if len(output) <= 32 {
			return nil, fmt.Errorf("%s[%d]: statelessOutputBytes too short", name, i)
		}
		out = append(out, statelessInput{
			testName:      name,
			blockIndex:    i,
			input:         input,
			expectedValid: output[32] == 0x01,
		})
	}
	return out, nil
}

// Names zkevm-fixtures JSON files.
func zkevmFixtureName(path string) string {
	data, err := os.ReadFile(path)
	if err == nil {
		var fixture zkevmFixture
		if json.Unmarshal(data, &fixture) == nil && fixture.Name != "" {
			return fixture.Name
		}
	}
	name := filepath.Base(path)
	return strings.TrimSuffix(name, filepath.Ext(name))
}

// Decodes 0x-prefixed hex bytes.
func hexBytes(s string) ([]byte, error) {
	if len(s) >= 2 && s[0] == '0' && (s[1] == 'x' || s[1] == 'X') {
		s = s[2:]
	}
	if len(s)%2 != 0 {
		return nil, fmt.Errorf("odd hex length")
	}
	return hex.DecodeString(s)
}

// Runs a command.
func run(w io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// Runs the guest.
func runGuest(guestDir, input, zkcFlags string) (bool, time.Duration) {
	cmd := exec.Command(
		"make", "--no-print-directory", "-C", guestDir, "exec",
		"INPUT="+input,
		"ZKC_EXEC_FLAGS="+zkcFlags,
	)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	err := cmd.Run()
	if cmd.ProcessState == nil {
		return err == nil, 0
	}
	return err == nil, cmd.ProcessState.UserTime()
}

// Escapes table cells.
func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

// Shortens long text.
func omitMiddle(s string, width int) string {
	if len(s) <= width {
		return s
	}
	const marker = "[...]"
	if width <= len(marker) {
		return s[len(s)-width:]
	}
	remaining := width - len(marker)
	prefix := remaining / 2
	suffix := remaining - prefix
	return s[:prefix] + marker + s[len(s)-suffix:]
}

// Exits on error.
func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
