package net.consensys.linea.contract.l1

import build.linea.contract.LineaRollupV8
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicBytes
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.generated.Uint256
import kotlin.collections.emptyList

internal object FunctionBuildersV8 {
  fun buildFinalizeBlocksFunctionV8(
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    val aggregationEndBlobInfo =
      LineaRollupV8.ShnarfData(
        // parentShnarf
        aggregationLastBlob.blobCompressionProof!!.prevShnarf,
        // snarkHash
        aggregationLastBlob.blobCompressionProof!!.snarkHash,
        // finalStateRootHash
        aggregationLastBlob.blobCompressionProof!!.finalStateRootHash,
        // dataEvaluationPoint
        aggregationLastBlob.blobCompressionProof!!.expectedX,
        // dataEvaluationClaim
        aggregationLastBlob.blobCompressionProof!!.expectedY,
      )

    // FinalizationDataV4(
    //   byte[] parentStateRootHash,
    //   BigInteger endBlockNumber,
    //   ShnarfData shnarfData,
    //   BigInteger lastFinalizedTimestamp,
    //   BigInteger finalTimestamp,
    //   byte[] lastFinalizedL1RollingHash,
    //   byte[] l1RollingHash,
    //   BigInteger lastFinalizedL1RollingHashMessageNumber,
    //   BigInteger l1RollingHashMessageNumber,
    //   BigInteger l2MerkleTreesDepth,
    //   BigInteger lastFinalizedForcedTransactionNumber,
    //   BigInteger finalForcedTransactionNumber,
    //   byte[] lastFinalizedForcedTransactionRollingHash,
    //   List<byte[]> l2MerkleRoots,
    //   List<String> filteredAddresses,
    //   byte[] l2MessagingBlocksOffsets
    // )

    val finalizationData =
      LineaRollupV8.FinalizationDataV4(
        // parentStateRootHash
        aggregationProof.parentStateRootHash,
        // endBlockNumber
        aggregationProof.endBlockNumber.toBigInteger(),
        // shnarfData
        aggregationEndBlobInfo,
        // lastFinalizedTimestamp
        aggregationProof.parentAggregationLastBlockTimestamp.epochSeconds.toBigInteger(),
        // finalTimestamp
        aggregationProof.finalTimestamp.epochSeconds.toBigInteger(),
        // lastFinalizedL1RollingHash
        parentL1RollingHash,
        // l1RollingHash
        aggregationProof.l1RollingHash,
        // lastFinalizedL1RollingHashMessageNumber
        parentL1RollingHashMessageNumber.toBigInteger(),
        // l1RollingHashMessageNumber
        aggregationProof.l1RollingHashMessageNumber.toBigInteger(),
        // l2MerkleTreesDepth
        aggregationProof.l2MerkleTreesDepth.toBigInteger(),
        // BigInteger lastFinalizedForcedTransactionNumber
        aggregationProof.parentAggregationFtxNumber.toBigInteger(),
        // BigInteger finalForcedTransactionNumber,
        aggregationProof.finalFtxNumber.toBigInteger(),
        // byte[] lastFinalizedForcedTransactionRollingHash,
        aggregationProof.parentAggregationFtxRollingHash,
        // l2MerkleRoots
        aggregationProof.l2MerkleRoots,
        //  List<String> filteredAddresses,
        aggregationProof.filteredAddresses.map { it.encodeHex() },
        // byte[] l2MessagingBlocksOffsets
        aggregationProof.l2MessagingBlocksOffsets,
      )

    /**
     *  function finalizeBlocks(
     *     bytes calldata _aggregatedProof,
     *     uint256 _proofType,
     *     FinalizationDataV4 calldata _finalizationData
     *   )
     */
    val function =
      Function(
        LineaRollupV8.FUNC_FINALIZEBLOCKS,
        listOf(
          DynamicBytes(aggregationProof.aggregatedProof),
          Uint256(aggregationProof.aggregatedVerifierIndex.toLong()),
          finalizationData,
        ),
        emptyList<TypeReference<*>>(),
      )
    return function
  }
}
