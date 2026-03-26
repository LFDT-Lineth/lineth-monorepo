package accumulator

import (
	"encoding/binary"
	"fmt"
	"io"

	"github.com/consensys/gnark-crypto/field/koalabear/vortex"
	poseidon2 "github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/exit"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// leafOpenings represents the structure for leaf openings
type leafOpenings struct {
	prev [common.NbElemForHasingU64][]field.Element
	next [common.NbElemForHasingU64][]field.Element
	hKey [common.NbElemPerHash][]field.Element
	hVal [common.NbElemPerHash][]field.Element
}

type hashType int

const (
	// denote the hashtype for hashing u64 values
	hashU64 hashType = iota
	// denote the hashtype for hashing field hashes
	hashFieldElements
	// denote the hashtype for hashing byte32 values for the sandwitch check
	hashByte32Sandwitch
)

// assignmentBuilder is used to build the assignment of the [Module] module
// and is implemented like a writer.
type assignmentBuilder struct {

	// Setting contains the setting with which the corresponding accumulator
	// module was instantiated.
	Settings

	// leaves stores the assignment of the column holding the leaves (so the
	// hash of the leaf openings) for which we give the merkle proof. This corresponds to the
	// [Accumulator.Cols.Leaves] column.
	leaves [common.NbElemPerHash][]field.Element
	// positions stores the positions of the leaves in the merkle tree for which we give the Merkle proof. This corresponds to the [Accumulator.Cols.Positions] column.
	positions [common.NbElemForHasingU64][]field.Element
	// roots stores the roots of the merkle tree. This corresponds
	// to the [Accumulator.Cols.Roots] column.
	roots [common.NbElemPerHash][]field.Element
	// proofs stores the path and siblings of the merkle proof. Those siblings corresponds
	// to the [Accumulator.Cols.Proofs] column.
	proofs []smt_koalabear.Proof
	// useNextMerkleProof is assigned starting from 1, then 0, followed by 0 and so on in each row of INSERT,
	// UPDATE and DELETE. For READZERO and READNONZERO it is set to 0 for each row
	useNextMerkleProof []field.Element
	// isActive is assigned to 1 for each row of every operation. This corresponds to the [Accumulator.Cols.IsActiveAccumulator] column
	isActive []field.Element
	// accumulatorCounter counts the number of rows in the accumulator. It is used to check the
	// sequentiality of leaves and roots in accumulator and the merkle module
	accumulatorCounter [common.NbElemForHasingU64][]field.Element
	// isFirst is one at the first row of any operation. This corresponds to the [Accumulator.IsFirst] column
	isFirst []field.Element
	// isInsert is one when we have an INSERT operation. It is
	// zero otherwise. This corresponds to the [Accumulator.Cols.IsInsert] column.
	isInsert []field.Element
	// isDelete is one when we have a DELETE operation. It is
	// zero otherwise. This corresponds to the [Accumulator.Cols.IsDelete] column.
	isDelete []field.Element
	// isUpdate is one when we have an UPDATE operation. It is
	// zero otherwise. This corresponds to the [Accumulator.Cols.IsUpdate] column.
	isUpdate []field.Element
	// isReadZero is one when we have  a staring of a ReadZero operation. It is
	// zero otherwise. This corresponds to the [Accumulator.Cols.IsReadZero] column
	isReadZero []field.Element
	// isReadNonZero is one when we have  a staring of a ReadNonZero operation. It is
	// zero otherwise. This corresponds to the [Accumulator.Cols.IsReadNonZero] column
	isReadNonZero []field.Element
	// hKey is the hash of the key of the trace. This corresponds to the [Accumulator.Column.HKey]
	hKey [common.NbElemPerHash][]field.Element
	// hKeyMinus is the hash of the key of the previous leaf. This corresponds to the [Accumulator.Column.HKeyMinus]
	hKeyMinus [common.NbElemPerHash][]field.Element
	// hKeyPlus is the hash of the key of the next leaf. This corresponds to the [Accumulator.Column.HKeyPlus]
	hKeyPlus [common.NbElemPerHash][]field.Element
	// hKeySandwitch is the hash of the key of the trace for the sandwitch check. This corresponds to the [Accumulator.Column.HKeySandwitch]
	hKeySandwitch [common.NbElemForHashingByte32Sandwitch][]field.Element
	// hKeyMinusSandwitch is the hash of the key of the previous leaf for the sandwitch check. This corresponds to the [Accumulator.Column.HKeyMinusSandwitch]
	hKeyMinusSandwitch [common.NbElemForHashingByte32Sandwitch][]field.Element
	// hKeyPlusSandwitch is the hash of the key of the next leaf for the sandwitch check. This corresponds to the [Accumulator.Column.HKeyPlusSandwitch]
	hKeyPlusSandwitch [common.NbElemForHashingByte32Sandwitch][]field.Element
	// Pointer check columns
	// leafMinusIndex is the index of the minus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafMinusIndex]
	leafMinusIndex [common.NbElemForHasingU64][]field.Element
	// leafMinusNext is the index of the Next leaf of the minus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafMinusNext]
	leafMinusNext [common.NbElemForHasingU64][]field.Element
	// leafMinusNext is the index of the plus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafPlusIndex]
	leafPlusIndex [common.NbElemForHasingU64][]field.Element
	// leafPlusPrev is the index of the Previous leaf of the plus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafPlusPrev]
	leafPlusPrev [common.NbElemForHasingU64][]field.Element
	// leafDeletedIndex is the index of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedIndex]
	leafDeletedIndex [common.NbElemForHasingU64][]field.Element
	// leafDeletedPrev is the index of the Previous leaf of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedPrev]
	leafDeletedPrev [common.NbElemForHasingU64][]field.Element
	// leafDeletedNext is the index of the Previous leaf of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedNext]
	leafDeletedNext [common.NbElemForHasingU64][]field.Element
	// leafOpening is a tuple of four columns containing
	// Prev, Next, HKey, HVal of a leaf. This corresponds to the [Accumulator.Column.LeafOpening]
	leafOpening leafOpenings
	// interm is a slice containing 5 intermediate hash states. This corresponds to the [Accumulator.Column.Interm]
	interm [common.NbElemPerHash][][]field.Element
	// leafHash contains sequential Poseidon2 hashes of leafOpening. It matches with Leaves except when there is empty leaf. This corresponds to the [Accumulator.Column.LeafHashes]
	leafHashes [common.NbElemPerHash][]field.Element
	// isEmptyLeaf is one when Leaves contains empty leaf and does not match with LeafHash
	isEmptyLeaf []field.Element
	// nextFreeNode contains the nextFreeNode for each row of every operation
	nextFreeNode [common.NbElemForHasingU64][]field.Element
	// insertionPath is the path of a newly inserted leaf when INSERT happens,
	// it is zero otherwise
	insertionPath [common.NbElemForHasingU64][]field.Element
	// isInsertRow3 is one for row 3 of INSERT operation
	isInsertRow3 []field.Element
	// intermZeroTopRoot contains the the first intermediate Poseidon2 state hash
	intermZeroTopRoot [common.NbElemPerHash][]field.Element
	// intermOneTopRoot contains the second intermediate Poseidon2 state hash
	intermOneTopRoot [common.NbElemPerHash][]field.Element
	// topRoot contains the Poseidon2 hash of SubTreeRoot and NextFreeNode
	topRoot [common.NbElemPerHash][]field.Element
}

// newAssignmentBuilder returns an empty builder
func newAssignmentBuilder(s Settings) *assignmentBuilder {
	amb := assignmentBuilder{}
	amb.Settings = s

	for i := 0; i < common.NbElemPerHash; i++ {
		amb.roots[i] = make([]field.Element, 0, amb.NumRows())
		amb.leaves[i] = make([]field.Element, 0, amb.NumRows())
		amb.hKey[i] = make([]field.Element, 0, amb.NumRows())
		amb.hKeyMinus[i] = make([]field.Element, 0, amb.NumRows())
		amb.hKeyPlus[i] = make([]field.Element, 0, amb.NumRows())
	}

	for i := 0; i < common.NbElemForHashingByte32Sandwitch; i++ {
		amb.hKeySandwitch[i] = make([]field.Element, 0, amb.NumRows())
		amb.hKeyMinusSandwitch[i] = make([]field.Element, 0, amb.NumRows())
		amb.hKeyPlusSandwitch[i] = make([]field.Element, 0, amb.NumRows())
	}

	for i := 0; i < len(amb.positions); i++ {
		amb.positions[i] = make([]field.Element, 0, amb.NumRows())

		amb.nextFreeNode[i] = make([]field.Element, 0, amb.NumRows())
		amb.insertionPath[i] = make([]field.Element, 0, amb.NumRows())
		amb.accumulatorCounter[i] = make([]field.Element, 0, amb.NumRows())

		amb.leafMinusIndex[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafMinusNext[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafPlusIndex[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafPlusPrev[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafDeletedIndex[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafDeletedPrev[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafDeletedNext[i] = make([]field.Element, 0, amb.NumRows())
	}

	for i := 0; i < len(amb.leafOpening.prev); i++ {
		amb.leafOpening.prev[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafOpening.next[i] = make([]field.Element, 0, amb.NumRows())
	}

	amb.proofs = make([]smt_koalabear.Proof, 0, amb.NumRows())
	amb.useNextMerkleProof = make([]field.Element, 0, amb.NumRows())
	amb.isActive = make([]field.Element, 0, amb.NumRows())
	amb.isFirst = make([]field.Element, 0, amb.NumRows())
	amb.isInsert = make([]field.Element, 0, amb.NumRows())
	amb.isDelete = make([]field.Element, 0, amb.NumRows())
	amb.isUpdate = make([]field.Element, 0, amb.NumRows())
	amb.isReadZero = make([]field.Element, 0, amb.NumRows())
	amb.isReadNonZero = make([]field.Element, 0, amb.NumRows())

	for i := 0; i < len(amb.leafHashes); i++ {
		amb.leafHashes[i] = make([]field.Element, 0, amb.NumRows())

		amb.leafOpening.hKey[i] = make([]field.Element, 0, amb.NumRows())
		amb.leafOpening.hVal[i] = make([]field.Element, 0, amb.NumRows())
		amb.intermZeroTopRoot[i] = make([]field.Element, 0, amb.NumRows())
		amb.topRoot[i] = make([]field.Element, 0, amb.NumRows())

		amb.interm[i] = make([][]field.Element, 5)
		for j := 0; j < len(amb.interm[i]); j++ {
			amb.interm[i][j] = make([]field.Element, 0, amb.NumRows())
		}
	}

	amb.isEmptyLeaf = make([]field.Element, 0, amb.NumRows())
	amb.isInsertRow3 = make([]field.Element, 0, amb.NumRows())

	return &amb
}

// Assign is a high level function which is used to arithmetize the columns
// of the Accumulator module from a slice of decoded traces
func (am *Module) Assign(
	run *wizard.ProverRuntime,
	// The traces parsed for the state-manager inspection process
	traces []statemanager.DecodedTrace,
) {

	if len(traces) == 0 {
		utils.Panic("no state-manager traces, that's impossible.")
	}

	var (
		builder    = newAssignmentBuilder(am.Settings)
		paddedSize = am.NumRows()
	)

	for _, trace := range traces {
		// only assign the traces that are flagged as not to be skipped
		if !trace.IsSkipped {
			switch t := trace.Underlying.(type) {
			case statemanager.UpdateTraceST:
				pushUpdateRows(builder, t)
			case statemanager.UpdateTraceWS:
				pushUpdateRows(builder, t)
			case statemanager.InsertionTraceST:
				pushInsertionRows(builder, t)
			case statemanager.InsertionTraceWS:
				pushInsertionRows(builder, t)
			case statemanager.DeletionTraceST:
				pushDeletionRows(builder, t)
			case statemanager.DeletionTraceWS:
				pushDeletionRows(builder, t)
			case statemanager.ReadZeroTraceST:
				pushReadZeroRows(builder, t)
			case statemanager.ReadZeroTraceWS:
				pushReadZeroRows(builder, t)
			case statemanager.ReadNonZeroTraceST:
				pushReadNonZeroRows(builder, t)
			case statemanager.ReadNonZeroTraceWS:
				pushReadNonZeroRows(builder, t)
			default:
				utils.Panic("Unexpected type : %T", t)
			}
		}
	}

	// Sanity check on the size
	if len(builder.leaves[0]) > am.MaxNumProofs {
		exit.OnLimitOverflow(
			am.MaxNumProofs,
			len(builder.leaves[0]),
			fmt.Errorf("we have registered %v proofs which is more than the maximum number of proofs %v", len(builder.leaves[0]), am.MaxNumProofs),
		)
	}

	// Assignments of columns
	cols := am.Cols
	// Assignment of the repeated pattern of the offset limbs
	for i := 0; i < common.NbElemForHasingU64; i++ {
		am.OffsetLimbRepeated[i].Assign(run)
	}

	cols.Proofs.Assign(run, builder.proofs)

	for i := 0; i < len(cols.Positions); i++ {
		run.AssignColumn(cols.Positions[i].GetColID(), smartvectors.RightZeroPadded(builder.positions[i], paddedSize))

		// Assignment for NextFreeNode checking columns
		run.AssignColumn(cols.NextFreeNode[i].GetColID(), smartvectors.RightZeroPadded(builder.nextFreeNode[i], paddedSize))
		run.AssignColumn(cols.InsertionPath[i].GetColID(), smartvectors.RightZeroPadded(builder.insertionPath[i], paddedSize))
		run.AssignColumn(cols.AccumulatorCounter[i].GetColID(), smartvectors.RightZeroPadded(builder.accumulatorCounter[i], paddedSize))

		// assignments for the pointer check columns
		run.AssignColumn(cols.LeafMinusIndex[i].GetColID(), smartvectors.RightZeroPadded(builder.leafMinusIndex[i], paddedSize))
		run.AssignColumn(cols.LeafMinusNext[i].GetColID(), smartvectors.RightZeroPadded(builder.leafMinusNext[i], paddedSize))
		run.AssignColumn(cols.LeafPlusIndex[i].GetColID(), smartvectors.RightZeroPadded(builder.leafPlusIndex[i], paddedSize))
		run.AssignColumn(cols.LeafPlusPrev[i].GetColID(), smartvectors.RightZeroPadded(builder.leafPlusPrev[i], paddedSize))
		run.AssignColumn(cols.LeafDeletedIndex[i].GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedIndex[i], paddedSize))
		run.AssignColumn(cols.LeafDeletedPrev[i].GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedPrev[i], paddedSize))
		run.AssignColumn(cols.LeafDeletedNext[i].GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedNext[i], paddedSize))
	}

	for i := 0; i < common.NbElemPerHash; i++ {
		run.AssignColumn(cols.Roots[i].GetColID(), smartvectors.RightZeroPadded(builder.roots[i], paddedSize))
		run.AssignColumn(cols.Leaves[i].GetColID(), smartvectors.RightZeroPadded(builder.leaves[i], paddedSize))

		// assignments for the sandwitch check columns
		run.AssignColumn(cols.HKey[i].GetColID(), smartvectors.RightZeroPadded(builder.hKey[i], paddedSize))
		run.AssignColumn(cols.HKeyMinus[i].GetColID(), smartvectors.RightZeroPadded(builder.hKeyMinus[i], paddedSize))
		run.AssignColumn(cols.HKeyPlus[i].GetColID(), smartvectors.RightZeroPadded(builder.hKeyPlus[i], paddedSize))
	}

	// assignments for the sandwitch check columns
	for i := 0; i < common.NbElemForHashingByte32Sandwitch; i++ {
		run.AssignColumn(cols.HKeySandwitch[i].GetColID(), smartvectors.RightZeroPadded(builder.hKeySandwitch[i], paddedSize))
		run.AssignColumn(cols.HKeyMinusSandwitch[i].GetColID(), smartvectors.RightZeroPadded(builder.hKeyMinusSandwitch[i], paddedSize))
		run.AssignColumn(cols.HKeyPlusSandwitch[i].GetColID(), smartvectors.RightZeroPadded(builder.hKeyPlusSandwitch[i], paddedSize))
	}

	// Boolean elements assignment
	run.AssignColumn(cols.UseNextMerkleProof.GetColID(), smartvectors.RightZeroPadded(builder.useNextMerkleProof, paddedSize))
	run.AssignColumn(cols.IsActiveAccumulator.GetColID(), smartvectors.RightZeroPadded(builder.isActive, paddedSize))
	run.AssignColumn(cols.IsFirst.GetColID(), smartvectors.RightZeroPadded(builder.isFirst, paddedSize))
	run.AssignColumn(cols.IsInsert.GetColID(), smartvectors.RightZeroPadded(builder.isInsert, paddedSize))
	run.AssignColumn(cols.IsDelete.GetColID(), smartvectors.RightZeroPadded(builder.isDelete, paddedSize))
	run.AssignColumn(cols.IsUpdate.GetColID(), smartvectors.RightZeroPadded(builder.isUpdate, paddedSize))
	run.AssignColumn(cols.IsReadZero.GetColID(), smartvectors.RightZeroPadded(builder.isReadZero, paddedSize))
	run.AssignColumn(cols.IsReadNonZero.GetColID(), smartvectors.RightZeroPadded(builder.isReadNonZero, paddedSize))

	// Assign Interm, LeafOpenings, and LeafHashes columns
	am.assignLeaf(run, builder)

	// Assignment for NextFreeNode checking columns
	run.AssignColumn(cols.IsInsertRow3.GetColID(), smartvectors.RightZeroPadded(builder.isInsertRow3, paddedSize))

	// Assign TopRoot hash checking columns
	am.assignTopRootCols(run, builder)

	// This prover action assigns all the Merkle proofs.
	am.MerkleProofVerification.Run(run)

	// Checks row-wise increment of AccumulatorCounter
	// IsActiveAccumulator[i+1] * (AccumulatorCounter[i+1] - AccumulatorCounter[i] - 1)
	am.AccumulatorCounterProver.Run(run)

	// Sandwich check
	// Checks that HKey > HKeyMinus
	am.HkeyHkeyMinusProver.Run(run)

	// Checks that HKeyPlus > HKey
	am.HkeyPlusHkeyProver.Run(run)

	// Checks that on insert NextFreeNode is incremented by 1
	am.NextFreeNodeShiftProver.Run(run)
}

func (am *Module) assignLeaf(
	run *wizard.ProverRuntime,
	builder *assignmentBuilder) {

	var (
		cols       = am.Cols
		paddedSize = am.NumRows()
	)

	run.AssignColumn(cols.IsEmptyLeaf.GetColID(), smartvectors.RightZeroPadded(builder.isEmptyLeaf, paddedSize))

	for i := range builder.leafOpening.prev {
		run.AssignColumn(cols.LeafOpenings.Prev[i].GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.prev[i], paddedSize))
		run.AssignColumn(cols.LeafOpenings.Next[i].GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.next[i], paddedSize))
	}

	for i := range cols.LeafHashes {
		run.AssignColumn(cols.LeafHashes[i].GetColID(), smartvectors.RightZeroPadded(builder.leafHashes[i], paddedSize))
		run.AssignColumn(cols.IntermZero[i].GetColID(), smartvectors.RightZeroPadded(builder.interm[i][0], paddedSize))
		run.AssignColumn(cols.IntermOne[i].GetColID(), smartvectors.RightZeroPadded(builder.interm[i][1], paddedSize))
		run.AssignColumn(cols.IntermTwo[i].GetColID(), smartvectors.RightZeroPadded(builder.interm[i][2], paddedSize))
		run.AssignColumn(cols.IntermThree[i].GetColID(), smartvectors.RightZeroPadded(builder.interm[i][3], paddedSize))
		run.AssignColumn(cols.IntermFour[i].GetColID(), smartvectors.RightZeroPadded(builder.interm[i][4], paddedSize))
		run.AssignColumn(cols.LeafOpenings.HKey[i].GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.hKey[i], paddedSize))
		run.AssignColumn(cols.LeafOpenings.HVal[i].GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.hVal[i], paddedSize))
	}

}

func (am *Module) assignTopRootCols(
	run *wizard.ProverRuntime,
	builder *assignmentBuilder) {
	cols := am.Cols
	paddedSize := am.NumRows()

	// compute the padding values for intermTopRoot and topRoot
	// var zeroBlock field.Octuplet
	// intermTopRootPadLimbs := vortex.CompressPoseidon2(zeroBlock, zeroBlock)
	// topRootPadLimbs := vortex.CompressPoseidon2(intermTopRootPadLimbs, zeroBlock)

	for i := range cols.IntermZeroTopRoot {
		run.AssignColumn(cols.IntermZeroTopRoot[i].GetColID(), smartvectors.RightPadded(builder.intermZeroTopRoot[i], field.Zero(), paddedSize))
		run.AssignColumn(cols.IntermOneTopRoot[i].GetColID(), smartvectors.RightPadded(builder.intermOneTopRoot[i], field.Zero(), paddedSize))
		run.AssignColumn(cols.TopRoot[i].GetColID(), smartvectors.RightPadded(builder.topRoot[i], field.Zero(), paddedSize))
	}
}

// This is a low level function used by all the operations (INSERT, UPDATE, DELETE, READ-ZERO, and READ-NONZERO)
// to update each row of various columns
func (a *assignmentBuilder) pushRow(
	leafOpening accumulator.LeafOpening,
	root types.KoalaOctuplet,
	proof smt_koalabear.Proof,
	nextFreeNode int,
	isFirst bool,
	isInsert bool,
	isInsertRow3 bool,
	isDelete bool,
	isUpdate bool,
	isReadZero bool,
	isReadNonZero bool,
	disableReuseMerkle bool,
	isSandwitchEnabled bool,
	isPointerEnabled bool,
	isEmptyLeaf bool,
) {
	// Populates leaves, leafHashes, and isEmptyLeaf from leafOpening by Posseidon2 block compression
	a.computeLeaf(leafOpening, isEmptyLeaf)

	rootFrLimbs := divideFieldBytesToFieldLimbs(root.ToBytes(), hashFieldElements)
	for i, limb := range rootFrLimbs {
		a.roots[i] = append(a.roots[i], limb)
	}

	// Insert Row 3 operations
	isInsRow3 := field.Zero()
	insPath := uint64(0)
	if isInsertRow3 {
		isInsRow3 = field.One()
		insPath = uint64(proof.Path)
	}

	insPathBytes := uint64To64Bytes(insPath)
	insPathLimbs := divideFieldBytesToFieldLimbs(insPathBytes[:], hashU64)
	a.isInsertRow3 = append(a.isInsertRow3, isInsRow3)

	// accumulatorCounter will increment when a new row is pushed
	accumulatorCounterBytes := uint64To64Bytes(uint64(len(a.accumulatorCounter[0])))
	accumulatorCounterLimbs := divideFieldBytesToFieldLimbs(accumulatorCounterBytes, hashU64)

	posBytes := uint64To64Bytes(uint64(proof.Path))
	posLimbs := divideFieldBytesToFieldLimbs(posBytes, hashU64)

	nextFreeNodeFrBytes := uint64To64Bytes(uint64(nextFreeNode))
	nextFreeNodeFrLimbs := divideFieldBytesToFieldLimbs(nextFreeNodeFrBytes, hashU64)

	for i, posLimb := range posLimbs {
		a.positions[i] = append(a.positions[i], posLimb)
		a.nextFreeNode[i] = append(a.nextFreeNode[i], nextFreeNodeFrLimbs[i])
		a.insertionPath[i] = append(a.insertionPath[i], insPathLimbs[i])

		// accumulatorCounter will increment when a new row is pushed
		a.accumulatorCounter[i] = append(a.accumulatorCounter[i], accumulatorCounterLimbs[i])
	}

	a.proofs = append(a.proofs, proof)

	// We assign intermZeroTopRoot = Poseidon2(zero, root), intermOneTopRoot = Poseidon2(intermZeroTopRoot, nextFreeNodeFirst)
	// and topRoot = Poseidon2(intermOneTopRoot, nextFreeNodeSecond).
	// We also assume nextFreeNodeFrLimbs is of length 2*common.NbElemPerHash
	nextFreeNodeFirst := nextFreeNodeFrLimbs[0:common.NbElemPerHash]
	nextFreeNodeSecond := nextFreeNodeFrLimbs[common.NbElemPerHash : 2*common.NbElemPerHash]
	zeroBlock := field.Octuplet{}

	intermZeroTopRootFrLimbs := vortex.CompressPoseidon2(zeroBlock, vortex.Hash(nextFreeNodeFirst))
	intermOneTopRootFrLimbs := vortex.CompressPoseidon2(intermZeroTopRootFrLimbs, vortex.Hash(nextFreeNodeSecond))
	topRootFrLimbs := vortex.CompressPoseidon2(intermOneTopRootFrLimbs, vortex.Hash(rootFrLimbs))
	for i := range a.intermZeroTopRoot {
		a.intermZeroTopRoot[i] = append(a.intermZeroTopRoot[i], intermZeroTopRootFrLimbs[i])
		a.intermOneTopRoot[i] = append(a.intermOneTopRoot[i], intermOneTopRootFrLimbs[i])
		a.topRoot[i] = append(a.topRoot[i], topRootFrLimbs[i])
	}

	// IsFirst
	isF := field.Zero()
	if isFirst {
		isF = field.One()
	}
	a.isFirst = append(a.isFirst, isF)
	// Insert operation
	isIns := field.Zero()
	if isInsert {
		isIns = field.One()
	}
	a.isInsert = append(a.isInsert, isIns)
	// Delete operation
	isDel := field.Zero()
	if isDelete {
		isDel = field.One()
	}
	a.isDelete = append(a.isDelete, isDel)
	// Update operation
	isUpd := field.Zero()
	if isUpdate {
		isUpd = field.One()
	}
	a.isUpdate = append(a.isUpdate, isUpd)
	// Read-Zero operation
	isReadZ := field.Zero()
	if isReadZero {
		isReadZ = field.One()
	}
	a.isReadZero = append(a.isReadZero, isReadZ)
	// Read-Non-Zero operation
	isReadNZ := field.Zero()
	if isReadNonZero {
		isReadNZ = field.One()
	}
	a.isReadNonZero = append(a.isReadNonZero, isReadNZ)

	// useNextMerkleProof is deduced from the length of the builder. This
	// leverages the property that this column has alternating values no
	// matter the type of traces being verified.
	useNextMerkleProof := len(a.useNextMerkleProof)%2 == 0
	if disableReuseMerkle {
		useNextMerkleProof = false
	}
	useNextMP := field.NewElement(0)
	if useNextMerkleProof {
		useNextMP = field.NewElement(1)
	}
	a.useNextMerkleProof = append(a.useNextMerkleProof, useNextMP)

	// no matter what we append, isActive will always be set to one. The zero
	// values will be appended during the padding phase.
	a.isActive = append(a.isActive, field.One())

	// if Sandwitch check is disabled then we append zero values to
	// hKey, hKeyPlus, hKeyMinus, hkeySandwitch, hKeyPlusSandwitch, hKeyMinusSandwitch
	if !isSandwitchEnabled {
		for i := range a.hKey {
			a.hKey[i] = append(a.hKey[i], field.Zero())
			a.hKeyMinus[i] = append(a.hKeyMinus[i], field.Zero())
			a.hKeyPlus[i] = append(a.hKeyPlus[i], field.Zero())
		}
		for i := range a.hKeySandwitch {
			a.hKeySandwitch[i] = append(a.hKeySandwitch[i], field.Zero())
			a.hKeyMinusSandwitch[i] = append(a.hKeyMinusSandwitch[i], field.Zero())
			a.hKeyPlusSandwitch[i] = append(a.hKeyPlusSandwitch[i], field.Zero())
		}
	}

	// if Pointer check is disabled then we append zero values to
	// leafMinusIndex, leafMinusNext, leafPlusIndex, leafPlusPrev,
	// leafDeletedIndex, leafDeletedPrev, leafDeletedNext
	if !isPointerEnabled {
		for i := range a.leafMinusIndex {
			a.leafMinusIndex[i] = append(a.leafMinusIndex[i], field.Zero())
			a.leafMinusNext[i] = append(a.leafMinusNext[i], field.Zero())
			a.leafPlusIndex[i] = append(a.leafPlusIndex[i], field.Zero())
			a.leafPlusPrev[i] = append(a.leafPlusPrev[i], field.Zero())
			a.leafDeletedIndex[i] = append(a.leafDeletedIndex[i], field.Zero())
			a.leafDeletedPrev[i] = append(a.leafDeletedPrev[i], field.Zero())
			a.leafDeletedNext[i] = append(a.leafDeletedNext[i], field.Zero())
		}
	}
}

func (a *assignmentBuilder) computeLeaf(leafOpening accumulator.LeafOpening, isEmptyLeaf bool) {
	var (
		zeroBlock field.Octuplet
	)
	if !isEmptyLeaf {
		prevFrBytes := uint64To64Bytes(uint64(leafOpening.Prev))
		prevFrLimbs := divideFieldBytesToFieldLimbs(prevFrBytes, hashU64)

		nextFrBytes := uint64To64Bytes(uint64(leafOpening.Next))
		nextFrLimbs := divideFieldBytesToFieldLimbs(nextFrBytes, hashU64)

		hKeyFrLimbs := divideFieldBytesToFieldLimbs(leafOpening.HKey.ToBytes(), hashFieldElements)
		hValFrLimbs := divideFieldBytesToFieldLimbs(leafOpening.HVal.ToBytes(), hashFieldElements)

		// As prevFrLimbs, nextFrLimbs are of 16 limbs, we hash them in two steps
		intermZeroLimbs := vortex.CompressPoseidon2(zeroBlock, vortex.Hash(prevFrLimbs[0:common.NbElemPerHash]))
		intermOneLimbs := vortex.CompressPoseidon2(intermZeroLimbs, vortex.Hash(prevFrLimbs[common.NbElemPerHash:common.NbElemForHasingU64]))
		intermTwoLimbs := vortex.CompressPoseidon2(intermOneLimbs, vortex.Hash(nextFrLimbs[0:common.NbElemPerHash]))
		intermThreeLimbs := vortex.CompressPoseidon2(intermTwoLimbs, vortex.Hash(nextFrLimbs[common.NbElemPerHash:common.NbElemForHasingU64]))
		intermFourLimbs := vortex.CompressPoseidon2(intermThreeLimbs, vortex.Hash(hKeyFrLimbs))
		leafLimbs := vortex.CompressPoseidon2(intermFourLimbs, vortex.Hash(hValFrLimbs))

		for i := range prevFrLimbs {
			a.leafOpening.prev[i] = append(a.leafOpening.prev[i], prevFrLimbs[i])
			a.leafOpening.next[i] = append(a.leafOpening.next[i], nextFrLimbs[i])
		}

		for i := range a.leaves {
			a.leaves[i] = append(a.leaves[i], leafLimbs[i])
			a.leafHashes[i] = append(a.leafHashes[i], leafLimbs[i])

			a.leafOpening.hKey[i] = append(a.leafOpening.hKey[i], hKeyFrLimbs[i])
			a.leafOpening.hVal[i] = append(a.leafOpening.hVal[i], hValFrLimbs[i])

			a.interm[i][0] = append(a.interm[i][0], intermZeroLimbs[i])
			a.interm[i][1] = append(a.interm[i][1], intermOneLimbs[i])
			a.interm[i][2] = append(a.interm[i][2], intermTwoLimbs[i])
			a.interm[i][3] = append(a.interm[i][3], intermThreeLimbs[i])
			a.interm[i][4] = append(a.interm[i][4], intermFourLimbs[i])
		}

		isEmpty := field.Zero()
		a.isEmptyLeaf = append(a.isEmptyLeaf, isEmpty)
	} else {
		intermZeroLimbs := vortex.CompressPoseidon2(zeroBlock, zeroBlock)
		intermOneLimbs := vortex.CompressPoseidon2(intermZeroLimbs, zeroBlock)
		intermTwoLimbs := vortex.CompressPoseidon2(intermOneLimbs, zeroBlock)
		intermThreeLimbs := vortex.CompressPoseidon2(intermTwoLimbs, zeroBlock)
		intermFourLimbs := vortex.CompressPoseidon2(intermThreeLimbs, zeroBlock)
		leafHashesLimbs := vortex.CompressPoseidon2(intermFourLimbs, zeroBlock)

		for i := range a.leafOpening.prev {
			a.leafOpening.prev[i] = append(a.leafOpening.prev[i], field.Zero())
			a.leafOpening.next[i] = append(a.leafOpening.next[i], field.Zero())
		}

		// We insert an empty leaf in the Leaves column in this case
		for i := range a.leaves {
			a.leaves[i] = append(a.leaves[i], zeroBlock[i])
			a.leafHashes[i] = append(a.leafHashes[i], leafHashesLimbs[i])

			a.leafOpening.hKey[i] = append(a.leafOpening.hKey[i], field.Zero())
			a.leafOpening.hVal[i] = append(a.leafOpening.hVal[i], field.Zero())

			a.interm[i][0] = append(a.interm[i][0], intermZeroLimbs[i])
			a.interm[i][1] = append(a.interm[i][1], intermOneLimbs[i])
			a.interm[i][2] = append(a.interm[i][2], intermTwoLimbs[i])
			a.interm[i][3] = append(a.interm[i][3], intermThreeLimbs[i])
			a.interm[i][4] = append(a.interm[i][4], intermFourLimbs[i])
		}

		isEmpty := field.One()
		a.isEmptyLeaf = append(a.isEmptyLeaf, isEmpty)
	}
}

/*
We have the below arithmetization for the UPDATE operation (the other columns are generated as per definition)
| UseNextProof 	| Proof       	| Root             	| Leaf    	| Pos              	|
|--------------	|-------------	|------------------	|---------	|------------------	|
| 1            	| trace.Proof 	| trace.OldSubRoot 	| OldLeaf 	| trace.Proof.Path 	|
| 0            	| trace.Proof 	| trace.NewSubRoot 	| NewLeaf 	| trace.Proof.Path 	|
*/
func pushUpdateRows[K, V io.WriterTo](
	a *assignmentBuilder,
	trace accumulator.UpdateTrace[K, V],
) {

	// row 1
	a.pushRow(
		trace.OldOpening,
		trace.OldSubRoot,
		trace.Proof,
		trace.NewNextFreeNode,
		true,  // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		true,  // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)
	newOpening := trace.OldOpening
	newOpening.HVal = hash(trace.NewValue)

	// row 2
	a.pushRow(
		newOpening,
		trace.NewSubRoot,
		trace.Proof,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		true,  // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)
}

/*
We have the below arithmetization for the INSERT operation (the other columns are generated as per definition)
| UseNextProof 	| Proof            	| Root                    	| Leaf         	| Pos                   	|
|--------------	|------------------	|-------------------------	|--------------	|-----------------------	|
| 1            	| trace.ProofMinus 	| trace.OldSubRoot        	| OldLeafMinus 	| trace.ProofMinus.Path 	|
| 0            	| trace.ProofMinus 	| trace.IntermediateRoot1 	| NewLeafMinus 	| trace.ProofMinus.Path 	|
| 1            	| trace.ProofNew   	| trace.IntermediateRoot1 	| emptyLeaf    	| trace.ProofNew.Path   	|
| 0            	| trace.ProofNew   	| trace.IntermediateRoot3 	| insertedLeaf 	| trace.ProofNew.Path   	|
| 1            	| trace.ProofPlus  	| trace.IntermediateRoot3 	| oldLeafPlus  	| trace.ProofPlus.Path  	|
| 0            	| trace.ProofPlus  	| trace.NewSubRoot        	| newLeafPlus  	| trace.ProofPlus.Path  	|
*/
func pushInsertionRows[K, V io.WriterTo](
	a *assignmentBuilder,
	trace accumulator.InsertionTrace[K, V],
) {

	// row 1
	a.pushRow(
		trace.OldOpenMinus,
		trace.OldSubRoot,
		trace.ProofMinus,
		trace.NewNextFreeNode-1,
		true,  // isFirst
		true,  // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		true,  // isSandwitchEnabled
		true,  // isPointerEnabled
		false, // isEmptyLeaf
	)
	// Sandwitch assignment for row 1
	hKey := hash(trace.Key)

	hKeyFrLimbs := divideFieldBytesToFieldLimbs(hKey.ToBytes(), hashFieldElements)
	hKeySandwitchFrLimbs := divideFieldBytesToFieldLimbs(hKey.ToBytes(), hashByte32Sandwitch)
	hKeyMinusFrLimbs := divideFieldBytesToFieldLimbs(trace.OldOpenMinus.HKey.ToBytes(), hashFieldElements)
	hKeyMinusSandwitchLimbs := divideFieldBytesToFieldLimbs(trace.OldOpenMinus.HKey.ToBytes(), hashByte32Sandwitch)
	hKeyPlusFrLimbs := divideFieldBytesToFieldLimbs(trace.OldOpenPlus.HKey.ToBytes(), hashFieldElements)
	hKeyPlusSandwitchLimbs := divideFieldBytesToFieldLimbs(trace.OldOpenPlus.HKey.ToBytes(), hashByte32Sandwitch)

	for i := range a.hKey {
		a.hKey[i] = append(a.hKey[i], hKeyFrLimbs[i])
		a.hKeyMinus[i] = append(a.hKeyMinus[i], hKeyMinusFrLimbs[i])
		a.hKeyPlus[i] = append(a.hKeyPlus[i], hKeyPlusFrLimbs[i])
	}
	for i := range a.hKeySandwitch {
		a.hKeySandwitch[i] = append(a.hKeySandwitch[i], hKeySandwitchFrLimbs[i])
		a.hKeyMinusSandwitch[i] = append(a.hKeyMinusSandwitch[i], hKeyMinusSandwitchLimbs[i])
		a.hKeyPlusSandwitch[i] = append(a.hKeyPlusSandwitch[i], hKeyPlusSandwitchLimbs[i])
	}

	// Pointer assignment for row 1
	leafMinusNextBytes := uint64To64Bytes(uint64(trace.OldOpenMinus.Next))
	leafMinusNextLimbs := divideFieldBytesToFieldLimbs(leafMinusNextBytes, hashU64)

	leafMinusIndexBytes := uint64To64Bytes(uint64(trace.ProofMinus.Path))
	leafMinusIndexLimbs := divideFieldBytesToFieldLimbs(leafMinusIndexBytes, hashU64)

	leafPlusIndexBytes := uint64To64Bytes(uint64(trace.ProofPlus.Path))
	leafPlusIndexLimbs := divideFieldBytesToFieldLimbs(leafPlusIndexBytes, hashU64)

	leafPlusPrevBytes := uint64To64Bytes(uint64(trace.OldOpenPlus.Prev))
	leafPlusPrevLimbs := divideFieldBytesToFieldLimbs(leafPlusPrevBytes, hashU64)

	for i := range leafMinusNextLimbs {
		a.leafMinusNext[i] = append(a.leafMinusNext[i], leafMinusNextLimbs[i])
		a.leafMinusIndex[i] = append(a.leafMinusIndex[i], leafMinusIndexLimbs[i])
		a.leafPlusIndex[i] = append(a.leafPlusIndex[i], leafPlusIndexLimbs[i])
		a.leafPlusPrev[i] = append(a.leafPlusPrev[i], leafPlusPrevLimbs[i])
		a.leafDeletedIndex[i] = append(a.leafDeletedIndex[i], field.Zero())
		a.leafDeletedPrev[i] = append(a.leafDeletedPrev[i], field.Zero())
		a.leafDeletedNext[i] = append(a.leafDeletedNext[i], field.Zero())
	}

	// row 1 assignment complete
	newLeafOpenMinus := trace.OldOpenMinus
	newLeafOpenMinus.Next = int64(trace.ProofNew.Path)

	var (
		newLeafMinus      = hash(&newLeafOpenMinus)
		intermediateRoot1 = computeRoot(newLeafMinus, trace.ProofMinus)
	)

	// row 2
	a.pushRow(
		newLeafOpenMinus,
		intermediateRoot1,
		trace.ProofMinus,
		trace.NewNextFreeNode-1,
		false, // isFirst
		true,  // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	// row 3
	a.pushRow(
		accumulator.LeafOpening{}, // meaning an empty leaf opening
		intermediateRoot1,
		trace.ProofNew,
		trace.NewNextFreeNode,
		false, // isFirst
		true,  // isInsert
		true,  // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		true,  // isEmptyLeaf
	)

	var (
		insertedLeafOpening = accumulator.LeafOpening{
			Prev: int64(trace.ProofMinus.Path),
			Next: int64(trace.ProofPlus.Path),
			HKey: hash(trace.Key),
			HVal: hash(trace.Val),
		}
		insertedLeaf      = hash(&insertedLeafOpening)
		intermediateRoot3 = computeRoot(insertedLeaf, trace.ProofNew)
	)

	// row 4
	a.pushRow(
		insertedLeafOpening,
		intermediateRoot3,
		trace.ProofNew,
		trace.NewNextFreeNode,
		false, // isFirst
		true,  // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	// row 5
	a.pushRow(
		trace.OldOpenPlus,
		intermediateRoot3,
		trace.ProofPlus,
		trace.NewNextFreeNode,
		false, // isFirst
		true,  // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	newLeafOpenPlus := trace.OldOpenPlus
	newLeafOpenPlus.Prev = int64(trace.ProofNew.Path)

	// row 6
	a.pushRow(
		newLeafOpenPlus,
		trace.NewSubRoot,
		trace.ProofPlus,
		trace.NewNextFreeNode,
		false, // isFirst
		true,  // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

}

/*
We have the below arithmetization for the DELETE operation (the other columns are generated as per definition)
| UseNextProof 	| Proof              	| Root                    	| Leaf         	| Pos                     	|
|--------------	|--------------------	|-------------------------	|--------------	|-------------------------	|
| 1            	| trace.ProofMinus   	| trace.OldSubRoot        	| OldLeafMinus 	| trace.ProofMinus.Path   	|
| 0            	| trace.ProofMinus   	| trace.IntermediateRoot1 	| NewLeafMinus 	| trace.ProofMinus.Path   	|
| 1            	| trace.ProofDeleted 	| trace.IntermediateRoot1 	| deletedLeaf  	| trace.ProofDeleted.Path 	|
| 0            	| trace.ProofDeleted 	| trace.IntermediateRoot3 	| emptyLeaf    	| trace.ProofDeleted.Path 	|
| 1            	| trace.ProofPlus    	| trace.IntermediateRoot3 	| oldLeafPlus  	| trace.ProofPlus.Path    	|
| 0            	| trace.ProofPlus    	| trace.NewSubRoot        	| newLeafPlus  	| trace.ProofPlus.Path    	|
*/
func pushDeletionRows[K, V io.WriterTo](
	a *assignmentBuilder,
	trace accumulator.DeletionTrace[K, V],
) {

	// row 1
	a.pushRow(
		trace.OldOpenMinus,
		trace.OldSubRoot,
		trace.ProofMinus,
		trace.NewNextFreeNode,
		true,  // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		true,  // isPointerEnabled
		false, // isEmptyLeaf
	)
	// Pointer assignment for row 1

	leafMinusNextBytes := uint64To64Bytes(uint64(trace.OldOpenMinus.Next))
	leafMinusNextLimbs := divideFieldBytesToFieldLimbs(leafMinusNextBytes, hashU64)

	leafMinusIndexBytes := uint64To64Bytes(uint64(trace.ProofMinus.Path))
	leafMinusIndexLimbs := divideFieldBytesToFieldLimbs(leafMinusIndexBytes, hashU64)

	leafPlusIndexBytes := uint64To64Bytes(uint64(trace.ProofPlus.Path))
	leafPlusIndexLimbs := divideFieldBytesToFieldLimbs(leafPlusIndexBytes, hashU64)

	leafPlusPrevBytes := uint64To64Bytes(uint64(trace.OldOpenPlus.Prev))
	leafPlusPrevLimbs := divideFieldBytesToFieldLimbs(leafPlusPrevBytes, hashU64)

	leafDeletedIndexBytes := uint64To64Bytes(uint64(trace.ProofDeleted.Path))
	leafDeletedIndexLimbs := divideFieldBytesToFieldLimbs(leafDeletedIndexBytes, hashU64)

	leafDeletedNextBytes := uint64To64Bytes(uint64(trace.DeletedOpen.Next))
	leafDeletedNextLimbs := divideFieldBytesToFieldLimbs(leafDeletedNextBytes, hashU64)

	leafDeletedPrevBytes := uint64To64Bytes(uint64(trace.DeletedOpen.Prev))
	leafDeletedPrevLimbs := divideFieldBytesToFieldLimbs(leafDeletedPrevBytes, hashU64)

	for i := range leafMinusNextLimbs {
		a.leafMinusNext[i] = append(a.leafMinusNext[i], leafMinusNextLimbs[i])
		a.leafMinusIndex[i] = append(a.leafMinusIndex[i], leafMinusIndexLimbs[i])
		a.leafPlusIndex[i] = append(a.leafPlusIndex[i], leafPlusIndexLimbs[i])
		a.leafPlusPrev[i] = append(a.leafPlusPrev[i], leafPlusPrevLimbs[i])
		a.leafDeletedIndex[i] = append(a.leafDeletedIndex[i], leafDeletedIndexLimbs[i])
		a.leafDeletedNext[i] = append(a.leafDeletedNext[i], leafDeletedNextLimbs[i])
		a.leafDeletedPrev[i] = append(a.leafDeletedPrev[i], leafDeletedPrevLimbs[i])
	}

	// row1 assignment complete

	newLeafOpenMinus := trace.OldOpenMinus
	newLeafOpenMinus.Next = int64(trace.ProofPlus.Path)

	var (
		newLeafMinus      = hash(&newLeafOpenMinus)
		intermediateRoot1 = computeRoot(newLeafMinus, trace.ProofMinus)
	)

	// row 2
	a.pushRow(
		newLeafOpenMinus,
		intermediateRoot1,
		trace.ProofMinus,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	// row 3
	a.pushRow(
		trace.DeletedOpen,
		intermediateRoot1,
		trace.ProofDeleted,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	var (
		intermediateRoot3 = computeRoot(types.KoalaOctuplet{}, trace.ProofDeleted)
	)

	// row 4
	a.pushRow(
		accumulator.LeafOpening{},
		intermediateRoot3,
		trace.ProofDeleted,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		true,  // isEmptyLeaf
	)

	// row 5
	a.pushRow(
		trace.OldOpenPlus,
		intermediateRoot3,
		trace.ProofPlus,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	newLeafOpenPlus := trace.OldOpenPlus
	newLeafOpenPlus.Prev = int64(trace.ProofMinus.Path)

	// row 6
	a.pushRow(
		newLeafOpenPlus,
		trace.NewSubRoot,
		trace.ProofPlus,
		trace.NewNextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		true,  // isDelete
		false, // isUpdate
		false, // isReadZero
		false, // isReadNonZero
		false, // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

}

/*
We have the below arithmetization for the READ-ZERO operation (the other columns are generated as per definition)
| UseNextProof 	| Proof            	| Root          	| Leaf      	| Pos                   	|
|--------------	|------------------	|---------------	|-----------	|-----------------------	|
| 0            	| trace.ProofMinus 	| trace.SubRoot 	| LeafMinus 	| trace.ProofMinus.Path 	|
| 0            	| trace.ProofPlus  	| trace.SubRoot 	| LeafPlus  	| trace.ProofPlus.Path  	|
*/
func pushReadZeroRows[K, V io.WriterTo](
	a *assignmentBuilder,
	trace accumulator.ReadZeroTrace[K, V],
) {

	// row 1
	a.pushRow(
		trace.OpeningMinus,
		trace.SubRoot,
		trace.ProofMinus,
		trace.NextFreeNode,
		true,  // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		true,  // isReadZero
		false, // isReadNonZero
		true,  // disableReuseMerkle
		true,  // isSandwitchEnabled
		true,  // isPointerEnabled
		false, // isEmptyLeaf
	)

	// Sandwitch assignment for row 1
	hKey := hash(trace.Key)
	hKeyFrLimbs := divideFieldBytesToFieldLimbs(hKey.ToBytes(), hashFieldElements)
	hKeySandwitchFrLimbs := divideFieldBytesToFieldLimbs(hKey.ToBytes(), hashByte32Sandwitch)
	hKeyMinusFrLimbs := divideFieldBytesToFieldLimbs(trace.OpeningMinus.HKey.ToBytes(), hashFieldElements)
	hKeyMinusSandwitchFrLimbs := divideFieldBytesToFieldLimbs(trace.OpeningMinus.HKey.ToBytes(), hashByte32Sandwitch)
	hKeyPlusFrLimbs := divideFieldBytesToFieldLimbs(trace.OpeningPlus.HKey.ToBytes(), hashFieldElements)
	hKeyPlusSandwitchFrLimbs := divideFieldBytesToFieldLimbs(trace.OpeningPlus.HKey.ToBytes(), hashByte32Sandwitch)
	for i := range a.hKey {
		a.hKey[i] = append(a.hKey[i], hKeyFrLimbs[i])
		a.hKeyMinus[i] = append(a.hKeyMinus[i], hKeyMinusFrLimbs[i])
		a.hKeyPlus[i] = append(a.hKeyPlus[i], hKeyPlusFrLimbs[i])
	}
	for i := range a.hKeySandwitch {
		a.hKeySandwitch[i] = append(a.hKeySandwitch[i], hKeySandwitchFrLimbs[i])
		a.hKeyMinusSandwitch[i] = append(a.hKeyMinusSandwitch[i], hKeyMinusSandwitchFrLimbs[i])
		a.hKeyPlusSandwitch[i] = append(a.hKeyPlusSandwitch[i], hKeyPlusSandwitchFrLimbs[i])
	}

	// Pointer assignment for row1

	leafMinusNextBytes := uint64To64Bytes(uint64(trace.OpeningMinus.Next))
	leafMinusNextLimbs := divideFieldBytesToFieldLimbs(leafMinusNextBytes, hashU64)

	leafMinusIndexBytes := uint64To64Bytes(uint64(trace.ProofMinus.Path))
	leafMinusIndexLimbs := divideFieldBytesToFieldLimbs(leafMinusIndexBytes, hashU64)

	leafPlusIndexBytes := uint64To64Bytes(uint64(trace.ProofPlus.Path))
	leafPlusIndexLimbs := divideFieldBytesToFieldLimbs(leafPlusIndexBytes, hashU64)

	leafPlusPrevBytes := uint64To64Bytes(uint64(trace.OpeningPlus.Prev))
	leafPlusPrevLimbs := divideFieldBytesToFieldLimbs(leafPlusPrevBytes, hashU64)

	for i := range leafMinusNextLimbs {
		a.leafMinusNext[i] = append(a.leafMinusNext[i], leafMinusNextLimbs[i])
		a.leafMinusIndex[i] = append(a.leafMinusIndex[i], leafMinusIndexLimbs[i])
		a.leafPlusIndex[i] = append(a.leafPlusIndex[i], leafPlusIndexLimbs[i])
		a.leafPlusPrev[i] = append(a.leafPlusPrev[i], leafPlusPrevLimbs[i])
		a.leafDeletedIndex[i] = append(a.leafDeletedIndex[i], field.Zero())
		a.leafDeletedPrev[i] = append(a.leafDeletedPrev[i], field.Zero())
		a.leafDeletedNext[i] = append(a.leafDeletedNext[i], field.Zero())
	}
	// row 1 assignment complete

	// row 2
	a.pushRow(
		trace.OpeningPlus,
		trace.SubRoot,
		trace.ProofPlus,
		trace.NextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		true,  // isReadZero
		false, // isReadNonZero
		true,  // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)
}

/*
We have the below arithmetization for the READ-NONZERO operation (the other columns are generated as per definition)
| UseNextProof 	| Proof       	| Root          	| Leaf 	| Pos              	|
|--------------	|-------------	|---------------	|------	|------------------	|
| 0            	| trace.Proof 	| trace.SubRoot 	| Leaf 	| trace.Proof.Path 	|
| 0            	| trace.Proof 	| trace.SubRoot 	| Leaf 	| trace.Proof.Path 	|
(repetition of row 1 to fecilitate verifying reuse of Merkle proofs for the other operations, we need even number of rows for each operation for the current technique of verifying reuse of Merkle proofs) todo (@arijit): think about avoiding this: (Idea1, to defer assigning for readNonZero traces, collect them in an array, assign them at the last, atmost one repetition at the last for odd number of traces, drawback: changes the order of the executions, might create problems, issue to be created if there is performance issue because of this.
*/
func pushReadNonZeroRows[K, V io.WriterTo](
	a *assignmentBuilder,
	trace accumulator.ReadNonZeroTrace[K, V],
) {

	// row 1
	a.pushRow(
		trace.LeafOpening,
		trace.SubRoot,
		trace.Proof,
		trace.NextFreeNode,
		true,  // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		true,  // isReadNonZero
		true,  // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)

	// row 2
	a.pushRow(
		trace.LeafOpening,
		trace.SubRoot,
		trace.Proof,
		trace.NextFreeNode,
		false, // isFirst
		false, // isInsert
		false, // isInsertRow3
		false, // isDelete
		false, // isUpdate
		false, // isReadZero
		true,  // isReadNonZero
		true,  // disableReuseMerkle
		false, // isSandwitchEnabled
		false, // isPointerEnabled
		false, // isEmptyLeaf
	)
}

// Generic hashing for object satisfying the io.WriterTo interface
func hash[T io.WriterTo](m T) types.KoalaOctuplet {
	hasher := poseidon2.NewMDHasher()
	m.WriteTo(hasher)
	digest := hasher.Sum(nil)
	return types.MustBytesToKoalaOctuplet(digest)
}

// Function to compute the root of a Merkle tree given proof and the leaf
func computeRoot(leaf types.KoalaOctuplet, proof smt_koalabear.Proof) types.KoalaOctuplet {
	current := leaf
	idx := proof.Path

	for _, sibling := range proof.Siblings {
		left, right := current, sibling
		if idx&1 == 1 {
			left, right = right, left
		}
		current = hashLR(left, right)
		idx >>= 1
	}

	// Sanity-check: the idx should be zero
	if idx != 0 {
		panic("idx should be zero")
	}

	return current
}

// Function to compute the hash given the left and the right node
func hashLR(nodeL, nodeR types.KoalaOctuplet) types.KoalaOctuplet {
	hasher := poseidon2.NewMDHasher()
	nodeL.WriteTo(hasher)
	nodeR.WriteTo(hasher)
	d := types.MustBytesToKoalaOctuplet(hasher.Sum(nil))
	return d
}

// divideFieldBytesToFieldLimbs divides a byte slice representing a field element into
// a slice of `field.Element`s, where each `field.Element` represents a "limb"
// of the original field element. This function assumes that each limb is
// 4 bytes long before being set as a `field.Element` in canonical form.
func divideFieldBytesToFieldLimbs(elementBytes []byte, toHash hashType) []field.Element {
	// Sanity check on input length
	switch toHash {
	case hashU64:
		if len(elementBytes) != common.NbBytesForEncodingU64 {
			panic("elementBytes length for u64 is not 64")
		}
	case hashFieldElements:
		if len(elementBytes) != common.NbBytesForEncodingFieldHash {
			panic("elementBytes length for field hash is not 32")
		}
	case hashByte32Sandwitch:
		if len(elementBytes) != common.NbBytesForEncodingFieldHash {
			panic("elementBytes length for field hash is not correct")
		}
	default:
		panic("unsupported hashType")
	}
	// declare the result slice of field elements
	var res []field.Element
	switch toHash {
	case hashU64:
		res = make([]field.Element, 0, common.NbElemForHasingU64)
	case hashFieldElements:
		res = make([]field.Element, 0, common.NbElemPerHash)
	case hashByte32Sandwitch:
		res = make([]field.Element, 0, common.NbElemForHashingByte32Sandwitch)
	default:
		panic("unsupported hashType")
	}
	// assign res
	switch toHash {
	case hashU64, hashFieldElements:
		// each field element is a chunk of 4 bytes
		for _, limbBytes := range common.SplitBytes(elementBytes, 4) {
			elementFr, _ := byteToField(limbBytes[:])
			res = append(res, elementFr)
		}
	case hashByte32Sandwitch:
		// each field element is a chunk of 2 bytes
		for _, limbBytes := range common.SplitBytes(elementBytes) {
			elementFr, _ := byteToField(limbBytes[:])
			res = append(res, elementFr)
		}
	default:
		panic("unsupported hashType")
	}
	// Sanity check on output length
	switch toHash {
	case hashU64:
		if len(res) != common.NbElemForHasingU64 {
			panic("res length is not equal to NbElemForHasingU64")
		}
	case hashFieldElements:
		if len(res) != common.NbElemPerHash {
			panic("res length is not equal to NbElemPerHash")
		}
	case hashByte32Sandwitch:
		if len(res) != common.NbElemForHashingByte32Sandwitch {
			panic("res length is not equal to NbElemForHashingByte32Sandwitch")
		}
	default:
		panic("unsupported hashType")
	}

	return res
}

// byteToField converts a byte slice into a field.Element assuming Big-Endian
// byte order. The input byte slice can be of length less than or equal to
// field.Bytes. If the length is less than field.Bytes, it is left-padded with
// zeros.
func byteToField(b []byte) (field.Element, error) {
	buf := make([]byte, field.Bytes)
	copy(buf[field.Bytes-len(b):], b)
	var x field.Element
	if err := x.SetBytesCanonical(buf); err != nil {
		return x, fmt.Errorf("could not create field element from bytes (%x)", b)
	}
	return x, nil
}

// uint64To64Bytes converts a `uint64` number into an 64-byte slice assuming
// Big-Endian byte order.
func uint64To64Bytes(num uint64) []byte {
	bytes := make([]byte, 8)
	binary.BigEndian.PutUint64(bytes, num)
	// We copy every 2 bytes from tmp into res, left padded by 2 zero-bytes.
	res := types.LeftPadded48Zeros(types.LeftPadded(bytes[:]))
	return res
}
