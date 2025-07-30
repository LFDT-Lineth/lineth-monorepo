package column

import (
	"fmt"
	"math/big"
	"reflect"
	"strings"

	"github.com/consensys/linea-monorepo/prover/maths/fft"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/utils"
	"github.com/consensys/linea-monorepo/prover/utils/collection"
)

// DeriveEvaluationPoint, given a query at point "x" on the current handle,
// returns the list of "x" values we should query the "leaves" of the current
// handle to get an equivalent polynomial query only on "Non-Composites"
// handles. The result is stored in the map `finalXs` which will only contain
// the "leaves" derived Xs as opposed to the `cachedXs` which computes all
// intermediate values.
//
//   - upstreamBranchRepr string : repr of the upstream-branch
//   - cachedXs : map reprs of full-branches to already derived
//     evaluation points. Essentially a cache for later uses.
//   - x is the 'x' value of the univariate query to evaluate
//
// @alex: we should completely refactor this function. As I am trying to retro-
// engineer it, None of the name or comment in this function make sense to me.
func DeriveEvaluationPoint(
	h ifaces.Column,
	upstream string,
	cachedXs collection.Mapping[string, field.Element],
	x field.Element,
) (xRes field.Element) {

	if !h.IsComposite() {
		// Just return x and cache it if necessary
		newUpstream := appendNodeToUpstream(upstream, h)
		// Store in the cache if necessary
		if !cachedXs.Exists(newUpstream) {
			// Else register the result in the cache
			cachedXs.InsertNew(newUpstream, x)
		}
		return x
	}

	switch inner := h.(type) {
	case Shifted:
		newUpstream := appendNodeToUpstream(upstream, inner)
		var derivedX field.Element
		// Early return if the result is cached
		if cachedXs.Exists(newUpstream) {
			derivedX = cachedXs.MustGet(newUpstream)
		} else {
			// If not, compute the shift on x and cache the result
			n := h.Size()
			omegaN := fft.GetOmega(n)
			omegaN.Exp(omegaN, big.NewInt(int64(inner.Offset)))
			derivedX.Mul(&x, &omegaN)
			cachedXs.InsertNew(newUpstream, derivedX)
		}
		return DeriveEvaluationPoint(inner.Parent, newUpstream, cachedXs, derivedX)

	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

/*
VerifyYConsistency verifies that the claimed values for y
  - upstream is a repr of the branch leading to the current node
  - cachedXs is the set of derived Xs computed by `DeriveEvaluationPoint`
  - finalYs is the map of the alleged evaluation for which we check the
    consistency.

@alex: we should completely refactor this function. As I am trying to retro-
engineer it, None of the name or comment in this function make sense to me.
*/
func VerifyYConsistency(
	h ifaces.Column, upstream string,
	cachedXs collection.Mapping[string, field.Element],
	finalYs collection.Mapping[string, field.Element],
) (y field.Element) {

	if !h.IsComposite() {
		// Get the Y from the map. An absence from this map is unexpected at
		// this level since it means
		newUpstream := appendNodeToUpstream(upstream, h)
		res := finalYs.MustGet(DerivedYRepr(newUpstream, h))
		return res
	}

	switch inner := h.(type) {

	case Shifted:
		newUpstream := appendNodeToUpstream(upstream, inner)
		res := VerifyYConsistency(inner.Parent, newUpstream, cachedXs, finalYs)
		// No need to test the error, because we would return it alonside the
		// nil result anyway.
		return res

	default:
		utils.Panic("unexpected type %v", reflect.TypeOf(inner))
	}
	panic("unreachable")
}

// Returns all subbranches starting from the current node (including the
// current node). It counts as a list of unique identifiers for the derivation path.
func DownStreamBranch(node ifaces.Column) string {

	if !node.IsComposite() {
		return getNodeRepr(node)
	}

	switch inner := node.(type) {
	case Shifted:
		downStreams := DownStreamBranch(inner.Parent)
		return prependNodeToDownstream(inner, downStreams)
	default:
		panic("unreachable")
	}
}

func appendNodeToUpstream(upstream string, node ifaces.Column) string {
	// This happens when the function is called on the root node
	// In this case, no need to prefix with a "_"
	if len(upstream) == 0 {
		return getNodeRepr(node)
	}
	return fmt.Sprintf("%v_%v", upstream, getNodeRepr(node))
}

func prependNodeToDownstream(node ifaces.Column, downstream string) string {
	nodeRepr := getNodeRepr(node)
	return fmt.Sprintf("%v_%v", nodeRepr, downstream)
}

func DerivedYRepr(upstream string, currNode ifaces.Column) string {
	ifaces.AssertNotComposite(currNode)
	// sanity-check
	if !strings.HasSuffix(upstream, nonComposite) {
		utils.Panic("upstream was not suffixed by `NONCOMPOSITE`")
	}
	return fmt.Sprintf("%v_%v",
		upstream,
		currNode.GetColID(),
	)
}

func getNodeRepr(node ifaces.Column) string {

	if !node.IsComposite() {
		return nonComposite
	}

	switch inner := node.(type) {
	case Shifted:
		// In a shifted, the size matters because this tells us which root of
		// unity to use to multiply `x`.
		return fmt.Sprintf("%v_%v_%v", shift, utils.PositiveMod(inner.Offset, node.Size()), node.Size())
	case ifaces.Column:
		// Common unexpectable case
		panic("don't pass a ifaces.Column there, pass the inner directly")
	default:
		utils.Panic("unexpected type %v\n", reflect.TypeOf(inner).String())
	}
	panic("unreachable")
}
