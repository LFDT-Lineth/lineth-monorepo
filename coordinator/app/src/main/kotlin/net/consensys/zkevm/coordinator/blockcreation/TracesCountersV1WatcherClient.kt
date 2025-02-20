package net.consensys.zkevm.coordinator.blockcreation

import com.github.michaelbull.result.Result
import linea.domain.BlockNumberAndHash
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.GetTracesCountersResponse
import net.consensys.zkevm.coordinator.clients.TracesCountersClientV1
import net.consensys.zkevm.coordinator.clients.TracesServiceErrorType
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.apache.tuweni.bytes.Bytes32
import tech.pegasys.teku.infrastructure.async.SafeFuture

class TracesCountersV1WatcherClient(
  private val tracesFilesManager: TracesFilesManager,
  private val tracesCountersClientV1: TracesCountersClientV1,
  private val log: Logger = LogManager.getLogger(TracesCountersV1WatcherClient::class.java)
) : TracesCountersClientV1 {
  override fun rollupGetTracesCounters(block: BlockNumberAndHash):
    SafeFuture<Result<GetTracesCountersResponse, ErrorResponse<TracesServiceErrorType>>> {
    return tracesFilesManager.waitRawTracesGenerationOf(block.number, Bytes32.wrap(block.hash)).thenCompose {
      log.trace("Traces file generated: block={}", block.number)
      tracesCountersClientV1.rollupGetTracesCounters(block)
    }
  }
}
