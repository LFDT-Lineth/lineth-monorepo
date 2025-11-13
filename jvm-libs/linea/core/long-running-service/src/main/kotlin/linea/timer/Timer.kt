package linea.timer

import kotlin.time.Duration

enum class TimerSchedule {
  /*
   * A fixed delay timer schedules the next execution of the task after a fixed delay
   * following the completion of the previous execution.
   */
  FIXED_DELAY,

  /*
   * A fixed rate timer schedules the next execution of the task based on a fixed frequency.
   * If a task execution is delayed, the next execution(s) will attempt to "catch up" to
   * maintain the fixed rate. Catching up is done by scheduling the next task(s)
   * immediately as many times as needed until the schedule is back on track.
   */
  FIXED_RATE,
}

/**
 *  * Interface representing a timer that can schedule and execute tasks at fixed intervals.
 *  * @param name The name of the timer.
 *  * @param task The task to be executed periodically. Can be blocking
 *  * @param initialDelay The initial delay before the first execution of the task.
 *  * @param period The period between successive executions of the task in case of Fixed Delay
 *  * and frequency = 1/period in case of fixed rate timers.
 *  * @param errorHandler A function to handle any errors that occur during task execution.
 *  */
interface Timer {
  val name: String
  val task: Runnable
  val initialDelay: Duration
  val period: Duration
  val errorHandler: (Throwable) -> Unit
  val timerSchedule: TimerSchedule

  /*
   * Start method to initiate the timer and begin executing the scheduled task.
   * It is idempotent. Calling start on an already started timer has no effect.
   */
  fun start()

  /*
   * Stop method to halt the timer and cease executing the scheduled task.
   * It is idempotent. Calling stop on an already stopped timer has no effect.
   */
  fun stop()
}

interface TimerFactory {
  fun createTimer(
    name: String,
    initialDelay: Duration,
    period: Duration,
    timerSchedule: TimerSchedule,
    errorHandler: (Throwable) -> Unit,
    task: Runnable,
  ): Timer
}
