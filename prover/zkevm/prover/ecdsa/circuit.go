package ecdsa

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/emulated/sw_emulated"
	"github.com/consensys/gnark/std/evmprecompiles"
	"github.com/consensys/gnark/std/math/bitslice"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/gnark/std/math/emulated/emparams"
	"github.com/consensys/linea-monorepo/prover/maths/field"
)

type EcRecoverInstance struct {
	PKXHi, PKXLo, PKYHi, PKYLo frontend.Variable `gnark:",public"`
	HHi, HLo                   frontend.Variable `gnark:",public"`
	VHi, VLo                   frontend.Variable `gnark:",public"`
	RHi, RLo                   frontend.Variable `gnark:",public"`
	SHi, SLo                   frontend.Variable `gnark:",public"`
	SUCCESS_BIT                frontend.Variable `gnark:",public"`
	ECRECOVERBIT               frontend.Variable `gnark:",public"`
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
	// gnark circuit works with 64 bits values, we need to split the 128 bits
	// values into high and low parts.
	// we leave the result unconstrained as field emulation constraints automatically
	msgLimbs := make([]frontend.Variable, 4)
	msgLimbs[2], msgLimbs[3] = bitslice.Partition(api, c.HHi, 64, bitslice.WithNbDigits(128))
	msgLimbs[0], msgLimbs[1] = bitslice.Partition(api, c.HLo, 64, bitslice.WithNbDigits(128))
	msg = fr.NewElement(msgLimbs)

	PXlimbs := make([]frontend.Variable, 4)
	PXlimbs[2], PXlimbs[3] = bitslice.Partition(api, c.PKXHi, 64, bitslice.WithNbDigits(128))
	PXlimbs[0], PXlimbs[1] = bitslice.Partition(api, c.PKXLo, 64, bitslice.WithNbDigits(128))
	PX := fp.NewElement(PXlimbs)

	PYlimbs := make([]frontend.Variable, 4)
	PYlimbs[2], PYlimbs[3] = bitslice.Partition(api, c.PKYHi, 64, bitslice.WithNbDigits(128))
	PYlimbs[0], PYlimbs[1] = bitslice.Partition(api, c.PKYLo, 64, bitslice.WithNbDigits(128))
	PY := fp.NewElement(PYlimbs)
	PK = &sw_emulated.AffinePoint[emparams.Secp256k1Fp]{
		X: *PX,
		Y: *PY,
	}

	// v is 27 or 28 in EVM, but arithmetization gives on two limbs. Ensure that the high limb is zero and use only the low limb.
	api.AssertIsEqual(c.VHi, 0)
	v = c.VLo

	// similarly, we split r and s into limbs compatible with gnark. But we work
	// over a different field (scalar field vs base field for PK).
	rLimbs := make([]frontend.Variable, 4)
	rLimbs[2], rLimbs[3] = bitslice.Partition(api, c.RHi, 64, bitslice.WithNbDigits(128))
	rLimbs[0], rLimbs[1] = bitslice.Partition(api, c.RLo, 64, bitslice.WithNbDigits(128))
	r = fr.NewElement(rLimbs)

	sLimbs := make([]frontend.Variable, 4)
	sLimbs[2], sLimbs[3] = bitslice.Partition(api, c.SHi, 64, bitslice.WithNbDigits(128))
	sLimbs[0], sLimbs[1] = bitslice.Partition(api, c.SLo, 64, bitslice.WithNbDigits(128))
	s = fr.NewElement(sLimbs)

	// SUCCESS_BIT indicates if the input is a valid signature (1 for valid, 0
	// for invalid). Recall that we also allow to verify invalid signatures (for
	// the ECRECOVER precompile call).
	isFailure = api.Sub(1, c.SUCCESS_BIT)
	// ECRECOVERBIT indicates if the input comes from the ECRECOVER precompile
	// or not (1 for ECRECOVER, 0 for TX).
	strictRange = api.Sub(1, c.ECRECOVERBIT)

	return
}

