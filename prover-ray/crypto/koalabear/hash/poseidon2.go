package hash

import (
	"github.com/consensys/gnark-crypto/field/koalabear"
	ext "github.com/consensys/gnark-crypto/field/koalabear/extensions"
	"github.com/consensys/gnark-crypto/field/koalabear/poseidon2"
)

// Poseidon2MDHasher is a padding-free overwrite-mode Merkle Damgard hasher with width 16,
// and an 8-element digest. It is used for Merkle node compression .
type Poseidon2MDHasher struct {
	Perm *poseidon2.Permutation

	state      [WIDTH]koalabear.Element
	pos        int
	wrote      bool
	compressed bool
	finalized  bool
}

// Poseidon2SpongeHasher is a padding-free overwrite-mode sponge with width 24,
// rate 16, and an 8-element digest. It is used for Fiat-Shamir and Merkle
// leaves, while Merkle node compression keeps the width-16 MD hasher.
type Poseidon2SpongeHasher struct {
	Perm *poseidon2.Permutation

	state     [SpongeWidth]koalabear.Element
	block     [SpongeRate]koalabear.Element
	blockLen  int
	wrote     bool
	finalized bool
}

const Poseidon2SpongeBatchSize = 16

// Poseidon2SpongeBatch16 is the batched counterpart of Poseidon2SpongeHasher.
// Its state is transposed: state element k contains 16 independent lanes.
type Poseidon2SpongeBatch16 struct {
	Perm *poseidon2.Permutation

	state     [SpongeWidth][Poseidon2SpongeBatchSize]koalabear.Element
	block     [SpongeRate][Poseidon2SpongeBatchSize]koalabear.Element
	blockLen  int
	wrote     bool
	finalized bool
}

// Poseidon2 permutations only hold immutable parameters/round keys. The
// permutation call mutates the input state slice, so hashers can share these
// values while keeping independent state buffers.
var (
	defaultPoseidon2Perm       = poseidon2.NewPermutation(WIDTH, NbFullRound, NbPartialRounds)
	defaultPoseidon2SpongePerm = poseidon2.NewPermutation(SpongeWidth, NbFullRound, NbPartialRounds)
)

func NewPoseidon2MDHasher() Poseidon2MDHasher {
	return Poseidon2MDHasher{
		Perm: defaultPoseidon2Perm,
	}
}

// NewPoseidon2SpongeHasher returns the width-24/rate-16 Poseidon2 sponge.
func NewPoseidon2SpongeHasher() Poseidon2SpongeHasher {
	return Poseidon2SpongeHasher{
		Perm: defaultPoseidon2SpongePerm,
	}
}

// NewPoseidon2SpongeBatch16 returns a width-24/rate-16 Poseidon2 sponge that
// hashes 16 independent inputs using gnark-crypto's transposed batch API.
func NewPoseidon2SpongeBatch16() Poseidon2SpongeBatch16 {
	return Poseidon2SpongeBatch16{
		Perm: defaultPoseidon2SpongePerm,
	}
}

func (ph *Poseidon2MDHasher) Reset() {
	for i := range ph.state {
		ph.state[i].SetZero()
	}
	ph.pos = 0
	ph.wrote = false
	ph.compressed = false
	ph.finalized = false
}

func (ph *Poseidon2MDHasher) WriteElements(elmts ...koalabear.Element) {
	for i := range elmts {
		ph.writeElement(elmts[i])
	}
}

func (ph *Poseidon2MDHasher) WriteExt(elmts ...ext.E6) {
	for _, elmt := range elmts {
		ph.writeElement(elmt.B0.A0)
		ph.writeElement(elmt.B0.A1)
		ph.writeElement(elmt.B1.A0)
		ph.writeElement(elmt.B1.A1)
		ph.writeElement(elmt.B2.A0)
		ph.writeElement(elmt.B2.A1)
	}
}

func (ph *Poseidon2MDHasher) Sum() Digest {
	if ph.finalized {
		return ph.digest()
	}
	if !ph.wrote {
		ph.finalized = true
		return Digest{}
	}
	if !ph.compressed || ph.pos > WIDTH/2 {
		ph.zeroFromPos()
		ph.compress()
	}
	ph.finalized = true
	return ph.digest()
}

func (ph *Poseidon2MDHasher) writeElement(elmt koalabear.Element) {
	ph.state[ph.pos].Set(&elmt)
	ph.pos++
	ph.wrote = true
	ph.finalized = false
	if ph.pos == WIDTH {
		ph.compress()
	}
}

