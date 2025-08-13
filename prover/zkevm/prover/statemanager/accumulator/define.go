package accumulator

import (
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/column"
	"github.com/consensys/linea-monorepo/prover/protocol/column/verifiercol"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/byte32cmp"
	"github.com/consensys/linea-monorepo/prover/protocol/dedicated/merkle"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/protocol/wizard"
	"github.com/consensys/linea-monorepo/prover/symbolic"
	"github.com/consensys/linea-monorepo/prover/utils/types"
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
they are for the same Merkle tree. We call it verifying "reuse of Merkle proof". These verifications are delegated
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

5. We check that Leave is the MiMC hash of the leaf opening, a tuple of {PREV, NEXT, HKEY, HVAL}.

6. We check that NextFreeNode is constant throughout the rows of each operation except INSERT. For INSERT, it is incremented by 1 in row 3.

7. We check that TopRoot is the MiMC hash of NextFreeNode and the ROOT.

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
	ACCUMULATOR_HKEY_NAME       ifaces.ColID = "ACCUMULATOR_HKEY"
	ACCUMULATOR_HKEY_MINUS_NAME ifaces.ColID = "ACCUMULATOR_HKEY_MINUS"
	ACCUMULATOR_HKEY_PLUS_NAME  ifaces.ColID = "ACCUMULATOR_HKEY_PLUS"
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
	ACCUMULATOR_NEXT_FREE_NODE_NAME ifaces.ColID = "ACCUMULATOR_NEXT_FREE_NODE"
	ACCUMULATOR_INSERTION_PATH_NAME ifaces.ColID = "ACCUMULATOR_INSERTION_PATH"
	ACCUMULATOR_IS_INSERT_ROW3_NAME ifaces.ColID = "ACCUMULATOR_IS_INSERT_ROW3"
	// Columns for hashing the top root
	ACCUMULATOR_INTERM_TOP_ROOT_NAME ifaces.ColID = "ACCUMULATOR_INTERM_TOP_ROOT"
	ACCUMULATOR_TOP_ROOT_NAME        ifaces.ColID = "ACCUMULATOR_TOP_ROOT"
)

// structure for leaf opening
type LeafOpenings struct {
	HKey ifaces.Column
	HVal ifaces.Column
	Prev ifaces.Column
	Next ifaces.Column
}

