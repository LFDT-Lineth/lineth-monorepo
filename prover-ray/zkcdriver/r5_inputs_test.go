package zkcdriver_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"maps"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	"github.com/stretchr/testify/require"
)

// r5GuestInputs builds the zkc inputs that run the verifier-ray R5 guest
// against the committed proof image.
//
// The guest reads its input raw: loadR5Input in verifier-ray/src/main.zig
// casts the address of `_in_start` straight to a VerifyInput, with no length
// header and no validation. The proof image must therefore begin exactly at
// elfmapping.DefaultInputOrigin.
//
// That rules out predecoding.PrepareInputs, which is the length-prefixed
// convention: it writes an 8-byte little-endian length at `_in_start` and puts
// the payload at `_in_start+8`. Under that layout the guest reads the length
// header as the leading fields of VerifyInput, verification fails, and the
// guest exits 1 — which surfaces here only as "EXIT CODE = 1" from the
// tracer. Its own documentation points at the split used below for guests
// expecting raw input.
//
// This mirrors what verifier-ray's `make zkc-verify` does, where elf_to_json
// is handed `@testdata/riscv_proof_image.bin` and places those bytes directly
// at `_in_start`.
func r5GuestInputs() (map[string][]byte, error) {
	elfBytes, err := os.ReadFile(r5VerifierPath)
	if err != nil {
		return nil, fmt.Errorf("reading R5 verifier ELF: %w", err)
	}
	image, err := os.ReadFile(r5ProofImagePath)
	if err != nil {
		return nil, fmt.Errorf("reading R5 proof image: %w", err)
	}

	program, err := elfmapping.Load(bytes.NewReader(elfBytes))
	if err != nil {
		return nil, fmt.Errorf("loading R5 verifier ELF: %w", err)
	}
	// Predecoding supplies the instruction_base and decoded inputs the zkc
	// RISC-V module requires; without them tracing fails with "missing input".
	decoded, err := predecoding.Predecode(program)
	if err != nil {
		return nil, fmt.Errorf("predecoding R5 verifier ELF: %w", err)
	}
	// No WithLengthPrefix: the payload starts at the origin itself.
	imageBlobs, err := elfmapping.NewData(elfmapping.DefaultInputOrigin, image)
	if err != nil {
		return nil, fmt.Errorf("mapping R5 proof image: %w", err)
	}
	inputs, err := elfmapping.EncodeInputs(program, imageBlobs)
	if err != nil {
		return nil, fmt.Errorf("encoding R5 inputs: %w", err)
	}
	maps.Copy(inputs, decoded.EncodeInputs())
	return inputs, nil
}

// TestR5GuestInputsPlaceImageRaw guards the convention the guest depends on:
// the proof image must start exactly at _in_start, with no length header in
// front of it. A regression here does not fail loudly — the guest reads the
// header as struct fields, rejects the proof, and the tracer reports only
// "EXIT CODE = 1" — so it is asserted on the encoding itself, which is cheap
// to check and does not need the VM.
func TestR5GuestInputsPlaceImageRaw(t *testing.T) {
	inputs, err := r5GuestInputs()
	if errors.Is(err, os.ErrNotExist) {
		t.Skipf("R5 fixture unavailable: %v", err)
	}
	require.NoError(t, err)

	image, err := os.ReadFile(r5ProofImagePath)
	require.NoError(t, err)

	// blobs_offset_and_size is a table of big-endian (address, size) pairs.
	// The blob at the input origin must be the whole image.
	table := inputs["blobs_offset_and_size"]
	require.NotEmpty(t, table)

	var found bool
	for off := 0; off+16 <= len(table); off += 16 {
		addr := binary.BigEndian.Uint64(table[off : off+8])
		size := binary.BigEndian.Uint64(table[off+8 : off+16])
		if addr != elfmapping.DefaultInputOrigin {
			continue
		}
		found = true
		require.Equalf(t, uint64(len(image)), size,
			"the blob at _in_start (%#x) is %d bytes, not the %d-byte proof image: "+
				"an 8-byte blob here is the length prefix the guest cannot read",
			addr, size, len(image))
	}
	require.True(t, found, "no blob placed at _in_start (%#x)", elfmapping.DefaultInputOrigin)
}
