package linea.coordinator.config.v2.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

/**
 * Verifies that config key paths are rendered by Hoplite's `KebabCaseParamMapper`, so the
 * documented keys stay consistent with how Hoplite maps property names when parsing the TOML.
 */
class ConfigKeyRenderingTest {
  private data class SampleToml(
    val hostname: String = "",
    val readPoolSize: Int = 0,
    val maxFeePerGasCap: Long = 0,
    val l1Endpoint: String = "",
    val l2NetworkGasPricing: String = "",
    val type2StateProofProvider: String = "",
    // consecutive capitals: Hoplite treats each uppercase as a boundary
    val httpTLSPort: Int = 0,
  )

  private fun pathsOf(): Set<String> = ConfigSchemaWalker.walk(SampleToml::class).map { it.path }.toSet()

  @Test
  fun `renders camelCase leaf names as Hoplite kebab-case keys`() {
    assertThat(pathsOf()).contains(
      "hostname",
      "read-pool-size",
      "max-fee-per-gas-cap",
      "l1-endpoint",
      "l2-network-gas-pricing",
      "type2-state-proof-provider",
    )
  }

  @Test
  fun `follows Hoplite acronym handling rather than collapsing acronyms`() {
    // Hoplite inserts a boundary before every uppercase, so 'TLS' does not collapse to 'tls'.
    assertThat(pathsOf()).contains("http-t-l-s-port")
  }
}
