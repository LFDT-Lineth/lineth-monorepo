package lineth.coordinator.config.v2.toml

import linea.config.docs.ConfigDoc
import linea.config.docs.ConfigSection
import lineth.coordinator.config.v2.FileBasedProverConfig
import lineth.coordinator.config.v2.FileBasedRiscvProverConfig
import lineth.coordinator.config.v2.PreRiscvProverConfig
import lineth.coordinator.config.v2.ProverConfig
import lineth.coordinator.config.v2.ProversConfig
import lineth.coordinator.config.v2.RiscvProverConfig
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
  @param:ConfigDoc(
    description = "Proof transport: `file` or a transport registered through a ProverClientFactoryBuilder. " +
      "Only `file` is supported for pre_riscv provers.",
    default = "file",
  )
  val transport: String = RiscvProverConfig.FILE_TRANSPORT,
  @param:ConfigSection("Execution (block) prover request/response directories.")
  val execution: FileBasedProverConfigToml? = null,
  @param:ConfigSection("Blob compression prover request/response directories.")
  val blobCompression: FileBasedProverConfigToml? = null,
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
      description = "Directory the coordinator writes prover request files to. Required when transport is `file`.",
      example = "/data/prover/v3/execution/requests",
    )
    val fsRequestsDirectory: String? = null,
    @param:ConfigDoc(
      description = "Directory the coordinator reads prover response files from. Required when transport is `file`.",
      example = "/data/prover/v3/execution/responses",
    )
    val fsResponsesDirectory: String? = null,
    @param:ConfigDoc(
      description = "Guest program identifier for the RISC-V prover.",
      example = "0xabcdef1234567890",
    )
    val programId: String? = null,
  )

  val isFileTransport: Boolean get() = transport.equals(RiscvProverConfig.FILE_TRANSPORT, ignoreCase = true)

  init {
    require(transport.isNotBlank()) { "transport must not be blank" }
  }

  private fun toFileBasedProverConfig(proverConfigToml: FileBasedProverConfigToml): FileBasedProverConfig =
    FileBasedProverConfig(
      requestsDirectory = Path.of(proverConfigToml.fsRequestsDirectory!!),
      responsesDirectory = Path.of(proverConfigToml.fsResponsesDirectory!!),
      inprogressProvingSuffixPattern = fsInprogressProvingSuffixPattern,
      inprogressRequestWritingSuffix = fsInprogressRequestWritingSuffix,
      pollingInterval = fsPollingInterval,
      pollingTimeout = fsPollingTimeout,
    )

  private fun toRiscvProverConfig(
    blockToml: FileBasedProverConfigToml,
    programId: String,
    forkName: String,
  ): FileBasedRiscvProverConfig =
    FileBasedRiscvProverConfig(
      fileBased = if (isFileTransport) toFileBasedProverConfig(blockToml) else null,
      programId = programId,
      provingSystemVersion = provingSystemVersion!!,
      forkName = forkName,
      pollingInterval = fsPollingInterval,
    )

  private fun toPreRiscvProverConfig(t: ProverToml): PreRiscvProverConfig =
    PreRiscvProverConfig(
      execution = t.toFileBasedProverConfig(t.execution!!),
      blobCompression = t.toFileBasedProverConfig(t.blobCompression!!),
      proofAggregation = t.toFileBasedProverConfig(t.proofAggregation!!),
    )

  private fun toProverConfig(t: ProverToml): RiscvProverConfig =
    RiscvProverConfig(
      l2Execution = t.toRiscvProverConfig(t.l2Execution!!, t.l2Execution.programId!!, t.forkName!!),
      rollup = t.toRiscvProverConfig(t.rollup!!, t.rollup.programId!!, t.forkName),
      rollupAggregation =
      t.toRiscvProverConfig(t.rollupAggregation!!, t.rollupAggregation.programId!!, t.forkName),
      transport = t.transport,
    )

  private fun requireFsDirectories(vararg blocks: FileBasedProverConfigToml) {
    require(blocks.all { it.fsRequestsDirectory != null && it.fsResponsesDirectory != null }) {
      "fs-requests-directory and fs-responses-directory are required for transport=" +
        RiscvProverConfig.FILE_TRANSPORT
    }
  }

  fun validateProverToml(proverToml: ProverToml) {
    when (proverToml.type) {
      ProverType.PRE_RISCV -> {
        require(
          proverToml.execution != null &&
            proverToml.blobCompression != null && proverToml.proofAggregation != null,
        ) {
          "Prover type of PRE-RISCV must configure execution, blobCompression, and proofAggregation"
        }
        require(proverToml.isFileTransport) {
          "Prover type of PRE-RISCV only supports transport=${RiscvProverConfig.FILE_TRANSPORT}"
        }
        requireFsDirectories(proverToml.execution, proverToml.blobCompression, proverToml.proofAggregation)
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
        if (proverToml.isFileTransport) {
          requireFsDirectories(proverToml.l2Execution, proverToml.rollup, proverToml.rollupAggregation)
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
    if (this.type == ProverType.RISCV && this.new != null) {
      require(this.new.type == ProverType.RISCV) {
        "Prover type of new must be RISCV if the current prover type is RISCV"
      }
    }
    validateProverToml(this)
    this.new?.run(::validateProverToml)

    fun buildGenericProverConfig(proverToml: ProverToml): ProverConfig {
      return ProverConfig(
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
      currentProver = buildGenericProverConfig(this),
      nextProver = this.new?.let { buildGenericProverConfig(it) },
      switchBlockNumberInclusive = mergedSwitchBlockNumberInclusive,
      switchBlockTimestamp = mergedSwitchBlockTimestamp,
      enableRequestFilesCleanup = this.enableRequestFilesCleanup,
    )
  }
}
