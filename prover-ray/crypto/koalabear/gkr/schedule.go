package gkr

import (
	"fmt"
	"slices"
)

// Ported from gnark's constraint/gkr.go and internal/gkr/gkrcore/schedule.go
// (branch fix/gkr/multisrc).

type (
	// ClaimSource identifies an incoming evaluation claim for a wire.
	// Level is the level that produced the claim.
	// OutgoingClaimIndex selects which of that level's outgoing evaluation points is referenced;
	// always 0 for SumcheckLevels, 0..M-1 for SkipLevels with M inherited evaluation points.
	// The initial verifier challenge is represented as {Level: len(schedule), OutgoingClaimIndex: 0}.
	ClaimSource struct {
		Level              int
		OutgoingClaimIndex int
	}

	// ClaimGroup represents a set of wires sharing identical claim sources.
	ClaimGroup struct {
		Wires        []int
		ClaimSources []ClaimSource
	}

	// ProvingLevel is a single level in the proving schedule.
	ProvingLevel interface {
		NbOutgoingEvalPoints() int
		// NbClaims returns the total number of claims at this level.
		NbClaims() int
		ClaimGroups() []ClaimGroup
		// FinalEvalProofIndex returns where to find the evaluationPointI'th evaluation claim
		// for the wireI'th input wire to the layer, in the layer's final evaluation proof.
		FinalEvalProofIndex(wireI, evaluationPointI int) int
	}

	// SkipLevel represents a level where zerocheck is skipped.
	// Claims propagate through at their existing evaluation points.
	SkipLevel ClaimGroup

	// SumcheckLevel represents a level where one or more zerochecks are batched
	// together in a single sumcheck. Each ClaimGroup within may have different
	// claim sources (sumcheck-level batching), or the same source (enabling
	// zerocheck-level batching with shared eq tables).
	SumcheckLevel []ClaimGroup

	// SingleSourceZeroCheckLevel represents a level where a single-claim-source
	// zerocheck is performed with the eq polynomial factored out, reducing the
	// sumcheck round polynomial degree by 1.
	SingleSourceZeroCheckLevel ClaimGroup

	// ProvingSchedule is a sequence of levels defining how to prove a GKR circuit.
	ProvingSchedule []ProvingLevel
)

func (g ClaimGroup) NbClaims() int { return len(g.Wires) * len(g.ClaimSources) }

func (l *SumcheckLevel) NbOutgoingEvalPoints() int { return 1 }
func (l *SumcheckLevel) NbClaims() int {
	n := 0
	for _, g := range *l {
		n += len(g.Wires) * len(g.ClaimSources)
	}
	return n
}
func (l *SumcheckLevel) ClaimGroups() []ClaimGroup            { return *l }
func (l *SumcheckLevel) FinalEvalProofIndex(wireI, _ int) int { return wireI }

func (l *SkipLevel) NbOutgoingEvalPoints() int { return len(l.ClaimSources) }
func (l *SkipLevel) NbClaims() int             { return ClaimGroup(*l).NbClaims() }
func (l *SkipLevel) ClaimGroups() []ClaimGroup { return []ClaimGroup{ClaimGroup(*l)} }
func (l *SkipLevel) FinalEvalProofIndex(wireI, evaluationPointI int) int {
	return wireI*l.NbOutgoingEvalPoints() + evaluationPointI
}

func (l *SingleSourceZeroCheckLevel) NbOutgoingEvalPoints() int { return 1 }
func (l *SingleSourceZeroCheckLevel) NbClaims() int             { return ClaimGroup(*l).NbClaims() }
func (l *SingleSourceZeroCheckLevel) ClaimGroups() []ClaimGroup {
	return []ClaimGroup{ClaimGroup(*l)}
}
func (l *SingleSourceZeroCheckLevel) FinalEvalProofIndex(wireI, _ int) int { return wireI }

// WireLevels returns, for each wire, the level it belongs to.
func (s ProvingSchedule) WireLevels(nbWires int) []ProvingLevel {
	res := make([]ProvingLevel, nbWires)
	for _, level := range s {
		for _, group := range level.ClaimGroups() {
			for _, w := range group.Wires {
				res[w] = level
			}
		}
	}
	return res
}

