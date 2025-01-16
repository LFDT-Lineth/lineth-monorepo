package linea.staterecover

import org.assertj.core.api.Assertions.assertThat
import org.assertj.core.api.Assertions.assertThatThrownBy
import org.junit.jupiter.api.Test
import org.junit.jupiter.api.io.TempDir
import java.nio.file.Files
import java.nio.file.Path
import java.nio.file.attribute.PosixFilePermissions

class FileRecoveryStatusPersistenceTest {
  private lateinit var recoveryStatusPersistence: RecoveryStatusPersistence

  @Test
  fun `should return null when no recovery start block number is saved`(
    @TempDir tempDir: Path
  ) {
    val recoveryStatusPersistence = FileBasedRecoveryStatusPersistence(tempDir.resolve("recovery-status.json"))
    assertThat(recoveryStatusPersistence.getRecoveryStartBlockNumber()).isNull()
  }

  @Test
  fun `should return the saved recovery start block number`(
    @TempDir tempDir: Path
  ) {
    FileBasedRecoveryStatusPersistence(tempDir.resolve("recovery-status.json"))
      .also { persistence ->
        assertThat(persistence.saveRecoveryStartBlockNumber(10U))
        assertThat(persistence.getRecoveryStartBlockNumber()).isEqualTo(10UL)
      }

    // simulate application restart, the saved recovery start block number should be loaded from file OK
    FileBasedRecoveryStatusPersistence(tempDir.resolve("recovery-status.json"))
      .also { persistence ->
        assertThat(persistence.getRecoveryStartBlockNumber()).isEqualTo(10UL)
        assertThat(persistence.saveRecoveryStartBlockNumber(11U))
        assertThat(persistence.getRecoveryStartBlockNumber()).isEqualTo(11UL)
      }

    // simulate application restart, the saved recovery start block number should be loaded from file OK
    FileBasedRecoveryStatusPersistence(tempDir.resolve("recovery-status.json"))
      .also { persistence ->
        assertThat(persistence.getRecoveryStartBlockNumber()).isEqualTo(11UL)
      }
  }

  @Test
  fun `shall throw when it cannot create the file`(
    @TempDir tempDir: Path
  ) {
    val dirWithoutWritePermissions = tempDir.resolve("dir-without-write-permissions")

    Files.createDirectory(dirWithoutWritePermissions)
    Files.setPosixFilePermissions(dirWithoutWritePermissions, PosixFilePermissions.fromString("r-xr-xr-x"))

    val file = dirWithoutWritePermissions.resolve("recovery-status.json")

    assertThatThrownBy {
      recoveryStatusPersistence = FileBasedRecoveryStatusPersistence(file)
    }
      .hasMessageContaining(file.toString())
      .hasMessageContaining("Permission denied")
  }

  @Test
  fun `should throw error when file version is not supported`(
    @TempDir tempDir: Path
  ) {
    val invalidJsonPayload = """
      {
        "version": "2",
        "recoveryStatus": {
          "recoveryStartBlockNumber": 10
        }
      }
    """.trimIndent()
    val file = tempDir.resolve("recovery-status.json")
    Files.write(file, invalidJsonPayload.toByteArray())

    assertThatThrownBy {
      recoveryStatusPersistence = FileBasedRecoveryStatusPersistence(file)
    }
      .isInstanceOf(IllegalStateException::class.java)
      .hasMessageContaining(file.toString())
      .hasMessageContaining("parse")
  }
}
