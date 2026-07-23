package linea.config.docs

/**
 * An application's config-docs configuration, loaded by the generic entry points
 * ([ConfigDocsCheckMain], [ConfigDocsGenerateMain]) from the fully-qualified class name passed by
 * the Gradle task.
 *
 * Implement this once per application as a Kotlin `object` (e.g. `CoordinatorConfigDocsSpec`) and
 * name it in the app's `build.gradle` via `configDocs { spec = "…CoordinatorConfigDocsSpec" }`.
 * This keeps all wiring declarative and app code free of check/generate entry-point boilerplate.
 *
 * @property files the config files to document, each pairing a label/description with a root class.
 * @property sectionDetector decides which parameter types are nested sections vs leaf values.
 * @property jsonSchemaPath committed JSON schema output path, relative to the repository root.
 * @property markdownPath committed Markdown reference output path, relative to the repository root.
 * @property markdownTitle the Markdown document's top-level title.
 * @property regenerateCommand the command shown in the Markdown "do not edit" banner.
 */
interface ConfigDocsSpec {
  val files: List<ConfigFileRoot>
  val sectionDetector: SectionDetector
  val jsonSchemaPath: String
  val markdownPath: String
  val markdownTitle: String
  val regenerateCommand: String
}
