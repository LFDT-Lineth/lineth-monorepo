package net.consensys.zkevm.ethereum.coordination.blob

import io.vertx.core.Vertx
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.linea.metrics.LineaMetricsCategory
import net.consensys.linea.metrics.MetricsFacade
import net.consensys.zkevm.coordinator.clients.BlobCompressionProverClientV2
import net.consensys.zkevm.domain.BlobRecord
import net.consensys.zkevm.domain.CompressionProofIndex
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ConcurrentLinkedDeque
import kotlin.time.Duration.Companion.milliseconds

class BlobCompressionProofPoller(
  private val blobCompressionProverClient: BlobCompressionProverClientV2,
  private val blobCompressionProofHandler: BlobCompressionProofHandler,
  vertx: Vertx,
  private val log: Logger,
  pollingIntervalMs: Long = 100.milliseconds.inWholeMilliseconds,
  name: String = "BlobCompressionProofPoller",
  timerSchedule: TimerSchedule = TimerSchedule.FIXED_DELAY,
  metricsFacade: MetricsFacade,
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingIntervalMs,
  log = log,
  name = name,
  timerSchedule = timerSchedule,
) {
  data class ProofInProgress(
    val proofIndex: CompressionProofIndex,
    val unProvenBlobRecord: BlobRecord,
  )

  private val proofRequestsInProgress = ConcurrentLinkedDeque<ProofInProgress>()

  init {
    metricsFacade.createGauge(
      category = LineaMetricsCategory.BLOB,
      name = "prover.pendingproofs",
      description = "Number of blob compression proof waiting responses",
      measurementSupplier = { proofRequestsInProgress.size },
    )
  }

  @Synchronized
  fun addProofRequestsInProgressForPolling(proofIndex: CompressionProofIndex, unProvenBlobRecord: BlobRecord) {
    proofRequestsInProgress.add(ProofInProgress(proofIndex, unProvenBlobRecord))
  }

  override fun action(): SafeFuture<*> {
    return if (proofRequestsInProgress.isNotEmpty()) {
      val proofInProgress = proofRequestsInProgress.peekFirst()
      blobCompressionProverClient.findProofResponse(proofInProgress.proofIndex)
        .thenCompose { proofResponse ->
          if (proofResponse != null) {
            log.info(
              "blob compression proof generated: blob={}",
              proofInProgress.unProvenBlobRecord.intervalString(),
            )
            val provenBlobRecord = proofInProgress.unProvenBlobRecord.copy(blobCompressionProof = proofResponse)
            blobCompressionProofHandler.acceptNewBlobCompressionProof(provenBlobRecord)
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
