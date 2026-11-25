package linea.timer

import io.vertx.core.Vertx
import io.vertx.junit5.VertxExtension
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
import org.mockito.Mockito.mock
import org.mockito.Mockito.times
import org.mockito.kotlin.any
import org.mockito.kotlin.atLeastOnce
import org.mockito.kotlin.eq
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

  // Upper bound for every wait in this class. It is also the failure bound of each test:
  // the tests run on the test thread, so a missed condition fails with the real assertion message.
  private val awaitTimeout = 10.seconds.toJavaDuration()

  // Waits on the returned future instead of assuming it is already completed.
  private fun <T> SafeFuture<T>.getWithTimeout(): T = get(awaitTimeout.toMillis(), TimeUnit.MILLISECONDS)

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `on error periodic polling service is passed to handleError`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
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

    pollingService.start().getWithTimeout()

    await()
      .atMost(awaitTimeout)
      .pollInterval(50.milliseconds.toJavaDuration())
      .untilAsserted {
        verify(log, atLeastOnce()).error(
          eq("Error polling: errorMessage={}"),
          eq("java.lang.IllegalStateException: Test error"),
        )
      }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `action handles a throw`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
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

    pollingService.start().getWithTimeout()

    await()
      .atMost(awaitTimeout)
      .pollInterval(50.milliseconds.toJavaDuration())
      .untilAsserted {
        verify(log, atLeastOnce()).error(
          eq("Error polling: errorMessage={}"),
          any<Any>(),
        )
      }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `periodicPollingService continues to poll after an exception`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
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

    pollingService.start().getWithTimeout()

    await()
      .atMost(awaitTimeout)
      .pollInterval(50.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(5)
        verify(log, times(2)).error(
          eq("Error polling: errorMessage={}"),
          eq("java.lang.IllegalStateException: Test error"),
        )
      }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `periodicPollingService continues to poll after throwing exception`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
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

    pollingService.start().getWithTimeout()

    await()
      .atMost(awaitTimeout)
      .pollInterval(50.milliseconds.toJavaDuration())
      .untilAsserted {
        assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(5)
      }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `ticks shouldn't run concurrently if execution is longer than polling interval`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
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

    pollingService.start().getWithTimeout()

    await()
      .timeout(2.seconds.toJavaDuration())
      .untilAsserted {
        assertThat(numberOfInvocations.get()).isGreaterThanOrEqualTo(3)
      }
    // assert invariant of not calling action in parallel
    assertThat(actionWasCalledInParallel.get()).isFalse()
  }

  fun asyncDelay(vertx: Vertx, delay: Duration): SafeFuture<Unit> {
    val future = SafeFuture<Unit>()
    vertx.setTimer(delay.inWholeMilliseconds) {
      future.complete(Unit)
    }
    return future
  }

  @Test
  fun `periodicPollingService start should be idempotent`() {
    val pollingInterval = 60.milliseconds
    val mockTimerFactory = mock<TimerFactory>()
    val mockTimer = mock<Timer>()
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
    ).thenReturn(mockTimer)

    pollingService.start().getWithTimeout()
    pollingService.start().getWithTimeout()

    // exactly one timer is created and started, no matter how many times start() is called
    verify(mockTimerFactory, times(1)).createTimer(any(), any(), any(), any(), any(), any())
    verify(mockTimer, times(1)).start()
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
  fun `periodicPollingService stop should be idempotent`(
    timerFactoryType: KClass<out TimerFactory>,
    timerSchedule: TimerSchedule,
    vertx: Vertx,
  ) {
    val actionCallCount = AtomicInteger(0)
    val pollingService =
      PollingService(
        pollingInterval,
        log,
        mockAction = {
          actionCallCount.incrementAndGet()
          SafeFuture.completedFuture(Unit)
        },
        timerFactory = createTimerFactory(timerFactoryType, vertx),
        timerSchedule = timerSchedule,
      )

    pollingService.start().getWithTimeout()
    // make sure stop() hits a service that is really polling
    await()
      .atMost(awaitTimeout)
      .untilAsserted {
        assertThat(actionCallCount.get()).isGreaterThanOrEqualTo(1)
      }

    // stop() completes once the in-flight action (if any) has finished,
    // so no action can be running or start after this point
    pollingService.stop().getWithTimeout()
    val callCountAfterStop = actionCallCount.get()

    // stopping an already stopped service is a no-op that must complete normally
    pollingService.stop().getWithTimeout()
    pollingService.stop().getWithTimeout()

    // the timer stays stopped: for several polling intervals no further action runs
    await()
      .atMost(awaitTimeout)
      .during((pollingInterval * 6).toJavaDuration())
      .untilAsserted {
        assertThat(actionCallCount.get()).isEqualTo(callCountAfterStop)
      }
  }

  @ParameterizedTest
  @MethodSource("timerFactories")
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