// Module module
type Module struct {
	// The compiled IOP
	Settings
	Comp *wizard.CompiledIOP
	Cols struct {
		Leaves    ifaces.Column
		Roots     ifaces.Column
		Positions ifaces.Column
		Proofs    *merkle.FlatProof
		// Column to verify reuse of Merkle proofs in INSERT, DELETE, and UPDATE operations
		UseNextMerkleProof ifaces.Column
		// Column denoting the active area of the accumulator module
		IsActiveAccumulator ifaces.Column
		// Column to check sequentiality of the accumulator module with Merkle module
		AccumulatorCounter ifaces.Column
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
		// Column storing the hash of the key of the trace
		HKey ifaces.Column
		// Column storing the hash of the key of the previous leaf
		HKeyMinus ifaces.Column
		// Column storing the hash of the key of the next leaf
		HKeyPlus ifaces.Column

		// Columns for the pointer check
		// Column storing the index of the minus leaf
		LeafMinusIndex ifaces.Column
		// Column storing the index of the next leaf of the minus leaf
		LeafMinusNext ifaces.Column
		// Column storing the index of the plus leaf
		LeafPlusIndex ifaces.Column
		// Column storing the index of the previous leaf of the plus leaf
		LeafPlusPrev ifaces.Column
		// Column storing the index of the deleted leaf
		LeafDeletedIndex ifaces.Column
		// Column storing the index of the previous leaf of the deleted leaf
		LeafDeletedPrev ifaces.Column
		// Column storing the index of the next leaf of the deleted leaf
		LeafDeletedNext ifaces.Column

		// Columns for leaf hashing check
		// LeafOpening contains four columns corresponding to HKey, HVal, Prev, and Next
		LeafOpenings LeafOpenings
		// Interm contains the three intermediate states corresponding to the MiMC block computation
		Interm []ifaces.Column
		// Zero contains the column with zero value, used in the MiMc query
		Zero ifaces.Column
		// LeafHash contains the leafHashes (the final MiMC block), equals with Leaves, except when it is empty leaf
		LeafHashes ifaces.Column
		// IsEmptyLeaf is one when Leaves contains empty leaf and does not match with LeafHash
		IsEmptyLeaf ifaces.Column

		// Columns to check NextFreeNode consistency
		// NextFreeNode stores the nextFreeNode for each row of every operation
		NextFreeNode ifaces.Column
		// InsertionPath stores the index of the newly inserted leaf by INSERT
		InsertionPath ifaces.Column
		// IsInsertRow3 is one for row 3 of INSERT operation
		IsInsertRow3 ifaces.Column

		// Columns for hashing the top root
		// IntermTopRoot contains the intermediate MiMC state hash
		IntermTopRoot ifaces.Column
		// TopRoot contains the MiMC hash of Roots and NextFreeNode
		TopRoot ifaces.Column
	}

	// MerkleProofVerification is the merkle tree verification wizard
	MerkleProofVerification *merkle.FlatMerkleProofVerification
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
	am.Cols.Leaves = comp.InsertCommit(am.Round, ACCUMULATOR_LEAVES_NAME, am.NumRows())
	am.Cols.Roots = comp.InsertCommit(am.Round, ACCUMULATOR_ROOTS_NAME, am.NumRows())
	am.Cols.Positions = comp.InsertCommit(am.Round, ACCUMULATOR_POSITIONS_NAME, am.NumRows())
	am.Cols.Proofs = merkle.NewProof(comp, am.Round, ACCUMULATOR_PROOFS_NAME, am.MerkleTreeDepth, am.NumRows())
	am.Cols.UseNextMerkleProof = comp.InsertCommit(am.Round, ACCUMULATOR_USE_NEXT_MERKLE_PROOF_NAME, am.NumRows())
	am.Cols.IsActiveAccumulator = comp.InsertCommit(am.Round, ACCUMULATOR_IS_ACTIVE_NAME, am.NumRows())
	am.Cols.AccumulatorCounter = comp.InsertCommit(am.Round, ACCUMULATOR_COUNTER_NAME, am.NumRows())
	am.Cols.IsFirst = comp.InsertCommit(am.Round, ACCUMULATOR_IS_FIRST_NAME, am.NumRows())
	am.Cols.IsInsert = comp.InsertCommit(am.Round, ACCUMULATOR_IS_INSERT_NAME, am.NumRows())
	am.Cols.IsDelete = comp.InsertCommit(am.Round, ACCUMULATOR_IS_DELETE_NAME, am.NumRows())
	am.Cols.IsUpdate = comp.InsertCommit(am.Round, ACCUMULATOR_IS_UPDATE_NAME, am.NumRows())
	am.Cols.IsReadZero = comp.InsertCommit(am.Round, ACCUMULATOR_IS_READ_ZERO_NAME, am.NumRows())
	am.Cols.IsReadNonZero = comp.InsertCommit(am.Round, ACCUMULATOR_IS_READ_NON_ZERO_NAME, am.NumRows())

	// columns for the sandwitch check
	am.Cols.HKey = comp.InsertCommit(am.Round, ACCUMULATOR_HKEY_NAME, am.NumRows())
	am.Cols.HKeyMinus = comp.InsertCommit(am.Round, ACCUMULATOR_HKEY_MINUS_NAME, am.NumRows())
	am.Cols.HKeyPlus = comp.InsertCommit(am.Round, ACCUMULATOR_HKEY_PLUS_NAME, am.NumRows())

	// columns for the pointer check
	am.Cols.LeafMinusIndex = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_MINUS_INDEX_NAME, am.NumRows())
	am.Cols.LeafMinusNext = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_MINUS_NEXT_NAME, am.NumRows())
	am.Cols.LeafPlusIndex = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_PLUS_INDEX_NAME, am.NumRows())
	am.Cols.LeafPlusPrev = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_PLUS_PREV_NAME, am.NumRows())
	am.Cols.LeafDeletedIndex = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_DELETED_INDEX_NAME, am.NumRows())
	am.Cols.LeafDeletedPrev = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_DELETED_PREV_NAME, am.NumRows())
	am.Cols.LeafDeletedNext = comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_DELETED_NEXT_NAME, am.NumRows())

	// Leaf hashing columns commitments
	am.commitLeafHashingCols()

	// NextFreeNode check columns commitments
	am.Cols.NextFreeNode = comp.InsertCommit(am.Round, ACCUMULATOR_NEXT_FREE_NODE_NAME, am.NumRows())
	am.Cols.InsertionPath = comp.InsertCommit(am.Round, ACCUMULATOR_INSERTION_PATH_NAME, am.NumRows())
	am.Cols.IsInsertRow3 = comp.InsertCommit(am.Round, ACCUMULATOR_IS_INSERT_ROW3_NAME, am.NumRows())

	// TopRoot hash check columns commitments
	am.Cols.IntermTopRoot = comp.InsertCommit(am.Round, ACCUMULATOR_INTERM_TOP_ROOT_NAME, am.NumRows())
	am.Cols.TopRoot = comp.InsertCommit(am.Round, ACCUMULATOR_TOP_ROOT_NAME, am.NumRows())

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

	// Sandwitch check for INSERT and READ-ZERO operations
	// We also check the consistency of the HKey, HKeyMinus, and HKeyPlus for INSERT and ReadZero operations.
	// i.e., they are consistent with the corresponding leaf opening values
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

