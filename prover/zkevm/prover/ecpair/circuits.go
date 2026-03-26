package ecpair

import (
	"fmt"
	"math/big"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra/emulated/fields_bn254"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/math/emulated/emparams"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

var fpParams sw_bn254.BaseField

type (
	fpField   = emulated.Field[emparams.BN254Fp]
	fpElement = emulated.Element[emparams.BN254Fp]
)

// G1ElementWizard represents G1 element as Wizard limbs (8 columns, 2 limbs of 128 bits)
type G1ElementWizard struct {
	PxHi, PxLo [common.NbLimbU128]frontend.Variable
	PyHi, PyLo [common.NbLimbU128]frontend.Variable
}

// ToG1Element converts G1ElementWizard to G1Affine used in circuit
func (c *G1ElementWizard) ToG1Element(api frontend.API, fp *emulated.Field[sw_bn254.BaseField]) sw_bn254.G1Affine {
	Px := gnarkutil.EmulatedFromHiLo(api, fp, c.PxHi[:], c.PxLo[:], 16)
	Py := gnarkutil.EmulatedFromHiLo(api, fp, c.PyHi[:], c.PyLo[:], 16)
	P := sw_bn254.G1Affine{
		X: *Px,
		Y: *Py,
	}
	return P
}

// G2ElementWizard represents G2 element as Wizard limbs (8 columns, 4 limbs of 16 bits)
type G2ElementWizard struct {
	QxBHi, QxBLo [common.NbLimbU128]frontend.Variable
	QxAHi, QxALo [common.NbLimbU128]frontend.Variable
	QyBHi, QyBLo [common.NbLimbU128]frontend.Variable
	QyAHi, QyALo [common.NbLimbU128]frontend.Variable
}

// ToG2Element converts G2ElementWizard to G2Affine used in circuit
func (c *G2ElementWizard) ToG2Element(api frontend.API, fp *emulated.Field[sw_bn254.BaseField]) sw_bn254.G2Affine {

	Qx := fields_bn254.E2{
		A0: *gnarkutil.EmulatedFromHiLo(api, fp, c.QxAHi[:], c.QxALo[:], 16),
		A1: *gnarkutil.EmulatedFromHiLo(api, fp, c.QxBHi[:], c.QxBLo[:], 16),
	}

	Qy := fields_bn254.E2{
		A0: *gnarkutil.EmulatedFromHiLo(api, fp, c.QyAHi[:], c.QyALo[:], 16),
		A1: *gnarkutil.EmulatedFromHiLo(api, fp, c.QyBHi[:], c.QyBLo[:], 16),
	}

	var Q sw_bn254.G2Affine
	Q.P.X = Qx
	Q.P.Y = Qy

	return Q
}

// GtElementWizard represents Gt element as Wizard limbs (24 limbs of 128 bits)
type GtElementWizard struct {
	// T represents the coordinates of the Gt element. They match the coordinates
	// of the Gt element on Ethereum but not on gnark.
	T [nbGtLimbs][common.NbLimbU128]frontend.Variable
}

// ToGtElement converts GtElementWizard to target group element used in circuit
func (c *GtElementWizard) ToGtElement(api frontend.API, fp *emulated.Field[sw_bn254.BaseField]) sw_bn254.GTEl {

	e12Tower := [12]*fpElement{
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[0][:], c.T[1][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[2][:], c.T[3][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[4][:], c.T[5][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[6][:], c.T[7][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[8][:], c.T[9][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[10][:], c.T[11][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[12][:], c.T[13][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[14][:], c.T[15][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[16][:], c.T[17][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[18][:], c.T[19][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[20][:], c.T[21][:], 16),
		gnarkutil.EmulatedFromHiLo(api, fp, c.T[22][:], c.T[23][:], 16),
	}

	return intoGtNoTower(fp, e12Tower)
}

// MultiG2GroupcheckCircuit is a circuit that checks multiple G2 group
// membership. Use [newMultiG2GroupcheckCircuit] to create a new instance with
// bounded number of allowed checks.
type MultiG2GroupcheckCircuit struct {
	Instances []G2GroupCheckInstance `gnark:",public"`
}

func newMultiG2GroupcheckCircuit(nbInstances int) *MultiG2GroupcheckCircuit {
	return &MultiG2GroupcheckCircuit{
		Instances: make([]G2GroupCheckInstance, nbInstances),
	}
}

func (c *MultiG2GroupcheckCircuit) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bn254.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field emulation: %w", err)
	}
	pairing, err := sw_bn254.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("instance %d check: %w", i, err)
		}
	}
	return nil
}

// G2GroupCheckInstance is a single instance of G2 group check.
type G2GroupCheckInstance struct {
	Q G2ElementWizard
	// IsSuccess is true if the point is on G2. It is formatted either as
	// [1, 0, 0, 0, 0, 0, 0, 0] or [0, 0, 0, 0, 0, 0, 0, 0] as it is in LE
	// limb format.
	IsSuccess [common.NbLimbU128]frontend.Variable
}

func (c *G2GroupCheckInstance) Check(api frontend.API, fp *emulated.Field[sw_bn254.BaseField], pairing *sw_bn254.Pairing) error {
	Q := c.Q.ToG2Element(api, fp)
	evmprecompiles.ECPairIsOnG2(api, &Q, c.IsSuccess[0])
	for i := 1; i < common.NbLimbU128; i++ {
		api.AssertIsEqual(c.IsSuccess[i], 0)
	}
	return nil
}

// MultiMillerLoopMulCircuit is a circuit that checks multiple Miller loop
// computation correctness. Use [newMultiMillerLoopMulCircuit] to create a new
// instance with bounded number of allowed checks.
type MultiMillerLoopMulCircuit struct {
	Instances []MillerLoopMulInstance `gnark:",public"`
}

func newMultiMillerLoopMulCircuit(nbInstance int) *MultiMillerLoopMulCircuit {
	return &MultiMillerLoopMulCircuit{
		Instances: make([]MillerLoopMulInstance, nbInstance),
	}
}

func (c *MultiMillerLoopMulCircuit) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bn254.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field emulation: %w", err)
	}
	pairing, err := sw_bn254.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("instance %d check: %w", i, err)
		}
	}
	return nil
}

// MillerLoopMulInstance is a single instance of Miller loop computation check.
type MillerLoopMulInstance struct {
	Prev    GtElementWizard
	P       G1ElementWizard
	Q       G2ElementWizard
	Current GtElementWizard
}

func (c *MillerLoopMulInstance) Check(api frontend.API, fp *emulated.Field[sw_bn254.BaseField], pairing *sw_bn254.Pairing) error {
	P := c.P.ToG1Element(api, fp)
	Q := c.Q.ToG2Element(api, fp)
	prev := c.Prev.ToGtElement(api, fp)
	current := c.Current.ToGtElement(api, fp)

	return evmprecompiles.ECPairMillerLoopAndMul(api, &prev, &P, &Q, &current)
}

// MultiMillerLoopFinalExpCircuit is a circuit that checks multiple Miller loop
// and final exponentiation checks. Use [newMultiMillerLoopFinalExpCircuit] to
// create a new instance with bounded number of allowed checks.
type MultiMillerLoopFinalExpCircuit struct {
	Instances []MillerLoopFinalExpInstance `gnark:",public"`
}

func newMultiMillerLoopFinalExpCircuit(nbInstance int) *MultiMillerLoopFinalExpCircuit {
	return &MultiMillerLoopFinalExpCircuit{
		Instances: make([]MillerLoopFinalExpInstance, nbInstance),
	}
}

func (c *MultiMillerLoopFinalExpCircuit) Define(api frontend.API) error {
	fp, err := emulated.NewField[sw_bn254.BaseField](api)
	if err != nil {
		return fmt.Errorf("new field emulation: %w", err)
	}
	pairing, err := sw_bn254.NewPairing(api)
	if err != nil {
		return fmt.Errorf("new pairing: %w", err)
	}
	for i := range c.Instances {
		if err := c.Instances[i].Check(api, fp, pairing); err != nil {
			return fmt.Errorf("instance %d check: %w", i, err)
		}
	}
	return nil
}

// MillerLoopFinalExpInstance is a single instance of Miller loop and final
// exponentiation check.
type MillerLoopFinalExpInstance struct {
	Prev GtElementWizard
	P    G1ElementWizard
	Q    G2ElementWizard
	// Expected is the expected result of the final exponentiation. The result
	// is over two limbs of 128 bits but it stores only a binary value.
	ExpectedHi, ExpectedLo [common.NbLimbU128]frontend.Variable
}

func (c *MillerLoopFinalExpInstance) Check(api frontend.API, fp *emulated.Field[sw_bn254.BaseField], pairing *sw_bn254.Pairing) error {
	P := c.P.ToG1Element(api, fp)
	Q := c.Q.ToG2Element(api, fp)
	prev := c.Prev.ToGtElement(api, fp)

	for _, l := range c.ExpectedHi[:] {
		api.AssertIsEqual(l, 0)
	}

	// Only the first limb corresponds to the success bit
	for _, l := range c.ExpectedLo[1:] {
		api.AssertIsEqual(l, 0)
	}

	return evmprecompiles.ECPairMillerLoopAndFinalExpCheck(api, &prev, &P, &Q, c.ExpectedLo[0])
}

// intoGtNoTower converts an E12 element as in the outputs of the pairing
// precompile on Ethereum into a non-tower representation of the same E12
// element.
func intoGtNoTower(api *fpField, coordinates [12]*fpElement) sw_bn254.GTEl {

	var (
		C0B0X = coordinates[0]
		C0B0Y = coordinates[1]
		C0B1X = coordinates[2]
		C0B1Y = coordinates[3]
		C0B2X = coordinates[4]
		C0B2Y = coordinates[5]
		C1B0X = coordinates[6]
		C1B0Y = coordinates[7]
		C1B1X = coordinates[8]
		C1B1Y = coordinates[9]
		C1B2X = coordinates[10]
		C1B2Y = coordinates[11]
	)

	var t *fpElement
	t = api.MulConst(C0B0Y, big.NewInt(9))
	c0 := api.Sub(C0B0X, t)
	t = api.MulConst(C1B0Y, big.NewInt(9))
	c1 := api.Sub(C1B0X, t)
	t = api.MulConst(C0B1Y, big.NewInt(9))
	c2 := api.Sub(C0B1X, t)
	t = api.MulConst(C1B1Y, big.NewInt(9))
	c3 := api.Sub(C1B1X, t)
	t = api.MulConst(C0B2Y, big.NewInt(9))
	c4 := api.Sub(C0B2X, t)
	t = api.MulConst(C1B2Y, big.NewInt(9))
	c5 := api.Sub(C1B2X, t)

	return sw_bn254.GTEl{
		A0:  *c0,
		A1:  *c1,
		A2:  *c2,
		A3:  *c3,
		A4:  *c4,
		A5:  *c5,
		A6:  *C0B0Y,
		A7:  *C1B0Y,
		A8:  *C0B1Y,
		A9:  *C1B1Y,
		A10: *C0B2Y,
		A11: *C1B2Y,
	}
}
