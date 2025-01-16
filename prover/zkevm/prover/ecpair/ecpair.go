package ecpair

import (
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
)

func createColFn(comp *wizard.CompiledIOP, rootName string, size int) func(name string) ifaces.Column {
	return func(name string) ifaces.Column {
		return comp.InsertCommit(roundNr, ifaces.ColIDf("%s_%s", rootName, name), size)
	}
}

const (
	roundNr                 = 0
	nameECPair              = "ECPAIR"
	namePairingData         = "ECPAIR_UNALIGNED_PAIRING_DATA"
	nameG2Data              = "ECPAIR_UNALIGNED_G2_DATA"
	nameAlignmentG2Subgroup = "ECPAIR_ALIGNMENT_G2"
	nameAlignmentMillerLoop = "ECPAIR_ALIGNMENT_ML"
	nameAlignmentFinalExp   = "ECPAIR_ALIGNMENT_FINALEXP"
)

const (
	nbG1Limbs = 4
	nbG2Limbs = 8
	nbGtLimbs = 24
)

// ECPair represents the constraints for proving the ECPAIR precompile. It is composed of:
// - ECPairSource: the source columns from arithmetization
// - UnalignedPairingData: the unaligned columns for the pairing data
// - UnalignedG2MembershipData: the unaligned columns for the G2 membership data
// - AlignedG2MembershipData: the aligned columns for the G2 check circuit
// - AlignedMillerLoopCircuit: the aligned columns for the MillerLoop circuit
// - AlignedFinalExpCircuit: the aligned columns for the FinalExp circuit
//
// Use [newECPair] to create a new instance of ECPair with the limits and source columns.
//
// By default, the gnark circuit is not attached to the module. Use methods
// [WithPairingCircuit] and [WithG2MembershipCircuit] for attaching the circuit
// and enforcing the actual checks at prover runtime.
type ECPair struct {
	*Limits

	IsActive ifaces.Column

	*ECPairSource
	*UnalignedPairingData
	*UnalignedG2MembershipData

	CptPrevEqualCurrID wizard.ProverAction

	AlignedG2MembershipData  *plonk.Alignment
	AlignedMillerLoopCircuit *plonk.Alignment
	AlignedFinalExpCircuit   *plonk.Alignment
}

func NewECPairZkEvm(comp *wizard.CompiledIOP, limits *Limits) *ECPair {
	return newECPair(
		comp,
		limits,
		&ECPairSource{
			CsEcpairing:       comp.Columns.GetHandle("ecdata.CIRCUIT_SELECTOR_ECPAIRING"),
			ID:                comp.Columns.GetHandle("ecdata.ID"),
			Limb:              comp.Columns.GetHandle("ecdata.LIMB"),
			SuccessBit:        comp.Columns.GetHandle("ecdata.SUCCESS_BIT"),
			Index:             comp.Columns.GetHandle("ecdata.INDEX"),
			IsEcPairingData:   comp.Columns.GetHandle("ecdata.IS_ECPAIRING_DATA"),
			IsEcPairingResult: comp.Columns.GetHandle("ecdata.IS_ECPAIRING_RESULT"),
			AccPairings:       comp.Columns.GetHandle("ecdata.ACC_PAIRINGS"),
			TotalPairings:     comp.Columns.GetHandle("ecdata.TOTAL_PAIRINGS"),
			CsG2Membership:    comp.Columns.GetHandle("ecdata.CIRCUIT_SELECTOR_G2_MEMBERSHIP"),
		},
	).WithG2MembershipCircuit(comp).
		WithPairingCircuit(comp, plonk.WithRangecheck(16, 6, true))
}

