package net.consensys.zkevm.ethereum.coordination.conflation

import com.github.michaelbull.result.Err
import com.github.michaelbull.result.Ok
import io.vertx.core.Vertx
import kotlinx.datetime.Instant
import linea.domain.Block
import net.consensys.linea.async.toSafeFuture
import net.consensys.linea.errors.ErrorResponse
import net.consensys.zkevm.coordinator.clients.GetTracesCountersResponse
import net.consensys.zkevm.coordinator.clients.TracesCountersClientV2
import net.consensys.zkevm.coordinator.clients.TracesServiceErrorType
import net.consensys.zkevm.domain.BlockCounters
import net.consensys.zkevm.encoding.BlockEncoder
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreated
import net.consensys.zkevm.ethereum.coordination.blockcreation.BlockCreationListener
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.Callable

class BlockToBatchSubmissionCoordinator(
  private val conflationService: ConflationService,
  private val tracesCountersClient: TracesCountersClientV2,
  private val vertx: Vertx,
  private val encoder: BlockEncoder,
  private val log: Logger = LogManager.getLogger(BlockToBatchSubmissionCoordinator::class.java),
) : BlockCreationListener {
  private fun getTracesCounters(block: Block): SafeFuture<GetTracesCountersResponse> {
    return tracesCountersClient
      .getTracesCounters(block.number)
      .thenCompose { result ->
        when (result) {
          is Err<ErrorResponse<TracesServiceErrorType>> -> {
            SafeFuture.failedFuture(result.error.asException("Traces api error: "))
          }

          is Ok<GetTracesCountersResponse> -> {
            SafeFuture.completedFuture(result.value)
          }
        }
      }
  }

  override fun acceptBlock(blockEvent: BlockCreated): SafeFuture<Unit> {
    log.debug("accepting new block={}", blockEvent.block.number)
    encodeBlock(blockEvent.block)
      .thenCombine(getTracesCounters(blockEvent.block)) { blockRLPEncoded, traces ->
        conflationService.newBlock(
          blockEvent.block,
          BlockCounters(
            blockNumber = blockEvent.block.number,
            blockTimestamp = Instant.fromEpochSeconds(blockEvent.block.timestamp.toLong()),
            tracesCounters = traces.tracesCounters,
            blockRLPEncoded = blockRLPEncoded,
            numOfTransactions = blockEvent.block.transactions.size.toUInt(),
            gasUsed = blockEvent.block.gasUsed,
          ),
        )
      }.whenException { th ->
        log.error(
          "Failed to conflate block={} errorMessage={}",
          blockEvent.block.number,
          th.message,
          th,
        )
      }

    // This is to parallelize `getTracesCounters` requests which would otherwise be sent sequentially
    return SafeFuture.completedFuture(Unit)
  }

  private fun encodeBlock(block: Block): SafeFuture<ByteArray> {
    return vertx.executeBlocking(
      Callable {
        encoder.encode(block)
      },
    )
      .toSafeFuture()
  }
}
