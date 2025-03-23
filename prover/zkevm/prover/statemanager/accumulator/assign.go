package accumulator

import (
	"io"

	"github.com/consensys/linea-monorepo/prover/backend/execution/statemanager"
	"github.com/consensys/linea-monorepo/prover/crypto/mimc"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt"
	"github.com/consensys/linea-monorepo/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/types"
)

// leafOpenings represents the structure for leaf openings
type leafOpenings struct {
	prev []field.Element
	next []field.Element
	hKey []field.Element
	hVal []field.Element
}

// assignmentBuilder is used to build the assignment of the [Module] module
// and is implemented like a writer.
type assignmentBuilder struct {

	// Setting contains the setting with which the corresponding accumulator
	// module was instantiated.
	Settings

	// leaves stores the assignment of the column holding the leaves (so the
	// hash of the leaf openings) for which we give the merkle proof. This corresponds to the
	// [Accumulator.Cols.Leaves] column.
	leaves []field.Element
	// positions stores the positions of the leaves in the merkle tree for which we give the Merkle proof. This corresponds to the [Accumulator.Cols.Positions] column.
	positions []field.Element
	// roots stores the roots of the merkle tree. This corresponds
	// to the [Accumulator.Cols.Roots] column.
	roots []field.Element
	// proofs stores the path and siblings of the merkle proof. Those siblings corresponds
	// to the [Accumulator.Cols.Proofs] column.
	proofs []smt.Proof
	// useNextMerkleProof is assigned starting from 1, then 0, followed by 0 and so on in each row of INSERT,
	// UPDATE and DELETE. For READZERO and READNONZERO it is set to 0 for each row
	useNextMerkleProof []field.Element
	// isActive is assigned to 1 for each row of every operation. This corresponds to the [Accumulator.Cols.IsActiveAccumulator] column
	isActive []field.Element
	// accumulatorCounter counts the number of rows in the accumulator. It is used to check the
	// sequentiality of leaves and roots in accumulator and the merkle module
	accumulatorCounter []field.Element
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
	hKey []field.Element
	// hKeyMinus is the hash of the key of the previous leaf. This corresponds to the [Accumulator.Column.HKeyMinus]
	hKeyMinus []field.Element
	// hKeyPlus is the hash of the key of the next leaf. This corresponds to the [Accumulator.Column.HKeyPlus]
	hKeyPlus []field.Element
	// Pointer check columns
	// leafMinusIndex is the index of the minus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafMinusIndex]
	leafMinusIndex []field.Element
	// leafMinusNext is the index of the Next leaf of the minus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafMinusNext]
	leafMinusNext []field.Element
	// leafMinusNext is the index of the plus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafPlusIndex]
	leafPlusIndex []field.Element
	// leafPlusPrev is the index of the Previous leaf of the plus leaf for INSERT, READZERO, and DELETE. This corresponds to the [Accumulator.Column.LeafPlusPrev]
	leafPlusPrev []field.Element
	// leafDeletedIndex is the index of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedIndex]
	leafDeletedIndex []field.Element
	// leafDeletedPrev is the index of the Previous leaf of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedPrev]
	leafDeletedPrev []field.Element
	// leafDeletedNext is the index of the Previous leaf of the Deleted leaf for DELETE. This corresponds to the [Accumulator.Column.LeafDeletedNext]
	leafDeletedNext []field.Element
	// leafOpening is a tuple of four columns containing
	// Prev, Next, HKey, HVal of a leaf. This corresponds to the [Accumulator.Column.LeafOpening]
	leafOpening leafOpenings
	// interm is a slice containing 3 intermediate hash states. This corresponds to the [Accumulator.Column.Interm]
	interm [][]field.Element
	// leafHash contains sequential MiMC hashes of leafOpening. It matches with Leaves except when there is empty leaf. This corresponds to the [Accumulator.Column.LeafHashes]
	leafHashes []field.Element
	// isEmptyLeaf is one when Leaves contains empty leaf and does not match with LeafHash
	isEmptyLeaf []field.Element
	// nextFreeNode contains the nextFreeNode for each row of every operation
	nextFreeNode []field.Element
	// insertionPath is the path of a newly inserted leaf when INSERT happens,
	// it is zero otherwise
	insertionPath []field.Element
	// isInsertRow3 is one for row 3 of INSERT operation
	isInsertRow3 []field.Element
	// intermTopRoot contains the intermediate MiMC state hash
	intermTopRoot []field.Element
	// topRoot contains the MiMC hash of SubTreeRoot and NextFreeNode
	topRoot []field.Element
}

