package linea.coordinator.config.v2

import linea.coordinator.config.v2.toml.ApiConfigToml
import linea.coordinator.config.v2.toml.parseConfig
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ApiConfigParsingTest {
  companion object {
    val toml = """
      [api]
      observability-port = 9546
    """.trimIndent()

    val config = ApiConfigToml(
      observabilityPort = 9546u,
    )

    val tomlMinimal = ""

    val configMinimal = ApiConfigToml(
      observabilityPort = 9545u,
    )
  }

  data class WrapperConfig(
    val api: ApiConfigToml = ApiConfigToml(),
  )

  @Test
  fun `should parse api full config`() {
    assertThat(parseConfig<WrapperConfig>(toml).api).isEqualTo(config)
  }

  @Test
  fun `should parse api minimal config`() {
    assertThat(parseConfig<WrapperConfig>(tomlMinimal).api).isEqualTo(configMinimal)
  }
}
