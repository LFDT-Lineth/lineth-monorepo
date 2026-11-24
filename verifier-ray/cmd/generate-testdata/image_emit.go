package main

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/verifierray/codegen"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	pcscompiler "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
)

// fixtureProofImage returns the binary proof image (encoded for proofserialization.GuestBase)
// of the honest proof of the first fixture case that populates every path test/proof_image_test.zig
// mutates, with that case's index. The Zig test verifies the image against the case's system
// in verify.zig (image_case_index), so the image and the system come from the same compilation.
//
// The devmode system is too small to stand in: its proof has no FRI folding.
func fixtureProofImage(cases []fixtureCase, systems []codegen.CompiledSystem) (int, []byte, error) {
	for i, tc := range cases {
		if tc.sys == nil {
			continue
		}
		proof, pub := tc.sys.Prove(tc.assign)
		input, err := proofserialization.Project(tc.sys, proof, pub)
		if err != nil {
			return 0, nil, fmt.Errorf("projecting %s: %w", tc.name, err)
		}
		if !imageCoversTests(input) || !hasPresentBatch0Column(tc, systems[i], proof.Cells) {
			continue
		}
		image, err := proofserialization.Encode(input, proofserialization.GuestBase)
		if err != nil {
			return 0, nil, fmt.Errorf("encoding %s: %w", tc.name, err)
		}
		return i, image, nil
	}
	return 0, nil, fmt.Errorf("no fixture case populates every path of test/proof_image_test.zig")
}

// imageCoversTests reports whether every image path the validateImage test mutates is non-empty.
func imageCoversTests(in proofserialization.VerifyInput) bool {
	opening := in.Proof.PcsOpening
	fri := opening.FriProof
	return len(in.PublicInputs) > 0 && len(in.Proof.Rounds) > 0 &&
		len(fri.FinalPoly) > 0 &&
		len(fri.RunningQueries) > 0 && len(fri.RunningQueries[0]) > 0 &&
		len(opening.InputCaps) > 0 && len(opening.InputCaps[0].Nodes) > 0 &&
		len(fri.RoundCaps) > 0 && len(fri.RoundCaps[0].Aux) > 0 &&
		len(opening.InputQueries) > 0 && len(opening.InputQueries[0]) > 0 &&
		len(opening.InputQueries[0][0].Leaves) > 0
}

// hasPresentBatch0Column reports whether the honest manifest commits a column of batch 0, the
// column the manifest-flip test elides. Batch 0's manifest is one cell per column of the batch, in
// column order, from cell CellStart of round Round.
func hasPresentBatch0Column(tc fixtureCase, compiled codegen.CompiledSystem, cells map[wiop.ObjectID]field.Gen) bool {
	if compiled.Pcs == nil || len(compiled.Pcs.BatchManifests) == 0 || compiled.Pcs.BatchManifests[0] == nil {
		return false
	}
	bm := compiled.Pcs.BatchManifests[0]
	k := 0
	for _, col := range compiled.Pcs.Columns {
		if col.BatchIdx != 0 {
			continue
		}
		cell := tc.sys.Rounds[bm.Round].Cells[bm.CellStart+k]
		k++
		if cells[cell.Context.ID].AsBase().Uint64() == uint64(pcscompiler.ManifestPresent) {
			return true
		}
	}
	return false
}
