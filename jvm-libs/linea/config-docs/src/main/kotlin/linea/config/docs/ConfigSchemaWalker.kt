package linea.config.docs

import com.sksamuel.hoplite.KebabCaseParamMapper
import kotlin.reflect.KClass
import kotlin.reflect.KFunction
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
 * Config keys are rendered with Hoplite's [KebabCaseParamMapper] so the documented path matches
 * how Hoplite maps property names when parsing the TOML.
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
  ) {
    val constructor = kClass.primaryConstructor ?: return
    val declaringClass = kClass.simpleName ?: kClass.qualifiedName ?: "?"

    for (parameter in constructor.parameters) {
      val propertyName = parameter.name ?: continue
      val path = prefix + kebabKey(parameter, constructor, kClass)
      val type = parameter.type
      val required = !parameter.isOptional && !type.isMarkedNullable
      val sectionClass = (type.classifier as? KClass<*>)?.takeIf { sectionDetector.isSection(it) }

      if (sectionClass != null) {
        val sectionAnnotation = parameter.findAnnotation<ConfigSection>()
        sink.add(
          ConfigKey(
            path = path,
            type = renderType(type),
            required = required,
            default = null,
            description = sectionAnnotation?.description ?: "",
            example = null,
            deprecated = sectionAnnotation?.deprecated ?: false,
            replacement = sectionAnnotation?.replacement.orNull(),
            isSection = true,
            annotated = sectionAnnotation != null,
            declaringClass = declaringClass,
            propertyName = propertyName,
          ),
        )
        if (sectionClass !in onStack) {
          walkClass(sectionClass, "$path.", onStack + kClass, sectionDetector, sink)
        }
      } else {
        val doc = parameter.findAnnotation<ConfigDoc>()
        sink.add(
          ConfigKey(
            path = path,
            type = renderType(type),
            required = required,
            default = doc?.default.orNull(),
            description = doc?.description ?: "",
            example = doc?.example.orNull(),
            deprecated = doc?.deprecated ?: false,
            replacement = doc?.replacement.orNull(),
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
   * Renders the kebab-case config key for a parameter using Hoplite's own
   * [KebabCaseParamMapper], so the documented key is guaranteed consistent with how Hoplite maps
   * property names when parsing the TOML. The mapper returns candidate lookup names (it adds a
   * trailing-digit variant such as `foo-2` alongside `foo2`); the first is the plain kebab form,
   * which is the natural one to display.
   */
  private fun kebabKey(parameter: KParameter, constructor: KFunction<*>, kClass: KClass<*>): String {
    @Suppress("UNCHECKED_CAST")
    return KebabCaseParamMapper.map(parameter, constructor as KFunction<Any>, kClass).first()
  }

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
