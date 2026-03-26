package accumulator

import (
	"fmt"
	"io"

	"github.com/consensys/linea-monorepo/prover/utils"

	//lint:ignore ST1001 -- the package contains a list of standard types for this repo
	"github.com/consensys/linea-monorepo/prover/utils/types"
	. "github.com/consensys/linea-monorepo/prover/utils/types"
)

// LeafOpening represents the opening of a leaf in the accumulator's tree.
//
// The `json` format and the order of the fields is important
type LeafOpening struct {
	Prev int64               `json:"prevLeaf"`
	Next int64               `json:"nextLeaf"`
	HKey types.KoalaOctuplet `json:"hkey"` //it is poseidon2 hash of the adress
	HVal types.KoalaOctuplet `json:"hval"` // is it poseidon2 of account
}

// KVOpeningTuple is simple a tuple type of (key, value) adding the
// corresponding leaf opening
type KVOpeningTuple[K, V io.WriterTo] struct {
	LeafOpening LeafOpening
	Key         K
	Value       V
}

// WriteTo implements the [io.WriterTo] interface and is used to hash the leaf
// opening into the leaves that we store in the tree.
func (leaf *LeafOpening) WriteTo(w io.Writer) (int64, error) {
	n0, _ := WriteInt64On64Bytes(w, leaf.Prev) // n0 = 64 (bytes)
	n1, _ := WriteInt64On64Bytes(w, leaf.Next) // n1 = 64 (bytes)
	n2, _ := leaf.HKey.WriteTo(w)              // n2 = 32 (bytes)
	n3, _ := leaf.HVal.WriteTo(w)              // n3 = 32 (bytes)
	// Sanity-check the written size of the leaf opening
	total := n0 + n1 + n2 + n3
	if total != 192 {
		utils.Panic("bad size")
	}
	return total, nil
}

// Hash returns a hash of the leaf opening
func (leaf LeafOpening) Hash() types.KoalaOctuplet {
	return hash(&leaf)
}

// Head returns the "head" of the accumulator set
func Head() LeafOpening {
	return LeafOpening{
		Prev: 0, // Points to itself
		Next: 1,
		HKey: types.KoalaOctuplet{},
		HVal: types.KoalaOctuplet{},
	}
}

// Tail returns the "tail" of the accumulator set
func Tail() LeafOpening {
	return LeafOpening{
		Prev: 0,
		Next: 1, // Points to itself
		HVal: types.KoalaOctuplet{},
		HKey: types.MaxKoalaOctuplet(),
	}
}

// HeadOrTail returns true if the leaf opening is either head or tail
func (leaf *LeafOpening) HeadOrTail() bool {
	return leaf.HKey.IsMaxOctuplet() || leaf.HKey == types.KoalaOctuplet{}
}

// CheckAndLeaf check the internal consistency of the tuple and returns the hash
// of the leaf opening (corresponding to a leaf).
func (t KVOpeningTuple[K, V]) CheckAndLeaf() (KoalaOctuplet, error) {

	if t.LeafOpening.HeadOrTail() {
		return t.LeafOpening.Hash(), nil
	}

	if t.LeafOpening.HKey != hash(t.Key) {
		return types.KoalaOctuplet{}, fmt.Errorf("inconsistent key and leaf openings")
	}

	if t.LeafOpening.HVal != hash(t.Value) {
		return types.KoalaOctuplet{}, fmt.Errorf("inconsistent val and leaf opening")
	}

	return t.LeafOpening.Hash(), nil
}

// CopyWithPrev copies the leaf opening and set the prev in the copy
func (leaf LeafOpening) CopyWithPrev(prev int64) LeafOpening {
	leaf.Prev = prev
	return leaf
}

// CopyWithNext copies the leaf opening and set the next in the copy
func (leaf LeafOpening) CopyWithNext(next int64) LeafOpening {
	leaf.Next = next
	return leaf
}

// MatchValue returns true if the leaf opening opening matches the value
func (l *LeafOpening) MatchValue(v io.WriterTo) bool {
	hval := hash(v)
	return l.HVal == hval
}

// MatchKey returns true if the leaf opening opening matches the value
func (l *LeafOpening) MatchKey(k io.WriterTo) bool {
	hkey := hash(k)
	return l.HKey == hkey
}

// CopyWithPrev copies the tuple and set the prev in the copy
func (t KVOpeningTuple[K, V]) CopyWithPrev(prev int64) KVOpeningTuple[K, V] {
	t.LeafOpening.Prev = prev
	return t
}

// CopyWithNext copies the tuple and set the next in the copy
func (t KVOpeningTuple[K, V]) CopyWithNext(next int64) KVOpeningTuple[K, V] {
	t.LeafOpening.Next = next
	return t
}

// CopyWithVal copies the tuple and give it a new new value
func (t KVOpeningTuple[K, V]) CopyWithVal(val V) KVOpeningTuple[K, V] {
	t.Value = val
	t.LeafOpening.HVal = hash(val)
	return t
}

// String pretty prints a leaf opening
func (l LeafOpening) String() string {
	return fmt.Sprintf(
		"LeafOpening{Prev: %d, Next: %d, HKey: %s, HVal: %s}",
		l.Prev, l.Next, l.HKey.Hex(), l.HVal.Hex(),
	)
}
