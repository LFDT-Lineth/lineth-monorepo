package accumulator

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/utils"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo

	. "github.com/consensys/linea-monorepo/prover/utils/types"
)

// Trace that allows checking a read zero operation: e.g. proof of non-membership
type ReadZeroTrace[K, V io.WriterTo] struct {
	Type         int                 `json:"type"`
	Location     string              `json:"location"`
	Key          K                   `json:"key"`
	SubRoot      KoalaOctuplet       `json:"subRoot"`
	NextFreeNode int                 `json:"nextFreeNode"`
	OpeningMinus LeafOpening         `json:"leftLeaf"`
	OpeningPlus  LeafOpening         `json:"rightLeaf"`
	ProofMinus   smt_koalabear.Proof `json:"leftProof"`
	ProofPlus    smt_koalabear.Proof `json:"rightProof"`
}

// ReadZeroAndProve performs a read-zero on the accumulator. Panics if the
// associated key exists in the tree. Returns a ReadZeroTrace object in case of
// success.
func (p *ProverState[K, V]) ReadZeroAndProve(key K) ReadZeroTrace[K, V] {

	// Find the position of the leaf containing our value
	_, found := p.FindKey(key)
	if found {
		utils.Panic("called read-zero, but the key was present")
	}

	iMinus, iPlus := p.findSandwich(key)
	dataMinus := p.Data.MustGet(iMinus)
	dataPlus := p.Data.MustGet(iPlus)

	return ReadZeroTrace[K, V]{
		Location:     p.Location,
		Key:          key,
		SubRoot:      p.SubTreeRoot(),
		ProofMinus:   p.Tree.MustProve(int(iMinus)),
		OpeningMinus: dataMinus.LeafOpening,
		ProofPlus:    p.Tree.MustProve(int(iPlus)),
		OpeningPlus:  dataPlus.LeafOpening,
		NextFreeNode: int(p.NextFreeNode),
	}
}

// ReadZeroVerify verifies a [ReadZeroTrace] and returns an error in case of
// failure.
func (v *VerifierState[K, V]) ReadZeroVerify(trace ReadZeroTrace[K, V]) error {

	// If the location does not match the we return an error
	if v.Location != trace.Location {
		return fmt.Errorf("inconsistent location : %v != %v", v.Location, trace.Location)
	}

	// Check that verifier's root is the same as the one in the traces
	if v.SubTreeRoot != trace.SubRoot {
		return fmt.Errorf("inconsistent root %v != %v", v.SubTreeRoot.Hex(), trace.SubRoot.Hex())
	}

	iMinus, iPlus := int64(trace.ProofMinus.Path), int64(trace.ProofPlus.Path)

	// Check that Plus and Minus point to each other
	// Check that the two sandwich leaf opening point to each other
	if trace.OpeningMinus.Next != iPlus || trace.OpeningPlus.Prev != iMinus {
		return fmt.Errorf(
			"bad sandwich prev and next do not point to each other : prev %++v, next %++v",
			trace.OpeningMinus, trace.OpeningPlus,
		)
	}

	// Check that the opening's hkeys make a correct sandwich
	hkey := hash(trace.Key)
	if hkey.Cmp(trace.OpeningMinus.HKey) < 1 || hkey.Cmp(trace.OpeningPlus.HKey) > -1 {
		return fmt.Errorf(
			"sandwich is incorrect expected %x < %x < %x",
			trace.OpeningMinus.HKey, hkey, trace.OpeningPlus.HKey,
		)
	}

	// Test membership of leaf minus
	leafMinus := hash(&trace.OpeningMinus)
	if smt_koalabear.Verify(&trace.ProofMinus, field.Octuplet(leafMinus), field.Octuplet(trace.SubRoot)) != nil {
		return fmt.Errorf("merkle proof verification failed : minus")
	}

	// Test membership of leaf plus
	leafPlus := hash(&trace.OpeningPlus)
	if smt_koalabear.Verify(&trace.ProofPlus, field.Octuplet(leafPlus), field.Octuplet(trace.SubRoot)) != nil {
		return fmt.Errorf("merkle proof verification failed : plus")
	}

	// Check that NextFreeNode for the Prover and the Verifier are the same
	if v.NextFreeNode != int64(trace.NextFreeNode) {
		return fmt.Errorf("inconsistent NextFreeNode %v != %v", v.NextFreeNode, trace.NextFreeNode)
	}

	return nil
}

// DeferMerkleChecks implements the [Trace] interface.
func (trace ReadZeroTrace[K, V]) DeferMerkleChecks(
	appendTo []smt_koalabear.ProvedClaim,
) []smt_koalabear.ProvedClaim {

	// Test membership of leaf minus
	leafMinus := hash(&trace.OpeningMinus)

	// Test membership of leaf plus
	leafPlus := hash(&trace.OpeningPlus)

	appendTo = append(appendTo, smt_koalabear.ProvedClaim{
		Proof: trace.ProofMinus,
		Root:  field.Octuplet(trace.SubRoot),
		Leaf:  field.Octuplet(leafMinus),
	})

	return append(appendTo, smt_koalabear.ProvedClaim{
		Proof: trace.ProofPlus,
		Root:  field.Octuplet(trace.SubRoot),
		Leaf:  field.Octuplet(leafPlus),
	})
}

func (trace ReadZeroTrace[K, V]) HKey() KoalaOctuplet {
	return hash(trace.Key)
}

func (trace ReadZeroTrace[K, V]) RWInt() int {
	return 0
}
