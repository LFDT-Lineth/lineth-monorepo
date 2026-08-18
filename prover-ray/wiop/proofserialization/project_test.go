package proofserialization_test

import (
	"testing"

	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/maths/koalabear/field"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/compilers/pcs"
	ps "github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/proofserialization"
	"github.com/LFDT-Lineth/lineth-monorepo/prover-ray/wiop/wioptest"
	"github.com/stretchr/testify/require"
)

// TestProjectEncodeDecode_EndToEnd is the full pipeline on a real proof:
// prove -> project -> encode -> decode, checking the decoded image against the
// projection it came from.
//
// Everything before this exercised the encoder on hand-built values. This is the
// first test where the bytes come from an actual PCS-compiled proof, so it is
// what catches a projection that produces something the encoder cannot faithfully
// represent.
func TestProjectEncodeDecode_EndToEnd(t *testing.T) {
	// Every scenario, but the deep struct comparison only on a few: with 229 FRI
	// queries each projected proof holds hundreds of thousands of values, and
	// comparing all of them across all scenarios dominates the suite's runtime.
	// The byte-level image identity below is the cheap check that still catches
	// an encoder that is not a function of its input.
	const deepCompareFirst = 3
	for idx, build := range wioptest.VanishingScenarios() {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)
			require.NoError(t, sc.Sys.Verify(proof, pub), "the projected proof must be a valid one")

			projected, err := ps.Project(sc.Sys, proof, pub)
			require.NoError(t, err)

			image, err := ps.Encode(projected, ps.GuestBase)
			require.NoError(t, err)
			require.NoError(t, ps.Validate(image, ps.GuestBase),
				"an image we just produced must pass the validator")

			got, err := ps.Decode(image, ps.GuestBase)
			require.NoError(t, err)

			again, err := ps.Encode(got, ps.GuestBase)
			require.NoError(t, err)
			require.Equal(t, image, again, "re-encoding a decoded image must be byte-identical")

			if idx < deepCompareFirst {
				require.Equal(t, projected, got, "the image must round-trip a real proof")
			}
		})
	}
}

// TestProject_CarriesTheProofFaithfully checks the projection against the
// wiop.Proof it came from, rather than only against itself via the round trip.
func TestProject_CarriesTheProofFaithfully(t *testing.T) {
	sc := wioptest.VanishingScenarios()[0]()
	compileFullPipeline(sc.Sys)
	proof, pub := sc.Sys.Prove(sc.AssignHonest)
	require.NoError(t, sc.Sys.Verify(proof, pub))

	projected, err := ps.Project(sc.Sys, proof, pub)
	require.NoError(t, err)

	require.Len(t, projected.Proof.Rounds, len(sc.Sys.Rounds), "one round message per round")

	// Cells are round-major and in declaration order, but OMIT public inputs:
	// verifier.Proof documents that rounds[*].cells excludes them and the flat
	// PublicInputs statement supplies them instead. Carrying them in both places
	// would absorb them twice and desynchronise the transcript replay.
	total := 0
	for _, rm := range projected.Proof.Rounds {
		total += len(rm.Cells)
	}
	require.Equal(t, len(proof.Cells), total,
		"every non-public-input cell must appear exactly once")
	require.Len(t, projected.PublicInputs, len(pub),
		"the public-input statement must carry one entry per registered cell")
	for i := range pub {
		require.Equal(t, ps.ScalarFrom(pub[i]), projected.PublicInputs[i],
			"public input %d must keep its registration-order slot", i)
	}

	// One Merkle root per committed round, and none anywhere else.
	commitments := 0
	for i, rm := range projected.Proof.Rounds {
		if _, committed := proof.Commitments[i]; committed {
			require.NotNil(t, rm.Commitment, "round %d commits, so it carries its root", i)
			require.Equal(t, ps.DigestFrom(proof.Commitments[i]), *rm.Commitment)
			commitments++
		} else {
			require.Nil(t, rm.Commitment, "round %d does not commit", i)
		}
	}
	require.Equal(t, len(proof.Commitments), commitments)

	require.Len(t, projected.Proof.ModuleSizes, len(proof.DynamicSizes),
		"one size per dynamic module")

	// The FRI opening must survive with its shape intact.
	require.NotNil(t, proof.PCSOpeningProof, "the full pipeline ends with the PCS pass")
	require.Len(t, projected.Proof.PcsOpening.Proof.InputQueries, pcs.FRINumQueries())
	require.Len(t, projected.Proof.PcsOpening.Proof.FriProof.RunningQueries, pcs.FRINumQueries())
	require.Len(t, projected.Proof.PcsOpening.Proof.FriProof.RoundRoots,
		len(proof.PCSOpeningProof.FRIProof.RoundRoots))
}

