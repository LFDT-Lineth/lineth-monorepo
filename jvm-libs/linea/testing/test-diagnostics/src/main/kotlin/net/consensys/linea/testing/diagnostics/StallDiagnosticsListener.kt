package net.consensys.linea.testing.diagnostics

import org.junit.platform.engine.TestExecutionResult
import org.junit.platform.launcher.TestExecutionListener
import org.junit.platform.launcher.TestIdentifier
import org.junit.platform.launcher.TestPlan
import java.lang.management.ManagementFactory
import java.nio.file.Path
import java.util.concurrent.TimeUnit
import java.util.concurrent.atomic.AtomicInteger

/**
 * Registered through the JUnit Platform ServiceLoader mechanism, so it applies to every test JVM that has this
 * module on its runtime classpath. It does nothing unless [DiagnosticsConfig.OUTPUT_DIR_PROPERTY] is set, which
 * the Gradle build does for every Test task.
 *
 * - Prints one line per test JVM with the processors and heap it sees, which are not the ones of the machine
 *   when running in a container.
 * - Writes a thread dump (see [ThreadDumpWriter]) when a test is still running after each configured threshold,
 *   to catch a test while it is stalled, which an exception raised once the test gave up cannot tell.
 *
 * Nothing in here may fail or slow down a test run, so every callback swallows its errors.
 */
class StallDiagnosticsListener : TestExecutionListener {
  private val config: DiagnosticsConfig? = DiagnosticsConfig.fromSystemProperties()

  @Volatile
  private var watchdog: StallWatchdog? = null
  private val lock = Any()
  private val dumpCount = AtomicInteger()
  private var lastDumpNanos: Long? = null

  override fun testPlanExecutionStarted(testPlan: TestPlan) {
    val config = config ?: return
    runCatching {
      watchdog = StallWatchdog(config.stallThresholdsMs, ::onStall)
      System.err.println(banner(config))
    }.onFailure { reportFailure(it) }
  }

  override fun executionStarted(testIdentifier: TestIdentifier) {
    if (testIdentifier.isTest) {
      runCatching {
        watchdog?.started(
          testIdentifier.uniqueId,
        )
      }.onFailure { reportFailure(it) }
    }
  }

  override fun executionFinished(testIdentifier: TestIdentifier, testExecutionResult: TestExecutionResult) {
    if (testIdentifier.isTest) {
      runCatching {
        watchdog?.finished(
          testIdentifier.uniqueId,
        )
      }.onFailure { reportFailure(it) }
    }
  }

  override fun testPlanExecutionFinished(testPlan: TestPlan) {
    runCatching { watchdog?.close() }.onFailure { reportFailure(it) }
  }

  private fun onStall(event: StallWatchdog.StallEvent) {
    val config = config ?: return
    val file = nextDumpFile(config) ?: return
    val header = buildList {
      add("trigger: test=${event.key} runningFor=${event.elapsedMs}ms (threshold ${event.thresholdMs}ms)")
      add("tests running (${event.running.size}), longest first:")
      event.running.entries.sortedByDescending { it.value }.forEach { add("  ${it.value}ms ${it.key}") }
    }
    ThreadDumpWriter.write(file, header)
    System.err.println(
      "[test-diagnostics] ${event.key} still running after ${event.elapsedMs}ms " +
        "(threshold ${event.thresholdMs}ms), thread dump: $file",
    )
  }

  /**
   * Reserves the next dump file, or returns null when the dump budget is spent or another dump was written less
   * than [MIN_DUMP_INTERVAL_MS] ago: when many tests stall together, one dump already shows all of them.
   */
  private fun nextDumpFile(config: DiagnosticsConfig): Path? = synchronized(lock) {
    val now = System.nanoTime()
    val last = lastDumpNanos
    if (dumpCount.get() >= config.maxDumps) return null
    if (last != null && now - last < TimeUnit.MILLISECONDS.toNanos(MIN_DUMP_INTERVAL_MS)) return null
    lastDumpNanos = now
    config.outputDir.resolve("thread-dump-${ProcessHandle.current().pid()}-${dumpCount.incrementAndGet()}.txt")
  }

  private fun banner(config: DiagnosticsConfig): String {
    val gcNames = ManagementFactory.getGarbageCollectorMXBeans().map { it.name }
    return "[test-diagnostics] pid=${ProcessHandle.current().pid()} " +
      "availableProcessors=${Runtime.getRuntime().availableProcessors()} " +
      "maxHeapMB=${Runtime.getRuntime().maxMemory() / (1024 * 1024)} gc=$gcNames " +
      "stallThresholdsMs=${config.stallThresholdsMs} dir=${config.outputDir}"
  }

  private fun reportFailure(error: Throwable) {
    System.err.println("[test-diagnostics] callback failed, the test run is not affected: $error")
  }

  private companion object {
    const val MIN_DUMP_INTERVAL_MS = 1_000L
  }
}
