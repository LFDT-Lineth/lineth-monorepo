package ecpair

import (
	"github.com/consensys/gnark-crypto/ecc/bn254"
	"github.com/consensys/gnark-crypto/ecc/bn254/fp"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/utils"
)

func convG1WizardToGnark(limbs [nbG1Limbs]field.Element) bn254.G1Affine {
	var res bn254.G1Affine
	var buf [fp.Bytes]byte

	copyTo := func(i int, dst *fp.Element) {
		l0 := limbs[i].Bytes()
		l1 := limbs[i+1].Bytes()
		copy(buf[0:16], l0[16:32])
		copy(buf[16:32], l1[16:32])
		dst.SetBytes(buf[:])
	}
	copyTo(0, &res.X)
	copyTo(2, &res.Y)

	return res
}

func convG2WizardToGnark(limbs [nbG2Limbs]field.Element) bn254.G2Affine {
	var res bn254.G2Affine
	var buf [fp.Bytes]byte

	copyTo := func(i int, dst *fp.Element) {
		l0 := limbs[i].Bytes()
		l1 := limbs[i+1].Bytes()
		copy(buf[0:16], l0[16:32])
		copy(buf[16:32], l1[16:32])
		dst.SetBytes(buf[:])
	}
	// arithmetization provides G2 coordinates in the following order:
	//   X_Im, X_Re, Y_Im, Y_Re
	// but in gnark we expect
	//   X_Re, X_Im, Y_Re, Y_Im
	// so we need to swap the limbs.
	copyTo(0, &res.X.A1)
	copyTo(2, &res.X.A0)
	copyTo(4, &res.Y.A1)
	copyTo(6, &res.Y.A0)

	return res
}

func convGtGnarkToWizard(elem bn254.GT) [nbGtLimbs]field.Element {
	var res [nbGtLimbs]field.Element
	var buf [fp.Bytes / 2]byte

	copyTo := func(i int, b [fp.Bytes]byte) {
		copy(buf[:], b[0:16])
		res[i].SetBytes(buf[:])
		copy(buf[:], b[16:32])
		res[i+1].SetBytes(buf[:])
	}
	copyTo(0, elem.C0.B0.A0.Bytes())
	copyTo(2, elem.C0.B0.A1.Bytes())
	copyTo(4, elem.C0.B1.A0.Bytes())
	copyTo(6, elem.C0.B1.A1.Bytes())
	copyTo(8, elem.C0.B2.A0.Bytes())
	copyTo(10, elem.C0.B2.A1.Bytes())
	copyTo(12, elem.C1.B0.A0.Bytes())
	copyTo(14, elem.C1.B0.A1.Bytes())
	copyTo(16, elem.C1.B1.A0.Bytes())
	copyTo(18, elem.C1.B1.A1.Bytes())
	copyTo(20, elem.C1.B2.A0.Bytes())
	copyTo(22, elem.C1.B2.A1.Bytes())

	return res
}

func init() {
	plonk.RegisterInputFiller(inputFillerMillerLoopKey, inputFillerMillerLoop)
	plonk.RegisterInputFiller(inputFillerFinalExpKey, inputFillerFinalExp)
	plonk.RegisterInputFiller(inputFillerG2MembershipKey, inputFillerG2Membership)
}

var (
	inputFillerMillerLoopKey   = "bn254-miller-loop-input-filler"
	inputFillerFinalExpKey     = "bn254-final-exp-input-filler"
	inputFillerG2MembershipKey = "bn254-g2-membership-input-filler"
)

func inputFillerMillerLoop(circuitInstance, inputIndex int) field.Element {
	// prev = 1
	// p = g1 gen
	// q = g2 gen
	// curr = e(p, q)
	tbl := []string{
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000001",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000001",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000002",
		"0x198e9393920d483a7260bfb731fb5d25",
		"0xf1aa493335a9e71297e485b7aef312c2",
		"0x1800deef121f1e76426a00665e5c4479",
		"0x674322d4f75edadd46debd5cd992f6ed",
		"0x090689d0585ff075ec9e99ad690c3395",
		"0xbc4b313370b38ef355acdadcd122975b",
		"0x12c85ea5db8c6deb4aab71808dcb408f",
		"0xe3d1e7690c43d37b4ce6cc0166fa7daa",
		"0x132504eb4501a5bf6b07af56b7db0fa2",
		"0xe466d6409233e66e4c91574681ba8566",
		"0x16edf1040913f3d354676315f5d158d6",
		"0x1ba6a7369eaac1bc4f0c2d72be31cee2",
		"0x132b66f5e20b77a759f41f13a5e6b041",
		"0x2c6112212e6365529f630c85192ad16f",
		"0x0770c0ea6116b19864141f82724368f7",
		"0xe62dc0d6b74c6235d06cd152fd8981d2",
		"0x258c92380ebc5b7819f43f64398a51e1",
		"0xa120ab1c39458a75df4f071d669f97d1",
		"0x21268548f1cacec5a8e6204eaead3b3f",
		"0xf317a4e81f0629d828e9b62d1a4e2c5e",
		"0x0f326699d970d1caa67566242f7e890d",
		"0xe6209956a9456136d1dc8268abbb37ed",
		"0x229254d20275bc1ceb9160311563e534",
		"0x219446c3d731aefb89d34d10fff95c96",
		"0x22a66075e8f04e7efd62a63433a2d91d",
		"0x2e5c673334cacb9339aaf16706d99466",
		"0x04278698c016ccc6a0ed44a6bd596c36",
		"0xfd06e50ce5e2ed5605f9f606a88ce821",
		"0x0b02989b4da34883f8736e7cf91e7ffb",
		"0x573732553ebd0ea9e9719e6d7ad47418",
		"0x12f11cc371163663d21b72b6f86808cb",
		"0x6481e1abded4465c6e15feb7a389480b",
	}
	var res field.Element
	_, err := res.SetString(tbl[inputIndex%(nbG1Limbs+nbG2Limbs+2*nbGtLimbs)])
	if err != nil {
		utils.Panic("failed to set string: %v", err)
	}
	return res
}

