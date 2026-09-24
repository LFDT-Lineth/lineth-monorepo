package lineth.coordinator.clients.prover

import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Instant

data class ProverConfigSwitch(
  val current: GenericProverConfig,
  val next: GenericProverConfig? = null,
)

data class ProversConfig(
  val proverSwitch: ProverConfigSwitch,
  val switchBlockNumberInclusive: ULong?,
  val switchBlockTimestamp: Instant?,
  val enableRequestFilesCleanup: Boolean = false,
)

data class GenericProverConfig(
  val preRiscvConfig: PreRiscvProverConfig? = null,
  val riscvConfig: ProverConfig? = null,
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
  val invalidity: FileBasedProverConfig? = null,
)

data class ProverConfig(
  val l2Execution: ProverClientConfig,
  val rollup: ProverClientConfig,
  val rollupAggregation: ProverClientConfig,
)

data class ProverClientConfig(
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
