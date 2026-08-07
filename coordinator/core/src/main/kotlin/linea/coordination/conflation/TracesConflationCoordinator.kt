package linea.coordination.conflation

import lineth.clients.GenerateTracesResponse
import lineth.clients.GetZkEVMStateMerkleProofResponse
import tech.pegasys.teku.infrastructure.async.SafeFuture

data class BlocksTracesConflated(
  val tracesResponse: GenerateTracesResponse,
  val zkStateTraces: GetZkEVMStateMerkleProofResponse,
)

interface TracesConflationCoordinator {
  fun conflateExecutionTraces(blockRange: ULongRange): SafeFuture<BlocksTracesConflated>
}