// newAssignmentBuilder returns an empty builder
func newAssignmentBuilder(s Settings) *assignmentBuilder {
	amb := assignmentBuilder{}
	amb.Settings = s
	amb.leaves = make([]field.Element, 0, amb.NumRows())
	amb.positions = make([]field.Element, 0, amb.NumRows())
	amb.roots = make([]field.Element, 0, amb.NumRows())
	amb.proofs = make([]smt.Proof, 0, amb.NumRows())
	amb.useNextMerkleProof = make([]field.Element, 0, amb.NumRows())
	amb.isActive = make([]field.Element, 0, amb.NumRows())
	amb.accumulatorCounter = make([]field.Element, 0, amb.NumRows())
	amb.isFirst = make([]field.Element, 0, amb.NumRows())
	amb.isInsert = make([]field.Element, 0, amb.NumRows())
	amb.isDelete = make([]field.Element, 0, amb.NumRows())
	amb.isUpdate = make([]field.Element, 0, amb.NumRows())
	amb.isReadZero = make([]field.Element, 0, amb.NumRows())
	amb.isReadNonZero = make([]field.Element, 0, amb.NumRows())
	amb.hKey = make([]field.Element, 0, amb.NumRows())
	amb.hKeyMinus = make([]field.Element, 0, amb.NumRows())
	amb.hKeyPlus = make([]field.Element, 0, amb.NumRows())
	amb.leafMinusIndex = make([]field.Element, 0, amb.NumRows())
	amb.leafMinusNext = make([]field.Element, 0, amb.NumRows())
	amb.leafPlusIndex = make([]field.Element, 0, amb.NumRows())
	amb.leafPlusPrev = make([]field.Element, 0, amb.NumRows())
	amb.leafDeletedIndex = make([]field.Element, 0, amb.NumRows())
	amb.leafDeletedPrev = make([]field.Element, 0, amb.NumRows())
	amb.leafDeletedNext = make([]field.Element, 0, amb.NumRows())
	amb.leafOpening.prev = make([]field.Element, 0, amb.NumRows())
	amb.leafOpening.next = make([]field.Element, 0, amb.NumRows())
	amb.leafOpening.hKey = make([]field.Element, 0, amb.NumRows())
	amb.leafOpening.hVal = make([]field.Element, 0, amb.NumRows())
	amb.interm = make([][]field.Element, 3)
	for i := 0; i < len(amb.interm); i++ {
		amb.interm[i] = make([]field.Element, 0, amb.NumRows())
	}
	amb.leafHashes = make([]field.Element, 0, amb.NumRows())
	amb.isEmptyLeaf = make([]field.Element, 0, amb.NumRows())
	amb.nextFreeNode = make([]field.Element, 0, amb.NumRows())
	amb.insertionPath = make([]field.Element, 0, amb.NumRows())
	amb.isInsertRow3 = make([]field.Element, 0, amb.NumRows())
	amb.intermTopRoot = make([]field.Element, 0, amb.NumRows())
	amb.topRoot = make([]field.Element, 0, amb.NumRows())

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
		builder         = newAssignmentBuilder(am.Settings)
		paddedSize      = am.NumRows()
		proofPaddedSize = am.merkleProofModNumRows()
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
	if len(builder.leaves) > am.MaxNumProofs {
		utils.Panic("We have registered %v proofs which is more than the maximum number of proofs %v", len(builder.leaves), am.MaxNumProofs)
	}

	// Assignments of columns
	var (
		proofs      = merkle.PackMerkleProofs(builder.proofs)
		proofsReg   = smartvectors.IntoRegVec(proofs)
		proofPadded = smartvectors.RightZeroPadded(proofsReg, proofPaddedSize)
		cols        = am.Cols
	)

	run.AssignColumn(cols.Proofs.GetColID(), proofPadded)
	run.AssignColumn(cols.Roots.GetColID(), smartvectors.RightZeroPadded(builder.roots, paddedSize))
	run.AssignColumn(cols.Positions.GetColID(), smartvectors.RightZeroPadded(builder.positions, paddedSize))
	run.AssignColumn(cols.Leaves.GetColID(), smartvectors.RightZeroPadded(builder.leaves, paddedSize))
	run.AssignColumn(cols.UseNextMerkleProof.GetColID(), smartvectors.RightZeroPadded(builder.useNextMerkleProof, paddedSize))
	run.AssignColumn(cols.IsActiveAccumulator.GetColID(), smartvectors.RightZeroPadded(builder.isActive, paddedSize))
	run.AssignColumn(cols.AccumulatorCounter.GetColID(), smartvectors.RightZeroPadded(builder.accumulatorCounter, paddedSize))
	run.AssignColumn(cols.IsFirst.GetColID(), smartvectors.RightZeroPadded(builder.isFirst, paddedSize))
	run.AssignColumn(cols.IsInsert.GetColID(), smartvectors.RightZeroPadded(builder.isInsert, paddedSize))
	run.AssignColumn(cols.IsDelete.GetColID(), smartvectors.RightZeroPadded(builder.isDelete, paddedSize))
	run.AssignColumn(cols.IsUpdate.GetColID(), smartvectors.RightZeroPadded(builder.isUpdate, paddedSize))
	run.AssignColumn(cols.IsReadZero.GetColID(), smartvectors.RightZeroPadded(builder.isReadZero, paddedSize))
	run.AssignColumn(cols.IsReadNonZero.GetColID(), smartvectors.RightZeroPadded(builder.isReadNonZero, paddedSize))

	// assignments for the sandwitch check columns
	run.AssignColumn(cols.HKey.GetColID(), smartvectors.RightZeroPadded(builder.hKey, paddedSize))
	run.AssignColumn(cols.HKeyMinus.GetColID(), smartvectors.RightZeroPadded(builder.hKeyMinus, paddedSize))
	run.AssignColumn(cols.HKeyPlus.GetColID(), smartvectors.RightZeroPadded(builder.hKeyPlus, paddedSize))

	// assignments for the pointer check columns
	run.AssignColumn(cols.LeafMinusIndex.GetColID(), smartvectors.RightZeroPadded(builder.leafMinusIndex, paddedSize))
	run.AssignColumn(cols.LeafMinusNext.GetColID(), smartvectors.RightZeroPadded(builder.leafMinusNext, paddedSize))
	run.AssignColumn(cols.LeafPlusIndex.GetColID(), smartvectors.RightZeroPadded(builder.leafPlusIndex, paddedSize))
	run.AssignColumn(cols.LeafPlusPrev.GetColID(), smartvectors.RightZeroPadded(builder.leafPlusPrev, paddedSize))
	run.AssignColumn(cols.LeafDeletedIndex.GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedIndex, paddedSize))
	run.AssignColumn(cols.LeafDeletedPrev.GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedPrev, paddedSize))
	run.AssignColumn(cols.LeafDeletedNext.GetColID(), smartvectors.RightZeroPadded(builder.leafDeletedNext, paddedSize))

	// Assign Interm, LeafOpenings, and LeafHashes columns
	am.assignLeaf(run, builder)

	// Assignment for NextFreeNode checking columns
	run.AssignColumn(cols.NextFreeNode.GetColID(), smartvectors.RightZeroPadded(builder.nextFreeNode, paddedSize))
	run.AssignColumn(cols.InsertionPath.GetColID(), smartvectors.RightZeroPadded(builder.insertionPath, paddedSize))
	run.AssignColumn(cols.IsInsertRow3.GetColID(), smartvectors.RightZeroPadded(builder.isInsertRow3, paddedSize))

	// Assign TopRoot hash checking columns
	am.assignTopRootCols(run, builder)
}

