package main

// Examples, from the repository root:
// Run 10 ssz files for each selected target folder
//   GOCACHE=/tmp/go-build go run ./riscv-guests/l2-execution/scripts/run_execution_specs_ssz_fixtures.go --root-folders for_amsterdam --target-folders amsterdam,prague --ssz-limit 10
// Convert one JSON fixture and run one generated SSZ file for every root/target folder combination
//   GOCACHE=/tmp/go-build go run ./riscv-guests/l2-execution/scripts/run_execution_specs_ssz_fixtures.go --root-folders for_amsterdam --target-folders '*' --json-limit 5 --ssz-limit 5

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

type fixtureSet struct {
	rootFolder   string
	targetFolder string
	outDir       string
	files        []string
}

func main() {
	rootsFlag := flag.String("root-folders", "for_amsterdam", "comma-separated ZKEVM_FIXTURES_ROOT_FOLDER values, or '*' for all")
	targetsFlag := flag.String("target-folders", "amsterdam", "comma-separated ZKEVM_FIXTURES_TARGET_FOLDER values, or '*' for all")
	suite := flag.String("suite", "blockchain_tests", "ZKEVM_FIXTURES_SUITE value")
	jsonLimit := flag.Int("json-limit", 0, "maximum JSON fixture files to convert per folder combination; 0 means all")
	sszLimit := flag.Int("ssz-limit", 0, "maximum generated SSZ files to run per folder combination; 0 means all")
	zkcFlags := flag.String("zkc-flags", "--gogen --fast -q", "flags forwarded to zkc exec")
	flag.Parse()

	if *jsonLimit < 0 || *sszLimit < 0 {
		must(fmt.Errorf("limits must be non-negative"))
	}

	root, err := repoRoot()
	must(err)

	guestDir := filepath.Join(root, "riscv-guests", "l2-execution")
	cacheDir := filepath.Join(guestDir, ".cache", "large-ssz-fixtures")

	roots := splitList(*rootsFlag)
	targets := splitList(*targetsFlag)
	fixtureSuiteDir := filepath.Join(cacheDir, "fixtures", *suite)
	if isWildcard(roots) || isWildcard(targets) {
		must(run(os.Stderr, "make", "-C", guestDir, "get-execution-specs-json-fixtures", "LARGE_SSZ_DIR="+cacheDir))
	}
	if isWildcard(roots) {
		roots, err = folderNames(fixtureSuiteDir)
		must(err)
	}

	must(run(os.Stderr, "make", "-C", guestDir, "compile"))

	var fixtureSets []fixtureSet
	hadError := false
	for _, rootFolder := range roots {
		targetsForRoot := targets
		if isWildcard(targets) {
			targetsForRoot, err = folderNames(filepath.Join(fixtureSuiteDir, rootFolder))
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s: %v\n", rootFolder, err)
				hadError = true
				continue
			}
		}

		for _, targetFolder := range targetsForRoot {
			outDir := filepath.Join(cacheDir, "ssz", *suite, rootFolder, targetFolder)
			logPath := filepath.Join(cacheDir, fmt.Sprintf("zkevm-runner-%s-%s.log", rootFolder, targetFolder))

			if err := os.RemoveAll(outDir); err != nil {
				fmt.Fprintf(os.Stderr, "clear %s: %v\n", outDir, err)
				hadError = true
				continue
			}

			err := run(os.Stderr,
				"make", "-C", guestDir, "gen-execution-specs-ssz-fixtures",
				"ZKEVM_FIXTURES_SUITE="+*suite,
				"ZKEVM_FIXTURES_ROOT_FOLDER="+rootFolder,
				"ZKEVM_FIXTURES_TARGET_FOLDER="+targetFolder,
				"LARGE_SSZ_DIR="+cacheDir,
				"LARGE_SSZ_OUT_DIR="+outDir,
				"LARGE_SSZ_LOG="+logPath,
				fmt.Sprintf("JSON_LIMIT=%d", *jsonLimit),
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s/%s: %v\n", rootFolder, targetFolder, err)
				hadError = true
				continue
			}

			files, err := sszFiles(outDir, *sszLimit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "list %s: %v\n", outDir, err)
				hadError = true
				continue
			}

			fixtureSets = append(fixtureSets, fixtureSet{
				rootFolder:   rootFolder,
				targetFolder: targetFolder,
				outDir:       outDir,
				files:        files,
			})
		}
	}

	printTableHeader()

	total := 0
	passed := 0
	for _, set := range fixtureSets {
		for _, file := range set.files {
			total++
			ok, elapsed := runGuest(guestDir, file, *zkcFlags)
			if ok {
				passed++
			} else {
				hadError = true
			}
			size := fileSize(file)
			testName, _ := filepath.Rel(set.outDir, file)
			printTableRow(set.rootFolder, set.targetFolder, testName, size, elapsed, ok)
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

func printTableHeader() {
	fmt.Printf("| %-32s | %-17s | %-96s | %10s | %10s | %-6s |\n",
		"root folder", "target folder", "test", "size bytes", "exec time", "result")
	fmt.Println("| -------------------------------- | ----------------- | ------------------------------------------------------------------------------------------------ | ---------- | ---------- | ------ |")
}

func printTableRow(rootFolder, targetFolder, testName string, size int64, elapsed time.Duration, ok bool) {
	fmt.Printf("| %-32s | %-17s | %-96s | %10d | %10s | %-6s |\n",
		rootFolder,
		targetFolder,
		escapeCell(testName),
		size,
		elapsed.Round(time.Millisecond),
		result(ok),
	)
}

func repoRoot() (string, error) {
	out, err := exec.Command("git", "rev-parse", "--show-toplevel").Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

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

func isWildcard(items []string) bool {
	return len(items) == 1 && items[0] == "*"
}

func folderNames(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	var folders []string
	for _, entry := range entries {
		if entry.IsDir() {
			folders = append(folders, entry.Name())
		}
	}
	sort.Strings(folders)
	return folders, nil
}

func run(w io.Writer, name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = w
	cmd.Stderr = w
	return cmd.Run()
}

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

func runGuest(guestDir, input, zkcFlags string) (bool, time.Duration) {
	start := time.Now()
	err := run(io.Discard,
		"make", "--no-print-directory", "-C", guestDir, "exec-only",
		"INPUT="+input,
		"ZKC_EXEC_FLAGS="+zkcFlags,
	)
	return err == nil, time.Since(start)
}

func fileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

func result(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func escapeCell(s string) string {
	return strings.ReplaceAll(s, "|", "\\|")
}

func must(err error) {
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
