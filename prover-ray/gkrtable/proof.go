package gkrtable

import (
	"fmt"
	"slices"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	gkrkoalabear "github.com/consensys/gnark/gkr/koalabear"
)

// proofAction is the prover action that proves T with GKR.
type proofAction struct{ *Table }

// proofVerifierAction is the verifier action that checks the GKR proof of T against the cells.
type proofVerifierAction struct{ *Table }

// wire returns the GKR wire of column c of T.
func (t *Table) wire(c int) int {
	switch {
	case c < blockLen:
		return int(t.compressor.Left[c])
	case c < 2*blockLen:
		return int(t.compressor.Right[c-blockLen])
	default:
		return int(t.compressor.Out[c-2*blockLen])
	}
}

// newHasher returns the hasher of the GKR transcript, which has the six coordinates of the seed written
// to it.
func newHasher(seed field.Ext) *poseidon2.Hasher {
	hasher := poseidon2.NewHasher()
	hasher.WriteElements(seed.AppendBase(nil)...)
	return hasher
}

// Run implements [wiop.ProverAction].
func (a *proofAction) Run(rt *wiop.Runtime) {
	column := func(c int) []koalabear.Element {
		return slices.Clone(rt.GetColumnAssignment(a.Columns[c]).Plain.AsBase())
	}
	left, right := make([][]koalabear.Element, blockLen), make([][]koalabear.Element, blockLen)
	for i := range blockLen {
		left[i], right[i] = column(i), column(blockLen+i)
	}

	hasher := newHasher(rt.GetCoinValue(a.Seed).AsExt())
	assignment := a.compressor.Assignment(left, right)
	proof, claims, err := gkrkoalabear.Prove(a.Circuit, a.Schedule, assignment, hasher)
	if err != nil {
		utils.Panic("gkrtable: GKR proof: %v", err)
	}

	var flat []field.Ext
	for _, e := range proof.Flatten() {
		flat = append(flat, *e)
	}
	rt.SetExtVector(a.Proof, flat)

	for c, cell := range a.Values {
		wireClaims := claims[a.wire(c)]
		if len(wireClaims) != 1 {
			utils.Panic("gkrtable: GKR proof: %d claims on column %d of T, expected 1", len(wireClaims), c)
		}
		rt.AssignCell(cell, field.ElemFromExt(wireClaims[0].Evaluation))
	}
	for j, cell := range a.Point {
		rt.AssignCell(cell, field.ElemFromExt(claims[a.wire(0)][0].EvaluationPoint[j]))
	}
}

// Check implements [wiop.VerifierAction]. It verifies the GKR proof with a transcript seeded with the seed,
// and checks that its claims are the values y at the point r* of the cells, one per column of T.
func (a *proofVerifierAction) Check(rt *wiop.Runtime) error {
	flat, _ := rt.GetExtVector(a.Proof)
	proof, err := gkrkoalabear.DeserializeProof(a.Circuit, a.Schedule, flat)
	if err != nil {
		return fmt.Errorf("gkrtable: GKR proof: %w", err)
	}

	hasher := newHasher(rt.GetCoinValue(a.Seed).AsExt())
	claims, err := gkrkoalabear.Verify(a.Circuit, a.Schedule, len(a.Point), proof, hasher)
	if err != nil {
		return fmt.Errorf("gkrtable: GKR proof: %w", err)
	}

	if len(claims) != NbColumns {
		return fmt.Errorf("gkrtable: GKR proof: claims on %d wires, expected %d", len(claims), NbColumns)
	}
	for c, cell := range a.Values {
		wireClaims := claims[a.wire(c)]
		if len(wireClaims) != 1 {
			return fmt.Errorf("gkrtable: GKR proof: %d claims on column %d of T, expected 1", len(wireClaims), c)
		}
		claim := wireClaims[0]
		if len(claim.EvaluationPoint) != len(a.Point) {
			return fmt.Errorf("gkrtable: GKR proof: the claim on column %d of T has a point of %d coordinates, expected %d",
				c, len(claim.EvaluationPoint), len(a.Point))
		}
		for j, coordinate := range a.Point {
			if claim.EvaluationPoint[j] != rt.GetCellValue(coordinate).AsExt() {
				return fmt.Errorf("gkrtable: GKR proof: the claim on column %d of T is at a point whose "+
					"coordinate %d is not r*", c, j)
			}
		}
		if claim.Evaluation != rt.GetCellValue(cell).AsExt() {
			return fmt.Errorf("gkrtable: GKR proof: the claim on column %d of T is not y", c)
		}
	}
	return nil
}
