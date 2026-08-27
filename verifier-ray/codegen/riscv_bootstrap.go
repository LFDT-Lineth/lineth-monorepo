package codegen

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
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
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/util/field"
	"github.com/LFDT-Lineth/zkc/pkg/util/field/koalabear"
	"github.com/LFDT-Lineth/zkc/pkg/util/source"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/ast"
	zkccodegen "github.com/LFDT-Lineth/zkc/pkg/zkc/compiler/codegen"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

var (
	zkcField = field.KOALABEAR_16
	zkcCfg   = zkccodegen.DEFAULT_CONFIG
)

// HonestRiscvArtifacts are the verifier-facing outputs from compiling the real
// RISC-V main.zkc entrypoint and proving an honest minimal guest witness.
type HonestRiscvArtifacts struct {
	CompiledSystem CompiledSystem
	VerifyInput    proofserialization.VerifyInput
}

// HonestRiscvGuest names one of the honest, halting guest ELFs proved against
// main.zkc. Every guest here must be accepted by main.zkc's real interpreter
// circuit; ExitOneGuestELF is deliberately excluded because main.zkc must
// reject it (see prover-ray/zkcdriver/r5_test.go's
// TestRisc5ExitOneGuestIsRejected), so it has no honest proof to produce.
type HonestRiscvGuest struct {
	// Name identifies this guest in generated file/const names (e.g.
	// "exit_zero", "poseidon2") — must be a valid Zig identifier suffix.
	Name string
	ELF  []byte
}

// HonestRiscvGuests lists every guest ELF proved end to end against the real
// main.zkc interpreter circuit, covering the full RV64I + M-extension +
// custom-precompile surface exercised by prover-ray/zkcdriver/r5_test.go's
// TestRisc5InstructionCoverageGuests, but through the real prove/PCS/verify
// pipeline rather than trace-and-check-constraints alone.
var HonestRiscvGuests = []HonestRiscvGuest{
	{Name: "exit_zero", ELF: zkc_r5.ExitZeroGuestELF},
	{Name: "memory_round_trip", ELF: zkc_r5.MemoryRoundTripGuestELF},
	{Name: "arithmetic", ELF: zkc_r5.ArithmeticGuestELF},
	{Name: "branches", ELF: zkc_r5.BranchesGuestELF},
	{Name: "load_store_widths", ELF: zkc_r5.LoadStoreWidthsGuestELF},
	{Name: "poseidon2", ELF: zkc_r5.Poseidon2GuestELF},
	{Name: "keccak", ELF: zkc_r5.KeccakGuestELF},
	{Name: "write_output", ELF: zkc_r5.WriteOutputGuestELF},
	{Name: "immediate_alu", ELF: zkc_r5.ImmediateALUGuestELF},
	{Name: "word_width", ELF: zkc_r5.WordWidthGuestELF},
}

type honestRiscvProof struct {
	Sys   *wiop.System
	Proof wiop.Proof
	Pub   wiop.PublicInput
}

// BuildHonestRiscvArtifacts exercises the real main.zkc -> wiop.System -> real
// proof path for zkc_r5.ExitZeroGuestELF and returns the verifier-facing
// artifacts derived from it. Kept for callers that only need one honest
// witness (e.g. codegen/generate-riscv-system's default single-guest fixture);
// proves only HonestRiscvGuests[0], not every guest — use
// BuildAllHonestRiscvArtifacts to cover every guest in HonestRiscvGuests
// against the same compiled system.
func BuildHonestRiscvArtifacts() (HonestRiscvArtifacts, error) {
	compiled, err := compileHonestRiscvSystem()
	if err != nil {
		return HonestRiscvArtifacts{}, err
	}
	result, err := compiled.proveGuest(HonestRiscvGuests[0])
	if err != nil {
		return HonestRiscvArtifacts{}, err
	}
	return result.Artifacts, nil
}

// HonestRiscvArtifactsForGuest pairs one HonestRiscvGuest with the verifier
// artifacts from proving it against main.zkc.
type HonestRiscvArtifactsForGuest struct {
	Guest     HonestRiscvGuest
	Artifacts HonestRiscvArtifacts
}

// compiledHonestRiscvSystem is main.zkc compiled exactly once: the shared
// wiop.System, its driver (already bound to sys via zkcdriver.NewZkCDriver,
// which declares sys's columns/constraints as a side effect — must not be
// reconstructed per guest), and the CompiledSystem every guest's
// HonestRiscvArtifacts reuses verbatim (main.zkc's circuit doesn't depend on
// the witness).
type compiledHonestRiscvSystem struct {
	sys            *wiop.System
	driver         *zkcdriver.ZkCDriver
	compiledSystem CompiledSystem
}

