package ecarith

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/algopts"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/limbs"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils/gnarkutil"
	"github.com/consensys/linea-monorepo/prover/zkevm/arithmetization"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

const (
	ROUND_NR   = 0
	NAME_ECMUL = "ECMUL_INTEGRATION"
)

const (
	nbRowsPerEcMul = 10
)

// EcMul integrated EC_MUL precompile call verification inside a gnark circuit.
type EcMul struct {
	*EcDataMulSource
	AlignedGnarkData *plonk.Alignment
	FlattenLimbs     *common.FlattenColumn
	Size             int
	*Limits
}

func NewEcMulZkEvm(comp *wizard.CompiledIOP, limits *Limits, arith *arithmetization.Arithmetization) *EcMul {

	src := &EcDataMulSource{
		CsEcMul: arith.ColumnOf(comp, "ecdata", "CIRCUIT_SELECTOR_ECMUL"),
		Index:   arith.ColumnOf(comp, "ecdata", "INDEX"),
		IsData:  arith.ColumnOf(comp, "ecdata", "IS_ECMUL_DATA"),
		IsRes:   arith.ColumnOf(comp, "ecdata", "IS_ECMUL_RESULT"),
		Limbs:   arith.GetLimbsOfU128Le(comp, "ecdata", "LIMB"),
	}

	return newEcMul(
		comp,
		limits,
		src,
		[]query.PlonkOption{query.PlonkRangeCheckOption(16, 1, true)},
	)
}

// newEcMul creates a new EC_MUL integration.
func newEcMul(comp *wizard.CompiledIOP, limits *Limits, src *EcDataMulSource, plonkOptions []query.PlonkOption) *EcMul {
	size := limits.sizeEcMulIntegration()

	flattenLimbs := common.NewFlattenColumn(comp, src.Limbs.AsDynSize(), src.CsEcMul)

	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_ECMUL + "_ALIGNMENT",
		Round:              ROUND_NR,
		DataToCircuitMask:  flattenLimbs.Mask,
		DataToCircuit:      flattenLimbs.Limbs,
		Circuit:            NewECMulCircuit(limits),
		NbCircuitInstances: limits.NbCircuitInstances,
		PlonkOptions:       plonkOptions,
		// not necessary: 0 * (0,0) = (0,0) with complete arithmetic since (0, 0)
		// encodes the point at infinity.
		InputFillerKey: "",
	}

	res := &EcMul{
		EcDataMulSource:  src,
		AlignedGnarkData: plonk.DefineAlignment(comp, toAlign),
		FlattenLimbs:     flattenLimbs,
		Size:             size,
	}

	flattenLimbs.CsFlattenProjection(comp)

	return res
}

// Assign assigns the data from the trace to the gnark inputs.
func (em *EcMul) Assign(run *wizard.ProverRuntime) {
	em.FlattenLimbs.Run(run)
	em.AlignedGnarkData.Assign(run)
}

// EcDataMulSource is a struct that holds the columns that are used to
// fetch data from the EC_DATA module from the arithmetization.
type EcDataMulSource struct {
	CsEcMul ifaces.Column
	Limbs   limbs.Uint128Le
	Index   ifaces.Column
	IsData  ifaces.Column
	IsRes   ifaces.Column
}

// MultiECMulCircuit is a circuit that can handle multiple EC_MUL instances. The
// length of the slice Instances should corresponds to the one defined in the
// Limits struct.
type MultiECMulCircuit struct {
	Instances []ECMulInstance
}

type ECMulInstance struct {
	// We work with the limbs decomposition coming from the arithmetization
	// where the values are split into 128 bits with high and low parts. The
	// high part is the most significant bits and the low part is the least
	// significant bits. The values are already range checked to be in 128 bit
	// range. Both parts are provided in little-endian form but the high-part is
	// always passed before the low part. So the parts can't effectively joined
	// into a single 256 bit value.

	P_X_HI, P_X_LO [common.NbLimbU128]frontend.Variable `gnark:",public"`
	P_Y_HI, P_Y_LO [common.NbLimbU128]frontend.Variable `gnark:",public"`
	N_HI, N_LO     [common.NbLimbU128]frontend.Variable `gnark:",public"`
	R_X_HI, R_X_LO [common.NbLimbU128]frontend.Variable `gnark:",public"`
	R_Y_HI, R_Y_LO [common.NbLimbU128]frontend.Variable `gnark:",public"`

	// The result of the multiplication. Is provided by the caller, we have to
	// ensure that the result is correct.
}

// NewECMulCircuit creates a new circuit for verifying the EC_MUL precompile
// based on the defined number of inputs.
func NewECMulCircuit(limits *Limits) *MultiECMulCircuit {
	return &MultiECMulCircuit{
		Instances: make([]ECMulInstance, limits.NbInputInstances),
	}
}

func (c *MultiECMulCircuit) Define(api frontend.API) error {

	f, err := emulated.NewField[sw_bn254.BaseField](api)
	if err != nil {
		return fmt.Errorf("field emulation: %w", err)
	}

	s, err := emulated.NewField[sw_bn254.ScalarField](api)
	if err != nil {
		return fmt.Errorf("field emulation: %w", err)
	}

	// gnark circuit works with 64 bits values, we need to split the 128 bits
	// values into high and low parts.
	nbInstances := len(c.Instances)
	Ps := make([]sw_bn254.G1Affine, nbInstances)
	Ns := make([]sw_bn254.Scalar, nbInstances)
	Rs := make([]sw_bn254.G1Affine, nbInstances)

	for i := range c.Instances {

		var (
			PX16 = append(c.Instances[i].P_X_LO[:], c.Instances[i].P_X_HI[:]...)
			PY16 = append(c.Instances[i].P_Y_LO[:], c.Instances[i].P_Y_HI[:]...)
			RX16 = append(c.Instances[i].R_X_LO[:], c.Instances[i].R_X_HI[:]...)
			RY16 = append(c.Instances[i].R_Y_LO[:], c.Instances[i].R_Y_HI[:]...)
			N16  = append(c.Instances[i].N_LO[:], c.Instances[i].N_HI[:]...)

			PX = gnarkutil.EmulatedFromLimbSlice(api, f, PX16, 16)
			PY = gnarkutil.EmulatedFromLimbSlice(api, f, PY16, 16)
			RX = gnarkutil.EmulatedFromLimbSlice(api, f, RX16, 16)
			RY = gnarkutil.EmulatedFromLimbSlice(api, f, RY16, 16)
			N  = gnarkutil.EmulatedFromLimbSlice(api, s, N16, 16)
			P  = sw_bn254.G1Affine{X: *PX, Y: *PY}
			R  = sw_bn254.G1Affine{X: *RX, Y: *RY}
		)

		Ps[i] = P
		Ns[i] = *N
		Rs[i] = R
	}

	curve, err := algebra.GetCurve[sw_bn254.ScalarField, sw_bn254.G1Affine](api)
	if err != nil {
		panic(err)
	}

	for i := range Rs {
		res := curve.ScalarMul(&Ps[i], &Ns[i], algopts.WithCompleteArithmetic())
		curve.AssertIsEqual(&Rs[i], res)
	}

	return nil
}
