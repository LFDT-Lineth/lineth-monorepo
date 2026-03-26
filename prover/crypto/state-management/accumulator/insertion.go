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

// InsertionTrace gathers all the input needed for a verifier to audit the
// insertion of a key in the map.
type InsertionTrace[K, V io.WriterTo] struct {
	// Identifier for the tree this trace belongs to
	Type     int    `json:"type"`
	Location string `json:"location"`

	NewNextFreeNode int           `json:"newNextFreeNode"`
	OldSubRoot      KoalaOctuplet `json:"oldSubRoot"`
	NewSubRoot      KoalaOctuplet `json:"newSubRoot"`

	// `New` correspond to the inserted leaf
	ProofMinus smt_koalabear.Proof `json:"leftProof"`
	ProofNew   smt_koalabear.Proof `json:"newProof"`
	ProofPlus  smt_koalabear.Proof `json:"rightProof"`
	Key        K                   `json:"key"`
	Val        V                   `json:"value"`

	// Value of the leaf opening before being modified
	OldOpenMinus LeafOpening `json:"priorLeftLeaf"`
	OldOpenPlus  LeafOpening `json:"priorRightLeaf"`
}

// InsertAndProve inserts in the accumulator and returns a trace. The function
// panics if the key is already in the accumulator or if the tree is corrupted.
func (p *ProverState[K, V]) InsertAndProve(key K, val V) (trace InsertionTrace[K, V]) {

	// Sanity-check : assert that the key is missing in the proof
	_posFound, found := p.FindKey(key)
	if found {
		utils.Panic("called insert, but the key was present : %v as position %v", key, _posFound)
	}

	// Fetch the leaf openings and add them in the trace
	iMinus, iPlus := p.findSandwich(key)
	tupleMinus := p.Data.MustGet(iMinus)
	tuplePlus := p.Data.MustGet(iPlus)

	trace = InsertionTrace[K, V]{
		Location: p.Location,
		Key:      key, Val: val, OldSubRoot: p.SubTreeRoot(),
		OldOpenMinus:    tupleMinus.LeafOpening,
		OldOpenPlus:     tuplePlus.LeafOpening,
		NewNextFreeNode: int(p.NextFreeNode) + 1,
	}

	// Progressively update and prove each leaf opening
	iInserted := p.NextFreeNode

	// 1/ The Prev
	newTupleMinus := tupleMinus.CopyWithNext(int64(iInserted))
	trace.ProofMinus = p.upsertTuple(iMinus, newTupleMinus)

	// 2/ The `New` (i.e) the inserted leaf
	insertedTuple := KVOpeningTuple[K, V]{
		Key: key, Value: val,
		LeafOpening: LeafOpening{
			Prev: int64(iMinus),
			Next: int64(iPlus),
			HKey: hash(key),
			HVal: hash(val),
		},
	}
	trace.ProofNew = p.upsertTuple(iInserted, insertedTuple)

	// 3/ The next
	newTuplePlus := tuplePlus.CopyWithPrev(int64(iInserted))
	trace.ProofPlus = p.upsertTuple(iPlus, newTuplePlus)

	// Fetch the root, and we are done
	p.NextFreeNode++ // And increment the next free node counter
	trace.NewSubRoot = p.SubTreeRoot()
	return trace
}

