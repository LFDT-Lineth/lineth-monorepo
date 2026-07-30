package linea.config.docs

import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path

class ConfigDocsRunnerTest {
  data class DocumentedToml(
    @param:ConfigDoc("The hostname.", example = "localhost")
    val hostname: String = "localhost",
  )

  data class UndocumentedToml(
    val hostname: String = "localhost",
  )

  private val detector = sectionByPackagePrefix("linea.config.docs")
  private fun files(root: kotlin.reflect.KClass<*>) = listOf(ConfigFileRoot("sample", "A sample file.", root))

  @Test
  fun `runCheck returns 0 and reports OK when fully documented`() {
    val out = StringBuilder()
    val code = ConfigDocsRunner.runCheck(files(DocumentedToml::class), detector, out = out, err = StringBuilder())
    assertThat(code).isEqualTo(0)
    assertThat(out.toString()).contains("all documented")
  }

  @Test
  fun `runCheck returns 1 and reports violations when undocumented`() {
    val err = StringBuilder()
    val code = ConfigDocsRunner.runCheck(files(UndocumentedToml::class), detector, out = StringBuilder(), err = err)
    assertThat(code).isEqualTo(1)
    assertThat(err.toString()).contains("hostname")
  }

  @Test
  fun `generate writes both artifacts and is idempotent`(@TempDir dir: Path) {
    val json = dir.resolve("schema.json")
    val md = dir.resolve("nested/reference.md")
    val logs = mutableListOf<String>()

    ConfigDocsRunner.generate(
      files(DocumentedToml::class),
      detector,
      json,
      md,
      "Sample Reference",
      "./gradlew generate",
    ) { logs.add(it) }

    assertThat(Files.readString(json)).contains("\"sample\"")
    assertThat(Files.readString(md)).contains("# Sample Reference")
    assertThat(logs).allMatch { it.startsWith("Wrote:") }

    // second run: unchanged content => reported as up to date
    val secondLogs = mutableListOf<String>()
    ConfigDocsRunner.generate(
      files(DocumentedToml::class),
      detector,
      json,
      md,
      "Sample Reference",
      "./gradlew generate",
    ) { secondLogs.add(it) }
    assertThat(secondLogs).allMatch { it.startsWith("Up to date:") }
  }
}
