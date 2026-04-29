package invalidity

import (
	"github.com/consensys/gnark/frontend"
	"github.com/consensys/linea-monorepo/prover/crypto/poseidon2_koalabear"
	ac "github.com/consensys/linea-monorepo/prover/crypto/state-management/accumulator"
	smt "github.com/consensys/linea-monorepo/prover/crypto/state-management/smt_koalabear"
	"github.com/consensys/linea-monorepo/prover/maths/field"
	"github.com/consensys/linea-monorepo/prover/maths/field/koalagnark"
	"github.com/consensys/linea-monorepo/prover/utils/types"
	"github.com/consensys/linea-monorepo/prover/zkevm/prover/common"
)

// AccountTrie proves membership (existing account) or non-membership
// (non-existing account) of an account in the state trie.
//
// Two Merkle proof slots are used:
//   - Existing account:  both ProofMinus and ProofPlus hold the same account proof.
//   - Non-existing account: ProofMinus = minus neighbor, ProofPlus = plus neighbor.
type AccountTrie struct {
	Account       GnarkAccount
	TargetHKey    koalagnark.Octuplet // Poseidon2(address) — always Hash(fromAddress)
	ProofMinus    smt.MerkleProofKoalagnark
	ProofPlus     smt.MerkleProofKoalagnark
	LeafMinus     GnarkLeafOpening
	LeafPlus      GnarkLeafOpening
	AccountExists frontend.Variable

	NextFreeNode [common.NbLimbU64]koalagnark.Element
	TopRoot      koalagnark.Octuplet
}

// GnarkAccount represent [types.Account] in gnark with Poseidon2-compatible layout
type GnarkAccount struct {
	Nonce             [common.NbLimbU64]koalagnark.Element  // 4 elements
	Balance           [common.NbLimbU256]koalagnark.Element // 16 elements
	StorageRoot       koalagnark.Octuplet                   // 8 elements (KoalaBear octuplet)
	LineaCodeHash     koalagnark.Octuplet                   // 8 elements (KoalaBear octuplet)
	KeccakCodeHashMSB [common.NbLimbU128]koalagnark.Element // 8 elements (Hi)
	KeccakCodeHashLSB [common.NbLimbU128]koalagnark.Element // 8 elements (Lo)
	CodeSize          [common.NbLimbU64]koalagnark.Element  // 4 elements
}

// GnarkLeafOpening represents a leaf opening in gnark with Poseidon2 octuplet outputs
type GnarkLeafOpening struct {
	Prev [common.NbLimbU64]koalagnark.Element
	Next [common.NbLimbU64]koalagnark.Element
	HKey koalagnark.Octuplet // 8 KoalaBear elements
	HVal koalagnark.Octuplet // 8 KoalaBear elements
}

// AccountTrieInputs collects the data for assigning the [AccountTrie]
type AccountTrieInputs struct {
	Account       types.Account
	TargetHKey    types.KoalaOctuplet // Poseidon2(address) — always Hash(fromAddress)
	ProofMinus    MerkleLeafProof     // existing: the account's proof; non-existing: minus neighbor
	ProofPlus     MerkleLeafProof     // existing: same as ProofMinus; non-existing: plus neighbor
	SubRoot       field.Octuplet
	NextFreeNode  int64
	TopRoot       field.Octuplet
	AccountExists bool
}

// MerkleLeafProof bundles a leaf opening with its Merkle proof and precomputed leaf hash.
type MerkleLeafProof struct {
	LeafOpening ac.LeafOpening
	Leaf        field.Octuplet
	Proof       smt.Proof
}