// VerifyInsertion audit the insertion of an entry in the accumulator w.r.t to
// the state of the verifier. It returns an error if the verification failed.
func (v *VerifierState[K, V]) VerifyInsertion(trace InsertionTrace[K, V]) error {

	// If the location does not match the we return an error
	if v.Location != trace.Location {
		return fmt.Errorf("inconsistent location : %v != %v", v.Location, trace.Location)
	}

	// Check that verifier's root is the same as the one in the traces
	if v.SubTreeRoot != trace.OldSubRoot {
		return fmt.Errorf("inconsistent root %v != %v", v.SubTreeRoot.Hex(), trace.OldSubRoot.Hex())
	}

	iMinus := int64(trace.ProofMinus.Path)
	iInserted := int64(trace.ProofNew.Path)
	iPlus := int64(trace.ProofPlus.Path)

	// Alledgedly, we write in the leaf #nextFreeNode
	if iInserted != v.NextFreeNode {
		return fmt.Errorf(
			"writing in the wrong place : %v (expected %v)",
			iInserted, v.NextFreeNode,
		)
	}

	// Check that the two sandwich leaf opening point to each other
	if trace.OldOpenMinus.Next != iPlus || trace.OldOpenPlus.Prev != iMinus {
		return fmt.Errorf(
			"bad sandwich prev and next do not point to each other : prev %++v, next %++v",
			trace.OldOpenMinus, trace.OldOpenPlus,
		)
	}

	// Also checks that the their hkey are lower/larger than the inserted one
	hkey := hash(trace.Key)
	if hkey.Cmp(trace.OldOpenMinus.HKey) < 1 || hkey.Cmp(trace.OldOpenPlus.HKey) > -1 {
		return fmt.Errorf(
			"sandwich is incorrect expected %x < %x < %x",
			trace.OldOpenMinus.HKey, hkey, trace.OldOpenPlus.HKey,
		)
	}

	currentRoot := trace.OldSubRoot

	// Audit the update of the "minus"
	oldLeafMinus := trace.OldOpenMinus.Hash()
	newLeafMinus := trace.OldOpenMinus.CopyWithNext(v.NextFreeNode).Hash()

	currentRoot, err := updateCheckRoot(trace.ProofMinus, currentRoot, oldLeafMinus, newLeafMinus)
	if err != nil {
		return fmt.Errorf("audit of the update of oldLeafMinus failed %v", err)
	}

	// Audit the update of the inserted new leaf
	newLeaf := LeafOpening{
		Prev: int64(trace.ProofMinus.Path),
		Next: int64(trace.ProofPlus.Path),
		HKey: hkey,
		HVal: hash(trace.Val),
	}.Hash()

	currentRoot, err = updateCheckRoot(trace.ProofNew, currentRoot, types.KoalaOctuplet(smt_koalabear.EmptyLeaf()), newLeaf)
	if err != nil {
		return fmt.Errorf("audit of the update of the middle leaf failed %v", err)
	}

	// Audit the update of the "plus"
	oldLeafPlus := trace.OldOpenPlus.Hash()
	newLeafPlus := trace.OldOpenPlus.CopyWithPrev(iInserted).Hash()

	currentRoot, err = updateCheckRoot(trace.ProofPlus, currentRoot, oldLeafPlus, newLeafPlus)
	if err != nil {
		return fmt.Errorf("audit of the update of oldLeafPlus failed %v", err)
	}

	// Check that the alleged new root is consistent with the one we reconstructed
	if currentRoot != trace.NewSubRoot {
		return fmt.Errorf("inconsistent root %v != %v", currentRoot.Hex(), trace.NewSubRoot.Hex())
	}

	// Every check passed : update the verifier state
	v.SubTreeRoot = currentRoot
	v.NextFreeNode++
	// Check that the next free node matches with the prover
	if v.NextFreeNode != int64(trace.NewNextFreeNode) {
		return fmt.Errorf("inconsistent next free node %v != %v", v.NextFreeNode, trace.NewNextFreeNode)
	}
	return nil
}

// DeferMerkleChecks implements [Trace]
func (trace InsertionTrace[K, V]) DeferMerkleChecks(
	appendTo []smt_koalabear.ProvedClaim,
) []smt_koalabear.ProvedClaim {

	iInserted := int64(trace.ProofNew.Path)

	// Also checks that the their hkey are lower/larger than the inserted one
	hkey := hash(trace.Key)

	currentRoot := trace.OldSubRoot

	// Audit the update of the "minus"
	oldLeafMinus := trace.OldOpenMinus.Hash()
	newLeafMinus := trace.OldOpenMinus.CopyWithNext(int64(trace.ProofNew.Path)).Hash()

	appendTo, currentRoot = deferCheckUpdateRoot(trace.ProofMinus, currentRoot, oldLeafMinus, newLeafMinus, appendTo)

	// Audit the update of the inserted new leaf
	newLeaf := LeafOpening{
		Prev: int64(trace.ProofMinus.Path),
		Next: int64(trace.ProofPlus.Path),
		HKey: hkey,
		HVal: hash(trace.Val),
	}.Hash()

	appendTo, currentRoot = deferCheckUpdateRoot(trace.ProofNew, currentRoot, types.KoalaOctuplet(smt_koalabear.EmptyLeaf()), newLeaf, appendTo)

	// Audit the update of the "plus"
	oldLeafPlus := trace.OldOpenPlus.Hash()
	newLeafPlus := trace.OldOpenPlus.CopyWithPrev(iInserted).Hash()

	appendTo, _ = deferCheckUpdateRoot(trace.ProofPlus, currentRoot, oldLeafPlus, newLeafPlus, appendTo)
	return appendTo
}

func (trace InsertionTrace[K, V]) HKey() KoalaOctuplet {
	return hash(trace.Key)
}

func (trace InsertionTrace[K, V]) RWInt() int {
	return 1
}
