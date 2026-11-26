package linea.timer

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
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
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
import kotlin.reflect.KClass
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration

@ExtendWith(VertxExtension::class)
class GenericPeriodicPollingServiceTest {
  private lateinit var log: Logger
  private val pollingInterval = 50.milliseconds

  companion object {
    @JvmStatic
    fun timerFactories() = listOf(
      Arguments.of(VertxTimerFactory::class, TimerSchedule.FIXED_RATE),
      Arguments.of(VertxTimerFactory::class, TimerSchedule.FIXED_DELAY),
      Arguments.of(JvmTimerFactory::class, TimerSchedule.FIXED_RATE),
      Arguments.of(JvmTimerFactory::class, TimerSchedule.FIXED_DELAY),
    )
  }

  @BeforeEach
  fun beforeEach() {
    log = Mockito.spy(LogManager.getLogger(PollingService::class.java))
  }

  fun createTimerFactory(timerFactoryType: KClass<out TimerFactory>, vertx: Vertx): TimerFactory {
    return when (timerFactoryType) {
      VertxTimerFactory::class -> VertxTimerFactory(vertx)
      JvmTimerFactory::class -> JvmTimerFactory()
      else -> throw IllegalArgumentException("Unknown timer factory type")
    }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `on error periodic polling service is passed to handleError`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val action = { _: Unit ->
      SafeFuture.failedFuture<Unit>(IllegalStateException("Test error"))
    }
    val pollingService = PollingService(
      pollingInterval,
      log,
      mockAction = action,
      timerFactory = createTimerFactory(timerFactoryType, vertx),
      timerSchedule = timerSchedule,
    )

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

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `action handles a throw`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val action = { _: Unit ->
      throw IllegalStateException("Throw test")
    }
    val pollingService = PollingService(
      pollingInterval,
      log,
      mockAction = action,
      timerFactory = createTimerFactory(timerFactoryType, vertx),
      timerSchedule = timerSchedule,
    )

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(log, atLeastOnce()).error(
              eq("Error polling: errorMessage={}"),
              any<Any>(),
            )
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService continues to poll after an exception`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val actionCallCount = AtomicInteger(0)

    val action = { _: Unit ->
      actionCallCount.incrementAndGet()
      if (actionCallCount.get() < 3) {
        SafeFuture.failedFuture<Unit>(IllegalStateException("Test error"))
      } else {
        asyncDelay(vertx, 5.milliseconds)
      }
    }
    val pollingService = PollingService(
      pollingInterval,
      log,
      mockAction = action,
      timerFactory = createTimerFactory(timerFactoryType, vertx),
      timerSchedule = timerSchedule,
    )

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

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService continues to poll after throwing exception`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val actionCallCount = AtomicInteger(0)

    val action = { _: Unit ->
      actionCallCount.incrementAndGet()
      if (actionCallCount.get() < 3) {
        throw IllegalStateException("Throw test")
      } else {
        asyncDelay(vertx, 5.milliseconds)
      }
    }
    val pollingService = PollingService(
      pollingInterval,
      log,
      mockAction = action,
      timerFactory = createTimerFactory(timerFactoryType, vertx),
      timerSchedule = timerSchedule,
    )

    pollingService.start()
      .thenApply {
        await()
          .pollInterval(50.milliseconds.toJavaDuration())
          .untilAsserted {
            assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(5)
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `ticks shouldn't run concurrently if execution is longer than polling interval`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val pollingInterval = 5.milliseconds
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
    val pollingService = PollingService(
      pollingInterval,
      log,
      mockAction = action,
      timerFactory = createTimerFactory(timerFactoryType, vertx),
      timerSchedule = timerSchedule,
    )
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
  fun `periodicPollingService start should be idempotent`(testContext: VertxTestContext) {
    val pollingInterval = 60.milliseconds
    val mockTimerFactory = mock<TimerFactory>()
    val pollingService = spy(
      PollingService(
        pollingInterval,
        log,
        timerFactory = mockTimerFactory,
        timerSchedule = mock<TimerSchedule>(),
      ),
    )

    whenever(
      mockTimerFactory.createTimer(
        any(),
        any(),
        any(),
        any(),
        any(),
        any(),
      ),
    ).thenReturn(mock<Timer>())

    pollingService.start().thenApply {
      pollingService.start()
    }
      .thenApply {
        await()
          .pollInterval(12.milliseconds.toJavaDuration())
          .untilAsserted {
            verify(mockTimerFactory, atMostOnce()).createTimer(any(), any(), any(), any(), any(), any())
          }
        testContext.completeNow()
      }.whenException(testContext::failNow)
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `periodicPollingService stop should be idempotent`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
    testContext: VertxTestContext,
  ) {
    val log: Logger = Mockito.spy(LogManager.getLogger(PollingService::class.java))
    val pollingService =
      PollingService(
        pollingInterval,
        log,
        timerFactory = createTimerFactory(timerFactoryType, vertx),
        timerSchedule = timerSchedule,
      )

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

  @ParameterizedTest
  @MethodSource("timerFactories")
  @Timeout(3, timeUnit = TimeUnit.SECONDS)
  fun `should not allow action of one timer affect or delay another timer`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
  ) {
    val timerFactory = createTimerFactory(timerFactoryType, vertx)
    val poller1Calls = AtomicInteger(0)
    val poller2Calls = AtomicInteger(0)
    // 10x faster than poller 2
    val poller1 = PollingService(
      pollingInterval = 10.milliseconds,
      log,
      timerFactory = timerFactory,
      timerSchedule = timerSchedule,
    ) {
      Thread.sleep(5L)
      poller1Calls.incrementAndGet()
      SafeFuture.completedFuture(Unit)
    }

    val poller2 = PollingService(
      pollingInterval = 100.milliseconds,
      log,
      timerFactory = timerFactory,
      timerSchedule = timerSchedule,
    ) {
      Thread.sleep(500L)
      poller2Calls.incrementAndGet()
      SafeFuture.completedFuture(Unit)
    }

    poller2.start()
    poller1.start()

    await()
      .atMost(20.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(poller1Calls.get()).isGreaterThanOrEqualTo(1)
        assertThat(poller2Calls.get()).isGreaterThanOrEqualTo(1)
        assertThat(poller1Calls.get()).isGreaterThanOrEqualTo(poller2Calls.get() * 8)
      }
  }
}

class PollingService(
  pollingInterval: Duration,
  private val log: Logger,
  timerFactory: TimerFactory,
  timerSchedule: TimerSchedule,
  val mockAction: (_: Unit) -> SafeFuture<Unit> = { SafeFuture.completedFuture(Unit) },
) : PeriodicPollingService(
  timerFactory = timerFactory,
  pollingInterval = pollingInterval,
  log = log,
  timerSchedule = timerSchedule,
  name = "PollingService",
) {
  override fun action(): SafeFuture<Unit> = mockAction(Unit)

  override fun handleError(error: Throwable) {
    log.error("Error polling: errorMessage={}", error.message)
  }
}
