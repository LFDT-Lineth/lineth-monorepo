package main

import (
	"encoding/csv"
	"fmt"
	"iter"
	"math/big"
	"os"
	"strconv"
	"strings"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	fr "github.com/consensys/gnark-crypto/ecc/bls12-381/fr"
)

type pairInputType int

const (
	pairFullTrivial                 pairInputType = iota // (0, 0)
	pairLeftTrivialValid                                 // (0, Q)
	pairLeftTrivialInvalidCurve                          // (0, Q') where Q' is not in C2
	pairLeftTrivialInvalidGroup                          // (0, Q') where Q' is not in G2 subgroup
	pairRightTrivialValid                                // (P, 0)
	pairRightTrivialInvalidCurve                         // (P', 0) where P' is not in C1
	pairRightTrivialInvalidGroup                         // (P', 0) where P' is not in G1 subgroup
	pairNonTrivialLeftInvalidCurve                       // (P, Q) where P is not in C1
	pairNonTrivialLeftInvalidGroup                       // (P, Q) where P is not in G1 subgroup
	pairNonTrivialRightInvalidCurve                      // (P, Q) where Q is not in C2
	pairNonTrivialRightInvalidGroup                      // (P, Q) where Q is not in G2 subgroup
	pairFullInvalidCurveCurve                            // (P, Q) where P and Q not in C1 and C2 respectively
	pairFullInvalidCurveGroup                            // (P, Q) where P and Q not in C1 and G2 respectively
	pairFullInvalidGroupCurve                            // (P, Q) where P and Q not in G1 and C2 respectively
	pairFullInvalidGroupGroup                            // (P, Q) where P and Q not in G1 and G2 subgroups respectively
	pairNonTrivial                                       // (P, Q)
)

func (p pairInputType) String() string {
	switch p {
	case pairFullTrivial:
		return "pairFullTrivial"
	case pairLeftTrivialValid:
		return "pairLeftTrivialValid"
	case pairLeftTrivialInvalidCurve:
		return "pairLeftTrivialInvalidCurve"
	case pairLeftTrivialInvalidGroup:
		return "pairLeftTrivialInvalidGroup"
	case pairRightTrivialValid:
		return "pairRightTrivialValid"
	case pairRightTrivialInvalidCurve:
		return "pairRightTrivialInvalidCurve"
	case pairRightTrivialInvalidGroup:
		return "pairRightTrivialInvalidGroup"
	case pairNonTrivialLeftInvalidCurve:
		return "pairNonTrivialLeftInvalidCurve"
	case pairNonTrivialLeftInvalidGroup:
		return "pairNonTrivialLeftInvalidGroup"
	case pairNonTrivialRightInvalidCurve:
		return "pairNonTrivialRightInvalidCurve"
	case pairNonTrivialRightInvalidGroup:
		return "pairNonTrivialRightInvalidGroup"
	case pairFullInvalidCurveCurve:
		return "pairFullInvalidCurveCurve"
	case pairFullInvalidCurveGroup:
		return "pairFullInvalidCurveGroup"
	case pairFullInvalidGroupCurve:
		return "pairFullInvalidGroupCurve"
	case pairFullInvalidGroupGroup:
		return "pairFullInvalidGroupGroup"
	case pairNonTrivial:
		return "pairNonTrivial"
	default:
		panic("unknown input type")
	}
}

type pairInput struct {
	P bls12381.G1Affine
	Q bls12381.G2Affine

	// ToPairingCircuit is true if the input is wellformed and fully non-trivial
	ToPairingCircuit bool
	// ToG1MembershipCircuit is true if P should be sent to the G1 membership circuit. The membership result should be in SUCCESS_BIT
	ToG1MembershipCircuit bool
	// ToG2MembershipCircuit is true if Q should be sent to the G2 membership circuit. The membership result should be in SUCCESS_BIT
	ToG2MembershipCircuit bool

	// Indicates if the G1/G2 membership check should pass
	MembershipSuccess bool
	// IsTrivialG1 is true if P is trivial (0, 0)
	IsTrivialG1 bool
	// IsTrivialG2 is true if Q is trivial (0, 0)
	IsTrivialG2 bool
	// IsTrivialAcc is true if either of the inputs is trivial
	IsTrivialAcc bool

	inputType pairInputType
}

