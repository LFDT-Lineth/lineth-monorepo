package main

import (
	"encoding/csv"
	"fmt"
	"iter"
	"os"
	"strconv"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
	fp_bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381/fp"
)

type mapInputType int

const (
	mapTrivial mapInputType = iota // 0
	mapInRange                     // element is in range of the field
)

type mapInput[T affine] struct {
	P   []fp_bls12381.Element // element to map. In case for G1 it is 1, in case for G2 it is 2
	Res T

	ToMapCircuit bool // whether to map. Always true, we don't perform error checks in circuit
}

func generateTrivialMapInput[T affine]() [][]fp_bls12381.Element {
	var res [][]fp_bls12381.Element
	switch any(new(T)).(type) {
	case *bls12381.G1Affine:
		res = append(res, []fp_bls12381.Element{fp_bls12381.NewElement(0)})
	case *bls12381.G2Affine:
		v0 := make([]fp_bls12381.Element, 2)
		v0[0].SetZero()
		v0[1].SetZero()
		res = append(res, v0)

		v1 := make([]fp_bls12381.Element, 2)
		v1[0].SetZero()
		v1[1].MustSetRandom()
		res = append(res, v1)

		v2 := make([]fp_bls12381.Element, 2)
		v2[0].MustSetRandom()
		v2[1].SetZero()
		res = append(res, v2)
	}
	return res
}

func generateNonTrivialMapInput[T affine]() []fp_bls12381.Element {
	var v []fp_bls12381.Element
	switch any(new(T)).(type) {
	case *bls12381.G1Affine:
		v = make([]fp_bls12381.Element, 1)
		v[0].MustSetRandom()
	case *bls12381.G2Affine:
		v = make([]fp_bls12381.Element, 2)
		v[0].MustSetRandom()
		v[1].MustSetRandom()
	}
	return v
}

func computeMap[T affine](p []fp_bls12381.Element) mapInput[T] {
	res := mapInput[T]{
		P:            p,
		ToMapCircuit: true,
	}
	switch vv := any(&res.Res).(type) {
	case *bls12381.G1Affine:
		if len(p) != 1 {
			panic("G1 map expects 1 element")
		}
		*vv = bls12381.MapToG1(p[0])
	case *bls12381.G2Affine:
		if len(p) != 2 {
			panic("G2 map expects 2 elements")
		}
		*vv = bls12381.MapToG2(bls12381.E2{A0: p[0], A1: p[1]})
	}
	return res
}

func generateMapInput[T affine](nbNonTrivial int) iter.Seq2[int, mapInput[T]] {
	return func(yield func(int, mapInput[T]) bool) {
		var id int
		for _, inputType := range []mapInputType{mapTrivial, mapInRange} {
			if inputType == mapTrivial {
				inputs := generateTrivialMapInput[T]()
				outputs := make([]mapInput[T], len(inputs))
				for i := range inputs {
					outputs[i] = computeMap[T](inputs[i])
				}
				for _, output := range outputs {
					if !yield(id, output) {
						return
					}
					id++
				}
			} else if inputType == mapInRange {
				for i := 0; i < nbNonTrivial; i++ {
					input := generateNonTrivialMapInput[T]()
					output := computeMap[T](input)
					if !yield(id, output) {
						return
					}
					id++
				}
			}
		}
	}
}

func (tc mapInput[T]) WriteCSV(w *csv.Writer, id int) error {
	var index int
	for i := range tc.P {
		plimbs := splitBaseToLimbs(tc.P[i])
		for j := range plimbs {
			if err := w.Write([]string{
				strconv.Itoa(id),                 // ID
				"1",                              // DATA_MAP_T_TO_T
				"0",                              // RSLT_MAP_T_TO_T
				strconv.Itoa(index),              // INDEX
				strconv.Itoa(index),              // CT. CT==INDEX
				"1",                              // IS_FIRST_INPUT
				"0",                              // IS_SECOND_INPUT. Never have second input
				formatBoolAsInt(tc.ToMapCircuit), // CIRCUIT_SELECTOR_MAP_T_TO_T
				plimbs[j],                        // LIMB
			}); err != nil {
				return fmt.Errorf("write map input: %w", err)
			}
			index++
		}
	}
	resLimbs := splitToLimbs(tc.Res)
	for i := range resLimbs {
		if err := w.Write([]string{
			strconv.Itoa(id),                 // ID
			"0",                              // DATA_MAP_T_TO_T
			"1",                              // RSLT_MAP_T_TO_T
			strconv.Itoa(i),                  // INDEX
			strconv.Itoa(i),                  // CT. CT==INDEX
			"0",                              // IS_FIRST_INPUT
			"0",                              // IS_SECOND_INPUT. Never have second input
			formatBoolAsInt(tc.ToMapCircuit), // CIRCUIT_SELECTOR_MAP_T_TO_T
			resLimbs[i],                      // LIMB
		}); err != nil {
			return fmt.Errorf("write map result: %w", err)
		}
	}
	return nil
}

func headerMap[T affine]() []string {
	var t, tt string
	switch any(new(T)).(type) {
	case *bls12381.G1Affine:
		t = "G1"
		tt = "FP"
	case *bls12381.G2Affine:
		t = "G2"
		tt = "FP2"
	default:
		panic("unknown type")
	}
	return []string{
		"ID",
		fmt.Sprintf("DATA_MAP_%s_TO_%s", tt, t),
		fmt.Sprintf("RSLT_MAP_%s_TO_%s", tt, t),
		"INDEX",
		"CT",
		"IS_FIRST_INPUT",
		"IS_SECOND_INPUT",
		fmt.Sprintf("CIRCUIT_SELECTOR_MAP_%s_TO_%s", tt, t),
		"LIMB",
	}
}

func mainMap() error {
	f, err := os.Create(path_map_g1)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	w := csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(headerMap[bls12381.G1Affine]()); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	for i, tc := range generateMapInput[bls12381.G1Affine](nbRepetitionsMap) {
		if err := tc.WriteCSV(w, i); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}
	f, err = os.Create(path_map_g2)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer f.Close()
	w = csv.NewWriter(f)
	defer w.Flush()
	if err := w.Write(headerMap[bls12381.G2Affine]()); err != nil {
		return fmt.Errorf("write headers: %w", err)
	}
	for i, tc := range generateMapInput[bls12381.G2Affine](nbRepetitionsMap) {
		if err := tc.WriteCSV(w, i); err != nil {
			return fmt.Errorf("write csv: %w", err)
		}
	}
	return nil
}
