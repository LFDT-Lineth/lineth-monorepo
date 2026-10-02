package lineth.coordinator.config.v2

import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Instant

data class ProversConfig(
  val currentProver: ProverConfig,
  val nextProver: ProverConfig? = null,
  val switchBlockNumberInclusive: ULong?,
  val switchBlockTimestamp: Instant?,
  val enableRequestFilesCleanup: Boolean = false,
)

data class ProverConfig(
  val preRiscvConfig: PreRiscvProverConfig? = null,
  val riscvConfig: RiscvProverConfig? = null,
) {
  init {
    require((preRiscvConfig != null) != (riscvConfig != null)) {
      "Either preRiscvConfig or riscvConfig must be configured but not both"
    }
  }
}

data class PreRiscvProverConfig(
  val execution: FileBasedProverConfig,
  val blobCompression: FileBasedProverConfig,
  val proofAggregation: FileBasedProverConfig,
)

data class RiscvProverConfig(
  val l2Execution: FileBasedRiscvProverConfig,
  val rollup: FileBasedRiscvProverConfig,
  val rollupAggregation: FileBasedRiscvProverConfig,
)

data class FileBasedRiscvProverConfig(
  val fileBased: FileBasedProverConfig,
  val programId: String,
  val provingSystemVersion: String,
  val forkName: String,
)

data class FileBasedProverConfig(
  val requestsDirectory: Path,
  val responsesDirectory: Path,
  val inprogressProvingSuffixPattern: String,
  val inprogressRequestWritingSuffix: String,
  val pollingInterval: Duration,
  val pollingTimeout: Duration,
)
