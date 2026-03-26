package selfrecursion

import (
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/common/smartvectors"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/maths/field"
	VortexCompiler "github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/compiler/vortex"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Registers the polynomial I(X) = (0, 1, 2, 3, 4, 5, ..., nBcol-1)
// and the SIS key chunks.
//
// Say we have a random ring-SIS key with dimension
//
//	(A0(X),A1(X),A2(X),…,An−1(X))
//
// and that I need k polynomials to hash a field element. We need
// k≥1, so that sets some conditions on the ring-SIS instance. If
// we plan to commit to m polynomials at once at some point of the
// protocol and that at that point I would already have committed
// to m′ polynomials. In that case, the key shard Ah would be
//
//	(0(X)×km′) ∥ (A0(X),…,Akm(X)) ∥ (0(X)…up to kmf)
//
// Namely, all key shards consists of the first entries of A with
// some offset and are zeroes everywhere else.

func (ctx *SelfRecursionCtx) Precomputations() {
	ctx.RegistersI()
	ctx.RegistersAh()
}

// Registers the polynomial I(X)
func (ctx *SelfRecursionCtx) RegistersI() {
	nBColEncoded := ctx.VortexCtx.NumEncodedCols()
	i := make([]field.Element, nBColEncoded)
	for k := range i {
		i[k].SetUint64(uint64(k))
	}
	ctx.Columns.I = ctx.Comp.InsertPrecomputed(ctx.iName(nBColEncoded), smartvectors.NewRegular(i))
}

// Registers the key shards, since some rounds are dried, some of the
// of the entries are nil
func (ctx *SelfRecursionCtx) RegistersAh() {
	// We need the length of total number of SIS rounds
	ahLength := ctx.VortexCtx.CommitmentsByRoundsSIS.Len()
	// Consider the precomputed columns.
	// We increase ahLength only if SIS is applied to
	// the precomputed columns.
	if ctx.VortexCtx.IsNonEmptyPrecomputed() && ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		ahLength += 1
	}
	ah := make([]ifaces.Column, 0, ahLength)

	// Tracks the number of rows already committed with SIS as we arrive in this
	// round
	maxSize := utils.NextPowerOfTwo(ctx.VortexCtx.CommittedRowsCountSIS)
	roundStartAt := 0

	// Consider the precomputed columns
	if ctx.VortexCtx.IsNonEmptyPrecomputed() && ctx.VortexCtx.IsSISAppliedToPrecomputed() {
		numPrecomputeds := len(ctx.VortexCtx.Items.Precomputeds.PrecomputedColums)

		// Sanity-check : if coms in precomputeds have length zero then the
		// associated Dh should be nil
		if (numPrecomputeds == 0) != (ctx.Columns.PrecompRoot == nil) {
			panic("nilness mismatch for precomputeds")
		}

		// The Vortex compiler is supposed to add "shadow columns" ensuring that
		// every round (counting the precomputations as a round) uses ring-SIS
		// polynomials fully. Otherwise, the compilation will not be able to
		// be successful.
		if (numPrecomputeds*ctx.SisKey().NumLimbs())%(1<<ctx.SisKey().LogTwoDegree) > 0 {
			panic("the ring-SIS polynomials are not fully used")
		}

		// Registers the commitment key, if this matches an existing key
		// then the preexisting precomputed key is reused.
		ah = append(ah, ctx.Comp.InsertPrecomputed(
			ctx.ahName(ctx.SisKey(), roundStartAt, numPrecomputeds, maxSize),
			flattenedKeyChunk(ctx.SisKey(), roundStartAt, numPrecomputeds, maxSize),
		))

		// And update the value of the start
		roundStartAt += numPrecomputeds
	}

	for i, comsInRoundsI := range ctx.VortexCtx.CommitmentsByRounds.GetInner() {
		// We need to consider only the SIS rounds
		if ctx.VortexCtx.RoundStatus[i] == VortexCompiler.IsSISApplied {
			// Sanity-check : if coms in rounds has length zero then the
			// associated Dh should be nil. That happens when the examinated round
			// is an "empty" round or when it has been self-recursed already.
			if (len(comsInRoundsI) == 0) != (ctx.Columns.Rooth[i] == nil) {
				utils.Panic("nilness mismatch for round=%v #coms-in-round=%v vs root-is-nil=%v", i, len(comsInRoundsI), ctx.Columns.Rooth[i] == nil)
			}

			// Check if there is no rows to commit
			if len(comsInRoundsI) == 0 {
				// and ah[i] is nil
				utils.Panic("We don't expect no polynomials to commit in a SIS round")
			}

			// The Vortex compiler is supposed to add "shadow columns" ensuring that
			// every round (counting the precomputations as a round) uses ring-SIS
			// polynomials fully. Otherwise, the compilation will not be able to
			// be successful.
			if (len(comsInRoundsI)*ctx.SisKey().NumLimbs())%(1<<ctx.SisKey().LogTwoDegree) > 0 {
				panic("the ring-SIS polynomials are not fully used")
			}

			// Registers the commitment key (if this matches an existing key
			// then the preexisting precomputed key is reused).
			ah = append(ah, ctx.Comp.InsertPrecomputed(
				ctx.ahName(ctx.SisKey(), roundStartAt, len(comsInRoundsI), maxSize),
				flattenedKeyChunk(ctx.SisKey(), roundStartAt, len(comsInRoundsI), maxSize),
			))

			// And update the value of the start
			roundStartAt += len(comsInRoundsI)
		} else {
			continue
		}
	}
	ctx.Columns.Ah = ah
}

// Returns the laid out keys
func flattenedKeyChunk(key *ringsis.Key, start, length, maxSize int) smartvectors.SmartVector {

	// Sanity-check : the chunkNo can't be off-bound
	if maxSize < start+length {
		utils.Panic("inconsistent arguments : %v + %v > %v", start, length, maxSize)
	}

	// Case for the last chunk
	flattenedKey := key.FlattenedKey()
	res := make([]field.Element, maxSize*key.NumLimbs())

	// Scales the start and the length according to how many limbs
	// are necessary to encode a field element.
	startAt := start * key.NumLimbs()
	numToWrite := length * key.NumLimbs()

	copy(res[startAt:], flattenedKey[:numToWrite])
	return smartvectors.NewRegular(res)
}
