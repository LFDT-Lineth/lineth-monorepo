package net.consensys.linea.testing.diagnostics

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.nio.file.Paths

class DiagnosticsConfigTest {
  private fun config(vararg properties: Pair<String, String>) =
    DiagnosticsConfig.fromSystemProperties(properties.toMap()::get)

  @Test
  fun `is disabled when the output directory is not set`() {
    assertThat(config()).isNull()
    assertThat(config(DiagnosticsConfig.OUTPUT_DIR_PROPERTY to "  ")).isNull()
  }

  @Test
  fun `uses the defaults when only the output directory is set`() {
    val config = config(DiagnosticsConfig.OUTPUT_DIR_PROPERTY to "/some/dir")

    assertThat(config).isEqualTo(
      DiagnosticsConfig(
        outputDir = Paths.get("/some/dir"),
        stallThresholdsMs = DiagnosticsConfig.DEFAULT_STALL_THRESHOLDS_MS,
        maxDumps = DiagnosticsConfig.DEFAULT_MAX_DUMPS,
      ),
    )
  }

  @Test
  fun `reads the thresholds and the dump budget`() {
    val config = config(
      DiagnosticsConfig.OUTPUT_DIR_PROPERTY to "/some/dir",
      DiagnosticsConfig.STALL_THRESHOLDS_PROPERTY to "500, 100",
      DiagnosticsConfig.MAX_DUMPS_PROPERTY to "3",
    )

    assertThat(config!!.stallThresholdsMs).containsExactly(100L, 500L)
    assertThat(config.maxDumps).isEqualTo(3)
  }

  @Test
  fun `ignores invalid thresholds and dump budgets`() {
    assertThat(DiagnosticsConfig.parseThresholdsMs("abc, -5, 0, 200, 200")).containsExactly(200L)
    assertThat(DiagnosticsConfig.parseThresholdsMs("abc, -5, 0"))
      .isEqualTo(DiagnosticsConfig.DEFAULT_STALL_THRESHOLDS_MS)
    assertThat(DiagnosticsConfig.parseThresholdsMs(null)).isEqualTo(DiagnosticsConfig.DEFAULT_STALL_THRESHOLDS_MS)
    assertThat(DiagnosticsConfig.parseThresholdsMs(" ")).isEqualTo(DiagnosticsConfig.DEFAULT_STALL_THRESHOLDS_MS)

    val config = config(
      DiagnosticsConfig.OUTPUT_DIR_PROPERTY to "/some/dir",
      DiagnosticsConfig.MAX_DUMPS_PROPERTY to "-1",
    )
    assertThat(config!!.maxDumps).isEqualTo(DiagnosticsConfig.DEFAULT_MAX_DUMPS)
  }
}
