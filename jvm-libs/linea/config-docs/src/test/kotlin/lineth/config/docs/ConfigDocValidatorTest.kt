package lineth.config.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigDocValidatorTest {
  private fun key(
    path: String,
    annotated: Boolean = true,
    description: String = "A documented value.",
    isSection: Boolean = false,
    deprecated: Boolean = false,
    replacement: String? = null,
    propertyName: String = "sample",
  ) = ConfigKey(
    path = path,
    type = if (isSection) "NestedToml" else "String",
    required = false,
    default = null,
    description = if (annotated) description else "",
    example = null,
    deprecated = deprecated,
    replacement = replacement,
    isSection = isSection,
    annotated = annotated,
    declaringClass = "SampleToml",
    propertyName = propertyName,
  )

  @Test
  fun `passes when every key is documented`() {
    val keys = listOf(key("database", isSection = true), key("database.hostname"))
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `flags a leaf missing ConfigDoc`() {
    val violations = ConfigDocValidator.validate(
      listOf(key("database.connection-timeout", annotated = false, propertyName = "connectionTimeout")),
    )
    assertThat(violations).singleElement().satisfies({
      assertThat(it.location).isEqualTo("SampleToml.connectionTimeout")
      assertThat(it.message).contains("@ConfigDoc")
    })
  }

  @Test
  fun `flags a section missing ConfigSection`() {
    val violations = ConfigDocValidator.validate(listOf(key("database", annotated = false, isSection = true)))
    assertThat(violations).singleElement().satisfies({ assertThat(it.message).contains("@ConfigSection") })
  }

  @Test
  fun `flags a blank description`() {
    val violations = ConfigDocValidator.validate(listOf(key("database.hostname").copy(description = "  ")))
    assertThat(violations).singleElement().satisfies({ assertThat(it.message).contains("must not be blank") })
  }

  @Test
  fun `accepts deprecated keys and sections without a replacement`() {
    val keys = listOf(
      key("database.legacy-host", deprecated = true, replacement = null),
      key("legacy-anchoring", isSection = true, deprecated = true, replacement = null),
    )
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `formatViolations lists violations sorted by path with guidance`() {
    val report = ConfigDocValidator.formatViolations(
      ConfigDocValidator.validate(listOf(key("zebra", annotated = false), key("alpha", annotated = false))),
    )
    assertThat(report).contains("Missing or invalid config documentation:")
    assertThat(report.indexOf("alpha")).isLessThan(report.indexOf("zebra"))
    assertThat(report).contains("Add @ConfigDoc")
  }
}
