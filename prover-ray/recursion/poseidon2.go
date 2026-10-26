package recursion

import (
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	gp2 "github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// Poseidon2 parameters of ray's compression function (vortex.CompressPoseidon2,
// behind poseidon2.MDHasher, Fiat–Shamir and the Merkle trees): width 16,
// 6 full rounds, 21 partial rounds, S-box x³. A compression of the state s and
// the block b (8 elements each) is P(s ‖ b)[8..15] + b: the second half of
// the permutation's output, fed forward with the block.
const (
	P2Width          = 16
	P2FullRounds     = 6
	P2PartialRounds  = 21
	p2HalfFullRounds = P2FullRounds / 2
	p2Half           = P2Width / 2 // the state, the block and the digest: an octuplet each
)

// p2Params are the round keys of the permutation (gnark-crypto's default
// seed, as vortex's compression permutation).
var p2Params = gp2.NewParameters(P2Width, P2FullRounds, P2PartialRounds)

// p2Perm is the permutation itself, to compute traces.
var p2Perm = gp2.NewPermutation(P2Width, P2FullRounds, P2PartialRounds)

// compressCell returns the compression of state and block, as
// poseidon2.Compress computes it.
func compressCell(state, block Cell) Cell { return Cell(poseidon2.Compress(state, block)) }

// p2Diag16 is the diagonal of the width-16 internal matrix, M = 1·1ᵀ + diag,
// as used by gnark-crypto's matMulInternalInPlace (which does not export it;
// the AIR needs it as constraint coefficients). TestPoseidon2AIR proves the
// AIR against gnark-crypto's permutation, so a drift would fail it.
//
//	[-2, 1, 2, 1/2, 3, 4, -1/2, -3, -4, 1/2^8, 1/8, 1/2^24, -1/2^8, -1/8, -1/16, -1/2^24]
func p2Diag16() [P2Width]field.Element {
	var d [P2Width]field.Element
	pow2Inv := func(n int, neg bool) field.Element {
		var r field.Element
		r.SetUint64(1 << n)
		r.Inverse(&r)
		if neg {
			r.Neg(&r)
		}
		return r
	}
	for i, v := range []int64{-2, 1, 2} {
		d[i].SetInt64(v)
	}
	d[3] = pow2Inv(1, false)
	d[4].SetInt64(3)
	d[5].SetInt64(4)
	d[6] = pow2Inv(1, true)
	d[7].SetInt64(-3)
	d[8].SetInt64(-4)
	d[9] = pow2Inv(8, false)
	d[10] = pow2Inv(3, false)
	d[11] = pow2Inv(24, false)
	d[12] = pow2Inv(8, true)
	d[13] = pow2Inv(3, true)
	d[14] = pow2Inv(4, true)
	d[15] = pow2Inv(24, true)
	return d
}

// p2Diag is the internal matrix's diagonal, computed once (matInternal runs
// 21 times per permutation).
var p2Diag = p2Diag16()

// matExternal applies circ(2·M4, M4, …, M4) in place (gnark-crypto's
// matMulExternalInPlace), M4 = [[2 3 1 1] [1 2 3 1] [1 1 2 3] [3 1 1 2]].
func matExternal(s []field.Element) {
	for c := 0; c < len(s)/4; c++ {
		x := s[4*c : 4*c+4]
		var t01, t23, t0123, t01123, t01233 field.Element
		t01.Add(&x[0], &x[1])
		t23.Add(&x[2], &x[3])
		t0123.Add(&t01, &t23)
		t01123.Add(&t0123, &x[1])
		t01233.Add(&t0123, &x[3])
		x[3].Double(&x[0]).Add(&x[3], &t01233)
		x[1].Double(&x[2]).Add(&x[1], &t01123)
		x[0].Add(&t01, &t01123)
		x[2].Add(&t23, &t01233)
	}
	var sums [4]field.Element
	for c := 0; c < len(s)/4; c++ {
		for j := range sums {
			sums[j].Add(&sums[j], &s[4*c+j])
		}
	}
	for i := range s {
		s[i].Add(&s[i], &sums[i%4])
	}
}

// matInternal applies 1·1ᵀ + diag in place (gnark-crypto's
// matMulInternalInPlace).
func matInternal(s []field.Element) {
	var sum field.Element
	for i := range s {
		sum.Add(&sum, &s[i])
	}
	for i, d := range p2Diag {
		var t field.Element
		t.Mul(&s[i], &d)
		s[i].Add(&sum, &t)
	}
}

// p2ME and p2MI are the external and internal matrices, dense: the linear
// layers applied to unit vectors.
var p2ME, p2MI = func() (me, mi [P2Width][P2Width]field.Element) {
	for j := range P2Width {
		var e, i [P2Width]field.Element
		e[j].SetOne()
		i[j].SetOne()
		matExternal(e[:])
		matInternal(i[:])
		for r := range P2Width {
			me[r][j], mi[r][j] = e[r], i[r]
		}
	}
	return me, mi
}()

func isFullRound(r int) bool {
	return r < p2HalfFullRounds || r >= p2HalfFullRounds+P2PartialRounds
}