func inputFillerFinalExp(circuitInstance, inputIndex int) field.Element {
	// accumulator e(-p, q)
	// inputs e(p, q)
	// result e(-p, q) * e(p, q) == 1
	tbl := []string{
		"0x132504eb4501a5bf6b07af56b7db0fa2",
		"0xe466d6409233e66e4c91574681ba8566",
		"0x16edf1040913f3d354676315f5d158d6",
		"0x1ba6a7369eaac1bc4f0c2d72be31cee2",
		"0x132b66f5e20b77a759f41f13a5e6b041",
		"0x2c6112212e6365529f630c85192ad16f",
		"0x0770c0ea6116b19864141f82724368f7",
		"0xe62dc0d6b74c6235d06cd152fd8981d2",
		"0x258c92380ebc5b7819f43f64398a51e1",
		"0xa120ab1c39458a75df4f071d669f97d1",
		"0x21268548f1cacec5a8e6204eaead3b3f",
		"0xf317a4e81f0629d828e9b62d1a4e2c5e",
		"0x2131e7d907c0ce5f11dadf925202cf4f",
		"0xb160d13abf2c69566a4409ae2cc1c55a",
		"0x0dd1f9a0debbe40cccbee5856c1d7329",
		"0x75ed23cd91401b91b24d3f05d883a0b1",
		"0x0dbdedfcf84151aabaed9f824dde7f40",
		"0x6925035e33a6fefa02759aafd1a368e1",
		"0x2c3cc7da211ad3631763010fc427ec26",
		"0x9a7a8584828edd37362696102ff01526",
		"0x2561b5d7938e57a5bfdcd7398862d862",
		"0x404a383c29b4bbe352aeeda95da8892f",
		"0x1d7331af701b69c5e634d2ff89194f92",
		"0x32ff88e5899d8430ce0a8d5f34f3b53c",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000001",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000002",
		"0x198e9393920d483a7260bfb731fb5d25",
		"0xf1aa493335a9e71297e485b7aef312c2",
		"0x1800deef121f1e76426a00665e5c4479",
		"0x674322d4f75edadd46debd5cd992f6ed",
		"0x090689d0585ff075ec9e99ad690c3395",
		"0xbc4b313370b38ef355acdadcd122975b",
		"0x12c85ea5db8c6deb4aab71808dcb408f",
		"0xe3d1e7690c43d37b4ce6cc0166fa7daa",
		"0x00000000000000000000000000000000",
		"0x00000000000000000000000000000001",
	}
	var res field.Element
	_, err := res.SetString(tbl[inputIndex%(nbG1Limbs+nbG2Limbs+nbGtLimbs+2)])
	if err != nil {
		utils.Panic("failed to set string: %v", err)
	}
	return res
}

func inputFillerG2Membership(circuitInstance, inputIndex int) field.Element {
	// g2 generator
	tbl := []string{
		"0x198e9393920d483a7260bfb731fb5d25",
		"0xf1aa493335a9e71297e485b7aef312c2",
		"0x1800deef121f1e76426a00665e5c4479",
		"0x674322d4f75edadd46debd5cd992f6ed",
		"0x090689d0585ff075ec9e99ad690c3395",
		"0xbc4b313370b38ef355acdadcd122975b",
		"0x12c85ea5db8c6deb4aab71808dcb408f",
		"0xe3d1e7690c43d37b4ce6cc0166fa7daa",
		"0x1",
	}
	var res field.Element
	_, err := res.SetString(tbl[inputIndex%(nbG2Limbs+1)])
	if err != nil {
		utils.Panic("failed to set string: %v", err)
	}
	return res
}
