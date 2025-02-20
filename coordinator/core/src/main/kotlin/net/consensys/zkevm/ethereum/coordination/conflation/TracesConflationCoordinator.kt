package net.consensys.zkevm.ethereum.coordination.conflation

import build.linea.clients.GetZkEVMStateMerkleProofResponse
import linea.domain.BlockNumberAndHash
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlocksTracesConflated(
  val tracesResponse: GenerateTracesResponse,
  val zkStateTraces: GetZkEVMStateMerkleProofResponse
)

interface TracesConflationCoordinator {
  fun conflateExecutionTraces(
    blocks: List<BlockNumberAndHash>
  ): SafeFuture<BlocksTracesConflated>
}