func (am *Module) commitLeafHashingCols() {
	ACCUMULATOR_INTERM := make([]ifaces.ColID, 3)
	ACCUMULATOR_LEAF_OPENING_PREV := "ACCUMULATOR_LEAF_OPENING_PREV"
	ACCUMULATOR_LEAF_OPENING_NEXT := "ACCUMULATOR_LEAF_OPENING_NEXT"
	ACCUMULATOR_LEAF_OPENING_HKEY := "ACCUMULATOR_LEAF_OPENING_HKEY"
	ACCUMULATOR_LEAF_OPENING_HVAL := "ACCUMULATOR_LEAF_OPENING_HVAL"
	ACCUMULATOR_INTERM[0] = "ACCUMULATOR_INTERM_PREV"
	ACCUMULATOR_INTERM[1] = "ACCUMULATOR_INTERM_NEXT"
	ACCUMULATOR_INTERM[2] = "ACCUMULATOR_INTERM_HKEY"
	am.Cols.Interm = make([]ifaces.Column, 3)
	am.Cols.Zero = verifiercol.NewConstantCol(field.Zero(), am.NumRows(), "merkle-tree-accumulator")
	am.Cols.LeafOpenings.Prev = am.Comp.InsertCommit(am.Round, ifaces.ColID(ACCUMULATOR_LEAF_OPENING_PREV), am.NumRows())
	am.Cols.LeafOpenings.Next = am.Comp.InsertCommit(am.Round, ifaces.ColID(ACCUMULATOR_LEAF_OPENING_NEXT), am.NumRows())
	am.Cols.LeafOpenings.HKey = am.Comp.InsertCommit(am.Round, ifaces.ColID(ACCUMULATOR_LEAF_OPENING_HKEY), am.NumRows())
	am.Cols.LeafOpenings.HVal = am.Comp.InsertCommit(am.Round, ifaces.ColID(ACCUMULATOR_LEAF_OPENING_HVAL), am.NumRows())
	for i := 0; i < len(am.Cols.Interm); i++ {
		am.Cols.Interm[i] = am.Comp.InsertCommit(am.Round, ACCUMULATOR_INTERM[i], am.NumRows())
	}
	am.Cols.LeafHashes = am.Comp.InsertCommit(am.Round, ACCUMULATOR_LEAF_HASHES_NAME, am.NumRows())
	am.Cols.IsEmptyLeaf = am.Comp.InsertCommit(am.Round, ACCUMULATOR_IS_EMPTY_LEAF_NAME, am.NumRows())

}

