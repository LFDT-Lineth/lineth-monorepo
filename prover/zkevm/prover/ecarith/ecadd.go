package ecarith

import (
	"fmt"

	"github.com/consensys/gnark/frontend"
	"github.com/consensys/gnark/std/algebra"
	"github.com/consensys/gnark/std/algebra/emulated/sw_bn254"
	"github.com/consensys/gnark/std/math/bitslice"
	"github.com/consensys/gnark/std/math/emulated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

const (
	NAME_ECADD = "ECADD_INTEGRATION"
)

const (
	nbRowsPerEcAdd = 12
)

// EcMulIntegration integrated EC_MUL precompile call verification inside a
// gnark circuit.
type EcAdd struct {
	*EcDataAddSource
	AlignedGnarkData *plonk.Alignment

	Size int
	*Limits
}

func NewEcAddZkEvm(comp *wizard.CompiledIOP, limits *Limits) *EcAdd {
	return newEcAdd(
		comp,
		limits,
		&EcDataAddSource{
			CsEcAdd: comp.Columns.GetHandle("ecdata.CIRCUIT_SELECTOR_ECADD"),
			Limb:    comp.Columns.GetHandle("ecdata.LIMB"),
			Index:   comp.Columns.GetHandle("ecdata.INDEX"),
			IsData:  comp.Columns.GetHandle("ecdata.IS_ECADD_DATA"),
			IsRes:   comp.Columns.GetHandle("ecdata.IS_ECADD_RESULT"),
		},
		[]query.PlonkOption{query.PlonkRangeCheckOption(16, 6, true)},
	)
}

// newEcAdd creates a new EC_ADD integration.
func newEcAdd(comp *wizard.CompiledIOP, limits *Limits, src *EcDataAddSource, plonkOptions []query.PlonkOption) *EcAdd {
	size := limits.sizeEcAddIntegration()

	toAlign := &plonk.CircuitAlignmentInput{
		Name:               NAME_ECADD + "_ALIGNMENT",
		Round:              ROUND_NR,
		DataToCircuitMask:  src.CsEcAdd,
		DataToCircuit:      src.Limb,
		Circuit:            NewECAddCircuit(limits),
		NbCircuitInstances: limits.NbCircuitInstances,
		PlonkOptions:       plonkOptions,
		// This resolves to the statement (0, 0) + (0, 0) = (0, 0), which is
		// correct as (0, 0) encodes the point at infinity.
		InputFillerKey: "",
	}

	res := &EcAdd{
		EcDataAddSource:  src,
		AlignedGnarkData: plonk.DefineAlignment(comp, toAlign),
		Size:             size,
	}

	return res
}

// Assign assigns the data from the trace to the gnark inputs.
func (em *EcAdd) Assign(run *wizard.ProverRuntime) {
	em.AlignedGnarkData.Assign(run)
}

// EcDataAddSource is a struct that holds the columns that are used to
// fetch data from the EC_DATA module from the arithmetization.
type EcDataAddSource struct {
	CsEcAdd ifaces.Column
	Limb    ifaces.Column
	Index   ifaces.Column
	IsData  ifaces.Column
	IsRes   ifaces.Column
}

// MultiECAddCircuit is a circuit that can handle multiple EC_ADD instances. The
// length of the slice Instances should corresponds to the one defined in the
// Limits struct.
type MultiECAddCircuit struct {
	Instances []ECAddInstance
}

type ECAddInstance struct {
	// First input to addition
	P_X_hi, P_X_lo frontend.Variable `gnark:",public"`
	P_Y_hi, P_Y_lo frontend.Variable `gnark:",public"`

	// Second input to addition
	Q_X_hi, Q_X_lo frontend.Variable `gnark:",public"`
	Q_Y_hi, Q_Y_lo frontend.Variable `gnark:",public"`

	// The result of the addition. Is provided non-deterministically by the
	// caller, we have to ensure that the result is correct.
	R_X_hi, R_X_lo frontend.Variable `gnark:",public"`
	R_Y_hi, R_Y_lo frontend.Variable `gnark:",public"`
}

// NewECMulCircuit creates a new circuit for verifying the EC_MUL precompile
// based on the defined number of inputs.
func NewECAddCircuit(limits *Limits) *MultiECAddCircuit {
	return &MultiECAddCircuit{
		Instances: make([]ECAddInstance, limits.NbInputInstances),
	}
}

func (c *MultiECAddCircuit) Define(api frontend.API) error {

	f, err := emulated.NewField[sw_bn254.BaseField](api)
	if err != nil {
		return fmt.Errorf("field emulation: %w", err)
	}

	// gnark circuit works with 64 bits values, we need to split the 128 bits
	// values into high and low parts.
	nbInstances := len(c.Instances)
	Ps := make([]sw_bn254.G1Affine, nbInstances)
	Qs := make([]sw_bn254.G1Affine, nbInstances)
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

		QXlimbs := make([]frontend.Variable, 4)
		QXlimbs[2], QXlimbs[3] = bitslice.Partition(api, c.Instances[i].Q_X_hi, 64, bitslice.WithNbDigits(128))
		QXlimbs[0], QXlimbs[1] = bitslice.Partition(api, c.Instances[i].Q_X_lo, 64, bitslice.WithNbDigits(128))
		QX := f.NewElement(QXlimbs)
		QYlimbs := make([]frontend.Variable, 4)
		QYlimbs[2], QYlimbs[3] = bitslice.Partition(api, c.Instances[i].Q_Y_hi, 64, bitslice.WithNbDigits(128))
		QYlimbs[0], QYlimbs[1] = bitslice.Partition(api, c.Instances[i].Q_Y_lo, 64, bitslice.WithNbDigits(128))
		QY := f.NewElement(QYlimbs)
		Q := sw_bn254.G1Affine{
			X: *QX,
			Y: *QY,
		}

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
		Qs[i] = Q
		Rs[i] = R
	}

	curve, err := algebra.GetCurve[sw_bn254.ScalarField, sw_bn254.G1Affine](api)
	if err != nil {
		panic(err)
	}
	for i := range Rs {
		res := curve.AddUnified(&Ps[i], &Qs[i])
		curve.AssertIsEqual(&Rs[i], res)
	}
	return nil
}
