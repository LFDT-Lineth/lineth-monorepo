package lineth.config.docs

/**
 * A single documentation-completeness problem found by [ConfigDocValidator].
 *
 * @property path the kebab-case config path, e.g. `database.connection-timeout`.
 * @property location the declaring `Class.property`, e.g. `DatabaseToml.connectionTimeout`.
 * @property message what is wrong and how to fix it.
 */
data class ConfigDocViolation(
  val path: String,
  val location: String,
  val message: String,
)

/**
 * Validates that every config key produced by [ConfigSchemaWalker] is documented.
 *
 * Rules:
 * - every leaf must carry `@ConfigDoc` and every section must carry `@ConfigSection`;
 * - the description must be non-blank.
 *
 * A `replacement` is optional, even for a deprecated key/section — deprecation without a
 * replacement (a plain removal) is valid.
 *
 * The validator is pure (operates on the walked [ConfigKey] list), so it is trivially testable
 * and reused by both the doc-check runner and its unit tests.
 */
object ConfigDocValidator {
  fun validate(keys: List<ConfigKey>): List<ConfigDocViolation> {
    val violations = mutableListOf<ConfigDocViolation>()
    for (key in keys) {
      val location = "${key.declaringClass}.${key.propertyName}"
      val annotation = if (key.isSection) "@ConfigSection" else "@ConfigDoc"

      if (!key.annotated) {
        violations.add(
          ConfigDocViolation(key.path, location, "missing $annotation on the constructor parameter"),
        )
        continue
      }

      if (key.description.isBlank()) {
        violations.add(
          ConfigDocViolation(key.path, location, "$annotation.description must not be blank"),
        )
      }
    }
    return violations
  }

  /** Renders violations as a human-readable, deterministically ordered report. */
  fun formatViolations(violations: List<ConfigDocViolation>): String {
    val sorted = violations.sortedWith(compareBy({ it.path }, { it.message }))
    return buildString {
      appendLine("Missing or invalid config documentation:")
      appendLine()
      for (violation in sorted) {
        appendLine("- ${violation.path} in ${violation.location}: ${violation.message}")
      }
      appendLine()
      appendLine(
        "Every config key must be documented. Add @ConfigDoc (leaf value) or @ConfigSection " +
          "(nested table) to the constructor parameter with a non-blank description.",
      )
    }
  }
}