func (ph *Poseidon2MDHasher) compress() {
	if ph.Perm == nil {
		ph.Perm = defaultPoseidon2Perm
	}
	var upper [WIDTH / 2]koalabear.Element
	copy(upper[:], ph.state[WIDTH/2:])
	if err := ph.Perm.Permutation(ph.state[:]); err != nil {
		panic(err)
	}
	for i := 0; i < WIDTH/2; i++ {
		ph.state[i].Add(&upper[i], &ph.state[WIDTH/2+i])
	}
	for i := WIDTH / 2; i < WIDTH; i++ {
		ph.state[i].SetZero()
	}
	ph.pos = WIDTH / 2
	ph.compressed = true
}

func (ph *Poseidon2MDHasher) zeroFromPos() {
	for i := ph.pos; i < WIDTH; i++ {
		ph.state[i].SetZero()
	}
}

func (ph *Poseidon2MDHasher) digest() Digest {
	var res Digest
	copy(res[:], ph.state[:WIDTH/2])
	return res
}

func (ph *Poseidon2SpongeHasher) Reset() {
	for i := range ph.state {
		ph.state[i].SetZero()
	}
	for i := range ph.block {
		ph.block[i].SetZero()
	}
	ph.blockLen = 0
	ph.wrote = false
	ph.finalized = false
}

func (ph *Poseidon2SpongeHasher) WriteElements(elmts ...koalabear.Element) {
	for i := range elmts {
		ph.writeElement(elmts[i])
	}
}

func (ph *Poseidon2SpongeHasher) WriteExt(elmts ...ext.E6) {
	for _, elmt := range elmts {
		ph.writeElement(elmt.B0.A0)
		ph.writeElement(elmt.B0.A1)
		ph.writeElement(elmt.B1.A0)
		ph.writeElement(elmt.B1.A1)
		ph.writeElement(elmt.B2.A0)
		ph.writeElement(elmt.B2.A1)
	}
}

func (ph *Poseidon2SpongeHasher) Sum() Digest {
	if ph.finalized {
		return ph.digest()
	}
	if !ph.wrote && ph.blockLen == 0 {
		ph.finalized = true
		return Digest{}
	}
	if ph.blockLen > 0 {
		ph.absorbPartialBlock()
	}
	ph.finalized = true
	return ph.digest()
}

func (ph *Poseidon2SpongeHasher) writeElement(elmt koalabear.Element) {
	ph.block[ph.blockLen].Set(&elmt)
	ph.blockLen++
	ph.finalized = false
	if ph.blockLen == SpongeRate {
		ph.absorbFullBlock()
	}
}

func (ph *Poseidon2SpongeHasher) absorbFullBlock() {
	copy(ph.state[:SpongeRate], ph.block[:])
	ph.permute()
	ph.clearBlock()
	ph.blockLen = 0
	ph.wrote = true
}

func (ph *Poseidon2SpongeHasher) absorbPartialBlock() {
	for i := 0; i < ph.blockLen; i++ {
		ph.state[i].Set(&ph.block[i])
	}
	ph.permute()
	ph.clearBlock()
	ph.blockLen = 0
	ph.wrote = true
}

func (ph *Poseidon2SpongeHasher) permute() {
	if ph.Perm == nil {
		ph.Perm = defaultPoseidon2SpongePerm
	}
	if err := ph.Perm.Permutation(ph.state[:]); err != nil {
		panic(err)
	}
}

func (ph *Poseidon2SpongeHasher) clearBlock() {
	for i := range ph.block {
		ph.block[i].SetZero()
	}
}

func (ph *Poseidon2SpongeHasher) digest() Digest {
	var res Digest
	copy(res[:], ph.state[:DigestNbElements])
	return res
}

func (ph *Poseidon2SpongeBatch16) Reset() {
	for i := range ph.state {
		for j := range ph.state[i] {
			ph.state[i][j].SetZero()
		}
	}
	for i := range ph.block {
		for j := range ph.block[i] {
			ph.block[i][j].SetZero()
		}
	}
	ph.blockLen = 0
	ph.wrote = false
	ph.finalized = false
}

func (ph *Poseidon2SpongeBatch16) WriteSameElement(elmt koalabear.Element) {
	var batch [Poseidon2SpongeBatchSize]koalabear.Element
	for i := range batch {
		batch[i].Set(&elmt)
	}
	ph.WriteElementBatch(batch)
}

func (ph *Poseidon2SpongeBatch16) WriteElementBatch(elmts [Poseidon2SpongeBatchSize]koalabear.Element) {
	ph.block[ph.blockLen] = elmts
	ph.blockLen++
	ph.finalized = false
	if ph.blockLen == SpongeRate {
		ph.absorbFullBlock()
	}
}

