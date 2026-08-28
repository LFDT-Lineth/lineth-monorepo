package linea.config.docs

import kotlin.reflect.KClass
import kotlin.reflect.KParameter
import kotlin.reflect.KType
import kotlin.reflect.full.findAnnotation
import kotlin.reflect.full.primaryConstructor

/**
 * Decides whether a config parameter's type is a nested *section* (a config table to recurse
 * into) rather than a leaf value. Applications supply their own detector because section classes
 * live in application-specific packages; see [sectionByPackagePrefix] for the common case.
 */
fun interface SectionDetector {
  fun isSection(kClass: KClass<*>): Boolean
}

/**
 * A [SectionDetector] that treats a Kotlin data class as a section when its qualified name starts
 * with any of the given [packagePrefixes]. This scopes recursion to the application's own config
 * classes and excludes library types (`Masked`, `Duration`, `BlockParameter`, maps, ...).
 */
fun sectionByPackagePrefix(vararg packagePrefixes: String): SectionDetector = SectionDetector { kClass ->
  kClass.isData && packagePrefixes.any { kClass.qualifiedName?.startsWith(it) == true }
}

/**
 * Walks the primary constructors of Hoplite TOML schema classes via reflection and produces a
 * flat, path-sorted list of [ConfigKey]s describing every config key.
 *
 * The walk is purely structural — it does not depend on the documentation annotations being
 * present — so undocumented parameters still appear in the output and can be reported by the
 * documentation-completeness check. Whether a parameter is a nested section (and recursed into)
 * is decided by the supplied [SectionDetector]; everything else (scalars, enums, durations, URLs,
 * `Masked`, `BlockParameter`, lists, maps, ...) is a leaf. Dynamic maps are documented as a single
 * key rather than enumerating their entries.
 *
 * Config keys are rendered with an acronym-aware camelCase→kebab-case converter so idiomatic
 * keys such as `fanoutTTL` → `fanout-ttl` match real TOML usage. Hoplite's runtime
 * `PathNormalizer` accepts both the idiomatic form and the naive per-capital form.
 *
 * When a `@ConfigSection` is marked deprecated, that status (and its replacement, if any)
 * cascades to nested keys so leaf Status columns stay accurate for shared nested DTOs.
 */
object ConfigSchemaWalker {
  /**
   * Display names for types whose reflected name is unhelpful. `Masked` is a typealias for
   * `com.sksamuel.hoplite.Secret`, and typealiases are erased in reflection, so the underlying
   * class surfaces; the config classes and docs use `Masked`.
   */
  private val TYPE_DISPLAY_NAMES = mapOf(
    "com.sksamuel.hoplite.Secret" to "Masked",
  )

  /** Walks a single root config class, returning its keys sorted by path. */
  fun walk(rootClass: KClass<*>, sectionDetector: SectionDetector): List<ConfigKey> {
    val keys = mutableListOf<ConfigKey>()
    walkClass(rootClass, prefix = "", onStack = emptySet(), sectionDetector = sectionDetector, sink = keys)
    return keys.sortedBy { it.path }
  }

  /** Walks several root config classes and merges their keys, sorted by path. */
  fun walkAll(rootClasses: List<KClass<*>>, sectionDetector: SectionDetector): List<ConfigKey> {
    val keys = mutableListOf<ConfigKey>()
    for (rootClass in rootClasses) {
      walkClass(rootClass, prefix = "", onStack = emptySet(), sectionDetector = sectionDetector, sink = keys)
    }
    return keys.sortedBy { it.path }
  }

