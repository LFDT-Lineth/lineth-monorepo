package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.RequestRetriesToml
import linea.coordinator.config.v2.toml.Type2StateProofManagerToml
import linea.coordinator.config.v2.toml.parseConfig
import linea.domain.BlockParameter
import linea.kotlin.toURL
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.time.Duration.Companion.seconds

class Type2StateProofProviderParsingTest {
  companion object {
    val toml =
      """
      [type2-state-proof-provider]
      disabled = false
      endpoints = ["http://shomei-frontend-i1:8888/", "http://shomei-frontend-i2:8888/"]
      l1-query-block-tag="SAFE"
      l1-polling-interval="PT12S"
      [type2-state-proof-provider.request-retries]
      max-retries = 3
      backoff-delay = "PT1S"
      failures-warning-threshold = 2
      """.trimIndent()

    val config =
      Type2StateProofManagerToml(
        disabled = false,
        endpoints = listOf("http://shomei-frontend-i1:8888/".toURL(), "http://shomei-frontend-i2:8888/".toURL()),
        l1QueryBlockTag = BlockParameter.Tag.SAFE,
        l1PollingInterval = 12.seconds,
        requestRetries =
        RequestRetriesToml(
          maxRetries = 3u,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 2u,
        ),
      )

    val tomlMinimal =
      """
      [type2-state-proof-provider]
      endpoints = ["http://shomei-frontend-i1:8888/", "http://shomei-frontend-i2:8888/"]
      """.trimIndent()

    val configMinimal =
      Type2StateProofManagerToml(
        disabled = false,
        endpoints = listOf("http://shomei-frontend-i1:8888/".toURL(), "http://shomei-frontend-i2:8888/".toURL()),
        l1QueryBlockTag = BlockParameter.Tag.FINALIZED,
        l1PollingInterval = 6.seconds,
        requestRetries =
        RequestRetriesToml(
          maxRetries = null,
          backoffDelay = 1.seconds,
          failuresWarningThreshold = 3u,
        ),
      )
  }

  data class WrapperConfig(
    val type2StateProofProvider: Type2StateProofManagerToml,
  )

  @Test
  fun `should parse type2-state-proof-provider full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).type2StateProofProvider)
      .isEqualTo(config)
  }

  @Test
  fun `should parse type2-state-proof-provider minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).type2StateProofProvider)
      .isEqualTo(configMinimal)
  }
}
