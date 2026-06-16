package hash

import (
	"crypto/sha256"
	"testing"

	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

func TestPoseidon2MDHasherSingleBlockMatchesPreviousCompression(t *testing.T) {
	for _, n := range []int{0, 1, WIDTH / 2, WIDTH - 1, WIDTH} {
		input := testElements(n)
		hasher := NewPoseidon2MDHasher()
		hasher.WriteElements(input...)

		got := hasher.Sum()
		want := referenceSingleBlockDigest(input)
		if got != want {
			t.Fatalf("n=%d: digest mismatch", n)
		}
	}
}

func TestPoseidon2MDHasherChunkingInvariant(t *testing.T) {
	for _, n := range []int{0, 1, 7, WIDTH, WIDTH + 1, WIDTH + WIDTH/2, 2*WIDTH + 1, 4 * WIDTH} {
		input := testElements(n)
		want := referenceStreamingDigest(input)

		allAtOnce := NewPoseidon2MDHasher()
		allAtOnce.WriteElements(input...)
		if got := allAtOnce.Sum(); got != want {
			t.Fatalf("all-at-once n=%d: digest mismatch", n)
		}

		oneByOne := NewPoseidon2MDHasher()
		for _, e := range input {
			oneByOne.WriteElements(e)
		}
		if got := oneByOne.Sum(); got != want {
			t.Fatalf("one-by-one n=%d: digest mismatch", n)
		}

		chunked := NewPoseidon2MDHasher()
		for i := 0; i < len(input); i += 3 {
			end := i + 3
			if end > len(input) {
				end = len(input)
			}
			chunked.WriteElements(input[i:end]...)
		}
		if got := chunked.Sum(); got != want {
			t.Fatalf("chunked n=%d: digest mismatch", n)
		}
	}
}

func TestPoseidon2MDHasherWriteExtMatchesCoordinates(t *testing.T) {
	input := []ext.E6{
		testExt(1, 2, 3, 4, 5, 6),
		testExt(7, 8, 9, 10, 11, 12),
		testExt(13, 14, 15, 16, 17, 18),
	}

	withExt := NewPoseidon2MDHasher()
	withExt.WriteExt(input...)

	withElements := NewPoseidon2MDHasher()
	for _, e := range input {
		withElements.WriteElements(ExtToElements(e)...)
	}

	if got, want := withExt.Sum(), withElements.Sum(); got != want {
		t.Fatalf("WriteExt digest mismatch")
	}
}

func TestPoseidon2MDHasherSumIsIdempotentAndResettable(t *testing.T) {
	input := testElements(WIDTH + 3)
	hasher := NewPoseidon2MDHasher()
	hasher.WriteElements(input...)

	first := hasher.Sum()
	second := hasher.Sum()
	if first != second {
		t.Fatal("Sum changed digest on repeated call")
	}

	hasher.Reset()
	hasher.WriteElements(input...)
	if got := hasher.Sum(); got != first {
		t.Fatal("Reset did not restore hasher to initial state")
	}
}

func TestPoseidon2SpongeHasherMatchesReference(t *testing.T) {
	for _, n := range []int{0, 1, 7, SpongeRate, SpongeRate + 1, SpongeWidth, 2*SpongeRate + 3, 4 * SpongeRate} {
		input := testElements(n)
		hasher := NewPoseidon2SpongeHasher()
		hasher.WriteElements(input...)

		got := hasher.Sum()
		want := referenceSpongeDigest(input)
		if got != want {
			t.Fatalf("n=%d: digest mismatch", n)
		}
	}
}

func TestPoseidon2SpongeHasherChunkingInvariant(t *testing.T) {
	for _, n := range []int{0, 1, SpongeRate - 1, SpongeRate, SpongeRate + 1, 3*SpongeRate + 5} {
		input := testElements(n)
		want := referenceSpongeDigest(input)

		allAtOnce := NewPoseidon2SpongeHasher()
		allAtOnce.WriteElements(input...)
		if got := allAtOnce.Sum(); got != want {
			t.Fatalf("all-at-once n=%d: digest mismatch", n)
		}

		oneByOne := NewPoseidon2SpongeHasher()
		for _, e := range input {
			oneByOne.WriteElements(e)
		}
		if got := oneByOne.Sum(); got != want {
			t.Fatalf("one-by-one n=%d: digest mismatch", n)
		}

		chunked := NewPoseidon2SpongeHasher()
		for i := 0; i < len(input); i += 5 {
			end := i + 5
			if end > len(input) {
				end = len(input)
			}
			chunked.WriteElements(input[i:end]...)
		}
		if got := chunked.Sum(); got != want {
			t.Fatalf("chunked n=%d: digest mismatch", n)
		}
	}
}

func TestPoseidon2SpongeHasherWriteExtMatchesCoordinates(t *testing.T) {
	input := []ext.E6{
		testExt(1, 2, 3, 4, 5, 6),
		testExt(7, 8, 9, 10, 11, 12),
		testExt(13, 14, 15, 16, 17, 18),
	}

	withExt := NewPoseidon2SpongeHasher()
	withExt.WriteExt(input...)

	withElements := NewPoseidon2SpongeHasher()
	for _, e := range input {
		withElements.WriteElements(ExtToElements(e)...)
	}

	if got, want := withExt.Sum(), withElements.Sum(); got != want {
		t.Fatalf("WriteExt digest mismatch")
	}
}

func TestPoseidon2SpongeHasherSumIsIdempotentAndResettable(t *testing.T) {
	input := testElements(SpongeRate + 3)
	hasher := NewPoseidon2SpongeHasher()
	hasher.WriteElements(input...)

	first := hasher.Sum()
	second := hasher.Sum()
	if first != second {
		t.Fatal("Sum changed digest on repeated call")
	}

	hasher.Reset()
	hasher.WriteElements(input...)
	if got := hasher.Sum(); got != first {
		t.Fatal("Reset did not restore hasher to initial state")
	}
}

func TestPoseidon2SpongeBatch16MatchesScalar(t *testing.T) {
	for _, n := range []int{0, 1, SpongeRate - 1, SpongeRate, SpongeRate + 1, SpongeWidth, 2*SpongeRate + 3} {
		var inputs [Poseidon2SpongeBatchSize][]koalabear.Element
		batchHasher := NewPoseidon2SpongeBatch16()
		for i := 0; i < n; i++ {
			var batch [Poseidon2SpongeBatchSize]koalabear.Element
			for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
				batch[lane].SetUint64(uint64((lane+1)*1000 + i + 1))
				inputs[lane] = append(inputs[lane], batch[lane])
			}
			batchHasher.WriteElementBatch(batch)
		}

		got := batchHasher.Sum()
		for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
			scalarHasher := NewPoseidon2SpongeHasher()
			scalarHasher.WriteElements(inputs[lane]...)
			if want := scalarHasher.Sum(); got[lane] != want {
				t.Fatalf("n=%d lane=%d: digest mismatch", n, lane)
			}
		}
	}
}