// InputMapping returns as uniqueInputs the deduplicated list of inputs to the level,
// and as inputIndices for every wire in the level the list of positions for each of its
// inputs in the uniqueInputs list.
// Input wires of the circuit are considered self-input, as a convenience for the sumcheck protocol.
func (c Circuit) InputMapping(level ProvingLevel) (uniqueInputs []int, inputIndices [][]int) {
	seen := make(map[int]int) // wire index → position in uniqueInputs
	for _, group := range level.ClaimGroups() {
		for _, wI := range group.Wires {
			wire := c[wI]
			inputs := wire.Inputs
			if wire.IsInput() {
				inputs = []int{wI}
			}

			indices := make([]int, len(inputs))
			for inWI, inW := range inputs {
				pos, ok := seen[inW]
				if !ok {
					pos = len(uniqueInputs)
					seen[inW] = pos
					uniqueInputs = append(uniqueInputs, inW)
				}
				indices[inWI] = pos
			}
			inputIndices = append(inputIndices, indices)
		}
	}
	return
}

// UniqueGateInputs returns the unique gate input wire indices for all wires in the level,
// deduplicated in batch-then-wire-then-input order (first occurrence wins).
// For circuit input wires (no gate inputs), the wire itself is returned.
func (c Circuit) UniqueGateInputs(level ProvingLevel) []int {
	uniqueInputs, _ := c.InputMapping(level)
	return uniqueInputs
}

// ZeroCheckDegree returns the degree in each variable of the level's round polynomial.
func (c Circuit) ZeroCheckDegree(level ProvingLevel) int {
	maxDeg := 0
	for _, group := range level.ClaimGroups() {
		for _, wI := range group.Wires {
			w := &c[wI]
			curr := 1
			if !w.IsInput() {
				curr = w.Gate.Degree
			}
			maxDeg = max(maxDeg, curr)
		}
	}

	switch level.(type) {
	case *SumcheckLevel:
		return maxDeg + 1
	case *SingleSourceZeroCheckLevel:
		return maxDeg
	case *SkipLevel:
		return 0
	}
	panic(fmt.Sprintf("ZeroCheckDegree: unknown proving level type %T", level))
}

// ProofSize returns the total number of field elements in a GKR proof.
func (c Circuit) ProofSize(schedule ProvingSchedule, logNbInstances int) int {
	size := 0
	for _, level := range schedule {
		// For every outgoing claim and unique input wire, there will be
		// an outgoing evaluation claim included in finalEvalProof.
		size += len(c.UniqueGateInputs(level)) * level.NbOutgoingEvalPoints()
		size += c.ZeroCheckDegree(level) * logNbInstances
	}
	return size
}

// ReduplicateInputs expands unique evaluations to per-wire gate input evaluation lists.
func ReduplicateInputs[F any](level ProvingLevel, c Circuit, uniqueEvals []F) [][]F {
	_, inputIndices := c.InputMapping(level)
	result := make([][]F, len(inputIndices))
	for wireInLevel := range inputIndices {
		wireInputs := make([]F, len(inputIndices[wireInLevel]))
		for gateInputJ, uniqueI := range inputIndices[wireInLevel] {
			wireInputs[gateInputJ] = uniqueEvals[uniqueI]
		}
		result[wireInLevel] = wireInputs
	}
	return result
}

// CollectOutgoingEvalPoints sets the outgoing evaluation points of a skip level,
// equal to its incoming ones.
func CollectOutgoingEvalPoints[F any](level *SkipLevel, levelI int, outgoingEvalPoints [][][]F) [][]F {
	outPoints := make([][]F, level.NbOutgoingEvalPoints())
	for k, src := range level.ClaimSources {
		outPoints[k] = outgoingEvalPoints[src.Level][src.OutgoingClaimIndex]
	}
	outgoingEvalPoints[levelI] = outPoints
	return outPoints
}

