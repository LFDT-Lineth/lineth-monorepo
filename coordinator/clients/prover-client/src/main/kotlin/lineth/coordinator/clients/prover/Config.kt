package lineth.coordinator.clients.prover

import java.net.URL
import java.nio.file.Path
import kotlin.time.Duration
import kotlin.time.Instant

data class ProverConfigSwitch<TProverConfig>(
  val current: TProverConfig,
  val next: TProverConfig? = null,
)

data class ProversConfig<TProverConfig>(
  val proverSwitch: ProverConfigSwitch<TProverConfig>,
  val switchBlockNumberInclusive: ULong?,
  val switchBlockTimestamp: Instant?,
  val enableRequestFilesCleanup: Boolean = false,
)

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
  val fileBased: FileBasedProverConfig?,
  val restfulBased: RestfulBasedProverConfig?,
  val programId: String,
  val provingSystemVersion: String,
  val forkName: String,
) {
  init {
    require((fileBased != null) != (restfulBased != null)) {
      "Either FileBased or restfulBased must be configured but not both"
    }
  }
}

data class RestfulBasedProverConfig(
  val endpoint: URL,
  val restfulApiBasePath: String,
  val restfulApiVersion: String,
  val pollingInterval: Duration,
  val pollingTimeout: Duration,
)

data class FileBasedProverConfig(
  val requestsDirectory: Path,
  val responsesDirectory: Path,
  val inprogressProvingSuffixPattern: String,
  val inprogressRequestWritingSuffix: String,
  val pollingInterval: Duration,
  val pollingTimeout: Duration,
)
