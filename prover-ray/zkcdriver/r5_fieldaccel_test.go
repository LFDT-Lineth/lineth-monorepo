package zkcdriver_test

import (
	"bytes"
	"io/fs"
	"os"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
)

const liveArithSrcDir = "../../arithmetization/src"

// TestRisc5FieldAccel traces and proves a guest issuing field accelerator
// calls: the native mulmod_256
// (NonNative) and the unconstrained hint_field_op_256 go through the full
// pipeline.
func TestRisc5FieldAccel(t *testing.T) {
	binf, err := embedded.CompiledBinaryFile(embedded.WithRootFS(os.DirFS(liveArithSrcDir)))
	if err != nil {
		t.Fatalf("failed to compile R5 arithmetization: %v", err)
	}
	proveFieldOpELF(t, binf)
}

// TestRisc5FieldAccelUnreducedHint runs the same guest with a dishonest hint
// that returns its answer plus p whenever that still fits in 256 bits. The hint
// is unconstrained and only pinned modulo p by the product, so field_op must
// reject it (hint < p), or the output would not be canonical.
func TestRisc5FieldAccelUnreducedHint(t *testing.T) {
	const (
		fieldZkc = "main/lib/field/field.zkc"
		honest   = "    var div_res:u256 = m1\n"
		// 2^256 - p for secp256k1 Fp: below it, div_res + p fits in 256 bits.
		dishonest = "    var div_res:u256 = m1 < 0x1000003d1 ? m1 + p:m1\n"
	)
	liveFS := os.DirFS(liveArithSrcDir)
	overlay := fstest.MapFS{}
	err := fs.WalkDir(liveFS, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, err := fs.ReadFile(liveFS, path)
		overlay[path] = &fstest.MapFile{Data: data}
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	src := string(overlay[fieldZkc].Data)
	if strings.Count(src, honest) != 1 {
		t.Fatalf("%s: DIV hint not found", fieldZkc)
	}
	overlay[fieldZkc] = &fstest.MapFile{Data: []byte(strings.Replace(src, honest, dishonest, 1))}

	binf, err := embedded.CompiledBinaryFile(embedded.WithRootFS(overlay))
	if err != nil {
		t.Fatalf("failed to compile R5 arithmetization with a dishonest hint: %v", err)
	}
	guestELF, _ := minimalelf.FieldOpELF()
	inputsMap, err := predecoding.PrepareInputs(guestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	_, err = traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, true)
	if err == nil || !strings.Contains(err.Error(), "field_op: bad hint") {
		t.Fatalf("dishonest hint not rejected: %v", err)
	}
}

func proveFieldOpELF(t *testing.T, binf *zkcdriver.BinaryFile) {
	t.Helper()
	guestELF, wantOutput := minimalelf.FieldOpELF()
	inputsMap, err := predecoding.PrepareInputs(guestELF, nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}
	outputs, err := traceZkc(binf, vm.DEFAULT_TRACE_CONFIG, inputsMap, true)
	if err != nil {
		t.Fatalf("failed to trace: %v", err)
	}
	if got := outputs["guest_output"]; !bytes.Equal(got.Bytes(), wantOutput) {
		t.Fatalf("unexpected guest_output:\n got %x\nwant %x", got.Bytes(), wantOutput)
	}
	if err := runProveVerify(&zkcdriver.PreReadInputs{Inputs: inputsMap}, binf, proverCompilePipeline); err != nil {
		t.Fatalf("failed to prove/verify: %v", err)
	}
}
