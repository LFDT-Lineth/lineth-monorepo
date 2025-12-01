package main

import (
	"encoding/csv"
	"fmt"
	"iter"
	"os"
	"strconv"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

type addInputType int

// for addition tests, we combine all possible cases from below
const (
	addTrivial    addInputType = iota // 0
	addOnCurve                        // P on curve not in subgroup
	addInSubgroup                     // P in subgroup
	addInvalid                        // P not on curve
)

type addInputCase[T affine] struct {
	left  addInputType
	right addInputType

	P, Q, Res T

	ToAddCircuit             bool
	LeftToCurveCheckCircuit  bool
	RightToCurveCheckCircuit bool
}

func generateAddInputCases[T affine]() iter.Seq2[int, addInputCase[T]] {
	return func(yield func(int, addInputCase[T]) bool) {
		var id int
		for _, left := range []addInputType{addTrivial, addOnCurve, addInSubgroup, addInvalid} {
			for _, right := range []addInputType{addTrivial, addOnCurve, addInSubgroup, addInvalid} {
				tc := addInputCase[T]{
					left:         left,
					right:        right,
					ToAddCircuit: true,
				}
				switch left {
				case addTrivial:
					tc.P = generateTrivial[T]()
				case addOnCurve:
					tc.P = generateOnCurve[T]()
				case addInSubgroup:
					tc.P = generateInSubgroup[T]()
				case addInvalid:
					tc.P = generateInvalid[T]()
					tc.LeftToCurveCheckCircuit = true
					tc.ToAddCircuit = false
				}
				switch right {
				case addTrivial:
					tc.Q = generateTrivial[T]()
				case addOnCurve:
					tc.Q = generateOnCurve[T]()
				case addInSubgroup:
					tc.Q = generateInSubgroup[T]()
				case addInvalid:
					tc.Q = generateInvalid[T]()
					if !tc.LeftToCurveCheckCircuit {
						// if the left point is invalid, we don't need to check the right point
						tc.RightToCurveCheckCircuit = true
						tc.ToAddCircuit = false
					}
				}
				if tc.left != addInvalid && tc.right != addInvalid {
					switch vv := any(&tc.Res).(type) {
					case *bls12381.G1Affine:
						vv.Add(any(&tc.P).(*bls12381.G1Affine), any(&tc.Q).(*bls12381.G1Affine))
					case *bls12381.G2Affine:
						vv.Add(any(&tc.P).(*bls12381.G2Affine), any(&tc.Q).(*bls12381.G2Affine))
					}
				}
				if !yield(id, tc) {
					return
				}
				id++
			}
		}
	}
}

func (a addInputCase[T]) WriteCSV(w *csv.Writer, id int) error {
	// columns:
	//  - id
	//  - data_T_add
	//  - RSLT_T_add
	//  - index (0-15 for inputs and then 0-7 for result)
	//  - ct (0-7 three times)
	//  - is_first_input
	//  - is_second_input
	//  - circuit_selector_T_add
	//  - circuit_selector_membership
	PLimbs := splitToLimbs(a.P)
	QLimbs := splitToLimbs(a.Q)
	ResLimbs := splitToLimbs(a.Res)

	records := make([][]string, len(PLimbs)+len(QLimbs)+len(ResLimbs))

	for i := range PLimbs {
		records[i] = []string{
			strconv.Itoa(id),
			"1",
			"0",
			strconv.Itoa(i),
			strconv.Itoa(i),
			"1",
			"0",
			formatBoolAsInt(a.ToAddCircuit),
			formatBoolAsInt(a.LeftToCurveCheckCircuit),
			PLimbs[i],
		}
	}
	for i := range QLimbs {
		records[len(PLimbs)+i] = []string{
			strconv.Itoa(id),
			"1",
			"0",
			strconv.Itoa(len(PLimbs) + i),
			strconv.Itoa(i),
			"0",
			"1",
			formatBoolAsInt(a.ToAddCircuit),
			formatBoolAsInt(a.RightToCurveCheckCircuit),
			QLimbs[i],
		}
	}
	for i := range ResLimbs {
		records[len(PLimbs)+len(QLimbs)+i] = []string{
			strconv.Itoa(id),
			"0",
			"1",
			strconv.Itoa(i),
			strconv.Itoa(i),
			"0",
			"0",
			formatBoolAsInt(a.ToAddCircuit),
			"0",
			ResLimbs[i],
		}
	}
	if err := w.WriteAll(records); err != nil {
		return fmt.Errorf("write all records: %w", err)
	}
	return nil
}

func headersAdd[T affine]() []string {
	var t, tt string
	switch any(new(T)).(type) {
	case *bls12381.G1Affine:
		t = "G1"
		tt = "C1"
	case *bls12381.G2Affine:
		t = "G2"
		tt = "C2"
	default:
		panic(fmt.Sprintf("unknown type for headersAdd: %T", new(T)))
	}
	return []string{
		"ID",
		fmt.Sprintf("DATA_%s_ADD", t),
		fmt.Sprintf("RSLT_%s_ADD", t),
		"INDEX",
		"CT",
		"IS_FIRST_INPUT",
		"IS_SECOND_INPUT",
		fmt.Sprintf("CIRCUIT_SELECTOR_%s_ADD", t),
		fmt.Sprintf("CIRCUIT_SELECTOR_%s_MEMBERSHIP", tt),
		"LIMB",
	}
}

func mainAdd() error {
	f, err := os.Create(path_add_g1)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(headersAdd[bls12381.G1Affine]()); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	for i, tc := range generateAddInputCases[bls12381.G1Affine]() {
		if err := tc.WriteCSV(w, i); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}

	f, err = os.Create(path_add_g2)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	w = csv.NewWriter(f)
	if err := w.Write(headersAdd[bls12381.G2Affine]()); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	for i, tc := range generateAddInputCases[bls12381.G2Affine]() {
		if err := tc.WriteCSV(w, i); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}
	return nil
}
