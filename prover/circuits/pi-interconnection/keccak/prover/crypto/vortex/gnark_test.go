//go:build !fuzzlight

package vortex

import (
	// "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	// "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	// "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"

	"testing"

	"github.com/consensys/gnark-crypto/ecc"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr"
	"github.com/consensys/gnark-crypto/ecc/bls12-377/fr/fft"
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/frontend/cs/scs"
	"github.com/consensys/gnark/std/hash"
	gmimc "github.com/consensys/gnark/std/hash/mimc"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils/types"
	"github.com/stretchr/testify/require"
)

// ------------------------------------------------------------
// test computeLagrange

func evalPoly(p []fr.Element, z fr.Element) fr.Element {
	var res fr.Element
	n := len(p)
	for i := 0; i < len(p); i++ {
		res.Mul(&res, &z)
		res.Add(&res, &p[n-1-i])
	}
	return res
}

type ComputeLagrangeCircuit struct {
	Domain fft.Domain
	Zeta   frontend.Variable   `gnark:",public"` // random variable
	Li     []frontend.Variable // expected results
}

func (circuit *ComputeLagrangeCircuit) Define(api frontend.API) error {

	n := circuit.Domain.Cardinality
	gen := circuit.Domain.Generator
	r := gnarkComputeLagrangeAtZ(api, circuit.Zeta, gen, n)

	for i := 0; i < len(r); i++ {
		api.AssertIsEqual(r[i], circuit.Li[i])
	}

	return nil
}

