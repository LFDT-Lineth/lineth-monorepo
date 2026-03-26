package ecdsa

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/emulated/sw_emulated"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/math/emulated/emparams"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

type EcRecoverInstance struct {
	PKXHi, PKXLo, PKYHi, PKYLo [common.NbLimbU128]frontend.Variable `gnark:",public"`
	HHi, HLo                   [common.NbLimbU128]frontend.Variable `gnark:",public"`
	VHi, VLo                   [common.NbLimbU128]frontend.Variable `gnark:",public"`
	RHi, RLo                   [common.NbLimbU128]frontend.Variable `gnark:",public"`
	SHi, SLo                   [common.NbLimbU128]frontend.Variable `gnark:",public"`
	SUCCESS_BIT                [common.NbLimbU128]frontend.Variable `gnark:",public"`
	ECRECOVERBIT               [common.NbLimbU128]frontend.Variable `gnark:",public"`
}

type MultiEcRecoverCircuit struct {
	Instances []EcRecoverInstance `gnark:",public"`
}

func newMultiEcRecoverCircuit(nbInstances int) *MultiEcRecoverCircuit {
	return &MultiEcRecoverCircuit{
		Instances: make([]EcRecoverInstance, nbInstances),
	}
}

func (c *MultiEcRecoverCircuit) Define(api frontend.API) error {
	curve, err := algebra.GetCurve[emparams.Secp256k1Fr, sw_emulated.AffinePoint[emparams.Secp256k1Fp]](api)
	if err != nil {
		return fmt.Errorf("get curve: %w", err)
	}
	for i := 0; i < len(c.Instances); i++ {
		PK, msg, v, r, s, strictRange, isFailure, err := c.Instances[i].splitInputs(api)
		if err != nil {
			return fmt.Errorf("split inputs: %w", err)
		}
		recovered := evmprecompiles.ECRecover(api, *msg, v, *r, *s, strictRange, isFailure)
		curve.AssertIsEqual(PK, recovered)
	}
	return nil
}

func (c *EcRecoverInstance) splitInputs(api frontend.API) (PK *sw_emulated.AffinePoint[emparams.Secp256k1Fp], msg *emulated.Element[emparams.Secp256k1Fr], v frontend.Variable, r, s *emulated.Element[emparams.Secp256k1Fr], strictRange, isFailure frontend.Variable, err error) {
	fr, err2 := emulated.NewField[emparams.Secp256k1Fr](api)
	if err2 != nil {
		err = fmt.Errorf("field emulation: %w", err2)
		return
	}
	fp, err2 := emulated.NewField[emparams.Secp256k1Fp](api)
	if err2 != nil {
		err = fmt.Errorf("field emulation: %w", err2)
		return
	}
	// Convert big-endian 16×16-bit limbs to little-endian 26×10-bit limbs
	// for gnark's emulated field over KoalaBear
	msg = gnarkutil.EmulatedFromHiLo(api, fr, c.HHi[:], c.HLo[:], 16)
	PK = &sw_emulated.AffinePoint[emparams.Secp256k1Fp]{
		X: *gnarkutil.EmulatedFromHiLo(api, fp, c.PKXHi[:], c.PKXLo[:], 16),
		Y: *gnarkutil.EmulatedFromHiLo(api, fp, c.PKYHi[:], c.PKYLo[:], 16),
	}

	// v is 27 or 28 in EVM, but arithmetization gives on two limbs. Ensure that all high limbs are zero.
	for i := 0; i < common.NbLimbU128; i++ {
		api.AssertIsEqual(c.VHi[i], 0)
	}
	// Also assert all but the last low limb are zero (v fits in ~5 bits)
	for i := 1; i < common.NbLimbU128; i++ {
		api.AssertIsEqual(c.VLo[i], 0)
	}
	v = c.VLo[0] // last limb since v is small (27 or 28).

	// Convert r and s similarly
	r = gnarkutil.EmulatedFromHiLo(api, fr, c.RHi[:], c.RLo[:], 16)
	s = gnarkutil.EmulatedFromHiLo(api, fr, c.SHi[:], c.SLo[:], 16)

	for i := 1; i < common.NbLimbU128; i++ {
		api.AssertIsEqual(c.SUCCESS_BIT[i], 0)
		api.AssertIsEqual(c.ECRECOVERBIT[i], 0)
	}

	// SUCCESS_BIT indicates if the input is a valid signature (1 for valid, 0
	// for invalid). Recall that we also allow to verify invalid signatures (for
	// the ECRECOVER precompile call).
	isFailure = api.Sub(1, c.SUCCESS_BIT[0])
	// ECRECOVERBIT indicates if the input comes from the ECRECOVER precompile
	// or not (1 for ECRECOVER, 0 for TX).
	strictRange = api.Sub(1, c.ECRECOVERBIT[0])

	return
}

