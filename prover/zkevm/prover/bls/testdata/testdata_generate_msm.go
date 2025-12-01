package main

import (
	"encoding/csv"
	"fmt"
	"iter"
	"math/big"
	"os"
	"strconv"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type msmInputType int

const (
	msmScalarTrivial msmInputType = iota // 0
	msmScalarRange                       // scalar is in range of the scalar field
	msmScalarBig                         // scalar is not in range of the scalar field. But it is still a valid scalar

	msmPointTrivial    // point is 0
	msmPointOnCurve    // point is on curve but not in subgroup
	msmPointInSubgroup // point is in subgroup
	msmPointInvalid    // point is not on curve
)

type msmInput[T affine] struct {
	scalar msmInputType
	point  msmInputType

	n *big.Int
	P T

	ToMSMCircuit        bool
	ToGroupCheckCircuit bool
}

type msmInputCase[T affine] struct {
	inputs []msmInput[T]

	Res T
}

func generateMsmInput[T affine]() iter.Seq2[int, func() msmInput[T]] {
	return func(yield func(int, func() msmInput[T]) bool) {
		var id int
		for _, scalar := range []msmInputType{msmScalarTrivial, msmScalarRange, msmScalarBig} {
			for _, point := range []msmInputType{msmPointTrivial, msmPointOnCurve, msmPointInSubgroup, msmPointInvalid} {
				tcf := func() msmInput[T] {
					tc := msmInput[T]{
						scalar:       scalar,
						point:        point,
						n:            generateScalar(scalar),
						ToMSMCircuit: true,
					}
					switch point {
					case msmPointTrivial:
						tc.P = generateTrivial[T]()
					case msmPointOnCurve:
						tc.P = generateOnCurve[T]()
					case msmPointInSubgroup:
						tc.P = generateInSubgroup[T]()
					case msmPointInvalid:
						tc.P = generateInvalid[T]()
					}
					return tc
				}
				if !yield(id, tcf) {
					return
				}
				id++
			}
		}
	}
}

func generateMsmInputCases[T affine](msmLength int) iter.Seq2[int, msmInputCase[T]] {
	return func(yield func(int, msmInputCase[T]) bool) {
		var id int
		for v := range cartesianProduct(msmLength, generateMsmInput[T]) {
			tc := msmInputCase[T]{
				inputs: v,
			}
			var hasFailure bool
			for _, input := range v {
				if input.point == msmPointInvalid || input.point == msmPointOnCurve {
					hasFailure = true
				}
			}
			// in case of failure, we don't need to compute the result. Also we
			// don't need to send the inputs to the MSM circuit, but we need to
			// send the first failing input to the group check circuit
			if hasFailure {
				for i := range tc.inputs {
					tc.inputs[i].ToMSMCircuit = false
				}
				for i := range tc.inputs {
					if tc.inputs[i].point == msmPointInvalid || tc.inputs[i].point == msmPointOnCurve {
						tc.inputs[i].ToGroupCheckCircuit = true
						break
					}
				}
			} else {
				switch vv := any(&tc.Res).(type) {
				case *bls12381.G1Affine:
					for _, input := range v {
						var tmp bls12381.G1Affine
						tmp.ScalarMultiplication(any(&input.P).(*bls12381.G1Affine), input.n)
						vv.Add(vv, &tmp)
					}
				case *bls12381.G2Affine:
					for _, input := range v {
						var tmp bls12381.G2Affine
						tmp.ScalarMultiplication(any(&input.P).(*bls12381.G2Affine), input.n)
						vv.Add(vv, &tmp)
					}
				}
			}
			if !yield(id, tc) {
				return
			}
			id++
		}
	}
}

func (tc msmInputCase[T]) WriteCSV(w *csv.Writer, id int) error {
	// columns:
	// - ID
	// - data_T_MSM
	// - RSLT_T_MSM
	// - INDEX (0-N for the inputs, then 0-7 for the result)
	// - CT (0-7 for point, 0-1 for scalar, 0-7 for result)
	// - IS_FIRST_INPUT point
	// - IS_SECOND_INPUT scalar
	// - CIRCUIT_SELECTOR_T_MSM - in case all goes to MSM
	// - CIRCUIT_SELECTOR_T_MEMBERSHIP - in case goes to subgroup non-membership check
	// - LIMB - limb of the scalar or point
	var index int
	var hasGroupCheck bool
	for _, input := range tc.inputs {
		hasGroupCheck = hasGroupCheck || input.ToGroupCheckCircuit
		nLimbs := splitScalarToLimbs(input.n)
		pLimbs := splitToLimbs(input.P)
		for j, limb := range pLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),
				"1",
				"0",
				strconv.Itoa(index),
				strconv.Itoa(j),
				"1",
				"0",
				formatBoolAsInt(input.ToMSMCircuit),
				formatBoolAsInt(input.ToGroupCheckCircuit),
				limb,
			}); err != nil {
				return fmt.Errorf("write point limb: %w", err)
			}
			index++
		}
		for j, limb := range nLimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),
				"1",
				"0",
				strconv.Itoa(index),
				strconv.Itoa(j),
				"0",
				"1",
				formatBoolAsInt(input.ToMSMCircuit),
				"0", // formatBoolAsInt(input.ToGroupCheckCircuit), /* scalars never go to group check circuit */
				limb,
			}); err != nil {
				return fmt.Errorf("write scalar limb: %w", err)
			}
			index++
		}
	}
	resLimbs := splitToLimbs(tc.Res)
	for j, limb := range resLimbs {
		if err := w.Write([]string{
			strconv.Itoa(id),
			"0",
			"1",
			strconv.Itoa(j),
			strconv.Itoa(j),
			"0",
			"0",
			formatBoolAsInt(!hasGroupCheck),
			formatBoolAsInt(false),
			limb,
		}); err != nil {
			return fmt.Errorf("write result limb: %w", err)
		}
		index++
	}
	return nil
}