  private fun walkClass(
    kClass: KClass<*>,
    prefix: String,
    onStack: Set<KClass<*>>,
    sectionDetector: SectionDetector,
    sink: MutableList<ConfigKey>,
    parentDeprecated: Boolean = false,
    parentReplacement: String? = null,
  ) {
    val constructor = kClass.primaryConstructor ?: return
    val declaringClass = kClass.simpleName ?: kClass.qualifiedName ?: "?"

    for (parameter in constructor.parameters) {
      val propertyName = parameter.name ?: continue
      val path = prefix + kebabKey(parameter)
      val type = parameter.type
      val required = !parameter.isOptional && !type.isMarkedNullable
      val sectionClass = (type.classifier as? KClass<*>)?.takeIf { sectionDetector.isSection(it) }

      if (sectionClass != null) {
        val sectionAnnotation = parameter.findAnnotation<ConfigSection>()
        val deprecated = (sectionAnnotation?.deprecated ?: false) || parentDeprecated
        val replacement = sectionAnnotation?.replacement.orNull() ?: parentReplacement
        sink.add(
          ConfigKey(
            path = path,
            type = renderType(type),
            required = required,
            default = null,
            description = sectionAnnotation?.description ?: "",
            example = null,
            deprecated = deprecated,
            replacement = replacement,
            isSection = true,
            annotated = sectionAnnotation != null,
            declaringClass = declaringClass,
            propertyName = propertyName,
          ),
        )
        if (sectionClass !in onStack) {
          walkClass(
            kClass = sectionClass,
            prefix = "$path.",
            onStack = onStack + kClass,
            sectionDetector = sectionDetector,
            sink = sink,
            parentDeprecated = deprecated,
            parentReplacement = replacement,
          )
        }
      } else {
        val doc = parameter.findAnnotation<ConfigDoc>()
        val deprecated = (doc?.deprecated ?: false) || parentDeprecated
        val replacement = doc?.replacement.orNull() ?: parentReplacement
        sink.add(
          ConfigKey(
            path = path,
            type = renderType(type),
            required = required,
            default = doc?.default.orNull(),
            description = doc?.description ?: "",
            example = doc?.example.orNull(),
            deprecated = deprecated,
            replacement = replacement,
            isSection = false,
            annotated = doc != null,
            declaringClass = declaringClass,
            propertyName = propertyName,
          ),
        )
      }
    }
  }

  /**
   * Renders an acronym-aware kebab-case config key for a parameter name.
   *
   * Examples: `fanoutTTL` → `fanout-ttl`, `httpTLSPort` → `http-tls-port`, `dLow` → `d-low`.
   * Prefer this over Hoplite's [com.sksamuel.hoplite.KebabCaseParamMapper], which inserts a hyphen
   * before every capital (`fanoutTTL` → `fanout-t-t-l`) and does not match idiomatic TOML keys.
   */
  private fun kebabKey(parameter: KParameter): String {
    val name = parameter.name ?: return ""
    return toKebabCase(name)
  }

  /** Acronym-aware camelCase → kebab-case. Visible for tests. */
  internal fun toKebabCase(name: String): String =
    name
      .replace(Regex("([A-Z]+)([A-Z][a-z])"), "$1-$2")
      .replace(Regex("([a-z0-9])([A-Z])"), "$1-$2")
      .lowercase()

  /** Maps the annotation's `""` "unset" sentinel (and null) to null for the ConfigKey model. */
  private fun String?.orNull(): String? = this?.ifEmpty { null }

  /** Renders a [KType] as a readable string such as `UInt?` or `Map<TracingModuleV4, UInt>`. */
  internal fun renderType(type: KType): String {
    val classifier = type.classifier
    val base = when (classifier) {
      is KClass<*> -> TYPE_DISPLAY_NAMES[classifier.qualifiedName]
        ?: classifier.simpleName ?: classifier.qualifiedName ?: "?"
      else -> classifier?.toString() ?: "?"
    }
    val rendered = if (type.arguments.isEmpty()) {
      base
    } else {
      base + type.arguments.joinToString(prefix = "<", postfix = ">") { argument ->
        argument.type?.let { renderType(it) } ?: "*"
      }
    }
    return if (type.isMarkedNullable) "$rendered?" else rendered
  }
}
