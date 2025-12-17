package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.L1FinalizationMonitorConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.domain.BlockParameter
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class L1FinalizationMonitorParsingTest {
  companion object {
    val toml =
      """
      [l1-finalization-monitor]
      l1-endpoint = "http://l1-el-node:8545"
      l2-endpoint = "http://sequencer:8545"
      l1-polling-interval = "PT1S"
      l1-query-block-tag="FINALIZED"
      """.trimIndent()

    val config =
      L1FinalizationMonitorConfigToml(
        l1Endpoint = "http://l1-el-node:8545".toURL(),
        l2Endpoint = "http://sequencer:8545".toURL(),
        l1PollingInterval = 1.seconds,
        l1QueryBlockTag = BlockParameter.Tag.FINALIZED,
      )

    val tomlMinimal =
      """
      [l1-finalization-monitor]
      l1-endpoint = "http://l1-el-node:8545"
      l2-endpoint = "http://sequencer:8545"
      """.trimIndent()

    val configMinimal =
      L1FinalizationMonitorConfigToml(
        l1Endpoint = "http://l1-el-node:8545".toURL(),
        l2Endpoint = "http://sequencer:8545".toURL(),
        l1PollingInterval = 6.seconds,
        l1QueryBlockTag = BlockParameter.Tag.FINALIZED,
      )
  }

  data class WrapperConfig(
    val l1FinalizationMonitor: L1FinalizationMonitorConfigToml,
  )

  @Test
  fun `should parse finalization monitor full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).l1FinalizationMonitor)
      .isEqualTo(config)
  }

  @Test
  fun `should parse finalization monitor minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).l1FinalizationMonitor)
      .isEqualTo(configMinimal)
  }
}