func newECPair(comp *wizard.CompiledIOP, limits *Limits, ecSource *ECPairSource) *ECPair {
	size := limits.sizeECPair()
	createCol := createColFn(comp, nameECPair, size)

	res := &ECPair{
		Limits:                    limits,
		ECPairSource:              ecSource,
		IsActive:                  createCol("IS_ACTIVE"),
		UnalignedPairingData:      newUnalignedPairingData(comp, limits),
		UnalignedG2MembershipData: newUnalignedG2MembershipData(comp, limits),
	}

	// IsActive activation - can only go from 1 to {0, 1} and from 0 to 0.
	res.csIsActiveActivation(comp)
	// masks and flags are binary
	res.csBinaryConstraints(comp)
	// IsActive is only active when we are either pulling or computing in the unaligned submodules
	res.csFlagConsistency(comp)
	// when not active, then all values are zero.
	res.csOffWhenInactive(comp)
	// projection queries
	res.csProjections(comp)

	// membership check result
	res.csMembershipComputedResult(comp)

	// pairing data constraints
	res.csConstantWhenIsComputing(comp)
	res.csInstanceIDChangeWhenNewInstance(comp)
	res.csAccumulatorInit(comp)
	res.csAccumulatorConsistency(comp)
	res.csTotalPairs(comp)
	res.csLastPairToFinalExp(comp)
	res.csIndexConsistency(comp)
	res.csAccumulatorMask(comp)
	// only Unaligned Pairing data or G2 membership data is active at a time
	res.csPairingDataOrMembershipActive(comp)
	// only to Miller loop or to FinalExp
	res.csExclusivePairingCircuitMasks(comp)

	return res
}

// WithPairingCircuit attaches the gnark circuit to the ECPair module for
// enforcing the pairing checks.
func (ec *ECPair) WithPairingCircuit(comp *wizard.CompiledIOP, options ...plonk.Option) *ECPair {
	alignInputMillerLoop := &plonk.CircuitAlignmentInput{
		Round:              roundNr,
		Name:               nameAlignmentMillerLoop,
		DataToCircuit:      ec.UnalignedPairingData.Limb,
		DataToCircuitMask:  ec.UnalignedPairingData.ToMillerLoopCircuitMask,
		Circuit:            newMultiMillerLoopMulCircuit(ec.NbMillerLoopInputInstances),
		InputFiller:        inputFillerMillerLoop,
		PlonkOptions:       options,
		NbCircuitInstances: ec.NbMillerLoopCircuits,
	}
	ec.AlignedMillerLoopCircuit = plonk.DefineAlignment(comp, alignInputMillerLoop)

	alignInputFinalExp := &plonk.CircuitAlignmentInput{
		Round:              roundNr,
		Name:               nameAlignmentFinalExp,
		DataToCircuit:      ec.UnalignedPairingData.Limb,
		DataToCircuitMask:  ec.UnalignedPairingData.ToFinalExpCircuitMask,
		Circuit:            newMultiMillerLoopFinalExpCircuit(ec.NbFinalExpInputInstances),
		InputFiller:        inputFillerFinalExp,
		PlonkOptions:       options,
		NbCircuitInstances: ec.NbFinalExpCircuits,
	}
	ec.AlignedFinalExpCircuit = plonk.DefineAlignment(comp, alignInputFinalExp)

	return ec
}

// WithG2MembershipCircuit attaches the gnark circuit to the ECPair module for
// enforcing the G2 membership checks.
func (ec *ECPair) WithG2MembershipCircuit(comp *wizard.CompiledIOP, options ...plonk.Option) *ECPair {
	alignInputG2Membership := &plonk.CircuitAlignmentInput{
		Round:              roundNr,
		Name:               nameAlignmentG2Subgroup,
		DataToCircuit:      ec.UnalignedG2MembershipData.Limb,
		DataToCircuitMask:  ec.UnalignedG2MembershipData.ToG2MembershipCircuitMask,
		Circuit:            newMultiG2GroupcheckCircuit(ec.NbG2MembershipInputInstances),
		InputFiller:        inputFillerG2Membership,
		PlonkOptions:       options,
		NbCircuitInstances: ec.NbG2MembershipCircuits,
	}
	ec.AlignedG2MembershipData = plonk.DefineAlignment(comp, alignInputG2Membership)

	return ec
}

// ECPairSource represents the source columns from the arithmetization of the
// ECPAIR precompile. We assume that the data in the columns is already
// well-formed.
type ECPairSource struct {
	ID            ifaces.Column
	Index         ifaces.Column
	Limb          ifaces.Column
	SuccessBit    ifaces.Column
	AccPairings   ifaces.Column
	TotalPairings ifaces.Column

	IsEcPairingData   ifaces.Column
	IsEcPairingResult ifaces.Column

	// corresponds that instance is for pairing check
	CsEcpairing ifaces.Column
	// corresponds that instance is for G2 subgroup check
	CsG2Membership ifaces.Column
}

