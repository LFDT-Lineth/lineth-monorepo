package lineth.config.docs

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
    // consecutive capitals: Hoplite treats each uppercase as a boundary
    val httpTLSPort: Int = 0,
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
  fun `renders kebab keys via Hoplite and sorts by path`() {
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
      "http-t-l-s-port",
    )
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
  fun `recurses into sections and carries section deprecation without cascading`() {
    assertThat(keys().byPath("retries").isSection).isTrue()
    assertThat(keys().byPath("retries.timeout").default).isEqualTo("PT1S")

    val legacy = keys().byPath("legacy-retries")
    assertThat(legacy.isSection).isTrue()
    assertThat(legacy.deprecated).isTrue()
    assertThat(legacy.replacement).isEqualTo("retries")
    assertThat(keys().byPath("legacy-retries.timeout").deprecated).isFalse()
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