// Define constrains membership or non-membership in the state trie.
//
// Unconditional checks (both cases):
//  1. TopRoot == Poseidon2(NextFreeNode || ProofMinus.Root)
//  2. ProofMinus.Root == ProofPlus.Root
//  3. Hash(LeafMinus) == ProofMinus.Leaf
//  4. Hash(LeafPlus)  == ProofPlus.Leaf
//  5. Verify ProofMinus against its root
//  6. Verify ProofPlus  against its root
//
// Existing account (AccountExists == 1):
//  7. Hash(Account) == LeafMinus.HVal
//  8. TargetHKey    == LeafMinus.HKey
//
// Non-existing account (AccountExists == 0):
//  9. LeafMinus.Next == ProofPlus.Path  (adjacency)
//  10. LeafPlus.Prev  == ProofMinus.Path (adjacency)
//  11. LeafMinus.HKey < TargetHKey < LeafPlus.HKey (wrapping)
func (at *AccountTrie) Define(api frontend.API) error {

	koalaAPI := koalagnark.NewAPI(api)
	hasher := poseidon2_koalabear.NewKoalagnarkMDHasher(koalaAPI)

	exists := at.AccountExists
	notExists := api.Sub(1, exists)
	existsKoala := koalaAPI.FromFrontendVar(exists)

	api.AssertIsBoolean(exists)

	// --- Unconditional: TopRoot = Poseidon2(NextFreeNode, SubTreeRoot) ---
	for i := 0; i < 16-common.NbLimbU64; i++ {
		hasher.Write(koalagnark.NewElement(0))
	}
	hasher.Write(at.NextFreeNode[:]...)
	hasher.WriteOctuplet(at.ProofMinus.Root)
	recoveredTopRoot := hasher.Sum()
	koalaAPI.AssertOctupletEqual(at.TopRoot, recoveredTopRoot)

	// --- Unconditional: both proofs share the same root ---
	koalaAPI.AssertOctupletEqual(at.ProofMinus.Root, at.ProofPlus.Root)

	// --- Unconditional: leaf hashes match proof leaves ---
	leafMinusHash := at.LeafMinus.Hash(hasher)
	koalaAPI.AssertOctupletEqual(leafMinusHash, at.ProofMinus.Leaf)

	leafPlusHash := at.LeafPlus.Hash(hasher)
	koalaAPI.AssertOctupletEqual(leafPlusHash, at.ProofPlus.Leaf)

	// --- Unconditional: verify both Merkle proofs ---
	smt.KoalagnarkVerifyMerkleProof(api, at.ProofMinus.Proofs, at.ProofMinus.Leaf, at.ProofMinus.Root)
	smt.KoalagnarkVerifyMerkleProof(api, at.ProofPlus.Proofs, at.ProofPlus.Leaf, at.ProofPlus.Root)

	// --- Existing: Hash(Account) == LeafMinus.HVal ---
	accountHash := at.Account.Hash(hasher)
	koalaAPI.AssertOctupletEqualIf(existsKoala, accountHash, at.LeafMinus.HVal)

	// --- Existing: TargetHKey == LeafMinus.HKey (address is in this leaf) ---
	koalaAPI.AssertOctupletEqualIf(existsKoala, at.TargetHKey, at.LeafMinus.HKey)

	// --- Non-existing: adjacency ---
	minusNext := combine16BitLimbs(api, toNativeSlice(at.LeafMinus.Next[:]))
	api.AssertIsEqual(api.Mul(notExists, api.Sub(minusNext, at.ProofPlus.Proofs.Path)), 0)

	plusPrev := combine16BitLimbs(api, toNativeSlice(at.LeafPlus.Prev[:]))
	api.AssertIsEqual(api.Mul(notExists, api.Sub(plusPrev, at.ProofMinus.Proofs.Path)), 0)

	// --- Non-existing: wrapping (minus.HKey < TargetHKey < plus.HKey) ---
	koalaAPI.AssertOctupletIsLessIf(notExists, at.LeafMinus.HKey, at.TargetHKey)
	koalaAPI.AssertOctupletIsLessIf(notExists, at.TargetHKey, at.LeafPlus.HKey)

	return nil
}

// Allocate the circuit
func (at *AccountTrie) Allocate(config Config) {
	at.ProofMinus.Proofs.Siblings = make([]poseidon2_koalabear.KoalagnarkOctuplet, config.Depth)
	at.ProofPlus.Proofs.Siblings = make([]poseidon2_koalabear.KoalagnarkOctuplet, config.Depth)
}

