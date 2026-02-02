package net.consensys.zkevm.ethereum.coordination.conflation

import build.linea.clients.GetStateMerkleProofRequest
import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.StateManagerClientV1
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.mapBoth
import linea.domain.BlockInterval
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.coordinator.clients.TracesConflationClientV2
import net.consensys.zkevm.coordinator.clients.TracesServiceErrorType
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesConflationCoordinatorImpl(
  private val tracesConflationClient: TracesConflationClientV2,
  private val zkStateClient: StateManagerClientV1,
  private val log: Logger = LogManager.getLogger(TracesConflationCoordinatorImpl::class.java),
) : TracesConflationCoordinator {

  private fun requestConflatedTraces(blockRange: ULongRange): SafeFuture<GenerateTracesResponse> {
    return tracesConflationClient
      .generateConflatedTracesToFile(
        startBlockNumber = blockRange.first(),
        endBlockNumber = blockRange.last(),
      )
      .thenCompose { result: Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>,
        ->
        result.mapBoth(
          { SafeFuture.completedFuture(it) },
          {
            log.error("Conflation service returned error: errorMessage={}", it.message, it)
            SafeFuture.failedFuture(it.asException("Conflation service error"))
          },
        )
      }
  }

  private fun requestStateMerkleProof(blockRange: ULongRange): SafeFuture<GetZkEVMStateMerkleProofResponse> {
    return zkStateClient.makeRequest(
      GetStateMerkleProofRequest(BlockInterval(blockRange.first(), blockRange.last())),
    )
  }

  override fun conflateExecutionTraces(blockRange: ULongRange): SafeFuture<BlocksTracesConflated> {
    return requestConflatedTraces(blockRange).thenCompose { tracesConflationResult: GenerateTracesResponse ->
      // these 2 requests can be done in parallel, but traces-api is much slower to respond so
      // requesting stateManger after traces-API because
      // we want to avoid having stateManager heavy JSON responses in memory in the meantime
      requestStateMerkleProof(blockRange).thenApply { zkStateUpdateResult: GetZkEVMStateMerkleProofResponse ->
        BlocksTracesConflated(tracesConflationResult, zkStateUpdateResult)
      }
    }
  }
}
