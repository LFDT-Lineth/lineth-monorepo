package ringsis

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Standard parameter that we use for ring-SIS they are benchmarked at achieve
// more than the 128 level of security.
var StdParams = Params{LogTwoBound: 16, LogTwoDegree: 6}

// Params encapsulates the parameters of a ring SIS instance
type Params struct {
	LogTwoBound, LogTwoDegree int
}

// NumLimbs number of limbs to represent a field element with the current
// representation
func (p *Params) NumLimbs() int {
	return utils.DivCeil(field.Bytes*8, p.LogTwoBound)
}

// Returns the number of field element composing the output. This coincides
// with the degree of the modulus polynomial characterizing the ring in which
// we instantiate ring-SIS.
func (p *Params) OutputSize() int {
	return 1 << p.LogTwoDegree
}

// modulusDegree returns the degree of the modulus polynomial and coincide with
// [Params.OutputSize]. The motivation for this method to exists is that it states
// more obviously that we are refering to the modulus polynomial degree and not
// the size of the output of the SIS hash itself.
func (p *Params) modulusDegree() int {
	return p.OutputSize()
}

// MaxNumFieldHashable returns a positive integer indicating how many field
// elements can be provided to the hasher together at once in a single hash.
func (key *Key) MaxNumFieldHashable() int {

	var (
		// numLimbsTotal counts the number of ring elements totalling the SIS key
		numLimbsTotal = key.maxNumLimbsHashable()
		// numLimbsPerField contains the number of limbs needed to represent one field
		// element. The DivCeil is important because we want to ensure that
		// *all* the bits of the inbound field element can be represented in
		// `numLimbsPerField`.
		numLimbsPerField = utils.DivCeil(8*field.Bytes, key.LogTwoBound)
	)

	return numLimbsTotal / numLimbsPerField
}

// maxNumLimbsHashable returns the total number of small norm limbs that can
// be hashed together in a single hash. This coincides with the total size of
// the SIS key counting the field elements composing it.
func (key *Key) maxNumLimbsHashable() int {
	return key.modulusDegree() * len(key.gnarkInternal.A)
}

// NumFieldPerPoly returns the number of field elements that can be hashed with
// a single polynomial accumulation. The function returns 0 is a single field
// requires more than one polynomial.
func (p *Params) NumFieldPerPoly() int {
	return (p.OutputSize() * p.LogTwoBound) / (8 * field.Bytes)
}