func (am *Module) assignLeaf(
	run *wizard.ProverRuntime,
	builder *assignmentBuilder) {

	var (
		cols       = am.Cols
		paddedSize = am.NumRows()
		intermZero = mimc.BlockCompression(field.Zero(), field.Zero())
		intermOne  = mimc.BlockCompression(intermZero, field.Zero())
		intermTwo  = mimc.BlockCompression(intermOne, field.Zero())
		leaf       = mimc.BlockCompression(intermTwo, field.Zero())
	)

	run.AssignColumn(cols.IsEmptyLeaf.GetColID(), smartvectors.RightZeroPadded(builder.isEmptyLeaf, paddedSize))
	run.AssignColumn(cols.Interm[0].GetColID(), smartvectors.RightPadded(builder.interm[0], intermZero, paddedSize))
	run.AssignColumn(cols.Interm[1].GetColID(), smartvectors.RightPadded(builder.interm[1], intermOne, paddedSize))
	run.AssignColumn(cols.Interm[2].GetColID(), smartvectors.RightPadded(builder.interm[2], intermTwo, paddedSize))

	run.AssignColumn(cols.LeafOpenings.Prev.GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.prev, paddedSize))
	run.AssignColumn(cols.LeafOpenings.Next.GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.next, paddedSize))
	run.AssignColumn(cols.LeafOpenings.HKey.GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.hKey, paddedSize))
	run.AssignColumn(cols.LeafOpenings.HVal.GetColID(), smartvectors.RightZeroPadded(builder.leafOpening.hVal, paddedSize))

	run.AssignColumn(cols.LeafHashes.GetColID(), smartvectors.RightPadded(builder.leafHashes, leaf, paddedSize))
}

