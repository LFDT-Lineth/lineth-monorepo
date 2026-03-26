package accumulator

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/utils"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo

	"github.com/consensys/linea-monorepo/prover/utils/types"
	. "github.com/consensys/linea-monorepo/prover/utils/types"
)

// DeletetionTrace gathers all the data necessary to audit the deletion of a
// key in the map.
type DeletionTrace[K, V io.WriterTo] struct {
	// Identifier for the tree this trace relates to
	Type     int    `json:"type"`
	Location string `json:"location"`

	// For consistency, we call it new next free node but
	// the value is not updated during a deletion
	NewNextFreeNode int           `json:"newNextFreeNode"`
	OldSubRoot      KoalaOctuplet `json:"oldSubRoot"`
	NewSubRoot      KoalaOctuplet `json:"newSubRoot"`

	// `New` correspond to the inserted leaf
	ProofMinus   smt_koalabear.Proof `json:"leftProof"`
	ProofDeleted smt_koalabear.Proof `json:"deletedProof"`
	ProofPlus    smt_koalabear.Proof `json:"rightProof"`
	Key          K                   `json:"key"`

	// Value of the leaf opening before being modified
	OldOpenMinus LeafOpening `json:"priorLeftLeaf"`
	DeletedOpen  LeafOpening `json:"priorDeletedLeaf"`
	OldOpenPlus  LeafOpening `json:"priorRightLeaf"`

	// The deleted value
	DeletedValue V `json:"deletedValue"`
}

// DeleteAndProve deletes an entry in the accumulator and returns a
// DeletionTrace, the function will panic on failure: if the key could not
// be found or if the Tree is corrupted.
func (p *ProverState[K, V]) DeleteAndProve(key K) (trace DeletionTrace[K, V]) {

	// Sanity-check : assert that the key is missing in the proof
	i, found := p.FindKey(key)
	if !found {
		utils.Panic("called delete, but the key was not present")
	}

	// No need to look for the sandwich, we can find it in the leafopening
	tuple := p.Data.MustGet(i)
	iMinus, iPlus := tuple.LeafOpening.Prev, tuple.LeafOpening.Next
	tupleMinus := p.Data.MustGet(iMinus)
	tuplePlus := p.Data.MustGet(iPlus)

	trace = DeletionTrace[K, V]{
		Location:        p.Location,
		Key:             key,
		OldSubRoot:      p.SubTreeRoot(),
		OldOpenMinus:    tupleMinus.LeafOpening,
		OldOpenPlus:     tuplePlus.LeafOpening,
		DeletedOpen:     tuple.LeafOpening,
		DeletedValue:    tuple.Value,
		NewNextFreeNode: int(p.NextFreeNode),
	}

	// 1/ The Prev
	newTupleMinus := tupleMinus.CopyWithNext(iPlus)
	trace.ProofMinus = p.upsertTuple(iMinus, newTupleMinus)

	// 2/ Delete the target leaf
	trace.ProofDeleted = p.rmTuple(i)

	// 3/ The next
	newTuplePlus := tuplePlus.CopyWithPrev(iMinus)
	trace.ProofPlus = p.upsertTuple(iPlus, newTuplePlus)

	trace.NewSubRoot = p.SubTreeRoot()
	return trace
}

