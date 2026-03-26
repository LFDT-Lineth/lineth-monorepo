package vortex

import (
	"errors"
	"math/big"
	"math/bits"

	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark/constraint/solver"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/hash"
	"github.com/consensys/gnark/std/lookup/logderivlookup"
	"github.com/consensys/gnark/std/multicommit"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

var (
	ErrPNotOfSizeCardinality = errors.New("p should be of size cardinality")
)

// Register fft inverse hint
func init() {
	solver.RegisterHint(FFTInverseBLS12377)
}

// FFTInverseBLS12377 hint for the inverse FFT on BN254 (the frField is harcoded...)
func FFTInverseBLS12377(_ *big.Int, inputs []*big.Int, results []*big.Int) error {

	// TODO store this somewhere (global variable or something, shouldn't regenerate it at each call)
	d := fft.NewDomain(uint64(len(inputs)), fft.WithoutPrecompute())

	v := make([]fr.Element, len(inputs))
	for i := 0; i < len(inputs); i++ {
		v[i].SetBigInt(inputs[i])
	}
	d.FFTInverse(v, fft.DIF)
	fft.BitReverse(v)

	for i := 0; i < len(results); i++ {
		results[i] = big.NewInt(0)
		v[i].BigInt(results[i])
	}

	return nil
}

// computes fft^-1(p) where the fft is done on <generator>, a set of size cardinality.
// It is assumed that p is correctly sized.
//
// The fft is hardcoded with bls12-377 for now, to be more efficient than bigInt...
// It is assumed that p is of size cardinality.
func FFTInverse(api frontend.API, p []frontend.Variable, genInv fr.Element, cardinality uint64) ([]frontend.Variable, error) {

	var cardInverse fr.Element
	cardInverse.SetUint64(cardinality).Inverse(&cardInverse)

	// res of the fft inverse
	res, err := api.Compiler().NewHint(FFTInverseBLS12377, len(p), p...)
	if err != nil {
		return nil, err
	}

	// probabilistically check the result of the FFT
	multicommit.WithCommitment(
		api,
		func(api frontend.API, x frontend.Variable) error {
			// evaluation canonical
			ec := gnarkEvalCanonical(api, res, x)

			// evaluation Lagrange
			var gen fr.Element
			gen.Inverse(&genInv)
			lagranges := gnarkComputeLagrangeAtZ(api, x, gen, cardinality)
			var el frontend.Variable
			el = 0
			for i := 0; i < len(p); i++ {
				tmp := api.Mul(p[i], lagranges[i])
				el = api.Add(el, tmp)
			}

			api.AssertIsEqual(ec, el)
			return nil
		},
		p...,
	)

	return res, nil
}

// gnarkEvalCanonical evaluates p at z where p represents the polnyomial ∑ᵢp[i]Xⁱ
func gnarkEvalCanonical(api frontend.API, p []frontend.Variable, z frontend.Variable) frontend.Variable {

	var res frontend.Variable
	res = 0
	s := len(p)
	for i := 0; i < len(p); i++ {
		res = api.Mul(res, z)
		res = api.Add(res, p[s-1-i])
	}
	return res
}

func gnarkInterpolate(api frontend.API, p []frontend.Variable, z frontend.Variable, gen fr.Element, cardinality uint64) frontend.Variable {

	var res frontend.Variable
	res = 0

	lagranges := gnarkComputeLagrangeAtZ(api, z, gen, cardinality)
	for i := uint64(0); i < cardinality; i++ {
		tmp := api.Mul(lagranges[i], p[i])
		res = api.Add(res, tmp)
	}

	return res
}

// computeLagrange returns Lᵢ(ζ) for i=1..n
// with lᵢ(ζ) = ωⁱ/n*(ζⁿ-1)/(ζ - ωⁱ)
// (the g stands for gnark)
func gnarkComputeLagrangeAtZ(api frontend.API, z frontend.Variable, gen fr.Element, cardinality uint64) []frontend.Variable {

	res := make([]frontend.Variable, cardinality)
	tb := bits.TrailingZeros(uint(cardinality))

	// ζⁿ-1
	res[0] = z
	for i := 0; i < tb; i++ {
		res[0] = api.Mul(res[0], res[0])
	}
	res[0] = api.Sub(res[0], 1)

	// ζ-1
	var accZetaMinusOmegai frontend.Variable
	accZetaMinusOmegai = api.Sub(z, 1)

	// (ζⁿ-1)/(ζ-1)
	res[0] = api.Div(res[0], accZetaMinusOmegai)

	// 1/n*(ζⁿ-1)/(ζ-1)
	res[0] = api.Div(res[0], cardinality)

	// res[i] <- res[i-1] * (ζ-ωⁱ⁻¹)/(ζ-ωⁱ) * ω
	var accOmega fr.Element
	accOmega.SetOne()

	for i := uint64(1); i < cardinality; i++ {
		res[i] = api.Mul(res[i-1], gen)              // res[i] <- ω * res[i-1]
		res[i] = api.Mul(res[i], accZetaMinusOmegai) // res[i] <- res[i]*(ζ-ωⁱ⁻¹)
		accOmega.Mul(&accOmega, &gen)                // accOmega <- accOmega * ω
		accZetaMinusOmegai = api.Sub(z, accOmega)    // accZetaMinusOmegai <- ζ-ωⁱ
		res[i] = api.Div(res[i], accZetaMinusOmegai) // res[i]  <- res[i]/(ζ-ωⁱ)
	}

	return res
}

// Checks that p is a polynomial of degree < cardinality/rate
// * p polynomial of size cardinality
// * genInv inverse of the generator of the subgroup of size cardinality
// * rate of the RS code
func assertIsCodeWord(api frontend.API, p []frontend.Variable, genInv fr.Element, cardinality, rate uint64) error {

	if uint64(len(p)) != cardinality {
		return ErrPNotOfSizeCardinality
	}

	// get the canonical form of p
	pCanonical, err := FFTInverse(api, p, genInv, cardinality)
	if err != nil {
		return err
	}

	// check that is of degree < cardinality/rate
	degree := (cardinality - (cardinality % rate)) / rate
	for i := degree; i < cardinality; i++ {
		api.AssertIsEqual(pCanonical[i], 0)
	}

	return nil
}

// Opening proof without Merkle proofs
type GProofWoMerkle struct {

	// columns on against which the linear combination is checked
	// (the i-th entry is the EntryList[i]-th column). The columns may
	// as well be dispatched in several matrices.
	// Columns [i][j][k] returns the k-th entry of the j-th selected
	// column of the i-th commitment
	Columns [][][]frontend.Variable

	// domain of the RS code
	RsDomain *fft.Domain

	// Rate of the RS code, Blowup factor in Vortex, inverse rate to be precise
	Rate uint64

	// Linear combination of the rows of the polynomial P written as a square matrix
	LinearCombination []frontend.Variable
}

// Opening proof with Merkle proofs
type GProof struct {
	GProofWoMerkle
	MerkleProofs [][]smt.GnarkProof
}

// Gnark params
type GParams struct {
	Key         *ringsis.Key
	HasherFunc  func(frontend.API) (hash.FieldHasher, error)
	NoSisHasher func(frontend.API) (hash.FieldHasher, error)
}

func (p *GParams) HasNoSisHasher() bool {
	return p.NoSisHasher != nil
}

func GnarkVerifyCommon(
	api frontend.API,
	params GParams,
	proof GProofWoMerkle,
	x frontend.Variable,
	ys [][]frontend.Variable,
	randomCoin frontend.Variable,
	entryList []frontend.Variable,
) ([][]frontend.Variable, error) {

	// check the linear combination is a codeword
	api.Compiler().Defer(func(api frontend.API) error {
		return assertIsCodeWord(
			api,
			proof.LinearCombination,
			proof.RsDomain.GeneratorInv,
			proof.RsDomain.Cardinality,
			proof.Rate,
		)
	})

	// Check the consistency of Ys and proof.Linearcombination
	yjoined := utils.Join(ys...)
	alphaY := gnarkInterpolate(
		api,
		proof.LinearCombination,
		x,
		proof.RsDomain.Generator,
		proof.RsDomain.Cardinality)
	alphaYPrime := gnarkEvalCanonical(api, yjoined, randomCoin)
	api.AssertIsEqual(alphaY, alphaYPrime)

	// Size of the hash of 1 column
	numRounds := len(ys)

	selectedColSisDigests := make([][]frontend.Variable, numRounds)
	tbl := logderivlookup.New(api)
	for i := range proof.LinearCombination {
		tbl.Insert(proof.LinearCombination[i])
	}
	for j, selectedColID := range entryList {

		// Will carry the concatenation of the columns for the same entry j
		fullCol := []frontend.Variable{}

		for i := range selectedColSisDigests {

			if j == 0 {
				selectedColSisDigests[i] = make([]frontend.Variable, len(entryList))
			}

			// Entries of the selected columns #j contained in the commitment #i.
			selectedSubCol := proof.Columns[i][j]
			fullCol = append(fullCol, selectedSubCol...)

			// Check consistency between the opened column and the commitment
			if !params.HasNoSisHasher() {
				panic("the vortex verifier circuit only supports a no-SIS hasher")
			}

			hasher, _ := params.NoSisHasher(api)
			hasher.Reset()
			hasher.Write(selectedSubCol...)
			digest := hasher.Sum()
			selectedColSisDigests[i][j] = digest
		}

		// Check the linear combination is consistent with the opened column
		y := gnarkEvalCanonical(api, fullCol, randomCoin)
		v := tbl.Lookup(selectedColID)[0]
		api.AssertIsEqual(y, v)

	}
	return selectedColSisDigests, nil
}