// UniqueInputIndices returns uniqueInputIndices[wI][claimI], the position of wire wI
// in the UniqueGateInputs list of the source level for its claimI-th claim source.
// The sentinel initial-challenge claim maps to 0 (unused at call sites).
func (c Circuit) UniqueInputIndices(schedule ProvingSchedule) [][]int {
	cache := make([]map[int]int, len(schedule)) // cache[levelI][wireI] is the unique input index of wireI in levelI.
	res := make([][]int, len(c))

	// This loop weaves the level's treatment both as a claim source and as the collection of input wires
	for levelI := len(schedule) - 1; levelI >= 0; levelI-- {
		level := schedule[levelI]
		cache[levelI] = make(map[int]int)

		for _, group := range level.ClaimGroups() {
			for _, wI := range group.Wires {

				for _, inputWI := range c[wI].Inputs {
					if _, ok := cache[levelI][inputWI]; !ok {
						cache[levelI][inputWI] = len(cache[levelI])
					}
				}

				for _, claimSource := range group.ClaimSources {
					if claimSource.Level == len(schedule) { // output
						res[wI] = append(res[wI], 0) // zero by convention
					} else {
						res[wI] = append(res[wI], cache[claimSource.Level][wI])
					}
				}
			}
		}
	}
	return res
}

// scheduleBuilder accumulates topology and per-wire claim sources while a schedule is being built.
// Steps are appended in out-to-in (topological) order and reversed by finalize.
// Claim source level values are stored as their index in the levels slice, with -1 as the
// sentinel for the initial challenge. finalize will map each src.Level to its final absolute index via
// n-1-src.level, where n = len(levels), so -1 → n (initial challenge) and i → n-1-i (real levels).
type scheduleBuilder struct {
	circuit Circuit
	// wireOutputs[i] indices of wires that wire i feeds into, in increasing order and deduplicated.
	wireOutputs   [][]int
	wireLevels    []int // wireLevels[i] which level wire i has been put in
	wireProcessed []bool
	// claimSourcesCache[i] is the result of claimSources(i), or nil if not yet computed.
	claimSourcesCache    [][]ClaimSource
	firstUnprocessedWire int
	levels               ProvingSchedule
}

// newScheduleBuilder initialises a builder for the given circuit.
// It computes the outputs inverse-adjacency list.
func newScheduleBuilder(c Circuit) scheduleBuilder {
	b := scheduleBuilder{
		circuit:              c,
		wireOutputs:          make([][]int, len(c)),
		wireLevels:           make([]int, len(c)),
		wireProcessed:        make([]bool, len(c)),
		claimSourcesCache:    make([][]ClaimSource, len(c)),
		firstUnprocessedWire: len(c) - 1,
	}
	seen := make(map[int]bool, len(c))
	for i := range c {
		clear(seen)
		for _, in := range c[i].Inputs {
			if seen[in] {
				continue // a gate reading the same wire twice yields one output edge
			}
			seen[in] = true
			b.wireOutputs[in] = append(b.wireOutputs[in], i)
		}
	}
	return b
}

// addSumcheckLevel appends a SumcheckLevel to the schedule. Each batch is a set of wire indices
// to be proven together in a single zerocheck; all wires in a batch must share the same claim sources.
// All wires across all batches must be ready.
func (b *scheduleBuilder) addSumcheckLevel(batches ...[]int) error {
	claimGroups, err := b.buildClaimGroups(batches)
	if err != nil {
		return err
	}
	lvl := SumcheckLevel(claimGroups)
	b.levels = append(b.levels, &lvl)
	return nil
}

// addSingleSourceZeroCheckLevel appends a SingleSourceZeroCheckLevel to the schedule.
// All wires must share the same single claim source and must be ready.
func (b *scheduleBuilder) addSingleSourceZeroCheckLevel(wireIndices []int) error {
	claimGroups, err := b.buildClaimGroups([][]int{wireIndices})
	if err != nil {
		return err
	}
	if len(claimGroups[0].ClaimSources) != 1 {
		return fmt.Errorf(
			"single source zerocheck level requires exactly 1 claim source, got %d",
			len(claimGroups[0].ClaimSources),
		)
	}
	lvl := SingleSourceZeroCheckLevel(claimGroups[0])
	b.levels = append(b.levels, &lvl)
	return nil
}

