package linea.coordinator.config.v2.docs

import java.nio.file.Files
import java.nio.file.Path

/**
 * Entry point for the `generateCoordinatorConfigDocs` Gradle task. Regenerates the committed
 * config documentation artifacts from the TOML schema classes.
 *
 * Paths are resolved relative to the task's working directory (the repository root). Optional
 * args override them: `args[0]` = JSON schema path, `args[1]` = Markdown reference path.
 */
object ConfigDocsGenerateMain {
  private const val DEFAULT_JSON_SCHEMA_PATH = "coordinator/app/src/docs/coordinator-config-schema.json"
  private const val DEFAULT_MARKDOWN_PATH = "docs/tech/components/coordinator-config-reference.md"

  @JvmStatic
  fun main(args: Array<String>) {
    val jsonSchemaPath = Path.of(args.getOrElse(0) { DEFAULT_JSON_SCHEMA_PATH })
    val markdownPath = Path.of(args.getOrElse(1) { DEFAULT_MARKDOWN_PATH })

    writeIfChanged(jsonSchemaPath, ConfigDocJsonGenerator.generate())
    writeIfChanged(markdownPath, ConfigDocMarkdownGenerator.generate())
  }

  private fun writeIfChanged(path: Path, content: String) {
    path.parent?.let { Files.createDirectories(it) }
    val existing = if (Files.exists(path)) Files.readString(path) else null
    if (existing == content) {
      println("Up to date: $path")
    } else {
      Files.writeString(path, content)
      println("Wrote: $path")
    }
  }
}
