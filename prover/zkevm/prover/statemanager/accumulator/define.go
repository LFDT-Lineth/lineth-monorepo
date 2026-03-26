package accumulator

import (
	"fmt"
	"strconv"

	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

/*
Below, we give a high level overview of the Accumulator module. The inputs of the
Accumulator module are the Shomei traces. Shomei is the state manager of Linea zkEVM.
It stores the various states of the account and storage trie in many sparse Merkle trees. As described in
https://docs.google.com/document/d/12oRcoDDql-2FpmRjcrBa8n7c_X41bZFO5leBzMcehlQ/edit#heading=h.9ssjg7avtkms,
Shomei generates traces for five operations on the Merkle trees, namely INSERT, UPDATE, DELETE, READ-ZERO,
and READ-NON-ZERO. Each of these operations can be either for the world state (a unique merkle tree) or
for the storage trie (many merkle trees). For a flowchart of the verification steps for each operation,
please refer to https://app.diagrams.net/?mode=google&gfw=1#G19bPkZosp4UQE-aLsA8p6g5skvUU-63SQ#%7B%22pageId%22%3A%22azjwsvsbn_RYYOIuM5ui%22%7D.

The main verification steps of the Accumulator module is as follows,

1. First and foremost is to verify each of the above operations from the traces. For example, an UPDATE operation
consists of two merkle proofs on the same tree: one with the old leaf and another with the new leaf. Therefore,
to verify an UPDATE, we need to separately verify these two merkle proofs. We also need to verify the fact that
they are for the same Merkle tree. We call it verifying the "reuse of Merkle proof". These verifications are delegated
to the Merkle module and the technique for this is described in merkle/merkleproof.md.

As a result, an UPDATE induces two rows in the Accumulator module. Also, INSERT and DELETE have three consicutive
UPDATEs at leafMinusPosition, insertPosition/deletePosition, and leafPlusPosition. Hence, they have six rows. The row
assignment for all the columns are done in accumulator/assign.go and the row structure for each operations is
described there.

2. Because of the above mentioned row structure, we have certain constraints. For example,
for INSERT and DELETE, we have equal roots in the second and third rows and the fourth and fifth rows.
Verifying these root equalities are again necessary to ensure that all the six merkle proofs for the six
rows are for the same tree. We also have empty leaves at certain rows for INSERT and DELETE. We have global
constraints checking all such properties.

3. For INSERT and READ-ZERO, we have the property:
hKeyMinus < hKey < hKeyPlus (refer to the document shared above). We call this the sandwitch property.
We use a dedicated wizard called Byte32cmp to verify this.

4. The sparse merkle tree follows a linked list structure. We need to verify that this property is maintained for every operations. For example, for INSERT, we neeed to verify that leafMinus.Next = leafPlus.Index before insertion. We call these verification the "pointer check". We have global constraints for this check for INSERT, DELETE, and READ-ZERO.

5. We check that Leave is the Poseidon2 hash of the leaf opening, a tuple of {PREV, NEXT, HKEY, HVAL}.

6. We check that NextFreeNode is constant throughout the rows of each operation except INSERT. For INSERT, it is incremented by 1 in row 3.

7. We check that TopRoot is the Poseidon2 hash of NextFreeNode and the ROOT.

8. We constraint that every column is zero in the inactive area.
*/

const (

	// Column names
	ACCUMULATOR_PROOFS_NAME                string       = "ACCUMULATOR_PROOFS"
	ACCUMULATOR_ROOTS_NAME                 ifaces.ColID = "ACCUMULATOR_ROOTS"
	ACCUMULATOR_POSITIONS_NAME             ifaces.ColID = "ACCUMULATOR_POSITIONS"
	ACCUMULATOR_LEAVES_NAME                ifaces.ColID = "ACCUMULATOR_LEAVES"
	ACCUMULATOR_USE_NEXT_MERKLE_PROOF_NAME ifaces.ColID = "ACCUMULATOR_USE_NEXT_PROOF"
	ACCUMULATOR_IS_ACTIVE_NAME             ifaces.ColID = "ACCUMULATOR_IS_ACTIVE"
	// Column for checking the sequentiality of merkle proofs
	ACCUMULATOR_COUNTER_NAME ifaces.ColID = "ACCUMULATOR_COUNTER"
	// Column for local consistency check
	ACCUMULATOR_IS_FIRST_NAME         ifaces.ColID = "ACCUMULATOR_IS_FIRST"
	ACCUMULATOR_IS_INSERT_NAME        ifaces.ColID = "ACCUMULATOR_IS_INSERT"
	ACCUMULATOR_IS_DELETE_NAME        ifaces.ColID = "ACCUMULATOR_IS_DELETE"
	ACCUMULATOR_IS_UPDATE_NAME        ifaces.ColID = "ACCUMULATOR_IS_UPDATE"
	ACCUMULATOR_IS_READ_ZERO_NAME     ifaces.ColID = "ACCUMULATOR_IS_READ_ZERO"
	ACCUMULATOR_IS_READ_NON_ZERO_NAME ifaces.ColID = "ACCUMULATOR_IS_READ_NON_ZERO"
	// Columns for sandwich check
	ACCUMULATOR_HKEY_NAME                 ifaces.ColID = "ACCUMULATOR_HKEY"
	ACCUMULATOR_HKEY_SANDWITCH_NAME       ifaces.ColID = "ACCUMULATOR_HKEY_SANDWITCH"
	ACCUMULATOR_HKEY_MINUS_NAME           ifaces.ColID = "ACCUMULATOR_HKEY_MINUS"
	ACCUMULATOR_HKEY_MINUS_SANDWITCH_NAME ifaces.ColID = "ACCUMULATOR_HKEY_MINUS_SANDWITCH"
	ACCUMULATOR_HKEY_PLUS_NAME            ifaces.ColID = "ACCUMULATOR_HKEY_PLUS"
	ACCUMULATOR_HKEY_PLUS_SANDWITCH_NAME  ifaces.ColID = "ACCUMULATOR_HKEY_PLUS_SANDWITCH"
	// Columns for pointer check
	ACCUMULATOR_LEAF_MINUS_INDEX_NAME   ifaces.ColID = "ACCUMULATOR_LEAF_MINUS_INDEX"
	ACCUMULATOR_LEAF_MINUS_NEXT_NAME    ifaces.ColID = "ACCUMULATOR_LEAF_MINUS_NEXT"
	ACCUMULATOR_LEAF_PLUS_INDEX_NAME    ifaces.ColID = "ACCUMULATOR_LEAF_PLUS_INDEX"
	ACCUMULATOR_LEAF_PLUS_PREV_NAME     ifaces.ColID = "ACCUMULATOR_LEAF_PLUS_PREV"
	ACCUMULATOR_LEAF_DELETED_INDEX_NAME ifaces.ColID = "ACCUMULATOR_LEAF_DELETED_INDEX"
	ACCUMULATOR_LEAF_DELETED_PREV_NAME  ifaces.ColID = "ACCUMULATOR_LEAF_DELETED_PREV"
	ACCUMULATOR_LEAF_DELETED_NEXT_NAME  ifaces.ColID = "ACCUMULATOR_LEAF_DELETED_NEXT"
	// Columns for leaf hash check (some more columns for this are declared below)
	ACCUMULATOR_LEAF_HASHES_NAME   ifaces.ColID = "ACCUMULATOR_LEAF_HASHES"
	ACCUMULATOR_IS_EMPTY_LEAF_NAME ifaces.ColID = "ACCUMULATOR_IS_EMPTY_LEAF"
	// Columns for NextFreeNode consistency check
	ACCUMULATOR_NEXT_FREE_NODE_NAME       ifaces.ColID = "ACCUMULATOR_NEXT_FREE_NODE"
	ACCUMULATOR_INSERTION_PATH_NAME       ifaces.ColID = "ACCUMULATOR_INSERTION_PATH"
	ACCUMULATOR_IS_INSERT_ROW3_NAME       ifaces.ColID = "ACCUMULATOR_IS_INSERT_ROW3"
	ACCUMULATOR_NEXT_FREE_NODE_SHIFT_NAME ifaces.ColID = "ACCUMULATOR_NEXT_FREE_NODE_SHIFT"
	// Columns for hashing the top root
	ACCUMULATOR_INTERM_ZERO_TOP_ROOT_NAME ifaces.ColID = "ACCUMULATOR_INTERM_ZERO_TOP_ROOT"
	ACCUMULATOR_INTERM_ONE_TOP_ROOT_NAME  ifaces.ColID = "ACCUMULATOR_INTERM_ONE_TOP_ROOT"
	ACCUMULATOR_TOP_ROOT_NAME             ifaces.ColID = "ACCUMULATOR_TOP_ROOT"
)

// structure for leaf opening
type LeafOpenings struct {
	Prev [common.NbElemForHasingU64]ifaces.Column
	Next [common.NbElemForHasingU64]ifaces.Column
	HKey [common.NbElemPerHash]ifaces.Column
	HVal [common.NbElemPerHash]ifaces.Column
}

// Module module
type Module struct {
	// The state manager accumulator settings
	Settings
	// compiled IOP
	Comp *wizard.CompiledIOP
	Cols struct {
		Leaves    [common.NbElemPerHash]ifaces.Column
		Roots     [common.NbElemPerHash]ifaces.Column
		Positions [common.NbElemForHasingU64]ifaces.Column
		Proofs    *merkle.FlatProof
		// Column to verify reuse of Merkle proofs in INSERT, DELETE, and UPDATE operations
		UseNextMerkleProof ifaces.Column
		// Column denoting the active area of the accumulator module
		IsActiveAccumulator ifaces.Column
		// Columns to check sequentiality of the accumulator module with Merkle module
		AccumulatorCounter [common.NbElemForHasingU64]ifaces.Column
		// Column to verify the two equalities of intermediateRoot1 and intermediateRoot3, and empty
		// leafs for INSERT and DELETE operation and one equality of root in IsReadZero operation
		IsFirst ifaces.Column
		// Column indicating an INSERT operation
		IsInsert ifaces.Column
		// Column indicating an DELETE operation
		IsDelete ifaces.Column
		// Column indicating an UPDATE operation
		IsUpdate ifaces.Column
		// Column indicating an READ-ZERO operation
		IsReadZero ifaces.Column
		// Column indicating an READ-NONZERO operation
		IsReadNonZero ifaces.Column

		// Columns for the sandwitch check
		// Columns storing the hash of the key of the trace
		HKey [common.NbElemPerHash]ifaces.Column
		// Columns storing the hash of the key of the previous leaf
		HKeyMinus [common.NbElemPerHash]ifaces.Column
		// Columns storing the hash of the key of the next leaf
		HKeyPlus [common.NbElemPerHash]ifaces.Column
		// HKeySandwitch stores the hash of the key of the trace, 2 byte per field element for sandwitch check
		HKeySandwitch [common.NbElemForHashingByte32Sandwitch]ifaces.Column
		// HKeyMinusSandwitch stores the hash of the key of the previous leaf, 2 byte per field element for sandwitch check
		HKeyMinusSandwitch [common.NbElemForHashingByte32Sandwitch]ifaces.Column
		// HKeyPlusSandwitch stores the hash of the key of the next leaf, 2 byte per field element for sandwitch check
		HKeyPlusSandwitch [common.NbElemForHashingByte32Sandwitch]ifaces.Column
		// HkeyHkeyMinusIsGreater contains boolean value indicating whether the HKey is greater than HKeyMinus
		HkeyHkeyMinusIsGreater ifaces.Column
		// HkeyPlusHkeyIsGreater contains boolean value indicating whether the HKeyPlus is greater than HKey
		HkeyPlusHkeyIsGreater ifaces.Column

		// Columns for the pointer check
		// Columns storing the index of the minus leaf
		LeafMinusIndex [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the next leaf of the minus leaf
		LeafMinusNext [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the plus leaf
		LeafPlusIndex [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the previous leaf of the plus leaf
		LeafPlusPrev [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the deleted leaf
		LeafDeletedIndex [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the previous leaf of the deleted leaf
		LeafDeletedPrev [common.NbElemForHasingU64]ifaces.Column
		// Columns storing the index of the next leaf of the deleted leaf
		LeafDeletedNext [common.NbElemForHasingU64]ifaces.Column

		// Columns for leaf hashing check
		// LeafOpening contains four columns corresponding to HKey, HVal, Prev, and Next
		LeafOpenings LeafOpenings
		// Interm contains the three intermediate states corresponding to the Poseidon2 block computation divided into limbs
		IntermZero, IntermOne, IntermTwo, IntermThree, IntermFour [common.NbElemPerHash]ifaces.Column
		// Zero contains the column with zero value, used in the Poseidon2 query
		Zero [common.NbElemPerHash]ifaces.Column
		// LeafHash contains the leafHashes (the final Poseidon2 block), equals with Leaves, except when it is empty leaf
		LeafHashes [common.NbElemPerHash]ifaces.Column
		// IsEmptyLeaf is one when Leaves contains empty leaf and does not match with LeafHash
		IsEmptyLeaf ifaces.Column

		// Columns to check NextFreeNode consistency
		// NextFreeNode stores the nextFreeNode for each row of every operation
		NextFreeNode [common.NbElemForHasingU64]ifaces.Column
		// NextFreeNodeIncremented stores a value that indicates how much NextFreeNode[i] is increased
		// relative to the NextFreeNode[i-1]
		NextFreeNodeIncremented [common.NbElemForHasingU64]ifaces.Column
		// InsertionPath stores the index of the newly inserted leaf by INSERT
		InsertionPath [common.NbElemForHasingU64]ifaces.Column
		// IsInsertRow3 is one for row 3 of INSERT operation
		IsInsertRow3 ifaces.Column

		// Columns for hashing the top root
		// IntermZeroTopRoot contains the the first intermediate Poseidon2 state hash
		IntermZeroTopRoot [common.NbElemPerHash]ifaces.Column
		// IntermOneTopRoot contains the second intermediate Poseidon2 state hash
		IntermOneTopRoot [common.NbElemPerHash]ifaces.Column
		// TopRoot contains the Poseidon2 hash of Roots and NextFreeNode
		TopRoot [common.NbElemPerHash]ifaces.Column
	}

	// MerkleProofVerification is the merkle tree verification wizard
	MerkleProofVerification *merkle.FlatMerkleProofVerification
	// AccumulatorCounterProver creates a new column which indicates the row-wise increment of AccumulatorCounter
	// with corresponding constraints
	AccumulatorCounterProver wizard.ProverAction
	// HkeyHkeyMinusProver creates a new column which indicates whether the HKey is greater than HkeyMinus
	HkeyHkeyMinusProver wizard.ProverAction
	// HkeyPlusHkeyProver creates a new column which indicates whether the HkeyPlus is greater than Hkey
	HkeyPlusHkeyProver wizard.ProverAction
	// NextFreeNodeShiftProver checks that NextFreeNode is increased when new node is inserted.
	NextFreeNodeShiftProver wizard.ProverAction
	// OffsetLimbRepeated is the limb version of the offset of the AccumulatorCounter. It is used to avoid error in the audit phase of the
	// distributed prove
	OffsetLimbRepeated [common.NbElemForHasingU64]*dedicated.RepeatedPattern
}

// NewModule generates and constraints the accumulator module. The accumulator
// module is entrusted to check all individual Linea's state accumulator traces.
func NewModule(comp *wizard.CompiledIOP, s Settings) Module {
	am := Module{}
	am.define(comp, s)
	return am
}

// Funtion registering and committing all the columns and queries in the Accumulator module
func (am *Module) define(comp *wizard.CompiledIOP, s Settings) {

	// All the columns and queries from the state-manager are for the round 0
	am.Settings = s
	am.Comp = comp

	// Initializes the columns
	for i := 0; i < common.NbElemPerHash; i++ {
		am.Cols.Leaves[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAVES_NAME, i), am.NumRows(), true)
		am.Cols.Roots[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_ROOTS_NAME, i), am.NumRows(), true)

		// columns for the sandwitch check
		am.Cols.HKey[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_HKEY_NAME, i), am.NumRows(), true)
		am.Cols.HKeyMinus[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_HKEY_MINUS_NAME, i), am.NumRows(), true)
		am.Cols.HKeyPlus[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_HKEY_PLUS_NAME, i), am.NumRows(), true)

		// TopRoot hash check columns commitments
		am.Cols.IntermZeroTopRoot[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_ZERO_TOP_ROOT_NAME, i), am.NumRows(), true)
		am.Cols.IntermOneTopRoot[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_ONE_TOP_ROOT_NAME, i), am.NumRows(), true)
		am.Cols.TopRoot[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_TOP_ROOT_NAME, i), am.NumRows(), true)
	}

	for i := 0; i < common.NbElemForHashingByte32Sandwitch; i++ {
		am.Cols.HKeySandwitch[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_sandwitch_%d", ACCUMULATOR_HKEY_SANDWITCH_NAME, i), am.NumRows(), true)
		am.Cols.HKeyMinusSandwitch[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_sandwitch_%d", ACCUMULATOR_HKEY_MINUS_SANDWITCH_NAME, i), am.NumRows(), true)
		am.Cols.HKeyPlusSandwitch[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_sandwitch_%d", ACCUMULATOR_HKEY_PLUS_SANDWITCH_NAME, i), am.NumRows(), true)
	}

	for i := 0; i < len(am.Cols.Positions); i++ {
		am.Cols.Positions[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_POSITIONS_NAME, i), am.NumRows(), true)
		am.Cols.NextFreeNode[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_NEXT_FREE_NODE_NAME, i), am.NumRows(), true)
		am.Cols.InsertionPath[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INSERTION_PATH_NAME, i), am.NumRows(), true)
		am.Cols.AccumulatorCounter[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_COUNTER_NAME, i), am.NumRows(), true)

		// columns for the pointer check
		am.Cols.LeafMinusIndex[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_MINUS_INDEX_NAME, i), am.NumRows(), true)
		am.Cols.LeafMinusNext[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_MINUS_NEXT_NAME, i), am.NumRows(), true)
		am.Cols.LeafPlusIndex[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_PLUS_INDEX_NAME, i), am.NumRows(), true)
		am.Cols.LeafPlusPrev[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_PLUS_PREV_NAME, i), am.NumRows(), true)
		am.Cols.LeafDeletedIndex[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_DELETED_INDEX_NAME, i), am.NumRows(), true)
		am.Cols.LeafDeletedPrev[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_DELETED_PREV_NAME, i), am.NumRows(), true)
		am.Cols.LeafDeletedNext[i] = comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_DELETED_NEXT_NAME, i), am.NumRows(), true)
	}

	am.Cols.Proofs = merkle.NewProof(comp, am.Round, ACCUMULATOR_PROOFS_NAME, am.MerkleTreeDepth, am.NumRows())
	am.Cols.UseNextMerkleProof = comp.InsertCommit(am.Round, ACCUMULATOR_USE_NEXT_MERKLE_PROOF_NAME, am.NumRows(), true)
	am.Cols.IsActiveAccumulator = comp.InsertCommit(am.Round, ACCUMULATOR_IS_ACTIVE_NAME, am.NumRows(), true)
	am.Cols.IsFirst = comp.InsertCommit(am.Round, ACCUMULATOR_IS_FIRST_NAME, am.NumRows(), true)
	am.Cols.IsInsert = comp.InsertCommit(am.Round, ACCUMULATOR_IS_INSERT_NAME, am.NumRows(), true)
	am.Cols.IsDelete = comp.InsertCommit(am.Round, ACCUMULATOR_IS_DELETE_NAME, am.NumRows(), true)
	am.Cols.IsUpdate = comp.InsertCommit(am.Round, ACCUMULATOR_IS_UPDATE_NAME, am.NumRows(), true)
	am.Cols.IsReadZero = comp.InsertCommit(am.Round, ACCUMULATOR_IS_READ_ZERO_NAME, am.NumRows(), true)
	am.Cols.IsReadNonZero = comp.InsertCommit(am.Round, ACCUMULATOR_IS_READ_NON_ZERO_NAME, am.NumRows(), true)

	// define the zero column for leaf hashing check
	am.defineZero()

	// Leaf hashing columns commitments
	am.commitLeafHashingCols()

	// NextFreeNode check columns commitments
	am.Cols.IsInsertRow3 = comp.InsertCommit(am.Round, ACCUMULATOR_IS_INSERT_ROW3_NAME, am.NumRows(), true)

	// Declare constraints

	// Checks the two Root equalities for Insert,
	// also checks the booleanity of the column IsInsertRow1
	am.checkInsert()

	// Checks the two Root equalities for Delete,
	// also checks the booleanity of the column IsDeleteRow1
	am.checkDelete()

	// Checks that the HKey remains the same for an update operation
	am.checkUpdate()

	// Checks the root equality for the ReadZero operation, also checks the
	// booleanity of the IsReadZeroRow1 column
	am.checkReadZero()

	/*
		We check the below constraints:
		1. Check that IsUpdate, IsReadNonZero, UseNextMerkleProof are boolean when IsActiveAccumulator is 1
		2. AccumulatorCounter[i+1] = AccumulatorCounter[i] - 1 when IsActiveAccumulator is 1
		3. Local constraint that AccumulatorCounter starts at zero
		4. IsActiveAccumulator is boolean
		5. IsActiveAccumulator[i] = 0 IMPLIES IsActiveAccumulator[i+1] = 0
		6. When IsActiveAccumulator is 1, sum of IsInsert, IsDelete, IsUpadate, IsReadZero, IsReadNonZero is 1
	*/
	am.checkConsistency()

	// check that Leaf[i+3] and Leaf[i+4] are empty leaves when there is a
	// INSERT or a DELETE operation respectively.
	am.checkEmptyLeaf()

	// // Sandwitch check for INSERT and READ-ZERO operations
	// // We also check the consistency of the HKey, HKeyMinus, and HKeyPlus for INSERT and ReadZero operations.
	// // i.e., they are consistent with the corresponding leaf opening values
	am.checkSandwitch()

	// Pointer check for INSERT, READ-ZERO, and DELETE operations
	am.checkPointer()

	// Check leaf hashes
	am.checkLeafHashes()

	// Check NextFreeNode is constant through a segment unless there is an INSERT operation
	am.checkNextFreeNode()

	// Check hashing of TopRoot
	am.checkTopRootHash()

	// Column values are zero when IsActiveAccumulator is 0
	am.checkZeroInInactive()

	// Send the columns to the Merkle gadget for the rest of verification (along
	// with the reuse of Merkle proofs) in the Merkle gadget
	//
	// @alex: it would make sense to refactor the merkle package with an input
	// struct so that the function signature is more readable.
	am.MerkleProofVerification = merkle.CheckFlatMerkleProofs(
		comp,
		merkle.FlatProofVerificationInputs{
			Name:     "ACCUMULATOR_MERKLE_PROOFS",
			Proof:    *am.Cols.Proofs,
			Roots:    am.Cols.Roots,
			Leaf:     am.Cols.Leaves,
			Position: am.Cols.Positions,
			IsActive: am.Cols.IsActiveAccumulator,
		},
	)

	am.MerkleProofVerification.AddProofReuseConstraint(comp, am.Cols.UseNextMerkleProof)
}

