package net.consensys.linea.testing.diagnostics

import java.nio.file.Path
import java.nio.file.Paths

/**
 * Configuration of the test diagnostics, read from the system properties the Gradle build passes to the test JVM.
 */
data class DiagnosticsConfig(
  val outputDir: Path,
  val stallThresholdsMs: List<Long>,
  val maxDumps: Int,
) {
  companion object {
    const val OUTPUT_DIR_PROPERTY = "lineth.test.diagnostics.dir"
    const val STALL_THRESHOLDS_PROPERTY = "lineth.test.stallDump.thresholdsMs"
    const val MAX_DUMPS_PROPERTY = "lineth.test.stallDump.maxDumps"

    /**
     * 3s catches the stalls of tests that give up after 5s, 30s and 4min catch real hangs before the 5min
     * JUnit default timeout kills the test.
     */
    val DEFAULT_STALL_THRESHOLDS_MS = listOf(3_000L, 30_000L, 240_000L)
    const val DEFAULT_MAX_DUMPS = 20

    /**
     * @return null when the output directory is not configured, e.g. when tests run from an IDE: the diagnostics
     * are then disabled.
     */
    fun fromSystemProperties(property: (String) -> String? = { System.getProperty(it) }): DiagnosticsConfig? {
      val outputDir = property(OUTPUT_DIR_PROPERTY)?.trim()?.takeIf { it.isNotEmpty() } ?: return null
      return DiagnosticsConfig(
        outputDir = Paths.get(outputDir),
        stallThresholdsMs = parseThresholdsMs(property(STALL_THRESHOLDS_PROPERTY)),
        maxDumps = property(MAX_DUMPS_PROPERTY)?.trim()?.toIntOrNull()?.takeIf { it >= 0 } ?: DEFAULT_MAX_DUMPS,
      )
    }

    /**
     * Parses a comma separated list of milliseconds. Entries that are not positive numbers are ignored and the
     * defaults are used when nothing valid is left.
     */
    fun parseThresholdsMs(value: String?): List<Long> {
      val parsed = value
        ?.split(',')
        ?.mapNotNull { it.trim().toLongOrNull() }
        ?.filter { it > 0 }
        ?.distinct()
        ?.sorted()
      return if (parsed.isNullOrEmpty()) DEFAULT_STALL_THRESHOLDS_MS else parsed
    }
  }
}
