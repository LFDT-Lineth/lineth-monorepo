package linea.clients

import linea.domain.BlockInterval
import linea.domain.BlockIntervalProofIndex
import linea.domain.StartBlockTimestampProvider
import linea.domain.assertConsecutiveIntervals
import linea.kotlin.byteArrayListEquals
import linea.kotlin.byteArrayListHashCode
import linea.kotlin.byteArrayListToHexString
import linea.kotlin.encodeHex
import kotlin.time.Instant

/**
 * Domain request of a rollup proof (rollup_spec §2.2, `RollupProofPrivateInput`).
 *
 * [parentDataTailTakeBytes] is 0 at a chunk boundary. A positive value means the proof starts mid-chunk: it takes the
 * last N bytes of the first chunk's unpacked payload and requires [boundaryPrevDataRollingHash] (§3.4).
 */
data class RollupProofRequestV1(
  val conflations: List<ConflationWitness>,
  val l2Executions: List<BlockIntervalProofIndex>,
  val chunks: List<ChunkWitness>,
  val parentDataRollingHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val boundaryPrevDataRollingHash: ByteArray? = null,
) : BlockInterval, StartBlockTimestampProvider {
  init {
    assertConsecutiveIntervals(l2Executions)
    require(parentDataTailTakeBytes in 0..MAX_TAIL_BYTES) {
      "parentDataTailTakeBytes=$parentDataTailTakeBytes must be in 0..$MAX_TAIL_BYTES"
    }
    require(parentDataTailTakeBytes == 0 || boundaryPrevDataRollingHash != null) {
      "boundaryPrevDataRollingHash is required when parentDataTailTakeBytes > 0"
    }
  }

  override val startBlockNumber: ULong
    get() = l2Executions.first().startBlockNumber
  override val endBlockNumber: ULong
    get() = l2Executions.last().endBlockNumber
  override val startBlockTimestamp: Instant
    get() = l2Executions.first().startBlockTimestamp

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofRequestV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (startBlockTimestamp != other.startBlockTimestamp) return false
    if (conflations != other.conflations) return false
    if (l2Executions != other.l2Executions) return false
    if (chunks != other.chunks) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (parentDataTailTakeBytes != other.parentDataTailTakeBytes) return false
    if (!boundaryPrevDataRollingHash.contentEquals(other.boundaryPrevDataRollingHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + startBlockTimestamp.hashCode()
    result = 31 * result + conflations.hashCode()
    result = 31 * result + l2Executions.hashCode()
    result = 31 * result + chunks.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + parentDataTailTakeBytes
    result = 31 * result + (boundaryPrevDataRollingHash?.contentHashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "RollupProofRequestV1(conflations=$conflations, l2Executions=$l2Executions, " +
      "chunks=$chunks, parentDataRollingHash=${parentDataRollingHash.encodeHex()}, " +
      "parentDataTailTakeBytes=$parentDataTailTakeBytes, " +
      "boundaryPrevDataRollingHash=${boundaryPrevDataRollingHash?.encodeHex()})"
  }

  companion object {
    /** Upper bound of a blob tail byte count (`tailCount` in the V1 schemas, §3.1). */
    const val MAX_TAIL_BYTES = 130046
  }
}

data class ConflationWitness(
  val blockRlps: List<ByteArray>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ConflationWitness

    if (!blockRlps.byteArrayListEquals(other.blockRlps)) return false

    return true
  }

  override fun hashCode(): Int = blockRlps.byteArrayListHashCode()

  override fun toString(): String {
    return "ConflationWitness(blockRlps=${blockRlps.byteArrayListToHexString()})"
  }
}

/**
 * One touched DA chunk of a rollup proof (rollup_spec §3.1, `ChunkWitness`).
 *
 * [chunkHash] is the chunk's L1-anchored binding hash: the KZG versioned hash of [kzgCommitment] for a blob chunk, or
 * `keccak256(calldataBytes)` for a calldata chunk. A blob chunk carries the physical EIP-4844 blob in [blobBytes], its
 * 48-byte [kzgCommitment], the 48-byte EIP-4844 blob proof [kzgProof] of [blobBytes] against that commitment, and
 * empty [calldataBytes]. A calldata chunk carries the exact submitted bytes in [calldataBytes] and empty [blobBytes],
 * [kzgCommitment] and [kzgProof].
 */
data class ChunkWitness(
  val chunkHash: ByteArray,
  val isCalldata: Boolean,
  val blobBytes: ByteArray,
  val calldataBytes: ByteArray,
  val kzgCommitment: ByteArray,
  val kzgProof: ByteArray,
) {
  init {
    if (isCalldata) {
      require(
        blobBytes.isEmpty() && calldataBytes.isNotEmpty() && kzgCommitment.isEmpty() && kzgProof.isEmpty(),
      ) {
        "calldata chunk must have non-empty calldataBytes and empty blobBytes, kzgCommitment and kzgProof"
      }
    } else {
      require(blobBytes.size == BLOB_BYTES_LENGTH && calldataBytes.isEmpty()) {
        "blob chunk must have exactly $BLOB_BYTES_LENGTH blobBytes and empty calldataBytes, got " +
          "blobBytes.size=${blobBytes.size} calldataBytes.size=${calldataBytes.size}"
      }
      require(kzgCommitment.size == KZG_POINT_BYTES_LENGTH && kzgProof.size == KZG_POINT_BYTES_LENGTH) {
        "blob chunk must have $KZG_POINT_BYTES_LENGTH-byte kzgCommitment and kzgProof, got " +
          "kzgCommitment.size=${kzgCommitment.size} kzgProof.size=${kzgProof.size}"
      }
    }
  }

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as ChunkWitness

    if (!chunkHash.contentEquals(other.chunkHash)) return false
    if (isCalldata != other.isCalldata) return false
    if (!blobBytes.contentEquals(other.blobBytes)) return false
    if (!calldataBytes.contentEquals(other.calldataBytes)) return false
    if (!kzgCommitment.contentEquals(other.kzgCommitment)) return false
    if (!kzgProof.contentEquals(other.kzgProof)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = chunkHash.contentHashCode()
    result = 31 * result + isCalldata.hashCode()
    result = 31 * result + blobBytes.contentHashCode()
    result = 31 * result + calldataBytes.contentHashCode()
    result = 31 * result + kzgCommitment.contentHashCode()
    result = 31 * result + kzgProof.contentHashCode()
    return result
  }

  override fun toString(): String {
    return "ChunkWitness(chunkHash=${chunkHash.encodeHex()}, isCalldata=$isCalldata, " +
      "blobBytes.size=${blobBytes.size}, calldataBytes.size=${calldataBytes.size}, " +
      "kzgCommitment=${kzgCommitment.encodeHex()}, kzgProof=${kzgProof.encodeHex()})"
  }

  companion object {
    /** Physical EIP-4844 blob size: 4096 field elements of 32 bytes. */
    const val BLOB_BYTES_LENGTH = 131072

    /** Compressed BLS12-381 G1 point, the size of a KZG commitment and of a KZG proof. */
    const val KZG_POINT_BYTES_LENGTH = 48
  }
}

/**
 * The PI tuple emitted by a rollup proof (rollup_spec §2.4, `RollupPublicInput`).
 *
 * Domain twin of `lineth.coordinator.clients.prover.RollupProofPublicInputsDto`. Kept here (rather than reusing the
 * DTO) because this module is depended upon by the prover-client modules, not the other way around. Where the DTO
 * uses `String` (hex) this uses `ByteArray`, and where the DTO uses `Long` this uses `ULong`.
 *
 * [programVks] is the set of guest program VKs verified beneath this proof, encoded as a distinct list sorted
 * ascending by byte value.
 */
data class RollupProofPublicInputs(
  val endBlockNumber: ULong,
  val endBlockTimestamp: Instant,
  val endL1L2BridgeMessageRollingHash: ByteArray,
  val endL1L2BridgeMessageNumber: ULong,
  val dynamicChainConfigHash: ByteArray,
  val parentFtxRollingHash: ByteArray,
  val parentFtxNumber: ULong,
  val endFtxRollingHash: ByteArray,
  val endFtxNumber: ULong,
  val parentDataRollingHash: ByteArray,
  val endDataRollingHash: ByteArray,
  val parentBlockHash: ByteArray,
  val endBlockHash: ByteArray,
  val parentDataTailTakeBytes: Int,
  val finalDataTailDiscardBytes: Int,
  val l2L1Messages: List<ByteArray>,
  val filteredAddresses: List<ByteArray>,
  val programVks: List<ByteArray>,
  val blockCount: ULong,
  val l2MessagingBlocksOffsets: List<ULong>,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofPublicInputs

    if (endBlockNumber != other.endBlockNumber) return false
    if (endBlockTimestamp != other.endBlockTimestamp) return false
    if (!endL1L2BridgeMessageRollingHash.contentEquals(other.endL1L2BridgeMessageRollingHash)) return false
    if (endL1L2BridgeMessageNumber != other.endL1L2BridgeMessageNumber) return false
    if (!dynamicChainConfigHash.contentEquals(other.dynamicChainConfigHash)) return false
    if (!parentFtxRollingHash.contentEquals(other.parentFtxRollingHash)) return false
    if (parentFtxNumber != other.parentFtxNumber) return false
    if (!endFtxRollingHash.contentEquals(other.endFtxRollingHash)) return false
    if (endFtxNumber != other.endFtxNumber) return false
    if (!parentDataRollingHash.contentEquals(other.parentDataRollingHash)) return false
    if (!endDataRollingHash.contentEquals(other.endDataRollingHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!endBlockHash.contentEquals(other.endBlockHash)) return false
    if (parentDataTailTakeBytes != other.parentDataTailTakeBytes) return false
    if (finalDataTailDiscardBytes != other.finalDataTailDiscardBytes) return false
    if (!l2L1Messages.byteArrayListEquals(other.l2L1Messages)) return false
    if (!filteredAddresses.byteArrayListEquals(other.filteredAddresses)) return false
    if (!programVks.byteArrayListEquals(other.programVks)) return false
    if (blockCount != other.blockCount) return false
    if (l2MessagingBlocksOffsets != other.l2MessagingBlocksOffsets) return false

    return true
  }

  override fun hashCode(): Int {
    var result = endBlockNumber.hashCode()
    result = 31 * result + endBlockTimestamp.hashCode()
    result = 31 * result + endL1L2BridgeMessageRollingHash.contentHashCode()
    result = 31 * result + endL1L2BridgeMessageNumber.hashCode()
    result = 31 * result + dynamicChainConfigHash.contentHashCode()
    result = 31 * result + parentFtxRollingHash.contentHashCode()
    result = 31 * result + parentFtxNumber.hashCode()
    result = 31 * result + endFtxRollingHash.contentHashCode()
    result = 31 * result + endFtxNumber.hashCode()
    result = 31 * result + parentDataRollingHash.contentHashCode()
    result = 31 * result + endDataRollingHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + endBlockHash.contentHashCode()
    result = 31 * result + parentDataTailTakeBytes
    result = 31 * result + finalDataTailDiscardBytes
    result = 31 * result + l2L1Messages.byteArrayListHashCode()
    result = 31 * result + filteredAddresses.byteArrayListHashCode()
    result = 31 * result + programVks.byteArrayListHashCode()
    result = 31 * result + blockCount.hashCode()
    result = 31 * result + l2MessagingBlocksOffsets.hashCode()
    return result
  }
}

/**
 * Response of a rollup proof.
 *
 * Mirrors `lineth.coordinator.clients.prover.RollupProofResponseDto`: the DTO's `String` (hex) fields are
 * `ByteArray` here so a proof response — whether read from a JSON file or returned by a REST endpoint — deserializes
 * into the DTO and maps onto this domain type. L2-to-L1 messages and filtered addresses live in [publicInputs]; a
 * rollup proof builds no message trees (the rollup-aggregation proof does).
 */
data class RollupProofResponseV1(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong,
  val proof: ByteArray,
  val publicInputs: RollupProofPublicInputs,
  val programVk: ByteArray,
) : BlockInterval {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as RollupProofResponseV1

    if (startBlockNumber != other.startBlockNumber) return false
    if (endBlockNumber != other.endBlockNumber) return false
    if (!proof.contentEquals(other.proof)) return false
    if (publicInputs != other.publicInputs) return false
    if (!programVk.contentEquals(other.programVk)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = startBlockNumber.hashCode()
    result = 31 * result + endBlockNumber.hashCode()
    result = 31 * result + proof.contentHashCode()
    result = 31 * result + publicInputs.hashCode()
    result = 31 * result + programVk.contentHashCode()
    return result
  }
}
