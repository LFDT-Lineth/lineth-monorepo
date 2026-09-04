package instrumentation_test

// Harness for collecting (WizardParameters, compile-time) data points from
// real ZKC inputs, which can then be passed to Fit to train a LinearModel.
//
// Expected testdata layout:
//
//	instrumentation/testdata/fixtures/
//	  foo.zkc      ZKC source file (compiled on-the-fly, same as zkcdriver tests)
//	  foo.accepts  JSONL file — one JSON inputs object per line
//	  bar.zkc
//	  bar.accepts
//	  ...
//
// The .accepts files are copied directly from the zkcdriver testdata.
// Each line is a JSON inputs object; running it through the ZKC program
// may produce multiple shards.
//
// Populate testdata with:
//
//	make download-instrumentation-testdata
//
// Run the collector test:
//
//	go test ./instrumentation/ -run TestCollectAndFit -v
//
// Run the per-shard compilation benchmark:
//
//	go test ./instrumentation/ -bench BenchmarkCompileShard -benchtime=1x

import (
	"bufio"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/instrumentation"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/logderivativesum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/lookuptologderivsum"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/nonnative"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/rangecheck"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

const (
	testdataGlob    = "testdata/fixtures/*.zkc"
	inputsExtension = ".accepts"
	zkcExtension    = ".zkc"

	// maxInputsPerFixture caps how many input lines are processed per fixture.
	// If an .accepts file has more entries, the remainder is silently ignored.
	maxInputsPerFixture = 50
)

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = codegen.DEFAULT_CONFIG
)

// fixtureCase holds one compiled binary (serialized) and all of its input
// lines. The binary is shared across every input of the same fixture.
type fixtureCase struct {
	name           string
	constraintsBin []byte
	inputs         [][]byte
}

// compileZkc compiles a .zkc source file to a serialized binary constraints
// file. Matches compileBinaryConstraints in zkcdriver_test.go.
func compileZkc(srcPath string) (bin []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during zkc compilation: %v", r)
		}
	}()

	srcZkc, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", srcPath, err)
	}
	src := source.NewSourceFile(srcPath, srcZkc)
	macroProgram, _, errs := compiler.Compile(zkcField, *src)
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s: zkc compile errors", srcPath)
	}
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	if len(errs) > 0 {
		return nil, fmt.Errorf("%s: ast compile errors", srcPath)
	}
	binFile := constraints.NewBinaryFile[koalabear.Element](nil, nil, zkcField, zkcCfg.GetMaxStaticHeight(), ir)
	return binFile.MarshalBinary()
}

// loadFixtureCases globs testdata/fixtures/*.zkc, compiles each source file,
// pairs it with the .accepts file of the same base name, and returns one
// fixtureCase per source file. Skips the caller's test if no testdata is found.
func loadFixtureCases(tb testing.TB) []fixtureCase {
	tb.Helper()

	zkcFiles, err := filepath.Glob(testdataGlob)
	if err != nil {
		tb.Fatalf("globbing %s: %v", testdataGlob, err)
	}
	if len(zkcFiles) == 0 {
		tb.Skipf("no fixture testdata found; run `make download-instrumentation-testdata`")
	}

	var fixtures []fixtureCase
	for _, zkcPath := range zkcFiles {
		baseName := strings.TrimSuffix(filepath.Base(zkcPath), zkcExtension)
		inputsPath := strings.TrimSuffix(zkcPath, zkcExtension) + inputsExtension

		constraintsBin, err := compileZkc(zkcPath)
		if err != nil {
			tb.Logf("skipping %s: %v", baseName, err)
			continue
		}
		inputsFile, err := os.Open(inputsPath)
		if err != nil {
			tb.Logf("skipping %s: no paired %s file: %v", baseName, inputsExtension, err)
			continue
		}

		var inputs [][]byte
		scanner := bufio.NewScanner(inputsFile)
		scanner.Buffer(make([]byte, 1<<20), 1<<20)
		for scanner.Scan() {
			line := scanner.Bytes()
			if !bytes.HasPrefix(line, []byte("{")) {
				continue
			}
			if len(inputs) >= maxInputsPerFixture {
				break
			}
			inputsCopy := make([]byte, len(line))
			copy(inputsCopy, line)
			inputs = append(inputs, inputsCopy)
		}
		inputsFile.Close()
		if err := scanner.Err(); err != nil {
			tb.Logf("error reading %s: %v", inputsPath, err)
		}

		if len(inputs) > 0 {
			fixtures = append(fixtures, fixtureCase{
				name:           baseName,
				constraintsBin: constraintsBin,
				inputs:         inputs,
			})
		}
	}

	if len(fixtures) == 0 {
		tb.Skipf("no fixtures found in testdata; run `make download-instrumentation-testdata`")
	}
	return fixtures
}