func (ph *Poseidon2SpongeBatch16) WriteExtBatch(elmts [Poseidon2SpongeBatchSize]ext.E6) {
	var batch [Poseidon2SpongeBatchSize]koalabear.Element
	for i := range elmts {
		batch[i].Set(&elmts[i].B0.A0)
	}
	ph.WriteElementBatch(batch)
	for i := range elmts {
		batch[i].Set(&elmts[i].B0.A1)
	}
	ph.WriteElementBatch(batch)
	for i := range elmts {
		batch[i].Set(&elmts[i].B1.A0)
	}
	ph.WriteElementBatch(batch)
	for i := range elmts {
		batch[i].Set(&elmts[i].B1.A1)
	}
	ph.WriteElementBatch(batch)
	for i := range elmts {
		batch[i].Set(&elmts[i].B2.A0)
	}
	ph.WriteElementBatch(batch)
	for i := range elmts {
		batch[i].Set(&elmts[i].B2.A1)
	}
	ph.WriteElementBatch(batch)
}

func (ph *Poseidon2SpongeBatch16) Sum() [Poseidon2SpongeBatchSize]Digest {
	if ph.finalized {
		return ph.digest()
	}
	if !ph.wrote && ph.blockLen == 0 {
		ph.finalized = true
		return [Poseidon2SpongeBatchSize]Digest{}
	}
	if ph.blockLen > 0 {
		ph.absorbPartialBlock()
	}
	ph.finalized = true
	return ph.digest()
}

func (ph *Poseidon2SpongeBatch16) absorbFullBlock() {
	copy(ph.state[:SpongeRate], ph.block[:])
	ph.permute()
	ph.clearBlock()
	ph.blockLen = 0
	ph.wrote = true
}

func (ph *Poseidon2SpongeBatch16) absorbPartialBlock() {
	for i := 0; i < ph.blockLen; i++ {
		ph.state[i] = ph.block[i]
	}
	ph.permute()
	ph.clearBlock()
	ph.blockLen = 0
	ph.wrote = true
}

func (ph *Poseidon2SpongeBatch16) permute() {
	if ph.Perm == nil {
		ph.Perm = defaultPoseidon2SpongePerm
	}
	ph.Perm.Permutation16x24(&ph.state)
}

func (ph *Poseidon2SpongeBatch16) clearBlock() {
	for i := range ph.block {
		for j := range ph.block[i] {
			ph.block[i][j].SetZero()
		}
	}
}

func (ph *Poseidon2SpongeBatch16) digest() [Poseidon2SpongeBatchSize]Digest {
	var res [Poseidon2SpongeBatchSize]Digest
	for lane := range res {
		for i := range res[lane] {
			res[lane][i].Set(&ph.state[i][lane])
		}
	}
	return res
}

// Poseidon2NodeCompress is a one-permutation Merkle node hasher using the
// width-24 sponge permutation. Layout:
//
//	state[0]      = nodeDomainTag (capacity, for domain separation from leaves)
//	state[1..7]   = 0             (rest of capacity)
//	state[8..15]  = left[0..7]    (rate, low half)
//	state[16..23] = right[0..7]   (rate, high half)
//
// One Permutation, then the digest is the first DigestNbElements (=8)
// state slots.
//
// This replaces the previous MD-style width-16 hasher (two permutations per
// node, no SIMD path). One permutation per node is enough for collision
// resistance because the input fits in a single rate-16 block.
func Poseidon2NodeCompress(nodeDomainTag uint64, left, right Digest) Digest {
	var state [SpongeWidth]koalabear.Element
	state[0].SetUint64(nodeDomainTag)
	for i := range left {
		state[8+i].Set(&left[i])
		state[16+i].Set(&right[i])
	}
	if err := defaultPoseidon2SpongePerm.Permutation(state[:]); err != nil {
		panic(err)
	}
	var res Digest
	copy(res[:], state[:DigestNbElements])
	return res
}

// Poseidon2NodeCompressBatch16 compresses 16 (left, right) pairs in parallel
// via the 16-lane batched sponge permutation (permutation16x24). Lanes are
// independent. Same layout as Poseidon2NodeCompress in each lane.
func Poseidon2NodeCompressBatch16(
	nodeDomainTag uint64,
	left, right *[Poseidon2SpongeBatchSize]Digest,
) [Poseidon2SpongeBatchSize]Digest {
	var state [SpongeWidth][Poseidon2SpongeBatchSize]koalabear.Element
	var tag koalabear.Element
	tag.SetUint64(nodeDomainTag)
	for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
		state[0][lane].Set(&tag)
	}
	for lane := 0; lane < Poseidon2SpongeBatchSize; lane++ {
		for i := 0; i < DigestNbElements; i++ {
			state[8+i][lane].Set(&left[lane][i])
			state[16+i][lane].Set(&right[lane][i])
		}
	}
	defaultPoseidon2SpongePerm.Permutation16x24(&state)

	var res [Poseidon2SpongeBatchSize]Digest
	for lane := range res {
		for i := range res[lane] {
			res[lane][i].Set(&state[i][lane])
		}
	}
	return res
}
