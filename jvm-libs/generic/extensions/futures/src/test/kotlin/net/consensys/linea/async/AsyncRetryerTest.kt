package net.consensys.linea.async

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.assertThrows
import org.junit.jupiter.api.extension.ExtendWith
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.ExecutionException
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.nanoseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class AsyncRetryerTest {
  @Test
  fun `Retryer should throw when maxRetries is less than zero`(vertx: Vertx) {
    var callCount = 0
    val exception = assertThrows<IllegalArgumentException> {
      SequentialAsyncActionRetryer(
        vertx,
        backoffDelay = 10.milliseconds,
        maxRetries = -1,
        initialDelay = 5.milliseconds,
        timeout = 2.seconds,
        stopRetriesPredicate = { result -> result == "20" }
      ) {
        callCount++
        SafeFuture.completedFuture("true")
      }
    }
    assertThat(exception.message).isEqualTo("maxRetries must be greater than zero. value=-1")
    assertThat(callCount).isEqualTo(0)
  }

  @Test
  fun `Retryer should throw when timeout is zero`(vertx: Vertx) {
    var callCount = 0
    val exception = assertThrows<IllegalArgumentException> {
      SequentialAsyncActionRetryer(
        vertx,
        backoffDelay = 10.milliseconds,
        maxRetries = 3,
        initialDelay = 5.milliseconds,
        timeout = 0.seconds,
        stopRetriesPredicate = { result -> result == "20" }
      ) {
        callCount++
        SafeFuture.completedFuture("true")
      }
    }
    assertThat(exception.message).isEqualTo("timeout must be >= 1ms. value=0s")
    assertThat(callCount).isEqualTo(0)
  }

  @Test
  fun `Retryer should retry endlessly until predicate is met when both timeout and maxRetries are null`(
    vertx: Vertx
  ) {
    val callCount = AtomicInteger(0)
    val expectedResult = "6"
    val result =
      AsyncRetryer.retry(
        vertx = vertx,
        backoffDelay = 5.milliseconds,
        stopRetriesPredicate = { result -> result == expectedResult }
      ) {
        SafeFuture.completedFuture("${callCount.incrementAndGet()}")
      }.get()

    assertThat(callCount.get()).isEqualTo(6)
    assertThat(result).isEqualTo(expectedResult)
  }

  @Test
  fun `Retryer should retry endlessly until stopRetriesOnErrorPredicate returns true`(
    vertx: Vertx
  ) {
    val callCount = AtomicInteger(0)

    val future = AsyncRetryer.retry(
      vertx = vertx,
      backoffDelay = 5.milliseconds,
      stopRetriesOnErrorPredicate = { error -> error.message == "stop now" }
    ) {
      if (callCount.incrementAndGet() < 3) {
        SafeFuture.failedFuture<String>(RuntimeException("${callCount.get()}"))
      } else {
        SafeFuture.failedFuture(RuntimeException("stop now"))
      }
    }
    await()
      .timeout(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(future).isCompletedExceptionally
      }

    runCatching { future.get() }
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(RuntimeException::class.java)
      .withMessageContaining("stop now")

    assertThat(callCount.get()).isEqualTo(3)
  }

  @Test
  fun `Retryer should retry endlessly if predicate is never met when both timeout and maxRetries are null`(
    vertx: Vertx,
    testContext: VertxTestContext
  ) {
    val callCount = AtomicInteger(0)
    val everPendingFuture =
      AsyncRetryer.retry(vertx = vertx, backoffDelay = 5.milliseconds, stopRetriesPredicate = { false }) {
        SafeFuture.completedFuture("${callCount.incrementAndGet()}")
      }

    await()
      .timeout(5.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(everPendingFuture).isNotDone
        assertThat(callCount).hasValueGreaterThan(5)
      }
    testContext.completeNow()
  }

  @Test
  fun `Retryer should throw when backoffDelay is less than 1 millisecond`(vertx: Vertx) {
    var callCount = 0
    val exception = assertThrows<IllegalArgumentException> {
      SequentialAsyncActionRetryer(
        vertx,
        backoffDelay = 50.nanoseconds,
        maxRetries = 3,
        initialDelay = 5.milliseconds,
        timeout = 2.seconds,
        stopRetriesPredicate = { result -> result == "20" }
      ) {
        callCount++
        SafeFuture.completedFuture("true")
      }
    }
    assertThat(exception.message).isEqualTo("backoffDelay must be >= 1ms. value=50ns")
    assertThat(callCount).isEqualTo(0)
  }

  @Test
  fun `Retryer should throw when initialDelay is less than 1 millisecond`(vertx: Vertx) {
    var callCount = 0
    val exception = assertThrows<IllegalArgumentException> {
      SequentialAsyncActionRetryer(
        vertx,
        backoffDelay = 2.milliseconds,
        maxRetries = 3,
        initialDelay = 50.nanoseconds,
        timeout = 2.seconds,
        stopRetriesPredicate = { result -> result == "20" }
      ) {
        callCount++
        SafeFuture.completedFuture("true")
      }
    }
    assertThat(exception.message).isEqualTo("initialDelay must be >= 1ms. value=50ns")
    assertThat(callCount).isEqualTo(0)
  }

  @Test
  fun `retry should resolve as soon as predicate is satisfied`(vertx: Vertx) {
    var callCount = AtomicInteger(0)
    val result =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 4, stopRetriesPredicate = { result -> result == "2" }) {
        SafeFuture.completedFuture("${callCount.incrementAndGet()}")
      }
        .get()

    assertThat(callCount.get()).isEqualTo(2)
    assertThat(result).isEqualTo("2")
  }

  @Test
  fun `retry should work with promises that return null values`(vertx: Vertx) {
    var callCount = 0
    val result =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 2) {
        callCount++
        SafeFuture.completedFuture<Unit>(null)
      }
        .get()

    assertThat(callCount).isEqualTo(1)
    assertThat(result).isNull()
  }

  @Test
  fun `retry should stop after max retries are exhausted - promise rejected`(vertx: Vertx) {
    var callCount = AtomicInteger(0)
    val future =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 4) {
        SafeFuture.failedFuture<String>(RuntimeException("Failure number ${callCount.incrementAndGet()}"))
      }
    runCatching { future.get() }
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(20.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(RuntimeException::class.java)
      .withMessageContaining("Failure number 5")

    assertThat(callCount.get()).isEqualTo(5)
  }

  @Test
  fun `retry should stop after max retries are exhausted - promise resolves, predicate evaluation`(vertx: Vertx) {
    var callCount = AtomicInteger(0)
    val future =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 4, stopRetriesPredicate = { result -> result == "20" }) {
        SafeFuture.completedFuture("${callCount.incrementAndGet()}")
      }
    runCatching { future.get() }
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(RetriedExecutionException::class.java)
      .withMessageContaining("Stop condition wasn't met after 4 retries")

    assertThat(callCount.get()).isEqualTo(5)
  }

  @Test
  fun `retry should stop after timeout is elapsed - promise rejected`(vertx: Vertx) {
    var callCount = AtomicInteger(0)
    val future =
      AsyncRetryer.retry(vertx, 20.milliseconds, timeout = 40.milliseconds) {
        SafeFuture.failedFuture<String>(RuntimeException("Failure number ${callCount.incrementAndGet()}"))
      }
    runCatching { future.get() }
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(RuntimeException::class.java)
      .withMessageContaining("Failure number")

    assertThat(callCount.get())
      .isGreaterThan(1)
      .isLessThanOrEqualTo(3)
  }

  @Test
  fun `retry should stop after timeout is elapsed - promise resolves, predicate evaluation`(vertx: Vertx) {
    val callCount = AtomicInteger(0)
    val future =
      AsyncRetryer.retry(
        vertx,
        backoffDelay = 5.milliseconds,
        timeout = 60.milliseconds,
        stopRetriesPredicate = { false } // stop condition will never be met
      ) {
        SafeFuture.completedFuture("${callCount.incrementAndGet()}")
      }
    assertThat(future)
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(RetriedExecutionException::class.java)
      .withMessageContaining("Stop condition wasn't met after timeout of 60ms")

    // 60ms / 5ms = 12 retries
    assertThat(callCount.get()).isBetween(1, 13)
  }

  @Test
  fun `retry should retry after backoffDelay and initialDelay`(vertx: Vertx) {
    var callCount = 0
    val delays = mutableListOf<Long>()
    val startTime = System.currentTimeMillis()
    var lastReferenceTime = startTime
    val result =
      AsyncRetryer.retry(vertx, maxRetries = 3, initialDelay = 100.milliseconds, backoffDelay = 300.milliseconds) {
        callCount++
        val now = System.currentTimeMillis()
        delays.add(now - lastReferenceTime)
        lastReferenceTime = now
        SafeFuture.failedFuture<Unit>(Exception("Upss"))
      }
    assertThrows<ExecutionException> { result.get() }
    assertThat(delays.first()).isGreaterThan(100)
    assertThat(delays.subList(1, delays.size)).allMatch { it >= 300 }
    assertThat(callCount).isEqualTo(4)
  }

  @Test
  fun `retry should retry if predicate throws exception and resolve when condition is met`(vertx: Vertx) {
    val callCount = AtomicInteger(0)
    val predicate = { actionReturnedValue: Int ->
      when {
        actionReturnedValue < 3 -> throw Exception("Error $actionReturnedValue")
        else -> true
      }
    }
    val result =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 10, stopRetriesPredicate = predicate) {
        SafeFuture.completedFuture(callCount.incrementAndGet())
      }
        .get()

    assertThat(callCount.get()).isEqualTo(3)
    assertThat(result).isEqualTo(3)
  }

  @Test
  fun `retry should retry if predicate throws exception and reject after maxRetries are reached`(vertx: Vertx) {
    val callCount = AtomicInteger(0)
    val predicate = { actionReturnedValue: Int -> throw IndexOutOfBoundsException("Error $actionReturnedValue") }
    val future =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 4, stopRetriesPredicate = predicate) {
        SafeFuture.completedFuture(callCount.incrementAndGet())
      }
    runCatching { future.get() }

    assertThat(callCount.get()).isEqualTo(5)
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(IndexOutOfBoundsException::class.java)
      .withMessageContaining("Error 5")
  }

  @Test
  fun `retry should retry if action throws exception and resolve when condition is met`(vertx: Vertx) {
    val callCount = AtomicInteger(0)
    val predicate = { actionReturnedValue: Int ->
      when {
        actionReturnedValue < 5 -> false
        else -> true
      }
    }
    val result =
      AsyncRetryer.retry(vertx, 20.milliseconds, maxRetries = 10, stopRetriesPredicate = predicate) {
        callCount.incrementAndGet()
        when {
          callCount.get() < 3 -> throw Exception("Error ${callCount.get()}")
          else -> SafeFuture.completedFuture(callCount.get())
        }
      }
        .get()

    assertThat(callCount.get()).isEqualTo(5)
    assertThat(result).isEqualTo(5)
  }

  @Test
  fun `retry should retry if action throws exception and reject after maxRetries are reached`(vertx: Vertx) {
    val callCount = AtomicInteger(0)
    val future =
      AsyncRetryer.retry<Int>(vertx, backoffDelay = 20.milliseconds, maxRetries = 3) {
        throw IndexOutOfBoundsException("Error ${callCount.incrementAndGet()}")
      }

    runCatching { future.get() }

    assertThat(callCount.get()).isEqualTo(4)
    assertThat(future)
      .isCompletedExceptionally
      .isNotCancelled
      .failsWithin(2.seconds.toJavaDuration())
      .withThrowableOfType(ExecutionException::class.java)
      .withCauseInstanceOf(IndexOutOfBoundsException::class.java)
      .withMessageContaining("Error 4")
  }

  @Test
  fun `retry should not cache responses`(vertx: Vertx) {
    val retryer = AsyncRetryer.retryer<Int>(vertx, backoffDelay = 20.milliseconds, maxRetries = 3)

    fun <T> delyedResult(delay: Duration, result: T): SafeFuture<T> {
      val future = SafeFuture<T>()
      vertx.setTimer(delay.inWholeMilliseconds) {
        future.complete(result)
      }
      return future
    }

    val future1 = retryer.retry { delyedResult(10.milliseconds, 1) }
    val future2 = retryer.retry { delyedResult(5.milliseconds, 2) }
    val future3 = retryer.retry { delyedResult(2.milliseconds, 3) }
    assertThat(future3.get()).isEqualTo(3)
    assertThat(future2.get()).isEqualTo(2)
    assertThat(future1.get()).isEqualTo(1)
  }
}
