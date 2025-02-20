package net.consensys.zkevm.coordinator.clients

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import linea.domain.Block
import linea.domain.BlockInterval

data class BatchExecutionProofRequestV1(
  val blocks: List<Block>,
  val tracesResponse: GenerateTracesResponse,
  val type2StateData: GetZkEVMStateMerkleProofResponse
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
