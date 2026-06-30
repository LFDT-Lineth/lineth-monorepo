package linea.coordinator.config.v2.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class NameMappingTest {
  @Test
  fun `converts simple camelCase to kebab-case`() {
    assertThat(camelToKebabCase("hostname")).isEqualTo("hostname")
    assertThat(camelToKebabCase("readPoolSize")).isEqualTo("read-pool-size")
    assertThat(camelToKebabCase("persistenceRetries")).isEqualTo("persistence-retries")
    assertThat(camelToKebabCase("maxFeePerGasCap")).isEqualTo("max-fee-per-gas-cap")
    assertThat(camelToKebabCase("genesisStateRootHash")).isEqualTo("genesis-state-root-hash")
  }

  @Test
  fun `keeps digits attached to the preceding token`() {
    assertThat(camelToKebabCase("l1Endpoint")).isEqualTo("l1-endpoint")
    assertThat(camelToKebabCase("l2NetworkGasPricing")).isEqualTo("l2-network-gas-pricing")
    assertThat(camelToKebabCase("l1HighestBlockTag")).isEqualTo("l1-highest-block-tag")
    assertThat(camelToKebabCase("type2StateProofProvider")).isEqualTo("type2-state-proof-provider")
    assertThat(camelToKebabCase("maxMessagesToAnchorPerL2Transaction"))
      .isEqualTo("max-messages-to-anchor-per-l2-transaction")
  }

  @Test
  fun `treats an acronym followed by a word as a boundary`() {
    assertThat(camelToKebabCase("httpServer")).isEqualTo("http-server")
    assertThat(camelToKebabCase("HTTPServer")).isEqualTo("http-server")
  }

  @Test
  fun `is idempotent for already lowercase names`() {
    assertThat(camelToKebabCase("disabled")).isEqualTo("disabled")
    assertThat(camelToKebabCase("port")).isEqualTo("port")
  }
}
