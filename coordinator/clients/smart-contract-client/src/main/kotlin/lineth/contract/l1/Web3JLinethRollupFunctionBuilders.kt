package lineth.contract.l1

import linea.contract.LinethRollupV6
import linea.contract.l1.LinethRollupContractVersion
import linea.domain.BlobRecord
import linea.domain.ProofToFinalize
import linea.kotlin.toBigInteger
import lineth.contract.l1.FunctionBuildersV8.buildFinalizeBlocksFunctionV8
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.DynamicBytes
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import java.math.BigInteger

internal object Web3JLinethRollupFunctionBuilders {
  fun buildSubmitBlobsFunction(version: LinethRollupContractVersion, blobs: List<BlobRecord>): Function {
    return when (version) {
      LinethRollupContractVersion.V6,
      LinethRollupContractVersion.V7,
      LinethRollupContractVersion.V8,
      -> buildSubmitBlobsFunctionV6(blobs)

      LinethRollupContractVersion.V9 ->
        throw UnsupportedOperationException("version=$version not supported, please use submitBlobsV9 instead")
    }
  }

  fun buildSubmitBlobsFunctionV6(blobs: List<BlobRecord>): Function {
    val blobsSubmissionData =
      blobs.map { blob ->
        val blobCompressionProof = blob.blobCompressionProof!!
        // BlobSubmission(BigInteger dataEvaluationClaim, byte[] kzgCommitment, byte[] kzgProof,
        //                byte[] finalStateRootHash, byte[] snarkHash)
        LinethRollupV6.BlobSubmission(
          // dataEvaluationClaim
          BigInteger(blobCompressionProof.expectedY),
          // kzgCommitment
          blobCompressionProof.commitment,
          // kzgProof
          blobCompressionProof.kzgProofContract,
          // finalStateRootHash
          blobCompressionProof.finalStateRootHash,
          // snarkHash
          blobCompressionProof.snarkHash,
        )
      }

    /**
     function submitBlobs(
     BlobSubmission[] calldata _blobSubmissions,
     bytes32 _parentShnarf,
     bytes32 _finalBlobShnarf
     )
     */
    return Function(
      LinethRollupV6.FUNC_SUBMITBLOBS,
      listOf(
        DynamicArray(LinethRollupV6.BlobSubmission::class.java, blobsSubmissionData),
        Bytes32(blobs.first().blobCompressionProof!!.prevShnarf),
        Bytes32(blobs.last().blobCompressionProof!!.expectedShnarf),
      ),
      emptyList<TypeReference<*>>(),
    )
  }

  fun buildFinalizeBlocksFunction(
    version: LinethRollupContractVersion,
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    return when (version) {
      LinethRollupContractVersion.V6,
      LinethRollupContractVersion.V7,
      -> {
        buildFinalizeBlockFunctionV6(
          aggregationProof,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber,
        )
      }

      LinethRollupContractVersion.V8 -> buildFinalizeBlocksFunctionV8(
        aggregationProof,
        aggregationLastBlob,
        parentL1RollingHash,
        parentL1RollingHashMessageNumber,
      )

      LinethRollupContractVersion.V9 ->
        throw UnsupportedOperationException("version=$version not supported, please use finalizeBlocksV9 instead")
    }
  }

  fun buildFinalizeBlockFunctionV6(
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    val compressionProof = requireNotNull(aggregationLastBlob.blobCompressionProof) {
      "aggregationLastBlob.blobCompressionProof must be set when building the finalization function"
    }
    val aggregationEndBlobInfo =
      LinethRollupV6.ShnarfData(
        // parentShnarf
        compressionProof.prevShnarf,
        // snarkHash
        compressionProof.snarkHash,
        // finalStateRootHash
        compressionProof.finalStateRootHash,
        // dataEvaluationPoint
        compressionProof.expectedX,
        // dataEvaluationClaim
        compressionProof.expectedY,
      )

//  FinalizationDataV3(
//    byte[] parentStateRootHash,
//    BigInteger endBlockNumber,
//    ShnarfData shnarfData,
//    BigInteger lastFinalizedTimestamp,
//    BigInteger finalTimestamp,
//    byte[] lastFinalizedL1RollingHash,
//    byte[] l1RollingHash,
//    BigInteger lastFinalizedL1RollingHashMessageNumber,
//    BigInteger l1RollingHashMessageNumber,
//    BigInteger l2MerkleTreesDepth,
//    List<byte[]> l2MerkleRoots,
//    byte[] l2MessagingBlocksOffsets
//    )

    val finalizationData =
      LinethRollupV6.FinalizationDataV3(
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
        // l2MerkleRoots
        aggregationProof.l2MerkleRoots,
        // l2MessagingBlocksOffsets
        aggregationProof.l2MessagingBlocksOffsets,
      )

    /**
     *  function finalizeBlocks(
     *     bytes calldata _aggregatedProof,
     *     uint256 _proofType,
     *     FinalizationDataV3 calldata _finalizationData
     *   )
     */
    val function =
      Function(
        LinethRollupV6.FUNC_FINALIZEBLOCKS,
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
