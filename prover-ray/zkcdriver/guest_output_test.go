package zkcdriver_test

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGuestOutputSystem builds a system over the embedded R5 arithmetization whose
// guest output is bound as public inputs, together with the inputs to prove the
// given guest ELF and the guest output bytes the tracer says the guest wrote.
// Each extraActions entry is registered in the trace round after the guest
// output binding, and before any compiler pass.
func newGuestOutputSystem(t *testing.T, elf []byte, extraActions ...wiop.ProverAction) (
	*wiop.System, *zkcdriver.ZkCDriver, *zkcdriver.PreReadInputs, []byte,
) {
	t.Helper()

	binF, err := embedded.CompiledBinaryFile()
	require.NoError(t, err)

	inputs, err := predecoding.PrepareInputs(elf, nil)
	require.NoError(t, err)

	outputs, err := traceZkc(binF, vm.DEFAULT_TRACE_CONFIG, inputs, false)
	require.NoError(t, err)

	compiled, err := binF.MarshalBinary()
	require.NoError(t, err)

	sys := wiop.NewSystemf("guest-output-test")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiled))

	risc5.RegisterGuestPublicOutputs(sys)
	for _, a := range extraActions {
		sys.Rounds[0].RegisterAction(a)
	}
	proverCompilePipeline(sys)
	guestOutput := outputs["guest_output"]
	return sys, driver, &zkcdriver.PreReadInputs{Inputs: inputs}, guestOutput.Bytes()
}

// TestGuestPublicOutputs proves and verifies the all-in-one guest, which writes
// more bytes than are bound, and checks that the public inputs carry the first
// [risc5.NumGuestPublicOutputs] bytes it wrote, in order.
func TestGuestPublicOutputs(t *testing.T) {
	elf, wantWritten := minimalelf.AllInOneElfProgram()
	sys, driver, inputs, written := newGuestOutputSystem(t, elf)

	require.Equal(t, wantWritten, written, "the tracer must agree on what the guest wrote")
	require.Greater(t, len(written), risc5.NumGuestPublicOutputs,
		"the guest must write more bytes than are bound, so that only a prefix is")

	output := zkcdriver.PublicOutputs(sys)
	assert.Equal(t, "guest_output", output.Name)
	require.Len(t, output.Address, 2, "a u32 address is split across two 16-bit limbs")
	for i, id := range append(output.Address, output.Data, output.AccessBit) {
		assert.NotNil(t, sys.LookupColumn(id), "column %d must resolve", i)
	}

	shard := traceSingleShard(t, driver, inputs)
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, shard, koalafield.Octuplet{})
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	require.NoError(t, sys.Verify(proof, pub))

	want := make([]koalafield.Element, risc5.NumGuestPublicOutputs)
	for k := range want {
		want[k].SetUint64(uint64(written[k]))
	}
	assert.Equal(t, want, risc5.GetGuestPublicOutputs(sys, pub),
		"the public inputs must carry the first bytes the guest wrote")
}

// TestGuestPublicOutputsRejectsForgedByte checks that the bound bytes are the
// ones of the guest output memory: a prover changing one of them after the
// honest assignment must not produce a proof that verifies.
func TestGuestPublicOutputsRejectsForgedByte(t *testing.T) {
	elf, _ := minimalelf.AllInOneElfProgram()
	sys, driver, inputs, _ := newGuestOutputSystem(t, elf, forgeGuestOutputByte{})

	assert.False(t, provesAndVerifiesFirstShardOnly(t, sys, driver, inputs),
		"a public output byte that differs from the guest output must not verify")
}

