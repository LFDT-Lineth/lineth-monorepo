package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.clients.prover.FileBasedProverConfig
import lineth.coordinator.clients.prover.PreRiscvProverConfig
import lineth.coordinator.clients.prover.ProverClientConfig
import lineth.coordinator.clients.prover.ProverConfig
import lineth.coordinator.clients.prover.ProverConfigSwitch
import lineth.coordinator.clients.prover.ProversConfig
import lineth.coordinator.clients.prover.RestfulBasedProverConfig
import java.net.URL
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

data class FileBasedProverConfigToml(
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

data class ProverConfigToml(
  @param:ConfigDoc(
    description = "Guest program identifier for the RISC-V prover.",
    example = "0xabcdef1234567890",
  )
  val programId: String,
  @param:ConfigSection(
    description = "Prover request/response directories for the file-based RISC-V prover.",
  )
  val fileBasedFolderConfig: FileBasedProverConfigToml? = null,
)

data class PreRiscvProverToml(
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
  val execution: FileBasedProverConfigToml,
  @param:ConfigSection("Blob compression prover request/response directories.")
  val blobCompression: FileBasedProverConfigToml,
  @param:ConfigSection("Invalidity prover request/response directories; omit to disable.")
  val invalidity: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Proof aggregation prover request/response directories.")
  val proofAggregation: FileBasedProverConfigToml,
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
  val new: PreRiscvProverToml? = null,
  @param:ConfigDoc(
    description = "Whether to delete request files after their responses are processed.",
    default = "false",
  )
  val enableRequestFilesCleanup: Boolean = false,
) {
  private fun toFileBasedProverConfig(proverConfigToml: FileBasedProverConfigToml): FileBasedProverConfig =
    FileBasedProverConfig(
      requestsDirectory = Path.of(proverConfigToml.fsRequestsDirectory),
      responsesDirectory = Path.of(proverConfigToml.fsResponsesDirectory),
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix,
      pollingInterval = fsPollingInterval,
      pollingTimeout = fsPollingTimeout,
    )

  private fun toPreRiscvProverConfig(t: PreRiscvProverToml): PreRiscvProverConfig =
    PreRiscvProverConfig(
      execution = t.toFileBasedProverConfig(t.execution),
      blobCompression = t.toFileBasedProverConfig(t.blobCompression),
      invalidity = t.invalidity?.let { t.toFileBasedProverConfig(it) },
      proofAggregation = t.toFileBasedProverConfig(t.proofAggregation),
    )

  fun reified(): ProversConfig<PreRiscvProverConfig> {
    val mergedSwitchBlockNumberInclusive = switchBlockNumberInclusive ?: new?.switchBlockNumberInclusive
    val mergedSwitchBlockTimestamp = switchBlockTimestamp ?: new?.switchBlockTimestamp
    require(!(mergedSwitchBlockNumberInclusive != null && mergedSwitchBlockTimestamp != null)) {
      "Only one of switchBlockNumberInclusive and switchBlockTimestamp may be set in [prover] config"
    }
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = toPreRiscvProverConfig(this),
        next = this.new?.let { toPreRiscvProverConfig(it) },
      ),
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
      enableRequestFilesCleanup = this.enableRequestFilesCleanup,
    )
  }
}

