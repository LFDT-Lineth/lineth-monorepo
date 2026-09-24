package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.clients.prover.FileBasedProverConfig
import lineth.coordinator.clients.prover.GenericProverConfig
import lineth.coordinator.clients.prover.PreRiscvProverConfig
import lineth.coordinator.clients.prover.ProverClientConfig
import lineth.coordinator.clients.prover.ProverConfig
import lineth.coordinator.clients.prover.ProverConfigSwitch
import lineth.coordinator.clients.prover.ProversConfig
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

data class ProverToml(
  @param:ConfigDoc(
    description = "Prover type: pre_riscv or riscv.",
    default = "pre_riscv",
  )
  val type: ProverType = ProverType.PRE_RISCV,
  @param:ConfigSection("Execution (block) prover request/response directories.")
  val execution: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Blob compression prover request/response directories.")
  val blobCompression: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Invalidity prover request/response directories; omit to disable.")
  val invalidity: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Proof aggregation prover request/response directories.")
  val proofAggregation: FileBasedProverConfigToml? = null,
  @param:ConfigSection("L2 execution RISC-V prover config.")
  val l2Execution: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Rollup RISC-V prover config.")
  val rollup: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Rollup aggregation RISC-V prover config.")
  val rollupAggregation: FileBasedProverConfigToml? = null,
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
  @param:ConfigDoc(
    description = "L2 EVM fork name included in RISC-V execution proof requests (e.g. \"amsterdam\").",
    example = "amsterdam",
  )
  val forkName: String? = null,
  @param:ConfigDoc(
    description = "Version for the RISC-V proving system.",
    example = "0xabcdef1234567890",
  )
  val provingSystemVersion: String? = null,
  @param:ConfigDoc(
    description = "Whether to delete request files after their responses are processed.",
    default = "false",
  )
  val enableRequestFilesCleanup: Boolean = false,
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
  enum class ProverType(val displayName: String) {
    PRE_RISCV("pre_riscv"),
    RISCV("riscv"),
  }

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
    @param:ConfigDoc(
      description = "Guest program identifier for the RISC-V prover.",
      example = "0xabcdef1234567890",
    )
    val programId: String? = null,
  )

  private fun toFileBasedProverConfig(proverConfigToml: FileBasedProverConfigToml): FileBasedProverConfig =
    FileBasedProverConfig(
      requestsDirectory = Path.of(proverConfigToml.fsRequestsDirectory),
      responsesDirectory = Path.of(proverConfigToml.fsResponsesDirectory),
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix,
      pollingInterval = fsPollingInterval,
      pollingTimeout = fsPollingTimeout,
    )

  private fun toPreRiscvProverConfig(t: ProverToml): PreRiscvProverConfig =
    PreRiscvProverConfig(
      execution = t.toFileBasedProverConfig(t.execution!!),
      blobCompression = t.toFileBasedProverConfig(t.blobCompression!!),
      invalidity = t.invalidity?.let { t.toFileBasedProverConfig(it) },
      proofAggregation = t.toFileBasedProverConfig(t.proofAggregation!!),
    )

  private fun toProverConfig(t: ProverToml): ProverConfig =
    ProverConfig(
      l2Execution = ProverClientConfig(
        fileBased = t.toFileBasedProverConfig(t.l2Execution!!),
        programId = t.l2Execution.programId!!,
        provingSystemVersion = t.provingSystemVersion!!,
        forkName = t.forkName!!,
      ),
      rollup = ProverClientConfig(
        fileBased = t.toFileBasedProverConfig(t.rollup!!),
        programId = t.rollup.programId!!,
        provingSystemVersion = t.provingSystemVersion,
        forkName = t.forkName,
      ),
      rollupAggregation = ProverClientConfig(
        fileBased = t.toFileBasedProverConfig(t.rollupAggregation!!),
        programId = t.rollupAggregation.programId!!,
        provingSystemVersion = t.provingSystemVersion,
        forkName = t.forkName,
      ),
    )

  fun validateProverToml(proverToml: ProverToml) {
    when (proverToml.type) {
      ProverType.PRE_RISCV -> {
        require(
          proverToml.execution != null &&
            proverToml.blobCompression != null && proverToml.proofAggregation != null,
        ) {
          "Prover type of PRE-RISCV must configure execution, blobCompression, and proofAggregation"
        }
      }
      ProverType.RISCV -> {
        require(
          proverToml.l2Execution != null &&
            proverToml.rollup != null && proverToml.rollupAggregation != null,
        ) {
          "Prover type of RISCV must configure l2Execution, rollup, and rollupAggregation"
        }
        require(
          proverToml.l2Execution.programId != null &&
            proverToml.rollup.programId != null && proverToml.rollupAggregation.programId != null,
        ) {
          "Prover type of RISCV must configure programId for l2Execution, rollup, and rollupAggregation"
        }
        require(proverToml.forkName != null && proverToml.provingSystemVersion != null) {
          "Prover type of RISCV must configure forkName and provingSystemVersion"
        }
      }
    }
  }

  fun reified(): ProversConfig {
    val mergedSwitchBlockNumberInclusive = switchBlockNumberInclusive ?: new?.switchBlockNumberInclusive
    val mergedSwitchBlockTimestamp = switchBlockTimestamp ?: new?.switchBlockTimestamp
    require(!(mergedSwitchBlockNumberInclusive != null && mergedSwitchBlockTimestamp != null)) {
      "Only one of switchBlockNumberInclusive and switchBlockTimestamp may be set in [prover] config"
    }
    if (mergedSwitchBlockTimestamp != null || mergedSwitchBlockNumberInclusive != null) {
      requireNotNull(this.new) {
        "prover.new must be configured if either switchBlockNumberInclusive or switchBlockTimestamp is set"
      }
    }
    if (this.type == ProverType.RISCV) {
      require(this.new?.type == ProverType.RISCV) {
        "Prover type of new must be RISCV if the current prover type is RISCV"
      }
    }
    validateProverToml(this)
    this.new?.run(::validateProverToml)

    fun buildGenericProverConfig(proverToml: ProverToml): GenericProverConfig {
      return GenericProverConfig(
        preRiscvConfig = if (proverToml.type == ProverType.PRE_RISCV) {
          toPreRiscvProverConfig(proverToml)
        } else {
          null
        },
        riscvConfig = if (proverToml.type == ProverType.RISCV) {
          toProverConfig(proverToml)
        } else {
          null
        },
      )
    }

    return ProversConfig(
      proverSwitch = ProverConfigSwitch(
        current = buildGenericProverConfig(this),
        next = this.new?.let { buildGenericProverConfig(it) },
      ),
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
      enableRequestFilesCleanup = this.enableRequestFilesCleanup,
    )
  }
}
