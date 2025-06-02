package net.consensys.linea.transactionexclusion.service

import io.vertx.core.Vertx
import kotlinx.datetime.Clock
import net.consensys.zkevm.PeriodicPollingService
import net.consensys.zkevm.persistence.dao.rejectedtransaction.RejectedTransactionsDao
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

class RejectedTransactionCleanupService(
  vertx: Vertx,
  private val config: Config,
  private val repository: RejectedTransactionsDao,
  private val clock: Clock = Clock.System,
  private val log: Logger = LogManager.getLogger(RejectedTransactionCleanupService::class.java),
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = config.pollingInterval.inWholeMilliseconds,
  log = log,
) {
  data class Config(
    val pollingInterval: Duration,
    val storagePeriod: Duration,
  )

  override fun action(): SafeFuture<*> {
    return this.repository.deleteRejectedTransactions(
      clock.now().minus(config.storagePeriod),
    ).thenPeek { deletedRows ->
      if (deletedRows > 0) {
        log.debug("deletedRows=$deletedRows")
      }
    }
  }

  override fun handleError(error: Throwable) {
    log.error(
      "Error with rejected transaction cleanup service: errorMessage={}",
      error.message,
      error,
    )
  }
}
