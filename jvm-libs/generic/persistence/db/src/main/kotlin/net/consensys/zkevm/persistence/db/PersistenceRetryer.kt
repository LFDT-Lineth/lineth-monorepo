package net.consensys.zkevm.persistence.db

import io.vertx.core.Vertx
import net.consensys.linea.async.AsyncRetryer
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import tech.pegasys.teku.infrastructure.async.SafeFuture
import kotlin.time.Duration

open class PersistenceRetryer(
  private val vertx: Vertx,
  private val config: Config,
  private val log: Logger = LogManager.getLogger(PersistenceRetryer::class.java),
) {
  data class Config(
    val backoffDelay: Duration,
    val maxRetries: Int? = null,
    val timeout: Duration? = null,
    val ignoreFirstExceptionsUntilTimeElapsed: Duration? = null,
  )

  fun <T> retryQuery(
    action: () -> SafeFuture<T>,
    stopRetriesOnErrorPredicate: (Throwable) -> Boolean = Companion::stopRetriesOnErrorPredicate,
    exceptionConsumer: (Throwable) -> Unit = { error ->
      when {
        isDuplicateKeyException(error) -> log.warn(
          "Persistence errorMessage={}",
          error.message,
        )
        else -> {
          log.warn(
            "Persistence errorMessage={}, it will retry again in {}",
            error.message,
            config.backoffDelay,
            error,
          )
        }
      }
    },
  ): SafeFuture<T> {
    return AsyncRetryer.retry(
      vertx = vertx,
      timeout = config.timeout,
      backoffDelay = config.backoffDelay,
      maxRetries = config.maxRetries,
      stopRetriesOnErrorPredicate = stopRetriesOnErrorPredicate,
      exceptionConsumer = exceptionConsumer,
      ignoreFirstExceptionsUntilTimeElapsed = config.ignoreFirstExceptionsUntilTimeElapsed,
      action = action,
    )
  }

  companion object {
    /**
     * Predicate with common errors that are not recoverable.
     * Populate this list as we find more errors that should not be retried.
     */
    fun stopRetriesOnErrorPredicate(th: Throwable): Boolean {
      return when {
        isDuplicateKeyException(th) -> true
        else -> false
      }
    }
  }
}
