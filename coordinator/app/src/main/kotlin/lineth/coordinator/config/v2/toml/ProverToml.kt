package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.clients.prover.FileBasedProverConfig
import lineth.coordinator.clients.prover.ProverConfig
import lineth.coordinator.clients.prover.ProversConfig
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
  val execution: ProverConfigToml,
  @param:ConfigSection("Blob compression prover request/response directories.")
  val blobCompression: ProverConfigToml? = null,
  @param:ConfigSection("Rollup prover config.")
  val rollup: ProverConfigToml? = null,
  @param:ConfigSection("Invalidity prover request/response directories; omit to disable.")
  val invalidity: ProverConfigToml? = null,
  @param:ConfigSection("Proof aggregation prover request/response directories.")
  val proofAggregation: ProverConfigToml,
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
  @param:ConfigDoc(
    description = "L2 EVM fork name included in RISC-V execution proof requests (e.g. \"cancun\"). " +
      "Required for the RISC-V prover; omit for the standard prover.",
    example = "cancun",
  )
  val forkName: String? = null,
) {
  init {
    require(blobCompression != null || rollup != null) {
      "Either blobCompression or rollup must be defined in prover config."
    }
    require(blobCompression == null || rollup == null) {
      "Only one of blobCompression or rollup may be defined in prover config."
    }
  }

  data class ProverConfigToml(
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
    @param:ConfigDoc(
      description = "Guest program identifier for the RISC-V prover. Required when this config is used " +
        "as the riscvProver; omit for the standard (EVM) prover.",
      example = "0xabcdef1234567890",
    )
    val guestProgramId: String? = null,
  )

  private fun toFileBasedProverConfig(dirs: ProverConfigToml): FileBasedProverConfig =
    FileBasedProverConfig(
      requestsDirectory = Path.of(dirs.fsRequestsDirectory),
      responsesDirectory = Path.of(dirs.fsResponsesDirectory),
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix,
      pollingInterval = fsPollingInterval,
      pollingTimeout = fsPollingTimeout,
      guestProgramId = dirs.guestProgramId,
    )

  private fun toProverConfig(t: ProverToml): ProverConfig =
    ProverConfig(
      execution = t.toFileBasedProverConfig(t.execution),
      blobCompression = t.blobCompression?.let { t.toFileBasedProverConfig(it) },
      rollup = t.rollup?.let { t.toFileBasedProverConfig(it) },
      invalidity = t.invalidity?.let { t.toFileBasedProverConfig(it) },
      proofAggregation = t.toFileBasedProverConfig(t.proofAggregation),
      forkName = t.forkName,
    )

  fun reified(): ProversConfig {
    val mergedSwitchBlockNumberInclusive = switchBlockNumberInclusive ?: new?.switchBlockNumberInclusive
    val mergedSwitchBlockTimestamp = switchBlockTimestamp ?: new?.switchBlockTimestamp
    require(!(mergedSwitchBlockNumberInclusive != null && mergedSwitchBlockTimestamp != null)) {
      "Only one of switchBlockNumberInclusive and switchBlockTimestamp may be set in [prover] config"
    }
    return ProversConfig(
      proverA = toProverConfig(this),
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
      proverB = this.new?.let { toProverConfig(it) },
      enableRequestFilesCleanup = this.enableRequestFilesCleanup,
    )
  }
}
