package common

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/symbolic"
)

const (
	// Number of 64bits lanes in a keccak block
	NumLanesInBlock = 17
	NumRounds       = 24
	// the number of slices of keccakf-lane (64bits) for working with BaseTheta and BaseChi:
	// 8 slices of single Byte
	NumSlices = 8

	BaseChi    = 11
	BaseTheta  = 4
	BaseChi4   = 14641 // 11^4
	BaseTheta4 = 256   // 4^4
)

type (
	//  each Lane is 64 bits, represented as [numSlices] bytes.
	Lane = [NumSlices]ifaces.Column
	// keccakf state is a 5x5 matrix of lanes.
	State = [5][5]Lane
	// state after each base conversion, each lane is decomposed into 16 slices of 4 bits each.
	StateIn4Bits = [5][5][16]ifaces.Column
	// state after bit rotation, each lane is decomposed into 64 bits.
	StateInBits = [5][5][64]ifaces.Column
)

var (
	BaseChi4Fr   = field.NewElement(BaseChi4)
	BaseChiFr    = field.NewElement(BaseChi)
	BaseThetaFr  = field.NewElement(BaseTheta)
	BaseTheta4Fr = field.NewElement(BaseTheta4)
	Base2Fr      = field.NewElement(2)

	BaseChiExpr    = symbolic.NewConstant(BaseChiFr)
	BaseChi4Expr   = symbolic.NewConstant(BaseChi4Fr)
	Base16Expr     = symbolic.NewConstant(16)
	BaseTheta4Expr = symbolic.NewConstant(BaseTheta4Fr)
)
