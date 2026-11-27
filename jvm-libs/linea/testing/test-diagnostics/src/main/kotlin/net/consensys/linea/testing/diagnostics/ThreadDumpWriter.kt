package net.consensys.linea.testing.diagnostics

import java.lang.management.ManagementFactory
import java.lang.management.ThreadInfo
import java.nio.file.Files
import java.nio.file.Path
import java.time.Instant
import java.util.Locale

/**
 * Writes a plain-text snapshot of the JVM meant to tell apart the usual causes of a stalled test:
 * - CPU starvation: low process CPU relative to the available processors, runnable threads with little CPU time
 * - GC pressure: heap close to its maximum, GC time growing between snapshots
 * - a lock or a deadlock: BLOCKED threads and the owner of the lock they wait on
 * - something never completing: every thread is idle in the stacks
 */
object ThreadDumpWriter {
  private const val TOP_CPU_THREADS = 15

  fun write(file: Path, headerLines: List<String>) {
    file.parent?.let { Files.createDirectories(it) }
    Files.writeString(file, render(headerLines))
  }

  fun render(headerLines: List<String>): String = buildString {
    appendLine("=== Test JVM snapshot ===")
    appendLine("time=${Instant.now()} pid=${ProcessHandle.current().pid()}")
    headerLines.forEach(::appendLine)
    appendLine()
    appendJvmFigures()
    appendLine()
    appendThreads()
  }

  private fun StringBuilder.appendJvmFigures() {
    val runtime = Runtime.getRuntime()
    appendLine("=== JVM ===")
    appendLine("uptime=${ManagementFactory.getRuntimeMXBean().uptime}ms")
    appendLine("availableProcessors=${runtime.availableProcessors()}")

    val heap = ManagementFactory.getMemoryMXBean().heapMemoryUsage
    appendLine("heap used=${heap.used.toMb()}MB committed=${heap.committed.toMb()}MB max=${heap.max.toMb()}MB")

    ManagementFactory.getGarbageCollectorMXBeans().forEach { gc ->
      appendLine("gc name='${gc.name}' collections=${gc.collectionCount} totalTime=${gc.collectionTime}ms")
    }

    val os = ManagementFactory.getOperatingSystemMXBean()
    appendLine("systemLoadAverage=${os.systemLoadAverage}")
    if (os is com.sun.management.OperatingSystemMXBean) {
      // The instantaneous cpuLoad/processCpuLoad are deltas since the previous call, so they read 0 on a first
      // sample. The average since the JVM started is meaningful: cores close to availableProcessors means this JVM
      // saturates its CPUs, much lower while tests are stalled means it was waiting or starved.
      val processCpuMs = os.processCpuTime / NANOS_PER_MILLI
      val uptimeMs = ManagementFactory.getRuntimeMXBean().uptime.coerceAtLeast(1)
      appendLine(
        "processCpuTime=${processCpuMs}ms avgCpuCoresSinceStart=${String.format(
          Locale.ROOT,
          "%.2f",
          processCpuMs.toDouble() / uptimeMs,
        )}",
      )
      appendLine("physicalMemory free=${os.freeMemorySize.toMb()}MB total=${os.totalMemorySize.toMb()}MB")
    }
  }

  private fun StringBuilder.appendThreads() {
    val threadBean = ManagementFactory.getThreadMXBean()
    val cpuTimeAvailable = threadBean.isThreadCpuTimeSupported && threadBean.isThreadCpuTimeEnabled
    val threads = threadBean.dumpAllThreads(true, true).filterNotNull()
    val cpuMs = threads.associate { thread ->
      val cpuNanos = if (cpuTimeAvailable) threadBean.getThreadCpuTime(thread.threadId) else -1L
      thread.threadId to if (cpuNanos < 0) -1L else cpuNanos / NANOS_PER_MILLI
    }

    appendLine("=== Threads: ${threads.size} (peak ${threadBean.peakThreadCount}) ===")
    appendLine(
      threads.groupingBy { it.threadState }.eachCount().entries
        .sortedBy { it.key }
        .joinToString(" ") { "${it.key}=${it.value}" },
    )
    threadBean.findDeadlockedThreads()?.let { ids ->
      val names = threads.filter { it.threadId in ids }.joinToString { "'${it.threadName}'" }
      appendLine("DEADLOCK among threads: $names")
    }
    appendLine()

    appendLine("=== Top $TOP_CPU_THREADS threads by CPU time (cpu=-1ms: not available) ===")
    threads
      .sortedByDescending { cpuMs[it.threadId] ?: -1L }
      .take(TOP_CPU_THREADS)
      .forEach { appendLine("cpu=${cpuMs[it.threadId]}ms state=${it.threadState} '${it.threadName}'") }
    appendLine()

    appendLine("=== All threads ===")
    threads.forEach { appendThread(it, cpuMs[it.threadId] ?: -1L) }
  }

  private fun StringBuilder.appendThread(thread: ThreadInfo, cpuMs: Long) {
    val daemon = if (thread.isDaemon) " daemon" else ""
    appendLine("\"${thread.threadName}\" #${thread.threadId}$daemon state=${thread.threadState} cpu=${cpuMs}ms")
    if (thread.lockName != null) {
      val owner = thread.lockOwnerName?.let { " owned by \"$it\" #${thread.lockOwnerId}" } ?: ""
      appendLine("    waiting on <${thread.lockName}>$owner")
    }
    thread.stackTrace.forEach { appendLine("    at $it") }
    appendLine()
  }

  private fun Long.toMb(): Long = if (this < 0) this else this / BYTES_PER_MB

  private const val NANOS_PER_MILLI = 1_000_000L
  private const val BYTES_PER_MB = 1024L * 1024L
}
