package linea.timer

import tech.pegasys.teku.infrastructure.async.SafeFuture
import java.util.Timer
import kotlin.concurrent.timerTask
import kotlin.time.Duration

class JvmTimer(
  override val name: String,
  override val initialDelay: Duration,
  override val period: Duration,
  override val timerSchedule: TimerSchedule,
  override val errorHandler: (Throwable) -> Unit,
  override val task: Runnable,
) : linea.timer.Timer {
  private var timer: Timer? = null
  private var inFlightExecution: SafeFuture<Unit>? = null

  internal fun timerReference(): Timer? = timer

  @Synchronized
  override fun start() {
    if (timer != null) {
      return
    }
    val newTimer = Timer(name, true)
    timer = newTimer
    val timerTask = timerTask {
      val execution = SafeFuture<Unit>()
      synchronized(this@JvmTimer) {
        // timer was stopped after this execution was dequeued: skip it
        if (timer !== newTimer) return@timerTask
        inFlightExecution = execution
      }
      try {
        task.run()
      } catch (t: Throwable) {
        try {
          errorHandler(t)
        } catch (handlerEx: Throwable) {
          System.err.println("JvmTimer[$name] errorHandler threw: ${handlerEx.message}")
        }
        if (t is VirtualMachineError || t is LinkageError) throw t
      } finally {
        synchronized(this@JvmTimer) {
          if (inFlightExecution === execution) {
            inFlightExecution = null
          }
        }
        execution.complete(Unit)
      }
    }
    when (timerSchedule) {
      TimerSchedule.FIXED_DELAY ->
        newTimer.schedule(timerTask, initialDelay.inWholeMilliseconds, period.inWholeMilliseconds)

      TimerSchedule.FIXED_RATE ->
        newTimer.scheduleAtFixedRate(timerTask, initialDelay.inWholeMilliseconds, period.inWholeMilliseconds)
    }
  }

  @Synchronized
  override fun stop(): SafeFuture<Unit> {
    timer?.cancel()
    timer = null
    return inFlightExecution ?: SafeFuture.completedFuture(Unit)
  }
}

class JvmTimerFactory : TimerFactory {
  override fun createTimer(
    name: String,
    initialDelay: Duration,
    period: Duration,
    timerSchedule: TimerSchedule,
    errorHandler: (Throwable) -> Unit,
    task: Runnable,
  ): linea.timer.Timer {
    return JvmTimer(
      name = name,
      initialDelay = initialDelay,
      period = period,
      timerSchedule = timerSchedule,
      errorHandler = errorHandler,
      task = task,
    )
  }
}
