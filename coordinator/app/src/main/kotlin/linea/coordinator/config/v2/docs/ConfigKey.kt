package linea.coordinator.config.v2.docs

/**
 * A single entry produced by [ConfigSchemaWalker] for one constructor parameter of a TOML
 * schema class. Both leaf values and nested sections are represented; [isSection]
 * distinguishes them.
 *
 * @property path kebab-case dotted config path, e.g. `database.persistence-retries.max-retries`.
 * @property type rendered Kotlin type, e.g. `UInt?`, `Duration`, `Map<TracingModuleV4, UInt>`.
 * @property required true when the parameter has no default and is non-nullable.
 * @property default author-declared rendered default value from `@ConfigDoc.default`, or null
 *   when none was declared (required parameters, or optionals missing a declared default).
 * @property description from `@ConfigDoc`/`@ConfigSection`, or empty when not annotated.
 * @property example from `@ConfigDoc`, or null when unset.
 * @property deprecated from `@ConfigDoc`.
 * @property replacement replacement key path from `@ConfigDoc`, or null when unset.
 * @property isSection true when the parameter is a nested config table rather than a leaf value.
 * @property annotated true when the relevant annotation (`@ConfigDoc` for leaves,
 *   `@ConfigSection` for sections) is present. Used by the documentation-completeness check.
 * @property declaringClass simple name of the data class declaring the parameter.
 * @property propertyName the original camelCase Kotlin parameter name.
 */
data class ConfigKey(
  val path: String,
  val type: String,
  val required: Boolean,
  val default: String?,
  val description: String,
  val example: String?,
  val deprecated: Boolean,
  val replacement: String?,
  val isSection: Boolean,
  val annotated: Boolean,
  val declaringClass: String,
  val propertyName: String,
)
