package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.docs.ConfigKey
import linea.coordinator.config.v2.docs.ConfigSchemaWalker
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

/**
 * Lives in the `...config.v2.toml` package so the sample classes are recognised as config
 * sections by [ConfigSchemaWalker] (which keys section detection on the config package).
 */
class ConfigSchemaWalkerAnnotationTest {
  data class SampleConfigToml(
    @param:ConfigDoc(
      description = "A documented leaf value.",
      default = "5432",
      example = "example-value",
    )
    val documentedLeaf: String = "5432",

    @param:ConfigDoc(
      description = "A deprecated leaf value.",
      deprecated = true,
      replacement = "sample.replacement-leaf",
    )
    val deprecatedLeaf: String? = null,

    val undocumentedLeaf: Int = 0,

    @param:ConfigSection("A documented nested section.")
    val section: NestedToml = NestedToml(),
  ) {
    data class NestedToml(
      @param:ConfigDoc("A leaf inside the section.")
      val nestedLeaf: String = "default",
    )
  }

  private fun List<ConfigKey>.byPath(path: String): ConfigKey = single { it.path == path }

  @Test
  fun `reads ConfigDoc metadata onto leaf keys`() {
    val keys = ConfigSchemaWalker.walk(SampleConfigToml::class)

    val leaf = keys.byPath("documented-leaf")
    assertThat(leaf.annotated).isTrue()
    assertThat(leaf.description).isEqualTo("A documented leaf value.")
    assertThat(leaf.default).isEqualTo("5432")
    assertThat(leaf.example).isEqualTo("example-value")
    assertThat(leaf.deprecated).isFalse()
    // unset annotation strings map to null in the model, not empty string
    assertThat(leaf.replacement).isNull()
  }

  @Test
  fun `reads deprecation metadata`() {
    val deprecated = ConfigSchemaWalker.walk(SampleConfigToml::class).byPath("deprecated-leaf")
    assertThat(deprecated.deprecated).isTrue()
    assertThat(deprecated.replacement).isEqualTo("sample.replacement-leaf")
    // no declared default/example => null in the model
    assertThat(deprecated.default).isNull()
    assertThat(deprecated.example).isNull()
  }

  @Test
  fun `marks unannotated leaves so the completeness check can flag them`() {
    val undocumented = ConfigSchemaWalker.walk(SampleConfigToml::class).byPath("undocumented-leaf")
    assertThat(undocumented.annotated).isFalse()
    assertThat(undocumented.description).isEmpty()
    assertThat(undocumented.default).isNull()
  }

  @Test
  fun `reads ConfigSection metadata and recurses`() {
    val keys = ConfigSchemaWalker.walk(SampleConfigToml::class)

    val section = keys.byPath("section")
    assertThat(section.isSection).isTrue()
    assertThat(section.annotated).isTrue()
    assertThat(section.description).isEqualTo("A documented nested section.")

    val nested = keys.byPath("section.nested-leaf")
    assertThat(nested.isSection).isFalse()
    assertThat(nested.annotated).isTrue()
    assertThat(nested.description).isEqualTo("A leaf inside the section.")
  }
}