// defineZero defines the precomputed column ZERO (always zero)
func (am *Module) defineZero() {
	for i := 0; i < common.NbElemPerHash; i++ {
		am.Cols.Zero[i] = verifiercol.NewConstantCol(field.Zero(), am.NumRows(), "")
	}
}

func (am *Module) commitLeafHashingCols() {
	ACCUMULATOR_LEAF_OPENING_PREV := "ACCUMULATOR_LEAF_OPENING_PREV"
	ACCUMULATOR_LEAF_OPENING_NEXT := "ACCUMULATOR_LEAF_OPENING_NEXT"
	ACCUMULATOR_LEAF_OPENING_HKEY := "ACCUMULATOR_LEAF_OPENING_HKEY"
	ACCUMULATOR_LEAF_OPENING_HVAL := "ACCUMULATOR_LEAF_OPENING_HVAL"
	ACCUMULATOR_INTERM_ZERO := "ACCUMULATOR_INTERM_PREV_FIRST"
	ACCUMULATOR_INTERM_ONE := "ACCUMULATOR_INTERM_PREV_SECOND"
	ACCUMULATOR_INTERM_TWO := "ACCUMULATOR_INTERM_NEXT_FIRST"
	ACCUMULATOR_INTERM_THREE := "ACCUMULATOR_INTERM_NEXT_SECOND"
	ACCUMULATOR_INTERM_FOUR := "ACCUMULATOR_INTERM_HKEY"
	for i := range am.Cols.LeafOpenings.Prev {
		am.Cols.LeafOpenings.Prev[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_OPENING_PREV, i), am.NumRows(), true)
		am.Cols.LeafOpenings.Next[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_OPENING_NEXT, i), am.NumRows(), true)
	}

	for i := 0; i < common.NbElemPerHash; i++ {
		am.Cols.LeafOpenings.HKey[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_OPENING_HKEY, i), am.NumRows(), true)
		am.Cols.LeafOpenings.HVal[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_OPENING_HVAL, i), am.NumRows(), true)
		am.Cols.LeafHashes[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_LEAF_HASHES_NAME, i), am.NumRows(), true)
		am.Cols.IntermZero[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_ZERO, i), am.NumRows(), true)
		am.Cols.IntermOne[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_ONE, i), am.NumRows(), true)
		am.Cols.IntermTwo[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_TWO, i), am.NumRows(), true)
		am.Cols.IntermThree[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_THREE, i), am.NumRows(), true)
		am.Cols.IntermFour[i] = am.Comp.InsertCommit(am.Round, ifaces.ColIDf("%s_%d", ACCUMULATOR_INTERM_FOUR, i), am.NumRows(), true)
	}

	am.Cols.IsEmptyLeaf = am.Comp.InsertCommit(am.Round, ACCUMULATOR_IS_EMPTY_LEAF_NAME, am.NumRows(), true)
}

