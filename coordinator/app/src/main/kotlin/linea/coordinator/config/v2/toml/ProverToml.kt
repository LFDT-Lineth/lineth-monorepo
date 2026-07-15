package linea.coordinator.config.v2.toml

import linea.coordinator.clients.prover.FileBasedProverConfig
import linea.coordinator.clients.prover.ProverConfig
import linea.coordinator.clients.prover.ProversConfig
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

data class ProverToml(
  @param:ConfigDoc(
    description = "Filename suffix appended while the coordinator is still writing a request file, " +
      "so provers ignore partially-written requests.",
    default = ".inprogress_coordinator_writing",
  )
  val fsInprogressRequestWritingSuffix: String = ".inprogress_coordinator_writing",
  @param:ConfigDoc(
    description = "Regex matching filenames a prover has claimed and is working on, so the " +
      "coordinator treats them as in-progress.",
    default = "\\.inprogress\\.prover.*",
  )
  val fsInprogressProvingSuffixPattern: String = "\\.inprogress\\.prover.*",
  @param:ConfigDoc(
    description = "Interval between scans of the prover response directories for new responses.",
    default = "PT15S",
  )
  val fsPollingInterval: Duration = 15.seconds,
  @param:ConfigDoc(
    description = "Maximum time to wait for a prover response before timing out. Defaults to no timeout.",
    default = "infinite",
  )
  val fsPollingTimeout: Duration = Duration.INFINITE,
  @param:ConfigSection("Execution (block) prover request/response directories.")
  val execution: ProverDirectoriesToml,
  @param:ConfigSection("Blob compression prover request/response directories.")
  val blobCompression: ProverDirectoriesToml,
  @param:ConfigSection("Invalidity prover request/response directories; omit to disable.")
  val invalidity: ProverDirectoriesToml? = null,
  @param:ConfigSection("Proof aggregation prover request/response directories.")
  val proofAggregation: ProverDirectoriesToml,
  @param:ConfigDoc(
    description = "Inclusive L2 block number at which to switch from this prover to the `new` prover. " +
      "Mutually exclusive with switchBlockTimestamp.",
    example = "1000000",
  )
  val switchBlockNumberInclusive: ULong? = null,
  @param:ConfigDoc(
    description = "Timestamp at which to switch from this prover to the `new` prover. " +
      "Mutually exclusive with switchBlockNumberInclusive.",
    example = "2024-01-01T00:00:00Z",
  )
  val switchBlockTimestamp: Instant? = null,
  @param:ConfigSection("Next prover version to switch over to at the configured switch block/timestamp.")
  val new: ProverToml? = null,
  @param:ConfigDoc(
    description = "Whether to delete request files after their responses are processed.",
    default = "false",
  )
  val enableRequestFilesCleanup: Boolean = false,
) {
  data class ProverDirectoriesToml(
    @param:ConfigDoc(
      description = "Directory the coordinator writes prover request files to.",
      example = "/data/prover/v3/execution/requests",
    )
    val fsRequestsDirectory: String,
    @param:ConfigDoc(
      description = "Directory the coordinator reads prover response files from.",
      example = "/data/prover/v3/execution/responses",
    )
    val fsResponsesDirectory: String,
  )

  fun reified(): ProversConfig {
    val mergedSwitchBlockNumberInclusive = switchBlockNumberInclusive ?: new?.switchBlockNumberInclusive
    val mergedSwitchBlockTimestamp = switchBlockTimestamp ?: new?.switchBlockTimestamp
    require(!(mergedSwitchBlockNumberInclusive != null && mergedSwitchBlockTimestamp != null)) {
      "Only one of switchBlockNumberInclusive and switchBlockTimestamp may be set in [prover] config"
    }
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
        invalidity = this.invalidity?.let {
          FileBasedProverConfig(
            requestsDirectory = Path.of(this.invalidity.fsRequestsDirectory),
            responsesDirectory = Path.of(this.invalidity.fsResponsesDirectory),
            inprogressProvingSuffixPattern = this.fsInprogressProvingSuffixPattern,
            inprogressRequestWritingSuffix = this.fsInprogressRequestWritingSuffix,
            pollingInterval = this.fsPollingInterval,
            pollingTimeout = this.fsPollingTimeout,
          )
        },
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
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
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
          invalidity = newProverConfig.invalidity?.let {
            FileBasedProverConfig(
              requestsDirectory = Path.of(newProverConfig.invalidity.fsRequestsDirectory),
              responsesDirectory = Path.of(newProverConfig.invalidity.fsResponsesDirectory),
              inprogressProvingSuffixPattern = newProverConfig.fsInprogressProvingSuffixPattern,
              inprogressRequestWritingSuffix = newProverConfig.fsInprogressRequestWritingSuffix,
              pollingInterval = newProverConfig.fsPollingInterval,
              pollingTimeout = newProverConfig.fsPollingTimeout,
            )
          },
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
