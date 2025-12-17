package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.DefaultsToml
import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class DefaultsParsingTest {
  companion object {
    val toml =
      """
      [defaults]
      l1-endpoint = "http://l1-el-node:8545"
      l2-endpoint = "http://sequencer:8545"
      [defaults.l1-request-retries]
      backoff-delay = "PT2S"
      failures-warning-threshold = 2
      timeout = "PT20S"
      [defaults.l2-request-retries]
      backoff-delay = "PT3S"
      failures-warning-threshold = 3
      timeout = "PT30S"
      """.trimIndent()

    val config =
      DefaultsToml(
        l1Endpoint = "http://l1-el-node:8545".toURL(),
        l2Endpoint = "http://sequencer:8545".toURL(),
        l1RequestRetries =
        RequestRetriesToml(
          backoffDelay = 2.seconds,
          failuresWarningThreshold = 2u,
          timeout = 20.seconds,
        ),
        l2RequestRetries =
        RequestRetriesToml(
          backoffDelay = 3.seconds,
          failuresWarningThreshold = 3u,
          timeout = 30.seconds,
        ),
      )

    val tomlMinimal =
      """
      """.trimIndent()

    val configMinimal =
      DefaultsToml(
        l1Endpoint = null,
        l2Endpoint = null,
        l1RequestRetries =
        RequestRetriesToml.endlessRetry(
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
        l2RequestRetries =
        RequestRetriesToml.endlessRetry(
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
      )
  }

  internal data class WrapperConfig(val defaults: DefaultsToml = DefaultsToml())

  @Test
  fun `should parse defaults full configs`() {
    assertThat(parseConfig<WrapperConfig>(toml).defaults).isEqualTo(config)
  }

  @Test
  fun `should parse defaults minimal configs`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).defaults).isEqualTo(configMinimal)
  }
}