func (am *Module) checkInsert() {
	cols := am.Cols

	for i := 0; i < common.NbElemPerHash; i++ {
		// (Root[i+1] - Root[i+2]) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i] = 0, The (i+1)th and (i+2)th roots are equal when there is an INSERT operation and the accumulator is active.
		expr1 := symbolic.Sub(column.Shift(cols.Roots[i], 1), column.Shift(cols.Roots[i], 2))
		expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr1)
		am.Comp.InsertGlobal(am.Round, am.qnamef("ROOT_EQUALITY_INSERT_1_%d", i), expr1)

		// (Root[i+3] - Root[i+4]) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i] = 0, The (i+3)th and (i+4)th roots are equal when there is an INSERT operation and the accumulator is active.
		expr2 := symbolic.Sub(column.Shift(cols.Roots[i], 3), column.Shift(cols.Roots[i], 4))
		expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr2)
		am.Comp.InsertGlobal(am.Round, am.qnamef("ROOT_EQUALITY_INSERT_2_%d", i), expr2)
	}

	// Booleanity of IsFirst
	expr3 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsFirst), cols.IsActiveAccumulator),
		cols.IsFirst)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_FIRST_BOOLEAN"), expr3)

	// Booleanity of IsInsert
	expr4 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsInsert), cols.IsActiveAccumulator),
		cols.IsInsert)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_INSERT_BOOLEAN"), expr4)
}

