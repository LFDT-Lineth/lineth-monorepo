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

type honestRiscvProof struct {
	Sys   *wiop.System
	Proof wiop.Proof
	Pub   wiop.PublicInput
}

// BuildHonestRiscvArtifacts exercises the real main.zkc -> wiop.System -> real
// proof path and returns the verifier-facing artifacts derived from it.
func BuildHonestRiscvArtifacts() (HonestRiscvArtifacts, error) {
	honest, err := buildHonestRiscvProof()
	if err != nil {
		return HonestRiscvArtifacts{}, err
	}

	compiledSystem, err := BuildCompiledSystem(honest.Sys)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("BuildCompiledSystem: %w", err)
	}
	verifyInput, err := proofserialization.Project(honest.Sys, honest.Proof, honest.Pub)
	if err != nil {
		return HonestRiscvArtifacts{}, fmt.Errorf("proofserialization.Project: %w", err)
	}

	return HonestRiscvArtifacts{
		CompiledSystem: compiledSystem,
		VerifyInput:    verifyInput,
	}, nil
}

func buildHonestRiscvProof() (honestRiscvProof, error) {
	sourcePath, err := honestRiscvSourcePath()
	if err != nil {
		return honestRiscvProof{}, err
	}

	binF, err := compileBinaryConstraints(sourcePath)
	if err != nil {
		return honestRiscvProof{}, fmt.Errorf("compiling %s: %w", sourcePath, err)
	}
	compiledConstraints, err := binF.MarshalBinary()
	if err != nil {
		return honestRiscvProof{}, fmt.Errorf("marshaling %s constraints: %w", sourcePath, err)
	}

	// The witness is a real halting guest ELF, not a synthetic verifier fixture.
	honestInputs, err := zkc_r5.PrepareInput(zkc_r5.ExitZeroGuestELF, nil)
	if err != nil {
		return honestRiscvProof{}, fmt.Errorf("PrepareInput(minimal exit guest): %w", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: honestInputs}

	sys := wiop.NewSystemf("zkc-riscv-system")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiledConstraints))
	runCompilePipeline(sys)

	proof, pub := sys.Prove(
		func(assignRt *wiop.Runtime) {
			driver.AssignWithPreRead(assignRt, inputs, koalafield.Octuplet{})
		},
		wiop.ProveOptions{CheckUnreducedQueries: true},
	)
	if err := sys.Verify(proof, pub); err != nil {
		return honestRiscvProof{}, fmt.Errorf("verifying %s proof: %w", sourcePath, err)
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
