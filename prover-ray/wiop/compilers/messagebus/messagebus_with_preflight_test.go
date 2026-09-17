package messagebus_test

import (
	"sort"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/fri"
	multisethashing "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/multiset_hashing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/preflight"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/utils"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/global"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/grandproduct"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/localvanishing"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/messagebus"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	"github.com/stretchr/testify/require"
)

// =============================================================================
// γ derived from the shards via FRI PCS, reflecting production behaviour
// =============================================================================

const (
	// busRoundID is where the bus columns sit. Round 0 carries γ and
	// nothing else, so the only committed data behind the contribution is the
	// bus traffic itself.
	busRoundID = 1
	// coinRoundID is where messagebus.Compile puts α and β. It coincides
	// with the bus round: the coins come from γ which is in the transcript
	coinRoundID = 1
)

// equal reports whether two field values coincide.
func equal(a, b field.Gen) bool {
	diff := a.Sub(b)
	return diff.IsZero()
}

// pairColAssignment pairs a column with the rows to write into it.
type pairColAssignment struct {
	col  *wiop.Column
	vals []uint64
}

// busColumnAssigner writes the bus columns. They live on round 1, and the hook
// [wiop.System.Prove] takes runs while the runtime is still on round 0 — where
// [wiop.Runtime.AssignColumn] would reject them — so an action registered on
// round 1 is what puts them in place. It is registered before [pcs.Compile], so
// it runs ahead of that round's commit action and the commitment covers the rows
// it wrote.
type busColumnAssigner struct {
	cols []pairColAssignment
}

func (a *busColumnAssigner) Run(rt *wiop.Runtime) {
	for _, c := range a.cols {
		rt.AssignColumn(c.col, makeVec(c.vals...))
	}
}

// seededShard is a bidirectional shard compiled with real shared randomness and
// real commitments, re-runnable under different γ.
type seededShard struct {
	sys     *wiop.System
	handles []string // alphabetical, matching Compile's public-input order
}

