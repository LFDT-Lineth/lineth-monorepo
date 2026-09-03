package proofserialization

// This file mirrors verifier-ray's proof ABI: the sizes, field offsets and
// discriminant values that the image is built from.
//
// verifier-ray/src/proof_abi.zig is the authority. It asserts these numbers
// against the Zig types at compile time, so Zig's layout cannot move out from
// under them silently. abi_agreement_test.go checks the other direction — that
// the numbers here still match the ones pinned there — because a pin updated on
// one side only would produce an image that still casts cleanly while the
// verifier reads misplaced bytes.
//
// Nothing here may be inferred from the Go struct layouts in types.go: Go orders
// its own fields and would disagree.

// Type sizes, in bytes. Identical on aarch64, x86_64 and the rv64 guest.
const (
	// SizeSlice is a Zig []const T: {ptr, len}, with no capacity field. This is
	// what lets a payload sit directly behind its header.
	SizeSlice = 16
	// SizeElement is one KoalaBear element: a u32 in Montgomery form.
	SizeElement = 4
	// SizeExt is a degree-6 extension element.
	SizeExt = 24
	// SizeDigest is a Poseidon2 digest / Merkle commitment, [8]Element.
	SizeDigest = 32
	// SizeUsize is a Zig usize on the guest (rv64) and on both host arches.
	SizeUsize = 8

	// SizeScalar is value.Scalar: a 24-byte Ext payload, a discriminant, padding.
	SizeScalar = 28
	// SizeOptCommitment is ?protocol.Commitment: a 32-byte Digest, a presence
	// flag, padding. A committed round carries only its Merkle root -- columns
	// never travel raw, so there is no column union.
	SizeOptCommitment = 36
	// SizeRoundMessage is protocol.RoundMessage.
	SizeRoundMessage = 56

	// SizeRowOpening is merkle.RowOpening: two slice headers.
	SizeRowOpening = 32
	// SizeRowPair is merkle.RowPair, i.e. [2]RowOpening.
	SizeRowPair = 2 * SizeRowOpening
	// SizeOptRowPair is ?merkle.RowPair: the 64-byte pair, a presence flag,
	// padding. The 8 bytes of flag and padding per slot are the single largest
	// piece of structural overhead in a real image.
	SizeOptRowPair = 72
	// SizeInputTreeOpen is merkle.InputTreeOpening.
	SizeInputTreeOpen = 32
	// SizeBranch is merkle.Branch.
	SizeBranch = 48
	// SizeOptDigest is ?poseidon2.Digest: a digest payload, presence flag, and
	// padding. Merkle-cap auxiliary nodes use this representation.
	SizeOptDigest = 36
	// SizeMerkleCap is merkle.MerkleCap: nodes and optional auxiliary nodes.
	SizeMerkleCap = 32
	// SizeInputCap is pcs.InputCap: frontier nodes and revealed tables.
	SizeInputCap = 32
	// SizeInputCapTable is pcs.InputCapTable. Zig places its slice header before
	// the one-byte size descriptor because slices have the greater alignment.
	SizeInputCapTable = 24

	// SizeFriProof is fri.Proof.
	SizeFriProof = 64
	// SizeOpeningProof is pcs.OpeningProof.
	SizeOpeningProof = 96
	// SizePcsOpening is verifier.PcsOpening. It carries only the FRI opening
	// proof and its authenticated Merkle caps; entry_claims are reconstructed
	// from rounds[*].cells by the verifier.
	SizePcsOpening = 96
	// SizeProof is verifier.Proof.
	SizeProof = 128
	// SizeVerifyInput is verifier.VerifyInput, the image ROOT: the proof plus the
	// flat public-input statement. This is what the loaders cast the input region
	// to, so it — not Proof — is what must sit at offset 0.
	SizeVerifyInput = 144
)

// Field offsets, in bytes from the start of the containing type.
//
// Zig lays fields out align-descending, so a slice (align 8) precedes an Element
// array (align 4) whatever the declaration says — see OffBranchLeaf, where the
// declaration was reordered to match so the source no longer disagrees with the
// layout.
const (
	OffVerifyInputProof        = 0
	OffVerifyInputPublicInputs = 128

	OffProofRounds      = 0
	OffProofModuleSizes = 16
	OffProofPcsOpening  = 32

	// PcsOpening now has a single field, so it sits at the struct's own offset 0.
	OffPcsOpeningProof = 0

	OffOpeningProofInputQueries = 0
	OffOpeningProofInputCaps    = 16
	OffOpeningProofFriProof     = 32

	OffFriProofRoundRoots     = 0
	OffFriProofRoundCaps      = 16
	OffFriProofFinalPoly      = 32
	OffFriProofRunningQueries = 48

	OffRoundMessageCells      = 0
	OffRoundMessageCommitment = 16

	// ?Commitment: the 32-byte Digest, then the presence flag.
	OffOptCommitmentPayload = 0
	OffOptCommitmentFlag    = 32

	// Scalar's payload is always a 24-byte Ext; only the tag distinguishes the
	// variants.
	OffScalarPayload = 0
	OffScalarTag     = 24

	OffInputTreeOpeningSiblings = 0
	OffInputTreeOpeningLeaves   = 16

	// ?RowPair: the 64-byte RowPair payload, then the presence flag.
	OffOptRowPairPayload = 0
	OffOptRowPairFlag    = 64

	OffRowOpeningBase = 0
	OffRowOpeningExt  = 16

	OffBranchSiblings = 0
	OffBranchLeaf     = 16

	OffMerkleCapNodes = 0
	OffMerkleCapAux   = 16

	OffInputCapNodes  = 0
	OffInputCapTables = 16

	OffInputCapTableRows     = 0
	OffInputCapTableSizeLog2 = 16

	OffOptDigestPayload = 0
	OffOptDigestFlag    = 32
)

// Discriminant values. A tagged union's discriminant is its variant's position
// in the Zig declaration, so inserting or reordering variants renumbers them —
// which is a wire change, and why proof_abi.zig pins these too.
const (
	TagScalarBase = 0
	TagScalarExt  = 1

	TagOptCommitmentNull    = 0
	TagOptCommitmentPresent = 1

	TagOptRowPairNull    = 0
	TagOptRowPairPresent = 1
)

// GuestBase is where the ZkC guest's input region starts
// (`_in_start = ORIGIN(IN)` in riscv-guests/build_common/linker_script.ld).
// Pointers in the image are absolute guest addresses, so an image is only valid
// at the base it was relocated for.
const GuestBase = 0x08800000

// MaxImageSize is the guest input region's length (`LENGTH(IN)`). An image
// larger than this cannot be loaded.
const MaxImageSize = 0x40000000
