package lineth.config.docs

import java.nio.file.Files
import java.nio.file.Path

/**
 * Reusable entry-point behaviour for the config-docs check and generate tasks, shared by every
 * application (coordinator, Maru, ...). Applications supply only their [ConfigFileRoot] list, a
 * [SectionDetector], and output locations; the walk/validate/render/write logic lives here.
 */
object ConfigDocsRunner {
  /**
   * Walks the [files], validates documentation completeness, and reports the outcome. Returns a
   * process exit code: 0 when everything is documented, 1 otherwise (with the violation report
   * written to [err]). Callers typically `exitProcess(...)` on the result.
   */
  fun runCheck(
    files: List<ConfigFileRoot>,
    sectionDetector: SectionDetector,
    out: Appendable = System.out,
    err: Appendable = System.err,
  ): Int {
    val keys = ConfigSchemaWalker.walkAll(files.map { it.rootClass }, sectionDetector)
    val violations = ConfigDocValidator.validate(keys)
    return if (violations.isEmpty()) {
      out.appendLine("Config docs OK: ${keys.size} keys, all documented.")
      0
    } else {
      err.appendLine(ConfigDocValidator.formatViolations(violations))
      1
    }
  }

  /**
   * Generates the JSON schema snapshot and Markdown reference, writing each only when its content
   * changes (so repeated runs are idempotent and produce clean diffs). When [mdxPartialPath] is
   * non-null, also writes an MDX-safe reference partial to that path (intended for a gitignored
   * build directory consumed by a publishing workflow); the committed Markdown and JSON outputs
   * are unaffected.
   */
  fun generate(
    files: List<ConfigFileRoot>,
    sectionDetector: SectionDetector,
    jsonSchemaPath: Path,
    markdownPath: Path,
    markdownTitle: String,
    regenerateCommand: String,
    mdxPartialPath: Path? = null,
    log: (String) -> Unit = ::println,
  ) {
    writeIfChanged(jsonSchemaPath, ConfigDocJsonGenerator.generate(files, sectionDetector), log)
    writeIfChanged(
      markdownPath,
      ConfigDocMarkdownGenerator.generate(files, sectionDetector, markdownTitle, regenerateCommand),
      log,
    )
    if (mdxPartialPath != null) {
      writeIfChanged(
        mdxPartialPath,
        ConfigDocMdxGenerator.generate(files, sectionDetector, regenerateCommand),
        log,
      )
    }
  }

  private fun writeIfChanged(path: Path, content: String, log: (String) -> Unit) {
    path.parent?.let { Files.createDirectories(it) }
    val existing = if (Files.exists(path)) Files.readString(path) else null
    if (existing == content) {
      log("Up to date: $path")
    } else {
      Files.writeString(path, content)
      log("Wrote: $path")
    }
  }
}
