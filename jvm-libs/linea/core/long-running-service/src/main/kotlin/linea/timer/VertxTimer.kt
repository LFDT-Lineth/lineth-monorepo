package linea.timer

import io.vertx.core.Vertx
import java.util.concurrent.Callable
import java.util.concurrent.atomic.AtomicInteger
import kotlin.concurrent.atomics.AtomicReference
import kotlin.concurrent.atomics.ExperimentalAtomicApi
import kotlin.time.Clock
import kotlin.time.Duration
import kotlin.time.Duration.Companion.milliseconds
import kotlin.time.ExperimentalTime
import kotlin.time.Instant

@OptIn(ExperimentalTime::class, ExperimentalAtomicApi::class)
class VertxTimer(
  private val vertx: Vertx,
  override val name: String,
  override val initialDelay: Duration,
  override val period: Duration,
  override val timerSchedule: TimerSchedule,
  override val errorHandler: (Throwable) -> Unit,
  override val task: Runnable,
) : Timer {
  init {
    require(period.inWholeMilliseconds >= 1L) { "Vertx Timer period must be at least 1 ms" }
  }
  init {
    require(initialDelay.inWholeMilliseconds >= 1L) { "Vertx Timer initial delay must be at least 1 ms" }
  }
  private var timerId: Long? = null
  private val invocationCounter = AtomicInteger(0)
  private var firstInvocationTime: AtomicReference<Instant?> = AtomicReference(null)

  internal fun timerIdReference(): Long? = timerId

  @Synchronized
  override fun start() {
    if (timerId == null) {
      timerId = vertx.setTimer(initialDelay.inWholeMilliseconds, this::taskHandler)
    }
  }

  @Suppress("UNUSED_PARAMETER")
  private fun taskHandler(_timerId: Long) {
    invocationCounter.incrementAndGet()
    firstInvocationTime.compareAndSet(null, Clock.System.now())

    val callable = Callable {
      task.run()
    }
    vertx.executeBlocking(callable, false).onComplete { result ->
      if (result.cause() != null) {
        errorHandler(result.cause())
      }
      synchronized(this) {
        if (timerId != null) {
          timerId = vertx.setTimer(nextInvocationDelay().inWholeMilliseconds, this::taskHandler)
        }
      }
    }
  }

  private fun nextInvocationDelay(): Duration {
    return when (timerSchedule) {
      TimerSchedule.FIXED_DELAY -> period
      TimerSchedule.FIXED_RATE -> {
        val firstTime = firstInvocationTime.load()!!
        val expectedNextInvocationTime = firstTime + period * invocationCounter.get()
        val now = Clock.System.now()
        val delay = expectedNextInvocationTime - now
        if (delay < 1.milliseconds) {
          1.milliseconds // Vertx requires a delay of at least 1 ms
        } else {
          delay
        }
      }
    }
  }

  @Synchronized
  override fun stop() {
    if (timerId != null) {
      vertx.cancelTimer(timerId!!)
      invocationCounter.set(0)
      firstInvocationTime.store(null)
      timerId = null
    }
  }
}

class VertxTimerFactory(private val vertx: Vertx) : TimerFactory {
  override fun createTimer(
    name: String,
    initialDelay: Duration,
    period: Duration,
    timerSchedule: TimerSchedule,
    errorHandler: (Throwable) -> Unit,
    task: Runnable,
  ): Timer {
    return VertxTimer(
      vertx = vertx,
      name = name,
      task = task,
      initialDelay = initialDelay,
      period = period,
      timerSchedule = timerSchedule,
      errorHandler = errorHandler,
    )
  }
}
