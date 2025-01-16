package testdata

import (
	"crypto/rand"
	"encoding/csv"
	"fmt"
	"math/big"
	"os"
	"strconv"
	"testing"

	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/gnark-crypto/ecc/bn254/fr"
)

type inputType int

const (
	fullTrivial            inputType = iota // (0, 0)
	leftTrivialValid                        // (0, Q)
	leftTrivialInvalid                      // (0, Q') where Q' is not in G2
	rightTrivialValid                       // (P, 0)
	rightTrivialInvalid                     // (P', 0) where P' is not in G1
	nonTrivialLeftInvalid                   // (P, Q) where P is not in G1
	nonTrivialRightInvalid                  // (P, Q) where Q is not in G2
	fullInvalid                             // (P, Q) where P and Q not in G1 and G2 respectively
	nonTrivial                              // (P, Q)
)

var choices = []inputType{fullTrivial, leftTrivialValid, leftTrivialInvalid, rightTrivialValid, rightTrivialInvalid, nonTrivialLeftInvalid, nonTrivialRightInvalid, fullInvalid, nonTrivial}

func (i inputType) String() string {
	switch i {
	case fullTrivial:
		return "full-trivial"
	case leftTrivialValid:
		return "left-trivial-valid"
	case leftTrivialInvalid:
		return "left-trivial-invalid"
	case rightTrivialValid:
		return "right-trivial-valid"
	case rightTrivialInvalid:
		return "right-trivial-invalid"
	case nonTrivialLeftInvalid:
		return "non-trivial-left-invalid"
	case nonTrivialRightInvalid:
		return "non-trivial-right-invalid"
	case fullInvalid:
		return "full-invalid"
	case nonTrivial:
		return "non-trivial"
	default:
		panic("unknown")
	}
}

func generateG1Infinity() bn254.G1Affine {
	var p bn254.G1Affine
	p.SetInfinity()
	return p
}

func generateG2Infinity() bn254.G2Affine {
	var q bn254.G2Affine
	q.SetInfinity()
	return q
}

func generateG1Invalid() bn254.G1Affine {
	var p bn254.G1Affine
	for {
		px, _ := rand.Int(rand.Reader, fp.Modulus())
		py, _ := rand.Int(rand.Reader, fp.Modulus())
		p.X.SetBigInt(px)
		p.Y.SetBigInt(py)
		if !p.IsOnCurve() {
			return p
		}
	}
}

func generateG2Invalid() bn254.G2Affine {
	var x, right, left, tmp, z, ZZ bn254.E2
	for {
		xa, _ := rand.Int(rand.Reader, fp.Modulus())
		xb, _ := rand.Int(rand.Reader, fp.Modulus())
		x.A0.SetBigInt(xa)
		x.A1.SetBigInt(xb)
		za, _ := rand.Int(rand.Reader, fp.Modulus())
		zb, _ := rand.Int(rand.Reader, fp.Modulus())
		z.A0.SetBigInt(za)
		z.A1.SetBigInt(zb)
		right.Square(&x).Mul(&right, &x)
		ZZ.Square(&z)
		tmp.Square(&ZZ).Mul(&tmp, &ZZ)
		tmp.MulBybTwistCurveCoeff(&tmp)
		right.Add(&right, &tmp)
		if right.Legendre() != 1 {
			continue
		}
		left.Sqrt(&right)
		QJac := bn254.G2Jac{
			X: x,
			Y: left,
			Z: z,
		}
		if !QJac.IsOnCurve() {
			panic("point is not on curve Jac")
		}
		var q bn254.G2Affine
		q.FromJacobian(&QJac)
		if !q.IsOnCurve() {
			continue
		}
		if q.IsInSubGroup() {
			continue
		}
		return q
	}
}

func generateG1Valid() bn254.G1Affine {
	var p bn254.G1Affine
	var s fr.Element
	s.SetRandom()
	p.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
	return p
}

func generateG2Valid() bn254.G2Affine {
	var q bn254.G2Affine
	var s fr.Element
	s.SetRandom()
	q.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
	return q
}

