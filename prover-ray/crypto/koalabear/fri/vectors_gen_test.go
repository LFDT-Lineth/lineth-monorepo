//go:build gen_vectors

// Package fri (gen_vectors build) generates JSON test vectors for
// verifier-ray's Zig low-degree-test core (crypto/merkle.zig, query/fri.zig).
//
// This lives in a white-box test file (package fri, not fri_test) because it
// needs package-private access: newCompleteBinaryTree (the plain
// running-layer tree verifier-ray's crypto.merkle mirrors), ProverState's
// internal per-round layers, and Level.EvalsAt (the DEEP-quotient
// combination a level applies before folding). None of that is reachable
// from outside the package, and none of it should be exported just to
// generate fixtures.
//
// The gen_vectors build tag keeps this file (and its encoding/json and os
// imports) out of every ordinary `go test ./...` run. Invoke explicitly:
//
//	go test -tags gen_vectors -run ^TestGenerateVectors$ .
package fri

import (
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/crypto/koalabear/poseidon2"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
)

// ─── JSON wire types ─────────────────────────────────────────────────────────
//
// Field/Ext elements are serialized as their canonical uint64 representation
// (KoalaBear's modulus is ~2^31, comfortably inside a JSON number); Octuplets
// and Exts flatten to fixed-length arrays in the same coordinate order
// verifier-ray's Zig types already use (Ext: [B0.a0, B0.a1, B1.a0, B1.a1,
// B2.a0, B2.a1], matching field.ExtToUint64s).

type jsonOctuplet [8]uint64
type jsonExt [6]uint64

type jsonBranch struct {
	Leaf     jsonOctuplet   `json:"leaf"`
	Siblings []jsonOctuplet `json:"siblings"`
}

type jsonPair struct {
	Self    jsonExt `json:"self"`
	Sibling jsonExt `json:"sibling"`
}

// jsonMerkleCase exercises crypto.merkle.Branch.recoverRoot against a tree
// prover-ray itself built and hashed (newCompleteBinaryTree), independent of
// verifier-ray's own Poseidon2 implementation.
type jsonMerkleCase struct {
	Name        string         `json:"name"`
	Leaf        jsonOctuplet   `json:"leaf"`
	Siblings    []jsonOctuplet `json:"siblings"`
	Index       int            `json:"index"`
	Root        jsonOctuplet   `json:"root"`
	ExpectMatch bool           `json:"expect_match"`
	ExpectError string         `json:"expect_error,omitempty"`
}

// jsonFoldCase exercises query.fri's checkOpeningProofShape,
// resolveRunningLayers, and checkFolds against a real multi-round FRI proof.
//
// LogCodewordSize/NumRounds/LogFinalPolySize/NumQueries are carried for the
// Zig test to cross-check against its own hardcoded, comptime `fri.Params`
// for this case name -- Params itself is not meant to travel through JSON,
// since checkOpeningProofShape/resolveRunningLayers/checkFolds all take it as
// a comptime parameter. Only the case contents below are runtime data.
type jsonFoldCase struct {
	Name             string `json:"name"`
	LogCodewordSize  uint8  `json:"log_codeword_size"`
	NumRounds        uint8  `json:"num_rounds"`
	LogFinalPolySize uint8  `json:"log_final_poly_size"`
	NumQueries       int    `json:"num_queries"`

	FoldAlphas      []jsonExt      `json:"fold_alphas"`      // len = NumRounds
	RoundRoots      []jsonOctuplet `json:"round_roots"`      // len = NumRounds-1
	FinalPoly       []jsonExt      `json:"final_poly"`       // len = 1<<LogFinalPolySize
	Position        int            `json:"position"`
	RunningBranches []jsonBranch   `json:"running_branches"` // len = NumRounds-1; round j's branch at index j-1

	// ExpectedRounds is the golden output of resolveRunningLayers: rounds[0]
	// is always the zero pair (never read; round 0 always has a level), and
	// rounds[j] for j >= 1 is decoded from RunningBranches[j-1].
	ExpectedRounds []jsonPair  `json:"expected_rounds"`
	Aux            []*jsonPair `json:"aux"` // len = NumRounds; nil where no level is introduced

	// Exactly one of these is set for a case expected to fail; both empty
	// means the whole honest sequence (shape, running layers, fold) must
	// accept.
	ExpectRunningError string `json:"expect_running_error,omitempty"`
	ExpectFoldError    string `json:"expect_fold_error,omitempty"`
}

type vectorFile struct {
	MerkleCases []jsonMerkleCase `json:"merkle_cases"`
	FoldCases   []jsonFoldCase   `json:"fold_cases"`
}

