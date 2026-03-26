package fiatshamir

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir_bls12377"
	"github.com/consensys/linea-monorepo/prover/crypto/fiatshamir_koalabear"
	"github.com/consensys/linea-monorepo/prover/crypto/hasherfactory_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/fext"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
)

// circuit
type GnarkFS interface {
	Update(vec ...koalagnark.Element)
	UpdateExt(vec ...koalagnark.Ext)
	UpdateVec(mat ...[]koalagnark.Element)
	RandomField() koalagnark.Octuplet
	RandomFieldExt() koalagnark.Ext
	RandomManyIntegers(num, upperBound int) []frontend.Variable
	RandomFieldFromSeed(seed koalagnark.Octuplet, name string) koalagnark.Ext
	SetState(state koalagnark.Octuplet)
	State() koalagnark.Octuplet
}

// NewGnarkFSKoalabearFromFactory creates a Fiat-Shamir instance using the provided
// HasherFactory. This enables external hasher optimization when configured.
func NewGnarkKoalaFSFromFactory(api frontend.API, factory hasherfactory_koalabear.HasherFactory) GnarkFS {
	return fiatshamir_koalabear.NewGnarkFSFromFactory(api, factory)
}

func NewGnarkFSKoalabear(api frontend.API) GnarkFS {
	return fiatshamir_koalabear.NewGnarkFSWV(api)
}

func NewGnarkFSBLS12377(api frontend.API) GnarkFS {
	return fiatshamir_bls12377.NewGnarkFS(api)
}

// non circuit
type FS interface {
	Update(vec ...field.Element)
	UpdateExt(vec ...fext.Element)
	UpdateGeneric(vec ...fext.GenericFieldElem)
	UpdateSV(sv smartvectors.SmartVector)
	RandomField() field.Octuplet
	RandomFext() fext.Element
	RandomFieldFromSeed(seed field.Octuplet, name string) fext.Element
	RandomManyIntegers(num, upperBound int) []int
	SetState(s field.Octuplet)
	State() field.Octuplet
}

func NewFSKoalabear() FS {
	return fiatshamir_koalabear.NewFS()
}

func NewFSBls12377() FS {
	return fiatshamir_bls12377.NewFS()
}
