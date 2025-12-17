package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.StateManagerToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class StateManagerParsingTest {
  companion object {
    val toml =
      """
      [state-manager]
      version = "2.2.0"
      endpoints = ["http://shomei:8888/"]
      request-limit-per-endpoint = 3
      request-timeout = "PT30S"
      [state-manager.request-retries]
      max-retries = 5
      backoff-delay = "PT2S"
      failures-warning-threshold = 2
      """.trimIndent()

    val config =
      StateManagerToml(
        version = "2.2.0",
        endpoints = listOf("http://shomei:8888/".toURL()),
        requestLimitPerEndpoint = 3u,
        requestTimeout = 30.seconds,
        requestRetries =
        RequestRetriesToml(
          maxRetries = 5u,
          backoffDelay = 2.seconds,
          failuresWarningThreshold = 2u,
        ),
      )

    val tomlMinimal =
      """
      [state-manager]
      version = "2.2.0"
      endpoints = ["http://shomei:8888/"]
      """.trimIndent()

    val configMinimal =
      StateManagerToml(
        version = "2.2.0",
        endpoints = listOf("http://shomei:8888/".toURL()),
        requestLimitPerEndpoint = UInt.MAX_VALUE,
        requestTimeout = null,
        requestRetries =
        RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
      )
  }

  data class WrapperConfig(
    val stateManager: StateManagerToml,
  )

  @Test
  fun `should parse state manager full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).stateManager)
      .isEqualTo(config)
  }

  @Test
  fun `should parse state manager minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).stateManager)
      .isEqualTo(configMinimal)
  }
}
