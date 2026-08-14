package proofserialization_test

import (
	"os"
	"regexp"
	"strconv"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/stretchr/testify/require"
)

// linkerScriptPath defines the guest's memory map, and therefore the address the
// image's pointers must be relocated for.
const linkerScriptPath = "../../../riscv-guests/build_common/linker_script.ld"

// TestGuestBaseMatchesLinkerScript checks GuestBase and MaxImageSize against the
// guest linker script.
//
// Nothing else pins them: every other test relocates relative to whatever
// GuestBase happens to be, so changing it keeps all of them green while making
// every produced image point at the wrong region. A mutation run confirmed that
// blind spot.
func TestGuestBaseMatchesLinkerScript(t *testing.T) {
	src, err := os.ReadFile(linkerScriptPath)
	if err != nil {
		t.Skipf("riscv-guests not checked out alongside prover-ray (%v); "+
			"this check needs %s", err, linkerScriptPath)
	}

	// IN      (r)  : ORIGIN = 0x08800000, LENGTH = 0x40000000
	re := regexp.MustCompile(`IN\s+\(r\)\s*:\s*ORIGIN\s*=\s*(0x[0-9a-fA-F]+),\s*LENGTH\s*=\s*(0x[0-9a-fA-F]+)`)
	m := re.FindStringSubmatch(string(src))
	require.Len(t, m, 3, "could not find the IN region in %s; has the memory map been "+
		"restructured?", linkerScriptPath)

	origin, err := strconv.ParseUint(m[1][2:], 16, 64)
	require.NoError(t, err)
	length, err := strconv.ParseUint(m[2][2:], 16, 64)
	require.NoError(t, err)

	require.Equal(t, origin, uint64(ps.GuestBase),
		"GuestBase must equal ORIGIN(IN) from %s: the guest casts &_in_start, so an image "+
			"relocated for any other address has every pointer wrong", linkerScriptPath)
	require.Equal(t, length, uint64(ps.MaxImageSize),
		"MaxImageSize must equal LENGTH(IN) from %s, or Encode's size check is wrong",
		linkerScriptPath)
}

// TestExtFrom checks every coordinate is carried across, in the right order.
//
// Six distinct values, so dropping or transposing any one of them fails. The
// conversion is deliberately not an unsafe cast — field.Ext and ps.Ext happen to
// have the same layout today, and relying on that would break silently.
func TestExtFrom(t *testing.T) {
	var e field.Ext
	e.B0.A0 = field.NewElement(11)
	e.B0.A1 = field.NewElement(22)
	e.B1.A0 = field.NewElement(33)
	e.B1.A1 = field.NewElement(44)
	e.B2.A0 = field.NewElement(55)
	e.B2.A1 = field.NewElement(66)

	got := ps.ExtFrom(e)

	// Montgomery form is preserved verbatim, so the expectation is the raw limbs
	// rather than the logical values.
	want := ps.Ext{
		ps.Element(e.B0.A0[0]), ps.Element(e.B0.A1[0]),
		ps.Element(e.B1.A0[0]), ps.Element(e.B1.A1[0]),
		ps.Element(e.B2.A0[0]), ps.Element(e.B2.A1[0]),
	}
	require.Equal(t, want, got, "every coordinate must survive, in memory order")

	// Guard the expectation itself: if the six limbs were not distinct, the
	// assertion above could not detect a transposition.
	seen := map[ps.Element]bool{}
	for _, x := range want {
		require.False(t, seen[x], "fixture limbs must be distinct for this test to have power")
		seen[x] = true
	}
}

func TestDigestFrom(t *testing.T) {
	var o field.Octuplet
	for i := range o {
		o[i] = field.NewElement(uint64(100 + i))
	}

	got := ps.DigestFrom(o)
	for i := range o {
		require.Equal(t, ps.Element(o[i][0]), got[i], "digest limb %d must be carried across", i)
	}
}

func TestElementsFrom(t *testing.T) {
	in := []field.Element{field.NewElement(1), field.NewElement(2), field.NewElement(3)}

	got := ps.ElementsFrom(in)
	require.Len(t, got, len(in))
	for i := range in {
		require.Equal(t, ps.Element(in[i][0]), got[i], "element %d", i)
	}

	require.Nil(t, ps.ElementsFrom(nil), "nil in, nil out")
}

func TestExtsFrom(t *testing.T) {
	var a, b field.Ext
	a.B0.A0 = field.NewElement(7)
	b.B2.A1 = field.NewElement(9)

	got := ps.ExtsFrom([]field.Ext{a, b})
	require.Equal(t, []ps.Ext{ps.ExtFrom(a), ps.ExtFrom(b)}, got,
		"each element must go through the same coordinate mapping")

	require.Nil(t, ps.ExtsFrom(nil), "nil in, nil out")
}

func TestDigestsFrom(t *testing.T) {
	var o field.Octuplet
	o[3] = field.NewElement(5)

	got := ps.DigestsFrom([]field.Octuplet{o})
	require.Equal(t, []ps.Digest{ps.DigestFrom(o)}, got)

	require.Nil(t, ps.DigestsFrom(nil), "nil in, nil out")
}

// TestScalarFrom_CarriesValue complements TestScalarFrom_InvertsGoTag, which only
// pins the discriminant. Without this, a conversion that got the tag right and
// the value wrong would pass.
func TestScalarFrom_CarriesValue(t *testing.T) {
	var e field.Ext
	e.B0.A0 = field.NewElement(11)
	e.B1.A1 = field.NewElement(22)
	e.B2.A0 = field.NewElement(33)

	got := ps.ScalarFrom(field.ElemFromExt(e))
	require.Equal(t, ps.ExtFrom(e), got.Value, "the 24-byte payload must be carried across")
	require.True(t, got.IsExt)
}
