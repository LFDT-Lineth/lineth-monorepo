package linea.coordinator.config.v2.docs

import linea.coordinator.config.v2.toml.DatabaseToml
import linea.coordinator.config.v2.toml.MessageAnchoringConfigToml
import linea.coordinator.config.v2.toml.TracesLimitsConfigFileV4Toml
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigSchemaWalkerTest {
  private fun List<ConfigKey>.byPath(path: String): ConfigKey =
    single { it.path == path }

  @Test
  fun `output is sorted by path`() {
    val paths = ConfigSchemaWalker.walk(DatabaseToml::class).map { it.path }
    assertThat(paths).isSorted
  }

  @Test
  fun `walks leaf parameters with type and required, deriving required-ness from reflection`() {
    val keys = ConfigSchemaWalker.walk(DatabaseToml::class)

    val hostname = keys.byPath("hostname")
    assertThat(hostname.type).isEqualTo("String")
    assertThat(hostname.required).isTrue()
    assertThat(hostname.isSection).isFalse()

    // optional (has a compiled default) => not required, even though the default value itself
    // is only surfaced once the parameter is annotated with @ConfigDoc(default = ...).
    val port = keys.byPath("port")
    assertThat(port.type).isEqualTo("UInt")
    assertThat(port.required).isFalse()

    val password = keys.byPath("password")
    assertThat(password.type).isEqualTo("Masked")
    assertThat(password.required).isTrue()
  }

  @Test
  fun `derives optional from nullability without a compiled default`() {
    val keys = ConfigSchemaWalker.walk(MessageAnchoringConfigToml::class)

    // nullable, no default => optional (Hoplite treats an absent nullable as null)
    val l1Endpoint = keys.byPath("l1-endpoint")
    assertThat(l1Endpoint.type).isEqualTo("URL?")
    assertThat(l1Endpoint.required).isFalse()
  }

  @Test
  fun `recurses into nested sections and emits both the section and its leaves`() {
    val keys = ConfigSchemaWalker.walk(DatabaseToml::class)

    val section = keys.byPath("persistence-retries")
    assertThat(section.isSection).isTrue()
    assertThat(section.type).isEqualTo("RequestRetriesToml")

    val backoff = keys.byPath("persistence-retries.backoff-delay")
    assertThat(backoff.isSection).isFalse()
    assertThat(backoff.type).isEqualTo("Duration")

    val maxRetries = keys.byPath("persistence-retries.max-retries")
    assertThat(maxRetries.type).isEqualTo("UInt?")
    assertThat(maxRetries.required).isFalse()
  }

  @Test
  fun `recurses through multiple levels of nested sections`() {
    val keys = ConfigSchemaWalker.walk(MessageAnchoringConfigToml::class)
    val paths = keys.map { it.path }

    assertThat(keys.byPath("l1-event-scraping").isSection).isTrue()
    assertThat(keys.byPath("gas").isSection).isTrue()
    assertThat(paths).contains(
      "l1-event-scraping.polling-interval",
      "gas.max-fee-per-gas-cap",
      "signer.type",
    )
  }

  @Test
  fun `treats nullable URL and BlockParameter as leaves`() {
    val keys = ConfigSchemaWalker.walk(MessageAnchoringConfigToml::class)

    val l1Endpoint = keys.byPath("l1-endpoint")
    assertThat(l1Endpoint.isSection).isFalse()
    assertThat(l1Endpoint.type).isEqualTo("URL?")

    val blockTag = keys.byPath("l1-highest-block-tag")
    assertThat(blockTag.isSection).isFalse()
    assertThat(blockTag.type).isEqualTo("BlockParameter")
  }

  @Test
  fun `documents a dynamic map as a single key rather than enumerating entries`() {
    val keys = ConfigSchemaWalker.walk(TracesLimitsConfigFileV4Toml::class)

    val tracesLimits = keys.byPath("traces-limits")
    assertThat(tracesLimits.isSection).isFalse()
    assertThat(tracesLimits.type).isEqualTo("Map<TracingModuleV4, UInt>")
    assertThat(tracesLimits.required).isTrue()
  }

  @Test
  fun `real classes are annotated with non-blank descriptions`() {
    val keys = ConfigSchemaWalker.walk(DatabaseToml::class)
    assertThat(keys).isNotEmpty
    assertThat(keys).allMatch { it.annotated }
    assertThat(keys).allMatch { it.description.isNotBlank() }
  }
}
