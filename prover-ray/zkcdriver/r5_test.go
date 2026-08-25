package zkcdriver_test

import (
	"os"
	"path/filepath"
	"testing"

	zkc_r5 "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/backend/zkc-r5"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/constraints"
)

// TestSecp256k1AcceleratorsRisc5 executes the self-checking Zig guests through
// the RISC-V arithmetization. The ELFs are generated artifacts; build them with:
//
//	zig build --build-file arithmetization/src/test/zig/build.zig -Dpath=ecrecover/ecrecover_zkc
//	zig build --build-file arithmetization/src/test/zig/build.zig -Dpath=ecrecover/evm_ecrecover_zkc
func TestSecp256k1AcceleratorsRisc5(t *testing.T) {
	const zkcPath = "../../arithmetization/src/main/riscv/main.zkc"
	guestDir := "../../arithmetization/src/test/zig/zig-out/bin"
	guests := []string{"ecrecover_zkc", "evm_ecrecover_zkc"}
	for _, guest := range guests {
		if _, err := os.Stat(filepath.Join(guestDir, guest)); err != nil {
			t.Skipf("generated secp256k1 guest ELFs unavailable; build them first: %v", err)
		}
	}

	binf, err := compileBinaryConstraints(zkcPath)
	if err != nil {
		t.Fatalf("compiling RISC-V arithmetization: %v", err)
	}
	for _, guest := range guests {
		t.Run(guest, func(t *testing.T) {
			elfPath := filepath.Join(guestDir, guest)
			guestELF, err := os.ReadFile(elfPath)
			if err != nil {
				t.Fatalf("reading guest ELF %s: %v", elfPath, err)
			}
			inputs, err := zkc_r5.PrepareInput(guestELF, nil)
			if err != nil {
				t.Fatalf("preparing guest input: %v", err)
			}
			outputs, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputs, true)
			if err != nil {
				t.Fatalf("executing guest through RISC-V arithmetization: %v", err)
			}
			for name, output := range outputs {
				if len(output) != 0 {
					t.Fatalf("expected empty %s output, got: %x", name, output)
				}
			}
		})
	}
}

// This is a benchmark for the RISC-V arithmetization and not a test so that we
// don't crash the CI on every PR.
func BenchmarkRisc5Arithmetization(b *testing.B) {
	const zkcPath = "../../arithmetization/src/main/riscv/main.zkc"

	verifPath := "../../verifier-ray/zig-out/bin/verifier-ray"
	verifElf, err := os.ReadFile(verifPath)
	if err != nil {
		b.Skipf("skipping integration test: verifier ELF not found at %s (%v)", verifPath, err)
	}
	payload := []byte("foobar")
	inputsMap, err := zkc_r5.PrepareInput(verifElf, payload)
	if err != nil {
		b.Fatalf("failed to prepare inputs: %v", err)
	}

	b.Logf("compiling zkc source: %s", zkcPath)
	binf, err := compileBinaryConstraints(zkcPath)
	if err != nil {
		b.Fatalf("failed to compile zkc source: %v", err)
	}
	b.Logf("tracing zkc")
	outputs, err := traceZkc(binf, constraints.DEFAULT_TRACE_CONFIG, inputsMap, false)
	if err != nil {
		b.Fatalf("failed to parse test case: %v", err)
	}
	if len(outputs) != 0 {
		b.Fatalf("expected no outputs, got: %v", outputs)
	}
	driverInputs := &zkcdriver.PreReadInputs{
		Inputs: inputsMap,
	}
	b.Logf("prover/verify")
	if err := runProveVerify(driverInputs, binf, proverCompilePipeline); err != nil {
		b.Fatalf("failed to run test case: %v", err)
	}
}
