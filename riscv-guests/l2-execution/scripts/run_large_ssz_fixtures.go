package main

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

func main() {
	rootsFlag := flag.String("root-folders", "for_amsterdam", "comma-separated ZKEVM_FIXTURES_ROOT_FOLDER values")
	targetsFlag := flag.String("target-folders", "amsterdam", "comma-separated ZKEVM_FIXTURES_TARGET_FOLDER values")
	suite := flag.String("suite", "blockchain_tests", "ZKEVM_FIXTURES_SUITE value")
	limit := flag.Int("limit", 0, "maximum SSZ files to run per folder combination; 0 means all")
	zkcFlags := flag.String("zkc-flags", "--gogen --fast -q", "flags forwarded to zkc exec")
	flag.Parse()

	root, err := repoRoot()
	must(err)

	guestDir := filepath.Join(root, "riscv-guests", "l2-execution")
	cacheDir := filepath.Join(guestDir, ".cache", "large-ssz-fixtures")
	bin := filepath.Join(guestDir, "zig-out", "bin", "evm_execution_guest")
	arithMakefile := filepath.Join(root, "arithmetization", "src", "test", "Makefile")

	must(run(os.Stderr, "make", "-C", guestDir, "compile"))

	fmt.Println("| root folder | target folder | test | size bytes | time | result |")
	fmt.Println("|---|---|---|---:|---:|---|")

	total := 0
	passed := 0
	for _, rootFolder := range splitList(*rootsFlag) {
		for _, targetFolder := range splitList(*targetsFlag) {
			outDir := filepath.Join(cacheDir, "ssz", *suite, rootFolder, targetFolder)
			logPath := filepath.Join(cacheDir, fmt.Sprintf("zkevm-runner-%s-%s.log", rootFolder, targetFolder))

			if err := os.RemoveAll(outDir); err != nil {
				fmt.Fprintf(os.Stderr, "clear %s: %v\n", outDir, err)
				continue
			}

			err := run(os.Stderr,
				"make", "-C", guestDir, "gen-large-ssz-fixtures",
				"ZKEVM_FIXTURES_SUITE="+*suite,
				"ZKEVM_FIXTURES_ROOT_FOLDER="+rootFolder,
				"ZKEVM_FIXTURES_TARGET_FOLDER="+targetFolder,
				"LARGE_SSZ_DIR="+cacheDir,
				"LARGE_SSZ_OUT_DIR="+outDir,
				"LARGE_SSZ_LOG="+logPath,
			)
			if err != nil {
				fmt.Fprintf(os.Stderr, "skip %s/%s: %v\n", rootFolder, targetFolder, err)
				continue
			}

			files, err := sszFiles(outDir, *limit)
			if err != nil {
				fmt.Fprintf(os.Stderr, "list %s: %v\n", outDir, err)
				continue
			}

			for _, file := range files {
				total++
				ok, elapsed := runGuest(arithMakefile, bin, file, *zkcFlags)
				if ok {
					passed++
				}
				size := fileSize(file)
				testName, _ := filepath.Rel(outDir, file)
				fmt.Printf("| %s | %s | %s | %d | %s | %s |\n",
					rootFolder,
					targetFolder,
					escapeCell(testName),
					size,
					elapsed.Round(time.Millisecond),
					result(ok),
				)
			}
		}
	}

	fmt.Fprintf(os.Stderr, "summary: %d/%d passed\n", passed, total)
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

func runGuest(arithMakefile, bin, input, zkcFlags string) (bool, time.Duration) {
	start := time.Now()
	err := run(io.Discard,
		"make", "-f", arithMakefile, "elf-exec",
		"BIN_EXT="+bin,
		"IN_BYTES=@"+input,
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