// TestProject_RejectsIncompleteInput pins the failure modes, so a malformed
// projection is reported rather than encoded into a plausible-looking image.
//
// This matters more than a usual input check: a missing cell silently encoded as
// zero would produce a well-formed image that fails Fiat-Shamir replay inside the
// guest, with nothing pointing back at the projection.
func TestProject_RejectsIncompleteInput(t *testing.T) {
	sc := wioptest.VanishingScenarios()[0]()
	compileFullPipeline(sc.Sys)
	proof, pub := sc.Sys.Prove(sc.AssignHonest)

	t.Run("nil system", func(t *testing.T) {
		_, err := ps.Project(nil, proof, pub)
		require.ErrorContains(t, err, "nil system")
	})

	t.Run("public input count mismatch", func(t *testing.T) {
		// Appending unconditionally: this scenario may declare none, and an
		// append of an empty slice would leave the length unchanged and quietly
		// assert nothing.
		tooMany := append(append(wiop.PublicInput{}, pub...), field.ElemFromBase(field.NewElement(1)))
		_, err := ps.Project(sc.Sys, proof, tooMany)
		require.ErrorContains(t, err, "public inputs")
	})

	t.Run("cell missing from the proof", func(t *testing.T) {
		cells := make(map[wiop.ObjectID]field.Gen, len(proof.Cells))
		for k, v := range proof.Cells {
			cells[k] = v
		}
		for k := range cells {
			delete(cells, k)
			break
		}
		require.Less(t, len(cells), len(proof.Cells), "the fixture must have a cell to drop")

		_, err := ps.Project(sc.Sys, wiop.Proof{
			Cells:           cells,
			DynamicSizes:    proof.DynamicSizes,
			Commitments:     proof.Commitments,
			PCSOpeningProof: proof.PCSOpeningProof,
		}, pub)
		require.ErrorContains(t, err, "absent from the proof",
			"a dropped cell must be named, not encoded as a zero")
	})

	t.Run("dynamic module size missing", func(t *testing.T) {
		if len(proof.DynamicSizes) == 0 {
			t.Skip("this scenario has no dynamic modules")
		}
		sizes := make(map[int]int, len(proof.DynamicSizes))
		for k, v := range proof.DynamicSizes {
			sizes[k] = v
		}
		for k := range sizes {
			delete(sizes, k)
			break
		}

		_, err := ps.Project(sc.Sys, wiop.Proof{
			Cells:           proof.Cells,
			DynamicSizes:    sizes,
			Commitments:     proof.Commitments,
			PCSOpeningProof: proof.PCSOpeningProof,
		}, pub)
		require.ErrorContains(t, err, "no size in the proof")
	})
}

// TestMeasureAgreesWithEncode cross-validates the size model against the encoder.
//
// Measure is arithmetic over the proof's shape; Encode produces actual bytes.
// Nothing forced them to agree, so the numbers reported in the spec rested on the
// model being right. Encode should come out slightly larger, the difference being
// alignment padding, which Measure does not model.
func TestMeasureAgreesWithEncode(t *testing.T) {
	for _, build := range wioptest.VanishingScenarios()[:5] {
		sc := build()
		t.Run(sc.Name, func(t *testing.T) {
			compileFullPipeline(sc.Sys)
			proof, pub := sc.Sys.Prove(sc.AssignHonest)

			stats := ps.Measure(sc.Sys, proof, pub)

			projected, err := ps.Project(sc.Sys, proof, pub)
			require.NoError(t, err)
			image, err := ps.Encode(projected, ps.GuestBase)
			require.NoError(t, err)

			padding := len(image) - stats.Total
			t.Logf("%-28s model %8d B, encoded %8d B, padding %5d B, overhead %.1f%%",
				sc.Name, stats.Total, len(image), padding,
				100*float64(stats.Overhead())/float64(stats.Total))

			// The model may under-count by alignment padding, which it does not
			// track, but must never over-count: that would inflate every size
			// figure the spec quotes.
			require.GreaterOrEqual(t, len(image), stats.Total,
				"the model over-counts by %d bytes -- it is claiming bytes the encoder "+
					"does not write", -padding)
			require.Less(t, padding, stats.Total/100,
				"padding should be a rounding error; %d bytes on a %d-byte model means the "+
					"model is missing something structural", padding, stats.Total)
		})
	}
}

// TestImageShapeIsCircuitDependent records how the image's structure varies with
// the circuit, since an earlier draft of the spec claimed it did not.
func TestImageShapeIsCircuitDependent(t *testing.T) {
	type shape struct {
		name       string
		perQuery   int
		total      int
		overheadPc float64
	}
	var shapes []shape

	for _, build := range wioptest.VanishingScenarios() {
		sc := build()
		compileFullPipeline(sc.Sys)
		proof, pub := sc.Sys.Prove(sc.AssignHonest)
		s := ps.Measure(sc.Sys, proof, pub)
		if !s.HasPCS || s.Queries == 0 {
			continue
		}
		shapes = append(shapes, shape{
			name:       sc.Name,
			perQuery:   s.LeafSlots / s.Queries,
			total:      s.Total,
			overheadPc: 100 * float64(s.Overhead()) / float64(s.Total),
		})
	}
	require.NotEmpty(t, shapes)

	minPer, maxPer := shapes[0].perQuery, shapes[0].perQuery
	for _, s := range shapes {
		minPer = min(minPer, s.perQuery)
		maxPer = max(maxPer, s.perQuery)
		t.Logf("%-30s %3d leaf slots/query, %8d B, overhead %5.1f%%",
			s.name, s.perQuery, s.total, s.overheadPc)
	}

	require.Greater(t, maxPer, minPer,
		"leaf slots per query (input trees x tree depth) must vary with the circuit; if this "+
			"ever stops being true the spec's size model can be simplified")
	t.Logf("leaf slots per query range: %d..%d across %d circuits", minPer, maxPer, len(shapes))
}