// TestGuestPublicOutputsTooShort checks that the prover panics on a guest
// writing fewer than [risc5.NumGuestPublicOutputs] bytes, which no assignment
// could bind, down to a guest writing nothing and up to one byte short, and that
// a guest writing exactly [risc5.NumGuestPublicOutputs] bytes is bound.
func TestGuestPublicOutputsTooShort(t *testing.T) {
	for _, n := range []int{0, risc5.NumGuestPublicOutputs - 1, risc5.NumGuestPublicOutputs} {
		t.Run(fmt.Sprintf("n=%d", n), func(t *testing.T) {
			elf, wantWritten := minimalelf.OutputELF(n)

			var (
				sys, driver, inputs, written = newGuestOutputSystem(t, elf)
				shard                        = traceSingleShard(t, driver, inputs)
			)
			require.Equal(t, wantWritten, written, "the tracer must agree on what the guest wrote")

			prove := func() (wiop.Proof, wiop.PublicInput) {
				return sys.Prove(func(rt *wiop.Runtime) {
					driver.AssignTraceShard(rt, shard, koalafield.Octuplet{})
				})
			}

			if n < risc5.NumGuestPublicOutputs {
				assert.PanicsWithValue(t,
					fmt.Sprintf("risc5: assignGuestOutputBytes: the guest wrote %d output bytes "+
						"but the first %d are bound as public inputs", n, risc5.NumGuestPublicOutputs),
					func() { prove() })
				return
			}

			proof, pub := prove()
			require.NoError(t, sys.Verify(proof, pub))
			want := make([]koalafield.Element, n)
			for k := range want {
				want[k].SetUint64(uint64(wantWritten[k]))
			}
			assert.Equal(t, want, risc5.GetGuestPublicOutputs(sys, pub))
		})
	}
}

// forgeGuestOutputByte flips the low bit of the first byte bound by
// [risc5.RegisterGuestPublicOutputs], once it has been assigned honestly.
type forgeGuestOutputByte struct{}

func (forgeGuestOutputByte) Run(rt *wiop.Runtime) {
	for _, mod := range rt.System.Modules {
		if !strings.Contains(mod.Context.Path(), "guest-public-outputs") {
			continue
		}
		for _, col := range mod.Columns {
			if col.Round() == &rt.System.PrecomputedRound.Round {
				continue
			}
			forged := rt.GetColumnAssignment(col).Plain.AsBase()
			forged = append([]koalafield.Element(nil), forged...)
			forged[0].SetUint64(forged[0].Uint64() ^ 1)
			rt.OverrideColumn(col, &wiop.ConcreteVector{Plain: koalafield.VecFromBase(forged)})
			return
		}
	}
	panic("forgeGuestOutputByte: no guest public output column found")
}

// provesAndVerifiesFirstShardOnly reports whether sys produces a proof that verifies. A
// violated constraint can surface either as a verification error or as a panic
// from the prover, so both count as a failure; the reason is logged so that a
// rejection for an unrelated reason does not pass for the one under test.
func provesAndVerifiesFirstShardOnly(
	t *testing.T, sys *wiop.System, driver *zkcdriver.ZkCDriver, inputs *zkcdriver.PreReadInputs,
) (ok bool) {

	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("rejected while proving: %v", r)
			ok = false
		}
	}()

	// Trace inputs to produce a single shard
	shard := traceSingleShard(t, driver, inputs)
	//
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, shard, koalafield.Octuplet{})
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	if err := sys.Verify(proof, pub); err != nil {
		t.Logf("rejected while verifying: %v", err)
		return false
	}

	return true
}

// Trace the given inputs using the given driver, whilst expecting everything to
// fit into a single shard and, if not, then this test fails.
func traceSingleShard(tb testing.TB, driver *zkcdriver.ZkCDriver, inputs *zkcdriver.PreReadInputs) zkcdriver.Shard {
	// Mark function as test helper
	tb.Helper()
	//
	var lazyTrace = driver.TraceZkcInputs(inputs)
	// Sanity check only a single shard
	if lazyTrace.Len() != 1 {
		tb.Fatalf("the test fixture is expected to only use a single public inputs")
	}
	// Trace the one shard
	shard, errs := lazyTrace.Get(0)
	// Sanity check for errors
	if len(errs) > 0 {
		tb.Fatalf("tracing failed %v", errors.Join(errs...))
	}
	// Done
	return shard.Unwrap()
}