// collectSample builds a fresh wiop.System from the constraints binary,
// extracts WizardParameters before compilation, then times the full Arcane
// compiler pipeline. Panics from NewZkCDriver or the compilers are caught
// and returned as errors.
func collectSample(constraintsBin []byte) (s instrumentation.Sample, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	sys := wiop.NewSystemf("instrumentation-fixture")
	sys.NewRound()
	_ = zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(constraintsBin))

	params := instrumentation.MeasureSystem(sys)

	start := time.Now()
	instrumentationCompilePipeline(sys)
	elapsed := time.Since(start)

	return instrumentation.Sample{Params: params, Duration: elapsed}, nil
}

// instrumentationCompilePipeline runs the full Arcane compiler pipeline used
// in production. Must be kept in sync with zkcdriver_test.go:proverCompilePipeline.
func instrumentationCompilePipeline(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)
}

// TestCollectAndFit collects one (WizardParameters, compile-time) sample per
// input line and fits a LinearModel when enough samples are available. Run
// with -v to see per-fixture measurements and the model's per-contribution
// breakdown.
func TestCollectAndFit(t *testing.T) {
	fixtures := loadFixtureCases(t)

	var samples []instrumentation.Sample
	for _, fx := range fixtures {
		t.Run(fx.name, func(t *testing.T) {
			for i := range fx.inputs {
				t.Run(fmt.Sprintf("input=%d", i), func(t *testing.T) {
					t.Parallel()
					sample, err := collectSample(fx.constraintsBin)
					if err != nil {
						t.Logf("skipping: %v", err)
						return
					}
					t.Logf("compile=%v cols=%d cells=%d",
						sample.Duration, sample.Params.NbColumns, sample.Params.NbCells)
					samples = append(samples, sample)
				})
			}
		})
	}

	t.Logf("collected %d samples", len(samples))
	if len(samples) < 17 {
		t.Skipf("need at least 17 samples for Fit, have %d", len(samples))
	}
	model, err := instrumentation.Fit(samples)
	if err != nil {
		t.Logf("Fit skipped: %v", err)
		return
	}
	t.Log("model contributions for sample[0]:")
	for _, c := range model.Contributions(samples[0].Params) {
		t.Logf("  %-48s %v", c.Name, c.Value)
	}
}

// BenchmarkCompileShard measures Define + full Arcane compiler pipeline for
// each input. Since all inputs of a fixture share the same binary,
// WizardParameters are extracted once per fixture for the custom metrics.
//
// Use -benchtime=1x for a single compilation timing per input.
func BenchmarkCompileShard(b *testing.B) {
	fixtures := loadFixtureCases(b)

	for _, fx := range fixtures {
		// WizardParameters are determined by the binary alone; extract once per fixture.
		sys0 := wiop.NewSystemf("probe")
		sys0.NewRound()
		_ = zkcdriver.NewZkCDriver(sys0, zkcdriver.Settings{}, bytes.NewReader(fx.constraintsBin))
		params := instrumentation.MeasureSystem(sys0)

		for i := range fx.inputs {
			b.Run(fmt.Sprintf("%s/input=%d", fx.name, i), func(b *testing.B) {
				for b.Loop() {
					sys := wiop.NewSystemf("instrumentation-fixture")
					sys.NewRound()
					_ = zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(fx.constraintsBin))
					instrumentationCompilePipeline(sys)
				}
				b.ReportMetric(float64(params.NbCells), "cells/op")
				b.ReportMetric(float64(params.NbColumns), "columns/op")
				b.ReportMetric(float64(params.NbVanishingConstraints), "vanishing/op")
			})
		}
	}
}
