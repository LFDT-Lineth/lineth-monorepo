package linea.ftx.conflation

import io.vertx.core.Vertx
import linea.persistence.ftx.ForcedTransactionsDao
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import net.consensys.zkevm.domain.ForcedTransactionRecord
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

/**
 * Coordinates forced transaction processing with invalidity proof generation.
 *
 * Responsibilities:
 * 1. Monitor FTX status updates (polling)
 * 2. Request invalidity proofs when FTX is processed
 * 3. Track invalidity proof generation status
 * 4. Retry proof requests on failure with exponential backoff
 */
class ForcedTransactionsInvalidityProofService(
  private val ftxDao: ForcedTransactionsDao,
  private val invalidityProofAssembler: InvalidityProofAssembler,
  vertx: Vertx,
  pollingInterval: Duration = 5.seconds,
  log: Logger = LogManager.getLogger(ForcedTransactionsInvalidityProofService::class.java),
) : VertxPeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval.inWholeMilliseconds,
  log = log,
  name = "ForcedTransactionsInvalidityProofService",
  timerSchedule = TimerSchedule.FIXED_DELAY,
) {
  override fun action(): SafeFuture<*> = tick()

  @Synchronized
  private fun tick(): SafeFuture<Void> {
    return ftxDao
      .list()
      .thenCompose { allFtxInDb ->
        val requestFutures = allFtxInDb
          .filter { it.proofStatus == ForcedTransactionRecord.ProofStatus.UNREQUESTED }
          .map { ftxRecord ->
            invalidityProofAssembler
              .requestInvalidityProof(ftxRecord)
              .thenCompose {
                ftxDao.save(ftxRecord.copy(proofStatus = ForcedTransactionRecord.ProofStatus.PROVEN))
              }
          }

        SafeFuture
          .collectAll(requestFutures.stream())
          .thenApply { null }
      }
  }
}
