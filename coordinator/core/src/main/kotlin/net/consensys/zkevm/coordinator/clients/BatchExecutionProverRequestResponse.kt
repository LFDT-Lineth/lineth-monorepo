package net.consensys.zkevm.coordinator.clients

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.domain.BlockInterval
import linea.domain.Block

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
