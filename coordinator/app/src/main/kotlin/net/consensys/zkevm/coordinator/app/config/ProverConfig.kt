package net.consensys.zkevm.coordinator.app.config

import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import java.nio.file.Path
import java.time.Duration
import kotlin.time.Duration.Companion.hours
import kotlin.time.Duration.Companion.seconds
import kotlin.time.toJavaDuration
import kotlin.time.toKotlinDuration

data class ProverConfigTomlDto(
  val switchBlockNumberInclusive: Long? = null,
  var fsInprogressRequestWritingSuffix: String? = null,
  var fsInprogressProvingSuffixPattern: String? = null,
  var fsPollingInterval: Duration? = null,
  var fsPollingTimeout: Duration? = null,
  val execution: FileSystemTomlDto,
  val blobCompression: FileSystemTomlDto,
  val proofAggregation: FileSystemTomlDto,
  val new: ProverConfigTomlDto? = null,
) {
  private fun asProverConfig(): ProverConfig {
    return ProverConfig(
      execution = execution.toDomain(),
      blobCompression = blobCompression.toDomain(),
      proofAggregation = proofAggregation.toDomain(),
    )
  }

  fun reified(): ProversConfig {
    fsInprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix ?: ".inprogress_coordinator_writing"
    fsInprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern ?: "\\.inprogress\\.prover.*"
    fsPollingInterval = fsPollingInterval ?: 1.seconds.toJavaDuration()
    fsPollingTimeout = fsPollingTimeout ?: 3.hours.toJavaDuration()
    execution.reifyWithRootDefaults(this)
    blobCompression.reifyWithRootDefaults(this)
    proofAggregation.reifyWithRootDefaults(this)

    if (new != null) {
      if (new.switchBlockNumberInclusive == null) {
        throw IllegalArgumentException("switchBlockNumberInclusive must be set when new prover is configured")
      }
      new.fsInprogressProvingSuffixPattern = new.fsInprogressProvingSuffixPattern
        ?: fsInprogressProvingSuffixPattern
      new.fsInprogressRequestWritingSuffix = new.fsInprogressRequestWritingSuffix
        ?: fsInprogressRequestWritingSuffix
      new.fsPollingInterval = new.fsPollingInterval ?: fsPollingInterval
      new.fsPollingTimeout = new.fsPollingTimeout ?: fsPollingTimeout
      new.execution.reifyWithRootDefaults(new)
      new.blobCompression.reifyWithRootDefaults(new)
      new.proofAggregation.reifyWithRootDefaults(new)
    }

    return ProversConfig(
      proverA = this.asProverConfig(),
      switchBlockNumberInclusive = new?.switchBlockNumberInclusive?.toULong(),
      proverB = new?.asProverConfig(),
    )
  }
}

data class FileSystemTomlDto(
  internal val fsRequestsDirectory: Path,
  internal val fsResponsesDirectory: Path,
  internal var fsInprogressRequestWritingSuffix: String?,
  internal var fsInprogressProvingSuffixPattern: String?,
  internal var fsPollingInterval: Duration?,
  internal var fsPollingTimeout: Duration?,
) {
  internal fun reifyWithRootDefaults(rootConfig: ProverConfigTomlDto) {
    fsInprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix
      ?: rootConfig.fsInprogressRequestWritingSuffix
    fsInprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern
      ?: rootConfig.fsInprogressProvingSuffixPattern
    fsPollingInterval = fsPollingInterval ?: rootConfig.fsPollingInterval
    fsPollingTimeout = fsPollingTimeout ?: rootConfig.fsPollingTimeout
  }

  fun toDomain(): FileBasedProverConfig {
    return FileBasedProverConfig(
      requestsDirectory = fsRequestsDirectory,
      responsesDirectory = fsResponsesDirectory,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix!!,
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern!!,
      pollingInterval = fsPollingInterval!!.toKotlinDuration(),
      pollingTimeout = fsPollingTimeout!!.toKotlinDuration(),
    )
  }
}
