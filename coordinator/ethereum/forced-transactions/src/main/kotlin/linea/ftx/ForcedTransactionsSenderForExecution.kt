package linea.ftx

import io.vertx.core.Vertx
import linea.LongRunningService
import linea.contract.events.ForcedTransactionAddedEvent
import linea.forcedtx.ForcedTransactionRequest
import linea.forcedtx.ForcedTransactionsClient
import linea.timer.TimerSchedule
import linea.timer.VertxPeriodicPollingService
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

/**
 * Takes ForcedTransactionEventAdded from the queue and sends them to the sequencer.
 */
class ForcedTransactionsSenderForExecution(
  vertx: Vertx,
  val unprocessedFtxProvider: ForcedTransactionsProvider,
  val ftxClient: ForcedTransactionsClient,
  val pollingInterval: Duration,
  val log: Logger = LogManager.getLogger(ForcedTransactionsSenderForExecution::class.java),
  val txLimitToSendPerTick: Int = 10,
) :
  VertxPeriodicPollingService(
    vertx = vertx,
    pollingIntervalMs = pollingInterval.inWholeMilliseconds,
    log = log,
    timerSchedule = TimerSchedule.FIXED_DELAY,
    name = "ForcedTransactionsRelayerForExecution",
  ),
  LongRunningService {

  override fun action(): SafeFuture<*> {
    return unprocessedFtxProvider
      .getUnprocessedForcedTransactions()
      .thenCompose { unprocessedFtx ->
        log.debug(
          "unprocessed forced transactions ready for execution {}, ftxs={}",
          unprocessedFtx.size,
          unprocessedFtx.map { it.forcedTransactionNumber },
        )
        if (unprocessedFtx.isEmpty()) {
          SafeFuture.completedFuture(null)
        } else {
          this.sendTransactions(unprocessedTransactions = unprocessedFtx.take(txLimitToSendPerTick))
        }
      }
  }

  private fun sendTransactions(unprocessedTransactions: List<ForcedTransactionAddedEvent>): SafeFuture<*> {
    log.info(
      "sending {} forced transactions for execution: ftxs={}",
      unprocessedTransactions.size,
      unprocessedTransactions,
    )
    val requests = unprocessedTransactions.map {
      ForcedTransactionRequest(
        ftxNumber = it.forcedTransactionNumber,
        deadlineBlockNumber = it.blockNumberDeadline,
        ftxRlp = it.rlpEncodedSignedTransaction,
      )
    }

    return ftxClient
      .lineaSendForcedRawTransaction(requests)
      .thenPeek { responses ->
        log.debug("successfully sent forced transactions for execution: responses={}", responses)
      }
      .whenException { exception ->
        log.warn(
          "Failed sending forced transactions to sequencer, will retry in {}. errorMessage={} ftx={}",
          pollingInterval,
          exception.message,
          unprocessedTransactions,
          exception,
        )
      }
  }
}
