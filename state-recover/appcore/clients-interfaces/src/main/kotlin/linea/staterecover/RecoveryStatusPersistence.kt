package linea.staterecover

import com.fasterxml.jackson.core.JacksonException
import com.fasterxml.jackson.module.kotlin.jacksonObjectMapper
import java.nio.file.Path

interface RecoveryStatusPersistence {
  fun saveRecoveryStartBlockNumber(recoveryStartBlockNumber: ULong)
  fun getRecoveryStartBlockNumber(): ULong?
}

class InMemoryRecoveryStatus : RecoveryStatusPersistence {
  private var recoveryStartBlockNumber: ULong? = null

  @Synchronized
  override fun saveRecoveryStartBlockNumber(recoveryStartBlockNumber: ULong) {
    this.recoveryStartBlockNumber = recoveryStartBlockNumber
  }

  @Synchronized
  override fun getRecoveryStartBlockNumber(): ULong? {
    return recoveryStartBlockNumber
  }
}

class FileBasedRecoveryStatusPersistence(
  filePath: Path
) : RecoveryStatusPersistence {
  // A little future proofing in case we need to change the file format in the future
  private enum class FileVersion {
    V1 // note: do not rename because it will fail to parse the file if already written
  }

  private data class RecoveryStatusEnvelopeDto(
    val version: FileVersion,
    val recoveryStatus: RecoveryStatusV1Dto?
  )

  private data class RecoveryStatusV1Dto(
    val recoveryStartBlockNumber: ULong
  )
  private val objectMapper = jacksonObjectMapper()
  private val file = filePath.toFile()
  private var currentStatus: RecoveryStatusV1Dto? = loadStatusFromFileOrCreateIfDoesNotExist()

  private fun saveToFile(status: RecoveryStatusV1Dto?) {
    file.writeText(
      objectMapper.writeValueAsString(
        RecoveryStatusEnvelopeDto(FileVersion.V1, status)
      )
    )
  }

  private fun loadStatusFromFileOrCreateIfDoesNotExist(): RecoveryStatusV1Dto? {
    // eager file write because if we cannot read/write the file, then application shall throw and fail early
    return if (!file.exists()) {
      // Create the file with an empty status
      saveToFile(status = null)
      null
    } else {
      // read status from file
      val envelope = try {
        objectMapper.readValue(file, RecoveryStatusEnvelopeDto::class.java)
      } catch (e: JacksonException) {
        throw IllegalStateException("failed to parse recovery status file: ${file.absolutePath} ", e)
      }
      envelope.recoveryStatus
    }
  }

  @Synchronized
  override fun saveRecoveryStartBlockNumber(recoveryStartBlockNumber: ULong) {
    saveToFile(status = RecoveryStatusV1Dto(recoveryStartBlockNumber))
    currentStatus = RecoveryStatusV1Dto(recoveryStartBlockNumber)
  }

  @Synchronized
  override fun getRecoveryStartBlockNumber(): ULong? {
    return currentStatus?.recoveryStartBlockNumber
  }
}
