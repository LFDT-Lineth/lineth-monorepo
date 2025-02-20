package net.consensys.zkevm.ethereum.coordination.conflation

import build.linea.clients.GetStateMerkleProofRequest
import build.linea.clients.GetZkEVMStateMerkleProofResponse
import build.linea.clients.StateManagerClientV1
import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import com.github.michaelbull.result.Result
import com.github.michaelbull.result.getOrElse
import com.github.michaelbull.result.map
import com.github.michaelbull.result.mapBoth
import linea.domain.BlockInterval
import linea.domain.BlockNumberAndHash
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.coordinator.clients.TracesConflationClientV1
import net.consensys.zkevm.coordinator.clients.TracesServiceErrorType
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesConflationCoordinatorImpl(
  private val tracesConflationClient: TracesConflationClientV1,
  private val zkStateClient: StateManagerClientV1
) : TracesConflationCoordinator {
  private val log: Logger = LogManager.getLogger(this::class.java)
  private fun requestConflatedTraces(
    blocks: List<BlockNumberAndHash>
  ): SafeFuture<GenerateTracesResponse> {
    return tracesConflationClient
      .rollupGenerateConflatedTracesToFile(blocks)
      .thenCompose { result: Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>
        ->
        result.mapBoth(
          { SafeFuture.completedFuture(it) },
          {
            log.error("Conflation service returned error: errorMessage={}", it.message, it)
            SafeFuture.failedFuture(it.asException("Conflation service error"))
          }
        )
      }
  }

  private fun requestStateMerkleProof(
    startBlockNumber: ULong,
    endBlockNumber: ULong
  ): SafeFuture<GetZkEVMStateMerkleProofResponse> {
    return zkStateClient.makeRequest(GetStateMerkleProofRequest(BlockInterval(startBlockNumber, endBlockNumber)))
  }

  override fun conflateExecutionTraces(
    blocks: List<BlockNumberAndHash>
  ): SafeFuture<BlocksTracesConflated> {
    return assertBlocksList(blocks).map { sortedByNumber ->
      requestConflatedTraces(blocks).thenCompose { tracesConflationResult: GenerateTracesResponse ->
        // these 2 requests can be done in parallel, but traces-api is much slower to respond so
        // requesting stateManger after traces-API because
        // and we want to avoid having stateManager heavy JSON responses in memory in the meantime
        requestStateMerkleProof(
          sortedByNumber.first().number,
          sortedByNumber.last().number
        ).thenApply { zkStateUpdateResult: GetZkEVMStateMerkleProofResponse ->
          BlocksTracesConflated(tracesConflationResult, zkStateUpdateResult)
        }
      }
    }.getOrElse { SafeFuture.failedFuture(it) }
  }
}

internal fun assertBlocksList(
  blocks: List<BlockNumberAndHash>
): Result<List<BlockNumberAndHash>, IllegalArgumentException> {
  if (blocks.isEmpty()) {
    return Err(IllegalArgumentException("Empty list of blocs"))
  }

  if (blocks.size == 1) {
    return Ok(blocks)
  }

  val sortedByNumber = blocks.sortedBy { it.number }
  var prevBlockNumber = sortedByNumber.first().number
  var gapFound = false
  for (i in 1 until sortedByNumber.size) {
    val block = sortedByNumber[i]
    if (block.number != prevBlockNumber + 1u) {
      gapFound = true
      break
    }
    prevBlockNumber = block.number
  }

  if (gapFound) {
    return Err(IllegalArgumentException("Conflated blocks list has non consecutive blocks!"))
  }
  return Ok(sortedByNumber)
}
