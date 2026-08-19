// Command generate-riscv-system compiles the r5_test.zkc program,
// proves an honest witness, and writes the resulting verifier-ray
// CompiledSystem to testdata/generated/riscv_system.zig. It exercises the full
// codegen path against a real proof rather than a synthetic fixture:
// AssertAllVerifierActionsHandled -> BuildCompiledSystem ->
// WriteCompiledSystemZig.
//
// compileBinaryConstraints and runCompilePipeline below mirror
// prover-ray/zkcdriver/zkcdriver_test.go's own private test helpers of the
// same shape, with one difference: pcs.Compile actually runs here (that test
// helper's comment still references an older ZKC lookup-constraint issue,
// but r5_test.zkc compiles and proves successfully with PCS enabled even
// though it uses pub-input memories).
package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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
	verifierraycodegen "github.com/consensys/linea-monorepo/verifier-ray/codegen"
)

const zkcSourcePath = "../../../prover-ray/zkcdriver/testdata/r5_test.zkc"

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = codegen.DEFAULT_CONFIG
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "generate-riscv-system:", err)
		os.Exit(1)
	}
}

func run() error {
	// Step 1: compile r5_test.zkc down to R5's binary constraint format.
	binF, err := compileBinaryConstraints(zkcSourcePath)
	if err != nil {
		return fmt.Errorf("compiling %s: %w", zkcSourcePath, err)
	}
	compiledConstraints, err := binF.MarshalBinary()
	if err != nil {
		return fmt.Errorf("marshaling %s constraints: %w", zkcSourcePath, err)
	}

	// r5_test.zkc processes two segments of tuple-valued rows. It checks each
	// row sum, each segment total, then the grand total across all segments.
	inputs := &zkcdriver.PreReadInputs{Inputs: map[string][]byte{
		// segments = [(offset=0, count=2), (offset=2, count=2)]
		"segments": {0x00, 0x02, 0x02, 0x02},
		// values = [(1,2), (3,4), (5,6), (7,8)]
		"values": {
			0x00, 0x01, 0x00, 0x02,
			0x00, 0x03, 0x00, 0x04,
			0x00, 0x05, 0x00, 0x06,
			0x00, 0x07, 0x00, 0x08,
		},
		// expected row sums = [3, 7, 11, 15]
		"expected": {
			0x00, 0x03,
			0x00, 0x07,
			0x00, 0x0b,
			0x00, 0x0f,
		},
		// segment totals = [10, 26]
		"segment_totals": {
			0x00, 0x0a,
			0x00, 0x1a,
		},
		// grand total = [36]
		"grand_total": {
			0x00, 0x24,
		},
	}}

	// Step 2: build the wiop.System driving the constraints through zkcdriver,
	// then run it through the full compiler pipeline (nonnative, rangecheck,
	// lookups, message bus, grand product, log-derivative sum, local
	// vanishing, global, PCS).
	sys := wiop.NewSystemf("zkc-riscv-system")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	runCompilePipeline(sys)

	// Step 3: assign the witness for the honest inputs above, produce a real
	// proof, and sanity-check it against the Go verifier before generating
	// any Zig fixtures from it.
	proof, pub := sys.Prove(
		func(assignRt *wiop.Runtime) {
			driver.AssignWithPreRead(assignRt, inputs)
		},
		wiop.ProveOptions{CheckUnreducedQueries: true})
	if err := sys.Verify(proof, pub); err != nil {
		return fmt.Errorf("verifying %s proof: %w", zkcSourcePath, err)
	}

	// Step 4: confirm codegen can handle every verifier action the compiled
	// system produced, then extract the CompiledSystem (spec, vanishing,
	// log-derivative, grand-product, row-limit, PCS) codegen needs.
	if err := verifierraycodegen.AssertAllVerifierActionsHandled(sys); err != nil {
		return fmt.Errorf("AssertAllVerifierActionsHandled: %w", err)
	}
	system, err := verifierraycodegen.BuildCompiledSystem(sys)
	if err != nil {
		return fmt.Errorf("BuildCompiledSystem: %w", err)
	}

	// Step 5: render the CompiledSystem (including PCS, via WritePcs) as Zig
	// source into systemBuf.
	var systemBuf bytes.Buffer
	if err := verifierraycodegen.WriteCompiledSystemZig(&systemBuf, 0, system, verifierraycodegen.CompiledSystemZigOptions{
		EmitHeader:         true,
		ProtocolImport:     `@import("verifier_ray").protocol`,
		FieldImport:        `@import("verifier_ray").field.koalabear`,
		VanishingImport:    `@import("verifier_ray").query.vanishing`,
		LogDerivImport:     `@import("verifier_ray").query.logderivativesum`,
		GrandProductImport: `@import("verifier_ray").query.grandproduct`,
		RowLimitImport:     `@import("verifier_ray").query.rowlimit`,
		WritePcs:           true,
		PcsImport:          `@import("verifier_ray").query.pcs`,
		FriImport:          `@import("verifier_ray").query.fri`,
	}); err != nil {
		return fmt.Errorf("WriteCompiledSystemZig: %w", err)
	}

	// Step 6: stitch the sub-verifier systems just written into a single
	// verifier.Systems value, the top-level struct verifier.verify expects.
	fmt.Fprintf(&systemBuf,
		"\nconst verifier = @import(\"verifier_ray\").verifier;\npub const system_0_systems = verifier.Systems{ .public_input = system_0_public_input, .vanishing = system_0, .logderivativesum = system_0_logderiv, .grandproduct = system_0_grandproduct, .rowlimit = system_0_rowlimit, .pcs = pcs_system_0 };\n",
	)

	outDir := "../../testdata/generated"
	systemPath := filepath.Join(outDir, "riscv_system.zig")
	formatted, err := runZigFmt(systemBuf.Bytes())
	if err != nil {
		return fmt.Errorf("zig fmt %s: %w", systemPath, err)
	}
	if err := os.WriteFile(systemPath, formatted, 0o644); err != nil {
		return fmt.Errorf("writing %s: %w", systemPath, err)
	}
	fmt.Println("wrote", systemPath)

	return nil
}