// VerifyDeletion audits the validity of a [DeletionTrace] w.r.t. to the
// VerifierState.
func (v *VerifierState[K, V]) VerifyDeletion(trace DeletionTrace[K, V]) error {

	// If the location does not match the we return an error
	if v.Location != trace.Location {
		return fmt.Errorf("inconsistent location : %v != %v", v.Location, trace.Location)
	}

	// Check that verifier's root is the same as the one in the traces
	if v.SubTreeRoot != trace.OldSubRoot {
		return fmt.Errorf("inconsistent root %v != %v", v.SubTreeRoot.Hex(), trace.OldSubRoot.Hex())
	}

	// Check that the deleted value is consistent with the leaf opening
	hVal := hash(trace.DeletedValue)
	if hVal != trace.DeletedOpen.HVal {
		return fmt.Errorf("the deleted value does not match the hVal of the opening")
	}

	iMinus := int64(trace.ProofMinus.Path)
	iDeleted := int64(trace.ProofDeleted.Path)
	iPlus := int64(trace.ProofPlus.Path)

	// Check that minus and the deleted branch point to each other
	if (trace.OldOpenMinus.Next != iDeleted) || (trace.DeletedOpen.Prev != iMinus) {
		return fmt.Errorf(
			"bad sandwich prev and next do not point to each other : prev %++v, next %++v",
			trace.OldOpenMinus, trace.DeletedOpen,
		)
	}

	// Check that the two sandwich leaf opening point to each other
	if (trace.DeletedOpen.Next != iPlus) || (trace.OldOpenPlus.Prev != iDeleted) {
		return fmt.Errorf(
			"bad sandwich prev and next do not point to each other : prev %++v, next %++v",
			trace.DeletedOpen, trace.OldOpenPlus,
		)
	}

	// Check that the deleted entry corresponds to the key we wish to remove
	if !trace.DeletedOpen.MatchKey(trace.Key) {
		return fmt.Errorf("deleting the wrong leaf : does not match our key : trace.Key %v - hkey %v", trace.Key, trace.DeletedOpen.HKey)
	}

	currentRoot := trace.OldSubRoot

	// Audit the update of the "minus"
	oldLeafMinus := trace.OldOpenMinus.Hash()
	newLeafMinus := trace.OldOpenMinus.CopyWithNext(iPlus).Hash()
	currentRoot, err := updateCheckRoot(trace.ProofMinus, currentRoot, oldLeafMinus, newLeafMinus)
	if err != nil {
		return fmt.Errorf("audit of the update of old leaf minus failed %v", err)
	}

	// Audit the update of the deleted leaf
	deletedLeaf := hash(&trace.DeletedOpen)
	currentRoot, err = updateCheckRoot(trace.ProofDeleted, currentRoot, deletedLeaf, types.KoalaOctuplet(smt_koalabear.EmptyLeaf()))
	if err != nil {
		return fmt.Errorf("audit of the update of the middle leaf failed %v", err)
	}

	// Audit the update of the "plus"
	oldLeafPlus := trace.OldOpenPlus.Hash()
	newLeafPlus := trace.OldOpenPlus.CopyWithPrev(iMinus).Hash()
	currentRoot, err = updateCheckRoot(trace.ProofPlus, currentRoot, oldLeafPlus, newLeafPlus)
	if err != nil {
		return fmt.Errorf("audit of the update of old leaf plus failed %v", err)
	}

	// Check that the alleged new root is consistent with the one we reconstructed
	if currentRoot != trace.NewSubRoot {
		return fmt.Errorf("inconsistent root %v != %v", currentRoot.Hex(), trace.NewSubRoot.Hex())
	}

	// Check that the next free node is consistent with the prover and the verifier
	if v.NextFreeNode != int64(trace.NewNextFreeNode) {
		return fmt.Errorf("inconsistent NextFreeNode %v != %v", v.NextFreeNode, trace.NewNextFreeNode)
	}

	// Every check passed : update the verifier state
	v.SubTreeRoot = currentRoot
	return nil
}

// DeferMerkleChecks implements [Trace]
func (trace DeletionTrace[K, V]) DeferMerkleChecks(
	appendTo []smt_koalabear.ProvedClaim,
) []smt_koalabear.ProvedClaim {
	currentRoot := trace.OldSubRoot
	iMinus := int64(trace.ProofMinus.Path)
	iPlus := int64(trace.ProofPlus.Path)

	// Audit the update of the "minus"
	oldLeafMinus := trace.OldOpenMinus.Hash()
	newLeafMinus := trace.OldOpenMinus.CopyWithNext(iPlus).Hash()
	appendTo, currentRoot = deferCheckUpdateRoot(trace.ProofMinus, currentRoot, oldLeafMinus, newLeafMinus, appendTo)

	// the proof verification for the deleted leaf
	deletedLeaf := hash(&trace.DeletedOpen)
	appendTo, currentRoot = deferCheckUpdateRoot(trace.ProofDeleted, currentRoot, deletedLeaf, types.KoalaOctuplet(smt_koalabear.EmptyLeaf()), appendTo)

	// Audit the update of the "plus"
	oldLeafPlus := trace.OldOpenPlus.Hash()
	newLeafPlus := trace.OldOpenPlus.CopyWithPrev(iMinus).Hash()
	appendTo, _ = deferCheckUpdateRoot(trace.ProofPlus, currentRoot, oldLeafPlus, newLeafPlus, appendTo)

	return appendTo
}

func (trace DeletionTrace[K, V]) HKey() KoalaOctuplet {
	return trace.DeletedOpen.HKey
}

func (trace DeletionTrace[K, V]) RWInt() int {
	return 1
}
