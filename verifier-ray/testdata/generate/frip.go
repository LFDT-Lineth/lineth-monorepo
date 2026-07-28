// FRI/PCS opening-proof fixture generator.
//
// Builds a REAL multi-round PCS opening proof with the prover-ray `fri` package
// (the source of truth), then serializes the OpeningProof + VerifyInputs into
// testdata/generated/frip.zig in the exact shapes verifier-ray's `pcs.verify`
// consumes. The Zig cross-check test replays this fixture and must accept it
// (and reject mutations). This is the authoritative byte-level gate: unlike the
// hand-written D=1 vector, every value here is produced by the Go prover.
package main

import (
	"bytes"
	"fmt"
	"math/big"
	"os"
	"path/filepath"

	fiatshamir "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fiatshamir"
	fri "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/polynomials"
	"github.com/consensys/gnark-crypto/field/koalabear"
)

// makeEncoders mirrors the fri test helper: one encoder per size 2^0..2^{n-1},
// each with the given inverse rate.
func makeEncoders(n, invRate int) []*fri.RSEncoder {
	encoders := make([]*fri.RSEncoder, n)
	for i := range n {
		enc := fri.NewEncoder(uint64(invRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}
	return encoders
}

// shiftedPoint replicates the (unexported) fri.PCS.shiftedPoint: the claim point
// zeta·omega_N^shift for a size-N = 2^sizeLog2 column.
func shiftedPoint(sizeLog2 uint8, shift int, zeta field.Ext) field.Ext {
	if sizeLog2 == 0 {
		return zeta // omega^0 = 1
	}
	size := uint64(1) << sizeLog2
	omega, err := koalabear.Generator(size)
	if err != nil {
		panic(err)
	}
	var rot koalabear.Element
	rot.Exp(omega, big.NewInt(int64(shift)))
	var out field.Ext
	out.MulByElement(&zeta, &rot)
	return out
}

// The fri.PCS opening flow, replayed with the exported API. Matches
// pcs_test.go's newPCSOpenVerifyFixture: NewParams(3,2,1), one batch with a
// size-2 (1 ext row) and a size-4 (2 ext rows) table; two fold rounds; 1 query.
func buildFripFixture() fripFixture {
	params, err := fri.NewParams(3, 2, 1)
	if err != nil {
		panic(err)
	}
	encoders := makeEncoders(int(params.LogPlainTextSize+1), 2) // sizes 2^0..2^2, invRate 2
	pcs, err := fri.NewPCS(params, encoders)
	if err != nil {
		panic(err)
	}

	// Deterministic witness (fixed values so the fixture is reproducible).
	witness := make(fri.Batch, 3)
	witness[1] = fri.SizedTable{Ext: [][]field.Ext{extVec(1000, 2)}}
	witness[2] = fri.SizedTable{Ext: [][]field.Ext{extVec(2000, 4), extVec(3000, 4)}}
	committed := pcs.Commit(witness)

	batchShifts := make(fri.BatchShifts, 3)
	batchShifts[1] = fri.SizedShifts{Ext: [][]int{{0}}}
	batchShifts[2] = fri.SizedShifts{Ext: [][]int{{0}, {1}}}
	shifts := []fri.BatchShifts{batchShifts}

	zeta := field.UintsToExt(19, 2, 3, 5, 7, 11)

	claimed := claimedValues(pcs, witness, batchShifts, zeta)
	if err := pcs.AddOpening(committed, zeta, batchShifts, claimed); err != nil {
		panic(err)
	}
	state, err := pcs.NewProverState()
	if err != nil {
		panic(err)
	}

	// Derive the fold challenges and query positions from a Fiat-Shamir
	// transcript, byte-for-byte as wiop/compilers/pcs `verify` does — so the
	// verifier-ray side can re-derive them from the emitted seed state rather
	// than trusting caller-supplied constants. A fixed seed makes the fixture
	// reproducible; the seed state (snapshot BEFORE the first squeeze) is emitted
	// so the Zig test can setState(seed) and derive identical challenges.
	fs := fiatshamir.NewFiatShamir()
	fs.UpdateExt(zeta) // arbitrary deterministic seed; the exact bytes don't matter
	fsSeedState := fs.State()

	var foldAlphas []field.Ext
	for state.HasNext() {
		alpha := fs.RandomFext()
		foldAlphas = append(foldAlphas, alpha)
		root := state.Fold(alpha)
		if state.HasNext() {
			fs.Update(root[:]...)
		}
	}
	fs.UpdateExt(state.FinalPoly...)
	codewordSize := 1 << params.LogCodewordSize
	queryPositions := fs.RandomManyIntegers(int(params.NumQueries), codewordSize)

	proof := pcs.Open(state, queryPositions)

	// Sanity: the proof must verify with the same inputs before we serialize it.
	in := fri.VerifyInputs{
		Roots:         []field.Octuplet{committed.Tree.Root()},
		Shapes:        []fri.Shape{witness.Shape()},
		Shifts:        shifts,
		ClaimedValues: []fri.BatchClaimedValues{claimed},
		Zeta:          zeta,
		Challenges:    fri.Challenges{FoldAlphas: foldAlphas, QueryPositions: queryPositions},
	}
	if err := pcs.Verify(in, proof); err != nil {
		panic(fmt.Sprintf("frip fixture does not self-verify: %v", err))
	}

	return fripFixture{
		params:         params,
		shape:          witness.Shape(),
		shifts:         batchShifts,
		root:           committed.Tree.Root(),
		zeta:           zeta,
		fsSeedState:    fsSeedState,
		foldAlphas:     foldAlphas,
		queryPositions: queryPositions,
		claimed:        claimed,
		proof:          proof,
	}
}

type fripFixture struct {
	params         fri.Params
	shape          fri.Shape
	shifts         fri.BatchShifts
	root           field.Octuplet
	zeta           field.Ext
	fsSeedState    field.Octuplet
	foldAlphas     []field.Ext
	queryPositions []int
	claimed        fri.BatchClaimedValues
	proof          fri.OpeningProof
}

// claimedValues mirrors pcs_test.go's claimedValuesForTest: evaluate every
// opened (size, row, shift) at zeta·omega_N^shift.
func claimedValues(pcs *fri.PCS, witness fri.Batch, shifts fri.BatchShifts, zeta field.Ext) fri.BatchClaimedValues {
	claimed := make(fri.BatchClaimedValues, len(shifts))
	for sizeLog2, sizedShifts := range shifts {
		sizedWitness := witness[sizeLog2]
		sized := fri.SizedClaimedValues{
			Base: make([][]field.Ext, len(sizedShifts.Base)),
			Ext:  make([][]field.Ext, len(sizedShifts.Ext)),
		}
		for rowIdx, rowShifts := range sizedShifts.Base {
			row := sizedWitness.Base[rowIdx]
			vals := make([]field.Ext, len(rowShifts))
			for i, shift := range rowShifts {
				pt := shiftedPoint(uint8(sizeLog2), shift, zeta)
				vals[i] = polynomials.EvalLagrange(field.VecFromBase(row), field.ElemFromExt(pt)).AsExt()
			}
			sized.Base[rowIdx] = vals
		}
		for rowIdx, rowShifts := range sizedShifts.Ext {
			row := sizedWitness.Ext[rowIdx]
			vals := make([]field.Ext, len(rowShifts))
			for i, shift := range rowShifts {
				pt := shiftedPoint(uint8(sizeLog2), shift, zeta)
				vals[i] = polynomials.EvalLagrange(field.VecFromExt(row), field.ElemFromExt(pt)).AsExt()
			}
			sized.Ext[rowIdx] = vals
		}
		claimed[sizeLog2] = sized
	}
	return claimed
}

func extVec(seed uint64, n int) []field.Ext {
	out := make([]field.Ext, n)
	for i := range out {
		s := seed + uint64(i)*7
		out[i] = field.UintsToExt(s, s+1, s+2, s+3, s+4, s+5)
	}
	return out
}

// ── serialization to frip.zig ────────────────────────────────────────────────

func writeFripFixture() error {
	fx := buildFripFixture()
	var out bytes.Buffer
	emitFrip(&out, fx)

	data := out.Bytes()
	if formatted, err := runZigFmt(data); err == nil {
		data = formatted
	}
	outputPath := filepath.Join("..", "generated", "frip.zig")
	return os.WriteFile(outputPath, data, 0o644)
}
