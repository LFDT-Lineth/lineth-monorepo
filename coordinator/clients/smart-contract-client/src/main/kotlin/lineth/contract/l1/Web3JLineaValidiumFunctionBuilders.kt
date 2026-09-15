package lineth.contract.l1

import linea.contract.ValidiumV1
import linea.contract.ValidiumV2
import linea.contract.l1.LineaValidiumContractVersion
import linea.domain.BlobRecord
import linea.domain.ProofToFinalize
import linea.kotlin.encodeHex
import linea.kotlin.toBigInteger
import org.web3j.abi.TypeReference
import org.web3j.abi.datatypes.DynamicBytes
import org.web3j.abi.datatypes.Function
import org.web3j.abi.datatypes.Type
import org.web3j.abi.datatypes.generated.Bytes32
import org.web3j.abi.datatypes.generated.Uint256
import java.util.Arrays

internal object Web3JLineaValidiumFunctionBuilders {
  fun buildAcceptShnarfDataFunction(version: LineaValidiumContractVersion, blobs: List<BlobRecord>): Function {
    return when (version) {
      // acceptShnarfData is byte-identical in V1 and V2, so both use the V1 encoding.
      LineaValidiumContractVersion.V1,
      LineaValidiumContractVersion.V2,
      -> buildAcceptShnarfDataFunctionV1(blobs)
    }
  }

  fun buildAcceptShnarfDataFunctionV1(blobs: List<BlobRecord>): Function {
    /**
     function acceptShnarfData(
     bytes32 _parentShnarf,
     bytes32 _shnarf,
     bytes32 _finalStateRootHash
     )
     */
    return Function(
      ValidiumV1.FUNC_ACCEPTSHNARFDATA,
      Arrays.asList<Type<*>>(
        Bytes32(blobs.first().blobCompressionProof!!.prevShnarf),
        Bytes32(blobs.last().blobCompressionProof!!.expectedShnarf),
        Bytes32(blobs.last().blobCompressionProof!!.finalStateRootHash),
      ),
      emptyList<TypeReference<*>>(),
    )
  }

  fun buildFinalizeBlocksFunction(
    version: LineaValidiumContractVersion,
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    when (version) {
      LineaValidiumContractVersion.V1 -> {
        return buildFinalizeBlockFunctionV1(
          aggregationProof,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber,
        )
      }

      LineaValidiumContractVersion.V2 -> {
        return buildFinalizeBlockFunctionV2(
          aggregationProof,
          aggregationLastBlob,
          parentL1RollingHash,
          parentL1RollingHashMessageNumber,
        )
      }
    }
  }

  fun buildFinalizeBlockFunctionV1(
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    val compressionProof = requireNotNull(aggregationLastBlob.blobCompressionProof) {
      "aggregationLastBlob.blobCompressionProof must be set when building the finalization function"
    }
    val aggregationEndBlobInfo =
      ValidiumV1.ShnarfData(
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

    val finalizationData =
      ValidiumV1.FinalizationDataV3(
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
        ValidiumV1.FUNC_FINALIZEBLOCKS,
        Arrays.asList<Type<*>>(
          DynamicBytes(aggregationProof.aggregatedProof),
          Uint256(aggregationProof.aggregatedVerifierIndex.toLong()),
          finalizationData,
        ),
        emptyList<TypeReference<*>>(),
      )
    return function
  }

  fun buildFinalizeBlockFunctionV2(
    aggregationProof: ProofToFinalize,
    aggregationLastBlob: BlobRecord,
    parentL1RollingHash: ByteArray,
    parentL1RollingHashMessageNumber: Long,
  ): Function {
    val compressionProof = requireNotNull(aggregationLastBlob.blobCompressionProof) {
      "aggregationLastBlob.blobCompressionProof must be set when building the finalization function"
    }
    val aggregationEndBlobInfo =
      ValidiumV2.ShnarfData(
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

    // V2's FinalizationDataV4 = V1's FinalizationDataV3 + forced-transaction fields (after
    // l2MerkleTreesDepth) + filteredAddresses (between l2MerkleRoots and l2MessagingBlocksOffsets).
    // Field order mirrors the rollup V8 builder (FunctionBuildersV8), which uses the same tuple.
    val finalizationData =
      ValidiumV2.FinalizationDataV4(
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
        // lastFinalizedForcedTransactionNumber
        aggregationProof.parentAggregationFtxNumber.toBigInteger(),
        // finalForcedTransactionNumber
        aggregationProof.finalFtxNumber.toBigInteger(),
        // lastFinalizedForcedTransactionRollingHash
        aggregationProof.parentAggregationFtxRollingHash,
        // l2MerkleRoots
        aggregationProof.l2MerkleRoots,
        // filteredAddresses
        aggregationProof.filteredAddresses.map { it.encodeHex() },
        // l2MessagingBlocksOffsets
        aggregationProof.l2MessagingBlocksOffsets,
      )

    /**
     *  function finalizeBlocks(
     *     bytes calldata _aggregatedProof,
     *     uint256 _proofType,
     *     FinalizationDataV4 calldata _finalizationData
     *   )
     */
    return Function(
      ValidiumV2.FUNC_FINALIZEBLOCKS,
      Arrays.asList<Type<*>>(
        DynamicBytes(aggregationProof.aggregatedProof),
        Uint256(aggregationProof.aggregatedVerifierIndex.toLong()),
        finalizationData,
      ),
      emptyList<TypeReference<*>>(),
    )
  }
}
