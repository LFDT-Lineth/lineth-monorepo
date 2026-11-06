package gkrtable

import (
	"errors"
	"fmt"

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
		return int(t.circuit.Left[c])
	case c < 2*blockLen:
		return int(t.circuit.Right[c-blockLen])
	default:
		return int(t.circuit.Out[c-2*blockLen])
	}
}

// newHasher returns the hasher of the GKR transcript, which has σ's six coordinates written to it.
func newHasher(sigma field.Ext) *poseidon2.Hasher {
	hasher := poseidon2.NewHasher()
	hasher.WriteElements(sigma.AppendBase(nil)...)
	return hasher
}

// Run implements [wiop.ProverAction].
func (a *proofAction) Run(rt *wiop.Runtime) {
	// gkrkoalabear.Prove may use the assignment's columns for its working tables, and T's columns are
	// the prover's witness, so it gets copies.
	column := func(c int) []koalabear.Element {
		return append([]koalabear.Element(nil), rt.GetColumnAssignment(a.Columns[c]).Plain.AsBase()...)
	}
	left, right := make([][]koalabear.Element, blockLen), make([][]koalabear.Element, blockLen)
	for i := range blockLen {
		left[i], right[i] = column(i), column(blockLen+i)
	}

	hasher := newHasher(rt.GetCoinValue(a.Sigma).AsExt())
	assignment := a.circuit.Assignment(left, right)
	proof, claims, err := gkrkoalabear.Prove(a.circuit.Circuit, a.circuit.Schedule, assignment, hasher)
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
		if c == 0 {
			for j, coordinate := range a.Point {
				rt.AssignCell(coordinate, field.ElemFromExt(wireClaims[0].EvaluationPoint[j]))
			}
		}
	}
}

// Check implements [wiop.VerifierAction]. It verifies the GKR proof with a transcript seeded with σ,
// and checks that its claims are the values y at the point r* of the cells, one per column of T.
func (a *proofVerifierAction) Check(rt *wiop.Runtime) error {
	flat, ok := rt.GetExtVector(a.Proof)
	if !ok {
		return errors.New("gkrtable: GKR proof: the proof is missing")
	}
	proof, err := gkrkoalabear.DeserializeProof(a.circuit.Circuit, a.circuit.Schedule, flat)
	if err != nil {
		return fmt.Errorf("gkrtable: GKR proof: %w", err)
	}

	hasher := newHasher(rt.GetCoinValue(a.Sigma).AsExt())
	claims, err := gkrkoalabear.Verify(a.circuit.Circuit, a.circuit.Schedule, a.logSize, proof, hasher)
	if err != nil {
		return fmt.Errorf("gkrtable: GKR proof: %w", err)
	}

	if len(claims) != nbColumns {
		return fmt.Errorf("gkrtable: GKR proof: claims on %d wires, expected %d", len(claims), nbColumns)
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
