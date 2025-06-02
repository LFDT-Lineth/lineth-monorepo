package net.consensys.linea.contract.l1

import build.linea.contract.LineaRollupV6
import linea.contract.l1.LineaContractVersion
import linea.kotlin.toBigInteger
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.ProofToFinalize
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicArray
import org.web3j.abi.datatypes.DynamicBytes
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.Type
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import java.math.BigInteger
import java.util.Arrays

internal fun buildSubmitBlobsFunction(
  version: LineaContractVersion,
  blobs: List<BlobRecord>,
): Function {
  return when (version) {
    LineaContractVersion.V6 -> buildSubmitBlobsFunctionV6(blobs)
  }
}

internal fun buildSubmitBlobsFunctionV6(
  blobs: List<BlobRecord>,
): Function {
  val blobsSubmissionData = blobs.map { blob ->
    val blobCompressionProof = blob.blobCompressionProof!!
    // BlobSubmission(BigInteger dataEvaluationClaim, byte[] kzgCommitment, byte[] kzgProof,
    //                byte[] finalStateRootHash, byte[] snarkHash)
    LineaRollupV6.BlobSubmission(
      /*dataEvaluationClaim*/
      BigInteger(blobCompressionProof.expectedY),
      /*kzgCommitment*/
      blobCompressionProof.commitment,
      /*kzgProof*/
      blobCompressionProof.kzgProofContract,
      /*finalStateRootHash*/
      blobCompressionProof.finalStateRootHash,
      /*snarkHash*/
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
    LineaRollupV6.FUNC_SUBMITBLOBS,
    Arrays.asList<Type<*>>(
      DynamicArray(LineaRollupV6.BlobSubmission::class.java, blobsSubmissionData),
      Bytes32(blobs.first().blobCompressionProof!!.prevShnarf),
      Bytes32(blobs.last().blobCompressionProof!!.expectedShnarf),
    ),
    emptyList<TypeReference<*>>(),
  )
}

fun buildFinalizeBlocksFunction(
  version: LineaContractVersion,
  aggregationProof: ProofToFinalize,
  aggregationLastBlob: BlobRecord,
  parentL1RollingHash: ByteArray,
  parentL1RollingHashMessageNumber: Long,
): Function {
  when (version) {
    LineaContractVersion.V6 -> {
      return buildFinalizeBlockFunctionV6(
        aggregationProof,
        aggregationLastBlob,
        parentL1RollingHash,
        parentL1RollingHashMessageNumber,
      )
    }
  }
}

internal fun buildFinalizeBlockFunctionV6(
  aggregationProof: ProofToFinalize,
  aggregationLastBlob: BlobRecord,
  parentL1RollingHash: ByteArray,
  parentL1RollingHashMessageNumber: Long,
): Function {
  val aggregationEndBlobInfo = LineaRollupV6.ShnarfData(
    /*parentShnarf*/
    aggregationLastBlob.blobCompressionProof!!.prevShnarf,
    /*snarkHash*/
    aggregationLastBlob.blobCompressionProof!!.snarkHash,
    /*finalStateRootHash*/
    aggregationLastBlob.blobCompressionProof!!.finalStateRootHash,
    /*dataEvaluationPoint*/
    aggregationLastBlob.blobCompressionProof!!.expectedX,
    /*dataEvaluationClaim*/
    aggregationLastBlob.blobCompressionProof!!.expectedY,
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

  val finalizationData = LineaRollupV6.FinalizationDataV3(
    /*parentStateRootHash*/
    aggregationProof.parentStateRootHash,
    /*endBlockNumber*/
    aggregationProof.endBlockNumber.toBigInteger(),
    /*shnarfData*/
    aggregationEndBlobInfo,
    /*lastFinalizedTimestamp*/
    aggregationProof.parentAggregationLastBlockTimestamp.epochSeconds.toBigInteger(),
    /*finalTimestamp*/
    aggregationProof.finalTimestamp.epochSeconds.toBigInteger(),
    /*lastFinalizedL1RollingHash*/
    parentL1RollingHash,
    /*l1RollingHash*/
    aggregationProof.l1RollingHash,
    /*lastFinalizedL1RollingHashMessageNumber*/
    parentL1RollingHashMessageNumber.toBigInteger(),
    /*l1RollingHashMessageNumber*/
    aggregationProof.l1RollingHashMessageNumber.toBigInteger(),
    /*l2MerkleTreesDepth*/
    aggregationProof.l2MerkleTreesDepth.toBigInteger(),
    /*l2MerkleRoots*/
    aggregationProof.l2MerkleRoots,
    /*l2MessagingBlocksOffsets*/
    aggregationProof.l2MessagingBlocksOffsets,
  )

  /**
   *  function finalizeBlocks(
   *     bytes calldata _aggregatedProof,
   *     uint256 _proofType,
   *     FinalizationDataV3 calldata _finalizationData
   *   )
   */
  val function = Function(
    LineaRollupV6.FUNC_FINALIZEBLOCKS,
    Arrays.asList<Type<*>>(
      DynamicBytes(aggregationProof.aggregatedProof),
      Uint256(aggregationProof.aggregatedVerifierIndex.toLong()),
      finalizationData,
    ),
    emptyList<TypeReference<*>>(),
  )
  return function
}