// UnalignedG2MembershipData represents the unaligned columns for the G2
// membership data.
//
// It performs a simple conversion and only appending the membership check
// result to the limbs.
//
// Use [newUnalignedG2MembershipData] to create a new instance of UnalignedG2MembershipData.
type UnalignedG2MembershipData struct {
	IsPulling  ifaces.Column
	IsComputed ifaces.Column

	Limb                      ifaces.Column
	SuccessBit                ifaces.Column
	ToG2MembershipCircuitMask ifaces.Column
}

func newUnalignedG2MembershipData(comp *wizard.CompiledIOP, limits *Limits) *UnalignedG2MembershipData {
	size := limits.sizeECPair()
	createCol := createColFn(comp, nameG2Data, size)

	return &UnalignedG2MembershipData{
		IsPulling:                 createCol("IS_PULLING"),
		IsComputed:                createCol("IS_COMPUTED"),
		Limb:                      createCol("LIMB"),
		SuccessBit:                createCol("SUCCESS_BIT"),
		ToG2MembershipCircuitMask: createCol("TO_G2_MEMBERSHIP_CIRCUIT"),
	}
}

// UnalignedPairingData represents the unaligned columns for the pairing data.
//
// As we check the Miller loops and final exponentiations separately, then this
// module is responsible for computing the intermediate accumulator results and
// its consistency. It also filters the limbs to be passed to different circuits
// (Miller loop and final exponentiation).
//
// Use [newUnalignedPairingData] to create a new instance of UnalignedPairingData.
type UnalignedPairingData struct {
	IsActive          ifaces.Column
	IsPulling         ifaces.Column
	IsComputed        ifaces.Column
	IsAccumulatorInit ifaces.Column
	IsAccumulatorCurr ifaces.Column
	IsAccumulatorPrev ifaces.Column

	InstanceID ifaces.Column
	PairID     ifaces.Column
	TotalPairs ifaces.Column
	Limb       ifaces.Column
	Index      ifaces.Column

	ToMillerLoopCircuitMask ifaces.Column
	ToFinalExpCircuitMask   ifaces.Column

	IsResultOfInstance           ifaces.Column
	IsFirstLineOfInstance        ifaces.Column
	IsFirstLineOfPrevAccumulator ifaces.Column
	IsFirstLineOfCurrAccumulator ifaces.Column
}

func newUnalignedPairingData(comp *wizard.CompiledIOP, limits *Limits) *UnalignedPairingData {
	size := limits.sizeECPair()
	createCol := createColFn(comp, namePairingData, size)

	return &UnalignedPairingData{
		IsActive:                     createCol("IS_ACTIVE"),
		IsPulling:                    createCol("IS_PULLING"),
		IsComputed:                   createCol("IS_COMPUTED"),
		Limb:                         createCol("LIMB"),
		InstanceID:                   createCol("INSTANCE_ID"),
		PairID:                       createCol("PAIR_ID"),
		TotalPairs:                   createCol("TOTAL_PAIRS"),
		ToMillerLoopCircuitMask:      createCol("TO_MILLER_LOOP_CIRCUIT"),
		ToFinalExpCircuitMask:        createCol("TO_FINAL_EXP_CIRCUIT"),
		IsFirstLineOfInstance:        createCol("IS_FIRST_LINE_OF_INSTANCE"),
		IsFirstLineOfPrevAccumulator: createCol("IS_FIRST_LINE_OF_PREV_ACC"),
		IsFirstLineOfCurrAccumulator: createCol("IS_FIRST_LINE_OF_CURR_ACC"),
		IsResultOfInstance:           createCol("IS_RESULT"),
		IsAccumulatorPrev:            createCol("IS_ACCUMULATOR_PREV"),
		IsAccumulatorCurr:            createCol("IS_ACCUMULATOR_CURR"),
		IsAccumulatorInit:            createCol("IS_ACCUMULATOR_INIT"),
		Index:                        createCol("INDEX"),
	}
}
