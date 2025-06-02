package net.consensys.zkevm

import io.vertx.core.Vertx
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture

abstract class PeriodicPollingService(
  private val vertx: Vertx,
  private val pollingIntervalMs: Long,
  private val log: Logger,
) : LongRunningService {
  private var timerId: Long? = null

  init {
    require(pollingIntervalMs > 0) {
      "pollingIntervalMs must be greater than 0"
    }
  }
  abstract fun action(): SafeFuture<*>
  open fun handleError(error: Throwable) {
    log.error("Error with period polling service: errorMessage={}", error.message, error)
  }

  @Synchronized
  override fun start(): SafeFuture<Unit> {
    if (timerId == null) {
      timerId = vertx.setTimer(pollingIntervalMs, this::actionHandler)
    }
    return SafeFuture.completedFuture(Unit)
  }

  @Synchronized
  override fun stop(): SafeFuture<Unit> {
    if (timerId != null) {
      vertx.cancelTimer(timerId!!)
      timerId = null
    } else {
      log.info("Service is not running to stop it!")
    }
    return SafeFuture.completedFuture(Unit)
  }

  @Suppress("UNUSED_PARAMETER")
  private fun actionHandler(_timerId: Long) {
    try {
      action()
        .whenComplete { _, error ->
          error?.let(::handleError)
          if (timerId != null) {
            timerId = vertx.setTimer(pollingIntervalMs, this::actionHandler)
          }
        }
    } catch (e: Exception) {
      handleError(e)
      if (timerId != null) {
        timerId = vertx.setTimer(pollingIntervalMs, this::actionHandler)
      }
    }
  }
}
