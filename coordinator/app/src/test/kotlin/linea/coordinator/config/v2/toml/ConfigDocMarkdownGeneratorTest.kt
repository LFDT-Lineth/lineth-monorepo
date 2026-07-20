package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.docs.ConfigDocMarkdownGenerator
import linea.coordinator.config.v2.docs.CoordinatorConfigFileRoot
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigDocMarkdownGeneratorTest {
  data class SampleFileToml(
    @param:ConfigSection("Database settings.")
    val database: DatabaseSectionToml = DatabaseSectionToml(),
    @param:ConfigDoc(
      description = "Legacy blob gas setting.",
      deprecated = true,
      replacement = "database.hostname",
    )
    val legacyBlobGas: ULong? = null,
  ) {
    data class DatabaseSectionToml(
      @param:ConfigDoc("The hostname.", example = "localhost")
      val hostname: String = "localhost",
      @param:ConfigDoc("The port.", default = "5432")
      val port: UInt = 5432u,
    )
  }

  private val files = listOf(CoordinatorConfigFileRoot("sample", "A sample file.", SampleFileToml::class))

  @Test
  fun `renders a file heading, section subheading and a leaf table`() {
    val md = ConfigDocMarkdownGenerator.generate(files)

    assertThat(md).contains("# Coordinator Configuration Reference")
    assertThat(md).contains("Do not edit by hand")
    assertThat(md).contains("## sample")
    assertThat(md).contains("### `database`")
    assertThat(md).contains("| Key | Type | Required | Default | Status | Description |")
    assertThat(md).contains("| `database.port` | `UInt` | no | `5432` | active | The port. |")
  }

  @Test
  fun `appends the example into the description cell`() {
    val md = ConfigDocMarkdownGenerator.generate(files)
    assertThat(md).contains("The hostname. Example: `localhost`.")
  }

  @Test
  fun `lists deprecated keys in the Deprecated Keys table`() {
    val md = ConfigDocMarkdownGenerator.generate(files)

    assertThat(md).contains("## Deprecated Keys")
    assertThat(md).contains("| File | Key | Replacement | Description |")
    assertThat(md).contains("| sample | `legacy-blob-gas` | `database.hostname` | Legacy blob gas setting. |")
  }

  @Test
  fun `says None when nothing is deprecated`() {
    val noDeprecations = listOf(
      CoordinatorConfigFileRoot("sample", "A sample file.", SampleFileToml.DatabaseSectionToml::class),
    )
    val md = ConfigDocMarkdownGenerator.generate(noDeprecations)
    assertThat(md).contains("## Deprecated Keys\n\nNone.")
  }
}
