package net.consensys.linea.contract.l1

import lineth.contract.LinethRollupV9
import lineth.contract.l1.BlobsSubmissionV9
import lineth.contract.l1.FinalizationDataV9
import lineth.kotlin.encodeHex
import lineth.kotlin.toBigInteger
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.DynamicBytes
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256

internal object FunctionBuildersV9 {
  /**
   * function submitBlobs(
   *   bytes32[] calldata _blobFinalBlockHashes,
   *   bytes32 _parentShnarf,
   *   bytes32 _finalBlobShnarf
   * )
   */
  fun buildSubmitBlobsFunctionV9(
    blobsSubmission: BlobsSubmissionV9,
  ): Function {
    return Function(
      LinethRollupV9.FUNC_SUBMITBLOBS,
      listOf(
        DynamicArray(Bytes32::class.java, blobsSubmission.blobFinalBlockHashes.map(::Bytes32)),
        Bytes32(blobsSubmission.parentShnarf),
        Bytes32(blobsSubmission.finalBlobShnarf),
      ),
      emptyList<TypeReference<*>>(),
    )
  }

  /**
   * function finalizeBlocks(
   *   bytes calldata _aggregatedProof,
   *   uint256 _proofType,
   *   FinalizationDataV5 calldata _finalizationData
   * )
   */
  fun buildFinalizeBlocksFunctionV9(
    finalization: FinalizationDataV9,
  ): Function {
    val shnarfData =
      LinethRollupV9.ShnarfData(
        // parentShnarf
        finalization.shnarfData.parentShnarf,
        // snarkHash
        finalization.shnarfData.snarkHash,
        // finalStateRootHash
        finalization.shnarfData.finalStateRootHash,
        // dataEvaluationPoint
        finalization.shnarfData.dataEvaluationPoint,
        // dataEvaluationClaim
        finalization.shnarfData.dataEvaluationClaim,
      )

    val finalizationData =
      LinethRollupV9.FinalizationDataV5(
        // parentStateRootHash
        finalization.parentStateRootHash,
        // parentBlockHash
        finalization.parentBlockHash,
        // endBlockNumber
        finalization.endBlockNumber.toBigInteger(),
        // shnarfData
        shnarfData,
        // lastFinalizedTimestamp
        finalization.lastFinalizedTimestamp.epochSeconds.toBigInteger(),
        // finalTimestamp
        finalization.finalTimestamp.epochSeconds.toBigInteger(),
        // lastFinalizedL1RollingHash
        finalization.lastFinalizedL1RollingHash,
        // l1RollingHash
        finalization.l1RollingHash,
        // lastFinalizedL1RollingHashMessageNumber
        finalization.lastFinalizedL1RollingHashMessageNumber.toBigInteger(),
        // l1RollingHashMessageNumber
        finalization.l1RollingHashMessageNumber.toBigInteger(),
        // l2MerkleTreesDepth
        finalization.l2MerkleTreesDepth.toBigInteger(),
        // lastFinalizedForcedTransactionNumber
        finalization.lastFinalizedForcedTransactionNumber.toBigInteger(),
        // finalForcedTransactionNumber
        finalization.finalForcedTransactionNumber.toBigInteger(),
        // lastFinalizedForcedTransactionRollingHash
        finalization.lastFinalizedForcedTransactionRollingHash,
        // finalBlockHash
        finalization.finalBlockHash,
        // finalBlobHash
        finalization.finalBlobHash,
        // l2MerkleRoots
        finalization.l2MerkleRoots,
        // filteredAddresses
        finalization.filteredAddresses.mapIndexed { i, address ->
          require(address.size == 20) { "filteredAddresses[$i] must be 20 bytes (address), got ${address.size}" }
          address.encodeHex()
        },
        // verifierKeys
        finalization.verifierKeys,
        // l2MessagingBlocksOffsets
        finalization.l2MessagingBlocksOffsets,
      )

    return Function(
      LinethRollupV9.FUNC_FINALIZEBLOCKS,
      listOf(
        DynamicBytes(finalization.aggregatedProof),
        Uint256(finalization.proofType.toLong()),
        finalizationData,
      ),
      emptyList<TypeReference<*>>(),
    )
  }
}
