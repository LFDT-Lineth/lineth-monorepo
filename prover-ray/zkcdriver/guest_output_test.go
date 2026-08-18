package zkcdriver_test

import (
	"bytes"
	"testing"

	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// guestOutputBytes is the number of bytes testdata/guest_output.zkc writes to its
// public output memory.
const guestOutputBytes = 4

// newGuestOutputSystem compiles testdata/guest_output.zkc and builds a compiled
// system whose guest output is bound to numBytes public inputs, together with the
// inputs to prove it and the output bytes the tracer says the program wrote.
func newGuestOutputSystem(t *testing.T, numBytes int) (
	*wiop.System, *zkcdriver.ZkCDriver, *zkcdriver.PreReadInputs, []byte,
) {
	t.Helper()

	const zkcPath = "testdata/guest_output.zkc"

	binF, err := compileBinaryConstraints(zkcPath)
	require.NoError(t, err)

	inputs, outputs, err := parseTestCase(
		zkcTestCase{ZkcFilePath: zkcPath, InputStr: `{"data": "0x01020304"}`},
		binF,
	)
	require.NoError(t, err)

	compiled, err := binF.MarshalBinary()
	require.NoError(t, err)

	sys := wiop.NewSystemf("guest-output-test")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiled))

	risc5.RegisterGuestPublicOutputs(sys, numBytes)
	proverCompilePipeline(sys)

	return sys, driver, inputs, outputs["guest_output"]
}

// TestGuestPublicOutputs proves and verifies a program with a public write-once
// output memory, and checks that the bytes recovered from the constrained columns
// are the ones the program wrote, in order.
func TestGuestPublicOutputs(t *testing.T) {
	sys, driver, inputs, written := newGuestOutputSystem(t, guestOutputBytes)

	require.Equal(t, []byte{0x01, 0x02, 0x03, 0x04}, written,
		"the tracer must agree on what the program wrote")
	require.Len(t, sys.PublicInputs, guestOutputBytes)

	// The memory is found through the schema's public-output flag rather than by
	// name, and the program's `pub input` memory must not be taken for one.
	output := zkcdriver.PublicOutputs(sys)
	assert.Equal(t, "guest_output", output.Name)
	assert.NotNil(t, sys.LookupColumn(output.Address), "the address column must resolve")
	assert.NotNil(t, sys.LookupColumn(output.Data), "the data column must resolve")

	var got []byte
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignWithPreRead(rt, inputs, koalafield.Octuplet{})
		got = risc5.GetGuestPublicOutputs(rt, guestOutputBytes)
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	require.NoError(t, sys.Verify(proof, pub))
	assert.Equal(t, written, got, "the public inputs must carry the bytes the program wrote")
}

// TestGuestPublicOutputsWrongLength covers the two ways a configured output size
// that disagrees with the guest is caught. The first is the one that matters for
// soundness: without the length constraints a prover could grow the memory and
// choose which of its rows become public inputs.
func TestGuestPublicOutputsWrongLength(t *testing.T) {

	t.Run("the length constraints reject the proof", func(t *testing.T) {
		// Bind one byte fewer than the program writes, so the address on the last
		// row is one more than the length constraint pins it to.
		sys, driver, inputs, _ := newGuestOutputSystem(t, guestOutputBytes-1)

		assert.False(t, provesAndVerifies(t, sys, driver, inputs),
			"a guest output whose length disagrees with the memory must not verify")
	})

	t.Run("the prover reports the mismatch", func(t *testing.T) {
		sys, driver, inputs, _ := newGuestOutputSystem(t, guestOutputBytes)

		assert.PanicsWithValue(t,
			"risc5: GetGuestPublicOutputs: the guest wrote 4 output bytes but the configured output size is 5",
			func() {
				sys.Prove(func(rt *wiop.Runtime) {
					driver.AssignWithPreRead(rt, inputs, koalafield.Octuplet{})
					risc5.GetGuestPublicOutputs(rt, guestOutputBytes+1)
				})
			})
	})
}

// provesAndVerifies reports whether sys produces a proof that verifies. A
// violated constraint can surface either as a verification error or as a panic
// from the prover, so both count as a failure; the reason is logged so that a
// rejection for an unrelated reason does not pass for the one under test.
func provesAndVerifies(
	t *testing.T, sys *wiop.System, driver *zkcdriver.ZkCDriver, inputs *zkcdriver.PreReadInputs,
) (ok bool) {

	t.Helper()

	defer func() {
		if r := recover(); r != nil {
			t.Logf("rejected while proving: %v", r)
			ok = false
		}
	}()

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignWithPreRead(rt, inputs, koalafield.Octuplet{})
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	if err := sys.Verify(proof, pub); err != nil {
		t.Logf("rejected while verifying: %v", err)
		return false
	}

	return true
}
