package zkcpipeline_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/zkcpipeline"
)

func TestRunProveWithRuntimeMatchesRunProve(t *testing.T) {
	binF, err := zkcpipeline.CompileBinaryConstraints("../testdata/no-memory.zkc")
	if err != nil {
		t.Fatalf("CompileBinaryConstraints() error = %v", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: map[string][]byte{}}

	sys, proof, pub, rt, err := zkcpipeline.RunProveWithRuntime(inputs, binF, zkcpipeline.RunCompilePipeline)
	if err != nil {
		t.Fatalf("RunProveWithRuntime() error = %v", err)
	}
	if sys == nil {
		t.Fatalf("RunProveWithRuntime() returned nil sys")
	}
	if rt == nil {
		t.Fatalf("RunProveWithRuntime() returned nil rt")
	}
	if len(pub) == 0 && len(proof.Cells) == 0 {
		t.Fatalf("RunProveWithRuntime() returned an empty proof")
	}

	// rt must reflect the SAME completed witness as proof: every cell in
	// proof.Cells must be readable off rt with an identical value.
	for id, want := range proof.Cells {
		cell := sys.LookupCell(id)
		if cell == nil {
			t.Fatalf("proof.Cells has cell id %v not found via sys.LookupCell", id)
		}
		got := rt.GetCellValue(cell)
		diff := got.Sub(want)
		if !diff.IsZero() {
			t.Fatalf("rt.GetCellValue(%v) = %v, want %v (from proof.Cells)", id, got, want)
		}
	}

	if err := sys.Verify(proof, pub); err != nil {
		t.Fatalf("sys.Verify() error = %v", err)
	}
}

func TestRunProveMatchesRunProveWithRuntime(t *testing.T) {
	binF, err := zkcpipeline.CompileBinaryConstraints("../testdata/no-memory.zkc")
	if err != nil {
		t.Fatalf("CompileBinaryConstraints() error = %v", err)
	}
	inputs := &zkcdriver.PreReadInputs{Inputs: map[string][]byte{}}

	sys, proof, pub, err := zkcpipeline.RunProve(inputs, binF, zkcpipeline.RunCompilePipeline)
	if err != nil {
		t.Fatalf("RunProve() error = %v", err)
	}
	if sys == nil {
		t.Fatalf("RunProve() returned nil sys")
	}
	if err := sys.Verify(proof, pub); err != nil {
		t.Fatalf("sys.Verify() error = %v", err)
	}
}