var (
	plonkInputFillerKey = "ecdsa-secp256k1-plonk-input-filler"
)

func init() {
	plonk.RegisterInputFiller(plonkInputFillerKey, PlonkInputFiller)
}

// nbInputsPerInstance is the total number of public inputs per EcRecoverInstance.
// Layout: PKXHi[8], PKXLo[8], PKYHi[8], PKYLo[8], HHi[8], HLo[8], VHi[8], VLo[8],
//
//	RHi[8], RLo[8], SHi[8], SLo[8], SuccessBitUnused[7], SUCCESS_BIT, EcrecoverBitUnused[7], ECRECOVERBIT
//
// Total: 12 * 8 + 7 + 1 + 7 + 1 = 112 inputs
const nbInputsPerInstance = 14 * common.NbLimbU128

// PlonkInputFiller is the input-filler that we use to assign the public inputs
// of incomplete circuits. This function must be registered via the
// [plonk.RegisterInputFiller] via the [init] function. But this has to be done
// manually if the package is not imported.
func PlonkInputFiller(circuitInstance, inputIndex int) field.Element {

	fillingData := []uint32{
		// public key: 1*G
		0x0b07, 0xce87, 0x6295, 0x55a0, 0xbbac, 0xf9dc, 0x667e, 0x79be,
		0x1798, 0x16f8, 0x815b, 0x59f2, 0x28d9, 0x2dce, 0xfcdb, 0x029b,
		0x08a8, 0x0e11, 0xfbfc, 0x5da4, 0xc465, 0x26a3, 0xda77, 0x483a,
		0xd4b8, 0xfb10, 0xd08f, 0x9c47, 0x5419, 0xa685, 0xb448, 0xfd17,

		// tx hash: 1000000...0000
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0100,
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,

		// VHI / VLO - valid signature for secret key 1 and tx hash 1 with nonce 2
		0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
		0x001b, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,

		// RHI / RLO - 	valid signature for secret key 1 and tx hash 1 with nonce 2
		0x7cd8, 0x95c0, 0x406e, 0x3045, 0x7d6d, 0x41ed, 0x7f94, 0xc604,
		0x9ee5, 0x5c70, 0x09b9, 0xabac, 0x3ca7, 0x8cef, 0x8e4b, 0x5c77,

		// SHI / SLO - valid signature for secret key 1 and tx hash 1 with nonce 2
		0x3e6b, 0x4ae0, 0xa037, 0x9822, 0xbeb6, 0x20f6, 0x3fca, 0xe382,
		0x7013, 0x1653, 0x3423, 0xb5bf, 0xee71, 0x1e1b, 0x3599, 0x8b93,

		// SuccessBit / EcRecoverBit - valid signature for secret key 1 and tx hash 1 with nonce 2
		0x0001, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
		0x0001, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000,
	}

	var (
		k = inputIndex % len(fillingData)
		x = uint64(fillingData[k])
	)

	return field.NewElement(x)
}