func headersMsm[T affine]() []string {
	var t string
	switch any(new(T)).(type) {
	case *bls12381.G1Affine:
		t = "G1"
	case *bls12381.G2Affine:
		t = "G2"
	default:
		panic("unknown type")
	}
	return []string{
		"ID",
		fmt.Sprintf("DATA_%s_MSM", t),
		fmt.Sprintf("RSLT_%s_MSM", t),
		"INDEX",
		"CT",
		"IS_FIRST_INPUT",
		"IS_SECOND_INPUT",
		fmt.Sprintf("CIRCUIT_SELECTOR_%s_MSM", t),
		fmt.Sprintf("CIRCUIT_SELECTOR_%s_MEMBERSHIP", t),
		"LIMB",
	}
}

func mainMsmGroup[T affine](path string, maxLength int) error {
	var err error
	var f *os.File
	var w *csv.Writer
	id := 0
	fileNr := 0

	for msmLength := 1; msmLength <= maxLength; msmLength++ {
		for _, tc := range generateMsmInputCases[T](msmLength) {
			if id%nbMsmPerOutput == 0 {
				if f != nil {
					w.Flush()
					f.Close()
				}
				f, err = os.Create(fmt.Sprintf(path, fileNr))
				if err != nil {
					return fmt.Errorf("create file: %w", err)
				}
				w = csv.NewWriter(f)
				if err := w.Write(headersMsm[T]()); err != nil {
					return fmt.Errorf("write headers: %w", err)
				}
				fileNr++
			}
			if err := tc.WriteCSV(w, id); err != nil {
				return fmt.Errorf("write csv: %w", err)
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

func mainMsm() error {
	if err := mainMsmGroup[bls12381.G1Affine](path_msm_g1, maxNbMsmInputs); err != nil {
		return fmt.Errorf("generate g1 msm: %w", err)
	}
	if err := mainMsmGroup[bls12381.G2Affine](path_msm_g2, maxNbMsmInputs); err != nil {
		return fmt.Errorf("generate g2 msm: %w", err)
	}
	return nil
}
