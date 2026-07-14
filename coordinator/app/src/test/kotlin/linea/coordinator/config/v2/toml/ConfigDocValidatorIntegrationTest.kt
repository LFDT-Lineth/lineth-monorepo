package linea.coordinator.config.v2.toml

import linea.coordinator.config.v2.docs.ConfigDocValidator
import linea.coordinator.config.v2.docs.ConfigSchemaWalker
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test

/**
 * Exercises the walker + validator together. Lives in the `...config.v2.toml` package so the
 * nested sample classes are recognised as sections by [ConfigSchemaWalker].
 */
class ConfigDocValidatorIntegrationTest {
  data class FullyDocumentedToml(
    @param:ConfigDoc("The hostname.", example = "localhost")
    val hostname: String,
    @param:ConfigDoc("The port.", default = "5432")
    val port: UInt = 5432u,
    @param:ConfigSection("Nested retry settings.")
    val retries: NestedToml = NestedToml(),
  ) {
    data class NestedToml(
      @param:ConfigDoc("Max attempts.", default = "3")
      val maxAttempts: UInt = 3u,
    )
  }

  data class PartlyDocumentedToml(
    @param:ConfigDoc("The hostname.")
    val hostname: String,
    // missing @ConfigDoc
    val port: UInt = 5432u,
    // section missing @ConfigSection, and its leaf is undocumented too
    val retries: NestedToml = NestedToml(),
  ) {
    data class NestedToml(
      val maxAttempts: UInt = 3u,
    )
  }

  @Test
  fun `a fully documented tree produces no violations`() {
    val keys = ConfigSchemaWalker.walk(FullyDocumentedToml::class)
    assertThat(ConfigDocValidator.validate(keys)).isEmpty()
  }

  @Test
  fun `undocumented leaves and sections are all reported`() {
    val keys = ConfigSchemaWalker.walk(PartlyDocumentedToml::class)

    val violations = ConfigDocValidator.validate(keys)

    assertThat(violations.map { it.path }).containsExactlyInAnyOrder(
      "port",
      "retries",
      "retries.max-attempts",
    )
  }
}