data class ProverToml(
  @param:ConfigSection("L2 execution RISC-V prover config.")
  val l2Execution: ProverConfigToml,
  @param:ConfigSection("Rollup RISC-V prover config.")
  val rollup: ProverConfigToml,
  @param:ConfigSection("Rollup aggregation RISC-V prover config.")
  val rollupAggregation: ProverConfigToml,
  @param:ConfigDoc(
    description = "Filename suffix appended while the coordinator is still writing a request file, " +
      "so provers ignore partially-written requests. Effective only if the prover is in file-based transport",
    default = ".inprogress_coordinator_writing",
  )
  val fsInprogressRequestWritingSuffix: String = ".inprogress_coordinator_writing",
  @param:ConfigDoc(
    description = "Regex matching filenames a prover has claimed and is working on, so the " +
      "coordinator treats them as in-progress. Effective only if the prover is in file-based transport",
    default = "\\.inprogress\\.prover.*",
  )
  val fsInprogressProvingSuffixPattern: String = "\\.inprogress\\.prover.*",
  @param:ConfigDoc(
    description = "Whether to delete request files after their responses are processed. " +
      "Effective only if the prover is in file-based transport",
    default = "false",
  )
  val fsEnableRequestFilesCleanup: Boolean = false,
  @param:ConfigDoc(
    description = "URL endpoint of the RESTful prover gateway service. " +
      "Effective only if the prover is in RESTful transport",
    example = "http://127.0.0.1:8090/",
  )
  val restfulEndpoint: URL? = null,
  @param:ConfigDoc(
    description = "Base path the JSON API is served under the RESTful prover gateway service. " +
      "(\"/api\" -> /api/health, /api/v1/...); \"\" or \"/\" serves " +
      "the API at the root. Effective only if the prover is in RESTful transport",
    default = "/api",
  )
  val restfulApiBasePath: String = "/api",
  @param:ConfigDoc(
    description = "Version of the JSON API served under the RESTful prover gateway service. " +
      "Effective only if the prover is in RESTful transport",
    default = "v1",
  )
  val restfulApiVersion: String = "v1",
  @param:ConfigDoc(
    description = "Interval between scans of the prover response for new responses.",
    default = "PT15S",
  )
  val pollingInterval: Duration = 15.seconds,
  @param:ConfigDoc(
    description = "Maximum time to wait for a prover response before timing out. Defaults to no timeout.",
    default = "infinite",
  )
  val pollingTimeout: Duration = Duration.INFINITE,
  @param:ConfigDoc(
    description = "L2 EVM fork name included in RISC-V execution proof requests (e.g. \"amsterdam\").",
    example = "amsterdam",
  )
  val forkName: String,
  @param:ConfigDoc(
    description = "Version for the RISC-V proving system.",
    example = "0xabcdef1234567890",
  )
  val provingSystemVersion: String,
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
) {
  private fun toFileBasedProverConfig(proverConfigToml: ProverConfigToml): FileBasedProverConfig {
    return FileBasedProverConfig(
      requestsDirectory = Path.of(proverConfigToml.fileBasedFolderConfig!!.fsRequestsDirectory),
      responsesDirectory = Path.of(proverConfigToml.fileBasedFolderConfig.fsResponsesDirectory),
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix,
      pollingInterval = pollingInterval,
      pollingTimeout = pollingTimeout,
    )
  }

  private fun toRestfulBasedProverConfig(): RestfulBasedProverConfig {
    return RestfulBasedProverConfig(
      endpoint = restfulEndpoint!!,
      restfulApiBasePath = restfulApiBasePath,
      restfulApiVersion = restfulApiVersion,
      pollingInterval = pollingInterval,
      pollingTimeout = pollingTimeout,
    )
  }

  private fun toProverClientConfig(proverConfigToml: ProverConfigToml): ProverClientConfig {
    require(proverConfigToml.fileBasedFolderConfig != null || restfulEndpoint != null) {
      "If fileBasedFolderConfig is not defined then restfulEndpoint must be defined in [prover] config"
    }
    return ProverClientConfig(
      fileBased = if (proverConfigToml.fileBasedFolderConfig != null) {
        toFileBasedProverConfig(proverConfigToml)
      } else {
        null
      },
      restfulBased = if (proverConfigToml.fileBasedFolderConfig == null) {
        toRestfulBasedProverConfig()
      } else {
        null
      },
      programId = proverConfigToml.programId,
      provingSystemVersion = provingSystemVersion,
      forkName = forkName,
    )
  }

  private fun toProverConfig(t: ProverToml): ProverConfig =
    ProverConfig(
      l2Execution = t.toProverClientConfig(t.l2Execution),
      rollup = t.toProverClientConfig(t.rollup),
      rollupAggregation = t.toProverClientConfig(t.rollupAggregation),
    )

  fun reified(): ProversConfig<ProverConfig> {
    val mergedSwitchBlockNumberInclusive = switchBlockNumberInclusive ?: new?.switchBlockNumberInclusive
    val mergedSwitchBlockTimestamp = switchBlockTimestamp ?: new?.switchBlockTimestamp
    require(!(mergedSwitchBlockNumberInclusive != null && mergedSwitchBlockTimestamp != null)) {
      "Only one of switchBlockNumberInclusive and switchBlockTimestamp may be set in [prover] config"
    }
    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = toProverConfig(this),
        next = this.new?.let { toProverConfig(it) },
      ),
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
      enableRequestFilesCleanup = this.fsEnableRequestFilesCleanup,
    )
  }
}