func TestPoseidon2SpongeBatch16WriteExtMatchesScalar(t *testing.T) {
	same := NewElement(42)
	var exts [Poseidon2SpongeBatchSize]ext.E6
	for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
		v := uint64((lane + 1) * 10)
		exts[lane] = testExt(v+1, v+2, v+3, v+4, v+5, v+6)
	}

	batchHasher := NewPoseidon2SpongeBatch16()
	batchHasher.WriteSameElement(same)
	batchHasher.WriteExtBatch(exts)
	got := batchHasher.Sum()

	for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
		scalarHasher := NewPoseidon2SpongeHasher()
		scalarHasher.WriteElements(same)
		scalarHasher.WriteExt(exts[lane])
		if want := scalarHasher.Sum(); got[lane] != want {
			t.Fatalf("lane=%d: digest mismatch", lane)
		}
	}
}

func TestSHA256FieldHasherWriteExtMatchesCoordinates(t *testing.T) {
	input := []ext.E6{
		testExt(1, 2, 3, 4, 5, 6),
		testExt(7, 8, 9, 10, 11, 12),
	}

	withExt := NewSHA256FieldHasher()
	withExt.WriteExt(input...)

	withElements := NewSHA256FieldHasher()
	for _, e := range input {
		withElements.WriteElements(ExtToElements(e)...)
	}

	if got, want := withExt.Sum(), withElements.Sum(); got != want {
		t.Fatalf("WriteExt digest mismatch")
	}
}

