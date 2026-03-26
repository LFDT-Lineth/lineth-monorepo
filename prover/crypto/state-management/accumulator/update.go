package accumulator

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo

	. "github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/pkg/errors"
)

// UpdateTrace contains all the necessary informations to carry an audit of an
// update of the value of a registered key in the tree.
type UpdateTrace[K, V io.WriterTo] struct {
	Type     int    `json:"type"`
	Location string `json:"location"`
	Key      K      `json:"key"`
	OldValue V      `json:"oldValue"`
	NewValue V      `json:"newValue"`
	// We call it new next free node, but the value is not updated
	// during the update.
	NewNextFreeNode int                 `json:"newNextFreeNode"`
	OldSubRoot      KoalaOctuplet       `json:"oldSubRoot"`
	NewSubRoot      KoalaOctuplet       `json:"newSubRoot"`
	OldOpening      LeafOpening         `json:"priorUpdatedLeaf"`
	Proof           smt_koalabear.Proof `json:"proof"`
}

// UpdateAndProve performs a read on the accumulator. Panics if the associated
// key is missing. Returns an [UpdateTrace] object in case of success.
func (p *ProverState[K, V]) UpdateAndProve(key K, newVal V) UpdateTrace[K, V] {

	// Find the position of the leaf containing our value
	i, found := p.FindKey(key)
	if !found {
		utils.Panic("called update, but the key was not present")
	}

	tuple := p.Data.MustGet(i)

	if hash(key) != hash(tuple.Key) {
		utils.Panic("sanity-check : the key mismatched")
	}

	oldRoot := p.SubTreeRoot()
	oldOpening := tuple.LeafOpening
	oldValue := tuple.Value

	// Compute the new value and update the tree
	tuple.Value = newVal
	tuple.LeafOpening.HVal = hash(tuple.Value)
	p.Data.Update(i, tuple)

	newLeaf := tuple.LeafOpening.Hash()
	p.Tree.Update(int(i), field.Octuplet(newLeaf))

	return UpdateTrace[K, V]{
		Location:        p.Location,
		Key:             tuple.Key,
		OldValue:        oldValue,
		NewValue:        newVal,
		OldOpening:      oldOpening,
		OldSubRoot:      oldRoot,
		NewSubRoot:      p.SubTreeRoot(),
		NewNextFreeNode: int(p.NextFreeNode),
		Proof:           p.Tree.MustProve(int(i)),
	}
}

// UpdateVerify verifies an [UpdateTrace] against the verifier state. Returns
// an error if the verification fails.
func (v *VerifierState[K, V]) UpdateVerify(trace UpdateTrace[K, V]) error {

	// If the location does not match the we return an error
	if v.Location != trace.Location {
		return fmt.Errorf("inconsistent location : %v != %v", v.Location, trace.Location)
	}

	// Check that verifier's root is the same as the one in the traces
	if v.SubTreeRoot != trace.OldSubRoot {
		return fmt.Errorf("inconsistent root %v != %v", v.SubTreeRoot.Hex(), trace.OldSubRoot.Hex())
	}

	tuple := KVOpeningTuple[K, V]{
		Key:         trace.Key,
		Value:       trace.OldValue,
		LeafOpening: trace.OldOpening,
	}

	leaf, err := tuple.CheckAndLeaf()
	if err != nil {
		return errors.WithMessage(err, "read update verifier failed")
	}

	if smt_koalabear.Verify(&trace.Proof, field.Octuplet(leaf), field.Octuplet(trace.OldSubRoot)) != nil {
		return fmt.Errorf("merkle proof verification failed")
	}

	newTuple := tuple
	newTuple.LeafOpening.HVal = hash(trace.NewValue)

	// We panic because if the consistency check passed
	newLeaf := hash(&newTuple.LeafOpening)

	newRoot, err := updateCheckRoot(trace.Proof, trace.OldSubRoot, leaf, newLeaf)
	if err != nil {
		return errors.Wrap(err, "update check failed : invalid proof")
	}

	if newRoot != trace.NewSubRoot {
		return errors.New("new root does not match")
	}

	// Check that NextFreeNode for the Prover and the Verifier are the same
	if v.NextFreeNode != int64(trace.NewNextFreeNode) {
		return fmt.Errorf("inconsistent NextFreeNode %v != %v", v.NextFreeNode, trace.NewNextFreeNode)
	}

	// Update the verifier's view
	v.SubTreeRoot = trace.NewSubRoot
	return nil
}

// DeferMerkleChecks implements the [Trace] interface.
func (trace UpdateTrace[K, V]) DeferMerkleChecks(
	appendTo []smt_koalabear.ProvedClaim,
) []smt_koalabear.ProvedClaim {

	tuple := KVOpeningTuple[K, V]{
		Key:         trace.Key,
		Value:       trace.OldValue,
		LeafOpening: trace.OldOpening,
	}

	leaf, _ := tuple.CheckAndLeaf()

	newTuple := tuple
	newTuple.LeafOpening.HVal = hash(trace.NewValue)

	// We panic because if the consistency check passed
	newLeaf := hash(&newTuple.LeafOpening)
	appendTo, _ = deferCheckUpdateRoot(trace.Proof, trace.OldSubRoot, leaf, newLeaf, appendTo)
	return appendTo
}

func (trace UpdateTrace[K, V]) HKey() KoalaOctuplet {
	return hash(trace.Key)
}

func (trace UpdateTrace[K, V]) RWInt() int {
	return 1
}