func TestComputeLagrangeCircuit(t *testing.T) {

	s := 16
	d := fft.NewDomain(uint64(s))
	var zeta fr.Element
	zeta.SetRandom()

	// prepare witness
	var witness ComputeLagrangeCircuit
	witness.Zeta = zeta.String()
	witness.Li = make([]frontend.Variable, s)
	for i := 0; i < s; i++ {
		buf := make([]fr.Element, s)
		buf[i].SetOne()
		d.FFTInverse(buf, fft.DIF)
		fft.BitReverse(buf)
		li := evalPoly(buf, zeta)
		witness.Li[i] = li.String()
	}

	var circuit ComputeLagrangeCircuit
	circuit.Domain = *d
	circuit.Li = make([]frontend.Variable, s)

	// compile...
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// ------------------------------------------------------------
// test FFT inverse

type FFTInverseCircuit struct {
	Domain fft.Domain
	P      []frontend.Variable
	R      []frontend.Variable
}

func (circuit *FFTInverseCircuit) Define(api frontend.API) error {

	f, err := FFTInverse(api, circuit.P, circuit.Domain.GeneratorInv, circuit.Domain.Cardinality)
	if err != nil {
		return err
	}

	for i := 0; i < len(f); i++ {
		api.AssertIsEqual(circuit.R[i], f[i])
	}

	return nil
}

func TestFFTInverseCircuit(t *testing.T) {

	s := 16
	d := fft.NewDomain(uint64(s))

	// prepare witness
	p := make([]fr.Element, s)
	for i := 0; i < s; i++ {
		p[i].SetRandom()
	}
	r := make([]fr.Element, s)
	copy(r, p)
	d.FFTInverse(r, fft.DIF)
	fft.BitReverse(r)
	var witness FFTInverseCircuit
	witness.P = make([]frontend.Variable, s)
	witness.R = make([]frontend.Variable, s)

	for i := 0; i < s; i++ {
		witness.P[i] = p[i].String()
		witness.R[i] = r[i].String()
	}

	var circuit FFTInverseCircuit
	circuit.P = make([]frontend.Variable, s)
	circuit.R = make([]frontend.Variable, s)
	circuit.Domain = *d

	// compile...
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// ------------------------------------------------------------
// test AssertIsCodeWord

type AssertIsCodeWordCircuit struct {
	rate uint64
	d    *fft.Domain
	P    []frontend.Variable `gnark:",public"`
}

func (circuit *AssertIsCodeWordCircuit) Define(api frontend.API) error {

	return assertIsCodeWord(api, circuit.P, circuit.d.GeneratorInv, circuit.d.Cardinality, circuit.rate)

}

func TestAssertIsCodeWord(t *testing.T) {

	// generate witness
	size := 2048
	d := fft.NewDomain(uint64(size))
	rate := 2
	p := make([]fr.Element, size)
	for i := 0; i < (size / rate); i++ {
		p[i].SetRandom()
	}
	d.FFT(p, fft.DIF)
	fft.BitReverse(p)

	var witness AssertIsCodeWordCircuit
	witness.P = make([]frontend.Variable, size)
	witness.rate = uint64(rate)
	witness.d = d
	for i := 0; i < size; i++ {
		witness.P[i] = p[i].String()
	}

	// compile the circuit
	var circuit AssertIsCodeWordCircuit
	circuit.P = make([]frontend.Variable, size)
	circuit.d = d
	circuit.rate = uint64(rate)
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// ------------------------------------------------------------
// Interpolate

type InterpolateCircuit struct {
	P []frontend.Variable
	X frontend.Variable `gnark:",public"`
	R frontend.Variable
	d *fft.Domain
}

func (circuit *InterpolateCircuit) Define(api frontend.API) error {

	res := gnarkInterpolate(api, circuit.P, circuit.X, circuit.d.Generator, circuit.d.Cardinality)

	api.AssertIsEqual(res, circuit.R)

	return nil
}

func TestInterpolateCircuit(t *testing.T) {

	// generate witness
	size := 16
	p := make([]fr.Element, size)
	for i := 0; i < size; i++ {
		p[i].SetRandom()
	}
	var x fr.Element
	x.SetRandom()
	var r fr.Element
	for i := 0; i < size; i++ {
		r.Mul(&r, &x)
		r.Add(&r, &p[len(p)-1-i])
	}

	d := fft.NewDomain(uint64(size))
	d.FFT(p, fft.DIF)
	fft.BitReverse(p)

	var witness InterpolateCircuit
	witness.P = make([]frontend.Variable, size)
	for i := 0; i < size; i++ {
		witness.P[i] = p[i].String()
	}
	witness.X = x
	witness.R = r

	// compile the circuit
	var circuit InterpolateCircuit
	circuit.P = make([]frontend.Variable, size)
	circuit.d = d
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit)
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}

}

// ------------------------------------------------------------
// N Commitments with Merkle opening

func getProofVortexNCommitmentsWithMerkleNoSis(t *testing.T, nCommitments, nPolys, polySize, blowUpFactor int) (
	proof *OpeningProof,
	randomCoin field.Element,
	x field.Element,
	yLists [][]field.Element,
	entryList []int,
	roots []types.Bytes32,
) {

	x = field.NewElement(478)
	randomCoin = field.NewElement(1523)
	entryList = []int{1, 5, 19, 645}

	params := NewParams(blowUpFactor, polySize, nPolys*nCommitments, ringsis.StdParams)

	polyLists := make([][]smartvectors.SmartVector, nCommitments)
	yLists = make([][]field.Element, nCommitments)
	for j := range polyLists {
		// Polynomials to commit to
		polys := make([]smartvectors.SmartVector, nPolys)
		ys := make([]field.Element, nPolys)
		for i := range polys {
			polys[i] = smartvectors.Rand(polySize)
			ys[i] = smartvectors.Interpolate(polys[i], x)
		}
		polyLists[j] = polys
		yLists[j] = ys
	}

	// Commits to it
	roots = make([]types.Bytes32, nCommitments)
	trees := make([]*smt.Tree, nCommitments)
	committedMatrices := make([]EncodedMatrix, nCommitments)
	isSISReplacedByMiMC := make([]bool, nCommitments)
	for j := range trees {
		// As Gnark does not support SIS, we commit without SIS hashing
		committedMatrices[j], trees[j], _ = params.CommitMerkleWithoutSIS(polyLists[j])
		roots[j] = trees[j].Root
		// We set the SIS replaced by MiMC to true, as Gnark does not support SIS
		isSISReplacedByMiMC[j] = true
	}

	// Generate the proof
	proof = params.InitOpeningWithLC(utils.Join(polyLists...), randomCoin)
	proof.Complete(entryList, committedMatrices, trees)

	// Check the proof
	err := VerifyOpening(&VerifierInputs{
		Params:              *params,
		MerkleRoots:         roots,
		X:                   x,
		Ys:                  yLists,
		OpeningProof:        *proof,
		RandomCoin:          randomCoin,
		EntryList:           entryList,
		IsSISReplacedByMiMC: isSISReplacedByMiMC,
	})
	require.NoError(t, err)

	return proof, randomCoin, x, yLists, entryList, roots
}

func TestGnarkVortexNCommitmentsWithMerkleNoSis(t *testing.T) {

	// generate witness
	nCommitments := 4
	nPolys := 15
	polySize := 1 << 10
	blowUpFactor := 2
	proof, randomCoin, x, ys, entryList, commitments := getProofVortexNCommitmentsWithMerkleNoSis(t, nCommitments, nPolys, polySize, blowUpFactor)

	rsSize := blowUpFactor * polySize
	rsDomainSize := uint64(rsSize)
	rsDomain := fft.NewDomain(rsDomainSize)
	var witness VerifyOpeningCircuitMerkleTree
	witness.Proof.RsDomain = rsDomain
	witness.Proof.Rate = uint64(blowUpFactor)
	AllocateCircuitVariablesWithMerkleTree(&witness, *proof, ys, entryList, commitments)
	AssignCicuitVariablesWithMerkleTree(&witness, *proof, ys, entryList, commitments)
	witness.RandomCoin = randomCoin.String()
	witness.X = x.String()
	witness.Params.HasherFunc = makeMimcHasherfunc
	witness.Params.NoSisHasher = makeMimcHasherfunc

	// compile the circuit
	var circuit VerifyOpeningCircuitMerkleTree
	circuit.Proof.LinearCombination = make([]frontend.Variable, rsSize)
	circuit.Proof.Rate = uint64(blowUpFactor)
	circuit.Proof.RsDomain = rsDomain
	circuit.Params.HasherFunc = makeMimcHasherfunc
	circuit.Params.NoSisHasher = makeMimcHasherfunc

	AllocateCircuitVariablesWithMerkleTree(&circuit, *proof, ys, entryList, commitments)
	ccs, err := frontend.Compile(ecc.BLS12_377.ScalarField(), scs.NewBuilder, &circuit, frontend.IgnoreUnconstrainedInputs())
	if err != nil {
		t.Fatal(err)
	}

	// solve the circuit
	twitness, err := frontend.NewWitness(&witness, ecc.BLS12_377.ScalarField())
	if err != nil {
		t.Fatal(err)
	}
	err = ccs.IsSolved(twitness)
	if err != nil {
		t.Fatal(err)
	}
}

func makeMimcHasherfunc(api frontend.API) (hash.FieldHasher, error) {
	h, err := gmimc.NewMiMC(api)
	return &h, err
}