func (p pairInput) String() string {
	return fmt.Sprintf("pairInput{P: %s, Q: %s, ToPairingCircuit: %t, ToG1MembershipCircuit: %t, ToG2MembershipCircuit: %t, MembershipSuccess: %t, IsTrivialG1: %t, IsTrivialG2: %t, IsTrivialAcc: %t, inputType: %s}",
		p.P.String(), p.Q.String(), p.ToPairingCircuit, p.ToG1MembershipCircuit, p.ToG2MembershipCircuit,
		p.MembershipSuccess, p.IsTrivialG1, p.IsTrivialG2, p.IsTrivialAcc, p.inputType)
}

type pairInputCase struct {
	inputs []pairInput
	result bool
}

func (p pairInputCase) String() string {
	var inputStrs []string
	for _, input := range p.inputs {
		inputStrs = append(inputStrs, input.String())
	}
	return fmt.Sprintf("[]{%s}, result: %t", strings.Join(inputStrs, ", "), p.result)
}

func generatePairInput() iter.Seq2[int, func() pairInput] {
	return func(yield func(int, func() pairInput) bool) {
		for i, inputType := range []pairInputType{
			pairFullTrivial,
			pairLeftTrivialValid,
			pairLeftTrivialInvalidCurve,
			pairLeftTrivialInvalidGroup,
			pairRightTrivialValid,
			pairRightTrivialInvalidCurve,
			pairRightTrivialInvalidGroup,
			pairNonTrivialLeftInvalidCurve,
			pairNonTrivialLeftInvalidGroup,
			pairNonTrivialRightInvalidCurve,
			pairNonTrivialRightInvalidGroup,
			pairFullInvalidCurveCurve,
			pairFullInvalidCurveGroup,
			pairFullInvalidGroupCurve,
			pairFullInvalidGroupGroup,
			pairNonTrivial,
		} {
			tcf := func() pairInput {
				var ip pairInput
				switch inputType {
				case pairFullTrivial:
					ip = pairInput{
						P:                     generateG1Trivial(),
						Q:                     generateG2Trivial(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     true,
						IsTrivialG1:           true,
						IsTrivialG2:           true,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairLeftTrivialValid:
					ip = pairInput{
						P:                     generateG1Trivial(),
						Q:                     generateG2InSubgroup(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: true,
						MembershipSuccess:     true,
						IsTrivialG1:           true,
						IsTrivialG2:           false,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairLeftTrivialInvalidCurve:
					ip = pairInput{
						P:                     generateG1Trivial(),
						Q:                     generateG2Invalid(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: true,
						MembershipSuccess:     false,
						IsTrivialG1:           true,
						IsTrivialG2:           false,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairLeftTrivialInvalidGroup:
					ip = pairInput{
						P:                     generateG1Trivial(),
						Q:                     generateG2OnCurve(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: true,
						MembershipSuccess:     false,
						IsTrivialG1:           true,
						IsTrivialG2:           false,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairRightTrivialValid:
					ip = pairInput{
						P:                     generateG1InSubgroup(),
						Q:                     generateG2Trivial(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     true,
						IsTrivialG1:           false,
						IsTrivialG2:           true,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairRightTrivialInvalidCurve:
					ip = pairInput{
						P:                     generateG1Invalid(),
						Q:                     generateG2Trivial(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           true,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairRightTrivialInvalidGroup:
					ip = pairInput{
						P:                     generateG1OnCurve(),
						Q:                     generateG2Trivial(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           true,
						IsTrivialAcc:          true,
						inputType:             inputType,
					}
				case pairNonTrivialLeftInvalidCurve:
					ip = pairInput{
						P:                generateG1InSubgroup(),
						Q:                generateG2Invalid(),
						ToPairingCircuit: false,
						// right is invalid, we send it to the G2 membership circuit
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: true,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairNonTrivialLeftInvalidGroup:
					ip = pairInput{
						P:                     generateG1InSubgroup(),
						Q:                     generateG2OnCurve(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: true,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairNonTrivialRightInvalidCurve:
					ip = pairInput{
						P:                     generateG1Invalid(),
						Q:                     generateG2InSubgroup(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairNonTrivialRightInvalidGroup:
					ip = pairInput{
						P:                     generateG1OnCurve(),
						Q:                     generateG2InSubgroup(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairFullInvalidCurveCurve:
					ip = pairInput{
						P:                generateG1Invalid(),
						Q:                generateG2Invalid(),
						ToPairingCircuit: false,
						// when both points are invalid, then we only send the easiest one for checking (G1)
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairFullInvalidCurveGroup:
					ip = pairInput{
						P:                     generateG1Invalid(),
						Q:                     generateG2OnCurve(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairFullInvalidGroupCurve:
					ip = pairInput{
						P:                     generateG1OnCurve(),
						Q:                     generateG2Invalid(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairFullInvalidGroupGroup:
					ip = pairInput{
						P:                     generateG1OnCurve(),
						Q:                     generateG2OnCurve(),
						ToPairingCircuit:      false,
						ToG1MembershipCircuit: true,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     false,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				case pairNonTrivial:
					// this will be overwritten to obtain input such that pairing check is 1 or 0
					ip = pairInput{
						P:                     generateG1InSubgroup(),
						Q:                     generateG2InSubgroup(),
						ToPairingCircuit:      true,
						ToG1MembershipCircuit: false,
						ToG2MembershipCircuit: false,
						MembershipSuccess:     true,
						IsTrivialG1:           false,
						IsTrivialG2:           false,
						IsTrivialAcc:          false,
						inputType:             inputType,
					}
				}
				return ip
			}
			if !yield(i, tcf) {
				return
			}
		}
	}
}

func generateValidPairing(nb int) ([]bls12381.G1Affine, []bls12381.G2Affine) {
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
	Ps := make([]bls12381.G1Affine, nb)
	Qs := make([]bls12381.G2Affine, nb)
	bi := new(big.Int)
	for i := 0; i < nb; i++ {
		Ps[i].ScalarMultiplicationBase(sp[i].BigInt(bi))
		Qs[i].ScalarMultiplicationBase(sq[i].BigInt(bi))
	}
	// sanity check
	ok, err := bls12381.PairingCheck(Ps, Qs)
	if err != nil {
		panic(err)
	}
	if !ok {
		panic("generated pairing check is not valid")
	}
	return Ps, Qs
}

func processPairInputs(v []pairInput) []pairInputCase {
	// count how many instances of what we have
	counts := make(map[pairInputType]int)
	for _, input := range v {
		counts[input.inputType]++
	}
	// if there is invalid point, we need to run membership check on that point
	if counts[pairLeftTrivialInvalidCurve] > 0 ||
		counts[pairLeftTrivialInvalidGroup] > 0 ||
		counts[pairNonTrivialLeftInvalidCurve] > 0 ||
		counts[pairNonTrivialLeftInvalidGroup] > 0 ||
		counts[pairRightTrivialInvalidCurve] > 0 ||
		counts[pairRightTrivialInvalidGroup] > 0 ||
		counts[pairNonTrivialRightInvalidCurve] > 0 ||
		counts[pairNonTrivialRightInvalidGroup] > 0 ||
		counts[pairFullInvalidCurveCurve] > 0 ||
		counts[pairFullInvalidCurveGroup] > 0 ||
		counts[pairFullInvalidGroupCurve] > 0 ||
		counts[pairFullInvalidGroupGroup] > 0 {
		isFirstInvalid := true
		for i := range v {
			v[i].ToPairingCircuit = false
			switch {
			case (v[i].ToG2MembershipCircuit || v[i].ToG1MembershipCircuit) && v[i].MembershipSuccess:
				// ensure that we only check non-membership
				v[i].ToG1MembershipCircuit = false
				v[i].ToG2MembershipCircuit = false
			case isFirstInvalid && (v[i].ToG1MembershipCircuit || v[i].ToG2MembershipCircuit) && !v[i].MembershipSuccess:
				// we only run membership check on the first invalid point. This
				// means we keep it as is
				isFirstInvalid = false
			case !isFirstInvalid && (v[i].ToG1MembershipCircuit || v[i].ToG2MembershipCircuit) && !v[i].MembershipSuccess:
				// we do not run membership check on the rest of invalid points,
				// so we set ToG1MembershipCircuit and ToG2MembershipCircuit to
				// false
				v[i].ToG1MembershipCircuit = false
				v[i].ToG2MembershipCircuit = false
			}
		}
		// now we set the success_bit to false on all inputs for this test case
		// (even if we have something where the membership check passes)
		for i := range v {
			v[i].MembershipSuccess = false
		}
		return []pairInputCase{pairInputCase{
			inputs: v,
			result: false,
		}}
	}
	if counts[pairNonTrivial] == 0 {
		// there is no non-trivial input, so every pair has trivial component.
		// In this case, the pairing result is always 0
		return []pairInputCase{pairInputCase{
			inputs: v,
			result: true,
		}}
	}
	if counts[pairNonTrivial] == 1 {
		// there is only a single non-trivial input. But it cannot be 0 (it is
		// not trivial), so it must be failing
		return []pairInputCase{pairInputCase{
			inputs: v,
			result: false,
		}}
	}
	if counts[pairNonTrivial] > 1 {
		// validV := v
		validV := make([]pairInput, len(v))
		copy(validV, v)
		// there are multiple non-trivial inputs. We generate two test cases -
		// one where the pairing check result is 1 and other where it is 0.
		Ps, Qs := generateValidPairing(counts[pairNonTrivial])
		for i := range validV {
			if validV[i].inputType == pairNonTrivial {
				validV[i].P = Ps[0]
				validV[i].Q = Qs[0]
				Ps = Ps[1:]
				Qs = Qs[1:]
			}
		}
		return []pairInputCase{
			pairInputCase{
				inputs: validV,
				result: true,
			},
			pairInputCase{
				inputs: v,
				result: false,
			},
		}
	}
	panic("unexpected input case")
}

func generatePairInputCases(length int) iter.Seq2[int, pairInputCase] {
	return func(yield func(int, pairInputCase) bool) {
		var id int
		for v := range cartesianProduct(length, generatePairInput) {
			tc := processPairInputs(v)
			for _, input := range tc {
				if !yield(id, input) {
					return
				}
				id++
			}
		}
	}
}

func (tc pairInputCase) WriteCSV(w *csv.Writer, id int) error {
	var index int
	hasPairing := false
	// we first need to check if there are inputs which don't belong to the
	// subgroup. In that case nothing goes to the pairing circuit
	for _, input := range tc.inputs {
		if input.ToPairingCircuit {
			hasPairing = true
		}
	}
	for i, input := range tc.inputs {
		pLimbs := splitToLimbs(input.P)
		qLimbs := splitToLimbs(input.Q)
		for j, limb := range pLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),                        // ID
				"1",                                     // DATA_PAIRING_CHECK
				"0",                                     // RSLT_PAIRING_CHECK
				strconv.Itoa(index),                     // INDEX
				strconv.Itoa(j),                         // CT
				"1",                                     // IS_FIRST_INPUT
				"0",                                     // IS_SECOND_INPUT
				formatBoolAsInt(!input.IsTrivialAcc),    // NONTRIVIAL_POP_BIT
				formatBoolAsInt(input.ToPairingCircuit), // CS_PAIRING_CHECK
				formatBoolAsInt(input.ToG1MembershipCircuit), // CS_G1_MEMBERSHIP
				"0",                                      // CS_G2_MEMBERSHIP. Always zero as P only goes to G1 membership
				formatBoolAsInt(input.MembershipSuccess), // SUCCESS_BIT
				strconv.Itoa(len(tc.inputs)),             // ACC_INPUTS
				limb,                                     // LIMB
			}); err != nil {
				return fmt.Errorf("failed to write pairing input P %d/%d: %w", i, j, err)
			}
			index++
		}
		for j, limb := range qLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),                        // ID
				"1",                                     // DATA_PAIRING_CHECK
				"0",                                     // RSLT_PAIRING_CHECK
				strconv.Itoa(index),                     // INDEX
				strconv.Itoa(j),                         // CT
				"0",                                     // IS_FIRST_INPUT
				"1",                                     // IS_SECOND_INPUT
				formatBoolAsInt(!input.IsTrivialAcc),    // NONTRIVIAL_POP_BIT
				formatBoolAsInt(input.ToPairingCircuit), // CS_PAIRING_CHECK
				"0",                                     // CS_G1_MEMBERSHIP. Always zero as Q only goes to G2 membership
				formatBoolAsInt(input.ToG2MembershipCircuit), // CS_G2_MEMBERSHIP
				formatBoolAsInt(input.MembershipSuccess),     // SUCCESS_BIT
				strconv.Itoa(len(tc.inputs)),                 // ACC_INPUTS
				limb,                                         // LIMB
			}); err != nil {
				return fmt.Errorf("failed to write pairing input Q %d/%d: %w", i, j, err)
			}
			index++
		}
	}
	if err := w.Write([]string{
		strconv.Itoa(id),            // ID
		"0",                         // DATA_PAIRING_CHECK
		"1",                         // RSLT_PAIRING_CHECK
		"0",                         // INDEX
		"0",                         // CT
		"0",                         // IS_FIRST_INPUT
		"0",                         // IS_SECOND_INPUT
		"0",                         // NONTRIVIAL_POP_BIT
		formatBoolAsInt(hasPairing), // CS_PAIRING_CHECK
		"0",                         // CS_G1_MEMBERSHIP
		"0",                         // CS_G2_MEMBERSHIP
		formatBoolAsInt(tc.inputs[0].MembershipSuccess), // SUCCESS_BIT
		strconv.Itoa(len(tc.inputs)),                    // ACC_INPUTS
		"0",                                             // LIMB
	}); err != nil {
		return fmt.Errorf("failed to write pairing result: %w", err)
	}
	if err := w.Write([]string{
		strconv.Itoa(id),            // ID
		"0",                         // DATA_PAIRING_CHECK
		"1",                         // RSLT_PAIRING_CHECK
		"1",                         // INDEX
		"1",                         // CT
		"0",                         // IS_FIRST_INPUT
		"0",                         // IS_SECOND_INPUT
		"0",                         // NONTRIVIAL_POP_BIT
		formatBoolAsInt(hasPairing), // CS_PAIRING_CHECK
		"0",                         // CS_G1_MEMBERSHIP
		"0",                         // CS_G2_MEMBERSHIP
		formatBoolAsInt(tc.inputs[0].MembershipSuccess), // SUCCESS_BIT
		strconv.Itoa(len(tc.inputs)),                    // ACC_INPUTS
		formatBoolAsInt(tc.result),                      // LIMB
	}); err != nil {
		return fmt.Errorf("failed to write pairing result: %w", err)
	}
	return nil
}

func pairHeader() []string {
	return []string{
		"ID",
		"DATA_PAIRING_CHECK",
		"RSLT_PAIRING_CHECK",
		"INDEX",
		"CT",
		"IS_FIRST_INPUT",
		"IS_SECOND_INPUT",
		"NONTRIVIAL_POP_BIT",
		"CS_PAIRING_CHECK",
		"CS_G1_MEMBERSHIP",
		"CS_G2_MEMBERSHIP",
		"SUCCESS_BIT",
		"ACC_INPUTS",
		"LIMB",
	}
}

func mainPairing() error {
	var err error
	var f *os.File
	var w *csv.Writer

	id := 0
	fileNr := 0
	for pairingLength := 1; pairingLength <= maxNbPairingInputs; pairingLength++ {
		for _, v := range generatePairInputCases(pairingLength) {
			if id%nbPairingPerOutput == 0 {
				if f != nil {
					w.Flush()
					f.Close()
				}
				f, err = os.Create(fmt.Sprintf(path_pairing, fileNr))
				if err != nil {
					return fmt.Errorf("failed to create pairing file: %w", err)
				}
				w = csv.NewWriter(f)
				if err := w.Write(pairHeader()); err != nil {
					return fmt.Errorf("failed to write pairing header: %w", err)
				}
				fileNr++
			}
			if err := v.WriteCSV(w, id); err != nil {
				return fmt.Errorf("failed to write pairing case %d: %w", id, err)
			}
			id++
		}
	}
	if f != nil {
		w.Flush()
		f.Close()
	}
	return nil
}
