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

	// prevAlpha is the challenge that folded layer[round-1] into layer[round]
	// (zero before round 0, and unused then since no level ever introduces at
	// round 0). A level introduced at round j is weighted by prevAlpha² --
	// the challenge from the round that produced the codeword it folds into.
	prevAlpha field.Ext
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

	return st, nil
}

// HasNext reports whether another folding challenge is expected.
func (st *ProverState) HasNext() bool {
	return st.round < st.p.numRounds
}

// Fold consumes one folding challenge. It folds the current layer into the next
// one (folding in, alongside it, any level introduced at this round -- weighted
// by the square of the challenge that produced the current layer), commits the
// new layer, and returns its Merkle root. On the final fold the running
// polynomial becomes the final polynomial — revealed in the clear rather than
// committed — and the returned root is the zero octuplet.
func (st *ProverState) Fold(alpha field.Ext) field.Octuplet {

	if !st.HasNext() {
		panic("fri: ProverState.Fold: all folding rounds already consumed")
	}

	j := st.round

	// A level introduced at this round is folded alongside the running
	// codeword, weighted by prevAlpha² (the challenge that produced
	// st.running). Its evaluation vector has length N>>j, exactly the size
	// of st.running, so it folds in directly. aux stays nil when no level is
	// introduced at round j (including round 0, which no level ever is).
	var aux []field.Ext
	var auxWeight field.Ext
	if l, ok := st.plan.levelAtRound[j]; ok {
		aux = st.levels[l].Evals
		auxWeight.Square(&st.prevAlpha)
	}

	st.running = foldLayerInternally(st.running, aux, auxWeight, alpha, st.p.domains[j], st.p.invTwo)
	st.layers[j+1] = st.running
	st.prevAlpha = alpha
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