func inputFiller(circuitInstance, inputIndex int) field.Element {
	// every instance has 14 inputs.
	// pubkey xHi, pubkey xLo, pubkey yHi, pubkey yLo, hHi, hLo, vHi, vLo, rHi, rLo, sHi, sLo, successBit, ecrecoverBit

	// public key 1*G
	placeholderPubkey := [64]byte{0x79, 0xbe, 0x66, 0x7e, 0xf9, 0xdc, 0xbb, 0xac, 0x55, 0xa0, 0x62, 0x95, 0xce, 0x87, 0xb, 0x7, 0x2, 0x9b, 0xfc, 0xdb, 0x2d, 0xce, 0x28, 0xd9, 0x59, 0xf2, 0x81, 0x5b, 0x16, 0xf8, 0x17, 0x98, 0x48, 0x3a, 0xda, 0x77, 0x26, 0xa3, 0xc4, 0x65, 0x5d, 0xa4, 0xfb, 0xfc, 0xe, 0x11, 0x8, 0xa8, 0xfd, 0x17, 0xb4, 0x48, 0xa6, 0x85, 0x54, 0x19, 0x9c, 0x47, 0xd0, 0x8f, 0xfb, 0x10, 0xd4, 0xb8}
	// 1000000...0000
	placeholderTxHash := [32]byte{0x1, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}
	// valid signature for secret key 1 and tx hash 1 with random nonce
	// valid signature for secret key 1 and tx hash 1 with nonce 2
	placeholderSignature := [66]byte{
		// r part of signature
		0xc6, 0x4, 0x7f, 0x94, 0x41, 0xed, 0x7d, 0x6d, 0x30, 0x45, 0x40, 0x6e, 0x95, 0xc0, 0x7c, 0xd8, 0x5c, 0x77, 0x8e, 0x4b, 0x8c, 0xef, 0x3c, 0xa7, 0xab, 0xac, 0x9, 0xb9, 0x5c, 0x70, 0x9e, 0xe5,
		// s part of signature
		0xe3, 0x82, 0x3f, 0xca, 0x20, 0xf6, 0xbe, 0xb6, 0x98, 0x22, 0xa0, 0x37, 0x4a, 0xe0, 0x3e, 0x6b, 0x8b, 0x93, 0x35, 0x99, 0x1e, 0x1b, 0xee, 0x71, 0xb5, 0xbf, 0x34, 0x23, 0x16, 0x53, 0x70, 0x13,
		// v part of signature (in EVM format, 27 is added to the recovery id)
		0x0, 0x1b,
	}
	var ret field.Element
	switch inputIndex % 14 {
	case 0: // PK X HI
		ret.SetBytes(placeholderPubkey[0:16])
	case 1: // PK X LO
		ret.SetBytes(placeholderPubkey[16:32])
	case 2: // PK Y HI
		ret.SetBytes(placeholderPubkey[32:48])
	case 3: // PK Y LO
		ret.SetBytes(placeholderPubkey[48:64])
	case 4: // MSG HI
		ret.SetBytes(placeholderTxHash[0:16])
	case 5: // MSH LO
		ret.SetBytes(placeholderTxHash[16:32])
	case 6: // v HI
		ret.SetUint64(0)
	case 7: // v LO
		ret.SetUint64(0x1b)
	case 8: // r HI
		ret.SetBytes(placeholderSignature[0:16])
	case 9: // r LO
		ret.SetBytes(placeholderSignature[16:32])
	case 10: // s HI
		ret.SetBytes(placeholderSignature[32:48])
	case 11: // s LO
		ret.SetBytes(placeholderSignature[48:64])
	case 12: // success bit
		ret.SetUint64(1)
	case 13: // ecrecover bit
		ret.SetUint64(1)
	}
	return ret
}
