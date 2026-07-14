package linea.coordinator.config.v2.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

class ConfigDocValidatorTest {
  private fun leaf(
    path: String,
    annotated: Boolean = true,
    description: String = "A documented value.",
    deprecated: Boolean = false,
    replacement: String? = null,
    declaringClass: String = "SampleToml",
    propertyName: String = "sample",
  ) = ConfigKey(
    path = path,
    type = "String",
    required = false,
    default = null,
    description = if (annotated) description else "",
    example = null,
    deprecated = deprecated,
    replacement = replacement,
    isSection = false,
    annotated = annotated,
    declaringClass = declaringClass,
    propertyName = propertyName,
  )

  private fun section(
    path: String,
    annotated: Boolean = true,
    description: String = "A documented section.",
    deprecated: Boolean = false,
    replacement: String? = null,
  ) = leaf(
    path = path,
    annotated = annotated,
    description = description,
    deprecated = deprecated,
    replacement = replacement,
  ).copy(isSection = true, type = "NestedToml")

  @Test
  fun `passes when every key is documented`() {
    val keys = listOf(
      section("database"),
      leaf("database.hostname"),
      leaf("database.port"),
    )
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `flags a leaf missing ConfigDoc`() {
    val keys = listOf(leaf("database.connection-timeout", annotated = false, propertyName = "connectionTimeout"))

    val violations = ConfigDocValidator.validate(keys)

    assertThat(violations).singleElement().satisfies({
      assertThat(it.path).isEqualTo("database.connection-timeout")
      assertThat(it.location).isEqualTo("SampleToml.connectionTimeout")
      assertThat(it.message).contains("@ConfigDoc")
    })
  }

  @Test
  fun `flags a section missing ConfigSection with the section-specific hint`() {
    val violations = ConfigDocValidator.validate(listOf(section("database", annotated = false)))

    assertThat(violations).singleElement().satisfies({
      assertThat(it.message).contains("@ConfigSection")
    })
  }

  @Test
  fun `flags a blank description`() {
    val keys = listOf(leaf("database.hostname").copy(description = "   "))

    val violations = ConfigDocValidator.validate(keys)

    assertThat(violations).singleElement().satisfies({
      assertThat(it.message).contains("must not be blank")
    })
  }

  @Test
  fun `accepts a deprecated key without a replacement (plain removal)`() {
    val keys = listOf(leaf("database.legacy-host", deprecated = true, replacement = null))
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `accepts a deprecated key with a replacement`() {
    val keys = listOf(leaf("database.legacy-host", deprecated = true, replacement = "database.hostname"))
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `accepts a deprecated section without a replacement`() {
    val keys = listOf(section("legacy-anchoring", deprecated = true, replacement = null))
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `formatViolations lists violations sorted by path with guidance`() {
    val keys = listOf(
      leaf("zebra", annotated = false),
      leaf("alpha", annotated = false),
    )

    val report = ConfigDocValidator.formatViolations(ConfigDocValidator.validate(keys))

    assertThat(report).contains("Missing or invalid Coordinator config documentation:")
    assertThat(report.indexOf("alpha")).isLessThan(report.indexOf("zebra"))
    assertThat(report).contains("Add @ConfigDoc")
  }
}
