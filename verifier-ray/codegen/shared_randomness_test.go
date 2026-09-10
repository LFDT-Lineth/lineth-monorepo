package codegen

import (
	"bytes"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
)

// newSharedRandomnessMessageBusHandle builds the same size-4 Send/Receive
// message-bus handle as newSingleMessageBusHandle, but compiled with
// messagebus.CompileOptions.SharedRandomness set — which additionally
// registers a messagebus.SharedRandomnessContributionChecker verifier action
// on the message-bus coin round. pcs.Compile is run too (mirroring the real
// compileFullPipeline order in testdata/generate/main.go) so the committed
// round(s) preceding the coin round carry realistic HasCommitment data for
// BuildSharedRandomnessSystem to read.
func newSharedRandomnessMessageBusHandle(t *testing.T) *wiop.System {
	t.Helper()
	sys := wiop.NewSystemf("mb-sr-codegen")
	r0 := sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)

	sys.NewMessageBusSend(sys.Context.Childf("send"), "shard", "route", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(sys.Context.Childf("recv"), "shard", "route", wiop.NewTable(colB.View()))

	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)
	global.Compile(sys)
	pcscompiler.Compile(sys)
	return sys
}


func TestBuildSharedRandomnessSystemExtractsContribution(t *testing.T) {
	sys := newSharedRandomnessMessageBusHandle(t)

	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		t.Fatalf("AssertAllVerifierActionsHandled() error = %v", err)
	}

	sr, err := BuildSharedRandomnessSystem(sys)
	if err != nil {
		t.Fatalf("BuildSharedRandomnessSystem() error = %v", err)
	}

	if len(sr.ContributionRefs) != messagebus.NumSharedRandomnessContribution {
		t.Fatalf("expected %d contribution refs, got %d", messagebus.NumSharedRandomnessContribution, len(sr.ContributionRefs))
	}

	// The coin round is round 1 (alpha/beta are declared there by
	// registerSharedRandomness). Verify the codegen faithfully mirrors the
	// actual wiop.Round state rather than hard-coding a value.
	if sr.CommitmentRound.RoundIndex != 1 {
		t.Fatalf("expected CommitmentRound.RoundIndex = 1, got %d", sr.CommitmentRound.RoundIndex)
	}
	if got, want := sr.CommitmentRound.HasCommitment, sys.Rounds[sr.CommitmentRound.RoundIndex].HasCommitment; got != want {
		t.Fatalf("CommitmentRound.HasCommitment = %v, want %v (wiop.Round.HasCommitment for round %d)",
			got, want, sr.CommitmentRound.RoundIndex)
	}

	// Every contribution ref must land strictly before the last wiop round
	// (global.Compile appends quotient+eval rounds after the message-bus result
	// round), otherwise protocol.replay would silently exclude it from ctx.rounds.
	lastSlot := len(sys.Rounds) - 1
	for i, ref := range sr.ContributionRefs {
		if ref.Round >= lastSlot {
			t.Fatalf("contribution ref %d is in the last wiop round (slot %d)", i, ref.Round)
		}
	}
}

func TestBuildSharedRandomnessSystemAbsentWithoutOption(t *testing.T) {
	sys := wiop.NewSystemf("mb-codegen-no-sr")
	r0 := sys.NewRound()
	sys.NewRound()
	sys.NewRound()
	modA := sys.NewSizedModule(sys.Context.Childf("modA"), 4, wiop.PaddingDirectionNone)
	modB := sys.NewSizedModule(sys.Context.Childf("modB"), 4, wiop.PaddingDirectionNone)
	colA := modA.NewColumn(sys.Context.Childf("A"), r0)
	colB := modB.NewColumn(sys.Context.Childf("B"), r0)
	sys.NewMessageBusSend(sys.Context.Childf("send"), "shard", "route", wiop.NewTable(colA.View()))
	sys.NewMessageBusReceive(sys.Context.Childf("recv"), "shard", "route", wiop.NewTable(colB.View()))

	messagebus.Compile(sys) // no SharedRandomness option
	grandproduct.Compile(sys)
	global.Compile(sys)

	if err := AssertAllVerifierActionsHandled(sys); err != nil {
		t.Fatalf("AssertAllVerifierActionsHandled() error = %v", err)
	}

	sr, err := BuildSharedRandomnessSystem(sys)
	if err != nil {
		t.Fatalf("BuildSharedRandomnessSystem() error = %v", err)
	}
	if sr.CommitmentRound != (CommitmentRoundCtx{}) || len(sr.ContributionRefs) != 0 {
		t.Fatalf("expected an empty SharedRandomnessSystem without the option, got %+v", sr)
	}
}

func TestWriteSharedRandomnessSystemZigRendersContribution(t *testing.T) {
	sys := newSharedRandomnessMessageBusHandle(t)
	sr, err := BuildSharedRandomnessSystem(sys)
	if err != nil {
		t.Fatalf("BuildSharedRandomnessSystem() error = %v", err)
	}

	var out bytes.Buffer
	if err := WriteSharedRandomnessSystemZig(&out, 0, sr); err != nil {
		t.Fatalf("WriteSharedRandomnessSystemZig() error = %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"const shared_randomness = @import",
		// coin round is round 1, carries no columns so has_commitment = false
		".commitment_round = .{ .round = 1, .has_commitment = false }",
		"system_0_shared_randomness_contribution_refs = [_]shared_randomness.ScalarRef{",
		"system_0_shared_randomness = shared_randomness.System{",
	} {
		if !strings.Contains(got, want) {
			t.Fatalf("generated Zig missing %q:\n%s", want, got)
		}
	}
}

func TestWriteSharedRandomnessSystemZigRendersEmptySystem(t *testing.T) {
	var out bytes.Buffer
	if err := WriteSharedRandomnessSystemZig(&out, 0, SharedRandomnessSystem{}); err != nil {
		t.Fatalf("WriteSharedRandomnessSystemZig() error = %v", err)
	}
	got := out.String()
	if !strings.Contains(got, "system_0_shared_randomness_contribution_refs = [_]shared_randomness.ScalarRef{\n};") {
		t.Fatalf("expected an empty contribution-refs array literal:\n%s", got)
	}
	// An absent system must not name a commitment round to hash: the zero
	// Round leaves has_commitment false, and verify() returns early anyway
	// because contribution_refs is empty.
	if !strings.Contains(got, ".commitment_round = .{ .round = 0, .has_commitment = false }") {
		t.Fatalf("expected a zero commitment round:\n%s", got)
	}
}
