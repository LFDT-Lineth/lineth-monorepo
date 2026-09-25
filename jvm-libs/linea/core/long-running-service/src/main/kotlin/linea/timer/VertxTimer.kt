package linea.timer

import io.vertx.core.Vertx
import tech.pegasys.teku.infrastructure.async.SafeFuture
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
  private var inFlightExecution: SafeFuture<Unit>? = null
  private val invocationCounter = AtomicInteger(0)
  private var firstInvocationTime: AtomicReference<Instant?> = AtomicReference(null)

  internal fun timerIdReference(): Long? = timerId

  @Synchronized
  override fun start() {
    if (timerId == null) {
      timerId = vertx.setTimer(initialDelay.inWholeMilliseconds, this::taskHandler)
    }
  }

  private fun taskHandler(firedTimerId: Long) {
    val execution = SafeFuture<Unit>()
    synchronized(this) {
      // timer was stopped (or stopped and restarted) after this one fired: skip execution
      if (timerId != firedTimerId) return
      inFlightExecution = execution
    }
    invocationCounter.incrementAndGet()
    firstInvocationTime.compareAndSet(null, Clock.System.now())

    val callable = Callable {
      task.run()
    }
    vertx.executeBlocking(callable, false).onComplete { result ->
      try {
        if (result.cause() != null) {
          errorHandler(result.cause())
        }
      } finally {
        synchronized(this) {
          if (timerId == firedTimerId) {
            timerId = vertx.setTimer(nextInvocationDelay().inWholeMilliseconds, this::taskHandler)
          }
          if (inFlightExecution === execution) {
            inFlightExecution = null
          }
        }
        execution.complete(Unit)
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
  override fun stop(): SafeFuture<Unit> {
    if (timerId != null) {
      vertx.cancelTimer(timerId!!)
      invocationCounter.set(0)
      firstInvocationTime.store(null)
      timerId = null
    }
    return inFlightExecution ?: SafeFuture.completedFuture(Unit)
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
