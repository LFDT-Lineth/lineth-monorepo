package net.consensys.zkevm

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import io.vertx.junit5.VertxExtension
import io.vertx.junit5.VertxTestContext
import org.apache.logging.log4j.LogManager
import org.apache.logging.log4j.Logger
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility.await
import org.junit.jupiter.api.BeforeEach
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.extension.ExtendWith
import org.mockito.Mockito
import org.mockito.Mockito.atMostOnce
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.eq
import org.mockito.kotlin.mockingDetails
import org.mockito.kotlin.spy
import org.mockito.kotlin.verify
import org.mockito.kotlin.whenever
import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicBoolean
import java.util.concurrent.atomic.AtomicInteger
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class PeriodicPollingServiceTest {
  private lateinit var log: Logger
  private val pollingInterval = 50.milliseconds.inWholeMilliseconds

  @BeforeEach
  fun beforeEach() {
    log = Mockito.spy(LogManager.getLogger(PollingService::class.java))
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `on error periodic polling service is passed to handleError`(vertx: Vertx, testContext: VertxTestContext) {
    val action = { _: Unit ->
      SafeFuture.failedFuture<Unit>(IllegalStateException("Test error"))
    }
    val pollingService = PollingService(vertx, pollingInterval, log, mockAction = action)

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(log, atLeastOnce()).error(
              eq("Error polling: errorMessage={}"),
              eq("java.lang.IllegalStateException: Test error"),
            )
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `action handles a throw`(vertx: Vertx, testContext: VertxTestContext) {
    val action = { _: Unit ->
      throw IllegalStateException("Throw test")
    }
    val pollingService = PollingService(vertx, pollingInterval, log, mockAction = action)

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(log, atLeastOnce()).error(
              eq("Error polling: errorMessage={}"),
              eq("java.lang.IllegalStateException: Throw test"),
            )
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService continues to poll after an exception`(vertx: Vertx, testContext: VertxTestContext) {
    val actionCallCount = AtomicInteger(0)

    val action = { _: Unit ->
      actionCallCount.incrementAndGet()
      if (actionCallCount.get() < 3) {
        SafeFuture.failedFuture<Unit>(IllegalStateException("Test error"))
      } else {
        val result = SafeFuture<Unit>()
        asyncDelay(vertx, 5.milliseconds).thenApply {
          result.complete(Unit)
        }
        result
      }
    }
    val pollingService = PollingService(vertx, pollingInterval, log, mockAction = action)

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(5)
            verify(log, times(2)).error(
              eq("Error polling: errorMessage={}"),
              eq("java.lang.IllegalStateException: Test error"),
            )
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService continues to poll after throwing exception`(vertx: Vertx, testContext: VertxTestContext) {
    val actionCallCount = AtomicInteger(0)

    val action = { _: Unit ->
      actionCallCount.incrementAndGet()
      if (actionCallCount.get() < 3) {
        throw IllegalStateException("Throw test")
      } else {
        val result = SafeFuture<Unit>()
        asyncDelay(vertx, 5.milliseconds).thenApply {
          result.complete(Unit)
        }
        result
      }
    }
    val pollingService = PollingService(vertx, pollingInterval, log, mockAction = action)

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(5)
            verify(log, times(2)).error(
              eq("Error polling: errorMessage={}"),
              eq("java.lang.IllegalStateException: Throw test"),
            )
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `ticks shouldn't run concurrently if execution is longer than polling interval`(
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val pollingInterval = 5.milliseconds.inWholeMilliseconds
    val numberOfInvocations = AtomicInteger(0)
    val prevActionIsRunning = AtomicBoolean(false)
    val actionWasCalledInParallel = AtomicBoolean(false)
    val action = { _: Unit ->
      numberOfInvocations.incrementAndGet()
      if (prevActionIsRunning.get()) {
        actionWasCalledInParallel.set(true)
      }
      prevActionIsRunning.set(true)
      asyncDelay(vertx, 10.milliseconds).thenApply { prevActionIsRunning.set(false) }
    }
    val pollingService = PollingService(vertx, pollingInterval, log, mockAction = action)
    pollingService.start()
      .thenApply {
        await()
          .timeout(2.seconds.toJavaDuration())
          .untilAsserted {
            assertThat(numberOfInvocations.get()).isGreaterThanOrEqualTo(3)
            // assert invariant of not calling action in parallel
            assertThat(actionWasCalledInParallel.get()).isFalse()
            testContext.completeNow()
          }
      }.whenException(testContext::failNow)
  }

  fun asyncDelay(vertx: Vertx, delay: Duration): SafeFuture<Unit> {
    val future = SafeFuture<Unit>()
    vertx.setTimer(delay.inWholeMilliseconds) {
      future.complete(Unit)
    }
    return future
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService start should be idempotent`(
    testContext: VertxTestContext,
  ) {
    val pollingInterval = 60.milliseconds.inWholeMilliseconds
    val mockVertx = mock<Vertx>()
    val pollingService = spy(PollingService(mockVertx, pollingInterval, log))

    whenever(mockVertx.setTimer(any(), any())).thenReturn(1)

    pollingService.start().thenApply {
      pollingService.start()
    }
      .thenApply {
        await()
          .pollInterval(12.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(mockVertx, atMostOnce()).setTimer(any(), any())
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @Test
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService stop should be idempotent`(
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val log: Logger = Mockito.spy(LogManager.getLogger(PollingService::class.java))
    val pollingService = PollingService(vertx, pollingInterval, log)

    pollingService.start()
    await()
      .untilAsserted {
        pollingService.stop()
          .thenCompose {
            val numberOfInvocations = mockingDetails(log).invocations
            pollingService.stop()
              .thenApply { pollingService.stop() }
              .thenApply {
                asyncDelay(vertx, 50.milliseconds).thenApply {
                  await()
                    .untilAsserted {
                      verify(log, times(numberOfInvocations.size))
                    }
                  testContext.completeNow()
                }
              }
          }.whenException(testContext::failNow)
      }
  }
}

class PollingService(
  private val vertx: Vertx,
  pollingInterval: Long,
  private val log: Logger,
  val mockAction: (_: Unit) -> SafeFuture<Unit> = { SafeFuture.completedFuture(Unit) },
) : PeriodicPollingService(
  vertx = vertx,
  pollingIntervalMs = pollingInterval,
  log = log,
) {
  override fun action(): SafeFuture<Unit> {
    val future = SafeFuture<Unit>()
    vertx.setTimer(1) { // just to make it async
      future.complete(Unit)
    }
    return future.thenCompose(mockAction)
  }

  override fun handleError(error: Throwable) {
    log.error("Error polling: errorMessage={}", error.message)
  }
}
