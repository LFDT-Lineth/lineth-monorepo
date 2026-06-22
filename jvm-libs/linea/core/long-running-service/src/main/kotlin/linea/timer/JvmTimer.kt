package linea.timer

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

  internal fun timerReference(): Timer? = timer

  @Synchronized
  override fun start() {
    if (timer != null) {
      return
    }
    timer = Timer(name, true)
    val timerTask = timerTask {
      try {
        task.run()
      } catch (t: Throwable) {
        try {
          errorHandler(t)
        } catch (handlerEx: Throwable) {
          System.err.println("JvmTimer[$name] errorHandler threw: ${handlerEx.message}")
        }
        if (t is VirtualMachineError || t is LinkageError) throw t
      }
    }
    when (timerSchedule) {
      TimerSchedule.FIXED_DELAY ->
        timer!!.schedule(timerTask, initialDelay.inWholeMilliseconds, period.inWholeMilliseconds)

      TimerSchedule.FIXED_RATE ->
        timer!!.scheduleAtFixedRate(timerTask, initialDelay.inWholeMilliseconds, period.inWholeMilliseconds)
    }
  }

  @Synchronized
  override fun stop() {
    if (timer == null) {
      return
    }
    timer?.cancel()
    timer = null
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
