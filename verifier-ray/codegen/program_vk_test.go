package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
)

// newProgramVKRiscvSystem compiles the real RISC-V arithmetization with the
// RISCV-ZKC public inputs registered, as BuildAllInOneHonestRiscvArtifacts
// does, without proving anything.
func newProgramVKRiscvSystem(t *testing.T) *wiop.System {
	t.Helper()
	binF, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("embedded.CompiledBinaryFile() error = %v", err)
	}
	binBytes, err := binF.MarshalBinary()
	if err != nil {
		t.Fatalf("MarshalBinary() error = %v", err)
	}

	sys := wiop.NewSystemf("program-vk-codegen")
	sys.NewRound()
	zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{TraceRound: 1}, bytes.NewReader(binBytes))
	risc5.RegisterPublicInputs(sys)
	if err := compilers.CompileFull(sys,
		compilers.ConfigureMessageBus(messagebus.WithoutSharedRandomness()),
	); err != nil {
		t.Fatalf("compilers.CompileFull() error = %v", err)
	}
	return sys
}

func TestBuildProgramVKSystemExtractsVK(t *testing.T) {
	sys := newProgramVKRiscvSystem(t)

	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		t.Fatalf("AssertAllVerifierActionsHandled() error = %v", err)
	}

	vk, err := BuildProgramVKSystem(sys)
	if err != nil {
		t.Fatalf("BuildProgramVKSystem() error = %v", err)
	}

	if len(vk.VKRefs) != risc5.NumProgramVK {
		t.Fatalf("expected %d VK refs, got %d", risc5.NumProgramVK, len(vk.VKRefs))
	}
	if vk.CommitmentRound.RoundIndex != zkcdriver.ProgramRound {
		t.Fatalf("expected CommitmentRound.RoundIndex = %d, got %d", zkcdriver.ProgramRound, vk.CommitmentRound.RoundIndex)
	}
	// The program round holds the decoded columns, so it must be committed:
	// otherwise the VK would be the zero octuplet for every program.
	if !vk.CommitmentRound.HasCommitment {
		t.Fatalf("expected the program round %d to carry a commitment", vk.CommitmentRound.RoundIndex)
	}

	// Every VK ref must land strictly before the last wiop round, otherwise
	// protocol.replay would silently exclude it from ctx.rounds.
	lastSlot := len(sys.Rounds) - 1
	for i, ref := range vk.VKRefs {
		if ref.Round >= lastSlot {
			t.Fatalf("VK ref %d is in the last wiop round (slot %d)", i, ref.Round)
		}
	}

	var out bytes.Buffer
	if err := WriteProgramVKSystemZig(&out, 0, vk); err != nil {
		t.Fatalf("WriteProgramVKSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"const program_vk = @import",
		".commitment_round = .{ .round = 0, .has_commitment = true }",
		"system_0_program_vk_refs = [_]program_vk.ScalarRef{",
		"system_0_program_vk = program_vk.System{",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, got)
		}
	}
}

func TestBuildProgramVKSystemAbsentWithoutRegistration(t *testing.T) {
	sys := newSharedRandomnessMessageBusHandle(t)

	vk, err := BuildProgramVKSystem(sys)
	if err != nil {
		t.Fatalf("BuildProgramVKSystem() error = %v", err)
	}
	if vk.CommitmentRound != (CommitmentRoundCtx{}) || len(vk.VKRefs) != 0 {
		t.Fatalf("expected an empty ProgramVKSystem, got %+v", vk)
	}

	var out bytes.Buffer
	if err := WriteProgramVKSystemZig(&out, 0, vk); err != nil {
		t.Fatalf("WriteProgramVKSystemZig() error = %v", err)
	}
	if !strings.Contains(out.String(), "system_0_program_vk_refs = [_]program_vk.ScalarRef{\n};") {
		t.Fatalf("expected an empty VK-refs array literal:\n%s", out.String())
	}
}
