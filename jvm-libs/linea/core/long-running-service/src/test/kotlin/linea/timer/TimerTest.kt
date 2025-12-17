package linea.timer

import io.vertx.core.Vertx
import io.vertx.junit5.Timeout
import org.assertj.core.api.Assertions
import org.assertj.core.api.Assertions.assertThat
import org.awaitility.Awaitility
import org.junit.jupiter.api.Test
import org.junit.jupiter.params.ParameterizedTest
import org.junit.jupiter.params.provider.Arguments
import org.junit.jupiter.params.provider.MethodSource
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.CountDownLatch
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.atomics.AtomicBoolean
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.random.Random
import kotlin.reflect.KClass
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.ExperimentalTime
import kotlin.time.Instant
import kotlin.time.toJavaDuration

@OptIn(ExperimentalAtomicApi::class)
class TimerTest {
  companion object {
    @JvmStatic
    fun timerTypes() = listOf(
      Arguments.of(JvmTimer::class, TimerSchedule.FIXED_DELAY),
      Arguments.of(JvmTimer::class, TimerSchedule.FIXED_RATE),
      Arguments.of(VertxTimer::class, TimerSchedule.FIXED_DELAY),
      Arguments.of(VertxTimer::class, TimerSchedule.FIXED_RATE),
    )

    fun timerReference(timer: Timer): Any? {
      return when (timer) {
        is VertxTimer -> timer.timerIdReference()
        is JvmTimer -> timer.timerReference()
        else -> throw IllegalArgumentException("Unknown timer type")
      }
    }
  }

  private val vertx = Vertx.vertx()

  fun createTimer(
    timerType: KClass<out Timer>,
    timerSchedule: TimerSchedule,
    task: Runnable,
    initialDelay: Duration = 50.milliseconds,
    period: Duration = 50.milliseconds,
    errorHandler: (Throwable) -> Unit,
  ): Timer {
    return when (timerType) {
      JvmTimer::class -> JvmTimer(
        name = "test-jvm-timer-${Random.Default.nextInt()}",
        task = task,
        initialDelay = initialDelay,
        period = period,
        errorHandler = errorHandler,
        timerSchedule = timerSchedule,
      )
      VertxTimer::class -> VertxTimer(
        vertx = vertx,
        name = "test-vertx-timer-${Random.Default.nextInt()}",
        task = task,
        initialDelay = initialDelay,
        period = period,
        errorHandler = errorHandler,
        timerSchedule = timerSchedule,
      )
      else -> throw IllegalArgumentException("Unknown timer type: $timerType")
    }
  }

  @ParameterizedTest
  @MethodSource("timerTypes")
  @Timeout(2, timeUnit = TimeUnit.SECONDS)
  fun `task exception is handled by error handler`(timerType: KClass<out Timer>, timerSchedule: TimerSchedule) {
    val errorHandled = AtomicBoolean(false)
    val error = AtomicReference<String>("")
    val timer = createTimer(
      timerType = timerType,
      timerSchedule = timerSchedule,
      task = Runnable { throw RuntimeException("Test error") },
      errorHandler = { t ->
        errorHandled.store(true)
        error.store(t.message!!)
      },
    )
    timer.start()
    Awaitility.await()
      .pollInterval(30.milliseconds.toJavaDuration())
      .untilAsserted {
        Assertions.assertThat(errorHandled.load()).isTrue()
        Assertions.assertThat(error.load()).isEqualTo("Test error")
      }
    timer.stop()
  }

  @ParameterizedTest
  @MethodSource("timerTypes")
  @Timeout(2, timeUnit = TimeUnit.SECONDS)
  fun `timer continues to tick after exception`(timerType: KClass<out Timer>, timerSchedule: TimerSchedule) {
    val latch = CountDownLatch(5)
    val timer = createTimer(
      timerType = timerType,
      timerSchedule = timerSchedule,
      task = Runnable {
        latch.countDown()
        throw RuntimeException("Test error")
      },
      errorHandler = { },
    )
    timer.start()
    Assertions.assertThat(latch.await(5, TimeUnit.SECONDS)).isTrue()
    timer.stop()
  }

  @OptIn(ExperimentalTime::class)
  @ParameterizedTest
  @MethodSource("timerTypes")
  @Timeout(10, timeUnit = TimeUnit.SECONDS)
  fun `ticks shouldn't run concurrently if execution is blocked for more than polling interval`(
    timerType: KClass<out Timer>,
    timerSchedule: TimerSchedule,
  ) {
    val pollingInterval = 50.milliseconds
    val numberOfInvocations = AtomicInteger(0)
    val prevActionIsRunning = AtomicBoolean(false)
    val actionWasCalledInParallel = AtomicBoolean(false)
    val timer = createTimer(
      timerType = timerType,
      timerSchedule = timerSchedule,
      task = Runnable {
        numberOfInvocations.incrementAndGet()
        if (prevActionIsRunning.load()) {
          actionWasCalledInParallel.store(true)
        }
        prevActionIsRunning.store(true)
        val executionStartTime = Clock.System.now()
        while (Clock.System.now() - executionStartTime < pollingInterval.times(3)) {
          Thread.sleep(10)
        }
        prevActionIsRunning.store(false)
      },
      errorHandler = { },
    )
    timer.start()
    Awaitility.await()
      .pollInterval(30.milliseconds.toJavaDuration())
      .untilAsserted {
        Assertions.assertThat(numberOfInvocations.get()).isGreaterThanOrEqualTo(5)
        Assertions.assertThat(actionWasCalledInParallel.load()).isFalse()
      }
    timer.stop()
  }

  @ParameterizedTest
  @MethodSource("timerTypes")
  fun `timer start is idempotent`(timerType: KClass<out Timer>, timerSchedule: TimerSchedule) {
    val timerReferences = mutableListOf<Any?>()
    val timer = createTimer(
      timerType = timerType,
      timerSchedule = timerSchedule,
      task = Runnable { },
      errorHandler = { },
    )
    repeat(5) {
      timer.start()
      timerReferences.add(timerReference(timer))
    }
    val firstReference = timerReferences.first()
    timerReferences.forEach {
      Assertions.assertThat(it).isEqualTo(firstReference)
    }
  }

  @OptIn(ExperimentalTime::class)
  @Test
  fun `Fixed rate timer executes tasks quickly if one is delayed more than polling interval`() {
    val invocationTimestamps = CopyOnWriteArrayList<Instant>()
    val pollingInterval = 50.milliseconds
    val timer = createTimer(
      timerType = VertxTimer::class,
      timerSchedule = TimerSchedule.FIXED_RATE,
      task = Runnable {
        invocationTimestamps.add(Clock.System.now())
        if (invocationTimestamps.size == 1) {
          Thread.sleep(pollingInterval.inWholeMilliseconds * 3)
        }
      },
      period = pollingInterval,
      errorHandler = { },
    )
    timer.start()
    Awaitility.await()
      .pollInterval(10.milliseconds.toJavaDuration())
      .until { invocationTimestamps.size >= 5 }
    timer.stop()

    val delays = invocationTimestamps.zipWithNext { a, b -> b - a }
    val avgPeriod = delays.reduce { acc, duration -> acc + duration } / delays.size
    assertThat(avgPeriod)
      .withFailMessage("Average period $avgPeriod, delays: $delays")
      .isLessThan(pollingInterval.times(1.5))
  }
}