func (i inputType) generatePair() (generatedPair inputPair) {
	// here the pairingResult return value indicates if this input pair is
	// invalid, i.e. it makes the whole pairing check to fail
	//
	// the cancelMemberships indicates if we should cancel the membership checks
	// for other inputs in the pairing check. This is usually when G1 point is
	// invalid in which case we don't need to check other G2 points.
	var ip inputPair
	ip.inputType = i
	switch i {
	case fullTrivial:
		ip.P = generateG1Infinity()
		ip.Q = generateG2Infinity()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = true
		return ip
	case leftTrivialValid:
		ip.P = generateG1Infinity()
		ip.Q = generateG2Valid()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = true
		ip.MembershipSuccess = true
		return ip
	case leftTrivialInvalid:
		ip.P = generateG1Infinity()
		ip.Q = generateG2Invalid()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = true
		ip.MembershipSuccess = false
		return ip
	case rightTrivialValid:
		ip.P = generateG1Valid()
		ip.Q = generateG2Infinity()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = true
		return ip
	case rightTrivialInvalid:
		ip.P = generateG1Invalid()
		ip.Q = generateG2Infinity()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = false
		return ip
	case nonTrivialLeftInvalid:
		ip.P = generateG1Invalid()
		ip.Q = generateG2Valid()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = false
		return ip
	case nonTrivialRightInvalid:
		ip.P = generateG1Valid()
		ip.Q = generateG2Invalid()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = true
		ip.MembershipSuccess = false
		return ip
	case fullInvalid:
		ip.P = generateG1Invalid()
		ip.Q = generateG2Invalid()
		ip.ToPairingCircuit = false
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = false
		return ip
	case nonTrivial:
		ip.P = generateG1Valid()
		ip.Q = generateG2Valid()
		ip.ToPairingCircuit = true
		ip.ToMembershipCircuit = false
		ip.MembershipSuccess = false
		return ip
	default:
		panic("not handled")
	}
}

type inputs []inputType

type inputPair struct {
	P bn254.G1Affine
	Q bn254.G2Affine

	ToPairingCircuit    bool
	ToMembershipCircuit bool

	MembershipSuccess bool

	inputType inputType
}

type testCase struct {
	inputPairs []inputPair
	inputs     inputs
	result     bool
}

func (l inputs) nbNonTrivial() int {
	count := 0
	for _, input := range l {
		if input == nonTrivial {
			count++
		}
	}
	return count
}

func (l inputs) generateValidInputs() []inputPair {
	nb := l.nbNonTrivial()
	sp := make([]fr.Element, nb)
	sq := make([]fr.Element, nb)
	for i := 0; i < nb; i++ {
		sp[i].SetRandom()
		sq[i].SetRandom()
	}
	var acc, tmp fr.Element
	for i := 0; i < nb-1; i++ {
		tmp.Mul(&sp[i], &sq[i])
		acc.Add(&acc, &tmp)
	}
	acc.Neg(&acc)
	sq[nb-1].Div(&acc, &sp[nb-1])

	pairs := make([]inputPair, nb)
	var Ps []bn254.G1Affine
	var Qs []bn254.G2Affine
	bi := new(big.Int)
	for i := 0; i < nb; i++ {
		pairs[i].P.ScalarMultiplicationBase(sp[i].BigInt(bi))
		pairs[i].Q.ScalarMultiplicationBase(sq[i].BigInt(bi))
		pairs[i].ToPairingCircuit = true
		pairs[i].ToMembershipCircuit = false
		pairs[i].inputType = nonTrivial
		pairs[i].MembershipSuccess = true
		// for sanity check
		Ps = append(Ps, pairs[i].P)
		Qs = append(Qs, pairs[i].Q)
	}
	// sanity check
	ok, err := bn254.PairingCheck(Ps, Qs)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("invalid pairing")
	}
	return pairs
}

func (l inputs) generateInvalidInputs() []inputPair {
	nb := l.nbNonTrivial()
	var s fr.Element
	pairs := make([]inputPair, nb)
	for i := 0; i < nb; i++ {
		s.SetRandom()
		pairs[i].P.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
		s.SetRandom()
		pairs[i].Q.ScalarMultiplicationBase(s.BigInt(new(big.Int)))
		pairs[i].ToPairingCircuit = true
		pairs[i].ToMembershipCircuit = false
		pairs[i].inputType = nonTrivial
		pairs[i].MembershipSuccess = true
	}
	return pairs
}

