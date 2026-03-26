package sha2

import (
	"fmt"

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
	// numLimbBytes each limb is 16 bits, so 2 bytes.
	numLimbBytes = 2
	// limbBytesStart is the start index in the Bytes representation of the
	// field.Element that corresponds to the limb data.
	limbBytesStart = field.Bytes - numLimbBytes
	// stateSizeBytes is SHA2 state size in bytes.
	stateSizeBytes = 32
	// blockSizeBytes is SHA2 block size in bytes.
	blockSizeBytes = 64
	// numLimbsPerState is the number of limbs to represent a single hash value - 256
	// bits are represented as 16 uint16.
	numLimbsPerState = stateSizeBytes / numLimbBytes
	// numLimbsPerBlock is the number of rows to represent a single block - 512 bits
	// are represented as 32 uint16.
	numLimbsPerBlock = blockSizeBytes / numLimbBytes
	// number of rows taken by a single instance of Sha2-block: 32 for the block
	// lanes, as the lane (256 bit) is packed in 32 uint16, 16 uint16 (256 bits
	// in total) for the initial hash and 16 uint16 (256 bits in total) for the
	// final hash.
	numRowPerInstance = numLimbsPerBlock + numLimbsPerState + numLimbsPerState
)

var (
	// initializationVector encodes the initialization vector of SHA2 in 2 field
	// elements storing each 128 bytes of the IV in big endian order.
	//
	// 0x6A09E667BB67AE853C6EF372A54FF53A510E527F9B05688C1F83D9AB5BE0CD19
	initializationVector = [16]field.Element{
		// 0x6A09E667BB67AE853C6EF372A54FF53A
		field.NewFromString("0x6a09"),
		field.NewFromString("0xe667"),
		field.NewFromString("0xbb67"),
		field.NewFromString("0xae85"),
		field.NewFromString("0x3c6e"),
		field.NewFromString("0xf372"),
		field.NewFromString("0xa54f"),
		field.NewFromString("0xf53a"),
		// 0x510E527F9B05688C1F83D9AB5BE0CD19
		field.NewFromString("0x510e"),
		field.NewFromString("0x527f"),
		field.NewFromString("0x9b05"),
		field.NewFromString("0x688c"),
		field.NewFromString("0x1f83"),
		field.NewFromString("0xd9ab"),
		field.NewFromString("0x5be0"),
		field.NewFromString("0xcd19"),
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

	// PackedUint16 contains the blocks given to the Sha2 hasher as sequences of
	// uint16.
	PackedUint16 ifaces.Column

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

	// IsEffFirstLaneOfNewHashShiftMin16 is a manually shifted version of the
	// [IsEffFirstLaneOfNewHash] column with an offset of -[numLimbsPerState].
	IsEffFirstLaneOfNewHashShiftMin16 *dedicated.ManuallyShifted

	// IsEffLastLaneOfCurrHash is a binary indicator column indicating with a 1
	// the last row of every hash. It is used to ensure that HashHi and HashLo
	// are well constructed.
	//
	// The column is constructed by summing (IsNewHash << 1) and
	// (isActive - isActive << 1).
	IsEffLastLaneOfCurrHash ifaces.Column

	// Limb stores the inputs to send to the circuit
	Limbs ifaces.Column

	// Hash stores parts of the hashing result. The columns are constants in the
	// span of a hash.
	Hash [numLimbsPerState]ifaces.Column

	HashIsZero    [numLimbsPerState]ifaces.Column
	ProverActions []wizard.ProverAction

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
				true,
			)
		}

		res = &sha2BlockModule{
			Inputs:                  inp,
			IsActive:                declareCommit("IS_ACTIVE"),
			IsEffBlock:              declareCommit("IS_EFF_BLOCK"),
			IsEffFirstLaneOfNewHash: declareCommit("IS_EFF_FIRST_LANE_OF_NEW_HASH"),
			IsEffLastLaneOfCurrHash: declareCommit("IS_EFF_LAST_LANE_OF_CURR_HASH"),
			Limbs:                   declareCommit("LIMBS"),
		}
	)

	pragmas.MarkRightPadded(res.IsActive)
	for i := range numLimbsPerState {
		res.Hash[i] = declareCommit(fmt.Sprintf("HASH_%d", i))
	}

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
		canBeBlockOfInstancePattern(),
		res.IsActive,
		"SHA2_BLOCK",
	)

	res.IsEffFirstLaneOfNewHashShiftMin16 = dedicated.ManuallyShift(comp, res.IsEffFirstLaneOfNewHash, -numLimbsPerState, "_IS_EFF_FIRST_LANE_OF_NEW_HASH_SHIFT_MIN_16")

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
		append(res.Hash[:], res.Limbs)...,
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

	for i := range initializationVector {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SET_IV_FOR_OLD_STATE_%d", inp.Name, i),
			sym.Mul(
				res.IsEffFirstLaneOfNewHash,
				sym.Sub(column.Shift(res.Limbs, i), initializationVector[i]),
			),
			true,
		)
	}

	// If we are not at the beginning of a new hash but are still at the beginning
	// of an instance, then the "oldState" value should be equal to the "newState"
	// value of the previous instance.
	//

	for i := range numLimbsPerState {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_REUSING_PREV_HASHING_STATE_%d", inp.Name, i),
			sym.Mul(
				sym.Sub(1, res.IsEffFirstLaneOfNewHash),
				sym.Mul(res.CanBeBeginningOfInstance.Natural, res.IsActive),
				sym.Sub(column.Shift(res.Limbs, i), column.Shift(res.Limbs, i-numLimbsPerState)),
			),
			true,
		)
	}

	// If we are at the end of the current hash, then the newState value must
	// be equals to HASH.
	//

	for i := range numLimbsPerState {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_SET_HASH_%d", inp.Name, i),
			sym.Mul(
				res.IsEffLastLaneOfCurrHash,
				sym.Sub(res.Hash[i], column.Shift(res.Limbs, -numLimbsPerState+i+1)),
			),
		)
	}

	// Unless the current row correspond to the end of the current hash, the
	// values of HASH should be equal to those of the next row.
	//

	for i := range numLimbsPerState {
		comp.InsertGlobal(0,
			ifaces.QueryIDf("%v_KEEP_HASH_%d", inp.Name, i),
			sym.Mul(
				sym.Sub(1, res.IsEffLastLaneOfCurrHash),
				sym.Sub(column.Shift(res.Hash[i], 1), res.Hash[i]),
			),
		)
	}

	// The following query ensures that the data in limbs corresponding to
	// limbs are exactly those provided by the input module.

	comp.InsertProjection(
		ifaces.QueryIDf("%v_PROJECTION_INPUT", inp.Name),
		query.ProjectionInput{
			ColumnA: []ifaces.Column{
				res.Inputs.IsFirstLaneOfNewHash,
				res.Inputs.PackedUint16,
			},
			ColumnB: []ifaces.Column{
				res.IsEffFirstLaneOfNewHashShiftMin16.Natural,
				res.Limbs,
			},
			FilterA: res.Inputs.Selector,
			FilterB: res.IsEffBlock,
		},
	)

	// As per the padding technique we use, the HashHi and HashLo should not
	// be zero when isActive.
	var ctxHash [numLimbsPerState]wizard.ProverAction

	sumHash := sym.NewConstant(0)
	for i := range numLimbsPerState {
		res.HashIsZero[i], ctxHash[i] = dedicated.IsZero(comp, res.Hash[i]).GetColumnAndProverAction()
		sumHash = sym.Add(sumHash, res.HashIsZero[i])
	}

	res.ProverActions = append(res.ProverActions, ctxHash[:]...)

	comp.InsertGlobal(0,
		ifaces.QueryIDf("%v_HASH_CANT_BE_BOTH_ZERO", inp.Name),
		sym.Mul(res.IsActive, sumHash),
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

func canBeBlockOfInstancePattern() []field.Element {
	pattern := make([]field.Element, numRowPerInstance)

	for i := range numLimbsPerState {
		pattern[i] = field.Zero()
	}

	offset := numLimbsPerState
	for i := range numLimbsPerBlock {
		pattern[offset+i] = field.One()
	}

	offset += numLimbsPerBlock
	for i := range numLimbsPerState {
		pattern[offset+i] = field.Zero()
	}

	return pattern
}
