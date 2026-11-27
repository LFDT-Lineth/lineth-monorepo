package net.consensys.linea.testing.diagnostics

import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.CopyOnWriteArrayList
import java.util.concurrent.ScheduledFuture
import java.util.concurrent.ScheduledThreadPoolExecutor
import java.util.concurrent.TimeUnit

/**
 * Calls [onStall] each time a started task is still running after one of [thresholdsMs].
 *
 * It is independent of JUnit so it can be tested on its own: [StallDiagnosticsListener] adapts test executions
 * to [started] and [finished].
 */
class StallWatchdog(
  private val thresholdsMs: List<Long>,
  private val onStall: (StallEvent) -> Unit,
) : AutoCloseable {
  data class StallEvent(
    val key: String,
    val thresholdMs: Long,
    val elapsedMs: Long,
    /** Elapsed milliseconds of every task running when the event fired, [key] included. */
    val running: Map<String, Long>,
  )

  private class Running(val startNanos: Long) {
    val scheduledChecks = CopyOnWriteArrayList<ScheduledFuture<*>>()
  }

  private val scheduler = ScheduledThreadPoolExecutor(1) { runnable ->
    Thread(runnable, "test-stall-watchdog").apply { isDaemon = true }
  }.apply { removeOnCancelPolicy = true }
  private val running = ConcurrentHashMap<String, Running>()

  fun started(key: String) {
    val entry = Running(System.nanoTime())
    running[key] = entry
    thresholdsMs.forEach { thresholdMs ->
      entry.scheduledChecks += scheduler.schedule({ check(key, thresholdMs) }, thresholdMs, TimeUnit.MILLISECONDS)
    }
  }

  fun finished(key: String) {
    running.remove(key)?.scheduledChecks?.forEach { it.cancel(false) }
  }

  /** Elapsed milliseconds of every task currently running. */
  fun runningTasks(): Map<String, Long> {
    val now = System.nanoTime()
    return running.mapValues { (_, entry) -> TimeUnit.NANOSECONDS.toMillis(now - entry.startNanos) }
  }

  internal fun scheduledCheckCount(): Int = scheduler.queue.size

  private fun check(key: String, thresholdMs: Long) {
    // diagnostics must never get in the way of the tests: a failing callback is reported and swallowed
    runCatching {
      val snapshot = runningTasks()
      val elapsedMs = snapshot[key] ?: return // finished meanwhile
      onStall(StallEvent(key, thresholdMs, elapsedMs, snapshot))
    }.onFailure { System.err.println("[test-diagnostics] stall callback failed: $it") }
  }

  override fun close() {
    scheduler.shutdownNow()
    running.clear()
  }
}
