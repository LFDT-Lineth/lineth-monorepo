package fri

import (
	"fmt"
	"sort"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// ProverState drives the FRI commit and query phases as a coin-fed state
// machine. Instead of consuming every folding challenge up front, the caller
// feeds one challenge at a time via [ProverState.Fold]: each call folds the
// current layer into the next, commits the new layer, and returns its Merkle
// root so the caller can derive the next challenge from it (Fiat-Shamir). After
// numRounds folds the running polynomial has collapsed to the final polynomial,
// and [ProverState.Open] consumes the query positions to produce the Proof.
//
// Lifecycle:
//
//	st, _ := NewProverState(p, levels)
//	for st.HasNext() {              // numRounds iterations
//	    root := st.Fold(alpha_j)    // commits layer j+1, returns its root
//	    // absorb root, derive alpha_{j+1} …
//	}
//	proof := st.Open(positions)
//
// The embedded Proof is the in-progress proof; it is fully populated only once
// Open returns.
type ProverState struct {
	// Proof is the in-progress proof. Roots are filled as folds happen, the
	// final polynomial on the last fold, and the queries by Open.
	Proof

	p      Params
	plan   provePlan
	levels []Level

	// round is the next folding round to run; equivalently the number of folds
	// performed so far and the index of the current running layer.
	round int

	running []field.Ext   // evaluations of layer[round]
	layers  [][]field.Ext // layers[0..round]; layers[numRounds] is the final polynomial
	trees   []*Tree       // trees[0..min(round, numRounds-1)]; the final layer has no tree
}

// NewProverState validates the levels, builds the folding schedule, and seeds
// the machine with the committed layer 0. levels is sorted in-place by
// decreasing degree (as buildProvePlan requires).
func NewProverState(p Params, levels []Level) (*ProverState, error) {

	sort.Slice(levels, func(i, j int) bool {
		return levels[i].D > levels[j].D
	})

	plan, err := buildProvePlan(p, levels)
	if err != nil {
		return nil, fmt.Errorf("fri: NewProverState: %w", err)
	}

	st := &ProverState{
		p:       p,
		plan:    plan,
		levels:  levels,
		running: make([]field.Ext, p.N),
		layers:  make([][]field.Ext, p.numRounds+1),
		trees:   make([]*Tree, p.numRounds),
	}

	// Layer 0 is committed up front; its root is supplied externally rather than stored in RoundRoots.
	copy(st.running, levels[0].Evals)
	st.layers[0] = st.running

	if p.numRounds > 1 {
		st.RoundRoots = make([]field.Octuplet, p.numRounds-1)
	}

	// D=1: the top level is already the final layer, so there is nothing to
	// fold (HasNext is false from the start) and Fold is never called to
	// populate FinalPolyExt. Reveal layer 0 directly, matching the size
	// N>>numRounds == N that checkOpeningProofShape expects.
	if p.numRounds == 0 {
		st.FinalPolyExt = st.running
	}

	return st, nil
}

// HasNext reports whether another folding challenge is expected.
func (st *ProverState) HasNext() bool {
	return st.round < st.p.numRounds
}

// Fold consumes one folding challenge. It folds the current layer into the next
// one (mixing in the auxiliary half-codeword scheduled at the next round, batched
// with alpha²), commits the new layer, and returns its Merkle root. On the final
// fold the running polynomial becomes the final polynomial — revealed in the
// clear rather than committed — and the returned root is the zero octuplet.
func (st *ProverState) Fold(alpha field.Ext) field.Octuplet {

	if !st.HasNext() {
		panic("fri: ProverState.Fold: all folding rounds already consumed")
	}

	j := st.round

	// The level scheduled to come online at round j+1 is mixed into the fold
	// output as the auxiliary half-codeword, batched with alpha². Its evaluation
	// vector has length N>>(j+1) = N_{j+1}, exactly the size of the fold output,
	// so it lands directly in committed layer j+1. aux stays nil when no level is
	// introduced at round j+1 (including the last round).
	var aux []field.Ext
	if l, ok := st.plan.levelAtRound[j+1]; ok {
		aux = st.levels[l].Evals
	}

	st.running = foldLayerInternally(st.running, aux, alpha, st.p.domains[j], st.p.invTwo)
	st.layers[j+1] = st.running
	st.round = j + 1

	if j+1 == st.p.numRounds {
		// Final layer: revealed directly, no Merkle commitment.
		st.FinalPolyExt = st.running
		return field.Octuplet{}
	}

	tree := buildTreeExt(st.running)
	st.trees[j+1] = tree
	root := tree.Root()
	st.RoundRoots[j] = root // root of layer j+1 → RoundRoots[(j+1)-1]
	return root
}

// Open runs the query phase for the given query positions and returns the
// completed Proof. It must be called after all numRounds folds.
func (st *ProverState) Open(openedPositions []int) Proof {

	if st.round != st.p.numRounds {
		panic("fri: ProverState.Open: called before all folding rounds were consumed")
	}

	st.RunningQueries = make([]RunningQuery, st.p.NumQueries)

	for k := range st.p.NumQueries {
		s := openedPositions[k]
		st.RunningQueries[k] = openRunningQueryExt(s, st.layers, st.trees, st.p.numRounds)
	}

	return st.Proof
}