func (am *Module) checkInsert() {
	cols := am.Cols

	// (Root[i+1] - Root[i+2]) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i] = 0, The (i+1)th and (i+2)th roots are equal when there is an INSERT operation and the accumulator is active.
	expr1 := symbolic.Sub(column.Shift(cols.Roots, 1), column.Shift(cols.Roots, 2))
	expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr1)
	am.Comp.InsertGlobal(am.Round, am.qname("ROOT_EQUALITY_INSERT_1"), expr1)

	// (Root[i+3] - Root[i+4]) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i] = 0, The (i+3)th and (i+4)th roots are equal when there is an INSERT operation and the accumulator is active.
	expr2 := symbolic.Sub(column.Shift(cols.Roots, 3), column.Shift(cols.Roots, 4))
	expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr2)
	am.Comp.InsertGlobal(am.Round, am.qname("ROOT_EQUALITY_INSERT_2"), expr2)

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

	// (Root[i+1] - Root[i+2]) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i] = 0, The (i+1)th and (i+2)th roots are equal when there is a DELETE operation and the accumulator is active.
	expr1 := symbolic.Sub(column.Shift(cols.Roots, 1), column.Shift(cols.Roots, 2))
	expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr1)
	am.Comp.InsertGlobal(am.Round, am.qname("ROOT_EQUALITY_DELETE_1"), expr1)

	// (Root[i+3] - Root[i+4]) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i] = 0, The (i+3)th and (i+4)th roots are equal when there is a DELETE operation and the accumulator is active.
	expr2 := symbolic.Sub(column.Shift(cols.Roots, 3), column.Shift(cols.Roots, 4))
	expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr2)
	am.Comp.InsertGlobal(am.Round, am.qname("ROOT_EQUALITY_DELETE_2"), expr2)

	// Booleanity of IsDelete
	expr3 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsDelete), cols.IsActiveAccumulator),
		cols.IsDelete)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_DELETE_BOOLEAN"), expr3)
}

func (am *Module) checkUpdate() {
	cols := am.Cols
	// HKey remains the same for an update operation, i.e,
	// IsActiveAccumulator[i] * IsUpdate[i] * IsFirst[i] * (HKey[i] - HKey[i+1])
	expr := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsUpdate,
		cols.IsFirst,
		symbolic.Sub(cols.HKey, column.Shift(cols.HKey, 1)))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_EQUAL_FOR_UPDATE"), expr)
}

func (am *Module) checkReadZero() {
	cols := am.Cols

	// (Root[i] - Root[i+1]) * IsActiveAccumulator[i] * IsFirst[i] * IsReadZero[i] = 0, The ith and (i+1)th roots are equal when there is a READ-ZERO operation and the accumulator is active.
	expr1 := symbolic.Sub(cols.Roots, column.Shift(cols.Roots, 1))
	expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsReadZero, expr1)
	am.Comp.InsertGlobal(am.Round, am.qname("ROOT_EQUALITY_READ_ZERO"), expr1)

	// Booleanity of IsReadZero
	expr2 := symbolic.Sub(
		symbolic.Mul(symbolic.Square(cols.IsReadZero), cols.IsActiveAccumulator),
		cols.IsReadZero)
	am.Comp.InsertGlobal(am.Round, am.qname("IS_READ_ZERO_BOOLEAN"), expr2)
}

func (am *Module) checkConsistency() {
	cols := am.Cols

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

	// Row-wise increment of AccumulatorCounter
	// IsActiveAccumulator[i+1] * (AccumulatorCounter[i+1] - AccumulatorCounter[i] - 1)
	expr4 := symbolic.Mul(column.Shift(cols.IsActiveAccumulator, 1),
		symbolic.Sub(column.Shift(cols.AccumulatorCounter, 1),
			cols.AccumulatorCounter, 1))
	am.Comp.InsertGlobal(am.Round, am.qname("COUNTER_INCREMENT"), expr4)
	// Local constraint that AccumulatorCounter starts at zero
	am.Comp.InsertLocal(am.Round, am.qname("COUNTER_LOCAL"), symbolic.Sub(cols.AccumulatorCounter, 0))

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
	emptyLeafBytes32 := types.Bytes32{}
	emptyLeafBytes := emptyLeafBytes32[:]
	var emptyLeafField field.Element
	if err := emptyLeafField.SetBytesCanonical(emptyLeafBytes[:]); err != nil {
		panic(err)
	}
	emptyLeaf := verifiercol.NewConstantCol(emptyLeafField, am.NumRows(), "accumulator-empty-leaves")
	cols := am.Cols

	// (Leaf[i+2] - emptyLeaf) * IsActiveAccumulator[i] * IsFirst[i] * IsInsert[i]
	expr1 := symbolic.Sub(column.Shift(cols.Leaves, 2), emptyLeaf)
	expr1 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsInsert, expr1)
	am.Comp.InsertGlobal(am.Round, am.qname("EMPTY_LEAVES_FOR_INSERT"), expr1)

	// (Leaf[i+3] - emptyLeaf) * IsActiveAccumulator[i] * IsFirst[i] * IsDelete[i]
	expr2 := symbolic.Sub(column.Shift(cols.Leaves, 3), emptyLeaf)
	expr2 = symbolic.Mul(cols.IsActiveAccumulator, cols.IsFirst, cols.IsDelete, expr2)
	am.Comp.InsertGlobal(am.Round, am.qname("EMPTY_LEAVES_FOR_DELETE"), expr2)
}

