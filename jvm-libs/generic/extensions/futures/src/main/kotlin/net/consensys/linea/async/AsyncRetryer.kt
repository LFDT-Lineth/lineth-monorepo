package net.consensys.linea.async

import io.vertx.core.Vertx
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.time.Instant
import java.util.function.Consumer
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds

class RetriedExecutionException(override val message: String) : RuntimeException(message)

fun <T> alwaysTruePredicate(@Suppress("UNUSED_PARAMETER") value: T) = true
fun <T> alwaysFalsePredicate(@Suppress("UNUSED_PARAMETER") value: T) = false

interface AsyncRetryer<T> {
  fun retry(action: () -> SafeFuture<T>): SafeFuture<T>
  fun retry(
    stopRetriesPredicate: (T) -> Boolean = ::alwaysTruePredicate,
    stopRetriesOnErrorPredicate: (Throwable) -> Boolean = ::alwaysFalsePredicate,
    exceptionConsumer: Consumer<Throwable>? = null,
    ignoreFirstExceptionsUntilTimeElapsed: Duration? = null,
    action: () -> SafeFuture<T>,
  ): SafeFuture<T>

  companion object {
    fun <T> retryer(
      vertx: Vertx,
      backoffDelay: Duration,
      maxRetries: Int? = null,
      timeout: Duration? = null,
      initialDelay: Duration? = null,
    ): AsyncRetryer<T> {
      return SequentialAsyncRetryerFactory(
        vertx = vertx,
        backoffDelay = backoffDelay,
        maxRetries = maxRetries,
        initialDelay = initialDelay,
        timeout = timeout,
      )
    }

    fun <T> retry(
      vertx: Vertx,
      backoffDelay: Duration,
      maxRetries: Int? = null,
      timeout: Duration? = null,
      initialDelay: Duration? = null,
      stopRetriesPredicate: (T) -> Boolean = ::alwaysTruePredicate,
      stopRetriesOnErrorPredicate: (Throwable) -> Boolean = ::alwaysFalsePredicate,
      exceptionConsumer: Consumer<Throwable>? = null,
      ignoreFirstExceptionsUntilTimeElapsed: Duration? = null,
      action: () -> SafeFuture<T>,
    ): SafeFuture<T> {
      return SequentialAsyncRetryerFactory<T>(
        vertx = vertx,
        backoffDelay = backoffDelay,
        maxRetries = maxRetries,
        timeout = timeout,
        initialDelay = initialDelay,
      ).retry(
        stopRetriesPredicate,
        stopRetriesOnErrorPredicate,
        exceptionConsumer,
        ignoreFirstExceptionsUntilTimeElapsed,
        action,
      )
    }
  }
}

/**
 * Retries an action until it succeeds or the maxRetries is reached or timeout has elapsed, whichever comes first.
 */
