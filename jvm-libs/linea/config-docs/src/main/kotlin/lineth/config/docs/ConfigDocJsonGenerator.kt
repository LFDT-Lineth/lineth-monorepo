package lineth.config.docs

import com.fasterxml.jackson.core.util.DefaultIndenter
import com.fasterxml.jackson.core.util.DefaultPrettyPrinter
import com.fasterxml.jackson.core.util.Separators
import com.fasterxml.jackson.databind.ObjectMapper

/**
 * Generates the machine-readable JSON schema snapshot of a config.
 *
 * Output is organised per config file (see [ConfigFileRoot]) because some files may share
 * top-level keys, then per config key sorted by path for stable diffs. This snapshot is committed
 * and consumed by config changelog tooling.
 *
 * Shape:
 * ```
 * {
 *   "coordinator": {
 *     "description": "Main Coordinator configuration.",
 *     "keys": {
 *       "database.hostname": {
 *         "type": "String", "section": false, "required": true, "default": null,
 *         "description": "...", "example": "postgres", "deprecated": false, "replacement": null
 *       }
 *     }
 *   }
 * }
 * ```
 */
object ConfigDocJsonGenerator {
  private val objectMapper: ObjectMapper = ObjectMapper()

  private val prettyPrinter: DefaultPrettyPrinter
    get() {
      val indenter = DefaultIndenter("  ", "\n")
      return DefaultPrettyPrinter()
        .withSeparators(Separators.createDefaultInstance().withObjectFieldValueSpacing(Separators.Spacing.AFTER))
        .apply {
          indentObjectsWith(indenter)
          indentArraysWith(indenter)
        }
    }

  fun generate(files: List<ConfigFileRoot>, sectionDetector: SectionDetector): String {
    val root = LinkedHashMap<String, Any>()
    for (file in files) {
      val keys = ConfigSchemaWalker.walk(file.rootClass, sectionDetector)
      val keysNode = LinkedHashMap<String, Any>()
      for (key in keys) {
        keysNode[key.path] = key.toJsonEntry()
      }
      root[file.label] = linkedMapOf(
        "description" to file.description,
        "keys" to keysNode,
      )
    }
    return objectMapper.writer(prettyPrinter).writeValueAsString(root) + "\n"
  }

  private fun ConfigKey.toJsonEntry(): Map<String, Any?> = linkedMapOf(
    "type" to type,
    "section" to isSection,
    "required" to required,
    "default" to default,
    "description" to description,
    "example" to example,
    "deprecated" to deprecated,
    "replacement" to replacement,
  )
}
