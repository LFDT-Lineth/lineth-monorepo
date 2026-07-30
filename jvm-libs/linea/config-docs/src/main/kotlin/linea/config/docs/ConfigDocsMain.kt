package linea.config.docs

import java.nio.file.Path
import kotlin.system.exitProcess

/**
 * Loads a [ConfigDocsSpec] by its fully-qualified class name. The spec is expected to be a Kotlin
 * `object` (or a class with a no-arg constructor). The class name is supplied by the Gradle task
 * (from the app's `configDocs { spec = "…" }` configuration), which keeps the wiring declarative
 * and avoids a `META-INF/services` file.
 */
internal fun loadSpec(className: String): ConfigDocsSpec {
  val kClass = Class.forName(className).kotlin
  val instance = kClass.objectInstance ?: kClass.java.getDeclaredConstructor().newInstance()
  return instance as? ConfigDocsSpec
    ?: error("$className does not implement ${ConfigDocsSpec::class.qualifiedName}")
}

private fun specFrom(args: Array<String>): ConfigDocsSpec {
  val className = args.firstOrNull()
    ?: error("Missing ConfigDocsSpec class name argument (configure `configDocs { spec = \"…\" }`)")
  return loadSpec(className)
}

/**
 * Entry point for the `checkConfigDocs` Gradle task. Validates that every config key of the
 * given [ConfigDocsSpec] (named in `args[0]`) is documented and exits non-zero on any violation.
 */
object ConfigDocsCheckMain {
  @JvmStatic
  fun main(args: Array<String>) {
    val spec = specFrom(args)
    exitProcess(ConfigDocsRunner.runCheck(spec.files, spec.sectionDetector))
  }
}

/**
 * Entry point for the `generateConfigDocs` Gradle task. Regenerates the committed JSON schema and
 * Markdown reference for the given [ConfigDocsSpec] (named in `args[0]`).
 */
object ConfigDocsGenerateMain {
  @JvmStatic
  fun main(args: Array<String>) {
    val spec = specFrom(args)
    ConfigDocsRunner.generate(
      files = spec.files,
      sectionDetector = spec.sectionDetector,
      jsonSchemaPath = Path.of(spec.jsonSchemaPath),
      markdownPath = Path.of(spec.markdownPath),
      markdownTitle = spec.markdownTitle,
      regenerateCommand = spec.regenerateCommand,
    )
  }
}
