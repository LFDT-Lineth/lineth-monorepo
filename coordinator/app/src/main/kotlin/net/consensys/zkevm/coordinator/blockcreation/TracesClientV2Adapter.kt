package net.consensys.zkevm.coordinator.blockcreation

import com.github.michaelbull.result.Result
import linea.domain.BlockNumberAndHash
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.GenerateTracesResponse
import net.consensys.zkevm.coordinator.clients.GetTracesCountersResponse
import net.consensys.zkevm.coordinator.clients.TracesConflationClientV1
import net.consensys.zkevm.coordinator.clients.TracesConflationClientV2
import net.consensys.zkevm.coordinator.clients.TracesCountersClientV1
import net.consensys.zkevm.coordinator.clients.TracesCountersClientV2
import net.consensys.zkevm.coordinator.clients.TracesServiceErrorType
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesCountersClientV2Adapter(
  private val tracesCountersClientV2: TracesCountersClientV2
) : TracesCountersClientV1 {
  override fun rollupGetTracesCounters(
    block: BlockNumberAndHash
  ): SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>> {
    return tracesCountersClientV2.getTracesCounters(block.number)
  }
}

class TracesConflationClientV2Adapter(
  private val tracesConflationClientV2: TracesConflationClientV2
) : TracesConflationClientV1 {
  override fun rollupGenerateConflatedTracesToFile(
    blocks: List<BlockNumberAndHash>
  ): SafeFuture<Result<GenerateTracesResponse, ErrorResponse<TracesServiceErrorType>>> {
    return tracesConflationClientV2.generateConflatedTracesToFile(
      startBlockNumber = blocks.minOf { it.number },
      endBlockNumber = blocks.maxOf { it.number }
    )
  }
}
