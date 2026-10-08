package lineth.coordinator.config.v2

import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Duration.Companion.seconds
import kotlin.time.Instant

data class ProversConfig(
  val currentProver: ProverConfig,
  val nextProver: ProverConfig? = null,
  val switchBlockNumberInclusive: ULong?,
  val switchBlockTimestamp: Instant?,
  val enableRequestFilesCleanup: Boolean = false,
) {
  val hasRiscvProverConfig: Boolean = (currentProver.riscvConfig ?: nextProver?.riscvConfig) != null
}

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

/**
 * RISC-V prover settings of one proof type. [fileBased] is required only when [transport] is
 * [FILE_TRANSPORT]; other transports are provided by a custom `ProverClientFactoryBuilder`.
 */
data class FileBasedRiscvProverConfig(
  val fileBased: FileBasedProverConfig?,
  val programId: String,
  val provingSystemVersion: String,
  val forkName: String,
  val transport: String = FILE_TRANSPORT,
  val pollingInterval: Duration = fileBased?.pollingInterval ?: DEFAULT_POLLING_INTERVAL,
) {
  init {
    require(!isFileTransport || fileBased != null) {
      "fileBased config is required for transport=$FILE_TRANSPORT"
    }
  }

  val isFileTransport: Boolean get() = transport.equals(FILE_TRANSPORT, ignoreCase = true)

  fun requireFileBased(): FileBasedProverConfig =
    requireNotNull(fileBased) {
      "Prover transport '$transport' is not supported by the file-based prover client factory; " +
        "provide a ProverClientFactoryBuilder that handles it"
    }

  companion object {
    const val FILE_TRANSPORT = "file"
    val DEFAULT_POLLING_INTERVAL: Duration = 15.seconds
  }
}

data class FileBasedProverConfig(
  val requestsDirectory: Path,
  val responsesDirectory: Path,
  val inprogressProvingSuffixPattern: String,
  val inprogressRequestWritingSuffix: String,
  val pollingInterval: Duration,
  val pollingTimeout: Duration,
)