func (l inputs) generateTestCase() []testCase {
	// count how many instances of what we have
	counts := make(map[inputType]int)
	for _, input := range l {
		counts[input]++
	}
	pairs := make([]inputPair, len(l))
	// if there is any invalid G1 point, we don't need to run anything
	if counts[rightTrivialInvalid] > 0 || counts[nonTrivialLeftInvalid] > 0 || counts[fullInvalid] > 0 {
		for i, input := range l {
			pairs[i] = input.generatePair()
			pairs[i].ToPairingCircuit = false
			pairs[i].ToMembershipCircuit = false
			pairs[i].MembershipSuccess = false
		}
		return []testCase{{inputPairs: pairs, result: false, inputs: l}}
	}
	// if there is any invalid G2 point, we only need to run membership check for that point
	if counts[leftTrivialInvalid] > 0 || counts[nonTrivialRightInvalid] > 0 {
		isFirstInvalid := true
		for i, input := range l {
			pairs[i] = input.generatePair()
			pairs[i].ToPairingCircuit = false
			if input == leftTrivialInvalid || input == nonTrivialRightInvalid {
				pairs[i].ToMembershipCircuit = true
				pairs[i].MembershipSuccess = false
				if isFirstInvalid {
					isFirstInvalid = false
				} else {
					pairs[i].ToPairingCircuit = false
				}
			} else {
				pairs[i].ToMembershipCircuit = false
			}
		}
		return []testCase{{inputPairs: pairs, result: false, inputs: l}}
	}
	// if for all points G2 are 0, we don't need to run anything
	if counts[rightTrivialValid] == len(l) {
		for i, input := range l {
			pairs[i] = input.generatePair()
		}
		return []testCase{{inputPairs: pairs, result: true, inputs: l}}
	}

	if l.nbNonTrivial() == 0 {
		// should be combination of fullTrivial and leftTrivialValid. Then the G2 points need to go to the membership circuit
		for i, input := range l {
			pairs[i] = input.generatePair()
		}
		return []testCase{{inputPairs: pairs, result: true, inputs: l}}
	}
	if l.nbNonTrivial() == 1 {
		invalidInputs := l.generateInvalidInputs()
		for i, input := range l {
			if input == nonTrivial {
				pairs[i] = invalidInputs[0]
			} else {
				pairs[i] = input.generatePair()
			}
		}
		return []testCase{{inputPairs: pairs, result: false, inputs: l}}
	}
	if l.nbNonTrivial() > 1 {
		validInputs := l.generateValidInputs()
		invalidInputs := l.generateInvalidInputs()
		validPairs := make([]inputPair, len(l))
		invalidPairs := make([]inputPair, len(l))
		for i, input := range l {
			if input == nonTrivial {
				validPairs[i] = validInputs[0]
				invalidPairs[i] = invalidInputs[0]
				validInputs = validInputs[1:]
				invalidInputs = invalidInputs[1:]
			} else {
				validPairs[i] = input.generatePair()
				invalidPairs[i] = input.generatePair()
			}
		}
		return []testCase{{inputPairs: validPairs, result: true, inputs: l}, {inputPairs: invalidPairs, result: false, inputs: l}}
	}
	panic("unexpected 2")
}

func (ip *inputPair) WriteCSV(w *csv.Writer, ecdataId, ecDataIndex, acc, total int) (newIndex int, err error) {
	// split G1 and G2 into limbs
	px := ip.P.X.Bytes()
	py := ip.P.Y.Bytes()
	qxre := ip.Q.X.A0.Bytes()
	qxim := ip.Q.X.A1.Bytes()
	qyre := ip.Q.Y.A0.Bytes()
	qyim := ip.Q.Y.A1.Bytes()

	limbs := []string{
		fmt.Sprintf("0x%x", px[0:16]),
		fmt.Sprintf("0x%x", px[16:32]),
		fmt.Sprintf("0x%x", py[0:16]),
		fmt.Sprintf("0x%x", py[16:32]),
		fmt.Sprintf("0x%x", qxim[0:16]),
		fmt.Sprintf("0x%x", qxim[16:32]),
		fmt.Sprintf("0x%x", qxre[0:16]),
		fmt.Sprintf("0x%x", qxre[16:32]),
		fmt.Sprintf("0x%x", qyim[0:16]),
		fmt.Sprintf("0x%x", qyim[16:32]),
		fmt.Sprintf("0x%x", qyre[0:16]),
		fmt.Sprintf("0x%x", qyre[16:32]),
	}
	records := make([][]string, len(limbs))
	var (
		inputPairSuccess = formatBoolAsInt(ip.MembershipSuccess)
		ToPairingCircuit = formatBoolAsInt(ip.ToPairingCircuit)
	)
	for i, limb := range limbs {
		isG2Part := i >= 4
		records[i] = []string{
			strconv.Itoa(ecdataId),
			limb,
			inputPairSuccess,
			strconv.Itoa(ecDataIndex + i),
			"1",
			"0",
			strconv.Itoa(acc),
			strconv.Itoa(total),
			ToPairingCircuit,
			formatBoolAsInt(ip.ToMembershipCircuit && isG2Part),
		}
	}
	if err := w.WriteAll(records); err != nil {
		return ecDataIndex, err
	}
	return ecDataIndex + len(limbs), nil
}

