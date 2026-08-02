package linea.config.docs

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test

class ConfigDocsSpecLoadingTest {
  object SampleSpec : ConfigDocsSpec {
    override val files = emptyList<ConfigFileRoot>()
    override val sectionDetector = sectionByPackagePrefix("linea.config.docs")
    override val jsonSchemaPath = "build/schema.json"
    override val markdownPath = "build/reference.md"
    override val markdownTitle = "Sample Reference"
    override val regenerateCommand = "./gradlew generateConfigDocs"
  }

  @Test
  fun `loads a Kotlin object spec by fully-qualified name`() {
    val spec = loadSpec("linea.config.docs.ConfigDocsSpecLoadingTest\$SampleSpec")
    assertThat(spec.markdownTitle).isEqualTo("Sample Reference")
    assertThat(spec).isSameAs(SampleSpec)
  }

  @Test
  fun `fails clearly when the class does not implement ConfigDocsSpec`() {
    assertThatThrownBy { loadSpec("java.lang.String") }
      .hasMessageContaining("does not implement")
  }

  @Test
  fun `fails when the class cannot be found`() {
    assertThatThrownBy { loadSpec("linea.config.docs.NoSuchSpec") }
      .isInstanceOf(ClassNotFoundException::class.java)
  }
}
