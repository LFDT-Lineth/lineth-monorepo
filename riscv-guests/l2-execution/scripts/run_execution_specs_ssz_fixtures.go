package main

// Examples, from the repository root:
// Run up to 100 SSZ files from each selected fixture path:
//   make -C riscv-guests/l2-execution run-execution-specs-ssz-fixtures
// Run all fixtures in each selected fixture path (blockchain_tests/for_amsterdam/amsterdam and blockchain_tests/for_amsterdam/osaka):
//   make -C riscv-guests/l2-execution run-execution-specs-ssz-fixtures EXECUTION_SPECS_RUN_SSZ_LIMIT=0

import (
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
)

type fixtureSet struct {
	fixturePath string
	jsonFile    string
	outDir      string
	files       []string
}

// Runs selected fixtures.
func main() {
	fixturePathsFlag := flag.String("fixture-paths", "blockchain_tests/for_amsterdam/amsterdam", "comma-separated fixture paths under the execution-specs fixture root")
	sszLimit := flag.Int("ssz-limit", 0, "maximum generated SSZ files to run per fixture path; 0 means all")
	zkcFlags := flag.String("zkc-flags", "--gogen --fast -q", "flags forwarded to zkc exec")
	flag.Parse()

	if *sszLimit < 0 {
		must(fmt.Errorf("limits must be non-negative"))
	}

	root, err := repoRoot()
	must(err)

	guestDir := filepath.Join(root, "riscv-guests", "l2-execution")
	jsonFixturesDir := filepath.Join(os.TempDir(), "execution-specs-json-fixtures")
	sszFixturesDir := filepath.Join(os.TempDir(), "execution-specs-ssz-fixtures")
	fixtureRoot := filepath.Join(jsonFixturesDir, "fixtures")

	fixturePaths := splitList(*fixturePathsFlag)
	if len(fixturePaths) == 0 {
		must(fmt.Errorf("fixture-paths must not be empty"))
	}

	must(run(os.Stderr, "make", "-C", guestDir, "get-execution-specs-json-fixtures", "EXECUTION_SPECS_JSON_FIXTURES_DIR="+jsonFixturesDir))
	must(run(os.Stderr, "make", "-C", guestDir, "compile"))

	var fixtureSets []fixtureSet
	hadError := false
	for _, fixturePath := range fixturePaths {
		fixturePath, targetDir, err := resolveFixturePath(fixtureRoot, fixturePath)
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

		selectedSSZ := 0
		for _, jsonPath := range jsonPaths {
			jsonRel, singleJSONDir, err := prepareSingleJSONDir(jsonFixturesDir, fixturePath, targetDir, jsonPath)
			if err != nil {
				fmt.Fprintf(os.Stderr, "prepare %s: %v\n", jsonPath, err)
				hadError = true
				continue
			}

			jsonName := strings.TrimSuffix(jsonRel, filepath.Ext(jsonRel))
			outDir := filepath.Join(sszFixturesDir, filepath.FromSlash(fixturePath), jsonName)
			logPath := filepath.Join(sszFixturesDir, "logs", filepath.FromSlash(fixturePath), jsonName+".log")

			if err := os.RemoveAll(outDir); err != nil {
				fmt.Fprintf(os.Stderr, "clear %s: %v\n", outDir, err)
				hadError = true
				continue
			}
			if err := os.MkdirAll(filepath.Dir(logPath), 0o755); err != nil {
				fmt.Fprintf(os.Stderr, "create log dir %s: %v\n", filepath.Dir(logPath), err)
				hadError = true
				continue
			}

			err = run(os.Stderr,
				"make", "-C", guestDir, "gen-execution-specs-ssz-fixtures",
				"EXECUTION_SPECS_FIXTURES_PATH="+fixturePath,
				"EXECUTION_SPECS_FIXTURES_TARGET_DIR="+singleJSONDir,
				"EXECUTION_SPECS_JSON_FIXTURES_DIR="+jsonFixturesDir,
				"EXECUTION_SPECS_SSZ_FIXTURES_DIR="+sszFixturesDir,
				"EXECUTION_SPECS_SSZ_OUT_DIR="+outDir,
				"EXECUTION_SPECS_SSZ_LOG="+logPath,
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s/%s: %v\n", fixturePath, jsonRel, err)
				hadError = true
				continue
			}

			files, err := sszFiles(outDir, *sszLimit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "list %s: %v\n", outDir, err)
				hadError = true
				continue
			}
			if len(files) == 0 {
				fmt.Fprintf(os.Stderr, "no SSZ files generated for %s/%s\n", fixturePath, jsonRel)
				hadError = true
				continue
			}
			if *sszLimit > 0 {
				remaining := *sszLimit - selectedSSZ
				if remaining <= 0 {
					break
				}
				if len(files) > remaining {
					files = files[:remaining]
				}
			}

			fixtureSets = append(fixtureSets, fixtureSet{
				fixturePath: fixturePath,
				jsonFile:    jsonRel,
				outDir:      outDir,
				files:       files,
			})
			selectedSSZ += len(files)
			if *sszLimit > 0 && selectedSSZ >= *sszLimit {
				break
			}
		}
	}

	printTableHeader()

	total := 0
	passed := 0
	for _, set := range fixtureSets {
		for _, file := range set.files {
			total++
			ok, userTime := runGuest(guestDir, file, *zkcFlags)
			if ok {
				passed++
			} else {
				hadError = true
			}
			size := fileSize(file)
			sszName, _ := filepath.Rel(set.outDir, file)
			testName := filepath.ToSlash(set.jsonFile) + ":" + filepath.ToSlash(sszName)
			printTableRow(set.fixturePath, testName, size, userTime, ok)
		}
	}

	fmt.Fprintf(os.Stderr, "summary: %d/%d passed\n", passed, total)
	if total == 0 {
		fmt.Fprintln(os.Stderr, "no tests ran")
		os.Exit(1)
	}
	if hadError || passed != total {
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
func printTableRow(fixturePath, testName string, size int64, userTime time.Duration, ok bool) {
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
	if cleanPath == "." || cleanPath == ".." || filepath.IsAbs(cleanPath) || strings.HasPrefix(cleanPath, ".."+string(os.PathSeparator)) {
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
		if !d.IsDir() && strings.HasSuffix(path, ".json") {
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

// Creates a one-JSON directory.
func prepareSingleJSONDir(jsonFixturesDir, fixturePath, targetDir, jsonPath string) (string, string, error) {
	jsonRel, err := filepath.Rel(targetDir, jsonPath)
	if err != nil {
		return "", "", err
	}
	if jsonRel == ".." || strings.HasPrefix(jsonRel, ".."+string(os.PathSeparator)) {
		return "", "", fmt.Errorf("JSON path is outside target dir: %s", jsonPath)
	}

	jsonName := strings.TrimSuffix(jsonRel, filepath.Ext(jsonRel))
	singleJSONDir := filepath.Join(jsonFixturesDir, "single-json", filepath.FromSlash(fixturePath), jsonName)
	if err := os.RemoveAll(singleJSONDir); err != nil {
		return "", "", err
	}

	dst := filepath.Join(singleJSONDir, jsonRel)
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return "", "", err
	}
	if err := copyFile(jsonPath, dst); err != nil {
		return "", "", err
	}
	return jsonRel, singleJSONDir, nil
}

// Copies one file.
func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}

// Runs a command.
func run(w io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

// Lists selected SSZ files.
func sszFiles(dir string, limit int) ([]string, error) {
	var files []string
	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".ssz") {
			files = append(files, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(files)
	if limit > 0 && len(files) > limit {
		files = files[:limit]
	}
	return files, nil
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

// Returns file size.
func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
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