func (tc *testCase) WriteCSV(w *csv.Writer, ecdataId int) error {
	var err error
	var hasPairing bool
	index := 0
	for i, ip := range tc.inputPairs {
		if index, err = ip.WriteCSV(w, ecdataId, index, i+1, len(tc.inputPairs)); err != nil {
			return err
		}
		hasPairing = hasPairing || ip.ToPairingCircuit
	}
	// write result
	if err = w.Write([]string{
		strconv.Itoa(ecdataId),
		"0",
		formatBoolAsInt(tc.result),
		"0",
		"0",
		"1",
		"0",
		strconv.Itoa(len(tc.inputPairs)),
		formatBoolAsInt(hasPairing),
		"0",
	}); err != nil {
		return err
	}
	if err = w.Write([]string{
		strconv.Itoa(ecdataId),
		formatBoolAsInt(tc.result),
		formatBoolAsInt(tc.result),
		"1",
		"0",
		"1",
		"0",
		strconv.Itoa(len(tc.inputPairs)),
		formatBoolAsInt(hasPairing),
		"0",
	}); err != nil {
		return err
	}
	return nil
}

func writeHeader(w *csv.Writer) error {
	// record is
	// - ECDATA_ID (int random increasing)
	// - ECDATA_LIMB (int 128 bits)
	// - ECDATA_SUCCESS_BIT (bool)
	// - ECDATA_INDEX (int)
	// - ECDATA_IS_DATA (bool)
	// - ECDATA_IS_RES (bool)
	// - ECDATA_ACC_PAIRINGS (int)
	// - ECDATA_TOTAL_PAIRINGS (int)
	// - ECDATA_CS_PAIRING (bool)
	// - ECDATA_CS_G2_MEMBERSHIP (bool)
	return w.Write([]string{
		"ECDATA_ID",
		"ECDATA_LIMB",
		"ECDATA_SUCCESS_BIT",
		"ECDATA_INDEX",
		"ECDATA_IS_DATA",
		"ECDATA_IS_RES",
		"ECDATA_ACC_PAIRINGS",
		"ECDATA_TOTAL_PAIRINGS",
		"ECDATA_CS_PAIRING",
		"ECDATA_CS_G2_MEMBERSHIP",
	})
}

func formatBoolAsInt(b bool) string {
	if b {
		return "1"
	}
	return "0"
}

func generateTestCases(length int) []testCase {
	// generate all possible combinations of inputs
	// for each combination, generate all possible test cases
	cartesianProduct := func(list []inputs) []inputs {
		var ret []inputs
		if len(list) == 0 {
			for _, input := range choices {
				ret = append(ret, inputs{input})
			}
			return ret
		}
		for _, curr := range list {
			for _, input := range choices {
				newCurr := make(inputs, len(curr), len(curr)+1)
				copy(newCurr, curr)
				ret = append(ret, append(newCurr, input))
			}
		}
		return ret
	}
	cases := cartesianProduct(nil)
	for i := 1; i < length; i++ {
		cases = cartesianProduct(cases)
	}
	var allTestCases []testCase
	for _, tc := range cases {
		allTestCases = append(allTestCases, tc.generateTestCase()...)
	}
	return allTestCases
}

func TestGenerateECPairTestCases(t *testing.T) {
	t.Skip("long test, run manually when needed")
	var generatedCases []testCase
	for i := 1; i <= 5; i++ {
		generatedCases = append(generatedCases, generateTestCases(i)...)
	}
	for i, tc := range generatedCases {
		f, err := os.Create(fmt.Sprintf("generated/case-%06d_input.csv", i+1))
		if err != nil {
			t.Fatal(err)
		}
		defer f.Close()
		w := csv.NewWriter(f)
		defer w.Flush()
		if err := writeHeader(w); err != nil {
			t.Fatal(err)
		}
		if err := tc.WriteCSV(w, i+1); err != nil {
			t.Fatal(err)
		}
	}
	fmt.Println(len(generatedCases))
}

func TestWriteTestCase(t *testing.T) {
	// sanity test that nothing fails/panics etc
	input := inputs{nonTrivial, nonTrivial, nonTrivial}
	w := csv.NewWriter(os.Stdout)
	if err := writeHeader(w); err != nil {
		panic(err)
	}
	defer w.Flush()
	testCases := input.generateTestCase()
	for i, tc := range testCases {
		if err := tc.WriteCSV(w, i); err != nil {
			panic(err)
		}
	}
}
