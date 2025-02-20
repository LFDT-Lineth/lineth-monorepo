package linea.staterecovery.datafetching

import io.vertx.core.Vertx
import linea.domain.BlockParameter
import linea.domain.BlockParameter.Companion.toBlockParameter
import linea.domain.EthLogEvent
import linea.staterecovery.DataFinalizedV3
import linea.staterecovery.FinalizationAndDataEventsV3
import linea.staterecovery.LineaRollupSubmissionEventsClient
import net.consensys.zkevm.PeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedQueue
import java.util.concurrent.atomic.AtomicReference
import kotlin.time.Duration

internal class SubmissionEventsFetchingTask(
  private val vertx: Vertx,
  private val l1PollingInterval: Duration,
  private val l1EarliestBlockWithFinalizationThatSupportRecovery: BlockParameter,
  private val l2StartBlockNumber: ULong,
  private val submissionEventsClient: LineaRollupSubmissionEventsClient,
  private val submissionEventsQueue: ConcurrentLinkedQueue<FinalizationAndDataEventsV3>,
  private val queueLimit: Int,
  private val debugForceSyncStopBlockNumber: ULong?,
  private val log: Logger = LogManager.getLogger(SubmissionEventsFetchingTask::class.java)
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = l1PollingInterval.inWholeMilliseconds,
  log = log
) {
  val latestFetchedFinalization: AtomicReference<EthLogEvent<DataFinalizedV3>> = AtomicReference(null)

  override fun action(): SafeFuture<*> {
    if (debugForceSyncStopBlockNumber != null &&
      (latestFetchedFinalization.get()?.event?.endBlockNumber ?: 0UL) >= debugForceSyncStopBlockNumber
    ) {
      log.debug(
        "Force stop fetching submission events from L1, reached debugForceSyncStopBlockNumber={}",
        debugForceSyncStopBlockNumber
      )
      return this.stop()
    }

    return fetchSubmissionEvents()
  }

  private fun fetchSubmissionEvents(): SafeFuture<Unit> {
    if (submissionEventsQueue.size >= queueLimit) {
      // Queue is full, no need to fetch more
      log.debug(
        "skipping fetching submission events from L1, internal queue is full size={}",
        submissionEventsQueue.size
      )
      return SafeFuture.completedFuture(Unit)
    }

    return findNextFinalizationSubmissionEvents()
      .thenCompose { finalizationAndDataEvents ->
        if (finalizationAndDataEvents != null) {
          submissionEventsQueue.add(finalizationAndDataEvents)
          latestFetchedFinalization.set(finalizationAndDataEvents.dataFinalizedEvent)
          // fetch next finalization event
          fetchSubmissionEvents()
        } else {
          // no more events to fetch for now, wait for the next polling
          SafeFuture.completedFuture(Unit)
        }
      }
  }

  private fun findNextFinalizationSubmissionEvents(): SafeFuture<FinalizationAndDataEventsV3?> {
    return if (latestFetchedFinalization.get() != null) {
      log.trace(
        "fetching submission events from L1 startBlockNumber={}",
        latestFetchedFinalization.get().event.endBlockNumber + 1u
      )
      submissionEventsClient.findFinalizationAndDataSubmissionV3Events(
        fromL1BlockNumber = latestFetchedFinalization.get().log.blockNumber.toBlockParameter(),
        finalizationStartBlockNumber = latestFetchedFinalization.get().event.endBlockNumber + 1u
      )
    } else {
      log.trace(
        "fetching submission events from L1 startBlockNumber={}",
        l2StartBlockNumber
      )
      submissionEventsClient
        .findFinalizationAndDataSubmissionV3EventsContainingL2BlockNumber(
          fromL1BlockNumber = l1EarliestBlockWithFinalizationThatSupportRecovery,
          l2BlockNumber = l2StartBlockNumber
        )
    }
  }
}
