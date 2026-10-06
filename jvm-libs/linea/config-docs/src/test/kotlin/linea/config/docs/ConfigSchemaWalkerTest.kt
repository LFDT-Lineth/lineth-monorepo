package linea.config.docs

import com.sksamuel.hoplite.Masked
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import java.net.URL
import kotlin.time.Duration

class ConfigSchemaWalkerTest {
  enum class Mode { A, B }

  data class SampleToml(
    @param:ConfigDoc("Hostname.", example = "localhost")
    val hostname: String,
    @param:ConfigDoc("Port.", default = "5432")
    val port: UInt = 5432u,
    @param:ConfigDoc("Password.")
    val password: Masked,
    @param:ConfigDoc("Optional endpoint.")
    val endpoint: URL? = null,
    @param:ConfigDoc("A dynamic map.")
    val limits: Map<Mode, UInt> = emptyMap(),
    @param:ConfigSection("Nested retries.")
    val retries: NestedToml = NestedToml(),
    @param:ConfigSection(description = "Legacy section.", deprecated = true, replacement = "retries")
    val legacyRetries: NestedToml = NestedToml(),
    val undocumented: Int = 0,
    // consecutive capitals: acronym-aware kebab keeps TLS together
    val httpTLSPort: Int = 0,
    val fanoutTTL: Duration = Duration.ZERO,
    val seenTTL: Duration = Duration.ZERO,
  ) {
    data class NestedToml(
      @param:ConfigDoc("Timeout.", default = "PT1S")
      val timeout: Duration = Duration.ZERO,
    )
  }

  private val detector = sectionByPackagePrefix("linea.config.docs")
  private fun keys() = ConfigSchemaWalker.walk(SampleToml::class, detector)
  private fun List<ConfigKey>.byPath(path: String) = single { it.path == path }

  @Test
  fun `renders acronym-aware kebab keys and sorts by path`() {
    val paths = keys().map { it.path }
    assertThat(paths).isSorted
    assertThat(paths).contains(
      "hostname",
      "port",
      "password",
      "endpoint",
      "limits",
      "retries",
      "retries.timeout",
      "legacy-retries",
      "legacy-retries.timeout",
      "undocumented",
      "http-tls-port",
      "fanout-ttl",
      "seen-ttl",
    )
  }

  @Test
  fun `toKebabCase keeps acronyms together`() {
    assertThat(ConfigSchemaWalker.toKebabCase("fanoutTTL")).isEqualTo("fanout-ttl")
    assertThat(ConfigSchemaWalker.toKebabCase("seenTTL")).isEqualTo("seen-ttl")
    assertThat(ConfigSchemaWalker.toKebabCase("httpTLSPort")).isEqualTo("http-tls-port")
    assertThat(ConfigSchemaWalker.toKebabCase("dLow")).isEqualTo("d-low")
  }

  @Test
  fun `classifies leaves with type, required and declared default`() {
    assertThat(keys().byPath("hostname").required).isTrue()
    keys().byPath("port").let {
      assertThat(it.type).isEqualTo("UInt")
      assertThat(it.required).isFalse()
      assertThat(it.default).isEqualTo("5432")
      assertThat(it.isSection).isFalse()
    }
    assertThat(keys().byPath("password").type).isEqualTo("Masked")
  }

  @Test
  fun `treats nullable URL as an optional leaf and maps as a single key`() {
    assertThat(keys().byPath("endpoint").type).isEqualTo("URL?")
    assertThat(keys().byPath("endpoint").required).isFalse()
    assertThat(keys().byPath("limits").type).isEqualTo("Map<Mode, UInt>")
    assertThat(keys().byPath("limits").isSection).isFalse()
  }

  @Test
  fun `recurses into sections and cascades section deprecation to nested leaves`() {
    assertThat(keys().byPath("retries").isSection).isTrue()
    assertThat(keys().byPath("retries.timeout").default).isEqualTo("PT1S")
    assertThat(keys().byPath("retries.timeout").deprecated).isFalse()

    val legacy = keys().byPath("legacy-retries")
    assertThat(legacy.isSection).isTrue()
    assertThat(legacy.deprecated).isTrue()
    assertThat(legacy.replacement).isEqualTo("retries")
    keys().byPath("legacy-retries.timeout").let {
      assertThat(it.deprecated).isTrue()
      assertThat(it.replacement).isEqualTo("retries")
    }
  }

  @Test
  fun `surfaces undocumented params with annotated=false and null doc fields`() {
    val undocumented = keys().byPath("undocumented")
    assertThat(undocumented.annotated).isFalse()
    assertThat(undocumented.description).isEmpty()
    assertThat(undocumented.default).isNull()
    assertThat(undocumented.example).isNull()
    assertThat(undocumented.replacement).isNull()
  }
}