func (am *Module) assignTopRootCols(
	run *wizard.ProverRuntime,
	builder *assignmentBuilder) {
	cols := am.Cols
	paddedSize := am.NumRows()

	// compute the padding values for intermTopRoot and topRoot
	intermTopRootPad := mimc.BlockCompression(field.Zero(), field.Zero())
	topRootPad := mimc.BlockCompression(intermTopRootPad, field.Zero())

	run.AssignColumn(cols.IntermTopRoot.GetColID(), smartvectors.RightPadded(builder.intermTopRoot, intermTopRootPad, paddedSize))
	run.AssignColumn(cols.TopRoot.GetColID(), smartvectors.RightPadded(builder.topRoot, topRootPad, paddedSize))
}

// This is a low level function used by all the operations (INSERT, UPDATE, DELETE, READ-ZERO, and READ-NONZERO)
// to update each row of various columns
func (a *assignmentBuilder) pushRow(
	leafOpening accumulator.LeafOpening,
	root types.Bytes32,
	proof smt.Proof,
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
	// Populates leaves, leafHashes, and isEmptyLeaf from leafOpening by MiMc block compression
	a.computeLeaf(leafOpening, isEmptyLeaf)
	var rootFr field.Element
	if err := rootFr.SetBytesCanonical(root[:]); err != nil {
		panic(err)
	}
	a.positions = append(a.positions, field.NewElement(uint64(proof.Path)))
	a.roots = append(a.roots, rootFr)
	a.proofs = append(a.proofs, proof)
	nextFreeNodeFr := field.NewElement(uint64(nextFreeNode))
	a.nextFreeNode = append(a.nextFreeNode, nextFreeNodeFr)

	// We assign intermTopRoot = MiMC(zero, root), and topRoot = MiMC(interm, nextFreeNode)
	intermTopRootFr := mimc.BlockCompression(field.Zero(), nextFreeNodeFr)
	topRootFr := mimc.BlockCompression(intermTopRootFr, rootFr)
	a.intermTopRoot = append(a.intermTopRoot, intermTopRootFr)
	a.topRoot = append(a.topRoot, topRootFr)

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
	// Insert Row 3 operations
	isInsRow3 := field.Zero()
	insPath := field.Zero()
	if isInsertRow3 {
		isInsRow3 = field.One()
		insPath = field.NewElement(uint64(proof.Path))
	}
	a.isInsertRow3 = append(a.isInsertRow3, isInsRow3)
	a.insertionPath = append(a.insertionPath, insPath)
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

	// accumulatorCounter will increment when a new row is pushed
	a.accumulatorCounter = append(a.accumulatorCounter, field.NewElement(uint64(len(a.accumulatorCounter))))

	// if Sandwitch check is disabled then we append zero values to
	// hKey, hKeyPlus, hKeyMinus
	if !isSandwitchEnabled {
		a.hKey = append(a.hKey, field.Zero())
		a.hKeyMinus = append(a.hKeyMinus, field.Zero())
		a.hKeyPlus = append(a.hKeyPlus, field.Zero())
	}

	// if Pointer check is disabled then we append zero values to
	// leafMinusIndex, leafMinusNext, leafPlusIndex, leafPlusPrev,
	// leafDeletedIndex, leafDeletedPrev, leafDeletedNext
	if !isPointerEnabled {
		a.leafMinusIndex = append(a.leafMinusIndex, field.Zero())
		a.leafMinusNext = append(a.leafMinusNext, field.Zero())
		a.leafPlusIndex = append(a.leafPlusIndex, field.Zero())
		a.leafPlusPrev = append(a.leafPlusPrev, field.Zero())
		a.leafDeletedIndex = append(a.leafDeletedIndex, field.Zero())
		a.leafDeletedPrev = append(a.leafDeletedPrev, field.Zero())
		a.leafDeletedNext = append(a.leafDeletedNext, field.Zero())
	}
}

