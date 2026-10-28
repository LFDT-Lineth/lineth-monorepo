package fiatshamir

import (
	"fmt"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/stretchr/testify/require"
)

// TestTranscriptGolden pins the KoalaBear transcript outputs, recorded before
// the transcript was made generic, so that any change of challenges is caught.
func TestTranscriptGolden(t *testing.T) {
	fs := NewFiatShamir()
	for i := range 11 {
		fs.Update(field.NewElement(uint64(i + 1)))
	}
	fs.UpdateExt(field.UintsToExt(1, 2, 3, 4, 5, 6))

	d := fs.RandomDigest()
	require.Equal(t, [32]byte{120, 31, 225, 243, 70, 110, 75, 98, 22, 119, 2, 242, 25, 191, 244, 183,
		5, 7, 38, 131, 53, 169, 69, 81, 45, 108, 218, 236, 60, 165, 154, 131}, field.OctupletToBytes(d))

	e := fs.RandomFext()
	require.Equal(t, "2010086283+2119284141*u+(439447832+1966743544*u)*v+(1879668421+1100833276*u)*v**2",
		e.String())

	require.Equal(t, "[95 147 870 19 126 259 486 984 1008 1012]",
		fmt.Sprint(fs.RandomManyIntegers(10, 1<<10)))

	s := fs.RandomFextFromSeed(d, "alpha")
	require.Equal(t, "317436949+587897565*u+(1505446148+937508824*u)*v+(2115335158+2000288874*u)*v**2",
		s.String())

	// RandomFextFromSeed must leave the transcript untouched.
	e2 := fs.RandomFext()
	require.Equal(t, "1454524612+88113669*u+(542841426+367948402*u)*v+(1550086187+2002470665*u)*v**2",
		e2.String())
}
