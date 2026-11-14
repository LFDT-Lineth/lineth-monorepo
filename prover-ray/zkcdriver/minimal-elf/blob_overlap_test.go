package minimalelf_test

import (
	"bytes"
	"encoding/binary"
	"errors"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/embedded"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
)

// TestBlobOverwritesRodata appends one blob over the .rodata of a
// [minimalelf.RodataPairELFs] guest, writing the R5 inputs by hand to skip the
// overlap check of [elfmapping.EncodeInputs]. Main.zkc loads the blobs in order
// and does not constrain their addresses, so the extra blob replaces .rodata in
// RAM, the trace satisfies every constraint, and the guest outputs the
// injected bytes.
func TestBlobOverwritesRodata(t *testing.T) {
	elfs, outputs := minimalelf.RodataPairELFs()
	inputs, err := predecoding.PrepareInputs(elfs[0], nil)
	if err != nil {
		t.Fatalf("failed to prepare inputs: %v", err)
	}

	injected := []byte("injected-over-rodata-32-bytes!!!")
	rodataAddr := uint64(minimalelf.DefaultSectionAddr + 64)

	entry, offsets, data := inputs[elfmapping.EntryPointAndBlobsCountInput],
		inputs[elfmapping.BlobsOffsetAndSizeInput], inputs[elfmapping.BlobsDataInput]
	entryBytes := entry.Bytes()
	count := binary.BigEndian.Uint32(entryBytes[8:])
	newEntry := binary.BigEndian.AppendUint32(append([]byte(nil), entryBytes[:8]...), count+1)
	newOffsets := binary.BigEndian.AppendUint64(append([]byte(nil), offsets.Bytes()...), rodataAddr)
	newOffsets = binary.BigEndian.AppendUint32(newOffsets, uint32(len(injected)))
	newData := append(append([]byte(nil), data.Bytes()...), injected...)
	inputs[elfmapping.EntryPointAndBlobsCountInput] = elfmapping.BitArrayFromBytes(newEntry...)
	inputs[elfmapping.BlobsOffsetAndSizeInput] = elfmapping.BitArrayFromBytes(newOffsets...)
	inputs[elfmapping.BlobsDataInput] = elfmapping.BitArrayFromBytes(newData...)

	binf, err := embedded.CompiledBinaryFile()
	if err != nil {
		t.Fatalf("failed to compile embedded R5 arithmetization: %v", err)
	}
	traced, tr, errs := binf.Trace(inputs, zkcCfg)
	if len(errs) > 0 {
		t.Fatalf("could not trace the binary file: %v", errors.Join(errs...))
	}
	if failures, errs := binf.Check(zkcCfg, tr.Unwrap()); len(failures) > 0 || len(errs) > 0 {
		t.Fatalf("the trace was rejected (%d constraint failures, %d errors)", len(failures), len(errs))
	}
	got := traced["guest_output"]
	if !bytes.Equal(got.Bytes(), injected) {
		t.Fatalf("guest_output = %q, want the injected %q (honest: %q)", got.Bytes(), injected, outputs[0])
	}
}