func (a *assignmentBuilder) computeLeaf(leafOpening accumulator.LeafOpening, isEmptyLeaf bool) {
	if !isEmptyLeaf {
		prevFr := field.NewElement(uint64(leafOpening.Prev))
		nextFr := field.NewElement(uint64(leafOpening.Next))
		var hKeyFr, hValFr field.Element
		if err := hKeyFr.SetBytesCanonical(leafOpening.HKey[:]); err != nil {
			panic(err)
		}
		if err := hValFr.SetBytesCanonical(leafOpening.HVal[:]); err != nil {
			panic(err)
		}
		intermZero := mimc.BlockCompression(field.Zero(), prevFr)
		intermOne := mimc.BlockCompression(intermZero, nextFr)
		intermTwo := mimc.BlockCompression(intermOne, hKeyFr)
		leaf := mimc.BlockCompression(intermTwo, hValFr)
		a.leafOpening.prev = append(a.leafOpening.prev, prevFr)
		a.leafOpening.next = append(a.leafOpening.next, nextFr)
		a.leafOpening.hKey = append(a.leafOpening.hKey, hKeyFr)
		a.leafOpening.hVal = append(a.leafOpening.hVal, hValFr)
		a.interm[0] = append(a.interm[0], intermZero)
		a.interm[1] = append(a.interm[1], intermOne)
		a.interm[2] = append(a.interm[2], intermTwo)
		a.leafHashes = append(a.leafHashes, leaf)
		a.leaves = append(a.leaves, leaf)
		isEmpty := field.Zero()
		a.isEmptyLeaf = append(a.isEmptyLeaf, isEmpty)
	} else {
		intermZero := mimc.BlockCompression(field.Zero(), field.Zero())
		intermOne := mimc.BlockCompression(intermZero, field.Zero())
		intermTwo := mimc.BlockCompression(intermOne, field.Zero())
		leaf := mimc.BlockCompression(intermTwo, field.Zero())
		a.leafOpening.prev = append(a.leafOpening.prev, field.Zero())
		a.leafOpening.next = append(a.leafOpening.next, field.Zero())
		a.leafOpening.hKey = append(a.leafOpening.hKey, field.Zero())
		a.leafOpening.hVal = append(a.leafOpening.hVal, field.Zero())
		a.interm[0] = append(a.interm[0], intermZero)
		a.interm[1] = append(a.interm[1], intermOne)
		a.interm[2] = append(a.interm[2], intermTwo)
		a.leafHashes = append(a.leafHashes, leaf)
		// We insert an empty leaf in the Leaves column in this case
		emptyLeafBytes32 := types.Bytes32{}
		emptyLeafBytes := emptyLeafBytes32[:]
		var emptyLeafFr field.Element
		if err := emptyLeafFr.SetBytesCanonical(emptyLeafBytes[:]); err != nil {
			panic(err)
		}
		a.leaves = append(a.leaves, emptyLeafFr)
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
	var hKeyFr, hKeyMinusFr, hKeyPlusFr field.Element
	hKey := hash(trace.Key)
	if err := hKeyFr.SetBytesCanonical(hKey[:]); err != nil {
		panic(err)
	}
	if err := hKeyMinusFr.SetBytesCanonical(trace.OldOpenMinus.HKey[:]); err != nil {
		panic(err)
	}
	if err := hKeyPlusFr.SetBytesCanonical(trace.OldOpenPlus.HKey[:]); err != nil {
		panic(err)
	}
	a.hKey = append(a.hKey, hKeyFr)
	a.hKeyMinus = append(a.hKeyMinus, hKeyMinusFr)
	a.hKeyPlus = append(a.hKeyPlus, hKeyPlusFr)

	// Pointer assignment for row 1
	a.leafMinusNext = append(a.leafMinusNext, field.NewElement(uint64(trace.OldOpenMinus.Next)))
	a.leafMinusIndex = append(a.leafMinusIndex, field.NewElement(uint64(trace.ProofMinus.Path)))
	a.leafPlusIndex = append(a.leafPlusIndex, field.NewElement(uint64(trace.ProofPlus.Path)))
	a.leafPlusPrev = append(a.leafPlusPrev, field.NewElement(uint64(trace.OldOpenPlus.Prev)))
	a.leafDeletedIndex = append(a.leafDeletedIndex, field.Zero())
	a.leafDeletedPrev = append(a.leafDeletedPrev, field.Zero())
	a.leafDeletedNext = append(a.leafDeletedNext, field.Zero())
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
	a.leafMinusNext = append(a.leafMinusNext, field.NewElement(uint64(trace.OldOpenMinus.Next)))
	a.leafMinusIndex = append(a.leafMinusIndex, field.NewElement(uint64(trace.ProofMinus.Path)))
	a.leafPlusIndex = append(a.leafPlusIndex, field.NewElement(uint64(trace.ProofPlus.Path)))
	a.leafPlusPrev = append(a.leafPlusPrev, field.NewElement(uint64(trace.OldOpenPlus.Prev)))
	a.leafDeletedIndex = append(a.leafDeletedIndex, field.NewElement(uint64(trace.ProofDeleted.Path)))
	a.leafDeletedNext = append(a.leafDeletedNext, field.NewElement(uint64(trace.DeletedOpen.Next)))
	a.leafDeletedPrev = append(a.leafDeletedPrev, field.NewElement(uint64(trace.DeletedOpen.Prev)))
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
		intermediateRoot3 = computeRoot(types.Bytes32{}, trace.ProofDeleted)
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
	var hKeyFr, hKeyMinusFr, hKeyPlusFr field.Element
	hKey := hash(trace.Key)
	if err := hKeyFr.SetBytesCanonical(hKey[:]); err != nil {
		panic(err)
	}
	if err := hKeyMinusFr.SetBytesCanonical(trace.OpeningMinus.HKey[:]); err != nil {
		panic(err)
	}
	if err := hKeyPlusFr.SetBytesCanonical(trace.OpeningPlus.HKey[:]); err != nil {
		panic(err)
	}
	a.hKey = append(a.hKey, hKeyFr)
	a.hKeyMinus = append(a.hKeyMinus, hKeyMinusFr)
	a.hKeyPlus = append(a.hKeyPlus, hKeyPlusFr)
	// Pointer assignment for row1
	a.leafMinusNext = append(a.leafMinusNext, field.NewElement(uint64(trace.OpeningMinus.Next)))
	a.leafMinusIndex = append(a.leafMinusIndex, field.NewElement(uint64(trace.ProofMinus.Path)))
	a.leafPlusIndex = append(a.leafPlusIndex, field.NewElement(uint64(trace.ProofPlus.Path)))
	a.leafPlusPrev = append(a.leafPlusPrev, field.NewElement(uint64(trace.OpeningPlus.Prev)))
	a.leafDeletedIndex = append(a.leafDeletedIndex, field.Zero())
	a.leafDeletedPrev = append(a.leafDeletedPrev, field.Zero())
	a.leafDeletedNext = append(a.leafDeletedNext, field.Zero())
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
func hash[T io.WriterTo](m T) types.Bytes32 {
	hasher := statemanager.MIMC_CONFIG.HashFunc()
	m.WriteTo(hasher)
	Bytes32 := hasher.Sum(nil)
	return types.AsBytes32(Bytes32)
}

// Function to compute the root of a Merkle tree given proof and the leaf
func computeRoot(leaf types.Bytes32, proof smt.Proof) types.Bytes32 {
	current := leaf
	idx := proof.Path

	for _, sibling := range proof.Siblings {
		left, right := current, sibling
		if idx&1 == 1 {
			left, right = right, left
		}
		current = hashLR(statemanager.MIMC_CONFIG, left, right)
		idx >>= 1
	}

	// Sanity-check: the idx should be zero
	if idx != 0 {
		panic("idx should be zero")
	}

	return current
}

// Function to compute the hash given the left and the right node
func hashLR(conf *smt.Config, nodeL, nodeR types.Bytes32) types.Bytes32 {
	hasher := conf.HashFunc()
	nodeL.WriteTo(hasher)
	nodeR.WriteTo(hasher)
	d := types.AsBytes32(hasher.Sum(nil))
	return d
}