func (am *Module) checkDelete() {
	cols := am.Cols

	for i := 0; i < common.NbElemPerHash; i++ {
		// (Root[i+1] - Root[i+2]) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i] = 0, The (i+1)th and (i+2)th roots are equal when there is a DELETE operation and the accumulator is active.
		expr1 := symbolic.Sub(column.Shift(cols.Roots[i], 1), column.Shift(cols.Roots[i], 2))
		expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr1)
		am.Comp.InsertGlobal(am.Round, am.qnamef("ROOT_EQUALITY_DELETE_1_%d", i), expr1)

		// (Root[i+3] - Root[i+4]) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i] = 0, The (i+3)th and (i+4)th roots are equal when there is a DELETE operation and the accumulator is active.
		expr2 := symbolic.Sub(column.Shift(cols.Roots[i], 3), column.Shift(cols.Roots[i], 4))
		expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr2)
		am.Comp.InsertGlobal(am.Round, am.qnamef("ROOT_EQUALITY_DELETE_2_%d", i), expr2)
	}

	// Booleanity of IsDelete
	expr3 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsDelete), cols.IsActiveAccumulator),
		cols.IsDelete)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_DELETE_BOOLEAN"), expr3)
}

func (am *Module) checkUpdate() {
	cols := am.Cols

	for i := 0; i < common.NbElemPerHash; i++ {
		// HKey remains the same for an update operation, i.e,
		// IsActiveAccumulator[i] * IsUpdate[i] * IsFirst[i] * (HKey[i] - HKey[i+1])
		expr := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsUpdate,
			cols.IsFirst,
			symbolic.Sub(cols.HKey[i], column.Shift(cols.HKey[i], 1)))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_EQUAL_FOR_UPDATE_%d", i), expr)
	}
}

