package linea.config.docs

import com.fasterxml.jackson.databind.ObjectMapper
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigDocGeneratorsTest {
  data class SampleFileToml(
    @param:ConfigDoc("The hostname.", example = "localhost")
    val hostname: String,
    @param:ConfigDoc("The port.", default = "5432")
    val port: UInt = 5432u,
    @param:ConfigSection("Nested settings.")
    val nested: NestedToml = NestedToml(),
    @param:ConfigDoc(description = "Legacy gas.", deprecated = true, replacement = "port")
    val legacyGas: ULong? = null,
  ) {
    data class NestedToml(
      @param:ConfigDoc("Max attempts.", default = "3")
      val maxAttempts: UInt = 3u,
    )
  }

  private val detector = sectionByPackagePrefix("linea.config.docs")
  private val files = listOf(ConfigFileRoot("sample", "A sample file.", SampleFileToml::class))
  private val mapper = ObjectMapper()

  @Test
  fun `json is organised per file then per sorted key with the section flag`() {
    val tree = mapper.readTree(ConfigDocJsonGenerator.generate(files, detector))

    assertThat(tree.path("sample").path("description").asText()).isEqualTo("A sample file.")
    val keyNames = tree.at("/sample/keys").fieldNames().asSequence().toList()
    assertThat(keyNames).containsExactly("hostname", "legacy-gas", "nested", "nested.max-attempts", "port")
    assertThat(tree.at("/sample/keys/nested/section").asBoolean()).isTrue()
    assertThat(tree.at("/sample/keys/port/section").asBoolean()).isFalse()
  }

  @Test
  fun `json serialises declared default and null for unset`() {
    val port = mapper.readTree(ConfigDocJsonGenerator.generate(files, detector)).at("/sample/keys/port")
    assertThat(port.path("default").asText()).isEqualTo("5432")
    assertThat(port.path("example").isNull).isTrue()
    assertThat(ConfigDocJsonGenerator.generate(files, detector)).endsWith("\n")
  }

  @Test
  fun `markdown renders headings, a leaf table with inlined example, and a deprecated table`() {
    val md = ConfigDocMarkdownGenerator.generate(files, detector, "Sample Reference", "./gradlew generateDocs")

    assertThat(md).contains("# Sample Reference")
    assertThat(md).contains("Do not edit by hand")
    assertThat(md).contains("## sample")
    assertThat(md).contains("### `nested`")
    assertThat(md).contains("| `nested.max-attempts` | `UInt` | no | `3` | active | Max attempts. |")
    assertThat(md).contains("The hostname. Example: `localhost`.")
    assertThat(md).contains("| File | Key | Replacement | Description |")
    assertThat(md).contains("| sample | `legacy-gas` | `port` | Legacy gas. |")
  }

  @Test
  fun `markdown says None when nothing is deprecated`() {
    val files = listOf(ConfigFileRoot("sample", "A sample file.", SampleFileToml.NestedToml::class))
    val md = ConfigDocMarkdownGenerator.generate(files, detector, "T", "cmd")
    assertThat(md).contains("## Deprecated Keys\n\nNone.")
  }
}
