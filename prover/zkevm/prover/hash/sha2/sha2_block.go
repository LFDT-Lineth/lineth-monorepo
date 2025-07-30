package sha2

import (
	"github.com/consensys/linea-monorepo/prover/maths/common/vector"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/plonk"
	"github.com/consensys/linea-monorepo/prover/protocol/distributed/pragmas"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/query"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	sym "github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils"
	commonconstraints "github.com/consensys/linea-monorepo/prover/zkevm/prover/common/common_constraints"
)

const (
	// number of rows taken by a single instance of Sha2-block. 16 for the block
	// 2 * uint128 for the initial hash and 2 * uint128 for the final hash.
	numRowPerInstance = 16 + 2 + 2
)

var (
	// initializationVector encodes the initialization vector of SHA2 in 2 field
	// elements storing each 128 bytes of the IV in big endian order.
	initializationVector = [2]field.Element{
		field.NewFromString("0x6A09E667BB67AE853C6EF372A54FF53A"),
		field.NewFromString("0x510E527F9B05688C1F83D9AB5BE0CD19"),
	}
)

// sha2BlocksInputs consists in the input columns to use to construct the SHA2
// verification circuit.
type sha2BlocksInputs struct {

	// Name allows the prover to provide context in a string which we derive to
	// to derive the name of the constraints and queries of the module.
	Name string

	// MaxNbBlock corresponds to the maximum number of blocks that can be handled
	// by the module.
	MaxNbBlockPerCirc int
	MaxNbCircuit      int

	// PackedUint32 contains the blocks given to the Sha2 hasher as sequences of
	// uint32.
	PackedUint32 ifaces.Column

	// Selector is a binary indicator column indicating which rows are to be
	// considered by the sha2 block module.
	Selector ifaces.Column

	// IsFirstLaneOfNewHash is an indicator column indicating when a new hash
	// is starting.
	IsFirstLaneOfNewHash ifaces.Column
}

// sha2BlockModule stores the compilation context of checking the correctness
// of the sha2 compression function.
type sha2BlockModule struct {

	// Inputs provided by the caller of [CheckSha2BlockHash]
	Inputs *sha2BlocksInputs

	// CanBeBeginningOfInstance is a precomputed column indicator column
	// marking with a 1 the beginning of a potential Sha2 instance. Shifting the
	// column by the right value gives the appropriate negative offset gives the
	// equivalent CanBeEndOfInstance. This is used to ensure that the IsActive
	// column can only transition to 0 at the end of an instance.
	CanBeBeginningOfInstance *dedicated.HeartBeatColumn

	// CanBeBlockOfInstance is a precomputed column indicating with 1s the
	// position corresponding potentially
	CanBeBlockOfInstance *dedicated.RepeatedPattern

	// CanBeEndOfInstance is a precomputed column indicating with 1s the position
	// corresponding to the end of blocks.
	CanBeEndOfInstance *dedicated.HeartBeatColumn

	// IsActive is a binary indicator column indicating with a 1 the rows that
	// are effectively used by the sha2BlockHashing module. This is used as a
	// selector for the alignment module.
	IsActive ifaces.Column

	// IsEffBlock is a binary indicator column indicating which rows are
	// effectively corresponding to a block. This is used for the projection
	// query between the input and the current module.
	IsEffBlock ifaces.Column

	// IsEffFirstLaneOfNewHash is a binary indicator column indicating if the
	// current row marks the beginning of a new hash. This is used add
	// constraints setting the values of the old state of the hasher.
	IsEffFirstLaneOfNewHash ifaces.Column

	// IsEffFirstLaneOfNewHashShiftMin2 is a manually shifted version of the
	// [IsFirstLaneOfNewHash] column with an offset of -2.
	IsEffFirstLaneOfNewHashShiftMin2 *dedicated.ManuallyShifted

	// IsEffLastLaneOfCurrHash is a binary indicator column indicating with a 1
	// the last row of every hash. It is used to ensure that HashHi and HashLo
	// are well constructed.
	//
	// The column is constructed by summing (IsNewHash << 1) and
	// (isActive - isActive << 1).
	IsEffLastLaneOfCurrHash ifaces.Column

	// Limb stores the inputs to send to the circuit
	Limbs ifaces.Column

	// HashHi and HashLo store respectively the HI and the LO part. The columns
	// are constants in the span of a hash.
	HashHi, HashLo ifaces.Column

	HashHiIsZero, HashLoIsZero ifaces.Column
	ProverActions              []wizard.ProverAction

	// GnarkCircuitConnector is the result of the Plonk alignement module. It
	// handles all the Plonk logic responsible for verifying the correctness of
	// each instance of the Sha2 compression function.
	GnarkCircuitConnector *plonk.Alignment

	// HasCircuit indicates whether the circuit has been set in the current module.
	// In production, it will always be set to true but for testing it is more
	// convenient to invoke the circuit in all the tests as this is a very a CPU
	// greedy part.
	HasCircuit bool
}

