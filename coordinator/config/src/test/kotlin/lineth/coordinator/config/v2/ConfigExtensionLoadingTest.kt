package lineth.coordinator.config.v2

import lineth.coordinator.config.v2.toml.ConfigExtension
import lineth.coordinator.config.v2.toml.loadConfigs
import lineth.coordinator.config.v2.toml.loadExtensionConfigs
import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path

class ConfigExtensionLoadingTest {
  @TempDir
  lateinit var tempDir: Path

  data class FooSectionToml(val enabled: Boolean, val endpoint: String = "http://localhost")

  data class FooExtensionToml(val foo: FooSectionToml)

  object FooExtension : ConfigExtension<FooExtensionToml> {
    override val topLevelKeys = setOf("foo")

    override fun load(configFiles: List<Path>, coordinatorConfig: CoordinatorConfig): FooExtensionToml =
      loadExtensionConfigs<FooExtensionToml>(configFiles, topLevelKeys, enforceStrict = true)

    override fun toPrettyLog(config: FooExtensionToml): String = config.toString()
  }

  private val baseConfig = Path.of("../../docker/config/coordinator/coordinator-config-v2.toml")
  private val localDevOverride = Path.of(
    "../../docker/config/coordinator/coordinator-config-v2-override-local-dev.toml",
  )

  private fun override(toml: String): Path = Files.writeString(Files.createTempFile(tempDir, "override", ".toml"), toml)

  private fun load(
    vararg overrides: Path,
    ignoredTopLevelKeys: Set<String> = emptySet(),
    enforceStrict: Boolean = true,
  ): CoordinatorConfig =
    loadConfigs(
      coordinatorConfigFiles = listOf(baseConfig, localDevOverride) + overrides,
      tracesLimitsFileV4 = Path.of("../../docker/config/common/traces-limits-v4.4.toml"),
      tracesLimitsFileV5 = Path.of("../../docker/config/common/traces-limits-v5.toml"),
      gasPriceCapTimeOfDayMultipliersFile = Path.of(
        "../../docker/config/common/gas-price-cap-time-of-day-multipliers.toml",
      ),
      enforceStrict = enforceStrict,
      ignoredTopLevelKeys = ignoredTopLevelKeys,
    )

  @Test
  fun `extension sections are ignored by the strict coordinator pass and loaded from the same files`() {
    val extraFile = override("[foo]\nenabled = true\n")
    val layered = override("[foo]\nendpoint = \"http://foo:8080\"\n")

    val configs = load(extraFile, layered, ignoredTopLevelKeys = FooExtension.topLevelKeys)
    val extension = FooExtension.load(listOf(baseConfig, localDevOverride, extraFile, layered), configs)

    assertThat(extension).isEqualTo(FooExtensionToml(FooSectionToml(enabled = true, endpoint = "http://foo:8080")))
  }

  @Test
  fun `extension sections are reported as unknown keys when not ignored`() {
    val extraFile = override("[foo]\nenabled = true\n")

    assertThatThrownBy { load(extraFile) }.hasMessageContaining("foo")
  }

  @Test
  fun `unknown keys outside the extension sections still fail strict and pass lenient`() {
    val extraFile = override("[foo]\nenabled = true\n[bar]\nx = 1\n")

    assertThatThrownBy { load(extraFile, ignoredTopLevelKeys = setOf("foo")) }.hasMessageContaining("bar")
    assertThat(load(extraFile, ignoredTopLevelKeys = setOf("foo"), enforceStrict = false).database.host)
      .isEqualTo("127.0.0.1")
  }

  @Test
  fun `extension loading is lenient on unknown keys and fails on invalid values`() {
    val files = listOf(baseConfig, override("[foo]\nenabled = true\nunknown = 1\n"))
    assertThat(
      loadExtensionConfigs<FooExtensionToml>(files, setOf("foo")).foo.enabled,
    ).isTrue()
    assertThatThrownBy {
      loadExtensionConfigs<FooExtensionToml>(files, setOf("foo"), enforceStrict = true)
    }.hasMessageContaining("unknown")

    val invalid = listOf(baseConfig, override("[foo]\nenabled = \"not-a-boolean\"\n"))
    assertThatThrownBy { loadExtensionConfigs<FooExtensionToml>(invalid, setOf("foo")) }
      .isInstanceOf(RuntimeException::class.java)
  }

  @Test
  fun `extension keys colliding with coordinator sections fail fast`() {
    assertThatThrownBy { load(ignoredTopLevelKeys = setOf("l1-submission")) }
      .isInstanceOf(IllegalArgumentException::class.java)
      .hasMessageContaining("l1-submission")
    assertThatThrownBy { load(ignoredTopLevelKeys = setOf("Database")) }
      .isInstanceOf(IllegalArgumentException::class.java)
  }

  @Test
  fun `deprecated database schema key is applied to database-name`() {
    val configs = load(override("[database]\nschema = \"legacy_db\"\n"))

    assertThat(configs.database.databaseName).isEqualTo("legacy_db")
  }

  @Test
  fun `deprecated key from a lower priority file does not override the new key of a higher one`() {
    val old = override("[database]\nschema = \"legacy_db\"\n")
    val new = override("[database]\ndatabase-name = \"new_db\"\n")

    assertThat(load(old, new).database.databaseName).isEqualTo("new_db")
    assertThat(load(new, old).database.databaseName).isEqualTo("legacy_db")
  }

  @Test
  fun `setting both the deprecated and the new key in a file fails`() {
    val both = override("[database]\nschema = \"a\"\ndatabase-name = \"b\"\n")

    assertThatThrownBy { load(both, enforceStrict = false) }
      .hasMessageContaining("database.schema")
      .hasMessageContaining("database.database-name")
  }
}
