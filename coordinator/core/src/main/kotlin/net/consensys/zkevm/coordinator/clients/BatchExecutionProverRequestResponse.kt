package net.consensys.zkevm.coordinator.clients

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import linea.domain.Block
import linea.domain.BlockInterval
import linea.domain.EthLog

data class BatchExecutionProofRequestV1(
  val blocks: List<Block>,
  val bridgeLogs: List<EthLog>,
  val tracesResponse: GenerateTracesResponse,
  val type2StateData: GetZkEVMStateMerkleProofResponse,
  val keccakParentStateRootHash: ByteArray
) : BlockInterval {
  override val startBlockNumber: ULong
    get() = blocks.first().number
  override val endBlockNumber: ULong
    get() = blocks.last().number

  override fun equals(other: Any?): Boolean {
    if (this === other) return true
    if (javaClass != other?.javaClass) return false

    other as BatchExecutionProofRequestV1

    if (blocks != other.blocks) return false
    if (bridgeLogs != other.bridgeLogs) return false
    if (tracesResponse != other.tracesResponse) return false
    if (type2StateData != other.type2StateData) return false
    if (!keccakParentStateRootHash.contentEquals(other.keccakParentStateRootHash)) return false

    return true
  }

  override fun hashCode(): Int {
    var result = blocks.hashCode()
    result = 31 * result + bridgeLogs.hashCode()
    result = 31 * result + tracesResponse.hashCode()
    result = 31 * result + type2StateData.hashCode()
    result = 31 * result + keccakParentStateRootHash.contentHashCode()
    return result
  }
}

data class BatchExecutionProofResponse(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong
) : BlockInterval
