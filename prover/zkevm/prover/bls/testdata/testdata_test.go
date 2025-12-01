package main

import (
	"testing"

	bls12381 "github.com/consensys/gnark-crypto/ecc/bls12-381"
)

func TestGenerators(t *testing.T) {
	// --- trivial
	p := generateTrivial[bls12381.G1Affine]()
	if !p.IsInfinity() {
		t.Errorf("G1 trivial generator is not infinity: %s", p.String())
	}
	q := generateTrivial[bls12381.G2Affine]()
	if !q.IsInfinity() {
		t.Errorf("G2 trivial generator is not infinity: %s", q.String())
	}
	// -- valid points
	p = generateInSubgroup[bls12381.G1Affine]()
	if !p.IsOnCurve() {
		t.Errorf("generated G1 point is not on curve: %s", p.String())
	}
	if !p.IsInSubGroup() {
		t.Errorf("generated G1 point is not in subgroup: %s", p.String())
	}
	if p.IsInfinity() {
		t.Errorf("generated G1 point is infinity: %s", p.String())
	}
	q = generateInSubgroup[bls12381.G2Affine]()
	if !q.IsOnCurve() {
		t.Errorf("generated G2 point is not on curve: %s", q.String())
	}
	if !q.IsInSubGroup() {
		t.Errorf("generated G2 point is not in subgroup: %s", q.String())
	}
	if q.IsInfinity() {
		t.Errorf("generated G2 point is infinity: %s", q.String())
	}
	// -- point on curve not in subgroup
	p = generateOnCurve[bls12381.G1Affine]()
	if !p.IsOnCurve() {
		t.Errorf("generated G1 point is not on curve: %s", p.String())
	}
	if p.IsInSubGroup() {
		t.Errorf("generated G1 point is in subgroup: %s", p.String())
	}
	q = generateOnCurve[bls12381.G2Affine]()
	if !q.IsOnCurve() {
		t.Errorf("generated G2 point is not on curve: %s", q.String())
	}
	if q.IsInSubGroup() {
		t.Errorf("generated G2 point is in subgroup: %s", q.String())
	}
	// -- invalid points
	p = generateInvalid[bls12381.G1Affine]()
	if p.IsOnCurve() {
		t.Errorf("generated G1 point is on curve: %s", p.String())
	}
	if p.IsInSubGroup() {
		t.Errorf("generated G1 point is in subgroup: %s", p.String())
	}
	if p.IsInfinity() {
		t.Errorf("generated G1 point is infinity: %s", p.String())
	}
	q = generateInvalid[bls12381.G2Affine]()
	if q.IsOnCurve() {
		t.Errorf("generated G2 point is on curve: %s", q.String())
	}
	if q.IsInSubGroup() {
		t.Errorf("generated G2 point is in subgroup: %s", q.String())
	}
	if q.IsInfinity() {
		t.Errorf("generated G2 point is infinity: %s", q.String())
	}
}
