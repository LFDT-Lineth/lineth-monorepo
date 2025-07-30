//go:build !fuzzlight

package keccakf

import (
	"testing"

	"github.com/consensys/linea-monorepo/prover/crypto/keccak"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/stretchr/testify/assert"
)

func TestLookupsBaseAToBaseB(t *testing.T) {

	baseADirty, baseBClean := valBaseXToBaseY(BaseA, BaseB, 0)

	for i := 0; i < BaseAPow4; i++ {
		// baseADirty is equal to i
		dirtyA := baseADirty.Get(i)
		assert.Equal(t, uint64(i), dirtyA.Uint64(), "base A dirty")

		// cleanB is consistent with the declaration that dirty
		cleanB := baseBClean.Get(i)
		assert.Equal(t, BaseXToU64(dirtyA, &BaseAFr), BaseXToU64(cleanB, &BaseBFr), "base B clean")

		if t.Failed() {
			t.Fatalf("BaseA to BaseB failed")
		}
	}
}

func TestLookupsBaseBToBaseA(t *testing.T) {

	baseBDirty, baseAClean := valBaseXToBaseY(BaseB, BaseA, 1)

	for i := 0; i < BaseBPow4; i++ {
		// baseBDirty is equal to i
		dirtyB := baseBDirty.Get(i)
		assert.Equal(t, uint64(i), dirtyB.Uint64(), "base B dirty")

		// cleanA is consistent with the declaration that dirty
		cleanA := baseAClean.Get(i)
		assert.Equal(
			t,
			BaseXToU64(dirtyB, &BaseBFr, 1),
			BaseXToU64(cleanA, &BaseAFr),
			"base A clean",
		)

		if t.Failed() {
			t.Fatalf("BaseB to BaseA failed at %v", i)
		}
	}
}

func TestVectorBaseBToBaseA(t *testing.T) {

	baseBDirty, baseAClean := valBaseXToBaseY(BaseB, BaseA, 1)

	cases := []struct {
		pos   int
		baseB field.Element
		baseA field.Element
	}{
		{pos: 1872, baseB: field.NewElement(1872), baseA: field.NewElement(1)},
		{pos: 642, baseB: field.NewElement(642), baseA: field.NewElement(12)},
		{pos: 7777, baseB: field.NewElement(7777), baseA: field.NewElement(12)},
	}

	for _, c := range cases {
		dirtyB, cleanA := baseBDirty.Get(c.pos), baseAClean.Get(c.pos)
		assert.Equal(t, c.baseB.String(), dirtyB.String(), "wrong baseB")
		assert.Equal(t, c.baseA.String(), cleanA.String(), "wrong baseA")
	}
}

func TestRC(t *testing.T) {

	rc := valRCBase2Pattern()

	for i := range rc {
		expected := keccak.RC[i]
		actual := BaseXToU64(rc[i], &BaseBFr)
		assert.Equal(t, expected, actual, "row %v", i)
	}
}