func TestSHA256FieldHasherMatchesByteEncoding(t *testing.T) {
	input := testElements(4)
	hasher := NewSHA256FieldHasher()
	hasher.WriteElements(input...)

	ref := sha256.New()
	for i := range input {
		b := input[i].Bytes()
		_, _ = ref.Write(b[:])
	}
	var sum [sha256.Size]byte
	copy(sum[:], ref.Sum(nil))
	if got, want := hasher.Sum(), DigestFromBytes32(sum); got != want {
		t.Fatalf("digest mismatch")
	}
}

func TestSHA256FieldHasherSumIsIdempotentAndResettable(t *testing.T) {
	input := testElements(7)
	hasher := NewSHA256FieldHasher()
	hasher.WriteElements(input...)

	first := hasher.Sum()
	second := hasher.Sum()
	if first != second {
		t.Fatal("Sum changed digest on repeated call")
	}

	hasher.Reset()
	hasher.WriteElements(input...)
	if got := hasher.Sum(); got != first {
		t.Fatal("Reset did not restore hasher to initial state")
	}
}

func referenceSingleBlockDigest(input []koalabear.Element) Digest {
	if len(input) > WIDTH {
		panic("referenceSingleBlockDigest only supports at most one block")
	}
	if len(input) == 0 {
		return Digest{}
	}
	var state [WIDTH]koalabear.Element
	copy(state[:], input)
	return compressReferenceBlock(&state)
}

func referenceStreamingDigest(input []koalabear.Element) Digest {
	if len(input) == 0 {
		return Digest{}
	}
	perm := poseidon2.NewPermutation(WIDTH, NbFullRound, NbPartialRounds)
	var state [WIDTH]koalabear.Element
	pos := 0
	compressed := false
	compress := func() {
		var upper [WIDTH / 2]koalabear.Element
		copy(upper[:], state[WIDTH/2:])
		if err := perm.Permutation(state[:]); err != nil {
			panic(err)
		}
		for i := 0; i < WIDTH/2; i++ {
			state[i].Add(&upper[i], &state[WIDTH/2+i])
		}
		for i := WIDTH / 2; i < WIDTH; i++ {
			state[i].SetZero()
		}
		pos = WIDTH / 2
		compressed = true
	}

	for _, e := range input {
		state[pos].Set(&e)
		pos++
		if pos == WIDTH {
			compress()
		}
	}
	if !compressed || pos > WIDTH/2 {
		for i := pos; i < WIDTH; i++ {
			state[i].SetZero()
		}
		compress()
	}

	var res Digest
	copy(res[:], state[:WIDTH/2])
	return res
}

func compressReferenceBlock(state *[WIDTH]koalabear.Element) Digest {
	perm := poseidon2.NewPermutation(WIDTH, NbFullRound, NbPartialRounds)
	var upper [WIDTH / 2]koalabear.Element
	copy(upper[:], state[WIDTH/2:])
	if err := perm.Permutation(state[:]); err != nil {
		panic(err)
	}
	var res Digest
	for i := 0; i < WIDTH/2; i++ {
		res[i].Add(&upper[i], &state[WIDTH/2+i])
	}
	return res
}

func referenceSpongeDigest(input []koalabear.Element) Digest {
	if len(input) == 0 {
		return Digest{}
	}
	perm := poseidon2.NewPermutation(SpongeWidth, NbFullRound, NbPartialRounds)
	var state [SpongeWidth]koalabear.Element
	for i := 0; i < len(input); i += SpongeRate {
		end := i + SpongeRate
		if end > len(input) {
			end = len(input)
		}
		copy(state[:], input[i:end])
		if err := perm.Permutation(state[:]); err != nil {
			panic(err)
		}
	}

	var res Digest
	copy(res[:], state[:DigestNbElements])
	return res
}

func testElements(n int) []koalabear.Element {
	res := make([]koalabear.Element, n)
	for i := range res {
		res[i].SetUint64(uint64(i + 1))
	}
	return res
}

func testExt(a0, a1, b0, b1 uint64, b2 ...uint64) ext.E6 {
	var e ext.E6
	e.B0.A0.SetUint64(a0)
	e.B0.A1.SetUint64(a1)
	e.B1.A0.SetUint64(b0)
	e.B1.A1.SetUint64(b1)
	if len(b2) > 0 {
		e.B2.A0.SetUint64(b2[0])
	}
	if len(b2) > 1 {
		e.B2.A1.SetUint64(b2[1])
	}
	return e
}
