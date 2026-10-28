package lineth.coordinator.config.v2.toml

import com.github.michaelbull.result.get
import org.assertj.core.api.Assertions.assertThat
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Path
import kotlin.io.path.writeText

class SmartContractErrorsLoadingTest {
  private val bundled = loadBundledSmartContractErrors().smartContractErrors

  @Test
  fun `bundled mapping is loaded when no override file is given`() {
    assertThat(bundled).containsEntry("0f06cd15", "DataAlreadySubmitted")

    val loaded = loadSmartContractErrors(overrideFile = null, strict = true).get()!!.smartContractErrors

    assertThat(loaded).isEqualTo(bundled)
  }

  @Test
  fun `override file entries extend and win over the bundled mapping`(@TempDir tempDir: Path) {
    val overrideFile = tempDir.resolve("override.toml")
    overrideFile.writeText(
      """
      [smart-contract-errors]
      "0f06cd15" = "CustomName"
      "deadbeef" = "CustomError"
      """.trimIndent(),
    )

    val loaded = loadSmartContractErrors(overrideFile, strict = true).get()!!.smartContractErrors

    assertThat(loaded).containsEntry("0f06cd15", "CustomName")
    assertThat(loaded).containsEntry("deadbeef", "CustomError")
    assertThat(loaded).hasSize(bundled.size + 1)
    assertThat(loaded["c01eab56"]).isEqualTo(bundled["c01eab56"])
  }
}
