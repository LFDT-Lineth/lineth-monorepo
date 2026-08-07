package lineth.config.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import kotlin.reflect.full.findAnnotation
import kotlin.reflect.full.primaryConstructor

class ConfigDocTest {
  private data class SampleToml(
    @param:ConfigDoc(description = "A documented leaf value.", default = "5432", example = "example-value")
    val leaf: String = "5432",

    @param:ConfigDoc(description = "A deprecated leaf value.", deprecated = true, replacement = "sample.new-leaf")
    val deprecatedLeaf: String? = null,

    @param:ConfigSection("A documented nested section.")
    val section: SampleToml? = null,

    @param:ConfigSection(description = "A deprecated section.", deprecated = true, replacement = "sample.new-section")
    val deprecatedSection: SampleToml? = null,

    val undocumented: String = "",
  )

  private val params = SampleToml::class.primaryConstructor!!.parameters.associateBy { it.name }

  @Test
  fun `ConfigDoc is runtime-readable with all fields`() {
    val doc = params.getValue("leaf").findAnnotation<ConfigDoc>()!!
    assertThat(doc.description).isEqualTo("A documented leaf value.")
    assertThat(doc.default).isEqualTo("5432")
    assertThat(doc.example).isEqualTo("example-value")
    assertThat(doc.deprecated).isFalse()
  }

  @Test
  fun `ConfigDoc carries deprecation metadata`() {
    val doc = params.getValue("deprecatedLeaf").findAnnotation<ConfigDoc>()!!
    assertThat(doc.deprecated).isTrue()
    assertThat(doc.replacement).isEqualTo("sample.new-leaf")
  }

  @Test
  fun `ConfigSection is runtime-readable and carries deprecation metadata`() {
    assertThat(params.getValue("section").findAnnotation<ConfigSection>()!!.description)
      .isEqualTo("A documented nested section.")

    val deprecated = params.getValue("deprecatedSection").findAnnotation<ConfigSection>()!!
    assertThat(deprecated.deprecated).isTrue()
    assertThat(deprecated.replacement).isEqualTo("sample.new-section")
  }

  @Test
  fun `parameters without an annotation report none`() {
    assertThat(params.getValue("undocumented").findAnnotation<ConfigDoc>()).isNull()
    assertThat(params.getValue("undocumented").findAnnotation<ConfigSection>()).isNull()
  }
}
