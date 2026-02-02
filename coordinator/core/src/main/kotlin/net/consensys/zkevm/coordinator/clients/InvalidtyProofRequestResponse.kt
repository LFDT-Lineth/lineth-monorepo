package net.consensys.zkevm.coordinator.clients

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.LineaAccountProof
import kotlin.time.Instant

enum class InvalidityReason {
  BadNonce,
  BadBalance,
  BadPrecompile,
  TooManyLogs,
  FilteredAddresses,
}

data class InvalidityProofRequest(
  val invalidityReason: InvalidityReason,
  val simulatedExecutionBlockNumber: ULong,
  val simulatedExecutionBlockTimestamp: Instant,
  val ftxNumber: ULong,
  val ftxBlockNumberDeadline: ULong,
  val ftxHash: ByteArray,
  val ftxRlp: ByteArray,
  val prevFtxRollingHash: ByteArray,
  val parentBlockHash: ByteArray,
  val zkParentStateRootHash: ByteArray,
  val tracesResponse: GenerateTracesResponse,
  /**
   * Account MerkleProof is defined when
   * invalidityReason = BadNonce, BadBalance
   */
  val accountProof: LineaAccountProof? = null,
  /**
   * type2StateData needs to needs provided when
   * invalidityReason is one of {BadPrecompile, TooManyLogs}, null otherwise
   */
  val zkStateMerkleProof: GetZkEVMStateMerkleProofResponse? = null,
) {
  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as InvalidityProofRequest

    if (invalidityReason != other.invalidityReason) return false
    if (simulatedExecutionBlockNumber != other.simulatedExecutionBlockNumber) return false
    if (simulatedExecutionBlockTimestamp != other.simulatedExecutionBlockTimestamp) return false
    if (ftxNumber != other.ftxNumber) return false
    if (ftxBlockNumberDeadline != other.ftxBlockNumberDeadline) return false
    if (!ftxHash.contentEquals(other.ftxHash)) return false
    if (!ftxRlp.contentEquals(other.ftxRlp)) return false
    if (!prevFtxRollingHash.contentEquals(other.prevFtxRollingHash)) return false
    if (!parentBlockHash.contentEquals(other.parentBlockHash)) return false
    if (!zkParentStateRootHash.contentEquals(other.zkParentStateRootHash)) return false
    if (tracesResponse != other.tracesResponse) return false
    if (accountProof != other.accountProof) return false
    if (zkStateMerkleProof != other.zkStateMerkleProof) return false

    return true
  }

  override fun hashCode(): Int {
    var result = invalidityReason.hashCode()
    result = 31 * result + simulatedExecutionBlockNumber.hashCode()
    result = 31 * result + simulatedExecutionBlockTimestamp.hashCode()
    result = 31 * result + ftxNumber.hashCode()
    result = 31 * result + ftxBlockNumberDeadline.hashCode()
    result = 31 * result + ftxHash.contentHashCode()
    result = 31 * result + ftxRlp.contentHashCode()
    result = 31 * result + prevFtxRollingHash.contentHashCode()
    result = 31 * result + parentBlockHash.contentHashCode()
    result = 31 * result + zkParentStateRootHash.contentHashCode()
    result = 31 * result + tracesResponse.hashCode()
    result = 31 * result + (accountProof?.hashCode() ?: 0)
    result = 31 * result + (zkStateMerkleProof?.hashCode() ?: 0)
    return result
  }

  override fun toString(): String {
    return "InvalidityProofRequest(" +
      "invalidityReason=$invalidityReason, " +
      "simulatedExecutionBlockNumber=$simulatedExecutionBlockNumber, " +
      "simulatedExecutionBlockTimestamp=$simulatedExecutionBlockTimestamp, " +
      "ftxNumber=$ftxNumber, " +
      "ftxBlockNumberDeadline=$ftxBlockNumberDeadline, " +
      "ftxRlp=${ftxRlp.contentToString()}, " +
      "prevFtxRollingHash=${prevFtxRollingHash.contentToString()}, " +
      "parentBlockHash=${parentBlockHash.contentToString()}, " +
      "zkParentStateRootHash=${zkParentStateRootHash.contentToString()}, " +
      "tracesResponse=$tracesResponse, " +
      "accountProof=$accountProof, " +
      "zkStateMerkleProof=$zkStateMerkleProof" +
      ")"
  }
}

data class InvalidityProofResponse(val ftxNumber: ULong)
