package linea.coordinator.config.v2.toml

import net.consensys.zkevm.coordinator.clients.prover.FileBasedProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProverConfig
import net.consensys.zkevm.coordinator.clients.prover.ProversConfig
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds

data class ProverToml(
  val version: String,
  val fsInprogressRequestWritingSuffix: String = ".inprogress_coordinator_writing",
  val fsInprogressProvingSuffixPattern: String = "\\.inprogress\\.prover.*",
  val fsPollingInterval: Duration = 15.seconds,
  val fsPollingTimeout: Duration = Duration.INFINITE,
  val execution: ProverDirectoriesToml,
  val blobCompression: ProverDirectoriesToml,
  val proofAggregation: ProverDirectoriesToml,
  val switchBlockNumberInclusive: ULong? = null,
  val new: ProverToml? = null,
  val enableRequestFilesCleanup: Boolean = false,
) {
  data class ProverDirectoriesToml(
    val fsRequestsDirectory: String,
    val fsResponsesDirectory: String,
  )

  fun reified(): ProversConfig {
    return ProversConfig(
      proverA =
      ProverConfig(
        execution =
        FileBasedProverConfig(
          requestsDirectory = Path.of(this.execution.fsRequestsDirectory),
          responsesDirectory = Path.of(this.execution.fsResponsesDirectory),
          inprogressProvingSuffixPattern = this.fsInprogressProvingSuffixPattern,
          inprogressRequestWritingSuffix = this.fsInprogressRequestWritingSuffix,
          pollingInterval = this.fsPollingInterval,
          pollingTimeout = this.fsPollingTimeout,
        ),
        blobCompression =
        FileBasedProverConfig(
          requestsDirectory = Path.of(this.blobCompression.fsRequestsDirectory),
          responsesDirectory = Path.of(this.blobCompression.fsResponsesDirectory),
          inprogressProvingSuffixPattern = this.fsInprogressProvingSuffixPattern,
          inprogressRequestWritingSuffix = this.fsInprogressRequestWritingSuffix,
          pollingInterval = this.fsPollingInterval,
          pollingTimeout = this.fsPollingTimeout,
        ),
        proofAggregation =
        FileBasedProverConfig(
          requestsDirectory = Path.of(this.proofAggregation.fsRequestsDirectory),
          responsesDirectory = Path.of(this.proofAggregation.fsResponsesDirectory),
          inprogressProvingSuffixPattern = this.fsInprogressProvingSuffixPattern,
          inprogressRequestWritingSuffix = this.fsInprogressRequestWritingSuffix,
          pollingInterval = this.fsPollingInterval,
          pollingTimeout = this.fsPollingTimeout,
        ),
      ),
      switchBlockNumberInclusive = this.switchBlockNumberInclusive ?: this.new?.switchBlockNumberInclusive,
      proverB =
      this.new?.let { newProverConfig ->
        ProverConfig(
          execution =
          FileBasedProverConfig(
            requestsDirectory = Path.of(newProverConfig.execution.fsRequestsDirectory),
            responsesDirectory = Path.of(newProverConfig.execution.fsResponsesDirectory),
            inprogressProvingSuffixPattern = newProverConfig.fsInprogressProvingSuffixPattern,
            inprogressRequestWritingSuffix = newProverConfig.fsInprogressRequestWritingSuffix,
            pollingInterval = newProverConfig.fsPollingInterval,
            pollingTimeout = newProverConfig.fsPollingTimeout,
          ),
          blobCompression =
          FileBasedProverConfig(
            requestsDirectory = Path.of(newProverConfig.blobCompression.fsRequestsDirectory),
            responsesDirectory = Path.of(newProverConfig.blobCompression.fsResponsesDirectory),
            inprogressProvingSuffixPattern = newProverConfig.fsInprogressProvingSuffixPattern,
            inprogressRequestWritingSuffix = newProverConfig.fsInprogressRequestWritingSuffix,
            pollingInterval = newProverConfig.fsPollingInterval,
            pollingTimeout = newProverConfig.fsPollingTimeout,
          ),
          proofAggregation =
          FileBasedProverConfig(
            requestsDirectory = Path.of(newProverConfig.proofAggregation.fsRequestsDirectory),
            responsesDirectory = Path.of(newProverConfig.proofAggregation.fsResponsesDirectory),
            inprogressProvingSuffixPattern = newProverConfig.fsInprogressProvingSuffixPattern,
            inprogressRequestWritingSuffix = newProverConfig.fsInprogressRequestWritingSuffix,
            pollingInterval = newProverConfig.fsPollingInterval,
            pollingTimeout = newProverConfig.fsPollingTimeout,
          ),
        )
      },
      enableRequestFilesCleanup = this.enableRequestFilesCleanup,
    )
  }
}
