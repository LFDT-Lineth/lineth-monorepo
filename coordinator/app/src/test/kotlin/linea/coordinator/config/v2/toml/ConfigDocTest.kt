package linea.coordinator.config.v2.toml

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.reflect.full.findAnnotation
import kotlin.reflect.full.primaryConstructor

class ConfigDocTest {
  private data class SampleToml(
    @param:ConfigDoc(
      description = "A documented leaf value.",
      example = "example-value",
    )
    val leaf: String,

    @param:ConfigDoc(
      description = "A deprecated leaf value.",
      deprecated = true,
      replacement = "sample.new-leaf",
    )
    val deprecatedLeaf: String? = null,

    @param:ConfigSection("A documented nested section.")
    val section: SampleToml? = null,

    val undocumented: String = "",
  )

  private val params = SampleToml::class.primaryConstructor!!.parameters.associateBy { it.name }

  @Test
  fun `ConfigDoc is retained at runtime and readable from the constructor value parameter`() {
    val doc = params.getValue("leaf").findAnnotation<ConfigDoc>()
    assertThat(doc).isNotNull
    assertThat(doc!!.description).isEqualTo("A documented leaf value.")
    assertThat(doc.example).isEqualTo("example-value")
    assertThat(doc.deprecated).isFalse()
    assertThat(doc.replacement).isEmpty()
  }

  @Test
  fun `ConfigDoc carries deprecation metadata`() {
    val doc = params.getValue("deprecatedLeaf").findAnnotation<ConfigDoc>()
    assertThat(doc).isNotNull
    assertThat(doc!!.deprecated).isTrue()
    assertThat(doc.replacement).isEqualTo("sample.new-leaf")
  }

  @Test
  fun `ConfigSection is retained at runtime and readable from the constructor value parameter`() {
    val section = params.getValue("section").findAnnotation<ConfigSection>()
    assertThat(section).isNotNull
    assertThat(section!!.description).isEqualTo("A documented nested section.")
  }

  @Test
  fun `parameters without an annotation report none`() {
    assertThat(params.getValue("undocumented").findAnnotation<ConfigDoc>()).isNull()
    assertThat(params.getValue("undocumented").findAnnotation<ConfigSection>()).isNull()
  }
}