func (am *Module) checkReadZero() {
	cols := am.Cols

	for i := 0; i < common.NbElemPerHash; i++ {
		// (Root[i] - Root[i+1]) * IsActiveAccumulator[i] * IsFirst[i] * IsReadZero[i] = 0, The ith and (i+1)th roots are equal when there is a READ-ZERO operation and the accumulator is active.
		expr1 := symbolic.Sub(cols.Roots[i], column.Shift(cols.Roots[i], 1))
		expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsReadZero, expr1)
		am.Comp.InsertGlobal(am.Round, am.qnamef("ROOT_EQUALITY_READ_ZERO_%d", i), expr1)
	}

	// Booleanity of IsReadZero
	expr2 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsReadZero), cols.IsActiveAccumulator),
		cols.IsReadZero)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_READ_ZERO_BOOLEAN"), expr2)
}

func (am *Module) checkConsistency() {
	cols := am.Cols
	// AccumulatorCounter[i+1] - AccumulatorCounter[i] = 1 when IsActiveAccumulator is 1
	shiftedAccumulatorCounter := make([]ifaces.Column, len(cols.AccumulatorCounter))
	for i := range shiftedAccumulatorCounter {
		shiftedAccumulatorCounter[i] = column.Shift(cols.AccumulatorCounter[i], 1)
	}

	offsetLimbs := make([]field.Element, len(am.Cols.AccumulatorCounter))
	for i := range offsetLimbs {
		if i == len(offsetLimbs)-1 {
			offsetLimbs[i] = field.NewElement(1)
		} else {
			offsetLimbs[i] = field.Zero()
		}
	}
	offsetCols := make([]ifaces.Column, len(am.Cols.AccumulatorCounter))
	for i := range offsetCols {
		am.OffsetLimbRepeated[i] = dedicated.NewRepeatedPattern(
			am.Comp,
			am.Round,
			[]field.Element{offsetLimbs[i]},
			verifiercol.NewConstantCol(field.One(), am.NumRows(), "accumulator-offset-limb-repeated"),
			"OFFSET_LIMB_REPEATED_"+strconv.Itoa(i),
		)
		offsetCols[i] = am.OffsetLimbRepeated[i].Natural
	}

	_, am.AccumulatorCounterProver = byte32cmp.NewMultiLimbAdd(am.Comp, &byte32cmp.MultiLimbAddIn{
		Name: string(ACCUMULATOR_COUNTER_NAME),
		ALimbs: byte32cmp.LimbColumns{
			Limbs:       am.Cols.AccumulatorCounter[:],
			LimbBitSize: 16,
			IsBigEndian: true,
		},
		BLimbs: byte32cmp.LimbColumns{
			Limbs:       offsetCols[:],
			LimbBitSize: 16,
			IsBigEndian: true,
		},
		Mask: symbolic.NewVariable(column.Shift(am.Cols.IsActiveAccumulator, 1)),
		Result: byte32cmp.LimbColumns{
			Limbs:       shiftedAccumulatorCounter,
			LimbBitSize: 16,
			IsBigEndian: true,
		},
	}, true)

	// Local constraint that AccumulatorCounter starts at zero
	for i := 0; i < len(cols.AccumulatorCounter); i++ {
		am.Comp.InsertLocal(am.Round,
			am.qnamef("ACCUMULATOR_COUNTER_STARTS_AT_ZERO_%d", i),
			symbolic.Sub(cols.AccumulatorCounter[i], 0))
	}

	// Booleanity of IsUpdate
	expr1 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsUpdate), cols.IsActiveAccumulator),
		cols.IsUpdate)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_UPDATE_BOOLEAN"), expr1)

	// Booleanity of IsReadNonZero
	expr2 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsReadNonZero), cols.IsActiveAccumulator),
		cols.IsReadNonZero)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_READ_NON_ZERO_BOOLEAN"), expr2)

	// Booleanity of UseNextMerkleProof
	expr3 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.UseNextMerkleProof), cols.IsActiveAccumulator),
		cols.UseNextMerkleProof)
	am.Comp.InsertGlobal(am.Round, am.qname("USE_NEXT_MERKLE_PROOF_BOOLEAN"), expr3)

	// Booleanity of IsActiveAccumulator
	expr5 := symbolic.Sub(
		symbolic.Square(cols.IsActiveAccumulator),
		cols.IsActiveAccumulator)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_ACTIVE_ACCUMULATOR_BOOLEAN"), expr5)

	// IsActiveAccumulator[i] = 0 IMPLIES IsActiveAccumulator[i+1] = 0 e.g. IsActiveAccumulator[i] = IsActiveAccumulator[i-1]*IsActiveAccumulator[i]
	expr6 := symbolic.Sub(cols.IsActiveAccumulator,
		symbolic.Mul(column.Shift(cols.IsActiveAccumulator, -1),
			cols.IsActiveAccumulator))
	am.Comp.InsertGlobal(am.Round, am.qname("IS_ACTIVE_ACCUMULATOR_ZERO_FOLLOWED_BY_ZERO"), expr6)

	// When IsActiveAccumulator is 1, sum of IsInsert, IsDelete, IsUpadate, IsReadZero, IsReadNonZero is 1 e.g., they are mutually exclusive
	expr7 := symbolic.Sub(cols.IsActiveAccumulator,
		symbolic.Add(cols.IsInsert, cols.IsDelete, cols.IsUpdate, cols.IsReadZero, cols.IsReadNonZero))
	am.Comp.InsertGlobal(am.Round, am.qname("ACCUMULATOR_OPS_MUTUALLY_EXCLUSIVE"), expr7)
}

func (am *Module) checkEmptyLeaf() {
	// Creating the emptyLeaf column
	var (
		emptyLeaf [common.NbElemPerHash]ifaces.Column
	)
	for i := range common.NbElemPerHash {
		emptyLeaf[i] = verifiercol.NewConstantCol(field.Zero(), am.NumRows(), "accumulator-empty-leaf-element")
	}

	cols := am.Cols

	for i := 0; i < common.NbElemPerHash; i++ {
		// (Leaf[i+2] - emptyLeaf) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i]
		expr1 := symbolic.Sub(column.Shift(cols.Leaves[i], 2), emptyLeaf[i])
		expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr1)
		am.Comp.InsertGlobal(am.Round, am.qnamef("EMPTY_LEAVES_FOR_INSERT_%d", i), expr1)

		// (Leaf[i+3] - emptyLeaf) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i]
		expr2 := symbolic.Sub(column.Shift(cols.Leaves[i], 3), emptyLeaf[i])
		expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr2)
		am.Comp.InsertGlobal(am.Round, am.qnamef("EMPTY_LEAVES_FOR_DELETE_%d", i), expr2)
	}
}

func (am *Module) compareMultiLimbs(colALimbs []ifaces.Column, colBLimbs []ifaces.Column) (ifaces.Column, wizard.ProverAction) {
	isGreater, _, _, prover := byte32cmp.CmpMultiLimbs(
		am.Comp,
		byte32cmp.LimbColumns{
			Limbs:       colALimbs,
			LimbBitSize: 16,
			IsBigEndian: true,
		},
		byte32cmp.LimbColumns{
			Limbs:       colBLimbs,
			LimbBitSize: 16,
			IsBigEndian: true,
		},
	)

	return isGreater, prover
}

