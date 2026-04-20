package net.consensys.zkevm.ethereum.coordination.aggregation

import io.vertx.core.Vertx
import linea.clients.ProofAggregationProverClientV2
import linea.domain.Aggregation
import linea.domain.AggregationProofIndex
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration.Companion.milliseconds

class AggregationProofPoller(
  private val aggregationProofClient: ProofAggregationProverClientV2,
  private val aggregationProofHandler: AggregationProofHandler,
  private val log: Logger,
  vertx: Vertx,
  pollingIntervalMs: Long = 100.milliseconds.inWholeMilliseconds,
  name: String = "AggregationProofPoller",
  timerSchedule: TimerSchedule = TimerSchedule.FIXED_DELAY,
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingIntervalMs,
  log = log,
  name = name,
  timerSchedule = timerSchedule,
) {
  data class ProofInProgress(
    val proofIndex: AggregationProofIndex,
    val unProvenAggregation: Aggregation,
  )

  private val proofRequestsInProgress = ConcurrentLinkedDeque<ProofInProgress>()

  @Synchronized
  fun addProofRequestsInProgressForPolling(proofIndex: AggregationProofIndex, unProvenAggregation: Aggregation) {
    proofRequestsInProgress.add(ProofInProgress(proofIndex, unProvenAggregation))
  }

  override fun action(): SafeFuture<*> {
    return if (proofRequestsInProgress.isNotEmpty()) {
      val proofInProgress = proofRequestsInProgress.peekFirst()
      aggregationProofClient.findProofResponse(proofInProgress.proofIndex)
        .thenCompose { proofResponse ->
          if (proofResponse != null) {
            log.info(
              "aggregation proof generated: aggregation={}",
              proofInProgress.unProvenAggregation.intervalString(),
            )

            val provenAggregation = proofInProgress.unProvenAggregation.copy(aggregationProof = proofResponse)
            aggregationProofHandler.acceptNewAggregation(provenAggregation)
              .thenApply {
                proofRequestsInProgress.remove(proofInProgress)
              }
          } else {
            SafeFuture.completedFuture(Unit)
          }
        }
    } else {
      SafeFuture.completedFuture(Unit)
    }
  }
}