func (am *Module) checkSandwitch() {
	cols := am.Cols
	// We want sandwitch check only at row 1 of INSERT and READ-ZERO
	activeRow := symbolic.Add(symbolic.Mul(cols.IsFirst, cols.IsInsert), symbolic.Mul(cols.IsFirst, cols.IsReadZero))
	byte32cmp.Bytes32Cmp(am.Comp, 16, 16, string(am.qname("CMP_HKEY_HKEY_MINUS")), am.Cols.HKey, am.Cols.HKeyMinus, activeRow)
	byte32cmp.Bytes32Cmp(am.Comp, 16, 16, string(am.qname("CMP_HKEY_PLUS_HKEY")), am.Cols.HKeyPlus, am.Cols.HKey, activeRow)

	// INSERT: The HKeyMinus in the leaf minus openings is the same as HKeyMinus column i.e.,
	// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKeyMinus[i] - LeafOpenings.Hkey[i])
	expr1 := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsInsert,
		cols.IsFirst,
		symbolic.Sub(cols.HKeyMinus, cols.LeafOpenings.HKey))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_MINUS_CONSISTENCY_INSERT"), expr1)

	// INSERT: The HKey in the inserted leaf openings (in the fourth row) is the same as HKey column i.e.,
	// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKey[i] - LeafOpenings.Hkey[i+3])
	expr2 := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsInsert,
		cols.IsFirst,
		symbolic.Sub(cols.HKey, column.Shift(cols.LeafOpenings.HKey, 3)))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_CONSISTENCY_INSERT"), expr2)

	// INSERT: The HKeyPlus in the plus leaf openings is the same as HKeyPlus column i.e.,
	// IsActiveAccumulator[i] * IsInsert[i] * IsFirst[i] * (HKeyPlus[i] - LeafOpenings.Hkey[i+4])
	expr3 := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsInsert,
		cols.IsFirst,
		symbolic.Sub(cols.HKeyPlus, column.Shift(cols.LeafOpenings.HKey, 4)))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_PLUS_CONSISTENCY_INSERT"), expr3)

	// READ-ZERO: The HKeyMinus in the minus leaf openings is the same as HKeyMinus column i.e.,
	// IsActiveAccumulator[i] * IsReadZero[i] * IsFirst[i] * (HKeyMinus[i] - LeafOpenings.Hkey[i])
	expr4 := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsReadZero,
		cols.IsFirst,
		symbolic.Sub(cols.HKeyMinus, cols.LeafOpenings.HKey))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_MINUS_CONSISTENCY_READ_ZERO"), expr4)

	// READ-ZERO: The HKeyPlus in the plus leaf openings is the same as HKeyPlus column i.e.,
	// IsActiveAccumulator[i] * IsReadZero[i] * IsFirst[i] * (HKeyPlus[i] - LeafOpenings.Hkey[i+1])
	expr5 := symbolic.Mul(cols.IsActiveAccumulator,
		cols.IsReadZero,
		cols.IsFirst,
		symbolic.Sub(cols.HKeyPlus, column.Shift(cols.LeafOpenings.HKey, 1)))
	am.Comp.InsertGlobal(am.Round, am.qname("HKEY_PLUS_CONSISTENCY_READ_ZERO"), expr5)
}