// Assign the circuit from [AccountTrieInputs]
func (at *AccountTrie) Assign(assi AccountTrieInputs) {

	at.Account.Assign(assi.Account)
	assignOctuplet(&at.TargetHKey, assi.TargetHKey)

	assignMerkleProof(&at.ProofMinus, assi.ProofMinus, assi.SubRoot)
	assignMerkleProof(&at.ProofPlus, assi.ProofPlus, assi.SubRoot)
	at.LeafMinus.Assign(assi.ProofMinus.LeafOpening)
	at.LeafPlus.Assign(assi.ProofPlus.LeafOpening)

	if assi.AccountExists {
		at.AccountExists = 1
	} else {
		at.AccountExists = 0
	}

	nfnLimbs := common.SplitBigEndianUint64(uint64(assi.NextFreeNode))
	for i := 0; i < common.NbLimbU64; i++ {
		at.NextFreeNode[i] = koalagnark.NewElement(limbBytesToUint64(nfnLimbs[i]))
	}

	for i := 0; i < 8; i++ {
		at.TopRoot[i] = koalagnark.NewElementFromKoala(assi.TopRoot[i])
	}
}

func assignOctuplet(dst *koalagnark.Octuplet, src types.KoalaOctuplet) {
	for i := 0; i < 8; i++ {
		dst[i] = koalagnark.NewElementFromKoala(field.Element(src[i]))
	}
}

func assignMerkleProof(mp *smt.MerkleProofKoalagnark, lo MerkleLeafProof, root field.Octuplet) {
	mp.Proofs.Siblings = make([]poseidon2_koalabear.KoalagnarkOctuplet, len(lo.Proof.Siblings))
	for j := range lo.Proof.Siblings {
		for i := 0; i < 8; i++ {
			mp.Proofs.Siblings[j][i] = koalagnark.NewElementFromKoala(lo.Proof.Siblings[j][i])
		}
	}
	mp.Proofs.Path = lo.Proof.Path

	for i := 0; i < 8; i++ {
		mp.Leaf[i] = koalagnark.NewElementFromKoala(lo.Leaf[i])
		mp.Root[i] = koalagnark.NewElementFromKoala(root[i])
	}
}

// Hash computes the Poseidon2 hash of the GnarkAccount using the provided hasher.
// The hasher is reset before use. Layout: 80 elements total.
func (a *GnarkAccount) Hash(hasher *poseidon2_koalabear.KoalagnarkMDHasher) poseidon2_koalabear.KoalagnarkOctuplet {
	hasher.Reset()

	for i := 0; i < 16-common.NbLimbU64; i++ {
		hasher.Write(koalagnark.NewElement(0))
	}
	hasher.Write(a.Nonce[:]...)
	hasher.Write(a.Balance[:]...)
	hasher.Write(a.StorageRoot[:]...)
	hasher.Write(a.LineaCodeHash[:]...)
	hasher.Write(a.KeccakCodeHashMSB[:]...)
	hasher.Write(a.KeccakCodeHashLSB[:]...)
	for i := 0; i < 16-common.NbLimbU64; i++ {
		hasher.Write(koalagnark.NewElement(0))
	}
	hasher.Write(a.CodeSize[:]...)

	return hasher.Sum()
}

// Hash computes the Poseidon2 hash of the leaf opening using the provided hasher.
// The hasher is reset before use. Layout: 48 elements total.
func (l *GnarkLeafOpening) Hash(hasher *poseidon2_koalabear.KoalagnarkMDHasher) poseidon2_koalabear.KoalagnarkOctuplet {
	hasher.Reset()

	for i := 0; i < 16-common.NbLimbU64; i++ {
		hasher.Write(koalagnark.NewElement(0))
	}
	hasher.Write(l.Prev[:]...)
	for i := 0; i < 16-common.NbLimbU64; i++ {
		hasher.Write(koalagnark.NewElement(0))
	}
	hasher.Write(l.Next[:]...)
	hasher.Write(l.HKey[:]...)
	hasher.Write(l.HVal[:]...)

	return hasher.Sum()
}

