package selfrecursion

import (
	"fmt"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/crypto/ringsis"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/coin"

	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/protocol/ifaces"
	"github.com/consensys/linea-monorepo/prover/circuits/pi-interconnection/keccak/prover/utils"
)

// Name of the polynomial I(x)
func (ctx *SelfRecursionCtx) iName(length int) ifaces.ColID {
	name := ifaces.ColIDf("PRECOMPUTED_%v_I_%v", ctx.SelfRecursionCnt, length)
	return maybePrefix(ctx, name)
}

// Name of the aH polynomials
func (ctx *SelfRecursionCtx) ahName(key *ringsis.Key, start, length, maxSize int) ifaces.ColID {
	// Sanity-check : the chunkNo can't be off-bound
	if maxSize < start+length {
		utils.Panic("inconsistent arguments : %v + %v > %v", start, length, maxSize)
	}

	subName := ifaces.ColIDf("SISKEY_%v_%v_%v", key.LogTwoBound, key.LogTwoDegree, key.MaxNumFieldHashable())
	name := ifaces.ColIDf("%v_%v_%v_%v_%v", subName, start, length, maxSize, ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the preimage in limb expanded. nameWhole is the name of the
// associated column without limb expansion.
func (ctx *SelfRecursionCtx) limbExpandedPreimageName(nameWhole ifaces.ColID) ifaces.ColID {
	name := ifaces.ColIDf("%v_LIMB_EXPANDED_%v", nameWhole, ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the UalphaQ column
func (ctx *SelfRecursionCtx) uAlphaQName() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_U_ALPHA_Q_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the UalphaQFilter column
func (ctx *SelfRecursionCtx) uAlphaQFilterName() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_U_ALPHA_Q_FILTER_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the self-recursed inclusion query
func (ctx *SelfRecursionCtx) selectQInclusion() ifaces.QueryID {
	name := ifaces.QueryIDf("SELFRECURSION_SELECT_Q_INCLUSION_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the collapse coin
func (ctx *SelfRecursionCtx) collapseCoin() coin.Name {
	name := coin.Namef("SELFRECURSION_COLLAPSE_COIN_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name for the coeff eval for consistency check between Ualphaq
// and the preimage (left-side, over UalphaA)
func (ctx *SelfRecursionCtx) constencyUalphaQPreimageLeft() string {
	name := fmt.Sprintf("SELFRECURSION_CONSISTENCY_UALPHA_PREIMAGE_LEFT_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name for the coeff eval for consistency check between Ualphaq
// and the preimage (right-side, over preiimages)
func (ctx *SelfRecursionCtx) constencyUalphaQPreimageRight() string {
	name := fmt.Sprintf("SELFRECURSION_CONSISTENCY_UALPHA_PREIMAGE_RIGHT_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of Edual
func (ctx *SelfRecursionCtx) eDual() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_E_DUAL_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name for the fold coin
func (ctx *SelfRecursionCtx) foldCoinName() coin.Name {
	name := coin.Namef("SELFRECURSION_FOLD_COIN_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the inner product between between PreimageCollapseFold and ACollapseFold
func (ctx *SelfRecursionCtx) preimagesAndAmergeIP() ifaces.QueryID {
	name := ifaces.QueryIDf("SELFRECURSION_PREIMAGE_A_IP_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the interpolation context for Ualpha
func (ctx *SelfRecursionCtx) interpolateUAlphaX() string {
	name := fmt.Sprintf("SELFRECURSION_INTERPOLATE_UALPHA_X_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the concatenation of the DhQs
func (ctx *SelfRecursionCtx) concatenatedDhQ() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_CONCAT_DHQ_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the concatenated MiMC hashes for the non SIS rounds
func (ctx *SelfRecursionCtx) nonSisLeaves(round int) ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_NON_SIS_LEAVES_%v_%v", ctx.SelfRecursionCnt, round)
	return maybePrefix(ctx, name)
}

// Name of the concatenated preimages for the non SIS rounds
func (ctx *SelfRecursionCtx) concatenatedMIMCPreimages(round int) ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_CONCAT_MIMC_PREIMAGES_%v_%v", ctx.SelfRecursionCnt, round)
	return maybePrefix(ctx, name)
}

// Name of the concatenated hashes for the precomputed rounds
func (ctx *SelfRecursionCtx) concatenatedPrecomputedHashes() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_CONCAT_PRECOMPUTED_HASHES_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the concatenated preimages for the precomputed rounds
func (ctx *SelfRecursionCtx) concatenatedPrecomputedPreimages() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_CONCAT_PRECOMPUTED_PREIMAGES_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the MerkleLeaves
func (ctx *SelfRecursionCtx) merkleLeavesName() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_MERKLE_LEAVES_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the MerklePositions
func (ctx *SelfRecursionCtx) merklePositionsName() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_MERKLE_POSITIONS_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the MerkleRoots
func (ctx *SelfRecursionCtx) merkleRootsName() ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_MERKLE_ROOTS_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the SIS rounds leaves
func (ctx *SelfRecursionCtx) sisRoundLeavesName(round int) ifaces.ColID {
	name := ifaces.ColIDf("SELFRECURSION_SIS_ROUND_LEAVES_%v_%v", ctx.SelfRecursionCnt, round)
	return maybePrefix(ctx, name)
}

// Name of the Merkle proof verification
func (ctx *SelfRecursionCtx) merkleProofVerificationName() string {
	name := fmt.Sprintf("SELFRECURSION_MERKLE_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the collapsed key
func (ctx *SelfRecursionCtx) aCollapsedName() string {
	name := fmt.Sprintf("SELFRECURSION_ACOLLAPSE_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Name of the collapsed key
func (ctx *SelfRecursionCtx) rootHasGlue() ifaces.QueryID {
	name := ifaces.QueryIDf("SELFRECURSION_ROOT_HASH_GLUE_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// Positions glue
func (ctx *SelfRecursionCtx) positionGlue() ifaces.QueryID {
	name := ifaces.QueryIDf("SELFRECURSION_POSITION_GLUE_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// linearHashVerificatioName returns the name passed to the wizard helper building the
// linear hash verifier.
func (ctx *SelfRecursionCtx) linearHashVerificationName() string {
	name := fmt.Sprintf("SELFRECURSION_LINEAR_HASH_VERIFICATION_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// nonSisRoundLinearHashVerificationName returns the name passed to the wizard helper building the
// non SIS round linear hash verifier.
func (ctx *SelfRecursionCtx) nonSisRoundLinearHashVerificationName(round int) string {
	name := fmt.Sprintf("SELFRECURSION_NON_SIS_ROUND_LINEAR_HASH_VERIFICATION_%v_%v", ctx.SelfRecursionCnt, round)
	return maybePrefix(ctx, name)
}

// leafConsistencyName returns the name passed to the wizard helper building the
// leaf consistency verifier.
func (ctx *SelfRecursionCtx) leafConsistencyName() ifaces.QueryID {
	name := ifaces.QueryIDf("SELFRECURSION_LINEAR_HASH_LEAF_CONSISTENCY_%v", ctx.SelfRecursionCnt)
	return maybePrefix(ctx, name)
}

// maybePrefix adds the prefix if defined in the context
func maybePrefix[T ~string](ctx *SelfRecursionCtx, name T) T {
	return T(ctx.NamePrefix+".") + name
}