func (am *Module) checkSandwitch() {
	cols := am.Cols
	// We want sandwitch check only at row 1 of INSERT and READ-ZERO
	activeRow := symbolic.Add(symbolic.Mul(cols.IsFirst, cols.IsInsert), symbolic.Mul(cols.IsFirst, cols.IsReadZero))

	am.Cols.HkeyHkeyMinusIsGreater, am.HkeyHkeyMinusProver = am.compareMultiLimbs(am.Cols.HKeySandwitch[:], am.Cols.HKeyMinusSandwitch[:])
	am.Comp.InsertGlobal(
		am.Round,
		am.qname("CMP_HKEY_HKEY_MINUS"),
		symbolic.Mul(
			activeRow,
			symbolic.Sub(1, am.Cols.HkeyHkeyMinusIsGreater),
		),
	)

	am.Cols.HkeyPlusHkeyIsGreater, am.HkeyPlusHkeyProver = am.compareMultiLimbs(am.Cols.HKeyPlusSandwitch[:], am.Cols.HKeySandwitch[:])
	am.Comp.InsertGlobal(
		am.Round,
		am.qname("CMP_HKEY_PLUS_HKEY"),
		symbolic.Mul(
			activeRow,
			symbolic.Sub(1, am.Cols.HkeyPlusHkeyIsGreater),
		),
	)

	for i := 0; i < common.NbElemPerHash; i++ {
		// constraint to check that hey, hkeyMinus, hkeyPlus are consistent with hKeySandwitch,
		// hKeyMinusSandwitch, hKeyPlusSandwitch respectively for INSERT and READ-ZERO operations
		mulOffset := 1 << 16 // 2^16, since each limb of hkeySandwitch is 16 bits, whereas 32 bits for hkey
		expr0_0 := symbolic.Mul(
			activeRow,
			symbolic.Sub(
				cols.HKey[i],
				symbolic.Add(
					cols.HKeySandwitch[2*i+1],
					symbolic.Mul(
						cols.HKeySandwitch[2*i],
						field.NewElement(uint64(mulOffset)),
					),
				),
			),
		)
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_SANDWITCH_CONSISTENCY_%d", i), expr0_0)

		expr0_1 := symbolic.Mul(
			activeRow,
			symbolic.Sub(
				cols.HKeyMinus[i],
				symbolic.Add(
					cols.HKeyMinusSandwitch[2*i+1],
					symbolic.Mul(
						cols.HKeyMinusSandwitch[2*i],
						field.NewElement(uint64(mulOffset)),
					),
				),
			),
		)
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_MINUS_SANDWITCH_CONSISTENCY_%d", i), expr0_1)

		expr0_2 := symbolic.Mul(
			activeRow,
			symbolic.Sub(
				cols.HKeyPlus[i],
				symbolic.Add(
					cols.HKeyPlusSandwitch[2*i+1],
					symbolic.Mul(
						cols.HKeyPlusSandwitch[2*i],
						field.NewElement(uint64(mulOffset)),
					),
				),
			),
		)
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_PLUS_SANDWITCH_CONSISTENCY_%d", i), expr0_2)

		// INSERT: The HKeyMinus in the leaf minus openings is the same as HKeyMinus column i.e.,
		// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKeyMinus[i] - LeafOpenings.Hkey[i])
		expr1 := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsInsert,
			cols.IsFirst,
			symbolic.Sub(cols.HKeyMinus[i], cols.LeafOpenings.HKey[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_MINUS_CONSISTENCY_INSERT_%d", i), expr1)

		// INSERT: The HKey in the inserted leaf openings (in the fourth row) is the same as HKey column i.e.,
		// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKey[i] - LeafOpenings.Hkey[i+3])
		expr2 := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsInsert,
			cols.IsFirst,
			symbolic.Sub(cols.HKey[i], column.Shift(cols.LeafOpenings.HKey[i], 3)))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_CONSISTENCY_INSERT_%d", i), expr2)

		// INSERT: The HKeyPlus in the plus leaf openings is the same as HKeyPlus column i.e.,
		// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKeyPlus[i] - LeafOpenings.Hkey[i+4])
		expr3 := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsInsert,
			cols.IsFirst,
			symbolic.Sub(cols.HKeyPlus[i], column.Shift(cols.LeafOpenings.HKey[i], 4)))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_PLUS_CONSISTENCY_INSERT_%d", i), expr3)

		// READ-ZERO: The HKeyMinus in the minus leaf openings is the same as HKeyMinus column i.e.,
		// IsActiveAccumulator[i] * IsReadZero[i] * IsFirst[i] * (HKeyMinus[i] - LeafOpenings.Hkey[i])
		expr4 := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsReadZero,
			cols.IsFirst,
			symbolic.Sub(cols.HKeyMinus[i], cols.LeafOpenings.HKey[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_MINUS_CONSISTENCY_READ_ZERO_%d", i), expr4)

		// READ-ZERO: The HKeyPlus in the plus leaf openings is the same as HKeyPlus column i.e.,
		// IsActiveAccumulator[i] * IsReadZero[i] * IsFirst[i] * (HKeyPlus[i] - LeafOpenings.Hkey[i+1])
		expr5 := symbolic.Mul(cols.IsActiveAccumulator,
			cols.IsReadZero,
			cols.IsFirst,
			symbolic.Sub(cols.HKeyPlus[i], column.Shift(cols.LeafOpenings.HKey[i], 1)))
		am.Comp.InsertGlobal(am.Round, am.qnamef("HKEY_PLUS_CONSISTENCY_READ_ZERO_%d", i), expr5)
	}
}

func (am *Module) checkPointer() {
	cols := am.Cols
	for i := range cols.LeafPlusIndex {
		// Check #1 for INSERT: IsFirst[i] * IsInsert[i] * (LeafMinusNext[i] - LeafPlusIndex[i])
		expr1 := symbolic.Mul(cols.IsFirst, cols.IsInsert, symbolic.Sub(cols.LeafMinusNext[i], cols.LeafPlusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("INSERT_POINTER_1_%d", i), expr1)

		// Check #2 for INSERT: IsFirst[i] * IsInsert[i] *(LeafPlusPrev[i] - LeafMinusIndex[i])
		expr2 := symbolic.Mul(cols.IsFirst, cols.IsInsert, symbolic.Sub(cols.LeafPlusPrev[i], cols.LeafMinusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("INSERT_POINTER_2_%d", i), expr2)
		// Check #1 for DELETE: IsFirst[i] * IsDelete[i] * (LeafMinusNext[i] - LeafDeletedIndex[i])
		expr3 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafMinusNext[i], cols.LeafDeletedIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("DELETE_POINTER_1_%d", i), expr3)

		// Check #2 for DELETE: IsFirst[i] * IsDelete[i] * (LeafDeletedPrev[i] - LeafMinusIndex[i])
		expr4 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafDeletedPrev[i], cols.LeafMinusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("DELETE_POINTER_2_%d", i), expr4)
		// Check #3 for DELETE: IsFirst[i] * IsDelete[i] * (LeafDeletedNext[i] - LeafPlusIndex[i])
		expr5 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafDeletedNext[i], cols.LeafPlusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("DELETE_POINTER_3_%d", i), expr5)

		// Check #4 for DELETE: IsFirst[i] * IsDelete[i] * (LeafPlusPrev[i] - LeafDeletedIndex[i])
		expr6 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafPlusPrev[i], cols.LeafDeletedIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("DELETE_POINTER_4_%d", i), expr6)
		// Check #1 for READ-ZERO: IsFirst[i] * IsReadZero[i] * (LeafMinusNext[i] - LeafPlusIndex[i])
		expr7 := symbolic.Mul(cols.IsFirst, cols.IsReadZero, symbolic.Sub(cols.LeafMinusNext[i], cols.LeafPlusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("READ_ZERO_POINTER_1_%d", i), expr7)

		// Check #2 for READ-ZERO: IsFirst[i] * IsReadZero[i] * (LeafPlusPrev[i] - LeafMinusIndex[i])
		expr8 := symbolic.Mul(cols.IsFirst, cols.IsReadZero, symbolic.Sub(cols.LeafPlusPrev[i], cols.LeafMinusIndex[i]))
		am.Comp.InsertGlobal(am.Round, am.qnamef("READ_ZERO_POINTER_2_%d", i), expr8)
	}
}

func (am *Module) checkLeafHashes() {
	cols := am.Cols
	// we assume prev and next length = 2 * common.NbElemPerHash
	prevFirst := cols.LeafOpenings.Prev[0:common.NbElemPerHash]
	nextFirst := cols.LeafOpenings.Next[0:common.NbElemPerHash]
	prevLast := cols.LeafOpenings.Prev[common.NbElemPerHash : 2*common.NbElemPerHash]
	nextLast := cols.LeafOpenings.Next[common.NbElemPerHash : 2*common.NbElemPerHash]

	// IntermZero = Poseidon2(Zero, PrevFirst), old = Zero, Block = PrevFirst
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_PREV_FIRST"), [8]ifaces.Column(prevFirst), cols.Zero, [8]ifaces.Column(cols.IntermZero), cols.IsActiveAccumulator)
	// IntermOne = Poseidon2(IntermZero, PrevLast), old = IntermZero, Block = PrevLast
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_PREV_LAST"), [8]ifaces.Column(prevLast), [8]ifaces.Column(cols.IntermZero), [8]ifaces.Column(cols.IntermOne), cols.IsActiveAccumulator)
	// IntermTwo = Poseidon2(IntermOne, NextFirst), old = IntermOne, Block = NextFirst
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_NEXT_FIRST"), [8]ifaces.Column(nextFirst), [8]ifaces.Column(cols.IntermOne), [8]ifaces.Column(cols.IntermTwo), cols.IsActiveAccumulator)
	// IntermThree = Poseidon2(IntermTwo, NextLast), old = IntermTwo, Block = NextLast
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_NEXT_LAST"), [8]ifaces.Column(nextLast), [8]ifaces.Column(cols.IntermTwo), [8]ifaces.Column(cols.IntermThree), cols.IsActiveAccumulator)
	// IntermFour = Poseidon2(IntermThree, HKey), old = IntermThree, Block = HKey
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_HKEY"), [8]ifaces.Column(cols.LeafOpenings.HKey), [8]ifaces.Column(cols.IntermThree), [8]ifaces.Column(cols.IntermFour), cols.IsActiveAccumulator)
	// LeafHashes = Poseidon2(IntermFour, HVal), old = IntermFour, Block = HVal
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_HVAL_LEAF"), [8]ifaces.Column(cols.LeafOpenings.HVal), [8]ifaces.Column(cols.IntermFour), [8]ifaces.Column(cols.LeafHashes), cols.IsActiveAccumulator)

	// Global: IsActive[i] * (1 - IsEmptyLeaf[i]) * (Leaves[i] - LeafHashes[i])
	for i := 0; i < common.NbElemPerHash; i++ {
		expr1 := symbolic.Sub(cols.Leaves[i], cols.LeafHashes[i])
		expr2 := symbolic.Sub(symbolic.NewConstant(1), cols.IsEmptyLeaf)
		expr3 := symbolic.Mul(cols.IsActiveAccumulator, expr1, expr2)
		am.Comp.InsertGlobal(am.Round, am.qnamef("LEAF_HASH_EQUALITY_%d", i), expr3)
	}

	// Booleaninty of IsEmptyLeaf: IsActive[i] * (IsEmptyLeaf^2[i] - IsEmptyLeaf[i])
	expr4 := symbolic.Sub(symbolic.Square(cols.IsEmptyLeaf), cols.IsEmptyLeaf)
	expr4 = symbolic.Mul(expr4, cols.IsActiveAccumulator)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_EMPTY_LEAF_BOOLEANITY"), expr4)

	// IsEmptyLeaf is set to true if and only if it is the third row for INSERT, or fourth row for DELETE
	// i.e. IsActiveAccumulator[i] * (IsEmptyLeaf[i] - IsFirst[i-2] * IsInsert[i-2] - IsFirst[i-3] * IsDelete[i-3])
	expr5 := symbolic.Mul(cols.IsActiveAccumulator,
		symbolic.Sub(cols.IsEmptyLeaf,
			symbolic.Mul(column.Shift(cols.IsFirst, -2), column.Shift(cols.IsInsert, -2)),
			symbolic.Mul(column.Shift(cols.IsFirst, -3), column.Shift(cols.IsDelete, -3))))
	am.Comp.InsertGlobal(am.Round, am.qname("IS_EMPTY_LEAF_ONE_FOR_INSERT_THIRD_ROW_AND_DELETE_FOURTH_ROW"), expr5)
}

func (am *Module) checkNextFreeNode() {
	cols := am.Cols
	/*
		IsActive[i] * (1 - IsFirst[i]) * (
		IsInsertRow3[i] * (NextFreeNode[i] - NextFreeNode[i-1] - 1)
		+ (1- IsInsertRow3[i]) * (NextFreeNode[i] - NextFreeNode[i-1])
		)
	*/

	shiftedNextFreeNode := make([]ifaces.Column, len(am.Cols.NextFreeNode))
	for i := range shiftedNextFreeNode {
		shiftedNextFreeNode[i] = column.Shift(am.Cols.NextFreeNode[i], -1)
	}

	// 1. Calc NextFreeNode[i] - NextFreeNode[i-1]
	nextFreeNodeIncremented, nextFreeNodeShiftProver := byte32cmp.NewMultiLimbAdd(am.Comp, &byte32cmp.MultiLimbAddIn{
		Name: string(ACCUMULATOR_NEXT_FREE_NODE_SHIFT_NAME),
		ALimbs: byte32cmp.LimbColumns{
			Limbs:       am.Cols.NextFreeNode[:],
			LimbBitSize: 16,
			IsBigEndian: true,
		},
		BLimbs: byte32cmp.LimbColumns{
			Limbs:       shiftedNextFreeNode,
			LimbBitSize: 16,
			IsBigEndian: true,
		},
		Mask:          symbolic.Mul(am.Cols.IsActiveAccumulator, symbolic.Sub(1, cols.IsFirst)),
		NoBoundCancel: true,
	},
		false)

	copy(am.Cols.NextFreeNodeIncremented[:], nextFreeNodeIncremented.Limbs)
	am.NextFreeNodeShiftProver = nextFreeNodeShiftProver

	// 2. Bound expression 1 with global constraint ensuring the result is 0, 0, 0,..., 1 applying mask IsActive[i] * (1 - IsFirst[i]) * IsInsertRow3[i]
	// First 15 limbs (most significant) has to be 0
	for i := 0; i < len(am.Cols.NextFreeNodeIncremented)-1; i++ {
		am.Comp.InsertGlobal(
			am.Round,
			am.qnamef("%v_RESULT_SUB_ONE_%d", ACCUMULATOR_NEXT_FREE_NODE_SHIFT_NAME, i),
			symbolic.Mul(
				am.Cols.IsActiveAccumulator,
				symbolic.Sub(1, cols.IsFirst),
				cols.IsInsertRow3,
				am.Cols.NextFreeNodeIncremented[i],
			),
		)
	}

	// Last 1 limb (least significant) has to be 1
	am.Comp.InsertGlobal(
		am.Round,
		am.qnamef(
			"%v_RESULT_SUB_ONE_%d",
			ACCUMULATOR_NEXT_FREE_NODE_SHIFT_NAME,
			len(am.Cols.NextFreeNodeIncremented)-1,
		),
		symbolic.Mul(am.Cols.IsActiveAccumulator,
			symbolic.Sub(1, cols.IsFirst),
			cols.IsInsertRow3,
			symbolic.Sub(1, am.Cols.NextFreeNodeIncremented[15]),
		),
	)

	// 3. Bound expression 1 with global constraint ensuring the result is 0, 0, 0, 0 applying mask IsActive[i] * (1 - IsFirst[i]) * (1- IsInsertRow3[i])
	for i := 0; i < len(am.Cols.NextFreeNodeIncremented); i++ {
		am.Comp.InsertGlobal(
			am.Round,
			am.qnamef("%v_RESULT_%d", ACCUMULATOR_NEXT_FREE_NODE_SHIFT_NAME, i),
			symbolic.Mul(
				am.Cols.IsActiveAccumulator,
				symbolic.Sub(1, cols.IsFirst),
				symbolic.Sub(1, cols.IsInsertRow3),
				am.Cols.NextFreeNodeIncremented[i],
			),
		)
	}

	// IsInsertRow3 is true if and only if it is row 3 for INSERT operation, i.e.,
	// IsActiveAccumulator[i] * (IsInsert[i] * IsEmptyLeaf[i] - IsInsertRow3[i]). The constraint that
	// IsEmptyLeaf is 1 if and only if it is row 3 for INSERT (and row 4 of DELETE) is imposed already.
	expr5 := symbolic.Mul(cols.IsActiveAccumulator,
		symbolic.Sub(symbolic.Mul(cols.IsInsert, cols.IsEmptyLeaf),
			cols.IsInsertRow3))
	am.Comp.InsertGlobal(am.Round, am.qname("IS_INSERT_ROW3_CONSISTENCY"), expr5)
}

func (am *Module) checkTopRootHash() {
	cols := am.Cols
	// we assume the length of NextFreeNode is 2 * common.NbElemPerHash
	// TopRoot = Hash (nextFreeNode, SubRoot)
	nextFreeNodeFirst := cols.NextFreeNode[0:common.NbElemPerHash]
	nextFreeNodeSecond := cols.NextFreeNode[common.NbElemPerHash : 2*common.NbElemPerHash]
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_INTERM_ZERO_TOP_ROOT"), [8]ifaces.Column(nextFreeNodeFirst), cols.Zero, cols.IntermZeroTopRoot, cols.IsActiveAccumulator)
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_INTERM_ONE_TOP_ROOT"), [8]ifaces.Column(nextFreeNodeSecond), cols.IntermZeroTopRoot, cols.IntermOneTopRoot, cols.IsActiveAccumulator)
	am.Comp.InsertPoseidon2(am.Round, am.qname("POSEIDON2_TOP_ROOT"), cols.Roots, cols.IntermOneTopRoot, cols.TopRoot, cols.IsActiveAccumulator)
}

func (am *Module) checkZeroInInactive() {
	cols := am.Cols
	for i := 0; i < common.NbElemPerHash; i++ {
		am.colZeroAtInactive(cols.Roots[i])
		am.colZeroAtInactive(cols.Leaves[i])

		am.colZeroAtInactive(cols.LeafOpenings.HKey[i])
		am.colZeroAtInactive(cols.LeafOpenings.HVal[i])
		am.colZeroAtInactive(cols.HKey[i])
		am.colZeroAtInactive(cols.HKeyMinus[i])
		am.colZeroAtInactive(cols.HKeyPlus[i])
	}

	for i := 0; i < len(cols.Positions); i++ {
		am.colZeroAtInactive(cols.Positions[i])

		am.colZeroAtInactive(cols.NextFreeNode[i])
		am.colZeroAtInactive(cols.InsertionPath[i])
		am.colZeroAtInactive(cols.AccumulatorCounter[i])
		am.colZeroAtInactive(cols.LeafMinusIndex[i])
		am.colZeroAtInactive(cols.LeafMinusNext[i])
		am.colZeroAtInactive(cols.LeafPlusIndex[i])
		am.colZeroAtInactive(cols.LeafPlusPrev[i])
		am.colZeroAtInactive(cols.LeafDeletedIndex[i])
		am.colZeroAtInactive(cols.LeafDeletedPrev[i])
		am.colZeroAtInactive(cols.LeafDeletedNext[i])
		am.colZeroAtInactive(cols.LeafOpenings.Prev[i])
		am.colZeroAtInactive(cols.LeafOpenings.Next[i])
	}
	// Skipping proof as it has unequal column length with IsActive
	// proof is unconstrained in this module, and the consistency check is done
	// in the Merkle module
	am.colZeroAtInactive(cols.UseNextMerkleProof)
	am.colZeroAtInactive(cols.IsFirst)
	am.colZeroAtInactive(cols.IsInsert)
	am.colZeroAtInactive(cols.IsDelete)
	am.colZeroAtInactive(cols.IsUpdate)
	am.colZeroAtInactive(cols.IsReadZero)
	am.colZeroAtInactive(cols.IsReadNonZero)
	// Skipping Interm, Zero, and LeafHashes as two of them contain zero hashes and
	// Zero is a verifier column. The padding area of Interm and LeafHashes
	// are already constrained by the Poseidon2 query
	am.colZeroAtInactive(cols.IsEmptyLeaf)
	am.colZeroAtInactive(cols.IsInsertRow3)
	// Again skipping IntermTopRoot and TopRoot as they contain zero hashes
}

// Function returning a query name for already-formed names (no printf)
func (am *Module) qname(name string) ifaces.QueryID {
	return ifaces.QueryIDf("%v_%v_%s", am.Name, am.Comp.SelfRecursionCount, name)
}

// Function returning a query name using printf-style formatting.
// Use this when the first arg is a format literal.
// nolint:printf
func (am *Module) qnamef(format string, args ...any) ifaces.QueryID {
	formatted := fmt.Sprintf(format, args...)
	return ifaces.QueryIDf("%v_%v_%s", am.Name, am.Comp.SelfRecursionCount, formatted)
}

// Function inserting a query that col is zero when IsActive is zero
func (am *Module) colZeroAtInactive(col ifaces.Column) {
	// col zero at inactive area, e.g., (1-IsActiveAccumulator[i]) * col[i] = 0
	name_ := string(col.GetColID() + "IN_INACTIVE")
	am.Comp.InsertGlobal(am.Round, am.qname(name_),
		symbolic.Mul(symbolic.Sub(1, am.Cols.IsActiveAccumulator), col))
}