// newSha2BlockModule generates all the constraints necessary to ensure that the
// calls to the sha2 compression function have been correctly called.
func newSha2BlockModule(comp *wizard.CompiledIOP, inp *sha2BlocksInputs) *sha2BlockModule {

	var (
		colSize       = utils.NextPowerOfTwo(inp.MaxNbBlockPerCirc * inp.MaxNbCircuit * numRowPerInstance)
		declareCommit = func(s string) ifaces.Column {
			return comp.InsertCommit(
				0,
				ifaces.ColID(inp.Name+"_"+s),
				colSize,
			)
		}

		res = &sha2BlockModule{
			Inputs:                  inp,
			IsActive:                declareCommit("IS_ACTIVE"),
			IsEffBlock:              declareCommit("IS_EFF_BLOCK"),
			IsEffFirstLaneOfNewHash: declareCommit("IS_EFF_FIRST_LANE_OF_NEW_HASH"),
			IsEffLastLaneOfCurrHash: declareCommit("IS_EFF_LAST_LANE_OF_CURR_HASH"),
			HashHi:                  declareCommit("HASH_HI"),
			HashLo:                  declareCommit("HASH_LO"),
			Limbs:                   declareCommit("LIMBS"),
		}
	)

	pragmas.MarkRightPadded(res.IsActive)

	res.CanBeBeginningOfInstance = dedicated.CreateHeartBeat(
		comp,
		0,
		numRowPerInstance,
		0,
		res.IsActive,
	)

	res.CanBeEndOfInstance = dedicated.CreateHeartBeat(
		comp,
		0,
		numRowPerInstance,
		numRowPerInstance-1,
		res.IsActive,
	)

	res.CanBeBlockOfInstance = dedicated.NewRepeatedPattern(
		comp,
		0,
		vector.ForTest(0, 0, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 1, 0, 0),
		res.IsActive,
	)

	res.IsEffFirstLaneOfNewHashShiftMin2 = dedicated.ManuallyShift(comp, res.IsEffFirstLaneOfNewHash, -2, "IS_EFF_FIRST_LANE_OF_NEW_HASH_SHIFT_MIN_2")

	commonconstraints.MustBeActivationColumns(comp, res.IsActive)

	// IsActive can only go from zero to 1 if isLastLane is set to one in the
	// row above.
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_IS_ACTIVE_FINISH_AFTER_END", inp.Name),
		sym.Mul(
			sym.Sub(column.Shift(res.IsActive, -1), res.IsActive),
			sym.Sub(1, column.Shift(res.CanBeEndOfInstance.Natural, -1)),
		),
	)

	csIsMasked := func(canBe, isEff ifaces.Column) {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_FROM_%v", isEff.GetColID(), canBe.GetColID()),
			sym.Sub(isEff, sym.Mul(canBe, res.IsActive, isEff)),
		)
	}

	csIsMasked(res.CanBeBlockOfInstance.Natural, res.IsEffBlock)
	csIsMasked(res.CanBeBeginningOfInstance.Natural, res.IsEffFirstLaneOfNewHash) // @alex: Unsure this is even needed.
	csIsMasked(res.CanBeEndOfInstance.Natural, res.IsEffLastLaneOfCurrHash)

	commonconstraints.MustZeroWhenInactive(
		comp,
		res.IsActive,
		res.HashHi,
		res.HashLo,
		res.Limbs,
	)

	// res.IsEffLastLaneOfCurrHash == 1 IFF EITHER
	//		- Next row has IsEffFirstLaneOfNewHash == 1
	// 		- Active[i] == 1 AND Active[i+1] == 0
	//
	//	Note: both conditions are incompatible
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_IS_EFF_LAST_LANE_IS_WELL_SET", inp.Name),
		sym.Sub(
			res.IsEffLastLaneOfCurrHash,
			column.Shift(res.IsEffFirstLaneOfNewHash, 1),
			sym.Sub(res.IsActive, column.Shift(res.IsActive, 1)),
		),
	)

	// If we are at the beginning of a new hash, then the "oldState" is some
	// specified initialization vector.
	//
	// The constraint is broken down in two smaller constraints: one for each
	// limb of the old state.
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SET_IV_0_FOR_OLD_STATE", inp.Name),
		sym.Mul(
			res.IsEffFirstLaneOfNewHash,
			sym.Sub(res.Limbs, initializationVector[0]),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SET_IV_1_FOR_OLD_STATE", inp.Name),
		sym.Mul(
			res.IsEffFirstLaneOfNewHash,
			sym.Sub(column.Shift(res.Limbs, 1), initializationVector[1]),
		),
	)

	// If we are not at the beginning of a new hash but are still at the beginning
	// of an instance, then the "oldState" value should be equal to the "newState"
	// value of the previous instance. This is done in two constraints.
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_REUSING_PREV_HASHING_STATE_0", inp.Name),
		sym.Mul(
			sym.Sub(1, res.IsEffFirstLaneOfNewHash),
			sym.Mul(res.CanBeBeginningOfInstance.Natural, res.IsActive),
			sym.Sub(res.Limbs, column.Shift(res.Limbs, -2)),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_REUSING_PREV_HASHING_STATE_1", inp.Name),
		sym.Mul(
			sym.Sub(1, res.IsEffFirstLaneOfNewHash),
			sym.Mul(res.CanBeBeginningOfInstance.Natural, res.IsActive),
			sym.Sub(column.Shift(res.Limbs, 1), column.Shift(res.Limbs, -1)),
		),
	)

	// If we are at the end of the current hash, then the newState value must
	// be equals to HASH_HI, HASH_LO
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SET_HASH_HI", inp.Name),
		sym.Mul(
			res.IsEffLastLaneOfCurrHash,
			sym.Sub(res.HashHi, column.Shift(res.Limbs, -1)),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_SET_HASH_LO", inp.Name),
		sym.Mul(
			res.IsEffLastLaneOfCurrHash,
			sym.Sub(res.HashLo, res.Limbs),
		),
	)

	// Unless the current row correspond to the end of the current hash, the
	// values of HASH_HI/LO should be equal to those of the next row.
	//

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_KEEP_HASH_HI", inp.Name),
		sym.Mul(
			sym.Sub(1, res.IsEffLastLaneOfCurrHash),
			sym.Sub(column.Shift(res.HashHi, 1), res.HashHi),
		),
	)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_KEEP_HASH_LO", inp.Name),
		sym.Mul(
			sym.Sub(1, res.IsEffLastLaneOfCurrHash),
			sym.Sub(column.Shift(res.HashLo, 1), res.HashLo),
		),
	)

	// The following query ensures that the data in limbs corresponding to
	// limbs are exactly those provided by the input module.

	comp.InsertProjection(
		ifaces.QueryIDf("%v_PROJECTION_INPUT", inp.Name),
		query.ProjectionInput{
			ColumnA: []ifaces.Column{
				res.Inputs.IsFirstLaneOfNewHash,
				res.Inputs.PackedUint32,
			},
			ColumnB: []ifaces.Column{
				res.IsEffFirstLaneOfNewHashShiftMin2.Natural,
				res.Limbs,
			},
			FilterA: res.Inputs.Selector,
			FilterB: res.IsEffBlock})

	// As per the padding technique we use, the HashHi and HashLo should not
	// be zero when isActive.
	var ctxLo, ctxHi wizard.ProverAction

	res.HashHiIsZero, ctxHi = dedicated.IsZero(comp, res.HashHi)
	res.HashLoIsZero, ctxLo = dedicated.IsZero(comp, res.HashLo)
	res.ProverActions = append(res.ProverActions, ctxHi, ctxLo)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_HASH_CANT_BE_BOTH_ZERO", inp.Name),
		sym.Mul(
			res.IsActive,
			sym.Add(res.HashHiIsZero, res.HashLoIsZero),
		),
	)

	return res
}

func (sbh *sha2BlockModule) WithCircuit(comp *wizard.CompiledIOP, options ...query.PlonkOption) *sha2BlockModule {

	sbh.HasCircuit = true

	sbh.GnarkCircuitConnector = plonk.DefineAlignment(
		comp,
		&plonk.CircuitAlignmentInput{
			Name:               sbh.Inputs.Name + "_SHA2_COMPRESSION_CIRCUIT",
			DataToCircuit:      sbh.Limbs,
			DataToCircuitMask:  sbh.IsActive,
			Circuit:            allocateSha2Circuit(sbh.Inputs.MaxNbBlockPerCirc),
			NbCircuitInstances: sbh.Inputs.MaxNbCircuit,
			PlonkOptions:       options,
		},
	)

	return sbh
}