// buildSeededBidirectionalShard mirrors [buildBidirectionalShard] — same traffic
// shape, one sent and one received column per handle, every entry carrying
// SkipInShardCheck so the cross-shard layer owns the balance check — with two
// deliberate differences.
//
// First, it compiles with [messagebus.CompileOptions.SharedRandomness], which is
// what declares the γ cells and the contribution cells and registers the checker.
//
// Second, it runs [pcs.Compile], which registers the per-round commit action that
// populates [wiop.Runtime.Commitments]. The contribution is the multiset hash of
// exactly that value, so without this pass there is nothing for γ to be built
// from.
//
// The bus columns sit on round 1 and round 0 holds only γ, so the bus round is
// the sole committed round behind the contribution. That is what lets the test
// claim γ was derived from the bus traffic rather than from incidental shard
// data: change a single sent row and the contribution moves.
func buildSeededBidirectionalShard(
	t *testing.T,
	name, originShard string,
	traffic []busTraffic,
) *seededShard {
	t.Helper()

	sys := wiop.NewSystemf("%s", name)
	sys.NewRound() // round 0: γ only, no columns
	r1 := sys.NewRound()
	mod := sys.NewSizedModule(sys.Context.Childf("mod"), 4, wiop.PaddingDirectionNone)

	busCols := make([]pairColAssignment, 0, 2*len(traffic))
	handles := make([]string, 0, len(traffic))

	for _, tr := range traffic {
		colA := mod.NewColumn(sys.Context.Childf("a-%s", tr.handle), r1)
		colB := mod.NewColumn(sys.Context.Childf("b-%s", tr.handle), r1)

		send := sys.NewMessageBusSend(
			sys.Context.Childf("send-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colA.View()))
		recv := sys.NewMessageBusReceive(
			sys.Context.Childf("recv-%s", tr.handle), originShard, tr.handle,
			wiop.NewTable(colB.View()))
		send.SkipInShardCheck = true
		recv.SkipInShardCheck = true

		busCols = append(busCols,
			pairColAssignment{colA, tr.sent},
			pairColAssignment{colB, tr.received})
		handles = append(handles, tr.handle)
	}

	r1.RegisterAction(&busColumnAssigner{cols: busCols})

	messagebus.Compile(sys, messagebus.CompileOptions{SharedRandomness: true})
	grandproduct.Compile(sys)
	localvanishing.Compile(sys)
	global.Compile(sys)
	pcs.Compile(sys)

	require.Len(t, sys.Rounds[coinRoundID].Coins, 2,
		"α and β must be declared on round %d", coinRoundID)

	sort.Strings(handles)
	return &seededShard{sys: sys, handles: handles}
}

// runProve proves the shard against g and returns the runtime alongside the proof, so
// the caller can both verify it and read the values the shard published.
func (s *seededShard) runProve(
	t *testing.T,
	g field.Octuplet,
) (rt *wiop.Runtime, proof wiop.Proof, pub wiop.PublicInput) {
	t.Helper()
	proof, pub = s.sys.Prove(func(r *wiop.Runtime) {
		rt = r
		// γ is the shard's only round-0 input; the bus columns are written by the
		// round-1 action.
		messagebus.AssignSharedRandomnessSeed(r, g)
	})
	return rt, proof, pub
}

// getContributionFromPI reads the shard's getContributionFromPI out of its public-input cells.
func (s *seededShard) getContributionFromPI(t *testing.T, rt *wiop.Runtime) multisethashing.MSetHash {
	t.Helper()
	var c multisethashing.MSetHash
	for i := range messagebus.NumSharedRandomnessContribution {
		cell, pos := s.sys.LookupPublicInputByTag(
			messagebus.SharedRandomnessSeedContributionPI, i)
		require.GreaterOrEqual(t, pos, 0, "contribution limb %d must be a public input", i)
		c[i] = rt.GetCellValue(cell).AsBase()
	}
	return c
}

// readSeedFromPI reads γ back out of the shard's public-input cells, which is where an
// aggregator would read it to compare against a sibling's.
func (s *seededShard) readSeedFromPI(t *testing.T, rt *wiop.Runtime) field.Octuplet {
	t.Helper()
	var g field.Octuplet
	for i := range messagebus.NumSharedRandomness {
		cell, pos := s.sys.LookupPublicInputByTag(messagebus.SharedRandomnessSeedPI, i)
		require.GreaterOrEqual(t, pos, 0, "γ limb %d must be a public input", i)
		g[i] = rt.GetCellValue(cell).AsBase()
	}
	return g
}

// getBusAccFromSys returns handle i's accumulator, in the alphabetical order Compile
// numbers the MessageBus public inputs by.
func (s *seededShard) getBusAccFromSys(rt *wiop.Runtime, i int) field.Gen {
	return rt.GetCellValue(s.sys.GrandProducts[i].Result)
}

// busInputSet bundles the shard's bus columns the way [preflight.Run] expects
// them: the raw column data as a [fri.MultiSizeTable], plus the matching RS
// encoders.
//
// Run deliberately does not build this — a caller supplies it, because only the
// orchestrator knows how each shard laid its columns out. Here that layout has to
// reproduce the one the PCS commits at prove time (same column order, same padded
// sizes, same encoder schedule; see commitToRound and buildEncoders), because the
// whole point is that the root obtained before any proof exists is the same one
// the shard will go on to publish as its contribution.
func (s *seededShard) busInputSet(t *testing.T) preflight.BusInputSet {
	t.Helper()
	round := s.sys.Rounds[busRoundID]

	// The rows the round-1 action will write. Reading them off that action is
	// what keeps preflight and the prover looking at the same data.
	var assigner *busColumnAssigner
	for _, a := range round.ProverActions {
		if bc, ok := a.(*busColumnAssigner); ok {
			assigner = bc
			break
		}
	}
	require.NotNil(t, assigner, "the bus round must carry its column assigner")

	rows := make(map[*wiop.Column][]uint64, len(assigner.cols))
	for _, c := range assigner.cols {
		rows[c.col] = c.vals
	}

	// Bucket each column by the log2 of its padded size, walking the round's
	// columns in declaration order so the layout matches the prover's.
	table := make(fri.MultiSizeTable, 64)
	maxSizeIndex := 0
	for _, col := range round.Columns {
		vals, ok := rows[col]
		require.True(t, ok, "column %q has no preflight rows", col.Context.Path())

		size := utils.NextPowerOfTwo(col.Module.Size())
		sizeIndex := utils.Log2Ceil(size)
		maxSizeIndex = max(maxSizeIndex, sizeIndex)

		padded := make([]field.Element, size)
		for i, v := range vals {
			padded[i].SetUint64(v)
		}
		table[sizeIndex].Base = append(table[sizeIndex].Base, padded)
	}

	encoders := make([]*fri.RSEncoder, maxSizeIndex+1)
	for i := range encoders {
		enc := fri.NewEncoder(uint64(1<<pcs.FRILogInverseRate)*(1<<i), 1<<i)
		encoders[i] = &enc
	}

	return preflight.BusInputSet{Table: table[:maxSizeIndex+1], Encoders: encoders}
}

// busInputSets collects one [preflight.BusInputSet] per shard, in the order
// given, ready to hand to [preflight.Run].
func busInputSets(t *testing.T, shards ...*seededShard) []preflight.BusInputSet {
	t.Helper()
	sets := make([]preflight.BusInputSet, len(shards))
	for i, s := range shards {
		sets[i] = s.busInputSet(t)
	}
	return sets
}

// seedOf computes γ from the shards' bus columns with the production
// [preflight.Run] — the same call the orchestrator makes. Going through Run
// rather than recombining the contributions by hand is what keeps these tests
// honest about how γ is actually derived: a change to Run's accumulation shows up
// here instead of being silently mirrored by the test.
//
// It runs entirely outside any proof, from column data alone — no shard has been
// proved at this point — which is exactly the property that lets an orchestrator
// hand γ to every shard before any of them starts proving.
func seedOf(t *testing.T, shards ...*seededShard) field.Octuplet {
	t.Helper()
	return preflight.Run(busInputSets(t, shards...))
}

// contributions returns each shard's individual contribution, Hash(root) of its
// own bus-column commitment.
//
// [preflight.Run] accumulates exactly these values but returns only the combined
// seed, so the per-shard terms it folds together are not recoverable from it.
// They are what lets the tests check that each shard publishes the contribution
// preflight computed for it, and that two shards with different traffic do not
// produce the same one.
func contributions(t *testing.T, shards ...*seededShard) []multisethashing.MSetHash {
	t.Helper()
	out := make([]multisethashing.MSetHash, len(shards))
	for i, set := range busInputSets(t, shards...) {
		cs := fri.Commit(set.Encoders, set.Table)
		out[i] = multisethashing.Hash(cs.Tree.Root())
	}
	return out
}

// TestSharedRandomness_SeedDerivedFromContributions closes the shared-randomness
// loop in the direction no other test covers: γ is computed *from* the shards
// rather than given *to* them.
//
// Two bidirectional shards commit their bus columns and publish a contribution
// each; γ is taken as ToSeed(Combine(c1, c2)), exactly as [preflight.Run] builds
// it. Feeding that γ back must leave the contributions untouched and reproduce
// the same γ — the fixed point that lets an orchestrator hand out a seed the
// shards can be held to — and both shards must then verify against it.
func TestSharedRandomness_SeedDerivedFromContributions(t *testing.T) {
	shard1 := buildSeededBidirectionalShard(
		t, "shard-1-bidir-seeded", "shard-1", crossShardTrafficShard1)
	shard2 := buildSeededBidirectionalShard(
		t, "shard-2-bidir-seeded", "shard-2", crossShardTrafficShard2)

	// Preflight: each shard's contribution, computed from its own bus columns.
	c := contributions(t, shard1, shard2)
	c1, c2 := c[0], c[1]

	require.NotEqual(t, c1, c2,
		"the two shards hold different bus traffic, so their contributions must differ; "+
			"equal contributions mean the PCS pass dropped out and both degenerated to Hash(0)")

	// γ from the shards, through the production preflight path.
	g := preflight.Run(busInputSets(t, shard1, shard2))

	// Bind the shards to the γ their own contributions produced.
	rt1, proof1, pub1 := shard1.runProve(t, g)
	rt2, proof2, pub2 := shard2.runProve(t, g)

	require.NoError(t, shard1.sys.Verify(proof1, pub1),
		"shard 1 must verify against the γ derived from both contributions")
	require.NoError(t, shard2.sys.Verify(proof2, pub2),
		"shard 2 must verify against the γ derived from both contributions")

	// The contributions read back out of the public inputs are the ones preflight
	// computed — so a contribution does not move with γ, and the γ above is not
	// circular.
	require.Equal(t, c1, shard1.getContributionFromPI(t, rt1),
		"shard 1 must publish the registered contribution")
	require.Equal(t, c2, shard2.getContributionFromPI(t, rt2),
		"shard 2 must publish the registered contribution")

	// γ itself is readable back from the public inputs, which is how an aggregator
	// checks two shards were handed the same seed.
	require.Equal(t, g, shard1.readSeedFromPI(t, rt1), "shard 1 must publish the γ it was given")
	require.Equal(t, g, shard2.readSeedFromPI(t, rt2), "shard 2 must publish the γ it was given")

	// The pair settles exactly, per handle. This is the only place the shards fold
	// their rows under α and β actually derived from a γ, with the PCS commitments
	// in the transcript: had the seeding failed, the two shards would fold under
	// different challenges and the products would not be inverses.
	require.Len(t, shard1.handles, len(crossShardHandles))
	for i, h := range shard1.handles {
		t.Run(h, func(t *testing.T) {
			p1 := shard1.getBusAccFromSys(rt1, i)
			p2 := shard2.getBusAccFromSys(rt2, i)

			require.False(t, equal(p1, field.ElemOne()),
				"shard 1 carries a net position on %q, so a product of one would mean the "+
					"folds degenerated and the inverse check below is vacuous", h)
			require.True(t, equal(p1.Mul(p2), field.ElemOne()),
				"the shards' net positions on %q must be inverses under the shared α and β", h)
		})
	}
}

// TestSharedRandomness_SeedDerivedFromContributions_Unbalanced is the soundness
// counterpart: on handle "route" shard 2 receives 41 where shard 1 sent 40, so
// that handle's union of receives no longer matches its union of sends.
//
// Both shards suppress their in-shard checks, so each still proves and verifies
// on its own even under a γ derived from the pair — the imbalance is invisible
// locally by construction. It surfaces only at the cross-shard join, and only on
// the tampered handle: "wire" shares α and β with "route" and must still settle,
// which is what keeps a break in one handle from smearing into the other.
func TestSharedRandomness_SeedDerivedFromContributions_Unbalanced(t *testing.T) {
	tampered := []busTraffic{
		{handle: "route", sent: []uint64{50, 60, 70, 80}, received: []uint64{30, 41, 70, 80}}, // 40 → 41
		{handle: "wire", sent: []uint64{150, 160, 170, 180}, received: []uint64{130, 140, 170, 180}},
	}

	shard1 := buildSeededBidirectionalShard(
		t, "shard-1-bidir-unbalanced", "shard-1", crossShardTrafficShard1)
	shard2 := buildSeededBidirectionalShard(
		t, "shard-2-bidir-unbalanced", "shard-2", tampered)

	g := preflight.Run(busInputSets(t, shard1, shard2))

	rt1, proof1, pub1 := shard1.runProve(t, g)
	rt2, proof2, pub2 := shard2.runProve(t, g)

	// Each shard is locally consistent: the tampering is not something a shard can
	// detect about itself once the in-shard check is deferred.
	require.NoError(t, shard1.sys.Verify(proof1, pub1),
		"a shard that defers its in-shard check must verify even when the pair is unbalanced")
	require.NoError(t, shard2.sys.Verify(proof2, pub2),
		"a shard that defers its in-shard check must verify even when the pair is unbalanced")

	// Both shards agreed on γ, so they folded under the same α and β — which is
	// what makes the join below meaningful rather than a comparison of unrelated
	// products.
	require.Equal(t, shard1.readSeedFromPI(t, rt1), shard2.readSeedFromPI(t, rt2),
		"both shards must have been handed the same γ")

	byHandle := func(s *seededShard, rt *wiop.Runtime, h string) field.Gen {
		for i, name := range s.handles {
			if name == h {
				return s.getBusAccFromSys(rt, i)
			}
		}
		t.Fatalf("handle %q not found", h)
		return field.Gen{}
	}

	routeJoin := byHandle(shard1, rt1, "route").Mul(byHandle(shard2, rt2, "route"))
	require.False(t, equal(routeJoin, field.ElemOne()),
		"a row received on no shard leaves route's joint product different from one")

	wireJoin := byHandle(shard1, rt1, "wire").Mul(byHandle(shard2, rt2, "wire"))
	require.True(t, equal(wireJoin, field.ElemOne()),
		"wire is untouched and must still settle, despite sharing α and β with route")
}
