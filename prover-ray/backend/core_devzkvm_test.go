package backend

import (
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/zkc/pkg/util/collection/bit"
	"github.com/LFDT-Lineth/zkc/pkg/zkc/vm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// specFrame builds keccak256(schema || pi) || schema || pi, the spec layout.
func specFrame(schema uint16, pi []byte) []byte {
	framed := binary.BigEndian.AppendUint16(nil, schema)
	framed = append(framed, pi...)
	return append(keccak256(framed), framed...)
}

// samplePublicInputs stands in for SSZ(public inputs); the frame check does not
// read them.
var samplePublicInputs = []byte("public inputs")

func TestClassifyGuestOutput(t *testing.T) {
	valid := specFrame(guestOutputSchemaID, samplePublicInputs)

	t.Run("valid output is returned", func(t *testing.T) {
		got, err := classifyGuestOutput(guestOutputs(valid), nil)
		require.NoError(t, err)
		assert.Equal(t, bitsFromBytes(valid...), got)
	})

	t.Run("guest rejection is distinguished from a VM bug", func(t *testing.T) {
		errs := []error{&vm.Failure{Message: "block 42 invalid: bad state root"}}
		_, err := classifyGuestOutput(nil, errs)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "guest rejected the block")
		assert.Contains(t, err.Error(), "bad state root")
	})

	t.Run("internal VM error is reported as such", func(t *testing.T) {
		_, err := classifyGuestOutput(nil, []error{fmt.Errorf("interpreter panic")})
		require.Error(t, err)
		assert.Contains(t, err.Error(), "guest VM execution failed")
	})

	t.Run("missing output means the block was rejected", func(t *testing.T) {
		_, err := classifyGuestOutput(map[string]bit.Array{"other": bitsFromBytes(1)}, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "no guest_output")
	})

	t.Run("a single blob in guest_output is not the split frame", func(t *testing.T) {
		out := map[string]bit.Array{guestOutputMemory: bitsFromBytes(valid...)}
		_, err := classifyGuestOutput(out, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), fmt.Sprintf("guest wrote %d bytes to guest_output, want the 32-byte hash", len(valid)))
	})

	t.Run("hash without guest_log is reported", func(t *testing.T) {
		out := map[string]bit.Array{guestOutputMemory: bitsFromBytes(valid[:guestOutputHashSize]...)}
		_, err := classifyGuestOutput(out, nil)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "guest wrote the hash but no guest_log")
	})

	badHash := specFrame(guestOutputSchemaID, samplePublicInputs)
	badHash[0] ^= 0xff

	rejected := map[string]struct {
		out     []byte
		wantErr string
	}{
		"hash mismatch":   {badHash, "hash does not match"},
		"wrong schema id": {specFrame(0x0002, samplePublicInputs), "schema id is 0x0002, want 0x0003"},
		"too short":       {make([]byte, 33), "shorter than the schema id"},
	}
	for name, tc := range rejected {
		t.Run(name, func(t *testing.T) {
			_, err := classifyGuestOutput(guestOutputs(tc.out), nil)
			require.Error(t, err)
			assert.Contains(t, err.Error(), "guest_output does not follow the spec")
			assert.Contains(t, err.Error(), tc.wantErr)
		})
	}
}

func bitsFromBytes(bytes ...byte) bit.Array {
	return bit.NewArray(bytes, uint(len(bytes)*8), bit.BigEndian)
}

// guestOutputs splits frame as the guest writes it: the first
// guestOutputHashSize bytes to guest_output and the rest to guest_log.
func guestOutputs(frame []byte) map[string]bit.Array {
	split := min(len(frame), guestOutputHashSize)
	return map[string]bit.Array{
		guestOutputMemory: bitsFromBytes(frame[:split]...),
		guestLogMemory:    bitsFromBytes(frame[split:]...),
	}
}