func compileHonestRiscvSystem() (compiledHonestRiscvSystem, error) {
	sourcePath, err := honestRiscvSourcePath()
	if err != nil {
		return compiledHonestRiscvSystem{}, err
	}

	binF, err := compileBinaryConstraints(sourcePath)
	if err != nil {
		return compiledHonestRiscvSystem{}, fmt.Errorf("compiling %s: %w", sourcePath, err)
	}
	compiledConstraints, err := binF.MarshalBinary()
	if err != nil {
		return compiledHonestRiscvSystem{}, fmt.Errorf("marshaling %s constraints: %w", sourcePath, err)
	}

	sys := wiop.NewSystemf("zkc-riscv-system")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	runCompilePipeline(sys)

	compiledSystem, err := BuildCompiledSystem(sys)
	if err != nil {
		return compiledHonestRiscvSystem{}, fmt.Errorf("BuildCompiledSystem: %w", err)
	}

	return compiledHonestRiscvSystem{sys: sys, driver: driver, compiledSystem: compiledSystem}, nil
}

func (c compiledHonestRiscvSystem) proveGuest(guest HonestRiscvGuest) (HonestRiscvArtifactsForGuest, error) {
	honest, err := proveHonestRiscvGuest(c.sys, c.driver, guest)
	if err != nil {
		return HonestRiscvArtifactsForGuest{}, fmt.Errorf("guest %q: %w", guest.Name, err)
	}
	verifyInput, err := proofserialization.Project(honest.Sys, honest.Proof, honest.Pub)
	if err != nil {
		return HonestRiscvArtifactsForGuest{}, fmt.Errorf("guest %q: proofserialization.Project: %w", guest.Name, err)
	}
	return HonestRiscvArtifactsForGuest{
		Guest: guest,
		Artifacts: HonestRiscvArtifacts{
			CompiledSystem: c.compiledSystem,
			VerifyInput:    verifyInput,
		},
	}, nil
}

// BuildAllHonestRiscvArtifacts compiles the real main.zkc entrypoint exactly
// once, then proves every guest in HonestRiscvGuests against that same
// compiled wiop.System, returning one HonestRiscvArtifacts per guest. Every
// artifacts' CompiledSystem describes the identical circuit (main.zkc is
// compiled once, independent of the witness); only the proof/public-input
// differ per guest. sys.Prove creates a fresh Runtime per call and never
// mutates sys itself, so reusing sys across guests is safe and produces fully
// independent proofs.
func BuildAllHonestRiscvArtifacts() ([]HonestRiscvArtifactsForGuest, error) {
	compiled, err := compileHonestRiscvSystem()
	if err != nil {
		return nil, err
	}

	results := make([]HonestRiscvArtifactsForGuest, len(HonestRiscvGuests))
	for i, guest := range HonestRiscvGuests {
		result, err := compiled.proveGuest(guest)
		if err != nil {
			return nil, err
		}
		results[i] = result
	}
	return results, nil
}

// proveHonestRiscvGuest proves guest.ELF against sys using driver, which must
// already have been constructed against sys (via zkcdriver.NewZkCDriver,
// which declares sys's columns/constraints as a side effect) before any
// guest is proved. driver itself carries no per-guest state — only
// AssignWithPreRead's arguments vary per call — so the same driver is reused
// across every guest.
func proveHonestRiscvGuest(sys *wiop.System, driver *zkcdriver.ZkCDriver, guest HonestRiscvGuest) (honestRiscvProof, error) {
	// The witness is a real halting guest ELF, not a synthetic verifier fixture.
	honestInputs, err := zkc_r5.PrepareInput(guest.ELF, nil)
	if err != nil {
		return honestRiscvProof{}, fmt.Errorf("PrepareInput: %w", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: honestInputs}

	proof, pub := sys.Prove(
		func(assignRt *wiop.Runtime) {
			driver.AssignWithPreRead(assignRt, inputs, koalafield.Octuplet{})
		},
		wiop.ProveOptions{CheckUnreducedQueries: true},
	)
	if err := sys.Verify(proof, pub); err != nil {
		return honestRiscvProof{}, fmt.Errorf("verifying proof: %w", err)
	}
	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		return honestRiscvProof{}, fmt.Errorf("AssertAllVerifierActionsHandled: %w", err)
	}

	return honestRiscvProof{
		Sys:   sys,
		Proof: proof,
		Pub:   pub,
	}, nil
}

func honestRiscvSourcePath() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("resolving codegen source path: runtime.Caller failed")
	}
	return filepath.Join(filepath.Dir(thisFile), "..", "..", "arithmetization", "src", "main", "riscv", "main.zkc"), nil
}

func compileBinaryConstraints(srcPath string) (binfile *constraints.BinaryFile[koalabear.Element], err error) {
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
	binfile = constraints.NewBinaryFile[koalabear.Element](nil, nil, zkcField, zkcCfg.GetMaxStaticHeight(), ir)
	return binfile, nil
}

func runCompilePipeline(sys *wiop.System) {
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
