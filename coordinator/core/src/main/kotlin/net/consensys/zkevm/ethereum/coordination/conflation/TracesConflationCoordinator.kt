package net.consensys.zkevm.ethereum.coordination.conflation

import linea.clients.GenerateTracesResponse
import linea.clients.GetZkEVMStateMerkleProofResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlocksTracesConflated(
  val tracesResponse: GenerateTracesResponse,
  val zkStateTraces: GetZkEVMStateMerkleProofResponse,
)

interface TracesConflationCoordinator {
  fun conflateExecutionTraces(blockRange: ULongRange): SafeFuture<BlocksTracesConflated>
}
