package minimalelf_test

import (
	"bytes"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/elfmapping"
	"github.com/LFDT-Lineth/lineth-monorepo/arithmetization/gopkg/predecoding"
	minimalelf "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/zkcdriver/minimal-elf"
)

// TestSameCodeGuests traces both guests of each pair, checks their trace and
// guest output, and checks that they have the same decoded program and
// instruction_base, but different outputs.
func TestSameCodeGuests(t *testing.T) {
	pairs := map[string]func() (elfs, outputs [2][]byte){
		"entry point": minimalelf.EntryPointPairELFs,
		"rodata":      minimalelf.RodataPairELFs,
	}
	for name, pair := range pairs {
		t.Run(name, func(t *testing.T) {
			elfs, outputs := pair()
			if bytes.Equal(elfs[0], elfs[1]) {
				t.Fatal("the two ELFs are the same")
			}
			if bytes.Equal(outputs[0], outputs[1]) {
				t.Fatalf("the two guests have the same output %q", outputs[0])
			}
			var programs [2]predecoding.DecodedProgram
			for i := range elfs {
				traceAndCheck(t, elfs[i], outputs[i])
				loaded, err := elfmapping.Load(bytes.NewReader(elfs[i]))
				if err != nil {
					t.Fatalf("failed to load ELF %d: %v", i, err)
				}
				program, err := predecoding.Predecode(loaded)
				if err != nil {
					t.Fatalf("failed to predecode ELF %d: %v", i, err)
				}
				programs[i] = program
			}
			if programs[0].InstructionBase != programs[1].InstructionBase {
				t.Fatalf("instruction_base %#x != %#x", programs[0].InstructionBase, programs[1].InstructionBase)
			}
			if !bytes.Equal(programs[0].Decoded, programs[1].Decoded) {
				t.Fatal("the decoded programs differ")
			}
		})
	}
}
