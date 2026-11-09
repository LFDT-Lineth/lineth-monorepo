// Package gkrtable defines a table of Poseidon2 compressions whose rows are proven by GKR.
//
// The table T has 24 columns: the eight elements of the left input, the eight of the right input and
// the eight of the output of the compression C(left, right) = P(left ‖ right)[8..16] + right, the
// columns of a [gkrverifier.CompressionTable]. In the round after T's, the verifier draws a coin σ,
// the seed of the GKR transcript, and the prover reduces the claim that every row of T is a compression
// to claims on the multilinear extensions of T's 24 columns at one point r* of E₆ⁿ, with n the log size
// of T: the values y₀, …, y₂₃, in the order of T's columns. The round has the cells that Ray's
// transcript absorbs: r* and y. The GKR proof itself travels in an [wiop.ExtVector], outside the
// transcript.
//
// The claims are about T's columns, which the GKR proof does not read: they are checked against T in
// the next rounds. The prover commits the column E with E[i] = eq(r*, i) in T's module, which
// constraints pin down from r*, and the verifier draws a coin λ. A log-derivative sum then makes the
// result Σᵢ (Σ_c λᶜ C_c[i]) E[i], which the verifier checks against Σ_c λᶜ y_c.
package gkrtable

import (
	"fmt"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/gkrverifier"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
	"github.com/consensys/gnark/constraint"
	"github.com/consensys/gnark/gkr"
	gkrposeidon2 "github.com/consensys/gnark/gkr/permutation/gkr-poseidon2/koalabear"
)

const (
	blockLen = poseidon2.BlockLen
	// NbColumns is the number of columns of T.
	NbColumns = 3 * blockLen
)

// Table is a table of 2ⁿ Poseidon2 compressions, with the GKR proof of its rows. See [Define].
type Table struct {
	// Module is T's module and Columns its columns.
	Module  *wiop.Module
	Columns gkrverifier.CompressionTable
	// Seed is the coin σ, the seed of the GKR transcript.
	Seed *wiop.CoinField
	// Point holds r*, with n coordinates, and Values holds y.
	Point  []*wiop.Cell
	Values [NbColumns]*wiop.Cell
	// Proof holds the flattened GKR proof, of the circuit and the schedule that Circuit and Schedule
	// hold.
	Proof    *wiop.ExtVector
	Circuit  gkr.SerializableCircuit
	Schedule constraint.GkrProvingSchedule
	// Lambda is the coin λ that batches the columns, and Result the result of the log-derivative sum,
	// which the verifier checks to be Σ_c λᶜ y_c.
	Lambda *wiop.CoinField
	Result *wiop.Cell

	compressor *gkrposeidon2.Compressor
	// eq is the column E.
	eq *wiop.Column
}

// Define declares in sys a table of 2ⁿ rows, whose columns are in round, and the GKR proof of its rows
// and the binding of the proof to the columns in the rounds after round. The caller assigns the columns
// with [Table.Assign] in a prover action of round.
func Define(sys *wiop.System, round *wiop.Round, n int) (*Table, error) {
	if n < 1 {
		return nil, fmt.Errorf("gkrtable: n is %d, it must be positive", n)
	}
	compressor, err := gkrposeidon2.NewCompressor(poseidon2.GetDefaultParameters())
	if err != nil {
		return nil, fmt.Errorf("gkrtable: the GKR circuit of the compression: %w", err)
	}

	ctx := sys.Context.Childf("gkrtable")
	t := &Table{compressor: compressor, Circuit: compressor.Circuit, Schedule: compressor.Schedule}
	t.Module = sys.NewSizedModule(ctx.Childf("table"), 1<<n, wiop.PaddingDirectionNone)
	for c := range t.Columns {
		t.Columns[c] = t.Module.NewColumn(ctx.Childf("column-%d", c), round)
	}

	gkrRound := round.EnsureNext()
	t.Seed = gkrRound.NewCoinField(ctx.Childf("seed"))
	t.Point = make([]*wiop.Cell, n)
	for j := range t.Point {
		t.Point[j] = gkrRound.NewCell(ctx.Childf("point-%d", j), true)
	}
	for c := range t.Values {
		t.Values[c] = gkrRound.NewCell(ctx.Childf("value-%d", c), true)
	}
	t.Proof = gkrRound.NewExtVector(ctx.Childf("proof"))

	gkrRound.RegisterAction(&proofAction{Table: t})
	gkrRound.RegisterVerifierAction(&proofVerifierAction{Table: t})
	t.defineBinding(sys, ctx)
	return t, nil
}

// Claims returns the claims that [gkrverifier.Inputs] expects for the GKR proof of T: for each wire of a
// column of T, the claim that the column has, at the point, the value that its handle in values holds.
func (t *Table) Claims(point *gkrverifier.Point, values [NbColumns][6]*wiop.Cell) map[int][]gkrverifier.Claim {
	claims := make(map[int][]gkrverifier.Claim, NbColumns)
	for c, value := range values {
		claims[t.wire(c)] = []gkrverifier.Claim{{Point: point, Value: value}}
	}
	return claims
}

// Assign assigns the columns of T to the rows, each of which holds the left input, the right input and
// the output of a compression, as the rows of [gkrverifier.Verifier.Compressions] do. The rows after
// them are the compression of zero. The outputs are not checked here: a row that is not a compression
// makes the verification fail. It returns an error if there are more than 2ⁿ rows.
func (t *Table) Assign(run *wiop.Runtime, rows [][NbColumns]field.Element) error {
	size := t.Module.Size()
	if len(rows) > size {
		return fmt.Errorf("gkrtable: %d rows in a table of %d", len(rows), size)
	}

	var padding [NbColumns]field.Element
	var left, right, out [blockLen]field.Element
	poseidon2.NewCompressor(poseidon2.NewDefaultPermutation()).CompressField(&out, &left, &right)
	copy(padding[2*blockLen:], out[:])

	for c, col := range t.Columns {
		values := make([]field.Element, size)
		for i := range values {
			if i < len(rows) {
				values[i] = rows[i][c]
			} else {
				values[i] = padding[c]
			}
		}
		run.AssignColumn(col, &wiop.ConcreteVector{Plain: field.VecFromBase(values)})
	}
	return nil
}
