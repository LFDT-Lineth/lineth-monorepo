package linea.coordinator.config.v2.toml

/**
 * Documents a leaf Coordinator config value that can appear directly in TOML.
 *
 * Use on scalar values, enums, lists, maps, durations, URLs, paths, and other leaf types,
 * i.e. constructor parameters that are not themselves config data classes. Nested config
 * data classes should be annotated with [ConfigSection] instead.
 *
 * Must be applied with the `@param:` use-site target so the annotation is retained on the
 * constructor's value parameter and readable via Kotlin reflection
 * (`KParameter.annotations`), e.g.:
 *
 * ```
 * data class DatabaseToml(
 *   @param:ConfigDoc("PostgreSQL hostname used by the coordinator persistence layer.", example = "postgres")
 *   val hostname: String,
 *   @param:ConfigDoc("PostgreSQL port.", default = "5432")
 *   val port: UInt = 5432u,
 * )
 * ```
 *
 * The [default] is author-declared rather than derived by reflection. Kotlin compiles
 * constructor default values into a synthetic method rather than storing them as retrievable
 * metadata, so they cannot be read without instantiating the class; instantiation is both
 * fragile (cross-field `init {}` guards) and subtly wrong for reused nested classes whose
 * defaults are overridden at the call site. Declaring the default here is 100% reliable.
 *
 * JVM annotation members cannot be nullable, so the optional string fields use `""` as an
 * "unset" sentinel; consumers work with the nullable model in `ConfigKey`, where `""` maps to
 * `null`. A config value whose actual default is an empty string is declared explicitly with a
 * rendered form such as `default = "\"\""`, since [default] is a display string, not the value.
 *
 * @property description Required human-readable explanation of what the key controls.
 * @property default Rendered default value for optional keys, shown in the generated docs.
 *   Leave empty for required keys (which have no default).
 * @property example Optional example value rendered in the generated docs.
 * @property deprecated Marks a config key as deprecated while keeping it documented.
 * @property replacement Optional replacement key path shown in the generated docs and changelog;
 *   set when [deprecated] is true and a replacement exists.
 */
@Target(AnnotationTarget.VALUE_PARAMETER)
@Retention(AnnotationRetention.RUNTIME)
annotation class ConfigDoc(
  val description: String,
  val default: String = "",
  val example: String = "",
  val deprecated: Boolean = false,
  val replacement: String = "",
)

/**
 * Documents a nested Coordinator config TOML table.
 *
 * Use when a constructor parameter is itself another config data class rather than a direct
 * value. Leaf values within that nested class should be annotated with [ConfigDoc].
 *
 * Must be applied with the `@param:` use-site target, e.g.:
 *
 * ```
 * data class CoordinatorConfigFileToml(
 *   @param:ConfigSection("Coordinator PostgreSQL persistence settings.")
 *   val database: DatabaseToml,
 * )
 * ```
 *
 * @property description Required human-readable explanation of what the section configures.
 */
@Target(AnnotationTarget.VALUE_PARAMETER)
@Retention(AnnotationRetention.RUNTIME)
annotation class ConfigSection(
  val description: String,
)