// addSkipLevel appends a SkipLevel to the schedule for a single set of wire indices.
// All wires in the batch must share the same claim sources and must be ready.
func (b *scheduleBuilder) addSkipLevel(wireIndices []int) error {
	claimGroups, err := b.buildClaimGroups([][]int{wireIndices})
	if err != nil {
		return err
	}
	lvl := SkipLevel(claimGroups[0])
	b.levels = append(b.levels, &lvl)
	return nil
}

// buildClaimGroups processes a set of batches, validates claim source consistency within each
// batch, updates wireLevels and wireProcessed, and returns the resulting ClaimGroups.
// Every ClaimSources slice is sorted. The user may reorder it to optimize eq handling.
func (b *scheduleBuilder) buildClaimGroups(batches [][]int) ([]ClaimGroup, error) {
	levelI := len(b.levels)
	claimGroups := make([]ClaimGroup, len(batches))
	for i, wireIndices := range batches {
		var claimSources []ClaimSource
		for j, wI := range wireIndices {
			wireClaims, ok := b.claimSources(wI)
			if !ok {
				return nil, fmt.Errorf("wire %d is not ready", wI)
			}
			if j == 0 {
				claimSources = wireClaims
			} else if !slices.Equal(claimSources, wireClaims) {
				return nil, fmt.Errorf("wires %d and %d in the same batch have different claim sources", wireIndices[0], wI)
			}
			b.wireLevels[wI] = levelI
			b.wireProcessed[wI] = true
			if wI == b.firstUnprocessedWire {
				for b.firstUnprocessedWire--; b.firstUnprocessedWire >= 0 &&
					b.wireProcessed[b.firstUnprocessedWire]; b.firstUnprocessedWire-- {
				}
			}
		}
		claimGroups[i] = ClaimGroup{Wires: slices.Clone(wireIndices), ClaimSources: claimSources}
	}
	return claimGroups, nil
}

// nextReady returns the highest wire index in the contiguous ready suffix starting at
// firstUnprocessedWire, along with each wire's claim sources in descending wire-index order
// (sources[0] belongs to firstUnprocessedWire, sources[i] to firstUnprocessedWire-i).
// Returns firstUnprocessedWire, nil if no wires are ready.
func (b *scheduleBuilder) nextReady() (highestWireI int, sources [][]ClaimSource) {
	for lowestWireI := b.firstUnprocessedWire; lowestWireI >= 0; lowestWireI-- {
		if b.wireProcessed[lowestWireI] {
			break
		}
		src, ok := b.claimSources(lowestWireI)
		if !ok {
			break
		}
		sources = append(sources, src)
	}
	return b.firstUnprocessedWire, sources
}

// claimSources checks whether all consumers of wire wI have already been processed.
// If so, it returns the deduplicated claim sources for wI and true.
// If not, it returns nil and false. Results are cached.
// SkipLevels are proper claim targets: a wire feeding into a SkipLevel L with M inherited
// evaluation points gets M claim sources {L, 0}, {L, 1}, ..., {L, M-1}.
func (b *scheduleBuilder) claimSources(wI int) ([]ClaimSource, bool) {
	if b.claimSourcesCache[wI] != nil {
		return b.claimSourcesCache[wI], true
	}
	var wireClaims []ClaimSource
	if b.circuit[wI].Exported || len(b.wireOutputs[wI]) == 0 {
		wireClaims = append(wireClaims, ClaimSource{Level: -1, OutgoingClaimIndex: 0})
	}
	for _, consumerWI := range b.wireOutputs[wI] {
		if !b.wireProcessed[consumerWI] {
			return nil, false
		}
		consumerLevel := b.wireLevels[consumerWI]
		switch b.levels[consumerLevel].(type) {
		case *SkipLevel:
			// SkipLevel inherits M evaluation points from its own claim sources.
			M := b.levels[consumerLevel].NbOutgoingEvalPoints()
			for k := range M {
				wireClaims = append(wireClaims, ClaimSource{Level: consumerLevel, OutgoingClaimIndex: k})
			}
		default:
			wireClaims = append(wireClaims, ClaimSource{Level: consumerLevel, OutgoingClaimIndex: 0})
		}
	}
	// Deduplicate while preserving order.
	seen := make(map[ClaimSource]bool, len(wireClaims))
	out := wireClaims[:0]
	for _, cs := range wireClaims {
		if !seen[cs] {
			seen[cs] = true
			out = append(out, cs)
		}
	}
	b.claimSourcesCache[wI] = out
	return out, true
}