internal class SequentialAsyncActionRetryer<T>(
  val vertx: Vertx,
  val backoffDelay: Duration,
  val maxRetries: Int?,
  val timeout: Duration?,
  val initialDelay: Duration?,
  val stopRetriesPredicate: (T) -> Boolean = ::alwaysTruePredicate,
  val stopRetriesOnErrorPredicate: (Throwable) -> Boolean = ::alwaysFalsePredicate,
  val exceptionConsumer: Consumer<Throwable>? = null,
  val ignoreFirstExceptionsUntilTimeElapsed: Duration? = null,
  val action: () -> SafeFuture<T>,
) {
  init {
    require(backoffDelay >= 1.milliseconds) { "backoffDelay must be >= 1ms. value=$backoffDelay" }
    maxRetries?.also {
      require(maxRetries > 0) { "maxRetries must be greater than zero. value=$maxRetries" }
    }
    timeout?.also {
      require(timeout >= 1.milliseconds) { "timeout must be >= 1ms. value=$timeout" }
    }
    initialDelay?.also {
      require(initialDelay >= 1.milliseconds) { "initialDelay must be >= 1ms. value=$initialDelay" }
    }
    ignoreFirstExceptionsUntilTimeElapsed?.also {
      require(ignoreFirstExceptionsUntilTimeElapsed >= 1.milliseconds) {
        "ignoreFirstExceptionsUntilTimeElapsed must be >= 1ms. value=$ignoreFirstExceptionsUntilTimeElapsed"
      }
    }
  }

  private val resultFuture = SafeFuture<T>()
  private var remainingRetries: Int? = maxRetries
  private var startTime: Instant = Instant.now()
  private var remainingTime: Long? = timeout?.inWholeMilliseconds

  fun retry(): SafeFuture<T> {
    if (initialDelay != null && initialDelay > 0.milliseconds) {
      vertx.setTimer(initialDelay.inWholeMilliseconds) {
        startTime = Instant.now()
        retryLoop()
      }
    } else {
      startTime = Instant.now()
      retryLoop()
    }

    return resultFuture
  }

  private fun retryLoop() {
    val actionFuture = try {
      action()
    } catch (actionError: Throwable) {
      SafeFuture.failedFuture(actionError)
    }

    actionFuture.handle { result, throwable ->
      val timeElapsedSinceStarted = (Instant.now().toEpochMilli() - startTime.toEpochMilli())
      var errorThrowable = throwable
      remainingTime =
        timeout?.let { it.inWholeMilliseconds - timeElapsedSinceStarted }
      val stopRetrying =
        if (errorThrowable != null) {
          stopRetriesOnErrorPredicate(throwable)
        } else {
          try {
            stopRetriesPredicate(result)
          } catch (predicateError: Throwable) {
            errorThrowable = predicateError
            false
          }
        }

      if (errorThrowable != null && timeElapsedSinceStarted >
        (ignoreFirstExceptionsUntilTimeElapsed?.inWholeMilliseconds ?: -1L)
      ) {
        exceptionConsumer?.runCatching { exceptionConsumer.accept(errorThrowable) }
      }

      if (stopRetrying) {
        if (errorThrowable != null) {
          resultFuture.completeExceptionally(errorThrowable)
        } else {
          resultFuture.complete(result)
        }
      } else {
        val hasMoreRetries = remainingRetries == null || remainingRetries!! > 0
        val hasMoreTime = remainingTime == null || remainingTime!! > 0
        if (hasMoreRetries && hasMoreTime) {
          vertx.setTimer(backoffDelay.inWholeMilliseconds) {
            if (remainingRetries != null) {
              remainingRetries = remainingRetries!! - 1
            }
            retryLoop()
          }
        } else {
          if (errorThrowable != null) {
            resultFuture.completeExceptionally(errorThrowable)
          } else {
            val message = if (!hasMoreRetries) {
              "Stop condition wasn't met after $maxRetries retries."
            } else {
              "Stop condition wasn't met after timeout of $timeout."
            }
            resultFuture.completeExceptionally(RetriedExecutionException(message))
          }
        }
      }
    }
  }
}

private class SequentialAsyncRetryerFactory<T>(
  val vertx: Vertx,
  val backoffDelay: Duration,
  val maxRetries: Int? = null,
  val timeout: Duration? = null,
  val initialDelay: Duration? = null,
) : AsyncRetryer<T> {
  override fun retry(action: () -> SafeFuture<T>): SafeFuture<T> {
    return SequentialAsyncActionRetryer(
      vertx = vertx,
      backoffDelay = backoffDelay,
      maxRetries = maxRetries,
      timeout = timeout,
      initialDelay = initialDelay,
      stopRetriesPredicate = ::alwaysTruePredicate,
      exceptionConsumer = null,
      ignoreFirstExceptionsUntilTimeElapsed = null,
      action = action,
    ).retry()
  }

  override fun retry(
    stopRetriesPredicate: (T) -> Boolean,
    stopRetriesOnErrorPredicate: (Throwable) -> Boolean,
    exceptionConsumer: Consumer<Throwable>?,
    ignoreFirstExceptionsUntilTimeElapsed: Duration?,
    action: () -> SafeFuture<T>,
  ): SafeFuture<T> {
    return SequentialAsyncActionRetryer(
      vertx = vertx,
      maxRetries = maxRetries,
      backoffDelay = backoffDelay,
      initialDelay = initialDelay,
      timeout = timeout,
      stopRetriesPredicate = stopRetriesPredicate,
      stopRetriesOnErrorPredicate = stopRetriesOnErrorPredicate,
      exceptionConsumer = exceptionConsumer,
      ignoreFirstExceptionsUntilTimeElapsed = ignoreFirstExceptionsUntilTimeElapsed,
      action = action,
    ).retry()
  }
}