// outputPath is relative to this package's directory (go test's working
// directory), which is fixed by the repository layout: prover-ray and
// verifier-ray are sibling modules under the monorepo root.
const outputPath = "../../../../verifier-ray/testdata/generated/fri_vectors.json"

// TestGenerateVectors writes verifier-ray's Zig test vectors for this
// package's low-degree-test core. Only ever compiled in under the
// gen_vectors build tag (see the package doc comment above), so it plays no
// part in an ordinary `go test` run.
func TestGenerateVectors(t *testing.T) {
	vf := vectorFile{
		MerkleCases: buildMerkleCases(),
		FoldCases:   buildFoldCases(t),
	}

	data, err := json.MarshalIndent(vf, "", "  ")
	if err != nil {
		t.Fatalf("marshal vectors: %v", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		t.Fatalf("write %s: %v", outputPath, err)
	}
}

// ─── field/octuplet/ext <-> JSON, and small deterministic constructors ──────

func toJSONOctuplet(o field.Octuplet) jsonOctuplet {
	var out jsonOctuplet
	for i, e := range o {
		out[i] = e.Uint64()
	}
	return out
}

func toJSONOctuplets(os []field.Octuplet) []jsonOctuplet {
	out := make([]jsonOctuplet, len(os))
	for i, o := range os {
		out[i] = toJSONOctuplet(o)
	}
	return out
}

func toJSONExt(e field.Ext) jsonExt {
	a0, a1, b0, b1, c0, c1 := field.ExtToUint64s(&e)
	return jsonExt{a0, a1, b0, b1, c0, c1}
}

func elem(v uint64) field.Element {
	var e field.Element
	e.SetUint64(v)
	return e
}

func extLift(v uint64) field.Ext {
	return field.Lift(elem(v))
}

// hashOne hashes a single small integer into a leaf octuplet via prover-ray's
// own Poseidon2, independent of whatever verifier-ray's Zig side does.
func hashOne(v uint64) field.Octuplet {
	h := poseidon2.NewMDHasher()
	h.WriteElements(elem(v))
	return h.SumDigest()
}

// wrongOctuplet/wrongExt stand in for "some value that must not match the
// honest one"; the exact value is immaterial as long as it differs.
func wrongOctuplet() jsonOctuplet { return toJSONOctuplet(hashOne(999_999)) }
func wrongExt() jsonExt           { return toJSONExt(extLift(999_999)) }

// ─── Merkle cases: newCompleteBinaryTree's own public-ish surface ──────────
//
// NewTree/OpenBranch/Root are exported, but the plain (no-aux) binary tree
// verifier-ray's crypto.merkle.Branch mirrors is only reachable through the
// unexported newCompleteBinaryTree -- hence generating from inside the
// package rather than from testdata/generate.

func buildMerkleCases() []jsonMerkleCase {
	var cases []jsonMerkleCase

	// Two-leaf tree: both parities in a single level.
	{
		leaves := []field.Octuplet{hashOne(1), hashOne(2)}
		tree := newCompleteBinaryTree(leaves)
		root := tree.Root()
		for _, idx := range []int{0, 1} {
			b := tree.OpenBranch(idx)
			cases = append(cases, jsonMerkleCase{
				Name:        fmt.Sprintf("two_leaf_index_%d", idx),
				Leaf:        toJSONOctuplet(b.Leaf),
				Siblings:    toJSONOctuplets(b.Siblings),
				Index:       idx,
				Root:        toJSONOctuplet(root),
				ExpectMatch: true,
			})
		}

		b := tree.OpenBranch(0)
		siblings := append([]field.Octuplet(nil), b.Siblings...)
		siblings[len(siblings)-1] = hashOne(999_999)
		cases = append(cases, jsonMerkleCase{
			Name:        "two_leaf_wrong_sibling",
			Leaf:        toJSONOctuplet(b.Leaf),
			Siblings:    toJSONOctuplets(siblings),
			Index:       0,
			Root:        toJSONOctuplet(root),
			ExpectMatch: false,
		})
	}

	// Four-leaf tree: a deeper tree, opened at two positions of different
	// parity so both branches of RecoverRoot's swap get exercised.
	{
		leaves := []field.Octuplet{hashOne(10), hashOne(20), hashOne(30), hashOne(40)}
		tree := newCompleteBinaryTree(leaves)
		root := tree.Root()
		for _, idx := range []int{1, 2} {
			b := tree.OpenBranch(idx)
			cases = append(cases, jsonMerkleCase{
				Name:        fmt.Sprintf("four_leaf_index_%d", idx),
				Leaf:        toJSONOctuplet(b.Leaf),
				Siblings:    toJSONOctuplets(b.Siblings),
				Index:       idx,
				Root:        toJSONOctuplet(root),
				ExpectMatch: true,
			})
		}
	}

	// A branch with no siblings must be rejected before any hashing; no tree
	// needed for this shape check.
	cases = append(cases, jsonMerkleCase{
		Name:        "empty_branch",
		Leaf:        toJSONOctuplet(hashOne(1)),
		Siblings:    nil,
		Index:       0,
		ExpectError: "EmptyBranch",
	})

	return cases
}

// ─── Fold cases: a real multi-round, multi-level FRI proof ─────────────────
//
// Levels are built directly (bypassing the PCS/DEEP-quotient layer entirely,
// since that is out of scope for the low-degree-test core this generates
// vectors for): each level is a single quotientColumn with zero Claims, which
// makes EvalsAt a pure alphaDeep-scaling of the running codeword -- exactly
// as meaningful a test of checkFolds' arithmetic as a real DEEP-reconstructed
// level, since checkFolds never looks at how a level's pair was derived.

func buildFoldCases(t *testing.T) []jsonFoldCase {
	t.Helper()

	base := buildHonestFoldCase(t, "single_level_3rounds", 4, 3, []int{16}, 13, []uint64{7, 11, 17})
	twoLevels := buildHonestFoldCase(t, "two_levels_3rounds", 4, 3, []int{16, 4}, 6, []uint64{7, 11, 17})

	cases := []jsonFoldCase{base, twoLevels}
	cases = append(cases, corruptRunningSibling(base))
	cases = append(cases, corruptRoundRoot(base))
	cases = append(cases, corruptAux(base))
	cases = append(cases, corruptFinal(base))
	return cases
}

// buildHonestFoldCase runs a real ProverState: it builds `levelSizes`
// (codeword lengths at which each level is introduced; levelSizes[0] must be
// 1<<logCodewordSize, the top level) directly as trivial zero-claim Levels,
// folds with the given per-round challenges, opens at `position`, and reads
// off exactly the values query.fri's functions need: the running-layer
// branches and roots from the real Proof, and the exact resolved
// rounds/aux pairs by re-deriving them from ProverState's own internal
// per-round layers (the same data Fold itself folds), not by recomputing the
// fold independently.
func buildHonestFoldCase(
	t *testing.T,
	name string,
	logCodewordSize, logPlainTextSize uint8,
	levelSizes []int,
	position int,
	foldAlphaSeeds []uint64,
) jsonFoldCase {
	t.Helper()

	params, err := NewParams(logCodewordSize, logPlainTextSize, 1)
	if err != nil {
		t.Fatalf("%s: NewParams: %v", name, err)
	}

	levels := make([]Level, len(levelSizes))
	for i, size := range levelSizes {
		levels[i] = Level{
			Trees:   []*Tree{{}}, // unused post aux-pairs refactor; buildProvePlan only checks non-nil
			Columns: []quotientColumn{{Evals: make([]field.Ext, size)}},
		}
	}

	st, err := NewProverState(params, levels)
	if err != nil {
		t.Fatalf("%s: NewProverState: %v", name, err)
	}

	numRounds := int(params.numRounds())
	if len(foldAlphaSeeds) != numRounds {
		t.Fatalf("%s: need %d fold alphas, got %d", name, numRounds, len(foldAlphaSeeds))
	}

	foldAlphas := make([]field.Ext, numRounds)
	combinedAtRound := make(map[uint8][]field.Ext) // round -> post-injection (pre-fold) codeword

	for st.HasNext() {
		j := st.round
		alpha := extLift(foldAlphaSeeds[j])
		foldAlphas[j] = alpha

		if l, ok := st.plan.levelAtRound[j]; ok {
			var alphaDeep field.Ext
			alphaDeep.Square(&alpha)
			// Mirrors exactly what Fold computes internally for `primary`,
			// using the same inputs (st.layers[j], this round's alphaDeep),
			// captured here so the vector can record it.
			combinedAtRound[j] = st.levels[l].EvalsAt(alphaDeep, st.layers[j])
		}

		st.Fold(alpha)
	}

	proof := st.Open([]int{position})

	rounds := make([]jsonPair, numRounds)
	for j := 1; j < numRounds; j++ {
		base := position >> uint(j)
		layer := st.layers[j]
		rounds[j] = jsonPair{Self: toJSONExt(layer[base]), Sibling: toJSONExt(layer[base^1])}
	}

	aux := make([]*jsonPair, numRounds)
	for j := 0; j < numRounds; j++ {
		combined, ok := combinedAtRound[uint8(j)]
		if !ok {
			continue
		}
		base := position >> uint(j)
		aux[j] = &jsonPair{Self: toJSONExt(combined[base]), Sibling: toJSONExt(combined[base^1])}
	}

	runningBranches := make([]jsonBranch, numRounds-1)
	for j := 1; j < numRounds; j++ {
		branch := proof.RunningQueries[0][j-1][0]
		runningBranches[j-1] = jsonBranch{Leaf: toJSONOctuplet(branch.Leaf), Siblings: toJSONOctuplets(branch.Siblings)}
	}

	jsonAlphas := make([]jsonExt, numRounds)
	for j, a := range foldAlphas {
		jsonAlphas[j] = toJSONExt(a)
	}

	if len(proof.FinalPoly) != 1 {
		t.Fatalf("%s: generator only supports LogFinalPolySize=0, got %d final coefficients", name, len(proof.FinalPoly))
	}

	return jsonFoldCase{
		Name:             name,
		LogCodewordSize:  logCodewordSize,
		NumRounds:        uint8(numRounds),
		LogFinalPolySize: 0,
		NumQueries:       1,
		FoldAlphas:       jsonAlphas,
		RoundRoots:       toJSONOctuplets(proof.RoundRoots),
		FinalPoly:        []jsonExt{toJSONExt(proof.FinalPoly[0])},
		Position:         position,
		RunningBranches:  runningBranches,
		ExpectedRounds:   rounds,
		Aux:              aux,
	}
}

// cloneFoldCase deep-copies every slice/pointer field so a corrupted
// derivative case can be mutated without aliasing the honest case it came
// from.
func cloneFoldCase(base jsonFoldCase, name string) jsonFoldCase {
	c := base
	c.Name = name
	c.FoldAlphas = append([]jsonExt(nil), base.FoldAlphas...)
	c.RoundRoots = append([]jsonOctuplet(nil), base.RoundRoots...)
	c.FinalPoly = append([]jsonExt(nil), base.FinalPoly...)

	c.RunningBranches = make([]jsonBranch, len(base.RunningBranches))
	for i, b := range base.RunningBranches {
		c.RunningBranches[i] = jsonBranch{Leaf: b.Leaf, Siblings: append([]jsonOctuplet(nil), b.Siblings...)}
	}

	c.ExpectedRounds = append([]jsonPair(nil), base.ExpectedRounds...)

	c.Aux = make([]*jsonPair, len(base.Aux))
	for i, a := range base.Aux {
		if a == nil {
			continue
		}
		cp := *a
		c.Aux[i] = &cp
	}
	return c
}

// corruptRunningSibling breaks round 1's branch (present in every case with
// num_rounds >= 2): resolveRunningLayers must reject it while re-deriving
// the root, before any fold arithmetic runs.
func corruptRunningSibling(base jsonFoldCase) jsonFoldCase {
	c := cloneFoldCase(base, "wrong_running_sibling")
	siblings := c.RunningBranches[0].Siblings
	siblings[len(siblings)-1] = wrongOctuplet()
	c.ExpectRunningError = "MerkleProofInvalid"
	return c
}

// corruptRoundRoot breaks the committed root itself rather than the branch:
// a different authentication-failure code path from corruptRunningSibling,
// same observable error.
func corruptRoundRoot(base jsonFoldCase) jsonFoldCase {
	c := cloneFoldCase(base, "wrong_round_root")
	c.RoundRoots[0] = wrongOctuplet()
	c.ExpectRunningError = "MerkleProofInvalid"
	return c
}

// corruptAux breaks the level pair introduced at round 0 (always present):
// checkFolds must reject it once its fold no longer matches round 1's leaf.
func corruptAux(base jsonFoldCase) jsonFoldCase {
	c := cloneFoldCase(base, "wrong_aux")
	cp := *c.Aux[0]
	cp.Self = wrongExt()
	c.Aux[0] = &cp
	c.ExpectFoldError = "FoldMismatch"
	return c
}

// corruptFinal breaks the revealed final polynomial: checkFolds must reject
// it at the last round.
func corruptFinal(base jsonFoldCase) jsonFoldCase {
	c := cloneFoldCase(base, "wrong_final")
	c.FinalPoly[0] = wrongExt()
	c.ExpectFoldError = "FinalPolyMismatch"
	return c
}