func (am *Module) checkPointer() {
	cols := am.Cols
	// Check #1 for INSERT: IsFirst[i] * IsInsert[i] * (LeafMinusNext[i] - LeafPlusIndex[i])
	expr1 := symbolic.Mul(cols.IsFirst, cols.IsInsert, symbolic.Sub(cols.LeafMinusNext, cols.LeafPlusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("INSERT_POINTER_1"), expr1)

	// Check #2 for INSERT: IsFirst[i] * IsInsert[i] *(LeafPlusPrev[i] - LeafMinusIndex[i])
	expr2 := symbolic.Mul(cols.IsFirst, cols.IsInsert, symbolic.Sub(cols.LeafPlusPrev, cols.LeafMinusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("INSERT_POINTER_2"), expr2)

	// Check #1 for DELETE: IsFirst[i] * IsDelete[i] * (LeafMinusNext[i] - LeafDeletedIndex[i])
	expr3 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafMinusNext, cols.LeafDeletedIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("DELETE_POINTER_1"), expr3)

	// Check #2 for DELETE: IsFirst[i] * IsDelete[i] * (LeafDeletedPrev[i] - LeafMinusIndex[i])
	expr4 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafDeletedPrev, cols.LeafMinusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("DELETE_POINTER_2"), expr4)

	// Check #3 for DELETE: IsFirst[i] * IsDelete[i] * (LeafDeletedNext[i] - LeafPlusIndex[i])
	expr5 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafDeletedNext, cols.LeafPlusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("DELETE_POINTER_3"), expr5)

	// Check #4 for DELETE: IsFirst[i] * IsDelete[i] * (LeafPlusPrev[i] - LeafDeletedIndex[i])
	expr6 := symbolic.Mul(cols.IsFirst, cols.IsDelete, symbolic.Sub(cols.LeafPlusPrev, cols.LeafDeletedIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("DELETE_POINTER_4"), expr6)

	// Check #1 for READ-ZERO: IsFirst[i] * IsReadZero[i] * (LeafMinusNext[i] - LeafPlusIndex[i])
	expr7 := symbolic.Mul(cols.IsFirst, cols.IsReadZero, symbolic.Sub(cols.LeafMinusNext, cols.LeafPlusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("READ_ZERO_POINTER_1"), expr7)

	// Check #2 for READ-ZERO: IsFirst[i] * IsReadZero[i] * (LeafPlusPrev[i] - LeafMinusIndex[i])
	expr8 := symbolic.Mul(cols.IsFirst, cols.IsReadZero, symbolic.Sub(cols.LeafPlusPrev, cols.LeafMinusIndex))
	am.Comp.InsertGlobal(am.Round, am.qname("READ_ZERO_POINTER_2"), expr8)
}

func (am *Module) checkLeafHashes() {
	cols := am.Cols
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_PREV"), cols.LeafOpenings.Prev, cols.Zero, cols.Interm[0], nil)
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_NEXT"), cols.LeafOpenings.Next, cols.Interm[0], cols.Interm[1], nil)
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_HKEY"), cols.LeafOpenings.HKey, cols.Interm[1], cols.Interm[2], nil)
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_HVAL_LEAF"), cols.LeafOpenings.HVal, cols.Interm[2], cols.LeafHashes, nil)
	// Global: IsActive[i] * (1 - IsEmptyLeaf[i]) * (Leaves[i] - LeafHashes[i])
	expr1 := symbolic.Sub(cols.Leaves, cols.LeafHashes)
	expr2 := symbolic.Sub(symbolic.NewConstant(1), cols.IsEmptyLeaf)
	expr3 := symbolic.Mul(cols.IsActiveAccumulator, expr1, expr2)
	am.Comp.InsertGlobal(am.Round, am.qname("LEAF_HASH_EQUALITY"), expr3)
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
	expr1 := symbolic.Mul(cols.IsInsertRow3,
		symbolic.Sub(cols.NextFreeNode, column.Shift(cols.NextFreeNode, -1), symbolic.NewConstant(1)))
	expr2 := symbolic.Mul(symbolic.Sub(symbolic.NewConstant(1), cols.IsInsertRow3),
		symbolic.Sub(cols.NextFreeNode, column.Shift(cols.NextFreeNode, -1)))
	expr3 := symbolic.Mul(cols.IsActiveAccumulator,
		symbolic.Sub(symbolic.NewConstant(1), cols.IsFirst),
		symbolic.Add(expr1, expr2))
	am.Comp.InsertGlobal(am.Round, am.qname("NEXT_FREE_NODE_CONSISTENCY_1"), expr3)

	// IsActive[i] * (1 - IsFirst[i]) * IsInsertRow3[i] * (NextFreeNode[i] - InsertionPath[i] - 1)
	expr4 := symbolic.Mul(cols.IsActiveAccumulator,
		symbolic.Sub(symbolic.NewConstant(1), cols.IsFirst),
		cols.IsInsertRow3,
		symbolic.Sub(cols.NextFreeNode, cols.InsertionPath, symbolic.NewConstant(1)))
	am.Comp.InsertGlobal(am.Round, am.qname("NEXT_FREE_NODE_CONSISTENCY_2"), expr4)

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
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_INTERM_TOP_ROOT"), cols.NextFreeNode, cols.Zero, cols.IntermTopRoot, nil)
	am.Comp.InsertMiMC(am.Round, am.qname("MIMC_TOP_ROOT"), cols.Roots, cols.IntermTopRoot, cols.TopRoot, nil)
}

func (am *Module) checkZeroInInactive() {
	cols := am.Cols
	am.colZeroAtInactive(cols.Leaves, "LEAVES_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.Roots, "ROOTS_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.Positions, "POSITIONS_ZERO_IN_INACTIVE")
	// Skipping proof as it has unequal column length with IsActive
	// proof is unconstrained in this module, and the consistency check is done
	// in the Merkle module
	am.colZeroAtInactive(cols.UseNextMerkleProof, "USE_NEXT_MERKLE_PROOF_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.AccumulatorCounter, "ACCUMULATOR_COUNTER_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsFirst, "IS_FIRST_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsInsert, "IS_INSERT_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsDelete, "IS_DELETE_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsUpdate, "IS_UPDATE_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsReadZero, "IS_READ_ZERO_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsReadNonZero, "IS_READ_NON_ZERO_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.HKey, "HKEY_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.HKeyMinus, "HKEY_MINUS_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.HKeyPlus, "HKEY_PLUS_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafMinusIndex, "LEAF_MINUS_INDEX_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafMinusNext, "LEAF_MINUS_NEXT_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafPlusIndex, "LEAF_PLUS_INDEX_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafPlusPrev, "LEAF_PLUS_PREV_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafDeletedIndex, "LEAF_DELETED_INDEX_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafDeletedPrev, "LEAF_DELETED_PREV_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafDeletedNext, "LEAF_DELETED_NEXT_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafOpenings.HKey, "LEAF_OPENING_HKEY_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafOpenings.HVal, "LEAF_OPENING_HVAL_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafOpenings.Prev, "LEAF_OPENING_PREV_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.LeafOpenings.Next, "LEAF_OPENING_NEXT_ZERO_IN_INACTIVE")
	// Skipping Interm, Zero, and LeafHashes as two of them contain zero hashes and
	// Zero is a verifier column. The padding area of Interm and LeafHashes
	// are already constrained by the MiMC query
	am.colZeroAtInactive(cols.IsEmptyLeaf, "IS_EMPTY_LEAF_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.NextFreeNode, "NEXT_FREE_NODE_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.InsertionPath, "INSERTION_PATH_ZERO_IN_INACTIVE")
	am.colZeroAtInactive(cols.IsInsertRow3, "IS_INSERT_ROW3_ZERO_IN_INACTIVE")
	// Again skipping IntermTopRoot and TopRoot as they contain zero hashes
}

// Function returning a query name
func (am *Module) qname(name string) ifaces.QueryID {
	return ifaces.QueryIDf("%s_%d", am.Name, am.Comp.SelfRecursionCount) + "_" + ifaces.QueryID(name)
}

// Function inserting a query that col is zero when IsActive is zero
func (am *Module) colZeroAtInactive(col ifaces.Column, name string) {
	// col zero at inactive area, e.g., (1-IsActiveAccumulator[i]) * col[i] = 0
	am.Comp.InsertGlobal(am.Round, am.qname(name),
		symbolic.Mul(symbolic.Sub(1, am.Cols.IsActiveAccumulator), col))
}
