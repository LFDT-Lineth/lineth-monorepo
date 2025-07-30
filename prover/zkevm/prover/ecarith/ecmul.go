package ecarith

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/algopts"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/bitslice"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
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

	Size int
	*Limits
}

func NewEcMulZkEvm(comp *wizard.CompiledIOP, limits *Limits) *EcMul {
	return newEcMul(
		comp,
		limits,
		&EcDataMulSource{
			CsEcMul: comp.Columns.GetHandle("ecdata.CIRCUIT_SELECTOR_ECMUL"),
			Limb:    comp.Columns.GetHandle("ecdata.LIMB"),
			Index:   comp.Columns.GetHandle("ecdata.INDEX"),
			IsData:  comp.Columns.GetHandle("ecdata.IS_ECMUL_DATA"),
			IsRes:   comp.Columns.GetHandle("ecdata.IS_ECMUL_RESULT"),
		},
		[]query.PlonkOption{query.PlonkRangeCheckOption(16, 6, true)},
	)
}

// newEcMul creates a new EC_MUL integration.
func newEcMul(comp *wizard.CompiledIOP, limits *Limits, src *EcDataMulSource, plonkOptions []query.PlonkOption) *EcMul {
	size := limits.sizeEcMulIntegration()

	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_ECMUL + "_ALIGNMENT",
		Round:              ROUND_NR,
		DataToCircuitMask:  src.CsEcMul,
		DataToCircuit:      src.Limb,
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
		Size:             size,
	}

	return res
}

// Assign assigns the data from the trace to the gnark inputs.
func (em *EcMul) Assign(run *wizard.ProverRuntime) {
	em.AlignedGnarkData.Assign(run)
}

// EcDataMulSource is a struct that holds the columns that are used to
// fetch data from the EC_DATA module from the arithmetization.
type EcDataMulSource struct {
	CsEcMul ifaces.Column
	Limb    ifaces.Column
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
	// range.

	P_X_hi, P_X_lo frontend.Variable `gnark:",public"`
	P_Y_hi, P_Y_lo frontend.Variable `gnark:",public"`

	N_hi, N_lo frontend.Variable `gnark:",public"`

	// The result of the multiplication. Is provided by the caller, we have to
	// ensure that the result is correct.
	R_X_hi, R_X_lo frontend.Variable `gnark:",public"`
	R_Y_hi, R_Y_lo frontend.Variable `gnark:",public"`
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

		PXlimbs := make([]frontend.Variable, 4)
		PXlimbs[2], PXlimbs[3] = bitslice.Partition(api, c.Instances[i].P_X_hi, 64, bitslice.WithNbDigits(128))
		PXlimbs[0], PXlimbs[1] = bitslice.Partition(api, c.Instances[i].P_X_lo, 64, bitslice.WithNbDigits(128))
		PX := f.NewElement(PXlimbs)
		PYlimbs := make([]frontend.Variable, 4)
		PYlimbs[2], PYlimbs[3] = bitslice.Partition(api, c.Instances[i].P_Y_hi, 64, bitslice.WithNbDigits(128))
		PYlimbs[0], PYlimbs[1] = bitslice.Partition(api, c.Instances[i].P_Y_lo, 64, bitslice.WithNbDigits(128))
		PY := f.NewElement(PYlimbs)
		P := sw_bn254.G1Affine{
			X: *PX,
			Y: *PY,
		}

		Nlimbs := make([]frontend.Variable, 4)
		Nlimbs[2], Nlimbs[3] = bitslice.Partition(api, c.Instances[i].N_hi, 64, bitslice.WithNbDigits(128))
		Nlimbs[0], Nlimbs[1] = bitslice.Partition(api, c.Instances[i].N_lo, 64, bitslice.WithNbDigits(128))
		N := s.NewElement(Nlimbs)

		RXlimbs := make([]frontend.Variable, 4)
		RXlimbs[2], RXlimbs[3] = bitslice.Partition(api, c.Instances[i].R_X_hi, 64, bitslice.WithNbDigits(128))
		RXlimbs[0], RXlimbs[1] = bitslice.Partition(api, c.Instances[i].R_X_lo, 64, bitslice.WithNbDigits(128))
		RX := f.NewElement(RXlimbs)
		RYlimbs := make([]frontend.Variable, 4)
		RYlimbs[2], RYlimbs[3] = bitslice.Partition(api, c.Instances[i].R_Y_hi, 64, bitslice.WithNbDigits(128))
		RYlimbs[0], RYlimbs[1] = bitslice.Partition(api, c.Instances[i].R_Y_lo, 64, bitslice.WithNbDigits(128))
		RY := f.NewElement(RYlimbs)
		R := sw_bn254.G1Affine{
			X: *RX,
			Y: *RY,
		}
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