func compileBinaryConstraints(srcPath string) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
	// recover panics. ZKC tends to panic when it fails compiling, so we want to catch those and return them as errors.
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during zkc compilation: %v", r)
		}
	}()

	srcZkc, err := os.ReadFile(srcPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read zkc source file: %w", err)
	}
	src := source.NewSourceFile(srcPath, srcZkc)
	macroProgram, _, errs := compiler.Compile(zkcField, *src)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	ir, errs := ast.Compile(macroProgram, zkcCfg)
	if len(errs) > 0 {
		for i := range errs {
			fmt.Printf("zkc compile error: %s\n", errs[i].Error())
		}
		return nil, fmt.Errorf("failed to compile zkc source")
	}
	binfile = constraints.NewBinaryFile[koalabear.Element](nil, nil, zkcField, zkcCfg.GetMaxStaticDepth(), ir)
	return binfile, nil
}

func runCompilePipeline(sys *wiop.System) {
	nonnative.Compile(sys)
	rangecheck.Compile(sys)
	lookuptologderivsum.Compile(sys)
	messagebus.Compile(sys)
	grandproduct.Compile(sys)
	logderivativesum.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)
}

// runZigFmt pipes data through `zig fmt` on a temp file and returns the
// formatted result, mirroring testdata/generate/main.go's own runZigFmt so
// generated output stays consistent with the rest of this repo's generated
// Zig files.
func runZigFmt(data []byte) ([]byte, error) {
	tmp, err := os.CreateTemp("", "verifier-ray-riscv-system-*.zig")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return nil, err
	}
	if err := tmp.Close(); err != nil {
		return nil, err
	}

	cmd := os.Getenv("ZIG")
	if cmd == "" {
		cmd = "zig"
	}
	if err := exec.Command(cmd, "fmt", tmp.Name()).Run(); err != nil {
		return nil, err
	}
	return os.ReadFile(tmp.Name())
}
