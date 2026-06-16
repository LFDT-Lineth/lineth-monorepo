package fiatshamirrefactor

import (
	"errors"
	"reflect"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/linea-monorepo/prover-ray/crypto/koalabear/hash"
)

type recordingHasher struct {
	current []koalabear.Element
	inputs  [][]koalabear.Element
}

func (h *recordingHasher) Reset() {
	h.current = h.current[:0]
}

func (h *recordingHasher) WriteElements(elements ...koalabear.Element) {
	h.current = append(h.current, elements...)
}

func (h *recordingHasher) WriteExt(elements ...ext.E6) {
	for _, e := range elements {
		h.current = hash.AppendExtElements(h.current, e)
	}
}

func (h *recordingHasher) Sum() hash.Digest {
	input := append([]koalabear.Element(nil), h.current...)
	h.inputs = append(h.inputs, input)

	var out hash.Digest
	for i := range out {
		out[i].SetUint64(uint64(100*len(h.inputs) + i + 1))
	}
	return out
}

func TestComputeChallengeAbsorbsChallengeID(t *testing.T) {
	hasher := &recordingHasher{}
	transcript := NewTranscript(hasher, "zeta", "alpha_DEEP")

	zetaBinding := testElement(11)
	if err := transcript.Bind("zeta", []koalabear.Element{zetaBinding}); err != nil {
		t.Fatalf("bind zeta: %v", err)
	}
	zeta, err := transcript.ComputeChallenge("zeta")
	if err != nil {
		t.Fatalf("compute zeta: %v", err)
	}

	alphaBinding := testElement(22)
	if err := transcript.Bind("alpha_DEEP", []koalabear.Element{alphaBinding}); err != nil {
		t.Fatalf("bind alpha: %v", err)
	}
	if _, err := transcript.ComputeChallenge("alpha_DEEP"); err != nil {
		t.Fatalf("compute alpha: %v", err)
	}

	if len(hasher.inputs) != 2 {
		t.Fatalf("expected 2 hash inputs, got %d", len(hasher.inputs))
	}

	expectedZetaInput := append(hash.StringToElements(challengeIDDomainTag, "zeta"), zetaBinding)
	if !reflect.DeepEqual(hasher.inputs[0], expectedZetaInput) {
		t.Fatalf("unexpected zeta input:\nwant %#v\n got %#v", expectedZetaInput, hasher.inputs[0])
	}

	expectedAlphaInput := append(hash.StringToElements(challengeIDDomainTag, "alpha_DEEP"), zeta[:]...)
	expectedAlphaInput = append(expectedAlphaInput, alphaBinding)
	if !reflect.DeepEqual(hasher.inputs[1], expectedAlphaInput) {
		t.Fatalf("unexpected alpha input:\nwant %#v\n got %#v", expectedAlphaInput, hasher.inputs[1])
	}
}

func TestChallengeIDDomainSeparation(t *testing.T) {
	binding := testElement(7)

	hasherA := hash.NewPoseidon2SpongeHasher()
	transcriptA := NewTranscript(&hasherA, "a")
	if err := transcriptA.Bind("a", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind a: %v", err)
	}
	a, err := transcriptA.ComputeChallenge("a")
	if err != nil {
		t.Fatalf("compute a: %v", err)
	}

	hasherB := hash.NewPoseidon2SpongeHasher()
	transcriptB := NewTranscript(&hasherB, "b")
	if err := transcriptB.Bind("b", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind b: %v", err)
	}
	b, err := transcriptB.ComputeChallenge("b")
	if err != nil {
		t.Fatalf("compute b: %v", err)
	}

	if a == b {
		t.Fatalf("different challenge IDs produced the same challenge")
	}
}

func TestComputeChallengeWithGrindingRecordsAndReplaysProofOfWork(t *testing.T) {
	const nbBits = 6

	binding := testElement(42)
	hasher := hash.NewPoseidon2SpongeHasher()
	transcript := NewTranscript(&hasher, "zeta")
	if err := transcript.Bind("zeta", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind zeta: %v", err)
	}

	zeta, err := transcript.ComputeChallenge("zeta", WithGrinding(nbBits))
	if err != nil {
		t.Fatalf("compute zeta with grinding: %v", err)
	}
	if !hasZeroGrindingBits(hash.Digest(zeta), nbBits) {
		t.Fatalf("challenge does not satisfy %d bits of grinding", nbBits)
	}

	pow, ok := transcript.ProofOfWork("zeta")
	if !ok {
		t.Fatalf("expected proof of work to be recorded")
	}
	if pow.NbBits != nbBits {
		t.Fatalf("unexpected proof-of-work bits: want %d, got %d", nbBits, pow.NbBits)
	}

	replayHasher := hash.NewPoseidon2SpongeHasher()
	replay := NewTranscript(&replayHasher, "zeta")
	if err := replay.Bind("zeta", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind replay zeta: %v", err)
	}
	if err := replay.SetProofOfWork("zeta", pow); err != nil {
		t.Fatalf("set proof of work: %v", err)
	}
	replayedZeta, err := replay.ComputeChallenge("zeta", WithGrinding(nbBits))
	if err != nil {
		t.Fatalf("replay zeta with grinding: %v", err)
	}
	if replayedZeta != zeta {
		t.Fatalf("replayed challenge mismatch")
	}
}

func TestComputeChallengeRejectsInvalidProofOfWork(t *testing.T) {
	const nbBits = 6

	binding := testElement(99)
	hasher := hash.NewPoseidon2SpongeHasher()
	transcript := NewTranscript(&hasher, "zeta")
	if err := transcript.Bind("zeta", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind zeta: %v", err)
	}

	var invalidPow ProofOfWork
	invalidPow.NbBits = nbBits
	for saltValue := uint64(0); ; saltValue++ {
		invalidPow.Salt.SetUint64(saltValue)
		value, err := transcript.computeChallengeDigest("zeta", 0, transcript.challenges[0], &invalidPow)
		if err != nil {
			t.Fatalf("compute candidate challenge: %v", err)
		}
		if !hasZeroGrindingBits(value, nbBits) {
			break
		}
	}

	replayHasher := hash.NewPoseidon2SpongeHasher()
	replay := NewTranscript(&replayHasher, "zeta")
	if err := replay.Bind("zeta", []koalabear.Element{binding}); err != nil {
		t.Fatalf("bind replay zeta: %v", err)
	}
	if err := replay.SetProofOfWork("zeta", invalidPow); err != nil {
		t.Fatalf("set invalid proof of work: %v", err)
	}
	if _, err := replay.ComputeChallenge("zeta", WithGrinding(nbBits)); !errors.Is(err, errInvalidProofOfWork) {
		t.Fatalf("expected invalid proof-of-work error, got %v", err)
	}
}

func testElement(v uint64) koalabear.Element {
	var e koalabear.Element
	e.SetUint64(v)
	return e
}
