package zkcdriver_test

import (
	"bytes"
	"encoding/binary"
	"testing"

	koalafield "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/risc5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// newGuestOutputSystem compiles the zkc program at zkcPath — which must declare a
// guest_output_hash memory and be fed its input bytes as `data` — and builds a
// compiled system whose guest output hash is bound as public inputs, together with
// the inputs to prove it and the hash bytes the tracer says the program wrote.
func newGuestOutputSystem(t *testing.T, zkcPath, inputHex string) (
	*wiop.System, *zkcdriver.ZkCDriver, *zkcdriver.PreReadInputs, []byte,
) {
	t.Helper()

	binF, err := compileBinaryConstraints(zkcPath)
	require.NoError(t, err)

	inputs, outputs, err := parseTestCase(
		zkcTestCase{ZkcFilePath: zkcPath, InputStr: `{"data": "` + inputHex + `"}`},
		binF,
		!testing.Short(),
	)
	require.NoError(t, err)

	compiled, err := binF.MarshalBinary()
	require.NoError(t, err)

	sys := wiop.NewSystemf("guest-output-test")
	sys.NewRound()
	driver := zkcdriver.NewZkCDriver(sys, zkcdriver.Settings{}, bytes.NewReader(compiled))

	risc5.RegisterGuestPublicOutputs(sys)
	proverCompilePipeline(sys)

	return sys, driver, inputs, outputs["guest_output_hash"]
}

// TestGuestPublicOutputs proves and verifies a program with a public write-once
// hash memory, and checks that the bytes recovered from the constrained columns
// are the ones the program wrote, in order.
func TestGuestPublicOutputs(t *testing.T) {
	sys, driver, inputs, written := newGuestOutputSystem(t,
		"testdata/guest_output_hash.zkc", "0x0102030405060708")

	// The eight u32 words the fixture writes, as the tracer serializes them:
	// big-endian, so 0x00010002 is 00 01 00 02.
	wantWritten := make([]byte, 0, 32)
	for i := byte(1); i <= 16; i++ {
		wantWritten = append(wantWritten, 0x00, i)
	}
	require.Equal(t, wantWritten, written,
		"the tracer must agree on what the program wrote")
	require.Len(t, sys.PublicInputs, risc5.GuestPublicOutputCells(sys))

	// The memory is found by name among the schema's public outputs, so the
	// `pub output guest_output` the fixture also declares must not be taken for it.
	output := zkcdriver.PublicOutputs(sys)
	assert.Equal(t, "guest_output_hash", output.Name)
	assert.NotNil(t, sys.LookupColumn(output.Address), "the address column must resolve")
	require.Len(t, output.Data, 2, "a u32 word is split across two 16-bit limbs")
	for i, id := range output.Data {
		assert.NotNil(t, sys.LookupColumn(id), "limb %d must resolve", i)
	}

	traces := driver.TraceZkcInputs(inputs)
	if len(traces) > 1 {
		t.Fatalf("the test fixture is expected to only use a single public inputs")
	}

	var got []koalafield.Element
	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, traces[0], koalafield.Octuplet{})
		got = risc5.GetGuestPublicOutputs(rt)
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	require.NoError(t, sys.Verify(proof, pub))

	// Each output element is one 16-bit limb, and the limbs run most-significant
	// first within a word, matching the order of the tracer's big-endian bytes. So
	// element k is the bytes the tracer saw at written[2k:2k+2].
	want := make([]koalafield.Element, len(written)/2)
	for k := range want {
		want[k].SetUint64(uint64(binary.BigEndian.Uint16(written[2*k:])))
	}
	assert.Equal(t, want, got, "the public inputs must carry the values the program wrote")
}

// TestGuestPublicOutputsWrongLength covers the two ways a guest whose output
// length disagrees with [risc5.NumGuestPublicOutputs] is caught. The guest here
// writes seven hash words instead of eight, so the last address of the output
// memory is one less than the length constraint pins it to. The first check is the
// one that matters for soundness.
func TestGuestPublicOutputsWrongLength(t *testing.T) {

	const (
		zkcPath  = "testdata/guest_output_wrong_length.zkc"
		inputHex = "0x010203040506070809"
	)

	t.Run("the length constraints reject the proof", func(t *testing.T) {
		sys, driver, inputs, _ := newGuestOutputSystem(t, zkcPath, inputHex)

		assert.False(t, provesAndVerifiesFirstShardOnly(t, sys, driver, inputs),
			"a guest output whose length disagrees with the memory must not verify")
	})

	t.Run("the prover reports the mismatch", func(t *testing.T) {
		sys, driver, inputs, _ := newGuestOutputSystem(t, zkcPath, inputHex)

		traces := driver.TraceZkcInputs(inputs)
		if len(traces) > 1 {
			t.Fatalf("the test fixture is expected to only use a single public inputs")
		}

		assert.PanicsWithValue(t,
			"risc5: GetGuestPublicOutputs: the guest wrote 7 outputs but the expected output size is 8",
			func() {
				sys.Prove(func(rt *wiop.Runtime) {
					driver.AssignTraceShard(rt, traces[0], koalafield.Octuplet{})
					risc5.GetGuestPublicOutputs(rt)
				})
			})
	})
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

	traces := driver.TraceZkcInputs(inputs)
	if len(traces) > 1 {
		t.Fatalf("the test fixture is expected to only use a single public inputs")
	}

	proof, pub := sys.Prove(func(rt *wiop.Runtime) {
		driver.AssignTraceShard(rt, traces[0], koalafield.Octuplet{})
	}, wiop.ProveOptions{CheckUnreducedQueries: true})

	if err := sys.Verify(proof, pub); err != nil {
		t.Logf("rejected while verifying: %v", err)
		return false
	}

	return true
}
