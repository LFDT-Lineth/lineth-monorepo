package net.consensys.zkevm.coordinator.clients

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import linea.domain.Block
import linea.domain.BlockInterval
import net.consensys.zkevm.domain.BridgeLogsData

data class BatchExecutionProofRequestV1(
  val blocks: List<Block>,
  val bridgeLogs: List<BridgeLogsData>,
  val tracesResponse: GenerateTracesResponse,
  val type2StateData: GetZkEVMStateMerkleProofResponse,
  val keccakParentStateRootHash: ByteArray
) : BlockInterval {
  override val startBlockNumber: ULong
    get() = blocks.first().number
  override val endBlockNumber: ULong
    get() = blocks.last().number
}

data class BatchExecutionProofResponse(
  override val startBlockNumber: ULong,
  override val endBlockNumber: ULong
) : BlockInterval