// Assign converts a types.Account to a GnarkAccount for circuit assignment.
func (gnarkAccount *GnarkAccount) Assign(a types.Account) {

	nonceLimbs := common.SplitBigEndianUint64(uint64(a.Nonce))
	for i := 0; i < common.NbLimbU64; i++ {
		gnarkAccount.Nonce[i] = koalagnark.NewElement(limbBytesToUint64(nonceLimbs[i]))
	}

	balanceLimbs := common.SplitBigEndianBigInt(a.Balance, 32)
	for i := 0; i < common.NbLimbU256; i++ {
		gnarkAccount.Balance[i] = koalagnark.NewElement(limbBytesToUint64(balanceLimbs[i]))
	}

	for i := 0; i < 8; i++ {
		gnarkAccount.StorageRoot[i] = koalagnark.NewElementFromKoala(field.Element(a.StorageRoot[i]))
	}

	for i := 0; i < 8; i++ {
		gnarkAccount.LineaCodeHash[i] = koalagnark.NewElementFromKoala(field.Element(a.LineaCodeHash[i]))
	}

	keccakHiLimbs := common.SplitBytes(a.KeccakCodeHash[:16])
	for i := 0; i < common.NbLimbU128; i++ {
		gnarkAccount.KeccakCodeHashMSB[i] = koalagnark.NewElement(limbBytesToUint64(keccakHiLimbs[i]))
	}

	keccakLoLimbs := common.SplitBytes(a.KeccakCodeHash[16:])
	for i := 0; i < common.NbLimbU128; i++ {
		gnarkAccount.KeccakCodeHashLSB[i] = koalagnark.NewElement(limbBytesToUint64(keccakLoLimbs[i]))
	}

	codeSizeLimbs := common.SplitBigEndianUint64(uint64(a.CodeSize))
	for i := 0; i < common.NbLimbU64; i++ {
		gnarkAccount.CodeSize[i] = koalagnark.NewElement(limbBytesToUint64(codeSizeLimbs[i]))
	}
}

// Assign converts an accumulator.LeafOpening to a GnarkLeafOpening for circuit assignment.
func (leafOpening *GnarkLeafOpening) Assign(l ac.LeafOpening) {
	prevLimbs := common.SplitBigEndianUint64(uint64(l.Prev))
	for i := 0; i < common.NbLimbU64; i++ {
		leafOpening.Prev[i] = koalagnark.NewElement(limbBytesToUint64(prevLimbs[i]))
	}

	nextLimbs := common.SplitBigEndianUint64(uint64(l.Next))
	for i := 0; i < common.NbLimbU64; i++ {
		leafOpening.Next[i] = koalagnark.NewElement(limbBytesToUint64(nextLimbs[i]))
	}

	for i := 0; i < 8; i++ {
		leafOpening.HKey[i] = koalagnark.NewElementFromKoala(field.Element(l.HKey[i]))
	}

	for i := 0; i < 8; i++ {
		leafOpening.HVal[i] = koalagnark.NewElementFromKoala(field.Element(l.HVal[i]))
	}
}

func limbBytesToUint64(limb []byte) uint64 {
	var res uint64
	for _, b := range limb {
		res = (res << 8) | uint64(b)
	}
	return res
}

// ComputeTopRoot computes TopRoot = Poseidon2(nextFreeNode || subTreeRoot).
func ComputeTopRoot(nextFreeNode int64, subTreeRoot types.KoalaOctuplet) types.KoalaOctuplet {
	hasher := poseidon2_koalabear.NewMDHasher()
	types.WriteInt64On64Bytes(hasher, nextFreeNode)
	subTreeRoot.WriteTo(hasher)
	digest := hasher.Sum(nil)
	r, err := types.BytesToKoalaOctuplet(digest)
	if err != nil {
		panic(err)
	}
	return r
}
