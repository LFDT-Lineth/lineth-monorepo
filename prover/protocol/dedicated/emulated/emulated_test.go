package emulated

import (
	"crypto/rand"
	"crypto/sha3"
	"math/big"
	"testing"

	"github.com/consensys/linea-monorepo/prover/protocol/compiler/dummy"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/stretchr/testify/require"
)

func TestEmulatedMultiplication(t *testing.T) {
	const nbEntries = (1 << 4) // test non power power of two as well
	const nbBits = 384
	// const round_nr = 0
	const nbBitsPerLimb = 16
	const nbLimbs = (nbBits + nbBitsPerLimb - 1) / nbBitsPerLimb
	var pa, pa2 *MultiplicationAssignmentProverAction
	var expected, expected2 limbs.Limbs[limbs.LittleEndian]
	define := func(b *wizard.Builder) {
		P := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "P", nbLimbs, nbEntries)
		A := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "A", nbLimbs, nbEntries)
		B := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "B", nbLimbs, nbEntries)
		expected = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "EXPECTED", nbLimbs, nbEntries)
		pa = NewMul(b.CompiledIOP, "TEST", A, B, P, nbBitsPerLimb)
		// check that the result matches expected
		limbs.NewGlobal(b.CompiledIOP, "EMULATED_RESULT_CORRECTNESS", symbolic.Sub(pa.Result, expected))
		P2 := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "P2", nbLimbs, nbEntries)
		A2 := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "A2", nbLimbs, nbEntries)
		B2 := limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "B2", nbLimbs, nbEntries)
		expected2 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "EXPECTED2", nbLimbs, nbEntries)
		// second case to ensure that all columns and queries are properly separated
		pa2 = NewMul(b.CompiledIOP, "TEST2", A2, B2, P2, nbBitsPerLimb)
		// check that the result matches expected
		limbs.NewGlobal(b.CompiledIOP, "EMULATED_RESULT2_CORRECTNESS", symbolic.Sub(pa2.Result, expected2))
	}

	assignmentA := make([]*big.Int, nbEntries)
	assignmentB := make([]*big.Int, nbEntries)
	assignmentP := make([]*big.Int, nbEntries)
	assignmentExpected := make([]*big.Int, nbEntries)
	assignmentA2 := make([]*big.Int, nbEntries)
	assignmentB2 := make([]*big.Int, nbEntries)
	assignmentP2 := make([]*big.Int, nbEntries)
	assignmentExpected2 := make([]*big.Int, nbEntries)
	bound := new(big.Int).Lsh(big.NewInt(1), nbBits)
	var err error
	reader := sha3.NewSHAKE256()
	for i := range nbEntries {
		assignmentP[i], err = rand.Int(reader, bound)
		require.NoError(t, err)
		assignmentA[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentB[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentExpected[i] = new(big.Int).Mul(assignmentA[i], assignmentB[i])
		assignmentExpected[i].Mod(assignmentExpected[i], assignmentP[i])
		// second case
		assignmentP2[i], err = rand.Int(reader, bound)
		require.NoError(t, err)
		assignmentA2[i], err = rand.Int(reader, assignmentP2[i])
		require.NoError(t, err)
		assignmentB2[i], err = rand.Int(reader, assignmentP2[i])
		require.NoError(t, err)
		assignmentExpected2[i] = new(big.Int).Mul(assignmentA2[i], assignmentB2[i])
		assignmentExpected2[i].Mod(assignmentExpected2[i], assignmentP2[i])
	}

	prover := func(run *wizard.ProverRuntime) {
		pa.TermL.AssignBigInts(run, assignmentA)
		pa.TermR.AssignBigInts(run, assignmentB)
		pa.Modulus.AssignBigInts(run, assignmentP)
		expected.AssignBigInts(run, assignmentExpected)

		pa2.TermL.AssignBigInts(run, assignmentA2)
		pa2.TermR.AssignBigInts(run, assignmentB2)
		pa2.Modulus.AssignBigInts(run, assignmentP2)
		expected2.AssignBigInts(run, assignmentExpected2)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err = wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestEmulatedEvaluation(t *testing.T) {
	const nbEntries = (1 << 4) // to ensure non power of two sizes are handled
	const nbBits = 384
	// const round_nr = 0
	const nbBitsPerLimb = 16
	const nbLimbs = (nbBits + nbBitsPerLimb - 1) / nbBitsPerLimb
	var T0, T1, T2, T3, P limbs.Limbs[limbs.LittleEndian]
	define := func(b *wizard.Builder) {
		P = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "P", nbLimbs, nbEntries)
		T0 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T0", nbLimbs, nbEntries)
		T1 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T1", nbLimbs, nbEntries)
		T2 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T2", nbLimbs, nbEntries)
		T3 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T3", nbLimbs, nbEntries)
		// define the emulated evaluation. We can omit the returned value if not needed
		NewEval(b.CompiledIOP, "TEST", nbBitsPerLimb, P, [][]limbs.Limbs[limbs.LittleEndian]{
			{T0, T1}, {T0, T1, T2}, {T3}, // T0*T1 + T0*T1*T2 + T3 == 0
		}, []int{1, 1, 1})
	}

	assignmentT0 := make([]*big.Int, nbEntries)
	assignmentT1 := make([]*big.Int, nbEntries)
	assignmentT2 := make([]*big.Int, nbEntries)
	assignmentT3 := make([]*big.Int, nbEntries)
	assignmentP := make([]*big.Int, nbEntries)
	bound := new(big.Int).Lsh(big.NewInt(1), nbBits)
	var err error
	reader := sha3.NewSHAKE256()
	tmp := new(big.Int)
	for i := range nbEntries {
		assignmentP[i], err = rand.Int(reader, bound)
		require.NoError(t, err)
		assignmentT0[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentT1[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentT2[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentT3[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		// set T3 = - (T0*T1 + T0*T1*T2) mod P
		tmp.Mul(assignmentT0[i], assignmentT1[i])
		tmp.Mod(tmp, assignmentP[i])
		assignmentT3[i] = new(big.Int).Set(tmp)
		tmp.Mul(tmp, assignmentT2[i])
		tmp.Mod(tmp, assignmentP[i])
		assignmentT3[i].Add(assignmentT3[i], tmp)
		assignmentT3[i].Mod(assignmentT3[i], assignmentP[i])
		assignmentT3[i].Sub(assignmentP[i], assignmentT3[i])
		assignmentT3[i].Mod(assignmentT3[i], assignmentP[i])
	}

	prover := func(run *wizard.ProverRuntime) {
		P.AssignBigInts(run, assignmentP)
		T0.AssignBigInts(run, assignmentT0)
		T1.AssignBigInts(run, assignmentT1)
		T2.AssignBigInts(run, assignmentT2)
		T3.AssignBigInts(run, assignmentT3)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err = wizard.Verify(comp, proof)
	require.NoError(t, err)
}

func TestEmulatedEvaluationWithCoefficients(t *testing.T) {
	const nbEntries = (1 << 4)
	const nbBits = 384
	const nbBitsPerLimb = 16
	const nbLimbs = (nbBits + nbBitsPerLimb - 1) / nbBitsPerLimb
	var T0, T1, T2, T3, P limbs.Limbs[limbs.LittleEndian]
	define := func(b *wizard.Builder) {
		P = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "P", nbLimbs, nbEntries)
		T0 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T0", nbLimbs, nbEntries)
		T1 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T1", nbLimbs, nbEntries)
		T2 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T2", nbLimbs, nbEntries)
		T3 = limbs.NewLimbs[limbs.LittleEndian](b.CompiledIOP, "T3", nbLimbs, nbEntries)
		// T0*T1 + T0*T1*T2 - T3 == 0 mod P
		// i.e. T3 = (T0*T1 + T0*T1*T2) mod P
		NewEval(b.CompiledIOP, "TEST_COEFF", nbBitsPerLimb, P, [][]limbs.Limbs[limbs.LittleEndian]{
			{T0, T1}, {T0, T1, T2}, {T3},
		}, []int{1, 1, -1})
	}

	assignmentT0 := make([]*big.Int, nbEntries)
	assignmentT1 := make([]*big.Int, nbEntries)
	assignmentT2 := make([]*big.Int, nbEntries)
	assignmentT3 := make([]*big.Int, nbEntries)
	assignmentP := make([]*big.Int, nbEntries)
	bound := new(big.Int).Lsh(big.NewInt(1), nbBits)
	var err error
	reader := sha3.NewSHAKE256()
	tmp := new(big.Int)
	for i := range nbEntries {
		assignmentP[i], err = rand.Int(reader, bound)
		require.NoError(t, err)
		assignmentT0[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentT1[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		assignmentT2[i], err = rand.Int(reader, assignmentP[i])
		require.NoError(t, err)
		// T3 = (T0*T1 + T0*T1*T2) mod P  (no negation needed with -1 coeff)
		tmp.Mul(assignmentT0[i], assignmentT1[i])
		tmp.Mod(tmp, assignmentP[i])
		assignmentT3[i] = new(big.Int).Set(tmp)
		tmp.Mul(tmp, assignmentT2[i])
		tmp.Mod(tmp, assignmentP[i])
		assignmentT3[i].Add(assignmentT3[i], tmp)
		assignmentT3[i].Mod(assignmentT3[i], assignmentP[i])
	}

	prover := func(run *wizard.ProverRuntime) {
		P.AssignBigInts(run, assignmentP)
		T0.AssignBigInts(run, assignmentT0)
		T1.AssignBigInts(run, assignmentT1)
		T2.AssignBigInts(run, assignmentT2)
		T3.AssignBigInts(run, assignmentT3)
	}

	comp := wizard.Compile(define, dummy.Compile)
	proof := wizard.Prove(comp, prover)
	err = wizard.Verify(comp, proof)
	require.NoError(t, err)
}