// finalize reverses the schedule into in-to-out order and fixes up Level indices in all
// ClaimSources. It errors if any wire has not been processed.
func (b *scheduleBuilder) finalize() (ProvingSchedule, error) {
	for i, processed := range b.wireProcessed {
		if !processed {
			return nil, fmt.Errorf("wire %d has not been processed", i)
		}
	}

	n := len(b.levels)
	slices.Reverse(b.levels)
	// Fix up ClaimSources: pre-reversal Level index src maps to n-1-src,
	// and the initial-challenge sentinel -1 maps to n.
	for _, level := range b.levels {
		for _, group := range level.ClaimGroups() {
			mirrorClaimSources(group.ClaimSources, n)
		}
	}

	return b.levels, nil
}

// mirrorClaimSources maps each pre-reversal Level index src.Level in-place to its post-reversal
// absolute index n-1-src.Level. The initial-challenge sentinel -1 maps to n.
func mirrorClaimSources(s []ClaimSource, n int) {
	n--
	for j := range s {
		s[j].Level = n - s[j].Level
	}
}

type levelKind uint8

const (
	kindSkip levelKind = iota
	kindSumcheck
	kindSingleSourceZeroCheck
)

func batchForWire(c Circuit, highWI int, readyWireClaimSources [][]ClaimSource) (batchWires []int, kind levelKind) {
	batchWires = []int{highWI}
	for len(batchWires) < len(readyWireClaimSources) {
		if c[highWI].Gate.Degree != c[highWI-len(batchWires)].Gate.Degree ||
			!slices.Equal(readyWireClaimSources[0], readyWireClaimSources[len(batchWires)]) {
			break
		}
		batchWires = append(batchWires, highWI-len(batchWires))
	}

	batchClaimSources := readyWireClaimSources[0]
	kind = kindSumcheck
	if c[highWI].Gate.Degree == 1 && len(batchClaimSources) == 1 { // certain that skipping won't cause a claim blowup
		kind = kindSkip
	} else if len(batchClaimSources) == 1 {
		kind = kindSingleSourceZeroCheck
	}
	return
}

// DefaultProvingSchedule generates a schedule that greedily batches input wires with the same
// single claim source into the same SkipLevel. Non-input wires, and input wires with multiple
// claim sources, each get their own SumcheckLevel.
func DefaultProvingSchedule(c Circuit) (ProvingSchedule, error) {
	b := newScheduleBuilder(c)

	for b.firstUnprocessedWire >= 0 {
		highWI, readyWireClaimSources := b.nextReady()
		// try and make a homogenous (same degree, same claims) batchWires
		w := c[highWI]
		batchClaimSources := readyWireClaimSources[0]
		if w.IsInput() && len(batchClaimSources) == 1 {
			if err := b.addSkipLevel([]int{highWI}); err != nil {
				return nil, err
			}
			continue
		}

		// there is an actual "gate" in question
		batchWires, kind := batchForWire(c, highWI, readyWireClaimSources)
		var err error
		switch kind {
		case kindSkip:
			err = b.addSkipLevel(batchWires)
		case kindSingleSourceZeroCheck:
			err = b.addSingleSourceZeroCheckLevel(batchWires)
		default:
			batches := [][]int{batchWires}
			nbLevelWires := len(batchWires)
			for nbLevelWires < len(readyWireClaimSources) {
				newBatchHighWI := highWI - nbLevelWires
				if c[newBatchHighWI].Gate.Degree != c[highWI].Gate.Degree {
					break
				}
				batchWires, kind = batchForWire(c, newBatchHighWI, readyWireClaimSources[nbLevelWires:])
				if kind != kindSumcheck {
					break
				}
				batches = append(batches, batchWires)
				nbLevelWires += len(batchWires)
			}
			err = b.addSumcheckLevel(batches...)
		}
		if err != nil {
			return nil, err
		}
	}
	return b.finalize()
}
